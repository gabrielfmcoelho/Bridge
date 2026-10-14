package coolify

import (
	"net/url"
	"sort"
	"strings"
)

// DomainRef is one hostname Coolify routes to a server.
type DomainRef struct {
	Domain     string
	HTTPS      bool
	ServerUUID string
	ServerIP   string
	Source     string // app/service name, for the record's observações
	// ResourceUUID is the application/service the domain routes to; Member is
	// the stack container (`<name>-<uuid>`) for a compose service's member.
	ResourceUUID string
	Member       string
}

// masterIP is the IP Coolify reports for its own (master) server.
const masterIP = "host.docker.internal"

// DomainRefs flattens every fqdn of apps (compose services included) and
// services into one ref per
// lowercased hostname (scheme, port and path dropped). A domain seen more than
// once keeps its first server/source. HTTPS is true for every real domain:
// Coolify's fqdn scheme is often plain http even when the proxy serves TLS, so
// only the auto-generated *.sslip.io names follow the scheme.
// The master server's IP is replaced by masterHost (the coolify_base_url host).
func DomainRefs(apps []Application, svcs []Service, masterHost string) []DomainRef {
	var out []DomainRef
	idx := map[string]int{}
	add := func(fqdn string, srv ServerRef, source, uuid, member string) {
		ip := srv.IP
		if ip == masterIP {
			ip = masterHost
		}
		for _, raw := range strings.Split(fqdn, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if !strings.Contains(raw, "://") {
				raw = "http://" + raw
			}
			u, err := url.Parse(raw)
			if err != nil || u.Hostname() == "" {
				continue
			}
			host := strings.ToLower(u.Hostname())
			https := u.Scheme == "https" || !strings.HasSuffix(host, ".sslip.io")
			if i, ok := idx[host]; ok {
				out[i].HTTPS = out[i].HTTPS || https
				continue
			}
			idx[host] = len(out)
			out = append(out, DomainRef{Domain: host, HTTPS: https, ServerUUID: srv.UUID, ServerIP: ip, Source: source,
				ResourceUUID: uuid, Member: member})
		}
	}
	for _, a := range apps {
		add(a.FQDN, a.Destination.Server, a.Name, a.UUID, "")
		// A compose app's containers are `<svc>-<uuid>-<ts>`; Member drops the
		// timestamp like ContainerKey. Sorted so the first-seen rule is stable.
		cd := a.ComposeDomains()
		svcs := make([]string, 0, len(cd))
		for svc := range cd {
			svcs = append(svcs, svc)
		}
		sort.Strings(svcs)
		for _, svc := range svcs {
			add(cd[svc], a.Destination.Server, a.Name, a.UUID, svc+"-"+a.UUID)
		}
	}
	for _, s := range svcs {
		for _, a := range s.Applications {
			member := ""
			if s.UUID != "" {
				member = a.Name + "-" + s.UUID
			}
			add(a.FQDN, s.Server, s.Name, s.UUID, member)
		}
	}
	return out
}
