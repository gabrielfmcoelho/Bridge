package sshtest

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ResourceUsageSnapshot is the "who is eating my CPU/RAM/disk" panel for a
// scanned host. The percentages on the main card answer "how loaded is the
// host?"; this answers "who do I yell at?". All three sublists are capped
// at 10 entries each — long enough to surface meaningful offenders, short
// enough that the JSON payload doesn't bloat for hosts with thousands of
// processes.
type ResourceUsageSnapshot struct {
	TopCPU  []ResourceProcess  `json:"top_cpu,omitempty"`
	TopMem  []ResourceProcess  `json:"top_mem,omitempty"`
	TopDisk []ResourceDiskItem `json:"top_disk,omitempty"`
}

// ResourceProcess captures one row from `ps`. Command holds the full argv
// (truncated to ~200 chars so a long Java classpath doesn't dominate the
// scan payload). RSSBytes is the resident set in bytes — what the OS
// actually has resident in RAM right now.
type ResourceProcess struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu_percent"`
	MEM     float64 `json:"mem_percent"`
	RSSBytes int64  `json:"rss_bytes"`
	Command string  `json:"command"`
}

// ResourceDiskItem is one path's disk footprint. We probe a curated set of
// well-known bloat locations rather than `du /` (which would walk the whole
// filesystem and could take minutes on a large host).
type ResourceDiskItem struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	HumanSize string `json:"human_size,omitempty"`
}

// commandFieldLimit caps the per-process argv length we ship to the
// frontend. Long Java/JVM classpaths and node_modules launchers easily
// exceed 1KB; truncating here keeps the snapshot section comfortably
// under a few KB even on busy hosts.
const commandFieldLimit = 200

// diskProbePaths is the curated list of directories we size up. These are
// the usual culprits for "where did all the disk go?" questions:
//   - /var/log:      journald, app logs
//   - /var/lib/docker: container images, volumes, overlay2
//   - /var/cache:    apt/yum/dnf caches
//   - /var/tmp /tmp: lingering temp files
//   - /opt:          third-party app installs
//   - /home /root:   user data
//   - /usr/local:    locally compiled software
//   - /var/lib/postgresql /var/lib/mysql: DB datadirs
//   - /var/lib/snapd: snap caches/installs
var diskProbePaths = []string{
	"/var/log",
	"/var/lib/docker",
	"/var/cache",
	"/var/tmp",
	"/tmp",
	"/opt",
	"/home",
	"/root",
	"/usr/local",
	"/var/lib/postgresql",
	"/var/lib/mysql",
	"/var/lib/snapd",
}

// captureResourceTop runs three lightweight probes in one SSH session:
//   - top 10 processes by %CPU
//   - top 10 processes by %MEM
//   - sizes of well-known bloat directories
// Each section is delimited so we can split with strings.Split and parse
// independently. A 10-second `timeout` wraps the disk probe because a
// heavily-fragmented filesystem can occasionally make `du` slow; we'd
// rather return partial data than hang the scan.
func captureResourceTop(client *ssh.Client) *ResourceUsageSnapshot {
	const delim = "---RESOURCE---"

	// Build the disk-probe arg list once. shellEscape isn't strictly
	// required (the paths are static and safe) but keeps us consistent
	// with the rest of the codebase.
	probeArgs := make([]string, 0, len(diskProbePaths))
	for _, p := range diskProbePaths {
		probeArgs = append(probeArgs, shellEscape(p))
	}
	diskCmd := `timeout 10s du -sb ` + strings.Join(probeArgs, " ") + ` 2>/dev/null | sort -k1 -n -r | head -10`

	cmd := strings.Join([]string{
		// Top 10 by %CPU. --no-headers + --sort=-%cpu gives us the rows
		// already sorted; head -10 caps the payload.
		`ps -eo pid,user,%cpu,%mem,rss,args --sort=-%cpu --no-headers 2>/dev/null | head -10`,
		`echo '` + delim + `'`,
		`ps -eo pid,user,%cpu,%mem,rss,args --sort=-%mem --no-headers 2>/dev/null | head -10`,
		`echo '` + delim + `'`,
		diskCmd,
	}, "; ")

	raw := runCmd(client, cmd)
	sections := strings.Split(raw, delim)
	section := func(i int) string {
		if i < len(sections) {
			return strings.TrimSpace(sections[i])
		}
		return ""
	}

	snap := &ResourceUsageSnapshot{
		TopCPU:  parseTopProcesses(section(0)),
		TopMem:  parseTopProcesses(section(1)),
		TopDisk: parseDiskTop(section(2)),
	}
	return snap
}

// parseTopProcesses parses `ps -eo pid,user,%cpu,%mem,rss,args` rows. The
// args column contains spaces, so we slice off the first 5 fields by
// width-aware splitting (strings.Fields collapses runs of whitespace, so
// the 6th "field" via Fields would be just argv[0]). Instead, use Fields
// for the first 5 and treat the rest of the line as the full command.
func parseTopProcesses(raw string) []ResourceProcess {
	if raw == "" {
		return nil
	}
	var out []ResourceProcess
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split off the first 5 columns, then everything else is argv.
		fields := strings.SplitN(line, " ", 6)
		// Filter out empty intermediate splits (multiple spaces).
		var clean []string
		for _, f := range fields {
			if f != "" {
				clean = append(clean, f)
			}
		}
		// strings.SplitN with " " doesn't collapse runs, so on rows
		// where ps padded columns we may end up with fewer than 6
		// non-empty entries. Re-split with Fields to recover the 5
		// numeric columns then take the rest of the line as args.
		all := strings.Fields(line)
		if len(all) < 6 {
			continue
		}
		pid, err := strconv.Atoi(all[0])
		if err != nil {
			continue
		}
		cpuPct, _ := strconv.ParseFloat(all[2], 64)
		memPct, _ := strconv.ParseFloat(all[3], 64)
		rssKB, _ := strconv.ParseInt(all[4], 10, 64)
		// Reconstruct args by trimming the first 5 fields off the
		// original line. Find the position of the 5th field in `line`
		// and keep everything after it.
		args := extractCommand(line, all[:5])
		if len(args) > commandFieldLimit {
			args = args[:commandFieldLimit] + "…"
		}
		out = append(out, ResourceProcess{
			PID:      pid,
			User:     all[1],
			CPU:      cpuPct,
			MEM:      memPct,
			RSSBytes: rssKB * 1024, // ps reports RSS in KB
			Command:  args,
		})
	}
	return out
}

// extractCommand slices off the first 5 whitespace-separated columns from
// the original `ps` row to recover the full argv (which can contain
// internal whitespace that strings.Fields would split). We walk the
// original line, skipping each known prefix column plus the run of
// whitespace that follows it.
func extractCommand(line string, knownFields []string) string {
	pos := 0
	for _, f := range knownFields {
		// Skip leading whitespace.
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}
		// Skip the field itself. We use length match because the
		// caller passed exactly the strings.Fields result.
		if pos+len(f) > len(line) {
			return ""
		}
		pos += len(f)
	}
	// Skip the whitespace before argv.
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	return line[pos:]
}

// parseDiskTop parses `du -sb` output: "<bytes>\t<path>" or "<bytes> <path>"
// depending on coreutils version. Sorted descending in the shell already.
func parseDiskTop(raw string) []ResourceDiskItem {
	if raw == "" {
		return nil
	}
	var out []ResourceDiskItem
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split on the first run of whitespace. du uses tab on GNU but
		// some distros (or `du -B1`) use a single space.
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		size, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		path := strings.Join(fields[1:], " ")
		out = append(out, ResourceDiskItem{
			Path:      path,
			SizeBytes: size,
			HumanSize: formatBytesBinary(size),
		})
	}
	return out
}

// formatBytesBinary returns "1.4 GiB"-style strings using binary units.
// Mirrors the docker logs report formatter so identical sizes look
// identical across panels.
func formatBytesBinary(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	const unit = 1024.0
	div, exp := unit, 0
	for v := float64(n) / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}[exp]
	return fmt.Sprintf("%.1f %s", float64(n)/div, suffix)
}
