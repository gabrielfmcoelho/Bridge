package sshtest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestParseP2Facts drives the P2 parser with synthetic markers for all four
// facts and asserts the decoded, sorted structs.
func TestParseP2Facts(t *testing.T) {
	raw := strings.Join([]string{
		"@@panel virtualmin",
		"@@panel webmin",
		"@@vdom " + b64("concursos.seadprev.pi.gov.br") + " " + b64("seadprev") + " 1646300000 1 1762797720 " + b64("Desligado pela equipe") + " " + b64("admin@seadprev.pi.gov.br") + " 1",
		"@@vdom " + b64("concursos.sead.pi.gov.br") + " " + b64("sead.pi.gov.br") + " 1684000000 0 0 " + b64("") + " " + b64("sead@sead.pi.gov.br") + " 1",
		"@@puser " + b64("root"),
		"@@puser " + b64("seadprev"),
		"@@pfail 3 2",
		"@@doc " + b64("/home/sead/public_html"),
		"@@fw codeigniter3 " + b64("3.1.11") + " " + b64(""),
		"@@fw codeigniter3 " + b64("3.1.13") + " " + b64("sasc"),
		"@@mtime 1763000000",
		"@@upload " + b64("arquivos") + " 34000000 1724200000 59498",
		"@@last suporteati 20230505",
		"@@last sead 20261110",
		"@@gecos " + b64("sead") + " " + b64("Sead Admin"),
		"@@sudoer " + b64("sead"),
		"@@sudoer " + b64("suporteati"),
		"@@sudoer " + b64("sead"), // dup dropped
		"@@authkey " + b64("root") + " " + b64("deploy@panet"),
		"@@ftp seaduser 202611 7",
		"@@ufw 0 0",
		"@@firewalld 1 18",
		"@@iptables 12",
		"@@f2b " + b64("sshd") + " 2",
		"@@f2b " + b64("proftpd") + " 0",
		"@@adminport 10000 all",
		"@@adminport 3306 loopback",
		"@@adminport 20000 all",
		"@@p2-done",
	}, "\n")

	cp, code, owner, fw := parseP2Facts(raw)
	if cp == nil || code == nil || owner == nil || fw == nil {
		t.Fatalf("empty section: cp=%v code=%v owner=%v fw=%v", cp, code, owner, fw)
	}

	// A6 panels
	if !reflect.DeepEqual(cp.Installed, []string{"virtualmin", "webmin"}) {
		t.Errorf("installed = %v", cp.Installed)
	}
	if !reflect.DeepEqual(cp.Users, []string{"root", "seadprev"}) {
		t.Errorf("users = %v", cp.Users)
	}
	if len(cp.Domains) != 2 || cp.Domains[0].Name != "concursos.sead.pi.gov.br" || cp.Domains[1].Name != "concursos.seadprev.pi.gov.br" {
		t.Fatalf("domains = %+v", cp.Domains)
	}
	d := cp.Domains[1]
	if !d.Disabled || d.DisabledTime != 1762797720 || d.DisabledReason != "Desligado pela equipe" || d.Owner != "seadprev" || d.Contact != "admin@seadprev.pi.gov.br" || !d.LetsEncrypt || d.Created != 1646300000 {
		t.Errorf("seadprev domain = %+v", d)
	}
	if cp.Domains[0].Disabled || cp.Domains[0].DisabledReason != "" {
		t.Errorf("sead domain should be enabled: %+v", cp.Domains[0])
	}
	if cp.FailedLogins == nil || cp.FailedLogins.Private != 1 || cp.FailedLogins.Public != 2 {
		t.Errorf("failed logins = %+v", cp.FailedLogins)
	}

	// A7 code
	if len(code.Docroots) != 1 {
		t.Fatalf("docroots = %+v", code.Docroots)
	}
	c := code.Docroots[0]
	if c.Docroot != "/home/sead/public_html" || c.NewestMtime != 1763000000 {
		t.Errorf("docroot = %+v", c)
	}
	wantApps := []CodeApp{{Path: "", Framework: "codeigniter3", Version: "3.1.11"}, {Path: "sasc", Framework: "codeigniter3", Version: "3.1.13"}}
	if !reflect.DeepEqual(c.Apps, wantApps) {
		t.Errorf("apps = %+v", c.Apps)
	}
	if len(c.Uploads) != 1 || c.Uploads[0].Dir != "arquivos" || c.Uploads[0].Files != 59498 || c.Uploads[0].SizeBytes != 34000000*1024 || c.Uploads[0].Newest != 1724200000 {
		t.Errorf("uploads = %+v", c.Uploads)
	}

	// A8 owner
	wantLast := []OwnerLast{
		{User: "sead", When: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC).Unix()},
		{User: "suporteati", When: time.Date(2023, 5, 5, 0, 0, 0, 0, time.UTC).Unix()},
	}
	if !reflect.DeepEqual(owner.LastLogins, wantLast) {
		t.Errorf("last logins = %+v", owner.LastLogins)
	}
	if !reflect.DeepEqual(owner.Gecos, []OwnerGecos{{User: "sead", Gecos: "Sead Admin"}}) {
		t.Errorf("gecos = %+v", owner.Gecos)
	}
	if !reflect.DeepEqual(owner.Sudoers, []string{"sead", "suporteati"}) {
		t.Errorf("sudoers = %+v", owner.Sudoers)
	}
	if !reflect.DeepEqual(owner.AuthKeys, []OwnerAuthKey{{User: "root", Comment: "deploy@panet"}}) {
		t.Errorf("authkeys = %+v", owner.AuthKeys)
	}
	if !reflect.DeepEqual(owner.FTP, []OwnerFTP{{User: "seaduser", Month: "2026-11", Upload: 7}}) {
		t.Errorf("ftp = %+v", owner.FTP)
	}

	// A10 firewall
	if fw.UFW == nil || fw.UFW.Active || fw.Firewalld == nil || !fw.Firewalld.Active || fw.Firewalld.Rules != 18 || fw.Iptables != 12 {
		t.Errorf("fw layers = %+v", fw)
	}
	if !reflect.DeepEqual(fw.Fail2ban, []F2bJail{{Jail: "proftpd", Banned: 0}, {Jail: "sshd", Banned: 2}}) {
		t.Errorf("fail2ban = %+v", fw.Fail2ban)
	}
	wantPorts := []AdminPort{{Port: 3306, Exposure: "loopback"}, {Port: 10000, Exposure: "all"}, {Port: 20000, Exposure: "all"}}
	if !reflect.DeepEqual(fw.AdminPorts, wantPorts) {
		t.Errorf("admin ports = %+v", fw.AdminPorts)
	}
}

func TestParseP2FactsNotRun(t *testing.T) {
	cp, code, owner, fw := parseP2Facts("")
	if cp != nil || code != nil || owner != nil || fw != nil {
		t.Fatal("no markers: want all nil")
	}
	cp, code, owner, fw = parseP2Facts("@@p2-done\n")
	if cp != nil || code != nil || owner != nil || fw != nil {
		t.Fatal("only sentinel: want all nil")
	}
}

// p2FakeRoot builds a fake root + a stubs dir modelling a Virtualmin host
// (host 35's shape) and returns the probe output. The stubs make last / ufw /
// firewall-cmd / nft / iptables / fail2ban-client / ss deterministic across
// machines; awkBin (empty = system) lets the portability test swap awk.
func p2FakeRoot(t *testing.T, awkBin string) (string, string) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string, mtime time.Time) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if !mtime.IsZero() {
			if err := os.Chtimes(p, mtime, mtime); err != nil {
				t.Fatal(err)
			}
		}
	}

	// A6: two Virtualmin domains (one disabled) + a non-domain file that must be
	// skipped; panel users; failed logins from public + private sources.
	write("etc/webmin/virtual-server/domains/123456789", strings.Join([]string{
		"dom=concursos.seadprev.pi.gov.br", "user=seadprev", "created=1646300000",
		"disabled=web,dns,mail", "disabled_time=1762797720", "disabled_reason=Desligado pela equipe",
		"emailto=admin@seadprev.pi.gov.br", "letsencrypt_last=1757000000", "",
	}, "\n"), time.Time{})
	write("etc/webmin/virtual-server/domains/987654321", strings.Join([]string{
		"dom=concursos.sead.pi.gov.br", "user=sead.pi.gov.br", "created=1684000000",
		"emailto=sead@sead.pi.gov.br", "letsencrypt_last=1757700000", "",
	}, "\n"), time.Time{})
	write("etc/webmin/virtual-server/domains/README.cgi", "not a domain\n", time.Time{})
	write("etc/webmin/miniserv.users", "root:x:0\nseadprev:y:0\n", time.Time{})
	write("var/webmin/miniserv.log", strings.Join([]string{
		"[10/Nov/2026:18:00:00] miniserv.pl: Invalid login as admin from 45.33.22.11",
		"[10/Nov/2026:18:01:00] miniserv.pl: Invalid login as root from 10.0.80.52",
		"[10/Nov/2026:18:02:00] miniserv.pl: Non-existent login as test from 203.0.113.9", "",
	}, "\n"), time.Time{})

	// A7: a CodeIgniter 3 docroot with a /sasc sub-app and an upload dir.
	doc := "home/sead/public_html"
	write(doc+"/system/core/CodeIgniter.php", "<?php\nconst CI_VERSION = '3.1.11';\n", time.Date(2025, 5, 15, 0, 0, 0, 0, time.UTC))
	write(doc+"/sasc/system/core/CodeIgniter.php", "<?php\nconst CI_VERSION = '3.1.13';\n", time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC))
	idxTime := time.Date(2026, 11, 13, 2, 19, 0, 0, time.UTC)
	write(doc+"/index.php", "<?php // entry\n", idxTime)
	upTime := time.Date(2024, 8, 21, 0, 0, 0, 0, time.UTC)
	write(doc+"/arquivos/edital.pdf", "%PDF fixture\n", upTime)
	write(doc+"/arquivos/anexo.pdf", "%PDF fixture 2\n", time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC))

	// A8: human GECOS (uid 1000), a system account to skip, sudo members, a root
	// authorized_keys with secret key material + a comment, FTP uploads.
	write("etc/passwd", strings.Join([]string{
		"root:x:0:0:root:/root:/bin/bash",
		"sead:x:1000:1000:Sead Admin:/home/sead:/bin/bash",
		"www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin", "",
	}, "\n"), time.Time{})
	write("etc/group", "sudo:x:27:sead,suporteati\nwww-data:x:33:\n", time.Time{})
	write("root/.ssh/authorized_keys",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAISECRETKEYMATERIAL deploy@panet\n", time.Time{})
	write("var/log/proftpd/xferlog",
		"Mon Nov 10 18:00:00 2026 1 203.0.113.5 1024 /home/sead/x.pdf b _ i r seaduser ftp 0 * c\n"+
			"Tue Nov 11 09:00:00 2026 2 198.51.100.7 2048 /home/sead/out.pdf b _ o r seaduser ftp 0 * c\n", time.Time{})

	// Stubs: deterministic firewall + last.
	stubs := t.TempDir()
	stub := func(name, body string) {
		if err := os.WriteFile(filepath.Join(stubs, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub("last", `cat <<'EOF'
sead     pts/0        10.0.80.52       Mon Nov 10 18:19:33 2026 - 18:25  (00:05)
suporteati pts/1      10.0.80.80       Fri May  5 09:00:00 2023 - 09:10  (00:10)
reboot   system boot  5.15.0-203       Fri Nov 13 01:47:00 2026
wtmp begins Wed Mar 12 00:00:00 2023
EOF`)
	stub("ufw", `printf 'Status: inactive\n'`)
	stub("firewall-cmd", `[ "$1" = --state ] && { echo running; exit 0; }; exit 0`)
	stub("nft", `cat <<'EOF'
table inet firewalld {
  chain filter_IN_public_allow {
    tcp dport 10000 accept
    tcp dport 443 accept
    tcp dport 22 accept
  }
}
EOF`)
	stub("iptables", `printf -- '-A INPUT -j ACCEPT\n-A FORWARD -j DROP\n'`)
	stub("fail2ban-client", `case "$2" in
'') printf 'Status\n|- Number of jail:\t2\n`+"`"+`- Jail list:\tsshd, proftpd\n';;
sshd) printf 'Status for the jail: sshd\n|- Filter\n`+"`"+`- Actions\n   `+"`"+`- Currently banned:\t2\n';;
proftpd) printf 'Status for the jail: proftpd\n   `+"`"+`- Currently banned:\t0\n';;
esac`)
	stub("ss", `cat <<'EOF'
LISTEN 0 128 0.0.0.0:10000 0.0.0.0:*
LISTEN 0 128 127.0.0.1:3306 0.0.0.0:*
LISTEN 0 128 10.0.122.19:443 0.0.0.0:*
LISTEN 0 128 [::]:20000 [::]:*
EOF`)
	// awk override for the portability test.
	if awkBin != "" {
		if err := os.Symlink(awkBin, filepath.Join(stubs, "awk")); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("sh", "-c", p2Script(true, []string{"/" + doc}))
	cmd.Env = []string{"PATH=" + stubs + ":" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return string(out), idxTime.Format("2006")
}

// TestP2ScriptRuns runs the real probe against the fake root and asserts both
// the parsed facts AND the privacy guarantee: no client IP, SSH key material,
// xferlog remote host or filename ever leaves the VM.
func TestP2ScriptRuns(t *testing.T) {
	out, _ := p2FakeRoot(t, "")

	// Privacy: none of these may appear in the probe output.
	leaks := []string{
		"10.0.80.52", "10.0.80.80", // last source hosts
		"45.33.22.11", "203.0.113.9", "10.0.80", // miniserv failed-login IPs
		"203.0.113.5", "198.51.100.7", // xferlog remote hosts
		"SECRETKEYMATERIAL", "AAAAC3", // SSH key material
		"x.pdf", "out.pdf", "/home/sead/x", // xferlog filenames
	}
	for _, s := range leaks {
		if strings.Contains(out, s) {
			t.Fatalf("%q leaked into probe output:\n%s", s, out)
		}
	}

	cp, code, owner, fw := parseP2Facts(out)
	blob, _ := json.Marshal(struct {
		CP *ControlPanels
		C  *CodeFingerprintSet
		O  *OwnerClues
		F  *Firewall
	}{cp, code, owner, fw})
	for _, s := range leaks {
		if strings.Contains(string(blob), s) {
			t.Fatalf("%q leaked into parsed structs: %s", s, blob)
		}
	}

	if cp == nil || code == nil || owner == nil || fw == nil {
		t.Fatalf("missing section:\n%s", out)
	}

	// A6
	if !reflect.DeepEqual(cp.Installed, []string{"virtualmin", "webmin"}) {
		t.Errorf("installed = %v\n%s", cp.Installed, out)
	}
	if len(cp.Domains) != 2 {
		t.Fatalf("domains = %+v\n%s", cp.Domains, out)
	}
	seadprev := cp.Domains[1]
	if seadprev.Name != "concursos.seadprev.pi.gov.br" || !seadprev.Disabled || seadprev.DisabledTime != 1762797720 ||
		seadprev.DisabledReason != "Desligado pela equipe" || seadprev.Owner != "seadprev" || !seadprev.LetsEncrypt {
		t.Errorf("seadprev domain = %+v\n%s", seadprev, out)
	}
	if !reflect.DeepEqual(cp.Users, []string{"root", "seadprev"}) {
		t.Errorf("users = %v\n%s", cp.Users, out)
	}
	if cp.FailedLogins == nil || cp.FailedLogins.Public != 2 || cp.FailedLogins.Private != 1 {
		t.Errorf("failed logins = %+v\n%s", cp.FailedLogins, out)
	}

	// A7
	if len(code.Docroots) != 1 {
		t.Fatalf("docroots = %+v\n%s", code.Docroots, out)
	}
	c := code.Docroots[0]
	if c.Docroot != "/home/sead/public_html" {
		t.Errorf("docroot = %q\n%s", c.Docroot, out)
	}
	wantApps := []CodeApp{{Path: "", Framework: "codeigniter3", Version: "3.1.11"}, {Path: "sasc", Framework: "codeigniter3", Version: "3.1.13"}}
	if !reflect.DeepEqual(c.Apps, wantApps) {
		t.Errorf("apps = %+v\n%s", c.Apps, out)
	}
	if c.NewestMtime != time.Date(2026, 11, 13, 2, 19, 0, 0, time.UTC).Unix() {
		t.Errorf("newest mtime = %d want %d\n%s", c.NewestMtime, time.Date(2026, 11, 13, 2, 19, 0, 0, time.UTC).Unix(), out)
	}
	if len(c.Uploads) != 1 || c.Uploads[0].Dir != "arquivos" || c.Uploads[0].Files != 2 ||
		c.Uploads[0].SizeBytes <= 0 || c.Uploads[0].Newest != time.Date(2024, 8, 21, 0, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("uploads = %+v\n%s", c.Uploads, out)
	}

	// A8
	if !reflect.DeepEqual(owner.Gecos, []OwnerGecos{{User: "sead", Gecos: "Sead Admin"}}) {
		t.Errorf("gecos = %+v\n%s", owner.Gecos, out)
	}
	if !reflect.DeepEqual(owner.Sudoers, []string{"sead", "suporteati"}) {
		t.Errorf("sudoers = %+v\n%s", owner.Sudoers, out)
	}
	if !reflect.DeepEqual(owner.AuthKeys, []OwnerAuthKey{{User: "root", Comment: "deploy@panet"}}) {
		t.Errorf("authkeys = %+v\n%s", owner.AuthKeys, out)
	}
	wantLast := []OwnerLast{
		{User: "sead", When: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC).Unix()},
		{User: "suporteati", When: time.Date(2023, 5, 5, 0, 0, 0, 0, time.UTC).Unix()},
	}
	if !reflect.DeepEqual(owner.LastLogins, wantLast) {
		t.Errorf("last logins = %+v\n%s", owner.LastLogins, out)
	}
	if !reflect.DeepEqual(owner.FTP, []OwnerFTP{{User: "seaduser", Month: "2026-11", Upload: 1}}) {
		t.Errorf("ftp (only the incoming transfer) = %+v\n%s", owner.FTP, out)
	}

	// A10
	if fw.UFW == nil || fw.UFW.Active {
		t.Errorf("ufw = %+v\n%s", fw.UFW, out)
	}
	if fw.Firewalld == nil || !fw.Firewalld.Active || fw.Firewalld.Rules != 3 {
		t.Errorf("firewalld = %+v want active 3 dport\n%s", fw.Firewalld, out)
	}
	if fw.Iptables != 2 {
		t.Errorf("iptables = %d want 2\n%s", fw.Iptables, out)
	}
	if !reflect.DeepEqual(fw.Fail2ban, []F2bJail{{Jail: "proftpd", Banned: 0}, {Jail: "sshd", Banned: 2}}) {
		t.Errorf("fail2ban = %+v\n%s", fw.Fail2ban, out)
	}
	wantPorts := []AdminPort{{Port: 3306, Exposure: "loopback"}, {Port: 10000, Exposure: "all"}, {Port: 20000, Exposure: "all"}}
	if !reflect.DeepEqual(fw.AdminPorts, wantPorts) {
		t.Errorf("admin ports = %+v\n%s", fw.AdminPorts, out)
	}
}

// TestP2ScriptAwkPortability runs the real probe with awk pointed at gawk, mawk
// and busybox awk in turn: the owner-clue and firewall awk programs must parse
// identically under each (no gawk-only features).
func TestP2ScriptAwkPortability(t *testing.T) {
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
			// busybox needs an "awk" argv[0]: a wrapper that execs `busybox awk`.
			dir := t.TempDir()
			w := filepath.Join(dir, "awk")
			if err := os.WriteFile(w, []byte("#!/bin/sh\nexec "+bin+" awk \"$@\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			awkBin = w
		}
		out, _ := p2FakeRoot(t, awkBin)
		_, _, owner, fw := parseP2Facts(out)
		if owner == nil || fw == nil {
			t.Fatalf("%s: owner/fw empty\n%s", name, out)
		}
		got, _ := json.Marshal(struct {
			O *OwnerClues
			F *Firewall
		}{owner, fw})
		t.Logf("%s owner/fw: %s", name, got)
		if baseline == "" {
			baseline = string(got)
		} else if string(got) != baseline {
			t.Errorf("%s diverged:\n got %s\nwant %s", name, got, baseline)
		}
	}
}
