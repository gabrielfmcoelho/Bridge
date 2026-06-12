package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// AppSettingsRepo owns SQL for the app_settings key/value table. It is the
// platform's config accessor, referenced across api handlers, auth providers,
// and integration settings loaders; built inline at each call site until the
// Phase 2 container hoists it.
type AppSettingsRepo struct {
	db *sql.DB
}

// NewAppSettingsRepo constructs an AppSettingsRepo over the given DB handle.
func NewAppSettingsRepo(db *sql.DB) *AppSettingsRepo { return &AppSettingsRepo{db: db} }

// Get returns the branding triple (app_name/app_color/app_logo).
func (r *AppSettingsRepo) Get(ctx context.Context) (*models.AppSettings, error) {
	s := &models.AppSettings{}
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM app_settings WHERE key IN ('app_name', 'app_color', 'app_logo')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		switch k {
		case "app_name":
			s.AppName = v
		case "app_color":
			s.AppColor = v
		case "app_logo":
			s.AppLogo = v
		}
	}
	return s, rows.Err()
}

// Value returns a single setting value by key, or "" if absent.
func (r *AppSettingsRepo) Value(ctx context.Context, key string) string {
	var value string
	if err := r.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value); err != nil {
		return ""
	}
	return value
}

// Set upserts a single setting value by key.
func (r *AppSettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

// Update upserts the branding triple in one transaction.
func (r *AppSettingsRepo) Update(ctx context.Context, s *models.AppSettings) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, kv := range []struct{ k, v string }{
		{"app_name", s.AppName},
		{"app_color", s.AppColor},
		{"app_logo", s.AppLogo},
	} {
		if _, err := stmt.ExecContext(ctx, kv.k, kv.v); err != nil {
			return err
		}
	}
	return tx.Commit()
}
