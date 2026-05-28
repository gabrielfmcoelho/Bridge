package vault

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// ShareLinkJanitor periodically deletes share-link rows past their grace
// period. Lives outside the request path so its only failure mode is a
// logged warning — never affects user-facing latency or correctness.
//
// Spec §6 / Plans.md Task 3.4: rows where (expires_at + 7d) < now are
// removed. The 7-day grace lets an operator who shared a link, lost the
// URL, and waited a few days still recover by inspecting the audit log
// — once the row is gone, the audit log row remains (secret_audit_log
// has no FK to share_links, by design).
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
			log.Printf("[share-janitor] startup: deleted %d expired share links", n)
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
					log.Printf("[share-janitor] deleted %d expired share links", n)
				}
			}
		}
	}()
}

// RunOnce executes one sweep. Returns the number of rows deleted.
// Threshold is computed in Go (not the DB) so the same comparison works
// across sqlite and postgres without dialect-specific date arithmetic.
func (j *ShareLinkJanitor) RunOnce(ctx context.Context) (int64, error) {
	threshold := time.Now().Add(-j.grace).UTC().Format(time.RFC3339Nano)
	res, err := j.db.ExecContext(ctx,
		`DELETE FROM secret_share_links WHERE expires_at < ?`,
		threshold,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
