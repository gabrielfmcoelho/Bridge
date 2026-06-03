package models

// Contact is a person/entity that can be linked as a responsável to hosts,
// services, projects, and DNS records. Persistence lives in
// internal/store.ContactRepo — this file is the pure data type only.
type Contact struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	Entity     string `json:"entity"`
	Notes      string `json:"notes"`
	IsExternal bool   `json:"is_external"`
}
