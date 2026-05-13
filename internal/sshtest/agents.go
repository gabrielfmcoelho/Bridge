package sshtest

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// agentSpec describes detection patterns for one well-known agent.
// Detection is non-strict: any matching signal (unit, binary, package,
// or listen-port owner) flags the agent as present. Multiple signals
// combine into the resulting Agent.Sources slice so the UI can render
// "installed but stopped" differently from "active and listening".
type agentSpec struct {
	Name        string   // canonical catalog key (matches Agent.Name)
	Label       string   // display name
	Vendor      string   // organization that ships the agent
	Category    string   // see Agent.Category for valid values
	Units       []string // systemd unit basenames (without .service); a trailing "@" matches templated instances
	Binaries    []string // process binary basenames (matched against ps comm + argv[0])
	Packages    []string // dpkg/rpm package names
	ConfigPaths []string // documentation only; first entry is exposed via Agent.ConfigPath
	// SignaturePorts are well-known TCP listen ports that count as a strong
	// detection signal even when the binary/unit/package patterns above
	// don't match — e.g. a custom-built zabbix-agent binary still listens
	// on :10050. Catalog ports are deduplicated globally: the first
	// matching spec claims the port; later specs that overlap will not
	// re-detect via the same port.
	SignaturePorts []int
}

// agentCatalog is the set of agents we detect. Keep this list focused on
// agents with operational impact (monitoring, security, configuration,
// remote access) — generic services like nginx or postgres belong in the
// services taxonomy, not here. Order does not matter.
var agentCatalog = []agentSpec{
	// ── Monitoring (metrics) ──
	{Name: "zabbix-agent2", Label: "Zabbix Agent 2", Vendor: "Zabbix", Category: "monitoring",
		Units: []string{"zabbix-agent2"}, Binaries: []string{"zabbix_agent2"},
		Packages: []string{"zabbix-agent2"}, ConfigPaths: []string{"/etc/zabbix/zabbix_agent2.conf"},
		// 10050 = passive check port (Zabbix server pulls metrics).
		// Iconic enough that nothing else routinely uses it, so the port
		// alone is sufficient to flag a host as being monitored.
		SignaturePorts: []int{10050}},
	{Name: "zabbix-agent", Label: "Zabbix Agent (Classic)", Vendor: "Zabbix", Category: "monitoring",
		Units: []string{"zabbix-agent"}, Binaries: []string{"zabbix_agentd"},
		Packages: []string{"zabbix-agent"}, ConfigPaths: []string{"/etc/zabbix/zabbix_agentd.conf"}},
	{Name: "zabbix-server", Label: "Zabbix Server", Vendor: "Zabbix", Category: "monitoring",
		Units: []string{"zabbix-server"}, Binaries: []string{"zabbix_server"},
		Packages:       []string{"zabbix-server-mysql", "zabbix-server-pgsql", "zabbix-server"},
		ConfigPaths:    []string{"/etc/zabbix/zabbix_server.conf"},
		SignaturePorts: []int{10051}},
	{Name: "zabbix-proxy", Label: "Zabbix Proxy", Vendor: "Zabbix", Category: "monitoring",
		Units: []string{"zabbix-proxy"}, Binaries: []string{"zabbix_proxy"},
		Packages: []string{"zabbix-proxy-mysql", "zabbix-proxy-pgsql", "zabbix-proxy-sqlite3", "zabbix-proxy"}},
	{Name: "node_exporter", Label: "Prometheus Node Exporter", Vendor: "Prometheus", Category: "monitoring",
		Units: []string{"node_exporter", "prometheus-node-exporter"}, Binaries: []string{"node_exporter"},
		Packages:       []string{"prometheus-node-exporter", "node_exporter"},
		ConfigPaths:    []string{"/etc/default/prometheus-node-exporter"},
		SignaturePorts: []int{9100}},
	{Name: "telegraf", Label: "Telegraf", Vendor: "InfluxData", Category: "monitoring",
		Units: []string{"telegraf"}, Binaries: []string{"telegraf"},
		Packages: []string{"telegraf"}, ConfigPaths: []string{"/etc/telegraf/telegraf.conf"}},
	{Name: "grafana-agent", Label: "Grafana Agent", Vendor: "Grafana Labs", Category: "monitoring",
		Units: []string{"grafana-agent", "grafana-agent-flow"}, Binaries: []string{"grafana-agent"},
		Packages: []string{"grafana-agent", "grafana-agent-flow"},
		ConfigPaths: []string{"/etc/grafana-agent.yaml", "/etc/grafana-agent.yml"}},
	{Name: "grafana-server", Label: "Grafana", Vendor: "Grafana Labs", Category: "monitoring",
		Units: []string{"grafana-server"}, Binaries: []string{"grafana-server", "grafana"},
		Packages:       []string{"grafana", "grafana-enterprise"},
		ConfigPaths:    []string{"/etc/grafana/grafana.ini"},
		SignaturePorts: []int{3000}},
	{Name: "netdata", Label: "Netdata", Vendor: "Netdata", Category: "monitoring",
		Units: []string{"netdata"}, Binaries: []string{"netdata"},
		Packages:       []string{"netdata"},
		ConfigPaths:    []string{"/etc/netdata/netdata.conf"},
		SignaturePorts: []int{19999}},
	{Name: "collectd", Label: "collectd", Vendor: "collectd", Category: "monitoring",
		Units: []string{"collectd"}, Binaries: []string{"collectd"},
		Packages: []string{"collectd"}, ConfigPaths: []string{"/etc/collectd/collectd.conf", "/etc/collectd.conf"}},
	{Name: "datadog-agent", Label: "Datadog Agent", Vendor: "Datadog", Category: "monitoring",
		Units: []string{"datadog-agent"}, Binaries: []string{"datadog-agent"},
		Packages: []string{"datadog-agent"}, ConfigPaths: []string{"/etc/datadog-agent/datadog.yaml"}},
	{Name: "newrelic-infra", Label: "New Relic Infrastructure", Vendor: "New Relic", Category: "monitoring",
		Units: []string{"newrelic-infra"}, Binaries: []string{"newrelic-infra"},
		Packages: []string{"newrelic-infra"}, ConfigPaths: []string{"/etc/newrelic-infra.yml"}},
	{Name: "prometheus", Label: "Prometheus", Vendor: "Prometheus", Category: "monitoring",
		Units: []string{"prometheus"}, Binaries: []string{"prometheus"},
		Packages:       []string{"prometheus"},
		ConfigPaths:    []string{"/etc/prometheus/prometheus.yml"},
		SignaturePorts: []int{9090}},

	// ── Logging ──
	{Name: "filebeat", Label: "Filebeat", Vendor: "Elastic", Category: "logging",
		Units: []string{"filebeat"}, Binaries: []string{"filebeat"},
		Packages: []string{"filebeat"}, ConfigPaths: []string{"/etc/filebeat/filebeat.yml"}},
	{Name: "metricbeat", Label: "Metricbeat", Vendor: "Elastic", Category: "monitoring",
		Units: []string{"metricbeat"}, Binaries: []string{"metricbeat"},
		Packages: []string{"metricbeat"}, ConfigPaths: []string{"/etc/metricbeat/metricbeat.yml"}},
	{Name: "auditbeat", Label: "Auditbeat", Vendor: "Elastic", Category: "security",
		Units: []string{"auditbeat"}, Binaries: []string{"auditbeat"},
		Packages: []string{"auditbeat"}, ConfigPaths: []string{"/etc/auditbeat/auditbeat.yml"}},
	{Name: "fluent-bit", Label: "Fluent Bit", Vendor: "Fluent", Category: "logging",
		Units: []string{"fluent-bit", "td-agent-bit"}, Binaries: []string{"fluent-bit", "td-agent-bit"},
		Packages: []string{"fluent-bit", "td-agent-bit"}, ConfigPaths: []string{"/etc/fluent-bit/fluent-bit.conf"}},
	{Name: "fluentd", Label: "Fluentd", Vendor: "Fluent", Category: "logging",
		Units: []string{"fluentd", "td-agent"}, Binaries: []string{"fluentd", "td-agent"},
		Packages: []string{"fluentd", "td-agent"},
		ConfigPaths: []string{"/etc/fluent/fluent.conf", "/etc/td-agent/td-agent.conf"}},
	{Name: "vector", Label: "Vector", Vendor: "Datadog", Category: "logging",
		Units: []string{"vector"}, Binaries: []string{"vector"},
		Packages: []string{"vector"}, ConfigPaths: []string{"/etc/vector/vector.toml", "/etc/vector/vector.yaml"}},
	{Name: "rsyslog", Label: "rsyslog", Vendor: "Adiscon", Category: "logging",
		Units: []string{"rsyslog"}, Binaries: []string{"rsyslogd"},
		Packages: []string{"rsyslog"}, ConfigPaths: []string{"/etc/rsyslog.conf"}},

	// ── Security / EDR ──
	{Name: "wazuh-agent", Label: "Wazuh Agent", Vendor: "Wazuh", Category: "security",
		Units: []string{"wazuh-agent"}, Binaries: []string{"wazuh-agentd"},
		Packages: []string{"wazuh-agent"}, ConfigPaths: []string{"/var/ossec/etc/ossec.conf"}},
	{Name: "osquery", Label: "osquery", Vendor: "osquery", Category: "security",
		Units: []string{"osqueryd"}, Binaries: []string{"osqueryd"},
		Packages: []string{"osquery"}, ConfigPaths: []string{"/etc/osquery/osquery.conf"}},
	{Name: "falcon-sensor", Label: "CrowdStrike Falcon", Vendor: "CrowdStrike", Category: "security",
		Units: []string{"falcon-sensor"}, Binaries: []string{"falcond", "falcon-sensor"},
		Packages: []string{"falcon-sensor"}, ConfigPaths: []string{"/opt/CrowdStrike"}},
	{Name: "qualys-cloud-agent", Label: "Qualys Cloud Agent", Vendor: "Qualys", Category: "security",
		Units: []string{"qualys-cloud-agent"}, Binaries: []string{"qualys-cloud-agent"},
		Packages: []string{"qualys-cloud-agent"}, ConfigPaths: []string{"/etc/qualys/cloud-agent"}},
	{Name: "clamav", Label: "ClamAV", Vendor: "Cisco", Category: "security",
		Units: []string{"clamav-daemon", "clamav-freshclam"}, Binaries: []string{"clamd", "freshclam"},
		Packages: []string{"clamav-daemon", "clamav"}, ConfigPaths: []string{"/etc/clamav/clamd.conf"}},
	{Name: "auditd", Label: "Linux Audit Daemon", Vendor: "Linux Audit", Category: "security",
		Units: []string{"auditd"}, Binaries: []string{"auditd"},
		Packages: []string{"auditd"}, ConfigPaths: []string{"/etc/audit/auditd.conf"}},
	{Name: "fail2ban", Label: "Fail2ban", Vendor: "Fail2ban", Category: "security",
		Units: []string{"fail2ban"}, Binaries: []string{"fail2ban-server"},
		Packages: []string{"fail2ban"}, ConfigPaths: []string{"/etc/fail2ban/jail.local", "/etc/fail2ban/jail.conf"}},

	// ── Inventory / ITSM ──
	{Name: "glpi-agent", Label: "GLPI Agent", Vendor: "Teclib", Category: "inventory",
		Units: []string{"glpi-agent"}, Binaries: []string{"glpi-agent"},
		Packages: []string{"glpi-agent"}, ConfigPaths: []string{"/etc/glpi-agent/agent.cfg"}},
	{Name: "fusioninventory-agent", Label: "FusionInventory Agent", Vendor: "FusionInventory", Category: "inventory",
		Units: []string{"fusioninventory-agent"}, Binaries: []string{"fusioninventory-agent"},
		Packages: []string{"fusioninventory-agent"}, ConfigPaths: []string{"/etc/fusioninventory/agent.cfg"}},
	{Name: "ocs-inventory-agent", Label: "OCS Inventory Agent", Vendor: "OCS Inventory", Category: "inventory",
		Units: []string{"ocsinventory-agent"}, Binaries: []string{"ocsinventory-agent"},
		Packages: []string{"ocsinventory-agent"}, ConfigPaths: []string{"/etc/ocsinventory"}},

	// ── Config management ──
	{Name: "puppet-agent", Label: "Puppet Agent", Vendor: "Puppet", Category: "config-mgmt",
		Units: []string{"puppet"}, Binaries: []string{"puppet"},
		Packages: []string{"puppet-agent", "puppet"},
		ConfigPaths: []string{"/etc/puppetlabs/puppet/puppet.conf"}},
	{Name: "salt-minion", Label: "Salt Minion", Vendor: "SaltStack", Category: "config-mgmt",
		Units: []string{"salt-minion"}, Binaries: []string{"salt-minion"},
		Packages: []string{"salt-minion"}, ConfigPaths: []string{"/etc/salt/minion"}},
	{Name: "chef-client", Label: "Chef Client", Vendor: "Progress (Chef)", Category: "config-mgmt",
		Units: []string{"chef-client"}, Binaries: []string{"chef-client"},
		Packages: []string{"chef"}, ConfigPaths: []string{"/etc/chef/client.rb"}},
	{Name: "ansible-pull", Label: "Ansible Pull", Vendor: "Red Hat", Category: "config-mgmt",
		Binaries: []string{"ansible-pull"}, Packages: []string{"ansible", "ansible-core"}},

	// ── Cloud / orchestration ──
	{Name: "amazon-ssm-agent", Label: "AWS SSM Agent", Vendor: "Amazon Web Services", Category: "cloud",
		Units: []string{"amazon-ssm-agent", "snap.amazon-ssm-agent.amazon-ssm-agent"},
		Binaries: []string{"amazon-ssm-agent"},
		Packages: []string{"amazon-ssm-agent"}, ConfigPaths: []string{"/etc/amazon/ssm"}},
	{Name: "cloud-init", Label: "cloud-init", Vendor: "Canonical", Category: "cloud",
		Units: []string{"cloud-init", "cloud-init-local", "cloud-config", "cloud-final"},
		Binaries: []string{"cloud-init"},
		Packages: []string{"cloud-init"}, ConfigPaths: []string{"/etc/cloud/cloud.cfg"}},
	{Name: "walinuxagent", Label: "Azure Linux Agent", Vendor: "Microsoft", Category: "cloud",
		Units: []string{"walinuxagent"}, Binaries: []string{"waagent"},
		Packages: []string{"walinuxagent"}, ConfigPaths: []string{"/etc/waagent.conf"}},
	{Name: "google-osconfig-agent", Label: "Google OS Config Agent", Vendor: "Google", Category: "cloud",
		Units: []string{"google-osconfig-agent"}, Binaries: []string{"google_osconfig_agent"},
		Packages: []string{"google-osconfig-agent"}, ConfigPaths: []string{"/etc/osconfig"}},
	{Name: "consul", Label: "HashiCorp Consul", Vendor: "HashiCorp", Category: "orchestration",
		Units: []string{"consul"}, Binaries: []string{"consul"},
		Packages: []string{"consul"}, ConfigPaths: []string{"/etc/consul.d", "/etc/consul/consul.hcl"}},
	{Name: "nomad", Label: "HashiCorp Nomad", Vendor: "HashiCorp", Category: "orchestration",
		Units: []string{"nomad"}, Binaries: []string{"nomad"},
		Packages: []string{"nomad"}, ConfigPaths: []string{"/etc/nomad.d"}},
	{Name: "kubelet", Label: "Kubernetes kubelet", Vendor: "CNCF", Category: "orchestration",
		Units: []string{"kubelet"}, Binaries: []string{"kubelet"},
		Packages: []string{"kubelet"}, ConfigPaths: []string{"/etc/kubernetes/kubelet.conf"}},
	{Name: "containerd", Label: "containerd", Vendor: "CNCF", Category: "orchestration",
		Units: []string{"containerd"}, Binaries: []string{"containerd"},
		Packages: []string{"containerd", "containerd.io"},
		ConfigPaths: []string{"/etc/containerd/config.toml"}},
	{Name: "k3s", Label: "k3s", Vendor: "Rancher (SUSE)", Category: "orchestration",
		Units: []string{"k3s", "k3s-agent"}, Binaries: []string{"k3s"},
		Packages: []string{"k3s"}, ConfigPaths: []string{"/etc/rancher/k3s/k3s.yaml"}},

	// ── Backup ──
	{Name: "bacula-fd", Label: "Bacula File Daemon", Vendor: "Bacula", Category: "backup",
		Units: []string{"bacula-fd"}, Binaries: []string{"bacula-fd"},
		Packages: []string{"bacula-fd"}, ConfigPaths: []string{"/etc/bacula/bacula-fd.conf"}},
	{Name: "bareos-fd", Label: "Bareos File Daemon", Vendor: "Bareos", Category: "backup",
		Units: []string{"bareos-fd"}, Binaries: []string{"bareos-fd"},
		Packages: []string{"bareos-filedaemon"}, ConfigPaths: []string{"/etc/bareos/bareos-fd.d"}},
	{Name: "restic", Label: "Restic", Vendor: "Restic", Category: "backup",
		Binaries: []string{"restic"}, Packages: []string{"restic"}},

	// ── Remote access / VPN ──
	{Name: "tailscale", Label: "Tailscale", Vendor: "Tailscale", Category: "remote-access",
		Units: []string{"tailscaled"}, Binaries: []string{"tailscaled"},
		Packages: []string{"tailscale"}, ConfigPaths: []string{"/var/lib/tailscale"}},
	{Name: "teleport", Label: "Teleport", Vendor: "Teleport", Category: "remote-access",
		Units: []string{"teleport"}, Binaries: []string{"teleport"},
		Packages: []string{"teleport"}, ConfigPaths: []string{"/etc/teleport.yaml"}},
	{Name: "wireguard", Label: "WireGuard", Vendor: "WireGuard", Category: "remote-access",
		Units: []string{"wg-quick@"}, Binaries: []string{"wg"},
		Packages: []string{"wireguard-tools"}, ConfigPaths: []string{"/etc/wireguard"}},
	{Name: "openvpn", Label: "OpenVPN", Vendor: "OpenVPN", Category: "remote-access",
		Units: []string{"openvpn", "openvpn@"}, Binaries: []string{"openvpn"},
		Packages: []string{"openvpn"}, ConfigPaths: []string{"/etc/openvpn"}},
}

// hostInventory is the per-scan snapshot of facts we cross-reference when
// classifying agents and services. It bundles the four enumerations that
// both the agent and service classifiers need so the scan only pays for
// one SSH round-trip instead of two.
type hostInventory struct {
	UnitFiles   map[string]string // <unit>.service → install state
	LoadedUnits map[string]string // <unit>.service → active state
	Packages    map[string]string // package name → version
	Procs       []procEntry       // running processes
	PortsByProc map[string][]int  // process basename → listen ports
	ListenPorts map[int]bool      // every TCP port with a listener (for signature-port detection)
}

// catalogPackageNames returns every package name referenced across the
// agent and service catalogs. We pre-shell-escape to keep the command
// safe even if a future catalog entry contains an exotic character —
// today everything is `[a-zA-Z0-9_.-]+` but that's not guaranteed.
func catalogPackageNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, spec := range agentCatalog {
		for _, p := range spec.Packages {
			add(p)
		}
	}
	for _, spec := range serviceCatalog {
		for _, p := range spec.Packages {
			add(p)
		}
	}
	return out
}

// collectHostInventory runs one SSH batch that enumerates installed unit
// files, loaded units, the catalog packages we actually care about, and
// running processes. The resulting snapshot is reused by captureAgents
// and captureServices so neither has to repeat the pulls.
//
// We intentionally don't dump the full dpkg/rpm package list — a busy
// server has 2000+ packages and that's 80–100 KB over the SSH link per
// scan, every time. Querying only the ~150 catalog package names cuts
// the payload to a couple of KB and shaves several seconds off slow
// connections (which is what was pushing the dev proxy past its idle
// timeout for some hosts). Packages outside the catalog are
// uninteresting for classification anyway.
func collectHostInventory(client *ssh.Client, portOwners []PortOwner) *hostInventory {
	const delim = "---HOSTINV---"
	pkgNames := catalogPackageNames()
	pkgArgs := make([]string, len(pkgNames))
	for i, n := range pkgNames {
		pkgArgs[i] = shellEscape(n)
	}
	pkgList := strings.Join(pkgArgs, " ")
	// dpkg-query exits non-zero when *any* requested package is absent
	// (the normal case — most catalog entries don't apply to a given
	// host), so a `||` fallback would trip on every Debian box. Detect
	// the package manager up front and dispatch to the right tool;
	// rpm-side filtering uses grep over the catalog names so we don't
	// drag the full installed-package list across the wire.
	rpmGrepPattern := strings.Join(pkgArgs, "|")
	cmd := strings.Join([]string{
		// 0: every installed service unit and its default state
		`systemctl list-unit-files --type=service --no-pager --no-legend 2>/dev/null`,
		`echo '` + delim + `'`,
		// 1: every loaded service unit and its current active state
		`systemctl list-units --type=service --all --no-pager --plain --no-legend 2>/dev/null`,
		`echo '` + delim + `'`,
		// 2: catalog packages only — keep the transfer small.
		`if command -v dpkg-query >/dev/null 2>&1; then ` +
			`dpkg-query -W -f='${Package}\t${Version}\n' ` + pkgList + ` 2>/dev/null; ` +
			`elif command -v rpm >/dev/null 2>&1; then ` +
			`rpm -qa --queryformat '%{NAME}\t%{VERSION}\n' 2>/dev/null | grep -E '^(` + rpmGrepPattern + `)\t'; ` +
			`fi | head -n 500`,
		`echo '` + delim + `'`,
		// 3: running processes — pid/user/comm/args; comm is the basename
		//    (truncated to 15 chars on Linux), so we also keep argv[0] for
		//    binaries whose name exceeds 15 chars (e.g. amazon-ssm-agent).
		`ps -eo pid=,user=,comm=,args= 2>/dev/null | head -n 5000`,
	}, "; ")

	raw := runCmd(client, cmd)
	sections := splitSections(raw, delim, 4)

	inv := &hostInventory{
		UnitFiles:   parseUnitFiles(sections[0]),
		LoadedUnits: parseLoadedUnits(sections[1]),
		Packages:    parsePackageList(sections[2]),
		Procs:       parsePsOutput(sections[3]),
		PortsByProc: map[string][]int{},
		ListenPorts: map[int]bool{},
	}

	// Pre-index port owners by process basename for O(1) lookup per spec
	// and build a flat set of every listening port so signature-port
	// detection (e.g., :10050 → Zabbix) works even when the process
	// owner doesn't match a catalog binary.
	for _, po := range portOwners {
		inv.ListenPorts[po.Port] = true
		key := strings.TrimSpace(po.Process)
		if key == "" && po.OwnerName != "" {
			key = po.OwnerName
		}
		if key == "" {
			continue
		}
		inv.PortsByProc[key] = append(inv.PortsByProc[key], po.Port)
	}
	return inv
}

// captureAgents intersects hostInventory with the agent catalog. The
// inventory is collected once per scan via collectHostInventory and shared
// with captureServices to avoid duplicate SSH work.
//
// Returns the matched agents and the set of signature ports they claimed,
// so captureServices can avoid re-attributing the same port (e.g., :10050
// → Zabbix Agent 2 should not also be flagged by any service spec).
func captureAgents(inv *hostInventory) ([]Agent, map[int]bool) {
	portClaims := map[int]bool{}
	var agents []Agent
	for _, spec := range agentCatalog {
		agent, ok := detectAgent(spec, inv, portClaims)
		if !ok {
			continue
		}
		for _, p := range spec.SignaturePorts {
			if inv.ListenPorts[p] {
				portClaims[p] = true
			}
		}
		agents = append(agents, agent)
	}
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Category != agents[j].Category {
			return agents[i].Category < agents[j].Category
		}
		return agents[i].Label < agents[j].Label
	})
	return agents, portClaims
}

// detectAgent returns ok=false if no signal in spec matched the captured
// host state. Otherwise returns an Agent populated with whichever fields
// the matching signals provided. portClaims is the set of signature
// ports already claimed by an earlier spec; ports in this set don't
// trigger detection here so the catalog can't double-count.
func detectAgent(spec agentSpec, inv *hostInventory, portClaims map[int]bool) (Agent, bool) {
	agent := Agent{Name: spec.Name, Label: spec.Label, Vendor: spec.Vendor, Category: spec.Category}
	if len(spec.ConfigPaths) > 0 {
		agent.ConfigPath = spec.ConfigPaths[0]
	}
	sources := map[string]bool{}

	// systemd unit-files (installed); drives Enabled state
	for _, want := range spec.Units {
		for unit, state := range inv.UnitFiles {
			if unitMatches(want, unit) {
				agent.Unit = stripUnitSuffix(unit)
				agent.Enabled = state == "enabled" || state == "static" || state == "alias" || state == "enabled-runtime"
				sources["systemd"] = true
				goto unitsDone
			}
		}
	}
unitsDone:
	// systemd loaded units (active state)
	for _, want := range spec.Units {
		for unit, active := range inv.LoadedUnits {
			if unitMatches(want, unit) {
				if agent.Unit == "" {
					agent.Unit = stripUnitSuffix(unit)
				}
				agent.State = active
				sources["systemd"] = true
				goto loadedDone
			}
		}
	}
loadedDone:

	// Running processes
	for _, p := range inv.Procs {
		for _, b := range spec.Binaries {
			if binaryMatches(b, p.comm, p.args) {
				if agent.PID == 0 {
					agent.PID = p.pid
				}
				if agent.State == "" {
					agent.State = "running"
				}
				sources["process"] = true
				goto procsDone
			}
		}
	}
procsDone:

	// Packages (provides version string when found)
	for _, pkg := range spec.Packages {
		if v, ok := inv.Packages[pkg]; ok {
			agent.Package = pkg
			agent.Version = v
			sources["package"] = true
			break
		}
	}

	// Listen ports — match owner process basename to any catalog binary,
	// then add SignaturePorts as a fallback for hosts where the agent
	// runs under a non-standard binary name (custom build, snap install,
	// containerized variant). Signature ports already claimed by an
	// earlier catalog entry are skipped so we never double-count.
	portSet := map[int]bool{}
	for _, b := range spec.Binaries {
		for _, port := range inv.PortsByProc[b] {
			portSet[port] = true
			sources["port"] = true
		}
	}
	for _, p := range spec.SignaturePorts {
		if !inv.ListenPorts[p] {
			continue
		}
		if portClaims[p] {
			continue
		}
		portSet[p] = true
		sources["port"] = true
	}
	if len(portSet) > 0 {
		for p := range portSet {
			agent.Ports = append(agent.Ports, p)
		}
		sort.Ints(agent.Ports)
	}

	if len(sources) == 0 {
		return Agent{}, false
	}

	// Finalize state. If we have a package signal but no systemd/process
	// signal, the agent is installed but not currently active.
	if agent.State == "" {
		if sources["package"] {
			agent.State = "stopped"
		}
	}

	for src := range sources {
		agent.Sources = append(agent.Sources, src)
	}
	sort.Strings(agent.Sources)
	return agent, true
}

// unitMatches reports whether `unit` (with optional .service suffix) matches
// the catalog unit basename. A trailing "@" on the catalog name matches
// templated systemd instances (wg-quick@wg0, openvpn@server, …).
func unitMatches(specUnit, unit string) bool {
	candidate := strings.TrimSuffix(unit, ".service")
	if strings.HasSuffix(specUnit, "@") {
		return strings.HasPrefix(candidate, specUnit)
	}
	return candidate == specUnit
}

func stripUnitSuffix(unit string) string {
	return strings.TrimSuffix(unit, ".service")
}

// binaryMatches handles Linux's 15-char comm truncation by also checking
// the basename of argv[0]. Exact match only — no substring matching, since
// substrings would catch "agent" everywhere.
func binaryMatches(specBinary, comm, args string) bool {
	if comm == specBinary {
		return true
	}
	// argv[0] basename fallback for binaries whose name exceeds 15 chars.
	if args != "" {
		if first := strings.Fields(args); len(first) > 0 {
			if path.Base(first[0]) == specBinary {
				return true
			}
		}
	}
	return false
}

// parseUnitFiles parses `systemctl list-unit-files --no-legend` output:
// "<unit>  <state>  [<vendor-preset>]". Returns unit → state.
func parseUnitFiles(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasSuffix(fields[0], ".service") {
			out[fields[0]] = fields[1]
		}
	}
	return out
}

// parseLoadedUnits parses `systemctl list-units --plain --no-legend` output:
// "<unit>  <load>  <active>  <sub>  <description...>". Returns unit → active.
func parseLoadedUnits(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.HasSuffix(fields[0], ".service") {
			out[fields[0]] = fields[2]
		}
	}
	return out
}

// parsePackageList parses tab-separated "name\tversion" lines from
// `dpkg-query -W` or `rpm -qa --queryformat`. Returns name → version.
func parsePackageList(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}

type procEntry struct {
	pid  int
	user string
	comm string
	args string
}

// parsePsOutput parses `ps -eo pid=,user=,comm=,args=` lines into
// procEntry slices. comm is the kernel-truncated basename; args is the
// full argv string used as a fallback when comm is truncated.
func parsePsOutput(s string) []procEntry {
	var out []procEntry
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split off pid (1st field), user (2nd), comm (3rd), args (rest).
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		args := ""
		if len(fields) >= 4 {
			args = strings.Join(fields[3:], " ")
		}
		out = append(out, procEntry{pid: pid, user: fields[1], comm: fields[2], args: args})
	}
	return out
}
