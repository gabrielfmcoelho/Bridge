package service_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestRecordDependencies_LinksContainerService(t *testing.T) {
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	h := &models.Host{Nickname: "vm-web", OficialSlug: "vm-web", Hostname: "10.0.0.7", Situacao: "active"}
	if err := store.NewHostRepo(d.SQL).Create(ctx, h); err != nil {
		t.Fatalf("host: %v", err)
	}
	web := sshtest.ContainerInfo{ID: "0123456789ab", Name: "web", Image: "nginx:1.25", Status: "Up 2 hours"}
	if err := store.NewServiceRepo(d.SQL).ReconcileDiscovered(ctx, h.ID, store.DiscoveredInventory{
		Containers: []sshtest.ContainerInfo{web}, ContainersKnown: true,
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	info := sshtest.VMInfo{
		ParsedContainers: []sshtest.ContainerInfo{web},
		ContainersKnown:  true,
		Dependencies: []sshtest.DepScope{
			{OS: sshtest.OSRelease{ID: "ubuntu", VersionID: "22.04"}, Runtimes: []sshtest.RuntimeVersion{{Name: "php", Version: "8.1.2"}}},
			{Container: "web", ContainerID: web.ID, Image: web.Image, OS: sshtest.OSRelease{ID: "debian", VersionID: "12"},
				ImageEnv: map[string]string{"NGINX_VERSION": "1.25.3"}},
		},
	}
	if err := service.RecordDependencies(ctx, d.SQL, h.ID, info); err != nil {
		t.Fatalf("record: %v", err)
	}
	rows, total, err := store.NewDependencyRepo(d.SQL).List(ctx, models.DependencyFilter{})
	if err != nil || total != 4 {
		t.Fatalf("rows = %+v (%d) %v; want ubuntu, php, debian, nginx", rows, total, err)
	}
	for _, r := range rows {
		if r.Container == "web" && r.ServiceID == nil {
			t.Fatalf("container row without its service: %+v", r)
		}
		if r.Container == "" && r.ServiceID != nil {
			t.Fatalf("host row with a service: %+v", r)
		}
		if r.Name == "nginx" && (r.Lookup != models.DepLookupNVD || r.Version != "1.25.3") {
			t.Fatalf("nginx = %+v", r)
		}
	}

	// The container stops being listed: its rows go, the host's stay.
	info.ParsedContainers, info.Dependencies = nil, info.Dependencies[:1]
	if err := service.RecordDependencies(ctx, d.SQL, h.ID, info); err != nil {
		t.Fatalf("record 2: %v", err)
	}
	if _, total, _ := store.NewDependencyRepo(d.SQL).List(ctx, models.DependencyFilter{}); total != 2 {
		t.Fatalf("after container removal total = %d; want 2", total)
	}
}
