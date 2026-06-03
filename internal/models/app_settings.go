package models

// AppSettings is the app branding triple stored in the app_settings key/value
// table. Persistence (and all other key/value access) lives in
// internal/store.AppSettingsRepo — this file is the pure data type only.
type AppSettings struct {
	AppName  string `json:"app_name"`
	AppColor string `json:"app_color"`
	AppLogo  string `json:"app_logo"`
}
