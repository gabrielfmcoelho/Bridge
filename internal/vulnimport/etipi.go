// Package vulnimport turns vulnerability reports from outside sources into
// models.VulnFinding values. It never touches the database: matching and
// persistence live in internal/store.VulnRepo.
package vulnimport

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// SourceETIPI is the vuln_findings.source value for ETIPI report files.
const SourceETIPI = "etipi"

// ETIPIReport is a parsed ETIPI file. Targets and Severities are what the
// file covers: an absent finding is only auto-resolved when its target's
// host and its severity both appear here, because a report may be a subset
// of everything that is open.
type ETIPIReport struct {
	Findings   []models.VulnFinding
	Targets    []string
	Severities []string
}

var (
	reTotal   = regexp.MustCompile(`^Total de achados em aberto:\s*(\d+)\s*$`)
	reSection = regexp.MustCompile(`^---\s*(.+?)\s*\((\d+)\)\s*---$`)
	reCVE     = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)
	reCVSS    = regexp.MustCompile(`\(CVSS\s+([0-9]+(?:\.[0-9]+)?)\)`)
	reURL     = regexp.MustCompile(`https?://\S+`)
	reMarker  = regexp.MustCompile(`\s*\[EXPLORAÇÃO CONHECIDA[^\]]*\]`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

var severities = map[string]string{
	"CRITICAL": "critical", "HIGH": "high", "MEDIUM": "medium", "LOW": "low",
	"INFO": "info", "INFORMATIONAL": "info",
}

// fields maps an ETIPI "Key:" prefix to the finding field it fills.
var fields = []struct {
	key   string
	field func(f *models.VulnFinding) *string
}{
	{"Alvo:", func(f *models.VulnFinding) *string { return &f.Target }},
	{"VM:", func(f *models.VulnFinding) *string { return &f.VMHint }},
	{"CVE:", func(f *models.VulnFinding) *string { return &f.CVE }},
	{"Categoria (CWE):", func(f *models.VulnFinding) *string { return &f.CWE }},
	{"Detectado por:", func(f *models.VulnFinding) *string { return &f.Scanner }},
	{"Evidência:", func(f *models.VulnFinding) *string { return &f.Evidence }},
	{"Risco:", func(f *models.VulnFinding) *string { return &f.Risk }},
	{"COMO CORRIGIR:", func(f *models.VulnFinding) *string { return &f.Remediation }},
	{"Referência:", func(f *models.VulnFinding) *string { return &f.Reference }},
}

// ParseETIPI parses the ETIPI text report ("Total de achados em aberto: N",
// "--- HIGH (n) ---" sections, one "•" block per finding). It rejects a file
// whose counts don't add up: auto-resolve trusts the parse to be complete.
func ParseETIPI(data []byte) (*ETIPIReport, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("file is not valid UTF-8")
	}
	// U+2011 (non-breaking hyphen) shows up inside CVE ids ("CVE‑2003‑1540").
	data = bytes.ReplaceAll(data, []byte("\u2011"), []byte("-"))
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	total := -1
	var (
		findings  []models.VulnFinding
		cur       *models.VulnFinding
		last      *string // field the previous line filled
		severity  string
		sectionN  int
		sectionAt int // len(findings) when the section started
		sections  []string
	)
	closeSection := func() error {
		if severity == "" {
			return nil
		}
		if got := len(findings) - sectionAt; got != sectionN {
			return fmt.Errorf("section %s announces %d findings, found %d", strings.ToUpper(severity), sectionN, got)
		}
		return nil
	}
	flush := func() {
		if cur != nil {
			findings = append(findings, *cur)
			cur, last = nil, nil
		}
	}

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if m := reTotal.FindStringSubmatch(trimmed); m != nil {
			total, _ = strconv.Atoi(m[1])
			continue
		}
		if m := reSection.FindStringSubmatch(trimmed); m != nil {
			flush()
			if err := closeSection(); err != nil {
				return nil, err
			}
			sev, ok := severities[strings.ToUpper(m[1])]
			if !ok {
				return nil, fmt.Errorf("line %d: unknown severity section %q", lineNo, m[1])
			}
			severity, sectionAt = sev, len(findings)
			sectionN, _ = strconv.Atoi(m[2])
			sections = append(sections, sev)
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "•"); ok {
			if severity == "" {
				return nil, fmt.Errorf("line %d: finding before any severity section", lineNo)
			}
			flush()
			cur = &models.VulnFinding{Source: SourceETIPI, Severity: severity, Title: strings.TrimSpace(rest)}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("line %d: unexpected text outside a finding: %q", lineNo, truncate(trimmed, 60))
		}
		matched := false
		for _, fd := range fields {
			if v, ok := strings.CutPrefix(trimmed, fd.key); ok {
				last = fd.field(cur)
				*last, matched = strings.TrimSpace(v), true
				break
			}
		}
		if !matched {
			// ponytail: a line with no known key continues the previous field
			// (no wrapped values in the sample); unknown keys land there too.
			if last == nil {
				return nil, fmt.Errorf("line %d: unknown field %q", lineNo, truncate(trimmed, 60))
			}
			*last += " " + trimmed
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	if err := closeSection(); err != nil {
		return nil, err
	}
	if total < 0 {
		return nil, errors.New(`missing "Total de achados em aberto" line`)
	}
	if total != len(findings) {
		return nil, fmt.Errorf("report announces %d findings, parsed %d", total, len(findings))
	}

	rep := &ETIPIReport{Severities: sections}
	byFP := map[string]int{}
	seenTarget := map[string]bool{}
	for _, f := range findings {
		normalize(&f)
		if t := strings.ToLower(f.Target); t != "" && !seenTarget[t] {
			seenTarget[t] = true
			rep.Targets = append(rep.Targets, t)
		}
		if i, dup := byFP[f.Fingerprint]; dup {
			// Same check on the same target twice (Nikto hits several paths):
			// one finding, every evidence kept.
			if f.Evidence != "" && !strings.Contains(rep.Findings[i].Evidence, f.Evidence) {
				rep.Findings[i].Evidence += " | " + f.Evidence
			}
			continue
		}
		byFP[f.Fingerprint] = len(rep.Findings)
		rep.Findings = append(rep.Findings, f)
	}
	return rep, nil
}

// normalize derives the computed fields of one parsed finding.
func normalize(f *models.VulnFinding) {
	if reMarker.MatchString(f.Title) {
		f.KnownExploited = true
		f.Title = reMarker.ReplaceAllString(f.Title, "")
	}
	f.Title = strings.TrimSpace(reSpaces.ReplaceAllString(f.Title, " "))

	cveLine := f.CVE
	f.CVE = reCVE.FindString(cveLine)
	if f.CVE == "" {
		f.CVE = reCVE.FindString(f.Title)
	}
	if m := reCVSS.FindStringSubmatch(cveLine); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			f.CVSS = &v
		}
	}
	if f.Reference == "" {
		f.Reference = reURL.FindString(f.Title)
	}
	// Identity: scanner + target + title. Never evidence: ZAP joins a URL
	// list whose order varies, Nikto probes random paths and session tokens.
	f.Fingerprint = fingerprint(f.Scanner, targetKey(f.Target), f.Title)
}

// targetKey is the target as identity: lowercased, scheme and trailing slash
// dropped, port kept ("http://x:8080" and "x" are different services).
func targetKey(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	t = strings.TrimPrefix(strings.TrimPrefix(t, "https://"), "http://")
	return strings.TrimRight(t, "/")
}

func fingerprint(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(h[:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
