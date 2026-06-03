package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type dnsHandlers struct {
	db *database.DB
}

func (h *dnsHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	records, err := models.ListDNSRecords(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list DNS records", err)
		return
	}

	tagMap, _ := store.NewTagRepo(h.db.SQL).GetAll(r.Context(), "dns")
	mainRespNames, _ := models.GetDNSMainResponsavelNamesBulk(h.db.SQL)
	type dnsWithTags struct {
		models.DNSRecord
		Tags                  []string `json:"tags"`
		HostIDs               []int64  `json:"host_ids"`
		MainResponsavelName   string   `json:"main_responsavel_name"`
	}
	result := make([]dnsWithTags, len(records))
	for i, rec := range records {
		hostIDs, _ := models.GetDNSHostIDs(h.db.SQL, rec.ID)
		result[i] = dnsWithTags{
			DNSRecord:           rec,
			Tags:                tagMap[rec.ID],
			HostIDs:             hostIDs,
			MainResponsavelName: mainRespNames[rec.ID],
		}
	}

	jsonOK(w, result)
}

func (h *dnsHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	rec, err := models.GetDNSRecord(h.db.SQL, id)
	if err != nil || rec == nil {
		jsonError(w, http.StatusNotFound, "DNS record not found")
		return
	}

	tags, _ := store.NewTagRepo(h.db.SQL).Get(r.Context(), "dns", rec.ID)
	hostIDs, _ := models.GetDNSHostIDs(h.db.SQL, rec.ID)
	responsaveis, _ := models.ListDNSResponsaveis(h.db.SQL, rec.ID)

	jsonOK(w, map[string]any{
		"dns_record":   rec,
		"tags":         tags,
		"host_ids":     hostIDs,
		"responsaveis": responsaveis,
	})
}

func (h *dnsHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.DNSRecord
		Tags         []string                      `json:"tags"`
		HostIDs      []int64                       `json:"host_ids"`
		Responsaveis []models.DNSResponsavelInput  `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Domain == "" {
		jsonError(w, http.StatusBadRequest, "domain is required")
		return
	}

	if err := models.CreateDNSRecord(h.db.SQL, &req.DNSRecord); err != nil {
		jsonError(w, http.StatusConflict, "domain already exists")
		return
	}

	if len(req.Tags) > 0 {
		store.NewTagRepo(h.db.SQL).Set(r.Context(), "dns", req.DNSRecord.ID, req.Tags)
	}
	if len(req.HostIDs) > 0 {
		models.SetDNSHostLinks(h.db.SQL, req.DNSRecord.ID, req.HostIDs)
	}
	if len(req.Responsaveis) > 0 {
		models.SyncDNSResponsaveis(h.db.SQL, req.DNSRecord.ID, req.Responsaveis)
	}

	jsonCreated(w, req.DNSRecord)
}

func (h *dnsHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := models.GetDNSRecord(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "DNS record not found")
		return
	}

	var req struct {
		models.DNSRecord
		Tags         []string                       `json:"tags"`
		HostIDs      []int64                        `json:"host_ids"`
		Responsaveis *[]models.DNSResponsavelInput  `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.DNSRecord.ID = id
	if err := models.UpdateDNSRecord(h.db.SQL, &req.DNSRecord); err != nil {
		jsonServerError(w, r, "failed to update DNS record", err)
		return
	}

	if req.Tags != nil {
		store.NewTagRepo(h.db.SQL).Set(r.Context(), "dns", id, req.Tags)
	}
	if req.HostIDs != nil {
		models.SetDNSHostLinks(h.db.SQL, id, req.HostIDs)
	}
	if req.Responsaveis != nil {
		models.SyncDNSResponsaveis(h.db.SQL, id, *req.Responsaveis)
	}

	jsonOK(w, req.DNSRecord)
}

func (h *dnsHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	store.NewTagRepo(h.db.SQL).Delete(r.Context(), "dns", id)
	if err := models.DeleteDNSRecord(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete DNS record", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
