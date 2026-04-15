package models

import "database/sql"

type AppSettings struct {
	AppName  string `json:"app_name"`
	AppColor string `json:"app_color"`
	AppLogo  string `json:"app_logo"`
}

func GetAppSettings(db *sql.DB) (*AppSettings, error) {
	s := &AppSettings{}
	rows, err := db.Query(`SELECT key, value FROM app_settings WHERE key IN ('app_name', 'app_color', 'app_logo')`)
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

// GetAppSettingValue returns a single setting value by key, or empty string if not found.
func GetAppSettingValue(db *sql.DB, key string) string {
	var value string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

// SetAppSettingValue upserts a single setting value by key.
func SetAppSettingValue(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

func UpdateAppSettings(db *sql.DB, s *AppSettings) error {
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

	for _, kv := range []struct{ k, v string }{
		{"app_name", s.AppName},
		{"app_color", s.AppColor},
		{"app_logo", s.AppLogo},
	} {
		if _, err := stmt.Exec(kv.k, kv.v); err != nil {
			return err
		}
	}
	return tx.Commit()
}
