package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// One component version installed on several hosts is one group: worst
// first, counted per host and installation, filters as the flat list.
func TestDependencyRepo_ListGroups(t *testing.T) {
	d := openDB(t)
	ctx := store.WithSystemScope(context.Background())
	hosts := map[string]int64{}
	for _, n := range []string{"vm-a", "vm-b"} {
		h := &models.Host{Nickname: n, OficialSlug: n, Hostname: "10.0.0." + n[3:], Situacao: "active"}
		if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
			t.Fatal(err)
		}
		hosts[n] = h.ID
	}
	ins := func(host, container, kind, name, version string, cves int, sev string, eol bool) {
		t.Helper()
		if _, err := d.SQL.Exec(`INSERT INTO dependency_inventory (host_id, container, kind, name, version, version_source, cve_count, max_severity, eol)
			VALUES (?, ?, ?, ?, ?, 'package', ?, ?, ?)`, hosts[host], container, kind, name, version, cves, sev, eol); err != nil {
			t.Fatal(err)
		}
	}
	ins("vm-a", "", "package", "libssl3", "3.0.11", 2, "medium", false)
	ins("vm-b", "web", "package", "libssl3", "3.0.11", 3, "high", false) // same version, worse answer in another distro
	ins("vm-b", "", "package", "libssl3", "3.0.11", 3, "high", false)    // and on vm-b's host too
	ins("vm-b", "", "package", "libssl3", "3.0.13", 0, "", false)
	ins("vm-a", "", "package", "zlib1g", "1.2.13", 1, "critical", false)
	ins("vm-a", "", "runtime", "php", "7.4.33", 0, "", true)

	repo := store.NewDependencyRepo(d.SQL)
	groups, total, err := repo.ListGroups(ctx, models.DependencyFilter{Kinds: []string{"package"}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(groups) != 3 {
		t.Fatalf("groups = %d (total %d): %+v", len(groups), total, groups)
	}
	if g := groups[0]; g.Name != "zlib1g" || g.MaxSeverity != "critical" {
		t.Errorf("first = %+v; want the critical one", g)
	}
	g := groups[1]
	if g.Name != "libssl3" || g.Version != "3.0.11" || g.Hosts != 2 || g.Installs != 3 || g.CVECount != 3 || g.MaxSeverity != "high" {
		t.Fatalf("libssl3 3.0.11 = %+v", g)
	}
	if len(g.Instances) != 3 || g.Instances[0].HostNickname != "vm-a" || g.Instances[2].Container != "web" || g.Instances[2].HostSlug != "vm-b" {
		t.Fatalf("instances = %+v", g.Instances)
	}
	if groups[2].Version != "3.0.13" || groups[2].MaxSeverity != "" {
		t.Errorf("last = %+v", groups[2])
	}

	// One host: its host + container installs of a version are one group.
	groups, _, _ = repo.ListGroups(ctx, models.DependencyFilter{Kinds: []string{"package"}, HostID: hosts["vm-b"]})
	if len(groups) != 2 || groups[0].Installs != 2 || groups[0].Hosts != 1 {
		t.Fatalf("vm-b groups = %+v", groups)
	}
	// Flags and search narrow the installations, not just the groups.
	groups, _, _ = repo.ListGroups(ctx, models.DependencyFilter{Kinds: []string{"package"}, HasCVEs: true, Search: "web"})
	if len(groups) != 1 || groups[0].Installs != 1 || groups[0].Instances[0].Container != "web" {
		t.Fatalf("search web = %+v", groups)
	}
	groups, _, _ = repo.ListGroups(ctx, models.DependencyFilter{Kinds: []string{"runtime"}, EOL: true})
	if len(groups) != 1 || !groups[0].EOL || groups[0].Name != "php" {
		t.Fatalf("eol = %+v", groups)
	}
	// Paging counts groups.
	groups, total, _ = repo.ListGroups(ctx, models.DependencyFilter{Kinds: []string{"package"}, Page: 2, PerPage: 2})
	if total != 3 || len(groups) != 1 {
		t.Fatalf("page 2 = %d of %d", len(groups), total)
	}
}
