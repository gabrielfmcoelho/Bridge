package sshtest

import (
	"strings"

	"golang.org/x/crypto/ssh"
)

// rancherPortsScript prints "<host port>[-<end>] <container>" for every TCP
// port a Rancher 1.6 (Cattle) host publishes. Managed networking DNATs it in
// the nat table (CATTLE_* chains) to the container's 10.42.x.x IP, so neither
// ss nor docker ps shows it. The join runs here: only port numbers and
// container names leave the VM. A rule with -d counts only for one of the
// host's own addresses (metadata's 169.254.169.250:80 is not a host port).
// No CATTLE DNAT rule (any other host): no output, docker never asked.
const rancherPortsScript = scanPath + `
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 10"
R=$( { $T iptables -t nat -S 2>/dev/null || $T sudo -n iptables -t nat -S 2>/dev/null; } | grep -E '^-A CATTLE[A-Z_]* .*-p tcp .*-j DNAT')
[ -n "$R" ] || exit 0
D=docker; $T docker ps -q >/dev/null 2>&1 || D="sudo -n docker"
I=$($T $D ps -q 2>/dev/null)
[ -n "$I" ] || exit 0
{
  ip -o addr show 2>/dev/null | awk '{sub(/\/.*/, "", $4); print "@addr", $4}'
  $T $D inspect -f '{{.Name}} {{index .Config.Labels "io.rancher.container.ip"}} {{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' $I 2>/dev/null
  printf '%s\n' "$R"
} | awk '
$1 == "@addr" { own[$2] = 1; next }
/^\// { n = substr($1, 2); for (i = 2; i <= NF; i++) { a = $i; sub(/\/.*/, "", a); ctr[a] = n } next }
{
  dp = ""; to = ""; d = ""
  for (i = 1; i < NF; i++) {
    if ($i == "--dport" && $(i-1) != "!") dp = $(i+1)
    else if ($i == "--to-destination") to = $(i+1)
    else if ($i == "-d" && $(i-1) != "!") d = $(i+1)
  }
  sub(/:.*/, "", to); sub(/\/.*/, "", d); sub(/:/, "-", dp)
  if (dp != "" && (to in ctr) && (d == "" || (d in own))) print dp, ctr[to]
}'
`

// collectRancherPorts runs rancherPortsScript as root with the stored password
// (iptables needs it), else unprivileged (the script tries sudo -n itself).
func collectRancherPorts(client *ssh.Client, sudoPassword string) string {
	if sudoPassword != "" {
		if out, err := runSudoCmd(client, sudoPassword, rancherPortsScript); err == nil {
			return out
		}
	}
	return runCmd(client, encodedScript(rancherPortsScript, "sh"))
}

// parseRancherPorts maps each published port to its container: the scanned
// one when docker ps listed it (image included), else just the name.
func parseRancherPorts(raw string, containers []ContainerInfo) map[int]*ContainerInfo {
	byName := map[string]*ContainerInfo{}
	for i := range containers {
		byName[containers[i].Name] = &containers[i]
	}
	out := map[int]*ContainerInfo{}
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		c := byName[f[1]]
		if c == nil {
			c = &ContainerInfo{Name: f[1]}
		}
		start, end := parsePortRange(f[0])
		for p := start; p > 0 && p <= end; p++ {
			out[p] = c
		}
	}
	return out
}
