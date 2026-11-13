package sshtest

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseGitProbe(t *testing.T) {
	raw := "@@git\n" +
		"@@repo /opt/app\n" +
		"@@head 0123456789abcdef0123456789abcdef01234567 main 1700001000 pull\n" +
		"[x]\n" +
		"[remote \"origin\"]\n" +
		"url = https://gitlab.example.gov.br/sead/app.git\n" +
		"[x]\n" +
		"url = https://gitlab.example.gov.br/sead/submodule.git\n" + // submodule / insteadOf: not a remote
		"[remote \"github\"]\n" +
		"url = \"git@github.com:org/tool.git\"\n" +
		"url = git@github.com:org/tool.git\n" + // duplicate
		"@@repo /home/u/creds\n" +
		"@@head NOTHEX main;rm 99999999999 rm\n" + // forged: every field rejected
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
		"@@head abababababababababababababababababababababababababababababababab - - -\n" + // detached HEAD, sha256 repo, no pull known
		"[remote \"origin\"]\n" +
		"url = https://gitlab.example.gov.br/sead/api.git\n" +
		"@@gitend\n" +
		"@@CTR deadbeef0000 distroless gcr.io/distroless/static\n@@exit 126\n" + // no shell: never ran, not partial
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
		{Path: "/opt/app", Name: "origin", URL: "https://gitlab.example.gov.br/sead/app.git", Commit: "0123456789abcdef0123456789abcdef01234567", Branch: "main", PulledAt: 1700001000, PullKind: "pull"},
		{Path: "/opt/app", Name: "github", URL: "git@github.com:org/tool.git", Commit: "0123456789abcdef0123456789abcdef01234567", Branch: "main", PulledAt: 1700001000, PullKind: "pull"},
		{Path: "/home/u/creds", Name: "origin", URL: "https://gitlab.example.gov.br/sead/creds.git"},
		{Path: "/home/u/creds", Name: "scp", URL: "deploy@gitlab.example.gov.br:sead/creds.git"},
		{Container: "api", ContainerImage: "nginx:1.25", Path: "/app", Name: "origin", URL: "https://gitlab.example.gov.br/sead/api.git", Commit: "abababababababababababababababababababababababababababababababab"},
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
		s := gitProbeScript(asRoot, budgets[CaptureSync])
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
[REMOTE "Up"]
	URL = https://deploy:tok-UP@gitlab.example.gov.br/sead/up.git
`)
	write("srv/app/node_modules/dep/.git/config", "[remote \"origin\"]\n\turl = https://example.com/dep.git\n")
	// HEAD on a branch file; the reflog's last pull is newer than FETCH_HEAD.
	// Its lines carry the author and the clone URL: neither may leave.
	write("srv/app/.git/HEAD", "ref: refs/heads/main\n")
	write("srv/app/.git/refs/heads/main", "0123456789abcdef0123456789abcdef01234567\n")
	write("srv/app/.git/logs/HEAD", "0000000000000000000000000000000000000000 aaaa Dev Person <dev@example.com> 1700000000 -0300\tclone: from https://oauth2:glpat-SECRET@gitlab.example.gov.br/sead/app.git\n"+
		"aaaa bbbb Dev Person <dev@example.com> 1700000500 -0300\tcommit: x\n"+
		"bbbb cccc Dev Person <dev@example.com> 1700001000 -0300\tpull: Fast-forward\n")
	write("srv/app/.git/FETCH_HEAD", "x\n")
	touch := func(rel string, unix int64) {
		if err := os.Chtimes(filepath.Join(root, rel), time.Unix(unix, 0), time.Unix(unix, 0)); err != nil {
			t.Fatal(err)
		}
	}
	touch("srv/app/.git/FETCH_HEAD", 1690000000)
	// Branch only in packed-refs, no reflog: the last fetch is the update.
	write("srv/api/.git/config", "[remote \"origin\"]\n\turl = https://gitlab.example.gov.br/sead/api.git\n")
	write("srv/api/.git/HEAD", "ref: refs/heads/dev\n")
	write("srv/api/.git/packed-refs", "# pack-refs with: peeled\nfefefefefefefefefefefefefefefefefefefefe refs/heads/dev\n")
	write("srv/api/.git/FETCH_HEAD", "x\n")
	touch("srv/api/.git/FETCH_HEAD", 1700002000)
	// A ref pointing out of .git: the probe may run as root, so it must not
	// read (and report) another file through it.
	write("secret.txt", "s3cr3t-line\n")
	write("srv/evil/.git/config", "[remote \"origin\"]\n\turl = https://gitlab.example.gov.br/sead/evil.git\n")
	write("srv/evil/.git/HEAD", "ref: refs/heads/../../../../secret.txt\n")
	// FIFOs where HEAD, the ref, packed-refs and the reflog belong: reading one
	// would block the walk (or, as /dev/stdin, eat the list of checkouts).
	fifo := func(rel string) {
		if out, err := exec.Command("mkfifo", filepath.Join(root, rel)).CombinedOutput(); err != nil {
			t.Fatalf("mkfifo: %v %s", err, out)
		}
	}
	write("srv/fifo-a/.git/config", "[remote \"origin\"]\n\turl = https://gitlab.example.gov.br/sead/fa.git\n")
	write("srv/fifo-a/.git/logs/.keep", "")
	fifo("srv/fifo-a/.git/HEAD")
	fifo("srv/fifo-a/.git/logs/HEAD")
	write("srv/fifo-b/.git/config", "[remote \"origin\"]\n\turl = https://gitlab.example.gov.br/sead/fb.git\n")
	write("srv/fifo-b/.git/HEAD", "ref: refs/heads/main\n")
	write("srv/fifo-b/.git/refs/heads/.keep", "")
	fifo("srv/fifo-b/.git/refs/heads/main")
	fifo("srv/fifo-b/.git/packed-refs")
	// Only a numeric time and the words pull / clone may leave the reflog.
	write("srv/fifo-b/.git/logs/HEAD", "a b N <n@x> 12abc -0300\tpullEVIL: x\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", gitInnerScript)
	cmd.WaitDelay = time.Second // a child blocked on a FIFO keeps the pipe open
	cmd.Env = append(os.Environ(), "GITPROBE_ROOTS="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	s := string(out)
	for _, secret := range []string{"glpat-SECRET", "hunter2", "c2VjcmV0", "extraHeader", "fetch =", "dep.git", "tok-UP", "dev@example.com", "Dev Person", "s3cr3t", "EVIL", "12abc"} {
		if strings.Contains(s, secret) {
			t.Fatalf("output leaks %q:\n%s", secret, s)
		}
	}
	got, complete := parseGitProbe(s)
	if !complete {
		t.Fatalf("a finished run must be complete:\n%s", s)
	}
	// Section names and keys are case-insensitive in git: [REMOTE "Up"] / URL = count.
	app := func(name, url string) GitRemote {
		return GitRemote{Path: filepath.Join(root, "srv/app"), Name: name, URL: url, Commit: "0123456789abcdef0123456789abcdef01234567", Branch: "main", PulledAt: 1700001000, PullKind: "pull"}
	}
	want := []GitRemote{
		{Path: filepath.Join(root, "srv/api"), Name: "origin", URL: "https://gitlab.example.gov.br/sead/api.git", Commit: "fefefefefefefefefefefefefefefefefefefefe", Branch: "dev", PulledAt: 1700002000, PullKind: "fetch"},
		app("Up", "https://gitlab.example.gov.br/sead/up.git"),
		app("origin", "https://gitlab.example.gov.br/sead/app.git"),
		{Path: filepath.Join(root, "srv/evil"), Name: "origin", URL: "https://gitlab.example.gov.br/sead/evil.git"},
		{Path: filepath.Join(root, "srv/fifo-a"), Name: "origin", URL: "https://gitlab.example.gov.br/sead/fa.git"},
		{Path: filepath.Join(root, "srv/fifo-b"), Name: "origin", URL: "https://gitlab.example.gov.br/sead/fb.git", Branch: "main"},
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Path+got[i].Name < got[j].Path+got[j].Name }) // find's order is the directory's
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
			cmd := exec.Command("sh", "-c", gitProbeScript(true, budgets[CaptureSync]))
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

// Each container frame ends with the docker exec status the host shell saw
// ("@@exit N"): split off the body (the web parser never sees it), the last
// one wins (a container can print its own), absent = -1.
func TestSplitCtrFramesExit(t *testing.T) {
	_, frames := splitCtrFrames("host\n@@CTR 0123456789ab a img\nbody\n@@exit 7\n\n" +
		"@@CTR 0123456789ac b img\n@@exit 0\nmore\n@@exit 126\n" +
		"@@CTR 0123456789ad c img\ncut by the cap")
	if len(frames) != 3 {
		t.Fatalf("frames = %+v", frames)
	}
	if f := frames[0]; f.Exit != 7 || strings.Contains(f.Body, "@@exit") || !strings.Contains(f.Body, "body") {
		t.Fatalf("frame a = %+v", f)
	}
	if f := frames[1]; f.Exit != 126 || !strings.Contains(f.Body, "@@exit 0") {
		t.Fatalf("frame b = %+v, want the last marker, the container's own kept in the body", f)
	}
	if f := frames[2]; f.Exit != -1 {
		t.Fatalf("frame c = %+v, want -1 without a marker", f)
	}
}

// A container that never started the walk is fine only when its image has no
// shell (exec 126/127). Paused, restarting or killed (any other status, or no
// status at all) means it wasn't searched: the list is incomplete.
func TestParseGitProbeExecFailure(t *testing.T) {
	host := "@@git\n@@gitend\n"
	for status, complete := range map[string]bool{"126": true, "127": true, "1": false, "124": false} {
		raw := host + "@@CTR 0123456789ab c img\n@@exit " + status + "\n"
		if _, got := parseGitProbe(raw); got != complete {
			t.Errorf("exec status %s: complete = %v, want %v", status, got, complete)
		}
	}
	if _, got := parseGitProbe(host + "@@CTR 0123456789ab c img\n"); got {
		t.Error("a frame with no walk and no exec status must be incomplete")
	}
}

// End to end with a fake docker: a paused container makes the list
// incomplete; one without a shell doesn't.
func TestGitProbeContainerExecStatus(t *testing.T) {
	for name, c := range map[string]struct {
		second   string
		complete bool
	}{
		"no shell": {second: "exit 126", complete: true},
		"paused":   {second: "echo 'Container is paused' >&2; exit 1", complete: false},
	} {
		t.Run(name, func(t *testing.T) {
			path := fakeBin(t, map[string]string{"docker": `case "$1" in
ps) echo '0123456789ab ok img:1'; echo '0123456789ac other img:2';;
exec) [ "$3" = 0123456789ab ] && exec sh; ` + c.second + `;;
esac`})
			cmd := exec.Command("sh", "-c", gitProbeScript(true, budgets[CaptureSync]))
			cmd.Env = append(os.Environ(), "GITPROBE_ROOTS="+t.TempDir(), path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("script: %v\n%s", err, out)
			}
			if _, complete := parseGitProbe(string(out)); complete != c.complete {
				t.Fatalf("complete = %v, want %v\n%s", complete, c.complete, out)
			}
		})
	}
}

// fakeBin writes executable scripts into a fresh directory and returns a PATH
// that finds them first.
func fakeBin(t *testing.T, scripts map[string]string) string {
	t.Helper()
	bin := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return "PATH=" + bin + ":" + os.Getenv("PATH")
}

// gitTree makes <root>/<rel>/.git/config with one origin remote per entry.
func gitTree(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		dir := filepath.Join(root, rel, ".git")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := "[remote \"origin\"]\n\turl = https://h.example/g/" + filepath.Base(rel) + ".git\n"
		if err := os.WriteFile(filepath.Join(dir, "config"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// On a host where sudo needs no password, the whole walk runs under one
// privileged shell: two entries in the auth log per scan, not two per checkout.
func TestGitInnerScriptOnePrivilegedShell(t *testing.T) {
	root, log := t.TempDir(), filepath.Join(t.TempDir(), "calls")
	gitTree(t, root, "a", "b", "c")
	// The fake logs each call, drops -n and runs the rest as is.
	path := fakeBin(t, map[string]string{"sudo": `echo "$*" >> "$PRIV_LOG"; shift; exec "$@"`})
	cmd := exec.Command("sh", "-c", "SUDO=1\n"+gitInnerScript)
	cmd.Env = append(os.Environ(), "GITPROBE_ROOTS="+root, "PRIV_LOG="+log, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	if got, complete := parseGitProbe(string(out)); len(got) != 3 || !complete {
		t.Fatalf("remotes = %+v complete=%v\n%s", got, complete, out)
	}
	calls, _ := os.ReadFile(log)
	if n := strings.Count(string(calls), "\n"); n != 2 {
		t.Fatalf("privileged calls = %d, want 2 (the check and one shell):\n%s", n, calls)
	}
}

// Docker's data root, wherever it is configured, is skipped: its volumes and
// layers would repeat checkouts (and eat the 100 cap).
func TestGitInnerScriptPrunesDockerRoot(t *testing.T) {
	root := t.TempDir()
	gitTree(t, root, "app", "dockerdata/volumes/v/_data/copy")
	path := fakeBin(t, map[string]string{"docker": `[ "$1" = info ] && echo "$DOCKER_ROOT"`})
	cmd := exec.Command("sh", "-c", gitInnerScript)
	cmd.Env = append(os.Environ(), "GITPROBE_ROOTS="+root, "DOCKER_ROOT="+filepath.Join(root, "dockerdata"), path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	got, _ := parseGitProbe(string(out))
	if len(got) != 1 || got[0].Path != filepath.Join(root, "app") {
		t.Fatalf("remotes = %+v, want only the app checkout\n%s", got, out)
	}
}
