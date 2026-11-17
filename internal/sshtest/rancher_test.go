package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// `iptables -t nat -S` on a Rancher 1.6 (Cattle) host fronted like
// SAPASP-DOCKER01: :80 to the LB, :9090 to Prometheus. Around them, rules that
// must not count: ipsec's UDP, metadata's 169.254.169.250:80, a stale rule to
// a gone container, MASQUERADE, docker's own chain.
const rancherNatSample = `-P PREROUTING ACCEPT
-P INPUT ACCEPT
-P OUTPUT ACCEPT
-P POSTROUTING ACCEPT
-N CATTLE_HOSTPORTS_POSTROUTING
-N CATTLE_NETWORK_POSTROUTING
-N CATTLE_OUTPUT
-N CATTLE_POSTROUTING
-N CATTLE_PREROUTING
-N DOCKER
-A PREROUTING -j CATTLE_PREROUTING
-A PREROUTING -m addrtype --dst-type LOCAL -j DOCKER
-A OUTPUT -j CATTLE_OUTPUT
-A OUTPUT ! -d 127.0.0.0/8 -m addrtype --dst-type LOCAL -j DOCKER
-A POSTROUTING -j CATTLE_POSTROUTING
-A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE
-A CATTLE_HOSTPORTS_POSTROUTING -s 10.42.12.34/32 -d 10.42.12.34/32 -p tcp -m tcp --dport 80 -j MASQUERADE
-A CATTLE_NETWORK_POSTROUTING -s 10.42.0.0/16 ! -d 10.42.0.0/16 -p tcp -j MASQUERADE --to-ports 1024-65535
-A CATTLE_OUTPUT -p tcp -m addrtype --dst-type LOCAL -m tcp --dport 9090 -j DNAT --to-destination 10.42.99.7:9090
-A CATTLE_POSTROUTING -j CATTLE_NETWORK_POSTROUTING
-A CATTLE_POSTROUTING -j CATTLE_HOSTPORTS_POSTROUTING
-A CATTLE_PREROUTING -d 169.254.169.250/32 -p tcp -m tcp --dport 80 -j DNAT --to-destination 10.42.5.9:80
-A CATTLE_PREROUTING -p udp -m addrtype --dst-type LOCAL -m udp --dport 500 -j DNAT --to-destination 10.42.20.5:500
-A CATTLE_PREROUTING -p udp -m addrtype --dst-type LOCAL -m udp --dport 4500 -j DNAT --to-destination 10.42.20.5:4500
-A CATTLE_PREROUTING -p tcp -m addrtype --dst-type LOCAL -m tcp --dport 80 -j DNAT --to-destination 10.42.12.34:80
-A CATTLE_PREROUTING -p tcp -m addrtype --dst-type LOCAL -m tcp --dport 9090 -j DNAT --to-destination 10.42.99.7:9090
-A CATTLE_PREROUTING -d 10.0.122.16/32 -p tcp -m tcp --dport 8443 -j DNAT --to-destination 10.42.12.34:443
-A CATTLE_PREROUTING -p tcp -m addrtype --dst-type LOCAL -m tcp --dport 7000:7001 -j DNAT --to-destination 10.42.12.34:7000-7001
-A CATTLE_PREROUTING -p tcp -m addrtype --dst-type LOCAL -m tcp --dport 6000 -j DNAT --to-destination 10.42.77.1:6000
-A DOCKER -i docker0 -j RETURN
-A DOCKER ! -i docker0 -p tcp -m tcp --dport 9216 -j DNAT --to-destination 172.17.0.4:9216
`

const (
	rancherLB   = "r-siace-siace-lb-1-0a1b2c3d"
	rancherProm = "r-02-prometheus-02-prometheus-main-1-5f3a9c21"
)

// runRancherScript runs rancherPortsScript with fake iptables, docker, ip and
// sudo first on PATH. docker leaves a marker when called.
func runRancherScript(t *testing.T, iptables string) (out string, dockerCalled bool) {
	t.Helper()
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "docker-called")
	for f, body := range map[string]string{
		"iptables": iptables,
		"sudo":     "exit 1",
		"ip": `printf '%s\n' '1: lo    inet 127.0.0.1/8 scope host lo\       valid_lft forever preferred_lft forever' \
  '2: ens160    inet 10.0.122.16/24 brd 10.0.122.255 scope global ens160\       valid_lft forever preferred_lft forever'`,
		"docker": `: > "$DOCKER_MARKER"
case "$1" in
ps) printf 'aaa\nbbb\nccc\nddd\n';;
inspect) printf '%s\n' '/` + rancherLB + ` 10.42.12.34/16 ' \
  '/` + rancherProm + ` 10.42.99.7/16 ' \
  '/r-network-services-metadata-1-1a2b3c4d 10.42.5.9/16 ' \
  '/r-ipsec-ipsec-router-1-77aa1b2c 10.42.20.5/16 ' \
  '/rancher-agent  172.17.0.3 ';;
esac`,
	} {
		if err := os.WriteFile(filepath.Join(bin, f), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "-c", rancherPortsScript)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "DOCKER_MARKER="+marker)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, b)
	}
	_, statErr := os.Stat(marker)
	return string(b), statErr == nil
}

func TestRancherPortsScript(t *testing.T) {
	out, _ := runRancherScript(t, "cat <<'EOF'\n"+rancherNatSample+"EOF")
	// Exactly these lines, in rule order: no address leaves the VM, and no
	// metadata, ipsec or gone-container rule slips in.
	wantOut := "9090 " + rancherProm + "\n80 " + rancherLB + "\n9090 " + rancherProm + "\n8443 " + rancherLB + "\n7000-7001 " + rancherLB + "\n"
	if out != wantOut {
		t.Fatalf("output:\n%s\nwant:\n%s", out, wantOut)
	}
	prom := ContainerInfo{Name: rancherProm, Image: "prom/prometheus:v2.1.0"}
	got := parseRancherPorts(out, []ContainerInfo{prom})
	want := map[int]string{80: rancherLB, 9090: rancherProm, 8443: rancherLB, 7000: rancherLB, 7001: rancherLB}
	if len(got) != len(want) {
		t.Fatalf("ports = %v, want %v\noutput:\n%s", got, want, out)
	}
	for p, name := range want {
		if got[p] == nil || got[p].Name != name {
			t.Fatalf("port %d = %+v, want %s\noutput:\n%s", p, got[p], name, out)
		}
	}
	if got[9090].Image != prom.Image {
		t.Fatalf("9090 lost the scanned container's image: %+v", got[9090])
	}
}

// Any other host prints nothing, fails nothing, and never asks docker.
func TestRancherPortsScriptOtherHosts(t *testing.T) {
	for name, iptables := range map[string]string{
		"docker only": "printf '%s\\n' '-P PREROUTING ACCEPT' '-N DOCKER' '-A PREROUTING -m addrtype --dst-type LOCAL -j DOCKER' '-A DOCKER ! -i docker0 -p tcp -m tcp --dport 8080 -j DNAT --to-destination 172.17.0.2:80'",
		"not root":    "echo \"iptables: can't initialize iptables table 'nat': Permission denied (you must be root)\" >&2; exit 3",
	} {
		t.Run(name, func(t *testing.T) {
			out, dockerCalled := runRancherScript(t, iptables)
			if out != "" || dockerCalled {
				t.Fatalf("output %q, docker called %v", out, dockerCalled)
			}
		})
	}
}
