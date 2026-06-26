package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	// ErrBundleTokenInUse is returned by ReissueBundle when a live bundle
	// already owns the supplied token — the caller should renew instead.
	ErrBundleTokenInUse = errors.New("share token already in use")
	// ErrBundleTokenInvalid is returned by ReissueBundle for an empty token.
	ErrBundleTokenInvalid = errors.New("share token is empty")
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
	// NoExpiry, when true, stores a NULL expires_at — the link never expires
	// (redeemable until revoked; never archived by the janitor). Overrides TTL.
	NoExpiry bool
}

// RenewBundleOpts controls a renew/extend operation. Renewing never changes the
// token, so a previously-issued URL keeps working.
type RenewBundleOpts struct {
	// TTL is the new lifetime measured from now. <= 0 defaults to 24h.
	TTL time.Duration
	// NoExpiry, when true, clears the expiry (NULL) instead of setting a window.
	NoExpiry bool
	// MaxViews, when non-nil, replaces the view cap: a value > 0 sets the cap,
	// <= 0 clears it (unlimited within TTL). nil leaves the existing cap as-is.
	MaxViews *int
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
	ExpiresAt     *time.Time       `json:"expires_at"` // nil = never expires
	MaxViews      *int             `json:"max_views,omitempty"`
	ViewCount     int              `json:"view_count"`
	CreatedBy     int64            `json:"created_by"`
	CreatedAt     time.Time        `json:"created_at"`
	RevokedAt     *time.Time       `json:"revoked_at,omitempty"`
	DeletedAt     *time.Time       `json:"deleted_at,omitempty"`
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

// CreateBundle generates a fresh token and persists a new bundle. Item access
// rules and persistence live in buildBundle.
func (r *SecretRepo) CreateBundle(ctx context.Context, actor ActorContext, items []BundleItemInput, opts CreateBundleOpts) (rawToken string, view *BundleView, err error) {
	tok, hash, err := secretshare.GenerateToken()
	if err != nil {
		return "", nil, err
	}
	v, err := r.buildBundle(ctx, actor, hash, items, opts)
	if err != nil {
		return "", nil, err
	}
	return tok, v, nil
}

// ReissueBundle rebuilds a bundle under a caller-supplied raw token — used to
// revive a link whose row was hard-deleted before soft-delete existed, so the
// exact URL the holder still has starts working again. The token's SHA-256
// becomes the row identity, exactly as a generated one would. Rejects a token a
// live bundle already owns (the owner should renew that one instead).
func (r *SecretRepo) ReissueBundle(ctx context.Context, actor ActorContext, rawToken string, items []BundleItemInput, opts CreateBundleOpts) (*BundleView, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrBundleTokenInvalid
	}
	hash := secretshare.HashToken(rawToken)
	var existingID int64
	switch err := r.db.QueryRowContext(ctx,
		`SELECT id FROM share_bundles WHERE token_hash = ?`, hash).Scan(&existingID); {
	case err == nil:
		return nil, ErrBundleTokenInUse
	case errors.Is(err, sql.ErrNoRows):
		// token is free to claim
	default:
		return nil, err
	}
	return r.buildBundle(ctx, actor, hash, items, opts)
}

// buildBundle validates every item's access and persists the bundle + items
// under the given token hash. For secret items it enforces the SAME rule as a
// single-secret share: the actor must be allowed to reveal it. For api_doc
// items it only checks existence (catalog browse is open to any authenticated
// user). Shared by CreateBundle (random token) and ReissueBundle (supplied one).
func (r *SecretRepo) buildBundle(ctx context.Context, actor ActorContext, hash []byte, items []BundleItemInput, opts CreateBundleOpts) (*BundleView, error) {
	if len(items) == 0 {
		return nil, ErrBundleEmpty
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
				return nil, err
			}
			dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
			if !dec.canSeeMetadata {
				return nil, ErrSecretNotFound
			}
			if !dec.canReveal {
				return nil, ErrSecretForbidden
			}
			itemViews = append(itemViews, BundleItemView{Type: it.Type, RefID: it.RefID, Label: v.Name})
		case BundleItemAPIDoc:
			a, err := store.NewAPICatalogRepo(r.db).Get(ctx, it.RefID)
			if err != nil {
				return nil, err
			}
			if a == nil {
				return nil, ErrBundleItemNotFound
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
			return nil, ErrBundleInvalidItem
		}
	}

	// expiresArg is what we store (NULL when NoExpiry); expiresView is echoed
	// back in the owner view (nil = never).
	var expiresArg any
	var expiresView *time.Time
	if !opts.NoExpiry {
		ttl := opts.TTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		t := time.Now().Add(ttl).UTC()
		expiresArg = t.Format(time.RFC3339Nano)
		expiresView = &t
	}

	var maxViews any
	if opts.MaxViews > 0 {
		maxViews = opts.MaxViews
	}
	var passphraseHash any
	hasPass := false
	if opts.Passphrase != "" {
		ph, err := hashPassphrase(opts.Passphrase)
		if err != nil {
			return nil, err
		}
		passphraseHash = ph
		hasPass = true
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id, err := database.InsertReturningID(tx,
		`INSERT INTO share_bundles
			(token_hash, title, expires_at, passphrase_hash, max_views, view_count, created_by)
		 VALUES (?, ?, ?, ?, ?, 0, ?)`,
		hash, opts.Title, expiresArg, passphraseHash, maxViews, actor.UserID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert share bundle: %w", err)
	}
	for i, it := range items {
		var selector any
		if it.Type == BundleItemAPIDoc && it.Selector != nil {
			b, err := json.Marshal(it.Selector)
			if err != nil {
				return nil, err
			}
			selector = string(b)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO share_bundle_items (bundle_id, item_type, ref_id, selector, sort_order)
			 VALUES (?, ?, ?, ?, ?)`,
			id, it.Type, it.RefID, selector, i,
		); err != nil {
			return nil, fmt.Errorf("insert bundle item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	view := &BundleView{
		ID:            id,
		Title:         opts.Title,
		ExpiresAt:     expiresView,
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
	return view, nil
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
		expiresStr sql.NullString
		revoked    sql.NullTime
		deleted    sql.NullTime
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, passphrase_hash, max_views, view_count, expires_at, revoked_at, deleted_at
		   FROM share_bundles WHERE token_hash = ?`, hash,
	).Scan(&id, &title, &passHash, &maxViews, &viewCount, &expiresStr, &revoked, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrShareLinkNotFound
	}
	if err != nil {
		return nil, err
	}

	// Archived (soft-deleted) bundles are not redeemable; collapse to
	// not-found so a guesser can't distinguish archived from never-existed.
	if deleted.Valid {
		return nil, ErrShareLinkNotFound
	}
	if revoked.Valid {
		return nil, ErrShareLinkRevoked
	}
	// NULL expires_at = never expires; only gate when a value is present.
	if expiresStr.Valid {
		expiresAt, _ := parseTime(expiresStr.String)
		if time.Now().After(expiresAt) {
			return nil, ErrShareLinkExpired
		}
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

// bundleViewColumns is the shared owner-facing projection used by every
// metadata read (list / list-for-item / single get). Kept in one place so the
// scan order in scanBundleViews stays in lock-step with the SELECT.
const bundleViewColumns = `id, title, expires_at, passphrase_hash, max_views, view_count, created_by, created_at, revoked_at, deleted_at`

// bundleViewColumnsPrefixed qualifies bundleViewColumns with a table alias for
// joined queries (where bare `id` would be ambiguous against share_bundle_items).
func bundleViewColumnsPrefixed(alias string) string {
	return alias + ".id, " + alias + ".title, " + alias + ".expires_at, " +
		alias + ".passphrase_hash, " + alias + ".max_views, " + alias + ".view_count, " +
		alias + ".created_by, " + alias + ".created_at, " + alias + ".revoked_at, " + alias + ".deleted_at"
}

// scanBundleViews consumes rows selecting bundleViewColumns, then attaches each
// bundle's item labels. Closes rows.
func (r *SecretRepo) scanBundleViews(ctx context.Context, rows *sql.Rows) ([]BundleView, error) {
	defer rows.Close()
	var out []BundleView
	for rows.Next() {
		var (
			v        BundleView
			passHash []byte
			maxViews sql.NullInt64
			revoked  sql.NullTime
			deleted  sql.NullTime
			created  string
			expires  sql.NullString
		)
		if err := rows.Scan(&v.ID, &v.Title, &expires, &passHash, &maxViews,
			&v.ViewCount, &v.CreatedBy, &created, &revoked, &deleted); err != nil {
			return nil, err
		}
		if expires.Valid {
			if t, err := parseTime(expires.String); err == nil {
				v.ExpiresAt = &t
			}
		}
		v.CreatedAt, _ = parseTime(created)
		if revoked.Valid {
			t := revoked.Time
			v.RevokedAt = &t
		}
		if deleted.Valid {
			t := deleted.Time
			v.DeletedAt = &t
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

// ListBundles returns the actor's own bundles (metadata + item labels).
func (r *SecretRepo) ListBundles(ctx context.Context, actor ActorContext) ([]BundleView, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bundleViewColumns+`
		   FROM share_bundles WHERE created_by = ? ORDER BY created_at DESC`, actor.UserID)
	if err != nil {
		return nil, err
	}
	return r.scanBundleViews(ctx, rows)
}

// ListBundlesForItem returns the actor's own bundles that CONTAIN an item of
// the given (itemType, refID) — e.g. every live link exposing a specific API
// doc or secret. It is the server-side, generalized form of the handler's
// ?secret_id= filter, and unlike that filter it also matches multi-item
// bundles (a link carrying the API doc plus attached secrets still counts).
func (r *SecretRepo) ListBundlesForItem(ctx context.Context, actor ActorContext, itemType string, refID int64) ([]BundleView, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT `+bundleViewColumnsPrefixed(`b`)+`
		   FROM share_bundles b
		   JOIN share_bundle_items i ON i.bundle_id = b.id
		  WHERE b.created_by = ? AND i.item_type = ? AND i.ref_id = ?
		  ORDER BY b.created_at DESC`, actor.UserID, itemType, refID)
	if err != nil {
		return nil, err
	}
	return r.scanBundleViews(ctx, rows)
}

// getBundleView reads a single owner-owned bundle as a BundleView, or
// ErrBundleNotFound if it does not exist / belongs to someone else.
func (r *SecretRepo) getBundleView(ctx context.Context, actor ActorContext, bundleID int64) (*BundleView, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bundleViewColumns+`
		   FROM share_bundles WHERE id = ? AND created_by = ?`, bundleID, actor.UserID)
	if err != nil {
		return nil, err
	}
	views, err := r.scanBundleViews(ctx, rows)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, ErrBundleNotFound
	}
	return &views[0], nil
}

// RenewBundle extends a bundle's expiry — and reactivates it if revoked —
// WITHOUT changing the token, so a previously-issued URL keeps resolving.
// Owner-only. Optionally adjusts max_views. view_count is preserved so the
// usage audit stays honest.
func (r *SecretRepo) RenewBundle(ctx context.Context, actor ActorContext, bundleID int64, opts RenewBundleOpts) (*BundleView, error) {
	// NULL clears the expiry (never expires); otherwise set a fresh window.
	var expiresArg any
	if !opts.NoExpiry {
		ttl := opts.TTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		expiresArg = time.Now().Add(ttl).UTC().Format(time.RFC3339Nano)
	}

	var (
		res sql.Result
		err error
	)
	if opts.MaxViews == nil {
		res, err = r.db.ExecContext(ctx,
			`UPDATE share_bundles SET expires_at = ?, revoked_at = NULL, deleted_at = NULL
			  WHERE id = ? AND created_by = ?`,
			expiresArg, bundleID, actor.UserID)
	} else {
		var mv any // NULL clears the cap (unlimited within TTL)
		if *opts.MaxViews > 0 {
			mv = *opts.MaxViews
		}
		res, err = r.db.ExecContext(ctx,
			`UPDATE share_bundles SET expires_at = ?, revoked_at = NULL, deleted_at = NULL, max_views = ?
			  WHERE id = ? AND created_by = ?`,
			expiresArg, mv, bundleID, actor.UserID)
	}
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrBundleNotFound
	}
	return r.getBundleView(ctx, actor, bundleID)
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
