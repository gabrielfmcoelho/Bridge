package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// recordScan files a command line's secret in the vault, as the user who ran
// the scan, before the scan is stored or answered: neither holds the value.
func TestRecordScanRedactsSecrets(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	users := store.NewUserRepo(d.SQL)
	admin := &models.User{Username: "admin", DisplayName: "Admin", Role: "admin", Email: "a@example.com"}
	editor := &models.User{Username: "eddie", DisplayName: "Eddie", Role: "editor", Email: "e@example.com"}
	for _, u := range []*models.User{admin, editor} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	host := &models.Host{Nickname: "compras01", OficialSlug: "compras01", Hostname: "10.0.0.1", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, host); err != nil {
		t.Fatal(err)
	}

	info := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{
		PID: "1", Command: "/opt/jdk/bin/java -DcacheServerPassword=abc123 -Djboss.home.dir=/opt/compras1 org.jboss.Main",
	}}}
	result := map[string]any{"vm_info": info}
	(&sshHandlers{db: d}).recordScan(auth.WithUser(ctx, editor), host, info, result)

	if e, ok := result["scan_secrets_error"]; ok {
		t.Fatalf("scan_secrets_error = %v", e)
	}
	answered, _ := json.Marshal(result)
	stored, err := store.NewHostScanRepo(d.SQL).GetLatest(ctx, host.ID)
	if err != nil || stored == nil {
		t.Fatalf("stored scan = %v, %v", stored, err)
	}
	for what, s := range map[string]string{"response": string(answered), "stored scan": stored.Data} {
		if strings.Contains(s, "abc123") || !strings.Contains(s, "-DcacheServerPassword={{secret:") {
			t.Errorf("%s = %s", what, s)
		}
	}
	var createdBy int64
	if err := d.SQL.QueryRow(`SELECT created_by FROM secrets WHERE group_label = 'scan-opt-compras1' AND name = 'CACHE_SERVER_PASSWORD'`).Scan(&createdBy); err != nil {
		t.Fatal(err)
	}
	if createdBy != editor.ID {
		t.Fatalf("created_by = %d, want the scanning user %d", createdBy, editor.ID)
	}
}
