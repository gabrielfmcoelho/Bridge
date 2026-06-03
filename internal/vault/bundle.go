package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/apicatalog"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/secretshare"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Share bundles (Phase D) — a single public link carrying multiple
// heterogeneous items (secrets and/or API docs). The token is the capability
// (its SHA-256 hash is the only persistent identity, like secret_share_links),
// but bundles resolve their contents LIVE at redeem time rather than sealing a
// snapshot at creation. That is the deliberate Phase D/E decision: a valid,
// unexpired token exposes the *current* secret value / API spec; deleting a
// source item simply makes it drop out of the redeemed payload; revoke/expiry
// is the only kill switch.
//
// The crypto reuse is total: secretshare.GenerateToken/HashToken give us the
// token + lookup hash. Secret plaintext is decrypted with the existing master
// key via revealRaw (gated by the valid token, not by an authenticated actor).
// API docs are filtered with apicatalog.Filter against their selector.

const (
	BundleItemSecret = "secret"
	BundleItemAPIDoc = "api_doc"
)

var (
	// ErrBundleEmpty is returned when a create request carries no items.
	ErrBundleEmpty = errors.New("a share bundle must contain at least one item")
	// ErrBundleItemNotFound is returned when an item references a missing
	// secret or API doc at creation time.
	ErrBundleItemNotFound = errors.New("share bundle item not found")
	// ErrBundleInvalidItem is returned for an unknown item_type.
	ErrBundleInvalidItem = errors.New("invalid share bundle item type")
	// ErrBundleNotFound is the owner-facing missing-bundle sentinel (the
	// public redeem path collapses all failures to ErrShareLinkNotFound).
	ErrBundleNotFound = errors.New("share bundle not found")
)

// BundleItemInput is one requested item. Selector applies to api_doc only;
// nil/empty means the whole spec.
type BundleItemInput struct {
	Type     string               `json:"type"`
	RefID    int64                `json:"ref_id"`
	Selector *apicatalog.Selector `json:"selector,omitempty"`
}

// CreateBundleOpts mirrors CreateShareLinkOpts.
type CreateBundleOpts struct {
	Title      string
	TTL        time.Duration
	MaxViews   int
	Passphrase string
}

// BundleItemView is the owner-facing projection of an item (no secret values).
type BundleItemView struct {
	Type     string  `json:"type"`
	RefID    int64   `json:"ref_id"`
	Label    string  `json:"label"`
	Selector *string `json:"selector,omitempty"` // raw JSON for api_doc
}

// BundleView is the owner-facing metadata projection (never the raw token).
type BundleView struct {
	ID            int64            `json:"id"`
	Title         string           `json:"title"`
	ExpiresAt     time.Time        `json:"expires_at"`
	MaxViews      *int             `json:"max_views,omitempty"`
	ViewCount     int              `json:"view_count"`
	CreatedBy     int64            `json:"created_by"`
	CreatedAt     time.Time        `json:"created_at"`
	RevokedAt     *time.Time       `json:"revoked_at,omitempty"`
	HasPassphrase bool             `json:"has_passphrase"`
	Items         []BundleItemView `json:"items"`
}

// BundleSecretItem / BundleAPIDocItem / BundlePayload are the resolved,
// guest-facing contents returned by RedeemBundle.
type BundleSecretItem struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type BundleAPIDocItem struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Version     string          `json:"version,omitempty"`
	ExternalURL string          `json:"external_url,omitempty"`
	Spec        json.RawMessage `json:"spec"`
}

type BundlePayload struct {
	Title   string             `json:"title"`
	Secrets []BundleSecretItem `json:"secrets"`
	APIDocs []BundleAPIDocItem `json:"api_docs"`
}

// CreateBundle validates every item's access, generates a token, and persists
// the bundle + items. For secret items it enforces the SAME rule as a
// single-secret share: the target must be personal and the actor must be
// allowed to reveal it. For api_doc items it only checks existence (catalog
// browse is open to any authenticated user).
func (r *SecretRepo) CreateBundle(ctx context.Context, actor ActorContext, items []BundleItemInput, opts CreateBundleOpts) (rawToken string, view *BundleView, err error) {
	if len(items) == 0 {
		return "", nil, ErrBundleEmpty
	}
	// Validate access for every item and capture owner-facing labels in one
	// pass (the returned view echoes them back so the create UI can confirm
	// what's in the link).
	itemViews := make([]BundleItemView, 0, len(items))
	for _, it := range items {
		switch it.Type {
		case BundleItemSecret:
			// Any secret the actor can reveal may be bundled — personal
			// (owner-only) and shared (viewer+) alike. decideAccess enforces
			// the per-visibility rule.
			v, err := r.loadView(ctx, it.RefID, false)
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
			itemViews = append(itemViews, BundleItemView{Type: it.Type, RefID: it.RefID, Label: v.Name})
		case BundleItemAPIDoc:
			a, err := store.NewAPICatalogRepo(r.db).Get(ctx, it.RefID)
			if err != nil {
				return "", nil, err
			}
			if a == nil {
				return "", nil, ErrBundleItemNotFound
			}
			iv := BundleItemView{Type: it.Type, RefID: it.RefID, Label: a.Name}
			if it.Selector != nil {
				if b, err := json.Marshal(it.Selector); err == nil {
					s := string(b)
					iv.Selector = &s
				}
			}
			itemViews = append(itemViews, iv)
		default:
			return "", nil, ErrBundleInvalidItem
		}
	}

	tok, hash, err := secretshare.GenerateToken()
	if err != nil {
		return "", nil, err
	}

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
		`INSERT INTO share_bundles
			(token_hash, title, expires_at, passphrase_hash, max_views, view_count, created_by)
		 VALUES (?, ?, ?, ?, ?, 0, ?)`,
		hash, opts.Title, expiresAt.Format(time.RFC3339Nano), passphraseHash, maxViews, actor.UserID,
	)
	if err != nil {
		return "", nil, fmt.Errorf("insert share bundle: %w", err)
	}
	for i, it := range items {
		var selector any
		if it.Type == BundleItemAPIDoc && it.Selector != nil {
			b, err := json.Marshal(it.Selector)
			if err != nil {
				return "", nil, err
			}
			selector = string(b)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO share_bundle_items (bundle_id, item_type, ref_id, selector, sort_order)
			 VALUES (?, ?, ?, ?, ?)`,
			id, it.Type, it.RefID, selector, i,
		); err != nil {
			return "", nil, fmt.Errorf("insert bundle item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}

	view = &BundleView{
		ID:            id,
		Title:         opts.Title,
		ExpiresAt:     expiresAt,
		ViewCount:     0,
		CreatedBy:     actor.UserID,
		CreatedAt:     time.Now().UTC(),
		HasPassphrase: hasPass,
		Items:         itemViews,
	}
	if opts.MaxViews > 0 {
		mv := opts.MaxViews
		view.MaxViews = &mv
	}
	return tok, view, nil
}

// RedeemBundle is the public path: token-gated, no actor. Walks the same gate
// ladder as RedeemShareLink, then LIVE-RESOLVES each item against current
// data. Items whose source has since been deleted are skipped (graceful), not
// fatal. Increments view_count on success.
func (r *SecretRepo) RedeemBundle(ctx context.Context, token, passphrase string) (*BundlePayload, error) {
	hash := secretshare.HashToken(token)

	var (
		id         int64
		title      string
		passHash   []byte
		maxViews   sql.NullInt64
		viewCount  int
		expiresStr string
		revoked    sql.NullTime
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, passphrase_hash, max_views, view_count, expires_at, revoked_at
		   FROM share_bundles WHERE token_hash = ?`, hash,
	).Scan(&id, &title, &passHash, &maxViews, &viewCount, &expiresStr, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrShareLinkNotFound
	}
	if err != nil {
		return nil, err
	}

	if revoked.Valid {
		return nil, ErrShareLinkRevoked
	}
	expiresAt, _ := parseTime(expiresStr)
	if time.Now().After(expiresAt) {
		return nil, ErrShareLinkExpired
	}
	if maxViews.Valid && viewCount >= int(maxViews.Int64) {
		return nil, ErrShareLinkMaxViews
	}
	if len(passHash) > 0 {
		if !verifyPassphrase(passphrase, passHash) {
			return nil, ErrShareLinkPassphraseBad
		}
	}

	items, err := r.loadBundleItems(ctx, id)
	if err != nil {
		return nil, err
	}

	payload := &BundlePayload{Title: title, Secrets: []BundleSecretItem{}, APIDocs: []BundleAPIDocItem{}}
	for _, it := range items {
		switch it.itemType {
		case BundleItemSecret:
			v, err := r.loadView(ctx, it.refID, false)
			if err != nil {
				continue // deleted/missing — skip gracefully
			}
			plain, err := r.revealRaw(ctx, it.refID)
			if err != nil {
				continue
			}
			payload.Secrets = append(payload.Secrets, BundleSecretItem{
				Name: v.Name, Type: string(v.Type), Payload: plain,
			})
		case BundleItemAPIDoc:
			a, err := store.NewAPICatalogRepo(r.db).Get(ctx, it.refID)
			if err != nil || a == nil {
				continue
			}
			spec, err := store.NewAPICatalogRepo(r.db).GetSpec(ctx, it.refID)
			if err != nil || spec == "" {
				continue
			}
			sel := apicatalog.Selector{Mode: apicatalog.SelectorAll}
			if it.selector != "" {
				_ = json.Unmarshal([]byte(it.selector), &sel)
			}
			filtered, err := apicatalog.Filter([]byte(spec), sel)
			if err != nil {
				filtered = []byte(spec)
			}
			// "Open externally" prefers the human docs page, then the API
			// base, then the spec-derived server.
			ext := a.DocsURL
			if ext == "" {
				ext = a.BaseURL
			}
			if ext == "" {
				ext = a.ExternalURL
			}
			payload.APIDocs = append(payload.APIDocs, BundleAPIDocItem{
				Name: a.Name, Title: a.Title, Version: a.VersionLabel,
				ExternalURL: ext, Spec: json.RawMessage(filtered),
			})
		}
	}

	if _, err := r.db.ExecContext(ctx,
		`UPDATE share_bundles SET view_count = view_count + 1 WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return payload, nil
}

type bundleItemRow struct {
	itemType string
	refID    int64
	selector string
}

func (r *SecretRepo) loadBundleItems(ctx context.Context, bundleID int64) ([]bundleItemRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT item_type, ref_id, selector FROM share_bundle_items
		  WHERE bundle_id = ? ORDER BY sort_order, id`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bundleItemRow
	for rows.Next() {
		var it bundleItemRow
		var sel sql.NullString
		if err := rows.Scan(&it.itemType, &it.refID, &sel); err != nil {
			return nil, err
		}
		if sel.Valid {
			it.selector = sel.String
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ListBundles returns the actor's own bundles (metadata + item labels).
func (r *SecretRepo) ListBundles(ctx context.Context, actor ActorContext) ([]BundleView, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, expires_at, passphrase_hash, max_views, view_count, created_by, created_at, revoked_at
		   FROM share_bundles WHERE created_by = ? ORDER BY created_at DESC`, actor.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BundleView
	for rows.Next() {
		var (
			v        BundleView
			passHash []byte
			maxViews sql.NullInt64
			revoked  sql.NullTime
			created  string
			expires  string
		)
		if err := rows.Scan(&v.ID, &v.Title, &expires, &passHash, &maxViews,
			&v.ViewCount, &v.CreatedBy, &created, &revoked); err != nil {
			return nil, err
		}
		v.ExpiresAt, _ = parseTime(expires)
		v.CreatedAt, _ = parseTime(created)
		if revoked.Valid {
			t := revoked.Time
			v.RevokedAt = &t
		}
		if maxViews.Valid {
			mv := int(maxViews.Int64)
			v.MaxViews = &mv
		}
		v.HasPassphrase = len(passHash) > 0
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach item labels (small N; fine for an owner list).
	for i := range out {
		items, err := r.loadBundleItems(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Items = r.labelItems(ctx, items)
	}
	return out, nil
}

func (r *SecretRepo) labelItems(ctx context.Context, items []bundleItemRow) []BundleItemView {
	views := make([]BundleItemView, 0, len(items))
	for _, it := range items {
		bv := BundleItemView{Type: it.itemType, RefID: it.refID, Label: "(deleted)"}
		switch it.itemType {
		case BundleItemSecret:
			if v, err := r.loadView(ctx, it.refID, false); err == nil {
				bv.Label = v.Name
			}
		case BundleItemAPIDoc:
			if a, err := store.NewAPICatalogRepo(r.db).Get(ctx, it.refID); err == nil && a != nil {
				bv.Label = a.Name
			}
			if it.selector != "" {
				s := it.selector
				bv.Selector = &s
			}
		}
		views = append(views, bv)
	}
	return views
}

// RevokeBundle marks a bundle revoked (owner-only).
func (r *SecretRepo) RevokeBundle(ctx context.Context, actor ActorContext, bundleID int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE share_bundles SET revoked_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND created_by = ? AND revoked_at IS NULL`,
		bundleID, actor.UserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrBundleNotFound
	}
	return nil
}
