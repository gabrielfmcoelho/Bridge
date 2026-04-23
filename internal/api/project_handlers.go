package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type projectHandlers struct {
	db *database.DB
}

func (h *projectHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	projects, err := models.ListProjects(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list projects", err)
		return
	}

	tagMap, _ := models.GetAllTags(h.db.SQL, "project")
	mainRespNames, _ := models.GetProjectMainResponsavelNamesBulk(h.db.SQL)
	type projectWithTags struct {
		models.Project
		Tags                  []string `json:"tags"`
		MainResponsavelName   string   `json:"main_responsavel_name"`
	}
	result := make([]projectWithTags, len(projects))
	for i, p := range projects {
		result[i] = projectWithTags{
			Project:             p,
			Tags:                tagMap[p.ID],
			MainResponsavelName: mainRespNames[p.ID],
		}
	}

	jsonOK(w, result)
}

func (h *projectHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	project, err := models.GetProject(h.db.SQL, id)
	if err != nil {
		jsonServerError(w, r, "project lookup failed", err)
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	tags, _ := models.GetTags(h.db.SQL, "project", id)
	responsaveis, _ := models.ListProjectResponsaveisContact(h.db.SQL, id)
	services, _ := models.ListServicesByProject(h.db.SQL, id)

	// Collect all host IDs and DNS IDs from project services
	hostIDSet := map[int64]bool{}
	dnsIDSet := map[int64]bool{}
	for _, svc := range services {
		hids, _ := models.GetServiceHostIDs(h.db.SQL, svc.ID)
		for _, hid := range hids {
			hostIDSet[hid] = true
		}
		dids, _ := models.GetServiceDNSIDs(h.db.SQL, svc.ID)
		for _, did := range dids {
			dnsIDSet[did] = true
		}
	}
	var hostIDs, dnsIDs []int64
	for id := range hostIDSet {
		hostIDs = append(hostIDs, id)
	}
	for id := range dnsIDSet {
		dnsIDs = append(dnsIDs, id)
	}

	jsonOK(w, map[string]any{
		"project":       project,
		"tags":          tags,
		"responsaveis":  responsaveis,
		"services":      services,
		"host_ids":      hostIDs,
		"dns_ids":       dnsIDs,
	})
}

func (h *projectHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Project
		Tags         []string                        `json:"tags"`
		Responsaveis []models.ProjectResponsavelInput `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := models.CreateProject(h.db.SQL, &req.Project); err != nil {
		jsonServerError(w, r, "failed to create project", err)
		return
	}

	if len(req.Tags) > 0 {
		models.SetTags(h.db.SQL, "project", req.Project.ID, req.Tags)
	}
	if len(req.Responsaveis) > 0 {
		models.SyncProjectResponsaveisContact(h.db.SQL, req.Project.ID, req.Responsaveis)
	}

	jsonCreated(w, req.Project)
}

func (h *projectHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := models.GetProject(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		models.Project
		Tags         []string                          `json:"tags"`
		Responsaveis *[]models.ProjectResponsavelInput `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.Project.ID = id
	if err := models.UpdateProject(h.db.SQL, &req.Project); err != nil {
		jsonServerError(w, r, "failed to update project", err)
		return
	}

	if req.Tags != nil {
		models.SetTags(h.db.SQL, "project", id, req.Tags)
	}
	if req.Responsaveis != nil {
		models.SyncProjectResponsaveisContact(h.db.SQL, id, *req.Responsaveis)
	}

	jsonOK(w, req.Project)
}

func (h *projectHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	models.DeleteTags(h.db.SQL, "project", id)
	if err := models.DeleteProject(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete project", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
