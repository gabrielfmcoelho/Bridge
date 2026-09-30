package auth

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// DefaultTokenRateLimit is a token's per-minute request budget when it sets none.
const DefaultTokenRateLimit = 600

// tokenMeter rate-limits API tokens and counts their requests.
//
// ponytail: in-memory fixed 60s windows in this process. Fine while Bridge
// runs as one binary; if it ever runs replicated, move the window to
// Postgres (or Redis) or each replica grants the full budget.
type tokenMeter struct {
	mu      sync.Mutex
	windows map[int64]window
	pending map[int64]int64 // requests not yet flushed to api_token_usage
}

type window struct {
	start time.Time
	count int
}

var meter = &tokenMeter{windows: map[int64]window{}, pending: map[int64]int64{}}

// allow counts one request of token id and reports whether it fits its
// per-minute limit, plus the seconds until the window resets.
func (m *tokenMeter) allow(id int64, limit int, now time.Time) (bool, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending[id]++
	w := m.windows[id]
	if now.Sub(w.start) >= time.Minute {
		w = window{start: now}
	}
	w.count++
	m.windows[id] = w
	retry := int(time.Minute-now.Sub(w.start))/int(time.Second) + 1
	return limit <= 0 || w.count <= limit, retry
}

// take hands the pending counts over (and forgets idle windows).
func (m *tokenMeter) take(now time.Time) map[int64]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.pending
	m.pending = map[int64]int64{}
	for id, w := range m.windows {
		if now.Sub(w.start) > 2*time.Minute {
			delete(m.windows, id)
		}
	}
	return out
}

// FlushTokenUsage writes the counted requests to api_token_usage. Counts that
// fail to write are put back for the next flush.
func FlushTokenUsage(ctx context.Context, db *sql.DB) {
	counts := meter.take(time.Now())
	if len(counts) == 0 {
		return
	}
	if err := store.NewAPITokenRepo(db).AddUsage(ctx, counts); err != nil {
		log.Printf("[auth] flush token usage: %v", err)
		meter.mu.Lock()
		for id, n := range counts {
			meter.pending[id] += n
		}
		meter.mu.Unlock()
	}
}

// StartTokenUsageFlusher flushes usage every minute until ctx ends.
func StartTokenUsageFlusher(ctx context.Context, db *sql.DB) {
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				FlushTokenUsage(context.Background(), db)
				return
			case <-t.C:
				FlushTokenUsage(ctx, db)
			}
		}
	}()
}
