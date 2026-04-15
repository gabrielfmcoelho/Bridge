package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var (
	migratePgDSN    string
	migratePgSource string
	migratePgDryRun bool
)

// migratePgCmd is a CLI shortcut for the same cross-dialect transfer that
// the /api/backup → /api/restore pair performs over HTTP. It is useful in
// airgap or one-shot-migration scenarios where running the web server is
// inconvenient.
//
// Under the hood it uses the portable backup format defined in
// internal/database/portable.go: it opens the source database, serialises
// every user-data table to an in-memory gzipped JSON buffer, opens the
// target database, and replays the buffer. Type coercion (int ↔ bool,
// weakly-typed sqlite strings in integer columns, column renames) comes
// for free from that shared code path.
var migratePgCmd = &cobra.Command{
	Use:   "migrate-pg",
	Short: "Copy all data from a SQLite database into a Postgres target",
	Long: `Reads the SQLite database at --source (defaults to the local sshcm.db)
and writes every row into the Postgres DSN given by --target. The target
must already have been initialised with sshcm's schema: run

    SSHCM_DB_DRIVER=postgres SSHCM_DB_DSN=<dsn> sshcm list

once to apply migrations before invoking migrate-pg.

This uses the portable backup format internally, so the same safety
guarantees (type coercion, FK deferral, sequence resets) apply as when
uploading a .sshcmbak file through the web UI.

The command never writes to SQLite.`,
	RunE: runMigratePg,
}

func init() {
	sourcePath := filepath.Join(sshcmConfigDir(), "sshcm.db")
	migratePgCmd.Flags().StringVar(&migratePgDSN, "target", "", "Postgres DSN, e.g. postgres://user:pass@host/db?sslmode=disable (required)")
	migratePgCmd.Flags().StringVar(&migratePgSource, "source", sourcePath, "Path to the source SQLite database")
	migratePgCmd.Flags().BoolVar(&migratePgDryRun, "dry-run", false, "Read source rows but do not write to target")
	rootCmd.AddCommand(migratePgCmd)
}

func runMigratePg(cmd *cobra.Command, args []string) error {
	if migratePgDSN == "" && !migratePgDryRun {
		return fmt.Errorf("--target is required (use --dry-run to only verify the source)")
	}
	if _, err := os.Stat(migratePgSource); err != nil {
		return fmt.Errorf("source sqlite db not found at %s: %w", migratePgSource, err)
	}

	fmt.Printf("source: %s\n", migratePgSource)
	srcDB, err := openDirectSqlite(migratePgSource)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer srcDB.SQL.Close()

	var buf bytes.Buffer
	if err := srcDB.WritePortableBackup(&buf); err != nil {
		return fmt.Errorf("dump source: %w", err)
	}
	fmt.Printf("dumped %d bytes of portable backup\n", buf.Len())

	backup, err := database.ReadPortableBackup(&buf)
	if err != nil {
		return fmt.Errorf("decode backup: %w", err)
	}
	total := 0
	for tbl, rows := range backup.Tables {
		if len(rows) == 0 {
			continue
		}
		fmt.Printf("  %-28s %6d rows\n", tbl, len(rows))
		total += len(rows)
	}
	fmt.Printf("\n%d rows across %d tables in source.\n", total, len(backup.Tables))

	if migratePgDryRun {
		fmt.Println("(dry-run: target was not modified)")
		return nil
	}

	tgtDB, err := openDirectPostgres(migratePgDSN)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer tgtDB.SQL.Close()

	if err := tgtDB.RestorePortable(backup); err != nil {
		return fmt.Errorf("restore into target: %w", err)
	}
	fmt.Println("\nDone. Target now contains the full source snapshot.")
	return nil
}

// openDirectSqlite opens a sqlite database at path without going through
// database.Open so the caller doesn't need an Encryptor / secret.key in
// the expected configDir layout. Portable dump and restore operate on raw
// column values so the Encryptor isn't touched.
func openDirectSqlite(path string) (*database.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &database.DB{SQL: db, Dialect: database.DialectSQLite}, nil
}

func openDirectPostgres(dsn string) (*database.DB, error) {
	sql.Register("pgx-migrate", &stdlib.Driver{})
	db, err := sql.Open("pgx-migrate", dsn)
	if err != nil {
		return nil, err
	}
	return &database.DB{SQL: db, Dialect: database.DialectPostgres}, nil
}
