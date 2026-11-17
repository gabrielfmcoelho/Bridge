package service

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// ReleaseReport is what an update run did to a host beyond its packages
// (os_update_results.report). It holds:
// - the release warnings of the scan just before the run;
// - why dpkg failed, when it did;
// - what the scans before and after differ by (CompareScans);
// - the host's sites before and after (CompareSites);
// - a repair's unit restarts and config files to review.
// Every kind but check and backup gets one; a release most needs it.
type ReleaseReport struct {
	WarningsBefore     []sshtest.RiskWarning `json:"warnings_before,omitempty"`
	DpkgErrors         []string              `json:"dpkg_errors,omitempty"` // "pkg: message"
	DpkgBroken         []string              `json:"dpkg_broken,omitempty"`
	NewFailedUnits     []string              `json:"new_failed_units,omitempty"`
	StoppedServices    []string              `json:"stopped_services,omitempty"` // running before, not after
	NewDistFiles       []string              `json:"new_dist_files,omitempty"`
	LostPHPHandlers    []string              `json:"lost_php_handlers,omitempty"` // "target → binary"
	NewForeignPackages []string              `json:"new_foreign_packages,omitempty"`
	Sites              []SiteCheck           `json:"sites,omitempty"`
	// A repair's: the failed units it restarted, the config files to review.
	UnitRestarts []sshtest.UnitRestart `json:"unit_restarts,omitempty"`
	DistFiles    []sshtest.DistFile    `json:"dist_files,omitempty"`
}

// CompareScans is what the scan after a run shows that the one before
// didn't. The release-risk parts need both scans to have run that probe
// (an older scan without it would make everything look new).
func CompareScans(before, after *sshtest.VMInfo) ReleaseReport {
	var r ReleaseReport
	if before == nil || after == nil {
		return r
	}
	running := map[string]bool{}
	for _, s := range after.SystemdServices {
		running[s.Unit] = true
	}
	for _, s := range before.SystemdServices {
		if !running[s.Unit] {
			r.StoppedServices = append(r.StoppedServices, s.Unit)
		}
	}
	b, a := before.ReleaseRisks, after.ReleaseRisks
	if b == nil || a == nil {
		return r
	}
	r.NewFailedUnits = added(b.FailedUnits, a.FailedUnits)
	r.NewDistFiles = added(b.PendingDistFiles, a.PendingDistFiles)
	r.NewForeignPackages = added(names(b.ForeignPackages), names(a.ForeignPackages))
	ok := map[string]bool{}
	for _, h := range b.PHPHandlers {
		if h.State == "ok" {
			ok[h.Target] = true
		}
	}
	for _, h := range a.PHPHandlers {
		if h.State == "missing" && ok[h.Target] {
			lost := h.Target
			if h.Runs != "" {
				lost += " → " + h.Runs
			}
			if !slices.Contains(r.LostPHPHandlers, lost) {
				r.LostPHPHandlers = append(r.LostPHPHandlers, lost)
			}
		}
	}
	return r
}

// added is what after has that before hadn't, in after's order.
func added(before, after []string) []string {
	var out []string
	for _, x := range after {
		if !slices.Contains(before, x) {
			out = append(out, x)
		}
	}
	return out
}

// names keeps the package names of "name version" items (an upgrade within
// the PPA isn't a new foreign package).
func names(items []string) []string {
	out := make([]string, len(items))
	for i, x := range items {
		out[i], _, _ = strings.Cut(x, " ")
	}
	return out
}

// SiteState is one name's "/" as Bridge sees it: the status (https, else
// http; 0 when neither answered, no redirect followed) and the PHP error
// text its body shows.
type SiteState struct {
	Status int
	Error  string
}

// SiteCheck is one name before and after a run. Broken: it answered
// (2xx/3xx) before and doesn't after, or its body now shows a PHP error.
type SiteCheck struct {
	Name   string `json:"name"`
	Before int    `json:"before"`
	After  int    `json:"after"`
	Error  string `json:"error,omitempty"`
	Broken bool   `json:"broken,omitempty"`
}

const (
	maxSiteProbes  = 30
	siteProbeLimit = 64 << 10
	siteProbeConc  = 8
)

// A PHP error printed into the page: CodeIgniter's own, and PHP's (plain or
// with html_errors, "<b>Deprecated</b>:"). A 200 full of them is still broken.
var phpErrorText = regexp.MustCompile(`A PHP Error was encountered|(Fatal error|Parse error|Deprecated)(</b>)?:`)

// ProbeSites reads "/" of each name, a few at a time. Only the status and
// which PHP error the body shows are kept, never the body.
// shortcut: only "/" of each name; check per-path (from the access logs)
// when a site hides its app under a subpath.
func ProbeSites(ctx context.Context, names []string) map[string]SiteState {
	if len(names) > maxSiteProbes {
		names = names[:maxSiteProbes]
	}
	out := make(map[string]SiteState, len(names))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, siteProbeConc)
	for _, n := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			st := probeSite(ctx, n)
			mu.Lock()
			out[n] = st
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func probeSite(ctx context.Context, name string) SiteState {
	code, body := httpProbe(ctx, "https://"+name+"/", siteProbeLimit)
	if code == 0 {
		code, body = httpProbe(ctx, "http://"+name+"/", siteProbeLimit)
	}
	st := SiteState{Status: code}
	if m := phpErrorText.FindStringSubmatch(body); m != nil {
		st.Error = m[0]
		if m[1] != "" {
			st.Error = m[1]
		}
	}
	return st
}

// CompareSites is one row per name probed after the run, by name.
func CompareSites(before, after map[string]SiteState) []SiteCheck {
	up := func(s int) bool { return s >= 200 && s < 400 }
	var out []SiteCheck
	for name, a := range after {
		b := before[name]
		out = append(out, SiteCheck{Name: name, Before: b.Status, After: a.Status, Error: a.Error,
			Broken: up(b.Status) && !up(a.Status) || a.Error != "" && b.Error == ""})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SiteNames are the names worth probing among a host's DNS records: no
// wildcard, nothing that isn't a host name.
func SiteNames(domains []string) []string {
	var out []string
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && !strings.ContainsAny(d, "*/ ") && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}
