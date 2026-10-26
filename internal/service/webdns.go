package service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Resolver answers whether a name exists in DNS (any addresses = yes).
type Resolver func(ctx context.Context, name string) ([]string, error)

// LookupHost is the production Resolver: the Bridge server's own resolver.
func LookupHost(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, name)
}

// WebDNSSummary is what one scan's web-server step did.
type WebDNSSummary struct {
	Names        int `json:"names"`         // distinct linkable names served
	Created      int `json:"created"`       // new DNS records
	Linked       int `json:"linked"`        // host links added
	ServiceLinks int `json:"service_links"` // service links added
	Unresolved   int `json:"unresolved"`    // new names that don't resolve: not created
	Trashed      int `json:"trashed"`       // names whose record is in the trash: left alone
}

const (
	webDNSLookupTimeout = 3 * time.Second
	webDNSStepTimeout   = 20 * time.Second
	webDNSParallel      = 8
)

// dnsNameRe is DnsForm.tsx's DOMAIN_RE without the wildcard prefix.
var dnsNameRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9-]{2,63}$`)

// LinkableName normalizes a server_name / ServerName and reports whether it
// can be a DNS record: not "_", localhost, an IP, a wildcard or a regex.
// Twin of isLinkableName in frontend/src/lib/webDns.ts.
func LinkableName(raw string) (string, bool) {
	n := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if n == "" || n == "_" || n == "localhost" || strings.HasSuffix(n, ".localhost") ||
		net.ParseIP(n) != nil || strings.ContainsAny(n, "*~") {
		return "", false
	}
	return n, dnsNameRe.MatchString(n)
}

// webName is one name to record: the services it should link to, whether
// any site serving it uses TLS, and who serves it (for the record's notes).
type webName struct {
	services map[int64]bool
	tls      bool
	servedBy string
}

// RecordWebDNS links the scan's web-server site names to DNS records: an
// existing record gains a link to the host and to the services serving or
// receiving the name; a new name that resolves in DNS gets a record (host's
// entidade grants, notes "Scan: …"). Only adds — a name that disappears keeps
// its links. A record in the trash is never recreated or linked. Stopped
// host servers and unreadable configs contribute nothing. Pattern:
// DNSService.SyncFromCoolify.
func RecordWebDNS(ctx context.Context, db *sql.DB, host *models.Host, info sshtest.VMInfo, lookup Resolver) (WebDNSSummary, error) {
	var sum WebDNSSummary
	ctx = store.WithSystemScope(ctx)
	svcs, err := store.NewServiceRepo(db).ListDiscoveredByHost(ctx, host.ID)
	if err != nil {
		return sum, err
	}
	byContainer := map[string]int64{}
	byHostKind := map[string]int64{}
	for _, s := range svcs {
		if s.ContainerName != "" {
			byContainer[s.ContainerName] = s.ID
		}
		if s.DiscoveryKind == "host" {
			byHostKind[s.DiscoveryKey] = s.ID
		}
	}
	portContainer := map[int]string{}
	for _, po := range info.PortOwners {
		if po.OwnerType == "container" {
			portContainer[po.Port] = po.OwnerName
		}
	}
	var containers []string
	for _, c := range info.ParsedContainers {
		containers = append(containers, c.Name)
	}
	hostAddrs := map[string]bool{"127.0.0.1": true, "localhost": true, "::1": true, "0.0.0.0": true}
	for _, a := range []string{host.Hostname, info.PublicIP} {
		if a != "" {
			hostAddrs[strings.ToLower(a)] = true
		}
	}

	names := map[string]*webName{}
	var order []string
	for _, ws := range info.WebServers {
		if ws.Stopped || ws.Error != "" {
			continue
		}
		own := byHostKind[ws.Kind]
		where := "host"
		if ws.Container != "" {
			own, where = byContainer[ws.Container], "container "+ws.Container
		}
		for _, site := range ws.Sites {
			targets := pointsTo(site, ws.Container, hostAddrs, portContainer, containers)
			for _, raw := range site.Names {
				n, ok := LinkableName(raw)
				if !ok {
					continue
				}
				w := names[n]
				if w == nil {
					w = &webName{services: map[int64]bool{}, servedBy: ws.Kind + " (" + where + ")"}
					names[n] = w
					order = append(order, n)
				}
				w.tls = w.tls || site.TLS
				if own != 0 {
					w.services[own] = true
				}
				for _, c := range targets {
					if id := byContainer[c]; id != 0 {
						w.services[id] = true
					}
				}
			}
		}
	}
	sum.Names = len(order)
	if len(order) == 0 {
		return sum, nil
	}

	dns := store.NewDNSRepo(db)
	ids := map[string]int64{}
	var missing []string
	for _, n := range order {
		id, found, trashed, err := dns.IDByDomain(ctx, n)
		if err != nil {
			return sum, err
		}
		switch {
		case trashed:
			sum.Trashed++
		case found:
			ids[n] = id
		default:
			missing = append(missing, n)
		}
	}

	// Only the lookups are time-bounded; a slow resolver means "unresolved".
	lctx, cancel := context.WithTimeout(ctx, webDNSStepTimeout)
	resolves := resolveAll(lctx, lookup, missing)
	cancel()
	if len(missing) > 0 {
		situacao, err := store.NewEnumOptionRepo(db).SituacaoValue(ctx, store.SituacaoActive)
		if err != nil {
			return sum, err
		}
		grants := store.NewAssetEntidadeRepo(db)
		for _, n := range missing {
			if !resolves[n] {
				sum.Unresolved++
				continue
			}
			rec := models.DNSRecord{Domain: n, HasHTTPS: names[n].tls, Situacao: situacao,
				Observacoes: fmt.Sprintf("Scan: %s — %s", host.OficialSlug, names[n].servedBy)}
			if err := dns.Create(ctx, &rec); err != nil {
				if !store.IsUniqueViolation(err) {
					return sum, err
				}
				// A concurrent scan created it first: link to that one.
				id, found, _, err := dns.IDByDomain(ctx, n)
				if err != nil {
					return sum, err
				}
				if found {
					ids[n] = id
				}
				continue
			}
			if err := grants.CopyFrom(ctx, db, store.AssetHost, host.ID, store.AssetDNS, rec.ID); err != nil {
				return sum, err
			}
			ids[n] = rec.ID
			sum.Created++
		}
	}

	for _, n := range order {
		id, ok := ids[n]
		if !ok {
			continue
		}
		added, err := dns.AddHostLink(ctx, id, host.ID)
		if err != nil {
			return sum, err
		}
		if added {
			sum.Linked++
		}
		for svc := range names[n].services {
			added, err := dns.AddServiceLink(ctx, id, svc)
			if err != nil {
				return sum, err
			}
			if added {
				sum.ServiceLinks++
			}
		}
	}
	return sum, nil
}

// resolveAll looks names up in parallel; true = it resolves.
func resolveAll(ctx context.Context, lookup Resolver, names []string) map[string]bool {
	out := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, webDNSParallel)
	for _, n := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func(n string) {
			defer func() { <-sem; wg.Done() }()
			lctx, cancel := context.WithTimeout(ctx, webDNSLookupTimeout)
			defer cancel()
			addrs, err := lookup(lctx, n)
			mu.Lock()
			out[n] = err == nil && len(addrs) > 0
			mu.Unlock()
		}(n)
	}
	wg.Wait()
	return out
}

// pointsTo returns the containers a site hands requests to: proxy_pass /
// fastcgi_pass to a container by name (or by its compose service name), or —
// from a host server — to a local address whose port a container publishes.
// ponytail: a compose service name matching containers of several projects
// links all of them; narrow by compose project if that gets noisy.
func pointsTo(site sshtest.WebSite, from string, hostAddrs map[string]bool, portContainer map[int]string, containers []string) []string {
	var out []string
	add := func(c string) {
		for _, v := range out {
			if v == c {
				return
			}
		}
		if c != from {
			out = append(out, c)
		}
	}
	for _, t := range append(append([]string{}, site.Proxy...), site.FastCGI...) {
		h, port, ok := splitTarget(t)
		if !ok {
			continue
		}
		if from == "" && hostAddrs[h] {
			if c, ok := portContainer[port]; ok {
				add(c)
			}
			continue
		}
		for _, c := range containers {
			if c == h || composeService(c) == h {
				add(c)
			}
		}
	}
	return out
}

var composeName = regexp.MustCompile(`^.+[-_](.+)[-_]\d+$`)

// composeService is the compose service part of "<project>-<service>-<n>".
func composeService(container string) string {
	if m := composeName.FindStringSubmatch(container); m != nil {
		return m[1]
	}
	return ""
}

// splitTarget parses "http://app:3000/x", "app:9000" or "app" into host and
// port (80 when absent). Unix sockets aren't network targets.
func splitTarget(t string) (string, int, bool) {
	if strings.HasPrefix(t, "unix:") {
		return "", 0, false
	}
	port := 80
	if i := strings.Index(t, "://"); i >= 0 {
		if strings.HasPrefix(t, "https") {
			port = 443
		}
		t = t[i+3:]
	}
	if i := strings.IndexByte(t, '/'); i >= 0 {
		t = t[:i]
	}
	h := t
	if j := strings.LastIndexByte(t, ':'); j >= 0 && !strings.HasSuffix(t, "]") {
		if p, err := strconv.Atoi(t[j+1:]); err == nil {
			port = p
		}
		h = t[:j]
	}
	h = strings.ToLower(strings.Trim(h, "[]"))
	return h, port, h != ""
}
