package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// AutoresVarreduraStatus is the background GitLab-author scan: under way, or
// how the last one ended.
type AutoresVarreduraStatus struct {
	Running    bool                     `json:"running"`
	StartedAt  *time.Time               `json:"started_at,omitempty"`
	FinishedAt *time.Time               `json:"finished_at,omitempty"`
	Result     *service.ResumoVarredura `json:"result,omitempty"`
	Error      string                   `json:"error,omitempty"`
}

// autoresJob runs one scan at a time in the background, so the scan outlives
// the request (the proxy in front of Bridge cuts requests at 2 minutes).
// ponytail: twin of evalJob (dependency_handlers.go): in memory, one process.
// Fold both into one generic job if a third background run shows up.
type autoresJob struct {
	mu     sync.Mutex
	status AutoresVarreduraStatus
}

var varreduraAutores = &autoresJob{}

const varreduraAutoresTimeout = 30 * time.Minute

func (j *autoresJob) get() AutoresVarreduraStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

// start launches run unless one is under way; either way it returns the
// current status and whether this call started it.
func (j *autoresJob) start(run func() (*service.ResumoVarredura, error)) (AutoresVarreduraStatus, bool) {
	j.mu.Lock()
	if j.status.Running {
		s := j.status
		j.mu.Unlock()
		return s, false
	}
	now := time.Now().UTC()
	j.status = AutoresVarreduraStatus{Running: true, StartedAt: &now}
	s := j.status
	j.mu.Unlock()

	go func() {
		var res *service.ResumoVarredura
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
