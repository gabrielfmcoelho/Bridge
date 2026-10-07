package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// maxCanvasBytes caps a canvas body (the whole board's JSON on every save).
const maxCanvasBytes = 2 << 20

// canvasHandlers serves the entidade idea boards. A canvas is visible iff its
// entidade is in the caller's visible set; invisible is always 404.
type canvasHandlers struct {
	canvases *store.CanvasRepo
}

type canvasCreateRequest struct {
	EntidadeID int64  `json:"entidade_id"`
	Title      string `json:"title"`
}

// canvasUpdateRequest is partial: nil keeps the field. Version is the one the
// caller loaded; a stale version answers 409.
type canvasUpdateRequest struct {
	Version int              `json:"version"`
	Title   *string          `json:"title"`
	Content *json.RawMessage `json:"content"`
}

// handleList godoc
//
//	@Summary		List canvases
//	@Description	Visible idea boards (without content), latest edit first. Any role.
//	@Tags			canvases
//	@Produce		json
//	@Param			entidade_id	query		int	false	"Only this entidade's boards"
//	@Success		200			{object}	ListEnvelope[models.Canvas]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/canvases [get]
func (h *canvasHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	var entidadeID int64
	if v := r.URL.Query().Get("entidade_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			jsonBadRequest(w, r, "invalid entidade_id", err)
			return
		}
		entidadeID = n
	}
	list, err := h.canvases.List(r.Context(), entidadeID)
	if err != nil {
		jsonServerError(w, r, "failed to list canvases", err)
		return
	}
	jsonPaged(w, r, list)
}

// handleGet godoc
//
//	@Summary		Get a canvas
//	@Description	The board with its content (xyflow JSON object). Any role.
//	@Tags			canvases
//	@Produce		json
//	@Param			id	path		int	true	"Canvas ID"
//	@Success		200	{object}	models.Canvas
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/canvases/{id} [get]
func (h *canvasHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.canvases.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load canvas", err)
		return
	}
	if c == nil {
		jsonError(w, http.StatusNotFound, "canvas not found")
		return
	}
	jsonOK(w, c)
}

// handleCreate godoc
//
//	@Summary		Create a canvas
//	@Description	Editor+. An empty board for an entidade the caller can see (404 otherwise).
//	@Tags			canvases
//	@Accept			json
//	@Produce		json
//	@Param			body	body		canvasCreateRequest	true	"Entidade and title"
//	@Success		201		{object}	models.Canvas
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/canvases [post]
func (h *canvasHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req canvasCreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if !requireFields(w, map[string]string{"title": req.Title}) {
		return
	}
	if s, _ := store.ScopeFrom(r.Context()); !s.Allows(req.EntidadeID) {
		jsonError(w, http.StatusNotFound, "entidade not found")
		return
	}
	c := &models.Canvas{EntidadeID: req.EntidadeID, Title: req.Title}
	if u := auth.UserFromContext(r.Context()); u != nil {
		c.CreatedBy = &u.ID
	}
	if err := h.canvases.Create(r.Context(), c); err != nil {
		// Admin with a nonexistent entidade_id lands here (FK violation).
		jsonErrorLogged(w, r, http.StatusNotFound, "entidade not found", err)
		return
	}
	created, err := h.canvases.Get(r.Context(), c.ID)
	if err != nil || created == nil {
		jsonServerError(w, r, "failed to load canvas", err)
		return
	}
	jsonCreated(w, created)
}

// handleUpdate godoc
//
//	@Summary		Update a canvas
//	@Description	Editor+. Partial: omitted title/content keep their value. version must be the one loaded; a stale one answers 409 with the current canvas.
//	@Tags			canvases
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Canvas ID"
//	@Param			body	body		canvasUpdateRequest	true	"Version and changed fields"
//	@Success		200		{object}	models.Canvas
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	models.Canvas
//	@Router			/api/canvases/{id} [put]
func (h *canvasHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCanvasBytes)
	var req canvasUpdateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			jsonError(w, http.StatusBadRequest, "title is required")
			return
		}
		req.Title = &t
	}
	var content *string
	if req.Content != nil {
		// The board is an object ({nodes, edges}); the server never looks inside.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(*req.Content, &obj); err != nil {
			jsonError(w, http.StatusBadRequest, "content must be a JSON object")
			return
		}
		s := string(*req.Content)
		content = &s
	}
	_, err := h.canvases.Update(r.Context(), id, req.Version, req.Title, content)
	if errors.Is(err, store.ErrVersionConflict) {
		cur, _ := h.canvases.Get(r.Context(), id)
		httpx.WriteJSON(w, http.StatusConflict, cur)
		return
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "canvas not found")
			return
		}
		jsonServerError(w, r, "failed to update canvas", err)
		return
	}
	c, err := h.canvases.Get(r.Context(), id)
	if err != nil || c == nil {
		jsonServerError(w, r, "failed to load canvas", err)
		return
	}
	c.Content = nil // the caller already holds it; keep autosave responses small
	jsonOK(w, c)
}

// handleDelete godoc
//
//	@Summary		Delete a canvas
//	@Description	Editor+. Issues the board sent to the backlog stay.
//	@Tags			canvases
//	@Param			id	path	int	true	"Canvas ID"
//	@Success		204
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/canvases/{id} [delete]
func (h *canvasHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if c, err := h.canvases.Get(r.Context(), id); err != nil || c == nil {
		jsonError(w, http.StatusNotFound, "canvas not found")
		return
	}
	if err := h.canvases.Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "failed to delete canvas", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *canvasHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/canvases", h.handleList)
	rr.auth("GET /api/canvases/{id}", h.handleGet)
	rr.role("editor", "POST /api/canvases", h.handleCreate)
	rr.role("editor", "PUT /api/canvases/{id}", h.handleUpdate)
	rr.role("editor", "DELETE /api/canvases/{id}", h.handleDelete)
}
