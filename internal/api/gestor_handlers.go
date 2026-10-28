package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gestor"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
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
	rr.role("admin", "GET /api/gestor/autores", h.handleAutores)
	rr.role("admin", "POST /api/gestor/autores/varrer", h.handleVarrer)
	rr.role("admin", "PUT /api/gestor/autores/{id}", h.handleAutorUpdate)
	rr.role("admin", "GET /api/gestor/pessoas", h.handlePessoas)
}

// handleAutores lists GitLab commit authors. status: sem_vinculo (default), com_sugestao, vinculados, ignorados, todos.
//
//	@Summary	List GitLab authors
//	@Tags		gestor
//	@Produce	json
//	@Param		status	query		string	false	"sem_vinculo|com_sugestao|vinculados|ignorados|todos"
//	@Param		q		query		string	false	"E-mail or name fragment"
//	@Success	200		{object}	ListEnvelope[models.GitLabAutor]
//	@Router		/api/gestor/autores [get]
func (h *gestorHandlers) handleAutores(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "sem_vinculo"
	}
	rows, err := store.NewGitLabAutorRepo(h.db.SQL).List(r.Context(), status, strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		jsonServerError(w, r, "failed to list authors", err)
		return
	}
	jsonPaged(w, r, rows)
}

// handleVarrer scans GitLab synchronously and stores authors and suggestions.
//
//	@Summary	Scan GitLab authors
//	@Tags		gestor
//	@Produce	json
//	@Success	200	{object}	service.ResumoVarredura
//	@Failure	400	{object}	httpx.ErrorResponse
//	@Failure	502	{object}	httpx.ErrorResponse
//	@Router		/api/gestor/autores/varrer [post]
func (h *gestorHandlers) handleVarrer(w http.ResponseWriter, r *http.Request) {
	res, err := service.NewGestorAutoresService(h.db.SQL, h.db.Encryptor).VarrerAutores(r.Context())
	switch {
	case errors.Is(err, service.ErrGitLabNaoConfigurado):
		jsonError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		jsonErrorLogged(w, r, http.StatusBadGateway, "Falha na varredura; veja os logs do servidor", err)
	default:
		jsonOK(w, res)
	}
}

type autorUpdate struct {
	// Present-but-null unlinks, so presence is tracked via the raw message.
	GestorPessoaID json.RawMessage `json:"gestor_pessoa_id"`
	GestorNome     *string         `json:"gestor_nome"`
	GestorEmail    *string         `json:"gestor_email"`
	Ignorado       *bool           `json:"ignorado"`
}

// handleAutorUpdate links/unlinks and/or (un)ignores one author. The Gestor
// has no lookup by id, so the person's name comes from the stored suggestion
// or from the client (the search result it picked).
//
//	@Summary	Update a GitLab author link
//	@Tags		gestor
//	@Accept		json
//	@Produce	json
//	@Param		id		path		int			true	"Author id"
//	@Param		body	body		autorUpdate	true	"Partial update"
//	@Success	200		{object}	models.GitLabAutor
//	@Router		/api/gestor/autores/{id} [put]
func (h *gestorHandlers) handleAutorUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req autorUpdate
	if !decodeBody(w, r, &req) {
		return
	}
	repo := store.NewGitLabAutorRepo(h.db.SQL)
	a, err := repo.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "Autor não encontrado")
		return
	} else if err != nil {
		jsonServerError(w, r, "failed to load author", err)
		return
	}
	if req.GestorPessoaID != nil {
		if string(req.GestorPessoaID) == "null" {
			err = repo.SetVinculo(r.Context(), id, nil, "", "", 0)
		} else {
			var pid int64
			if json.Unmarshal(req.GestorPessoaID, &pid) != nil || pid <= 0 {
				jsonError(w, http.StatusBadRequest, "gestor_pessoa_id inválido")
				return
			}
			nome, email := "", ""
			if req.GestorNome != nil {
				nome = strings.TrimSpace(*req.GestorNome)
			}
			if req.GestorEmail != nil {
				email = strings.TrimSpace(*req.GestorEmail)
			}
			if a.SugestaoPessoaID != nil && *a.SugestaoPessoaID == pid && nome == "" {
				nome = a.SugestaoNome
			}
			if nome == "" {
				jsonError(w, http.StatusBadRequest, "gestor_nome é obrigatório")
				return
			}
			err = repo.SetVinculo(r.Context(), id, &pid, nome, email, actorID(r))
		}
		if err != nil {
			jsonServerError(w, r, "failed to link author", err)
			return
		}
	}
	if req.Ignorado != nil {
		if err := repo.SetIgnorado(r.Context(), id, *req.Ignorado); err != nil {
			jsonServerError(w, r, "failed to update author", err)
			return
		}
	}
	a, err = repo.Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load author", err)
		return
	}
	jsonOK(w, a)
}

// handlePessoas searches Gestor people by name for the "Escolher…" picker.
// Always 200: a Gestor failure comes back in "error" so the screen degrades.
//
//	@Summary	Search Gestor people
//	@Tags		gestor
//	@Produce	json
//	@Param		q	query		string	false	"Name fragment"
//	@Success	200	{object}	map[string]interface{}
//	@Router		/api/gestor/pessoas [get]
func (h *gestorHandlers) handlePessoas(w http.ResponseWriter, r *http.Request) {
	c, _, err := gestorClientFor(h.db)
	if err != nil {
		jsonServerError(w, r, "failed to read gestor settings", err)
		return
	}
	if c == nil {
		jsonOK(w, map[string]any{"enabled": false, "data": []gestor.Pessoa{}})
		return
	}
	out := map[string]any{"enabled": true, "data": []gestor.Pessoa{}}
	// Empty q: the screen only asks whether the integration is on.
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		jsonOK(w, out)
		return
	}
	ps, err := c.BuscarPessoas(r.Context(), gestor.PessoaQuery{Nome: q, Limite: 20})
	if err != nil {
		log.Printf("[api] GET /api/gestor/pessoas: %v", err)
		out["error"] = "Gestor indisponível"
	} else {
		out["data"] = ps
	}
	jsonOK(w, out)
}

// resolveGestorLinks fills wr.GestorLinks from the request: ids deduped, each
// name fetched from Gestor. Nothing is written here; on failure it answers
// (400 unknown project, 502 Gestor unavailable, 409 already linked elsewhere) and returns false.
func (h *projectHandlers) resolveGestorLinks(w http.ResponseWriter, r *http.Request, projectID int64, in *[]gestorLinkInput, wr *service.ProjectWrite) bool {
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
		// ponytail: one Projeto() call per link, sequential; batch if lists grow large.
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
		// Checked before any write; the UNIQUE index stays the race backstop.
		ids := make([]int64, len(links))
		for i, l := range links {
			ids[i] = l.GestorProjetoID
		}
		taken, err := h.project.GestorLinksOwnedByOthers(r.Context(), projectID, ids)
		if err != nil {
			jsonServerError(w, r, "failed to check gestor links", err)
			return false
		}
		if taken {
			h.gestorInUse(w, store.ErrGestorProjetoEmUso)
			return false
		}
	}
	wr.GestorLinks = &links
	return true
}
