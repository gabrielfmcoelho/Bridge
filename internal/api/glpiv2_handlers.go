package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	glpiv2client "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/glpiv2"
)

// glpiv2Handlers talks to the SEAD/NTGD GLPI instance (High-Level REST API
// v2, OAuth2). Independent from glpiHandlers (the ETIPI/legacy GLPI
// integration) — different protocol, different credential, configured via
// GLPI_SEAD_* env vars rather than app_settings/Settings UI. See
// internal/integrations/glpiv2 for why.
type glpiv2Handlers struct {
	client *glpiv2client.Client // nil when GLPI_SEAD_ENABLED isn't fully configured
}

func newGlpiv2Handlers() *glpiv2Handlers {
	return &glpiv2Handlers{client: glpiv2client.NewClient(glpiv2client.LoadSettingsFromEnv())}
}

func (h *glpiv2Handlers) requireClient(w http.ResponseWriter) *glpiv2client.Client {
	if h.client == nil {
		jsonError(w, http.StatusServiceUnavailable, "GLPI SEAD/NTGD integration is not configured (GLPI_SEAD_* env vars)")
		return nil
	}
	return h.client
}

// handleStatus reports whether the integration is configured, without
// leaking any credential — used by the frontend to show/hide this source.
//
//	@Summary		GLPI SEAD/NTGD integration status
//	@Description	Any role. {enabled: bool}.
//	@Tags			glpi-sead
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/api/glpi-sead/status [get]
func (h *glpiv2Handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{"enabled": h.client != nil})
}

// handleListTickets godoc
//
//	GET /api/glpi-sead/tickets?q=<substring on name>&start=0&limit=50
//
//	@Summary		List GLPI SEAD/NTGD tickets
//	@Description	Any role. q filters by ticket name substring (RSQL `name=like=`). 503 when not configured; 502 when GLPI rejects the call.
//	@Tags			glpi-sead
//	@Produce		json
//	@Param			q		query		string	false	"Substring filter on ticket name"
//	@Param			start	query		int		false	"Pagination offset (default 0)"
//	@Param			limit	query		int		false	"Page size (default 50)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/tickets [get]
func (h *glpiv2Handlers) handleListTickets(w http.ResponseWriter, r *http.Request) {
	client := h.requireClient(w)
	if client == nil {
		return
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	var filter string
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		filter = `name=like="*` + strings.ReplaceAll(q, `"`, `\"`) + `*"`
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tickets, err := client.ListTickets(ctx, filter, start, limit)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "list failed: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"tickets": tickets, "count": len(tickets)})
}

// handleGetTicket godoc
//
//	GET /api/glpi-sead/tickets/{id}
//
//	@Summary		Get a GLPI SEAD/NTGD ticket
//	@Tags			glpi-sead
//	@Produce		json
//	@Param			id	path		int	true	"Ticket ID"
//	@Success		200	{object}	glpiv2.Ticket
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Failure		503	{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/tickets/{id} [get]
func (h *glpiv2Handlers) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	client := h.requireClient(w)
	if client == nil {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	t, err := client.GetTicket(ctx, id)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+err.Error())
		return
	}
	jsonOK(w, t)
}

// handleGetTicketTimeline godoc
//
//	GET /api/glpi-sead/tickets/{id}/timeline
//
//	@Summary		GLPI SEAD/NTGD ticket timeline
//	@Description	Raw timeline rows (followups/tasks/solutions/documents/validations) — each row carries its own `itemtype` field.
//	@Tags			glpi-sead
//	@Produce		json
//	@Param			id	path		int	true	"Ticket ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Failure		503	{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/tickets/{id}/timeline [get]
func (h *glpiv2Handlers) handleGetTicketTimeline(w http.ResponseWriter, r *http.Request) {
	client := h.requireClient(w)
	if client == nil {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	items, err := client.GetTicketTimeline(ctx, id)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"items": items, "count": len(items)})
}

// handleGetTicketTeam godoc
//
//	GET /api/glpi-sead/tickets/{id}/team
//
//	@Summary		GLPI SEAD/NTGD ticket requesters/observers/assignees
//	@Tags			glpi-sead
//	@Produce		json
//	@Param			id	path		int	true	"Ticket ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Failure		503	{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/tickets/{id}/team [get]
func (h *glpiv2Handlers) handleGetTicketTeam(w http.ResponseWriter, r *http.Request) {
	client := h.requireClient(w)
	if client == nil {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	team, err := client.GetTicketTeam(ctx, id)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"team": team, "count": len(team)})
}

// createTicketSeadRequest is the create-ticket body for the SEAD/NTGD GLPI.
type createTicketSeadRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        int    `json:"type,omitempty"`
	Urgency     int    `json:"urgency,omitempty"`
	Impact      int    `json:"impact,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	CategoryID  int    `json:"category_id,omitempty"`
	LocationID  int    `json:"location_id,omitempty"`
	RequestType int    `json:"request_type_id,omitempty"`
	EntityID    int    `json:"entity_id,omitempty"`
}

// handleCreateTicket godoc
//
//	POST /api/glpi-sead/tickets
//
//	@Summary		Create a GLPI SEAD/NTGD ticket
//	@Description	Editor+.
//	@Tags			glpi-sead
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createTicketSeadRequest	true	"Ticket"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/tickets [post]
func (h *glpiv2Handlers) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	client := h.requireClient(w)
	if client == nil {
		return
	}
	var req createTicketSeadRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	in := glpiv2client.TicketCreateInput{
		Name:     req.Title,
		Content:  req.Description,
		Type:     req.Type,
		Urgency:  req.Urgency,
		Impact:   req.Impact,
		Priority: req.Priority,
	}
	if req.CategoryID > 0 {
		in.Category = &glpiv2client.RefInput{ID: req.CategoryID}
	}
	if req.LocationID > 0 {
		in.Location = &glpiv2client.RefInput{ID: req.LocationID}
	}
	if req.RequestType > 0 {
		in.RequestType = &glpiv2client.RefInput{ID: req.RequestType}
	}
	if req.EntityID > 0 {
		in.Entity = &glpiv2client.RefInput{ID: req.EntityID}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	id, err := client.CreateTicket(ctx, in)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "create failed: "+err.Error())
		return
	}

	resp := map[string]any{"ticket_id": id}
	// GLPI v2 doesn't auto-assign the authenticated account as requester on
	// create (unlike the legacy API) — add it explicitly so tickets opened
	// through Bridge show the service account as the requester, not nobody.
	if meID, merr := client.MeID(ctx); merr == nil && meID > 0 {
		if aerr := client.AddTeamMember(ctx, id, "User", meID, "requester"); aerr != nil {
			resp["warning"] = "ticket created but failed to set requester: " + aerr.Error()
		}
	} else if merr != nil {
		resp["warning"] = "ticket created but failed to resolve requester: " + merr.Error()
	}
	jsonCreated(w, resp)
}

// handleAddFollowup godoc
//
//	POST /api/glpi-sead/tickets/{id}/followups
//
//	@Summary		Add a followup to a GLPI SEAD/NTGD ticket
//	@Description	Editor+.
//	@Tags			glpi-sead
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int										true	"Ticket ID"
//	@Param			body	body		object{content=string,is_private=bool}	true	"Followup"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/tickets/{id}/followups [post]
func (h *glpiv2Handlers) handleAddFollowup(w http.ResponseWriter, r *http.Request) {
	client := h.requireClient(w)
	if client == nil {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	var req struct {
		Content   string `json:"content"`
		IsPrivate bool   `json:"is_private"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		jsonError(w, http.StatusBadRequest, "content is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	fid, err := client.AddFollowup(ctx, id, req.Content, req.IsPrivate)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "create failed: "+err.Error())
		return
	}
	jsonCreated(w, map[string]any{"id": fid})
}

// seadAllowedDropdownItemtypes allow-lists /Dropdowns/{itemtype} — extend as
// new form fields need it.
func seadAllowedDropdownItemtype(t string) bool {
	switch t {
	case "ITILCategory", "Location", "RequestType", "TaskCategory":
		return true
	}
	return false
}

// seadAllowedAdministrationItemtype allow-lists /Administration/{itemtype}.
func seadAllowedAdministrationItemtype(t string) bool {
	switch t {
	case "User", "Entity", "Group":
		return true
	}
	return false
}

// handleSearchDropdown godoc
//
//	GET /api/glpi-sead/dropdowns/{itemtype}/search?q=<substring>
//
//	@Summary		Search a GLPI SEAD/NTGD dropdown
//	@Tags			glpi-sead
//	@Produce		json
//	@Param			itemtype	path		string	true	"GLPI itemtype"	Enums(ITILCategory, Location, RequestType, TaskCategory)
//	@Param			q			query		string	false	"Substring filter"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		502			{object}	httpx.ErrorResponse
//	@Failure		503			{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/dropdowns/{itemtype}/search [get]
func (h *glpiv2Handlers) handleSearchDropdown(w http.ResponseWriter, r *http.Request) {
	itemtype := r.PathValue("itemtype")
	if !seadAllowedDropdownItemtype(itemtype) {
		jsonError(w, http.StatusBadRequest, "itemtype not allowed")
		return
	}
	client := h.requireClient(w)
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	items, err := client.SearchDropdown(ctx, itemtype, r.URL.Query().Get("q"), 50)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"items": items, "count": len(items)})
}

// handleSearchAdministration godoc
//
//	GET /api/glpi-sead/administration/{itemtype}/search?q=<substring>
//
//	@Summary		Search GLPI SEAD/NTGD users/entities/groups
//	@Tags			glpi-sead
//	@Produce		json
//	@Param			itemtype	path		string	true	"GLPI itemtype"	Enums(User, Entity, Group)
//	@Param			q			query		string	false	"Substring filter"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		502			{object}	httpx.ErrorResponse
//	@Failure		503			{object}	httpx.ErrorResponse
//	@Router			/api/glpi-sead/administration/{itemtype}/search [get]
func (h *glpiv2Handlers) handleSearchAdministration(w http.ResponseWriter, r *http.Request) {
	itemtype := r.PathValue("itemtype")
	if !seadAllowedAdministrationItemtype(itemtype) {
		jsonError(w, http.StatusBadRequest, "itemtype not allowed")
		return
	}
	client := h.requireClient(w)
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	items, err := client.SearchAdministration(ctx, itemtype, r.URL.Query().Get("q"), 50)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"items": items, "count": len(items)})
}

// registerRoutes binds the SEAD/NTGD GLPI integration under /api/glpi-sead/*.
func (h *glpiv2Handlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/glpi-sead/status", h.handleStatus)
	rr.auth("GET /api/glpi-sead/tickets", h.handleListTickets)
	rr.auth("GET /api/glpi-sead/tickets/{id}", h.handleGetTicket)
	rr.auth("GET /api/glpi-sead/tickets/{id}/timeline", h.handleGetTicketTimeline)
	rr.auth("GET /api/glpi-sead/tickets/{id}/team", h.handleGetTicketTeam)
	rr.role("editor", "POST /api/glpi-sead/tickets", h.handleCreateTicket)
	rr.role("editor", "POST /api/glpi-sead/tickets/{id}/followups", h.handleAddFollowup)
	rr.auth("GET /api/glpi-sead/dropdowns/{itemtype}/search", h.handleSearchDropdown)
	rr.auth("GET /api/glpi-sead/administration/{itemtype}/search", h.handleSearchAdministration)
}
