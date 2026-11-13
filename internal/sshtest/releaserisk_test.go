package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseReleaseRisks(t *testing.T) {
	if parseReleaseRisks("") != nil || parseReleaseRisks("@@df\nx\n") != nil {
		t.Fatal("no @@os: the probe didn't run, nil")
	}
	r := parseReleaseRisks(`@@os
ubuntu "22.04"
@@foreign
php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1
foo 1.0~ubuntu22.04.1~ppa1
bad;name 1.0+ubuntu18.04
@@local
libssl1.1 1.1.1f-1ubuntu2.20
php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1
@@failed
● fail2ban.service
bad unit$(x)
@@audit
proftpd-core
@@disabled
relative/path
/etc/apt/sources.list.d/a.list
/etc/apt/sources.list.d/a.list
@@apache
/etc/apache2/ports.conf:1:Listen 80
/etc/apache2/ports.conf:3:    Listen [::]:443 https
/etc/apache2/sites-enabled/s.conf:2:<Directory "/home/s/public_html">
/etc/apache2/sites-enabled/s.conf:3:  FCGIWrapper "/home/s/fcgi-bin/php7.2.fcgi" .php
/etc/apache2/sites-enabled/s.conf:4:</Directory>
/etc/apache2/sites-enabled/s.conf:9:  SetHandler "proxy:unix:/run/php/php8.1-fpm.sock|fcgi://localhost"
/etc/apache2/sites-enabled/s.conf:10:  SetHandler fcgid-script
@@fcgi
/home/s/fcgi-bin/php7.2.fcgi /usr/bin/php-cgi7.2 missing
@@sockets
/run/php/php8.1-fpm.sock ok
`)
	if r.ForeignCount != 2 || !reflect.DeepEqual(r.ForeignPackages, []string{"libssl1.1 1.1.1f-1ubuntu2.20", "php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1"}) {
		t.Fatalf("foreign = %d %q", r.ForeignCount, r.ForeignPackages)
	}
	if !reflect.DeepEqual(r.FailedUnits, []string{"fail2ban.service"}) || !reflect.DeepEqual(r.DpkgBroken, []string{"proftpd-core"}) ||
		!reflect.DeepEqual(r.DisabledSources, []string{"/etc/apt/sources.list.d/a.list"}) {
		t.Fatalf("lists = %+v", r)
	}
	if !reflect.DeepEqual(r.ApacheListen, []int{80, 443}) {
		t.Fatalf("listen = %v", r.ApacheListen)
	}
	want := []PHPHandler{
		{Source: "/etc/apache2/sites-enabled/s.conf:3", Dir: "/home/s/public_html", Kind: "fcgi", Target: "/home/s/fcgi-bin/php7.2.fcgi",
			Runs: "/usr/bin/php-cgi7.2", Version: "7.2", State: "missing"},
		{Source: "/etc/apache2/sites-enabled/s.conf:9", Kind: "fpm", Target: "/run/php/php8.1-fpm.sock", Version: "8.1", State: "ok"},
	}
	if !reflect.DeepEqual(r.PHPHandlers, want) {
		t.Fatalf("handlers = %+v", r.PHPHandlers)
	}
}

// The probe, run for real on a fake root: what SAPASP-CONCURSO looked like
// after its release, with stubbed dpkg / apt / systemctl. Only names, paths
// and ports come back: never a line of a wrapper or a conffile.
func TestReleaseRiskScriptRuns(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string, mode os.FileMode) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("etc/os-release", "ID=ubuntu\nVERSION_ID=\"22.04\"\n", 0o644)
	write("etc/proftpd/proftpd.conf", "IdentLookups off\n", 0o644)
	write("etc/ssh/sshd_config", "PasswordAuthentication no\n", 0o644)
	write("etc/proftpd/modules.conf.ucf-dist", "x\n", 0o644)
	write("etc/logrotate.conf.dpkg-dist", "x\n", 0o644)
	write("etc/apt/sources.list.d/ondrej-php.list", "# deb http://ppa.launchpad.net/ondrej/php/ubuntu focal main # disabled on upgrade to jammy\n", 0o644)
	write("etc/apt/sources.list.d/zabbix.list.distUpgrade", "deb http://repo.zabbix.com/zabbix/6.0/ubuntu focal main\n", 0o644)
	write("etc/apt/sources.list.d/good.list", "deb https://user:s3cret@repo.example jammy main\n# deb-src https://repo.example jammy main\n", 0o644)
	write("etc/apt/sources.list.d/off.sources", "Types: deb\nEnabled: no\n", 0o644)
	write("etc/apache2/ports.conf", "Listen 80\n<IfModule ssl_module>\n\tListen 443\n</IfModule>\n", 0o644)
	write("etc/apache2/sites-enabled/sasc.conf", `<VirtualHost *:444>
	ServerName sasc.example.gov.br
	<Directory /home/sasc/public_html>
		AddHandler fcgid-script .php
		FCGIWrapper /home/sasc/fcgi-bin/php7.2.fcgi .php
	</Directory>
</VirtualHost>
<VirtualHost *:80>
	<FilesMatch \.php$>
		SetHandler "proxy:unix:/run/php/php8.1-fpm.sock|fcgi://localhost"
	</FilesMatch>
	FcgidWrapper "/home/novo/fcgi-bin/php8.4.fcgi" .php
	FCGIWrapper /home/gone/fcgi-bin/php5.fcgi .php
	FCGIWrapper /home/locked/fcgi-bin/php7.4.fcgi .php
</VirtualHost>
`, 0o644)
	// The wrappers hold what they hold: only the exec'd binary leaves.
	write("home/sasc/fcgi-bin/php7.2.fcgi", "#!/bin/bash\nexport DB_PASSWORD=hunter2\nexec /usr/bin/php-cgi7.2\n", 0o755)
	write("home/novo/fcgi-bin/php8.4.fcgi", "#!/bin/bash\nexec /usr/bin/php-cgi8.4\n", 0o755)
	write("usr/bin/php-cgi8.4", "", 0o755)
	write("home/gone/fcgi-bin/.keep", "", 0o644)
	write("home/locked/fcgi-bin/php7.4.fcgi", "#!/bin/bash\nexec /usr/bin/php-cgi7.4\n", 0o755)
	locked := filepath.Join(root, "home/locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	stubs := t.TempDir()
	for name, body := range map[string]string{
		"dpkg-query": `case "$*" in
*Conffiles*) printf ' /etc/proftpd/proftpd.conf 00000000000000000000000000000000\n /etc/ssh/sshd_config %s\n /etc/old.conf 11111111111111111111111111111111 obsolete\n\n' "$(md5sum "$BRIDGE_ROOT/etc/ssh/sshd_config" | cut -d' ' -f1)" ;;
*Status-Abbrev*) printf 'ii  php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1\nii  libxml2 2.9.14+dfsg-1.3~ubuntu18.04.1+deb.sury.org+1\nii  curl 7.81.0-1ubuntu1.16\nii  zabbix-agent2 1:6.0.1-1+ubuntu20.04\nii  foo 1.0~ubuntu22.04.1~ppa1\nrc  gone 1.0+ubuntu18.04\n' ;;
esac`,
		"apt":       `[ "$*" = "list --installed" ] && printf 'Listing...\nlibssl1.1/now 1.1.1f-1ubuntu2.20 amd64 [installed,local]\ncurl/jammy-updates,now 7.81.0-1ubuntu1.16 amd64 [installed]\n'`,
		"dpkg":      `[ "$1" = --audit ] && printf 'The following packages are only half configured, probably due to problems\nconfiguring them the first time.  The configuration should be retried using\ndpkg --configure <package> or the configure menu option in dselect:\n proftpd-core         Versatile, security-focused FTP server\n\n'`,
		"systemctl": `printf 'fail2ban.service  loaded failed failed Fail2Ban Service\nlogrotate.service loaded failed failed Rotate log files\n'`,
	} {
		if err := os.WriteFile(filepath.Join(stubs, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "-c", releaseRiskScript(true))
	cmd.Env = []string{"PATH=" + stubs + ":" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, secret := range []string{"hunter2", "s3cret", "IdentLookups", "PasswordAuthentication", root} {
		if strings.Contains(string(out), secret) {
			t.Fatalf("%q left the VM:\n%s", secret, out)
		}
	}
	r := parseReleaseRisks(string(out))
	if r == nil {
		t.Fatalf("no result:\n%s", out)
	}
	check := func(name string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v, want %#v\n%s", name, got, want, out)
		}
	}
	check("foreign", r.ForeignPackages, []string{"libssl1.1 1.1.1f-1ubuntu2.20", "libxml2 2.9.14+dfsg-1.3~ubuntu18.04.1+deb.sury.org+1",
		"php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1", "zabbix-agent2 1:6.0.1-1+ubuntu20.04"})
	check("disabled", r.DisabledSources, []string{"/etc/apt/sources.list.d/off.sources", "/etc/apt/sources.list.d/ondrej-php.list",
		"/etc/apt/sources.list.d/zabbix.list.distUpgrade"})
	check("conffiles", r.ModifiedConffiles, []string{"/etc/proftpd/proftpd.conf"})
	check("dist", r.PendingDistFiles, []string{"/etc/logrotate.conf.dpkg-dist", "/etc/proftpd/modules.conf.ucf-dist"})
	check("failed", r.FailedUnits, []string{"fail2ban.service", "logrotate.service"})
	check("audit", r.DpkgBroken, []string{"proftpd-core"})
	check("listen", r.ApacheListen, []int{80, 443})
	states := map[string]string{}
	for _, h := range r.PHPHandlers {
		states[h.Target] = h.State + " " + h.Runs + " " + h.Dir
	}
	lockedState := "unknown  " // can't see into the directory
	if os.Geteuid() == 0 {
		lockedState = "missing /usr/bin/php-cgi7.4 "
	}
	check("handlers", states, map[string]string{
		"/home/sasc/fcgi-bin/php7.2.fcgi":   "missing /usr/bin/php-cgi7.2 /home/sasc/public_html",
		"/run/php/php8.1-fpm.sock":          "missing  ",
		"/home/novo/fcgi-bin/php8.4.fcgi":   "ok /usr/bin/php-cgi8.4 ",
		"/home/gone/fcgi-bin/php5.fcgi":     "missing  ",
		"/home/locked/fcgi-bin/php7.4.fcgi": lockedState,
	})

	// The warnings, with Apache's own view of its vhosts: sasc's :444 is
	// never served (no Listen 444); a container's Apache is not this one.
	info := &VMInfo{ReleaseRisks: r, WebServers: []WebServer{
		{Kind: "apache", Sites: []WebSite{{Names: []string{"sasc.example.gov.br"}, Listen: []int{444}}, {Names: []string{"ok.example"}, Listen: []int{80}}}},
		{Kind: "apache", Container: "app", Sites: []WebSite{{Names: []string{"c.example"}, Listen: []int{8080}}}},
	}}
	got := map[string]RiskWarning{}
	for _, w := range deriveReleaseWarnings(info) {
		got[w.Code] = w
	}
	if len(got) != 8 || got["foreign_packages"].Count != 4 || got["php_handler_missing"].Count != 3 ||
		got["vhost_port_unlistened"].Detail != "sasc.example.gov.br:444" || got["dpkg_broken"].Detail != "proftpd-core" ||
		!strings.Contains(got["php_handler_missing"].Detail, "/home/sasc/fcgi-bin/php7.2.fcgi → /usr/bin/php-cgi7.2") {
		t.Fatalf("warnings = %+v", got)
	}
	if deriveReleaseWarnings(&VMInfo{}) != nil || len(deriveReleaseWarnings(&VMInfo{ReleaseRisks: &ReleaseRisks{}})) != 0 {
		t.Fatal("nothing found: no warnings")
	}
}
