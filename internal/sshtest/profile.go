package sshtest

import (
	"fmt"
	"strings"
	"time"
)

// HostProfile classifies a host as "idle" (likely unused / a candidate for
// decommission) when neither of the two workload indicators below detect
// any activity. The heuristic is intentionally narrow: only what the
// operator actually deploys counts. Management agents (zabbix, filebeat,
// …) and SSH login activity are deliberately *not* considered — those
// reflect monitoring and access patterns, not whether the host is doing
// any real work.
//
// Rules (both must pass for idle=true):
//
//	R1 — service_inventory is empty (no nginx/postgres/redis/queue/runtime/…
//	     i.e. no application services classified by the unified taxonomy)
//	R2 — parsed_containers is empty (no Docker workloads)
//
// Reasons captures the rules that *passed* (justifying the idle verdict).
// Counterfacts captures the rules that *failed* with a brief explanation
// for each. The slices are independent so the UI can show "why idle?" or
// "why not idle?" without re-deriving the data.
type HostProfile struct {
	Idle         bool     `json:"idle"`
	Reasons      []string `json:"reasons,omitempty"`
	Counterfacts []string `json:"counterfacts,omitempty"`
}

// idleRuleCount is the number of rules in the heuristic; idle requires
// every one of them to pass. Kept as a constant so the verdict check at
// the bottom of computeHostProfile stays in lockstep with the rule list
// even if rules are added/removed later.
const idleRuleCount = 2

// computeHostProfile evaluates the idle heuristic over an already-populated
// VMInfo. The `now` parameter is unused by the current rules but kept on
// the signature so future rules can reintroduce time-based checks
// (e.g. "no human login within N days") without a downstream churn.
//
// Safe to call on partial scans: missing fields are treated as "no
// signal", which is conservative for idle classification.
func computeHostProfile(info *VMInfo, _ time.Time) *HostProfile {
	profile := &HostProfile{}
	rulePassed := func(reason string) { profile.Reasons = append(profile.Reasons, reason) }
	ruleFailed := func(counterfact string) { profile.Counterfacts = append(profile.Counterfacts, counterfact) }

	// ── R1: no application services in the inventory ──
	if len(info.ServiceInventory) == 0 {
		rulePassed("R1: no application services detected (no web/db/cache/queue/runtime workloads)")
	} else {
		ruleFailed(fmt.Sprintf("R1: %d application service(s) detected (%s)",
			len(info.ServiceInventory), summarizeServiceNames(info.ServiceInventory, 3)))
	}

	// ── R2: no Docker containers ──
	if len(info.ParsedContainers) == 0 {
		rulePassed("R2: no Docker containers running")
	} else {
		var imgs []string
		for _, c := range info.ParsedContainers {
			imgs = append(imgs, c.Image)
		}
		ruleFailed(fmt.Sprintf("R2: %d container(s) running (%s)", len(info.ParsedContainers), joinFirstN(imgs, 3)))
	}

	// Idle ⇔ every rule passed. Reasons already lists exactly those.
	profile.Idle = len(profile.Counterfacts) == 0 && len(profile.Reasons) == idleRuleCount
	return profile
}

func summarizeServiceNames(svcs []DiscoveredService, n int) string {
	names := make([]string, 0, len(svcs))
	for _, s := range svcs {
		names = append(names, s.Label)
	}
	return joinFirstN(names, n)
}

func joinFirstN(items []string, n int) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:n], ", ") + fmt.Sprintf(", +%d more", len(items)-n)
}
