package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func openDB(t *testing.T) *database.DB {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOAuthStateRepo_CreateValidateSingleUse(t *testing.T) {
	ctx := context.Background()
	repo := store.NewOAuthStateRepo(openDB(t).SQL)

	s, err := repo.Create(ctx, "gitlab")
	if err != nil || s.State == "" || s.Provider != "gitlab" {
		t.Fatalf("create = %+v, %v", s, err)
	}

	// First validate consumes and returns it.
	got, err := repo.Validate(ctx, s.State)
	if err != nil || got == nil || got.Provider != "gitlab" {
		t.Fatalf("validate = %+v, %v", got, err)
	}
	// Second validate of the same token returns (nil, nil) — single use.
	got2, err := repo.Validate(ctx, s.State)
	if err != nil || got2 != nil {
		t.Fatalf("re-validate = %+v, %v; want nil, nil", got2, err)
	}
	// Unknown token -> (nil, nil).
	if g, err := repo.Validate(ctx, "nope"); g != nil || err != nil {
		t.Fatalf("unknown = %+v, %v; want nil,nil", g, err)
	}
}

func TestProjectAIAnalysisRepo_UpsertOverwrites(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	// project_ai_analyses.project_id FKs projects; seed one.
	var pid int64
	if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p') RETURNING id`).Scan(&pid); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	repo := store.NewProjectAIAnalysisRepo(d.SQL)

	if got, _ := repo.Get(ctx, pid); got != nil {
		t.Fatalf("get on empty = %+v, want nil", got)
	}
	if err := repo.Upsert(ctx, &models.ProjectAIAnalysis{ProjectID: pid, Content: "v1", Locale: "en", CommitsUsed: 3, ReposUsed: 1}); err != nil {
		t.Fatalf("upsert v1: %v", err)
	}
	if err := repo.Upsert(ctx, &models.ProjectAIAnalysis{ProjectID: pid, Content: "v2", Locale: "pt", CommitsUsed: 5, ReposUsed: 2}); err != nil {
		t.Fatalf("upsert v2: %v", err)
	}
	got, err := repo.Get(ctx, pid)
	if err != nil || got == nil || got.Content != "v2" || got.Locale != "pt" || got.CommitsUsed != 5 {
		t.Fatalf("after overwrite = %+v, %v; want v2/pt/5", got, err)
	}
}

func TestAlertSettingsRepo_DefaultsAndUpdate(t *testing.T) {
	ctx := context.Background()
	repo := store.NewAlertSettingsRepo(openDB(t).SQL)

	// Fresh DB: migrations seed defaults (80/60/5).
	t0, err := repo.GetThresholds(ctx)
	if err != nil || t0.ResourceCritical != 80 || t0.ResourceWarning != 60 || t0.ResourceInfoLow != 5 {
		t.Fatalf("defaults = %+v, %v", t0, err)
	}

	if err := repo.UpdateThresholds(ctx, &models.AlertThresholds{ResourceCritical: 90, ResourceWarning: 70, ResourceInfoLow: 10}); err != nil {
		t.Fatalf("update: %v", err)
	}
	t1, _ := repo.GetThresholds(ctx)
	if t1.ResourceCritical != 90 || t1.ResourceWarning != 70 || t1.ResourceInfoLow != 10 {
		t.Fatalf("after update = %+v, want 90/70/10", t1)
	}
}
