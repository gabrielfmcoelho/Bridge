package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// resolveKeySecret turns a request's key choice into a vault key id the
// caller may see: key_secret_id, or — from clients before v91 — an ssh_keys
// id mapped to the key it was migrated to. 0 means no key chosen. Writes a
// 404 (existence isn't leaked) and returns false when the key isn't usable.
func resolveKeySecret(w http.ResponseWriter, r *http.Request, db *database.DB, keySecretID, legacySSHKeyID int64) (int64, bool) {
	id := keySecretID
	if id == 0 && legacySSHKeyID > 0 {
		if id = vault.KeySecretForLegacy(r.Context(), db.SQL, legacySSHKeyID); id == 0 {
			jsonError(w, http.StatusNotFound, "ssh key not found")
			return 0, false
		}
	}
	if id == 0 {
		return 0, true
	}
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return 0, false
	}
	if _, err := vault.NewSecretRepo(db).GetMetadata(r.Context(), actor, id); err != nil {
		jsonError(w, http.StatusNotFound, "ssh key not found")
		return 0, false
	}
	return id, true
}
