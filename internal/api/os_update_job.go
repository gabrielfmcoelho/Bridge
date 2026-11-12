package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	minioclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/minio"

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
	osReleaseTimeout    = 3 * time.Hour    // a release upgrade downloads and installs the whole system
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
	// A release runs its own checks first: a refusal ends the run before the
	// backup and the package update, not after them.
	if run.Kind == sshtest.OSUpdateRelease {
		err = sshtest.StartDetached(client, priv, run.ID, sshtest.ReleasePrecheck(run.UpdateFirst))
		client.Close()
		code, tail := 0, ""
		if err == nil {
			code, tail, err = pollOSRun(dial, run.ID, osUpdateHostTimeout)
		}
		rep := sshtest.ParseOSUpdateLog(tail)
		switch {
		case rep.Skip != "":
			return models.OSUpdateResult{Status: models.OSResultSkipped, Reason: rep.Skip, Output: tail}
		case err != nil:
			res.Reason, res.Output = "release check: "+err.Error(), tail
			return res
		case code != 0 || !rep.Ready:
			res.Reason, res.Output = "release check: "+exitReason(rep, code), tail
			return res
		}
		if client, err = dial(); err != nil {
			res.Reason = "SSH connect: " + err.Error()
			return res
		}
	}
	// The backup comes first: no backup, no update.
	if run.Backup && run.Kind != sshtest.OSUpdateCheck {
		key, size, paths, err := h.backupHost(ctx, run, host, client, priv)
		res.BackupInclude, res.BackupExclude = paths.Include, paths.Exclude
		if err != nil {
			client.Close()
			res.BackupStatus = models.OSBackupFailed
			res.Reason = "backup failed, nothing was updated: " + err.Error()
			if run.Kind == sshtest.OSUpdateBackup {
				res.Reason = "backup failed: " + err.Error()
			}
			return res
		}
		res.BackupKey, res.BackupSize, res.BackupStatus = key, &size, models.OSBackupDone
	}
	// A backup-only run is done here: nothing changed on the host.
	if run.Kind == sshtest.OSUpdateBackup {
		client.Close()
		res.Status = models.OSResultSuccess
		return res
	}
	// A chained release updates the packages first, and reboots when that
	// needs it, so the release's own checks pass. If the update fails the
	// release never starts (the host is still rescanned: apt may have changed
	// half the packages).
	chained := run.Kind == sshtest.OSUpdateRelease && run.UpdateFirst
	var pre sshtest.OSUpdateReport
	var preOut string
	if chained {
		err = sshtest.StartOSUpdate(client, priv, run.ID, sshtest.OSUpdatePackages)
		client.Close()
		if err != nil {
			res.Reason = "package update: start: " + err.Error()
			return res
		}
		var code int
		code, preOut, err = pollOSRun(dial, run.ID, osUpdateHostTimeout)
		pre = sshtest.ParseOSUpdateLog(preOut)
		res.Output, res.PkgManager, res.Upgraded = preOut, pre.PkgManager, upgradedCount(sshtest.OSUpdatePackages, pre)
		if err != nil {
			res.Reason = "package update: " + err.Error()
			return res
		}
		if code != 0 || pre.Skip != "" {
			why := pre.Skip
			if why == "" {
				why = exitReason(pre, code)
			}
			res.Reason = "package update failed, the release did not start: " + why
			return h.rescanHost(ctx, run, host, dial, priv, res)
		}
		if pre.RebootRequired {
			if err := rebootHost(dial, priv); err != nil {
				res.Reason = "reboot after the package update: " + err.Error()
				return h.rescanHost(ctx, run, host, dial, priv, res)
			}
			res.Rebooted = true
		}
		if client = waitForSSH(dial, res.Rebooted); client == nil {
			res.Reason = "the host did not answer SSH after the package update"
			return res
		}
	}

	if run.Kind == sshtest.OSUpdateRestore {
		err = h.startRestore(ctx, client, priv, run)
	} else {
		err = sshtest.StartOSUpdate(client, priv, run.ID, run.Kind)
	}
	client.Close()
	if err != nil {
		res.Reason = "start: " + err.Error()
		if chained {
			return h.rescanHost(ctx, run, host, dial, priv, res)
		}
		return res
	}

	limit := osUpdateHostTimeout
	if run.Kind == sshtest.OSUpdateRelease || run.Kind == sshtest.OSUpdateRestore {
		limit = osReleaseTimeout
	}
	exitCode, logTail, err := pollOSRun(dial, run.ID, limit)
	res.Output = logTail
	if chained {
		res.Output = preOut + "\n" + logTail
	}
	if err != nil {
		res.Reason = err.Error()
		return res
	}

	rep := sshtest.ParseOSUpdateLog(logTail)
	res.PkgManager, res.RebootRequired = rep.PkgManager, rep.RebootRequired
	// Upgradable / security are what's still pending once the run is over:
	// the "pending updates" column shows the latest of them.
	res.Upgradable, res.SecurityUpgradable, res.Upgraded = rep.Upgradable, rep.Security, upgradedCount(run.Kind, rep)
	if run.Kind != sshtest.OSUpdateCheck {
		res.Upgradable, res.SecurityUpgradable = rep.Remaining, rep.RemainingSecurity
	}
	if chained {
		// What the run installed is the package update's: a release counts nothing.
		res.PkgManager, res.Upgraded = pre.PkgManager, upgradedCount(sshtest.OSUpdatePackages, pre)
	}
	// A failed update still rescans: apt may have changed half the packages.
	// A skip changed nothing, so it doesn't (unless a chained update ran).
	switch {
	case exitCode != 0:
		res.Reason = exitReason(rep, exitCode)
	case rep.Skip != "":
		res.Status, res.Reason = models.OSResultSkipped, rep.Skip
		if !chained {
			return res
		}
	default:
		res.Status = models.OSResultSuccess
	}

	if res.Status == models.OSResultSuccess && run.Reboot && rep.RebootRequired && run.Kind != sshtest.OSUpdateCheck {
		if err := rebootHost(dial, priv); err != nil {
			res.Reason = "reboot: " + err.Error()
		} else {
			res.Rebooted = true
		}
	}
	return h.rescanHost(ctx, run, host, dial, priv, res)
}

// rescanHost rescans the host (after a reboot, once SSH answers again) so the
// OS map, dependencies and EOL show it as it is now, then deletes a success's
// backup unless the run keeps it.
func (h *sshHandlers) rescanHost(ctx context.Context, run models.OSUpdateRun, host *models.Host, dial func() (*ssh.Client, error),
	priv sshtest.Privilege, res models.OSUpdateResult) models.OSUpdateResult {
	client := waitForSSH(dial, res.Rebooted)
	if client == nil {
		res.Reason = joinReason(res.Reason, "the host did not answer SSH after the update")
		return res
	}
	defer client.Close()
	info, err := sshtest.TestCapture(client, priv.Password)
	if err != nil {
		res.Reason = joinReason(res.Reason, "rescan: "+err.Error())
		return res
	}
	h.recordScan(ctx, host, info, map[string]any{})
	res.OSAfter, res.RebootRequired = info.OS, info.RebootRequired
	// Updated, and the host answered and rescanned: the backup has done its job.
	if res.Status == models.OSResultSuccess && res.BackupStatus == models.OSBackupDone && !run.KeepBackup {
		if c, _, err := h.minio(); err != nil {
			res.Reason = joinReason(res.Reason, "backup kept: "+err.Error())
		} else if err := c.Delete(ctx, res.BackupKey); err != nil {
			res.Reason = joinReason(res.Reason, "backup kept, delete failed: "+err.Error())
		} else {
			res.BackupStatus = models.OSBackupDeleted
		}
	}
	return res
}

// pollOSRun waits for the run's detached script on the host, on a fresh
// session each poll, and returns its exit code and log tail.
func pollOSRun(dial func() (*ssh.Client, error), runID int64, limit time.Duration) (int, string, error) {
	deadline := time.Now().Add(limit)
	logTail := ""
	for {
		if time.Now().After(deadline) {
			return 0, logTail, fmt.Errorf("no result after %s: the update may still be running on the host", limit)
		}
		time.Sleep(osUpdatePoll)
		c, err := dial()
		if err != nil {
			continue // transient: the deadline decides
		}
		done, code, tail, err := sshtest.PollOSUpdate(c, runID)
		c.Close()
		if err != nil && !done {
			return 0, logTail, err
		}
		if logTail = tail; done {
			return code, logTail, nil
		}
	}
}

// rebootHost schedules the reboot and waits out the shutdown.
func rebootHost(dial func() (*ssh.Client, error), priv sshtest.Privilege) error {
	c, err := dial()
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	err = sshtest.Reboot(c, priv)
	c.Close()
	if err != nil {
		return err
	}
	time.Sleep(osUpdateRebootWait)
	return nil
}

// exitReason is why a script failed: its error marker, else its exit code.
func exitReason(rep sshtest.OSUpdateReport, code int) string {
	switch {
	case rep.Error != "":
		return rep.Error
	case code == 0:
		return "the script ended without a result"
	}
	return "exit " + strconv.Itoa(code)
}

// osRestoreLinkTTL is how long the host's download link of a restore works:
// the whole download has to start within it.
const osRestoreLinkTTL = 6 * time.Hour

// startRestore launches the restore of run's source backup on the host,
// detached; the host downloads it straight from MinIO.
func (h *sshHandlers) startRestore(ctx context.Context, client *ssh.Client, priv sshtest.Privilege, run models.OSUpdateRun) error {
	if run.SourceRunID == nil || run.SourceHostID == nil {
		return errors.New("restore without a source backup")
	}
	src, err := store.NewOSUpdateRepo(h.db.SQL).GetResult(ctx, *run.SourceRunID, *run.SourceHostID)
	if err != nil {
		return err
	}
	if src == nil || src.BackupStatus != models.OSBackupDone {
		return errors.New("the source backup is no longer kept")
	}
	members, err := sshtest.RestoreMembers(run.RestorePaths)
	if err != nil {
		return err
	}
	c, _, err := h.minio()
	if err != nil {
		return err
	}
	link, err := c.FetchURL(ctx, src.BackupKey, osRestoreLinkTTL)
	if err != nil {
		return err
	}
	script, err := sshtest.RestoreScript(run.ID, run.RestoreMode, src.BackupKey, link, members)
	if err != nil {
		return err
	}
	return sshtest.StartDetached(client, priv, run.ID, script)
}

// osBackupTimeout bounds one host's backup (a few hundred GB at LAN speed).
var osBackupTimeout = 3 * time.Hour

// minio returns a client on the configured MinIO connection and its
// settings, or an error when it isn't set up. What backups take and keep is
// the feature's: service.LoadBackupDefaults.
func (h *sshHandlers) minio() (*minioclient.Client, minioclient.Settings, error) {
	s, err := minioclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		return nil, s, err
	}
	if !s.Ready() {
		return nil, s, errors.New("MinIO is not configured (Settings › Integrations › MinIO)")
	}
	c, err := minioclient.New(s)
	return c, s, err
}

// backupPaths is what a backup of host in run takes: the run's override,
// else the host's profile, else the defaults (each list on its own).
func (h *sshHandlers) backupPaths(ctx context.Context, run models.OSUpdateRun, hostID int64, d service.BackupDefaults) (service.BackupPaths, error) {
	p, err := store.NewHostBackupProfileRepo(h.db.SQL).Get(ctx, hostID)
	if err != nil {
		return service.BackupPaths{}, err
	}
	paths := service.EffectivePaths(d, service.BackupPaths{Include: p.Include, Exclude: p.Exclude},
		service.BackupPaths{Include: run.BackupInclude, Exclude: run.BackupExclude})
	if run.Kind == sshtest.OSUpdateRestore {
		paths.Include = run.RestorePaths // the undo of a restore in place: just what it overwrites
	}
	return paths, nil
}

// backupHost streams the host's files to MinIO and returns the object key,
// its size and the paths it took. A failed backup leaves no object behind.
func (h *sshHandlers) backupHost(ctx context.Context, run models.OSUpdateRun, host *models.Host, client *ssh.Client, priv sshtest.Privilege) (string, int64, service.BackupPaths, error) {
	d := service.LoadBackupDefaults(ctx, h.db.SQL)
	paths, err := h.backupPaths(ctx, run, host.ID, d)
	if err != nil {
		return "", 0, paths, err
	}
	c, _, err := h.minio()
	if err != nil {
		return "", 0, paths, err
	}
	ext := sshtest.BackupExt(client)
	contentType := "application/gzip"
	if ext == "tar.zst" {
		contentType = "application/zstd"
	}
	key := d.BackupKey(host.OficialSlug, run.ID, ext, time.Now())
	ctx, cancel := context.WithTimeout(ctx, osBackupTimeout)
	defer cancel()
	size, err := streamTo(
		func(r io.Reader) (int64, error) {
			return c.Put(ctx, key, r, contentType)
		},
		func(w io.Writer) error {
			return sshtest.StreamBackup(ctx, client, priv, paths.Include, paths.Exclude, w)
		},
	)
	if err != nil {
		c.Delete(context.WithoutCancel(ctx), key) // a partial upload may have completed
		return "", 0, paths, err
	}
	return key, size, paths, nil
}

// streamTo pipes what produce writes into consume, concurrently: the archive
// is never held whole. A produce error aborts consume (and the reverse).
func streamTo(consume func(io.Reader) (int64, error), produce func(io.Writer) error) (int64, error) {
	pr, pw := io.Pipe()
	type result struct {
		n   int64
		err error
	}
	got := make(chan result, 1)
	go func() {
		n, err := consume(pr)
		pr.CloseWithError(err) // unblocks produce if consume gave up
		got <- result{n, err}
	}()
	perr := produce(pw)
	pw.CloseWithError(perr)
	r := <-got
	if perr != nil {
		return 0, perr
	}
	if r.err != nil {
		return 0, r.err
	}
	return r.n, nil
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
	Backup      bool    `json:"backup"`      // back each host up to MinIO first
	KeepBackup  bool    `json:"keep_backup"` // keep the backup after a success
	// This run's backup paths, absolute; empty = each host's profile, else the defaults.
	BackupInclude []string `json:"backup_include"`
	BackupExclude []string `json:"backup_exclude"`
	// Release only: update the packages first, rebooting if that needs it.
	UpdateFirst bool `json:"update_first"`
}

var osUpdateKinds = map[string]bool{sshtest.OSUpdateCheck: true, sshtest.OSUpdatePackages: true, sshtest.OSUpdateSecurity: true,
	sshtest.OSUpdateRelease: true, sshtest.OSUpdateBackup: true}

// handleStartOSUpdate godoc
//
//	@Summary		Start an OS update run
//	@Description	Admin. Runs kind on host_ids in the background — check: refresh the package lists and count upgrades; packages: every upgrade within the current release; security: security upgrades only; release: the next release (Ubuntu LTS → LTS, Debian N → N+1, Alpine 3.x → 3.x+1), exactly one host, always rebooting, up to 3 h; its checks run before the backup, update_first updates the packages (and reboots if needed) first in the same run, and with MinIO set up it backs up unless backup is false and the host already has a kept backup; backup: back up only, and keep it — concurrency hosts at a time (1–10, default 3), as root (the SSH user if root, sudo with the stored password, else passwordless sudo). reboot reboots a host that needs it after a successful upgrade. Every host is rescanned at the end. Answers 202 with the run; poll GET …/runs/{id}. 404 when a host isn't visible.
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
		jsonError(w, http.StatusBadRequest, "kind must be check, packages, security, release or backup")
		return
	}
	include, err := service.CleanPaths(req.BackupInclude)
	if err == nil {
		req.BackupExclude, err = service.CleanPaths(req.BackupExclude)
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.BackupInclude = include
	// A release upgrade can leave a host unbootable: one host per run, and it
	// always reboots (the release finishes on boot).
	if req.Kind == sshtest.OSUpdateRelease {
		if len(req.HostIDs) != 1 {
			jsonError(w, http.StatusBadRequest, "a release upgrade takes exactly one host")
			return
		}
		req.Concurrency, req.Reboot = 1, true
	}
	req.UpdateFirst = req.UpdateFirst && req.Kind == sshtest.OSUpdateRelease
	// A check changes nothing: no backup. A backup run only backs up, and
	// keeps it. A release without a new backup is checked below.
	_, _, minioErr := h.minio()
	switch req.Kind {
	case sshtest.OSUpdateCheck:
		req.Backup, req.KeepBackup = false, false
	case sshtest.OSUpdateBackup:
		req.Backup, req.KeepBackup, req.Reboot = true, true, false
	}
	if req.Backup && minioErr != nil {
		jsonError(w, http.StatusBadRequest, minioErr.Error())
		return
	}
	if !req.Backup {
		req.BackupInclude, req.BackupExclude = nil, nil
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
	// A release can leave a VM unbootable: with MinIO set up it skips its own
	// backup only when the host already has one to go back to.
	if req.Kind == sshtest.OSUpdateRelease && !req.Backup && minioErr == nil && h.lastFullBackup(r.Context(), req.HostIDs[0]) == nil {
		jsonError(w, http.StatusBadRequest, "this host has no kept backup: a version change backs it up first")
		return
	}
	run := models.OSUpdateRun{Kind: req.Kind, Reboot: req.Reboot && req.Kind != sshtest.OSUpdateCheck, Concurrency: req.Concurrency,
		Backup: req.Backup, KeepBackup: req.Backup && req.KeepBackup, BackupInclude: req.BackupInclude, BackupExclude: req.BackupExclude,
		UpdateFirst: req.UpdateFirst}
	if u := auth.UserFromContext(r.Context()); u != nil {
		run.RequestedBy = &u.ID
	}
	if run.Backup {
		// Re-apply the retention (the setting may have changed); best effort:
		// a key that can't manage lifecycle still backs up.
		if c, _, err := h.minio(); err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			d := service.LoadBackupDefaults(ctx, h.db.SQL)
			if err := c.EnsureRule(ctx, service.BackupRuleID, d.Prefix, d.RetentionDays); err != nil {
				log.Printf("[osupdate] backup lifecycle rule: %v", err)
			}
			cancel()
		}
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
//	@Description	Admin. The 50 newest runs, newest first, with how many hosts are in each status; host_id keeps only the runs that include that host.
//	@Tags			ssh
//	@Produce		json
//	@Param			host_id	query	int	false	"Only runs of this host"
//	@Success		200		{array}	models.OSUpdateRun
//	@Router			/api/ssh/os-updates/runs [get]
func (h *sshHandlers) handleListOSUpdates(w http.ResponseWriter, r *http.Request) {
	runs, err := store.NewOSUpdateRepo(h.db.SQL).ListRuns(r.Context(), 50, hostIDParam(r))
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

// handleOSUpdateConfig godoc
//
//	@Summary		OS update options available
//	@Description	Admin. Whether hosts can be backed up to MinIO before an update (the integration is on and complete), and the paths a backup takes.
//	@Tags			ssh
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/api/ssh/os-updates/config [get]
func (h *sshHandlers) handleOSUpdateConfig(w http.ResponseWriter, r *http.Request) {
	_, _, err := h.minio()
	d := service.LoadBackupDefaults(r.Context(), h.db.SQL)
	jsonOK(w, map[string]any{"backup_available": err == nil, "backup_include": d.Include, "backup_exclude": d.Exclude,
		"retention_days": d.RetentionDays, "backup_prefix": d.Prefix})
}

// osBackupFromPath loads {id}/{host_id}'s result when it has a backup in
// MinIO, or writes 404.
func (h *sshHandlers) osBackupFromPath(w http.ResponseWriter, r *http.Request) *models.OSUpdateResult {
	runID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	hostID, _ := strconv.ParseInt(r.PathValue("host_id"), 10, 64)
	x, err := store.NewOSUpdateRepo(h.db.SQL).GetResult(r.Context(), runID, hostID)
	if err != nil {
		jsonServerError(w, r, "failed to load the result", err)
		return nil
	}
	if x == nil || x.BackupStatus != models.OSBackupDone || x.BackupKey == "" {
		jsonError(w, http.StatusNotFound, "no backup for this host in this run")
		return nil
	}
	return x
}

// osBackupLinkTTL is how long a download link works.
const osBackupLinkTTL = time.Hour

// handleOSBackupLink godoc
//
//	@Summary		Download link of a host's backup
//	@Description	Admin. A link straight to MinIO, valid for one hour, that saves the backup (tar.zst / tar.gz) on the caller's computer. 404 when the result has no backup kept.
//	@Tags			ssh
//	@Produce		json
//	@Param			id		path		int	true	"Run ID"
//	@Param			host_id	path		int	true	"Host ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/runs/{id}/hosts/{host_id}/backup [get]
func (h *sshHandlers) handleOSBackupLink(w http.ResponseWriter, r *http.Request) {
	x := h.osBackupFromPath(w, r)
	if x == nil {
		return
	}
	c, _, err := h.minio()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := c.DownloadURL(r.Context(), x.BackupKey, osBackupLinkTTL)
	if err != nil {
		jsonServerError(w, r, "failed to sign the link", err)
		return
	}
	jsonOK(w, map[string]any{"url": u, "key": x.BackupKey, "size": x.BackupSize, "expires_in": int(osBackupLinkTTL.Seconds())})
}

// handleOSBackupDelete godoc
//
//	@Summary		Delete a host's backup
//	@Description	Admin. Removes the backup from MinIO and marks it deleted. 404 when the result has no backup kept.
//	@Tags			ssh
//	@Param			id		path	int	true	"Run ID"
//	@Param			host_id	path	int	true	"Host ID"
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/runs/{id}/hosts/{host_id}/backup [delete]
func (h *sshHandlers) handleOSBackupDelete(w http.ResponseWriter, r *http.Request) {
	x := h.osBackupFromPath(w, r)
	if x == nil {
		return
	}
	c, _, err := h.minio()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := c.Delete(r.Context(), x.BackupKey); err != nil {
		jsonServerError(w, r, "failed to delete the backup", err)
		return
	}
	if err := store.NewOSUpdateRepo(h.db.SQL).SetBackupStatus(r.Context(), x.RunID, x.HostID, models.OSBackupDeleted); err != nil {
		jsonServerError(w, r, "failed to record the deletion", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

type osRestoreRequest struct {
	RunID  int64    `json:"run_id"`  // the run whose backup to restore
	HostID int64    `json:"host_id"` // its host: the restore goes back there
	Mode   string   `json:"mode"`    // staging | paths
	Paths  []string `json:"paths"`   // absolute; required for paths
}

// handleStartOSRestore godoc
//
//	@Summary		Restore a kept backup onto its host
//	@Description	Admin. Starts a restore run of the backup kept by run_id for host_id, back on that host, in the background. mode staging extracts into /var/lib/bridge-restore/run-<id> (root-only) and changes nothing else; paths may narrow it. mode paths extracts the given absolute paths over /, after backing exactly those paths up to MinIO (kept: the undo). Restoring all of / in place is refused. The host downloads the backup itself from MinIO through a six-hour link, then is rescanned. Answers 202 with the run. 404 when the backup isn't kept or the host isn't visible.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			body	body		osRestoreRequest	true	"Restore"
//	@Success		202		{object}	models.OSUpdateRun
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/restores [post]
func (h *sshHandlers) handleStartOSRestore(w http.ResponseWriter, r *http.Request) {
	var req osRestoreRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Mode != sshtest.RestoreStaging && req.Mode != sshtest.RestorePaths {
		jsonError(w, http.StatusBadRequest, "mode must be staging or paths")
		return
	}
	var paths []string
	for _, p := range req.Paths {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	if req.Mode == sshtest.RestorePaths && len(paths) == 0 {
		jsonError(w, http.StatusBadRequest, "a restore in place needs at least one path")
		return
	}
	if _, err := sshtest.RestoreMembers(paths); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	src, err := store.NewOSUpdateRepo(h.db.SQL).GetResult(r.Context(), req.RunID, req.HostID)
	if err != nil {
		jsonServerError(w, r, "failed to load the backup", err)
		return
	}
	if src == nil || src.BackupStatus != models.OSBackupDone {
		jsonError(w, http.StatusNotFound, "no backup kept for this host in this run")
		return
	}
	if host, err := store.NewHostRepo(h.db.SQL).GetByID(r.Context(), req.HostID); err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if _, _, err := h.minio(); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	run := models.OSUpdateRun{Kind: sshtest.OSUpdateRestore, Concurrency: 1, RestoreMode: req.Mode, RestorePaths: paths,
		SourceRunID: &req.RunID, SourceHostID: &req.HostID,
		// In place, what gets overwritten is backed up first and kept: the undo.
		Backup: req.Mode == sshtest.RestorePaths, KeepBackup: req.Mode == sshtest.RestorePaths}
	if u := auth.UserFromContext(r.Context()); u != nil {
		run.RequestedBy = &u.ID
	}
	repo := store.NewOSUpdateRepo(h.db.SQL)
	if err := repo.CreateRun(r.Context(), &run, []int64{req.HostID}); err != nil {
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

// hostIDParam is the ?host_id= filter (0 when absent or invalid).
func hostIDParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.URL.Query().Get("host_id"), 10, 64)
	return max(id, 0)
}

// osBackupStatCap bounds the MinIO lookups one list makes.
const osBackupStatCap = 20

// handleListOSBackups godoc
//
//	@Summary		Kept host backups
//	@Description	Admin. With host_id: that host's backups still kept in MinIO, newest first. Each is checked in MinIO (up to 20): one the bucket's lifecycle rule already removed is marked deleted and left out. Without host_id: each host's newest kept backup, for the indicators; those older than the retention are left out by age, without asking MinIO. Each has run_kind, the paths it took and expires_at (none when backups are kept until deleted). 404 when the host isn't visible.
//	@Tags			ssh
//	@Produce		json
//	@Param			host_id	query		int	false	"One host's backups"
//	@Success		200		{array}		models.OSUpdateResult
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/os-updates/backups [get]
func (h *sshHandlers) handleListOSBackups(w http.ResponseWriter, r *http.Request) {
	hostID := hostIDParam(r)
	if hostID > 0 {
		if host, err := store.NewHostRepo(h.db.SQL).GetByID(r.Context(), hostID); err != nil || host == nil {
			jsonError(w, http.StatusNotFound, "host not found")
			return
		}
	}
	out, err := h.keptBackups(r.Context(), hostID)
	if err != nil {
		jsonServerError(w, r, "failed to list the backups", err)
		return
	}
	jsonOK(w, out)
}

// keptBackups lists the kept backups with their expiry: the newest of each
// host (hostID 0, expired ones dropped by age), or one host's, each checked
// in MinIO — the lifecycle rule deletes without telling Bridge; a gone one
// is marked deleted. MinIO unreachable: what Bridge knows.
func (h *sshHandlers) keptBackups(ctx context.Context, hostID int64) ([]models.OSUpdateResult, error) {
	repo := store.NewOSUpdateRepo(h.db.SQL)
	rows, err := repo.KeptBackups(ctx, hostID)
	if err != nil {
		return nil, err
	}
	d := service.LoadBackupDefaults(ctx, h.db.SQL)
	c, _, minioErr := h.minio()
	now := time.Now()
	out := make([]models.OSUpdateResult, 0, len(rows))
	for i, x := range rows {
		if x.FinishedAt != nil {
			x.ExpiresAt = d.ExpiresAt(*x.FinishedAt)
		}
		if hostID == 0 {
			if x.ExpiresAt == nil || x.ExpiresAt.After(now) {
				out = append(out, x)
			}
			continue
		}
		if minioErr == nil && i < osBackupStatCap {
			sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			ok, err := c.Exists(sctx, x.BackupKey)
			cancel()
			if err == nil && !ok {
				repo.SetBackupStatus(ctx, x.RunID, x.HostID, models.OSBackupDeleted)
				continue
			}
		}
		out = append(out, x)
	}
	return out, nil
}

// lastFullBackup is the host's newest kept backup a version change can go
// back to: not a restore's undo, which holds only the restored paths.
func (h *sshHandlers) lastFullBackup(ctx context.Context, hostID int64) *models.OSUpdateResult {
	kept, err := h.keptBackups(ctx, hostID)
	if err != nil {
		return nil
	}
	for _, x := range kept {
		if x.RunKind != sshtest.OSUpdateRestore {
			return &x
		}
	}
	return nil
}

// backupProfileView is a host's backup profile with what it resolves to.
type backupProfileView struct {
	Profile   *models.HostBackupProfile `json:"profile"`
	Defaults  service.BackupDefaults    `json:"defaults"`
	Effective service.BackupPaths       `json:"effective"`
}

func (h *sshHandlers) backupProfileView(ctx context.Context, hostID int64) (*backupProfileView, error) {
	p, err := store.NewHostBackupProfileRepo(h.db.SQL).Get(ctx, hostID)
	if err != nil {
		return nil, err
	}
	d := service.LoadBackupDefaults(ctx, h.db.SQL)
	return &backupProfileView{Profile: p, Defaults: d,
		Effective: service.EffectivePaths(d, service.BackupPaths{Include: p.Include, Exclude: p.Exclude}, service.BackupPaths{})}, nil
}

// handleGetBackupProfile godoc
//
//	@Summary		A host's backup profile
//	@Description	Admin. The host's own backup paths (an empty list inherits the default), the defaults (Settings › Host backups) and the paths a backup of the host takes. 404 when the host isn't visible.
//	@Tags			ssh
//	@Produce		json
//	@Param			slug	path		string	true	"Host slug"
//	@Success		200		{object}	backupProfileView
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/backup-profile [get]
func (h *sshHandlers) handleGetBackupProfile(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	v, err := h.backupProfileView(r.Context(), host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to load the backup profile", err)
		return
	}
	jsonOK(w, v)
}

// backupProfileRequest is a partial update: an omitted (or null) list is
// kept; [] goes back to the default; absolute paths otherwise.
type backupProfileRequest struct {
	Include *[]string `json:"include"`
	Exclude *[]string `json:"exclude"`
}

// handleUpdateBackupProfile godoc
//
//	@Summary		Update a host's backup profile
//	@Description	Admin. Partial: an omitted list keeps its value, [] goes back to the default, otherwise absolute paths (spaces or lines). Returns the profile view. 400 on a relative path, 404 when the host isn't visible.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string					true	"Host slug"
//	@Param			body	body		backupProfileRequest	true	"Profile"
//	@Success		200		{object}	backupProfileView
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/api/hosts/{slug}/backup-profile [put]
func (h *sshHandlers) handleUpdateBackupProfile(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	var req backupProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	repo := store.NewHostBackupProfileRepo(h.db.SQL)
	p, err := repo.Get(r.Context(), host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to load the backup profile", err)
		return
	}
	for _, f := range []struct {
		in  *[]string
		out *[]string
	}{{req.Include, &p.Include}, {req.Exclude, &p.Exclude}} {
		if f.in == nil {
			continue
		}
		clean, err := service.CleanPaths(*f.in)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		*f.out = clean
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		p.UpdatedBy = &u.ID
	}
	if err := repo.Upsert(r.Context(), p); err != nil {
		jsonServerError(w, r, "failed to save the backup profile", err)
		return
	}
	v, err := h.backupProfileView(r.Context(), host.ID)
	if err != nil {
		jsonServerError(w, r, "failed to load the backup profile", err)
		return
	}
	jsonOK(w, v)
}
