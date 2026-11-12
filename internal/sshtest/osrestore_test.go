package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreMembers(t *testing.T) {
	got, err := RestoreMembers([]string{"/etc/nginx/", "/opt/app data"})
	if err != nil || strings.Join(got, "|") != "etc/nginx|opt/app data" {
		t.Fatalf("members = %q, %v", got, err)
	}
	for _, bad := range []string{"etc", "/", "//", "/etc/../root", "/./etc"} {
		if _, err := RestoreMembers([]string{bad}); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

// restoreHost is a fake root plus a curl stub that serves archive and records
// what it got on stdin (the link must arrive there, never as an argument).
func restoreHost(t *testing.T, archive string, curlExit int) (root, path, seen string) {
	t.Helper()
	root = t.TempDir()
	seen = filepath.Join(t.TempDir(), "curl")
	path = fakeHost(t, map[string]string{
		"curl": `echo "args: $*" > ` + seen + `; cat >> ` + seen + `; cat ` + archive + `; exit ` + map[bool]string{true: "0", false: "22"}[curlExit == 0],
	})
	return
}

// archive builds a tar.gz like a backup's: members without the leading /.
func archive(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	for rel, body := range map[string]string{"etc/nginx/nginx.conf": "worker 4", "etc/hosts": "127.0.0.1 old", "opt/app/v.txt": "1.0"} {
		os.MkdirAll(filepath.Join(src, filepath.Dir(rel)), 0o755)
		os.WriteFile(filepath.Join(src, rel), []byte(body), 0o644)
	}
	out := filepath.Join(t.TempDir(), "b.tar.gz")
	if b, err := exec.Command("sh", "-c", "tar -cf - -C "+src+" etc opt | gzip > "+out).CombinedOutput(); err != nil {
		t.Fatalf("archive: %v %s", err, b)
	}
	return out
}

func runRestore(t *testing.T, root, path, script string) (OSUpdateReport, string, error) {
	t.Helper()
	cmd := exec.Command(filepath.Join(path, "sh"), "-c", script)
	cmd.Env = []string{"PATH=" + path, "BRIDGE_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	return ParseOSUpdateLog(string(out)), string(out), err
}

const link = "http://10.0.122.76:9000/ops/b.tar.gz?X-Amz-Signature=abc&X-Amz-Expires=21600"

// Staging: the whole backup lands in the root-only review folder; nothing
// else is touched; the link reaches curl on stdin only.
func TestRestoreScript_Staging(t *testing.T) {
	root, path, seen := restoreHost(t, archive(t), 0)
	os.MkdirAll(filepath.Join(root, "etc"), 0o755)
	os.WriteFile(filepath.Join(root, "etc/hosts"), []byte("127.0.0.1 new"), 0o644)
	script, err := RestoreScript(9, RestoreStaging, "bridge/os-backups/vm1/x-run3.tar.gz", link, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, out, err := runRestore(t, root, path, script)
	if err != nil || r.Error != "" {
		t.Fatalf("staging: %v %+v\n%s", err, r, out)
	}
	dir := filepath.Join(root, "var/lib/bridge-restore/run-9")
	if b, _ := os.ReadFile(filepath.Join(dir, "etc/nginx/nginx.conf")); string(b) != "worker 4" {
		t.Fatalf("staged nginx.conf = %q\n%s", b, out)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "etc/hosts")); string(b) != "127.0.0.1 new" {
		t.Fatal("staging must not touch the live files")
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("review folder mode = %v", fi.Mode().Perm())
	}
	got, _ := os.ReadFile(seen)
	if !strings.Contains(string(got), `url = "`+link+`"`) || strings.Contains(strings.SplitN(string(got), "\n", 2)[0], "Signature") {
		t.Fatalf("curl saw %q: the link must come on stdin, not as an argument", got)
	}
	if strings.Contains(out, "Signature") {
		t.Fatal("the link leaked into the log")
	}
}

// Paths in place: only the chosen paths are overwritten.
func TestRestoreScript_Paths(t *testing.T) {
	root, path, _ := restoreHost(t, archive(t), 0)
	for rel, body := range map[string]string{"etc/hosts": "127.0.0.1 new", "etc/nginx/nginx.conf": "worker 8", "opt/app/v.txt": "2.0"} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755)
		os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644)
	}
	members, _ := RestoreMembers([]string{"/etc/nginx"})
	script, _ := RestoreScript(9, RestorePaths, "k.tar.gz", link, members)
	if r, out, err := runRestore(t, root, path, script); err != nil || r.Error != "" {
		t.Fatalf("paths: %v %+v\n%s", err, r, out)
	}
	for rel, want := range map[string]string{"etc/nginx/nginx.conf": "worker 4", "etc/hosts": "127.0.0.1 new", "opt/app/v.txt": "2.0"} {
		if b, _ := os.ReadFile(filepath.Join(root, rel)); string(b) != want {
			t.Errorf("%s = %q, want %q", rel, b, want)
		}
	}

	// A path the backup doesn't have fails the restore.
	members, _ = RestoreMembers([]string{"/srv/nothing"})
	script, _ = RestoreScript(9, RestorePaths, "k.tar.gz", link, members)
	if r, out, err := runRestore(t, root, path, script); err == nil || !strings.Contains(r.Error, "tar exit") {
		t.Fatalf("missing path: %v %+v\n%s", err, r, out)
	}
}

// A failed download (expired link) is reported as such.
func TestRestoreScript_DownloadFails(t *testing.T) {
	root, path, _ := restoreHost(t, "/dev/null", 22)
	script, _ := RestoreScript(9, RestoreStaging, "k.tar.gz", link, nil)
	if r, out, err := runRestore(t, root, path, script); err == nil || !strings.Contains(r.Error, "download failed") {
		t.Fatalf("download: %v %+v\n%s", err, r, out)
	}
	if _, err := RestoreScript(9, RestorePaths, "k.tar.gz", link, nil); err == nil {
		t.Fatal("in place without paths must be refused")
	}
	if _, err := RestoreScript(9, "full", "k.tar.gz", link, nil); err == nil {
		t.Fatal("unknown mode must be refused")
	}
}

// The run's script (which may carry a link) is root-only on the host.
func TestOSUpdateLauncher_ScriptIsPrivate(t *testing.T) {
	dir := t.TempDir()
	launcher := strings.ReplaceAll(osUpdateLauncher(5, "exit 0\n"), osUpdateDir, dir)
	if out, err := exec.Command("sh", "-c", launcher).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	fi, err := os.Stat(filepath.Join(dir, "run-5.sh"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("script mode = %v, %v", fi.Mode().Perm(), err)
	}
}
