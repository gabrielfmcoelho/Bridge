package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAPICatalogRepo_EntidadeScope(t *testing.T) {
	d := openDB(t)
	ctx := context.Background()
	u := &models.User{Username: "cat-owner", DisplayName: "Owner", Role: "editor", Email: "cat@example.com"}
	if err := store.NewUserRepo(d.SQL).Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	repo := store.NewAPICatalogRepo(d.SQL)
	mk := func(name string) int64 {
		a := &models.APICatalog{Scope: models.APICatalogScopeAvulso, Name: name, SourceType: models.APICatalogSourceUpload,
			SpecJSON: `{"openapi":"3.0.0"}`, OwnerUserID: u.ID, CreatedBy: u.ID}
		if err := repo.Create(ctx, a, nil); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return a.ID
	}
	mine, other := mk("mine"), mk("other")

	var sga int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug='sga'`).Scan(&sga); err != nil {
		t.Fatalf("sga: %v", err)
	}
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(ctx, d.SQL, store.AssetAPICatalog, mine, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	scoped := store.WithScope(ctx, store.Scope{EntidadeIDs: []int64{sga}})

	list, err := repo.List(scoped, models.APICatalogFilter{})
	if err != nil || len(list) != 1 || list[0].ID != mine {
		t.Fatalf("scoped list = %+v, %v; want only %d", list, err, mine)
	}
	if got, err := repo.Get(scoped, other); err != nil || got != nil {
		t.Fatalf("scoped get other = %+v, %v; want nil", got, err)
	}
	if spec, err := repo.GetSpec(scoped, other); err != nil || spec != "" {
		t.Fatalf("scoped getspec other = %q, %v; want empty", spec, err)
	}
	if err := repo.UpdateMeta(scoped, other, "renamed", "", "", ""); err != nil {
		t.Fatalf("scoped update other: %v", err)
	}
	if err := repo.SoftDelete(scoped, other); err != nil {
		t.Fatalf("scoped delete other: %v", err)
	}
	if got, err := repo.Get(ctx, other); err != nil || got == nil || got.Name != "other" {
		t.Fatalf("unscoped get other after scoped update/delete = %+v, %v; want untouched", got, err)
	}
	if list, err := repo.List(ctx, models.APICatalogFilter{}); err != nil || len(list) != 2 {
		t.Fatalf("unscoped list = %d, %v; want 2", len(list), err)
	}
}
