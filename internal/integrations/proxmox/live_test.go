package proxmox

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestLiveCluster runs the sync's read path against a real PVE and never
// writes anywhere. Opt-in, credentials from either:
//
//	PROXMOX_URL=https://pve:8006 PROXMOX_TOKEN_ID='user@pve!name' \
//	PROXMOX_TOKEN_SECRET=... PROXMOX_SKIP_VERIFY=1 \
//	go test ./internal/integrations/proxmox -run TestLiveCluster -v
//
// or a server stored in Bridge's DB (the .env `make dev` uses; the first by
// name unless PROXMOX_SERVER_ID picks one), which also reports how many
// machines are already linked to a host of that server:
//
//	set -a; . ./.env; set +a; PROXMOX_FROM_DB=1 \
//	go test ./internal/integrations/proxmox -run TestLiveCluster -v
func TestLiveCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := liveSettings{
		BaseURL: os.Getenv("PROXMOX_URL"), TokenID: os.Getenv("PROXMOX_TOKEN_ID"),
		TokenSecret: os.Getenv("PROXMOX_TOKEN_SECRET"), SkipVerify: os.Getenv("PROXMOX_SKIP_VERIFY") == "1",
	}
	var linked map[string]int64
	if os.Getenv("PROXMOX_FROM_DB") == "1" {
		s, linked = settingsFromDB(ctx, t)
	}
	if s.BaseURL == "" {
		t.Skip("PROXMOX_URL / PROXMOX_FROM_DB not set")
	}
	t.Logf("PVE %s token %s skip_verify=%v", s.BaseURL, s.TokenID, s.SkipVerify)
	c := NewClient(s.BaseURL, s.TokenID, s.TokenSecret, s.SkipVerify)

	v, err := c.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	t.Logf("Proxmox VE %s", v)

	ms, err := c.Machines(ctx)
	if err != nil {
		t.Fatalf("machines: %v", err)
	}
	kinds, noIP, isLinked := map[string]int{}, 0, 0
	for _, m := range ms {
		kinds[m.Kind]++
		if m.IP == "" {
			noIP++
		}
		host := ""
		if id := linked[m.ProxmoxID]; id != 0 {
			isLinked++
			host = "host#" + strconv.FormatInt(id, 10)
		}
		t.Logf("%-12s %-6s running=%-5v ip=%-15s %s  %s %s %s  %s %s", m.ProxmoxID, m.Kind, m.Running, m.IP, m.Name, m.CPU, m.RAM, m.Disk, host, m.NoIPReason)
	}
	t.Logf("%d machines %v, %d without IP", len(ms), kinds, noIP)
	if linked != nil {
		t.Logf("%d already linked to a host, %d hosts linked in DB", isLinked, len(linked))
	}
}

type liveSettings struct {
	BaseURL, TokenID, TokenSecret string
	SkipVerify                    bool
}

// settingsFromDB loads a stored Proxmox server (secret decrypted with
// SSHCM_SECRET_KEY) and its proxmox_id → host index, in a read-only session
// without running migrations.
func settingsFromDB(ctx context.Context, t *testing.T) (liveSettings, map[string]int64) {
	dsn := os.Getenv("SSHCM_DB_DSN")
	if dsn == "" || os.Getenv("SSHCM_SECRET_KEY") == "" {
		t.Fatal("PROXMOX_FROM_DB needs SSHCM_DB_DSN and SSHCM_SECRET_KEY (source the .env)")
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := database.OpenNoMigrate(dsn + sep + "default_transaction_read_only=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	enc, err := database.NewEncryptor(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	servers, err := store.NewProxmoxServerRepo(db).List(ctx, false)
	if err != nil {
		t.Fatalf("load servers: %v", err)
	}
	want, _ := strconv.ParseInt(os.Getenv("PROXMOX_SERVER_ID"), 10, 64)
	var srv *models.ProxmoxServer
	for i := range servers {
		if want == 0 || servers[i].ID == want {
			srv = &servers[i]
			break
		}
	}
	if srv == nil {
		t.Fatal("no Proxmox server stored (or PROXMOX_SERVER_ID not found)")
	}
	if !srv.Enabled {
		t.Logf("warning: server %q is disabled — the sync would skip it", srv.Name)
	}
	if store.NewAppSettingsRepo(db).Value(ctx, "proxmox_enabled") != "true" {
		t.Log("warning: proxmox_enabled is not true in the DB — the sync endpoint would refuse")
	}
	s := liveSettings{BaseURL: srv.BaseURL, TokenID: srv.TokenID, SkipVerify: srv.SkipVerify}
	if srv.HasToken {
		if s.TokenSecret, err = enc.Decrypt(srv.TokenCipher, srv.TokenNonce); err != nil {
			t.Fatalf("decrypt token: %v", err)
		}
	}
	byPID, _, _, err := store.NewHostRepo(db).ProxmoxIndex(store.WithSystemScope(ctx), srv.ID)
	if err != nil {
		t.Fatalf("host index: %v", err)
	}
	delete(byPID, "")
	return s, byPID
}
