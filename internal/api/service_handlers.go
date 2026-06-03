package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type serviceHandlers struct {
	db *database.DB
}

func (h *serviceHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	services, err := models.ListServices(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list services", err)
		return
	}

	tagMap, _ := store.NewTagRepo(h.db.SQL).GetAll(r.Context(), "service")
	mainRespNames, _ := models.GetServiceMainResponsavelNamesBulk(h.db.SQL)
	type serviceWithMeta struct {
		models.Service
		Tags                  []string `json:"tags"`
		HostIDs               []int64  `json:"host_ids"`
		DNSIDs                []int64  `json:"dns_ids"`
		DependsOnIDs          []int64  `json:"depends_on_ids"`
		MainResponsavelName   string   `json:"main_responsavel_name"`
	}
	result := make([]serviceWithMeta, len(services))
	for i, svc := range services {
		hostIDs, _ := models.GetServiceHostIDs(h.db.SQL, svc.ID)
		dnsIDs, _ := models.GetServiceDNSIDs(h.db.SQL, svc.ID)
		depIDs, _ := models.GetServiceDependencyIDs(h.db.SQL, svc.ID)
		result[i] = serviceWithMeta{
			Service:             svc,
			Tags:                tagMap[svc.ID],
			HostIDs:             hostIDs,
			DNSIDs:              dnsIDs,
			DependsOnIDs:        depIDs,
			MainResponsavelName: mainRespNames[svc.ID],
		}
	}

	jsonOK(w, result)
}

func (h *serviceHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	svc, err := models.GetService(h.db.SQL, id)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	tags, _ := store.NewTagRepo(h.db.SQL).Get(r.Context(), "service", id)
	hostIDs, _ := models.GetServiceHostIDs(h.db.SQL, id)
	dnsIDs, _ := models.GetServiceDNSIDs(h.db.SQL, id)
	dependsOnIDs, _ := models.GetServiceDependencyIDs(h.db.SQL, id)
	dependentIDs, _ := models.GetServiceDependentIDs(h.db.SQL, id)

	// Credentials are fetched separately by the frontend via /api/secrets —
	// the legacy inline credentials[] field was removed in Phase 1 cutover.

	responsaveis, _ := models.ListServiceResponsaveis(h.db.SQL, id)

	jsonOK(w, map[string]any{
		"service":        svc,
		"tags":           tags,
		"host_ids":       hostIDs,
		"dns_ids":        dnsIDs,
		"depends_on_ids": dependsOnIDs,
		"dependent_ids":  dependentIDs,
		"responsaveis":   responsaveis,
	})
}

func (h *serviceHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Service
		Tags         []string                          `json:"tags"`
		HostIDs      []int64                           `json:"host_ids"`
		DNSIDs       []int64                           `json:"dns_ids"`
		DependsOnIDs []int64                           `json:"depends_on_ids"`
		Responsaveis []models.ServiceResponsavelInput  `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Nickname == "" {
		jsonError(w, http.StatusBadRequest, "nickname is required")
		return
	}

	if err := models.CreateService(h.db.SQL, &req.Service); err != nil {
		jsonServerError(w, r, "failed to create service", err)
		return
	}

	if len(req.Tags) > 0 {
		store.NewTagRepo(h.db.SQL).Set(r.Context(), "service", req.Service.ID, req.Tags)
	}
	if len(req.HostIDs) > 0 {
		models.SetServiceHostLinks(h.db.SQL, req.Service.ID, req.HostIDs)
	}
	if len(req.DNSIDs) > 0 {
		models.SetServiceDNSLinks(h.db.SQL, req.Service.ID, req.DNSIDs)
	}
	if len(req.DependsOnIDs) > 0 {
		models.SetServiceDependencies(h.db.SQL, req.Service.ID, req.DependsOnIDs)
	}
	if len(req.Responsaveis) > 0 {
		models.SyncServiceResponsaveis(h.db.SQL, req.Service.ID, req.Responsaveis)
	}

	// Fire-and-forget auto-provision of the default Grafana dashboard.
	h.maybeProvisionGrafanaDashboard(req.Service)

	jsonCreated(w, req.Service)
}

// maybeProvisionGrafanaDashboard runs ProvisionServiceDashboard in a goroutine
// if the integration is enabled. Failures only log — creation never blocks.
func (h *serviceHandlers) maybeProvisionGrafanaDashboard(svc models.Service) {
	if store.NewAppSettingsRepo(h.db.SQL).Value(context.Background(), "grafana_enabled") != "true" {
		return
	}
	go func(svc models.Service) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("[grafana-provision] service %d panic: %v", svc.ID, rv)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := ProvisionServiceDashboard(ctx, h.db, &svc); err != nil {
			log.Printf("[grafana-provision] service %d (%s): %v", svc.ID, svc.Nickname, err)
		}
	}(svc)
}

func (h *serviceHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := models.GetService(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	var req struct {
		models.Service
		Tags         []string                            `json:"tags"`
		HostIDs      []int64                             `json:"host_ids"`
		DNSIDs       []int64                             `json:"dns_ids"`
		DependsOnIDs []int64                             `json:"depends_on_ids"`
		Responsaveis *[]models.ServiceResponsavelInput   `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.Service.ID = id
	if err := models.UpdateService(h.db.SQL, &req.Service); err != nil {
		jsonServerError(w, r, "failed to update service", err)
		return
	}

	if req.Tags != nil {
		store.NewTagRepo(h.db.SQL).Set(r.Context(), "service", id, req.Tags)
	}
	if req.HostIDs != nil {
		models.SetServiceHostLinks(h.db.SQL, id, req.HostIDs)
	}
	if req.DNSIDs != nil {
		models.SetServiceDNSLinks(h.db.SQL, id, req.DNSIDs)
	}
	if req.DependsOnIDs != nil {
		models.SetServiceDependencies(h.db.SQL, id, req.DependsOnIDs)
	}
	if req.Responsaveis != nil {
		models.SyncServiceResponsaveis(h.db.SQL, id, *req.Responsaveis)
	}

	jsonOK(w, req.Service)
}

func (h *serviceHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	store.NewTagRepo(h.db.SQL).Delete(r.Context(), "service", id)
	actor, _ := actorFrom(r)
	if err := store.DeleteParent(r.Context(), h.db.SQL, actor, models.SecretScopeService, id); err != nil {
		jsonServerError(w, r, "failed to delete service", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// Legacy service-credential handlers were removed in the Phase 1 cutover.
// Callers query /api/secrets?scope=service&parent_id=<id> (list metadata)
// and /api/secrets/{id}/reveal (decrypt) instead.

// handleFixate converts an auto-discovered service to a fixed service.
func (h *serviceHandlers) handleFixate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	svc, err := models.GetService(h.db.SQL, id)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.Source != "auto" {
		jsonError(w, http.StatusBadRequest, "only auto-discovered services can be fixated")
		return
	}

	if err := models.FixateService(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to fixate service", err)
		return
	}

	svc.Source = "fixed"
	jsonOK(w, svc)
}

// handleUpdateContainer rebinds a fixed/manual service to a different container.
func (h *serviceHandlers) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	svc, err := models.GetService(h.db.SQL, id)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.Source == "auto" {
		jsonError(w, http.StatusBadRequest, "cannot rebind auto services; fixate first")
		return
	}

	var req struct {
		ContainerName string `json:"container_name"`
		ContainerID   string `json:"container_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	if err := models.UpdateContainerBinding(h.db.SQL, id, req.ContainerName, req.ContainerID); err != nil {
		jsonServerError(w, r, "failed to update container binding", err)
		return
	}

	svc.ContainerName = req.ContainerName
	svc.ContainerID = req.ContainerID
	jsonOK(w, svc)
}

// handleListAllCredentials was removed in the Phase 1 cutover. The
// frontend now grouping-by-service is done client-side on top of
// /api/secrets?scope=service&visibility=shared + /api/services.
