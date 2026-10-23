package vulnimport

import (
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// SourceInventory is the vuln_findings.source value for CVEs matched to the
// dependency inventory.
const SourceInventory = "inventory"

// InventoryHit is one advisory matching one inventory row.
type InventoryHit struct {
	Dep  models.Dependency
	Vuln models.DependencyVuln
}

// InventoryFingerprint identifies a finding: host, container, what was
// looked up and the advisory. A version change is a new fingerprint, so the
// old finding resolves and a new one opens.
func InventoryFingerprint(d models.Dependency, vulnID string) string {
	return fingerprint(SourceInventory, fmt.Sprint(d.HostID), d.Container, d.Lookup, d.LookupName, d.LookupVersion, vulnID)
}

// FromInventory maps hits to findings linked to the host and, for a
// container, its service. The target is the host's slug.
func FromInventory(hits []InventoryHit) []models.VulnFinding {
	out := make([]models.VulnFinding, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		d, v := h.Dep, h.Vuln
		fp := InventoryFingerprint(d, v.ID)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		where := "host"
		if d.Container != "" {
			where = "container " + d.Container
			if d.Image != "" {
				where += " (" + d.Image + ")"
			}
		}
		f := models.VulnFinding{
			Source:      SourceInventory,
			Fingerprint: fp,
			Title:       truncate(fmt.Sprintf("%s · %s %s", v.ID, d.Name, d.Version)+containerSuffix(d), 300),
			Severity:    v.Severity,
			CVE:         reCVE.FindString(v.ID),
			Target:      d.HostSlug,
			Evidence:    fmt.Sprintf("%s %s on %s (version from %s; checked as %s %s %s)", d.Name, d.Version, where, d.VersionSource, strings.ToUpper(d.Lookup), firstNonEmpty(d.Ecosystem, d.LookupName), d.LookupVersion),
			Risk:        v.Summary,
			Links:       []models.VulnLink{{EntityType: "host", EntityID: d.HostID}},
		}
		if v.CVSS > 0 {
			cvss := v.CVSS
			f.CVSS = &cvss
		}
		if v.Fixed != "" {
			f.Remediation = "fixed: " + v.Fixed
		}
		if v.Source == models.DepLookupNVD {
			f.Scanner, f.Reference = "NVD", "https://nvd.nist.gov/vuln/detail/"+v.ID
		} else {
			f.Scanner, f.Reference = "OSV", "https://osv.dev/vulnerability/"+v.OSVID
		}
		if d.ServiceID != nil {
			f.Links = append(f.Links, models.VulnLink{EntityType: "service", EntityID: *d.ServiceID})
		}
		out = append(out, f)
	}
	return out
}

func containerSuffix(d models.Dependency) string {
	if d.Container == "" {
		return ""
	}
	return " (" + d.Container + ")"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
