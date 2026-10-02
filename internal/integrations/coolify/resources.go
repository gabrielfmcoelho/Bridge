package coolify

import (
	"net/url"
	"regexp"
	"strings"
)

// Resource is one Coolify application, service (compose stack) or database,
// flattened with its project/environment — what the inventory sync stores on
// the Bridge services it matches.
type Resource struct {
	UUID          string
	Type          string // application | service | database
	Name          string
	Project       string
	Environment   string
	GitRepository string // sanitized
	GitBranch     string
	Server        ServerRef
	FQDNs         []string // raw fqdn strings (comma-separated lists allowed)
	// Members are the container names Coolify gives a stack's parts
	// (`<name>-<uuid>`); empty for applications and databases (`<uuid>`).
	Members []string
}

// Resources flattens apps, services and databases, resolving each resource's
// environment_id through the projects' environments. masterHost replaces the
// master server's host.docker.internal IP, like DomainRefs.
func Resources(apps []Application, svcs []Service, dbs []Database, projects []Project, masterHost string) []Resource {
	type pe struct{ project, env string }
	envs := map[int64]pe{}
	for _, p := range projects {
		for _, e := range p.Environments {
			envs[e.ID] = pe{p.Name, e.Name}
		}
	}
	srv := func(s ServerRef) ServerRef {
		if s.IP == masterIP {
			s.IP = masterHost
		}
		return s
	}
	var out []Resource
	for _, a := range apps {
		e := envs[a.EnvironmentID]
		out = append(out, Resource{UUID: a.UUID, Type: "application", Name: a.Name, Project: e.project, Environment: e.env,
			GitRepository: SanitizeRepoURL(a.GitRepository), GitBranch: a.GitBranch, Server: srv(a.Destination.Server),
			FQDNs: []string{a.FQDN}})
	}
	for _, s := range svcs {
		e := envs[s.EnvironmentID]
		r := Resource{UUID: s.UUID, Type: "service", Name: s.Name, Project: e.project, Environment: e.env, Server: srv(s.Server)}
		for _, m := range s.Applications {
			r.FQDNs = append(r.FQDNs, m.FQDN)
			r.Members = append(r.Members, m.Name+"-"+s.UUID)
		}
		for _, m := range s.Databases {
			r.Members = append(r.Members, m.Name+"-"+s.UUID)
		}
		out = append(out, r)
	}
	for _, d := range dbs {
		e := envs[d.EnvironmentID]
		out = append(out, Resource{UUID: d.UUID, Type: "database", Name: d.Name, Project: e.project, Environment: e.env,
			Server: srv(d.Destination.Server)})
	}
	return out
}

// Hosts returns the lowercased hostnames of r's fqdns (scheme, port, path dropped).
func (r Resource) Hosts() []string {
	var out []string
	for _, fqdn := range r.FQDNs {
		for _, raw := range strings.Split(fqdn, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if !strings.Contains(raw, "://") {
				raw = "http://" + raw
			}
			if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
				out = append(out, strings.ToLower(u.Hostname()))
			}
		}
	}
	return out
}

// SanitizeRepoURL drops credentials from a repository URL
// ("https://gitlab+deploy-token-1:gldt-x@host/g/r.git" → "https://host/g/r.git").
// SCP-style ("git@host:g/r.git") and plain paths come back unchanged.
func SanitizeRepoURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Unparseable but may still hold user:pass@ — cut it by hand.
		if i, j := strings.Index(raw, "://"), strings.LastIndex(raw, "@"); i >= 0 && j > i {
			return raw[:i+3] + raw[j+1:]
		}
		return raw
	}
	u.User = nil
	return u.String()
}

// containerName: optional role prefix, the 24-char resource uuid, optional deploy timestamp.
var containerName = regexp.MustCompile(`^(?:(.+)-)?([a-z0-9]{24})(?:-(\d{12}))?$`)

// ContainerKey is the stable identity of a Coolify-managed container: its name
// without the deploy timestamp Coolify appends on every redeploy
// ("app-<uuid>-143303502430" → "app-<uuid>"), plus the resource uuid inside.
// Names that aren't Coolify's come back unchanged with uuid "".
func ContainerKey(name string) (key, uuid string) {
	m := containerName.FindStringSubmatch(name)
	if m == nil {
		return name, ""
	}
	if m[1] == "" {
		return m[2], m[2]
	}
	return m[1] + "-" + m[2], m[2]
}
