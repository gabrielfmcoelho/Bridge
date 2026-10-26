package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// databaseHandlers serves the Atlas databases catalogue (/api/databases/*).
// Browsing is authenticated; creating and editing is editor; delete and
// restore are admin. Visibility is the entidades contract (asset type
// "database"). A database runs on an instance (a services row) or is
// external; consumers are the services that use it.
type databaseHandlers struct {
	db *database.DB
}

func (h *databaseHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/databases", h.handleList)
	rr.auth("GET /api/databases/instances", h.handleInstances)
	rr.auth("GET /api/databases/trash", h.handleListTrash)
	rr.role("editor", "POST /api/databases", h.handleCreate)
	rr.auth("GET /api/databases/{id}", h.handleGet)
	rr.role("editor", "PUT /api/databases/{id}", h.handleUpdate)
	rr.role("admin", "DELETE /api/databases/{id}", h.handleDelete)
	rr.role("admin", "POST /api/databases/{id}/restore", h.handleRestore)
}

// applyOpt overwrites *dst when the field was sent (null clears it).
func applyOpt[T any](o optional[T], dst **T) {
	if o.Set {
		*dst = o.Value
	}
}

// databaseRequest creates or edits a database. On update every field is
// optional: omitted keeps the current value; "" clears a text field, null
// clears a nullable one (instance_service_id null = external), [] clears
// links and responsáveis. Grants change only when a grant field is sent.
type databaseRequest struct {
	Name                *string                    `json:"name"`
	DBName              *string                    `json:"db_name"`
	Description         *string                    `json:"description"`
	UseCases            *string                    `json:"use_cases"`
	Engine              *string                    `json:"engine"`
	InstanceServiceID   optional[int64]            `json:"instance_service_id" swaggertype:"integer"`
	ExternalProvider    *string                    `json:"external_provider"`
	Host                *string                    `json:"host"`
	Port                optional[int]              `json:"port" swaggertype:"integer"`
	ConnectionMode      *string                    `json:"connection_mode"`
	TunnelHostID        optional[int64]            `json:"tunnel_host_id" swaggertype:"integer"`
	TunnelTarget        *string                    `json:"tunnel_target"`
	TLSMode             *string                    `json:"tls_mode"`
	Sensitivity         *string                    `json:"sensitivity"`
	Environment         *string                    `json:"environment"`
	BackupPolicy        *string                    `json:"backup_policy"`
	BackupRetentionDays optional[int]              `json:"backup_retention_days" swaggertype:"integer"`
	LastBackupAt        optional[time.Time]        `json:"last_backup_at" swaggertype:"string"`
	ConsumerServiceIDs  *[]int64                   `json:"consumer_service_ids"`
	ProjectIDs          *[]int64                   `json:"project_ids"`
	Responsaveis        *[]models.ResponsavelInput `json:"responsaveis"`
	models.AssetGrantsInput
}

// merge applies the sent fields onto d.
func (req databaseRequest) merge(d *models.Database) {
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&d.Name, req.Name)
	set(&d.DBName, req.DBName)
	set(&d.Engine, req.Engine)
	set(&d.ExternalProvider, req.ExternalProvider)
	set(&d.Host, req.Host)
	set(&d.ConnectionMode, req.ConnectionMode)
	set(&d.TunnelTarget, req.TunnelTarget)
	set(&d.TLSMode, req.TLSMode)
	set(&d.Sensitivity, req.Sensitivity)
	set(&d.Environment, req.Environment)
	set(&d.BackupPolicy, req.BackupPolicy)
	if req.Description != nil {
		d.Description = *req.Description
	}
	if req.UseCases != nil {
		d.UseCases = *req.UseCases
	}
	applyOpt(req.InstanceServiceID, &d.InstanceServiceID)
	applyOpt(req.Port, &d.Port)
	applyOpt(req.TunnelHostID, &d.TunnelHostID)
	applyOpt(req.BackupRetentionDays, &d.BackupRetentionDays)
	applyOpt(req.LastBackupAt, &d.LastBackupAt)
}

// handleList godoc
//
//	@Summary		List catalogued databases
//	@Description	Any role. Only databases visible to the caller (entidade scoping).
//	@Tags			atlas
//	@Produce		json
//	@Param			q					query		string	false	"Search text"
//	@Param			engine				query		string	false	"Engine"
//	@Param			environment			query		string	false	"prod, homolog or dev"
//	@Param			sensitivity			query		string	false	"publico, interno, pessoal or sensivel"
//	@Param			instance_service_id	query		int		false	"Only databases on this instance"
//	@Param			service_id			query		int		false	"Only databases this service consumes"
//	@Param			project_id			query		int		false	"Only databases of this project, directly or through its services"
//	@Param			page				query		int		false	"Page (1-based)"
//	@Param			per_page			query		int		false	"Page size (max 200); omit for every row"
//	@Success		200					{object}	ListEnvelope[models.Database]
//	@Failure		401					{object}	httpx.ErrorResponse
//	@Router			/api/databases [get]
func (h *databaseHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := models.DatabaseFilter{Query: q.Get("q"), Engine: q.Get("engine"), Environment: q.Get("environment"), Sensitivity: q.Get("sensitivity")}
	f.InstanceServiceID, _ = strconv.ParseInt(q.Get("instance_service_id"), 10, 64)
	f.ConsumerServiceID, f.ProjectID = linkFilterParams(r)
	list, err := store.NewDatabaseRepo(h.db.SQL).List(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "list databases", err)
		return
	}
	if names, err := store.NewResponsavelRepo(h.db.SQL).MainNamesBulk(r.Context(), string(store.AssetDatabase)); err == nil {
		for i := range list {
			list[i].MainResponsavelName = names[list[i].ID]
		}
	}
	jsonPaged(w, r, list)
}

// handleInstances godoc
//
//	@Summary		List database instances
//	@Description	Any role. Visible engines found by the scan or Coolify (services of kind database/cache, Coolify databases), with the inferred engine and how many catalogued databases point at each.
//	@Tags			atlas
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.DatabaseInstance]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/databases/instances [get]
func (h *databaseHandlers) handleInstances(w http.ResponseWriter, r *http.Request) {
	list, err := store.NewDatabaseRepo(h.db.SQL).Instances(r.Context())
	if err != nil {
		jsonServerError(w, r, "list database instances", err)
		return
	}
	jsonPaged(w, r, list)
}

// handleGet godoc
//
//	@Summary		Get a catalogued database
//	@Description	Any role; invisible databases answer 404. Includes links, the instance summary, responsáveis and entidade grants.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"Database ID"
//	@Success		200	{object}	models.Database
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/databases/{id} [get]
func (h *databaseHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d, err := store.NewDatabaseRepo(h.db.SQL).Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get database", err)
		return
	}
	if d == nil {
		jsonError(w, http.StatusNotFound, "database not found")
		return
	}
	h.attachDetail(r.Context(), d)
	jsonOK(w, d)
}

func (h *databaseHandlers) attachDetail(ctx context.Context, d *models.Database) {
	if g, err := store.NewAssetEntidadeRepo(h.db.SQL).Get(ctx, store.AssetDatabase, d.ID); err == nil {
		d.Entidades = &g
	}
	d.Responsaveis = []models.Responsavel{}
	if rs, err := store.NewResponsavelRepo(h.db.SQL).List(ctx, string(store.AssetDatabase), d.ID); err == nil && rs != nil {
		d.Responsaveis = rs
	}
}

// checkRefs rejects references the caller can't see (404, like the asset
// itself would answer) and a free endpoint set by a non-admin: host is where
// Bridge will connect to, so only admins may point it anywhere; editors use
// the endpoint derived from the instance.
func (h *databaseHandlers) checkRefs(w http.ResponseWriter, r *http.Request, d *models.Database, prevHost string, serviceIDs, projectIDs []int64) bool {
	if d.InstanceServiceID != nil {
		serviceIDs = append(append([]int64{}, serviceIDs...), *d.InstanceServiceID)
	}
	for _, id := range serviceIDs {
		if s, err := store.NewServiceRepo(h.db.SQL).Get(r.Context(), id); err != nil || s == nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("service %d not found", id))
			return false
		}
	}
	for _, id := range projectIDs {
		if p, err := store.NewProjectRepo(h.db.SQL).Get(r.Context(), id); err != nil || p == nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("project %d not found", id))
			return false
		}
	}
	if d.TunnelHostID != nil {
		if hst, err := store.NewHostRepo(h.db.SQL).GetByID(r.Context(), *d.TunnelHostID); err != nil || hst == nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("host %d not found", *d.TunnelHostID))
			return false
		}
	}
	if d.Host != prevHost && d.Host != "" {
		if actor, _ := actorFrom(r); actor.Role != "admin" {
			jsonError(w, http.StatusForbidden, "only admins may set a database's host")
			return false
		}
	}
	return true
}

func idsOrNil(p *[]int64) []int64 {
	if p == nil {
		return nil
	}
	return append([]int64{}, *p...)
}

// handleCreate godoc
//
//	@Summary		Register a database
//	@Description	Editor+. name and engine are required; instance_service_id null means external (then external_provider or host is required). Setting host is admin-only. Linked services, projects and the tunnel host must be visible (404 otherwise).
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			body	body		databaseRequest	true	"Database, links and entidade grants"
//	@Success		201		{object}	models.Database
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/databases [post]
func (h *databaseHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req databaseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	d := &models.Database{}
	req.merge(d)
	d.ApplyDefaults()
	if actor.UserID != 0 {
		uid := actor.UserID
		d.CreatedBy = &uid
	}
	if err := d.Validate(); err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	consumers, projects := idsOrNil(req.ConsumerServiceIDs), idsOrNil(req.ProjectIDs)
	if !h.checkRefs(w, r, d, "", consumers, projects) {
		return
	}
	g, ok := resolveGrants(w, r, req.AssetGrantsInput, nil)
	if !ok {
		return
	}
	repo := store.NewDatabaseRepo(h.db.SQL)
	if err := repo.Create(r.Context(), d); err != nil {
		if store.IsUniqueViolation(err) {
			jsonError(w, http.StatusConflict, "this instance already has a database with that db_name")
			return
		}
		jsonBadRequest(w, r, "could not save database: "+err.Error(), err)
		return
	}
	// Grants land before the reload: without them a scoped caller can't see
	// the row it just created.
	if err := store.NewAssetEntidadeRepo(h.db.SQL).Replace(r.Context(), h.db.SQL, store.AssetDatabase, d.ID, g); err != nil {
		jsonServerError(w, r, "failed to set entidades", err)
		return
	}
	if err := repo.SetLinks(r.Context(), d.ID, dedupeOrEmpty(consumers), dedupeOrEmpty(projects)); err != nil {
		jsonServerError(w, r, "failed to set database links", err)
		return
	}
	if req.Responsaveis != nil {
		if err := store.NewResponsavelRepo(h.db.SQL).Sync(r.Context(), string(store.AssetDatabase), d.ID, *req.Responsaveis); err != nil {
			jsonBadRequest(w, r, err.Error(), err)
			return
		}
	}
	reloaded, err := repo.Get(r.Context(), d.ID)
	if err != nil {
		jsonServerError(w, r, "reload database", err)
		return
	}
	if reloaded != nil {
		h.attachDetail(r.Context(), reloaded)
	}
	jsonCreated(w, reloaded)
}

// handleUpdate godoc
//
//	@Summary		Update a catalogued database
//	@Description	Editor+; invisible databases answer 404. Partial: an omitted field keeps its value; null clears a nullable field; grants change only when a grant field is sent. Changing host is admin-only.
//	@Tags			atlas
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"Database ID"
//	@Param			body	body		databaseRequest	true	"Fields to change"
//	@Success		200		{object}	models.Database
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/databases/{id} [put]
func (h *databaseHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req databaseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	repo := store.NewDatabaseRepo(h.db.SQL)
	d, err := repo.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "get database", err)
		return
	} else if d == nil {
		jsonError(w, http.StatusNotFound, "database not found")
		return
	}
	prevHost := d.Host
	req.merge(d)
	if err := d.Validate(); err != nil {
		jsonBadRequest(w, r, err.Error(), err)
		return
	}
	consumers, projects := idsOrNil(req.ConsumerServiceIDs), idsOrNil(req.ProjectIDs)
	if !h.checkRefs(w, r, d, prevHost, consumers, projects) {
		return
	}
	if err := repo.Update(r.Context(), d); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			jsonError(w, http.StatusNotFound, "database not found")
		case store.IsUniqueViolation(err):
			jsonError(w, http.StatusConflict, "this instance already has a database with that db_name")
		default:
			jsonBadRequest(w, r, err.Error(), err)
		}
		return
	}
	if err := repo.SetLinks(r.Context(), id, consumers, projects); err != nil {
		jsonServerError(w, r, "failed to set database links", err)
		return
	}
	if req.Responsaveis != nil {
		if err := store.NewResponsavelRepo(h.db.SQL).Sync(r.Context(), string(store.AssetDatabase), id, *req.Responsaveis); err != nil {
			jsonBadRequest(w, r, err.Error(), err)
			return
		}
	}
	if req.AssetGrantsInput.Present() {
		grants := store.NewAssetEntidadeRepo(h.db.SQL)
		existing, _ := grants.Get(r.Context(), store.AssetDatabase, id)
		g, ok := resolveGrants(w, r, req.AssetGrantsInput, &existing)
		if !ok {
			return
		}
		if err := grants.Replace(r.Context(), h.db.SQL, store.AssetDatabase, id, g); err != nil {
			jsonServerError(w, r, "failed to set entidades", err)
			return
		}
	}
	reloaded, err := repo.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "reload database", err)
		return
	}
	if reloaded == nil { // grants just moved it out of the caller's sight
		jsonError(w, http.StatusNotFound, "database not found")
		return
	}
	h.attachDetail(r.Context(), reloaded)
	jsonOK(w, reloaded)
}

// handleDelete godoc
//
//	@Summary		Delete a catalogued database
//	@Description	Admin. Soft delete, together with its vault secrets; restore brings both back.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path	int	true	"Database ID"
//	@Success		204
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/databases/{id} [delete]
func (h *databaseHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if d, err := store.NewDatabaseRepo(h.db.SQL).Get(r.Context(), id); err != nil {
		jsonServerError(w, r, "get database", err)
		return
	} else if d == nil {
		jsonError(w, http.StatusNotFound, "database not found")
		return
	}
	actor, _ := actorFrom(r)
	if err := store.DeleteParent(r.Context(), h.db.SQL, actor, models.SecretScopeDatabase, id); err != nil {
		jsonServerError(w, r, "delete database", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListTrash godoc
//
//	@Summary		List trashed databases
//	@Description	Any role. Soft-deleted databases visible to the caller, newest first.
//	@Tags			atlas
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.Database]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Router			/api/databases/trash [get]
func (h *databaseHandlers) handleListTrash(w http.ResponseWriter, r *http.Request) {
	items, err := store.NewDatabaseRepo(h.db.SQL).ListTrash(r.Context())
	if err != nil {
		jsonServerError(w, r, "list database trash", err)
		return
	}
	jsonPaged(w, r, items)
}

// handleRestore godoc
//
//	@Summary		Restore a database from the trash
//	@Description	Admin. The database comes back with its links and vault secrets.
//	@Tags			atlas
//	@Produce		json
//	@Param			id	path		int	true	"Database ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/databases/{id}/restore [post]
func (h *databaseHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	found, err := store.NewDatabaseRepo(h.db.SQL).Restore(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "restore database", err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "database not found in trash")
		return
	}
	actor, _ := actorFrom(r)
	if err := store.RestoreParent(r.Context(), h.db.SQL, actor, models.SecretScopeDatabase, id); err != nil {
		jsonServerError(w, r, "restore database secrets", err)
		return
	}
	jsonOK(w, StatusResponse{Status: "restored"})
}
