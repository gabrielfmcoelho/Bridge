package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

func TestUpsertSomaENaoTocaVinculo(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	r := store.NewGitLabAutorRepo(d.SQL)
	now := time.Now()
	novo, err := r.UpsertContagem(ctx, " A@X.com ", "Ana", 3, 1, now)
	if err != nil || !novo {
		t.Fatalf("novo=%v err=%v", novo, err)
	}
	all, _ := r.List(ctx, "todos", "")
	id := all[0].ID
	pid := int64(7)
	var uid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('adm','x','admin') RETURNING id`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVinculo(ctx, id, &pid, "Ana G", "ana@pi.gov.br", uid); err != nil {
		t.Fatal(err)
	}
	if err := r.SetIgnorado(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	novo, err = r.UpsertContagem(ctx, "a@x.com", "Ana B", 10, 2, now)
	if err != nil || novo {
		t.Fatalf("novo=%v err=%v", novo, err)
	}
	all, _ = r.List(ctx, "todos", "")
	a := all[0]
	if len(all) != 1 || a.Commits != 10 || a.Repos != 2 || a.NomeGitLab != "Ana B" ||
		a.GestorPessoaID == nil || *a.GestorPessoaID != 7 || !a.Ignorado || a.VinculadoPor == nil || *a.VinculadoPor != uid {
		t.Errorf("row = %+v", a)
	}
	if err := r.SetVinculo(ctx, id, nil, "", "", 0); err != nil {
		t.Fatal(err)
	}
	all, _ = r.List(ctx, "todos", "")
	if all[0].GestorPessoaID != nil || all[0].GestorNome != "" || all[0].VinculadoPor != nil {
		t.Errorf("unlink left %+v", all[0])
	}
}

func TestListFiltros(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	r := store.NewGitLabAutorRepo(d.SQL)
	now := time.Now()
	for _, e := range []string{"livre@x", "sug@x", "vinc@x", "ign@x"} {
		if _, err := r.UpsertContagem(ctx, e, "N "+e, 1, 1, now); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := r.List(ctx, "todos", "")
	id := map[string]int64{}
	for _, a := range all {
		id[a.Email] = a.ID
	}
	_ = r.SetSugestao(ctx, id["sug@x"], 5, "Sug", "email")
	p := int64(9)
	var uid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('adm','x','admin') RETURNING id`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVinculo(ctx, id["vinc@x"], &p, "V", "v@pi", uid); err != nil {
		t.Fatal(err)
	}
	_ = r.SetIgnorado(ctx, id["ign@x"], true)

	want := map[string]int{"sem_vinculo": 2, "com_sugestao": 1, "vinculados": 1, "ignorados": 1, "todos": 4}
	for st, n := range want {
		got, err := r.List(ctx, st, "")
		if err != nil || len(got) != n {
			t.Errorf("%s: got %d (err %v), want %d", st, len(got), err, n)
		}
	}
	if got, _ := r.List(ctx, "todos", "SUG"); len(got) != 1 {
		t.Errorf("q filter got %d", len(got))
	}
	pend, _ := r.PendentesDeSugestao(ctx)
	if len(pend) != 1 || pend[0].Email != "livre@x" {
		t.Errorf("pendentes = %+v", pend)
	}
}

func TestPendentesPulaQuemFoiTentadoNaSemana(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	r := store.NewGitLabAutorRepo(d.SQL)
	now := time.Now()
	for _, e := range []string{"novo@x.com", "recente@x.com", "antigo@x.com"} {
		if _, err := r.UpsertContagem(ctx, e, "N", 1, 1, now); err != nil {
			t.Fatal(err)
		}
	}
	ids := map[string]int64{}
	all, _ := r.List(ctx, "todos", "")
	for _, a := range all {
		ids[a.Email] = a.ID
	}
	if err := r.MarcarTentativa(ctx, ids["recente@x.com"]); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL.Exec(`UPDATE gitlab_autores SET sugestao_tentada_em = CURRENT_TIMESTAMP - INTERVAL '8 days' WHERE email = 'antigo@x.com'`); err != nil {
		t.Fatal(err)
	}
	pend, err := r.PendentesDeSugestao(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, a := range pend {
		got[a.Email] = true
	}
	if len(got) != 2 || !got["novo@x.com"] || !got["antigo@x.com"] {
		t.Errorf("pendentes = %v", got)
	}
}
