package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestSentinelRepo(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	var linked, plain, svcA, svcB int64
	mustScan := func(q string, dst *int64) {
		t.Helper()
		if err := d.SQL.QueryRow(q).Scan(dst); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustScan(`INSERT INTO hosts (nickname, oficial_slug, coolify_server_uuid) VALUES ('nodo-f', 'nodo-f', 'h7t2q26dp5r8ty3oazurbv41') RETURNING id`, &linked)
	mustScan(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('plain', 'plain') RETURNING id`, &plain)
	mustScan(`INSERT INTO services (nickname) VALUES ('airflow') RETURNING id`, &svcA)
	mustScan(`INSERT INTO services (nickname) VALUES ('flower') RETURNING id`, &svcB)
	for _, l := range [][2]int64{{svcA, plain}, {svcA, linked}, {svcB, linked}} {
		if _, err := d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, l[0], l[1]); err != nil {
			t.Fatal(err)
		}
	}

	repo := store.NewSentinelRepo(d.SQL)

	hosts, err := repo.CoolifyHosts(ctx)
	if err != nil || len(hosts) != 1 || hosts[0].ID != linked {
		t.Fatalf("coolify hosts = %+v, %v", hosts, err)
	}
	if h, err := repo.CoolifyHostForService(ctx, svcA); err != nil || h == nil || h.ID != linked {
		t.Fatalf("host for service (skips the unlinked host) = %+v, %v", h, err)
	}

	if got, _ := repo.GetHost(ctx, linked); got != nil {
		t.Fatal("no snapshot before the first sync")
	}
	cpu := 42.5
	if err := repo.UpsertHost(ctx, &models.SentinelHostSnapshot{HostID: linked, State: "ok", MetricsEnabled: true, CPUP95_24h: &cpu}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertHost(ctx, &models.SentinelHostSnapshot{HostID: linked, State: "metrics_off"}); err != nil {
		t.Fatal(err)
	}
	if h, _ := repo.GetHost(ctx, linked); h == nil || h.State != "metrics_off" || h.CPUP95_24h != nil {
		t.Fatalf("upsert must replace the whole row: %+v", h)
	}

	// The replaced restart count becomes prev_restart_count.
	for _, n := range []int{3, 5} {
		n := n
		if err := repo.UpsertService(ctx, &models.SentinelServiceSnapshot{ServiceID: svcA, HostID: linked, RestartCount: &n}); err != nil {
			t.Fatal(err)
		}
	}
	v, err := repo.GetService(ctx, svcA)
	if err != nil || v == nil || *v.RestartCount != 5 || v.PrevRestartCount == nil || *v.PrevRestartCount != 3 || v.Req24h != nil || v.ServiceName != "airflow" {
		t.Fatalf("service snapshot = %+v, %v", v, err)
	}

	if err := repo.UpsertService(ctx, &models.SentinelServiceSnapshot{ServiceID: svcB, HostID: linked}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PruneServices(ctx, linked, []int64{svcB}); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ServicesBulk(ctx, 0)
	if _, gone := all[svcA]; err != nil || gone || len(all) != 1 {
		t.Fatalf("prune kept %+v, %v", all, err)
	}
	if err := repo.PruneServices(ctx, linked, nil); err != nil {
		t.Fatal(err)
	}
	if all, _ := repo.ServicesBulk(ctx, linked); len(all) != 0 {
		t.Fatalf("prune(nil) must clear the host: %+v", all)
	}
}
