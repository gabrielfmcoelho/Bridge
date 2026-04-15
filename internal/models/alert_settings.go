package models

import (
	"database/sql"
	"strconv"
)

type AlertThresholds struct {
	ResourceCritical int `json:"resource_critical"`
	ResourceWarning  int `json:"resource_warning"`
	ResourceInfoLow  int `json:"resource_info_low"`
}

func GetAlertThresholds(db *sql.DB) (*AlertThresholds, error) {
	t := &AlertThresholds{ResourceCritical: 80, ResourceWarning: 60, ResourceInfoLow: 5}
	rows, err := db.Query(`SELECT key, value FROM app_settings WHERE key IN ('alert_resource_critical', 'alert_resource_warning', 'alert_resource_info_low')`)
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

func UpdateAlertThresholds(db *sql.DB, t *AlertThresholds) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
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
		if _, err := stmt.Exec(kv.k, strconv.Itoa(kv.v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
