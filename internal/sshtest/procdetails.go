package sshtest

import (
	"strings"

	"golang.org/x/crypto/ssh"
)

const procSep = "\x1f"

// processDetailsScript prints one line per matching process (read-only):
// PID US USER US CPU US MEM US CWD US EXE US PCOMM US CGROUP-LINE US PORTS US CMD,
// with US = 0x1f and the command last so a "|" inside it survives. App-server
// JVMs (a main class detectAppServers knows) are listed first, up to 30, then
// up to 20 other runtimes: a host with many workers before its JBoss in PID
// order still reports the JBoss.
func processDetailsScript() string {
	const (
		sel = `$11 ~ /(python|node|java|ruby|php|gunicorn|uvicorn|celery|pm2)/ && $11 !~ /(^|\/)(grep|ps|awk)$/`
		row = `{printf "%s %s %s %s", $2, $1, $3, $4; for(i=11;i<=NF;i++) printf " %s", $i; print ""}`
	)
	var classes []string
	for _, spec := range serviceCatalog {
		for _, c := range spec.MainClasses {
			classes = append(classes, strings.ReplaceAll(c, ".", `\.`))
		}
	}
	app := "(" + strings.Join(classes, "|") + ")"
	return strings.Join([]string{
		scanPath,
		`ss_out=$(ss -tlnp 2>/dev/null)`,
		`{ ps aux 2>/dev/null | awk '` + sel + ` && $0 ~ /` + app + `/ ` + row + `' | head -30
  ps aux 2>/dev/null | awk '` + sel + ` && $0 !~ /` + app + `/ ` + row + `' | head -20; } | while read -r pid user cpu mem cmd; do
	cwd=$(readlink /proc/$pid/cwd 2>/dev/null)
	exe=$(readlink /proc/$pid/exe 2>/dev/null)
	ppid=$(awk '/^PPid:/{print $2}' /proc/$pid/status 2>/dev/null)
	pcomm=$(cat /proc/$ppid/comm 2>/dev/null)
	cg=$(grep -E 'name=systemd|^0::' /proc/$pid/cgroup 2>/dev/null | head -1)
	ports=$(printf '%s\n' "$ss_out" | grep "pid=$pid," | awk '{split($4,a,":"); print a[length(a)]}' | sort -nu | tr '\n' ',' | sed 's/,$//')
	printf '%s\037%s\037%s\037%s\037%s\037%s\037%s\037%s\037%s\037%s\n' "$pid" "$user" "$cpu" "$mem" "$cwd" "$exe" "$pcomm" "$cg" "$ports" "$cmd"
done`,
	}, "\n")
}

// unitFromCgroup returns the systemd service a /proc/<pid>/cgroup names, "" when
// the process sits in a scope, slice or no unit. The name=systemd (v1) line wins.
func unitFromCgroup(cgroup string) string {
	line := ""
	for _, l := range strings.Split(cgroup, "\n") {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "name=systemd:") {
			line = l
			break
		}
		if line == "" && strings.HasPrefix(l, "0::") {
			line = l
		}
	}
	if i := strings.LastIndex(line, ":"); i >= 0 {
		line = line[i+1:]
	}
	unit := ""
	for _, c := range strings.Split(line, "/") {
		if strings.HasSuffix(c, ".service") {
			unit = c
		}
	}
	return unit
}

func parseProcessDetails(raw string) []ProcessDetail {
	var details []ProcessDetail
	for _, line := range splitLines(raw) {
		f := strings.SplitN(line, procSep, 10)
		if len(f) < 10 || strings.TrimSpace(f[9]) == "" {
			continue
		}
		exe, parentComm := f[5], f[6]
		unit := unitFromCgroup(f[7])
		_, inContainer := cgroupContainer(f[7])

		venv := ""
		for _, marker := range []string{"/venv/", "/.venv/", "/virtualenv/", "/env/"} {
			if idx := strings.Index(exe, marker); idx >= 0 {
				venv = exe[:idx+len(marker)-1]
				break
			}
		}

		startedVia := "unknown"
		switch parentComm {
		case "systemd", "init":
			startedVia = "systemd"
		case "cron", "crond", "anacron":
			startedVia = "cron"
		case "sshd", "bash", "sh", "zsh", "tmux: server", "screen":
			startedVia = "manual"
		case "supervisord":
			startedVia = "supervisor"
		case "containerd-shim", "docker":
			startedVia = "docker"
		default:
			if parentComm != "" {
				startedVia = "parent:" + parentComm
			}
		}
		if unit != "" {
			startedVia = "systemd"
		}

		details = append(details, ProcessDetail{
			PID: f[0], User: f[1], CPU: f[2] + "%", MEM: f[3] + "%",
			Command: strings.TrimSpace(f[9]), CWD: f[4], StartedVia: startedVia,
			Venv: venv, Ports: f[8], Unit: unit, InContainer: inContainer,
		})
	}
	return details
}

// captureProcessDetails runs the script in one session, as root when the stored
// password allows it (root's /proc/*/cwd and socket owners are invisible otherwise).
func captureProcessDetails(client *ssh.Client, sudoPassword string) []ProcessDetail {
	script := processDetailsScript()
	if sudoPassword != "" {
		if out, err := runSudoCmd(client, sudoPassword, script); err == nil {
			return parseProcessDetails(out)
		}
	}
	return parseProcessDetails(runCmd(client, script))
}
