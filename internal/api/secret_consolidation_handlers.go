package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// secret_consolidation_handlers.go — admin: preview and apply turning the
// same password/key repeated on many hosts into one shared credential.
//   GET  /api/secrets/consolidation            -> groups
//   POST /api/secrets/consolidation {keys: []} -> apply the chosen groups

// handleConsolidationPlan godoc
//
//	@Summary		Preview host-credential consolidation
//	@Description	Admin. Groups of 2+ hosts holding identical per-host password or key copies.
//	@Tags			secrets
//	@Produce		json
//	@Success		200	{array}		vault.ConsolidationGroup
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/secrets/consolidation [get]
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

// secretConsolidationApplyRequest picks the preview groups to consolidate.
type secretConsolidationApplyRequest struct {
	Keys []string `json:"keys"`
}

// handleConsolidationApply godoc
//
//	@Summary		Consolidate host credentials
//	@Description	Admin. For each chosen group (keys from the preview), in one transaction: find or create the shared credential, link every host and trash the copies.
//	@Tags			secrets
//	@Accept			json
//	@Produce		json
//	@Param			body	body		secretConsolidationApplyRequest	true	"Group keys to consolidate"
//	@Success		200		{object}	vault.ConsolidationResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/secrets/consolidation [post]
func (h *secretHandlers) handleConsolidationApply(w http.ResponseWriter, r *http.Request) {
	var req secretConsolidationApplyRequest
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
