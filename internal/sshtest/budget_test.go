package sshtest

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// A client is on the job budget only while a CaptureJob capture runs on it;
// every other use (the synchronous scan, the OS update scripts) keeps the
// budget that fits the browser's 120 s request.
func TestBudgetOf(t *testing.T) {
	c := &ssh.Client{}
	if b := budgetOf(c); b != budgets[CaptureSync] || b.cmd != 30*time.Second {
		t.Fatalf("unregistered = %+v", b)
	}
	done := enterMode(c, CaptureJob)
	if b := budgetOf(c); b != budgets[CaptureJob] || b.cmd <= budgets[CaptureSync].cmd {
		t.Fatalf("job = %+v", b)
	}
	if b := budgetOf(&ssh.Client{}); b != budgets[CaptureSync] {
		t.Fatalf("another client = %+v", b)
	}
	done()
	if b := budgetOf(c); b != budgets[CaptureSync] {
		t.Fatalf("after the job = %+v", b)
	}
	enterMode(c, CaptureSync)() // a sync capture registers nothing
	if b := budgetOf(c); b != budgets[CaptureSync] {
		t.Fatalf("sync = %+v", b)
	}
}

// Job mode raises the walk and the containers' start budget; the output caps
// (100 checkouts, 30 containers) are the same in both.
func TestGitProbeScriptBudgets(t *testing.T) {
	sync, job := gitProbeScript(false, budgets[CaptureSync]), gitProbeScript(false, budgets[CaptureJob])
	if strings.Count(sync, "timeout 20") == 0 || strings.Contains(sync, "timeout 180") {
		t.Error("sync walk is 20 s")
	}
	if !strings.Contains(sync, "-ge 30 ]") || !strings.Contains(job, "-ge 300 ]") {
		t.Error("containers started within 30 s (sync) / 300 s (job)")
	}
	// The host's walk runs inline on the host and base64'd inside containers.
	if !strings.Contains(job, "timeout 180") || !strings.Contains(job, base64.StdEncoding.EncodeToString([]byte(gitInner(180)))) {
		t.Error("job walk is 180 s, on the host and in containers")
	}
	for _, s := range []string{sync, job} {
		if !strings.Contains(s, "head -n 101") || !strings.Contains(s, "head -n 30 ") {
			t.Error("caps: 100 checkouts, 30 containers")
		}
	}
	if gitInnerScript != gitInner(20) {
		t.Error("gitInnerScript is the sync one")
	}
	if budgets[CaptureJob].gitProbe <= time.Duration(budgets[CaptureJob].gitWalk+budgets[CaptureJob].gitCtrs)*time.Second {
		t.Error("the unprivileged deadline must outlast the walk and the containers' budget")
	}
}
