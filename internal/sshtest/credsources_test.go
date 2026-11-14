package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// credFakeHost returns a PATH holding only the base tools the probe uses, so
// the script runs for real against a planted BRIDGE_ROOT without touching the
// machine. The probe uses POSIX sh plus stat/head/base64/tr/cat (no awk).
func credFakeHost(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range []string{"sh", "stat", "head", "base64", "tr", "cat"} {
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s not available", tool)
		}
		if err := os.Symlink(p, filepath.Join(dir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func plant(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runCredProbe(t *testing.T, root string, roots, files []string) []credFile {
	t.Helper()
	bin := credFakeHost(t)
	cmd := exec.Command(filepath.Join(bin, "sh"), "-c", credProbeScript(roots, files))
	cmd.Env = []string{"PATH=" + bin, "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "@@CSBEGIN") {
		t.Fatalf("no @@CSBEGIN in output:\n%s", out)
	}
	return parseCredProbe(string(out))
}

func TestCredProbeFakeRoot(t *testing.T) {
	root := t.TempDir()
	plant(t, root, "etc/mysql/debian.cnf", "[client]\nuser=debian-sys-maint\npassword=Zxy123abc\n")
	plant(t, root, "root/.my.cnf", "[client]\nuser=root\npassword=RootMyCnf1\n")
	plant(t, root, "root/.ssh/id_rsa", "-----BEGIN OPENSSH PRIVATE KEY-----\nAAAA\n-----END OPENSSH PRIVATE KEY-----\n")
	plant(t, root, "root/.ssh/id_rsa.pub", "ssh-rsa AAAAB3Nz root@host\n") // must be skipped
	plant(t, root, "var/www/app/.env", "DB_PASSWORD=envpw\n")
	plant(t, root, "var/www/app/application/config/database.php",
		"<?php\n$db['default']['username']='root';\n$db['default']['password']='ci3pw';\n")
	plant(t, root, "opt/bkp.sh", "#!/bin/sh\nPGPASSWORD=pgpw pg_dump x\n")

	files := runCredProbe(t, root, []string{"/var/www/app"}, []string{"/opt/bkp.sh"})
	byPath := map[string]credFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	want := map[string]string{
		"/etc/mysql/debian.cnf": "password=Zxy123abc",
		"/root/.my.cnf":         "password=RootMyCnf1",
		"/root/.ssh/id_rsa":     "OPENSSH PRIVATE KEY",
		"/var/www/app/.env":     "DB_PASSWORD=envpw",
		"/var/www/app/application/config/database.php": "ci3pw",
		"/opt/bkp.sh": "PGPASSWORD=pgpw",
	}
	for p, needle := range want {
		f, ok := byPath[p]
		if !ok {
			t.Fatalf("missing %s; got %v", p, keysOf(byPath))
		}
		if !strings.Contains(f.Content, needle) {
			t.Fatalf("%s content = %q, want to contain %q", p, f.Content, needle)
		}
		if f.Owner == "" {
			t.Fatalf("%s: no owner", p)
		}
	}
	if _, ok := byPath["/root/.ssh/id_rsa.pub"]; ok {
		t.Fatal("a .pub key must never be streamed")
	}

	// End-to-end: the probe output detects into the expected credentials.
	hits := detectCreds(files, map[string]bool{"/opt/bkp.sh": true}, nil)
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.Detector] = true
	}
	for _, d := range []string{"mysql_debian_cnf", "mysql_root_mycnf", "dotenv", "ci3_database", "cron_embedded", "root_ssh_keys"} {
		if !seen[d] {
			t.Errorf("detector %s produced no hit from the probe output", d)
		}
	}
}

// A file over the per-file cap is streamed truncated, flagged, and still parses
// its head.
func TestCredProbeTruncates(t *testing.T) {
	root := t.TempDir()
	big := "[client]\nuser=root\npassword=HeadSecret1\n" + strings.Repeat("# pad\n", 60000) // > 256 KiB
	plant(t, root, "root/.my.cnf", big)
	files := runCredProbe(t, root, nil, nil)
	var f credFile
	for _, x := range files {
		if x.Path == "/root/.my.cnf" {
			f = x
		}
	}
	if !f.Truncated {
		t.Fatalf("large file not flagged truncated (len=%d)", len(f.Content))
	}
	if len(f.Content) > credPerFileCap {
		t.Fatalf("content len %d exceeds cap %d", len(f.Content), credPerFileCap)
	}
	if !strings.Contains(f.Content, "password=HeadSecret1") {
		t.Fatal("head of the file was lost")
	}
}

// Without a stored root password the probe doesn't run (it reads 0600 root
// files): it returns scanned=false and touches no client.
func TestCollectCredSourcesNeedsRoot(t *testing.T) {
	if files, scanned := collectCredSources(nil, "", nil, nil); scanned || files != nil {
		t.Fatalf("no password: scanned=%v files=%v, want false/nil", scanned, files)
	}
}

func keysOf(m map[string]credFile) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
