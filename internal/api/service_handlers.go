package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// serviceHandlers is a Phase 2 (R2) handler: parse → call service → render.
// Enrichment/orchestration lives in service.ServiceService. It retains db only
// for the Grafana auto-provision side-effect (an integration concern migrated
// in a later phase).
type serviceHandlers struct {
	service *service.ServiceService
	db      *database.DB
}

func (h *serviceHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	services, err := h.service.List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list services", err)
		return
	}
	jsonPaged(w, r, services)
}

func (h *serviceHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	detail, err := h.service.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "service lookup failed", err)
		return
	}
	if detail == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	jsonOK(w, detail)
}

// serviceWriteRequest is the create/update wire shape: a Service plus its
// relations. Pointer slices distinguish "absent" (leave unchanged) from
// "present" (set) on update.
type serviceWriteRequest struct {
	models.Service
	Tags         *[]string                  `json:"tags"`
	HostIDs      *[]int64                   `json:"host_ids"`
	DNSIDs       *[]int64                   `json:"dns_ids"`
	DependsOnIDs *[]int64                   `json:"depends_on_ids"`
	Responsaveis *[]models.ResponsavelInput `json:"responsaveis"`
}

func (req *serviceWriteRequest) toWrite() *service.ServiceWrite {
	return &service.ServiceWrite{
		Service:      req.Service,
		Tags:         req.Tags,
		HostIDs:      req.HostIDs,
		DNSIDs:       req.DNSIDs,
		DependsOnIDs: req.DependsOnIDs,
		Responsaveis: req.Responsaveis,
	}
}

func (h *serviceHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req serviceWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !requireFields(w, map[string]string{"nickname": req.Nickname}) {
		return
	}
	wr := req.toWrite()
	if err := h.service.Create(r.Context(), wr); err != nil {
		jsonServerError(w, r, "failed to create service", err)
		return
	}
	// Fire-and-forget auto-provision of the default Grafana dashboard.
	h.maybeProvisionGrafanaDashboard(wr.Service)
	jsonCreated(w, wr.Service)
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
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req serviceWriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	wr := req.toWrite()
	found, err := h.service.Update(r.Context(), id, wr)
	if err != nil {
		jsonServerError(w, r, "failed to update service", err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	jsonOK(w, wr.Service)
}

func (h *serviceHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actor, _ := actorFrom(r)
	if err := h.service.Delete(r.Context(), actor, id); err != nil {
		jsonServerError(w, r, "failed to delete service", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleFixate converts an auto-discovered service to a fixed service.
func (h *serviceHandlers) handleFixate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	svc, err := h.service.Fixate(r.Context(), id)
	if errors.Is(err, service.ErrServiceNotAuto) {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to fixate service", err)
		return
	}
	if svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	jsonOK(w, svc)
}

// handleUpdateContainer rebinds a fixed/manual service to a different container.
func (h *serviceHandlers) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		ContainerName string `json:"container_name"`
		ContainerID   string `json:"container_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	svc, err := h.service.UpdateContainer(r.Context(), id, req.ContainerName, req.ContainerID)
	if errors.Is(err, service.ErrServiceCannotRebindAuto) {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		jsonServerError(w, r, "failed to update container binding", err)
		return
	}
	if svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	jsonOK(w, svc)
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *serviceHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/services", h.handleList)
	rr.auth("GET /api/services/trash", h.handleListTrash)
	rr.role("admin", "POST /api/services/{id}/restore", h.handleRestore)
	rr.role("editor", "POST /api/services", h.handleCreate)
	rr.auth("GET /api/services/{id}", h.handleGet)
	rr.role("editor", "PUT /api/services/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/services/{id}", h.handleDelete)
	rr.role("editor", "POST /api/services/{id}/fixate", h.handleFixate)
	rr.role("editor", "PUT /api/services/{id}/container", h.handleUpdateContainer)
}

func (h *serviceHandlers) handleListTrash(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListTrash(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list service trash", err)
		return
	}
	jsonPaged(w, r, items)
}

func (h *serviceHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actor, _ := actorFrom(r)
	if err := h.service.Restore(r.Context(), actor, id); err != nil {
		jsonServerError(w, r, "failed to restore service", err)
		return
	}
	jsonOK(w, map[string]string{"status": "restored"})
}
