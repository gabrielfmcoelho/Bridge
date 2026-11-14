package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// scanDBFixture sets up a host with a discovered MariaDB instance service and a
// discovered Apache web service serving /var/www/cvi, plus the VMInfo the A5
// probe + Phase 1 credential_sources would produce: two schemas (one personal,
// one plain) and an app config under the docroot naming the personal one.
func scanDBFixture(t *testing.T) (*database.DB, *models.Host, sshtest.VMInfo, map[string]int64) {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	sys := store.WithSystemScope(ctx)

	h := &models.Host{Nickname: "vm-db", OficialSlug: "vm-db", Hostname: "10.0.0.9", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	var entID int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades ORDER BY id LIMIT 1`).Scan(&entID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO asset_entidades (asset_type, asset_id, entidade_id, relation) VALUES ('host', ?, ?, 'creator') ON CONFLICT DO NOTHING`, h.ID, entID); err != nil {
		t.Fatal(err)
	}
	if err := store.NewServiceRepo(d.SQL).ReconcileDiscovered(ctx, h.ID, store.DiscoveredInventory{
		ContainersKnown: true,
		Services: []sshtest.DiscoveredService{
			{Name: "mariadb", Label: "MariaDB", Kind: "database", HostRunning: true},
			{Name: "apache", Label: "Apache", Kind: "web", HostRunning: true},
		},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	svcID := map[string]int64{}
	for _, s := range mustListDiscovered(t, d, sys, h.ID) {
		if s.DiscoveryKind == "host" {
			svcID[s.DiscoveryKey] = s.ID
		}
	}
	info := sshtest.VMInfo{
		DBMeta: &sshtest.DBMeta{
			Engine: "mariadb",
			Schemas: []sshtest.DBSchema{
				{Name: "concurso", SizeMB: 800, Tables: 40, PII: true},
				{Name: "logs", SizeMB: 5, Tables: 3},
			},
		},
		WebServers: []sshtest.WebServer{
			{Kind: "apache", Sites: []sshtest.WebSite{{Names: []string{"cvi.x.gov.br"}, Root: "/var/www/cvi", Listen: []int{80}}}},
		},
		CredentialSources: []sshtest.CredentialSource{
			{Detector: "ci3_database", Path: "/var/www/cvi/application/config/database.php", Locator: "default", Kind: "cred",
				Fields: map[string]string{"engine": "mysql", "host": "localhost", "db": "concurso", "user": "cvi_app"}, Token: "{{secret:7}}"},
		},
	}
	return d, h, info, svcID
}

func mustListDiscovered(t *testing.T, d *database.DB, ctx context.Context, hostID int64) []models.Service {
	t.Helper()
	svcs, err := store.NewServiceRepo(d.SQL).ListDiscoveredByHost(ctx, hostID)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	return svcs
}

func TestRecordScanDatabases(t *testing.T) {
	d, h, info, svcID := scanDBFixture(t)
	ctx := context.Background()

	sum, err := service.RecordScanDatabases(ctx, d.SQL, h, info)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if sum.Created != 2 || sum.Consumers != 1 || sum.Skipped != 0 {
		t.Fatalf("summary = %+v, want created 2, consumers 1, skipped 0", sum)
	}

	// The personal schema: discovered, ssh_tunnel to the host, the mariadb
	// instance, sensitivity pessoal, apache as consumer, host grants copied.
	var (
		name, engine, mode, sensitivity string
		discovered                      bool
		tunnel, instance                *int64
		dbID                            int64
	)
	row := d.SQL.QueryRow(`SELECT id, name, engine, connection_mode, sensitivity, discovered, tunnel_host_id, instance_service_id
		FROM databases WHERE db_name = 'concurso' AND deleted_at IS NULL`)
	if err := row.Scan(&dbID, &name, &engine, &mode, &sensitivity, &discovered, &tunnel, &instance); err != nil {
		t.Fatalf("scan concurso row: %v", err)
	}
	if name != "concurso @ vm-db" || engine != "mariadb" || mode != "ssh_tunnel" || sensitivity != "pessoal" || !discovered {
		t.Errorf("concurso = name %q engine %q mode %q sens %q discovered %v", name, engine, mode, sensitivity, discovered)
	}
	if tunnel == nil || *tunnel != h.ID {
		t.Errorf("tunnel_host_id = %v, want %d", tunnel, h.ID)
	}
	if instance == nil || *instance != svcID["mariadb"] {
		t.Errorf("instance_service_id = %v, want %d", instance, svcID["mariadb"])
	}
	var consumer int64
	if err := d.SQL.QueryRow(`SELECT service_id FROM database_consumer_links WHERE database_id = ?`, dbID).Scan(&consumer); err != nil {
		t.Fatalf("consumer link: %v", err)
	}
	if consumer != svcID["apache"] {
		t.Errorf("consumer = %d, want apache %d", consumer, svcID["apache"])
	}
	var grants int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM asset_entidades WHERE asset_type = 'database' AND asset_id = ?`, dbID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if grants == 0 {
		t.Error("host grants were not copied to the discovered database")
	}

	// The plain schema has no personal-data hint and no consumer.
	var plainSens string
	if err := d.SQL.QueryRow(`SELECT sensitivity FROM databases WHERE db_name = 'logs'`).Scan(&plainSens); err != nil {
		t.Fatalf("logs row: %v", err)
	}
	if plainSens != "interno" {
		t.Errorf("logs sensitivity = %q, want interno (default)", plainSens)
	}

	// Rerun: both schemas already exist → skip, nothing recreated.
	sum2, err := service.RecordScanDatabases(ctx, d.SQL, h, info)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if sum2.Created != 0 || sum2.Skipped != 2 {
		t.Fatalf("rerun summary = %+v, want created 0, skipped 2", sum2)
	}

	// Trash the discovered row; a scan must not recreate it.
	if _, err := d.SQL.Exec(`UPDATE databases SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, dbID); err != nil {
		t.Fatal(err)
	}
	sum3, err := service.RecordScanDatabases(ctx, d.SQL, h, info)
	if err != nil {
		t.Fatalf("after trash: %v", err)
	}
	if sum3.Created != 0 {
		t.Fatalf("after trash created = %d, a trashed row must never be recreated", sum3.Created)
	}
	var live int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM databases WHERE db_name = 'concurso' AND deleted_at IS NULL`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Errorf("concurso live rows = %d, want 0 (stays trashed)", live)
	}
}

func TestRecordScanDatabasesNoProbe(t *testing.T) {
	d, h, info, _ := scanDBFixture(t)
	info.DBMeta = nil // probe didn't run
	sum, err := service.RecordScanDatabases(context.Background(), d.SQL, h, info)
	if err != nil || sum.Created != 0 {
		t.Fatalf("nil DBMeta: sum=%+v err=%v, want a no-op", sum, err)
	}
}

func TestRecordScanDatabasesNoInstance(t *testing.T) {
	d, h, info, _ := scanDBFixture(t)
	// A schema whose engine has no discovered instance service on the host.
	info.DBMeta = &sshtest.DBMeta{Engine: "postgres", Schemas: []sshtest.DBSchema{{Name: "pg"}}}
	sum, err := service.RecordScanDatabases(context.Background(), d.SQL, h, info)
	if err != nil || sum.Created != 0 {
		t.Fatalf("no matching instance: sum=%+v err=%v, want a no-op", sum, err)
	}
}
