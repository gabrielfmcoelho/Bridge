package api

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type settingsHandlers struct {
	db *database.DB
}

func (h *settingsHandlers) handleGetAppearance(w http.ResponseWriter, r *http.Request) {
	s, err := models.GetAppSettings(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load settings", err)
		return
	}
	jsonOK(w, s)
}

func (h *settingsHandlers) handleUpdateAppearance(w http.ResponseWriter, r *http.Request) {
	var req models.AppSettings
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if req.AppName == "" {
		jsonError(w, http.StatusBadRequest, "app_name is required")
		return
	}
	if req.AppColor == "" {
		req.AppColor = "#06b6d4"
	}
	if err := models.UpdateAppSettings(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to save settings", err)
		return
	}
	jsonOK(w, req)
}

const maxLogoSize = 512 * 1024 // 512KB

func (h *settingsHandlers) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoSize+1024)
	if err := r.ParseMultipartForm(maxLogoSize + 1024); err != nil {
		jsonBadRequest(w, r, "file too large (max 512KB)", err)
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		jsonBadRequest(w, r, "missing logo file", err)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		jsonError(w, http.StatusBadRequest, "file must be an image")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		jsonServerError(w, r, "failed to read file", err)
		return
	}

	dataURI := fmt.Sprintf("data:%s;base64,%s", ct, base64.StdEncoding.EncodeToString(data))

	s, err := models.GetAppSettings(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load settings", err)
		return
	}
	s.AppLogo = dataURI
	if err := models.UpdateAppSettings(h.db.SQL, s); err != nil {
		jsonServerError(w, r, "failed to save logo", err)
		return
	}

	jsonOK(w, map[string]string{"logo": dataURI})
}

func (h *settingsHandlers) handleGetAlertThresholds(w http.ResponseWriter, r *http.Request) {
	t, err := models.GetAlertThresholds(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load alert thresholds", err)
		return
	}
	jsonOK(w, t)
}

func (h *settingsHandlers) handleUpdateAlertThresholds(w http.ResponseWriter, r *http.Request) {
	var req models.AlertThresholds
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if req.ResourceCritical < 0 || req.ResourceCritical > 100 ||
		req.ResourceWarning < 0 || req.ResourceWarning > 100 ||
		req.ResourceInfoLow < 0 || req.ResourceInfoLow > 100 {
		jsonError(w, http.StatusBadRequest, "threshold values must be between 0 and 100")
		return
	}
	if err := models.UpdateAlertThresholds(h.db.SQL, &req); err != nil {
		jsonServerError(w, r, "failed to save alert thresholds", err)
		return
	}
	jsonOK(w, req)
}

func (h *settingsHandlers) handleDeleteLogo(w http.ResponseWriter, r *http.Request) {
	s, err := models.GetAppSettings(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load settings", err)
		return
	}
	s.AppLogo = ""
	if err := models.UpdateAppSettings(h.db.SQL, s); err != nil {
		jsonServerError(w, r, "failed to remove logo", err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
