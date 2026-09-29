package store_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Project-side links replace only among what the caller can see: an invisible
// host/DNS/service keeps its link to the project, and can't be added either.
func TestProjectRepo_LinksRespectScope(t *testing.T) {
	bg := context.Background()
	d := openDB(t)
	repo := store.NewProjectRepo(d.SQL)
	grants := store.NewAssetEntidadeRepo(d.SQL)
	sga := entidadeID(t, d, "sga")
	scoped := store.WithScope(bg, store.Scope{EntidadeIDs: []int64{sga}})

	q := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := d.SQL.QueryRow(sql, args...).Scan(&id); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return id
	}
	grant := func(a store.AssetType, id int64) {
		t.Helper()
		if err := grants.Replace(bg, d.SQL, a, id, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
			t.Fatalf("grant: %v", err)
		}
	}
	proj := q(`INSERT INTO projects (name) VALUES ('p') RETURNING id`)
	other := q(`INSERT INTO projects (name) VALUES ('other') RETURNING id`)
	visHost := q(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('v', 'v') RETURNING id`)
	visHost2 := q(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('v2', 'v2') RETURNING id`)
	hidHost := q(`INSERT INTO hosts (nickname, oficial_slug) VALUES ('h', 'h') RETURNING id`)
	visDNS := q(`INSERT INTO dns_records (domain) VALUES ('v.gov') RETURNING id`)
	visSvc := q(`INSERT INTO services (nickname) VALUES ('vs') RETURNING id`)
	movedSvc := q(`INSERT INTO services (nickname, project_id) VALUES ('ms', ?) RETURNING id`, other)
	hidSvc := q(`INSERT INTO services (nickname, project_id) VALUES ('hs', ?) RETURNING id`, proj)
	for _, id := range []int64{visHost, visHost2} {
		grant(store.AssetHost, id)
	}
	grant(store.AssetDNS, visDNS)
	grant(store.AssetService, visSvc)
	grant(store.AssetService, movedSvc)

	// Existing links: one visible host (to be dropped), one hidden host.
	for _, h := range []int64{visHost, hidHost} {
		if _, err := d.SQL.Exec(`INSERT INTO project_host_links (project_id, host_id) VALUES (?, ?)`, proj, h); err != nil {
			t.Fatalf("seed link: %v", err)
		}
	}

	// The scoped caller keeps only visHost2 (and tries to add the hidden one).
	if err := repo.SetDirectHosts(scoped, proj, []int64{visHost2, hidHost}); err != nil {
		t.Fatalf("SetDirectHosts: %v", err)
	}
	if err := repo.SetDirectDNS(scoped, proj, []int64{visDNS}); err != nil {
		t.Fatalf("SetDirectDNS: %v", err)
	}
	if err := repo.SetServices(scoped, proj, []int64{visSvc, movedSvc}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}

	ids := func(sql string) []int64 {
		t.Helper()
		rows, err := d.SQL.Query(sql, proj)
		if err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			out = append(out, id)
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	sorted := func(v ...int64) []int64 { sort.Slice(v, func(i, j int) bool { return v[i] < v[j] }); return v }

	if got := ids(`SELECT host_id FROM project_host_links WHERE project_id = ?`); !reflect.DeepEqual(got, sorted(visHost2, hidHost)) {
		t.Fatalf("hosts = %v, want visHost2 + the hidden one kept (%v)", got, sorted(visHost2, hidHost))
	}
	if got := ids(`SELECT dns_id FROM project_dns_links WHERE project_id = ?`); !reflect.DeepEqual(got, []int64{visDNS}) {
		t.Fatalf("dns = %v, want [%d]", got, visDNS)
	}
	// visSvc joined, movedSvc moved over from the other project, the hidden one stays.
	if got := ids(`SELECT id FROM services WHERE project_id = ?`); !reflect.DeepEqual(got, sorted(visSvc, movedSvc, hidSvc)) {
		t.Fatalf("services = %v, want %v", got, sorted(visSvc, movedSvc, hidSvc))
	}

	// Dropping visSvc from the list takes it out of the project.
	if err := repo.SetServices(scoped, proj, []int64{movedSvc}); err != nil {
		t.Fatalf("SetServices 2: %v", err)
	}
	if got := ids(`SELECT id FROM services WHERE project_id = ?`); !reflect.DeepEqual(got, sorted(movedSvc, hidSvc)) {
		t.Fatalf("after drop services = %v, want %v", got, sorted(movedSvc, hidSvc))
	}
}
