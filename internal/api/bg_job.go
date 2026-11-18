package api

import (
	"fmt"
	"sync"
	"time"
)

// bgJobStatus is a background run: under way, or how the last one ended.
type bgJobStatus[R any] struct {
	Running    bool       `json:"running"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Result     *R         `json:"result,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// bgJob runs one R-producing run at a time in the background, so the run
// outlives the request (the proxy in front of Bridge cuts requests at 2
// minutes). Used by the CVE evaluation, the GitLab-author scan and the app
// dependencies sync.
// ponytail: in memory, one process — a restart forgets a run under way (the
// next click starts over); move it to a table if Bridge ever runs replicated.
type bgJob[R any] struct {
	mu     sync.Mutex
	status bgJobStatus[R]
}

func (j *bgJob[R]) get() bgJobStatus[R] {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

// start launches run unless one is under way; either way it returns the
// current status and whether this call started it. An error or a panic of
// run ends up in the status.
func (j *bgJob[R]) start(run func() (*R, error)) (bgJobStatus[R], bool) {
	j.mu.Lock()
	if j.status.Running {
		s := j.status
		j.mu.Unlock()
		return s, false
	}
	now := time.Now().UTC()
	j.status = bgJobStatus[R]{Running: true, StartedAt: &now}
	s := j.status
	j.mu.Unlock()

	go func() {
		var res *R
		var err error
		defer func() {
			if rv := recover(); rv != nil {
				err = fmt.Errorf("panic: %v", rv)
			}
			end := time.Now().UTC()
			j.mu.Lock()
			j.status.Running, j.status.FinishedAt, j.status.Result = false, &end, res
			if err != nil {
				j.status.Error = err.Error()
			}
			j.mu.Unlock()
		}()
		res, err = run()
	}()
	return s, true
}
