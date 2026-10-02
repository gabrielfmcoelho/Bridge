package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestCoolifyInventorySync(t *testing.T) {
	ctx := context.Background()
	dnsSvc, d := newDNSService(t)
	const (
		appUUID   = "a1rwyyxk4rby396m4ooq4ubu"
		stackUUID = "kefmb9um8tyct5itlwaml1tp"
		ghostUUID = "zwkg80s48g4gogko44o8gg4g"
	)
	var host, manualProject, mappedProject, envProject int64
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES ('h','h','10.0.0.91') RETURNING id`).Scan(&host); err != nil {
		t.Fatal(err)
	}
	for name, dst := range map[string]*int64{"Manual": &manualProject, "Mapped": &mappedProject, "Env": &envProject} {
		if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES (?) RETURNING id`, name).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	projects := store.NewProjectRepo(d.SQL)
	// Project-wide mapping for "Gestor", overridden for its "homologa" env.
	if err := projects.SetCoolifyLinks(ctx, mappedProject, []models.ProjectCoolifyLink{{CoolifyProject: "Gestor"}}); err != nil {
		t.Fatal(err)
	}
	if err := projects.SetCoolifyLinks(ctx, envProject, []models.ProjectCoolifyLink{{CoolifyProject: "Gestor", CoolifyEnvironment: "homologa"}}); err != nil {
		t.Fatal(err)
	}

	// The scan found the app container and one stack member; the member already
	// has a project chosen by hand.
	repo := store.NewServiceRepo(d.SQL)
	if err := repo.ReconcileDiscovered(ctx, host, store.DiscoveredInventory{ContainersKnown: true, Containers: []sshtest.ContainerInfo{
		{ID: "c1", Name: appUUID + "-111111111111", Image: "api:latest"},
		{ID: "c2", Name: "apisix-" + stackUUID, Image: "apache/apisix"},
	}}); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	rows, _ := d.SQL.Query(`SELECT id, discovery_key FROM services`)
	for rows.Next() {
		var id int64
		var k string
		rows.Scan(&id, &k)
		ids[k] = id
	}
	rows.Close()
	if _, err := d.SQL.Exec(`UPDATE services SET project_id = ? WHERE id = ?`, manualProject, ids["apisix-"+stackUUID]); err != nil {
		t.Fatal(err)
	}
	// An API whose URL is the app's domain, not linked to anything yet.
	var apiID, userID int64
	if err := d.SQL.QueryRow(`SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('t', 'x', 'admin') RETURNING id`).Scan(&userID); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.SQL.QueryRow(`INSERT INTO api_catalog (scope, name, source_type, spec_json, owner_user_id, created_by, base_url)
		VALUES ('avulso', 'Bridge', 'url', '{}', ?, ?, 'https://bridge.x/api') RETURNING id`, userID, userID).Scan(&apiID); err != nil {
		t.Fatal(err)
	}

	srv := coolify.ServerRef{UUID: "srv-91", IP: "10.0.0.91"}
	resources := []coolify.Resource{
		{UUID: appUUID, Type: "application", Name: "Bridge - API", Project: "Gestor", Environment: "homologa",
			GitRepository: "https://h/r.git", GitBranch: "main", Server: srv, FQDNs: []string{"https://bridge.x"}},
		{UUID: stackUUID, Type: "service", Name: "gateway", Project: "Gestor", Environment: "production", Server: srv,
			Members: []string{"apisix-" + stackUUID}},
		{UUID: ghostUUID, Type: "database", Name: "Old DB", Project: "Gestor", Environment: "production", Server: srv},
		{UUID: "nohostnohostnohostnohos1", Type: "application", Name: "Elsewhere", Server: coolify.ServerRef{UUID: "x", IP: "1.1.1.1"}},
	}
	inv := service.NewCoolifyInventoryService(d.SQL)
	sum, err := inv.Sync(ctx, resources)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Matched != 2 || sum.Created != 1 || sum.HostsUUIDFilled != 1 || len(sum.Unmatched) != 1 || sum.Unmatched[0] != "Elsewhere" {
		t.Fatalf("summary = %+v", sum)
	}
	app, _ := repo.Get(ctx, ids[appUUID])
	if app.CoolifyProject != "Gestor" || app.CoolifyEnvironment != "homologa" || app.CoolifyStack != "Bridge - API" || app.GitBranch != "main" {
		t.Errorf("app fields = %+v", app)
	}
	if app.ProjectID == nil || *app.ProjectID != envProject {
		t.Errorf("app project = %v, want the exact-environment mapping %d", app.ProjectID, envProject)
	}
	member, _ := repo.Get(ctx, ids["apisix-"+stackUUID])
	if member.ProjectID == nil || *member.ProjectID != manualProject || member.CoolifyStack != "gateway" {
		t.Errorf("member = project %v stack %q; manual project must win", member.ProjectID, member.CoolifyStack)
	}
	var ghostProject int64
	if err := d.SQL.QueryRow(`SELECT COALESCE(project_id, 0) FROM services WHERE discovery_key = ? AND source = 'coolify' AND container_status = 'offline'`, ghostUUID).Scan(&ghostProject); err != nil {
		t.Fatalf("placeholder for the unseen database: %v", err)
	}
	if ghostProject != mappedProject {
		t.Errorf("placeholder project = %d, want the project-wide mapping %d", ghostProject, mappedProject)
	}

	// DNS sync after the inventory: the domain links to the app's service.
	dsum, err := dnsSvc.SyncFromCoolify(ctx, []coolify.DomainRef{{Domain: "bridge.x", HTTPS: true, ServerUUID: "srv-91", ResourceUUID: appUUID, Source: "Bridge - API"}})
	if err != nil {
		t.Fatal(err)
	}
	if dsum.ServiceLinksAdded != 1 {
		t.Fatalf("dns summary = %+v, want one service link", dsum)
	}
	// Second inventory sync: idempotent, and now the API links through the DNS.
	sum2, err := inv.Sync(ctx, resources)
	if err != nil {
		t.Fatal(err)
	}
	if sum2.Created != 0 || sum2.Updated != 0 || sum2.ProjectsSet != 0 || sum2.APIsLinked != 1 {
		t.Fatalf("second sync = %+v, want only the API linked", sum2)
	}
	var apiSvc, apiProj int64
	d.SQL.QueryRow(`SELECT COUNT(*) FROM api_service_links WHERE api_id = ? AND service_id = ?`, apiID, ids[appUUID]).Scan(&apiSvc)
	d.SQL.QueryRow(`SELECT COUNT(*) FROM api_project_links WHERE api_id = ? AND project_id = ?`, apiID, envProject).Scan(&apiProj)
	if apiSvc != 1 || apiProj != 1 {
		t.Errorf("api links: service=%d project=%d", apiSvc, apiProj)
	}
}
