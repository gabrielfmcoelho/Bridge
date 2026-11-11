package sshtest

import (
	"encoding/base64"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/coolify"
)

// GitRemote is one remote of a git checkout found by the scan, on the host
// (Container "") or inside a running container. The scan stores it in the
// host_scans JSON; service.RecordGitLinks links the host to the repos on the
// configured GitLab.
type GitRemote struct {
	Container      string `json:"container,omitempty"`
	ContainerImage string `json:"container_image,omitempty"`
	Path           string `json:"path"` // the working tree (the .git directory's parent)
	Name           string `json:"name"` // remote name, e.g. origin
	URL            string `json:"url"`  // credentials stripped
	// The checkout's HEAD and when it last came from the remote. Per
	// checkout, so every remote of one checkout carries the same values.
	Commit   string `json:"commit,omitempty"`    // 40 or 64 hex
	Branch   string `json:"branch,omitempty"`    // "" when HEAD is detached
	PulledAt int64  `json:"pulled_at,omitempty"` // unix seconds
	PullKind string `json:"pull_kind,omitempty"` // clone, pull or fetch
}

// The whole probe must finish well inside the browser's 120 s scan request
// (DEFAULT_REQUEST_TIMEOUT_MS), or the scan itself is lost: host walk ≤ 20 s
// (in gitInnerScript), containers started within 30 s, each ≤ 8 s.
const (
	maxGitContainers = 30
	gitOutputCap     = 131072
	gitCtrTimeout    = 8
	gitCtrBudget     = 30
	gitProbeTimeout  = 60 * time.Second
	maxGitPathLen    = 512
	maxGitURLLen     = 1024
)

// gitWalkScript walks "/" and every other local filesystem (-xdev each,
// depth 6; pseudo, docker — wherever its data root is —, podman, cache and
// snapshot trees and node_modules pruned) for .git directories, up to 100.
// Per checkout: "@@repo <worktree>", then only its config's section headers
// and url lines — headers other than [remote "…"] blanked to [x],
// user:password@ cut; git's names are case-insensitive, so the matching is
// too. Nothing else of the config (extraHeader, credential helpers) leaves
// the machine. @@gitend closes the run only when find exited 0 or 1 (1 = some
// directory unreadable) and the cap wasn't hit, so a cut list never deletes
// links. find's status stays out of the output: a directory name can hold any
// line. timeout(1) is used only if "timeout 1 true" works (old busybox wants
// -t). GITPROBE_ROOTS overrides the start points (tests).
// After "@@repo", "@@head <commit> <branch> <unix> <kind>" ("-" = unknown):
// HEAD read from the files (no git binary needed), the branch's ref only
// under refs/heads with no ".." (the walk may run as root: a planted HEAD
// must not make it read another file), the commit printed only if hex. The
// time is the reflog's last clone/pull, or FETCH_HEAD's mtime when a fetch
// came later; of the reflog only that number and word leave (its lines also
// hold the author and the clone URL).
// Every file is read only after [ -f ]: opening a FIFO there would block the walk.
// ponytail: refs/heads only; a HEAD on a tag or a nested symref reads as unknown.
const gitWalkScript = `T=""; TI=""; command -v timeout >/dev/null 2>&1 && timeout 1 true 2>/dev/null && T="timeout 20" && TI="timeout 5"
R="$GITPROBE_ROOTS"
[ -n "$R" ] || R="/ $(awk '$3 ~ /^(ext[234]|xfs|btrfs|zfs|f2fs)$/ && $2 != "/" {print $2}' /proc/mounts 2>/dev/null | sort -u | tr '\n' ' ')"
X=""; DR=$($TI docker info --format '{{.DockerRootDir}}' 2>/dev/null); case "$DR" in /?*) X="-o -path $DR";; esac
L=$($T find $R -xdev -maxdepth 6 \( -path /proc -o -path /sys -o -path /dev -o -path /run -o -path /var/lib/docker -o -path /var/lib/containerd -o -path /var/lib/containers -o -path /snap -o -path /var/cache -o -path /.snapshots -o -path /usr/lib -o -name node_modules -o -name .cache $X \) -prune -o -type d -name .git -print -prune 2>/dev/null)
rc=$?
printf '%s\n' "$L" | head -n 101 | { n=0; while IFS= read -r g; do
  [ -n "$g" ] || continue
  n=$((n+1)); [ "$n" -gt 100 ] && exit 0
  [ -f "$g/config" ] || continue
  printf '@@repo %s\n' "${g%/.git}"
  C=-; B=-; H=$([ -f "$g/HEAD" ] && head -c 300 "$g/HEAD" 2>/dev/null | head -n 1)
  case "$H" in
    "ref: refs/heads/"?*) r=${H#ref: }
      case "$r" in *..*|*[!A-Za-z0-9._/-]*) ;; *) B=${r#refs/heads/}
        C=$([ -f "$g/$r" ] && head -c 100 "$g/$r" 2>/dev/null | head -n 1)
        [ -n "$C" ] || C=$([ -f "$g/packed-refs" ] && awk -v r="$r" '$2 == r { print $1; exit }' "$g/packed-refs" 2>/dev/null);; esac;;
    "ref: "*) ;;
    *) C=$H;;
  esac
  case "$C" in ""|*[!0-9a-f]*) C=-;; esac
  P=$([ -f "$g/logs/HEAD" ] && tail -n 2000 "$g/logs/HEAD" 2>/dev/null | awk -F'\t' '$2 ~ /^(pull|clone)/ { n = split($1, a, " "); w = $2; sub(/[ :].*/, "", w); if (a[n-1] ~ /^[0-9]+$/ && (w == "pull" || w == "clone")) { t = a[n-1]; k = w } } END { if (t != "") print t, k }')
  F=$(stat -c %Y "$g/FETCH_HEAD" 2>/dev/null)
  case "$F" in ""|*[!0-9]*) ;; *) { [ -n "$P" ] && [ "${P%% *}" -ge "$F" ] 2>/dev/null; } || P="$F fetch";; esac
  case "$P" in [0-9]*" "[a-z]*) ;; *) P="- -";; esac
  printf '@@head %s %s %s\n' "$C" "$B" "$P"
  head -c 65536 "$g/config" | grep -iE '^[[:space:]]*(\[|url[[:space:]]*=)' | sed -e 's#://[^@]*@#://#' -e '/^[[:space:]]*\[[Rr][Ee][Mm][Oo][Tt][Ee] /!s/^[[:space:]]*\[.*/[x]/' | head -n 40
done; [ "$rc" -gt 1 ] || echo @@gitend; }
`

// gitInnerScript runs on the host and inside each container: @@git opens the
// run, then the walk. On the host with SUDO=1 and a sudo that needs no
// password, the whole walk runs in one privileged shell — two entries in the
// auth log per scan, not two per checkout.
var gitInnerScript = `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
echo @@git
if [ -n "$SUDO" ] && sudo -n true 2>/dev/null; then
  printf %s ` + base64.StdEncoding.EncodeToString([]byte(gitWalkScript)) + ` | base64 -d | sudo -n sh
else
` + gitWalkScript + `fi
`

// gitProbeScript is the whole remote script: host, then every running
// container (up to maxGitContainers, started within gitCtrBudget). One SSH session.
func gitProbeScript(asRoot bool) string {
	return hostAndContainersScript(gitInnerScript, maxGitContainers, gitOutputCap, gitCtrTimeout, gitCtrBudget, asRoot)
}

// collectGitRemotes runs the probe — as root with the stored password when
// there is one (other users' homes are 0700), else unprivileged with sudo -n.
// nil: the probe didn't run. complete=false: some part stopped early.
func collectGitRemotes(client *ssh.Client, sudoPassword string) (remotes []GitRemote, complete bool) {
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, gitProbeScript(true))
		if remotes, complete = parseGitProbe(out); remotes != nil {
			return remotes, complete
		}
		log.Printf("[sshscan] git remotes as root failed, falling back to unprivileged: %v", err)
	}
	raw, _ := runCmdWithDeadline(client, encodedScript(gitProbeScript(false), "sh"), gitProbeTimeout)
	return fallbackGitResult(raw, sudoPassword != "")
}

// fallbackGitResult parses the unprivileged run. After a failed root attempt
// (rootFailed) it can't have entered 0700 homes or /root, so its list is
// never complete: it adds links, it never prunes them.
func fallbackGitResult(raw string, rootFailed bool) ([]GitRemote, bool) {
	remotes, complete := parseGitProbe(raw)
	return remotes, complete && !rootFailed
}

// parseGitProbe reads the host part and every container frame. It returns nil
// when the host part never started (@@git missing). A part that started but
// has no @@gitend makes the result incomplete; a container with no @@git at
// all (no shell in the image) just contributes nothing.
func parseGitProbe(raw string) ([]GitRemote, bool) {
	host, frames := splitCtrFrames(raw)
	remotes, started, complete := parseGitPart(host, "", "")
	if !started {
		return nil, false
	}
	for _, f := range frames {
		rs, started, done := parseGitPart(f.Body, f.Name, f.Image)
		switch {
		case started && !done:
			complete = false
		case !started && f.Exit != 126 && f.Exit != 127:
			// Not searched at all — paused, restarting, killed — and not
			// because the image has no shell: this container's links stay.
			complete = false
		}
		remotes = append(remotes, rs...)
	}
	return remotes, complete
}

func parseGitPart(text, container, image string) (out []GitRemote, started, complete bool) {
	out = []GitRemote{}
	seen := map[GitRemote]bool{}
	path, remote, head := "", "", GitRemote{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "@@git", line == "@@skipped": // skipped: past the container budget
			started = true
		case line == "@@gitend":
			complete = true
		case strings.HasPrefix(line, "@@repo "):
			path, remote, head = strings.TrimPrefix(line, "@@repo "), "", GitRemote{}
			if !validGitPath(path) {
				path = ""
			}
		case strings.HasPrefix(line, "@@head "):
			head = parseGitHead(line)
		case strings.HasPrefix(line, "["):
			remote = ""
			if len(line) > 9 && strings.EqualFold(line[:9], `[remote "`) { // git section names are case-insensitive
				remote, _, _ = strings.Cut(line[9:], `"`)
			}
		case path != "" && remote != "":
			key, val, ok := strings.Cut(line, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "url") {
				continue
			}
			url := stripSCPPassword(coolify.SanitizeRepoURL(strings.Trim(strings.TrimSpace(val), `"`)))
			if url == "" || len(url) > maxGitURLLen || strings.ContainsFunc(url, unicode.IsControl) {
				continue
			}
			r := GitRemote{Container: container, ContainerImage: image, Path: path, Name: remote, URL: url,
				Commit: head.Commit, Branch: head.Branch, PulledAt: head.PulledAt, PullKind: head.PullKind}
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out, started, complete
}

var (
	gitCommitRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	gitBranchRe = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,255}$`)
)

// parseGitHead reads "@@head <commit> <branch> <unix> <kind>" into the
// head fields of a GitRemote. The VM wrote it: each field is kept only if
// valid ("-" = unknown); a time past 2100 is forged.
func parseGitHead(line string) (h GitRemote) {
	f := strings.Fields(strings.TrimPrefix(line, "@@head "))
	if len(f) != 4 {
		return h
	}
	if gitCommitRe.MatchString(f[0]) {
		h.Commit = f[0]
	}
	if f[1] != "-" && gitBranchRe.MatchString(f[1]) {
		h.Branch = f[1]
	}
	if t, err := strconv.ParseInt(f[2], 10, 64); err == nil && t > 0 && t < 4102444800 {
		switch f[3] {
		case "clone", "pull", "fetch":
			h.PulledAt, h.PullKind = t, f[3]
		}
	}
	return h
}

// stripSCPPassword cuts a password from an scp-style remote
// (user:pass@host:path → user@host:path); SanitizeRepoURL only handles "://".
func stripSCPPassword(u string) string {
	at := strings.Index(u, "@")
	if strings.Contains(u, "://") || at < 0 {
		return u
	}
	if c := strings.Index(u[:at], ":"); c >= 0 {
		return u[:c] + u[at:]
	}
	return u
}

// validGitPath: an absolute path of printable characters, bounded.
func validGitPath(p string) bool {
	return strings.HasPrefix(p, "/") && len(p) <= maxGitPathLen && !strings.ContainsFunc(p, unicode.IsControl)
}
