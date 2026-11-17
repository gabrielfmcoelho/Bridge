package sshtest

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// testdata/kong-admin-3.4.txt is kongInnerScript's real output against a
// DB-less Kong 3.4.2 (kong:3.4-ubuntu) with an upstream (two targets), a
// route on two hosts and two paths, and a host-less regex route.
func TestParseKongCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/kong-admin-3.4.txt")
	if err != nil {
		t.Fatal(err)
	}
	servers := webServersFrom(webSections(string(raw)))
	if len(servers) != 1 || servers[0].Kind != "kong" || servers[0].Error != "" || servers[0].Kong == nil {
		t.Fatalf("servers = %+v, want one readable kong node", servers)
	}
	k := servers[0].Kong
	if k.Prefix != "/usr/local/kong" || k.Version != "3.4.2" || k.Database != "off" || k.DBHost != "" || k.DBReachable != nil {
		t.Errorf("node = %+v, want 3.4.2 DB-less at /usr/local/kong", k)
	}
	if !reflect.DeepEqual(k.ProxyPorts, []int{80, 443}) || !reflect.DeepEqual(k.AdminPorts, []int{8001, 8002, 8444, 8445}) {
		t.Errorf("ports proxy=%v admin=%v", k.ProxyPorts, k.AdminPorts)
	}
	if k.Services != 3 || k.Routes != 3 || k.Connections == 0 || k.Requests == 0 {
		t.Errorf("counts = %+v", k)
	}
	if len(k.Upstreams) != 1 || k.Upstreams[0].Name != "compras-upstream" || len(k.Upstreams[0].Targets) != 2 ||
		k.Upstreams[0].Targets[0].Health != "HEALTHCHECKS_OFF" {
		t.Errorf("upstreams = %+v", k.Upstreams)
	}

	got := map[string][]string{}
	for _, s := range servers[0].Sites {
		if !s.TLS || !reflect.DeepEqual(s.Listen, []int{80, 443}) {
			t.Errorf("site %v: tls=%v listen=%v", s.Names, s.TLS, s.Listen)
		}
		for _, l := range s.Locations {
			ts := append([]string{}, l.Proxy...)
			sort.Strings(ts)
			got[s.Names[0]+" "+l.Path] = ts
		}
	}
	want := map[string][]string{
		"_ ~/(auth|hal)":           {"http://10.0.122.16:80"},
		"bi.pi.gov.br /bi":         {"http://10.0.122.15:8080"},
		"bi.pi.gov.br /pentaho":    {"http://10.0.122.15:8080"},
		"siace.pi.gov.br /bi":      {"http://10.0.122.15:8080"},
		"siace.pi.gov.br /pentaho": {"http://10.0.122.15:8080"},
		"siace.pi.gov.br /sgc":     {"http://10.0.122.11:8080/sgc", "http://10.0.122.12:8080/sgc"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("locations =\n%v\nwant\n%v", got, want)
	}
}

// A node whose Admin API didn't answer is kept, with an error, so the scan
// keeps its previous web routes instead of dropping them.
func TestParseKongErrors(t *testing.T) {
	for in, want := range map[string]string{
		"@error admin API off":                     "Kong Admin API: admin API off",
		"@root curl: (7) Failed to connect to 127": "Kong Admin API: curl: (7) Failed to connect to 127",
		`@root {"message":"Unauthorized"}`:         `Kong Admin API: Unauthorized`,
	} {
		servers := webServersFrom(webSections("@@kong\n@prefix /usr/local/kong\n" + in + "\n"))
		if len(servers) != 1 || servers[0].Kind != "kong" || !strings.HasPrefix(servers[0].Error, want) {
			t.Errorf("%q → %+v, want error %q", in, servers, want)
		}
	}
}

// The probe's Kong node owns its listeners: the nginx entry (ss names Kong's
// processes "nginx") loses them and the Kong service is created running.
func TestClaimKongPorts(t *testing.T) {
	info := &VMInfo{
		ServiceInventory: []DiscoveredService{{Name: "nginx", Label: "Nginx", Ports: []int{80, 81, 8001}, HostPorts: []int{80, 81, 8001}}},
		WebServers: []WebServer{
			{Kind: "kong", Kong: &KongNode{Version: "2.8.1", ProxyPorts: []int{80, 443}, AdminPorts: []int{8001, 8444}}},
			{Kind: "kong", Container: "kong-1", Kong: &KongNode{ProxyPorts: []int{9000}}}, // a container's ports are docker's
		},
	}
	claimKongPorts(info)
	if len(info.ServiceInventory) != 2 {
		t.Fatalf("inventory = %+v", info.ServiceInventory)
	}
	nginx, kong := info.ServiceInventory[0], info.ServiceInventory[1]
	if !reflect.DeepEqual(nginx.HostPorts, []int{81}) || !reflect.DeepEqual(nginx.Ports, []int{81}) {
		t.Errorf("nginx kept %v / %v, want only 81", nginx.Ports, nginx.HostPorts)
	}
	if kong.Name != "kong" || kong.Label != "Kong Gateway" || !kong.HostRunning || kong.Version != "2.8.1" ||
		!reflect.DeepEqual(kong.HostPorts, []int{80, 443, 8001, 8444}) {
		t.Errorf("kong = %+v", kong)
	}
}

// Kong's OpenResty master and workers are "nginx" in ps: they must not make
// the host's nginx look running.
func TestDetectService_KongOpenRestyIsNotNginx(t *testing.T) {
	inv := &hostInventory{
		UnitFiles:   map[string]string{"nginx.service": "enabled"},
		LoadedUnits: map[string]string{"nginx.service": "inactive"},
		Packages:    map[string]string{"nginx": "1.18.0"},
		Procs: []procEntry{
			{pid: 10, user: "root", comm: "nginx", args: "nginx: master process /usr/local/openresty/nginx/sbin/nginx -p /usr/local/kong -c nginx.conf"},
			{pid: 11, user: "kong", comm: "nginx", args: "nginx: worker process"},
		},
		PortsByProc:   map[string][]int{},
		ListenPorts:   map[int]bool{80: true, 8001: true},
		ContainerPIDs: map[int]string{},
	}
	svc, ok := detectService(catalogSpec(t, "nginx"), inv, nil, map[int]*ContainerInfo{}, map[int]bool{}, map[int]bool{})
	if !ok {
		t.Fatal("nginx package not detected")
	}
	if svc.HostRunning || svc.PID != 0 {
		t.Errorf("nginx = %+v, want installed but not running (the processes are Kong's)", svc)
	}
}
