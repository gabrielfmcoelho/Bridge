package store

import (
	"context"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

func TestParentRegistry_KnownScopes(t *testing.T) {
	cases := []struct {
		scope      models.SecretScope
		wantTable  string
		wantSoft   bool
		registered bool
	}{
		{models.SecretScopeService, "services", true, true},
		{models.SecretScopeHost, "hosts", true, true},
		{models.SecretScopeProjeto, "projects", true, true},
		{models.SecretScopeTool, "external_tools", true, true},
		{models.SecretScopeAvulso, "", false, false}, // avulso never has a parent
	}
	for _, tc := range cases {
		spec, ok := ParentOf(tc.scope)
		if ok != tc.registered {
			t.Fatalf("scope %q registered=%v, want %v", tc.scope, ok, tc.registered)
		}
		if !tc.registered {
			continue
		}
		if spec.Table != tc.wantTable || spec.SoftDelete != tc.wantSoft {
			t.Fatalf("scope %q = %+v, want {Table:%q SoftDelete:%v}", tc.scope, spec, tc.wantTable, tc.wantSoft)
		}
	}
}

// DeleteParent must reject an unregistered scope BEFORE touching the DB, so a
// nil *sql.DB is safe here and proves the guard short-circuits.
func TestDeleteParent_UnregisteredScopeErrors(t *testing.T) {
	err := DeleteParent(context.Background(), nil, models.ActorContext{}, models.SecretScopeAvulso, 1)
	if err == nil || !strings.Contains(err.Error(), "not a registered") {
		t.Fatalf("err = %v, want 'not a registered' guard", err)
	}
}

// All registered parents are soft-delete now (R3), so the hard-delete guard is
// unreachable via a real scope; the meaningful guard is the unregistered-scope
// one (avulso never has a parent). Both guard before touching the DB, so nil is safe.
func TestRestoreParent_UnregisteredScopeErrors(t *testing.T) {
	err := RestoreParent(context.Background(), nil, models.ActorContext{}, models.SecretScopeAvulso, 1)
	if err == nil || !strings.Contains(err.Error(), "not a registered") {
		t.Fatalf("err = %v, want 'not a registered' guard", err)
	}
}
