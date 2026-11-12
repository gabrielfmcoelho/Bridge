package sshtest

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The script archives the include paths minus the excludes (names with
// spaces and quotes included) and reports tar's exit on stderr. The fixture
// lives on disk (not t.TempDir): /tmp is often tmpfs, which the script skips.
func TestBackupScript_Archives(t *testing.T) {
	root, err := os.MkdirTemp(".", "backup-fixture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	root, _ = filepath.Abs(root)
	for rel, body := range map[string]string{"etc/app.conf": "x=1", "data/keep me.txt": "k", "data/cache/big.bin": "c", "it's/f": "q"} {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	script := backupScript([]string{root}, []string{filepath.Join(root, "data/cache")})
	cmd := exec.Command("sh", "-c", script)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("script: %v %s", err, stderr.String())
	}
	if err := backupResult(nil, stderr.String()); err != nil {
		t.Fatalf("result: %v (stderr %q)", err, stderr.String())
	}

	// Decompress with whatever the script picked, then list.
	unz := "gzip -dc"
	if _, err := exec.LookPath("zstd"); err == nil {
		unz = "zstd -dc"
	}
	list := exec.Command("sh", "-c", unz+" | tar -tf -")
	list.Stdin = &out
	names, err := list.Output()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := string(names)
	for _, want := range []string{"etc/app.conf", "data/keep me.txt", "it's/f"} {
		if !strings.Contains(got, want) {
			t.Errorf("archive misses %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "big.bin") {
		t.Errorf("excluded path archived:\n%s", got)
	}

	// An exclude wins over an include beneath it: excluding docker's root
	// drops its volumes even when they are listed. To keep the volumes, the
	// exclude must name the other subdirectories instead (what the UI
	// suggests, frontend/src/lib/backupPreview.ts).
	docker := filepath.Join(root, "var/lib/docker")
	for _, rel := range []string{"volumes/v1/_data/db", "overlay2/l/layer"} {
		os.MkdirAll(filepath.Dir(filepath.Join(docker, rel)), 0o755)
		os.WriteFile(filepath.Join(docker, rel), []byte("x"), 0o644)
	}
	archive := func(include, exclude []string) string {
		t.Helper()
		var out bytes.Buffer
		cmd := exec.Command("sh", "-c", backupScript(include, exclude))
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		list := exec.Command("sh", "-c", unz+" | tar -tf -")
		list.Stdin = &out
		names, _ := list.Output()
		return string(names)
	}
	if got := archive([]string{root, filepath.Join(docker, "volumes")}, []string{docker}); strings.Contains(got, "volumes/v1") {
		t.Errorf("an include under an exclude was archived:\n%s", got)
	}
	got = archive([]string{root}, []string{filepath.Join(docker, "overlay2")})
	if !strings.Contains(got, "volumes/v1/_data/db") || strings.Contains(got, "overlay2/l/layer") {
		t.Errorf("volumes without the layers:\n%s", got)
	}
}

func TestBackupResult(t *testing.T) {
	cases := []struct {
		err    error
		stderr string
		want   string // "" = ok
	}{
		{nil, "@@tar_exit 0\n", ""},
		{nil, "@@tar_exit 1\n", ""}, // files changed while read
		{nil, "@@tar_exit 2\n", "tar exit 2"},
		{errors.New("exit 1"), "@@tar_exit 0\n", "compression failed"},
		{errors.New("exit 1"), "sudo: a password is required\n", "store the host's password"},
		{errors.New("exit 127"), "sh: tar: not found", "did not run"},
		{nil, "", "did not report"},
	}
	for _, c := range cases {
		err := backupResult(c.err, c.stderr)
		if (c.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%v %q = %v, want %q", c.err, c.stderr, err, c.want)
		}
	}
}
