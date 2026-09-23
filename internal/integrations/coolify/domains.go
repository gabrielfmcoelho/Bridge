package coolify

import (
	"net/url"
	"strings"
)

// DomainRef is one hostname Coolify routes to a server.
type DomainRef struct {
	Domain     string
	HTTPS      bool
	ServerUUID string
	ServerIP   string
	Source     string // app/service name, for the record's observações
}

// masterIP is the IP Coolify reports for its own (master) server.
const masterIP = "host.docker.internal"

// DomainRefs flattens every fqdn of apps and services into one ref per
// lowercased hostname (scheme, port and path dropped). A domain seen more than
// once keeps its first server/source; HTTPS is true if any occurrence is https.
// The master server's IP is replaced by masterHost (the coolify_base_url host).
func DomainRefs(apps []Application, svcs []Service, masterHost string) []DomainRef {
	var out []DomainRef
	idx := map[string]int{}
	add := func(fqdn string, srv ServerRef, source string) {
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
			https := u.Scheme == "https"
			if i, ok := idx[host]; ok {
				out[i].HTTPS = out[i].HTTPS || https
				continue
			}
			idx[host] = len(out)
			out = append(out, DomainRef{Domain: host, HTTPS: https, ServerUUID: srv.UUID, ServerIP: ip, Source: source})
		}
	}
	for _, a := range apps {
		add(a.FQDN, a.Destination.Server, a.Name)
	}
	for _, s := range svcs {
		for _, a := range s.Applications {
			add(a.FQDN, s.Server, s.Name)
		}
	}
	return out
}
