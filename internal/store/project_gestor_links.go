package store

import (
	"context"
	"errors"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ErrGestorProjetoEmUso: the Gestor project is already linked to another Bridge project.
var ErrGestorProjetoEmUso = errors.New("gestor project already linked to another project")

// GestorLinksFor returns one project's Gestor links (never nil).
func (r *ProjectRepo) GestorLinksFor(ctx context.Context, projectID int64) ([]models.ProjectGestorLink, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT project_id, gestor_projeto_id, gestor_projeto_nome FROM project_gestor_links WHERE project_id = ? ORDER BY gestor_projeto_nome, gestor_projeto_id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProjectGestorLink{}
	for rows.Next() {
		var l models.ProjectGestorLink
		if err := rows.Scan(&l.ProjectID, &l.GestorProjetoID, &l.GestorProjetoNome); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SetGestorLinks replaces a project's Gestor links (one tx). A Gestor project
// linked elsewhere yields ErrGestorProjetoEmUso and nothing changes.
func (r *ProjectRepo) SetGestorLinks(ctx context.Context, projectID int64, links []models.ProjectGestorLink) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_gestor_links WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project_gestor_links (project_id, gestor_projeto_id, gestor_projeto_nome) VALUES (?, ?, ?) ON CONFLICT (project_id, gestor_projeto_id) DO NOTHING`,
			projectID, l.GestorProjetoID, l.GestorProjetoNome); err != nil {
			if IsUniqueViolation(err) {
				return ErrGestorProjetoEmUso
			}
			return err
		}
	}
	return tx.Commit()
}
