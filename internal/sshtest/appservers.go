package sshtest

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// detectAppServers turns every Java process running a known app-server main
// class into one service per instance (unit, else the install directory).
// A process in a container (its own cgroup line, or containerPIDs) is
// skipped: ps lists it on the host too, but it belongs to the container's own
// service.
func detectAppServers(procs []ProcessDetail, containerPIDs map[int]string) []DiscoveredService {
	var out []DiscoveredService
	seen := map[string]bool{}
	for _, p := range procs {
		if p.InContainer {
			continue
		}
		if pid, err := strconv.Atoi(p.PID); err == nil {
			if _, inContainer := containerPIDs[pid]; inContainer {
				continue
			}
		}
		fields := strings.Fields(p.Command)
		for _, spec := range serviceCatalog {
			if !matchesMainClass(spec, fields) {
				continue
			}
			inst := p.Unit
			if inst == "" {
				inst = AppHome(p.Command)
			}
			if inst == "" || seen[spec.Name+"@"+inst] {
				break
			}
			seen[spec.Name+"@"+inst] = true
			var ports []int
			for _, s := range strings.Split(p.Ports, ",") {
				if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
					ports = append(ports, n)
				}
			}
			ports = httpPortFirst(ports)
			pid, _ := strconv.Atoi(p.PID)
			out = append(out, DiscoveredService{
				Name: spec.Name, Label: spec.Label, Vendor: spec.Vendor, Kind: spec.Kind,
				Instance: inst, Unit: p.Unit, PID: pid, State: "running",
				Ports: ports, HostPorts: ports, HostRunning: true, Sources: []string{"process"},
			})
			break
		}
	}
	return out
}

func matchesMainClass(spec serviceSpec, fields []string) bool {
	for _, c := range spec.MainClasses {
		for _, f := range fields {
			if f == c || strings.HasSuffix(f, "/"+c) {
				return true
			}
		}
	}
	return false
}

// AppHome finds the install (or instance base) directory in a java command line.
func AppHome(cmd string) string {
	fields := strings.Fields(cmd)
	for _, key := range []string{"-Djboss.home.dir=", "-Dcatalina.base=", "-Dcatalina.home=", "-Dcom.sun.aas.installRoot="} {
		for _, f := range fields {
			if v := strings.TrimPrefix(f, key); v != f && v != "" {
				return v
			}
		}
	}
	for _, f := range fields {
		for _, tok := range strings.Split(f, ":") {
			if strings.HasSuffix(tok, "/bin/run.jar") {
				return strings.TrimSuffix(tok, "/bin/run.jar")
			}
			if strings.HasSuffix(tok, "/jboss-modules.jar") {
				return path.Dir(tok)
			}
		}
	}
	return ""
}

// appDeployDirs lists where an instance of the catalog entry keeps its deployed applications.
func appDeployDirs(catalog, home, cmd string) []string {
	if home == "" {
		return nil
	}
	switch catalog {
	case "jboss":
		conf := "default"
		f := strings.Fields(cmd)
		for i := 0; i+1 < len(f); i++ {
			if f[i] == "-c" {
				conf = f[i+1]
			}
		}
		return []string{home + "/server/" + conf + "/deploy"}
	case "wildfly":
		return []string{home + "/standalone/deployments"}
	case "tomcat":
		return []string{home + "/webapps"}
	case "glassfish":
		return []string{home + "/glassfish/domains/domain1/applications"}
	}
	return nil
}

var appServerArtefacts = map[string]bool{
	"jmx-console.war": true, "admin-console.war": true, "ROOT.war": true,
	"docs": true, "examples": true, "manager": true, "host-manager": true, "ROOT": true,
}

// parseAppsListing reads the "@@dir <catalog> <path>" framed `ls -1p` output
// into the deployed applications per directory. Allow-list: JBoss <= 6 deploys
// only *.war / *.ear (file or exploded directory); Tomcat, WildFly and GlassFish
// also deploy extension-less directories (trailing "/"). The server's own stock
// entries are dropped.
func parseAppsListing(raw string) map[string][]string {
	out := map[string][]string{}
	dir, catalog := "", ""
	for _, l := range splitLines(raw) {
		l = strings.TrimSpace(l)
		if f, ok := strings.CutPrefix(l, "@@dir "); ok {
			catalog, dir, _ = strings.Cut(f, " ")
			continue
		}
		if dir == "" || l == "" || strings.HasPrefix(l, ".") {
			continue
		}
		name, isDir := strings.CutSuffix(l, "/")
		if appServerArtefacts[name] {
			continue
		}
		pkg := strings.HasSuffix(name, ".war") || strings.HasSuffix(name, ".ear")
		if pkg || (isDir && catalog != "jboss") {
			out[dir] = append(out[dir], name)
		}
	}
	return out
}

// httpPortFirst moves the probable HTTP port to the front: 8080, 8180, … first,
// else any port in 8000-8999 (AJP 8009 is not HTTP, but beats nothing).
func httpPortFirst(ports []int) []int {
	pick := -1
	for i, p := range ports {
		if p >= 8000 && p <= 8999 && p%100 == 80 {
			pick = i
			break
		}
	}
	if pick < 0 {
		for i, p := range ports {
			if p >= 8000 && p <= 8999 {
				pick = i
				break
			}
		}
	}
	if pick > 0 {
		p := ports[pick]
		ports = append([]int{p}, append(ports[:pick:pick], ports[pick+1:]...)...)
	}
	return ports
}

// captureAppServers detects the instances and lists their deploy directories
// in one read-only script (root when the stored password allows it).
func captureAppServers(client *ssh.Client, sudoPassword string, procs []ProcessDetail, containerPIDs map[int]string) []DiscoveredService {
	svcs := detectAppServers(procs, containerPIDs)
	if len(svcs) == 0 {
		return nil
	}
	dirsOf := make([][]string, len(svcs))
	var all []string // "<catalog> <dir>" frames
	for i, s := range svcs {
		for _, p := range procs {
			if strconv.Itoa(s.PID) != p.PID {
				continue
			}
			dirsOf[i] = appDeployDirs(s.Name, AppHome(p.Command), p.Command)
			for _, d := range dirsOf[i] {
				all = append(all, s.Name+" "+d)
			}
		}
	}
	if len(all) == 0 {
		return svcs
	}
	quoted := make([]string, len(all))
	for i, f := range all {
		cat, d, _ := strings.Cut(f, " ")
		quoted[i] = shQuote(cat) + " " + shQuote(d)
	}
	script := scanPath + "\n" + `set -- ` + strings.Join(quoted, " ") + `; while [ $# -ge 2 ]; do echo "@@dir $1 $2"; ls -1p "$2" 2>/dev/null; shift 2; done`
	raw := ""
	if sudoPassword != "" {
		if out, err := runSudoCmd(client, sudoPassword, script); err == nil {
			raw = out
		}
	}
	if raw == "" {
		raw = runCmd(client, script)
	}
	listing := parseAppsListing(raw)
	for i := range svcs {
		for _, d := range dirsOf[i] {
			svcs[i].Apps = append(svcs[i].Apps, listing[d]...)
		}
		sort.Strings(svcs[i].Apps)
	}
	return svcs
}
