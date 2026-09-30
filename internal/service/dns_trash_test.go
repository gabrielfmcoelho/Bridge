package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// A deleted DNS record leaves every live view — list, count, a host's
// records, a project's dns_ids — sits in the trash with its links and tags,
// comes back whole on restore, and Coolify sync neither recreates nor links it.
func TestDNSService_TrashAndRestore(t *testing.T) {
	ctx := context.Background()
	svc, d := newDNSService(t)
	var hostID, projectID int64
	d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, coolify_server_uuid) VALUES ('h','h','10.0.0.9','srv-h') RETURNING id`).Scan(&hostID)
	d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p') RETURNING id`).Scan(&projectID)
	w := &service.DNSWrite{Record: models.DNSRecord{Domain: "gone.x.gov.br", Situacao: "active"}}
	if err := svc.Create(ctx, w); err != nil {
		t.Fatal(err)
	}
	id := w.Record.ID
	dns := store.NewDNSRepo(d.SQL)
	dns.SetHostLinks(ctx, id, []int64{hostID})
	dns.SetProjectLinks(ctx, id, []int64{projectID})
	store.NewTagRepo(d.SQL).Set(ctx, "dns", id, []string{"keep"})

	if err := svc.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if rec, _ := dns.Get(ctx, id); rec != nil {
		t.Fatal("Get still returns the trashed record")
	}
	if n, _ := dns.Count(ctx); n != 0 {
		t.Fatalf("count = %d, want 0", n)
	}
	if recs, _ := dns.RecordsByHost(ctx, hostID); len(recs) != 0 {
		t.Fatalf("host still lists %d records", len(recs))
	}
	if ids, _ := store.NewProjectRepo(d.SQL).DirectDNSIDs(ctx, projectID); len(ids) != 0 {
		t.Fatalf("project still links %v", ids)
	}
	trash, _ := svc.ListTrash(ctx)
	if len(trash) != 1 || trash[0].ID != id || trash[0].DeletedAt == nil {
		t.Fatalf("trash = %+v", trash)
	}
	if in, _ := svc.DomainInTrash(ctx, "GONE.x.gov.br"); !in {
		t.Fatal("DomainInTrash = false")
	}

	// Coolify: the trashed domain is neither recreated nor linked.
	sum, err := svc.SyncFromCoolify(ctx, []coolify.DomainRef{{Domain: "gone.x.gov.br", ServerUUID: "srv-h", Source: "app"}})
	if err != nil || sum.Created != 0 || sum.LinksAdded != 0 {
		t.Fatalf("sync = %+v, %v", sum, err)
	}

	if ok, err := svc.Restore(ctx, id); err != nil || !ok {
		t.Fatalf("restore = %v, %v", ok, err)
	}
	if hosts, _ := dns.HostIDs(ctx, id); len(hosts) != 1 {
		t.Fatalf("host links after restore = %v", hosts)
	}
	if tags, _ := store.NewTagRepo(d.SQL).Get(ctx, "dns", id); len(tags) != 1 {
		t.Fatalf("tags after restore = %v", tags)
	}
	if ids, _ := store.NewProjectRepo(d.SQL).DirectDNSIDs(ctx, projectID); len(ids) != 1 {
		t.Fatalf("project link after restore = %v", ids)
	}
}
