package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// envBulkRequest is the body of POST /api/secrets/env/bulk (Task 2.2).
// All vars in a single request share the same group/visibility. A request may
// fan out to several targets via `targets` (project AND service, etc.); the
// legacy single `scope`/`parent_id` pair is still accepted and synthesised
// into one target so older callers keep working.
type envBulkRequest struct {
	Targets    []envTarget       `json:"targets,omitempty"` // preferred: multi-target
	Scope      string            `json:"scope,omitempty"`   // legacy single-target
	ParentID   *int64            `json:"parent_id,omitempty"`
	Visibility string            `json:"visibility,omitempty"` // defaults to "shared"
	GroupLabel string            `json:"group_label"`
	Vars       []envBulkVarEntry `json:"vars"`
}

// envTarget is one (scope, parent) destination inside a bulk request.
type envTarget struct {
	Scope    string `json:"scope"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

type envBulkVarEntry struct {
	Name        string  `json:"name"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

func (h *secretHandlers) handleEnvBulk(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req envBulkRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Visibility == "" {
		req.Visibility = string(models.SecretVisibilityShared)
	}

	upserts := make([]vault.EnvVarUpsert, len(req.Vars))
	for i, v := range req.Vars {
		upserts[i] = vault.EnvVarUpsert{
			Name:        v.Name,
			Value:       v.Value,
			Description: v.Description,
		}
	}

	// Resolve targets: prefer the `targets` array; fall back to the legacy
	// single scope/parent pair. The repo validates emptiness + invariants.
	targets := make([]vault.EnvVarTarget, 0, len(req.Targets)+1)
	if len(req.Targets) > 0 {
		for _, tg := range req.Targets {
			targets = append(targets, vault.EnvVarTarget{
				Scope:    models.SecretScope(tg.Scope),
				ParentID: tg.ParentID,
			})
		}
	} else {
		targets = append(targets, vault.EnvVarTarget{
			Scope:    models.SecretScope(req.Scope),
			ParentID: req.ParentID,
		})
	}

	res, err := h.repo.BulkUpsertEnvVarsMulti(
		r.Context(), actor,
		targets,
		models.SecretVisibility(req.Visibility),
		req.GroupLabel,
		upserts,
	)
	if err != nil {
		switch {
		case errors.Is(err, vault.ErrSecretForbidden):
			jsonError(w, http.StatusForbidden, "forbidden")
		case errors.Is(err, vault.ErrSecretNotFound):
			jsonError(w, http.StatusNotFound, "secret not found")
		default:
			// Validation failures (bad name regex, duplicate name, empty
			// value, bad group_label) are 400s — the repo surfaces them as
			// plain errors and we don't want to leak server-error semantics.
			jsonBadRequest(w, r, err.Error(), err)
		}
		return
	}
	jsonOK(w, map[string]any{
		"created": res.Created,
		"updated": res.Updated,
	})
}

// handleEnvList serves GET /api/secrets/env (Task 2.3). Returns metadata
// only (no payload, never any value), grouped by group_label so the UI can
// render the bundle view directly. Filters: scope, parent_id, group_label.
//
// Visibility is implicit per-row: callers see shared rows + their own
// personal rows. The grouping ignores ownership — both personal and shared
// vars in the same group appear under the same key (the UI is responsible
// for visually disambiguating if needed, e.g. by checking visibility).
func (h *secretHandlers) handleEnvList(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	q := r.URL.Query()
	filter := vault.SecretFilter{
		Type:       models.SecretTypeEnvVar,
		Scope:      models.SecretScope(q.Get("scope")),
		GroupLabel: q.Get("group_label"),
	}
	if v := q.Get("parent_id"); v != "" {
		if pid, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.ParentID = &pid
		}
	}
	views, err := h.repo.List(r.Context(), actor, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	grouped := make(map[string][]vault.SecretView)
	for _, v := range views {
		g := ""
		if v.GroupLabel != nil {
			g = *v.GroupLabel
		}
		grouped[g] = append(grouped[g], v)
	}
	// Always emit at least the empty object so the frontend can `data["prod"]`
	// without a null-check on the wrapper.
	if grouped == nil {
		grouped = map[string][]vault.SecretView{}
	}
	jsonOK(w, grouped)
}
