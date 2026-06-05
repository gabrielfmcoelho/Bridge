package api

import (
	"context"
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
		Responsaveis []models.HostResponsavelInput `json:"responsaveis"`
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

	// Key material is attached via linkSSHKey (post-create) which writes
	// to the unified vault. Callers cannot set a filesystem path on the
	// host row — the vault is the source of truth so backups restore
	// cleanly onto any machine.
	req.Host.KeyPath = ""
	req.Host.HasKey = false

	// has_password is a cached flag; the actual password payload is
	// stored in the vault by the post-CreateHost dual-write below.
	req.Host.HasPassword = req.Password != ""

	preferredAuth, prefErr := normalizePreferredAuth(req.Host.HasPassword, req.Host.HasKey || req.SSHKeyID > 0, req.Host.PreferredAuth)
	if prefErr != nil {
		jsonError(w, http.StatusBadRequest, prefErr.Error())
		return
	}
	req.Host.PreferredAuth = preferredAuth

	if err := store.NewHostRepo(h.db.SQL).Create(r.Context(), &req.Host); err != nil {
		jsonServerError(w, r, "failed to create host", err)
		return
	}

	// Stage 1 dual-write: mirror the password to the unified vault so
	// SSH/Coolify/import handlers reading via vault.HostGetPassword see
	// it. Stage 2 will drop the column-side write and the host row will
	// only carry the has_password boolean.
	if req.Password != "" {
		actorID := actorID(r)
		if err := vault.HostSetPassword(r.Context(), h.db, req.Host.ID, actorID, req.Password); err != nil {
			log.Printf("[hosts] vault dual-write on create slug=%s: %v", req.OficialSlug, err)
		}
	}

	if len(req.Tags) > 0 {
		store.NewTagRepo(h.db.SQL).Set(r.Context(), "host", req.Host.ID, req.Tags)
	}

	// Sync responsaveis and chamados
	if len(req.Responsaveis) > 0 {
		if err := models.SyncHostResponsaveis(h.db.SQL, req.Host.ID, req.Responsaveis); err != nil {
			log.Printf("[hosts] SyncHostResponsaveis error on create: %v", err)
		}
	}
	if len(req.Chamados) > 0 {
		if err := store.NewHostChamadoRepo(h.db.SQL).Sync(r.Context(), req.Host.ID, req.Chamados); err != nil {
			log.Printf("[hosts] SyncHostChamados error on create: %v", err)
		}
	}
	if len(req.Entidades) > 0 {
		if err := store.NewHostEntidadeRepo(h.db.SQL).Sync(r.Context(), req.Host.ID, req.Entidades); err != nil {
			log.Printf("[hosts] SyncHostEntidades error on create: %v", err)
		}
	}

	// Link SSH key from DB if provided. A failure here doesn't abort host
	// creation (the host row already exists at this point) but it MUST be
	// surfaced to the client so the user isn't misled into thinking the key
	// was attached. See linkSSHKey for the silent-failure modes this guards.
	if req.SSHKeyID > 0 {
		if linkErr := h.linkSSHKey(req.Host.ID, req.SSHKeyID, req.OficialSlug); linkErr != nil {
			jsonServerError(w, r, "host created but ssh key link failed: "+linkErr.Error(), linkErr)
			return
		}
	}

	// Link DNS records and services if provided.
	if len(req.DNSIDs) > 0 {
		if err := store.NewDNSRepo(h.db.SQL).SetLinksForHost(r.Context(), req.Host.ID, req.DNSIDs); err != nil {
			log.Printf("[hosts] SetHostDNSLinks error on create: %v", err)
		}
	}
	if len(req.ServiceIDs) > 0 {
		if err := store.NewServiceRepo(h.db.SQL).SetServicesForHost(r.Context(), req.Host.ID, req.ServiceIDs); err != nil {
			log.Printf("[hosts] SetServicesForHost error on create: %v", err)
		}
	}
	if len(req.ProjectIDs) > 0 {
		if err := store.NewProjectRepo(h.db.SQL).SetProjectsForHost(r.Context(), req.Host.ID, req.ProjectIDs); err != nil {
			log.Printf("[hosts] SetProjectsForHost error on create: %v", err)
		}
	}

	// Fire-and-forget: if Grafana integration is fully configured, auto-provision
	// a default dashboard for this new host in the background. Failures just log —
	// we never block host creation on Grafana being reachable.
	h.maybeProvisionGrafanaDashboard(req.Host)

	jsonCreated(w, req.Host)
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
		Responsaveis *[]models.HostResponsavelInput `json:"responsaveis"`
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
		if linkErr := h.linkSSHKey(existing.ID, req.SSHKeyID, existing.OficialSlug); linkErr != nil {
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

	if err := store.NewHostRepo(h.db.SQL).Update(r.Context(), &req.Host); err != nil {
		jsonServerError(w, r, "failed to update host", err)
		return
	}

	// Stage 1 dual-write: keep the vault row in sync. Password rotation
	// upserts; clearing the key removes the vault row. SSH key linking
	// (req.SSHKeyID > 0) flows through linkSSHKey which writes to vault
	// inside that helper.
	actorID := actorID(r)
	if req.Password != "" {
		if err := vault.HostSetPassword(r.Context(), h.db, existing.ID, actorID, req.Password); err != nil {
			log.Printf("[hosts] vault dual-write on update slug=%s: %v", existing.OficialSlug, err)
		}
	}
	if req.ClearKey {
		if err := vault.HostSetSSHKey(r.Context(), h.db, existing.ID, actorID, vault.HostSSHKey{}); err != nil {
			log.Printf("[hosts] vault clear-key on update slug=%s: %v", existing.OficialSlug, err)
		}
	}

	if req.Tags != nil {
		store.NewTagRepo(h.db.SQL).Set(r.Context(), "host", existing.ID, req.Tags)
	}

	// Sync responsaveis and chamados if provided
	if req.Responsaveis != nil {
		if err := models.SyncHostResponsaveis(h.db.SQL, existing.ID, *req.Responsaveis); err != nil {
			log.Printf("[hosts] SyncHostResponsaveis error on update: %v", err)
		}
	}
	if req.Chamados != nil {
		if err := store.NewHostChamadoRepo(h.db.SQL).Sync(r.Context(), existing.ID, *req.Chamados); err != nil {
			log.Printf("[hosts] SyncHostChamados error on update: %v", err)
		}
	}
	if req.Entidades != nil {
		if err := store.NewHostEntidadeRepo(h.db.SQL).Sync(r.Context(), existing.ID, *req.Entidades); err != nil {
			log.Printf("[hosts] SyncHostEntidades error on update: %v", err)
		}
	}

	// Link SSH key from DB if provided. Host was already updated above with
	// the preserved key data, so this will overwrite with the selected key.
	// Failure must be surfaced so the client knows the link didn't stick.
	if req.SSHKeyID > 0 {
		if linkErr := h.linkSSHKey(existing.ID, req.SSHKeyID, existing.OficialSlug); linkErr != nil {
			jsonServerError(w, r, "host updated but ssh key link failed: "+linkErr.Error(), linkErr)
			return
		}
	}

	// Sync DNS and service links if provided.
	if req.DNSIDs != nil {
		if err := store.NewDNSRepo(h.db.SQL).SetLinksForHost(r.Context(), existing.ID, *req.DNSIDs); err != nil {
			log.Printf("[hosts] SetHostDNSLinks error on update: %v", err)
		}
	}
	if req.ServiceIDs != nil {
		if err := store.NewServiceRepo(h.db.SQL).SetServicesForHost(r.Context(), existing.ID, *req.ServiceIDs); err != nil {
			log.Printf("[hosts] SetServicesForHost error on update: %v", err)
		}
	}
	if req.ProjectIDs != nil {
		if err := store.NewProjectRepo(h.db.SQL).SetProjectsForHost(r.Context(), existing.ID, *req.ProjectIDs); err != nil {
			log.Printf("[hosts] SetProjectsForHost error on update: %v", err)
		}
	}

	jsonOK(w, req.Host)
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

	store.NewTagRepo(h.db.SQL).Delete(r.Context(), "host", host.ID)
	actor, _ := actorFrom(r)
	if err := store.DeleteParent(r.Context(), h.db.SQL, actor, models.SecretScopeHost, host.ID); err != nil {
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

// linkSSHKey copies the encrypted key blob from the ssh_keys table onto the
// host row. It does NOT materialize the key to the filesystem — key-auth SSH
// decrypts the blob in-memory at connection time via resolveHostKeyPEM.
//
// Returns an error so callers can surface linking failures instead of the
// previous silent-return behavior that made "I selected a key but it didn't
// save" bugs impossible to diagnose in production.
func (h *hostHandlers) linkSSHKey(hostID, sshKeyID int64, slug string) error {
	k, err := store.NewSSHKeyRepo(h.db.SQL).Get(context.Background(), sshKeyID)
	if err != nil {
		log.Printf("[hosts] linkSSHKey slug=%s key_id=%d: GetSSHKey error: %v", slug, sshKeyID, err)
		return fmt.Errorf("load ssh key %d: %w", sshKeyID, err)
	}
	if k == nil {
		log.Printf("[hosts] linkSSHKey slug=%s key_id=%d: ssh key not found", slug, sshKeyID)
		return fmt.Errorf("ssh key %d not found", sshKeyID)
	}
	if len(k.PrivKeyCiphertext) == 0 {
		log.Printf("[hosts] linkSSHKey slug=%s key_id=%d: ssh key has no private key stored (name=%q, credential_type=%q)",
			slug, sshKeyID, k.Name, k.CredentialType)
		return fmt.Errorf("ssh key %q has no stored private key — add a private key to the entry or pick a different key", k.Name)
	}
	// Update the host flag + path metadata (the encrypted payload lives
	// in the vault). identities_only="yes" forces SSH to use only the
	// host's linked key rather than agent or other identities.
	if err := store.NewHostRepo(h.db.SQL).UpdateKeyMeta(context.Background(), hostID, true, "", "yes"); err != nil {
		log.Printf("[hosts] linkSSHKey slug=%s key_id=%d: UpdateHostKeyMeta error: %v", slug, sshKeyID, err)
		return fmt.Errorf("update host key flags: %w", err)
	}

	// Decrypt the ssh_keys row and re-seal into the host's vault entry.
	priv, perr := h.db.Encryptor.Decrypt(k.PrivKeyCiphertext, k.PrivKeyNonce)
	if perr != nil {
		log.Printf("[hosts] linkSSHKey decrypt priv slug=%s: %v", slug, perr)
		return fmt.Errorf("decrypt ssh key %d private payload: %w", sshKeyID, perr)
	}
	pub := ""
	if len(k.PubKeyCiphertext) > 0 {
		if p, derr := h.db.Encryptor.Decrypt(k.PubKeyCiphertext, k.PubKeyNonce); derr == nil {
			pub = p
		}
	}
	// actor_user_id=0 falls through to the vault's resolveSystemActor —
	// linkSSHKey is called from several paths (create + update + sync
	// from coolify) where the caller's user isn't easily threaded down.
	if vErr := vault.HostSetSSHKey(context.Background(), h.db, hostID, 0,
		vault.HostSSHKey{PrivateKeyPEM: priv, PublicKey: pub}); vErr != nil {
		log.Printf("[hosts] linkSSHKey vault write slug=%s: %v", slug, vErr)
		return fmt.Errorf("store host ssh key in vault: %w", vErr)
	}

	log.Printf("[hosts] linkSSHKey slug=%s key_id=%d: linked key %q (fingerprint=%s)", slug, sshKeyID, k.Name, k.Fingerprint)
	return nil
}
