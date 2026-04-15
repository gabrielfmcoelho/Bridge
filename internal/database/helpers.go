package database

import "database/sql"

// Execer is the common subset of *sql.DB and *sql.Tx used by the helpers
// below. It lets call sites pass either a connection or an open transaction
// without duplicating logic.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// InsertReturningID runs an INSERT and returns the auto-generated primary
// key. For SQLite it uses res.LastInsertId(); for Postgres it appends
// "RETURNING id" to the query and scans the result, since the pgx stdlib
// driver does not populate LastInsertId.
//
// The caller must supply an INSERT whose primary key column is named `id`.
// Queries that need a different column name can use InsertReturningCol.
func InsertReturningID(db Execer, query string, args ...any) (int64, error) {
	return InsertReturningCol(db, query, "id", args...)
}

// InsertReturningCol is like InsertReturningID but lets the caller specify
// the column whose value should be returned on Postgres.
func InsertReturningCol(db Execer, query, col string, args ...any) (int64, error) {
	if active == DialectPostgres {
		var id int64
		if err := db.QueryRow(query+" RETURNING "+col, args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
