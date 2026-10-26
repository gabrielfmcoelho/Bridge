package api

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// waitIdle polls until the job finishes (or fails the test after 2s).
func waitIdle(t *testing.T, j *evalJob) DependencyEvalStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s := j.get(); !s.Running {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job still running")
	return DependencyEvalStatus{}
}

// One run at a time: a second click while one runs returns that run; the
// result, an error or a panic all end up in the status.
func TestEvalJob(t *testing.T) {
	j := &evalJob{}
	release := make(chan struct{})
	calls := 0
	run := func() (*service.DependencyEvalResult, error) {
		calls++
		<-release
		return &service.DependencyEvalResult{NVDQueried: 7}, nil
	}

	s1, started := j.start(run)
	if !started || !s1.Running || s1.StartedAt == nil {
		t.Fatalf("first start = %+v %v", s1, started)
	}
	s2, started := j.start(run)
	if started || !s2.Running || !s2.StartedAt.Equal(*s1.StartedAt) {
		t.Fatalf("second start while running = %+v %v; want the running job", s2, started)
	}
	close(release)
	done := waitIdle(t, j)
	if calls != 1 || done.Result == nil || done.Result.NVDQueried != 7 || done.FinishedAt == nil || done.Error != "" {
		t.Fatalf("finished = %+v (calls %d)", done, calls)
	}

	j.start(func() (*service.DependencyEvalResult, error) { return nil, errors.New("db down") })
	if s := waitIdle(t, j); s.Error != "db down" || s.Result != nil {
		t.Fatalf("error run = %+v", s)
	}
	j.start(func() (*service.DependencyEvalResult, error) { panic("boom") })
	if s := waitIdle(t, j); !strings.Contains(s.Error, "panic: boom") {
		t.Fatalf("panic run = %+v", s)
	}
}
