package api

import (
	"encoding/json"
	"fmt"
	"testing"
)

func (e *gestorEnv) autor(t *testing.T, email string, sug bool) int64 {
	t.Helper()
	var id int64
	q := `INSERT INTO gitlab_autores (email, nome_gitlab, commits, repos) VALUES (?, 'Ana', 3, 1) RETURNING id`
	if sug {
		q = `INSERT INTO gitlab_autores (email, nome_gitlab, commits, repos, sugestao_pessoa_id, sugestao_nome, sugestao_motivo) VALUES (?, 'Ana', 3, 1, 7, 'Ana Souza', 'email') RETURNING id`
	}
	if err := e.db.QueryRow(q, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *gestorEnv) row(t *testing.T, id int64) (pessoa *int64, nome string, por *int64, ign bool) {
	t.Helper()
	if err := e.db.QueryRow(`SELECT gestor_pessoa_id, gestor_nome, vinculado_por, ignorado FROM gitlab_autores WHERE id = ?`, id).Scan(&pessoa, &nome, &por, &ign); err != nil {
		t.Fatal(err)
	}
	return
}

func TestAutoresListaSoAdmin(t *testing.T) {
	e := newGestorEnv(t, true)
	e.autor(t, "a@x.gov", false)
	for _, rt := range [][2]string{{"GET", "/api/gestor/autores"}, {"POST", "/api/gestor/autores/varrer"}, {"PUT", "/api/gestor/autores/1"}, {"GET", "/api/gestor/pessoas?q=a"}} {
		if code, _ := e.do(e.editor, rt[0], rt[1], `{}`); code != 403 {
			t.Errorf("editor %v = %d, want 403", rt, code)
		}
	}
	code, raw := e.do(e.admin, "GET", "/api/gestor/autores?status=sem_vinculo", "")
	var out struct{ Data []map[string]any }
	_ = json.Unmarshal([]byte(raw), &out)
	if code != 200 || len(out.Data) != 1 {
		t.Fatalf("admin = %d %s", code, raw)
	}
}

func TestAceitarSugestaoEDesfazer(t *testing.T) {
	e := newGestorEnv(t, true)
	id := e.autor(t, "a@x.gov", true)
	path := fmt.Sprintf("/api/gestor/autores/%d", id)
	if code, raw := e.do(e.admin, "PUT", path, `{"gestor_pessoa_id":7}`); code != 200 {
		t.Fatalf("accept = %d %s", code, raw)
	}
	var admID int64
	e.db.QueryRow(`SELECT id FROM users WHERE username='adm'`).Scan(&admID)
	p, nome, por, _ := e.row(t, id)
	if p == nil || *p != 7 || nome != "Ana Souza" || por == nil || *por != admID {
		t.Fatalf("link = %v %q %v", p, nome, por)
	}
	// Another person without a name -> 400; with a name -> linked.
	if code, _ := e.do(e.admin, "PUT", path, `{"gestor_pessoa_id":9}`); code != 400 {
		t.Errorf("no name = %d, want 400", code)
	}
	if code, _ := e.do(e.admin, "PUT", path, `{"gestor_pessoa_id":9,"gestor_nome":"Bia","gestor_email":"b@x.gov"}`); code != 200 {
		t.Errorf("choose = %d", code)
	}
	if p, nome, _, _ := e.row(t, id); *p != 9 || nome != "Bia" {
		t.Errorf("chosen = %v %q", *p, nome)
	}
	if code, _ := e.do(e.admin, "PUT", path, `{"gestor_pessoa_id":null}`); code != 200 {
		t.Fatal("unlink failed")
	}
	if p, nome, por, _ := e.row(t, id); p != nil || nome != "" || por != nil {
		t.Errorf("not unlinked: %v %q %v", p, nome, por)
	}
	if code, _ := e.do(e.admin, "PUT", "/api/gestor/autores/99999", `{"ignorado":true}`); code != 404 {
		t.Errorf("unknown = %d, want 404", code)
	}
}

func TestIgnorarAutor(t *testing.T) {
	e := newGestorEnv(t, true)
	id := e.autor(t, "a@x.gov", false)
	path := fmt.Sprintf("/api/gestor/autores/%d", id)
	if code, _ := e.do(e.admin, "PUT", path, `{"ignorado":true}`); code != 200 {
		t.Fatal("ignore failed")
	}
	if _, _, _, ign := e.row(t, id); !ign {
		t.Error("not ignored")
	}
	if code, _ := e.do(e.admin, "PUT", path, `{"ignorado":false}`); code != 200 {
		t.Fatal("reactivate failed")
	}
	if _, _, _, ign := e.row(t, id); ign {
		t.Error("still ignored")
	}
}

func TestVarrerSemGitLabE400(t *testing.T) {
	e := newGestorEnv(t, true)
	code, raw := e.do(e.admin, "POST", "/api/gestor/autores/varrer", "")
	if code != 400 {
		t.Fatalf("= %d %s", code, raw)
	}
}

func TestPessoas(t *testing.T) {
	e := newGestorEnv(t, true)
	code, raw := e.do(e.admin, "GET", "/api/gestor/pessoas?q=ana", "")
	var out struct {
		Enabled bool
		Data    []map[string]any
	}
	_ = json.Unmarshal([]byte(raw), &out)
	if code != 200 || !out.Enabled || len(out.Data) != 1 || out.Data[0]["nome"] != "Ana Souza" {
		t.Fatalf("= %d %s", code, raw)
	}
	*e.down = true
	code, raw = e.do(e.admin, "GET", "/api/gestor/pessoas?q=ana", "")
	_ = json.Unmarshal([]byte(raw), &out)
	if code != 200 || !out.Enabled || len(out.Data) != 0 {
		t.Fatalf("down = %d %s", code, raw)
	}
	off := newGestorEnv(t, false)
	code, raw = off.do(off.admin, "GET", "/api/gestor/pessoas?q=ana", "")
	if code != 200 || raw == "" {
		t.Fatalf("off = %d %s", code, raw)
	}
	var o2 map[string]any
	_ = json.Unmarshal([]byte(raw), &o2)
	if o2["enabled"] != false {
		t.Errorf("off enabled = %v", o2["enabled"])
	}
}
