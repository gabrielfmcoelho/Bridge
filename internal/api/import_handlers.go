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

func (h *importHandlers) handleImportHosts(w http.ResponseWriter, r *http.Request) {
	var items []struct {
		models.Host
		Tags     []string `json:"tags"`
		Password string   `json:"password"`
	}
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

		result.Created++
	}

	jsonOK(w, result)
}

func (h *importHandlers) handleImportDNS(w http.ResponseWriter, r *http.Request) {
	var items []struct {
		models.DNSRecord
		Tags    []string `json:"tags"`
		HostIDs []int64  `json:"host_ids"`
	}
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

		result.Created++
	}

	jsonOK(w, result)
}

func (h *importHandlers) handleImport(w http.ResponseWriter, r *http.Request) {
	// Generic import endpoint that auto-detects type from the JSON structure
	var raw json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		jsonBadRequest(w, r, "invalid JSON body", err)
		return
	}

	// Check if it's a wrapped object with "type" field
	var wrapper struct {
		Type string            `json:"type"`
		Data []json.RawMessage `json:"data"`
	}
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
