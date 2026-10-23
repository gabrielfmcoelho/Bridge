package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// VulnRepo owns SQL for vuln_findings and vuln_finding_links.
//
// Visibility: a finding has no grants of its own. It is visible iff the
// caller is admin or one of its stored links points at an asset the caller
// can see; an unlinked finding is admin-only. Relations (a DNS record's
// hosts, a service's hosts) only add rows to an entity tab the caller
// already sees — they never grant visibility.
type VulnRepo struct {
	db *sql.DB
}

// NewVulnRepo constructs a VulnRepo over the given DB handle.
func NewVulnRepo(db *sql.DB) *VulnRepo { return &VulnRepo{db: db} }

const vulnCols = `f.id, f.source, f.fingerprint, f.title, f.severity, f.cve, f.cvss, f.cwe, f.scanner, f.target, f.vm_hint,
	f.evidence, f.risk, f.remediation, f.reference, f.known_exploited, f.status, f.status_note, f.status_by,
	f.issue_id, f.glpi_ticket_id, f.glpi_ticket_url, f.first_seen_at, f.last_seen_at, f.resolved_at, f.created_at, f.updated_at`

func scanVuln(s interface{ Scan(...any) error }, f *models.VulnFinding, extra ...any) error {
	return s.Scan(append([]any{&f.ID, &f.Source, &f.Fingerprint, &f.Title, &f.Severity, &f.CVE, &f.CVSS, &f.CWE,
		&f.Scanner, &f.Target, &f.VMHint, &f.Evidence, &f.Risk, &f.Remediation, &f.Reference, &f.KnownExploited,
		&f.Status, &f.StatusNote, &f.StatusBy, &f.IssueID, &f.GlpiTicketID, &f.GlpiTicketURL,
		&f.FirstSeenAt, &f.LastSeenAt, &f.ResolvedAt, &f.CreatedAt, &f.UpdatedAt}, extra...)...)
}

// vulnVisible is the visibility predicate on alias f.
func vulnVisible(ctx context.Context) (string, []any) {
	if _, skip := unscoped(ctx); skip {
		return "TRUE", nil
	}
	vis, args := VisibleExprDyn(ctx, "l.entity_type", "l.entity_id")
	return `EXISTS (SELECT 1 FROM vuln_finding_links l WHERE l.finding_id = f.id AND ` + vis + `)`, args
}

// vulnEntitySet returns a (finding_id, via) row source for an entity tab:
// the entity's own links plus findings on the hosts / DNS records it is
// related to. via is "" for a direct link, else "<type>:<name>". A hop only
// goes through a related asset the caller can see, so a hidden host / DNS
// record neither adds rows nor leaks its name in via.
func vulnEntitySet(ctx context.Context, entityType string, id int64) (string, []any, error) {
	direct := `SELECT finding_id, '' AS via FROM vuln_finding_links WHERE entity_type = ? AND entity_id = ?`
	args := []any{entityType, id}
	var parts []string
	// hop adds one related-asset branch; its JOIN binds id, then the
	// related asset's visibility predicate.
	hop := func(sel string, at AssetType, idExpr string) {
		vis, vargs := VisibleExpr(ctx, at, idExpr)
		parts = append(parts, sel+` AND `+vis)
		args = append(append(args, id), vargs...)
	}
	viaDNS := func(join string) {
		hop(`SELECT l.finding_id, 'dns:' || d.domain FROM vuln_finding_links l
			JOIN dns_records d ON d.id = l.entity_id AND d.deleted_at IS NULL `+join+`
			WHERE l.entity_type = 'dns'`, AssetDNS, "d.id")
	}
	viaHost := func(join string) {
		hop(`SELECT l.finding_id, 'host:' || h.nickname FROM vuln_finding_links l
			JOIN hosts h ON h.id = l.entity_id AND h.deleted_at IS NULL `+join+`
			WHERE l.entity_type = 'host'`, AssetHost, "h.id")
	}
	switch entityType {
	case "host":
		viaDNS(`JOIN dns_host_links x ON x.dns_id = l.entity_id AND x.host_id = ?`)
	case "dns":
	case "service":
		viaHost(`JOIN service_host_links x ON x.host_id = l.entity_id AND x.service_id = ?`)
		viaDNS(`JOIN service_dns_links x ON x.dns_id = l.entity_id AND x.service_id = ?`)
	case "project":
		viaHost(`JOIN project_host_links x ON x.host_id = l.entity_id AND x.project_id = ?`)
		viaDNS(`JOIN project_dns_links x ON x.dns_id = l.entity_id AND x.project_id = ?`)
		hop(`SELECT l.finding_id, 'service:' || s.nickname FROM vuln_finding_links l
				JOIN services s ON s.id = l.entity_id AND s.deleted_at IS NULL AND s.project_id = ?
				WHERE l.entity_type = 'service'`, AssetService, "s.id")
	default:
		return "", nil, fmt.Errorf("unknown entity type %q", entityType)
	}
	// A finding reached several ways shows its best path: direct ('') first.
	return `SELECT finding_id, MIN(via) AS via FROM (` + strings.Join(append([]string{direct}, parts...), " UNION ALL ") + `) u GROUP BY finding_id`, args, nil
}

// List returns one page of visible findings and the total match count,
// most severe and most recently seen first.
func (r *VulnRepo) List(ctx context.Context, flt models.VulnFilter) ([]models.VulnFinding, int, error) {
	from := `vuln_findings f`
	viaCol := `''`
	var args []any
	if flt.EntityType != "" {
		set, sargs, err := vulnEntitySet(ctx, flt.EntityType, flt.EntityID)
		if err != nil {
			return nil, 0, err
		}
		from = `vuln_findings f JOIN (` + set + `) es ON es.finding_id = f.id`
		viaCol = `es.via`
		args = append(args, sargs...)
	}

	vis, vargs := vulnVisible(ctx)
	where := []string{vis}
	args = append(args, vargs...)
	if flt.Status != "" {
		where = append(where, `f.status = ANY(?)`)
		args = append(args, strings.Split(flt.Status, ","))
	}
	if flt.Severity != "" {
		where = append(where, `f.severity = ANY(?)`)
		args = append(args, strings.Split(flt.Severity, ","))
	}
	if flt.Source != "" {
		where = append(where, `f.source = ?`)
		args = append(args, flt.Source)
	}
	if q := strings.TrimSpace(flt.Search); q != "" {
		where = append(where, `(f.title ILIKE ? OR f.cve ILIKE ? OR f.target ILIKE ? OR f.vm_hint ILIKE ?)`)
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	if flt.Unmatched {
		where = append(where, `NOT EXISTS (SELECT 1 FROM vuln_finding_links ul WHERE ul.finding_id = f.id)`)
	}

	perPage := flt.PerPage
	if perPage <= 0 || perPage > 500 {
		perPage = 50
	}
	page := flt.Page
	if page < 1 {
		page = 1
	}
	args = append(args, perPage, (page-1)*perPage)

	rows, err := r.db.QueryContext(ctx, `SELECT `+vulnCols+`, `+viaCol+`, COUNT(*) OVER () FROM `+from+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY CASE f.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,
			f.known_exploited DESC, f.last_seen_at DESC, f.id
		LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.VulnFinding{}
	total := 0
	for rows.Next() {
		var f models.VulnFinding
		if err := scanVuln(rows, &f, &f.Via, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}

// Get returns a visible finding with its links, or (nil, nil) when absent
// or invisible.
func (r *VulnRepo) Get(ctx context.Context, id int64) (*models.VulnFinding, error) {
	vis, vargs := vulnVisible(ctx)
	f := &models.VulnFinding{}
	err := scanVuln(r.db.QueryRowContext(ctx, `SELECT `+vulnCols+` FROM vuln_findings f WHERE f.id = ? AND `+vis,
		append([]any{id}, vargs...)...), f)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.Links, err = r.links(ctx, id)
	return f, err
}

// ByFingerprints returns the visible findings of source with the given
// fingerprints (a dependency's findings, for its drawer).
func (r *VulnRepo) ByFingerprints(ctx context.Context, source string, fps []string) ([]models.VulnFinding, error) {
	out := []models.VulnFinding{}
	if len(fps) == 0 {
		return out, nil
	}
	vis, vargs := vulnVisible(ctx)
	rows, err := r.db.QueryContext(ctx, `SELECT `+vulnCols+` FROM vuln_findings f
		WHERE f.source = ? AND f.fingerprint = ANY(?) AND `+vis+` ORDER BY f.id`,
		append([]any{source, fps}, vargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f models.VulnFinding
		if err := scanVuln(rows, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// links lists a finding's links with display names. Links to trashed or
// purged assets are left out (entity_id has no FK).
func (r *VulnRepo) links(ctx context.Context, id int64) ([]models.VulnLink, error) {
	// A visible finding may also link to assets the caller can't see: their
	// names must not leak, so each link is filtered on its own.
	vis, vargs := VisibleExprDyn(ctx, "l.entity_type", "l.entity_id")
	rows, err := r.db.QueryContext(ctx, `SELECT l.entity_type, l.entity_id, l.origin,
			COALESCE(h.nickname, d.domain, s.nickname, p.name), COALESCE(h.oficial_slug, '')
		FROM vuln_finding_links l
		LEFT JOIN hosts h       ON l.entity_type = 'host'    AND h.id = l.entity_id AND h.deleted_at IS NULL
		LEFT JOIN dns_records d ON l.entity_type = 'dns'     AND d.id = l.entity_id AND d.deleted_at IS NULL
		LEFT JOIN services s    ON l.entity_type = 'service' AND s.id = l.entity_id AND s.deleted_at IS NULL
		LEFT JOIN projects p    ON l.entity_type = 'project' AND p.id = l.entity_id AND p.deleted_at IS NULL
		WHERE l.finding_id = ? AND COALESCE(h.id, d.id, s.id, p.id) IS NOT NULL AND `+vis+`
		ORDER BY l.entity_type, l.entity_id`, append([]any{id}, vargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.VulnLink{}
	for rows.Next() {
		var l models.VulnLink
		if err := rows.Scan(&l.EntityType, &l.EntityID, &l.Origin, &l.Name, &l.Slug); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// VulnUpdate is a partial update: nil keeps the column. A zero IssueID /
// GlpiTicketID clears it.
type VulnUpdate struct {
	Status        *string
	StatusNote    *string
	IssueID       *int64
	GlpiTicketID  *int64
	GlpiTicketURL *string
}

// Update applies u to finding id. Visibility is the caller's job (Get
// first). userID is recorded as status_by when the status changes.
// Entering 'resolved' stamps resolved_at; leaving it clears it.
func (r *VulnRepo) Update(ctx context.Context, id int64, u VulnUpdate, userID int64) error {
	var set []string
	var args []any
	if u.Status != nil {
		set = append(set, `status = ?`, `status_by = ?`,
			`resolved_at = CASE WHEN ? = 'resolved' THEN COALESCE(resolved_at, CURRENT_TIMESTAMP) ELSE NULL END`)
		args = append(args, *u.Status, nullID(userID), *u.Status)
	}
	if u.StatusNote != nil {
		set = append(set, `status_note = ?`)
		args = append(args, *u.StatusNote)
	}
	if u.IssueID != nil {
		set = append(set, `issue_id = ?`)
		args = append(args, nullID(*u.IssueID))
	}
	if u.GlpiTicketID != nil {
		set = append(set, `glpi_ticket_id = ?`)
		args = append(args, nullID(*u.GlpiTicketID))
	}
	if u.GlpiTicketURL != nil {
		set = append(set, `glpi_ticket_url = ?`)
		args = append(args, *u.GlpiTicketURL)
	}
	if len(set) == 0 {
		return nil
	}
	set = append(set, `updated_at = CURRENT_TIMESTAMP`)
	_, err := r.db.ExecContext(ctx, `UPDATE vuln_findings SET `+strings.Join(set, ", ")+` WHERE id = ?`, append(args, id)...)
	return err
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// AddLink attaches a finding to an asset by hand (kept across imports).
// An existing auto link on the same key becomes manual.
func (r *VulnRepo) AddLink(ctx context.Context, id int64, entityType string, entityID int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO vuln_finding_links (finding_id, entity_type, entity_id, origin)
		VALUES (?, ?, ?, 'manual') ON CONFLICT (finding_id, entity_type, entity_id) DO UPDATE SET origin = 'manual'`,
		id, entityType, entityID)
	return err
}

// RemoveLink detaches a finding from an asset (auto or manual). An auto
// link comes back on the next import that still matches it.
func (r *VulnRepo) RemoveLink(ctx context.Context, id int64, entityType string, entityID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM vuln_finding_links WHERE finding_id = ? AND entity_type = ? AND entity_id = ?`,
		id, entityType, entityID)
	return err
}

// VulnCoverage is what a partial report covers. An absent open finding is
// auto-resolved only when its target's host (scheme and port dropped:
// "x.gov.br" and "x.gov.br:443" are the same host) and its severity are both
// in the report. nil coverage = the import is the complete list.
type VulnCoverage struct {
	Targets    []string
	Severities []string
}

// ApplyImport upserts findings of one source by (source, fingerprint),
// rewrites their auto links, and resolves the open findings of that source
// the import no longer lists. A resolved finding that reappears reopens;
// accepted / false_positive keep their status. One transaction; dryRun
// rolls it back after counting. Runs unscoped (system work).
//
// Writes are batched (multi-row INSERT / UPDATE … FROM VALUES, chunks of
// 400): the dependency inventory sends thousands of findings per scan.
// Findings may carry their own Links (origin auto) on top of the matched
// host / DNS ones.
func (r *VulnRepo) ApplyImport(ctx context.Context, source string, findings []models.VulnFinding, cover *VulnCoverage, dryRun bool) (*models.VulnImportResult, error) {
	res := &models.VulnImportResult{Parsed: len(findings), DryRun: dryRun}
	m, err := r.newMatcher(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// A fingerprint listed twice is one finding: the last copy wins.
	byFP := make(map[string]int, len(findings))
	uniq := make([]models.VulnFinding, 0, len(findings))
	for _, f := range findings {
		if i, ok := byFP[f.Fingerprint]; ok {
			uniq[i] = f
			continue
		}
		byFP[f.Fingerprint] = len(uniq)
		uniq = append(uniq, f)
	}
	seen := make([]string, len(uniq))
	for i, f := range uniq {
		seen[i] = f.Fingerprint
	}

	// Existing rows of this source, in one query.
	type prior struct {
		id     int64
		status string
	}
	existing := map[string]prior{}
	rows, err := tx.QueryContext(ctx, `SELECT fingerprint, id, status FROM vuln_findings WHERE source = ? AND fingerprint = ANY(?)`, source, seen)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var fp string
		var p prior
		if err := rows.Scan(&fp, &p.id, &p.status); err != nil {
			rows.Close()
			return nil, err
		}
		existing[fp] = p
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batched writes: findings arrive by the thousand from the dependency
	// inventory, one statement per finding would take minutes.
	const chunk = 400
	ids := make([]int64, len(uniq))
	var fresh, known []int
	for i, f := range uniq {
		if p, ok := existing[f.Fingerprint]; ok {
			ids[i] = p.id
			known = append(known, i)
			if p.status == "resolved" {
				res.Reopened++
			} else {
				res.Updated++
			}
		} else {
			fresh = append(fresh, i)
		}
	}
	res.New = len(fresh)
	for start := 0; start < len(fresh); start += chunk {
		part := fresh[start:min(start+chunk, len(fresh))]
		var b strings.Builder
		b.WriteString(`INSERT INTO vuln_findings (source, fingerprint, title, severity, cve, cvss, cwe, scanner,
			target, vm_hint, evidence, risk, remediation, reference, known_exploited) VALUES `)
		args := make([]any, 0, len(part)*15)
		for k, i := range part {
			if k > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
			f := uniq[i]
			args = append(args, source, f.Fingerprint, f.Title, f.Severity, f.CVE, f.CVSS, f.CWE, f.Scanner,
				f.Target, f.VMHint, f.Evidence, f.Risk, f.Remediation, f.Reference, f.KnownExploited)
		}
		b.WriteString(" RETURNING id, fingerprint")
		rows, err := tx.QueryContext(ctx, b.String(), args...)
		if err != nil {
			return nil, fmt.Errorf("insert findings: %w", err)
		}
		for rows.Next() {
			var id int64
			var fp string
			if err := rows.Scan(&id, &fp); err != nil {
				rows.Close()
				return nil, err
			}
			ids[byFP[fp]] = id
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for start := 0; start < len(known); start += chunk {
		part := known[start:min(start+chunk, len(known))]
		var b strings.Builder
		b.WriteString(`UPDATE vuln_findings v SET title = u.title, severity = u.severity, cve = u.cve, cvss = u.cvss, cwe = u.cwe,
			scanner = u.scanner, target = u.target, vm_hint = u.vm_hint, evidence = u.evidence, risk = u.risk,
			remediation = u.remediation, reference = u.reference, known_exploited = u.known_exploited,
			status = CASE WHEN v.status = 'resolved' THEN 'open' ELSE v.status END,
			resolved_at = CASE WHEN v.status = 'resolved' THEN NULL ELSE v.resolved_at END,
			last_seen_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			FROM (VALUES `)
		args := make([]any, 0, len(part)*14)
		for k, i := range part {
			if k > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?::bigint,?,?,?,?::double precision,?,?,?,?,?,?,?,?,?::boolean)")
			f := uniq[i]
			args = append(args, ids[i], f.Title, f.Severity, f.CVE, f.CVSS, f.CWE, f.Scanner, f.Target, f.VMHint,
				f.Evidence, f.Risk, f.Remediation, f.Reference, f.KnownExploited)
		}
		b.WriteString(`) AS u(id, title, severity, cve, cvss, cwe, scanner, target, vm_hint, evidence, risk, remediation, reference, known_exploited)
			WHERE v.id = u.id`)
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return nil, fmt.Errorf("update findings: %w", err)
		}
	}

	// Auto links: matched host / DNS plus the links the source names itself
	// (the inventory knows the container's service).
	if _, err := tx.ExecContext(ctx, `DELETE FROM vuln_finding_links WHERE finding_id = ANY(?) AND origin = 'auto'`, ids); err != nil {
		return nil, err
	}
	type link struct {
		id  int64
		typ string
		eid int64
	}
	var links []link
	for i, f := range uniq {
		for _, l := range append(m.match(f), f.Links...) {
			links = append(links, link{ids[i], l.EntityType, l.EntityID})
		}
	}
	for start := 0; start < len(links); start += chunk {
		part := links[start:min(start+chunk, len(links))]
		var b strings.Builder
		b.WriteString(`INSERT INTO vuln_finding_links (finding_id, entity_type, entity_id, origin) VALUES `)
		args := make([]any, 0, len(part)*3)
		for k, l := range part {
			if k > 0 {
				b.WriteString(",")
			}
			b.WriteString("(?,?,?,'auto')")
			args = append(args, l.id, l.typ, l.eid)
		}
		b.WriteString(" ON CONFLICT DO NOTHING")
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return nil, err
		}
	}
	linked := map[int64]bool{}
	lrows, err := tx.QueryContext(ctx, `SELECT DISTINCT finding_id FROM vuln_finding_links WHERE finding_id = ANY(?)`, ids)
	if err != nil {
		return nil, err
	}
	for lrows.Next() {
		var id int64
		if err := lrows.Scan(&id); err != nil {
			lrows.Close()
			return nil, err
		}
		linked[id] = true
	}
	lrows.Close()
	if err := lrows.Err(); err != nil {
		return nil, err
	}
	unmatched := map[string]bool{}
	for i, f := range uniq {
		if !linked[ids[i]] {
			res.Unmatched++
			unmatched[f.Target] = true
		}
	}

	gone, err := absentFindings(ctx, tx, source, seen, cover)
	if err != nil {
		return nil, err
	}
	if len(gone) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE vuln_findings SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE id = ANY(?)`, gone); err != nil {
			return nil, err
		}
	}
	res.Resolved = len(gone)
	res.UnmatchedTargets = make([]string, 0, len(unmatched))
	for t := range unmatched {
		res.UnmatchedTargets = append(res.UnmatchedTargets, t)
	}
	sort.Strings(res.UnmatchedTargets)

	if dryRun {
		return res, nil // deferred Rollback discards everything
	}
	return res, tx.Commit()
}

// absentFindings returns the open findings of source the import no longer
// lists and, for a partial report, whose host and severity it covers.
func absentFindings(ctx context.Context, tx *sql.Tx, source string, seen []string, cover *VulnCoverage) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, target, severity FROM vuln_findings
		WHERE source = ? AND status = 'open' AND NOT (fingerprint = ANY(?))`, source, seen)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hosts, sevs := map[string]bool{}, map[string]bool{}
	if cover != nil {
		for _, t := range cover.Targets {
			hosts[targetHost(t)] = true
		}
		for _, s := range cover.Severities {
			sevs[s] = true
		}
	}
	var ids []int64
	for rows.Next() {
		var id int64
		var target, sev string
		if err := rows.Scan(&id, &target, &sev); err != nil {
			return nil, err
		}
		if cover == nil || (hosts[targetHost(target)] && sevs[sev]) {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// vulnMatcher resolves a finding's target / VM hint to live hosts and DNS
// records, loaded once per import.
type vulnMatcher struct {
	dns       map[string]int64 // lowercased domain
	hostExact map[string]int64 // lowercased nickname / oficial_slug / hostname
	hostLabel map[string]int64 // first label of a DNS hostname; 0 = ambiguous
}

func (r *VulnRepo) newMatcher(ctx context.Context) (*vulnMatcher, error) {
	m := &vulnMatcher{dns: map[string]int64{}, hostExact: map[string]int64{}, hostLabel: map[string]int64{}}
	rows, err := r.db.QueryContext(ctx, `SELECT id, lower(domain) FROM dns_records WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var d string
		if err := rows.Scan(&id, &d); err != nil {
			rows.Close()
			return nil, err
		}
		if _, ok := m.dns[d]; !ok {
			m.dns[d] = id
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.QueryContext(ctx, `SELECT id, lower(COALESCE(nickname, '')), lower(COALESCE(oficial_slug, '')), lower(COALESCE(hostname, '')) FROM hosts WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var nick, slug, hostname string
		if err := rows.Scan(&id, &nick, &slug, &hostname); err != nil {
			return nil, err
		}
		for _, k := range []string{nick, slug, hostname} {
			if k = strings.TrimSpace(k); k != "" {
				if _, ok := m.hostExact[k]; !ok {
					m.hostExact[k] = id
				}
			}
		}
		if _, err := netip.ParseAddr(hostname); err == nil {
			continue
		}
		if label, _, ok := strings.Cut(hostname, "."); ok && label != "" {
			if _, dup := m.hostLabel[label]; dup {
				m.hostLabel[label] = 0
			} else {
				m.hostLabel[label] = id
			}
		}
	}
	return m, rows.Err()
}

// match returns the auto links for a finding: the DNS record named by the
// target's host part, and the host named by the VM hint (ETIPI) or by the
// target itself (a Wazuh agent name, or a bare hostname / IP).
func (m *vulnMatcher) match(f models.VulnFinding) []models.VulnLink {
	var out []models.VulnLink
	fqdn := targetHost(f.Target)
	if id, ok := m.dns[fqdn]; ok {
		out = append(out, models.VulnLink{EntityType: "dns", EntityID: id})
	}
	name := strings.ToLower(strings.TrimSpace(f.VMHint))
	if name == "" {
		name = fqdn
	}
	if id := m.host(name); id != 0 {
		out = append(out, models.VulnLink{EntityType: "host", EntityID: id})
	}
	return out
}

func (m *vulnMatcher) host(name string) int64 {
	if name == "" {
		return 0
	}
	if id, ok := m.hostExact[name]; ok {
		return id
	}
	if strings.Contains(name, ".") {
		return 0 // a full DNS name only matches exactly
	}
	return m.hostLabel[name]
}

// targetHost reduces "https://x.gov.br:8080/path" / "x.gov.br:443" to the
// lowercased host part.
func targetHost(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if i := strings.Index(t, "://"); i >= 0 {
		t = t[i+3:]
	}
	if i := strings.IndexAny(t, "/?#"); i >= 0 {
		t = t[:i]
	}
	if h, _, ok := strings.Cut(t, ":"); ok {
		t = h
	}
	return strings.TrimSuffix(t, ".")
}

// EntityExists reports whether a live host / dns / service / project row
// exists (vuln_finding_links.entity_id has no FK), unscoped.
func (r *VulnRepo) EntityExists(ctx context.Context, entityType string, id int64) (bool, error) {
	spec, ok := AssetOf(AssetType(entityType))
	if !ok || (entityType != "host" && entityType != "dns" && entityType != "service" && entityType != "project") {
		return false, nil
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM `+spec.Table+` WHERE id = ? AND deleted_at IS NULL)`, id).Scan(&exists)
	return exists, err
}
