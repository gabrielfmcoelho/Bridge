package sshtest

import (
	"os/exec"
	"strings"
	"testing"
)

// The largest-files script is valid sh in both variants; only the
// unprivileged one tries sudo -n, and both walk every local partition.
func TestDiskTopScript(t *testing.T) {
	for _, asRoot := range []bool{false, true} {
		s := diskTopScript(asRoot)
		if out, err := exec.Command("sh", "-n", "-c", s).CombinedOutput(); err != nil {
			t.Fatalf("root=%v sh -n: %v %s", asRoot, err, out)
		}
		if strings.Contains(s, "sudo -n true") == asRoot {
			t.Fatalf("root=%v: sudo -n probe present = %v", asRoot, !asRoot)
		}
		if !strings.Contains(s, "find $M -xdev -type f") || !strings.Contains(s, "df -lP") {
			t.Fatalf("root=%v: not a walk of the local partitions:\n%s", asRoot, s)
		}
	}
}

func TestParseDiskTop_Files(t *testing.T) {
	got := parseDiskTop("53687091200\t/banco/pg/base/16384/2613.1\n1073741824\t/mnt/backup/dump 2026-10-01.sql.gz\n\nnot-a-size\t/x\n")
	if len(got) != 2 || got[0].Path != "/banco/pg/base/16384/2613.1" || got[0].SizeBytes != 53687091200 ||
		got[1].Path != "/mnt/backup/dump 2026-10-01.sql.gz" || got[1].HumanSize != "1.0 GiB" {
		t.Fatalf("parsed = %+v", got)
	}
}
