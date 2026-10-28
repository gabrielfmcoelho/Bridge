package gestor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
)

const gClient, gSecret = "gestor-bridge", "g3st0r"

func setup(t *testing.T, h http.HandlerFunc) (*Client, *kcfake.Keycloak) {
	t.Helper()
	fake, kcSrv, _ := kcfake.New(t, "keycloak.example", "apis", nil)
	fake.Mu.Lock()
	fake.Creds[gClient] = gSecret
	fake.Mu.Unlock()
	gw := httptest.NewServer(h)
	t.Cleanup(gw.Close)
	kc := kcadmin.New(kcadmin.Config{BaseURL: "http://keycloak.example", InternalURL: kcSrv.URL,
		HostHeader: "keycloak.example", Realm: "apis"})
	c := NewClient(Settings{Enabled: true, BaseURL: gw.URL, ClientID: gClient, ClientSecret: gSecret}, kc)
	if c == nil {
		t.Fatal("client is nil")
	}
	return c, fake
}

func TestBuscarPessoasPorEmail(t *testing.T) {
	var q map[string][]string
	var auth string
	c, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		q, auth = r.URL.Query(), r.Header.Get("Authorization")
		fmt.Fprint(w, `{"pessoa":[{"id_pessoa":10148,"nome":"Ana","lotacao":"SEAD","cargo":null,"email":"ana@pi.gov.br"}]}`)
	})
	got, err := c.BuscarPessoas(context.Background(), PessoaQuery{Email: "ana@pi.gov.br", Limite: 5})
	if err != nil {
		t.Fatal(err)
	}
	if q["email"][0] != "ana@pi.gov.br" || q["incluir_email"][0] != "true" || q["limite"][0] != "5" {
		t.Errorf("query = %v", q)
	}
	if auth != "Bearer tok-"+gClient+"|"+tokenScope {
		t.Errorf("Authorization = %q", auth)
	}
	if len(got) != 1 || got[0] != (Pessoa{IDPessoa: 10148, Nome: "Ana", Email: "ana@pi.gov.br", Lotacao: "SEAD"}) {
		t.Errorf("got %+v", got)
	}
}

func TestBuscarProjetosLeIdDoDicionario(t *testing.T) {
	var body string
	c, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 200)
		n, _ := r.Body.Read(b)
		body = r.Method + " " + r.URL.Path + " " + string(b[:n])
		fmt.Fprint(w, `{"projetos":{"Bridge":{"id_projeto":25,"concluido":false,"arquivado":true},"Atlas":{"id_projeto":7,"concluido":true,"arquivado":false}}}`)
	})
	got, err := c.BuscarProjetos(context.Background(), "x", 20)
	if err != nil {
		t.Fatal(err)
	}
	want := []Projeto{{7, "Atlas", true, false}, {25, "Bridge", false, true}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v", got)
	}
	if body != `POST /projeto/ {"incluir_encerrados":true,"limite":20,"nome_projeto":"x"}` {
		t.Errorf("request = %s", body)
	}
}

func TestProjetoDetalhe404(t *testing.T) {
	c, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"detail":"Projeto não encontrado"}`)
	})
	_, err := c.Projeto(context.Background(), 99)
	var e *Error
	if !errors.As(err, &e) || e.Status != 404 {
		t.Errorf("err = %v", err)
	}
}

func TestUpstreamErroVira502Erro(t *testing.T) {
	c, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"detail":"pessoa: erro interno (detalhe no log da API)"}`)
	})
	_, err := c.BuscarPessoas(context.Background(), PessoaQuery{Nome: "a"})
	var e *Error
	if !errors.As(err, &e) || e.Status != 500 || e.Message != "pessoa: erro interno (detalhe no log da API)" {
		t.Errorf("err = %v", err)
	}
}

func TestTokenEmCache(t *testing.T) {
	c, fake := setup(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"pessoa":[]}`) })
	for range 2 {
		if _, err := c.BuscarPessoas(context.Background(), PessoaQuery{Nome: "a"}); err != nil {
			t.Fatal(err)
		}
	}
	fake.Mu.Lock()
	defer fake.Mu.Unlock()
	if len(fake.TokenScopes) != 1 {
		t.Errorf("token requests = %d, want 1", len(fake.TokenScopes))
	}
}

func TestNewClientNilQuandoIncompleto(t *testing.T) {
	if NewClient(Settings{BaseURL: "x", ClientID: "a", ClientSecret: "b"}, kcadmin.New(kcadmin.Config{})) != nil {
		t.Error("disabled integration must give a nil client")
	}
}
