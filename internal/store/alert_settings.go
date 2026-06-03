package store

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// AlertSettingsRepo owns the alert-threshold slice of app_settings (the three
// alert_resource_* keys). It is a focused repo over a handful of key/value
// rows rather than a dedicated table.
type AlertSettingsRepo struct {
	db *sql.DB
}

// NewAlertSettingsRepo constructs an AlertSettingsRepo over the given DB handle.
func NewAlertSettingsRepo(db *sql.DB) *AlertSettingsRepo { return &AlertSettingsRepo{db: db} }

// GetThresholds reads the alert thresholds, falling back to sane defaults for
// any key that is absent or unparseable.
func (r *AlertSettingsRepo) GetThresholds(ctx context.Context) (*models.AlertThresholds, error) {
	t := &models.AlertThresholds{ResourceCritical: 80, ResourceWarning: 60, ResourceInfoLow: 5}
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, value FROM app_settings WHERE key IN ('alert_resource_critical', 'alert_resource_warning', 'alert_resource_info_low')`)
	if err != nil {
		return t, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		switch k {
		case "alert_resource_critical":
			t.ResourceCritical = n
		case "alert_resource_warning":
			t.ResourceWarning = n
		case "alert_resource_info_low":
			t.ResourceInfoLow = n
		}
	}
	return t, rows.Err()
}

// UpdateThresholds upserts the three threshold keys atomically.
func (r *AlertSettingsRepo) UpdateThresholds(ctx context.Context, t *models.AlertThresholds) error {
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

	for _, kv := range []struct {
		k string
		v int
	}{
		{"alert_resource_critical", t.ResourceCritical},
		{"alert_resource_warning", t.ResourceWarning},
		{"alert_resource_info_low", t.ResourceInfoLow},
	} {
		if _, err := stmt.ExecContext(ctx, kv.k, strconv.Itoa(kv.v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
