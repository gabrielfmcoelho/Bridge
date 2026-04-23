package glpi

// initSessionResponse is the shape returned by GET /apirest.php/initSession.
type initSessionResponse struct {
	SessionToken string `json:"session_token"`
}

// Ticket is a subset of GLPI's Ticket resource — just what sshcm renders.
type Ticket struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`     // = title
	Content         string `json:"content"`  // description (HTML-ish)
	Status          int    `json:"status"`   // 1=new 2=assigned 3=planned 4=waiting 5=solved 6=closed
	Priority        int    `json:"priority"`
	Urgency         int    `json:"urgency"`
	Impact          int    `json:"impact"`
	EntitiesID      int    `json:"entities_id"`
	Date            string `json:"date"`
	DateMod         string `json:"date_mod"`
	UsersIDRequester int   `json:"users_id_requester,omitempty"`
}

// TicketCreateInput is the body for POST /apirest.php/Ticket.
type TicketCreateInput struct {
	Name       string `json:"name"`
	Content    string `json:"content"`
	EntitiesID int    `json:"entities_id,omitempty"`
	ITILCategoriesID int `json:"itilcategories_id,omitempty"`
	Priority   int    `json:"priority,omitempty"`
	Urgency    int    `json:"urgency,omitempty"`
	Impact     int    `json:"impact,omitempty"`
}

// Followup is a single comment/message on a ticket (ITILFollowup).
// Content is HTML as GLPI stores it — callers sanitize before rendering.
type Followup struct {
	ID        int    `json:"id"`
	ItemsID   int    `json:"items_id"`
	Content   string `json:"content"`
	Date      string `json:"date"`
	DateMod   string `json:"date_mod"`
	UsersID   int    `json:"users_id"`
	IsPrivate int    `json:"is_private"`
}

// Task is a TicketTask — internal work items tied to a ticket.
type Task struct {
	ID               int    `json:"id"`
	TicketsID        int    `json:"tickets_id"`
	Content          string `json:"content"`
	State            int    `json:"state"`             // 0=info 1=todo 2=done
	Date             string `json:"date"`
	DateMod          string `json:"date_mod"`
	ActionTime       int    `json:"actiontime"`        // seconds planned
	UsersID          int    `json:"users_id"`
	UsersIDTech      int    `json:"users_id_tech,omitempty"`
}

// Solution is an ITILSolution attached to a ticket.
// GLPI emits `date_creation` here instead of `date`, and some older instances
// only populate `date_mod`. We accept all three; callers prefer the first
// non-empty via BestDate().
type Solution struct {
	ID           int    `json:"id"`
	ItemsID      int    `json:"items_id"`
	Content      string `json:"content"`
	Status       int    `json:"status"` // 1=proposed 2=accepted 3=refused
	Date         string `json:"date"`
	DateCreation string `json:"date_creation"`
	DateMod      string `json:"date_mod"`
	UsersID      int    `json:"users_id"`
}

// BestDate returns the most reliable timestamp the ITILSolution exposes.
func (s *Solution) BestDate() string {
	if s.Date != "" {
		return s.Date
	}
	if s.DateCreation != "" {
		return s.DateCreation
	}
	return s.DateMod
}

// GlpiUser is the trimmed User used for attributing followups/tasks.
type GlpiUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Realname  string `json:"realname,omitempty"`
	Firstname string `json:"firstname,omitempty"`
}

// DisplayName returns the best human label for a GLPI user.
func (u *GlpiUser) DisplayName() string {
	if u == nil {
		return ""
	}
	full := u.Firstname
	if u.Realname != "" {
		if full != "" {
			full += " "
		}
		full += u.Realname
	}
	if full != "" {
		return full
	}
	return u.Name
}

// Entity mirrors GLPI's Entity resource.
type Entity struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	CompleteName string `json:"completename"`
}

// Computer mirrors GLPI's Computer asset.
type Computer struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	SerialNumber string `json:"serial,omitempty"`
	EntitiesID int    `json:"entities_id"`
}

// MyProfilesResponse shapes GET /apirest.php/getMyProfiles.
type MyProfilesResponse struct {
	MyProfiles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"myprofiles"`
}

// StatusLabel returns a human label for a GLPI ticket status code. Matches the
// values defined in GLPI (see inc/commonitilobject.class.php).
func StatusLabel(code int) string {
	switch code {
	case 1:
		return "Novo"
	case 2:
		return "Atribuído"
	case 3:
		return "Planejado"
	case 4:
		return "Pendente"
	case 5:
		return "Resolvido"
	case 6:
		return "Fechado"
	default:
		return "Desconhecido"
	}
}

// StatusSlug is a lowercased-underscore form used by the UI for color mapping
// without coupling to the Portuguese-locale labels.
func StatusSlug(code int) string {
	switch code {
	case 1:
		return "new"
	case 2:
		return "assigned"
	case 3:
		return "planned"
	case 4:
		return "waiting"
	case 5:
		return "solved"
	case 6:
		return "closed"
	default:
		return "unknown"
	}
}

// IsActive reports whether a status code represents a ticket that still needs work.
func IsActive(code int) bool {
	return code < 5
}
