package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gestor"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

type gestorHandlers struct {
	db *database.DB
}

// gestorLinkInput is one entry of a project's gestor_links request field.
type gestorLinkInput struct {
	GestorProjetoID int64 `json:"gestor_projeto_id"`
}

// gestorClientFor returns the configured Gestor client, or nil when the
// integration is off or incomplete. enabled reports the on/off switch alone.
func gestorClientFor(db *database.DB) (c *gestor.Client, enabled bool, err error) {
	s, err := gestor.LoadSettings(db.SQL, db.Encryptor)
	if err != nil {
		return nil, false, err
	}
	return gestor.NewClient(s, gestor.KeycloakFromSettings(db.SQL)), s.Enabled, nil
}

// handleProjetos searches Gestor projects by name for the project form.
//
//	@Summary		Search Gestor projects
//	@Description	Editor+. Always 200: {enabled, configured, data:[{id,nome,concluido,arquivado}], error?}; a Gestor failure comes back in "error" so the form degrades.
//	@Tags			gestor
//	@Produce		json
//	@Param			q	query		string	false	"Name fragment"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/gestor/projetos [get]
func (h *gestorHandlers) handleProjetos(w http.ResponseWriter, r *http.Request) {
	c, enabled, err := gestorClientFor(h.db)
	if err != nil {
		jsonServerError(w, r, "failed to read gestor settings", err)
		return
	}
	if c == nil {
		jsonOK(w, map[string]any{"enabled": false, "configured": false, "data": []gestor.Projeto{}})
		return
	}
	out := map[string]any{"enabled": enabled, "configured": true, "data": []map[string]any{}}
	ps, err := c.BuscarProjetos(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), 20)
	if err != nil {
		out["error"] = err.Error()
		jsonOK(w, out)
		return
	}
	data := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		data = append(data, map[string]any{"id": p.ID, "nome": p.Nome, "concluido": p.Concluido, "arquivado": p.Arquivado})
	}
	out["data"] = data
	jsonOK(w, out)
}

func (h *gestorHandlers) registerRoutes(rr routeRegistrar) {
	rr.role("editor", "GET /api/gestor/projetos", h.handleProjetos)
}

// resolveGestorLinks fills wr.GestorLinks from the request: ids deduped, each
// name fetched from Gestor. Nothing is written here; on failure it answers
// (400 unknown project, 502 Gestor unavailable) and returns false.
func (h *projectHandlers) resolveGestorLinks(w http.ResponseWriter, r *http.Request, in *[]gestorLinkInput, wr *service.ProjectWrite) bool {
	if in == nil {
		return true
	}
	links := []models.ProjectGestorLink{}
	if len(*in) > 0 {
		c, _, err := gestorClientFor(h.db)
		if err != nil {
			jsonServerError(w, r, "failed to read gestor settings", err)
			return false
		}
		if c == nil {
			jsonError(w, http.StatusBadRequest, "A integração com o Gestor está desligada ou incompleta")
			return false
		}
		seen := map[int64]bool{}
		for _, l := range *in {
			if seen[l.GestorProjetoID] {
				continue
			}
			seen[l.GestorProjetoID] = true
			p, err := c.Projeto(r.Context(), l.GestorProjetoID)
			var ge *gestor.Error
			switch {
			case errors.As(err, &ge) && ge.Status == http.StatusNotFound:
				jsonError(w, http.StatusBadRequest, "Projeto do Gestor não encontrado")
				return false
			case err != nil:
				jsonErrorLogged(w, r, http.StatusBadGateway, "Gestor indisponível", err)
				return false
			}
			links = append(links, models.ProjectGestorLink{GestorProjetoID: p.ID, GestorProjetoNome: p.Nome})
		}
	}
	wr.GestorLinks = &links
	return true
}
