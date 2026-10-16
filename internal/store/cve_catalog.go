package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// CVECatalogRepo owns SQL for cve_catalog: one row per CVE id, rewritten in
// place on re-analysis.
//
// Visibility: a CVE text is public, but which CVEs hit our assets is not. A
// scoped caller sees an entry only while a finding they can see carries it.
type CVECatalogRepo struct {
	db *sql.DB
}

// NewCVECatalogRepo constructs a CVECatalogRepo over the given DB handle.
func NewCVECatalogRepo(db *sql.DB) *CVECatalogRepo { return &CVECatalogRepo{db: db} }

// cveVisible is the visibility predicate on alias c.
func cveVisible(ctx context.Context) (string, []any) {
	if _, skip := unscoped(ctx); skip {
		return "TRUE", nil
	}
	vis, args := vulnVisible(ctx)
	return `EXISTS (SELECT 1 FROM vuln_findings f WHERE f.cve = c.cve_id AND ` + vis + `)`, args
}

// cveOpenCount counts the caller's visible open findings with the CVE.
func cveOpenCount(ctx context.Context) (string, []any) {
	vis, args := vulnVisible(ctx)
	return `(SELECT COUNT(*) FROM vuln_findings f WHERE f.cve = c.cve_id AND f.status = 'open' AND ` + vis + `)`, args
}

func scanCVE(s interface{ Scan(...any) error }, e *models.CVECatalogEntry, extra ...any) error {
	var refs string
	if err := s.Scan(append([]any{&e.CVEID, &e.Description, &e.CVSS, &e.CVSSVector, &e.CWE, &e.PublishedAt, &refs,
		&e.AnalysisPT, &e.AnalysisEN, &e.Model, &e.AnalyzedBy, &e.AnalyzedAt, &e.OpenFindings}, extra...)...); err != nil {
		return err
	}
	e.References = []string{}
	if refs != "" {
		e.References = strings.Split(refs, "\n")
	}
	return nil
}

// Get returns a visible entry, or (nil, nil) when absent or invisible.
func (r *CVECatalogRepo) Get(ctx context.Context, cveID string) (*models.CVECatalogEntry, error) {
	count, cargs := cveOpenCount(ctx)
	vis, vargs := cveVisible(ctx)
	args := append(append(cargs, cveID), vargs...)
	e := &models.CVECatalogEntry{}
	err := scanCVE(r.db.QueryRowContext(ctx, `SELECT c.cve_id, c.description, c.cvss, c.cvss_vector, c.cwe, c.published_at,
		c.refs, c.analysis_pt, c.analysis_en, c.model, c.analyzed_by, c.analyzed_at, `+count+`
		FROM cve_catalog c WHERE c.cve_id = ? AND `+vis, args...), e)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// List returns one page of visible entries (analysis text left empty) and
// the total, most recently analysed first. search matches id or description.
func (r *CVECatalogRepo) List(ctx context.Context, search string, page, perPage int) ([]models.CVECatalogEntry, int, error) {
	count, args := cveOpenCount(ctx)
	vis, vargs := cveVisible(ctx)
	where := []string{vis}
	args = append(args, vargs...)
	if q := strings.TrimSpace(search); q != "" {
		where = append(where, `(c.cve_id ILIKE ? OR c.description ILIKE ?)`)
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	if perPage <= 0 || perPage > 500 {
		perPage = 50
	}
	if page < 1 {
		page = 1
	}
	args = append(args, perPage, (page-1)*perPage)

	rows, err := r.db.QueryContext(ctx, `SELECT c.cve_id, c.description, c.cvss, c.cvss_vector, c.cwe, c.published_at,
		c.refs, '', '', c.model, c.analyzed_by, c.analyzed_at, `+count+`, COUNT(*) OVER ()
		FROM cve_catalog c WHERE `+strings.Join(where, " AND ")+`
		ORDER BY c.analyzed_at DESC, c.cve_id
		LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.CVECatalogEntry{}
	total := 0
	for rows.Next() {
		var e models.CVECatalogEntry
		if err := scanCVE(rows, &e, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// Upsert writes the entry, replacing a previous analysis of the same CVE
// and stamping analyzed_at.
func (r *CVECatalogRepo) Upsert(ctx context.Context, e *models.CVECatalogEntry) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO cve_catalog (cve_id, description, cvss, cvss_vector, cwe, published_at,
			refs, analysis_pt, analysis_en, model, analyzed_by, analyzed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (cve_id) DO UPDATE SET
			description = excluded.description, cvss = excluded.cvss, cvss_vector = excluded.cvss_vector,
			cwe = excluded.cwe, published_at = excluded.published_at, refs = excluded.refs,
			analysis_pt = excluded.analysis_pt, analysis_en = excluded.analysis_en, model = excluded.model,
			analyzed_by = excluded.analyzed_by, analyzed_at = CURRENT_TIMESTAMP`,
		e.CVEID, e.Description, e.CVSS, e.CVSSVector, e.CWE, e.PublishedAt, strings.Join(e.References, "\n"),
		e.AnalysisPT, e.AnalysisEN, e.Model, e.AnalyzedBy)
	return err
}
