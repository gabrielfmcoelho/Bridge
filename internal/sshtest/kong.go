package sshtest

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// kongInnerScript is webInnerScript's Kong part (it reuses $T and P). Each
// running Kong node — an OpenResty master whose -p prefix holds a .kong_env —
// answers its Admin API on localhost: "@prefix", then one "@<what> <json>"
// line per call (root, status, services, routes, upstreams, "health <name>"),
// or "@error …". Of .kong_env only admin_listen is read: the file also holds
// pg_password. Kong images ship without curl, so resty (bundled with every
// Kong) fetches instead.
const kongInnerScript = `for p in $(ps -eo args= 2>/dev/null | sed -n 's/^nginx: master process .* -p \([^ ]*\).*/\1/p' | sort -u); do
  p=${p%/}; [ -f "$p/.kong_env" ] || continue
  P kong; echo "@prefix $p"
  l=$(sed -n 's/^admin_listen = //p' "$p/.kong_env" | tr ',' '\n')
  s=http; a=$(printf '%s\n' "$l" | grep -v ' ssl' | awk 'NF {print $1; exit}')
  if [ -z "$a" ]; then s=https; a=$(printf '%s\n' "$l" | awk 'NF {print $1; exit}'); fi
  case "$a" in ""|off) echo "@error admin API off"; continue ;; 0.0.0.0:*|"[::]:"*) a="127.0.0.1:${a##*:}" ;; esac
  if command -v curl >/dev/null 2>&1; then F(){ $T curl -sSk -m 5 "$s://$a$1" 2>&1; }
  elif command -v resty >/dev/null 2>&1; then F(){ $T resty -e "local h=require('resty.http').new() h:set_timeout(5000) local r,e=h:request_uri('$s://$a$1',{ssl_verify=false}) io.write(r and r.body or ('error: '..tostring(e)))" 2>&1; }
  else echo "@error neither curl nor resty found"; continue; fi
  K(){ printf '@%s ' "$1"; F "$2" | head -c 262144 | tr -d '\n'; echo; }
  K root /; K status /status; K services '/services?size=1000'; K routes '/routes?size=1000'; K upstreams '/upstreams?size=1000'
  for u in $(F '/upstreams?size=1000' | grep -o '"name":"[^"]*"' | sed 's/^"name":"//; s/"$//' | head -n 50); do K "health $u" "/upstreams/$u/health"; done
done
`

// KongNode is one Kong gateway node the scan reached through its Admin API.
// Its routes become the WebServer's sites (route host → site, route path →
// location, service or upstream targets → proxy targets).
type KongNode struct {
	Prefix   string `json:"prefix"`
	Version  string `json:"version,omitempty"`
	Database string `json:"database,omitempty"` // postgres | off (DB-less)
	DBHost   string `json:"db_host,omitempty"`
	DBPort   int    `json:"db_port,omitempty"`
	// DBReachable is nil when Kong doesn't report it (DB-less, old versions).
	DBReachable *bool          `json:"db_reachable,omitempty"`
	Connections int            `json:"connections_active,omitempty"`
	Requests    int64          `json:"total_requests,omitempty"`
	ProxyPorts  []int          `json:"proxy_ports,omitempty"`
	AdminPorts  []int          `json:"admin_ports,omitempty"` // Admin API, Kong Manager, status
	Services    int            `json:"services"`
	Routes      int            `json:"routes"`
	Upstreams   []KongUpstream `json:"upstreams,omitempty"`
}

// KongUpstream is a Kong load-balancing upstream and its targets' health.
type KongUpstream struct {
	Name    string       `json:"name"`
	Targets []KongTarget `json:"targets,omitempty"`
}

// KongTarget health: HEALTHY, UNHEALTHY, DNS_ERROR or HEALTHCHECKS_OFF.
type KongTarget struct {
	Target string `json:"target"`
	Health string `json:"health,omitempty"`
}

type kongListener struct {
	Port int `json:"port"`
}

// kongList reads a JSON list, and {} as an empty one: Kong's cjson encodes an
// empty Lua table as an object (status_listeners: {} when there are none).
type kongList[T any] []T

func (l *kongList[T]) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		return json.Unmarshal(b, (*[]T)(l))
	}
	*l = nil
	return nil
}

type kongService struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Path     string `json:"path"`
}

type kongRoute struct {
	Hosts     kongList[string] `json:"hosts"`
	Paths     kongList[string] `json:"paths"`
	Protocols kongList[string] `json:"protocols"`
	Service   *struct {
		ID string `json:"id"`
	} `json:"service"`
}

// parseKong turns the @@kong section (one or more nodes) into one WebServer
// of kind "kong" per node. A node whose Admin API didn't answer carries an
// Error, so its previous web routes are kept (see RecordWebDNS).
func parseKong(text string) []WebServer {
	var out []WebServer
	var raw map[string]string
	var health map[string]string
	var prefix, nodeErr string
	flush := func() {
		if prefix != "" {
			out = append(out, kongServer(prefix, nodeErr, raw, health))
		}
	}
	for _, line := range strings.Split(text, "\n") {
		tag, body, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch tag {
		case "@prefix":
			flush()
			prefix, nodeErr, raw, health = body, "", map[string]string{}, map[string]string{}
		case "@error":
			nodeErr = body
		case "@root", "@status", "@services", "@routes", "@upstreams":
			raw[tag[1:]] = body
		case "@health":
			if name, js, ok := strings.Cut(body, " "); ok {
				health[name] = js
			}
		}
	}
	flush()
	return out
}

func kongServer(prefix, nodeErr string, raw, health map[string]string) WebServer {
	node := &KongNode{Prefix: prefix}
	ws := WebServer{Kind: "kong", Kong: node}
	var root struct {
		Version       string `json:"version"`
		Message       string `json:"message"`
		Configuration struct {
			Database          string                 `json:"database"`
			PGHost            string                 `json:"pg_host"`
			PGPort            int                    `json:"pg_port"`
			ProxyListeners    kongList[kongListener] `json:"proxy_listeners"`
			AdminListeners    kongList[kongListener] `json:"admin_listeners"`
			AdminGUIListeners kongList[kongListener] `json:"admin_gui_listeners"`
			StatusListeners   kongList[kongListener] `json:"status_listeners"`
		} `json:"configuration"`
	}
	if nodeErr == "" && (json.Unmarshal([]byte(raw["root"]), &root) != nil || root.Version == "") {
		nodeErr = firstLine(root.Message + raw["root"])
	}
	if nodeErr != "" {
		ws.Error = "Kong Admin API: " + nodeErr
		return ws
	}
	c := root.Configuration
	node.Version, node.Database = root.Version, c.Database
	if c.Database != "" && c.Database != "off" {
		node.DBHost, node.DBPort = c.PGHost, c.PGPort
	}
	node.ProxyPorts = listenerPorts(c.ProxyListeners)
	node.AdminPorts = listenerPorts(c.AdminListeners, c.AdminGUIListeners, c.StatusListeners)

	var status struct {
		Database *struct {
			Reachable bool `json:"reachable"`
		} `json:"database"`
		Server struct {
			ConnectionsActive int   `json:"connections_active"`
			TotalRequests     int64 `json:"total_requests"`
		} `json:"server"`
	}
	if json.Unmarshal([]byte(raw["status"]), &status) == nil {
		if status.Database != nil {
			node.DBReachable = &status.Database.Reachable
		}
		node.Connections, node.Requests = status.Server.ConnectionsActive, status.Server.TotalRequests
	}

	upstreams := map[string][]KongTarget{}
	var ups struct {
		Data kongList[struct {
			Name string `json:"name"`
		}] `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw["upstreams"]), &ups)
	for _, u := range ups.Data {
		var h struct {
			Data kongList[KongTarget] `json:"data"`
		}
		_ = json.Unmarshal([]byte(health[u.Name]), &h)
		upstreams[u.Name] = h.Data
		node.Upstreams = append(node.Upstreams, KongUpstream{Name: u.Name, Targets: []KongTarget(h.Data)})
	}

	var svcs struct {
		Data kongList[kongService] `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw["services"]), &svcs)
	byID := map[string]kongService{}
	for _, s := range svcs.Data {
		byID[s.ID] = s
	}
	var routes struct {
		Data kongList[kongRoute] `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw["routes"]), &routes)
	node.Services, node.Routes = len(svcs.Data), len(routes.Data)
	ws.Sites = kongSites(routes.Data, byID, upstreams, node.ProxyPorts)
	return ws
}

// kongSites groups routes by host ("_" when a route has none, as nginx's
// default server) and path ("" when it has none: the whole site), each
// location forwarding to its service — or, when the service's host is an
// upstream, to every target of that upstream.
func kongSites(routes []kongRoute, svcs map[string]kongService, upstreams map[string][]KongTarget, proxyPorts []int) []WebSite {
	type site struct {
		tls  bool
		locs map[string][]string
	}
	sites := map[string]*site{}
	for _, r := range routes {
		if r.Service == nil {
			continue
		}
		s, ok := svcs[r.Service.ID]
		if !ok {
			continue
		}
		var targets []string
		if ts, isUpstream := upstreams[s.Host]; isUpstream {
			for _, t := range ts {
				targets = append(targets, s.Protocol+"://"+t.Target+s.Path)
			}
		} else {
			targets = []string{s.Protocol + "://" + s.Host + ":" + strconv.Itoa(s.Port) + s.Path}
		}
		hosts, paths := r.Hosts, r.Paths
		if len(hosts) == 0 {
			hosts = []string{"_"}
		}
		if len(paths) == 0 {
			paths = []string{""}
		}
		tls := false
		for _, p := range r.Protocols {
			tls = tls || p == "https" || p == "grpcs" || p == "tls"
		}
		for _, h := range hosts {
			st := sites[h]
			if st == nil {
				st = &site{locs: map[string][]string{}}
				sites[h] = st
			}
			st.tls = st.tls || tls
			for _, p := range paths {
				for _, t := range targets {
					st.locs[p] = appendUnique(st.locs[p], t)
				}
			}
		}
	}
	names := make([]string, 0, len(sites))
	for n := range sites {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []WebSite
	for _, n := range names {
		st := sites[n]
		ws := WebSite{Names: []string{n}, Listen: proxyPorts, TLS: st.tls}
		paths := make([]string, 0, len(st.locs))
		for p := range st.locs {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			ws.Locations = append(ws.Locations, WebLocation{Path: p, Proxy: st.locs[p]})
			for _, t := range st.locs[p] {
				ws.Proxy = appendUnique(ws.Proxy, t)
			}
		}
		out = append(out, ws)
	}
	return out
}

func listenerPorts(lists ...kongList[kongListener]) []int {
	var out []int
	for _, l := range lists {
		for _, x := range l {
			if x.Port > 0 {
				out = appendUniqueInt(out, x.Port)
			}
		}
	}
	sort.Ints(out)
	return out
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if len(s) > 160 {
		s = s[:160]
	}
	if s == "" {
		return "no answer"
	}
	return s
}

// isKongProc: Kong's OpenResty shows as "nginx" in ps — the master runs with
// -p <Kong prefix>, the workers as the kong user (nginx_user's default).
// shortcut: a renamed prefix and a custom nginx_user both escape this; the
// Admin API probe still finds the node and claims its ports.
func isKongProc(user, args string) bool {
	return user == "kong" || strings.Contains(args, "kong")
}

// claimKongPorts gives the host's Kong service the listeners of every host
// Kong node the probe reached (creating the service when the catalog didn't
// see it) and takes them off other services: ss names Kong's processes
// "nginx", so the nginx entry may have claimed them.
func claimKongPorts(info *VMInfo) {
	var ports []int
	version := ""
	for _, ws := range info.WebServers {
		if ws.Kind == "kong" && ws.Container == "" && ws.Kong != nil && ws.Error == "" {
			for _, p := range append(append([]int{}, ws.Kong.ProxyPorts...), ws.Kong.AdminPorts...) {
				ports = appendUniqueInt(ports, p)
			}
			if version == "" {
				version = ws.Kong.Version
			}
		}
	}
	if len(ports) == 0 {
		return
	}
	sort.Ints(ports)
	claimed := map[int]bool{}
	for _, p := range ports {
		claimed[p] = true
	}
	idx := -1
	for i := range info.ServiceInventory {
		s := &info.ServiceInventory[i]
		if s.Name == "kong" {
			idx = i
			continue
		}
		s.Ports, s.HostPorts = dropPorts(s.Ports, claimed), dropPorts(s.HostPorts, claimed)
	}
	if idx < 0 {
		svc := DiscoveredService{Name: "kong"}
		for _, spec := range serviceCatalog {
			if spec.Name == "kong" {
				svc.Label, svc.Kind, svc.Vendor = spec.Label, spec.Kind, spec.Vendor
			}
		}
		info.ServiceInventory = append(info.ServiceInventory, svc)
		idx = len(info.ServiceInventory) - 1
	}
	k := &info.ServiceInventory[idx]
	k.State, k.HostRunning, k.Ports, k.HostPorts = "running", true, ports, ports
	k.Sources = appendUnique(k.Sources, "probe")
	if k.Version == "" {
		k.Version = version
	}
}

func dropPorts(ps []int, drop map[int]bool) []int {
	var out []int
	for _, p := range ps {
		if !drop[p] {
			out = append(out, p)
		}
	}
	return out
}
