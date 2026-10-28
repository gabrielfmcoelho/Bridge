// Package gestor is the client of the SEAD Gestor API (projects and people),
// reached through the gateway with a Keycloak client_credentials token.
package gestor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
)

const tokenScope = "gestor:pessoas gestor:projetos"

// Client talks to Gestor. Nil when the integration is off or incomplete.
type Client struct {
	s    Settings
	kc   *kcadmin.Client
	http *http.Client
}

// NewClient returns nil unless the integration is enabled and fully set.
func NewClient(s Settings, kc *kcadmin.Client) *Client {
	if !s.Enabled || s.BaseURL == "" || s.ClientID == "" || s.ClientSecret == "" || kc == nil {
		return nil
	}
	return &Client{s: s, kc: kc, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	tok, err := c.kc.Token(ctx, c.s.ClientID, c.s.ClientSecret, tokenScope)
	if err != nil {
		return err
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.s.BaseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("gestor: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(raw))
		var e struct {
			Detail any `json:"detail"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Detail != nil {
			msg = fmt.Sprint(e.Detail)
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return &Error{Status: resp.StatusCode, Message: msg}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("gestor: unexpected response: %w", err)
	}
	return nil
}

// BuscarPessoas searches the people directory by name and/or e-mail.
func (c *Client) BuscarPessoas(ctx context.Context, q PessoaQuery) ([]Pessoa, error) {
	v := url.Values{"incluir_email": {"true"}}
	if q.Nome != "" {
		v.Set("pessoa", q.Nome)
	}
	if q.Email != "" {
		v.Set("email", q.Email)
	}
	if q.Limite > 0 {
		v.Set("limite", strconv.Itoa(q.Limite))
	}
	var out struct {
		Pessoa []struct {
			ID      int64   `json:"id_pessoa"`
			Nome    string  `json:"nome"`
			Email   *string `json:"email"`
			Lotacao *string `json:"lotacao"`
		} `json:"pessoa"`
	}
	if err := c.do(ctx, http.MethodGet, "/pessoa/?"+v.Encode(), nil, &out); err != nil {
		return nil, err
	}
	res := make([]Pessoa, 0, len(out.Pessoa))
	for _, p := range out.Pessoa {
		x := Pessoa{IDPessoa: p.ID, Nome: p.Nome}
		if p.Email != nil {
			x.Email = *p.Email
		}
		if p.Lotacao != nil {
			x.Lotacao = *p.Lotacao
		}
		res = append(res, x)
	}
	return res, nil
}

// BuscarProjetos searches projects by name, closed ones included.
func (c *Client) BuscarProjetos(ctx context.Context, nome string, limite int) ([]Projeto, error) {
	var out struct {
		Projetos map[string]struct {
			ID        int64 `json:"id_projeto"`
			Concluido bool  `json:"concluido"`
			Arquivado bool  `json:"arquivado"`
		} `json:"projetos"`
	}
	body := map[string]any{"nome_projeto": nome, "incluir_encerrados": true, "limite": limite}
	if err := c.do(ctx, http.MethodPost, "/projeto/", body, &out); err != nil {
		return nil, err
	}
	res := make([]Projeto, 0, len(out.Projetos))
	for nome, p := range out.Projetos { // the API keys the dictionary by project name
		res = append(res, Projeto{ID: p.ID, Nome: nome, Concluido: p.Concluido, Arquivado: p.Arquivado})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Nome < res[j].Nome })
	return res, nil
}

// Projeto reads one project; *Error with Status 404 when it doesn't exist.
func (c *Client) Projeto(ctx context.Context, id int64) (*Projeto, error) {
	var out struct {
		ID        int64  `json:"id_projeto"`
		Objetivo  string `json:"objetivo"`
		Concluido bool   `json:"concluido"`
		Arquivado bool   `json:"arquivado"`
	}
	if err := c.do(ctx, http.MethodGet, "/projeto/detalhe/"+strconv.FormatInt(id, 10), nil, &out); err != nil {
		return nil, err
	}
	// Gestor's "objetivo" IS the project name.
	return &Projeto{ID: out.ID, Nome: out.Objetivo, Concluido: out.Concluido, Arquivado: out.Arquivado}, nil
}
