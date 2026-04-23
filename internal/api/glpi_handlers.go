package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	glpiclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/glpi"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type glpiHandlers struct {
	db    *database.DB
	cache *glpiclient.SessionCache
}

// resolveClient loads instance settings and returns a ready-to-use Client, or
// nil + a human-readable reason string when the integration isn't usable.
func (h *glpiHandlers) resolveClient() (*glpiclient.Client, glpiclient.Settings, string) {
	settings, err := glpiclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		return nil, settings, "failed to load settings"
	}
	if !settings.Enabled {
		return nil, settings, "GLPI integration is disabled"
	}
	if settings.BaseURL == "" {
		return nil, settings, "GLPI base URL is not configured"
	}
	client := glpiclient.NewServiceClient(settings)
	if client == nil {
		return nil, settings, "GLPI client unavailable"
	}
	return client, settings, ""
}

// sessionFor wraps the cache lookup for a single profile — decrypting the
// stored user token on demand.
func (h *glpiHandlers) sessionFor(ctx context.Context, client *glpiclient.Client, profileID int64) (string, error) {
	return h.cache.Get(ctx, client, profileID, func(id int64) (string, error) {
		tok, err := models.GetGlpiToken(h.db.SQL, id)
		if err != nil {
			return "", err
		}
		if tok == nil {
			return "", fmt.Errorf("token profile %d not found", id)
		}
		if !tok.HasToken {
			return "", fmt.Errorf("profile %q has no user token stored", tok.Name)
		}
		return h.db.Encryptor.Decrypt(tok.UserTokenCipher, tok.UserTokenNonce)
	})
}

// ─── Admin CRUD for token profiles ───────────────────────────────────────────

func (h *glpiHandlers) handleListTokenProfiles(w http.ResponseWriter, r *http.Request) {
	tokens, err := models.ListGlpiTokens(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list profiles", err)
		return
	}
	if tokens == nil {
		tokens = []models.GlpiToken{}
	}
	jsonOK(w, tokens)
}

func (h *glpiHandlers) handleCreateTokenProfile(w http.ResponseWriter, r *http.Request) {
	var req models.GlpiTokenInput
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(req.UserToken) == "" {
		jsonError(w, http.StatusBadRequest, "user_token is required on create")
		return
	}

	cipher, nonce, err := h.db.Encryptor.Encrypt(strings.TrimSpace(req.UserToken))
	if err != nil {
		jsonServerError(w, r, "encrypt failed", err)
		return
	}

	tok := &models.GlpiToken{
		Name:            req.Name,
		Description:     req.Description,
		UserTokenCipher: cipher,
		UserTokenNonce:  nonce,
		DefaultEntityID: req.DefaultEntityID,
	}
	if err := models.CreateGlpiToken(h.db.SQL, tok); err != nil {
		jsonError(w, http.StatusConflict, "create failed (name conflict?): "+err.Error())
		return
	}
	tok.HasToken = true
	jsonCreated(w, tok)
}

func (h *glpiHandlers) handleUpdateTokenProfile(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	existing, err := models.GetGlpiToken(h.db.SQL, id)
	if err != nil {
		jsonServerError(w, r, "lookup failed", err)
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "profile not found")
		return
	}

	var req models.GlpiTokenInput
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	update := &models.GlpiToken{
		ID:              id,
		Name:            strings.TrimSpace(req.Name),
		Description:     req.Description,
		DefaultEntityID: req.DefaultEntityID,
	}
	// If the admin supplied a new plaintext token, re-encrypt. Otherwise leave stored cipher intact.
	if tok := strings.TrimSpace(req.UserToken); tok != "" && tok != "••••••••" {
		cipher, nonce, err := h.db.Encryptor.Encrypt(tok)
		if err != nil {
			jsonServerError(w, r, "encrypt failed", err)
			return
		}
		update.UserTokenCipher = cipher
		update.UserTokenNonce = nonce
	}
	if err := models.UpdateGlpiToken(h.db.SQL, update); err != nil {
		jsonServerError(w, r, "update failed", err)
		return
	}
	// Invalidate the cached session so the next call picks up any token change.
	h.cache.Invalidate(id)
	jsonOK(w, map[string]any{"status": "updated"})
}

func (h *glpiHandlers) handleDeleteTokenProfile(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	if err := models.DeleteGlpiToken(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "delete failed", err)
		return
	}
	h.cache.Invalidate(id)
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleTestTokenProfile hits /initSession + /getMyProfiles for a specific
// profile and returns the list of accessible profile names. Admins use this
// to sanity-check which identity the stored token belongs to.
func (h *glpiHandlers) handleTestTokenProfile(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonOK(w, map[string]any{"success": false, "error": reason})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	h.cache.Invalidate(id)
	session, err := h.sessionFor(ctx, client, id)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": mapGlpiError(err)})
		return
	}
	profiles, err := client.GetMyProfiles(ctx, session)
	if err != nil {
		jsonOK(w, map[string]any{"success": false, "error": mapGlpiError(err)})
		return
	}
	names := []string{}
	for _, p := range profiles.MyProfiles {
		names = append(names, p.Name)
	}
	jsonOK(w, map[string]any{
		"success":  true,
		"profiles": names,
	})
}

// ─── Dropdown catalogue CRUD (admin) ────────────────────────────────────────

// handleListDropdownCatalogues returns a summary per itemtype (counts + last
// updated). The full options payload isn't in this response — it's only
// fetched when the admin opens the editor for one row.
func (h *glpiHandlers) handleListDropdownCatalogues(w http.ResponseWriter, r *http.Request) {
	list, err := models.ListGlpiDropdownCatalogues(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list catalogues", err)
		return
	}
	if list == nil {
		list = []models.GlpiDropdownCatalogueSummary{}
	}
	jsonOK(w, map[string]any{
		"catalogues":     list,
		"allowed_itemtypes": allowedCatalogueItemtypes(),
	})
}

// handleGetDropdownCatalogue returns the full JSON options body for one
// itemtype. Empty response with 404 when the admin hasn't seeded it yet.
func (h *glpiHandlers) handleGetDropdownCatalogue(w http.ResponseWriter, r *http.Request) {
	itemtype := r.PathValue("itemtype")
	if !allowedDropdownItemtype(itemtype) {
		jsonError(w, http.StatusBadRequest, "itemtype not allowed")
		return
	}
	cat, err := models.GetGlpiDropdownCatalogue(h.db.SQL, itemtype)
	if err != nil {
		jsonServerError(w, r, "catalogue lookup failed", err)
		return
	}
	if cat == nil {
		// Return an empty shell so the editor modal can render a seeded form
		// without a special "not-yet-created" branch on the frontend.
		jsonOK(w, map[string]any{
			"itemtype":     itemtype,
			"options":      []catalogueOption{},
			"option_count": 0,
		})
		return
	}
	jsonOK(w, cat)
}

// handleUpsertDropdownCatalogue accepts a JSON body shaped like:
//
//	{"options": [{"id": 1, "name": "…", "completename": "…", "parent_id": 0}, …]}
//
// Validates every row has id>0 and a non-empty name, then persists.
func (h *glpiHandlers) handleUpsertDropdownCatalogue(w http.ResponseWriter, r *http.Request) {
	itemtype := r.PathValue("itemtype")
	if !allowedDropdownItemtype(itemtype) {
		jsonError(w, http.StatusBadRequest, "itemtype not allowed")
		return
	}
	var req struct {
		Options []catalogueOption `json:"options"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON body", err)
		return
	}
	// Re-validate + normalize: drop rows with id<=0 or empty name so garbage
	// from the scraper snippet doesn't pollute the DB.
	clean := make([]catalogueOption, 0, len(req.Options))
	seen := make(map[int]struct{}, len(req.Options))
	for _, o := range req.Options {
		if o.ID <= 0 || strings.TrimSpace(o.Name) == "" {
			continue
		}
		if _, dup := seen[o.ID]; dup {
			continue
		}
		seen[o.ID] = struct{}{}
		o.Name = strings.TrimSpace(o.Name)
		o.Completename = strings.TrimSpace(o.Completename)
		clean = append(clean, o)
	}
	payload, err := json.Marshal(clean)
	if err != nil {
		jsonServerError(w, r, "re-marshal failed", err)
		return
	}

	var userID *int64
	if u := auth.UserFromContext(r.Context()); u != nil {
		id := u.ID
		userID = &id
	}
	if err := models.UpsertGlpiDropdownCatalogue(h.db.SQL, itemtype, payload, len(clean), userID); err != nil {
		jsonServerError(w, r, "catalogue save failed", err)
		return
	}
	jsonOK(w, map[string]any{
		"itemtype":     itemtype,
		"option_count": len(clean),
	})
}

// handleDeleteDropdownCatalogue wipes the row for an itemtype — picker falls
// back to the REST path next request.
func (h *glpiHandlers) handleDeleteDropdownCatalogue(w http.ResponseWriter, r *http.Request) {
	itemtype := r.PathValue("itemtype")
	if !allowedDropdownItemtype(itemtype) {
		jsonError(w, http.StatusBadRequest, "itemtype not allowed")
		return
	}
	if err := models.DeleteGlpiDropdownCatalogue(h.db.SQL, itemtype); err != nil {
		jsonServerError(w, r, "catalogue delete failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// allowedCatalogueItemtypes returns the allow-listed itemtypes the admin can
// maintain catalogues for. Mirrors allowedDropdownItemtype but returns a slice
// so the frontend can render checkbox rows.
func allowedCatalogueItemtypes() []string {
	return []string{
		"Entity", "ITILCategory", "Location", "Supplier",
		"Computer", "Monitor", "NetworkEquipment", "Printer", "Phone",
		"User", "Group", "Software", "State",
	}
}

// ─── Ticket operations ──────────────────────────────────────────────────────

// createTicketRequest is the body used by every ticket-creation handler — project,
// host, or alert-scoped. The handler fills in sensible defaults from the context
// before POSTing to GLPI.
type createTicketRequest struct {
	ProfileID     int64  `json:"profile_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	EntityID      int    `json:"entity_id"`
	CategoryID    int    `json:"category_id"`
	HostSlug      string `json:"host_slug,omitempty"`       // optional — persists as host_chamado
	AlertID       int64  `json:"alert_id,omitempty"`        // optional — links via alert_chamado_links
	LinkComputer  bool   `json:"link_computer,omitempty"`   // if true + host_slug, find & link GLPI Computer
}

type createTicketResponse struct {
	TicketID        int    `json:"ticket_id"`
	TicketURL       string `json:"ticket_url"`
	ChamadoID       int64  `json:"chamado_id,omitempty"`
	ComputerLinked  bool   `json:"computer_linked,omitempty"`
	Warning         string `json:"warning,omitempty"`
}

// handleCreateTicket is the single creation entry point. Route:
//   POST /api/glpi/tickets
func (h *glpiHandlers) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	var req createTicketRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.ProfileID == 0 {
		jsonError(w, http.StatusBadRequest, "profile_id is required")
		return
	}

	client, settings, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	entityID := req.EntityID
	if entityID == 0 {
		// Fall back to profile default → instance default.
		tok, _ := models.GetGlpiToken(h.db.SQL, req.ProfileID)
		if tok != nil && tok.DefaultEntityID > 0 {
			entityID = tok.DefaultEntityID
		} else {
			entityID = settings.DefaultEntityID
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, req.ProfileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	ticketID, err := client.CreateTicket(ctx, session, glpiclient.TicketCreateInput{
		Name:             req.Title,
		Content:          req.Description,
		EntitiesID:       entityID,
		ITILCategoriesID: req.CategoryID,
	})
	if err != nil {
		jsonError(w, http.StatusBadGateway, "create failed: "+mapGlpiError(err))
		return
	}
	ticketURL := fmt.Sprintf("%s/front/ticket.form.php?id=%d", client.WebBaseURL(), ticketID)

	resp := createTicketResponse{TicketID: ticketID, TicketURL: ticketURL}

	// Optional link to a GLPI Computer asset (matched by slug or hostname).
	if req.HostSlug != "" && req.LinkComputer {
		host, _ := models.GetHostBySlug(h.db.SQL, req.HostSlug)
		if host != nil {
			if comp, cerr := client.FindComputerByName(ctx, session, host.OficialSlug); cerr == nil && comp != nil {
				if lerr := client.LinkComputerToTicket(ctx, session, ticketID, comp.ID); lerr == nil {
					resp.ComputerLinked = true
				} else {
					resp.Warning = "ticket created but computer link failed: " + lerr.Error()
				}
			}
		}
	}

	// Persist as host_chamado when we have a host context.
	if req.HostSlug != "" {
		host, _ := models.GetHostBySlug(h.db.SQL, req.HostSlug)
		if host != nil {
			userID := int64(0)
			if u := auth.UserFromContext(r.Context()); u != nil {
				userID = u.ID
			}
			cid, cerr := models.CreateExternalHostChamado(
				h.db.SQL,
				host.ID,
				userID,
				strconv.Itoa(ticketID),
				req.Title,
				"in_execution",
				time.Now().Format("02/01/2006"),
				"glpi",
				ticketURL,
			)
			if cerr == nil {
				resp.ChamadoID = cid
				// If this came from an alert, link them.
				if req.AlertID > 0 {
					_, _ = h.db.SQL.Exec(
						`INSERT INTO alert_chamado_links (alert_id, chamado_id) VALUES (?, ?)`,
						req.AlertID, cid,
					)
				}
			} else {
				if resp.Warning == "" {
					resp.Warning = "ticket created but failed to persist locally: " + cerr.Error()
				}
			}
		}
	}

	jsonCreated(w, resp)
}

// handleRefreshChamadoCache re-fetches a chamado's ticket from GLPI and updates
// cached_title + cached_status. Called by the host detail page to keep the
// existing ChamadoSection showing live GLPI data.
//   POST /api/hosts/{slug}/chamados/{chamadoId}/glpi/refresh?profile_id=<N>
func (h *glpiHandlers) handleRefreshChamadoCache(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	chamadoID, err := pathInt64(r, "chamadoId")
	if err != nil {
		jsonBadRequest(w, r, "invalid chamado id", err)
		return
	}
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	chamado, err := models.GetHostChamado(h.db.SQL, chamadoID)
	if err != nil || chamado == nil || chamado.HostID != host.ID {
		jsonError(w, http.StatusNotFound, "chamado not found for this host")
		return
	}

	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	ticketID, err := strconv.Atoi(strings.TrimSpace(chamado.ChamadoID))
	if err != nil || ticketID <= 0 {
		jsonError(w, http.StatusBadRequest, "chamado_id is not a numeric GLPI ticket id")
		return
	}

	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}
	t, err := client.GetTicket(ctx, session, ticketID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+mapGlpiError(err))
		return
	}

	ticketURL := fmt.Sprintf("%s/front/ticket.form.php?id=%d", client.WebBaseURL(), t.ID)
	if err := models.UpdateChamadoCache(h.db.SQL, chamadoID, "glpi", ticketURL, t.Name, glpiclient.StatusSlug(t.Status)); err != nil {
		jsonServerError(w, r, "cache update failed", err)
		return
	}

	jsonOK(w, enrichTicket(client, t))
}

// handleGetTicket returns a single ticket from GLPI. Used by the chamado live-sync
// refresh path. Route:
//   GET /api/glpi/tickets/{id}?profile_id=<N>
func (h *glpiHandlers) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	ticketIDStr := r.PathValue("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil || ticketID <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}
	t, err := client.GetTicket(ctx, session, ticketID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+mapGlpiError(err))
		return
	}
	jsonOK(w, enrichTicket(client, t))
}

// ticketEvent is one entry in the ticket timeline — followup, task, or solution.
// Normalized so the UI can render them in a single chronological list.
type ticketEvent struct {
	Type      string `json:"type"` // "followup" | "task" | "solution"
	ID        int    `json:"id"`
	Content   string `json:"content"`
	Date      string `json:"date"`
	UserID    int    `json:"user_id"`
	UserName  string `json:"user_name,omitempty"`
	IsPrivate bool   `json:"is_private,omitempty"`
	State     int    `json:"state,omitempty"`       // tasks only (0=info 1=todo 2=done)
	Status    int    `json:"status,omitempty"`      // solutions only (1=proposed 2=accepted 3=refused)
}

// handleGetTicketDetails returns the ticket plus its followups, tasks and
// solutions merged into a chronological timeline. Route:
//
//	GET /api/glpi/tickets/{id}/details?profile_id=<N>
func (h *glpiHandlers) handleGetTicketDetails(w http.ResponseWriter, r *http.Request) {
	ticketIDStr := r.PathValue("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil || ticketID <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	t, err := client.GetTicket(ctx, session, ticketID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+mapGlpiError(err))
		return
	}

	// Fan out the three related collections in parallel. Each failure degrades
	// gracefully to an empty slice + warning so the ticket body still renders.
	var (
		followups []glpiclient.Followup
		tasks     []glpiclient.Task
		solutions []glpiclient.Solution
		warnings  []string
		wg        sync.WaitGroup
		mu        sync.Mutex
	)
	addWarn := func(s string) { mu.Lock(); warnings = append(warnings, s); mu.Unlock() }

	wg.Add(3)
	go func() {
		defer wg.Done()
		f, err := client.GetTicketFollowups(ctx, session, ticketID)
		if err != nil {
			addWarn("followups: " + mapGlpiError(err))
			return
		}
		followups = f
	}()
	go func() {
		defer wg.Done()
		ts, err := client.GetTicketTasks(ctx, session, ticketID)
		if err != nil {
			addWarn("tasks: " + mapGlpiError(err))
			return
		}
		tasks = ts
	}()
	go func() {
		defer wg.Done()
		sols, err := client.GetTicketSolutions(ctx, session, ticketID)
		if err != nil {
			addWarn("solutions: " + mapGlpiError(err))
			return
		}
		solutions = sols
	}()
	wg.Wait()

	// Resolve every referenced user id once. Cache to avoid re-fetching the
	// same user across many followups written by the same technician.
	userIDs := map[int]struct{}{}
	if t.UsersIDRequester > 0 {
		userIDs[t.UsersIDRequester] = struct{}{}
	}
	for _, f := range followups {
		if f.UsersID > 0 {
			userIDs[f.UsersID] = struct{}{}
		}
	}
	for _, tk := range tasks {
		if tk.UsersID > 0 {
			userIDs[tk.UsersID] = struct{}{}
		}
		if tk.UsersIDTech > 0 {
			userIDs[tk.UsersIDTech] = struct{}{}
		}
	}
	for _, s := range solutions {
		if s.UsersID > 0 {
			userIDs[s.UsersID] = struct{}{}
		}
	}
	users := map[int]string{}
	for id := range userIDs {
		u, err := client.GetUser(ctx, session, id)
		if err != nil || u == nil {
			continue
		}
		users[id] = u.DisplayName()
	}

	// Build the timeline in chronological order (oldest first, like a chat log).
	events := make([]ticketEvent, 0, len(followups)+len(tasks)+len(solutions))
	for _, f := range followups {
		events = append(events, ticketEvent{
			Type:      "followup",
			ID:        f.ID,
			Content:   f.Content,
			Date:      f.Date,
			UserID:    f.UsersID,
			UserName:  users[f.UsersID],
			IsPrivate: f.IsPrivate == 1,
		})
	}
	for _, tk := range tasks {
		events = append(events, ticketEvent{
			Type:     "task",
			ID:       tk.ID,
			Content:  tk.Content,
			Date:     tk.Date,
			UserID:   firstNonZero(tk.UsersIDTech, tk.UsersID),
			UserName: users[firstNonZero(tk.UsersIDTech, tk.UsersID)],
			State:    tk.State,
		})
	}
	for _, s := range solutions {
		events = append(events, ticketEvent{
			Type:     "solution",
			ID:       s.ID,
			Content:  s.Content,
			Date:     s.BestDate(),
			UserID:   s.UsersID,
			UserName: users[s.UsersID],
			Status:   s.Status,
		})
	}
	// Oldest → newest. Entries with no date are pushed to the end rather than
	// the front so a GLPI Solution missing a timestamp doesn't jump above real
	// follow-ups in the timeline.
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i].Date, events[j].Date
		if a == "" && b == "" {
			return false
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a < b
	})

	out := map[string]any{
		"ticket":        enrichTicket(client, t),
		"glpi_base_url": client.WebBaseURL(),
		"requester":     map[string]any{"id": t.UsersIDRequester, "name": users[t.UsersIDRequester]},
		"events":        events,
		"event_counts":  map[string]int{"followup": len(followups), "task": len(tasks), "solution": len(solutions)},
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	jsonOK(w, out)
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// ─── Formcreator ────────────────────────────────────────────────────────────

// handleListForms returns the Formcreator forms the profile can see.
// GET /api/glpi/forms?profile_id=<N>&q=<optional substring>
func (h *glpiHandlers) handleListForms(w http.ResponseWriter, r *http.Request) {
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	// Use the plain GET (no server-side filter) so GLPI's own visibility rules
	// decide what the profile can see. The /search endpoint with `is_active=1`
	// depends on a search-option field id (8) that's unstable across Formcreator
	// releases — it silently returned 0 rows on at least one real instance. We
	// filter active-ness client-side below, which is version-proof.
	forms, err := client.ListFormcreatorForms(ctx, session, false, 0, 199)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "list failed: "+mapGlpiError(err))
		return
	}
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	if !includeInactive {
		kept := forms[:0]
		for _, f := range forms {
			if f.IsActive == 1 {
				kept = append(kept, f)
			}
		}
		forms = kept
	}
	needle := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if needle != "" {
		kept := forms[:0]
		for _, f := range forms {
			if strings.Contains(strings.ToLower(f.Name), needle) ||
				strings.Contains(strings.ToLower(f.Description), needle) {
				kept = append(kept, f)
			}
		}
		forms = kept
	}
	jsonOK(w, map[string]any{"forms": forms, "count": len(forms)})
}

// handleGetFormBundle returns form + sections + questions + conditions merged
// into a single payload. Fan-out matches handleGetTicketDetails so partial
// failures degrade gracefully to warnings.
// GET /api/glpi/forms/{id}?profile_id=<N>
func (h *glpiHandlers) handleGetFormBundle(w http.ResponseWriter, r *http.Request) {
	formID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || formID <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid form id")
		return
	}
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	// Form metadata is required — fail hard if it doesn't load.
	form, err := client.GetFormcreatorForm(ctx, session, formID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "form lookup failed: "+mapGlpiError(err))
		return
	}

	// Initialize slices so JSON emits [] rather than null when a sub-query
	// fails or returns nothing — the frontend iterates each one directly.
	var (
		sections   = []glpiclient.FormcreatorSection{}
		questions  = []glpiclient.FormcreatorQuestion{}
		conditions = []glpiclient.FormcreatorCondition{}
		warnings   []string
		wg         sync.WaitGroup
		mu         sync.Mutex
	)
	addWarn := func(s string) { mu.Lock(); warnings = append(warnings, s); mu.Unlock() }

	wg.Add(3)
	go func() {
		defer wg.Done()
		out, err := client.ListFormcreatorSectionsByForm(ctx, session, formID)
		if err != nil {
			addWarn("sections: " + mapGlpiError(err))
			return
		}
		sections = out
	}()
	go func() {
		defer wg.Done()
		out, err := client.ListFormcreatorQuestionsByForm(ctx, session, formID)
		if err != nil {
			addWarn("questions: " + mapGlpiError(err))
			return
		}
		questions = out
	}()
	go func() {
		defer wg.Done()
		out, err := client.ListFormcreatorConditionsByForm(ctx, session, formID)
		if err != nil {
			addWarn("conditions: " + mapGlpiError(err))
			return
		}
		conditions = out
	}()
	wg.Wait()

	out := map[string]any{
		"form":          form,
		"sections":      sections,
		"questions":     questions,
		"conditions":    conditions,
		"glpi_base_url": client.WebBaseURL(),
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	jsonOK(w, out)
}

// handleSubmitForm posts answers back to Formcreator. Body:
//
//	{"answers": {"<question_id>": value, ...}}
//
// Values are passed through as-is; arrays for multiselect/checkboxes, strings
// for single-value. The wrapping into `formcreator_field_<id>` keys happens
// inside the client.
func (h *glpiHandlers) handleSubmitForm(w http.ResponseWriter, r *http.Request) {
	formID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || formID <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid form id")
		return
	}
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	var req struct {
		Answers map[string]any `json:"answers"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	res, err := client.SubmitFormcreatorFormAnswer(ctx, session, formID, req.Answers)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "submit failed: "+mapGlpiError(err))
		return
	}

	// Surface the GLPI link so the UI can "Open in GLPI" on success. Ticket
	// enumeration (Formcreator → Target_Ticket → Ticket) is left to Phase 2.
	if res.FormAnswerID > 0 {
		res.URL = fmt.Sprintf("%s/plugins/formcreator/front/formanswer.form.php?id=%d",
			client.WebBaseURL(), res.FormAnswerID)
	}
	jsonCreated(w, res)
}

// handleSearchDropdown powers the Formcreator dropdown/glpiselect/itemtype
// pickers. Itemtype is allow-listed to avoid the endpoint turning into a
// generic GLPI search proxy; extend the map as new form fields need support.
//
//	GET /api/glpi/dropdowns/{itemtype}/search?profile_id=<N>&q=<substring>
func (h *glpiHandlers) handleSearchDropdown(w http.ResponseWriter, r *http.Request) {
	itemtype := r.PathValue("itemtype")
	if !allowedDropdownItemtype(itemtype) {
		jsonError(w, http.StatusBadRequest, "itemtype not allowed")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// Catalogue-first: if an admin has populated a manual catalogue for this
	// itemtype, serve from it — matches the Formcreator "forms rarely change"
	// scenario where GLPI REST is blocked by profile rights.
	if items, ok := h.serveDropdownFromCatalogue(w, r, itemtype, query); ok {
		_ = items
		return
	}

	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}
	raw, err := client.SearchDropdownItems(ctx, session, itemtype, query, 0, 49)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+mapGlpiError(err))
		return
	}
	out := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		id := toInt(row["2"])
		if id <= 0 {
			continue
		}
		out = append(out, map[string]any{
			"id":   id,
			"name": toString(row["1"]),
		})
	}
	jsonOK(w, map[string]any{"items": out, "count": len(out), "source": "rest"})
}

// catalogueOption mirrors the JSON shape admins paste into the catalogue
// editor. Unused optional fields stay zero-valued and are elided by omitempty.
type catalogueOption struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Completename string `json:"completename,omitempty"`
	ParentID     int    `json:"parent_id,omitempty"`
}

// serveDropdownFromCatalogue reads the manual catalogue for an itemtype and,
// if present and non-empty, writes the filtered response and returns ok=true.
// A nil/empty catalogue returns ok=false so the caller falls through to REST.
func (h *glpiHandlers) serveDropdownFromCatalogue(w http.ResponseWriter, r *http.Request, itemtype, query string) ([]map[string]any, bool) {
	cat, err := models.GetGlpiDropdownCatalogue(h.db.SQL, itemtype)
	if err != nil || cat == nil || len(cat.Options) == 0 {
		return nil, false
	}
	var options []catalogueOption
	if err := json.Unmarshal(cat.Options, &options); err != nil || len(options) == 0 {
		return nil, false
	}
	needle := strings.ToLower(query)
	out := make([]map[string]any, 0, len(options))
	for _, o := range options {
		if o.ID <= 0 || o.Name == "" {
			continue
		}
		if needle != "" {
			hay := strings.ToLower(o.Name + " " + o.Completename)
			if !strings.Contains(hay, needle) {
				continue
			}
		}
		row := map[string]any{"id": o.ID, "name": o.Name}
		if o.Completename != "" {
			row["completename"] = o.Completename
		}
		out = append(out, row)
		if len(out) >= 50 {
			break
		}
	}
	jsonOK(w, map[string]any{"items": out, "count": len(out), "source": "catalogue"})
	return out, true
}

// toInt / toString mirror the helpers in the glpi package but live here for
// convenience to avoid exporting them.
func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func allowedDropdownItemtype(t string) bool {
	switch t {
	case "Entity", "ITILCategory", "Location", "Supplier", "Computer",
		"Monitor", "NetworkEquipment", "Printer", "Phone", "User",
		"Group", "Software", "State":
		return true
	}
	return false
}

// handleSearchUsers is a specialised picker for Formcreator's actor question.
// Returns enough to render "Firstname Realname (login)" in the UI.
//
//	GET /api/glpi/users/search?profile_id=<N>&q=<substring>
func (h *glpiHandlers) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// Catalogue-first: User questions can draw from the manual catalogue too.
	// Map {id, name, completename} → {id, login, display} so the response
	// shape stays identical to the REST path.
	if ok := h.serveUsersFromCatalogue(w, query); ok {
		return
	}

	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}
	raw, err := client.SearchUsers(ctx, session, query, 0, 49)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+mapGlpiError(err))
		return
	}
	out := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		id := toInt(row["2"])
		if id <= 0 {
			continue
		}
		first := toString(row["9"])
		real := toString(row["34"])
		display := strings.TrimSpace(first + " " + real)
		if display == "" {
			display = toString(row["1"])
		}
		out = append(out, map[string]any{
			"id":      id,
			"login":   toString(row["1"]),
			"display": display,
			"email":   toString(row["5"]),
		})
	}
	jsonOK(w, map[string]any{"users": out, "count": len(out), "source": "rest"})
}

// serveUsersFromCatalogue checks the manual User catalogue. When populated,
// writes the response and returns true. The shape maps {name → display,
// completename → login} so callers can swap seamlessly between sources.
func (h *glpiHandlers) serveUsersFromCatalogue(w http.ResponseWriter, query string) bool {
	cat, err := models.GetGlpiDropdownCatalogue(h.db.SQL, "User")
	if err != nil || cat == nil || len(cat.Options) == 0 {
		return false
	}
	var options []catalogueOption
	if err := json.Unmarshal(cat.Options, &options); err != nil || len(options) == 0 {
		return false
	}
	needle := strings.ToLower(query)
	out := make([]map[string]any, 0, len(options))
	for _, o := range options {
		if o.ID <= 0 {
			continue
		}
		if needle != "" {
			hay := strings.ToLower(o.Name + " " + o.Completename)
			if !strings.Contains(hay, needle) {
				continue
			}
		}
		row := map[string]any{
			"id":      o.ID,
			"login":   o.Completename,
			"display": o.Name,
		}
		out = append(out, row)
		if len(out) >= 50 {
			break
		}
	}
	jsonOK(w, map[string]any{"users": out, "count": len(out), "source": "catalogue"})
	return true
}

// handleSearchFormcreatorTags surfaces PluginFormcreatorTag rows for the tag
// question type. Supports a substring query so the picker doesn't ship the
// full tag catalogue on every keystroke.
//
//	GET /api/glpi/formcreator/tags/search?profile_id=<N>&q=<substring>
func (h *glpiHandlers) handleSearchFormcreatorTags(w http.ResponseWriter, r *http.Request) {
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	raw, err := client.ListFormcreatorTags(ctx, session, query, 0, 99)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+mapGlpiError(err))
		return
	}
	out := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		id := toInt(row["2"])
		if id <= 0 {
			continue
		}
		out = append(out, map[string]any{
			"id":    id,
			"name":  toString(row["1"]),
			"color": toString(row["3"]),
		})
	}
	jsonOK(w, map[string]any{"tags": out, "count": len(out)})
}

// handleUploadFormDocument accepts a multipart file upload from the frontend,
// forwards it to GLPI's Document endpoint using the active profile's session,
// and returns the newly-created document id. The Formcreator file question
// submission references these ids as its answer value.
//
//	POST /api/glpi/forms/uploads?profile_id=<N>   multipart "file"
func (h *glpiHandlers) handleUploadFormDocument(w http.ResponseWriter, r *http.Request) {
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	// Cap uploads at 50 MiB — matches GLPI's common default and keeps a single
	// form submission bounded.
	const maxUpload = 50 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		jsonBadRequest(w, r, "multipart parse failed", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonBadRequest(w, r, "missing file field", err)
		return
	}
	defer file.Close()

	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	mimeType := header.Header.Get("Content-Type")
	docID, err := client.UploadDocumentMultipart(ctx, session, header.Filename, mimeType, file)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "upload failed: "+mapGlpiError(err))
		return
	}
	jsonCreated(w, map[string]any{
		"id":       docID,
		"filename": header.Filename,
		"mime":     mimeType,
		"size":     header.Size,
	})
}

// handleGetGlpiDocument streams a GLPI Document (image attachment, PDF, etc.)
// through sshcm so the browser doesn't need its own GLPI cookie session.
// Route:
//
//	GET /api/glpi/documents/{id}?profile_id=<N>
//
// Uses the profile's cached session token to authenticate against GLPI and
// forwards Content-Type/Disposition from the upstream response.
func (h *glpiHandlers) handleGetGlpiDocument(w http.ResponseWriter, r *http.Request) {
	docID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || docID <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "profile_id query param is required")
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	// Documents can be large-ish (MB-range) — give the stream a generous window.
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	upstream, err := client.GetDocumentBinary(ctx, session, docID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "fetch failed: "+mapGlpiError(err))
		return
	}
	defer upstream.Body.Close()

	ct := upstream.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	if cd := upstream.Header.Get("Content-Disposition"); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	}
	if cl := upstream.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	// Cache for 5 minutes — the doc content is immutable per id, and repeat
	// renders inside a ticket drawer shouldn't pay the GLPI round-trip.
	w.Header().Set("Cache-Control", "private, max-age=300")
	io.Copy(w, upstream.Body)
}

// handleListProjectTickets returns open tickets for a project, using the
// project's linked profile. Empty profile → empty list.
func (h *glpiHandlers) handleListProjectTickets(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid project id", err)
		return
	}
	project, err := models.GetProject(h.db.SQL, projectID)
	if err != nil || project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	env := projectTicketEnvelope{Tickets: []any{}}
	if project.GlpiTokenID == nil || *project.GlpiTokenID == 0 {
		env.Warning = "no_profile_linked"
		jsonOK(w, env)
		return
	}

	client, _, reason := h.resolveClient()
	if client == nil {
		env.Warning = reason
		jsonOK(w, env)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, *project.GlpiTokenID)
	if err != nil {
		env.Warning = mapGlpiError(err)
		jsonOK(w, env)
		return
	}

	// Build search criteria. Scope by entity (GLPI field 80) if the project set one.
	// Always exclude closed tickets (status field id 12 != 6).
	criteria := []map[string]string{
		{"field": "12", "searchtype": "notequals", "value": "6", "link": "AND"},
	}
	if project.GlpiEntityID > 0 {
		criteria = append(criteria, map[string]string{
			"field": "80", "searchtype": "equals", "value": strconv.Itoa(project.GlpiEntityID), "link": "AND",
		})
	}
	if project.GlpiCategoryID > 0 {
		criteria = append(criteria, map[string]string{
			"field": "7", "searchtype": "equals", "value": strconv.Itoa(project.GlpiCategoryID), "link": "AND",
		})
	}

	tickets, err := client.SearchTickets(ctx, session, criteria, 0, 49)
	if err != nil {
		env.Warning = mapGlpiError(err)
		jsonOK(w, env)
		return
	}

	out := make([]map[string]any, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, enrichTicket(client, &t))
	}
	env.Tickets = out
	jsonOK(w, env)
}

type projectTicketEnvelope struct {
	Tickets any    `json:"tickets"`
	Warning string `json:"warning,omitempty"`
}

// handleListProfileTickets returns every ticket the given profile can see in
// GLPI — bulk visualization. No entity/category scope. Query params:
//
//	status=open (default)  → excludes closed (GLPI status 6)
//	status=all             → includes closed
//	range=0-199 (default)  → GLPI paginates; we expose the same window format
func (h *glpiHandlers) handleListProfileTickets(w http.ResponseWriter, r *http.Request) {
	profileID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid profile id", err)
		return
	}
	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	includeClosed := r.URL.Query().Get("status") == "all"
	rangeParam := r.URL.Query().Get("range")
	start, end := 0, 199
	if rangeParam != "" {
		if s, e, ok := parseRange(rangeParam); ok {
			start, end = s, e
		}
	}

	criteria := []map[string]string{}
	if !includeClosed {
		criteria = append(criteria, map[string]string{
			"field": "12", "searchtype": "notequals", "value": "6", "link": "AND",
		})
	}

	tickets, err := client.SearchTickets(ctx, session, criteria, start, end)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "search failed: "+mapGlpiError(err))
		return
	}
	out := make([]map[string]any, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, enrichTicket(client, &t))
	}
	jsonOK(w, map[string]any{
		"tickets": out,
		"count":   len(out),
		"range":   fmt.Sprintf("%d-%d", start, end),
	})
}

// parseRange handles the "start-end" syntax GLPI itself expects.
func parseRange(s string) (int, int, bool) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	a, err := strconv.Atoi(parts[0])
	if err != nil || a < 0 {
		return 0, 0, false
	}
	b, err := strconv.Atoi(parts[1])
	if err != nil || b < a {
		return 0, 0, false
	}
	if b-a > 499 {
		b = a + 499 // cap per-request window to 500 rows
	}
	return a, b, true
}

// handleListHostTickets tries to resolve a GLPI Computer matching the host's
// oficial_slug, then returns its non-closed tickets. Used by the per-host
// chamado block on the host detail page.
func (h *glpiHandlers) handleListHostTickets(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	// Use the first project's profile, or fallback to the first configured profile.
	// Simpler MVP: require an explicit profile_id query param.
	profileIDStr := r.URL.Query().Get("profile_id")
	if profileIDStr == "" {
		jsonOK(w, map[string]any{"tickets": []any{}, "warning": "profile_id required"})
		return
	}
	profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
	if err != nil || profileID <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid profile_id")
		return
	}

	client, _, reason := h.resolveClient()
	if client == nil {
		jsonError(w, http.StatusServiceUnavailable, reason)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	session, err := h.sessionFor(ctx, client, profileID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "auth failed: "+mapGlpiError(err))
		return
	}

	computer, err := client.FindComputerByName(ctx, session, host.OficialSlug)
	if err != nil {
		jsonOK(w, map[string]any{"tickets": []any{}, "warning": mapGlpiError(err)})
		return
	}
	if computer == nil {
		jsonOK(w, map[string]any{"tickets": []any{}, "computer": nil})
		return
	}

	// GLPI's relation endpoint returns tickets attached to the asset directly.
	var tickets []glpiclient.Ticket
	rel := fmt.Sprintf("/Computer/%d/Ticket", computer.ID)
	if err := client.Get(ctx, session, rel, &tickets); err != nil {
		jsonOK(w, map[string]any{"tickets": []any{}, "computer": computer, "warning": mapGlpiError(err)})
		return
	}

	out := make([]map[string]any, 0, len(tickets))
	for _, t := range tickets {
		if t.Status >= 6 {
			continue
		}
		out = append(out, enrichTicket(client, &t))
	}
	jsonOK(w, map[string]any{"tickets": out, "computer": computer})
}

// ─── Misc helpers ───────────────────────────────────────────────────────────

func enrichTicket(client *glpiclient.Client, t *glpiclient.Ticket) map[string]any {
	return map[string]any{
		"id":           t.ID,
		"name":         t.Name,
		"content":      t.Content,
		"status":       t.Status,
		"status_label": glpiclient.StatusLabel(t.Status),
		"status_slug":  glpiclient.StatusSlug(t.Status),
		"priority":     t.Priority,
		"entities_id":  t.EntitiesID,
		"date":         t.Date,
		"date_mod":     t.DateMod,
		"url":          fmt.Sprintf("%s/front/ticket.form.php?id=%d", client.WebBaseURL(), t.ID),
	}
}

func mapGlpiError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"):
		return "Authentication failed — check App-Token and user token"
	case strings.Contains(msg, "403"):
		// 403 means the session is valid but the active profile lacks read
		// rights on this GLPI itemtype. Point users at the real cause.
		return "Permission denied — this GLPI profile can't read this resource"
	case strings.Contains(msg, "404"):
		return "Endpoint not found — check Base URL (should end in /apirest.php)"
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "connection refused"):
		return "Host unreachable — check Base URL"
	case strings.Contains(msg, "context deadline exceeded"):
		return "Timed out — GLPI took too long"
	}
	return msg
}

