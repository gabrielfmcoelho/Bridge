package sshtest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// DockerLogsReport summarizes log-rotation health for a Docker host. It
// answers two questions:
//   1. Is global log rotation configured? (Without it, container logs grow
//      until the disk fills up — a common cause of "host suddenly broken"
//      tickets.)
//   2. Which containers are sitting on large log files right now?
//
// The report is read-only; the operator decides whether to apply the
// recommended /etc/docker/daemon.json policy via DockerLogsApplyRotation.
type DockerLogsReport struct {
	// Engine view of the active log driver and its global options.
	// Source: `docker info --format '{{json .}}'`. When the driver is
	// "json-file" (the default) and no max-size is set, every container
	// inherits unbounded logs — that's the dangerous case.
	LogDriver  string            `json:"log_driver"`
	LogOptions map[string]string `json:"log_options,omitempty"`

	// What /etc/docker/daemon.json actually says, parsed when readable.
	// Differs from LogOptions when daemon.json was edited after the
	// daemon last reloaded — surfacing both lets the operator catch
	// "config drift" cases.
	DaemonJSONPath    string            `json:"daemon_json_path,omitempty"`
	DaemonJSONExists  bool              `json:"daemon_json_exists"`
	DaemonLogDriver   string            `json:"daemon_log_driver,omitempty"`
	DaemonLogOpts     map[string]string `json:"daemon_log_opts,omitempty"`
	DaemonJSONUnclean bool              `json:"daemon_json_unclean,omitempty"` // failed to parse JSON

	// Per-container log sizes. Sorted descending by size so the noisy
	// containers float to the top of the UI.
	Containers []DockerContainerLogInfo `json:"containers,omitempty"`

	// Aggregate metrics — the headline numbers for the operator.
	TotalLogBytes      int64 `json:"total_log_bytes"`
	LargestLogBytes    int64 `json:"largest_log_bytes"`
	UnboundedContainers int  `json:"unbounded_containers"` // count with neither container-level nor daemon-level rotation

	// Verdict + recommendation, computed server-side so the UI doesn't
	// re-derive the same logic.
	RotationConfigured bool   `json:"rotation_configured"`
	RiskLevel          string `json:"risk_level"` // "ok" | "warning" | "critical"
	Recommendation     string `json:"recommendation,omitempty"`
}

// DockerContainerLogInfo is one container's log-file state. Path is the
// absolute log file (typically /var/lib/docker/containers/<id>/<id>-json.log
// for json-file driver) so the operator can verify it exists. SizeBytes is
// the actual on-disk size at scan time.
type DockerContainerLogInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	LogPath    string            `json:"log_path,omitempty"`
	SizeBytes  int64             `json:"size_bytes"`
	HumanSize  string            `json:"human_size,omitempty"`
	LogDriver  string            `json:"log_driver,omitempty"`
	LogOptions map[string]string `json:"log_options,omitempty"`
	HasRotation bool             `json:"has_rotation"` // either container-level max-size or daemon-level
}

// CaptureDockerLogs collects the report. sudoPassword is optional — without
// it some containers' log files (owned by root, mode 0640) may not be
// readable; the function falls back to `sudo -n du -b` and then to "size
// unknown" rather than failing the whole op.
//
// Returns an error only when docker isn't reachable at all (daemon down or
// CLI missing). Partial data ("we couldn't read daemon.json but containers
// were enumerated") yields a populated report with the relevant fields
// blank so the UI can show what was retrievable.
func CaptureDockerLogs(client *ssh.Client, sudoPassword string) (*DockerLogsReport, error) {
	report := &DockerLogsReport{LogOptions: map[string]string{}}

	// Engine-level view: log driver + options. `docker info --format`
	// emits a Go template; we ask for JSON so we can safely parse without
	// regex.
	infoOut, _ := runCmdRaw(client, `docker info --format '{{json .}}' 2>/dev/null || sudo -n docker info --format '{{json .}}' 2>/dev/null`)
	if strings.TrimSpace(infoOut) == "" {
		return nil, fmt.Errorf("docker not reachable on host (daemon stopped, not installed, or no permission)")
	}
	var info struct {
		LoggingDriver string   `json:"LoggingDriver"`
		ServerVersion string   `json:"ServerVersion"`
		Plugins       struct{} `json:"Plugins"`
	}
	if err := json.Unmarshal([]byte(infoOut), &info); err == nil {
		report.LogDriver = info.LoggingDriver
	}

	// Read /etc/docker/daemon.json — both the unprivileged path (works
	// when the file is world-readable) and a sudo fallback. If both
	// fail we leave DaemonLogDriver/Opts empty.
	const daemonJSON = "/etc/docker/daemon.json"
	report.DaemonJSONPath = daemonJSON
	daemonRaw, _ := runCmdRaw(client, `cat `+daemonJSON+` 2>/dev/null || sudo -n cat `+daemonJSON+` 2>/dev/null`)
	if sudoPassword != "" && strings.TrimSpace(daemonRaw) == "" {
		// Privileged retry only if needed (avoids burning PAM faildelay
		// when the file just doesn't exist).
		if out, err := runSudoCmd(client, sudoPassword, "cat "+daemonJSON+" 2>/dev/null"); err == nil {
			daemonRaw = out
		}
	}
	if strings.TrimSpace(daemonRaw) != "" {
		report.DaemonJSONExists = true
		var parsed struct {
			LogDriver string                 `json:"log-driver"`
			LogOpts   map[string]interface{} `json:"log-opts"`
		}
		if err := json.Unmarshal([]byte(daemonRaw), &parsed); err == nil {
			report.DaemonLogDriver = parsed.LogDriver
			if parsed.LogOpts != nil {
				report.DaemonLogOpts = stringifyMap(parsed.LogOpts)
			}
		} else {
			report.DaemonJSONUnclean = true
		}
	}

	// Per-container log path + size + container-level log driver/opts.
	// `docker inspect` emits one JSON object per container; the loop
	// runs all containers in a single SSH session via `xargs`-style
	// expansion. The `du -b` reports apparent size in bytes; if the
	// log file path is null (shouldn't happen for json-file but does
	// for journald) we mark size as 0 and skip the du call.
	listOut, _ := runCmdRaw(client, `docker ps -aq 2>/dev/null || sudo -n docker ps -aq 2>/dev/null`)
	ids := strings.Fields(strings.TrimSpace(listOut))
	if len(ids) > 0 {
		// Inspect everything in one shot. The Go template emits a
		// pipe-delimited record per container so we don't have to JSON-
		// parse a giant array.
		fmtTpl := `{{.Id}}|{{.Name}}|{{.Config.Image}}|{{.LogPath}}|{{.HostConfig.LogConfig.Type}}|{{json .HostConfig.LogConfig.Config}}`
		inspectCmd := fmt.Sprintf(`docker inspect --format '%s' %s 2>/dev/null`, fmtTpl, strings.Join(ids, " "))
		inspectOut, _ := runCmdRaw(client, inspectCmd)
		if strings.TrimSpace(inspectOut) == "" {
			// Fallback to sudo if non-root can't inspect.
			inspectOut, _ = runCmdRaw(client, "sudo -n "+inspectCmd)
			if strings.TrimSpace(inspectOut) == "" && sudoPassword != "" {
				if out, err := runSudoCmd(client, sudoPassword, inspectCmd); err == nil {
					inspectOut = out
				}
			}
		}

		// Build du command for log paths we'll need to size up.
		var logPaths []string
		var entries []DockerContainerLogInfo
		for _, line := range splitLines(inspectOut) {
			parts := strings.SplitN(line, "|", 6)
			if len(parts) < 6 {
				continue
			}
			driver := parts[4]
			var opts map[string]string
			if parts[5] != "" && parts[5] != "null" {
				var raw map[string]interface{}
				if err := json.Unmarshal([]byte(parts[5]), &raw); err == nil {
					opts = stringifyMap(raw)
				}
			}
			entry := DockerContainerLogInfo{
				ID:         parts[0],
				Name:       strings.TrimPrefix(parts[1], "/"),
				Image:      parts[2],
				LogPath:    parts[3],
				LogDriver:  driver,
				LogOptions: opts,
			}
			if entry.LogPath != "" && entry.LogPath != "<no value>" {
				logPaths = append(logPaths, entry.LogPath)
			}
			entries = append(entries, entry)
		}

		// Size every log file in one SSH session. `du -b` gives bytes;
		// stat fallback handles BusyBox du which doesn't accept -b.
		// Privilege-escalate only if the unprivileged read returned
		// nothing — most json-file logs are root-owned mode 0640.
		sizes := map[string]int64{}
		if len(logPaths) > 0 {
			var b strings.Builder
			b.WriteString(`for f in `)
			for i, p := range logPaths {
				if i > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(shellEscape(p))
			}
			b.WriteString(`; do `)
			b.WriteString(`if [ -r "$f" ]; then sz=$(stat -c '%s' "$f" 2>/dev/null); echo "$sz $f"; fi; `)
			b.WriteString(`done`)
			out, _ := runCmdRaw(client, b.String())
			if strings.TrimSpace(out) == "" {
				if sudoPassword != "" {
					if so, serr := runSudoCmd(client, sudoPassword, b.String()); serr == nil {
						out = so
					}
				} else {
					if so, _ := runCmdRaw(client, "sudo -n "+b.String()); strings.TrimSpace(so) != "" {
						out = so
					}
				}
			}
			for _, line := range splitLines(out) {
				fields := strings.SplitN(line, " ", 2)
				if len(fields) != 2 {
					continue
				}
				if n, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
					sizes[fields[1]] = n
				}
			}
		}

		// Stitch sizes into entries and compute per-container rotation
		// status. A container has rotation iff (a) its own LogConfig
		// sets max-size, OR (b) the daemon default max-size applies and
		// the container inherits the default driver. Daemon-level
		// max-size only applies to containers using the daemon's driver
		// without overriding it.
		for i := range entries {
			e := &entries[i]
			if sz, ok := sizes[e.LogPath]; ok {
				e.SizeBytes = sz
				e.HumanSize = humanizeBytes(sz)
			}
			containerHasMaxSize := e.LogOptions["max-size"] != ""
			daemonHasMaxSize := report.DaemonLogOpts["max-size"] != "" &&
				(e.LogDriver == "" || e.LogDriver == "json-file" || e.LogDriver == report.DaemonLogDriver)
			e.HasRotation = containerHasMaxSize || daemonHasMaxSize
			if !e.HasRotation {
				report.UnboundedContainers++
			}
			report.TotalLogBytes += e.SizeBytes
			if e.SizeBytes > report.LargestLogBytes {
				report.LargestLogBytes = e.SizeBytes
			}
		}
		// Largest first — operator wants the noisy containers up top.
		sort.Slice(entries, func(i, j int) bool { return entries[i].SizeBytes > entries[j].SizeBytes })
		report.Containers = entries
	}

	// Daemon-level rotation flag. We treat "max-size set on json-file"
	// as the canonical "rotation configured" verdict — that's the lever
	// every operator gets via daemon.json. Other drivers (journald,
	// fluentd) handle rotation externally and we don't second-guess.
	report.RotationConfigured = report.DaemonLogOpts["max-size"] != "" ||
		(report.DaemonLogDriver != "" && report.DaemonLogDriver != "json-file")
	switch {
	case report.UnboundedContainers > 0 && report.TotalLogBytes > 1<<30: // 1 GiB
		report.RiskLevel = "critical"
		report.Recommendation = "Total container logs exceed 1 GiB and rotation is not enforced. Apply the recommended daemon.json policy below to cap each container at 30 MiB × 3 files."
	case report.UnboundedContainers > 0:
		report.RiskLevel = "warning"
		report.Recommendation = "Some containers have no log rotation configured. Apply a daemon.json policy or set per-container --log-opt max-size."
	case !report.RotationConfigured:
		report.RiskLevel = "warning"
		report.Recommendation = "Docker daemon is using the default json-file driver without max-size. Logs will grow until disk fills."
	default:
		report.RiskLevel = "ok"
		report.Recommendation = ""
	}
	return report, nil
}

// DockerLogsApplyRotation writes /etc/docker/daemon.json with sensible log
// rotation defaults and reloads the daemon. Existing daemon.json content
// (other keys: registry mirrors, insecure registries, storage driver, …)
// is preserved by reading + merging. Requires sudoPassword.
//
// On success the function reloads the docker daemon — preferred over
// restart because reload preserves running containers. Some old daemons
// don't support SIGHUP for log-driver options; we fall back to restart in
// that case (warning surfaced via the returned message).
type DockerLogsRotationOptions struct {
	MaxSize string // e.g. "30m"
	MaxFile int    // e.g. 3 → keep 3 rotated files
	Driver  string // "json-file" (default) or "local" (binary, smaller)
}

// DockerLogsApplyRotation merges the requested rotation policy into
// /etc/docker/daemon.json on the remote host and triggers a daemon
// reload. The merge preserves any unrelated keys already in the file
// (storage-driver, registry-mirrors, …). Returns the updated daemon.json
// content and a status message describing what happened.
func DockerLogsApplyRotation(client *ssh.Client, sudoPassword string, opts DockerLogsRotationOptions) (string, string, error) {
	if sudoPassword == "" {
		return "", "", fmt.Errorf("sudo password required to write /etc/docker/daemon.json")
	}
	if opts.MaxSize == "" {
		opts.MaxSize = "30m"
	}
	if opts.MaxFile <= 0 {
		opts.MaxFile = 3
	}
	if opts.Driver == "" {
		opts.Driver = "json-file"
	}

	// Read existing config (best-effort) so we can preserve other keys.
	existing, _ := runSudoCmd(client, sudoPassword, "cat /etc/docker/daemon.json 2>/dev/null")
	parsed := map[string]interface{}{}
	if strings.TrimSpace(existing) != "" {
		if err := json.Unmarshal([]byte(existing), &parsed); err != nil {
			return "", "", fmt.Errorf("existing /etc/docker/daemon.json is not valid JSON; refusing to overwrite (got: %v)", err)
		}
	}
	parsed["log-driver"] = opts.Driver
	logOpts := map[string]string{
		"max-size": opts.MaxSize,
		"max-file": strconv.Itoa(opts.MaxFile),
	}
	parsed["log-opts"] = logOpts

	merged, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal daemon.json: %w", err)
	}
	mergedStr := string(merged) + "\n"

	// Atomic write: tee to a tmpfile, then mv into place. Avoids a half-
	// written daemon.json if the SSH connection drops mid-write.
	script := `set -e
mkdir -p /etc/docker
TMP=$(mktemp /etc/docker/daemon.json.XXXXXX)
cat <<'EOF_DAEMON_JSON' > "$TMP"
` + mergedStr + `EOF_DAEMON_JSON
chmod 0644 "$TMP"
mv "$TMP" /etc/docker/daemon.json
# Prefer reload (preserves running containers); fall back to restart only
# if the daemon doesn't accept the new options without restarting.
if systemctl reload docker 2>/dev/null; then
  echo "RELOADED"
else
  systemctl restart docker
  echo "RESTARTED"
fi`
	out, err := runSudoCmd(client, sudoPassword, script)
	if err != nil {
		return mergedStr, out, fmt.Errorf("apply rotation: %w", err)
	}

	msg := "Log rotation applied: max-size=" + opts.MaxSize + ", max-file=" + strconv.Itoa(opts.MaxFile) + ", driver=" + opts.Driver
	if strings.Contains(out, "RESTARTED") {
		msg += " (docker daemon was restarted because reload wasn't supported — running containers were briefly stopped)"
	} else {
		msg += " (docker daemon reloaded; running containers were not affected)"
	}
	return mergedStr, msg, nil
}

// stringifyMap coerces a parsed json map[string]interface{} into
// map[string]string. Numbers stringify via Sprintf, nested maps/slices
// JSON-encode back to a single-line string so the UI can show them.
func stringifyMap(in map[string]interface{}) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch t := v.(type) {
		case string:
			out[k] = t
		case float64:
			out[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			out[k] = strconv.FormatBool(t)
		default:
			if b, err := json.Marshal(v); err == nil {
				out[k] = string(b)
			}
		}
	}
	return out
}

// humanizeBytes turns a byte count into a short string operators read at
// a glance ("1.4 GiB", "230 MiB"). Uses binary units (1024-based) since
// docker uses MiB/GiB conventions.
func humanizeBytes(n int64) string {
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
