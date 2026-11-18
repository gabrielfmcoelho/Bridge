package sshtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The CPU figure is the host's load before the scan starts working: the sample
// window (1 s) closes before any probe runs. SAPASP-AVALIE04 read 94% with a
// load average of 0.00 when the window bracketed the scan's own startup batch.
func TestCPUSampleWindowClosesBeforeTheScanWorks(t *testing.T) {
	root := t.TempDir()
	stat := filepath.Join(root, "proc", "stat")
	if err := os.MkdirAll(filepath.Dir(stat), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(line string) string { return "printf '" + line + "\\n' > " + stat }
	script := strings.Join([]string{
		write("cpu 100 0 0 100"), // before the window
		"(sleep 0.4; " + write("cpu 150 0 0 150") + ") & " + cpuSampleCmd, // inside the window: 50% busy
		write("cpu 1150 0 0 150"), // the scan's own burst, after the window
		cpuPrintCmd,
	}, "; ")
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "BRIDGE_ROOT="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "50%" {
		t.Fatalf("cpu = %q, want 50%% (95%% means the scan's burst was measured)", got)
	}
}
