package glpiv2

// Ref is GLPI v2's nested relation shape — every FK is returned as
// {"id":N,"name":"..."} instead of a flat numeric column like the legacy
// API. Used for category/location/request_type/entity/sla_* etc.
type Ref struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// RefInput is the write-side shape for a Ref field: GLPI accepts just the id.
type RefInput struct {
	ID int `json:"id"`
}

// TicketType mirrors the Ticket.type enum (1=Incident, 2=Request).
const (
	TicketTypeIncident = 1
	TicketTypeRequest  = 2
)

// TicketStatus ids (Ticket.status.id) — matches GLPI's CommonITILObject
// status constants, same values as the legacy API.
const (
	StatusNew      = 1
	StatusApproval = 10
	StatusAssigned = 2
	StatusPlanned  = 3
	StatusWaiting  = 4
	StatusSolved   = 5
	StatusClosed   = 6
)

// Ticket is a subset of GLPI v2's Ticket resource — only what Bridge renders.
type Ticket struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Content      string `json:"content"`
	Type         int    `json:"type"`
	Urgency      int    `json:"urgency"`
	Impact       int    `json:"impact"`
	Priority     int    `json:"priority"`
	Status       Ref    `json:"status"`
	Category     Ref    `json:"category"`
	Location     Ref    `json:"location"`
	RequestType  Ref    `json:"request_type"`
	Entity       Ref    `json:"entity"`
	Date         string `json:"date"`
	DateMod      string `json:"date_mod"`
	DateCreation string `json:"date_creation"`
}

// TeamMember is one row of Ticket.team — a requester/observer/assignee,
// either a User or a Group (Type distinguishes them).
type TeamMember struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "User" | "Group"
	Role string `json:"role"` // "requester" | "observer" | "assigned"
}

// TicketCreateInput is the body for POST /Assistance/Ticket. Omitted refs
// (zero ID) are left out so GLPI applies its own defaults.
type TicketCreateInput struct {
	Name        string    `json:"name"`
	Content     string    `json:"content"`
	Type        int       `json:"type,omitempty"`
	Urgency     int       `json:"urgency,omitempty"`
	Impact      int       `json:"impact,omitempty"`
	Priority    int       `json:"priority,omitempty"`
	Category    *RefInput `json:"category,omitempty"`
	Location    *RefInput `json:"location,omitempty"`
	RequestType *RefInput `json:"request_type,omitempty"`
	Entity      *RefInput `json:"entity,omitempty"`
}

// FollowupInput is the body for POST .../Timeline/Followup.
type FollowupInput struct {
	Content   string `json:"content"`
	IsPrivate bool   `json:"is_private,omitempty"`
}

// DropdownItem is the trimmed shape returned by /Dropdowns/{itemtype} and
// /Administration/{itemtype} searches — enough for a name-search picker.
type DropdownItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	CompleteName string `json:"completename,omitempty"`
}

// listEnvelope is how GLPI v2 may wrap paginated list responses; some
// deployments return a bare JSON array instead. decodeList in client.go
// handles both shapes.
type listEnvelope[T any] struct {
	Items []T `json:"items"`
}
