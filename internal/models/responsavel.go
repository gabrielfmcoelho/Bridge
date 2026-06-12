package models

// Responsavel is a contact linked to an entity (host/dns/service/project) as a
// responsável. The persisted fields (entity_type/entity_id/contact_id/is_main)
// live in the polymorphic `responsaveis` table; the contact fields
// (name…is_external) are joined from `contacts` at read time. The JSON shape
// matches the frontend's unified EntityResponsavel — no per-entity id field.
type Responsavel struct {
	ID         int64  `json:"id"`
	ContactID  int64  `json:"contact_id"`
	IsMain     bool   `json:"is_main"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	Entity     string `json:"entity"`
	Notes      string `json:"notes"`
	IsExternal bool   `json:"is_external"`
}

// ResponsavelInput is the write payload: link an existing contact (ContactID
// required) to an entity, optionally as the main responsável. Inline contact
// creation is not supported — manage contacts via /api/contacts.
type ResponsavelInput struct {
	ContactID int64 `json:"contact_id"`
	IsMain    bool  `json:"is_main"`
}
