package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type enumHandlers struct {
	enum *store.EnumOptionRepo
}

// handleList godoc
//
//	@Summary		List a category's options
//	@Description	Any role.
//	@Tags			enums
//	@Produce		json
//	@Param			category	path		string	true	"Enum category"
//	@Param			page		query		int		false	"Page (1-based)"
//	@Param			per_page	query		int		false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.EnumOption]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/enums/{category} [get]
func (h *enumHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	options, err := h.enum.List(r.Context(), r.PathValue("category"))
	if err != nil {
		jsonServerError(w, r, "failed to list options", err)
		return
	}
	jsonPaged(w, r, options)
}

// handleListAll godoc
//
//	@Summary		List every enum option
//	@Description	Any role. Options grouped by category.
//	@Tags			enums
//	@Produce		json
//	@Success		200	{object}	map[string][]models.EnumOption
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/enums [get]
func (h *enumHandlers) handleListAll(w http.ResponseWriter, r *http.Request) {
	options, err := h.enum.ListAll(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list options", err)
		return
	}
	jsonOK(w, options)
}

// enumCreateRequest is a new option; color is an optional #rrggbb hex.
type enumCreateRequest struct {
	Value     string `json:"value"`
	SortOrder int    `json:"sort_order"`
	Color     string `json:"color"`
}

// handleCreate godoc
//
//	@Summary		Create an enum option
//	@Description	Admin. value is required; color, when set, must be #rrggbb.
//	@Tags			enums
//	@Accept			json
//	@Produce		json
//	@Param			category	path		string				true	"Enum category"
//	@Param			body		body		enumCreateRequest	true	"Option"
//	@Success		201			{object}	models.EnumOption
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/enums/{category} [post]
func (h *enumHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	var req enumCreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Value == "" {
		jsonBadRequest(w, r, "value is required", nil)
		return
	}
	req.Color = strings.TrimSpace(req.Color)
	if req.Color != "" && !hexColorPattern.MatchString(req.Color) {
		jsonBadRequest(w, r, "color must be a hex value like #10b981", nil)
		return
	}
	o := &models.EnumOption{Category: category, Value: req.Value, SortOrder: req.SortOrder, Color: req.Color}
	if err := h.enum.Create(r.Context(), o); err != nil {
		jsonServerError(w, r, "failed to create option", err)
		return
	}
	jsonCreated(w, o)
}

// enumUpdateRequest renames/recolors an option; color is an optional #rrggbb hex.
type enumUpdateRequest struct {
	Value string `json:"value"`
	Color string `json:"color"`
}

// handleUpdate godoc
//
//	@Summary		Update an enum option
//	@Description	Admin. Renames and/or recolors the option; value is required, color when set must be #rrggbb.
//	@Tags			enums
//	@Accept			json
//	@Produce		json
//	@Param			category	path		string				true	"Enum category"
//	@Param			value		path		string				true	"Current option value"
//	@Param			body		body		enumUpdateRequest	true	"New value and color"
//	@Success		200			{object}	StatusResponse
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/enums/{category}/{value} [put]
func (h *enumHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	oldValue := r.PathValue("value")
	var req enumUpdateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Value == "" {
		jsonBadRequest(w, r, "value is required", nil)
		return
	}
	req.Color = strings.TrimSpace(req.Color)
	if req.Color != "" && !hexColorPattern.MatchString(req.Color) {
		jsonBadRequest(w, r, "color must be a hex value like #10b981", nil)
		return
	}
	if err := h.enum.Update(r.Context(), category, oldValue, req.Value, req.Color); err != nil {
		jsonServerError(w, r, "failed to update option", err)
		return
	}
	jsonOK(w, map[string]string{"status": "updated"})
}

// handleDelete godoc
//
//	@Summary		Delete an enum option
//	@Description	Admin.
//	@Tags			enums
//	@Produce		json
//	@Param			category	path		string	true	"Enum category"
//	@Param			value		path		string	true	"Current option value"
//	@Success		200			{object}	StatusResponse
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/enums/{category}/{value} [delete]
func (h *enumHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.enum.Delete(r.Context(), r.PathValue("category"), r.PathValue("value")); err != nil {
		jsonServerError(w, r, "failed to delete option", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *enumHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/enums", h.handleListAll)
	rr.auth("GET /api/enums/{category}", h.handleList)
	rr.role("admin", "POST /api/enums/{category}", h.handleCreate)
	rr.role("admin", "PUT /api/enums/{category}/{value}", h.handleUpdate)
	rr.role("admin", "DELETE /api/enums/{category}/{value}", h.handleDelete)
}
