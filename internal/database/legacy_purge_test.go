package database

import (
	"os"
	"testing"
)

// TestDropLegacyServiceCredentialsTable asserts the purge is idempotent and
// removes the table. Always-runs sqlite test — postgres parity is implicit
// because both dialects accept `DROP TABLE IF EXISTS`.
func TestDropLegacyServiceCredentialsTable(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	// Table should exist after migrations applied.
	var n int
	if err := d.SQL.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='service_credentials'`,
	).Scan(&n); err != nil {
		t.Fatalf("count pre: %v", err)
	}
	if n != 1 {
		t.Fatalf("service_credentials should exist before purge, got %d rows", n)
	}

	if err := DropLegacyServiceCredentialsTable(d.SQL); err != nil {
		t.Fatalf("drop: %v", err)
	}

	if err := d.SQL.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='service_credentials'`,
	).Scan(&n); err != nil {
		t.Fatalf("count post: %v", err)
	}
	if n != 0 {
		t.Errorf("service_credentials should be gone after purge, got %d rows", n)
	}

	// Idempotent: second call on already-dropped table is a no-op.
	if err := DropLegacyServiceCredentialsTable(d.SQL); err != nil {
		t.Errorf("second purge should be a no-op, got %v", err)
	}
}

// TestMaybeDropLegacySecrets_EnvGated covers both branches of the env
// switch: the default (off) leaves the table, and =1 drops it.
func TestMaybeDropLegacySecrets_EnvGated(t *testing.T) {
	// Branch 1: env unset → table stays.
	t.Run("env unset is no-op", func(t *testing.T) {
		t.Setenv(MigrateDropLegacySecretsEnv, "")
		dir := t.TempDir()
		d, err := Open(dir)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer d.Close()
		var n int
		if err := d.SQL.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='service_credentials'`,
		).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		if n != 1 {
			t.Errorf("expected service_credentials present, got %d", n)
		}
	})

	// Branch 2: env =1 → table gets dropped during Open's migration tail.
	t.Run("env set drops table on Open", func(t *testing.T) {
		t.Setenv(MigrateDropLegacySecretsEnv, "1")
		dir := t.TempDir()
		d, err := Open(dir)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer d.Close()
		var n int
		if err := d.SQL.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='service_credentials'`,
		).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		if n != 0 {
			t.Errorf("expected service_credentials dropped, got %d", n)
		}
	})

	// Sanity: env value other than "1" doesn't fire the drop. Catches the
	// "set MIGRATE_DROP_LEGACY_SECRETS=true and wonder why nothing happened"
	// foot-gun before it hits production.
	t.Run("env =true (not 1) is no-op", func(t *testing.T) {
		t.Setenv(MigrateDropLegacySecretsEnv, "true")
		dir := t.TempDir()
		d, err := Open(dir)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer d.Close()
		var n int
		_ = d.SQL.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='service_credentials'`,
		).Scan(&n)
		if n != 1 {
			t.Errorf("expected service_credentials still present when env != '1', got %d", n)
		}
		// Explicit unset for following tests in this file. t.Setenv handles
		// this automatically, but the assertion makes the intent obvious.
		_ = os.Unsetenv(MigrateDropLegacySecretsEnv)
	})
}
