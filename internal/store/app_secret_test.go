package store_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestAppSecretRepo_StoreRevealConfiguredClear(t *testing.T) {
	ctx := context.Background()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	repo := store.NewAppSecretRepo(d.SQL)

	// Absent key: not configured, Reveal returns (_, false, nil).
	if repo.Configured(ctx, "grafana_api_token") {
		t.Fatal("absent key should not be configured")
	}
	if v, ok, err := repo.Reveal(ctx, d.Encryptor, "grafana_api_token"); ok || v != "" || err != nil {
		t.Fatalf("reveal(absent) = %q,%v,%v; want \"\",false,nil", v, ok, err)
	}

	// Store → encrypted at rest (hex), revealed back to plaintext.
	if err := repo.Store(ctx, d.Encryptor, "grafana_api_token", "glsa-secret-xyz"); err != nil {
		t.Fatalf("store: %v", err)
	}
	if !repo.Configured(ctx, "grafana_api_token") {
		t.Fatal("after store, key should be configured")
	}
	v, ok, err := repo.Reveal(ctx, d.Encryptor, "grafana_api_token")
	if err != nil || !ok || v != "glsa-secret-xyz" {
		t.Fatalf("reveal = %q,%v,%v; want plaintext,true,nil", v, ok, err)
	}

	// At-rest value is NOT plaintext (encryption actually happened).
	var cipher string
	d.SQL.QueryRow(`SELECT cipher FROM app_secrets WHERE key='grafana_api_token'`).Scan(&cipher)
	if cipher == "" || cipher == "glsa-secret-xyz" {
		t.Fatalf("at-rest cipher = %q, want non-empty encrypted hex", cipher)
	}

	// Store again rotates (upsert, not duplicate).
	if err := repo.Store(ctx, d.Encryptor, "grafana_api_token", "rotated"); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if v, _, _ := repo.Reveal(ctx, d.Encryptor, "grafana_api_token"); v != "rotated" {
		t.Fatalf("after rotate reveal = %q, want rotated", v)
	}

	// Clear removes it.
	if err := repo.Clear(ctx, "grafana_api_token"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if repo.Configured(ctx, "grafana_api_token") {
		t.Fatal("after clear, key should not be configured")
	}
}

// TestAppSecretRepo_TableMigrated proves the v71 migration created app_secrets
// and removed the legacy _cipher/_nonce rows from app_settings.
func TestAppSecretRepo_TableMigrated(t *testing.T) {
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='app_secrets'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("app_secrets table missing (n=%d, err=%v)", n, err)
	}
	// No cipher/nonce rows should remain in app_settings on a fresh DB.
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key LIKE '%\_cipher' ESCAPE '\' OR key LIKE '%\_nonce' ESCAPE '\'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("legacy cipher rows remain in app_settings (n=%d, err=%v)", n, err)
	}
}
