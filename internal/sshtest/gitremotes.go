package sshtest

import (
	"log"
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
}

const (
	maxGitContainers = 30
	gitOutputCap     = 131072
	gitCtrTimeout    = 8
	gitProbeTimeout  = 2 * time.Minute
	maxGitPathLen    = 512
	maxGitURLLen     = 1024
)

// gitInnerScript runs on the host (SUDO=1: read as sudo -n when allowed) and
// inside each container. It walks "/" and every other local filesystem
// (-xdev each, depth 6, pseudo/docker/cache trees and node_modules pruned)
// for .git directories, up to 100. Per checkout: "@@repo <worktree>", then
// only its config's section headers and url lines — headers other than
// [remote "…"] blanked to [x], user:password@ cut. Nothing else of the
// config (extraHeader, credential helpers) leaves the machine.
// @@git opens the run; @@gitend closes it only when find finished in time
// and the cap wasn't hit, so an incomplete list never deletes links.
// GITPROBE_ROOTS overrides the start points (tests).
const gitInnerScript = `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
echo @@git
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 30"
P=""; [ -n "$SUDO" ] && sudo -n true 2>/dev/null && P="sudo -n"
R="$GITPROBE_ROOTS"
[ -n "$R" ] || R="/ $(awk '$3 ~ /^(ext[234]|xfs|btrfs|zfs|f2fs)$/ && $2 != "/" {print $2}' /proc/mounts 2>/dev/null | sort -u | tr '\n' ' ')"
L=$($P $T find $R -xdev -maxdepth 6 \( -path /proc -o -path /sys -o -path /dev -o -path /run -o -path /var/lib/docker -o -path /var/lib/containerd -o -path /snap -o -path /var/cache -o -path /usr/share -o -path /usr/lib -o -name node_modules -o -name .cache \) -prune -o -type d -name .git -print -prune 2>/dev/null; echo "@@rc $?")
printf '%s\n' "$L" | head -n 101 | { n=0; while IFS= read -r g; do
  case "$g" in "@@rc 124") exit 0;; "@@rc "*) echo @@gitend; exit 0;; esac
  n=$((n+1)); [ "$n" -gt 100 ] && exit 0
  $P test -f "$g/config" || continue
  printf '@@repo %s\n' "${g%/.git}"
  $P head -c 65536 "$g/config" | grep -E '^[[:space:]]*(\[|url[[:space:]]*=)' | sed -e 's#://[^@]*@#://#' -e '/^[[:space:]]*\[remote /!s/^[[:space:]]*\[.*/[x]/' | head -n 40
done; }
`

// gitProbeScript is the whole remote script: host, then every running
// container (up to maxGitContainers). One SSH session.
func gitProbeScript(asRoot bool) string {
	return hostAndContainersScript(gitInnerScript, maxGitContainers, gitOutputCap, gitCtrTimeout, asRoot)
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
	return parseGitProbe(raw)
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
		if started && !done {
			complete = false
		}
		remotes = append(remotes, rs...)
	}
	return remotes, complete
}

func parseGitPart(text, container, image string) (out []GitRemote, started, complete bool) {
	out = []GitRemote{}
	seen := map[GitRemote]bool{}
	path, remote := "", ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "@@git":
			started = true
		case line == "@@gitend":
			complete = true
		case strings.HasPrefix(line, "@@repo "):
			path, remote = strings.TrimPrefix(line, "@@repo "), ""
			if !validGitPath(path) {
				path = ""
			}
		case strings.HasPrefix(line, "["):
			remote = ""
			if name, ok := strings.CutPrefix(line, `[remote "`); ok {
				remote, _, _ = strings.Cut(name, `"`)
			}
		case path != "" && remote != "":
			key, val, ok := strings.Cut(line, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "url") {
				continue
			}
			url := coolify.SanitizeRepoURL(strings.Trim(strings.TrimSpace(val), `"`))
			if url == "" || len(url) > maxGitURLLen || strings.ContainsFunc(url, unicode.IsControl) {
				continue
			}
			r := GitRemote{Container: container, ContainerImage: image, Path: path, Name: remote, URL: url}
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out, started, complete
}

// validGitPath: an absolute path of printable characters, bounded.
func validGitPath(p string) bool {
	return strings.HasPrefix(p, "/") && len(p) <= maxGitPathLen && !strings.ContainsFunc(p, unicode.IsControl)
}
