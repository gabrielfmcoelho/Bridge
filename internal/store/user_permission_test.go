package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestUserRepo_CRUDAndCount(t *testing.T) {
	ctx := context.Background()
	repo := store.NewUserRepo(openDB(t).SQL)

	if n, _ := repo.Count(ctx); n != 0 {
		t.Fatalf("count empty = %d, want 0", n)
	}

	u := &models.User{Username: "ada", PasswordHash: "h", DisplayName: "Ada", Role: "editor"}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 || u.AuthProvider != "local" {
		t.Fatalf("create = %+v, want ID + auth_provider=local", u)
	}

	byID, _ := repo.GetByID(ctx, u.ID)
	byName, _ := repo.GetByUsername(ctx, "ada")
	if byID == nil || byName == nil || byID.ID != byName.ID || byID.Role != "editor" {
		t.Fatalf("byID=%+v byName=%+v", byID, byName)
	}

	u.Role = "admin"
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := repo.UpdatePassword(ctx, u.ID, "h2"); err != nil {
		t.Fatalf("updatepw: %v", err)
	}
	got, _ := repo.GetByID(ctx, u.ID)
	if got.Role != "admin" || got.PasswordHash != "h2" {
		t.Fatalf("after updates = %+v", got)
	}

	if list, _ := repo.List(ctx); len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	if n, _ := repo.Count(ctx); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.GetByID(ctx, u.ID); got != nil {
		t.Fatalf("after delete = %+v, want nil", got)
	}
}

func TestPermissionRepo_RBAC(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPermissionRepo(openDB(t).SQL)

	// Migrations seed permissions + role_permissions.
	perms, err := repo.ListPermissions(ctx)
	if err != nil || len(perms) == 0 {
		t.Fatalf("listpermissions = %d, %v; want seeded", len(perms), err)
	}

	// Admin always passes, even for an unknown permission.
	if !repo.Has(ctx, "admin", "anything.at.all") {
		t.Fatal("admin should always have permission")
	}

	// Grant a custom permission to 'editor' and check.
	pick := perms[0].Code
	if repo.Has(ctx, "viewer", pick) {
		// viewer may or may not have it by seed; force a known role we control.
	}
	if err := repo.SetForRole(ctx, "customrole", []string{pick}); err != nil {
		t.Fatalf("setforrole: %v", err)
	}
	if !repo.Has(ctx, "customrole", pick) {
		t.Fatalf("customrole should have %q after SetForRole", pick)
	}
	if repo.Has(ctx, "customrole", "no.such.perm") {
		t.Fatal("customrole should NOT have an ungranted perm")
	}
	forRole, _ := repo.ListForRole(ctx, "customrole")
	if len(forRole) != 1 || forRole[0] != pick {
		t.Fatalf("listforrole = %+v, want [%s]", forRole, pick)
	}
}

func TestPermissionRepo_RoleMappings(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPermissionRepo(openDB(t).SQL)

	m := &models.AuthRoleMapping{ProviderName: "keycloak", ExternalGroup: "admins", LocalRole: "admin"}
	if err := repo.CreateRoleMapping(ctx, m); err != nil {
		t.Fatalf("create mapping: %v", err)
	}
	_ = repo.CreateRoleMapping(ctx, &models.AuthRoleMapping{ProviderName: "keycloak", ExternalGroup: "devs", LocalRole: "editor"})

	// Resolve picks the highest-privilege match.
	role := repo.ResolveRoleFromExternalGroups(ctx, "keycloak", []string{"devs", "admins"})
	if role != "admin" {
		t.Fatalf("resolve = %q, want admin (highest)", role)
	}
	if role := repo.ResolveRoleFromExternalGroups(ctx, "keycloak", []string{"unknown"}); role != "" {
		t.Fatalf("resolve(unknown) = %q, want empty", role)
	}

	mappings, _ := repo.ListRoleMappings(ctx)
	if len(mappings) != 2 {
		t.Fatalf("mappings = %+v, want 2", mappings)
	}
	if err := repo.DeleteRoleMapping(ctx, m.ID); err != nil {
		t.Fatalf("delete mapping: %v", err)
	}
	mappings, _ = repo.ListRoleMappings(ctx)
	if len(mappings) != 1 {
		t.Fatalf("after delete = %+v, want 1", mappings)
	}
}
