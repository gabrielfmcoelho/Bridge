package vulnimport

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/wazuh"
)

func TestFromWazuh(t *testing.T) {
	var h wazuh.Hit
	h.Agent.ID, h.Agent.Name = "001", "web-01"
	h.Package.Name, h.Package.Version = "openssl", "3.0.2"
	h.Vulnerability.ID, h.Vulnerability.Severity = "CVE-2024-0001", "High"
	untriaged := h
	untriaged.Vulnerability.ID, untriaged.Vulnerability.Severity = "CVE-2024-0002", "Untriaged"

	got := FromWazuh([]wazuh.Hit{h, h, untriaged})
	if len(got) != 2 {
		t.Fatalf("findings = %d, want 2 (duplicate hit collapsed)", len(got))
	}
	f := got[0]
	if f.Source != "wazuh" || f.Severity != "high" || f.CVE != "CVE-2024-0001" || f.Target != "web-01" ||
		f.Title != "CVE-2024-0001 · openssl 3.0.2" || f.Fingerprint == "" {
		t.Fatalf("finding = %+v", f)
	}
	if got[1].Severity != "info" {
		t.Fatalf("untriaged severity = %q, want info", got[1].Severity)
	}
	upgraded := h
	upgraded.Package.Version = "3.0.3"
	if FromWazuh([]wazuh.Hit{upgraded})[0].Fingerprint == f.Fingerprint {
		t.Fatal("a new package version must be a new finding")
	}
}
