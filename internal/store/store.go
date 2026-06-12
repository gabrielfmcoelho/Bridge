// Package store is the repository layer: it owns SQL and dialect handling so
// that models become pure data types and handlers/services never embed queries
// directly. It generalizes the pattern proven by internal/vault (the secrets
// repository) to the rest of the codebase.
//
// Dependency direction: api/service -> store -> {vault, models, database}.
// store may import vault (the secrets exemplar) but nothing imports store back,
// so there is no cycle.
package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Execer is the minimal write surface common to *sql.DB and *sql.Tx, letting
// repository helpers run inside or outside a caller-supplied transaction.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Queryer is the minimal read surface common to *sql.DB and *sql.Tx.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// SoftDeleteByID sets deleted_at on a single live row and returns the number of
// rows affected (0 if already deleted or absent). `table` MUST be a trusted
// in-code constant — it is interpolated into the statement, never user input.
func SoftDeleteByID(ctx context.Context, exec Execer, table string, id int64) (int64, error) {
	res, err := exec.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, table),
		id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RestoreByID clears deleted_at on a single soft-deleted row and returns the
// number of rows affected (0 if not currently deleted or absent). `table` MUST
// be a trusted in-code constant.
func RestoreByID(ctx context.Context, exec Execer, table string, id int64) (int64, error) {
	res, err := exec.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, table),
		id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// HardDeleteByID removes a single row by id and returns rows affected. `table`
// MUST be a trusted in-code constant.
func HardDeleteByID(ctx context.Context, exec Execer, table string, id int64) (int64, error) {
	res, err := exec.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table), id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
