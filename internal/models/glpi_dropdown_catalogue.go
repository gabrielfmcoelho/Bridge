package models

import (
	"encoding/json"
	"time"
)

// GlpiDropdownCatalogue is an admin-curated list of options for a single GLPI
// itemtype (ITILCategory, Entity, Location, Supplier, User, Group, …). Used by
// the Formcreator picker when the REST profile can't read the itemtype directly.
// Persistence lives in internal/store.GlpiDropdownCatalogueRepo — this file is
// the pure data types only.
type GlpiDropdownCatalogue struct {
	ID          int64           `json:"id"`
	Itemtype    string          `json:"itemtype"`
	Options     json.RawMessage `json:"options"`
	OptionCount int             `json:"option_count"`
	UpdatedAt   time.Time       `json:"updated_at"`
	UpdatedBy   *int64          `json:"updated_by,omitempty"`
}

// GlpiDropdownCatalogueSummary is the list-view shape (no options body).
type GlpiDropdownCatalogueSummary struct {
	Itemtype    string    `json:"itemtype"`
	OptionCount int       `json:"option_count"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   *int64    `json:"updated_by,omitempty"`
}
