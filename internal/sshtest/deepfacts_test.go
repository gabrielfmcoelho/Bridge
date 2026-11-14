package sshtest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// TestParseDeepFacts drives the parser with synthetic output: patch markers,
// flows whose tokens carry passwords (a URL and an scp login), and connections
// that must be filtered (inbound by a listening local port, loopback peer).
// It asserts the stale cutoff, the credential stripping, and — the privacy
// guarantee — that no fixture password survives into the stored struct.
func TestParseDeepFacts(t *testing.T) {
	// Freeze "now" so the 14-day stale cutoff is deterministic.
	const now = int64(1_800_000_000)
	old := nowUnix
	nowUnix = func() int64 { return now }
	t.Cleanup(func() { nowUnix = old })

	idx := now - 40*86400 // 40 days stale
	raw := strings.Join([]string{
		"@@pm apt",
		"@@idx " + itoa(idx),
		"@@upg " + itoa(now-2*86400),
		"@@auto unattended-upgrades 1",
		"@@upgradable 5",
		"@@script " + b64("/root/bkp.sh"),
		"@@flow rsync " + b64("root@vps3.panet.com.br:/bkp/concursos/"),
		"@@flow curl " + b64("https://user:s3cret@api.example.com/upload"),
		"@@flow scp " + b64("deploy:hunter2@host.example.com:/srv"),
		"@@script " + b64("/root/noflows.sh"),
		"@@conns",
		`0 0 10.0.122.19:54321 45.233.130.110:22 users:(("rsync",pid=1,fd=3))`,
		`0 0 10.0.122.19:443 1.2.3.4:55000 users:(("apache2",pid=2,fd=4))`, // inbound: local 443 listens
		`0 0 127.0.0.1:5000 127.0.0.1:6379 users:(("redis",pid=3,fd=5))`,   // loopback peer
		`0 0 10.0.122.19:40000 10.0.100.54:10051 users:(("zabbix_agent2",pid=4,fd=6))`,
	}, "\n")

	df := parseDeepFacts(raw, map[int]bool{443: true})
	if df == nil || df.Patch == nil || df.Outbound == nil {
		t.Fatalf("empty result: %+v", df)
	}

	p := df.Patch
	if p.Manager != "apt" || p.Upgradable != 5 || !p.AutoUpdatesOn || p.AutoUpdates != "unattended-upgrades" {
		t.Fatalf("patch = %+v", p)
	}
	if !p.Stale {
		t.Fatalf("expected stale: idx %d is 40 days before %d with auto-updates on", p.LastIndexUpdate, now)
	}

	wantFlows := []ScriptFlow{
		{Script: "/root/bkp.sh", Program: "rsync", Target: "root@vps3.panet.com.br:/bkp/concursos/"},
		{Script: "/root/bkp.sh", Program: "curl", Target: "https://api.example.com/upload"},
		{Script: "/root/bkp.sh", Program: "scp", Target: "deploy@host.example.com:/srv"},
	}
	if !reflect.DeepEqual(df.Outbound.Scripts, wantFlows) {
		t.Fatalf("flows = %+v", df.Outbound.Scripts)
	}

	wantConns := []OutboundConn{
		{Peer: "45.233.130.110", Port: 22, Process: "rsync", Public: true},
		{Peer: "10.0.100.54", Port: 10051, Process: "zabbix_agent2", Public: false},
	}
	if !reflect.DeepEqual(df.Outbound.Connections, wantConns) {
		t.Fatalf("conns = %+v", df.Outbound.Connections)
	}

	// Privacy: no fixture secret may reach the stored struct.
	blob, _ := json.Marshal(df)
	for _, secret := range []string{"s3cret", "hunter2"} {
		if strings.Contains(string(blob), secret) {
			t.Fatalf("%q leaked into DeepFacts: %s", secret, blob)
		}
	}
}

func TestParseDeepFactsNotRun(t *testing.T) {
	if parseDeepFacts("", nil) != nil {
		t.Fatal("no markers: the probe didn't run, want nil")
	}
	if parseDeepFacts("some unrelated output\n", nil) != nil {
		t.Fatal("no @@pm/@@conns: want nil")
	}
}

// TestDeepFactsScriptRuns runs the real root script against a fake root with
// stubbed apt-get / ss, an apt layout and a backup script that embeds a
// password. It proves the shell side never emits the password and that the
// parsed facts (last index update, upgradable, the masked rsync/curl target,
// the outbound peer) are correct.
func TestDeepFactsScriptRuns(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string, mode os.FileMode) string {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// apt layout: a success stamp dated to a fixed old time, a history with an
	// upgrade block, and auto-upgrades turned on.
	stamp := write("var/lib/apt/periodic/update-success-stamp", "", 0o644)
	stampTime := time.Date(2024, 1, 8, 6, 0, 0, 0, time.UTC)
	if err := os.Chtimes(stamp, stampTime, stampTime); err != nil {
		t.Fatal(err)
	}
	write("var/log/apt/history.log",
		"Start-Date: 2024-01-04  06:00:00\nCommandline: apt-get upgrade\nEnd-Date: 2024-01-04  06:17:32\n", 0o644)
	write("etc/apt/apt.conf.d/20auto-upgrades",
		"APT::Periodic::Update-Package-Lists \"1\";\nAPT::Periodic::Unattended-Upgrade \"1\";\n", 0o644)

	// A backup script a cron job runs, with an embedded password that must stay
	// on the VM: only the host+path of its destinations may leave.
	write("root/bkp.sh", "#!/bin/sh\n"+
		"rsync -avz /home/ root@vps3.panet.com.br:/bkp/concursos/\n"+
		"curl -s https://admin:s3cret@api.example.com/upload\n"+
		"mysqldump -phunter2 -h db.internal concursos > /tmp/d.sql\n", 0o755)

	stubs := t.TempDir()
	for name, body := range map[string]string{
		// apt-get -s upgrade → two Inst lines (the upgradable count).
		"apt-get": "printf 'Inst libc6 [1] (2 Ubuntu)\\nInst curl [7] (7 Ubuntu)\\n'",
		// ss established: one public outbound (rsync), one loopback (dropped),
		// one inbound on a listening port (dropped by Go), one private peer.
		"ss": `cat <<'EOF'
0 0 10.0.122.19:54321 45.233.130.110:22 users:(("rsync",pid=1,fd=3))
0 0 127.0.0.1:5000 127.0.0.1:6379 users:(("redis",pid=2,fd=4))
0 0 10.0.122.19:443 1.2.3.4:40000 users:(("apache2",pid=3,fd=5))
0 0 10.0.122.19:40000 10.0.100.54:10051 users:(("zabbix_agent2",pid=4,fd=6))
EOF`,
	} {
		if err := os.WriteFile(filepath.Join(stubs, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("sh", "-c", deepFactsScript(true, []string{"/root/bkp.sh"}))
	cmd.Env = []string{"PATH=" + stubs + ":" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, secret := range []string{"s3cret", "hunter2", root} {
		if strings.Contains(string(out), secret) {
			t.Fatalf("%q left the VM:\n%s", secret, out)
		}
	}

	df := parseDeepFacts(string(out), map[int]bool{443: true})
	if df == nil || df.Patch == nil || df.Outbound == nil {
		t.Fatalf("no result:\n%s", out)
	}
	if df.Patch.Manager != "apt" || df.Patch.LastIndexUpdate != stampTime.Unix() {
		t.Errorf("patch idx = %+v want %d\n%s", df.Patch, stampTime.Unix(), out)
	}
	if df.Patch.Upgradable != 2 {
		t.Errorf("upgradable = %d want 2\n%s", df.Patch.Upgradable, out)
	}
	if !df.Patch.AutoUpdatesOn {
		t.Errorf("auto-updates should be on\n%s", out)
	}
	wantFlows := []ScriptFlow{
		{Script: "/root/bkp.sh", Program: "rsync", Target: "root@vps3.panet.com.br:/bkp/concursos/"},
		{Script: "/root/bkp.sh", Program: "curl", Target: "https://api.example.com/upload"},
	}
	if !reflect.DeepEqual(df.Outbound.Scripts, wantFlows) {
		t.Errorf("flows = %+v\n%s", df.Outbound.Scripts, out)
	}
	wantConns := []OutboundConn{
		{Peer: "45.233.130.110", Port: 22, Process: "rsync", Public: true},
		{Peer: "10.0.100.54", Port: 10051, Process: "zabbix_agent2", Public: false},
	}
	if !reflect.DeepEqual(df.Outbound.Connections, wantConns) {
		t.Errorf("conns = %+v\n%s", df.Outbound.Connections, out)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// TestParseSiteTraffic drives the A2 parser with synthetic markers: the
// aggregate line, several path segments (sorted and capped), error categories,
// and the last-seen date. It also checks that per-day is derived from the
// distinct-day count.
func TestParseSiteTraffic(t *testing.T) {
	raw := strings.Join([]string{
		"@@site virtualmin " + b64("concursos.sead.pi.gov.br") + " " + b64("/var/log/virtualmin/concursos.sead.pi.gov.br_access_log") + " 0",
		"@@sa req=102 s2=70 s3=10 s4=15 s5=7 human=80 bot=22 priv=4 pub=98 days=30 last=20261113",
		"@@sp 50 arquivos",
		"@@sp 30 /",
		"@@sp 12 sasc",
		"@@se 637 not-found",
		"@@se 40 php-error",
		"@@site virtualmin " + b64("app.seadprev.pi.gov.br") + " " + b64("/var/log/virtualmin/app.seadprev.pi.gov.br_access_log") + " 1",
		"@@sa req=0 s2=0 s3=0 s4=0 s5=0 human=0 bot=0 priv=0 pub=0 days=0 last=0",
		"@@sites-done",
	}, "\n")

	sites := parseSiteTraffic(raw)
	if len(sites) != 2 {
		t.Fatalf("want 2 sites, got %d: %+v", len(sites), sites)
	}

	s := sites[0]
	if s.Site != "concursos.sead.pi.gov.br" || s.Server != "virtualmin" {
		t.Fatalf("site[0] identity = %+v", s)
	}
	if s.Requests != 102 || s.Status2xx != 70 || s.Status5xx != 7 || s.Human != 80 || s.Bot != 22 {
		t.Fatalf("site[0] aggregates = %+v", s)
	}
	if s.Days != 30 || s.PerDay != 3 { // 102 / 30 = 3
		t.Fatalf("site[0] per-day = %d (days %d)", s.PerDay, s.Days)
	}
	if s.LastSeen != time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("site[0] last-seen = %d", s.LastSeen)
	}
	wantPaths := []PathCount{{Path: "/arquivos", Count: 50}, {Path: "/", Count: 30}, {Path: "/sasc", Count: 12}}
	if !reflect.DeepEqual(s.TopPaths, wantPaths) {
		t.Fatalf("site[0] top paths = %+v", s.TopPaths)
	}
	wantErrs := []ErrorSig{{Category: "not-found", Count: 637}, {Category: "php-error", Count: 40}}
	if !reflect.DeepEqual(s.Errors, wantErrs) {
		t.Fatalf("site[0] errors = %+v", s.Errors)
	}

	if sites[1].Site != "app.seadprev.pi.gov.br" || sites[1].Requests != 0 || !sites[1].Partial {
		t.Fatalf("site[1] (the dead site) = %+v", sites[1])
	}
}

func TestParseSiteTrafficNotRun(t *testing.T) {
	if parseSiteTraffic("") != nil {
		t.Fatal("no markers: want nil")
	}
	if parseSiteTraffic("@@sites-done\n") != nil {
		t.Fatal("no sites: want nil")
	}
}

// TestSiteTrafficScriptRuns runs the real A2 script against a fake root holding
// a Virtualmin access + error log. The access log mixes 2xx/4xx/5xx, browser
// and bot User-Agents, and private and public client IPs — and every request
// carries a query string with a fake secret. The error log carries file paths
// and a client IP. The test asserts the aggregates are right AND — the whole
// point of this probe — that no client IP, query string, User-Agent or error
// message ever appears in the probe output or the parsed struct.
func TestSiteTrafficScriptRuns(t *testing.T) {
	// Freeze "now" so the 30-day window is deterministic (cutoff 2026-10-15).
	old := nowUnix
	nowUnix = func() int64 { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC).Unix() }
	t.Cleanup(func() { nowUnix = old })

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Combined log format. Mixed statuses, UAs, IPs; every URL has a query
	// string with a secret; the last line is outside the 30-day window.
	access := strings.Join([]string{
		`203.0.113.45 - - [13/Nov/2026:10:00:00 +0000] "GET /arquivos/edital.pdf?token=SECRET123 HTTP/1.1" 200 1234 "-" "Mozilla/5.0 (Windows NT 10.0) Chrome/120"`,
		`198.51.100.7 - - [13/Nov/2026:11:00:00 +0000] "GET /?debug=1 HTTP/1.1" 500 10 "-" "curl/7.68.0"`,
		`10.0.122.5 - - [12/Nov/2026:09:00:00 +0000] "GET /sasc/login?pw=hunter2 HTTP/1.1" 200 55 "-" "Mozilla/5.0 Firefox/120"`,
		`192.168.1.9 - - [12/Nov/2026:09:30:00 +0000] "POST /wp-admin/admin-ajax.php?action=leak HTTP/1.1" 404 0 "-" "Googlebot/2.1"`,
		`45.33.22.11 - - [13/Nov/2026:12:00:00 +0000] "GET /arquivos/anexo.pdf HTTP/1.1" 200 20 "-" "Mozilla/5.0 Safari/16"`,
		`8.8.8.8 - - [01/Jan/2020:00:00:00 +0000] "GET /old?k=v HTTP/1.1" 200 1 "-" "Mozilla/5.0"`,
		``,
	}, "\n")
	write("var/log/virtualmin/example.com_access_log", access)
	write("var/log/virtualmin/example.com_error_log", strings.Join([]string{
		`[Fri Nov 13 10:00:00.111 2026] [php:error] [pid 7] [client 203.0.113.45:5678] PHP Fatal error: Uncaught in /home/u/secret_path.php`,
		`[Fri Nov 13 10:01:00.222 2026] [core:error] [client 198.51.100.7:40] No space left on device: /var/www/html`,
		``,
	}, "\n"))

	cmd := exec.Command("sh", "-c", siteTrafficScript(true, siteCutoff()))
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	// The privacy guarantee: none of these may leave the VM.
	leaks := []string{
		"203.0.113.45", "198.51.100.7", "10.0.122.5", "192.168.1.9", "45.33.22.11", "8.8.8.8",
		"SECRET123", "hunter2", "token=", "debug=", "pw=", "action=leak", "?",
		"Mozilla", "Chrome", "curl", "Googlebot", "Safari",
		"secret_path", "/home/u", "/var/www/html", "edital", "anexo", "admin-ajax",
	}
	sites := parseSiteTraffic(string(out))
	blob, _ := json.Marshal(sites)
	for _, s := range leaks {
		if strings.Contains(string(out), s) {
			t.Fatalf("%q leaked into probe output:\n%s", s, out)
		}
		if strings.Contains(string(blob), s) {
			t.Fatalf("%q leaked into parsed struct: %s", s, blob)
		}
	}

	if len(sites) != 1 {
		t.Fatalf("want 1 site, got %d:\n%s", len(sites), out)
	}
	st := sites[0]
	if st.Site != "example.com" || st.Server != "virtualmin" {
		t.Fatalf("identity = %+v\n%s", st, out)
	}
	if st.Requests != 5 { // the 2020 line is outside the window
		t.Errorf("requests = %d want 5\n%s", st.Requests, out)
	}
	if st.Status2xx != 3 || st.Status4xx != 1 || st.Status5xx != 1 {
		t.Errorf("status classes = 2xx %d 4xx %d 5xx %d\n%s", st.Status2xx, st.Status4xx, st.Status5xx, out)
	}
	if st.Human != 3 || st.Bot != 2 {
		t.Errorf("human/bot = %d/%d want 3/2\n%s", st.Human, st.Bot, out)
	}
	if st.PublicIPs != 3 || st.PrivateIPs != 2 {
		t.Errorf("public/private = %d/%d want 3/2\n%s", st.PublicIPs, st.PrivateIPs, out)
	}
	if st.Days != 2 || st.LastSeen != time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("days %d last %d\n%s", st.Days, st.LastSeen, out)
	}
	if len(st.TopPaths) == 0 || st.TopPaths[0].Path != "/arquivos" || st.TopPaths[0].Count != 2 {
		t.Errorf("top path = %+v want /arquivos×2\n%s", st.TopPaths, out)
	}
	// Error categories come through as fixed labels only.
	gotErr := map[string]int{}
	for _, e := range st.Errors {
		gotErr[e.Category] = e.Count
	}
	if gotErr["php-error"] != 1 || gotErr["disk-full"] != 1 {
		t.Errorf("error categories = %+v want php-error:1 disk-full:1\n%s", st.Errors, out)
	}
}
