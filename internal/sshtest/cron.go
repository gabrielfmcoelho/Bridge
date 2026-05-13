package sshtest

import (
	"fmt"
	"path"
	"strings"

	"golang.org/x/crypto/ssh"
)

// captureCronInfo collects the cron daemon state and every scheduled task
// (system crontabs, /etc/cron.d/*, /etc/cron.{hourly,daily,weekly,monthly}/,
// per-user crontabs in /var/spool/cron/, /etc/anacrontab, and systemd timers).
//
// Reading per-user crontabs and /etc/crontab usually requires root, so the
// collector tries `sudo -n` first and retries the user-spool block via
// `sudo -S` (with the saved sudo password) when the unprivileged attempt
// returns nothing. The current SSH user's `crontab -l` is always captured as
// a final fallback so we never come back completely empty.
//
// Returns nil only when the host appears to have no cron infrastructure and
// no timers — in practice, every Linux box yields at least a daemon block.
func captureCronInfo(client *ssh.Client, sudoPassword string) *CronInfo {
	const delim = "---CRON---"
	cmd := strings.Join([]string{
		// 0: daemon active state
		`(systemctl is-active cron 2>/dev/null || systemctl is-active crond 2>/dev/null) | head -n1`,
		`echo '` + delim + `'`,
		// 1: daemon enabled-at-boot state
		`(systemctl is-enabled cron 2>/dev/null || systemctl is-enabled crond 2>/dev/null) | head -n1`,
		`echo '` + delim + `'`,
		// 2: daemon name (cron or crond) so the UI can show which one
		`if systemctl cat cron --no-pager >/dev/null 2>&1; then echo cron; ` +
			`elif systemctl cat crond --no-pager >/dev/null 2>&1; then echo crond; ` +
			`else echo; fi`,
		`echo '` + delim + `'`,
		// 3: /etc/crontab (system-wide, has user field)
		`cat /etc/crontab 2>/dev/null || sudo -n cat /etc/crontab 2>/dev/null`,
		`echo '` + delim + `'`,
		// 4: /etc/cron.d/* (one block per file, prefixed with ###FILE### name)
		`for f in /etc/cron.d/*; do [ -f "$f" ] && echo "###FILE### $(basename "$f")" && ` +
			`(cat "$f" 2>/dev/null || sudo -n cat "$f" 2>/dev/null); done 2>/dev/null`,
		`echo '` + delim + `'`,
		// 5: cron.{hourly,daily,weekly,monthly} script names (one per line: "<dir>/<name>")
		`for d in /etc/cron.hourly /etc/cron.daily /etc/cron.weekly /etc/cron.monthly; do ` +
			`[ -d "$d" ] && for f in "$d"/*; do bn=$(basename "$f"); ` +
			`[ -e "$f" ] && [ "$bn" != "0anacron" ] && echo "$(basename "$d")/$bn"; done; ` +
			`done 2>/dev/null`,
		`echo '` + delim + `'`,
		// 6: per-user crontabs (sudo -n attempt; returns "###USER### <name>" + content)
		`SP=$([ -d /var/spool/cron/crontabs ] && echo /var/spool/cron/crontabs || echo /var/spool/cron); ` +
			`sudo -n sh -c 'for u in '"$SP"'/*; do [ -f "$u" ] && echo "###USER### $(basename "$u")" && cat "$u"; done' 2>/dev/null`,
		`echo '` + delim + `'`,
		// 7: current user's own crontab (always works without sudo; used as a fallback)
		`echo "###USER### $(whoami)"`,
		`crontab -l 2>/dev/null`,
		`echo '` + delim + `'`,
		// 8: anacrontab
		`cat /etc/anacrontab 2>/dev/null || sudo -n cat /etc/anacrontab 2>/dev/null`,
		`echo '` + delim + `'`,
		// 9: systemd timers as KEY=VALUE blocks. `systemctl show` accepts
		// multiple unit names and emits property blocks separated by a
		// blank line, so we get every timer in one SSH round trip
		// instead of one per timer (a stock Ubuntu cloud image has 20+
		// timers, multiplying RTT by 20+).
		`UNITS=$(systemctl list-unit-files --type=timer --no-pager --no-legend 2>/dev/null | awk '{print $1}' | grep -v '^$' | tr '\n' ' '); ` +
			`[ -n "$UNITS" ] && systemctl show $UNITS -p Id,ActiveState,UnitFileState,NextElapseUSecRealtime,LastTriggerUSec,Triggers,Description --no-pager 2>/dev/null`,
	}, "; ")

	raw := runCmd(client, cmd)
	sections := splitSections(raw, delim, 10)

	info := &CronInfo{}
	info.DaemonActive = strings.EqualFold(strings.TrimSpace(sections[0]), "active")
	enabledOut := strings.TrimSpace(sections[1])
	info.DaemonEnabled = strings.EqualFold(enabledOut, "enabled") ||
		strings.EqualFold(enabledOut, "enabled-runtime") ||
		strings.EqualFold(enabledOut, "alias") ||
		strings.EqualFold(enabledOut, "static")
	info.DaemonName = strings.TrimSpace(sections[2])
	info.DaemonInstalled = info.DaemonName != "" || info.DaemonActive

	// /etc/crontab
	for _, line := range strings.Split(sections[3], "\n") {
		if job, ok := parseCronJobLine(line, "system", "cron", true, ""); ok {
			info.Jobs = append(info.Jobs, job)
		}
	}

	// /etc/cron.d/<file>
	parseFileBlocks(sections[4], "###FILE###", func(fileName, body string) {
		source := "/etc/cron.d/" + fileName
		for _, line := range strings.Split(body, "\n") {
			if job, ok := parseCronJobLine(line, source, "cron", true, ""); ok {
				info.Jobs = append(info.Jobs, job)
			}
		}
	})

	// /etc/cron.{hourly,daily,weekly,monthly}/<script>
	for _, line := range strings.Split(sections[5], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir, file := path.Split(line)
		dir = strings.Trim(dir, "/")
		schedule := ""
		switch dir {
		case "cron.hourly":
			schedule = "@hourly"
		case "cron.daily":
			schedule = "@daily"
		case "cron.weekly":
			schedule = "@weekly"
		case "cron.monthly":
			schedule = "@monthly"
		default:
			schedule = dir
		}
		info.Jobs = append(info.Jobs, CronJob{
			Source:   "/etc/" + dir,
			Kind:     "cron",
			Schedule: schedule,
			User:     "root",
			Command:  "/etc/" + dir + "/" + file,
		})
	}

	// Per-user crontabs (sudo -n attempt or current-user fallback)
	userJobs := parseUserCrontabSection(sections[6])
	if len(userJobs) == 0 && sudoPassword != "" {
		// Retry with `sudo -S` for hosts where sudo requires a password.
		sudoCmd := `SP=$([ -d /var/spool/cron/crontabs ] && echo /var/spool/cron/crontabs || echo /var/spool/cron); ` +
			`for u in $SP/*; do [ -f "$u" ] && echo "###USER### $(basename "$u")" && cat "$u"; done`
		if out, err := runSudoCmd(client, sudoPassword, sudoCmd); err == nil {
			userJobs = parseUserCrontabSection(out)
		}
	}
	// Fallback: current user's own crontab. Section 7 is "###USER### <whoami>"
	// followed by the crontab body. Skip if we already have that user's jobs
	// from section 6 (privileged read of /var/spool).
	currentUserJobs := parseUserCrontabSection(sections[7])
	if len(currentUserJobs) > 0 {
		seen := make(map[string]bool, len(userJobs))
		for _, j := range userJobs {
			seen[j.User] = true
		}
		for _, j := range currentUserJobs {
			if !seen[j.User] {
				userJobs = append(userJobs, j)
			}
		}
	}
	info.Jobs = append(info.Jobs, userJobs...)

	// Anacron — format: "<period> <delay> <job-id> <command...>"
	for _, line := range strings.Split(sections[8], "\n") {
		if job, ok := parseAnacronLine(line); ok {
			info.Jobs = append(info.Jobs, job)
		}
	}

	// Systemd timers — `systemctl show` separates per-unit property
	// blocks with a single blank line. The blocks-by-blank-line split
	// (regex `\n\s*\n`) tolerates trailing whitespace from older systemd.
	for _, block := range splitOnBlankLines(sections[9]) {
		if job, ok := parseTimerBlock(block); ok {
			info.Jobs = append(info.Jobs, job)
		}
	}

	return info
}

// splitOnBlankLines splits a multi-line string on runs of blank lines.
// Used to separate `systemctl show <unit1> <unit2> …` output where each
// unit's properties are followed by a blank line.
func splitOnBlankLines(s string) []string {
	if s == "" {
		return nil
	}
	var blocks []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			blocks = append(blocks, current.String())
			current.Reset()
		}
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	flush()
	return blocks
}

// splitSections splits raw on delim and returns at least n sections (padding
// missing trailing sections with empty strings) so callers can use indices
// without bounds checks.
func splitSections(raw, delim string, n int) []string {
	parts := strings.Split(raw, delim)
	out := make([]string, n)
	for i := 0; i < n && i < len(parts); i++ {
		out[i] = strings.TrimSpace(parts[i])
	}
	return out
}

// parseFileBlocks splits s on lines beginning with markerPrefix (e.g.
// "###FILE###") and invokes fn(name, body) for each block. The marker line
// format is "<markerPrefix> <name>" — everything after the prefix up to the
// next marker is the body.
func parseFileBlocks(s, markerPrefix string, fn func(name, body string)) {
	if s == "" {
		return
	}
	var name string
	var body strings.Builder
	flush := func() {
		if name != "" {
			fn(name, body.String())
		}
		name = ""
		body.Reset()
	}
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, markerPrefix) {
			flush()
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, markerPrefix))
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	flush()
}

// parseUserCrontabSection processes a block of "###USER### <name>"-prefixed
// crontabs and returns the parsed jobs. Entries get Source="user:<name>"
// and User=<name> (since user crontab format omits the user field, but
// every job runs as the file owner).
func parseUserCrontabSection(s string) []CronJob {
	var jobs []CronJob
	parseFileBlocks(s, "###USER###", func(user, body string) {
		if user == "" {
			return
		}
		source := "user:" + user
		for _, line := range strings.Split(body, "\n") {
			if job, ok := parseCronJobLine(line, source, "cron", false, user); ok {
				jobs = append(jobs, job)
			}
		}
	})
	return jobs
}

// parseCronJobLine wraps parseCronLine and returns a fully-populated CronJob.
// runAs overrides the user field for user crontabs (where the line format
// has no user field, but every job still runs as the spool file's owner).
func parseCronJobLine(line, source, kind string, hasUserField bool, runAs string) (CronJob, bool) {
	schedule, user, cmd, disabled, ok := parseCronLine(line, hasUserField)
	if !ok {
		return CronJob{}, false
	}
	if user == "" && runAs != "" {
		user = runAs
	}
	return CronJob{
		Source:   source,
		Kind:     kind,
		Schedule: schedule,
		User:     user,
		Command:  cmd,
		Disabled: disabled,
	}, true
}

// parseAnacronLine parses one line from /etc/anacrontab. Format:
//
//	<period-in-days> <delay-in-min> <job-id> <command...>
//
// Period may be a number ("1", "7", "30") or a macro ("@daily", "@weekly",
// "@monthly"). Empty/comment/env-assignment lines return ok=false.
func parseAnacronLine(line string) (CronJob, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return CronJob{}, false
	}
	disabled := false
	if strings.HasPrefix(line, "#") {
		stripped := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if stripped == "" {
			return CronJob{}, false
		}
		job, ok := parseAnacronLine(stripped)
		if !ok {
			return CronJob{}, false
		}
		job.Disabled = true
		return job, true
	}
	// env assignments
	if eq := strings.Index(line, "="); eq > 0 {
		before := line[:eq]
		if !strings.ContainsAny(before, " \t") && isCronEnvKey(before) {
			return CronJob{}, false
		}
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return CronJob{}, false
	}
	period := fields[0]
	if !strings.HasPrefix(period, "@") {
		// must be numeric
		for _, r := range period {
			if r < '0' || r > '9' {
				return CronJob{}, false
			}
		}
		period = fmt.Sprintf("every %s day(s)", fields[0])
	}
	return CronJob{
		Source:   "anacron",
		Kind:     "anacron",
		Schedule: period,
		User:     "root",
		Command:  strings.Join(fields[3:], " "),
		Disabled: disabled,
	}, true
}

// parseTimerBlock parses one KEY=VALUE block from `systemctl show <timer>`.
func parseTimerBlock(block string) (CronJob, bool) {
	props := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		props[line[:eq]] = strings.TrimSpace(line[eq+1:])
	}
	id := props["Id"]
	if id == "" {
		return CronJob{}, false
	}
	state := strings.ToLower(props["ActiveState"])
	command := props["Triggers"]
	if command == "" {
		command = props["Description"]
	}
	return CronJob{
		Source:   "timer",
		Kind:     "timer",
		Schedule: id,
		Command:  command,
		NextRun:  cleanTimerTime(props["NextElapseUSecRealtime"]),
		LastRun:  cleanTimerTime(props["LastTriggerUSec"]),
		Disabled: state == "inactive" || state == "failed",
	}, true
}

// cleanTimerTime normalizes systemd's "n/a" placeholder to an empty string
// so the UI can decide whether to render the slot.
func cleanTimerTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "n/a" || s == "0" {
		return ""
	}
	return s
}

// parseCronLine parses a single line from a crontab file.
// hasUserField=true for system crontabs (/etc/crontab, /etc/cron.d/*) which
// follow "<schedule> <user> <command>". For user crontabs
// (/var/spool/cron/<user>) the format is "<schedule> <command>" without a
// user field (every job runs as the file owner).
//
// Returns ok=false for blank lines, environment assignments
// (PATH=, MAILTO=, SHELL=, etc.), and free-form comments. A "#"-prefixed
// line that itself contains a valid cron entry is returned with
// disabled=true so the UI can render it muted.
func parseCronLine(line string, hasUserField bool) (schedule, user, cmd string, disabled, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, "#") {
		stripped := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if stripped == "" {
			return
		}
		s, u, c, _, parsed := parseCronLine(stripped, hasUserField)
		if !parsed {
			return
		}
		return s, u, c, true, true
	}

	// Skip env assignments: KEY=VALUE where KEY is a valid identifier with
	// no whitespace before the `=`.
	if eq := strings.Index(line, "="); eq > 0 {
		before := line[:eq]
		if !strings.ContainsAny(before, " \t") && isCronEnvKey(before) {
			return
		}
	}

	var sched string
	var rest []string
	if strings.HasPrefix(line, "@") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return
		}
		sched = fields[0]
		rest = fields[1:]
	} else {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			return
		}
		for i := 0; i < 5; i++ {
			if !cronFieldLooksValid(fields[i]) {
				return
			}
		}
		sched = strings.Join(fields[:5], " ")
		rest = fields[5:]
	}

	if hasUserField {
		if len(rest) < 2 {
			return
		}
		if !isCronEnvKey(rest[0]) {
			return
		}
		return sched, rest[0], strings.Join(rest[1:], " "), false, true
	}
	return sched, "", strings.Join(rest, " "), false, true
}

func isCronEnvKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// cronFieldLooksValid reports whether s could plausibly be one of the five
// cron schedule fields (minute/hour/dom/month/dow). Allows digits, the usual
// metacharacters (`*`, `/`, `,`, `-`), and month/day-name aliases like
// "JAN" or "MON". The check is intentionally permissive — its job is only
// to reject lines that obviously aren't cron schedules (e.g. shell commands
// that happened to land at the start of a line).
func cronFieldLooksValid(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '*' || r == '/' || r == ',' || r == '-':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		default:
			return false
		}
	}
	return true
}
