package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAppSettingsRepo_ValueSetAndBranding(t *testing.T) {
	ctx := context.Background()
	repo := store.NewAppSettingsRepo(openDB(t).SQL)

	// Unknown key -> "".
	if v := repo.Value(ctx, "does_not_exist"); v != "" {
		t.Fatalf("unknown key = %q, want empty", v)
	}

	// Set then read back; Set upserts (no duplicate).
	if err := repo.Set(ctx, "custom_key", "v1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v := repo.Value(ctx, "custom_key"); v != "v1" {
		t.Fatalf("value = %q, want v1", v)
	}
	if err := repo.Set(ctx, "custom_key", "v2"); err != nil {
		t.Fatalf("set2: %v", err)
	}
	if v := repo.Value(ctx, "custom_key"); v != "v2" {
		t.Fatalf("value after upsert = %q, want v2", v)
	}

	// Update branding triple, then Get reflects it.
	if err := repo.Update(ctx, &models.AppSettings{AppName: "Bridge", AppColor: "#123456", AppLogo: "logo.png"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.Get(ctx)
	if err != nil || got.AppName != "Bridge" || got.AppColor != "#123456" || got.AppLogo != "logo.png" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	// Value sees the same row Update wrote.
	if v := repo.Value(ctx, "app_name"); v != "Bridge" {
		t.Fatalf("value(app_name) = %q, want Bridge", v)
	}
}
