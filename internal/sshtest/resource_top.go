package sshtest

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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

// ResourceDiskItem is one of the largest files across the host's local
// partitions (diskTopScript).
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

// captureResourceTop answers "who is eating CPU / RAM / disk": the top 10
// processes by %CPU and by %MEM (one quick session), then the 10 largest
// files across every local partition (diskTopScript, its own session — as
// root with the stored password, so root-only data dirs count).
func captureResourceTop(client *ssh.Client, sudoPassword string) *ResourceUsageSnapshot {
	const delim = "---RESOURCE---"
	cmd := strings.Join([]string{
		// Top 10 by %CPU. --no-headers + --sort=-%cpu gives us the rows
		// already sorted; head -10 caps the payload.
		`ps -eo pid,user,%cpu,%mem,rss,args --sort=-%cpu --no-headers 2>/dev/null | head -10`,
		`echo '` + delim + `'`,
		`ps -eo pid,user,%cpu,%mem,rss,args --sort=-%mem --no-headers 2>/dev/null | head -10`,
	}, "; ")
	sections := strings.Split(runCmd(client, cmd), delim)
	section := func(i int) string {
		if i < len(sections) {
			return strings.TrimSpace(sections[i])
		}
		return ""
	}

	disk := ""
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, diskTopScript(true))
		if out != "" {
			disk = out
		} else if err != nil {
			log.Printf("[sshscan] largest files as root failed, falling back to unprivileged: %v", err)
		}
	}
	if disk == "" {
		disk, _ = runCmdWithDeadline(client, encodedScript(diskTopScript(false), "sh"), diskTopTimeout+15*time.Second)
	}
	return &ResourceUsageSnapshot{
		TopCPU:  parseTopProcesses(section(0)),
		TopMem:  parseTopProcesses(section(1)),
		TopDisk: parseDiskTop(disk),
	}
}

// diskTopTimeout bounds the find walk: it reads every inode of every local
// partition, at idle I/O priority; on timeout the largest files found so
// far are what's reported.
// ponytail: a full metadata walk per scan; cache it or sample if a host with
// tens of millions of files makes scans slow.
const diskTopTimeout = 45 * time.Second

// diskTopScript lists the 10 largest files (over 10 MB) of every local
// partition — the same set the partition table shows (no tmpfs, overlays,
// snaps, container-runtime mounts) — as "<bytes>\t<path>". Each start point
// stays on its own filesystem (-xdev). Not root: find goes through sudo -n
// when that is allowed, else sees what the SSH user can read.
func diskTopScript(asRoot bool) string {
	elevate := `S=""; sudo -n true 2>/dev/null && S="sudo -n"`
	if asRoot {
		elevate = `S=""`
	}
	return `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout ` + strconv.Itoa(int(diskTopTimeout.Seconds())) + `"
N=""; command -v ionice >/dev/null 2>&1 && N="ionice -c3"; command -v nice >/dev/null 2>&1 && N="$N nice -n 19"
` + elevate + `
M=$(df -lP -x tmpfs -x devtmpfs -x overlay -x squashfs -x efivarfs 2>/dev/null | awk 'NR>1 && $6 !~ "^/(run|snap|sys|proc|dev)(/|$)" && $6 !~ "^/var/lib/docker/" {print $6}')
[ -n "$M" ] && $S $T $N find $M -xdev -type f -size +10M -printf '%s\t%p\n' 2>/dev/null | sort -nr | head -10
`
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
