package vulnimport

import (
	"os"
	"strings"
	"testing"
)

func readSample(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/etipi_sample.txt")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseETIPI_Sample(t *testing.T) {
	rep, err := ParseETIPI(readSample(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 7 blocks; the two "Alerta Nikto" blocks on the same target merge.
	if len(rep.Findings) != 6 {
		t.Fatalf("findings = %d, want 6", len(rep.Findings))
	}
	if strings.Join(rep.Severities, ",") != "critical,high" {
		t.Fatalf("severities = %v", rep.Severities)
	}
	if len(rep.Targets) != 4 {
		t.Fatalf("targets = %v, want 4 distinct", rep.Targets)
	}

	meta := rep.Findings[0]
	if meta.Title != "Metabase - Unauthenticated SQL Injection" || !meta.KnownExploited {
		t.Fatalf("marker not stripped/flagged: %q exploited=%v", meta.Title, meta.KnownExploited)
	}
	if meta.CVE != "CVE-2026-0001" || meta.CVSS == nil || *meta.CVSS != 10.0 || meta.CWE != "CWE-89" {
		t.Fatalf("cve/cvss/cwe = %q %v %q", meta.CVE, meta.CVSS, meta.CWE)
	}
	if meta.Severity != "critical" || meta.Scanner != "Nuclei" || meta.Risk == "" || meta.Remediation == "" {
		t.Fatalf("fields = %+v", meta)
	}

	osvdb := rep.Findings[1]
	if osvdb.CVE != "CVE-2003-1540" || osvdb.VMHint != "VM-PROXY-01" {
		t.Fatalf("U+2011 CVE / VM hint = %q %q", osvdb.CVE, osvdb.VMHint)
	}
	if ref := rep.Findings[3].Reference; ref != "https://packetstorm.example.test/xmas.txt.html" {
		t.Fatalf("trailing title URL → reference = %q", ref)
	}
	if ev := rep.Findings[5].Evidence; !strings.Contains(ev, "/one/<script>") || !strings.Contains(ev, "/two") {
		t.Fatalf("merged evidence = %q", ev)
	}
}

// The fingerprint ignores evidence (ZAP URL order, Nikto random paths) and
// the known-exploited marker, so a re-scan keeps the finding's identity.
func TestParseETIPI_FingerprintStable(t *testing.T) {
	a, _ := ParseETIPI(readSample(t))
	changed := strings.NewReplacer(
		"http://app.example.test:8080/login/ | http://app.example.test:8080/login/?next=%2Fhome",
		"http://app.example.test:8080/login/?next=%2Fhome | http://app.example.test:8080/login/",
		"  [EXPLORAÇÃO CONHECIDA — CORRIGIR COM URGÊNCIA]", "",
	).Replace(string(readSample(t)))
	b, err := ParseETIPI([]byte(changed))
	if err != nil {
		t.Fatal(err)
	}
	for i := range a.Findings {
		if a.Findings[i].Fingerprint != b.Findings[i].Fingerprint {
			t.Fatalf("finding %d (%s) fingerprint changed", i, a.Findings[i].Title)
		}
	}
}

func TestParseETIPI_Rejects(t *testing.T) {
	s := string(readSample(t))
	cases := map[string]string{
		"total mismatch":   strings.Replace(s, "aberto: 7", "aberto: 8", 1),
		"section mismatch": strings.Replace(s, "HIGH (5)", "HIGH (4)", 1),
		"no total":         strings.Replace(s, "Total de achados em aberto: 7", "", 1),
		"unknown section":  strings.Replace(s, "HIGH (5)", "SEVERE (5)", 1),
		"not utf-8":        s + "\xff",
	}
	for name, in := range cases {
		if _, err := ParseETIPI([]byte(in)); err == nil {
			t.Errorf("%s: parse succeeded, want error", name)
		}
	}
}
