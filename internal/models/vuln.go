package models

import "time"

// VulnFinding is one vulnerability finding reported by a scanner source
// (ETIPI report file, Wazuh). Most ETIPI findings carry no CVE (SQLi, SNMP,
// Nikto paths), so CVE is optional. Identity across imports is
// (Source, Fingerprint). Persistence lives in internal/store.VulnRepo.
type VulnFinding struct {
	ID             int64      `json:"id"`
	Source         string     `json:"source"`
	Fingerprint    string     `json:"-"`
	Title          string     `json:"title"`
	Severity       string     `json:"severity"`
	CVE            string     `json:"cve"`
	CVSS           *float64   `json:"cvss"`
	CWE            string     `json:"cwe"`
	Scanner        string     `json:"scanner"`
	Target         string     `json:"target"`
	VMHint         string     `json:"vm_hint"`
	Evidence       string     `json:"evidence"`
	Risk           string     `json:"risk"`
	Remediation    string     `json:"remediation"`
	Reference      string     `json:"reference"`
	KnownExploited bool       `json:"known_exploited"`
	Status         string     `json:"status"`
	StatusNote     string     `json:"status_note"`
	StatusBy       *int64     `json:"status_by"`
	IssueID        *int64     `json:"issue_id"`
	GlpiTicketID   *int64     `json:"glpi_ticket_id"`
	GlpiTicketURL  string     `json:"glpi_ticket_url"`
	FirstSeenAt    time.Time  `json:"first_seen_at"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Via is set only on entity-tab lists: "" for a direct link, otherwise
	// the related asset the finding reached this entity through
	// ("dns:portal.sead.pi.gov.br", "host:<nickname>").
	Via string `json:"via,omitempty"`
	// Links is filled on the detail read.
	Links []VulnLink `json:"links,omitempty"`
}

// VulnLink attaches a finding to a host / dns / service / project. Origin is
// "auto" (rewritten by each import) or "manual" (kept). Name is the entity's
// display label and Slug a host's URL key, filled on reads.
type VulnLink struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Origin     string `json:"origin"`
	Name       string `json:"name"`
	Slug       string `json:"slug,omitempty"`
}

// VulnFilter is the list predicate set for store.VulnRepo.List. Status may be
// a comma-separated set ("accepted,false_positive"). EntityType+EntityID
// select an entity tab (direct links plus findings reached through related
// hosts / DNS records). Unmatched selects findings with no link at all.
type VulnFilter struct {
	Status     string
	Severity   string
	Source     string
	Search     string
	Unmatched  bool
	EntityType string
	EntityID   int64
	Page       int
	PerPage    int
}

// VulnImportResult is what an import / sync did (or would do, on a dry run).
type VulnImportResult struct {
	Parsed    int  `json:"parsed"`
	New       int  `json:"new"`
	Updated   int  `json:"updated"`
	Reopened  int  `json:"reopened"`
	Resolved  int  `json:"resolved"`
	Unmatched int  `json:"unmatched"`
	DryRun    bool `json:"dry_run"`
	// UnmatchedTargets are the distinct targets (an ETIPI "Alvo", a Wazuh
	// agent name) whose findings matched no asset, sorted.
	UnmatchedTargets []string `json:"unmatched_targets"`
}

// CVECatalogEntry is one CVE's public NVD text plus a generic LLM analysis
// in pt-BR and en, shared by every finding carrying that CVE. OpenFindings
// is the caller's visible open findings with this CVE, filled on reads.
// List reads leave AnalysisPT / AnalysisEN empty.
type CVECatalogEntry struct {
	CVEID        string     `json:"cve_id"`
	Description  string     `json:"description"`
	CVSS         *float64   `json:"cvss"`
	CVSSVector   string     `json:"cvss_vector"`
	CWE          string     `json:"cwe"`
	PublishedAt  *time.Time `json:"published_at"`
	References   []string   `json:"references"`
	AnalysisPT   string     `json:"analysis_pt"`
	AnalysisEN   string     `json:"analysis_en"`
	Model        string     `json:"model"`
	AnalyzedBy   *int64     `json:"analyzed_by"`
	AnalyzedAt   time.Time  `json:"analyzed_at"`
	OpenFindings int        `json:"open_findings"`
}
