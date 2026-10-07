package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrBundleNotLive is returned by BundleForSend for a revoked, archived,
// expired or exhausted bundle: mailing a dead link helps no one.
var ErrBundleNotLive = errors.New("share bundle link is not live")

// BundleDelivery is what an emailed share carries: the raw link token, the
// passphrase (if any), and the linked contact's email as the default address.
type BundleDelivery struct {
	Title          string
	Description    string
	ExpiresAt      *time.Time
	Token          string
	Passphrase     string
	RecipientName  string
	RecipientEmail string
}

// BundleForSend decrypts a live bundle's token and passphrase so the server
// can mail them; they never go back to the caller. Owner or admin
// (ErrBundleNotFound otherwise); a pre-v105 bundle is ErrBundleNotRecoverable.
// Every address mailed must be audited with RecordBundleSend first.
func (r *SecretRepo) BundleForSend(ctx context.Context, actor ActorContext, bundleID int64) (*BundleDelivery, error) {
	scope, args := ownerScope(actor)
	var (
		d                                  BundleDelivery
		maxViews                           sql.NullInt64
		viewCount                          int
		expires                            sql.NullString
		revoked, deleted                   sql.NullTime
		tokCT, tokNonce, passCT, passNonce []byte
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT b.title, b.description, b.expires_at, b.revoked_at, b.deleted_at, b.max_views, b.view_count,
		        b.token_cipher, b.token_nonce, b.passphrase_cipher, b.passphrase_nonce,
		        COALESCE(c.name, ''), COALESCE(c.email, '')
		   FROM share_bundles b
		   LEFT JOIN contacts c ON c.id = b.recipient_contact_id AND c.deleted_at IS NULL
		  WHERE b.id = ? AND `+scope, append([]any{bundleID}, args...)...,
	).Scan(&d.Title, &d.Description, &expires, &revoked, &deleted, &maxViews, &viewCount,
		&tokCT, &tokNonce, &passCT, &passNonce, &d.RecipientName, &d.RecipientEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBundleNotFound
	}
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		t, _ := parseTime(expires.String)
		d.ExpiresAt = &t
	}
	if revoked.Valid || deleted.Valid ||
		(d.ExpiresAt != nil && time.Now().After(*d.ExpiresAt)) ||
		(maxViews.Valid && viewCount >= int(maxViews.Int64)) {
		return nil, ErrBundleNotLive
	}
	sec, err := r.decryptBundleSecrets(tokCT, tokNonce, passCT, passNonce)
	if err != nil {
		return nil, err
	}
	d.Token, d.Passphrase = sec.Token, sec.Passphrase
	return &d, nil
}

// RecordBundleSend writes the audit row for mailing a bundle to one address
// (action 'send', actor, sent_to) — no row, no send. undo deletes it again
// when the SMTP delivery then fails, so the log lists only mails that left.
func (r *SecretRepo) RecordBundleSend(ctx context.Context, actor ActorContext, bundleID int64, to string) (undo func() error, err error) {
	var logID int64
	if err := r.db.QueryRowContext(ctx,
		`INSERT INTO share_bundle_access_log (bundle_id, action, actor_user_id, sent_to)
		 VALUES (?, 'send', ?, ?) RETURNING id`, bundleID, actor.UserID, to).Scan(&logID); err != nil {
		return nil, fmt.Errorf("audit send: %w", err)
	}
	return func() error {
		_, err := r.db.ExecContext(context.WithoutCancel(ctx),
			`DELETE FROM share_bundle_access_log WHERE id = ?`, logID)
		return err
	}, nil
}

// decryptBundleSecrets opens a bundle's stored token/passphrase ciphers
// (shared by RevealBundle and BundleForSend). No token cipher = pre-v105.
func (r *SecretRepo) decryptBundleSecrets(tokCT, tokNonce, passCT, passNonce []byte) (*BundleSecrets, error) {
	if len(tokCT) == 0 {
		return nil, ErrBundleNotRecoverable
	}
	out := &BundleSecrets{}
	var err error
	if out.Token, err = r.enc.Decrypt(tokCT, tokNonce); err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	if len(passCT) > 0 {
		if out.Passphrase, err = r.enc.Decrypt(passCT, passNonce); err != nil {
			return nil, fmt.Errorf("decrypt passphrase: %w", err)
		}
	}
	return out, nil
}
