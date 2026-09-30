package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// secret_consolidation_handlers.go — admin: preview and apply turning the
// same password/key repeated on many hosts into one shared credential.
//   GET  /api/secrets/consolidation            -> groups
//   POST /api/secrets/consolidation {keys: []} -> apply the chosen groups

func (h *secretHandlers) handleConsolidationPlan(w http.ResponseWriter, r *http.Request) {
	groups, err := vault.PlanHostCredentialConsolidation(r.Context(), h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to plan consolidation", err)
		return
	}
	if groups == nil {
		groups = []vault.ConsolidationGroup{}
	}
	jsonOK(w, groups)
}

func (h *secretHandlers) handleConsolidationApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil || len(req.Keys) == 0 {
		jsonError(w, http.StatusBadRequest, "keys is required")
		return
	}
	res, err := vault.ApplyHostCredentialConsolidation(r.Context(), h.db.SQL, h.db.Encryptor, actorID(r), req.Keys)
	if err != nil {
		jsonServerError(w, r, "failed to consolidate", err)
		return
	}
	jsonOK(w, res)
}
