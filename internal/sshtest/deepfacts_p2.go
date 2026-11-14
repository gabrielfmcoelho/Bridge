package sshtest

import (
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// P2 deep facts (the "who/what/where" enrichment): control panels (A6), per-
// docroot code fingerprint (A7), owner clues (A8) and firewall reality (A10).
// Like A2/A3/A4 they run only in a background scan (CaptureJob), need root
// (the stored SSH password), and emit ONLY allowlisted tokens — names, counts,
// dates, classes. A client IP, key material, filename, GECOS value of a real
// person aside, query string or log parameter never leaves the VM: IPs are
// classified to private/public on the box and only the class/count crosses SSH.
//
// Deferred to a Phase 5b (see the PR): A11 mail/DNS role, site services from
// docroots (C.3), composer/npm dependency ingestion, and the free-form Webmin
// action log (its column format can't be verified offline and carries params).

// ControlPanels (A6) is the Virtualmin/Webmin picture: which panels are
// installed, their virtual-server domains (with the disable event), panel
// users, and the failed-login tally (brute-force signal), counts only.
type ControlPanels struct {
	Installed    []string           `json:"installed,omitempty"` // virtualmin | webmin
	Domains      []PanelDomain      `json:"domains,omitempty"`
	Users        []string           `json:"users,omitempty"`
	FailedLogins *PanelFailedLogins `json:"failed_logins,omitempty"`
}

// PanelDomain is one Virtualmin virtual server. DisabledReason is the panel's
// own free-text note; the actor (which Webmin user disabled it) lives in the
// action log, deferred to 5b.
type PanelDomain struct {
	Name           string `json:"name"`
	Owner          string `json:"owner,omitempty"`
	Created        int64  `json:"created,omitempty"`
	Disabled       bool   `json:"disabled,omitempty"`
	DisabledTime   int64  `json:"disabled_time,omitempty"`
	DisabledReason string `json:"disabled_reason,omitempty"`
	Contact        string `json:"contact,omitempty"`
	LetsEncrypt    bool   `json:"letsencrypt,omitempty"`
}

// PanelFailedLogins counts failed panel logins by source class (never the IP).
type PanelFailedLogins struct {
	Private int `json:"private,omitempty"`
	Public  int `json:"public,omitempty"`
}

// CodeFingerprint (A7) is one web docroot's apps, newest code mtime and upload
// directories — the "what runs here / how much data is at rest" signal.
type CodeFingerprint struct {
	Docroot     string      `json:"docroot"`
	Apps        []CodeApp   `json:"apps,omitempty"`
	NewestMtime int64       `json:"newest_mtime,omitempty"` // newest *.php/.phtml/.inc mtime under the docroot
	Uploads     []UploadDir `json:"uploads,omitempty"`
}

// CodeApp is one framework found at the docroot (Path "") or an immediate
// subdirectory (the real host-35 case: /sasc, /SAF-SASC under one docroot).
type CodeApp struct {
	Path      string `json:"path,omitempty"` // relative to the docroot, "" = the root itself
	Framework string `json:"framework"`      // wordpress|laravel|codeigniter3|codeigniter4|joomla|drupal|moodle|scriptcase
	Version   string `json:"version,omitempty"`
}

// UploadDir is an upload/attachment directory under a docroot: how much data,
// how fresh, how many files (approximate — capped file walk).
type UploadDir struct {
	Dir       string `json:"dir"` // relative to the docroot
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Newest    int64  `json:"newest,omitempty"`
	Files     int    `json:"files,omitempty"`
}

// OwnerClues (A8) are leads to who owns/touches a VM nobody claims — each with
// its source so the UI can label it. Privacy: last-login source hosts, xferlog
// remote hosts and filenames, and authorized_keys key material are all dropped
// on the VM; only usernames, dates, GECOS text and key comments cross SSH.
type OwnerClues struct {
	LastLogins []OwnerLast    `json:"last_logins,omitempty"` // most recent login per user, with year
	Gecos      []OwnerGecos   `json:"gecos,omitempty"`       // /etc/passwd full-name field
	Sudoers    []string       `json:"sudoers,omitempty"`     // sudo/wheel/admin group members
	AuthKeys   []OwnerAuthKey `json:"auth_keys,omitempty"`   // authorized_keys comment fields
	FTP        []OwnerFTP     `json:"ftp,omitempty"`         // FTP/SFTP uploaders per month
}

type OwnerLast struct {
	User string `json:"user"`
	When int64  `json:"when"` // unix (day) of the most recent login
}
type OwnerGecos struct {
	User  string `json:"user"`
	Gecos string `json:"gecos"`
}
type OwnerAuthKey struct {
	User    string `json:"user"`
	Comment string `json:"comment"`
}
type OwnerFTP struct {
	User   string `json:"user"`
	Month  string `json:"month"` // YYYY-MM
	Upload int    `json:"upload"`
}

// Firewall (A10) is the real, often-invisible firewall posture: ufw AND
// firewalld/nft (host 35's firewalld doesn't show up today), iptables rule
// count, fail2ban jails (counts), and which admin ports are exposed, with how
// wide (all interfaces / one IP / loopback) — never the bind address itself.
type Firewall struct {
	UFW        *FirewallTool `json:"ufw,omitempty"`
	Firewalld  *FirewallTool `json:"firewalld,omitempty"`
	Nft        *FirewallTool `json:"nft,omitempty"`
	Iptables   int           `json:"iptables_rules,omitempty"`
	Fail2ban   []F2bJail     `json:"fail2ban,omitempty"`
	AdminPorts []AdminPort   `json:"admin_ports,omitempty"`
}

// FirewallTool is one firewall layer: active and how many rules.
type FirewallTool struct {
	Active bool `json:"active"`
	Rules  int  `json:"rules,omitempty"`
}
type F2bJail struct {
	Jail   string `json:"jail"`
	Banned int    `json:"banned,omitempty"`
}

// AdminPort is a sensitive port found listening, with its exposure breadth.
type AdminPort struct {
	Port     int    `json:"port"`
	Exposure string `json:"exposure"` // all | one-ip | loopback
}

const (
	maxPanelDomains = 60
	maxPanelUsers   = 60
	maxP2Docroots   = 40
	maxAppsPerDoc   = 12
	maxUploadDirs   = 12
	maxOwnerRows    = 120
	maxF2bJails     = 20
	p2FileWalkCap   = 20000 // files stat'd for newest-mtime / counted per dir
)

// p2AdminPorts are the sensitive service ports A10 reports exposure for.
var p2AdminPorts = []int{10000, 20000, 2083, 8443, 3306, 33060, 5432, 21}

// p2Script is one root-capable session emitting A6/A7/A8/A10 markers. docroots
// (from the scan's web servers) are the roots A7 fingerprints. BRIDGE_ROOT
// prefixes every system read so the fake-root tests drive the real script.
// Not root: SUDO=1 makes each read retry through sudo -n; without root most
// sections simply emit nothing (the cards then show "didn't run"/empty).
func p2Script(asRoot bool, docroots []string) string {
	head := "SUDO=1\n"
	if asRoot {
		head = ""
	}
	var sb strings.Builder
	sb.WriteString(head)
	sb.WriteString(`export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
R="${BRIDGE_ROOT:-}"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 20"
S=""; if [ -n "$SUDO" ] && sudo -n true 2>/dev/null; then S="sudo -n"; fi
b64(){ printf %s "$1" | base64 2>/dev/null | tr -d '\n'; }
# ── A6 control panels ──
[ -d "$R/etc/webmin/virtual-server" ] && echo "@@panel virtualmin"
[ -d "$R/etc/webmin" ] && echo "@@panel webmin"
for f in "$R"/etc/webmin/virtual-server/domains/*; do
  [ -f "$f" ] || continue
  case "${f##*/}" in *.*) continue;; esac
  body=$($S cat "$f" 2>/dev/null)
  get(){ printf '%s\n' "$body" | sed -n "s/^$1=//p" | head -n1; }
  dom=$(get dom); [ -n "$dom" ] || continue
  owner=$(get user); created=$(get created)
  dis=$(get disabled); distime=$(get disabled_time)
  reason=$(get disabled_reason); [ -n "$reason" ] || reason=$(get disabled_why)
  contact=$(get emailto); [ -n "$contact" ] || contact=$(get email)
  le=0; [ -n "$(get letsencrypt_last)" ] && le=1
  d=0; [ -n "$dis" ] && d=1
  printf '@@vdom %s %s %s %s %s %s %s %s\n' "$(b64 "$dom")" "$(b64 "$owner")" "${created:-0}" "$d" "${distime:-0}" "$(b64 "$reason")" "$(b64 "$contact")" "$le"
done
if [ -f "$R/etc/webmin/miniserv.users" ]; then
  $S cut -d: -f1 "$R/etc/webmin/miniserv.users" 2>/dev/null | while IFS= read -r u; do
    [ -n "$u" ] && printf '@@puser %s\n' "$(b64 "$u")"
  done
fi
# Failed panel logins by source class. All aggregation is a grep -c count; the
# source IP is matched only to classify public vs private and is never emitted.
FPAT='Invalid login|Non-existent login|Failed login|password mismatch'
for lg in "$R/var/webmin/miniserv.log" "$R/var/log/webmin/miniserv.log"; do
  [ -f "$lg" ] || continue
  tot=$($S grep -hcE "$FPAT" "$lg" 2>/dev/null); tot=${tot:-0}
  pub=$($S grep -hE "$FPAT" "$lg" 2>/dev/null | tail -n 20000 | grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}' | grep -vcE '^(10\.|192\.168\.|127\.|169\.254\.|172\.(1[6-9]|2[0-9]|3[01])\.)'); pub=${pub:-0}
  [ "$tot" -gt 0 ] 2>/dev/null && printf '@@pfail %d %d\n' "$tot" "$pub"
  break
done
`)
	// ── A7 code fingerprint per docroot ──
	sb.WriteString(`emitfw(){ printf '@@fw %s %s %s\n' "$1" "$(b64 "$2")" "$(b64 "$3")"; }
qv(){ sed -n "s/.*['\"]\\([0-9][0-9A-Za-z._-]*\\)['\"].*/\\1/p" 2>/dev/null | head -n1; }
detect(){ dd=$1; rel=$2
  if [ -f "$dd/wp-includes/version.php" ]; then
    emitfw wordpress "$(grep -m1 'wp_version =' "$dd/wp-includes/version.php" 2>/dev/null | qv)" "$rel"; return
  fi
  if [ -f "$dd/system/core/CodeIgniter.php" ]; then
    emitfw codeigniter3 "$(grep -m1 CI_VERSION "$dd/system/core/CodeIgniter.php" 2>/dev/null | qv)" "$rel"; return
  fi
  if [ -f "$dd/system/CodeIgniter.php" ]; then
    emitfw codeigniter4 "$(grep -m1 CI_VERSION "$dd/system/CodeIgniter.php" 2>/dev/null | qv)" "$rel"; return
  fi
  if [ -f "$dd/artisan" ] && [ -d "$dd/app" ]; then
    emitfw laravel "$(grep -m1 'laravel/framework' "$dd/composer.lock" 2>/dev/null | sed -n 's/.*"version"[^"]*"\\([^"]*\\)".*/\\1/p' | head -n1)" "$rel"; return
  fi
  if [ -f "$dd/administrator/manifests/files/joomla.xml" ]; then
    emitfw joomla "$(sed -n 's:.*<version>\\(.*\\)</version>.*:\\1:p' "$dd/administrator/manifests/files/joomla.xml" 2>/dev/null | head -n1)" "$rel"; return
  fi
  if [ -f "$dd/core/lib/Drupal.php" ]; then
    emitfw drupal "$(grep -m1 "const VERSION" "$dd/core/lib/Drupal.php" 2>/dev/null | qv)" "$rel"; return
  fi
  if [ -f "$dd/includes/bootstrap.inc" ] && grep -q "define('VERSION'" "$dd/includes/bootstrap.inc" 2>/dev/null; then
    emitfw drupal "$(grep -m1 "define('VERSION'" "$dd/includes/bootstrap.inc" 2>/dev/null | qv)" "$rel"; return
  fi
  if [ -f "$dd/version.php" ] && grep -q 'release' "$dd/version.php" 2>/dev/null; then
    emitfw moodle "$(grep -m1 'release' "$dd/version.php" 2>/dev/null | qv)" "$rel"; return
  fi
  if [ -d "$dd/_lib" ] && { [ -d "$dd/prod" ] || [ -f "$dd/conexao.ini" ]; }; then
    emitfw scriptcase "" "$rel"; return
  fi
}
`)
	for _, d := range docroots {
		q := shQuote(d)
		sb.WriteString(`dr="$R"` + q + `
if [ -d "$dr" ]; then
  echo "@@doc $(b64 ` + q + `)"
  detect "$dr" ""
  n=0
  for sub in "$dr"/*/; do
    [ -d "$sub" ] || continue
    n=$((n+1)); [ "$n" -gt ` + strconv.Itoa(maxAppsPerDoc) + ` ] && break
    rel=${sub%/}; rel=${rel##*/}
    detect "${sub%/}" "$rel"
  done
  nm=$($T find "$dr" -type f \( -name '*.php' -o -name '*.phtml' -o -name '*.inc' \) 2>/dev/null | head -n ` + strconv.Itoa(p2FileWalkCap) + ` | xargs stat -c %Y 2>/dev/null | sort -rn | head -n1)
  [ -n "$nm" ] && echo "@@mtime $nm"
  $T find "$dr" -maxdepth 4 -type d \( -iname upload -o -iname uploads -o -iname arquivos -o -iname files -o -iname media -o -iname attachments -o -iname storage \) 2>/dev/null | head -n ` + strconv.Itoa(maxUploadDirs) + ` | while IFS= read -r ud; do
    rel=${ud#"$dr"/}; [ "$rel" = "$ud" ] && rel=${ud##*/}
    kib=$($T du -sk "$ud" 2>/dev/null | awk '{print $1; exit}')
    cnt=$($T find "$ud" -type f 2>/dev/null | head -n 200000 | grep -c .)
    new=$($T find "$ud" -type f 2>/dev/null | head -n ` + strconv.Itoa(p2FileWalkCap) + ` | xargs stat -c %Y 2>/dev/null | sort -rn | head -n1)
    printf '@@upload %s %s %s %s\n' "$(b64 "$rel")" "${kib:-0}" "${new:-0}" "${cnt:-0}"
  done
fi
`)
	}
	// ── A8 owner clues ──
	sb.WriteString(`if command -v last >/dev/null 2>&1; then
  $S last -F -w 2>/dev/null | awk '
    BEGIN{m["Jan"]=1;m["Feb"]=2;m["Mar"]=3;m["Apr"]=4;m["May"]=5;m["Jun"]=6;m["Jul"]=7;m["Aug"]=8;m["Sep"]=9;m["Oct"]=10;m["Nov"]=11;m["Dec"]=12}
    $1==""||$1=="reboot"||$1=="wtmp"||$1=="shutdown"{next}
    { u=$1; f=0
      for(i=2;i<=NF;i++){ if($i ~ /^(Mon|Tue|Wed|Thu|Fri|Sat|Sun)$/){ mo=m[$(i+1)]; dd=$(i+2)+0; yy=$(i+4)+0; f=1; break } }
      if(!f||mo==""||yy==0||dd==0)next
      k=(yy*10000)+(mo*100)+dd; if(k>mx[u])mx[u]=k }
    END{for(u in mx)printf "@@last %s %d\n",u,mx[u]}'
fi
if [ -f "$R/etc/passwd" ]; then
  # Human accounts only (uid >= 1000) with a non-empty full-name field — the
  # owner signal. System accounts' descriptions (www-data, backup) are noise.
  while IFS=: read -r u x1 uid x2 gecos x3; do
    g=${gecos%%,*}; [ -n "$g" ] || continue
    case "$uid" in ''|*[!0-9]*) continue;; esac
    [ "$uid" -ge 1000 ] && [ "$uid" -ne 65534 ] || continue
    printf '@@gecos %s %s\n' "$(b64 "$u")" "$(b64 "$g")"
  done < "$R/etc/passwd"
fi
for grp in sudo wheel admin; do
  line=$(grep -m1 "^$grp:" "$R/etc/group" 2>/dev/null); [ -n "$line" ] || continue
  mem=${line##*:}; oIFS=$IFS; IFS=,
  for u in $mem; do [ -n "$u" ] && printf '@@sudoer %s\n' "$(b64 "$u")"; done
  IFS=$oIFS
done
for h in "$R/root" "$R"/home/*; do
  ak="$h/.ssh/authorized_keys"; [ -f "$ak" ] || { [ -f "$h/.ssh/authorized_keys2" ] && ak="$h/.ssh/authorized_keys2" || continue; }
  user=${h##*/}
  $S awk '{ for(i=1;i<=NF;i++){ if($i ~ /^(ssh-|ecdsa-|sk-)/){ c=""; for(j=i+2;j<=NF;j++)c=c (c==""?"":" ") $j; if(c!="")print c; break } } }' "$ak" 2>/dev/null | while IFS= read -r c; do
    printf '@@authkey %s %s\n' "$(b64 "$user")" "$(b64 "$c")"
  done
done
for xf in "$R"/var/log/proftpd/xferlog "$R"/var/log/xferlog "$R"/var/log/vsftpd.log; do
  [ -f "$xf" ] || continue
  $S tail -c 4194304 "$xf" 2>/dev/null | awk '
    BEGIN{m["Jan"]=1;m["Feb"]=2;m["Mar"]=3;m["Apr"]=4;m["May"]=5;m["Jun"]=6;m["Jul"]=7;m["Aug"]=8;m["Sep"]=9;m["Oct"]=10;m["Nov"]=11;m["Dec"]=12}
    NF>=14 && $12=="i" && $14!="" && $14!="-" { mo=m[$2]; yy=$5+0; if(mo==""||yy==0)next
      k=$14 SUBSEP (yy*100+mo); c[k]++; us[k]=$14; ym[k]=(yy*100+mo) }
    END{for(k in c)printf "@@ftp %s %d %d\n",us[k],ym[k],c[k]}'
  break
done
`)
	// ── A10 firewall reality ──
	sb.WriteString(`if command -v ufw >/dev/null 2>&1; then
  st=$($S $T ufw status 2>/dev/null)
  act=0; printf '%s\n' "$st" | grep -qi 'Status: active' && act=1
  r=$(printf '%s\n' "$st" | grep -cE 'ALLOW|DENY|REJECT')
  printf '@@ufw %d %d\n' "$act" "${r:-0}"
fi
if command -v firewall-cmd >/dev/null 2>&1; then
  act=0; [ "$($S $T firewall-cmd --state 2>/dev/null)" = running ] && act=1
  r=$($S $T nft list ruleset 2>/dev/null | grep -c dport)
  printf '@@firewalld %d %d\n' "$act" "${r:-0}"
elif command -v nft >/dev/null 2>&1; then
  rs=$($S $T nft list ruleset 2>/dev/null)
  if [ -n "$rs" ]; then
    tbl=$(printf '%s\n' "$rs" | grep -c '^table ')
    r=$(printf '%s\n' "$rs" | grep -cE 'dport|accept|drop|reject')
    printf '@@nft %d %d\n' "${tbl:-0}" "${r:-0}"
  fi
fi
if command -v iptables >/dev/null 2>&1; then
  r=$($S $T iptables -S 2>/dev/null | grep -c '^-A')
  [ "${r:-0}" -gt 0 ] 2>/dev/null && printf '@@iptables %d\n' "$r"
fi
if command -v fail2ban-client >/dev/null 2>&1; then
  $S $T fail2ban-client status 2>/dev/null | sed -n 's/.*Jail list:[[:space:]]*//p' | tr ',' ' ' | while IFS= read -r js; do
    for j in $js; do
      j=$(printf %s "$j" | tr -d ' '); [ -n "$j" ] || continue
      b=$($S $T fail2ban-client status "$j" 2>/dev/null | sed -n 's/.*Currently banned:[[:space:]]*\([0-9]*\).*/\1/p' | head -n1)
      printf '@@f2b %s %d\n' "$(b64 "$j")" "${b:-0}"
    done
  done
fi
{ if command -v ss >/dev/null 2>&1; then $S $T ss -ltnH 2>/dev/null; elif command -v netstat >/dev/null 2>&1; then $S $T netstat -ltn 2>/dev/null; fi; } | awk '
  BEGIN{split("10000 20000 2083 8443 3306 33060 5432 21",A," ");for(i in A)adm[A[i]+0]=1;rk["all"]=3;rk["one-ip"]=2;rk["loopback"]=1}
  { la=$4; n=split(la,P,":"); if(n<2)next; port=P[n]+0; if(!(port in adm))next
    addr=substr(la,1,length(la)-length(P[n])-1)
    if(addr=="0.0.0.0"||addr=="*"||addr=="::"||addr=="[::]"||addr=="")cls="all"
    else if(addr=="127.0.0.1"||addr=="::1"||addr=="[::1]"||addr ~ /^127\./)cls="loopback"
    else cls="one-ip"
    if(rk[cls]>rk[best[port]])best[port]=cls }
  END{for(p in best)printf "@@adminport %d %s\n",p,best[p]}'
echo "@@p2-done"
`)
	return sb.String()
}

// collectP2Facts runs the A6/A7/A8/A10 probe — root with the stored password
// (webmin configs, other users' authorized_keys, wtmp, firewall tools are
// root-only), else unprivileged. docroots are the scan's web docroots.
// Returns nil when nothing ran.
func collectP2Facts(client *ssh.Client, sudoPassword string, docroots []string) (*ControlPanels, *CodeFingerprintSet, *OwnerClues, *Firewall) {
	var raw string
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, p2Script(true, docroots))
		if strings.Contains(out, "@@p2-done") {
			raw = out
		} else {
			log.Printf("[sshscan] p2 facts as root failed, falling back to unprivileged: %v", err)
		}
	}
	if raw == "" {
		raw = runCmd(client, encodedScript(p2Script(false, docroots), "sh"))
	}
	if !strings.Contains(raw, "@@p2-done") {
		return nil, nil, nil, nil
	}
	return parseP2Facts(raw)
}

// CodeFingerprintSet wraps the per-docroot rows so parseP2Facts can return a
// single value the caller folds into DeepFacts.Code.
type CodeFingerprintSet struct {
	Docroots []CodeFingerprint
}

// webDocroots is the deduped set of host (non-container) web docroots from the
// scan — the roots A7 fingerprints. Container roots aren't on the host
// filesystem, so they're skipped. Absolute paths only, capped.
func webDocroots(servers []WebServer) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range servers {
		if s.Container != "" {
			continue
		}
		for _, site := range s.Sites {
			r := strings.TrimRight(site.Root, "/")
			if r == "" {
				r = site.Root // keep "/" itself
			}
			if !strings.HasPrefix(r, "/") || seen[r] || len(r) > maxTargetLen {
				continue
			}
			seen[r] = true
			out = append(out, r)
			if len(out) >= maxP2Docroots {
				return out
			}
		}
	}
	return out
}

var p2NameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9._-]{0,63}$`)
var p2FwRe = regexp.MustCompile(`^[a-z0-9]+$`)

// parseP2Facts turns the probe markers into the four P2 structs. Each is nil
// when its section emitted nothing.
func parseP2Facts(raw string) (*ControlPanels, *CodeFingerprintSet, *OwnerClues, *Firewall) {
	cp := &ControlPanels{}
	code := &CodeFingerprintSet{}
	owner := &OwnerClues{}
	fw := &Firewall{}
	var curDoc *CodeFingerprint
	flushDoc := func() {
		if curDoc != nil {
			if len(code.Docroots) < maxP2Docroots {
				code.Docroots = append(code.Docroots, *curDoc)
			}
			curDoc = nil
		}
	}
	seenInstalled := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		head, rest, _ := strings.Cut(line, " ")
		switch head {
		case "@@panel":
			if (rest == "virtualmin" || rest == "webmin") && !seenInstalled[rest] {
				seenInstalled[rest] = true
				cp.Installed = append(cp.Installed, rest)
			}
		case "@@vdom":
			if d := parsePanelDomain(rest); d != nil && len(cp.Domains) < maxPanelDomains {
				cp.Domains = append(cp.Domains, *d)
			}
		case "@@puser":
			if u := decodeToken(rest); printableText(u) && len(cp.Users) < maxPanelUsers {
				cp.Users = append(cp.Users, u)
			}
		case "@@pfail":
			if pr, pu, ok := parseTwoInts(rest); ok {
				cp.FailedLogins = &PanelFailedLogins{Private: pr - pu, Public: pu}
				if cp.FailedLogins.Private < 0 {
					cp.FailedLogins.Private = 0
				}
			}
		case "@@doc":
			flushDoc()
			if dr := decodeToken(rest); printableText(dr) {
				curDoc = &CodeFingerprint{Docroot: dr}
			}
		case "@@fw":
			if curDoc != nil {
				if a := parseCodeApp(rest); a != nil && len(curDoc.Apps) < maxAppsPerDoc {
					curDoc.Apps = append(curDoc.Apps, *a)
				}
			}
		case "@@mtime":
			if curDoc != nil {
				curDoc.NewestMtime = parseUnix(rest)
			}
		case "@@upload":
			if curDoc != nil {
				if u := parseUploadDir(rest); u != nil && len(curDoc.Uploads) < maxUploadDirs {
					curDoc.Uploads = append(curDoc.Uploads, *u)
				}
			}
		case "@@last":
			if u, when, ok := parseOwnerLast(rest); ok && len(owner.LastLogins) < maxOwnerRows {
				owner.LastLogins = append(owner.LastLogins, OwnerLast{User: u, When: when})
			}
		case "@@gecos":
			u, g, ok := parseTwoTokens(rest)
			if ok && p2NameRe.MatchString(u) && printableText(g) && len(owner.Gecos) < maxOwnerRows {
				owner.Gecos = append(owner.Gecos, OwnerGecos{User: u, Gecos: g})
			}
		case "@@sudoer":
			if u := decodeToken(rest); p2NameRe.MatchString(u) && len(owner.Sudoers) < maxOwnerRows && !contains(owner.Sudoers, u) {
				owner.Sudoers = append(owner.Sudoers, u)
			}
		case "@@authkey":
			u, c, ok := parseTwoTokens(rest)
			if ok && printableText(u) && printableText(c) && len(owner.AuthKeys) < maxOwnerRows {
				owner.AuthKeys = append(owner.AuthKeys, OwnerAuthKey{User: u, Comment: c})
			}
		case "@@ftp":
			if f := parseOwnerFTP(rest); f != nil && len(owner.FTP) < maxOwnerRows {
				owner.FTP = append(owner.FTP, *f)
			}
		case "@@ufw":
			fw.UFW = parseFirewallTool(rest)
		case "@@firewalld":
			fw.Firewalld = parseFirewallTool(rest)
		case "@@nft":
			fw.Nft = parseFirewallTool(rest)
		case "@@iptables":
			if n, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
				fw.Iptables = n
			}
		case "@@f2b":
			b64, nStr, ok := strings.Cut(rest, " ")
			if j := decodeToken(b64); ok && printableText(j) && len(fw.Fail2ban) < maxF2bJails {
				n, _ := strconv.Atoi(strings.TrimSpace(nStr))
				fw.Fail2ban = append(fw.Fail2ban, F2bJail{Jail: j, Banned: n})
			}
		case "@@adminport":
			if p := parseAdminPort(rest); p != nil && len(fw.AdminPorts) < len(p2AdminPorts) {
				fw.AdminPorts = append(fw.AdminPorts, *p)
			}
		}
	}
	flushDoc()
	finalizeP2(cp, code, owner, fw)
	cpNil, codeNil, ownerNil, fwNil := emptyCP(cp), len(code.Docroots) == 0, emptyOwner(owner), emptyFirewall(fw)
	if cpNil {
		cp = nil
	}
	if codeNil {
		code = nil
	}
	if ownerNil {
		owner = nil
	}
	if fwNil {
		fw = nil
	}
	return cp, code, owner, fw
}

func parsePanelDomain(s string) *PanelDomain {
	// Split (not Fields): fields are single-space separated and some (the
	// disable reason) may be an empty base64 token — Fields would collapse it.
	f := strings.Split(s, " ")
	if len(f) < 8 {
		return nil
	}
	name := decodeToken(f[0])
	if !printableText(name) {
		return nil
	}
	return &PanelDomain{
		Name:           name,
		Owner:          decodeToken(f[1]),
		Created:        parseUnix(f[2]),
		Disabled:       f[3] == "1",
		DisabledTime:   parseUnix(f[4]),
		DisabledReason: decodeToken(f[5]),
		Contact:        decodeToken(f[6]),
		LetsEncrypt:    f[7] == "1",
	}
}

func parseCodeApp(s string) *CodeApp {
	fw, rest, ok := strings.Cut(s, " ")
	if !ok || !p2FwRe.MatchString(fw) {
		return nil
	}
	verB64, pathB64, _ := strings.Cut(rest, " ")
	ver := decodeToken(verB64)
	if !printableText(ver) && ver != "" {
		ver = ""
	}
	path := decodeToken(pathB64)
	if !printableText(path) {
		path = ""
	}
	return &CodeApp{Path: path, Framework: fw, Version: ver}
}

func parseUploadDir(s string) *UploadDir {
	f := strings.Fields(s)
	if len(f) < 4 {
		return nil
	}
	dir := decodeToken(f[0])
	if !printableText(dir) {
		return nil
	}
	kib, _ := strconv.ParseInt(f[1], 10, 64)
	cnt, _ := strconv.Atoi(f[3])
	return &UploadDir{Dir: dir, SizeBytes: kib * 1024, Newest: parseUnix(f[2]), Files: cnt}
}

func parseOwnerLast(s string) (string, int64, bool) {
	u, ymd, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok || !p2NameRe.MatchString(u) {
		return "", 0, false
	}
	return u, ymdToUnix(ymd), ok
}

func parseOwnerFTP(s string) *OwnerFTP {
	f := strings.Fields(s)
	if len(f) < 3 {
		return nil
	}
	if !p2NameRe.MatchString(f[0]) || len(f[1]) != 6 {
		return nil
	}
	n, err := strconv.Atoi(f[2])
	if err != nil || n <= 0 {
		return nil
	}
	return &OwnerFTP{User: f[0], Month: f[1][:4] + "-" + f[1][4:], Upload: n}
}

func parseFirewallTool(s string) *FirewallTool {
	a, r, ok := parseTwoInts(s)
	if !ok {
		return nil
	}
	return &FirewallTool{Active: a == 1, Rules: r}
}

func parseAdminPort(s string) *AdminPort {
	portStr, cls, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil
	}
	switch cls {
	case "all", "one-ip", "loopback":
		return &AdminPort{Port: port, Exposure: cls}
	}
	return nil
}

// finalizeP2 sorts the rows so the UI reads top-signal first and the output is
// deterministic (tests, diffs).
func finalizeP2(cp *ControlPanels, code *CodeFingerprintSet, owner *OwnerClues, fw *Firewall) {
	sort.SliceStable(cp.Domains, func(i, j int) bool { return cp.Domains[i].Name < cp.Domains[j].Name })
	sort.Strings(cp.Users)
	sort.SliceStable(owner.LastLogins, func(i, j int) bool { return owner.LastLogins[i].When > owner.LastLogins[j].When })
	sort.SliceStable(owner.Gecos, func(i, j int) bool { return owner.Gecos[i].User < owner.Gecos[j].User })
	sort.Strings(owner.Sudoers)
	sort.SliceStable(owner.FTP, func(i, j int) bool {
		if owner.FTP[i].Month != owner.FTP[j].Month {
			return owner.FTP[i].Month > owner.FTP[j].Month
		}
		return owner.FTP[i].Upload > owner.FTP[j].Upload
	})
	sort.SliceStable(fw.Fail2ban, func(i, j int) bool { return fw.Fail2ban[i].Jail < fw.Fail2ban[j].Jail })
	sort.SliceStable(fw.AdminPorts, func(i, j int) bool { return fw.AdminPorts[i].Port < fw.AdminPorts[j].Port })
}

func emptyCP(cp *ControlPanels) bool {
	return len(cp.Installed) == 0 && len(cp.Domains) == 0 && len(cp.Users) == 0 && cp.FailedLogins == nil
}
func emptyOwner(o *OwnerClues) bool {
	return len(o.LastLogins) == 0 && len(o.Gecos) == 0 && len(o.Sudoers) == 0 && len(o.AuthKeys) == 0 && len(o.FTP) == 0
}
func emptyFirewall(f *Firewall) bool {
	return f.UFW == nil && f.Firewalld == nil && f.Nft == nil && f.Iptables == 0 && len(f.Fail2ban) == 0 && len(f.AdminPorts) == 0
}

// parseTwoTokens splits "<b64> <b64>" and decodes both.
func parseTwoTokens(s string) (string, string, bool) {
	a, b, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return "", "", false
	}
	return decodeToken(a), decodeToken(b), true
}

func parseTwoInts(s string) (int, int, bool) {
	f := strings.Fields(s)
	if len(f) < 2 {
		return 0, 0, false
	}
	a, e1 := strconv.Atoi(f[0])
	b, e2 := strconv.Atoi(f[1])
	if e1 != nil || e2 != nil {
		return 0, 0, false
	}
	return a, b, true
}

// ymdToUnix turns a YYYYMMDD integer string into midnight-UTC unix seconds.
func ymdToUnix(s string) int64 {
	if len(s) != 8 {
		return 0
	}
	y, _ := strconv.Atoi(s[:4])
	mo, _ := strconv.Atoi(s[4:6])
	d, _ := strconv.Atoi(s[6:8])
	if y < 1970 || y > 2100 || mo < 1 || mo > 12 || d < 1 || d > 31 {
		return 0
	}
	return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC).Unix()
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// printableText accepts a non-empty, control-free, bounded string. Spaces are
// allowed (GECOS, key comments), unlike printable() for log-path tokens.
func printableText(s string) bool {
	if s == "" || len(s) > maxTargetLen {
		return false
	}
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || (r < 0x20) {
			return false
		}
	}
	return true
}
