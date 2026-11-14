package sshtest

import (
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// A11 mail / DNS role (deepfacts_roles.go): what a generic "infrastructure" VM
// actually is — a mail server (Postfix / Dovecot), a DNS server (BIND), both or
// neither. Like the other deep facts it runs only in a background scan
// (CaptureJob), needs root (the stored SSH password) for the mail log and
// named.conf, and emits ONLY allowlisted tokens: config domain names, the
// relay host, and counts. A recipient address, a client IP, a Dovecot
// username and a forwarder address are all aggregated or dropped on the VM —
// never crosses SSH.

// HostRoles is the VM's stated server role. Each sub-struct is nil when its
// software isn't present, so "neither" (nil, nil → nil HostRoles) means the
// box plays no mail/DNS role at all.
type HostRoles struct {
	Mail *MailRole `json:"mail,omitempty"`
	DNS  *DNSRole  `json:"dns,omitempty"`
}

// MailRole is the Postfix/Dovecot picture: which daemons run, the domains
// Postfix accepts mail for and where it relays, delivery counts over the log
// window (counts only — never a recipient), and Dovecot login counts.
type MailRole struct {
	Postfix      bool      `json:"postfix,omitempty"`
	Dovecot      bool      `json:"dovecot,omitempty"`
	Destinations []string  `json:"destinations,omitempty"` // mydestination + virtual/relay domains
	Relayhost    string    `json:"relayhost,omitempty"`    // where outbound mail is relayed
	Sent         int       `json:"sent,omitempty"`         // status=sent in the log window
	Bounced      int       `json:"bounced,omitempty"`      // status=bounced
	Deferred     int       `json:"deferred,omitempty"`     // status=deferred
	ImapLogins   int       `json:"imap_logins,omitempty"`  // dovecot imap-login successes
	Pop3Logins   int       `json:"pop3_logins,omitempty"`  // dovecot pop3-login successes
	PerDay       []MailDay `json:"per_day,omitempty"`      // delivery counts per day (busiest last)
}

// MailDay is one day's delivery counts. Day is "MM-DD" (the log tail is
// recent, so the year is unambiguous enough for an activity sparkline).
type MailDay struct {
	Day     string `json:"day"`
	Sent    int    `json:"sent,omitempty"`
	Bounced int    `json:"bounced,omitempty"`
}

// DNSRole is the BIND picture: authoritative zones (master/slave) versus a
// resolver (recursion on, no authoritative zones). Zone names are config
// domain names, not secrets.
type DNSRole struct {
	Named         bool     `json:"named,omitempty"`
	Authoritative []string `json:"authoritative,omitempty"` // master/slave zone names
	Recursion     bool     `json:"recursion,omitempty"`     // resolver (recursive) resolver
}

const (
	maxMailDays      = 21  // per-day rows kept (busiest last)
	maxMailDests     = 40  // mydestination/virtual/relay domains kept
	maxAuthZones     = 100 // authoritative zone names kept
	mailLogTailCap   = 8388608
	bindConfCatLimit = "head -c 1048576" // bound the concatenated named.conf
)

// rolesMailAwk aggregates a Postfix/Dovecot mail-log stream. It bumps a
// per-status counter and a per-day (MMDD) counter from the syslog/ISO date,
// counts Dovecot logins, and emits ONLY counts — the recipient address
// (to=<…>), the client IP (rip=…) and the Dovecot username never leave the VM.
const rolesMailAwk = `BEGIN{m["Jan"]=1;m["Feb"]=2;m["Mar"]=3;m["Apr"]=4;m["May"]=5;m["Jun"]=6;m["Jul"]=7;m["Aug"]=8;m["Sep"]=9;m["Oct"]=10;m["Nov"]=11;m["Dec"]=12}
{
  if($0 ~ /imap-login: Login/)imap++
  else if($0 ~ /pop3-login: Login/)pop++
  mmdd=""
  if($1 ~ /^[0-9][0-9][0-9][0-9]-/)mmdd=substr($1,6,2) substr($1,9,2)
  else if($1 in m)mmdd=sprintf("%02d%02d",m[$1],$2+0)
  if(match($0,/status=(sent|bounced|deferred)/)){
    st=substr($0,RSTART+7,RLENGTH-7)
    if(st=="sent"){sent++;if(mmdd!=""){dsent[mmdd]++;days[mmdd]=1}}
    else if(st=="bounced"){bounced++;if(mmdd!=""){dbounced[mmdd]++;days[mmdd]=1}}
    else if(st=="deferred")deferred++
  }
}
END{
  printf "@@mailtot %d %d %d\n",sent,bounced,deferred
  if(imap>0||pop>0)printf "@@maillogin %d %d\n",imap,pop
  for(d in days)printf "@@mailday %s %d %d\n",d,dsent[d]+0,dbounced[d]+0
}`

// rolesBindAwk reads a concatenated named.conf stream and emits one "@@zone
// <name>" per authoritative (master/slave/primary/secondary) zone and one
// "@@dns rec=<0|1>" recursion flag. Forward/hint/stub zones are not
// authoritative and emit nothing; forwarder addresses are never emitted.
const rolesBindAwk = `{
  if(match($0,/zone[ \t]+"[^"]+"/)){z=substr($0,RSTART,RLENGTH);sub(/zone[ \t]+"/,"",z);sub(/"$/,"",z);curzone=z}
  if(curzone!=""&&match($0,/type[ \t]+(master|slave|primary|secondary)/)){print "@@zone " curzone;curzone=""}
  else if(match($0,/type[ \t]+(forward|hint|stub)/))curzone=""
  if($0 ~ /recursion[ \t]+yes/)rec=1
  if($0 ~ /recursion[ \t]+no/)rec=0
}
END{printf "@@dns rec=%d\n",rec}`

// rolesScript is one root-capable session emitting the A11 mail/DNS markers.
// BRIDGE_ROOT prefixes every system read so the fake-root test drives the real
// script. Not root: SUDO=1 retries root-only reads (the mail log, named.conf)
// through sudo -n; without root most sections emit nothing.
func rolesScript(asRoot bool) string {
	head := "SUDO=1\n"
	if asRoot {
		head = ""
	}
	return head + `export PATH="$PATH:/usr/local/sbin:/usr/sbin:/sbin"
R="${BRIDGE_ROOT:-}"
T=""; command -v timeout >/dev/null 2>&1 && T="timeout 20"
S=""; if [ -n "$SUDO" ] && sudo -n true 2>/dev/null; then S="sudo -n"; fi
b64(){ printf %s "$1" | base64 2>/dev/null | tr -d '\n'; }
# ── A11 mail role: Postfix ──
mc="$R/etc/postfix/main.cf"
if [ -f "$mc" ]; then
  echo "@@postfix 1"
  getpf(){ sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" "$mc" 2>/dev/null | head -n1; }
  for key in mydestination virtual_mailbox_domains relay_domains virtual_alias_domains; do
    printf '%s\n' "$(getpf "$key")" | tr ',' ' ' | tr -s ' ' '\n' | while IFS= read -r d; do
      [ -n "$d" ] || continue
      case "$d" in ''|localhost|*'$'*|/*) continue;; esac
      printf '@@maildest %s\n' "$(b64 "$d")"
    done
  done
  rh=$(getpf relayhost)
  case "$rh" in ''|*'$'*) ;; *) printf '@@relayhost %s\n' "$(b64 "$rh")";; esac
fi
# Dovecot present?
if [ -d "$R/etc/dovecot" ] || [ -f "$R/etc/dovecot/dovecot.conf" ]; then echo "@@dovecot 1"; fi
# Mail-log aggregation (counts only — a recipient, client IP or username never leaves the box).
for ml in "$R/var/log/mail.log" "$R/var/log/maillog" "$R/var/log/mail/mail.log"; do
  [ -f "$ml" ] || continue
  $S $T tail -c ` + strconv.Itoa(mailLogTailCap) + ` "$ml" 2>/dev/null | awk '` + rolesMailAwk + `'
  break
done
# ── A11 DNS role: BIND ──
for nc in "$R/etc/named.conf" "$R/etc/bind/named.conf"; do
  [ -f "$nc" ] || continue
  echo "@@named 1"
  d=$(dirname "$nc")
  $S cat "$nc" "$d"/named.conf.* "$d"/*.zones 2>/dev/null | ` + bindConfCatLimit + ` | awk '` + rolesBindAwk + `'
  break
done
echo "@@roles-done"
`
}

// collectHostRoles runs the A11 probe — root with the stored password (the mail
// log and named.conf are usually root-only), else unprivileged. Returns nil
// when neither mail nor DNS software is present or nothing ran.
func collectHostRoles(client *ssh.Client, sudoPassword string) *HostRoles {
	var raw string
	if sudoPassword != "" {
		out, err := runSudoCmd(client, sudoPassword, rolesScript(true))
		if strings.Contains(out, "@@roles-done") {
			raw = out
		} else {
			log.Printf("[sshscan] host roles as root failed, falling back to unprivileged: %v", err)
		}
	}
	if raw == "" {
		raw = runCmd(client, encodedScript(rolesScript(false), "sh"))
	}
	if !strings.Contains(raw, "@@roles-done") {
		return nil
	}
	return parseHostRoles(raw)
}

var roleDomainRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]{0,252})$`)

// validRoleDomain accepts a config domain / zone name (never a secret): a
// bounded label-and-dot string with no "..".
func validRoleDomain(s string) bool {
	return roleDomainRe.MatchString(s) && !strings.Contains(s, "..")
}

// parseHostRoles turns the probe markers into HostRoles. Mail is present when
// Postfix or Dovecot is; DNS when named is. nil when neither.
func parseHostRoles(raw string) *HostRoles {
	if !strings.Contains(raw, "@@roles-done") {
		return nil
	}
	mail := &MailRole{}
	dns := &DNSRole{}
	days := map[string]MailDay{} // key: MMDD
	var dayKeys []string
	for _, line := range strings.Split(raw, "\n") {
		head, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch head {
		case "@@postfix":
			mail.Postfix = true
		case "@@dovecot":
			mail.Dovecot = true
		case "@@maildest":
			if d := decodeToken(rest); validRoleDomain(d) && len(mail.Destinations) < maxMailDests && !contains(mail.Destinations, d) {
				mail.Destinations = append(mail.Destinations, d)
			}
		case "@@relayhost":
			if r := decodeToken(rest); printableText(r) && !strings.ContainsAny(r, " \t") {
				mail.Relayhost = r
			}
		case "@@mailtot":
			if f := strings.Fields(rest); len(f) == 3 {
				mail.Sent, _ = strconv.Atoi(f[0])
				mail.Bounced, _ = strconv.Atoi(f[1])
				mail.Deferred, _ = strconv.Atoi(f[2])
			}
		case "@@maillogin":
			if i, p, ok := parseTwoInts(rest); ok {
				mail.ImapLogins, mail.Pop3Logins = i, p
			}
		case "@@mailday":
			if f := strings.Fields(rest); len(f) == 3 && len(f[0]) == 4 {
				if _, err := strconv.Atoi(f[0]); err == nil {
					s, _ := strconv.Atoi(f[1])
					b, _ := strconv.Atoi(f[2])
					if _, seen := days[f[0]]; !seen {
						dayKeys = append(dayKeys, f[0])
					}
					days[f[0]] = MailDay{Day: f[0][:2] + "-" + f[0][2:], Sent: s, Bounced: b}
				}
			}
		case "@@named":
			dns.Named = true
		case "@@zone":
			if z := strings.TrimSpace(rest); validRoleDomain(z) && len(dns.Authoritative) < maxAuthZones && !contains(dns.Authoritative, z) {
				dns.Authoritative = append(dns.Authoritative, z)
			}
		case "@@dns":
			if v, ok := strings.CutPrefix(rest, "rec="); ok {
				dns.Recursion = strings.TrimSpace(v) == "1"
			}
		}
	}
	// Busiest days first, then cap: the UI shows recent activity.
	sort.SliceStable(dayKeys, func(i, j int) bool {
		di, dj := days[dayKeys[i]], days[dayKeys[j]]
		if di.Sent+di.Bounced != dj.Sent+dj.Bounced {
			return di.Sent+di.Bounced > dj.Sent+dj.Bounced
		}
		return dayKeys[i] > dayKeys[j]
	})
	for _, k := range dayKeys {
		if len(mail.PerDay) >= maxMailDays {
			break
		}
		mail.PerDay = append(mail.PerDay, days[k])
	}
	sort.Strings(mail.Destinations)
	sort.Strings(dns.Authoritative)

	var hr HostRoles
	if mail.Postfix || mail.Dovecot {
		hr.Mail = mail
	}
	if dns.Named {
		hr.DNS = dns
	}
	if hr.Mail == nil && hr.DNS == nil {
		return nil
	}
	return &hr
}
