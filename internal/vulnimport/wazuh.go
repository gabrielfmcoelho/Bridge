package vulnimport

import (
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/wazuh"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// SourceWazuh is the vuln_findings.source value for the Wazuh sync.
const SourceWazuh = "wazuh"

// FromWazuh maps indexer vulnerability states to findings: one per
// (agent, CVE, package, version). A package upgrade that still carries the
// CVE is a new finding and the old one resolves. The target is the agent
// name, which the import matches against host names.
func FromWazuh(hits []wazuh.Hit) []models.VulnFinding {
	out := make([]models.VulnFinding, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		v, p := h.Vulnerability, h.Package
		fp := fingerprint(h.Agent.ID, v.ID, p.Name, p.Version)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		title := strings.TrimSpace(v.ID + " · " + strings.TrimSpace(p.Name+" "+p.Version))
		f := models.VulnFinding{
			Source:      SourceWazuh,
			Fingerprint: fp,
			Title:       title,
			Severity:    wazuhSeverity(v.Severity),
			CVE:         reCVE.FindString(v.ID),
			CVSS:        v.Score.Base,
			Scanner:     "Wazuh",
			Target:      h.Agent.Name,
			Evidence:    strings.TrimSpace(p.Name + " " + p.Version),
			Risk:        v.Description,
			Reference:   v.Reference,
		}
		out = append(out, f)
	}
	return out
}

// wazuhSeverity maps Wazuh's "Critical" / "High" / ... to our levels.
// ponytail: "Untriaged" / "-" (no CVSS yet) land on info; revisit if
// those turn out to hide serious ones.
func wazuhSeverity(s string) string {
	if v, ok := severities[strings.ToUpper(strings.TrimSpace(s))]; ok {
		return v
	}
	return "info"
}
