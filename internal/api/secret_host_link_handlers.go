package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// secret_host_link_handlers.go — link/unlink/list the hosts that reuse a shared
// host credential — an avulso `password` (host_remote_users.secret_id) or
// `sshkey` (key_secret_id).
// This is the "create a credential once, reuse across N hosts" surface; per-host
// passwords stay on the existing scope=host path.
//
// Routes are registered inline in router.go (these are methods on secretHandlers):
//   GET    /api/secrets/{id}/hosts            -> handleListLinkedHosts
//   POST   /api/secrets/{id}/hosts {host_ids} -> handleLinkHosts
//   DELETE /api/secrets/{id}/hosts/{host_id}  -> handleUnlinkHost

// loadSharedHostCredential returns the credential's type iff it is an avulso
// password or SSH key (the kinds hosts can share). It reuses GetMetadata so
// the caller's read-ACL is enforced. On failure it writes the response and
// returns ok=false.
func (h *secretHandlers) loadSharedHostCredential(w http.ResponseWriter, r *http.Request, id int64) (models.SecretType, bool) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return "", false
	}
	v, err := h.repo.GetMetadata(r.Context(), actor, id)
	if err != nil {
		writeErr(w, r, err)
		return "", false
	}
	if (v.Type != models.SecretTypePassword && v.Type != models.SecretTypeSSHKey) || v.Scope != models.SecretScopeAvulso {
		jsonError(w, http.StatusBadRequest,
			"only an avulso password or SSH key can be shared across hosts")
		return "", false
	}
	return v.Type, true
}

// requireSharedWriter enforces the shared-secret write ACL (editor/admin),
// matching BulkUpsertEnvVarsMulti. Returns ok=false (response written) otherwise.
func requireSharedWriter(w http.ResponseWriter, r *http.Request) bool {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	if actor.Role != "editor" && actor.Role != "admin" {
		jsonError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

// hostLoginUser returns hosts.ssh_user for a live host, or ("", false) when the
// host is missing/deleted.
func (h *secretHandlers) hostLoginUser(r *http.Request, hostID int64) (string, bool, error) {
	var sshUser string
	err := h.db.SQL.QueryRowContext(r.Context(),
		`SELECT ssh_user FROM hosts WHERE id = ? AND deleted_at IS NULL`, hostID).Scan(&sshUser)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return sshUser, true, nil
}

func (h *secretHandlers) handleListLinkedHosts(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	if _, ok := h.loadSharedHostCredential(w, r, id); !ok {
		return
	}
	ids, err := store.NewHostRemoteUserRepo(h.db.SQL).ListHostsBySecret(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if ids == nil {
		ids = []int64{}
	}
	jsonOK(w, map[string]any{"host_ids": ids})
}

func (h *secretHandlers) handleLinkHosts(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	if !requireSharedWriter(w, r) {
		return
	}
	typ, ok := h.loadSharedHostCredential(w, r, id)
	if !ok {
		return
	}
	if typ == models.SecretTypeSSHKey {
		if _, err := vault.LoadSharedKey(r.Context(), h.db, id); err != nil {
			jsonBadRequest(w, r, err.Error(), nil)
			return
		}
	}
	var req struct {
		HostIDs []int64 `json:"host_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if len(req.HostIDs) == 0 {
		jsonBadRequest(w, r, "host_ids is required", nil)
		return
	}
	linked := 0
	for _, hostID := range req.HostIDs {
		sshUser, ok, err := h.hostLoginUser(r, hostID)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if !ok {
			jsonBadRequest(w, r, "host not found", nil)
			return
		}
		if err := vault.LinkRemoteUserCredential(r.Context(), h.db.SQL, hostID, sshUser, id, typ); err != nil {
			writeErr(w, r, err)
			return
		}
		if typ == models.SecretTypeSSHKey {
			// The host now authenticates by key, as when a key is picked in its form.
			if err := store.NewHostRepo(h.db.SQL).UpdateKeyMeta(r.Context(), hostID, true, "", "yes"); err != nil {
				writeErr(w, r, err)
				return
			}
		}
		linked++
	}
	jsonOK(w, map[string]any{"linked": linked})
}

func (h *secretHandlers) handleUnlinkHost(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid secret id", err)
		return
	}
	hostID, err := pathInt64(r, "host_id")
	if err != nil {
		jsonBadRequest(w, r, "invalid host id", err)
		return
	}
	if !requireSharedWriter(w, r) {
		return
	}
	typ, ok := h.loadSharedHostCredential(w, r, id)
	if !ok {
		return
	}
	sshUser, ok, err := h.hostLoginUser(r, hostID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if !ok {
		jsonBadRequest(w, r, "host not found", nil)
		return
	}
	if err := vault.LinkRemoteUserCredential(r.Context(), h.db.SQL, hostID, sshUser, 0, typ); err != nil {
		writeErr(w, r, err)
		return
	}
	jsonOK(w, map[string]any{"unlinked": true})
}
