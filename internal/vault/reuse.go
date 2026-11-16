package vault

import (
	"context"
	"strings"
)

// reuse.go — "this credential is on N VMs". One deploy password or app login
// is often captured on many hosts. The keyed value fingerprint already groups
// identical values without decrypting; this counts, per fingerprint, the
// distinct hosts carrying a shared host-scope secret with it — scoped to the
// hosts the caller can see (secretVisibleSQL), so a reuse count never hints at
// a secret on a host out of reach.

// stampReuse fills ReuseHosts on the views that share their value with another
// visible host. No-op when none of the views carry a fingerprint (an admin
// browsing someone's personal secret has its fingerprint nilled upstream).
func (r *SecretRepo) stampReuse(ctx context.Context, views []SecretView) error {
	seen := map[string]bool{}
	var fps [][]byte
	for i := range views {
		fp := views[i].valueFP
		if fp == nil || seen[string(fp)] {
			continue
		}
		seen[string(fp)] = true
		fps = append(fps, fp)
	}
	if len(fps) == 0 {
		return nil
	}
	counts, err := r.reuseByFingerprint(ctx, fps)
	if err != nil {
		return err
	}
	for i := range views {
		if n := counts[string(views[i].valueFP)]; n > 1 {
			views[i].ReuseHosts = n
		}
	}
	return nil
}

// reuseByFingerprint maps each given value fingerprint to the number of
// distinct hosts (parent_id) whose visible shared host-scope secrets hold that
// value. Fingerprints only on a single host are included at 1; the caller
// (stampReuse) keeps only counts > 1.
func (r *SecretRepo) reuseByFingerprint(ctx context.Context, fps [][]byte) (map[string]int, error) {
	ph := make([]string, len(fps))
	args := make([]any, 0, len(fps)+2)
	for i, fp := range fps {
		ph[i] = "?"
		args = append(args, fp)
	}
	vis, vargs := secretVisibleSQL(ctx, "secrets")
	args = append(args, vargs...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT value_fingerprint, COUNT(DISTINCT parent_id)
		  FROM secrets
		 WHERE scope = 'host' AND visibility = 'shared' AND deleted_at IS NULL
		   AND value_fingerprint IN (`+strings.Join(ph, ",")+`)
		   AND `+vis+`
		 GROUP BY value_fingerprint`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var fp []byte
		var n int
		if err := rows.Scan(&fp, &n); err != nil {
			return nil, err
		}
		out[string(fp)] = n
	}
	return out, rows.Err()
}
