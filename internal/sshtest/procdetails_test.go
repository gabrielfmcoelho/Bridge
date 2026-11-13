package sshtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnitFromCgroup(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"v1 systemd", "1:name=systemd:/system.slice/compras1.service", "compras1.service"},
		{"v2", "0::/system.slice/nginx.service", "nginx.service"},
		{"session scope", "0::/user.slice/user-1008.slice/session-7.scope", ""},
		{"docker scope", "0::/system.slice/docker-ab12.scope", ""},
		{"sub-cgroup", "0::/system.slice/foo.service/sub", "foo.service"},
		{"v1 multi-line", "4:cpu,cpuacct:/system.slice/other.service\n3:memory:/system.slice/other.service\n1:name=systemd:/system.slice/compras1.service\n", "compras1.service"},
	}
	for _, c := range cases {
		if got := unitFromCgroup(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseProcessDetails(t *testing.T) {
	const s = "\x1f"
	raw := "1234" + s + "jboss" + s + "1.5" + s + "8.2" + s + "/opt/jboss/bin" + s + "/usr/bin/java" + s + "sh" + s +
		"1:name=systemd:/system.slice/compras1.service" + s + "8080,8083,1099" + s +
		"java -Dpass=x -jar run.jar | tee out\n" +
		"1235" + s + "jboss" + s + "0.5" + s + "3.0" + s + "/opt/jboss/bin" + s + "/usr/bin/java" + s + "sh" + s +
		"1:name=systemd:/system.slice/compras1.service" + s + "8080" + s + "java -jar b.jar\n" +
		"77" + s + "app" + s + "0.0" + s + "1.0" + s + "" + s + "" + s + "bash" + s + "" + s + "" + s + "python app.py\n"
	got := parseProcessDetails(raw)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	a := got[0]
	if a.PID != "1234" || a.StartedVia != "systemd" || a.Unit != "compras1.service" || a.Ports != "8080,8083,1099" ||
		a.Command != "java -Dpass=x -jar run.jar | tee out" || a.CPU != "1.5%" || a.CWD != "/opt/jboss/bin" {
		t.Errorf("first = %+v", a)
	}
	if got[2].StartedVia != "manual" || got[2].Unit != "" {
		t.Errorf("third = %+v", got[2])
	}
}

// The script run locally against a fake ps: 25 python processes come before
// a JBoss in PID order, yet the JBoss is reported (app servers are picked
// first), the rest stays capped at 20, and every line has its 10 fields.
func TestProcessDetailsScript_AppServersFirst(t *testing.T) {
	dir := t.TempDir()
	var ps strings.Builder
	ps.WriteString("echo 'USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND'\n")
	for i := 0; i < 25; i++ {
		fmt.Fprintf(&ps, "echo 'app %d 0.1 0.2 1 1 ? S 10:00 0:00 /usr/bin/python3 worker%d.py'\n", 990000+i, i)
	}
	ps.WriteString("echo 'jboss 990100 3.0 9.0 1 1 ? Sl 10:00 1:00 /opt/jdk/bin/java -Dprogram.name=run.sh -classpath /opt/compras1/bin/run.jar org.jboss.Main -c default'\n")
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\n"+ps.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", processDetailsScript())
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	for _, line := range splitLines(string(out)) {
		if n := strings.Count(line, procSep); n != 9 {
			t.Errorf("%d separators, want 9 (10 fields): %q", n, line)
		}
	}
	got := parseProcessDetails(string(out))
	jboss, others := false, 0
	for _, p := range got {
		if p.PID == "990100" {
			jboss = true
		} else {
			others++
		}
	}
	if !jboss || others != 20 {
		t.Fatalf("jboss reported = %v, others = %d (want true, 20)", jboss, others)
	}
}
