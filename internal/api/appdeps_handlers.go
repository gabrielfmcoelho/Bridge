package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
	gitlabclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// appDepsHandlers serves the app dependencies module: the catalogs
// (catalog/dependencies.yml) read from every GitLab repository, and the
// background sync that reads them. Every authenticated user reads; editors
// sync.
type appDepsHandlers struct {
	db *database.DB
}

// AppDepSyncStatus is the background app dependencies sync: under way, or
// how the last one ended.
type AppDepSyncStatus = bgJobStatus[service.AppDepSyncResult]

var appDepsSyncJob = &bgJob[service.AppDepSyncResult]{}

const appDepsSyncTimeout = 30 * time.Minute

func (h *appDepsHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/app-deps", h.handleList)
	rr.auth("GET /api/app-deps/sync", h.handleSyncStatus)
	rr.role("editor", "POST /api/app-deps/sync", h.handleSync)
	rr.auth("GET /api/app-deps/links", h.handleLinks)
	rr.role("editor", "POST /api/app-deps/links", h.handleAddLink)
	rr.role("editor", "DELETE /api/app-deps/links/{ci}/{type}/{id}", h.handleRemoveLink)
}

// handleList godoc
//
//	@Summary		App dependency catalogs
//	@Description	Every repository's catalog/dependencies.yml as last synced from GitLab, parsed (catalog-parser schema: ci, components, upstream/downstream links). catalog is null and parse_error set when the file failed to parse. Not linted.
//	@Tags			app-deps
//	@Produce		json
//	@Success		200	{object}	ListEnvelope[models.AppDepSource]
//	@Router			/api/app-deps [get]
func (h *appDepsHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	rows, err := store.NewAppDepRepo(h.db.SQL).List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list app dependency catalogs", err)
		return
	}
	jsonPaged(w, r, rows)
}

// handleSync starts, in the background, the read of every repository's
// catalog from GitLab.
//
//	@Summary		Sync app dependency catalogs (background)
//	@Description	Editor. Answers 202 with the run's status at once; a sync already under way is returned instead of starting another. Poll GET for the result. Reads catalog/dependencies.yml at each repository's default branch, in the groups of the gitlab_catalog_groups setting (or every project the service token is a member of). A missing file removes the repository's row; a read error keeps it. 400 when GitLab is not configured.
//	@Tags			app-deps
//	@Produce		json
//	@Success		202	{object}	AppDepSyncStatus
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		502	{object}	httpx.ErrorResponse
//	@Router			/api/app-deps/sync [post]
func (h *appDepsHandlers) handleSync(w http.ResponseWriter, r *http.Request) {
	svc := service.NewAppDepsService(h.db.SQL, h.db.Encryptor)
	switch err := svc.Ready(); {
	case errors.Is(err, service.ErrGitLabNaoConfigurado):
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		jsonErrorLogged(w, r, http.StatusBadGateway, "Falha ao sincronizar; veja os logs do servidor", err)
		return
	}
	// Detached: the sync outlives this request (and a proxy cutting it).
	base := context.WithoutCancel(r.Context())
	st, _ := appDepsSyncJob.start(func() (*service.AppDepSyncResult, error) {
		ctx, cancel := context.WithTimeout(base, appDepsSyncTimeout)
		defer cancel()
		start := time.Now()
		res, err := svc.Sync(ctx)
		if err != nil {
			log.Printf("[app-deps] sync failed after %s: %v", time.Since(start).Round(time.Second), err)
		} else {
			log.Printf("[app-deps] sync in %s: repos=%d found=%d parsed=%d failed=%d removed=%d errors=%d",
				time.Since(start).Round(time.Second), res.Repos, res.Found, res.Parsed, res.Failed, res.Removed, len(res.Errors))
		}
		return &res, err
	})
	httpx.WriteJSON(w, http.StatusAccepted, st)
}

// handleSyncStatus godoc
//
//	@Summary		Status of the background app dependencies sync
//	@Description	Whether a sync is under way, and the result or error of the last one (since the server started).
//	@Tags			app-deps
//	@Produce		json
//	@Success		200	{object}	AppDepSyncStatus
//	@Router			/api/app-deps/sync [get]
func (h *appDepsHandlers) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, appDepsSyncJob.get())
}

// appDepLinks is service.AppDepLinks for the request's caller.
func appDepLinks(r *http.Request, db *database.DB, f service.AppDepLinkFilter) ([]models.AppDepAssetLink, error) {
	return service.AppDepLinks(r.Context(), db.SQL, gitlabclient.BaseURL(db.SQL), f)
}

// handleLinks godoc
//
//	@Summary		Application ↔ asset links
//	@Description	Links between applications (CIs of the synced catalogs) and the hosts, DNS records, services and projetos the caller can see. Derived from the CI's GitLab repository (repo, group and scan links of hosts and DNS, projeto GitLab links, service repository fields) plus the links editors add; sources lists where each comes from. Filter by ci, or by entity_type + entity_id.
//	@Tags			app-deps
//	@Produce		json
//	@Param			ci			query		string	false	"Application (CI)"
//	@Param			entity_type	query		string	false	"host, dns, service or project"
//	@Param			entity_id	query		int		false	"Asset id (with entity_type)"
//	@Success		200			{object}	ListEnvelope[models.AppDepAssetLink]
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Router			/api/app-deps/links [get]
func (h *appDepsHandlers) handleLinks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := service.AppDepLinkFilter{CI: strings.TrimSpace(q.Get("ci")), EntityType: q.Get("entity_type")}
	if f.EntityType != "" {
		id, err := strconv.ParseInt(q.Get("entity_id"), 10, 64)
		if err != nil || !slices.Contains(models.AppDepLinkTypes, f.EntityType) {
			jsonError(w, http.StatusBadRequest, "entity_type must be host, dns, service or project, with an entity_id")
			return
		}
		f.EntityID = id
	}
	links, err := appDepLinks(r, h.db, f)
	if err != nil {
		jsonServerError(w, r, "failed to list application links", err)
		return
	}
	jsonPaged(w, r, links)
}

// readManualLink validates a link's ends: a known type, a CI some catalog
// declares (404 otherwise) and a live asset the caller can see (404
// otherwise, never 403). ok false means the response is written.
func (h *appDepsHandlers) readManualLink(w http.ResponseWriter, r *http.Request, l models.AppDepManualLink) bool {
	if !slices.Contains(models.AppDepLinkTypes, l.EntityType) || l.CI == "" || l.EntityID <= 0 {
		jsonError(w, http.StatusBadRequest, "ci, entity_type (host, dns, service or project) and entity_id are required")
		return false
	}
	repo := store.NewAppDepRepo(h.db.SQL)
	ok, err := repo.HasCI(r.Context(), l.CI)
	if err != nil {
		jsonServerError(w, r, "failed to read application", err)
		return false
	}
	if !ok {
		jsonError(w, http.StatusNotFound, "application not found")
		return false
	}
	if ok, err = repo.EntityLiveVisible(r.Context(), l.EntityType, l.EntityID); err != nil {
		jsonServerError(w, r, "failed to read asset", err)
		return false
	} else if !ok {
		jsonError(w, http.StatusNotFound, l.EntityType+" not found")
		return false
	}
	return true
}

// handleAddLink godoc
//
//	@Summary		Link an application to an asset
//	@Description	Editor. Stores a manual link between an application (CI) and a host, DNS record, service or projeto; adding one that exists is a no-op. 404 when the CI is in no synced catalog or the asset is not visible.
//	@Tags			app-deps
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.AppDepManualLink	true	"The link"
//	@Success		201		{object}	models.AppDepManualLink
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/app-deps/links [post]
func (h *appDepsHandlers) handleAddLink(w http.ResponseWriter, r *http.Request) {
	var l models.AppDepManualLink
	if err := decodeJSON(r, &l); err != nil {
		jsonBadRequest(w, r, "invalid body", err)
		return
	}
	l.CI = strings.TrimSpace(l.CI)
	if !h.readManualLink(w, r, l) {
		return
	}
	if err := store.NewAppDepRepo(h.db.SQL).AddLink(r.Context(), l, actorID(r)); err != nil {
		jsonServerError(w, r, "failed to link application", err)
		return
	}
	jsonCreated(w, l)
}

// handleRemoveLink godoc
//
//	@Summary		Unlink an application from an asset
//	@Description	Editor. Removes a manual link. Links derived from the repository stay: remove them at their source (the host/DNS/projeto GitLab link, the service's repository field). 404 when there is no such manual link or the asset is not visible.
//	@Tags			app-deps
//	@Param			ci		path	string	true	"Application (CI)"
//	@Param			type	path	string	true	"host, dns, service or project"
//	@Param			id		path	int		true	"Asset id"
//	@Success		204
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/app-deps/links/{ci}/{type}/{id} [delete]
func (h *appDepsHandlers) handleRemoveLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	l := models.AppDepManualLink{CI: r.PathValue("ci"), EntityType: r.PathValue("type"), EntityID: id}
	if !slices.Contains(models.AppDepLinkTypes, l.EntityType) {
		jsonError(w, http.StatusBadRequest, "type must be host, dns, service or project")
		return
	}
	repo := store.NewAppDepRepo(h.db.SQL)
	if ok, err := repo.EntityLiveVisible(r.Context(), l.EntityType, l.EntityID); err != nil {
		jsonServerError(w, r, "failed to read asset", err)
		return
	} else if !ok {
		jsonError(w, http.StatusNotFound, l.EntityType+" not found")
		return
	}
	removed, err := repo.RemoveLink(r.Context(), l)
	if err != nil {
		jsonServerError(w, r, "failed to unlink application", err)
		return
	}
	if !removed {
		jsonError(w, http.StatusNotFound, "link not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
