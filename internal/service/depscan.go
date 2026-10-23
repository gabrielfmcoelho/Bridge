package service

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// RecordDependencies writes a scan's dependency scopes (host + containers)
// to dependency_inventory. Synchronous and offline: it runs right after
// ReconcileDiscovered so each container row can point at its service.
// Scopes that weren't probed keep their previous rows; containers docker ps
// no longer lists are dropped. Then ApplyCache updates the host's CVE
// counts and findings from the cache.
func RecordDependencies(ctx context.Context, db *sql.DB, hostID int64, info sshtest.VMInfo) error {
	ctx = store.WithSystemScope(ctx)
	svcs, err := store.NewServiceRepo(db).ListDiscoveredByHost(ctx, hostID)
	if err != nil {
		return err
	}
	svcByContainer := map[string]int64{}
	for _, s := range svcs {
		if s.ContainerName != "" {
			svcByContainer[s.ContainerName] = s.ID
		}
	}
	var rows []models.Dependency
	sc := store.HostScopes{RunningKnown: info.ContainersKnown}
	for _, c := range info.ParsedContainers {
		sc.Running = append(sc.Running, c.Name)
	}
	for _, scope := range info.Dependencies {
		var svc *int64
		if id, ok := svcByContainer[scope.Container]; ok && scope.Container != "" {
			svc = &id
		}
		rows = append(rows, NormalizeDependencies(scope, svc)...)
		if scope.Error == "" {
			sc.Probed = append(sc.Probed, scope.Container)
		}
	}
	if err := store.NewDependencyRepo(db).ReplaceForHost(ctx, hostID, store.DedupeDependencies(rows), sc); err != nil {
		return err
	}
	// Counts, EOL and findings from what is already cached — no network:
	// a version that changed resolves its findings right away.
	_, err = ApplyCache(ctx, db, []int64{hostID})
	return err
}
