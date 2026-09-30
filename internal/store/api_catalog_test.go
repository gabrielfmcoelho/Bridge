package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// newCatalogDB opens a fresh migrated DB and returns it plus a user id to
// satisfy the owner_user_id / created_by foreign keys.
func newCatalogDB(t *testing.T) (*database.DB, int64) {
	t.Helper()
	d := openDB(t)
	u := &models.User{Username: "owner", DisplayName: "Owner", Role: "editor", Email: "o@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return d, u.ID
}

func sampleCatalog(owner int64) (*models.APICatalog, []models.APIOperation) {
	a := &models.APICatalog{
		Name:         "Petstore",
		Description:  "pets and more",
		SourceType:   models.APICatalogSourceUpload,
		SpecVersion:  "openapi-3.0.1",
		SpecJSON:     `{"openapi":"3.0.1","info":{"title":"Petstore"}}`,
		SpecHash:     "deadbeef",
		Title:        "Petstore",
		VersionLabel: "1.0.0",
		OwnerUserID:  owner,
		CreatedBy:    owner,
	}
	ops := []models.APIOperation{
		{Method: "GET", Path: "/pets", OperationID: "listPets", Summary: "List", Tags: []string{"pets"}, OpKey: "listPets"},
		{Method: "POST", Path: "/pets", Summary: "Create", Description: "Registers a brandnewzebra in the herd", Tags: []string{"pets"}, OpKey: "POST /pets"},
	}
	return a, ops
}

func TestAPICatalogRepo_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	d, owner := newCatalogDB(t)
	repo := store.NewAPICatalogRepo(d.SQL)

	a, ops := sampleCatalog(owner)
	if err := repo.Create(ctx, a, ops); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == 0 {
		t.Fatal("expected assigned id")
	}

	got, err := repo.Get(ctx, a.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v (nil=%v)", err, got == nil)
	}
	if got.OperationCount != 2 || len(got.Operations) != 2 {
		t.Errorf("operations: count=%d len=%d", got.OperationCount, len(got.Operations))
	}
	if got.Operations[0].OpKey != "listPets" {
		t.Errorf("op order/op_key wrong: %+v", got.Operations[0])
	}
	if len(got.Operations[0].Tags) != 1 || got.Operations[0].Tags[0] != "pets" {
		t.Errorf("tags not round-tripped: %v", got.Operations[0].Tags)
	}

	spec, err := repo.GetSpec(ctx, a.ID)
	if err != nil || spec != a.SpecJSON {
		t.Errorf("spec mismatch: err=%v", err)
	}
}

// TestAPICatalogRepo_LinksAndTrash covers the v95 links: an API linked to a
// service shows up under that service and, through it, under the service's
// project; a direct project link works too; trash keeps the links.
func TestAPICatalogRepo_LinksAndTrash(t *testing.T) {
	ctx := context.Background()
	d, owner := newCatalogDB(t)
	repo := store.NewAPICatalogRepo(d.SQL)

	proj := &models.Project{Name: "folha"}
	other := &models.Project{Name: "other"}
	for _, p := range []*models.Project{proj, other} {
		if err := store.NewProjectRepo(d.SQL).Create(ctx, p); err != nil {
			t.Fatalf("create project: %v", err)
		}
	}
	svc := &models.Service{Nickname: "api-servidores", ProjectID: &proj.ID}
	if err := store.NewServiceRepo(d.SQL).Create(ctx, svc); err != nil {
		t.Fatalf("create service: %v", err)
	}

	viaService, ops := sampleCatalog(owner)
	direct, ops2 := sampleCatalog(owner)
	direct.Name = "Direct"
	loose, ops3 := sampleCatalog(owner)
	loose.Name = "Loose"
	for _, c := range []struct {
		a   *models.APICatalog
		ops []models.APIOperation
	}{{viaService, ops}, {direct, ops2}, {loose, ops3}} {
		if err := repo.Create(ctx, c.a, c.ops); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if err := repo.SetLinks(ctx, viaService.ID, []int64{svc.ID, svc.ID}, []int64{}); err != nil {
		t.Fatalf("set links: %v", err)
	}
	if err := repo.SetLinks(ctx, direct.ID, nil, []int64{other.ID}); err != nil {
		t.Fatalf("set links: %v", err)
	}

	names := func(f models.APICatalogFilter) []string {
		t.Helper()
		list, err := repo.List(ctx, f)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		out := []string{}
		for _, a := range list {
			out = append(out, a.Name)
		}
		return out
	}
	if got := names(models.APICatalogFilter{ServiceID: svc.ID}); len(got) != 1 || got[0] != "Petstore" {
		t.Errorf("by service = %v, want [Petstore]", got)
	}
	if got := names(models.APICatalogFilter{ProjectID: proj.ID}); len(got) != 1 || got[0] != "Petstore" {
		t.Errorf("by project (via service) = %v, want [Petstore]", got)
	}
	if got := names(models.APICatalogFilter{ProjectID: other.ID}); len(got) != 1 || got[0] != "Direct" {
		t.Errorf("by project (direct) = %v, want [Direct]", got)
	}

	got, err := repo.Get(ctx, viaService.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.ServiceIDs) != 1 || got.ServiceIDs[0] != svc.ID || len(got.ProjectIDs) != 0 {
		t.Errorf("links = services %v projects %v, want [%d] []", got.ServiceIDs, got.ProjectIDs, svc.ID)
	}
	if l, _ := repo.Get(ctx, loose.ID); l == nil || l.ServiceIDs == nil || len(l.ServiceIDs) != 0 {
		t.Errorf("loose api links should be [] not nil: %+v", l)
	}

	// Trash → gone from lists, listed in trash; restore → back with its links.
	if found, err := repo.SoftDelete(ctx, viaService.ID); err != nil || !found {
		t.Fatalf("soft delete: found=%v err=%v", found, err)
	}
	if found, _ := repo.SoftDelete(ctx, viaService.ID); found {
		t.Error("second soft delete should report not found")
	}
	if got := names(models.APICatalogFilter{ServiceID: svc.ID}); len(got) != 0 {
		t.Errorf("trashed api still listed: %v", got)
	}
	trash, err := repo.ListTrash(ctx)
	if err != nil || len(trash) != 1 || trash[0].ID != viaService.ID {
		t.Fatalf("trash = %+v err=%v", trash, err)
	}
	if found, err := repo.Restore(ctx, viaService.ID); err != nil || !found {
		t.Fatalf("restore: found=%v err=%v", found, err)
	}
	if got := names(models.APICatalogFilter{ServiceID: svc.ID}); len(got) != 1 {
		t.Errorf("restored api lost its service link: %v", got)
	}
}

func TestAPICatalogRepo_ListSearchAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	d, owner := newCatalogDB(t)
	repo := store.NewAPICatalogRepo(d.SQL)
	a, ops := sampleCatalog(owner)
	if err := repo.Create(ctx, a, ops); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := repo.List(ctx, models.APICatalogFilter{Query: "pet"})
	if err != nil || len(list) != 1 {
		t.Fatalf("list query: err=%v n=%d", err, len(list))
	}
	if list[0].OperationCount != 2 {
		t.Errorf("op count in list = %d", list[0].OperationCount)
	}

	hits, err := repo.SearchOperations(ctx, "listPets", 0, 0)
	if err != nil || len(hits) != 1 {
		t.Fatalf("search ops: err=%v n=%d", err, len(hits))
	}
	if hits[0].OpKey != "listPets" {
		t.Errorf("search hit op_key = %q", hits[0].OpKey)
	}

	// Documentation (description) is searchable: the word lives ONLY in the
	// POST op's description, nowhere in its path/summary/tags.
	docHits, err := repo.SearchOperations(ctx, "brandnewzebra", 0, 0)
	if err != nil || len(docHits) != 1 {
		t.Fatalf("description search: err=%v n=%d", err, len(docHits))
	}
	if docHits[0].OpKey != "POST /pets" || docHits[0].Description == "" {
		t.Errorf("description hit unexpected: %+v", docHits[0])
	}

	if _, err := repo.SoftDelete(ctx, a.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	got, err := repo.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Error("expected soft-deleted catalog to be invisible")
	}
}
