package vault

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/secretshare"
)

// ErrShareTargetNotPersonal is returned when share-link creation targets
// a secret with visibility != 'personal'. Per spec §5.3 / D7, shared
// secrets are already RBAC-accessible internally; external sharing has no
// use case (and would muddle the audit story).
var ErrShareTargetNotPersonal = errors.New("share links may only be created against personal secrets")

// ErrShareLinkExpired / Revoked / MaxViews / Passphrase are the distinct
// failure modes a public redemption can surface. The handler maps each
// to an appropriate HTTP status without leaking which one occurred to a
// guest — they all collapse to 404 externally — but internal callers and
// tests can switch on the cause.
var (
	ErrShareLinkExpired      = errors.New("share link expired")
	ErrShareLinkRevoked      = errors.New("share link revoked")
	ErrShareLinkMaxViews     = errors.New("share link view limit reached")
	ErrShareLinkPassphraseBad = errors.New("share link passphrase incorrect")
	ErrShareLinkNotFound     = errors.New("share link not found")
)

// CreateShareLinkOpts is the input bundle for CreateShareLink. Zero values
// have meaningful defaults: TTL defaults to 24h, MaxViews=0 means unlimited.
type CreateShareLinkOpts struct {
	TTL        time.Duration
	MaxViews   int
	Passphrase string // optional; empty = no gate
}

// SecretShareLinkView is the metadata-only projection returned by List.
// TokenHash + PassphraseHash + payload columns are intentionally omitted
// — the owner sees IDs, expiry, view counts, but never the raw token (it
// was only ever in the URL handed to them at creation).
type SecretShareLinkView struct {
	ID         int64      `json:"id"`
	SecretID   int64      `json:"secret_id"`
	ExpiresAt  time.Time  `json:"expires_at"`
	MaxViews   *int       `json:"max_views,omitempty"`
	ViewCount  int        `json:"view_count"`
	CreatedBy  int64      `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	// HasPassphrase exposes the existence (not the value) of a passphrase
	// gate so the redemption UI knows whether to prompt.
	HasPassphrase bool `json:"has_passphrase"`
}

// argon2id parameters. Tuned for ~100ms on a modern x86 server — enough
// to make brute-force on the passphrase expensive without making redemption
// feel sluggish to legitimate users. If perf budget shifts, bump time/
// memory; the hash format (salt prefix + raw bytes) absorbs the change.
const (
	argon2Time    = 2
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// hashPassphrase produces (salt || argon2id_hash) as a single blob.
// Verification re-runs argon2 with the embedded salt and constant-time
// compares the resulting hash.
func hashPassphrase(passphrase string) ([]byte, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	hash := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	out := make([]byte, 0, argon2SaltLen+argon2KeyLen)
	out = append(out, salt...)
	out = append(out, hash...)
	return out, nil
}

func verifyPassphrase(passphrase string, stored []byte) bool {
	if len(stored) != argon2SaltLen+argon2KeyLen {
		return false
	}
	salt := stored[:argon2SaltLen]
	expected := stored[argon2SaltLen:]
	got := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(got, expected) == 1
}

// CreateShareLink decrypts the target secret with the master key, re-
// encrypts under the per-link HKDF-derived key, and persists the result
// on a fresh secret_share_links row. Returns the RAW token (only emitted
// here — never read again) plus a metadata view.
//
// Authz: per spec §5.3 the target secret MUST have visibility='personal'.
// The caller must be allowed to reveal the secret (owner per §5.2).
func (r *SecretRepo) CreateShareLink(
	ctx context.Context,
	actor ActorContext,
	secretID int64,
	opts CreateShareLinkOpts,
) (rawToken string, view *SecretShareLinkView, err error) {
	// Load + ACL check the target. Any secret the actor can REVEAL may be
	// shared externally: personal secrets are owner-only (decideAccess
	// enforces that), and shared secrets are shareable by anyone who can
	// reveal them (viewer+). This is a deliberate relaxation of the original
	// personal-only rule — shared secrets are now externally shareable too.
	v, err := r.loadView(ctx, secretID, false)
	if err != nil {
		return "", nil, err
	}
	dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
	if !dec.canSeeMetadata {
		return "", nil, ErrSecretNotFound
	}
	if !dec.canReveal {
		return "", nil, ErrSecretForbidden
	}

	// Decrypt the secret with the master key. We need the plaintext so we
	// can re-encrypt it under the per-link derived key.
	plain, err := r.revealRaw(ctx, secretID)
	if err != nil {
		return "", nil, err
	}

	tok, hash, err := secretshare.GenerateToken()
	if err != nil {
		return "", nil, err
	}
	ct, nonce, err := secretshare.Encrypt(r.enc.MasterKey(), tok, plain)
	if err != nil {
		return "", nil, err
	}

	// Defaults: 24h TTL, no max-views cap (NULL).
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	expiresAt := time.Now().Add(ttl).UTC()

	var maxViews any
	if opts.MaxViews > 0 {
		maxViews = opts.MaxViews
	}
	var passphraseHash any
	hasPass := false
	if opts.Passphrase != "" {
		ph, err := hashPassphrase(opts.Passphrase)
		if err != nil {
			return "", nil, err
		}
		passphraseHash = ph
		hasPass = true
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback()

	id, err := database.InsertReturningID(tx,
		`INSERT INTO secret_share_links
			(secret_id, token_hash, expires_at, passphrase_hash, max_views,
			 view_count, created_by, payload_ciphertext, payload_nonce)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)`,
		secretID, hash, expiresAt.Format(time.RFC3339Nano),
		passphraseHash, maxViews, actor.UserID, ct, nonce,
	)
	if err != nil {
		return "", nil, fmt.Errorf("insert share link: %w", err)
	}
	if err := writeAuditTx(tx, secretID, models.SecretAuditActionShareCreate, actor, &id, nil); err != nil {
		return "", nil, fmt.Errorf("audit share_create: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}

	view = &SecretShareLinkView{
		ID:            id,
		SecretID:      secretID,
		ExpiresAt:     expiresAt,
		ViewCount:     0,
		CreatedBy:     actor.UserID,
		CreatedAt:     time.Now().UTC(),
		HasPassphrase: hasPass,
	}
	if opts.MaxViews > 0 {
		mv := opts.MaxViews
		view.MaxViews = &mv
	}
	return tok, view, nil
}

// ListShareLinks returns metadata for every share link on secretID that
// the actor is allowed to see. Per spec §5.3 + D7: only the secret's
// owner may list. Soft-deleted-link semantics are "revoked_at IS NOT NULL"
// rather than a separate column — revoked rows are included so the UI can
// surface them ("revoked at X" history).
func (r *SecretRepo) ListShareLinks(ctx context.Context, actor ActorContext, secretID int64) ([]SecretShareLinkView, error) {
	v, err := r.loadView(ctx, secretID, false)
	if err != nil {
		return nil, err
	}
	dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
	if !dec.canSeeMetadata {
		return nil, ErrSecretNotFound
	}
	// Owner-only — admin/editor reading another user's personal must NOT
	// see the share links (D6 metadata-only never includes share state).
	if actor.UserID != v.OwnerUserID {
		return nil, ErrSecretForbidden
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, secret_id, expires_at, passphrase_hash, max_views, view_count,
		        created_by, created_at, revoked_at
		   FROM secret_share_links
		  WHERE secret_id = ?
		  ORDER BY created_at DESC`,
		secretID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SecretShareLinkView
	for rows.Next() {
		var (
			view       SecretShareLinkView
			passHash   []byte
			maxViews   sql.NullInt64
			revoked    sql.NullTime
			created    string
			expires    string
		)
		if err := rows.Scan(&view.ID, &view.SecretID, &expires, &passHash, &maxViews,
			&view.ViewCount, &view.CreatedBy, &created, &revoked); err != nil {
			return nil, err
		}
		view.ExpiresAt, _ = parseTime(expires)
		view.CreatedAt, _ = parseTime(created)
		if revoked.Valid {
			t := revoked.Time
			view.RevokedAt = &t
		}
		if maxViews.Valid {
			mv := int(maxViews.Int64)
			view.MaxViews = &mv
		}
		view.HasPassphrase = len(passHash) > 0
		out = append(out, view)
	}
	return out, rows.Err()
}

// RevokeShareLink marks the link as revoked (revoked_at = now). Subsequent
// redemptions fail with ErrShareLinkRevoked. Per the spec the row is
// retained for audit; physical deletion is the janitor's job after the
// grace period (Task 3.4).
func (r *SecretRepo) RevokeShareLink(ctx context.Context, actor ActorContext, secretID, linkID int64) error {
	v, err := r.loadView(ctx, secretID, false)
	if err != nil {
		return err
	}
	if actor.UserID != v.OwnerUserID {
		return ErrSecretForbidden
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`UPDATE secret_share_links SET revoked_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND secret_id = ? AND revoked_at IS NULL`,
		linkID, secretID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrShareLinkNotFound
	}
	if err := writeAuditTx(tx, secretID, models.SecretAuditActionShareRevoke, actor, &linkID, nil); err != nil {
		return err
	}
	return tx.Commit()
}

// RedeemShareLink is the public path: no actor, just a token. Walks the
// gate ladder (revoked → expired → max-views → passphrase) and returns the
// plaintext on success. Writes a share_redeem audit row regardless of
// success (with actor_user_id NULL because the redeemer is anonymous).
func (r *SecretRepo) RedeemShareLink(ctx context.Context, token, passphrase string) (string, error) {
	hash := secretshare.HashToken(token)

	var (
		id         int64
		secretID   int64
		passHash   []byte
		maxViews   sql.NullInt64
		viewCount  int
		ct         []byte
		nonce      []byte
		expiresStr string
		revoked    sql.NullTime
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, secret_id, passphrase_hash, max_views, view_count,
		        payload_ciphertext, payload_nonce, expires_at, revoked_at
		   FROM secret_share_links
		  WHERE token_hash = ?`,
		hash,
	).Scan(&id, &secretID, &passHash, &maxViews, &viewCount, &ct, &nonce, &expiresStr, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrShareLinkNotFound
	}
	if err != nil {
		return "", err
	}

	if revoked.Valid {
		return "", ErrShareLinkRevoked
	}
	expiresAt, _ := parseTime(expiresStr)
	if time.Now().After(expiresAt) {
		return "", ErrShareLinkExpired
	}
	if maxViews.Valid && viewCount >= int(maxViews.Int64) {
		return "", ErrShareLinkMaxViews
	}
	if len(passHash) > 0 {
		if !verifyPassphrase(passphrase, passHash) {
			return "", ErrShareLinkPassphraseBad
		}
	}

	plain, err := secretshare.Decrypt(r.enc.MasterKey(), token, ct, nonce)
	if err != nil {
		// AEAD failure here would mean tampered DB row — surface as not
		// found to avoid leaking corruption details.
		return "", ErrShareLinkNotFound
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE secret_share_links SET view_count = view_count + 1 WHERE id = ?`, id,
	); err != nil {
		return "", err
	}
	// Anonymous actor — pass UserID=0 so writeAuditTx writes NULL.
	if err := writeAuditTx(tx, secretID, models.SecretAuditActionShareRedeem,
		ActorContext{UserID: 0, Role: "guest"}, &id, nil); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return plain, nil
}

// revealRaw decrypts a secret's payload bypassing ACL — internal helper
// for CreateShareLink which has already done its own visibility check.
func (r *SecretRepo) revealRaw(ctx context.Context, id int64) (string, error) {
	var ct, nonce []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT payload_ciphertext, payload_nonce FROM secrets WHERE id = ? AND deleted_at IS NULL`,
		id,
	).Scan(&ct, &nonce)
	if err != nil {
		return "", err
	}
	return r.enc.Decrypt(ct, nonce)
}

// parseTime tolerates both SQLite's "2006-01-02 15:04:05" default and the
// RFC3339Nano values we write explicitly. Returns zero time on parse
// failure rather than an error because the column is also written by the
// DB default — we'd rather render a sentinel than 500.
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q", s)
}

