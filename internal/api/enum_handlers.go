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
		jsonError(w, http.StatusInternalServerError, "failed to list options")
		return
	}
	jsonOK(w, options)
}

func (h *enumHandlers) handleListAll(w http.ResponseWriter, r *http.Request) {
	options, err := models.ListAllEnumOptions(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list options")
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
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Value == "" {
		jsonError(w, http.StatusBadRequest, "value is required")
		return
	}
	req.Color = strings.TrimSpace(req.Color)
	if req.Color != "" && !hexColorPattern.MatchString(req.Color) {
		jsonError(w, http.StatusBadRequest, "color must be a hex value like #10b981")
		return
	}

	o := &models.EnumOption{
		Category:  category,
		Value:     req.Value,
		SortOrder: req.SortOrder,
		Color:     req.Color,
	}
	if err := models.CreateEnumOption(h.db.SQL, o); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create option")
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
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Value == "" {
		jsonError(w, http.StatusBadRequest, "value is required")
		return
	}
	req.Color = strings.TrimSpace(req.Color)
	if req.Color != "" && !hexColorPattern.MatchString(req.Color) {
		jsonError(w, http.StatusBadRequest, "color must be a hex value like #10b981")
		return
	}

	if err := models.UpdateEnumOption(h.db.SQL, category, oldValue, req.Value, req.Color); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update option")
		return
	}
	jsonOK(w, map[string]string{"status": "updated"})
}

func (h *enumHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	value := r.PathValue("value")

	if err := models.DeleteEnumOption(h.db.SQL, category, value); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete option")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
