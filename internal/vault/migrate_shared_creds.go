package vault

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// MigrateSharedHostPasswordsEnv gates the startup consolidation run. Set
// MIGRATE_SHARED_HOST_PASSWORDS=1 in the deployment environment to run Phase 1
// once at boot (idempotent + non-destructive, so leaving it on is safe).
const MigrateSharedHostPasswordsEnv = "MIGRATE_SHARED_HOST_PASSWORDS"

// SharedCredStats reports what MigrateSharedHostPasswords did.
type SharedCredStats struct {
	Credentials int // shared credentials created
	Links       int // host_remote_users rows linked (or relinked)
	Skipped     int // single-host groups left as per-host
}

// migrateSharedCredMeta marks audit rows created by the consolidation so
// operators can distinguish them from hand-created credentials.
var migrateSharedCredMeta = []byte(`{"migration":"shared_host_pw"}`)

// MigrateSharedHostPasswords consolidates per-host password secrets that share
// the same (ssh_user, plaintext) into one 'avulso' shared credential referenced
// by host_remote_users.secret_id.
//
// Non-destructive: the per-host `secrets` rows are LEFT IN PLACE — HostGetPassword
// prefers the link, and the old rows are the fallback/safety net (a later,
// separately-gated Phase 2 soft-deletes them). Idempotent: a re-run reuses the
// avulso credential by name and the ON CONFLICT link upsert is a no-op when the
// link already points at it. Single transaction — any failure rolls the whole
// batch back.
//
// Groups of a single host are skipped (already effectively per-host). Grouping
// is by the STORED value, so it reproduces the current vault exactly, including
// hosts whose real server password has since diverged — those can be split back
// to a per-host credential afterwards (the model supports it).
func MigrateSharedHostPasswords(
	ctx context.Context, db *sql.DB, enc *database.Encryptor, actorUserID int64,
) (SharedCredStats, error) {
	var st SharedCredStats
	if actorUserID <= 0 {
		return st, fmt.Errorf("actorUserID must be > 0")
	}
	if enc == nil {
		return st, fmt.Errorf("encryptor is nil")
	}

	// 1. Load live host-password secrets + the host's login user, decrypt, and
	// group by (ssh_user, plaintext). Drain fully before opening the tx.
	rows, err := db.QueryContext(ctx, `
		SELECT s.parent_id, h.ssh_user, s.payload_ciphertext, s.payload_nonce
		  FROM secrets s
		  JOIN hosts h ON h.id = s.parent_id
		 WHERE s.type = 'password' AND s.scope = 'host'
		   AND s.visibility = 'shared' AND s.deleted_at IS NULL
		   AND h.deleted_at IS NULL
		 ORDER BY h.ssh_user, s.parent_id`)
	if err != nil {
		return st, err
	}
	type hostPw struct {
		hostID  int64
		sshUser string
		value   string
	}
	// Preserve group discovery order so credential naming is deterministic.
	groups := map[string][]hostPw{}
	var order []string
	for rows.Next() {
		var hp hostPw
		var ct, nonce []byte
		if err := rows.Scan(&hp.hostID, &hp.sshUser, &ct, &nonce); err != nil {
			rows.Close()
			return st, err
		}
		val, derr := enc.Decrypt(ct, nonce)
		if derr != nil {
			rows.Close()
			return st, fmt.Errorf("host %d decrypt: %w", hp.hostID, derr)
		}
		hp.value = val
		key := hp.sshUser + "\x1f" + val
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], hp)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer tx.Rollback()

	// Disambiguate credential names when one ssh_user has multiple distinct
	// passwords across hosts (e.g. "deploy", "deploy-2").
	nameCount := map[string]int{}

	for _, key := range order {
		members := groups[key]
		if len(members) < 2 {
			st.Skipped++
			continue
		}
		sshUser := members[0].sshUser
		value := members[0].value

		name := sshUser
		if n := nameCount[sshUser]; n > 0 {
			name = fmt.Sprintf("%s-%d", sshUser, n+1)
		}
		nameCount[sshUser]++

		// find-or-create the avulso shared credential by name (idempotency).
		var credID int64
		lookupErr := tx.QueryRowContext(ctx, `
			SELECT id FROM secrets
			 WHERE scope = 'avulso' AND type = 'password' AND visibility = 'shared'
			   AND name = ? AND deleted_at IS NULL`, name).Scan(&credID)
		switch {
		case lookupErr == sql.ErrNoRows:
			ct, nonce, encErr := enc.Encrypt(value)
			if encErr != nil {
				return st, encErr
			}
			credID, err = database.InsertReturningID(tx, `
				INSERT INTO secrets
				  (type, scope, visibility, parent_id, owner_user_id, name,
				   payload_ciphertext, payload_nonce, key_version, created_by)
				VALUES ('password', 'avulso', 'shared', NULL, ?, ?, ?, ?, 1, ?)`,
				actorUserID, name, ct, nonce, actorUserID)
			if err != nil {
				return st, fmt.Errorf("insert credential %q: %w", name, err)
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO secret_audit_log (secret_id, action, actor_user_id, metadata)
				VALUES (?, 'create', ?, ?)`, credID, actorUserID, migrateSharedCredMeta); err != nil {
				return st, fmt.Errorf("audit credential %q: %w", name, err)
			}
			st.Credentials++
		case lookupErr != nil:
			return st, fmt.Errorf("lookup credential %q: %w", name, lookupErr)
		}

		// link each host on the (host_id, username) unique key. The conditional
		// update makes a re-run a no-op when the link already points at credID.
		for _, hp := range members {
			res, err := tx.ExecContext(ctx, `
				INSERT INTO host_remote_users (host_id, username, secret_id)
				VALUES (?, ?, ?)
				ON CONFLICT (host_id, username) DO UPDATE
				   SET secret_id = EXCLUDED.secret_id, updated_at = CURRENT_TIMESTAMP
				 WHERE host_remote_users.secret_id IS DISTINCT FROM EXCLUDED.secret_id`,
				hp.hostID, hp.sshUser, credID)
			if err != nil {
				return st, fmt.Errorf("link host %d: %w", hp.hostID, err)
			}
			if n, _ := res.RowsAffected(); n > 0 {
				st.Links++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return st, err
	}
	return st, nil
}

// RunSharedHostPasswordMigrationIfEnabled runs Phase 1 consolidation at startup
// when MIGRATE_SHARED_HOST_PASSWORDS=1, attributing the audit rows to the same
// maintenance user host-secret writes use. No-op (nil) when the flag is unset.
// Logs the outcome so operators can confirm the gate fired during deploy.
// Mirrors the env-gated pattern of database.maybeDropLegacySecrets, but lives
// here because it needs the vault encryptor (database can't import vault).
func RunSharedHostPasswordMigrationIfEnabled(ctx context.Context, db *database.DB) error {
	if os.Getenv(MigrateSharedHostPasswordsEnv) != "1" {
		return nil
	}
	actor := resolveSystemActor(ctx, db)
	if actor == 0 {
		return fmt.Errorf("shared host-password migration: no user to attribute writes to")
	}
	st, err := MigrateSharedHostPasswords(ctx, db.SQL, db.Encryptor, actor)
	if err != nil {
		return fmt.Errorf("shared host-password migration: %w", err)
	}
	log.Printf("[vault] %s=1: consolidated host passwords — %d credential(s), %d link(s), %d singleton(s) left per-host",
		MigrateSharedHostPasswordsEnv, st.Credentials, st.Links, st.Skipped)
	return nil
}

// MigrateCleanupSharedHostPasswordsEnv gates Phase 2 cleanup. Set
// MIGRATE_CLEANUP_SHARED_HOST_PASSWORDS=1 to soft-delete redundant per-host
// password rows AFTER the link-first resolution change is live in production.
const MigrateCleanupSharedHostPasswordsEnv = "MIGRATE_CLEANUP_SHARED_HOST_PASSWORDS"

// SharedCredCleanupStats reports what CleanupRedundantHostPasswords did.
type SharedCredCleanupStats struct {
	Deleted int // redundant per-host rows soft-deleted
	Kept    int // per-host rows whose value differs from the link (left intact)
}

// CleanupRedundantHostPasswords (Phase 2) soft-deletes per-host password secrets
// that are now redundant — their host resolves the identical value through a
// shared-credential link. Reversible (clear deleted_at) and audited.
//
// Run ONLY after the link-first resolution change is live: deleting these rows
// before HostGetPassword understands links would make those hosts read "no
// password". A per-host row is deleted only when its host has a live link whose
// credential decrypts to the SAME plaintext; a divergent value (which
// break-sharing should already have unlinked) is left intact and counted in
// Kept. Idempotent (already-deleted rows are excluded), single transaction.
func CleanupRedundantHostPasswords(
	ctx context.Context, db *sql.DB, enc *database.Encryptor, actorUserID int64,
) (SharedCredCleanupStats, error) {
	var st SharedCredCleanupStats
	if actorUserID <= 0 {
		return st, fmt.Errorf("actorUserID must be > 0")
	}
	if enc == nil {
		return st, fmt.Errorf("encryptor is nil")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT s.id, s.payload_ciphertext, s.payload_nonce,
		       cred.payload_ciphertext, cred.payload_nonce
		  FROM secrets s
		  JOIN hosts h ON h.id = s.parent_id AND h.deleted_at IS NULL
		  JOIN host_remote_users hru ON hru.host_id = s.parent_id
		       AND hru.username = h.ssh_user AND hru.secret_id IS NOT NULL
		  JOIN secrets cred ON cred.id = hru.secret_id
		       AND cred.type = 'password' AND cred.deleted_at IS NULL
		 WHERE s.type = 'password' AND s.scope = 'host'
		   AND s.visibility = 'shared' AND s.deleted_at IS NULL`)
	if err != nil {
		return st, err
	}
	type candidate struct {
		id       int64
		redundant bool
	}
	var cands []candidate
	for rows.Next() {
		var id int64
		var phCT, phN, credCT, credN []byte
		if err := rows.Scan(&id, &phCT, &phN, &credCT, &credN); err != nil {
			rows.Close()
			return st, err
		}
		phVal, e1 := enc.Decrypt(phCT, phN)
		credVal, e2 := enc.Decrypt(credCT, credN)
		if e1 != nil || e2 != nil {
			rows.Close()
			return st, fmt.Errorf("cleanup decrypt (per-host secret %d): %v / %v", id, e1, e2)
		}
		cands = append(cands, candidate{id: id, redundant: phVal == credVal})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer tx.Rollback()

	for _, c := range cands {
		if !c.redundant {
			st.Kept++
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE secrets SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			c.id); err != nil {
			return st, fmt.Errorf("soft-delete secret %d: %w", c.id, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO secret_audit_log (secret_id, action, actor_user_id, metadata)
			VALUES (?, 'delete', ?, ?)`, c.id, actorUserID, migrateSharedCredMeta); err != nil {
			return st, fmt.Errorf("audit delete %d: %w", c.id, err)
		}
		st.Deleted++
	}

	if err := tx.Commit(); err != nil {
		return st, err
	}
	return st, nil
}

// RunSharedHostPasswordCleanupIfEnabled runs Phase 2 at startup when
// MIGRATE_CLEANUP_SHARED_HOST_PASSWORDS=1. No-op (nil) when unset. Logs outcome.
func RunSharedHostPasswordCleanupIfEnabled(ctx context.Context, db *database.DB) error {
	if os.Getenv(MigrateCleanupSharedHostPasswordsEnv) != "1" {
		return nil
	}
	actor := resolveSystemActor(ctx, db)
	if actor == 0 {
		return fmt.Errorf("shared host-password cleanup: no user to attribute writes to")
	}
	st, err := CleanupRedundantHostPasswords(ctx, db.SQL, db.Encryptor, actor)
	if err != nil {
		return fmt.Errorf("shared host-password cleanup: %w", err)
	}
	log.Printf("[vault] %s=1: cleaned up redundant per-host passwords — %d deleted, %d kept (value differs)",
		MigrateCleanupSharedHostPasswordsEnv, st.Deleted, st.Kept)
	return nil
}
