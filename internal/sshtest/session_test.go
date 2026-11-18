package sshtest

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func refused() error { return &ssh.OpenChannelError{Reason: ssh.Prohibited} }

func TestRetrySessionOpenRetriesRefusal(t *testing.T) {
	calls := 0
	var waits []int
	_, err := retrySessionOpen(func() (*ssh.Session, error) {
		calls++
		if calls <= 2 {
			return nil, refused()
		}
		return nil, nil
	}, 5, func(a int) { waits = append(waits, a) })
	if err != nil || calls != 3 || !reflect.DeepEqual(waits, []int{1, 2}) {
		t.Fatalf("err=%v calls=%d waits=%v", err, calls, waits)
	}
}

// OpenSSH 7.6+ refuses a session over MaxSessions with "connect failed (open
// failed)", not "administratively prohibited" (SAPASP-AVALIE04, Ubuntu 22.04).
func TestRetrySessionOpenRetriesConnectFailed(t *testing.T) {
	calls := 0
	_, err := retrySessionOpen(func() (*ssh.Session, error) {
		calls++
		if calls == 1 {
			return nil, &ssh.OpenChannelError{Reason: ssh.ConnectionFailed, Message: "open failed"}
		}
		return nil, nil
	}, 5, func(int) {})
	if err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetrySessionOpenOtherErrorReturnsAtOnce(t *testing.T) {
	calls := 0
	_, err := retrySessionOpen(func() (*ssh.Session, error) { calls++; return nil, io.EOF }, 5, func(int) {})
	if err != io.EOF || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetrySessionOpenGivesUp(t *testing.T) {
	calls := 0
	_, err := retrySessionOpen(func() (*ssh.Session, error) { calls++; return nil, refused() }, 4, func(int) {})
	var oce *ssh.OpenChannelError
	if !errors.As(err, &oce) || calls != 4 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestLoginsServicesPortsScriptExportsSbin(t *testing.T) {
	if !strings.HasPrefix(loginsServicesPortsScript("X"), scanPath) {
		t.Fatal("script must start with scanPath")
	}
}

func TestServicesNameDropsColon(t *testing.T) {
	got := cleanServiceBinaries([]string{"nginx:", "/usr/bin/node"})
	if !reflect.DeepEqual(got, []string{"nginx", "/usr/bin/node"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanSlotsBlockAtTheCap(t *testing.T) {
	st := newScanState()
	var releases []func()
	for i := 0; i < maxScanSessions; i++ {
		releases = append(releases, st.acquire())
	}
	got := make(chan func(), 1)
	go func() { got <- st.acquire() }()
	select {
	case <-got:
		t.Fatal("a session over the cap was let through")
	case <-time.After(50 * time.Millisecond):
	}
	releases[0]()
	releases[0]() // a second Close must not free a second slot
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("a freed slot was not handed over")
	}
	if len(st.slots) != maxScanSessions {
		t.Fatalf("slots in use = %d, want %d", len(st.slots), maxScanSessions)
	}
}

func TestNoScanStateMeansNoCap(t *testing.T) {
	var st *scanState
	st.acquire()() // outside a capture: nothing to wait for
	if w := st.incompleteWarning(); w != "" {
		t.Fatalf("warning %q", w)
	}
}

func TestIncompleteWarningCountsCommandsThatDidNotRun(t *testing.T) {
	st := newScanState()
	if w := st.incompleteWarning(); w != "" {
		t.Fatalf("clean scan warned: %q", w)
	}
	st.refused.Add(2)
	st.timedOut.Add(1)
	w := st.incompleteWarning()
	if !strings.Contains(w, "3 scan commands") || !strings.Contains(w, "2 refused") || !strings.Contains(w, "1 timed out") {
		t.Fatalf("warning %q", w)
	}
}
