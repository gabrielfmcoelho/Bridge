package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// hostHandlers retains db for the write paths (create/update with vault
// dual-writes, SSH-key resealing, Grafana provisioning) which are not yet
// lifted into the service. Read paths (list/get) delegate to host.
type hostHandlers struct {
	host *service.HostService
	db   *database.DB
}

func (h *hostHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	f := models.HostFilter{
		Situacao:            r.URL.Query().Get("situacao"),
		Tag:                 r.URL.Query().Get("tag"),
		Hospedagem:          r.URL.Query().Get("hospedagem"),
		Search:              r.URL.Query().Get("search"),
		EntidadeResponsavel: r.URL.Query().Get("entidade_responsavel"),
		ResponsavelInterno:  r.URL.Query().Get("responsavel_interno"),
		KeyTestStatus:       r.URL.Query().Get("key_test_status"),
		PasswordTestStatus:  r.URL.Query().Get("password_test_status"),
		ScanResult:          r.URL.Query().Get("scan_result"),
		HasScan:             r.URL.Query().Get("has_scan"),
		SortBy:              r.URL.Query().Get("sort_by"),
		SortDir:             r.URL.Query().Get("sort_dir"),
	}
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			f.Page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 && v <= 100 {
			f.PerPage = v
		}
	}

	result, err := h.host.List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list hosts", err)
		return
	}

	// Post-enrichment filter: alert_level (operates on the computed alerts).
	if alertLevelFilter := r.URL.Query().Get("alert_level"); alertLevelFilter != "" {
		filtered := result[:0]
		for _, hwt := range result {
			if alertLevelFilter == "none" {
				if len(hwt.Alerts) == 0 {
					filtered = append(filtered, hwt)
				}
			} else {
				for _, a := range hwt.Alerts {
					if a.Level == alertLevelFilter {
						filtered = append(filtered, hwt)
						break
					}
				}
			}
		}
		result = filtered
	}

	// Post-enrichment filter: idle. The idle flag comes from the HostProfile
	// heuristic embedded in scan_data, so it runs after enrichment. "active"
	// keeps only hosts that have scan data AND were classified not-idle; hosts
	// with no scan are excluded (we can't tell either way).
	if idleFilter := r.URL.Query().Get("idle"); idleFilter == "idle" || idleFilter == "active" {
		filtered := result[:0]
		for _, hwt := range result {
			if idleFilter == "idle" && hwt.Idle {
				filtered = append(filtered, hwt)
			} else if idleFilter == "active" && hwt.HasScan && !hwt.Idle {
				filtered = append(filtered, hwt)
			}
		}
		result = filtered
	}

	// If paginated, wrap in an envelope with the total count.
	if f.PerPage > 0 {
		total, _ := h.host.Count(r.Context(), f)
		totalPages := (total + f.PerPage - 1) / f.PerPage
		page := f.Page
		if page < 1 {
			page = 1
		}
		jsonOK(w, map[string]any{
			"data":        result,
			"total":       total,
			"page":        page,
			"per_page":    f.PerPage,
			"total_pages": totalPages,
		})
		return
	}

	jsonOK(w, result)
}

func (h *hostHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	detail, err := h.host.Get(r.Context(), slug)
	if err != nil {
		jsonServerError(w, r, "database error", err)
		return
	}
	if detail == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	jsonOK(w, detail)
}

func (h *hostHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Host
		Tags         []string                      `json:"tags"`
		Password     string                        `json:"password"`
		SSHKeyID     int64                         `json:"ssh_key_id"`
		Responsaveis []models.ResponsavelInput `json:"responsaveis"`
		Chamados     []models.HostChamadoInput     `json:"chamados"`
		Entidades    []models.HostEntidadeInput    `json:"entidades"`
		DNSIDs       []int64                       `json:"dns_ids"`
		ServiceIDs   []int64                       `json:"service_ids"`
		ProjectIDs   []int64                       `json:"project_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Nickname == "" || req.OficialSlug == "" {
		jsonError(w, http.StatusBadRequest, "nickname and oficial_slug are required")
		return
	}

	exists, _ := store.NewHostRepo(h.db.SQL).SlugExists(r.Context(), req.OficialSlug, 0)
	if exists {
		jsonError(w, http.StatusConflict, "slug already exists")
		return
	}

	// Flag derivation (transport-input shaping). Key material is attached via
	// the vault by the service's SSH-key link — callers can't set a filesystem
	// path; has_password is a cached flag for the vault payload.
	req.Host.KeyPath = ""
	req.Host.HasKey = false
	req.Host.HasPassword = req.Password != ""
	preferredAuth, prefErr := normalizePreferredAuth(req.Host.HasPassword, req.Host.HasKey || req.SSHKeyID > 0, req.Host.PreferredAuth)
	if prefErr != nil {
		jsonError(w, http.StatusBadRequest, prefErr.Error())
		return
	}
	req.Host.PreferredAuth = preferredAuth

	w2 := &service.HostWrite{
		Host:         req.Host,
		Password:     req.Password,
		SSHKeyID:     req.SSHKeyID,
		Tags:         &req.Tags,
		Responsaveis: &req.Responsaveis,
		Chamados:     &req.Chamados,
		Entidades:    &req.Entidades,
		DNSIDs:       &req.DNSIDs,
		ServiceIDs:   &req.ServiceIDs,
		ProjectIDs:   &req.ProjectIDs,
	}
	host, err := h.host.Create(r.Context(), actorID(r), w2)
	if errors.Is(err, service.ErrHostKeyLink) {
		jsonServerError(w, r, "host created but "+err.Error(), err)
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to create host", err)
		return
	}

	// Fire-and-forget: auto-provision a default Grafana dashboard if enabled.
	h.maybeProvisionGrafanaDashboard(*host)

	jsonCreated(w, *host)
}

// maybeProvisionGrafanaDashboard runs ProvisionHostDashboard in a goroutine if
// the integration is enabled. Keep the call site cheap — a couple of DB reads
// worst case when it's disabled.
func (h *hostHandlers) maybeProvisionGrafanaDashboard(host models.Host) {
	if store.NewAppSettingsRepo(h.db.SQL).Value(context.Background(), "grafana_enabled") != "true" {
		return
	}
	go func(host models.Host) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("[grafana-provision] host %d panic: %v", host.ID, rv)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := ProvisionHostDashboard(ctx, h.db, &host); err != nil {
			log.Printf("[grafana-provision] host %d (%s): %v", host.ID, host.OficialSlug, err)
		}
	}(host)
}

func (h *hostHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	existing, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	var req struct {
		models.Host
		Tags         []string                       `json:"tags"`
		Password     string                         `json:"password"`
		SSHKeyID     int64                          `json:"ssh_key_id"`
		ClearKey     bool                           `json:"clear_key"`
		Responsaveis *[]models.ResponsavelInput `json:"responsaveis"`
		Chamados     *[]models.HostChamadoInput     `json:"chamados"`
		Entidades    *[]models.HostEntidadeInput    `json:"entidades"`
		DNSIDs       *[]int64                       `json:"dns_ids"`
		ServiceIDs   *[]int64                       `json:"service_ids"`
		ProjectIDs   *[]int64                       `json:"project_ids"`
	}
	req.Host = *existing
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	// If only ssh_key_id is provided (no host data), just link the key and return.
	if req.SSHKeyID > 0 && req.Nickname == "" {
		if linkErr := h.host.LinkSSHKey(r.Context(), existing.ID, req.SSHKeyID, existing.OficialSlug); linkErr != nil {
			jsonServerError(w, r, "failed to link ssh key: "+linkErr.Error(), linkErr)
			return
		}
		updatedHost, getErr := store.NewHostRepo(h.db.SQL).GetByID(r.Context(), existing.ID)
		if getErr == nil && updatedHost != nil {
			preferredAuth, normErr := normalizePreferredAuth(updatedHost.HasPassword, updatedHost.HasKey, updatedHost.PreferredAuth)
			if normErr == nil && preferredAuth != updatedHost.PreferredAuth {
				updatedHost.PreferredAuth = preferredAuth
				_ = store.NewHostRepo(h.db.SQL).Update(r.Context(), updatedHost)
			}
		}
		jsonOK(w, existing)
		return
	}

	req.Host.ID = existing.ID
	if req.Host.OficialSlug == "" {
		req.Host.OficialSlug = existing.OficialSlug
	}
	req.Host.OficialSlug = strings.TrimSpace(req.Host.OficialSlug)
	if req.Host.OficialSlug == "" {
		jsonError(w, http.StatusBadRequest, "oficial_slug is required")
		return
	}
	if req.Host.OficialSlug != existing.OficialSlug {
		exists, slugErr := store.NewHostRepo(h.db.SQL).SlugExists(r.Context(), req.Host.OficialSlug, existing.ID)
		if slugErr != nil {
			jsonServerError(w, r, "failed to validate slug", slugErr)
			return
		}
		if exists {
			jsonError(w, http.StatusConflict, "slug already exists")
			return
		}
	}

	// Flag updates. Actual payloads live in the vault — the post-Update
	// dual-write below calls vault.HostSetPassword / HostSetSSHKey when
	// the request changes them. Carrying-forward an unchanged secret
	// requires no action because the vault row already holds it.
	if req.Password != "" {
		req.Host.HasPassword = true
	} else {
		req.Host.HasPassword = existing.HasPassword
	}

	if req.ClearKey {
		req.Host.HasKey = false
		req.Host.KeyPath = ""
		req.Host.IdentitiesOnly = ""
	} else {
		req.Host.HasKey = existing.HasKey
		req.Host.KeyPath = existing.KeyPath
		req.Host.IdentitiesOnly = existing.IdentitiesOnly
	}

	if req.Host.PreferredAuth == "" {
		req.Host.PreferredAuth = existing.PreferredAuth
	}
	preferredAuth, prefErr := normalizePreferredAuth(req.Host.HasPassword, req.Host.HasKey || req.SSHKeyID > 0, req.Host.PreferredAuth)
	if prefErr != nil {
		jsonError(w, http.StatusBadRequest, prefErr.Error())
		return
	}
	req.Host.PreferredAuth = preferredAuth

	w2 := &service.HostWrite{
		Host:         req.Host,
		Password:     req.Password,
		SSHKeyID:     req.SSHKeyID,
		ClearKey:     req.ClearKey,
		Responsaveis: req.Responsaveis,
		Chamados:     req.Chamados,
		Entidades:    req.Entidades,
		DNSIDs:       req.DNSIDs,
		ServiceIDs:   req.ServiceIDs,
		ProjectIDs:   req.ProjectIDs,
	}
	// Tags is a non-pointer slice here; apply only when present in the request.
	if req.Tags != nil {
		w2.Tags = &req.Tags
	}
	host, err := h.host.Update(r.Context(), actorID(r), w2)
	if errors.Is(err, service.ErrHostKeyLink) {
		jsonServerError(w, r, "host updated but "+err.Error(), err)
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to update host", err)
		return
	}

	jsonOK(w, *host)
}

func normalizePreferredAuth(hasPassword, hasKey bool, preferredAuth string) (string, error) {
	switch {
	case hasPassword && hasKey:
		if preferredAuth != "password" && preferredAuth != "key" {
			return "", fmt.Errorf("preferred_auth must be 'password' or 'key' when both auth methods are configured")
		}
		return preferredAuth, nil
	case hasPassword:
		return "password", nil
	case hasKey:
		return "key", nil
	default:
		return "", nil
	}
}

func (h *hostHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	actor, _ := actorFrom(r)
	if err := h.host.Delete(r.Context(), actor, host.ID); err != nil {
		jsonServerError(w, r, "failed to delete host", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *hostHandlers) handleGetPassword(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := store.NewHostRepo(h.db.SQL).GetBySlug(r.Context(), slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	password, ok, err := vault.HostGetPassword(r.Context(), h.db, host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to load host password", err)
		return
	}
	if !ok {
		jsonError(w, http.StatusNotFound, "no password stored")
		return
	}
	jsonOK(w, map[string]string{"password": password})
}

// resolveHostKeyPEM returns the decrypted private-key PEM for a host, read
// directly from the encrypted blob stored on the host row. The filesystem is
// never touched — this is what keeps key-auth working after a DB backup is
// restored onto a machine where host.key_path no longer exists.
func resolveHostKeyPEM(db *database.DB, host *models.Host) ([]byte, error) {
	if host == nil {
		return nil, fmt.Errorf("host has no stored private key — link an SSH key via the host editor")
	}
	key, ok, err := vault.HostGetSSHKey(context.Background(), db, host.ID)
	if err != nil {
		return nil, fmt.Errorf("load host key: %w", err)
	}
	if !ok || key.PrivateKeyPEM == "" {
		return nil, fmt.Errorf("host has no stored private key — link an SSH key via the host editor")
	}
	return []byte(key.PrivateKeyPEM), nil
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *hostHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/hosts", h.handleList)
	rr.role("editor", "POST /api/hosts", h.handleCreate)
	rr.auth("GET /api/hosts/{slug}", h.handleGet)
	rr.role("editor", "PUT /api/hosts/{slug}", h.handleUpdate)
	rr.role("admin", "DELETE /api/hosts/{slug}", h.handleDelete)
	rr.role("admin", "GET /api/hosts/{slug}/password", h.handleGetPassword)
}
