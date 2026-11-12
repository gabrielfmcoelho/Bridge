package sshtest

import (
	"fmt"
	"strings"
)

// Restores (kind "restore"): a kept backup comes back from MinIO onto its
// host, detached like an update. The host downloads it itself through a
// presigned link fed to curl on stdin (never on a command line, where any
// local user would see it in /proc) and pipes it into tar. Two modes:
//
//	staging: extract into /var/lib/bridge-restore/run-<id> (root-only),
//	         touching nothing else; paths may narrow it.
//	paths:   extract the given paths over / (the runner backs those paths up
//	         to MinIO first: that copy is the undo).
//
// Markers: @@restored_to <dir>, @@error <reason>.
const (
	RestoreStaging = "staging"
	RestorePaths   = "paths"
)

// restoreDir is where staging restores land.
const restoreDir = "/var/lib/bridge-restore"

const osRestoreScript = `R="${BRIDGE_ROOT:-}"
say() { echo "@@$*"; }
MODE=%[1]s
case "$MODE" in
staging) T="$R%[2]s/run-%[3]d"; mkdir -p "$T" && chmod 700 "$R%[2]s" "$T" || { say error could not create $T; exit 1; } ;;
paths) T="$R/" ;;
*) say error unknown mode; exit 1 ;;
esac
case %[4]s in *.zst) Z="zstd -dc" ;; *) Z="gzip -dc" ;; esac
command -v ${Z%%%% *} >/dev/null 2>&1 || { say error "${Z%%%% *} is not installed"; exit 1; }
if command -v curl >/dev/null 2>&1; then
  fetch() { printf 'url = "%%s"\n' "$U" | curl -fsS -K -; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { printf '%%s\n' "$U" | wget -qO- -i -; }
else say error "neither curl nor wget is installed"; exit 1; fi
U=$(cat <<'BRIDGE_URL'
%[5]s
BRIDGE_URL
)
set --
if tar --version 2>/dev/null | grep -q GNU; then set -- --numeric-owner --acls --xattrs --same-permissions; fi
F="$R%[6]s.fetch"
mkdir -p "$R%[8]s" && rm -f "$F"
{ fetch; echo $? > "$F"; } | $Z | tar -xf - -C "$T" "$@" %[7]s
TE=$?
FE=$(cat "$F" 2>/dev/null); rm -f "$F"
[ "${FE:-1}" = 0 ] || { say error "download failed (exit ${FE:-?}): the link may have expired or MinIO is unreachable from the host"; exit 1; }
[ "$TE" = 0 ] || { say error "tar exit $TE (a path not in the backup?)"; exit 1; }
say restored_to "$T"
exit 0
`

// RestoreMembers turns absolute paths into the archive's member names (tar
// stored them without the leading /). Rejects relative paths, "..", and the
// whole root (a full in-place restore isn't supported).
func RestoreMembers(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !strings.HasPrefix(p, "/") {
			return nil, fmt.Errorf("%q is not an absolute path", p)
		}
		m := strings.Trim(p, "/")
		if m == "" {
			return nil, fmt.Errorf("restoring all of / in place is not supported: pick paths, or use the review folder")
		}
		for _, part := range strings.Split(m, "/") {
			if part == ".." || part == "." {
				return nil, fmt.Errorf("%q must not contain . or ..", p)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// RestoreScript is the root script restoring object key (fetched from url)
// as run runID, in mode, limited to paths (members, see RestoreMembers).
func RestoreScript(runID int64, mode, key, url string, members []string) (string, error) {
	if mode != RestoreStaging && mode != RestorePaths {
		return "", fmt.Errorf("unknown restore mode %q", mode)
	}
	if mode == RestorePaths && len(members) == 0 {
		return "", fmt.Errorf("a restore in place needs paths")
	}
	if strings.ContainsAny(url, "\n") {
		return "", fmt.Errorf("bad download link")
	}
	q := make([]string, len(members))
	for i, m := range members {
		q[i] = shQuote(m)
	}
	return fmt.Sprintf(osRestoreScript, mode, restoreDir, runID, shQuote(key), url, osUpdateFile(runID), strings.Join(q, " "), osUpdateDir), nil
}
