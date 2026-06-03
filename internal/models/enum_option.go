package models

// EnumOption is a configurable dropdown value for a category (situacao,
// hospedagem, service_type, …). Persistence lives in
// internal/store.EnumOptionRepo — this file is the pure data type only.
type EnumOption struct {
	Category  string `json:"category"`
	Value     string `json:"value"`
	SortOrder int    `json:"sort_order"`
	Color     string `json:"color"`
}
