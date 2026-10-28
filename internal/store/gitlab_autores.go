package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// GitLabAutorRepo owns SQL for gitlab_autores.
type GitLabAutorRepo struct {
	db *sql.DB
}

func NewGitLabAutorRepo(db *sql.DB) *GitLabAutorRepo { return &GitLabAutorRepo{db: db} }

const gitlabAutorSelect = `SELECT id, email, nome_gitlab, commits, repos, visto_em,
	sugestao_pessoa_id, sugestao_nome, sugestao_motivo,
	gestor_pessoa_id, gestor_nome, gestor_email, vinculado_por, vinculado_em,
	ignorado, criado_em, atualizado_em FROM gitlab_autores`

func (r *GitLabAutorRepo) query(ctx context.Context, where string, args ...any) ([]models.GitLabAutor, error) {
	rows, err := r.db.QueryContext(ctx, gitlabAutorSelect+where+` ORDER BY commits DESC, email`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.GitLabAutor{}
	for rows.Next() {
		var a models.GitLabAutor
		if err := rows.Scan(&a.ID, &a.Email, &a.NomeGitLab, &a.Commits, &a.Repos, &a.VistoEm,
			&a.SugestaoPessoaID, &a.SugestaoNome, &a.SugestaoMotivo,
			&a.GestorPessoaID, &a.GestorNome, &a.GestorEmail, &a.VinculadoPor, &a.VinculadoEm,
			&a.Ignorado, &a.CriadoEm, &a.AtualizadoEm); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpsertContagem sets the scan counts of an e-mail (lowercased here). Link,
// ignore flag and suggestion are never touched.
func (r *GitLabAutorRepo) UpsertContagem(ctx context.Context, email, nome string, commits, repos int, visto time.Time) (bool, error) {
	var novo bool
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO gitlab_autores (email, nome_gitlab, commits, repos, visto_em)
		VALUES (LOWER(TRIM(?)), ?, ?, ?, ?)
		ON CONFLICT (email) DO UPDATE SET nome_gitlab = EXCLUDED.nome_gitlab, commits = EXCLUDED.commits,
			repos = EXCLUDED.repos, visto_em = EXCLUDED.visto_em, atualizado_em = CURRENT_TIMESTAMP
		RETURNING (xmax = 0)`, email, nome, commits, repos, visto).Scan(&novo)
	return novo, err
}

// PendentesDeSugestao lists rows with no link, not ignored, no suggestion yet
// and not asked about in the last 7 days.
func (r *GitLabAutorRepo) PendentesDeSugestao(ctx context.Context) ([]models.GitLabAutor, error) {
	return r.query(ctx, ` WHERE gestor_pessoa_id IS NULL AND NOT ignorado AND sugestao_pessoa_id IS NULL
		AND (sugestao_tentada_em IS NULL OR sugestao_tentada_em < CURRENT_TIMESTAMP - INTERVAL '7 days')`)
}

func (r *GitLabAutorRepo) SetSugestao(ctx context.Context, id, pessoaID int64, nome, motivo string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE gitlab_autores SET sugestao_pessoa_id = ?, sugestao_nome = ?, sugestao_motivo = ?,
		sugestao_tentada_em = CURRENT_TIMESTAMP, atualizado_em = CURRENT_TIMESTAMP WHERE id = ?`, pessoaID, nome, motivo, id)
	return err
}

// MarcarTentativa records that the scan asked Gestor about this e-mail.
func (r *GitLabAutorRepo) MarcarTentativa(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE gitlab_autores SET sugestao_tentada_em = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// SetVinculo links the author to a Gestor person; nil pessoaID unlinks.
func (r *GitLabAutorRepo) SetVinculo(ctx context.Context, id int64, pessoaID *int64, nome, email string, por int64) error {
	if pessoaID == nil {
		_, err := r.db.ExecContext(ctx, `UPDATE gitlab_autores SET gestor_pessoa_id = NULL, gestor_nome = '', gestor_email = '',
			vinculado_por = NULL, vinculado_em = NULL, atualizado_em = CURRENT_TIMESTAMP WHERE id = ?`, id)
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE gitlab_autores SET gestor_pessoa_id = ?, gestor_nome = ?, gestor_email = ?,
		vinculado_por = ?, vinculado_em = CURRENT_TIMESTAMP, atualizado_em = CURRENT_TIMESTAMP WHERE id = ?`,
		*pessoaID, nome, email, por, id)
	return err
}

func (r *GitLabAutorRepo) SetIgnorado(ctx context.Context, id int64, v bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE gitlab_autores SET ignorado = ?, atualizado_em = CURRENT_TIMESTAMP WHERE id = ?`, v, id)
	return err
}

// List filters by status (sem_vinculo, com_sugestao, vinculados, ignorados,
// todos) and an optional e-mail/name substring.
func (r *GitLabAutorRepo) List(ctx context.Context, status, q string) ([]models.GitLabAutor, error) {
	conds := []string{}
	switch status {
	case "sem_vinculo":
		conds = append(conds, "gestor_pessoa_id IS NULL AND NOT ignorado")
	case "com_sugestao":
		conds = append(conds, "gestor_pessoa_id IS NULL AND NOT ignorado AND sugestao_pessoa_id IS NOT NULL")
	case "vinculados":
		conds = append(conds, "gestor_pessoa_id IS NOT NULL")
	case "ignorados":
		conds = append(conds, "ignorado")
	}
	var args []any
	if q != "" {
		conds = append(conds, "(email ILIKE ? OR nome_gitlab ILIKE ?)")
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	where := ""
	for i, c := range conds {
		if i == 0 {
			where = " WHERE " + c
		} else {
			where += " AND " + c
		}
	}
	return r.query(ctx, where, args...)
}

// Get returns one author, or sql.ErrNoRows.
func (r *GitLabAutorRepo) Get(ctx context.Context, id int64) (*models.GitLabAutor, error) {
	rows, err := r.query(ctx, ` WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return &rows[0], nil
}
