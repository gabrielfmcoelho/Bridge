package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type enumHandlers struct {
	db *database.DB
}

func (h *enumHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	options, err := models.ListEnumOptions(h.db.SQL, category)
	if err != nil {
		jsonServerError(w, r, "failed to list options", err)
		return
	}
	jsonOK(w, options)
}

func (h *enumHandlers) handleListAll(w http.ResponseWriter, r *http.Request) {
	options, err := models.ListAllEnumOptions(h.db.SQL)
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
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
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

	o := &models.EnumOption{
		Category:  category,
		Value:     req.Value,
		SortOrder: req.SortOrder,
		Color:     req.Color,
	}
	if err := models.CreateEnumOption(h.db.SQL, o); err != nil {
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
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
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

	if err := models.UpdateEnumOption(h.db.SQL, category, oldValue, req.Value, req.Color); err != nil {
		jsonServerError(w, r, "failed to update option", err)
		return
	}
	jsonOK(w, map[string]string{"status": "updated"})
}

func (h *enumHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	value := r.PathValue("value")

	if err := models.DeleteEnumOption(h.db.SQL, category, value); err != nil {
		jsonServerError(w, r, "failed to delete option", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
