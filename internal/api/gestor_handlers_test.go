package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/kcadmin/kcfake"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type gestorEnv struct {
	do        func(tok, method, path, body string) (int, string)
	admin     string // session token
	editor    string // session token
	db        *sql.DB
	queries   *[]string // "nome_projeto" values searched
	down      *bool     // Gestor answers 500
	projects  map[int64]string
	newProj   func(name string) int64
	newHidden func(name string) int64
}

// newGestorEnv: router + fake Keycloak + fake Gestor (nome por id), integration
// enabled when on is true.
func newGestorEnv(t *testing.T, on bool) *gestorEnv {
	t.Helper()
	d, err := dbtest.Open(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	router := NewRouter(d, "/tmp/sshcm-test-config")
	fake, kcSrv, _ := kcfake.New(t, "keycloak.example", "apis", nil)
	fake.Creds["gestor-bridge"] = "g3st0r"
	e := &gestorEnv{queries: &[]string{}, down: new(bool), projects: map[int64]string{10: "Alfa", 11: "Beta"}}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *e.down {
			w.WriteHeader(500)
			fmt.Fprint(w, `{"detail":"boom"}`)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/projeto/":
			var b struct {
				Nome   string `json:"nome_projeto"`
				Limite int    `json:"limite"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			*e.queries = append(*e.queries, fmt.Sprintf("%s|%d", b.Nome, b.Limite))
			fmt.Fprint(w, `{"projetos":{"Alfa":{"id_projeto":10,"concluido":true,"arquivado":false}}}`)
		case r.Method == "GET" && r.URL.Path == "/pessoa/":
			fmt.Fprint(w, `{"pessoa":[{"id_pessoa":7,"nome":"Ana Souza","email":"ana@x.gov","lotacao":"SEAD"}]}`)
		case strings.HasPrefix(r.URL.Path, "/projeto/detalhe/"):
			var id int64
			fmt.Sscan(strings.TrimPrefix(r.URL.Path, "/projeto/detalhe/"), &id)
			nome, ok := e.projects[id]
			if !ok {
				w.WriteHeader(404)
				fmt.Fprint(w, `{"detail":"nao existe"}`)
				return
			}
			fmt.Fprintf(w, `{"id_projeto":%d,"objetivo":%q,"concluido":false,"arquivado":false}`, id, nome)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(gw.Close)

	mk := func(name, role string) string {
		u := &models.User{Username: name, Role: role, AuthProvider: "local"}
		if err := store.NewUserRepo(d.SQL).Create(t.Context(), u); err != nil {
			t.Fatal(err)
		}
		tok, _, _ := auth.CreateSession(d.SQL, u.ID)
		return tok
	}
	admin, editor := mk("adm", "admin"), mk("ed", "editor")
	e.editor, e.admin = editor, admin
	e.do = func(tok, method, path, body string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "sshcm_session", Value: tok})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	for path, body := range map[string]string{
		"/api/settings/integrations/keycloak_apis": fmt.Sprintf(`{"kc_apis_base_url":"http://keycloak.example","kc_apis_internal_url":%q,"kc_apis_host_header":"keycloak.example"}`, kcSrv.URL),
		"/api/settings/integrations/gestor":        fmt.Sprintf(`{"gestor_base_url":%q,"gestor_client_id":"gestor-bridge","gestor_client_secret":"g3st0r","gestor_integration_enabled":"%v"}`, gw.URL, on),
	} {
		if code, raw := e.do(admin, "PUT", path, body); code != 200 {
			t.Fatalf("PUT %s = %d %s", path, code, raw)
		}
	}
	// The editor belongs to sga; newProj grants the project to sga (no grant = admin-only).
	var uid, sga int64
	d.SQL.QueryRow(`SELECT id FROM users WHERE username = 'ed'`).Scan(&uid)
	d.SQL.QueryRow(`SELECT id FROM entidades WHERE slug = 'sga'`).Scan(&sga)
	if _, err := d.SQL.Exec(`INSERT INTO user_entidades (user_id, entidade_id, is_primary) VALUES (?, ?, TRUE)`, uid, sga); err != nil {
		t.Fatal(err)
	}
	insert := func(name string, grant bool) int64 {
		var id int64
		if err := d.SQL.QueryRow(`INSERT INTO projects (name) VALUES (?) RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if grant {
			if err := store.NewAssetEntidadeRepo(d.SQL).Replace(t.Context(), d.SQL, store.AssetProject, id, models.AssetGrants{CreatorEntidadeID: &sga}); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	e.newProj = func(name string) int64 { return insert(name, true) }
	e.newHidden = func(name string) int64 { return insert(name, false) }
	e.db = d.SQL
	return e
}

func (e *gestorEnv) links(t *testing.T, id int64) []models.ProjectGestorLink {
	t.Helper()
	code, raw := e.do(e.editor, "GET", fmt.Sprintf("/api/projects/%d", id), "")
	if code != 200 {
		t.Fatalf("GET project = %d %s", code, raw)
	}
	var out struct {
		GestorLinks []models.ProjectGestorLink `json:"gestor_links"`
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out.GestorLinks
}

func TestPutProjetoGestorLinksGravaNome(t *testing.T) {
	e := newGestorEnv(t, true)
	id := e.newProj("p")
	path := fmt.Sprintf("/api/projects/%d", id)
	code, raw := e.do(e.editor, "PUT", path, `{"gestor_links":[{"gestor_projeto_id":10},{"gestor_projeto_id":11},{"gestor_projeto_id":10}]}`)
	if code != 200 {
		t.Fatalf("PUT = %d %s", code, raw)
	}
	got := e.links(t, id)
	if len(got) != 2 || got[0].GestorProjetoNome != "Alfa" || got[1].GestorProjetoNome != "Beta" {
		t.Errorf("links = %+v", got)
	}
	// Gestor unknown id -> 400, Gestor down -> 502; both leave the links as they were.
	if code, _ := e.do(e.editor, "PUT", path, `{"gestor_links":[{"gestor_projeto_id":99}]}`); code != 400 {
		t.Errorf("unknown gestor project = %d, want 400", code)
	}
	*e.down = true
	if code, _ := e.do(e.editor, "PUT", path, `{"gestor_links":[{"gestor_projeto_id":10}]}`); code != 502 {
		t.Errorf("gestor down = %d, want 502", code)
	}
	*e.down = false
	if got := e.links(t, id); len(got) != 2 {
		t.Errorf("links changed on failure: %+v", got)
	}
	// [] clears.
	if code, _ := e.do(e.editor, "PUT", path, `{"gestor_links":[]}`); code != 200 || len(e.links(t, id)) != 0 {
		t.Errorf("clear failed")
	}
}

func TestPutGestorLinksNilMantem(t *testing.T) {
	e := newGestorEnv(t, true)
	id := e.newProj("p")
	path := fmt.Sprintf("/api/projects/%d", id)
	e.do(e.editor, "PUT", path, `{"gestor_links":[{"gestor_projeto_id":10}]}`)
	*e.down = true // an omitted field must not even call Gestor
	if code, raw := e.do(e.editor, "PUT", path, `{"description":"x"}`); code != 200 {
		t.Fatalf("PUT = %d %s", code, raw)
	}
	if got := e.links(t, id); len(got) != 1 {
		t.Errorf("links = %+v, want kept", got)
	}
}

func TestPutGestorLinks409Generico(t *testing.T) {
	e := newGestorEnv(t, true)
	a, b := e.newProj("p-a"), e.newProj("segredo-do-outro")
	e.do(e.editor, "PUT", fmt.Sprintf("/api/projects/%d", b), `{"gestor_links":[{"gestor_projeto_id":10}]}`)
	code, raw := e.do(e.editor, "PUT", fmt.Sprintf("/api/projects/%d", a), `{"description":"mudou","gestor_links":[{"gestor_projeto_id":10}]}`)
	var desc string
	e.db.QueryRow(`SELECT description FROM projects WHERE id = ?`, a).Scan(&desc)
	if desc == "mudou" {
		t.Error("409 left the project's other fields written")
	}
	if code != 409 {
		t.Fatalf("PUT = %d %s, want 409", code, raw)
	}
	if !strings.Contains(raw, "já está ligado a outro projeto do Bridge") {
		t.Errorf("message = %s", raw)
	}
	if strings.Contains(raw, "segredo-do-outro") || strings.Contains(raw, fmt.Sprint(b)+`"`) {
		t.Errorf("409 leaks the other project: %s", raw)
	}
}

func TestGestorProjetosBuscaRepassaQ(t *testing.T) {
	e := newGestorEnv(t, true)
	code, raw := e.do(e.editor, "GET", "/api/gestor/projetos?q=alf", "")
	if code != 200 {
		t.Fatalf("GET = %d %s", code, raw)
	}
	var out struct {
		Enabled, Configured bool
		Data                []struct {
			ID        int64  `json:"id"`
			Nome      string `json:"nome"`
			Concluido bool   `json:"concluido"`
		}
		Error string
	}
	_ = json.Unmarshal([]byte(raw), &out)
	if !out.Enabled || !out.Configured || len(out.Data) != 1 || out.Data[0].ID != 10 || out.Data[0].Nome != "Alfa" || !out.Data[0].Concluido {
		t.Errorf("out = %s", raw)
	}
	if len(*e.queries) != 1 || (*e.queries)[0] != "alf|20" {
		t.Errorf("queries = %v, want [alf|20]", *e.queries)
	}
	*e.down = true
	code, raw = e.do(e.editor, "GET", "/api/gestor/projetos?q=x", "")
	_ = json.Unmarshal([]byte(raw), &out)
	if code != 200 || out.Error == "" || len(out.Data) != 0 || !out.Enabled {
		t.Errorf("upstream error = %d %s", code, raw)
	}
}

func TestGestorProjetosDesligadoDevolveEnabledFalse(t *testing.T) {
	e := newGestorEnv(t, false)
	code, raw := e.do(e.editor, "GET", "/api/gestor/projetos?q=x", "")
	if code != 200 || !strings.Contains(raw, `"enabled":false`) || !strings.Contains(raw, `"configured":false`) || !strings.Contains(raw, `"data":[]`) {
		t.Errorf("= %d %s", code, raw)
	}
	if len(*e.queries) != 0 {
		t.Errorf("called Gestor while disabled")
	}
}

func TestGestorLinksProjetoInvisivel404(t *testing.T) {
	e := newGestorEnv(t, true)
	hidden, db := e.newHidden("oculto"), e.db
	code, _ := e.do(e.editor, "PUT", fmt.Sprintf("/api/projects/%d", hidden), `{"gestor_links":[{"gestor_projeto_id":10}]}`)
	if code != 404 {
		t.Errorf("PUT hidden = %d, want 404", code)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM project_gestor_links WHERE project_id = ?`, hidden).Scan(&n)
	if n != 0 {
		t.Errorf("wrote %d links on a hidden project", n)
	}
}

func TestPostProjetoGestorLinks(t *testing.T) {
	e := newGestorEnv(t, true)
	code, raw := e.do(e.editor, "POST", "/api/projects", `{"name":"novo","gestor_links":[{"gestor_projeto_id":10}]}`)
	if code != 201 {
		t.Fatalf("POST = %d %s", code, raw)
	}
	var p struct{ ID int64 }
	_ = json.Unmarshal([]byte(raw), &p)
	if got := e.links(t, p.ID); len(got) != 1 || got[0].GestorProjetoNome != "Alfa" {
		t.Errorf("links = %+v", got)
	}
	// 10 is taken now: a second create must 409 and leave no project behind.
	code, raw = e.do(e.editor, "POST", "/api/projects", `{"name":"duplicado","gestor_links":[{"gestor_projeto_id":10}]}`)
	if code != 409 {
		t.Fatalf("POST dup = %d %s, want 409", code, raw)
	}
	var n int
	e.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE name = 'duplicado'`).Scan(&n)
	if n != 0 {
		t.Errorf("409 left %d project rows", n)
	}
}
