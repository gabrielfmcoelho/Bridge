package sshtest

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParseGitProbe(t *testing.T) {
	raw := "@@git\n" +
		"@@repo /opt/app\n" +
		"[x]\n" +
		"[remote \"origin\"]\n" +
		"url = https://gitlab.example.gov.br/sead/app.git\n" +
		"[x]\n" +
		"url = https://gitlab.example.gov.br/sead/submodule.git\n" + // submodule / insteadOf: not a remote
		"[remote \"github\"]\n" +
		"url = \"git@github.com:org/tool.git\"\n" +
		"url = git@github.com:org/tool.git\n" + // duplicate
		"@@repo /home/u/creds\n" +
		"[remote \"origin\"]\n" +
		"url = https://oauth2:glpat-SECRET@gitlab.example.gov.br/sead/creds.git\n" +
		"@@repo relative/bad\n" +
		"[remote \"origin\"]\n" +
		"url = https://x.example/y.git\n" +
		"@@gitend\n" +
		"@@CTR 0123456789ab api nginx:1.25\n" +
		"@@git\n" +
		"@@repo /app\n" +
		"[remote \"origin\"]\n" +
		"url = https://gitlab.example.gov.br/sead/api.git\n" +
		"@@gitend\n" +
		"@@CTR deadbeef0000 distroless gcr.io/distroless/static\n" + // no shell: never ran, not partial
		"@@CTR nothex! evil img\n" +
		"@@git\n" +
		"@@repo /pwn\n" +
		"[remote \"origin\"]\n" +
		"url = https://evil.example/x.git\n"
	got, complete := parseGitProbe(raw)
	if !complete {
		t.Fatal("every part that ran reached its end marker: want complete")
	}
	want := []GitRemote{
		{Path: "/opt/app", Name: "origin", URL: "https://gitlab.example.gov.br/sead/app.git"},
		{Path: "/opt/app", Name: "github", URL: "git@github.com:org/tool.git"},
		{Path: "/home/u/creds", Name: "origin", URL: "https://gitlab.example.gov.br/sead/creds.git"},
		{Container: "api", ContainerImage: "nginx:1.25", Path: "/app", Name: "origin", URL: "https://gitlab.example.gov.br/sead/api.git"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseGitProbe =\n%+v\nwant\n%+v", got, want)
	}
}

// nil means the probe never ran (its start marker is missing); a part that
// started but never reached @@gitend (find timed out, repo cap hit, container
// exec killed) is incomplete. Neither may delete what earlier scans found.
func TestParseGitProbeFailedPartialEmpty(t *testing.T) {
	if got, _ := parseGitProbe("sudo: a password is required\n"); got != nil {
		t.Fatalf("failed probe = %+v, want nil", got)
	}
	got, complete := parseGitProbe("@@git\n@@gitend\n")
	if got == nil || len(got) != 0 || !complete {
		t.Fatalf("probe with no repos = %#v complete=%v, want empty non-nil, complete", got, complete)
	}
	if _, complete := parseGitProbe("@@git\n@@repo /a\n[remote \"origin\"]\nurl = https://h.example/g/a.git\n"); complete {
		t.Fatal("host part without @@gitend must be incomplete")
	}
	if _, complete := parseGitProbe("@@git\n@@gitend\n@@CTR 0123456789ab slow img\n@@git\n"); complete {
		t.Fatal("container part without @@gitend must be incomplete")
	}
}

// Both variants are valid sh and carry the inner script; the root variant
// (run through runSudoCmd) retries nothing with sudo -n.
func TestGitProbeScript(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte(gitInnerScript))
	for _, asRoot := range []bool{false, true} {
		s := gitProbeScript(asRoot)
		if out, err := exec.Command("sh", "-n", "-c", s).CombinedOutput(); err != nil {
			t.Fatalf("root=%v sh -n: %v %s", asRoot, err, out)
		}
		for _, p := range []string{"{MAX}", "{B64}", "{CAP}", "{FALLBACK}", "{TO}"} {
			if strings.Contains(s, p) {
				t.Fatalf("root=%v placeholder %s left unreplaced", asRoot, p)
			}
		}
		if !strings.Contains(s, "printf %s "+b64+" | base64 -d") {
			t.Fatalf("root=%v container part does not carry the inner script", asRoot)
		}
		retries := strings.Contains(s, "SUDO=1") || strings.Contains(s, `D="sudo -n docker"`)
		if retries == asRoot {
			t.Fatalf("root=%v: sudo -n retries present = %v", asRoot, retries)
		}
	}
}

// A find killed by timeout(1) (exit 124) or a run past the 100-repo cap never
// prints @@gitend, so the scan can't mistake a cut list for the whole one.
func TestGitInnerScriptIncomplete(t *testing.T) {
	run := func(t *testing.T, roots string, env ...string) string {
		cmd := exec.Command("sh", "-c", gitInnerScript)
		cmd.Env = append(append(os.Environ(), "GITPROBE_ROOTS="+roots), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("script: %v\n%s", err, out)
		}
		return string(out)
	}
	t.Run("timeout", func(t *testing.T) {
		bin := t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "timeout"), []byte("#!/bin/sh\nexit 124\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		out := run(t, t.TempDir(), "PATH="+bin+":"+os.Getenv("PATH"))
		if _, complete := parseGitProbe(out); complete || !strings.Contains(out, "@@git") {
			t.Fatalf("timed-out find must start but not complete:\n%s", out)
		}
	})
	t.Run("cap", func(t *testing.T) {
		root := t.TempDir()
		for i := 0; i < 101; i++ {
			if err := os.MkdirAll(filepath.Join(root, "r"+strconv.Itoa(i), ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, complete := parseGitProbe(run(t, root)); complete {
			t.Fatal("101 checkouts must be incomplete")
		}
	})
}

// The inner script, run for real over a fake tree: only remote headers and
// url lines leave the machine, credentials stripped, other secrets never.
func TestGitInnerScriptRuns(t *testing.T) {
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
	write("srv/app/.git/config", `[core]
	bare = false
[remote "origin"]
	url = https://oauth2:glpat-SECRET@gitlab.example.gov.br/sead/app.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[http "https://deploy:hunter2@gitlab.example.gov.br/"]
	extraHeader = AUTHORIZATION: basic c2VjcmV0
[submodule "lib"]
	url = https://gitlab.example.gov.br/sead/lib.git
`)
	write("srv/app/node_modules/dep/.git/config", "[remote \"origin\"]\n\turl = https://example.com/dep.git\n")

	cmd := exec.Command("sh", "-c", gitInnerScript)
	cmd.Env = append(os.Environ(), "GITPROBE_ROOTS="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	s := string(out)
	for _, secret := range []string{"glpat-SECRET", "hunter2", "c2VjcmV0", "extraHeader", "fetch", "dep.git"} {
		if strings.Contains(s, secret) {
			t.Fatalf("output leaks %q:\n%s", secret, s)
		}
	}
	got, complete := parseGitProbe(s)
	if !complete {
		t.Fatalf("a finished run must be complete:\n%s", s)
	}
	want := []GitRemote{{Path: filepath.Join(root, "srv/app"), Name: "origin", URL: "https://gitlab.example.gov.br/sead/app.git"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remotes = %+v, want %+v\noutput:\n%s", got, want, s)
	}
}
