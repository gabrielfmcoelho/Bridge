// Package pgtest provides an ephemeral Postgres for tests via testcontainers,
// handing each test an isolated schema. It deliberately does NOT import
// internal/database, so the database package's own (white-box) tests can use it
// without an import cycle. Higher-level packages use internal/dbtest, which wraps
// this to return a ready *database.DB.
package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	once      sync.Once
	baseDSN   string
	admin     *sql.DB
	startErr  error
	schemaSeq int64
)

func start() {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("sshcm_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		startErr = fmt.Errorf("start postgres container: %w", err)
		return
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		startErr = fmt.Errorf("container dsn: %w", err)
		return
	}
	baseDSN = dsn
	sql.Register("pgx-pgtest-admin", &stdlib.Driver{})
	admin, err = sql.Open("pgx-pgtest-admin", dsn)
	if err != nil {
		startErr = fmt.Errorf("admin open: %w", err)
		return
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		if pingErr := admin.PingContext(ctx); pingErr == nil {
			break
		} else if time.Now().After(deadline) {
			startErr = fmt.Errorf("admin ping: %w", pingErr)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	// The container is reaped by testcontainers' Ryuk when the process exits.
}

// SchemaDSN starts the shared container (once per test binary), creates a fresh
// uniquely-named schema, registers a cleanup to drop it, and returns a pgx DSN
// pinned to that schema (search_path). Skips the test if Docker/Postgres can't
// start. Pass the DSN to database.OpenDSN.
func SchemaDSN(t *testing.T) string {
	t.Helper()
	once.Do(start)
	if startErr != nil {
		t.Skipf("pgtest: %v (Docker required for Postgres-backed tests)", startErr)
		return ""
	}
	schema := fmt.Sprintf("t_%d", atomic.AddInt64(&schemaSeq, 1))
	if _, err := admin.Exec(`CREATE SCHEMA "` + schema + `"`); err != nil {
		t.Fatalf("pgtest: create schema: %v", err)
		return ""
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`) })
	return withSearchPath(baseDSN, schema)
}

// withSearchPath appends a libpq `options=-c search_path=<schema>` runtime param
// so every pooled connection lands in the test's schema.
func withSearchPath(dsn, schema string) string {
	sep := "&"
	if !strings.Contains(dsn, "?") {
		sep = "?"
	}
	return dsn + sep + "options=" + url.QueryEscape("-c search_path="+schema)
}
