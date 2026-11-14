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
