package sshtest

// osRepairScript finishes what a failed update left half done, as root (kind
// repair, one host). In order:
//  1. dpkg --configure -a, keeping the local config files; on failure it
//     diagnoses (osScriptCommon), says @@error and stops;
//  2. apt-get -f install, only when apt-get check fails (same on failure);
//  3. @@dist_file <path> <n>: each *.dpkg-dist / *.ucf-dist under /etc and
//     how many lines differ from the file in use (or all its lines when there
//     is none), never the contents: a config diff can hold secrets. Applying
//     one stays manual;
//  4. @@unit_restart <unit> ok|failed: each failed unit restarted, judged by
//     systemctl is-active a moment later, not by the restart's exit code.
//     user@* units are left for the reboot;
//  5. @@reboot yes when one is pending, or the running kernel is installed
//     under /boot but isn't the newest there.
//
// Steps 1–3 need dpkg and apt; 4–5 run on any host. BRIDGE_ROOT prefixes the
// files it reads (tests only).
const osRepairScript = osScriptCommon + `export DEBIAN_FRONTEND=noninteractive UCF_FORCE_CONFFOLD=1
if command -v dpkg >/dev/null 2>&1 && command -v apt-get >/dev/null 2>&1; then
  say pm apt
  dpkg --force-confdef --force-confold --configure -a || fail dpkg --configure -a failed
  if ! apt-get check >/dev/null 2>&1; then
    apt-get -y -q -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold -f install || fail apt-get -f install failed
  fi
  find "$R/etc" -xdev \( -name '*.dpkg-dist' -o -name '*.ucf-dist' \) -type f 2>/dev/null | sort | head -n 100 | while IFS= read -r f; do
    o=${f%.*}; [ -f "$o" ] || o=/dev/null
    say dist_file "${f#"$R"} $(diff "$o" "$f" 2>/dev/null | grep -c '^[<>]')"
  done
fi
if command -v systemctl >/dev/null 2>&1; then
  UNITS=$(systemctl --failed --plain --no-legend --no-pager 2>/dev/null | awk '{u = $1; if (u !~ /^[A-Za-z0-9]/) u = $2; if (u ~ /^[A-Za-z0-9][A-Za-z0-9@._:-]*$/ && u !~ /^user@/) print u}' | head -n 50)
  for u in $UNITS; do systemctl reset-failed "$u" >/dev/null 2>&1; timeout 300 systemctl restart "$u" >/dev/null 2>&1; done
  [ -n "$UNITS" ] && sleep 5
  for u in $UNITS; do
    if [ "$(systemctl is-active "$u" 2>/dev/null)" = active ]; then say unit_restart "$u ok"; else say unit_restart "$u failed"; fi
  done
fi
K=$(uname -r 2>/dev/null)
NEWEST=$(for k in "$R"/boot/vmlinuz-*; do [ -f "$k" ] && echo "${k##*/vmlinuz-}"; done | sort -V | tail -n 1)
if [ -f "$R/var/run/reboot-required" ]; then say reboot yes
elif [ -n "$K" ] && [ -f "$R/boot/vmlinuz-$K" ] && [ "$NEWEST" != "$K" ]; then say reboot yes
fi
exit 0
`

// UnitRestart is a failed unit a repair restarted, and whether it is active now.
type UnitRestart struct {
	Unit string `json:"unit"`
	OK   bool   `json:"ok"`
}

// DistFile is a config file a package left for review (*.dpkg-dist,
// *.ucf-dist) and how many lines differ from the one in use.
type DistFile struct {
	Path    string `json:"path"`
	Changed int    `json:"changed"`
}
