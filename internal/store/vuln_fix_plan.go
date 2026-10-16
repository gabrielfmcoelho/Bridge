package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// VulnFixPlanRepo owns SQL for vuln_fix_plans: one plan per finding,
// rewritten in place. No visibility of its own — callers load the finding
// (which is scoped) first.
type VulnFixPlanRepo struct {
	db *sql.DB
}

// NewVulnFixPlanRepo constructs a VulnFixPlanRepo over the given DB handle.
func NewVulnFixPlanRepo(db *sql.DB) *VulnFixPlanRepo { return &VulnFixPlanRepo{db: db} }

// Get returns the finding's plan, or (nil, nil) when none was generated.
func (r *VulnFixPlanRepo) Get(ctx context.Context, findingID int64) (*models.VulnFixPlan, error) {
	p := &models.VulnFixPlan{}
	var steps, repos string
	err := r.db.QueryRowContext(ctx, `SELECT finding_id, content, steps, repos, model, generated_by, generated_at
		FROM vuln_fix_plans WHERE finding_id = ?`, findingID,
	).Scan(&p.FindingID, &p.Content, &steps, &repos, &p.Model, &p.GeneratedBy, &p.GeneratedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Steps = []byte(steps)
	p.Repos = []string{}
	if repos != "" {
		p.Repos = strings.Split(repos, "\n")
	}
	return p, nil
}

// Upsert writes the plan, replacing the previous one and stamping
// generated_at.
func (r *VulnFixPlanRepo) Upsert(ctx context.Context, p *models.VulnFixPlan) error {
	steps := string(p.Steps)
	if steps == "" {
		steps = "[]"
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO vuln_fix_plans (finding_id, content, steps, repos, model, generated_by, generated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (finding_id) DO UPDATE SET
			content = excluded.content, steps = excluded.steps, repos = excluded.repos,
			model = excluded.model, generated_by = excluded.generated_by, generated_at = CURRENT_TIMESTAMP`,
		p.FindingID, p.Content, steps, strings.Join(p.Repos, "\n"), p.Model, p.GeneratedBy)
	return err
}
