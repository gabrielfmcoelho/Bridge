package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func TestMachines(t *testing.T) {
	res := []Resource{
		{ID: "qemu/101", Type: "qemu", Node: "pve1", Name: "web", VMID: 101, Status: "running", MaxCPU: 4, MaxMem: 8 << 30, MaxDisk: 100 << 30},
		{ID: "qemu/900", Type: "qemu", Node: "pve1", Name: "tpl", VMID: 900, Template: 1},
		{ID: "lxc/102", Type: "lxc", Node: "pve1", VMID: 102, Status: "stopped", MaxMem: 512 << 20},
		{ID: "storage/pve1/local", Type: "storage", Node: "pve1"},
		{ID: "node/pve1", Type: "node", Node: "pve1", Status: "online", MaxCPU: 32},
	}
	got := Machines(res, map[string]string{"pve1": "10.0.0.1"}, map[string]string{"qemu/101": "10.0.0.10"})
	want := []Machine{
		{ProxmoxID: "node/pve1", Kind: "node", Name: "pve1", Node: "pve1", IP: "10.0.0.1", Running: true, CPU: "32 vCPU"},
		{ProxmoxID: "lxc/102", Kind: "lxc", Name: "lxc-102", Node: "pve1", VMID: 102, RAM: "512 MB", NoIPReason: "stopped"},
		{ProxmoxID: "qemu/101", Kind: "qemu", Name: "web", Node: "pve1", VMID: 101, IP: "10.0.0.10", Running: true, CPU: "4 vCPU", RAM: "8 GB", Disk: "100 GB"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Machines =\n%+v\nwant\n%+v", got, want)
	}
}

func TestPickIPv4(t *testing.T) {
	agent := []AgentIface{
		{Name: "lo", IPAddresses: []AgentAddr{{"127.0.0.1", "ipv4"}}},
		{Name: "eth0", IPAddresses: []AgentAddr{{"fe80::1", "ipv6"}, {"169.254.1.1", "ipv4"}, {"10.1.2.3", "ipv4"}}},
	}
	if got := AgentIPv4(agent); got != "10.1.2.3" {
		t.Fatalf("AgentIPv4 = %q", got)
	}
	lxc := []LXCIface{{Name: "lo", Inet: "127.0.0.1/8"}, {Name: "eth0", Inet: "192.168.5.9/24"}}
	if got := LXCIPv4(lxc); got != "192.168.5.9" {
		t.Fatalf("LXCIPv4 = %q", got)
	}
}

func TestClientAuthAndEnvelope(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=bridge@pve!sync=s3cr3t" {
			http.Error(w, `{"data":null}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api2/json/version":
			w.Write([]byte(`{"data":{"version":"8.2.4"}}`))
		case "/api2/json/cluster/status":
			w.Write([]byte(`{"data":[{"type":"cluster","name":"c"},{"type":"node","name":"pve1","ip":"10.0.0.1"}]}`))
		case "/api2/json/nodes/pve1/lxc/102/interfaces":
			w.Write([]byte(`{"data":[{"name":"eth0","inet":"10.0.0.7/24"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	if _, err := NewClient(srv.URL, "bridge@pve!sync", "s3cr3t", false).Version(ctx); err == nil {
		t.Fatal("self-signed cert accepted without skipVerify")
	}
	c := NewClient(srv.URL+"/", "bridge@pve!sync", "s3cr3t", true)
	if v, err := c.Version(ctx); err != nil || v != "8.2.4" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	if ips, err := c.NodeIPs(ctx); err != nil || !reflect.DeepEqual(ips, map[string]string{"pve1": "10.0.0.1"}) {
		t.Fatalf("NodeIPs = %v, %v", ips, err)
	}
	if ip, err := c.GuestIP(ctx, Resource{Type: "lxc", Node: "pve1", VMID: 102}); err != nil || ip != "10.0.0.7" {
		t.Fatalf("GuestIP = %q, %v", ip, err)
	}
	if _, err := NewClient(srv.URL, "bridge@pve!sync", "wrong", true).Version(ctx); err == nil {
		t.Fatal("bad token accepted")
	}
}

// TestClientMachines drives the whole collection path against a fake PVE:
// QEMU agent + LXC IPs, a guest whose agent errors, stopped guests and
// templates never queried, and /cluster/status failing without aborting.
func TestClientMachines(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/api2/json/cluster/resources":
			w.Write([]byte(`{"data":[
				{"id":"node/pve1","type":"node","node":"pve1","status":"online","maxcpu":32},
				{"id":"node/pve2","type":"node","node":"pve2","status":"offline"},
				{"id":"qemu/101","type":"qemu","node":"pve1","name":"web","vmid":101,"status":"running","maxcpu":4,"maxmem":8589934592,"maxdisk":107374182400},
				{"id":"qemu/104","type":"qemu","node":"pve2","name":"noagent","vmid":104,"status":"running"},
				{"id":"qemu/105","type":"qemu","node":"pve1","name":"off","vmid":105,"status":"stopped"},
				{"id":"qemu/900","type":"qemu","node":"pve1","name":"tpl","vmid":900,"status":"stopped","template":1},
				{"id":"lxc/102","type":"lxc","node":"pve1","name":"cache","vmid":102,"status":"running"},
				{"id":"storage/pve1/local","type":"storage","node":"pve1"}]}`))
		case "/api2/json/cluster/status":
			http.Error(w, "boom", http.StatusInternalServerError)
		case "/api2/json/nodes/pve1/qemu/101/agent/network-get-interfaces":
			w.Write([]byte(`{"data":{"result":[
				{"name":"lo","ip-addresses":[{"ip-address":"127.0.0.1","ip-address-type":"ipv4"}]},
				{"name":"eth0","ip-addresses":[{"ip-address":"10.0.0.10","ip-address-type":"ipv4"}]}]}}`))
		case "/api2/json/nodes/pve2/qemu/104/agent/network-get-interfaces":
			http.Error(w, `{"data":null,"message":"QEMU guest agent is not running"}`, http.StatusInternalServerError)
		case "/api2/json/nodes/pve1/lxc/102/interfaces":
			w.Write([]byte(`{"data":[{"name":"eth0","inet":"10.0.0.13/24"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "t", "s", false).Machines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Machine{
		{ProxmoxID: "node/pve1", Kind: "node", Name: "pve1", Node: "pve1", Running: true, CPU: "32 vCPU", NoIPReason: "node not in /cluster/status"},
		{ProxmoxID: "node/pve2", Kind: "node", Name: "pve2", Node: "pve2", NoIPReason: "node not in /cluster/status"},
		{ProxmoxID: "lxc/102", Kind: "lxc", Name: "cache", Node: "pve1", VMID: 102, IP: "10.0.0.13", Running: true},
		{ProxmoxID: "qemu/101", Kind: "qemu", Name: "web", Node: "pve1", VMID: 101, IP: "10.0.0.10", Running: true, CPU: "4 vCPU", RAM: "8 GB", Disk: "100 GB"},
		{ProxmoxID: "qemu/104", Kind: "qemu", Name: "noagent", Node: "pve2", VMID: 104, Running: true, NoIPReason: "500 Internal Server Error: QEMU guest agent is not running"},
		{ProxmoxID: "qemu/105", Kind: "qemu", Name: "off", Node: "pve1", VMID: 105, NoIPReason: "stopped"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Machines =\n%+v\nwant\n%+v", got, want)
	}
	for _, p := range []string{"/api2/json/nodes/pve1/qemu/105/agent/network-get-interfaces", "/api2/json/nodes/pve1/qemu/900/agent/network-get-interfaces"} {
		if hits[p] != 0 {
			t.Errorf("%s queried for a stopped/template guest", p)
		}
	}
}

func TestClientMachinesResourcesFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "permission check failed", http.StatusForbidden)
	}))
	defer srv.Close()
	if ms, err := NewClient(srv.URL, "t", "s", false).Machines(context.Background()); err == nil || ms != nil {
		t.Fatalf("Machines = %v, %v; want nil + error (so the sync never deactivates the fleet)", ms, err)
	}
}
