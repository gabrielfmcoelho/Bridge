package sshtest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestParseHostRoles drives the A11 parser with synthetic markers and asserts
// the decoded, sorted structs for a box that is both a mail and a DNS server.
func TestParseHostRoles(t *testing.T) {
	raw := strings.Join([]string{
		"@@postfix 1",
		"@@maildest " + b64("sead.pi.gov.br"),
		"@@maildest " + b64("mail.sead.pi.gov.br"),
		"@@maildest " + b64("sead.pi.gov.br"), // dup dropped
		"@@relayhost " + b64("[smtp.sead.pi.gov.br]:587"),
		"@@dovecot 1",
		"@@mailtot 142 7 3",
		"@@maillogin 58 4",
		"@@mailday 1114 90 2",
		"@@mailday 1113 52 5",
		"@@named 1",
		"@@zone concursos.sead.pi.gov.br",
		"@@zone 0.0.127.in-addr.arpa",
		"@@dns rec=0",
		"@@roles-done",
	}, "\n")

	hr := parseHostRoles(raw)
	if hr == nil || hr.Mail == nil || hr.DNS == nil {
		t.Fatalf("empty: %+v", hr)
	}
	m := hr.Mail
	if !m.Postfix || !m.Dovecot || m.Sent != 142 || m.Bounced != 7 || m.Deferred != 3 || m.ImapLogins != 58 || m.Pop3Logins != 4 {
		t.Errorf("mail = %+v", m)
	}
	if !reflect.DeepEqual(m.Destinations, []string{"mail.sead.pi.gov.br", "sead.pi.gov.br"}) {
		t.Errorf("destinations = %v", m.Destinations)
	}
	if m.Relayhost != "[smtp.sead.pi.gov.br]:587" {
		t.Errorf("relayhost = %q", m.Relayhost)
	}
	// Busiest day first.
	wantDays := []MailDay{{Day: "11-14", Sent: 90, Bounced: 2}, {Day: "11-13", Sent: 52, Bounced: 5}}
	if !reflect.DeepEqual(m.PerDay, wantDays) {
		t.Errorf("per_day = %+v", m.PerDay)
	}
	d := hr.DNS
	if !d.Named || d.Recursion {
		t.Errorf("dns = %+v", d)
	}
	if !reflect.DeepEqual(d.Authoritative, []string{"0.0.127.in-addr.arpa", "concursos.sead.pi.gov.br"}) {
		t.Errorf("zones = %v", d.Authoritative)
	}
}

func TestParseHostRolesNone(t *testing.T) {
	if hr := parseHostRoles(""); hr != nil {
		t.Fatal("no sentinel: want nil")
	}
	if hr := parseHostRoles("@@roles-done\n"); hr != nil {
		t.Fatal("no mail/dns software: want nil")
	}
	// A pure resolver (recursion on, no authoritative zones) is still a DNS role.
	hr := parseHostRoles("@@named 1\n@@dns rec=1\n@@roles-done\n")
	if hr == nil || hr.Mail != nil || hr.DNS == nil || !hr.DNS.Recursion || len(hr.DNS.Authoritative) != 0 {
		t.Fatalf("resolver-only = %+v", hr)
	}
}

// rolesFakeRoot builds a fake root modelling a Postfix+Dovecot+BIND host and
// returns the probe output. awkBin (empty = system) lets the portability test
// swap awk.
func rolesFakeRoot(t *testing.T, awkBin string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("etc/postfix/main.cf", strings.Join([]string{
		"myhostname = mail.sead.pi.gov.br",
		"mydestination = $myhostname, localhost.$mydomain, localhost, sead.pi.gov.br",
		"virtual_mailbox_domains = concursos.sead.pi.gov.br",
		"relayhost = [smtp.sead.pi.gov.br]:587",
		"",
	}, "\n"))
	write("etc/dovecot/dovecot.conf", "protocols = imap pop3\n")
	// Mail log: recipient addresses, client IPs and a Dovecot username that must
	// NOT leave the box — only sent/bounced/deferred counts and login counts do.
	write("var/log/mail.log", strings.Join([]string{
		"Nov 14 10:23:45 host postfix/smtp[1234]: A1B2C3: to=<candidato@victim.gov.br>, relay=mx.victim.gov.br[203.0.113.9]:25, status=sent (250 Ok)",
		"Nov 14 10:24:00 host postfix/smtp[1235]: D4E5F6: to=<outro@victim.gov.br>, status=sent (250 Ok)",
		"Nov 14 10:25:00 host postfix/smtp[1236]: 778899: to=<bad@nowhere.example>, status=bounced (host said: 550 no such user)",
		"Nov 13 22:00:00 host postfix/smtp[1000]: 001122: to=<x@victim.gov.br>, status=deferred (connection timed out)",
		"Nov 14 09:00:00 host dovecot: imap-login: Login: user=<secretadmin>, method=PLAIN, rip=198.51.100.7, lip=10.0.0.5",
		"Nov 14 09:05:00 host dovecot: pop3-login: Login: user=<secretadmin>, rip=198.51.100.7",
		"",
	}, "\n"))
	// BIND: one authoritative forward zone, one authoritative reverse zone, a
	// hint zone (not authoritative), recursion off, a forwarder (never emitted).
	write("etc/named.conf", strings.Join([]string{
		"options {",
		"    directory \"/var/named\";",
		"    recursion no;",
		"    forwarders { 8.8.8.8; 192.0.2.53; };",
		"};",
		"zone \".\" IN { type hint; file \"named.ca\"; };",
		"zone \"concursos.sead.pi.gov.br\" IN {",
		"    type master;",
		"    file \"concursos.sead.pi.gov.br.zone\";",
		"};",
		"zone \"0.0.127.in-addr.arpa\" { type master; file \"named.localhost\"; };",
		"",
	}, "\n"))

	stubs := t.TempDir()
	if awkBin != "" {
		if err := os.Symlink(awkBin, filepath.Join(stubs, "awk")); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "-c", rolesScript(true))
	cmd.Env = []string{"PATH=" + stubs + ":" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return string(out)
}

// TestRolesScriptRuns runs the real probe against the fake root and asserts
// both the parsed role AND the privacy guarantee: no recipient address, client
// IP, Dovecot username or forwarder address ever leaves the VM.
func TestRolesScriptRuns(t *testing.T) {
	out := rolesFakeRoot(t, "")

	leaks := []string{
		"candidato", "outro", "victim.gov.br", "nowhere.example", // recipient addresses
		"203.0.113.9", "198.51.100.7", "10.0.0.5", // client/relay IPs
		"secretadmin",         // dovecot username
		"8.8.8.8", "192.0.2.53", // bind forwarders
	}
	for _, s := range leaks {
		if strings.Contains(out, s) {
			t.Fatalf("%q leaked into probe output:\n%s", s, out)
		}
	}

	hr := parseHostRoles(out)
	blob, _ := json.Marshal(hr)
	for _, s := range leaks {
		if strings.Contains(string(blob), s) {
			t.Fatalf("%q leaked into parsed struct: %s", s, blob)
		}
	}

	if hr == nil || hr.Mail == nil || hr.DNS == nil {
		t.Fatalf("missing role:\n%s", out)
	}
	m := hr.Mail
	if !m.Postfix || !m.Dovecot {
		t.Errorf("daemons = postfix:%v dovecot:%v\n%s", m.Postfix, m.Dovecot, out)
	}
	if m.Sent != 2 || m.Bounced != 1 || m.Deferred != 1 {
		t.Errorf("delivery counts = sent:%d bounced:%d deferred:%d\n%s", m.Sent, m.Bounced, m.Deferred, out)
	}
	if m.ImapLogins != 1 || m.Pop3Logins != 1 {
		t.Errorf("logins = imap:%d pop3:%d\n%s", m.ImapLogins, m.Pop3Logins, out)
	}
	if !reflect.DeepEqual(m.Destinations, []string{"concursos.sead.pi.gov.br", "sead.pi.gov.br"}) {
		t.Errorf("destinations = %v\n%s", m.Destinations, out)
	}
	if m.Relayhost != "[smtp.sead.pi.gov.br]:587" {
		t.Errorf("relayhost = %q\n%s", m.Relayhost, out)
	}
	// "Nov 14" is the busiest day (2 sent), "Nov 13" has 0 sent / 0 bounced
	// (only a deferred, which carries no per-day row).
	if len(m.PerDay) != 1 || m.PerDay[0].Day != "11-14" || m.PerDay[0].Sent != 2 || m.PerDay[0].Bounced != 1 {
		t.Errorf("per_day = %+v\n%s", m.PerDay, out)
	}
	d := hr.DNS
	if !d.Named || d.Recursion {
		t.Errorf("dns named:%v recursion:%v\n%s", d.Named, d.Recursion, out)
	}
	if !reflect.DeepEqual(d.Authoritative, []string{"0.0.127.in-addr.arpa", "concursos.sead.pi.gov.br"}) {
		t.Errorf("zones = %v (hint zone must be excluded)\n%s", d.Authoritative, out)
	}
}

// TestRolesScriptAwkPortability runs the probe with awk pointed at gawk, mawk
// and busybox awk in turn: the mail and bind awk programs must parse
// identically under each (no gawk-only features).
func TestRolesScriptAwkPortability(t *testing.T) {
	impls := map[string]string{"gawk": "", "mawk": "", "busybox-awk": ""}
	for name := range impls {
		bin := name
		if name == "busybox-awk" {
			bin = "busybox"
		}
		p, err := exec.LookPath(bin)
		if err != nil {
			delete(impls, name)
			continue
		}
		impls[name] = p
	}
	if len(impls) == 0 {
		t.Skip("no alternate awk implementation available")
	}

	var baseline string
	for name, bin := range impls {
		awkBin := bin
		if name == "busybox-awk" {
			dir := t.TempDir()
			w := filepath.Join(dir, "awk")
			if err := os.WriteFile(w, []byte("#!/bin/sh\nexec "+bin+" awk \"$@\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			awkBin = w
		}
		out := rolesFakeRoot(t, awkBin)
		hr := parseHostRoles(out)
		if hr == nil || hr.Mail == nil || hr.DNS == nil {
			t.Fatalf("%s: empty role\n%s", name, out)
		}
		got, _ := json.Marshal(hr)
		t.Logf("%s roles: %s", name, got)
		if baseline == "" {
			baseline = string(got)
		} else if string(got) != baseline {
			t.Errorf("%s diverged:\n got %s\nwant %s", name, got, baseline)
		}
	}
}
