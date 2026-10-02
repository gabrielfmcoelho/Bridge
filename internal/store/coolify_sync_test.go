package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

const coolifyUUID = "a1rwyyxk4rby396m4ooq4ubu"

func seedCoolifyHost(t *testing.T, d *database.DB, slug string) int64 {
	t.Helper()
	var id int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES (?, ?, ?) RETURNING id`, slug, slug, "10.0.0."+slug).Scan(&id); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	return id
}

func liveServices(t *testing.T, d *database.DB) map[int64][3]string {
	t.Helper()
	rows, err := d.SQL.Query(`SELECT id, discovery_key, container_status, source FROM services WHERE deleted_at IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int64][3]string{}
	for rows.Next() {
		var id int64
		var k, st, src string
		if err := rows.Scan(&id, &k, &st, &src); err != nil {
			t.Fatal(err)
		}
		out[id] = [3]string{k, st, src}
	}
	return out
}

func scan(t *testing.T, repo *store.ServiceRepo, hostID int64, name, id string) {
	t.Helper()
	inv := store.DiscoveredInventory{ContainersKnown: true, Containers: []sshtest.ContainerInfo{{ID: id, Name: name, Image: "app:latest"}}}
	if err := repo.ReconcileDiscovered(context.Background(), hostID, inv); err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
}

// A Coolify redeploy renames the container (new deploy timestamp) and gives it
// a new engine id: the same row must follow it.
func TestReconcile_CoolifyRedeployKeepsTheRow(t *testing.T) {
	repo, d := newServiceRepo(t)
	host := seedCoolifyHost(t, d, "1")
	scan(t, repo, host, coolifyUUID+"-111111111111", "c1")
	scan(t, repo, host, coolifyUUID+"-222222222222", "c2")
	svcs := liveServices(t, d)
	if len(svcs) != 1 {
		t.Fatalf("after redeploy = %v, want one row", svcs)
	}
	for _, s := range svcs {
		if s[0] != coolifyUUID || s[1] != "online" {
			t.Errorf("row = %v, want key %s online", s, coolifyUUID)
		}
	}
}

// Copies left by redeploys before the rule (timestamped keys, offline) fold
// into the live row with their links; the copies go to the trash.
func TestMergeContainerDuplicates(t *testing.T) {
	ctx := context.Background()
	repo, d := newServiceRepo(t)
	host := seedCoolifyHost(t, d, "1")
	ins := func(key, status string) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO services (nickname, source, discovery_kind, discovery_key, container_name, container_status)
			VALUES (?, 'auto', 'container', ?, ?, ?) RETURNING id`, key, key, key, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, id, host); err != nil {
			t.Fatal(err)
		}
		return id
	}
	old := ins("api-"+coolifyUUID+"-111111111111", "offline")
	live := ins("api-"+coolifyUUID+"-222222222222", "online")
	other := ins("worker-"+coolifyUUID+"-111111111111", "offline") // different role: not a copy
	var projectID, dnsID int64
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('P') RETURNING id`).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('api.x') RETURNING id`).Scan(&dnsID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`UPDATE services SET project_id = ? WHERE id = ?`, projectID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO service_dns_links (service_id, dns_id) VALUES (?, ?)`, old, dnsID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`INSERT INTO service_dependencies (service_id, depends_on_id) VALUES (?, ?)`, other, old); err != nil {
		t.Fatal(err)
	}

	n, err := repo.MergeContainerDuplicates(ctx)
	if err != nil || n != 1 {
		t.Fatalf("merge = %d, %v; want 1", n, err)
	}
	svcs := liveServices(t, d)
	if _, ok := svcs[old]; ok {
		t.Errorf("copy %d still live", old)
	}
	if svcs[live][0] != "api-"+coolifyUUID || svcs[other][0] != "worker-"+coolifyUUID {
		t.Errorf("keys not normalized: %v", svcs)
	}
	var gotProject, gotDNS, gotDep int64
	d.SQL.QueryRow(`SELECT COALESCE(project_id, 0) FROM services WHERE id = ?`, live).Scan(&gotProject)
	d.SQL.QueryRow(`SELECT COUNT(*) FROM service_dns_links WHERE service_id = ? AND dns_id = ?`, live, dnsID).Scan(&gotDNS)
	d.SQL.QueryRow(`SELECT COUNT(*) FROM service_dependencies WHERE service_id = ? AND depends_on_id = ?`, other, live).Scan(&gotDep)
	if gotProject != projectID || gotDNS != 1 || gotDep != 1 {
		t.Errorf("links not moved: project=%d dns=%d dep=%d", gotProject, gotDNS, gotDep)
	}
	if n, _ := repo.MergeContainerDuplicates(ctx); n != 0 {
		t.Errorf("second merge = %d, want 0 (idempotent)", n)
	}
}

// A placeholder the Coolify sync created is adopted by the scan that finally
// sees its container — no twin.
func TestReconcile_AdoptsCoolifyPlaceholder(t *testing.T) {
	repo, d := newServiceRepo(t)
	host := seedCoolifyHost(t, d, "1")
	id, err := repo.CreateCoolifyPlaceholder(context.Background(), host, coolifyUUID, "Bridge - API",
		store.CoolifyFields{ResourceUUID: coolifyUUID, ResourceType: "application"})
	if err != nil {
		t.Fatal(err)
	}
	if s := liveServices(t, d)[id]; s[1] != "offline" || s[2] != "coolify" {
		t.Fatalf("placeholder = %v", s)
	}
	scan(t, repo, host, coolifyUUID+"-333333333333", "c9")
	svcs := liveServices(t, d)
	if len(svcs) != 1 || svcs[id][1] != "online" || svcs[id][2] != "auto" {
		t.Fatalf("after scan = %v, want the placeholder online as auto", svcs)
	}
}
