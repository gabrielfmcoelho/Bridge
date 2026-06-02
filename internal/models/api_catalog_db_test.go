package models_test

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// newCatalogDB opens a fresh migrated sqlite DB and returns it plus a user id
// to satisfy the owner_user_id / created_by foreign keys.
func newCatalogDB(t *testing.T) (*database.DB, int64) {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	u := &models.User{Username: "owner", DisplayName: "Owner", Role: "editor", Email: "o@example.com"}
	if err := models.CreateUser(d.SQL, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return d, u.ID
}

func sampleCatalog(scope string, parentID *int64, owner int64) (*models.APICatalog, []models.APIOperation) {
	a := &models.APICatalog{
		Scope:        scope,
		ParentID:     parentID,
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

func TestCreateAndGetAPICatalog(t *testing.T) {
	d, owner := newCatalogDB(t)

	a, ops := sampleCatalog(models.APICatalogScopeAvulso, nil, owner)
	if err := models.CreateAPICatalog(d.SQL, a, ops); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == 0 {
		t.Fatal("expected assigned id")
	}

	got, err := models.GetAPICatalog(d.SQL, a.ID)
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

	spec, err := models.GetAPICatalogSpec(d.SQL, a.ID)
	if err != nil || spec != a.SpecJSON {
		t.Errorf("spec mismatch: err=%v", err)
	}
}

func TestAPICatalogScopeValidation(t *testing.T) {
	d, owner := newCatalogDB(t)

	// projeto without parent_id must be rejected by Validate (before DB).
	bad, ops := sampleCatalog(models.APICatalogScopeProjeto, nil, owner)
	if err := models.CreateAPICatalog(d.SQL, bad, ops); err == nil {
		t.Error("expected projeto-without-parent to be rejected")
	}

	// avulso with a parent_id must be rejected too.
	pid := int64(99)
	bad2, ops2 := sampleCatalog(models.APICatalogScopeAvulso, &pid, owner)
	if err := models.CreateAPICatalog(d.SQL, bad2, ops2); err == nil {
		t.Error("expected avulso-with-parent to be rejected")
	}
}

func TestListSearchAndSoftDelete(t *testing.T) {
	d, owner := newCatalogDB(t)
	a, ops := sampleCatalog(models.APICatalogScopeAvulso, nil, owner)
	if err := models.CreateAPICatalog(d.SQL, a, ops); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := models.ListAPICatalog(d.SQL, models.APICatalogFilter{Query: "pet"})
	if err != nil || len(list) != 1 {
		t.Fatalf("list query: err=%v n=%d", err, len(list))
	}
	if list[0].OperationCount != 2 {
		t.Errorf("op count in list = %d", list[0].OperationCount)
	}

	hits, err := models.SearchOperations(d.SQL, "listPets", "", nil)
	if err != nil || len(hits) != 1 {
		t.Fatalf("search ops: err=%v n=%d", err, len(hits))
	}
	if hits[0].OpKey != "listPets" {
		t.Errorf("search hit op_key = %q", hits[0].OpKey)
	}

	// Documentation (description) is searchable: the word lives ONLY in the
	// POST op's description, nowhere in its path/summary/tags.
	docHits, err := models.SearchOperations(d.SQL, "brandnewzebra", "", nil)
	if err != nil || len(docHits) != 1 {
		t.Fatalf("description search: err=%v n=%d", err, len(docHits))
	}
	if docHits[0].OpKey != "POST /pets" || docHits[0].Description == "" {
		t.Errorf("description hit unexpected: %+v", docHits[0])
	}

	if err := models.SoftDeleteAPICatalog(d.SQL, a.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	got, err := models.GetAPICatalog(d.SQL, a.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Error("expected soft-deleted catalog to be invisible")
	}
}
