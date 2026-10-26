package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestGitLabRepoLinkRepo(t *testing.T) {
	ctx := context.Background()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	var dnsID, hostID int64
	if err := d.SQL.QueryRow(`INSERT INTO dns_records (domain) VALUES ('sgi.example') RETURNING id`).Scan(&dnsID); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('vm1', 'vm1') RETURNING id`).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	repo := store.NewGitLabRepoLinkRepo(d.SQL)
	mk := func(typ string, id int64) error {
		return repo.Create(ctx, &models.GitLabRepoLink{EntityType: typ, EntityID: id, GitLabProjectID: 7,
			GitLabBaseURL: "https://gl.example", GitLabPath: "g/sgi"}, nil)
	}
	if err := mk("dns", dnsID); err != nil {
		t.Fatalf("create dns: %v", err)
	}
	if err := mk("host", hostID); err != nil {
		t.Fatalf("create host: %v", err)
	}
	if err := mk("dns", dnsID); err == nil {
		t.Fatal("duplicate link accepted")
	}
	if err := mk("service", 1); err == nil {
		t.Fatal("entity_type outside dns/host accepted")
	}

	all, err := repo.List(ctx)
	if err != nil || len(all) != 2 || all[0].EntityName != "sgi.example" || all[1].EntityName != "vm1" {
		t.Fatalf("List = %+v, %v", all, err)
	}
	if ls, err := repo.ListFor(ctx, "dns", []int64{dnsID}); err != nil || len(ls) != 1 || ls[0].EntityType != "dns" {
		t.Fatalf("ListFor(dns) = %+v, %v", ls, err)
	}

	// A trashed DNS record hides its link; restoring it brings the link back.
	if _, err := d.SQL.Exec(`UPDATE dns_records SET deleted_at = NOW() WHERE id = ?`, dnsID); err != nil {
		t.Fatal(err)
	}
	if all, _ := repo.List(ctx); len(all) != 1 || all[0].EntityType != "host" {
		t.Fatalf("after trash, List = %+v; want only the host link", all)
	}
	if _, err := d.SQL.Exec(`UPDATE dns_records SET deleted_at = NULL WHERE id = ?`, dnsID); err != nil {
		t.Fatal(err)
	}
	if all, _ := repo.List(ctx); len(all) != 2 {
		t.Fatalf("after restore, List has %d links; want 2", len(all))
	}

	if ok, err := repo.Delete(ctx, all[0].ID); err != nil || !ok {
		t.Fatalf("Delete = %v, %v", ok, err)
	}
	if ok, _ := repo.Delete(ctx, all[0].ID); ok {
		t.Fatal("second Delete reported a row")
	}
}
