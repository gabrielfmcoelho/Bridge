package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/httpx"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Background host scans: a run of kind 'scan' over a set of hosts, on the OS
// update runner (os_update_job.go), so it outlives the browser tab and the
// 120 s request. Each host is dialed like the synchronous scan
// (handleTestConnection: its auth method, the pinned host key), captured on
// the job budget (sshtest.CaptureJob), then goes through the same
// bookkeeping: test status, operation log, recordScan.

const (
	scanDefaultConcurrency = 10 // the browser's pool before scans were runs
	scanAttempts           = 3  // dials, as the browser's batch retried
)

// vars so tests can shrink them.
var (
	scanRetryWait   = 10 * time.Second
	scanHostTimeout = 20 * time.Minute // well past every probe's own budget
)

// scanMethod is the SSH method a scan uses: want when given, else the one the
// browser's batch scan picked — the preferred one when the host has both
// (the key when none is preferred), else the one it has. "" when it has
// neither (ResolveSSHAuth then says why).
func scanMethod(host *models.Host, want string) string {
	switch {
	case want != "":
		return want
	case host.HasKey && host.HasPassword && host.PreferredAuth == "password":
		return "password"
	case host.HasKey:
		return "key"
	case host.HasPassword:
		return "password"
	}
	return ""
}

// scanRunner is the hostRunner of scan runs: h.osHostRunner in tests.
func (h *sshHandlers) scanRunner() hostRunner {
	if h.osHostRunner != nil {
		return h.osHostRunner
	}
	return h.scanHostRunner
}

// scanHostRunner scans one host of a run. Its output is recordScan's summary
// (JSON: what was saved, linked, and what failed along the way).
func (h *sshHandlers) scanHostRunner(ctx context.Context, run models.OSUpdateRun, host *models.Host) models.OSUpdateResult {
	if host.User == "" {
		return models.OSUpdateResult{Status: models.OSResultSkipped, Reason: "host has no SSH user"}
	}
	method, authn, err := service.ResolveSSHAuth(ctx, h.db, host, scanMethod(host, run.AuthMethod))
	var cfgErr *service.SSHConfigError
	if errors.As(err, &cfgErr) {
		return models.OSUpdateResult{Status: models.OSResultSkipped, Reason: cfgErr.Msg}
	} else if err != nil {
		return models.OSUpdateResult{Status: models.OSResultFailed, Reason: err.Error()}
	}
	info, err := h.captureHost(ctx, host, authn)
	// The capture may have taken minutes: write onto the host as it is now.
	if fresh, ferr := store.NewHostRepo(h.db.SQL).GetByID(ctx, host.ID); ferr == nil && fresh != nil {
		host = fresh
	}
	var userID int64
	if run.RequestedBy != nil {
		userID = *run.RequestedBy
	}
	h.noteTest(ctx, host, method, userID, err)
	if err != nil {
		return models.OSUpdateResult{Status: models.OSResultFailed, Reason: err.Error()}
	}
	summary := map[string]any{}
	h.recordScan(ctx, host, info, summary)
	out, _ := json.Marshal(summary)
	return models.OSUpdateResult{Status: models.OSResultSuccess, OSAfter: info.OS, RebootRequired: info.RebootRequired, Output: string(out)}
}

// captureHost dials host — again, up to scanAttempts, when a retry can help —
// and captures it on the job budget within scanHostTimeout. Past it the
// client is closed and the scan fails: a closed client makes every probe come
// back empty, and recording that would mark the host's services offline.
func (h *sshHandlers) captureHost(ctx context.Context, host *models.Host, authn sshtest.Auth) (*sshtest.VMInfo, error) {
	var client *ssh.Client
	var err error
	for attempt := 1; ; attempt++ {
		if client, err = service.DialHost(ctx, h.db, host, host.User, authn); err == nil || attempt == scanAttempts || !retryableDial(err) {
			break
		}
		time.Sleep(scanRetryWait)
	}
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	timer := time.AfterFunc(scanHostTimeout, func() { client.Close() })
	info, err := sshtest.TestCapture(client, authn.Password(), sshtest.CaptureJob)
	if !timer.Stop() {
		return nil, fmt.Errorf("the scan took longer than %s and was stopped", scanHostTimeout)
	}
	return info, err
}

// retryableDial: a dropped or refused connection may answer next time; a
// refused login or a changed host key won't.
func retryableDial(err error) bool {
	var changed *service.HostKeyChangedError
	return !errors.As(err, &changed) && !strings.Contains(err.Error(), "unable to authenticate")
}

// noteTest saves a connection test's outcome on the host — the method's test
// status and the failure count — and a "test" row in its operation log,
// written as userID (none when 0). Shared by the synchronous scan and the runs.
func (h *sshHandlers) noteTest(ctx context.Context, host *models.Host, method string, userID int64, err error) {
	status, out := "success", ""
	if err != nil {
		status, out = "failed", err.Error()
		host.ConnectionsFailed++
	} else {
		host.ConnectionsFailed = 0
	}
	if method == "password" {
		host.PasswordTestStatus = &status
	} else {
		host.KeyTestStatus = &status
	}
	store.NewHostRepo(h.db.SQL).Update(ctx, host)
	if userID == 0 {
		return
	}
	if err := store.NewOperationLogRepo(h.db.SQL).Create(ctx, &models.OperationLog{HostID: host.ID, UserID: userID,
		OperationType: "test", AuthMethod: &method, Status: status, Output: out}); err != nil {
		log.Printf("[ssh] failed to log operation: %v", err)
	}
}

// ── Handlers ────────────────────────────────────────────────────────────

type scanRunRequest struct {
	HostIDs     []int64 `json:"host_ids"`
	Concurrency int     `json:"concurrency"` // 1–10, default 10
	// "password" or "key" for every host; empty = each host's own.
	Method string `json:"method"`
}

// handleStartScan godoc
//
//	@Summary		Start a background scan
//	@Description	Editor+. Scans host_ids on the server, concurrency at a time (1–10, default 10), each like POST /api/ssh/test/{slug} with capture (the host's auth method — or method for every host —, its pinned host key, the same bookkeeping) but with longer probe budgets, retrying a dropped connection. The run outlives the request and the browser tab; poll GET …/scans/{id}. Answers 202 with the run. 404 when a host isn't visible.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			body	body		scanRunRequest	true	"Hosts"
//	@Success		202		{object}	models.OSUpdateRun
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/scans [post]
func (h *sshHandlers) handleStartScan(w http.ResponseWriter, r *http.Request) {
	var req scanRunRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Method != "" && req.Method != "password" && req.Method != "key" {
		jsonError(w, http.StatusBadRequest, "method must be password, key or empty")
		return
	}
	concurrency, ok := h.checkRunHosts(w, r, req.HostIDs, req.Concurrency, scanDefaultConcurrency)
	if !ok {
		return
	}
	run := models.OSUpdateRun{Kind: models.RunKindScan, Concurrency: concurrency, AuthMethod: req.Method}
	if u := auth.UserFromContext(r.Context()); u != nil {
		run.RequestedBy = &u.ID
	}
	repo := store.NewOSUpdateRepo(h.db.SQL)
	if err := repo.CreateRun(r.Context(), &run, req.HostIDs); err != nil {
		jsonServerError(w, r, "failed to create the run", err)
		return
	}
	osUpdates.start(h.db.SQL, run, h.scanRunner())
	got, err := repo.GetRun(r.Context(), run.ID)
	if err != nil || got == nil {
		got = &run
	}
	httpx.WriteJSON(w, http.StatusAccepted, got)
}

// handleListScans godoc
//
//	@Summary		List background scans
//	@Description	Editor+. The caller's 50 newest scan runs (an admin's: everyone's), newest first, with how many hosts are in each status; active=1 keeps the running ones, host_id the ones that include that host.
//	@Tags			ssh
//	@Produce		json
//	@Param			active	query	int	false	"1: only running runs"
//	@Param			host_id	query	int	false	"Only runs of this host"
//	@Success		200		{array}	models.OSUpdateRun
//	@Router			/api/ssh/scans [get]
func (h *sshHandlers) handleListScans(w http.ResponseWriter, r *http.Request) {
	f := store.ScanRunFilter{HostID: hostIDParam(r), Active: r.URL.Query().Get("active") == "1"}
	if u := auth.UserFromContext(r.Context()); u == nil || u.Role != "admin" {
		f.RequestedBy = -1 // nobody's, unless a user is known
		if u != nil {
			f.RequestedBy = u.ID
		}
	}
	runs, err := store.NewOSUpdateRepo(h.db.SQL).ListScanRuns(r.Context(), f)
	if err != nil {
		jsonServerError(w, r, "failed to list runs", err)
		return
	}
	jsonOK(w, runs)
}

// scanRunFromPath loads the {id} scan run when the caller may see it — the
// user who started it, or an admin — or writes 404 (someone else's run
// doesn't exist for them: its host names are not theirs to read).
func (h *sshHandlers) scanRunFromPath(w http.ResponseWriter, r *http.Request) *models.OSUpdateRun {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	run, err := store.NewOSUpdateRepo(h.db.SQL).GetRun(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "failed to load the run", err)
		return nil
	}
	u := auth.UserFromContext(r.Context())
	if run == nil || run.Kind != models.RunKindScan || u == nil ||
		(u.Role != "admin" && (run.RequestedBy == nil || *run.RequestedBy != u.ID)) {
		jsonError(w, http.StatusNotFound, "run not found")
		return nil
	}
	return run
}

// handleGetScan godoc
//
//	@Summary		Get a background scan
//	@Description	Editor+. The run with every host's result: status, reason, OS, and as output a JSON summary of what the scan recorded. 404 unless the caller started it or is an admin.
//	@Tags			ssh
//	@Produce		json
//	@Param			id	path		int	true	"Run ID"
//	@Success		200	{object}	models.OSUpdateRun
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/ssh/scans/{id} [get]
func (h *sshHandlers) handleGetScan(w http.ResponseWriter, r *http.Request) {
	if run := h.scanRunFromPath(w, r); run != nil {
		jsonOK(w, run)
	}
}

// handleCancelScan godoc
//
//	@Summary		Cancel a background scan
//	@Description	Editor+. Hosts not started yet are skipped; hosts being scanned finish. Returns the run. 404 unless the caller started it or is an admin.
//	@Tags			ssh
//	@Produce		json
//	@Param			id	path		int	true	"Run ID"
//	@Success		200	{object}	models.OSUpdateRun
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/ssh/scans/{id}/cancel [post]
func (h *sshHandlers) handleCancelScan(w http.ResponseWriter, r *http.Request) {
	run := h.scanRunFromPath(w, r)
	if run == nil {
		return
	}
	if run.Status == models.OSRunRunning {
		osUpdates.cancel(run.ID)
		store.NewOSUpdateRepo(h.db.SQL).SkipPending(r.Context(), run.ID, "cancelled")
	}
	if got := h.scanRunFromPath(w, r); got != nil {
		jsonOK(w, got)
	}
}
