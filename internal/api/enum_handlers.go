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

func (h *enumHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	options, err := h.enum.List(r.Context(), r.PathValue("category"))
	if err != nil {
		jsonServerError(w, r, "failed to list options", err)
		return
	}
	jsonOK(w, options)
}

func (h *enumHandlers) handleListAll(w http.ResponseWriter, r *http.Request) {
	options, err := h.enum.ListAll(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list options", err)
		return
	}
	jsonOK(w, options)
}

func (h *enumHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	var req struct {
		Value     string `json:"value"`
		SortOrder int    `json:"sort_order"`
		Color     string `json:"color"`
	}
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

func (h *enumHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	oldValue := r.PathValue("value")
	var req struct {
		Value string `json:"value"`
		Color string `json:"color"`
	}
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

func (h *enumHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.enum.Delete(r.Context(), r.PathValue("category"), r.PathValue("value")); err != nil {
		jsonServerError(w, r, "failed to delete option", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
