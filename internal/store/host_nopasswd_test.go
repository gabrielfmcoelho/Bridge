package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// hosts.sudo_nopasswd is what the scan found, not something a host form
// edits: a save from a stale copy of the host must not undo it.
func TestHostRepo_SudoNoPasswdIsScanOwned(t *testing.T) {
	ctx := context.Background()
	repo, _ := newHostRepo(t)

	h := &models.Host{Nickname: "key-only", OficialSlug: "key-only", Hostname: "10.0.0.9", User: "ops", Situacao: "active", HasKey: true}
	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got, _ := repo.GetByID(ctx, h.ID); got.SudoNoPasswd {
		t.Fatal("a new host starts as needing a password")
	}

	if err := repo.SetSudoNoPasswd(ctx, h.ID, true); err != nil {
		t.Fatalf("SetSudoNoPasswd: %v", err)
	}
	if got, _ := repo.GetByID(ctx, h.ID); !got.SudoNoPasswd {
		t.Fatal("the scan's finding was not stored")
	}

	h.Nickname = "renamed" // h still says SudoNoPasswd=false
	if err := repo.Update(ctx, h); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := repo.GetByID(ctx, h.ID); !got.SudoNoPasswd || got.Nickname != "renamed" {
		t.Fatalf("after an edit: %+v", got)
	}

	if err := repo.SetSudoNoPasswd(ctx, h.ID, false); err != nil {
		t.Fatalf("SetSudoNoPasswd: %v", err)
	}
	if got, _ := repo.GetByID(ctx, h.ID); got.SudoNoPasswd {
		t.Fatal("a later scan that needs a password must clear it")
	}
}
