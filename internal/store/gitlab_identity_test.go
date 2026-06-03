package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestUserGitLabTokenRepo_UpsertGetDelete(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	ures, _ := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin')`)
	userID, _ := ures.LastInsertId()
	repo := store.NewUserGitLabTokenRepo(d.SQL)

	tok := &models.UserGitLabToken{UserID: userID, GitLabBaseURL: "https://gl", AccessTokenCipher: []byte{1}, AccessTokenNonce: []byte{2}, GitLabUsername: "ada"}
	if err := repo.Upsert(ctx, tok); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := repo.Get(ctx, userID, "https://gl")
	if err != nil || got == nil || got.GitLabUsername != "ada" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	// Upsert again (same key) -> update, no duplicate.
	tok.GitLabUsername = "ada2"
	if err := repo.Upsert(ctx, tok); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	got, _ = repo.Get(ctx, userID, "https://gl")
	if got.GitLabUsername != "ada2" {
		t.Fatalf("after upsert2 = %+v", got)
	}
	if err := repo.Delete(ctx, userID, "https://gl"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.Get(ctx, userID, "https://gl"); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestProjectGitLabLinkRepo_CRUDAndOwnershipDelete(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	p1, _ := d.SQL.Exec(`INSERT INTO projects (name) VALUES ('p1')`)
	pid1, _ := p1.LastInsertId()
	p2, _ := d.SQL.Exec(`INSERT INTO projects (name) VALUES ('p2')`)
	pid2, _ := p2.LastInsertId()
	repo := store.NewProjectGitLabLinkRepo(d.SQL)

	l := &models.ProjectGitLabLink{ProjectID: pid1, GitLabProjectID: 10, GitLabBaseURL: "https://gl", GitLabPath: "a/b"}
	if err := repo.Create(ctx, l); err != nil {
		t.Fatalf("create: %v", err)
	}
	if l.Kind != models.GitLabLinkKindProject {
		t.Fatalf("kind = %q, want default project", l.Kind)
	}

	first, _ := repo.GetFirst(ctx, pid1)
	if first == nil || first.GitLabProjectID != 10 {
		t.Fatalf("getfirst = %+v", first)
	}
	list, _ := repo.List(ctx, pid1)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}

	// DeleteByID refuses cross-project deletion.
	ok, err := repo.DeleteByID(ctx, l.ID, pid2)
	if err != nil || ok {
		t.Fatalf("cross-project delete = %v, %v; want (false, nil)", ok, err)
	}
	// Correct owner deletes.
	ok, err = repo.DeleteByID(ctx, l.ID, pid1)
	if err != nil || !ok {
		t.Fatalf("owner delete = %v, %v; want (true, nil)", ok, err)
	}
}

func TestUserIdentityRepo_CreateGetList(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	ures, _ := d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin')`)
	userID, _ := ures.LastInsertId()
	repo := store.NewUserIdentityRepo(d.SQL)

	id := &models.UserExternalIdentity{UserID: userID, ProviderName: "keycloak", ExternalID: "kc-1"}
	if err := repo.Create(ctx, id); err != nil {
		t.Fatalf("create: %v", err)
	}
	if id.ExternalData != "{}" {
		t.Fatalf("external_data default = %q, want {}", id.ExternalData)
	}
	got, err := repo.GetByProviderAndExternalID(ctx, "keycloak", "kc-1")
	if err != nil || got == nil || got.UserID != userID {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if g, _ := repo.GetByProviderAndExternalID(ctx, "keycloak", "nope"); g != nil {
		t.Fatalf("get(missing) = %+v, want nil", g)
	}
	list, _ := repo.ListByUser(ctx, userID)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
}
