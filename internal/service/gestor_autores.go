package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gestor"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// ErrGitLabNaoConfigurado: the scan cannot start (the handler answers 400).
var ErrGitLabNaoConfigurado = errors.New("GitLab não configurado")

// ResumoVarredura is the outcome of VarrerAutores.
type ResumoVarredura struct {
	Repos              int      `json:"repos"`
	AutoresNovos       int      `json:"autores_novos"`
	AutoresAtualizados int      `json:"autores_atualizados"`
	Sugestoes          int      `json:"sugestoes"`
	Erros              []string `json:"erros"`
}

// fontesAutores are the scan's collaborators; gs is nil when Gestor is off.
type fontesAutores struct {
	gl     *gitlab.Client
	gs     *gestor.Client
	grupos []string
}

// GestorAutoresService scans GitLab commit authors into gitlab_autores and
// stores Gestor-user suggestions (never links).
type GestorAutoresService struct {
	repo *store.GitLabAutorRepo
	load func() (fontesAutores, error)
	// pausa precedes every Gestor lookup, to stay under the Bridge key's
	// 600 req/min limit.
	// ponytail: 150ms ≈ 400 lookups/min; add a sugestao_tentada_em column
	// (skip recently tried no-match rows) if scans become too slow.
	pausa time.Duration
}

func (s *GestorAutoresService) esperar(ctx context.Context) error {
	if s.pausa <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.pausa):
		return nil
	}
}

// NewGestorAutoresService resolves the clients from settings on each scan.
func NewGestorAutoresService(db *sql.DB, enc *database.Encryptor) *GestorAutoresService {
	return &GestorAutoresService{repo: store.NewGitLabAutorRepo(db), pausa: 150 * time.Millisecond, load: func() (fontesAutores, error) {
		gls, err := gitlab.LoadSettings(db, enc)
		if err != nil {
			return fontesAutores{}, err
		}
		if !gls.Enabled {
			return fontesAutores{}, fmt.Errorf("%w: integração desativada", ErrGitLabNaoConfigurado)
		}
		gl := gitlab.NewServiceClient(gls)
		if gl == nil {
			return fontesAutores{}, fmt.Errorf("%w: sem token de serviço", ErrGitLabNaoConfigurado)
		}
		gss, err := gestor.LoadSettings(db, enc)
		if err != nil {
			return fontesAutores{}, err
		}
		return fontesAutores{gl: gl, gs: gestor.NewClient(gss, gestor.KeycloakFromSettings(db)), grupos: gss.GitLabGrupos}, nil
	}}
}

// NewGestorAutoresServiceWith uses fixed clients (tests). gs may be nil.
func NewGestorAutoresServiceWith(db *sql.DB, gl *gitlab.Client, gs *gestor.Client, grupos []string, pausa time.Duration) *GestorAutoresService {
	return &GestorAutoresService{repo: store.NewGitLabAutorRepo(db), pausa: pausa, load: func() (fontesAutores, error) {
		if gl == nil {
			return fontesAutores{}, ErrGitLabNaoConfigurado
		}
		return fontesAutores{gl: gl, gs: gs, grupos: grupos}, nil
	}}
}

type acumulado struct {
	commits, repos int
	nomes          map[string]int
}

func (a *acumulado) nome() (best string) {
	max := -1
	for n, c := range a.nomes {
		if c > max || (c == max && n < best) {
			best, max = n, c
		}
	}
	return best
}

// VarrerAutores lists every repo of the configured groups, aggregates the
// contributors per lowercase e-mail, upserts the counts and suggests Gestor
// users for rows still without link or suggestion.
func (s *GestorAutoresService) VarrerAutores(ctx context.Context) (ResumoVarredura, error) {
	var res ResumoVarredura
	f, err := s.load()
	if err != nil {
		return res, err
	}
	res.Erros = []string{}

	var projs []gitlab.Project
	seen := map[int]bool{}
	for _, g := range f.grupos {
		ps, err := f.gl.ListGroupProjectsAll(g, true)
		if err != nil {
			return res, fmt.Errorf("listar projetos do grupo %s: %w", g, err)
		}
		for _, p := range ps {
			if !seen[p.ID] {
				seen[p.ID] = true
				projs = append(projs, p)
			}
		}
	}
	res.Repos = len(projs)

	var mu sync.Mutex
	agg := map[string]*acumulado{}
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for _, p := range projs {
		if ctx.Err() != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			cs, err := f.gl.ListContributors(p.ID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Erros = append(res.Erros, fmt.Sprintf("%s: %v", p.PathWithNamespace, err))
				return
			}
			inRepo := map[string]bool{}
			for _, c := range cs {
				e := strings.ToLower(strings.TrimSpace(c.Email))
				if e == "" {
					continue
				}
				a := agg[e]
				if a == nil {
					a = &acumulado{nomes: map[string]int{}}
					agg[e] = a
				}
				a.commits += c.Commits
				a.nomes[strings.TrimSpace(c.Name)] += c.Commits
				if !inRepo[e] {
					inRepo[e] = true
					a.repos++
				}
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return res, err
	}

	now := time.Now()
	for e, a := range agg {
		novo, err := s.repo.UpsertContagem(ctx, e, a.nome(), a.commits, a.repos, now)
		if err != nil {
			return res, err
		}
		if novo {
			res.AutoresNovos++
		} else {
			res.AutoresAtualizados++
		}
	}

	if f.gs == nil {
		res.Erros = append(res.Erros, "gestor indisponível: integração desativada ou incompleta")
		return res, nil
	}
	pend, err := s.repo.PendentesDeSugestao(ctx)
	if err != nil {
		return res, err
	}
	for _, a := range pend {
		pessoa, motivo, err := s.sugerir(ctx, f.gs, a.Email, a.NomeGitLab)
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if err != nil {
			log.Printf("[gestor] sugestão de autores: %v", err)
			res.Erros = append(res.Erros, "gestor indisponível")
			break
		}
		if pessoa == nil {
			continue
		}
		if err := s.repo.SetSugestao(ctx, a.ID, pessoa.IDPessoa, pessoa.Nome, motivo); err != nil {
			return res, err
		}
		res.Sugestoes++
	}
	return res, nil
}

// sugerir: by e-mail first, then by normalized full name; only a single
// result counts as a suggestion.
func (s *GestorAutoresService) sugerir(ctx context.Context, gs *gestor.Client, email, nome string) (*gestor.Pessoa, string, error) {
	if err := s.esperar(ctx); err != nil {
		return nil, "", err
	}
	ps, err := gs.BuscarPessoas(ctx, gestor.PessoaQuery{Email: email, Limite: 5})
	if err != nil {
		return nil, "", err
	}
	if len(ps) == 1 {
		return &ps[0], "email", nil
	}
	n := normalizarNome(nome)
	if n == "" {
		return nil, "", nil
	}
	if err := s.esperar(ctx); err != nil {
		return nil, "", err
	}
	ps, err = gs.BuscarPessoas(ctx, gestor.PessoaQuery{Nome: n, Limite: 5})
	if err != nil {
		return nil, "", err
	}
	if len(ps) == 1 {
		return &ps[0], "nome", nil
	}
	return nil, "", nil
}

// normalizarNome lowercases, strips accents and collapses spaces.
func normalizarNome(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
