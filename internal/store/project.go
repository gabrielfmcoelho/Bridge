package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ProjectRepo owns SQL for the projects table and the project_host_links
// junction. Used by the project service (enrichment) and inline by
// host/graph/dashboard/outline/glpi/ai/api-catalog handlers.
type ProjectRepo struct {
	db *sql.DB
}

// NewProjectRepo constructs a ProjectRepo over the given DB handle.
func NewProjectRepo(db *sql.DB) *ProjectRepo { return &ProjectRepo{db: db} }

const projectCols = `id, name, description, situacao, setor_responsavel, responsavel,
	tem_empresa_externa_responsavel, contato_empresa_responsavel,
	is_directly_managed, is_responsible, gitlab_url, documentation_url, outline_collection_id,
	glpi_token_id, glpi_entity_id, glpi_category_id, created_at, updated_at`

// projectColsP is projectCols qualified with the `p.` table alias, for joins.
const projectColsP = `p.id, p.name, p.description, p.situacao, p.setor_responsavel, p.responsavel,
	p.tem_empresa_externa_responsavel, p.contato_empresa_responsavel,
	p.is_directly_managed, p.is_responsible, p.gitlab_url, p.documentation_url, p.outline_collection_id,
	p.glpi_token_id, p.glpi_entity_id, p.glpi_category_id, p.created_at, p.updated_at`

func scanProject(scanner interface{ Scan(...any) error }, p *models.Project) error {
	return scanner.Scan(&p.ID, &p.Name, &p.Description, &p.Situacao, &p.SetorResponsavel, &p.Responsavel,
		&p.TemEmpresaExternaResponsavel, &p.ContatoEmpresaResponsavel,
		&p.IsDirectlyManaged, &p.IsResponsible, &p.GitlabURL, &p.DocumentationURL, &p.OutlineCollectionID,
		&p.GlpiTokenID, &p.GlpiEntityID, &p.GlpiCategoryID, &p.CreatedAt, &p.UpdatedAt)
}

// Create inserts a project and sets p.ID.
func (r *ProjectRepo) Create(ctx context.Context, p *models.Project) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO projects (name, description, situacao, setor_responsavel, responsavel,
			tem_empresa_externa_responsavel, contato_empresa_responsavel,
			is_directly_managed, is_responsible, gitlab_url, documentation_url, outline_collection_id,
			glpi_token_id, glpi_entity_id, glpi_category_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.Situacao, p.SetorResponsavel, p.Responsavel,
		p.TemEmpresaExternaResponsavel, p.ContatoEmpresaResponsavel,
		p.IsDirectlyManaged, p.IsResponsible, p.GitlabURL, p.DocumentationURL, p.OutlineCollectionID,
		p.GlpiTokenID, p.GlpiEntityID, p.GlpiCategoryID,
	)
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

// Get returns a project by id, or (nil, nil) if absent.
func (r *ProjectRepo) Get(ctx context.Context, id int64) (*models.Project, error) {
	p := &models.Project{}
	err := scanProject(r.db.QueryRowContext(ctx, `SELECT `+projectCols+` FROM projects WHERE id = ? AND deleted_at IS NULL`, id), p)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// List returns all projects ordered by name.
func (r *ProjectRepo) List(ctx context.Context) ([]models.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+projectCols+` FROM projects WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := scanProject(rows, &p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// projectWhere builds the shared WHERE predicates (and bound args) for the
// filtered list/count queries. Mirrors HostRepo's filter assembly: dynamic
// clauses with `?` placeholders (the driver rebinds `?`→$N), ILIKE search via
// database.LikeOp(), and a tag subquery against the unified tags table.
func projectWhere(f models.ProjectFilter) ([]string, []any) {
	var args []any
	where := []string{"deleted_at IS NULL"}

	if f.Search != "" {
		op := database.LikeOp()
		where = append(where, "(name "+op+" ? OR description "+op+" ? OR setor_responsavel "+op+" ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s, s)
	}
	if f.Situacao != "" {
		where = append(where, "situacao = ?")
		args = append(args, f.Situacao)
	}
	if f.Tag != "" {
		where = append(where, "id IN (SELECT entity_id FROM tags WHERE entity_type = 'project' AND tag = ?)")
		args = append(args, f.Tag)
	}
	return where, args
}

// ListFiltered returns projects matching the filter, with sort + pagination
// applied. Mirrors HostRepo.List. When f.PerPage <= 0 the result is unbounded
// (no LIMIT) — the path the frontend's full-list query uses.
func (r *ProjectRepo) ListFiltered(ctx context.Context, f models.ProjectFilter) ([]models.Project, error) {
	query := `SELECT ` + projectCols + ` FROM projects`
	where, args := projectWhere(f)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	allowedSorts := map[string]string{
		"name":     "name",
		"situacao": "situacao",
		"setor":    "setor_responsavel",
	}
	sortCol := "name"
	if col, ok := allowedSorts[f.SortBy]; ok {
		sortCol = col
	}
	sortDir := "ASC"
	if f.SortDir == "desc" {
		sortDir = "DESC"
	}
	query += " ORDER BY " + sortCol + " " + sortDir

	if f.PerPage > 0 {
		offset := 0
		if f.Page > 1 {
			offset = (f.Page - 1) * f.PerPage
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PerPage, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := scanProject(rows, &p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// CountFiltered returns the number of projects matching the same predicates
// ListFiltered paginates over (no order/limit).
func (r *ProjectRepo) CountFiltered(ctx context.Context, f models.ProjectFilter) (int, error) {
	query := `SELECT COUNT(*) FROM projects`
	where, args := projectWhere(f)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// Update writes the mutable fields of a project by id.
func (r *ProjectRepo) Update(ctx context.Context, p *models.Project) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, situacao = ?, setor_responsavel = ?, responsavel = ?,
			tem_empresa_externa_responsavel = ?, contato_empresa_responsavel = ?,
			is_directly_managed = ?, is_responsible = ?, gitlab_url = ?, documentation_url = ?, outline_collection_id = ?,
			glpi_token_id = ?, glpi_entity_id = ?, glpi_category_id = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		p.Name, p.Description, p.Situacao, p.SetorResponsavel, p.Responsavel,
		p.TemEmpresaExternaResponsavel, p.ContatoEmpresaResponsavel,
		p.IsDirectlyManaged, p.IsResponsible, p.GitlabURL, p.DocumentationURL, p.OutlineCollectionID,
		p.GlpiTokenID, p.GlpiEntityID, p.GlpiCategoryID,
		p.ID,
	)
	return err
}

// Delete removes a project row by id. (Vault cascade is handled separately via
// the cascade registry in the delete path.)
func (r *ProjectRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	return err
}

// Count returns the total number of projects.
func (r *ProjectRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// SetProjectsForHost replaces all project links for a host (one tx).
func (r *ProjectRepo) SetProjectsForHost(ctx context.Context, hostID int64, projectIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_host_links WHERE host_id = ?`, hostID); err != nil {
		return err
	}
	for _, pid := range projectIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_host_links (project_id, host_id) VALUES (?, ?)`, pid, hostID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ProjectsByHost returns projects directly linked to a host, fully populated.
func (r *ProjectRepo) ProjectsByHost(ctx context.Context, hostID int64) ([]models.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+projectColsP+`
		 FROM projects p JOIN project_host_links l ON p.id = l.project_id
		 WHERE l.host_id = ? AND p.deleted_at IS NULL ORDER BY p.name`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := scanProject(rows, &p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// HostIDs returns host ids directly linked to a project.
func (r *ProjectRepo) HostIDs(ctx context.Context, projectID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT host_id FROM project_host_links WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	return scanInt64s(rows)
}

// ListTrash returns soft-deleted projects (deleted_at set).
func (r *ProjectRepo) ListTrash(ctx context.Context) ([]models.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+projectCols+` FROM projects WHERE deleted_at IS NOT NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := scanProject(rows, &p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
