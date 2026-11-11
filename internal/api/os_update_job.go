package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// OS update runs: check / package / security upgrades over a set of hosts,
// in the background (they outlive any request and the proxy's 2-minute cut).
// Every state change is written to os_update_runs / os_update_results, so the
// UI polls the tables; a restart marks the runs it cut short interrupted
// (MarkInterrupted, at startup). On each host the script itself runs
// detached (sshtest.StartOSUpdate), so it survives a dropped session too.

const (
	osUpdateMaxHosts       = 500
	osUpdateMaxConcurrency = 10
)

// Timings, vars so tests can shrink them.
var (
	osUpdatePoll        = 15 * time.Second
	osUpdateHostTimeout = 60 * time.Minute
	osUpdateRebootWait  = 90 * time.Second // before trying to reconnect
	osUpdateRebootLimit = 10 * time.Minute // for the host to answer SSH again
)

// hostRunner does one host of a run and returns its outcome; the job writes it.
type hostRunner func(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult

// osUpdateJob runs the runs. cancelled holds the runs asked to stop: their
// pending hosts are skipped, running ones finish (a running apt is never
// killed).
// shortcut: one process — a second Bridge replica wouldn't see a cancel
// made on the first; move cancel to the table if Bridge ever runs replicated.
type osUpdateJob struct {
	mu        sync.Mutex
	cancelled map[int64]bool
}

var osUpdates = &osUpdateJob{cancelled: map[int64]bool{}}

func (j *osUpdateJob) cancel(runID int64) {
	j.mu.Lock()
	j.cancelled[runID] = true
	j.mu.Unlock()
}

func (j *osUpdateJob) isCancelled(runID int64) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cancelled[runID]
}

// start works through the run's pending hosts, run.Concurrency at a time,
// then closes the run. Detached from any request; returns a channel closed
// when the run is over (tests wait on it).
func (j *osUpdateJob) start(db *sql.DB, run models.OSUpdateRun, one hostRunner) <-chan struct{} {
	done := make(chan struct{})
	repo := store.NewOSUpdateRepo(db)
	go func() {
		defer close(done)
		ctx := store.WithSystemScope(context.Background())
		ids, err := repo.PendingHosts(ctx, run.ID)
		if err != nil {
			log.Printf("[osupdate] run %d: %v", run.ID, err)
		}
		queue := make(chan int64, len(ids))
		for _, id := range ids {
			queue <- id
		}
		close(queue)
		var wg sync.WaitGroup
		for range max(1, min(run.Concurrency, osUpdateMaxConcurrency)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for id := range queue {
					if j.isCancelled(run.ID) {
						return
					}
					j.doHost(ctx, db, run, id, one)
				}
			}()
		}
		wg.Wait()
		status := models.OSRunDone
		if j.isCancelled(run.ID) {
			status = models.OSRunCancelled
			repo.SkipPending(ctx, run.ID, "cancelled")
		}
		repo.FinishRun(ctx, run.ID, status)
		j.mu.Lock()
		delete(j.cancelled, run.ID)
		j.mu.Unlock()
		log.Printf("[osupdate] run %d (%s) %s", run.ID, run.Kind, status)
	}()
	return done
}

// doHost runs one host and writes its result and audit row. A panic in the
// runner fails that host only.
func (j *osUpdateJob) doHost(ctx context.Context, db *sql.DB, run models.OSUpdateRun, hostID int64, one hostRunner) {
	repo := store.NewOSUpdateRepo(db)
	if ok, err := repo.StartResult(ctx, run.ID, hostID, latestOS(ctx, db, hostID)); err != nil || !ok {
		return // cancelled meanwhile
	}
	var res models.OSUpdateResult
	if host, err := store.NewHostRepo(db).GetByID(ctx, hostID); err != nil || host == nil {
		res = models.OSUpdateResult{Status: models.OSResultSkipped, Reason: "host not found"}
	} else {
		res = func() (res models.OSUpdateResult) {
			defer func() {
				if rv := recover(); rv != nil {
					res = models.OSUpdateResult{Status: models.OSResultFailed, Reason: fmt.Sprintf("internal error: %v", rv)}
				}
			}()
			return one(ctx, run, host)
		}()
	}
	res.RunID, res.HostID = run.ID, hostID
	if err := repo.FinishResult(ctx, &res); err != nil {
		log.Printf("[osupdate] run %d host %d: save result: %v", run.ID, hostID, err)
	}
	if run.RequestedBy != nil {
		status := map[string]string{models.OSResultSuccess: "success", models.OSResultFailed: "failed"}[res.Status]
		if status == "" {
			status = "info"
		}
		store.NewOperationLogRepo(db).Create(ctx, &models.OperationLog{HostID: hostID, UserID: *run.RequestedBy,
			OperationType: "os-update:" + run.Kind, Status: status,
			Output: fmt.Sprintf("run %d: %s %s", run.ID, res.Status, res.Reason)})
	}
}

// latestOS is the host's OS as its latest scan saw it ("" when unknown).
func latestOS(ctx context.Context, db *sql.DB, hostID int64) string {
	scan, err := store.NewHostScanRepo(db).GetLatest(ctx, hostID)
	if err != nil || scan == nil {
		return ""
	}
	var v struct {
		OS string `json:"os"`
	}
	json.Unmarshal([]byte(scan.Data), &v)
	return v.OS
}

// runner is the hostRunner the handlers use: h.osHostRunner in tests, else
// the real SSH one.
func (h *sshHandlers) runner() hostRunner {
	if h.osHostRunner != nil {
		return h.osHostRunner
	}
	return h.sshHostRunner
}

// sshHostRunner is the real hostRunner: launch the script detached, poll it,
// optionally reboot, then rescan through the scan's own path (recordScan).
func (h *sshHandlers) sshHostRunner(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
	res := models.OSUpdateResult{Status: models.OSResultFailed}
	if host.User == "" {
		return models.OSUpdateResult{Status: models.OSResultSkipped, Reason: "host has no SSH user"}
	}
	_, authn, err := service.ResolveSSHAuth(ctx, h.db, host, "")
	var cfgErr *service.SSHConfigError
	if errors.As(err, &cfgErr) {
		return models.OSUpdateResult{Status: models.OSResultSkipped, Reason: cfgErr.Msg}
	} else if err != nil {
		res.Reason = err.Error()
		return res
	}
	priv := sshtest.Privilege{User: host.User, Password: authn.Password()}
	dial := func() (*ssh.Client, error) { return sshtest.Dial(host.Hostname, host.Port, host.User, authn) }

	client, err := dial()
	if err != nil {
		res.Reason = "SSH connect: " + err.Error()
		return res
	}
	err = sshtest.StartOSUpdate(client, priv, run.ID, run.Kind)
	client.Close()
	if err != nil {
		res.Reason = "start: " + err.Error()
		return res
	}

	// Poll on fresh connections until the exit file shows up.
	deadline := time.Now().Add(osUpdateHostTimeout)
	var exitCode int
	var logTail string
	for done := false; !done; {
		if time.Now().After(deadline) {
			res.Reason, res.Output = fmt.Sprintf("no result after %s: the update may still be running on the host", osUpdateHostTimeout), logTail
			return res
		}
		time.Sleep(osUpdatePoll)
		c, err := dial()
		if err != nil {
			continue // transient: the deadline decides
		}
		var pollErr error
		done, exitCode, logTail, pollErr = sshtest.PollOSUpdate(c, run.ID)
		c.Close()
		if pollErr != nil && !done {
			res.Reason = pollErr.Error()
			return res
		}
	}

	rep := sshtest.ParseOSUpdateLog(logTail)
	res.Output, res.PkgManager, res.RebootRequired = logTail, rep.PkgManager, rep.RebootRequired
	// Upgradable / security are what's still pending once the run is over:
	// the "pending updates" column shows the latest of them.
	res.Upgradable, res.SecurityUpgradable, res.Upgraded = rep.Upgradable, rep.Security, upgradedCount(run.Kind, rep)
	if run.Kind != sshtest.OSUpdateCheck {
		res.Upgradable, res.SecurityUpgradable = rep.Remaining, rep.RemainingSecurity
	}
	switch {
	case exitCode != 0:
		res.Reason = rep.Error
		if res.Reason == "" {
			res.Reason = "exit " + strconv.Itoa(exitCode)
		}
		return res
	case rep.Skip != "":
		res.Status, res.Reason = models.OSResultSkipped, rep.Skip
		return res
	}
	res.Status = models.OSResultSuccess

	if run.Reboot && rep.RebootRequired && run.Kind != sshtest.OSUpdateCheck {
		if c, err := dial(); err != nil {
			res.Reason = "reboot: SSH connect: " + err.Error()
		} else {
			err := sshtest.Reboot(c, priv)
			c.Close()
			if err != nil {
				res.Reason = "reboot: " + err.Error()
			} else {
				res.Rebooted = true
				time.Sleep(osUpdateRebootWait)
			}
		}
	}

	// Rescan, so the OS map, dependencies and EOL show the new state; after a
	// reboot, wait for SSH to come back first.
	client = waitForSSH(dial, res.Rebooted)
	if client == nil {
		res.Reason = joinReason(res.Reason, "the host did not answer SSH after the update")
		return res
	}
	defer client.Close()
	info, err := sshtest.TestCapture(client, authn.Password())
	if err != nil {
		res.Reason = joinReason(res.Reason, "rescan: "+err.Error())
		return res
	}
	h.recordScan(ctx, host, info, map[string]any{})
	res.OSAfter, res.RebootRequired = info.OS, info.RebootRequired
	return res
}

// waitForSSH dials once, or for up to osUpdateRebootLimit after a reboot.
func waitForSSH(dial func() (*ssh.Client, error), rebooted bool) *ssh.Client {
	deadline := time.Now()
	if rebooted {
		deadline = deadline.Add(osUpdateRebootLimit)
	}
	for {
		if c, err := dial(); err == nil {
			return c
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(osUpdatePoll)
	}
}

// upgradedCount is how many packages the run upgraded: what was upgradable
// minus what still is (nil for a check, or when a count is missing).
func upgradedCount(kind string, rep sshtest.OSUpdateReport) *int {
	before, after := rep.Upgradable, rep.Remaining
	if kind == sshtest.OSUpdateSecurity {
		before, after = rep.Security, rep.RemainingSecurity
	}
	if kind == sshtest.OSUpdateCheck || before == nil || after == nil {
		return nil
	}
	n := max(*before-*after, 0)
	return &n
}

func joinReason(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// ── Handlers ────────────────────────────────────────────────────────────

type osUpdateRunRequest struct {
	Kind        string  `json:"kind"`
	HostIDs     []int64 `json:"host_ids"`
	Reboot      bool    `json:"reboot"`
	Concurrency int     `json:"concurrency"`
}

var osUpdateKinds = map[string]bool{sshtest.OSUpdateCheck: true, sshtest.OSUpdatePackages: true, sshtest.OSUpdateSecurity: true}

// handleStartOSUpdate godoc
//
//	@Summary		Start an OS update run
//	@Description	Admin. Runs kind on host_ids in the background — check: refresh the package lists and count upgrades; packages: every upgrade within the current release; security: security upgrades only — concurrency hosts at a time (1–10, default 3), as root (the SSH user if root, sudo with the stored password, else passwordless sudo). reboot reboots a host that needs it after a successful upgrade. Every host is rescanned at the end. Answers 202 with the run; poll GET …/runs/{id}. 404 when a host isn't visible.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			body	body		osUpdateRunRequest	true	"Run"
//	@Success		202		{object}	models.OSUpdateRun
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/runs [post]
func (h *sshHandlers) handleStartOSUpdate(w http.ResponseWriter, r *http.Request) {
	var req osUpdateRunRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if !osUpdateKinds[req.Kind] {
		jsonError(w, http.StatusBadRequest, "kind must be check, packages or security")
		return
	}
	if len(req.HostIDs) == 0 || len(req.HostIDs) > osUpdateMaxHosts {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("host_ids must have 1 to %d hosts", osUpdateMaxHosts))
		return
	}
	if req.Concurrency == 0 {
		req.Concurrency = 3
	}
	if req.Concurrency < 1 || req.Concurrency > osUpdateMaxConcurrency {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("concurrency must be 1 to %d", osUpdateMaxConcurrency))
		return
	}
	hosts := store.NewHostRepo(h.db.SQL)
	for _, id := range req.HostIDs {
		if host, err := hosts.GetByID(r.Context(), id); err != nil || host == nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("host %d not found", id))
			return
		}
	}
	run := models.OSUpdateRun{Kind: req.Kind, Reboot: req.Reboot && req.Kind != sshtest.OSUpdateCheck, Concurrency: req.Concurrency}
	if u := auth.UserFromContext(r.Context()); u != nil {
		run.RequestedBy = &u.ID
	}
	repo := store.NewOSUpdateRepo(h.db.SQL)
	if err := repo.CreateRun(r.Context(), &run, req.HostIDs); err != nil {
		jsonServerError(w, r, "failed to create the run", err)
		return
	}
	osUpdates.start(h.db.SQL, run, h.runner())
	got, err := repo.GetRun(r.Context(), run.ID)
	if err != nil || got == nil {
		got = &run
	}
	httpx.WriteJSON(w, http.StatusAccepted, got)
}

// handleListOSUpdates godoc
//
//	@Summary		List OS update runs
//	@Description	Admin. The 50 newest runs, newest first, with how many hosts are in each status.
//	@Tags			ssh
//	@Produce		json
//	@Success		200	{array}	models.OSUpdateRun
//	@Router			/api/ssh/os-updates/runs [get]
func (h *sshHandlers) handleListOSUpdates(w http.ResponseWriter, r *http.Request) {
	runs, err := store.NewOSUpdateRepo(h.db.SQL).ListRuns(r.Context(), 50)
	if err != nil {
		jsonServerError(w, r, "failed to list runs", err)
		return
	}
	jsonOK(w, runs)
}

// osRunFromPath loads the {id} run, or writes 404.
func (h *sshHandlers) osRunFromPath(w http.ResponseWriter, r *http.Request) *models.OSUpdateRun {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	run, err := store.NewOSUpdateRepo(h.db.SQL).GetRun(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load the run", err)
		return nil
	}
	if run == nil {
		jsonError(w, http.StatusNotFound, "run not found")
	}
	return run
}

// handleGetOSUpdate godoc
//
//	@Summary		Get an OS update run
//	@Description	Admin. The run with every host's result: status, reason, package manager, OS before and after, upgradable / security / upgraded counts, reboot, and the tail of the host's log.
//	@Tags			ssh
//	@Produce		json
//	@Param			id	path		int	true	"Run ID"
//	@Success		200	{object}	models.OSUpdateRun
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/runs/{id} [get]
func (h *sshHandlers) handleGetOSUpdate(w http.ResponseWriter, r *http.Request) {
	if run := h.osRunFromPath(w, r); run != nil {
		jsonOK(w, run)
	}
}

// handleCancelOSUpdate godoc
//
//	@Summary		Cancel an OS update run
//	@Description	Admin. Hosts not started yet are skipped; hosts already updating finish (an upgrade is never killed halfway). Returns the run.
//	@Tags			ssh
//	@Produce		json
//	@Param			id	path		int	true	"Run ID"
//	@Success		200	{object}	models.OSUpdateRun
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/runs/{id}/cancel [post]
func (h *sshHandlers) handleCancelOSUpdate(w http.ResponseWriter, r *http.Request) {
	run := h.osRunFromPath(w, r)
	if run == nil {
		return
	}
	if run.Status == models.OSRunRunning {
		osUpdates.cancel(run.ID)
		store.NewOSUpdateRepo(h.db.SQL).SkipPending(r.Context(), run.ID, "cancelled")
	}
	if got := h.osRunFromPath(w, r); got != nil {
		jsonOK(w, got)
	}
}

// handleLatestOSUpdates godoc
//
//	@Summary		Each host's latest update counts
//	@Description	Admin. Per host, the newest successful run that counted upgrades (any kind): upgradable and security counts, when, and the reboot flag. Without output.
//	@Tags			ssh
//	@Produce		json
//	@Success		200	{array}	models.OSUpdateResult
//	@Router			/api/ssh/os-updates/latest [get]
func (h *sshHandlers) handleLatestOSUpdates(w http.ResponseWriter, r *http.Request) {
	rows, err := store.NewOSUpdateRepo(h.db.SQL).LatestByHost(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to load the latest results", err)
		return
	}
	jsonOK(w, rows)
}
