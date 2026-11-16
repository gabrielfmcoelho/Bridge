package service

import (
	"context"
	"database/sql"
	"encoding/json"
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
	Routes       int `json:"routes"`        // web routes recorded (name × path × target)
	Dependencies int `json:"dependencies"`  // service dependencies added (Kong → its database)
	// RoutesUnresolved: routes whose backend host or service Bridge doesn't know.
	RoutesUnresolved int `json:"routes_unresolved"`
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
	hosts    map[int64]bool // other hosts the site proxies to
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
//
// It also replaces the host's web routes (store.WebRoute): every proxy_pass /
// fastcgi_pass per site name and location path, with the host and service it
// reaches. LinkInboundRoutes fills them from the backend's side.
func RecordWebDNS(ctx context.Context, db *sql.DB, host *models.Host, info sshtest.VMInfo, lookup Resolver) (WebDNSSummary, error) {
	var sum WebDNSSummary
	ctx = store.WithSystemScope(ctx)
	services := store.NewServiceRepo(db)
	svcs, err := services.ListDiscoveredByHost(ctx, host.ID)
	if err != nil {
		return sum, err
	}
	byContainer := map[string]int64{}
	byHostKind := map[string]int64{}
	bySite := map[string]int64{} // C.3: (vhost name + docroot) → site service id
	for _, s := range svcs {
		if s.ContainerName != "" {
			byContainer[s.ContainerName] = s.ID
		}
		if s.DiscoveryKind == "host" {
			byHostKind[s.DiscoveryKey] = s.ID
		}
		if s.DiscoveryKind == "site" {
			bySite[s.DiscoveryKey] = s.ID
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

	remote := newRemoteIndex(db, lookup)
	// resolve finds the host and service a server (own, in container ctr)
	// reaches at addr:port: its own loopback, a port of this host's latest
	// scan (already stored), a container by name, or another VM.
	resolve := func(addr string, port int, ctr string, own int64) (int64, int64, error) {
		switch {
		case hostAddrs[addr] && ctr != "":
			return host.ID, own, nil // the container's own loopback
		case hostAddrs[addr]:
			ports, err := remote.portServices(ctx, host.ID)
			return host.ID, ports[port], err
		case net.ParseIP(addr) == nil && !strings.Contains(addr, "."):
			for _, c := range containers {
				if c == addr || composeService(c) == addr {
					return host.ID, byContainer[c], nil
				}
			}
			return 0, 0, nil
		default:
			return remote.target(ctx, addr, port)
		}
	}
	names := map[string]*webName{}
	var order []string
	var routes []store.WebRoute
	// A server whose config couldn't be read keeps its previous routes (by its
	// service); one with no service keeps the whole host's. shortcut: a scan
	// with no web server at all keeps them too (it can't tell "removed" from
	// "probe didn't run"); the next readable scan replaces them.
	replaceRoutes := len(info.WebServers) > 0
	var keepRoutes []int64
	for _, ws := range info.WebServers {
		own := byHostKind[ws.Kind]
		where := "host"
		if ws.Container != "" {
			own, where = byContainer[ws.Container], "container "+ws.Container
		}
		if ws.Error != "" {
			if own == 0 {
				replaceRoutes = false
			}
			keepRoutes = append(keepRoutes, own)
		}
		if ws.Stopped || ws.Error != "" {
			continue
		}
		// Kong → the database it uses (pg_host:pg_port). Only adds.
		if k := ws.Kong; k != nil && k.DBHost != "" && own != 0 {
			_, dep, err := resolve(strings.ToLower(k.DBHost), k.DBPort, ws.Container, own)
			if err != nil {
				return sum, err
			}
			if dep != 0 && dep != own {
				added, err := services.AddDependency(ctx, own, dep)
				if err != nil {
					return sum, err
				}
				if added {
					sum.Dependencies++
				}
			}
		}
		for _, site := range ws.Sites {
			if site.Disabled {
				continue // Virtualmin-disabled vhost: serves disabled_by_virtualmin.html, not the app.
			}
			// C.3: the site service (if any) for this docroot — every DNS name
			// this vhost serves links to it.
			siteSvc := int64(0)
			if site.Root != "" {
				if name, ok := primarySiteName(site); ok {
					siteSvc = bySite[store.SiteDiscoveryKey(name, site.Root)]
				}
			}
			targets := pointsTo(site, ws.Container, hostAddrs, portContainer, containers)
			remHosts, remSvcs, err := remote.resolve(ctx, remoteTargets(site, hostAddrs))
			if err != nil {
				return sum, err
			}
			siteNames := []string{}
			for _, raw := range site.Names {
				if n, ok := LinkableName(raw); ok {
					siteNames = append(siteNames, n)
				}
			}
			if len(siteNames) == 0 {
				siteNames = []string{""} // default / IP-only site: still a route
			}
			for _, loc := range siteLocations(site) {
				for _, t := range loc.targets {
					addr, port, ok := splitTarget(t)
					if !ok {
						continue
					}
					th, ts, err := resolve(addr, port, ws.Container, own)
					if err != nil {
						return sum, err
					}
					for _, n := range siteNames {
						routes = append(routes, store.WebRoute{ProxyHostID: host.ID, ProxyServiceID: own,
							ServerName: n, Path: loc.path, Target: t, TargetAddr: addr, TargetPort: port,
							TargetHostID: th, TargetServiceID: ts})
					}
				}
			}
			for _, raw := range site.Names {
				n, ok := LinkableName(raw)
				if !ok {
					continue
				}
				w := names[n]
				if w == nil {
					w = &webName{services: map[int64]bool{}, hosts: map[int64]bool{}, servedBy: ws.Kind + " (" + where + ")"}
					names[n] = w
					order = append(order, n)
				}
				w.tls = w.tls || site.TLS
				if own != 0 {
					w.services[own] = true
				}
				if siteSvc != 0 {
					w.services[siteSvc] = true
				}
				for _, id := range remHosts {
					w.hosts[id] = true
				}
				for _, id := range remSvcs {
					w.services[id] = true
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
	saveRoutes := func(ids map[string]int64) error {
		if !replaceRoutes {
			return nil
		}
		for i := range routes {
			routes[i].DNSID = ids[routes[i].ServerName]
			if routes[i].TargetHostID == 0 || routes[i].TargetServiceID == 0 {
				sum.RoutesUnresolved++
			}
		}
		sum.Routes = len(routes)
		return store.NewWebRouteRepo(db).ReplaceForHost(ctx, host.ID, keepRoutes, routes)
	}
	if len(order) == 0 {
		return sum, saveRoutes(nil)
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
		for hid := range names[n].hosts {
			added, err := dns.AddHostLink(ctx, id, hid)
			if err != nil {
				return sum, err
			}
			if added {
				sum.Linked++
			}
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
	return sum, saveRoutes(ids)
}

// webLocation is one path of a site and the targets it forwards to.
type webLocation struct {
	path    string
	targets []string
}

// siteLocations lists a site's forwarding paths: its nginx locations, or the
// whole site ("") when it has none (Apache, or nginx without locations).
func siteLocations(site sshtest.WebSite) []webLocation {
	if len(site.Locations) == 0 {
		return []webLocation{{"", append(append([]string{}, site.Proxy...), site.FastCGI...)}}
	}
	out := make([]webLocation, 0, len(site.Locations))
	for _, l := range site.Locations {
		out = append(out, webLocation{l.Path, append(append([]string{}, l.Proxy...), l.FastCGI...)})
	}
	return out
}

// InboundSummary is what a backend's scan did to other hosts' web routes.
type InboundSummary struct {
	Routes       int `json:"routes"`        // other hosts' routes aimed at this host
	Updated      int `json:"updated"`       // routes whose resolved host/service changed
	Linked       int `json:"linked"`        // DNS → host links added
	ServiceLinks int `json:"service_links"` // DNS → service links added
}

// LinkInboundRoutes is RecordWebDNS seen from the backend: other hosts'
// routes aimed at this host (by its address, or already resolved to it) are
// resolved again against this scan, and their DNS records gain links to this
// host and the service owning the port. So whichever side is scanned last
// closes the link — the proxy needn't be rescanned. Only adds DNS links.
// shortcut: a target written as a DNS name that only now resolves to this
// host is caught on the proxy's next scan, not here.
func LinkInboundRoutes(ctx context.Context, db *sql.DB, host *models.Host, info sshtest.VMInfo, lookup Resolver) (InboundSummary, error) {
	var sum InboundSummary
	ctx = store.WithSystemScope(ctx)
	addrs := []string{}
	for _, a := range []string{host.Hostname, info.PublicIP} {
		if a != "" {
			addrs = append(addrs, strings.ToLower(a))
		}
	}
	repo := store.NewWebRouteRepo(db)
	rs, err := repo.ListInbound(ctx, host.ID, addrs)
	if err != nil || len(rs) == 0 {
		return sum, err
	}
	sum.Routes = len(rs)
	remote := newRemoteIndex(db, lookup)
	dns := store.NewDNSRepo(db)
	for _, w := range rs {
		hid, sid, err := remote.target(ctx, w.TargetAddr, w.TargetPort)
		if err != nil {
			return sum, err
		}
		changed, err := repo.SetTarget(ctx, w, hid, sid)
		if err != nil {
			return sum, err
		}
		if changed {
			sum.Updated++
		}
		if hid == 0 || w.DNSID == 0 {
			continue
		}
		added, err := dns.AddHostLink(ctx, w.DNSID, hid)
		if err != nil {
			return sum, err
		}
		if added {
			sum.Linked++
		}
		if sid == 0 {
			continue
		}
		if added, err = dns.AddServiceLink(ctx, w.DNSID, sid); err != nil {
			return sum, err
		}
		if added {
			sum.ServiceLinks++
		}
	}
	return sum, nil
}

// remoteTarget is a proxy_pass/fastcgi_pass destination outside this host.
type remoteTarget struct {
	Addr string
	Port int
}

// remoteTargets lists a site's destinations that aren't this host: IP
// literals and dotted names (a bare name is a container or compose service).
func remoteTargets(site sshtest.WebSite, hostAddrs map[string]bool) []remoteTarget {
	var out []remoteTarget
	seen := map[remoteTarget]bool{}
	for _, t := range append(append([]string{}, site.Proxy...), site.FastCGI...) {
		h, port, ok := splitTarget(t)
		if !ok || hostAddrs[h] || (net.ParseIP(h) == nil && !strings.Contains(h, ".")) {
			continue
		}
		rt := remoteTarget{h, port}
		if !seen[rt] {
			seen[rt] = true
			out = append(out, rt)
		}
	}
	return out
}

// remoteIndex maps remote targets to the Bridge host (hostname match, only
// when exactly one live host has it) and the service on that host owning the
// port in its latest scan. Caches per RecordWebDNS call.
type remoteIndex struct {
	db     *sql.DB
	lookup Resolver
	hosts  map[string][]models.Host
	scans  map[int64]map[int]int64 // host id -> port -> service id
}

func newRemoteIndex(db *sql.DB, lookup Resolver) *remoteIndex {
	return &remoteIndex{db: db, lookup: lookup, hosts: map[string][]models.Host{}, scans: map[int64]map[int]int64{}}
}

func (r *remoteIndex) hostsFor(ctx context.Context, addr string) ([]models.Host, error) {
	if hs, ok := r.hosts[addr]; ok {
		return hs, nil
	}
	repo := store.NewHostRepo(r.db)
	hs, err := repo.LiveByHostname(ctx, addr)
	if err == nil && len(hs) == 0 && net.ParseIP(addr) == nil && r.lookup != nil {
		lctx, cancel := context.WithTimeout(ctx, webDNSLookupTimeout)
		addrs, _ := r.lookup(lctx, addr)
		cancel()
		for _, a := range addrs {
			if hs, err = repo.LiveByHostname(ctx, a); err != nil || len(hs) > 0 {
				break
			}
		}
	}
	r.hosts[addr] = hs
	return hs, err
}

func (r *remoteIndex) portServices(ctx context.Context, hostID int64) (map[int]int64, error) {
	if m, ok := r.scans[hostID]; ok {
		return m, nil
	}
	m := map[int]int64{}
	r.scans[hostID] = m
	scan, err := store.NewHostScanRepo(r.db).GetLatest(ctx, hostID)
	if err != nil || scan == nil {
		return m, err
	}
	var info sshtest.VMInfo
	if json.Unmarshal([]byte(scan.Data), &info) != nil {
		return m, nil
	}
	svcs, err := store.NewServiceRepo(r.db).ListDiscoveredByHost(ctx, hostID)
	if err != nil {
		return m, err
	}
	byKey := map[string]int64{}
	byContainer := map[string]int64{}
	for _, s := range svcs {
		if s.DiscoveryKind == "host" {
			byKey[s.DiscoveryKey] = s.ID
		}
		if s.ContainerName != "" {
			byContainer[s.ContainerName] = s.ID
		}
	}
	for _, d := range info.ServiceInventory {
		key := d.Name
		if d.Instance != "" {
			key += "@" + d.Instance
		}
		if id := byKey[key]; id != 0 {
			for _, p := range d.HostPorts {
				m[p] = id
			}
		}
	}
	// A port a container publishes (docker, Coolify) belongs to its container.
	for _, po := range info.PortOwners {
		if id := byContainer[po.OwnerName]; po.OwnerType == "container" && id != 0 && m[po.Port] == 0 {
			m[po.Port] = id
		}
	}
	return m, nil
}

// target resolves one destination: the only live host with that address
// (0 when none or several) and the service owning the port in its latest scan.
func (r *remoteIndex) target(ctx context.Context, addr string, port int) (hostID, svcID int64, err error) {
	hs, err := r.hostsFor(ctx, addr)
	if err != nil || len(hs) != 1 {
		return 0, 0, err
	}
	ports, err := r.portServices(ctx, hs[0].ID)
	return hs[0].ID, ports[port], err
}

func (r *remoteIndex) resolve(ctx context.Context, targets []remoteTarget) (hostIDs, svcIDs []int64, err error) {
	for _, t := range targets {
		hid, sid, err := r.target(ctx, t.Addr, t.Port)
		if err != nil {
			return nil, nil, err
		}
		if hid != 0 {
			hostIDs = append(hostIDs, hid)
		}
		if sid != 0 {
			svcIDs = append(svcIDs, sid)
		}
	}
	return hostIDs, svcIDs, nil
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
