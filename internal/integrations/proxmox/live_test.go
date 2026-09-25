package proxmox

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveCluster runs the sync's read path against a real PVE. Opt-in: it is
// skipped unless PROXMOX_URL is set, and it never writes anywhere.
//
//	PROXMOX_URL=https://pve:8006 PROXMOX_TOKEN_ID='user@pve!name' \
//	PROXMOX_TOKEN_SECRET=... PROXMOX_SKIP_VERIFY=1 \
//	go test ./internal/integrations/proxmox -run TestLiveCluster -v
func TestLiveCluster(t *testing.T) {
	url := os.Getenv("PROXMOX_URL")
	if url == "" {
		t.Skip("PROXMOX_URL not set")
	}
	c := NewClient(url, os.Getenv("PROXMOX_TOKEN_ID"), os.Getenv("PROXMOX_TOKEN_SECRET"), os.Getenv("PROXMOX_SKIP_VERIFY") == "1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	v, err := c.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	t.Logf("Proxmox VE %s", v)

	ms, err := c.Machines(ctx)
	if err != nil {
		t.Fatalf("machines: %v", err)
	}
	kinds, noIP := map[string]int{}, 0
	for _, m := range ms {
		kinds[m.Kind]++
		if m.IP == "" {
			noIP++
		}
		t.Logf("%-12s %-6s running=%-5v ip=%-15s %s  %s %s %s", m.ProxmoxID, m.Kind, m.Running, m.IP, m.Name, m.CPU, m.RAM, m.Disk)
	}
	t.Logf("%d machines %v, %d without IP", len(ms), kinds, noIP)
}
