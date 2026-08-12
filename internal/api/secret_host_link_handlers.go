package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// secret_host_link_handlers.go — link/unlink/list the hosts that reuse a shared
// password credential (an avulso `password` secret) via host_remote_users.secret_id.
// This is the "create a credential once, reuse across N hosts" surface; per-host
// passwords stay on the existing scope=host path.
//
// Routes are registered inline in router.go (these are methods on secretHandlers):
//   GET    /api/secrets/{id}/hosts            -> handleListLinkedHosts
//   POST   /api/secrets/{id}/hosts {host_ids} -> handleLinkHosts
//   DELETE /api/secrets/{id}/hosts/{host_id}  -> handleUnlinkHost

// loadSharedPasswordCredential returns the secret view iff it is an avulso
// `password` credential (the only kind shareable across hosts). It reuses
// GetMetadata so the caller's read-ACL is enforced. On failure it writes the
// response and returns ok=false.
func (h *secretHandlers) loadSharedPasswordCredential(w http.ResponseWriter, r *http.Request, id int64) bool {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	v, err := h.repo.GetMetadata(r.Context(), actor, id)
	if err != nil {
		writeErr(w, r, err)
		return false
	}
	if v.Type != models.SecretTypePassword || v.Scope != models.SecretScopeAvulso {
		jsonError(w, http.StatusBadRequest,
			"only an avulso password credential can be shared across hosts")
		return false
	}
	return true
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
	if !h.loadSharedPasswordCredential(w, r, id) {
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
	if !h.loadSharedPasswordCredential(w, r, id) {
		return
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
	repo := store.NewHostRemoteUserRepo(h.db.SQL)
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
		if err := repo.SetSecret(r.Context(), hostID, sshUser, &id); err != nil {
			writeErr(w, r, err)
			return
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
	if !h.loadSharedPasswordCredential(w, r, id) {
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
	if err := store.NewHostRemoteUserRepo(h.db.SQL).SetSecret(r.Context(), hostID, sshUser, nil); err != nil {
		writeErr(w, r, err)
		return
	}
	jsonOK(w, map[string]any{"unlinked": true})
}
