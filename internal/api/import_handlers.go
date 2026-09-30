package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

func readCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

type importHandlers struct {
	db *database.DB
}

type importItemResult struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

type importResult struct {
	Created int                `json:"created"`
	Skipped int                `json:"skipped"`
	Failed  int                `json:"failed"`
	Errors  []importItemResult `json:"errors,omitempty"`
}

// importHostsRequest is one element of the host import array.
type importHostsRequest struct {
	models.Host
	models.AssetGrantsInput          // optional per-item entidade grants; absent ⇒ admin-only until triaged
	Tags                    []string `json:"tags"`
	Password                string   `json:"password"`
}

// handleImportHosts godoc
//
//	@Summary		Bulk-import hosts
//	@Description	Admin. 1 to 500 hosts; each needs nickname and oficial_slug. Taken slugs are skipped, per-item failures are reported in errors, never fail the call. Items without grants stay admin-only until triaged.
//	@Tags			import
//	@Accept			json
//	@Produce		json
//	@Param			body	body		[]importHostsRequest	true	"Hosts to import"
//	@Success		200		{object}	importResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/import/hosts [post]
func (h *importHandlers) handleImportHosts(w http.ResponseWriter, r *http.Request) {
	var items []importHostsRequest
	if err := decodeJSON(r, &items); err != nil {
		jsonBadRequest(w, r, "invalid JSON: expected array of host objects", err)
		return
	}
	if len(items) == 0 {
		jsonError(w, http.StatusBadRequest, "empty array")
		return
	}
	if len(items) > 500 {
		jsonError(w, http.StatusBadRequest, "maximum 500 items per import")
		return
	}

	result := importResult{}

	for i, item := range items {
		name := item.Nickname
		if name == "" {
			name = item.OficialSlug
		}

		// Validate required fields
		if item.Nickname == "" || item.OficialSlug == "" {
			result.Failed++
			result.Errors = append(result.Errors, importItemResult{Index: i, Name: name, Error: "nickname and oficial_slug are required"})
			continue
		}

		// Check slug uniqueness
		exists, _ := store.NewHostRepo(h.db.SQL).SlugExists(r.Context(), item.OficialSlug, 0)
		if exists {
			result.Skipped++
			result.Errors = append(result.Errors, importItemResult{Index: i, Name: name, Error: "slug already exists (skipped)"})
			continue
		}

		// Flag-only: the actual password payload is written to the vault
		// by the post-CreateHost step below. The encrypt step is gone
		// because Encryptor is no longer the storage layer for host
		// secrets — the vault is.
		item.Host.HasPassword = item.Password != ""

		// Normalize preferred auth
		preferredAuth, prefErr := normalizePreferredAuth(item.Host.HasPassword, item.Host.HasKey, item.Host.PreferredAuth)
		if prefErr != nil {
			// Auto-fix: clear preferred auth if invalid
			preferredAuth = ""
		}
		item.Host.PreferredAuth = preferredAuth

		// Create host
		if err := store.NewHostRepo(h.db.SQL).Create(r.Context(), &item.Host); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importItemResult{Index: i, Name: name, Error: fmt.Sprintf("create failed: %v", err)})
			continue
		}

		// Stage 1 dual-write: mirror the password to the unified vault.
		// actorUserID=0 because bulk import doesn't carry a request actor
		// — secret_audit_log.actor_user_id ends up NULL for these rows,
		// matching the legacy migration's import provenance.
		if item.Password != "" {
			if err := vault.HostSetPassword(r.Context(), h.db, item.Host.ID, 0, item.Password); err != nil {
				log.Printf("[import] vault dual-write host slug=%s: %v", item.Host.OficialSlug, err)
			}
		}

		// Set tags
		if len(item.Tags) > 0 {
			store.NewTagRepo(h.db.SQL).Set(r.Context(), "host", item.Host.ID, item.Tags)
		}
		h.applyImportGrants(r, store.AssetHost, item.Host.ID, item.AssetGrantsInput)

		result.Created++
	}

	jsonOK(w, result)
}

// importDNSRequest is one element of the DNS import array.
type importDNSRequest struct {
	models.DNSRecord
	models.AssetGrantsInput          // optional per-item entidade grants; absent ⇒ admin-only until triaged
	Tags                    []string `json:"tags"`
	HostIDs                 []int64  `json:"host_ids"`
}

// handleImportDNS godoc
//
//	@Summary		Bulk-import DNS records
//	@Description	Admin. 1 to 500 records; each needs a domain. Existing domains are skipped, per-item failures are reported in errors, never fail the call. Items without grants stay admin-only until triaged.
//	@Tags			import
//	@Accept			json
//	@Produce		json
//	@Param			body	body		[]importDNSRequest	true	"DNS records to import"
//	@Success		200		{object}	importResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/import/dns [post]
func (h *importHandlers) handleImportDNS(w http.ResponseWriter, r *http.Request) {
	var items []importDNSRequest
	if err := decodeJSON(r, &items); err != nil {
		jsonBadRequest(w, r, "invalid JSON: expected array of DNS objects", err)
		return
	}
	if len(items) == 0 {
		jsonError(w, http.StatusBadRequest, "empty array")
		return
	}
	if len(items) > 500 {
		jsonError(w, http.StatusBadRequest, "maximum 500 items per import")
		return
	}

	result := importResult{}

	for i, item := range items {
		name := item.Domain

		if item.Domain == "" {
			result.Failed++
			result.Errors = append(result.Errors, importItemResult{Index: i, Name: name, Error: "domain is required"})
			continue
		}

		// Try to create — unique constraint on domain will reject duplicates
		if err := store.NewDNSRepo(h.db.SQL).Create(r.Context(), &item.DNSRecord); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, importItemResult{Index: i, Name: name, Error: "domain already exists (skipped)"})
			continue
		}

		if len(item.Tags) > 0 {
			store.NewTagRepo(h.db.SQL).Set(r.Context(), "dns", item.DNSRecord.ID, item.Tags)
		}
		if len(item.HostIDs) > 0 {
			store.NewDNSRepo(h.db.SQL).SetHostLinks(r.Context(), item.DNSRecord.ID, item.HostIDs)
		}
		h.applyImportGrants(r, store.AssetDNS, item.DNSRecord.ID, item.AssetGrantsInput)

		result.Created++
	}

	jsonOK(w, result)
}

// importRequest is the generic import body: type picks the importer, data is
// its array (importHostsRequest or importDNSRequest items).
type importRequest struct {
	Type string            `json:"type"`
	Data []json.RawMessage `json:"data" swaggertype:"array,object"`
}

// handleImport godoc
//
//	@Summary		Bulk-import hosts or DNS records
//	@Description	Admin. {"type": "hosts"|"dns", "data": [...]} dispatched to /api/import/hosts or /api/import/dns, with the same rules.
//	@Tags			import
//	@Accept			json
//	@Produce		json
//	@Param			body	body		importRequest	true	"Import type and items"
//	@Success		200		{object}	importResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Router			/api/import [post]
func (h *importHandlers) handleImport(w http.ResponseWriter, r *http.Request) {
	// Generic import endpoint that auto-detects type from the JSON structure
	var raw json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		jsonBadRequest(w, r, "invalid JSON body", err)
		return
	}

	// Check if it's a wrapped object with "type" field
	var wrapper importRequest
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Type != "" && wrapper.Data != nil {
		// Re-encode data as array body and dispatch
		body, _ := json.Marshal(wrapper.Data)
		switch wrapper.Type {
		case "hosts":
			r.Body = readCloser(body)
			h.handleImportHosts(w, r)
		case "dns":
			r.Body = readCloser(body)
			h.handleImportDNS(w, r)
		default:
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown import type: %s (expected 'hosts' or 'dns')", wrapper.Type))
		}
		return
	}

	jsonError(w, http.StatusBadRequest, "expected JSON object with 'type' ('hosts' or 'dns') and 'data' (array) fields")
}

// applyImportGrants writes an imported item's entidade grants when the item
// carried any. Import is admin-only, so ResolveGrants imposes no scope rule;
// items without grants stay unassigned (admin-only) until triaged in Settings.
func (h *importHandlers) applyImportGrants(r *http.Request, t store.AssetType, id int64, in models.AssetGrantsInput) {
	if !in.Present() {
		return
	}
	g, err := store.ResolveGrants(r.Context(), in, nil)
	if err == nil {
		err = store.NewAssetEntidadeRepo(h.db.SQL).Replace(r.Context(), h.db.SQL, t, id, g)
	}
	if err != nil {
		log.Printf("[import] entidade grants %s id=%d: %v", t, id, err)
	}
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *importHandlers) registerRoutes(rr routeRegistrar) {
	rr.role("admin", "POST /api/import", h.handleImport)
	rr.role("admin", "POST /api/import/hosts", h.handleImportHosts)
	rr.role("admin", "POST /api/import/dns", h.handleImportDNS)
}
