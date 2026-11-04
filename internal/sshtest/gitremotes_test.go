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
		"[remote \"scp\"]\n" +
		"url = deploy:hunter2@gitlab.example.gov.br:sead/creds.git\n" + // scp-style with a password
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
		{Path: "/home/u/creds", Name: "scp", URL: "deploy@gitlab.example.gov.br:sead/creds.git"},
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
	// fakeTimeout runs the command, then exits rc — except for the script's
	// "timeout 1 true" usability check, which it passes.
	fakeTimeout := func(t *testing.T, rc string) string {
		bin := t.TempDir()
		script := "#!/bin/sh\nshift\n\"$@\"\n[ \"$1\" = true ] && exit 0\nexit " + rc + "\n"
		if err := os.WriteFile(filepath.Join(bin, "timeout"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return "PATH=" + bin + ":" + os.Getenv("PATH")
	}
	for name, rc := range map[string]string{"GNU timeout": "124", "busybox timeout": "143", "killed": "137"} {
		t.Run(name, func(t *testing.T) {
			out := run(t, t.TempDir(), fakeTimeout(t, rc))
			if _, complete := parseGitProbe(out); complete || !strings.Contains(out, "@@git") {
				t.Fatalf("find ended by exit %s must start but not complete:\n%s", rc, out)
			}
		})
	}
	t.Run("forged rc line", func(t *testing.T) {
		// A directory name can hold newlines: one that spells "@@rc 0" must
		// not pass for find's own exit status.
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "x\n@@rc 0\nz", ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, complete := parseGitProbe(run(t, root, fakeTimeout(t, "124"))); complete {
			t.Fatal("a forged @@rc line made a timed-out walk complete")
		}
	})
	t.Run("unusable timeout", func(t *testing.T) {
		// Old busybox only takes "timeout -t N": the walk runs without one
		// (the session deadline bounds it) and still completes.
		bin := t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "timeout"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "app", ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "app", ".git", "config"), []byte("[remote \"origin\"]\n\turl = https://h.example/g/app.git\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, complete := parseGitProbe(run(t, root, "PATH="+bin+":"+os.Getenv("PATH")))
		if !complete || len(got) != 1 {
			t.Fatalf("remotes = %+v complete=%v, want the one checkout, complete", got, complete)
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

// The container loop has a time budget: a container past it is framed as
// started but never finished, so the list is incomplete instead of silently
// short — and the whole probe stays inside the browser's scan timeout.
func TestGitProbeContainerBudget(t *testing.T) {
	for name, c := range map[string]struct {
		date     string // fake date(1): the loop's start, then every later call
		complete bool
	}{
		"in budget":   {date: "echo 0", complete: true},
		"over budget": {date: "if [ -f \"$DATE_STATE\" ]; then echo 1000; else : > \"$DATE_STATE\"; echo 0; fi", complete: false},
	} {
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			for f, body := range map[string]string{
				"docker": "#!/bin/sh\ncase \"$1\" in ps) echo '0123456789ab one img:1'; echo '0123456789ac two img:2';; exec) exec sh;; esac\n",
				"date":   "#!/bin/sh\n" + c.date + "\n",
			} {
				if err := os.WriteFile(filepath.Join(bin, f), []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("sh", "-c", gitProbeScript(true))
			cmd.Env = append(os.Environ(), "GITPROBE_ROOTS="+t.TempDir(), "DATE_STATE="+filepath.Join(t.TempDir(), "s"),
				"PATH="+bin+":"+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("script: %v\n%s", err, out)
			}
			remotes, complete := parseGitProbe(string(out))
			if remotes == nil || complete != c.complete || strings.Count(string(out), "@@CTR ") != 2 {
				t.Fatalf("remotes=%v complete=%v (want %v)\n%s", remotes, complete, c.complete, out)
			}
		})
	}
}

// After a failed root attempt the unprivileged walk can't enter 0700 homes,
// so its list is never complete (nothing gets pruned on it).
func TestGitFallbackAfterFailedRoot(t *testing.T) {
	if _, complete := fallbackGitResult("@@git\n@@gitend\n", true); complete {
		t.Fatal("fallback after a failed root run must be incomplete")
	}
	if _, complete := fallbackGitResult("@@git\n@@gitend\n", false); !complete {
		t.Fatal("unprivileged run with no password to try is complete")
	}
}

// /usr/share holds nginx's default docroot (RHEL, upstream packages), so it
// is walked; btrfs snapshots would only repeat every checkout.
func TestGitProbePrunes(t *testing.T) {
	if strings.Contains(gitInnerScript, "-path /usr/share ") || !strings.Contains(gitInnerScript, "-path /.snapshots ") {
		t.Fatal("prune list: walk /usr/share, skip /.snapshots")
	}
}
