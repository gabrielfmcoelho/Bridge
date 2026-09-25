package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
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
		{ProxmoxID: "lxc/102", Kind: "lxc", Name: "lxc-102", Node: "pve1", VMID: 102, RAM: "512 MB"},
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
