package service_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gestor"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/gitlab"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
)

// fakeGitLab: group "g" has repos 1 (a/one) and 2 (a/two) and 3 (a/forbidden).
func fakeGitLab(t *testing.T, contrib map[string]string) *gitlab.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/api/v4/groups/g/projects":
			if r.URL.Query().Get("page") > "1" {
				fmt.Fprint(w, `[]`)
				return
			}
			fmt.Fprint(w, `[{"id":1,"path_with_namespace":"a/one"},{"id":2,"path_with_namespace":"a/two"},{"id":3,"path_with_namespace":"a/forbidden"}]`)
		case strings.HasSuffix(p, "/repository/contributors"):
			id := strings.Split(p, "/")[4]
			if id == "3" {
				http.Error(w, `{"message":"403 Forbidden"}`, http.StatusForbidden)
				return
			}
			fmt.Fprint(w, contrib[id])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return gitlab.NewClient(srv.URL, "tok")
}

func fakeGestor(t *testing.T, h func(q map[string][]string) string) *gestor.Client {
	t.Helper()
	fake, kcSrv, _ := kcfake.New(t, "keycloak.example", "apis", nil)
	fake.Mu.Lock()
	fake.Creds["gc"] = "gs"
	fake.Mu.Unlock()
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, h(r.URL.Query()))
	}))
	t.Cleanup(gw.Close)
	kc := kcadmin.New(kcadmin.Config{BaseURL: "http://keycloak.example", InternalURL: kcSrv.URL, HostHeader: "keycloak.example", Realm: "apis"})
	return gestor.NewClient(gestor.Settings{Enabled: true, BaseURL: gw.URL, ClientID: "gc", ClientSecret: "gs"}, kc)
}

const nenhum = `{"pessoa":[]}`

func pessoa(id int, nome string) string {
	return fmt.Sprintf(`{"id_pessoa":%d,"nome":%q,"email":null,"lotacao":null}`, id, nome)
}

var contribDefault = map[string]string{
	"1": `[{"name":"Ana","email":"A@x.com","commits":3},{"name":"Bia","email":"b@x.com","commits":1}]`,
	"2": `[{"name":"Ana Maria","email":"a@x.com","commits":5}]`,
}

func TestVarrerAgregaPorEmailMinusculo(t *testing.T) {
	ctx := context.Background()
	d, _ := dbtest.Open(t)
	defer d.Close()
	svc := service.NewGestorAutoresServiceWith(d.SQL, fakeGitLab(t, contribDefault), nil, []string{"g"})
	res, err := svc.VarrerAutores(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Repos != 3 || res.AutoresNovos != 2 || res.AutoresAtualizados != 0 {
		t.Errorf("res = %+v", res)
	}
	as, _ := store.NewGitLabAutorRepo(d.SQL).List(ctx, "todos", "a@x.com")
	if len(as) != 1 || as[0].Commits != 8 || as[0].Repos != 2 || as[0].NomeGitLab != "Ana Maria" || as[0].VistoEm == nil {
		t.Errorf("a@x.com = %+v", as)
	}
}

func TestVarrerRepoComErroSegue(t *testing.T) {
	d, _ := dbtest.Open(t)
	defer d.Close()
	svc := service.NewGestorAutoresServiceWith(d.SQL, fakeGitLab(t, contribDefault), nil, []string{"g"})
	res, err := svc.VarrerAutores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range res.Erros {
		found = found || strings.HasPrefix(e, "a/forbidden:")
	}
	if !found || res.AutoresNovos != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestSugestaoPorEmailDepoisNome(t *testing.T) {
	ctx := context.Background()
	d, _ := dbtest.Open(t)
	defer d.Close()
	var nomeQ string
	gs := fakeGestor(t, func(q map[string][]string) string {
		if e := q["email"]; len(e) > 0 {
			if e[0] == "a@x.com" {
				return `{"pessoa":[` + pessoa(11, "Ana Silva") + `]}`
			}
			return nenhum
		}
		nomeQ = q["pessoa"][0]
		return `{"pessoa":[` + pessoa(22, "Bia Açaí") + `]}`
	})
	contrib := map[string]string{"1": `[{"name":"  Bía   AÇAÍ ","email":"b@x.com","commits":1},{"name":"Ana","email":"a@x.com","commits":2}]`}
	svc := service.NewGestorAutoresServiceWith(d.SQL, fakeGitLab(t, contrib), gs, []string{"g"})
	res, err := svc.VarrerAutores(ctx)
	if err != nil || res.Sugestoes != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if nomeQ != "bia acai" {
		t.Errorf("name query = %q", nomeQ)
	}
	r := store.NewGitLabAutorRepo(d.SQL)
	as, _ := r.List(ctx, "todos", "")
	got := map[string]string{}
	for _, a := range as {
		if a.GestorPessoaID != nil {
			t.Errorf("suggestion became a link: %+v", a)
		}
		got[a.Email] = fmt.Sprint(*a.SugestaoPessoaID, a.SugestaoMotivo)
	}
	if got["a@x.com"] != "11email" || got["b@x.com"] != "22nome" {
		t.Errorf("got %v", got)
	}
}

func TestNomeAmbiguoNaoSugere(t *testing.T) {
	ctx := context.Background()
	d, _ := dbtest.Open(t)
	defer d.Close()
	gs := fakeGestor(t, func(q map[string][]string) string {
		if len(q["email"]) > 0 {
			return nenhum
		}
		return `{"pessoa":[` + pessoa(1, "Ana A") + `,` + pessoa(2, "Ana B") + `]}`
	})
	svc := service.NewGestorAutoresServiceWith(d.SQL, fakeGitLab(t, contribDefault), gs, []string{"g"})
	res, err := svc.VarrerAutores(ctx)
	if err != nil || res.Sugestoes != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestGestorIndisponivelMantemContagem(t *testing.T) {
	ctx := context.Background()
	d, _ := dbtest.Open(t)
	defer d.Close()
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer gw.Close()
	fake, kcSrv, _ := kcfake.New(t, "keycloak.example", "apis", nil)
	fake.Mu.Lock()
	fake.Creds["gc"] = "gs"
	fake.Mu.Unlock()
	kc := kcadmin.New(kcadmin.Config{BaseURL: "http://keycloak.example", InternalURL: kcSrv.URL, HostHeader: "keycloak.example", Realm: "apis"})
	gs := gestor.NewClient(gestor.Settings{Enabled: true, BaseURL: gw.URL, ClientID: "gc", ClientSecret: "gs"}, kc)
	svc := service.NewGestorAutoresServiceWith(d.SQL, fakeGitLab(t, contribDefault), gs, []string{"g"})
	res, err := svc.VarrerAutores(ctx)
	if err != nil || res.AutoresNovos != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	var n int
	for _, e := range res.Erros {
		if strings.HasPrefix(e, "gestor indisponível: ") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("erros = %v", res.Erros)
	}
}

func TestGitLabNaoConfiguradoErra(t *testing.T) {
	d, _ := dbtest.Open(t)
	defer d.Close()
	if _, err := service.NewGestorAutoresServiceWith(d.SQL, nil, nil, []string{"g"}).VarrerAutores(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestRevarrerMantemVinculoEIgnorado(t *testing.T) {
	ctx := context.Background()
	d, _ := dbtest.Open(t)
	defer d.Close()
	svc := service.NewGestorAutoresServiceWith(d.SQL, fakeGitLab(t, contribDefault), nil, []string{"g"})
	if _, err := svc.VarrerAutores(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`UPDATE gitlab_autores SET gestor_pessoa_id = 9, gestor_nome = 'X' WHERE email = 'a@x.com'`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`UPDATE gitlab_autores SET ignorado = true WHERE email = 'b@x.com'`); err != nil {
		t.Fatal(err)
	}
	res, err := svc.VarrerAutores(ctx)
	if err != nil || res.AutoresNovos != 0 || res.AutoresAtualizados != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	as, _ := store.NewGitLabAutorRepo(d.SQL).List(ctx, "todos", "")
	for _, a := range as {
		if a.Email == "a@x.com" && (a.GestorPessoaID == nil || a.GestorNome != "X") {
			t.Errorf("link lost: %+v", a)
		}
		if a.Email == "b@x.com" && !a.Ignorado {
			t.Errorf("ignorado lost: %+v", a)
		}
	}
}
