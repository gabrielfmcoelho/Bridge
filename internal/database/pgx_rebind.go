package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync"

	"github.com/jackc/pgx/v5/stdlib"
)

// The pgx driver uses $1/$2/... placeholders, but the entire models layer
// writes queries with "?" (the SQLite idiom). Instead of rewriting every
// call site, we register a custom database/sql driver that wraps pgx and
// rewrites each query string with database.Rebind before handing it off to
// the underlying connection. The rest of the codebase can then use the
// same SQL text against either backend.

const pgxRebindDriverName = "pgx-rebind"

var registerOnce sync.Once

func registerPgxRebindDriver() {
	registerOnce.Do(func() {
		for _, name := range sql.Drivers() {
			if name == pgxRebindDriverName {
				return
			}
		}
		sql.Register(pgxRebindDriverName, &rebindDriver{inner: &stdlib.Driver{}})
	})
}

type rebindDriver struct{ inner driver.Driver }

func (d *rebindDriver) Open(dsn string) (driver.Conn, error) {
	c, err := d.inner.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &rebindConn{inner: c}, nil
}

type rebindConn struct {
	inner driver.Conn
}

func (c *rebindConn) Prepare(query string) (driver.Stmt, error) {
	return c.inner.Prepare(Rebind(query))
}

func (c *rebindConn) Close() error { return c.inner.Close() }

func (c *rebindConn) Begin() (driver.Tx, error) { return c.inner.Begin() }

func (c *rebindConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if cp, ok := c.inner.(driver.ConnPrepareContext); ok {
		return cp.PrepareContext(ctx, Rebind(query))
	}
	return c.Prepare(query)
}

func (c *rebindConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := c.inner.(driver.ExecerContext); ok {
		return ec.ExecContext(ctx, Rebind(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *rebindConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := c.inner.(driver.QueryerContext); ok {
		return qc.QueryContext(ctx, Rebind(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *rebindConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := c.inner.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	return c.Begin()
}

func (c *rebindConn) CheckNamedValue(nv *driver.NamedValue) error {
	if nvc, ok := c.inner.(driver.NamedValueChecker); ok {
		return nvc.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

func (c *rebindConn) ResetSession(ctx context.Context) error {
	if rs, ok := c.inner.(driver.SessionResetter); ok {
		return rs.ResetSession(ctx)
	}
	return nil
}

func (c *rebindConn) IsValid() bool {
	if v, ok := c.inner.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}
