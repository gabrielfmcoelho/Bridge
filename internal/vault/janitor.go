package vault

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// ShareLinkJanitor periodically ARCHIVES share-bundle rows past their grace
// period (soft-delete: sets deleted_at). Lives outside the request path so its
// only failure mode is a logged warning — never affects user-facing latency or
// correctness.
//
// Rows where (expires_at + grace) < now get deleted_at stamped. Soft-delete
// (rather than the original hard DELETE) keeps the token identity and bundle
// items intact, so an owner who lost the URL can revive the exact same link via
// RenewBundle long after it lapsed — the link is archived, never destroyed.
type ShareLinkJanitor struct {
	db       *sql.DB
	interval time.Duration
	grace    time.Duration
	logger   *log.Logger // nil = default
}

// NewShareLinkJanitor wires defaults: 1 hour interval, 7-day grace.
// Callers in tests can poke the fields directly via the helpers below
// or via a tighter-budget construction.
func NewShareLinkJanitor(db *sql.DB) *ShareLinkJanitor {
	return &ShareLinkJanitor{
		db:       db,
		interval: 1 * time.Hour,
		grace:    7 * 24 * time.Hour,
	}
}

// WithInterval / WithGrace are test helpers — useful when a test wants a
// 0-second grace + immediate tick so the run-loop can be exercised under
// a deadline. Production callers don't need them.
func (j *ShareLinkJanitor) WithInterval(d time.Duration) *ShareLinkJanitor {
	j.interval = d
	return j
}
func (j *ShareLinkJanitor) WithGrace(d time.Duration) *ShareLinkJanitor {
	j.grace = d
	return j
}

// Start spawns the run loop. Returns immediately. Cancel ctx to stop.
// Logs each successful sweep with the deleted count so operators have a
// trail; logs errors at the same level so a permanently-broken janitor
// is visible without dedicated metrics.
func (j *ShareLinkJanitor) Start(ctx context.Context) {
	go func() {
		// First sweep right at startup so a freshly-deployed binary
		// doesn't wait a full interval before clearing accumulated debt.
		if n, err := j.RunOnce(ctx); err != nil {
			log.Printf("[share-janitor] startup sweep: %v", err)
		} else if n > 0 {
			log.Printf("[share-janitor] startup: archived %d expired share links", n)
		}

		t := time.NewTicker(j.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n, err := j.RunOnce(ctx)
				if err != nil {
					log.Printf("[share-janitor] sweep: %v", err)
					continue
				}
				if n > 0 {
					log.Printf("[share-janitor] archived %d expired share links", n)
				}
			}
		}
	}()
}

// RunOnce executes one sweep. Returns the number of rows archived.
// Threshold is computed in Go (not the DB) so the comparison needs no
// dialect-specific date arithmetic. The sweep SOFT-deletes (sets deleted_at)
// rather than dropping the row, so the token identity and items survive and an
// owner can revive the exact link via RenewBundle. Already-archived rows are
// skipped (deleted_at IS NULL guard) so deleted_at reflects first-archival time.
func (j *ShareLinkJanitor) RunOnce(ctx context.Context) (int64, error) {
	threshold := time.Now().Add(-j.grace).UTC().Format(time.RFC3339Nano)
	res, err := j.db.ExecContext(ctx,
		`UPDATE share_bundles SET deleted_at = ? WHERE expires_at < ? AND deleted_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339Nano), threshold,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
