package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// miscSGAScope returns the sga entidade id and a ctx scoped to it.
func miscSGAScope(t *testing.T, d *database.DB) (int64, context.Context) {
	t.Helper()
	var sga int64
	if err := d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga); err != nil {
		t.Fatalf("sga: %v", err)
	}
	return sga, store.WithScope(context.Background(), store.Scope{EntidadeIDs: []int64{sga}})
}

func miscGrantSGA(t *testing.T, d *database.DB, at store.AssetType, id, sga int64) {
	t.Helper()
	if err := store.NewAssetEntidadeRepo(d.SQL).Replace(context.Background(), d.SQL, at, id, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
		t.Fatalf("grant: %v", err)
	}
}

func TestContactRepo_Scope(t *testing.T) {
	d := openDB(t)
	sga, scoped := miscSGAScope(t, d)
	repo := store.NewContactRepo(d.SQL)
	bg := context.Background()

	mine := &models.Contact{Name: "mine"}
	other := &models.Contact{Name: "other"}
	for _, c := range []*models.Contact{mine, other} {
		if err := repo.Create(bg, c); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	miscGrantSGA(t, d, store.AssetContact, mine.ID, sga)

	if got, err := repo.List(scoped); err != nil || len(got) != 1 || got[0].ID != mine.ID {
		t.Fatalf("scoped list = %+v, %v; want only mine", got, err)
	}
	if got, err := repo.Get(scoped, other.ID); err != nil || got != nil {
		t.Fatalf("scoped get other = %+v, %v; want nil,nil", got, err)
	}
	if err := repo.Delete(scoped, other.ID); err != nil {
		t.Fatalf("scoped delete: %v", err)
	}
	if got, err := repo.List(bg); err != nil || len(got) != 2 {
		t.Fatalf("unscoped list = %+v, %v; want both (scoped delete must be a no-op)", got, err)
	}
}

func TestExternalToolRepo_Scope(t *testing.T) {
	d := openDB(t)
	sga, scoped := miscSGAScope(t, d)
	repo := store.NewExternalToolRepo(d.SQL)
	bg := context.Background()

	mine := &models.ExternalTool{Name: "mine", URL: "http://a"}
	other := &models.ExternalTool{Name: "other", URL: "http://b"}
	for _, x := range []*models.ExternalTool{mine, other} {
		if err := repo.Create(bg, x); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	miscGrantSGA(t, d, store.AssetTool, mine.ID, sga)

	if got, err := repo.List(scoped); err != nil || len(got) != 1 || got[0].ID != mine.ID {
		t.Fatalf("scoped list = %+v, %v; want only mine", got, err)
	}
	if got, err := repo.Get(scoped, other.ID); err != nil || got != nil {
		t.Fatalf("scoped get other = %+v, %v; want nil,nil", got, err)
	}
	if err := repo.Delete(scoped, other.ID); err != nil {
		t.Fatalf("scoped delete: %v", err)
	}
	if got, err := repo.List(bg); err != nil || len(got) != 2 {
		t.Fatalf("unscoped list = %+v, %v; want both (scoped delete must be a no-op)", got, err)
	}
}

func TestSSHKeyRepo_Scope(t *testing.T) {
	d := openDB(t)
	sga, scoped := miscSGAScope(t, d)
	repo := store.NewSSHKeyRepo(d.SQL)
	bg := context.Background()

	mine := &models.SSHKey{Name: "mine"}
	other := &models.SSHKey{Name: "other"}
	for _, k := range []*models.SSHKey{mine, other} {
		if err := repo.Create(bg, k); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	miscGrantSGA(t, d, store.AssetSSHKey, mine.ID, sga)

	if got, err := repo.List(scoped); err != nil || len(got) != 1 || got[0].ID != mine.ID {
		t.Fatalf("scoped list = %+v, %v; want only mine", got, err)
	}
	if got, err := repo.Get(scoped, other.ID); err != nil || got != nil {
		t.Fatalf("scoped get other = %+v, %v; want nil,nil", got, err)
	}
	if err := repo.Delete(scoped, other.ID); err != nil {
		t.Fatalf("scoped delete: %v", err)
	}
	if got, err := repo.List(bg); err != nil || len(got) != 2 {
		t.Fatalf("unscoped list = %+v, %v; want both (scoped delete must be a no-op)", got, err)
	}
}
