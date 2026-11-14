package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestServiceRepo_ReconcileSites exercises the C.3 site-service reconcile:
// create, the fill-when-empty framework enrichment, operator edits sticking,
// skip-existing (no duplicate), the offline sweep, and the never-recreate-a-
// trashed-site guarantee.
func TestServiceRepo_ReconcileSites(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	var hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h1','h1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatalf("seed host: %v", err)
	}

	keyA := store.SiteDiscoveryKey("app.sead.pi.gov.br", "/var/www/app")
	// site row (nil when absent) for keyA.
	type row struct {
		id       int64
		nickname string
		stack    string
		version  string
		status   string
	}
	siteRow := func(t *testing.T) *row {
		t.Helper()
		var r row
		err := d.SQL.QueryRow(
			`SELECT s.id, s.nickname, s.technology_stack, s.version, s.container_status
			 FROM services s JOIN service_host_links l ON s.id = l.service_id
			 WHERE l.host_id = $1 AND s.deleted_at IS NULL AND s.discovery_kind = 'site' AND s.discovery_key = $2`,
			hostID, keyA).Scan(&r.id, &r.nickname, &r.stack, &r.version, &r.status)
		if err != nil {
			return nil
		}
		return &r
	}

	// 1. Sync-mode create: the vhost is known but the framework isn't yet.
	if err := repo.ReconcileSites(ctx, hostID, []store.DiscoveredSite{
		{Name: "app.sead.pi.gov.br", Docroot: "/var/www/app"},
	}); err != nil {
		t.Fatalf("reconcile create: %v", err)
	}
	r := siteRow(t)
	if r == nil || r.nickname != "app.sead.pi.gov.br" || r.stack != "" || r.status != "online" {
		t.Fatalf("after create = %+v", r)
	}
	created := r.id

	// 2. Job-mode enrich: the code fingerprint fills the empty framework/version.
	if err := repo.ReconcileSites(ctx, hostID, []store.DiscoveredSite{
		{Name: "app.sead.pi.gov.br", Docroot: "/var/www/app", Framework: "laravel", Version: "10.3"},
	}); err != nil {
		t.Fatalf("reconcile enrich: %v", err)
	}
	r = siteRow(t)
	if r == nil || r.id != created || r.stack != "laravel" || r.version != "10.3" {
		t.Fatalf("after enrich = %+v (want same row, laravel/10.3)", r)
	}

	// 3. Operator edits nickname + stack; a later scan with a different framework
	// must NOT clobber them (fill-when-empty), only refresh last_seen/status.
	if _, err := d.SQL.Exec(
		`UPDATE services SET nickname = 'Portal de Concursos', technology_stack = 'laravel-custom', container_status = 'offline' WHERE id = $1`,
		created); err != nil {
		t.Fatalf("operator edit: %v", err)
	}
	if err := repo.ReconcileSites(ctx, hostID, []store.DiscoveredSite{
		{Name: "app.sead.pi.gov.br", Docroot: "/var/www/app", Framework: "laravel", Version: "11.0"},
	}); err != nil {
		t.Fatalf("reconcile post-edit: %v", err)
	}
	r = siteRow(t)
	if r == nil || r.nickname != "Portal de Concursos" || r.stack != "laravel-custom" || r.version != "10.3" || r.status != "online" {
		t.Fatalf("operator edits must stick = %+v", r)
	}

	// 4. Skip-existing: reconciling the same site again makes no duplicate row.
	if err := repo.ReconcileSites(ctx, hostID, []store.DiscoveredSite{
		{Name: "app.sead.pi.gov.br", Docroot: "/var/www/app"},
	}); err != nil {
		t.Fatalf("reconcile idempotent: %v", err)
	}
	var liveCount int
	if err := d.SQL.QueryRow(
		`SELECT COUNT(*) FROM services WHERE discovery_kind = 'site' AND deleted_at IS NULL AND discovery_key = $1`,
		keyA).Scan(&liveCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if liveCount != 1 {
		t.Fatalf("live site rows = %d, want 1 (no duplicate)", liveCount)
	}

	// 5. Offline sweep: the scan no longer serves the site.
	if err := repo.ReconcileSites(ctx, hostID, nil); err != nil {
		t.Fatalf("reconcile offline: %v", err)
	}
	if r = siteRow(t); r == nil || r.status != "offline" {
		t.Fatalf("after offline sweep = %+v, want offline", r)
	}

	// 6. Trashed: the operator deletes the site service; the scan must NEVER
	// recreate it even though it still serves that docroot.
	if _, err := d.SQL.Exec(`UPDATE services SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1`, created); err != nil {
		t.Fatalf("trash: %v", err)
	}
	if err := repo.ReconcileSites(ctx, hostID, []store.DiscoveredSite{
		{Name: "app.sead.pi.gov.br", Docroot: "/var/www/app", Framework: "laravel", Version: "10.3"},
	}); err != nil {
		t.Fatalf("reconcile after trash: %v", err)
	}
	if r = siteRow(t); r != nil {
		t.Fatalf("trashed site was recreated = %+v", r)
	}
	var total int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM services WHERE discovery_key = $1`, keyA).Scan(&total); err != nil {
		t.Fatalf("total count: %v", err)
	}
	if total != 1 {
		t.Fatalf("site rows for key (incl. trashed) = %d, want 1 (the trashed one, not recreated)", total)
	}
}
