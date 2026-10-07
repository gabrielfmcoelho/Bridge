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
	outlineclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/outline"
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
	// BundleItemWikiDoc / BundleItemWikiCollection reference an Outline document
	// or collection by its string UUID (carried in ref_key, not the BIGINT
	// ref_id). Both resolve LIVE at redeem via the shared Outline client.
	BundleItemWikiDoc        = "wiki_doc"
	BundleItemWikiCollection = "wiki_collection"
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
	// ErrBundleItemForbidden is returned when the actor tries to bundle a wiki
	// doc/collection outside the collections they may share (common + project).
	ErrBundleItemForbidden = errors.New("share bundle item forbidden")
	// ErrBundleWikiUnavailable is returned when a wiki item is requested but the
	// Outline integration is disabled or unconfigured.
	ErrBundleWikiUnavailable = errors.New("wiki integration unavailable")
	// ErrBundleNotRecoverable is returned by RevealBundle for a bundle created
	// before the token/passphrase were stored encrypted (v105).
	ErrBundleNotRecoverable = errors.New("share bundle link is not recoverable")
	// ErrBundleRecipientInvalid is returned when recipient_contact_id names no
	// contact.
	ErrBundleRecipientInvalid = errors.New("share bundle recipient contact not found")
)

// BundleItemInput is one requested item. Selector applies to api_doc only;
// nil/empty means the whole spec. RefKey carries the string UUID for wiki
// items (wiki_doc / wiki_collection), whose Outline IDs don't fit the numeric
// RefID; secret / api_doc items leave it empty and use RefID.
type BundleItemInput struct {
	Type     string               `json:"type"`
	RefID    int64                `json:"ref_id"`
	RefKey   string               `json:"ref_key,omitempty"`
	Selector *apicatalog.Selector `json:"selector,omitempty"`
}

// CreateBundleOpts mirrors CreateShareLinkOpts.
type CreateBundleOpts struct {
	Title string
	// Description is an optional free-text note shown on the guest reveal page.
	Description string
	TTL         time.Duration
	MaxViews    int
	Passphrase  string
	// NoExpiry, when true, stores a NULL expires_at — the link never expires
	// (redeemable until revoked; never archived by the janitor). Overrides TTL.
	NoExpiry bool
	// RecipientContactID / RecipientLabel say who the link is for: a contact,
	// or free text when the recipient isn't one. Both optional.
	RecipientContactID *int64
	RecipientLabel     string
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
	RefKey   string  `json:"ref_key,omitempty"` // string UUID for wiki items
	Label    string  `json:"label"`
	Selector *string `json:"selector,omitempty"` // raw JSON for api_doc
}

// BundleView is the owner-facing metadata projection (never the raw token).
type BundleView struct {
	ID            int64            `json:"id"`
	Title         string           `json:"title"`
	Description   string           `json:"description"`
	ExpiresAt     *time.Time       `json:"expires_at"` // nil = never expires
	MaxViews      *int             `json:"max_views,omitempty"`
	ViewCount     int              `json:"view_count"`
	CreatedBy     int64            `json:"created_by"`
	CreatedAt     time.Time        `json:"created_at"`
	RevokedAt     *time.Time       `json:"revoked_at,omitempty"`
	DeletedAt     *time.Time       `json:"deleted_at,omitempty"`
	HasPassphrase bool             `json:"has_passphrase"`
	Items         []BundleItemView `json:"items"`
	// RecipientContactID / RecipientName come from the linked contact;
	// RecipientLabel is the free-text recipient.
	RecipientContactID *int64     `json:"recipient_contact_id,omitempty"`
	RecipientName      string     `json:"recipient_name,omitempty"`
	RecipientEmail     string     `json:"recipient_email,omitempty"` // the contact's, default for a send
	RecipientLabel     string     `json:"recipient_label"`
	CreatedByName      string     `json:"created_by_name"`
	LastAccessAt       *time.Time `json:"last_access_at,omitempty"` // last guest redemption
	// Recoverable is true when the token (and passphrase) are stored encrypted,
	// so an admin can reveal the link again.
	Recoverable bool `json:"recoverable"`
}

// BundleSecrets is what RevealBundle hands an admin: the raw token and the
// passphrase ("" when the link has none).
type BundleSecrets struct {
	Token      string `json:"token"`
	Passphrase string `json:"passphrase"`
}

// BundleSecretItem / BundleAPIDocItem / BundlePayload are the resolved,
// guest-facing contents returned by RedeemBundle.
type BundleSecretItem struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	// Key is set when the secret is an API access key's (api_keys.secret_id):
	// what the guest needs to use it — client id, API, scopes, token URL.
	Key *BundleKeyInfo `json:"key,omitempty"`
}

// BundleKeyInfo describes the API key behind a shared secret. Scopes carry
// the key's names; the public redeem handler fills Kind/Description from the
// API's GET /escopos and TokenURL from the keycloak_apis settings
// (best-effort, both may stay empty).
type BundleKeyInfo struct {
	APIID              int64             `json:"-"`
	ClientID           string            `json:"client_id,omitempty"`
	APIName            string            `json:"api_name"`
	APIBaseURL         string            `json:"api_base_url,omitempty"`
	Scopes             []BundleScopeInfo `json:"scopes"`
	RateLimitPerMinute *int              `json:"rate_limit_per_minute,omitempty"`
	TokenURL           string            `json:"token_url,omitempty"`
}

// BundleScopeInfo is one scope a shared key carries. Kind is "route" or
// "modifier" (empty when the API's catalogue couldn't be read).
type BundleScopeInfo struct {
	Name        string `json:"name"`
	Kind        string `json:"kind,omitempty"`
	Description string `json:"description,omitempty"`
}

type BundleAPIDocItem struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Version     string          `json:"version,omitempty"`
	ExternalURL string          `json:"external_url,omitempty"`
	Spec        json.RawMessage `json:"spec" swaggertype:"object"`
}

// BundleWikiDoc is one resolved Outline document — raw markdown plus a nested
// child tree (populated only when it belongs to a shared collection). The
// markdown is delivered raw; the frontend renders it through a sanitizing
// pipeline (the redeem page is public/anonymous).
type BundleWikiDoc struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Emoji     string          `json:"emoji,omitempty"`
	Markdown  string          `json:"markdown"`
	BrowseURL string          `json:"browse_url,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
	UpdatedBy string          `json:"updated_by,omitempty"`
	Children  []BundleWikiDoc `json:"children,omitempty"`
}

// BundleWikiItem is one resolved wiki bundle item: a single document (Kind
// "doc", one root in Documents) or a whole collection (Kind "collection", the
// collection's document tree in Documents). Truncated is set when a large
// collection's fan-out was capped.
type BundleWikiItem struct {
	Kind           string          `json:"kind"` // "doc" | "collection"
	Title          string          `json:"title"`
	CollectionName string          `json:"collection_name,omitempty"`
	Description    string          `json:"description,omitempty"` // collection description
	BrowseURL      string          `json:"browse_url,omitempty"`
	Documents      []BundleWikiDoc `json:"documents"`
	Truncated      bool            `json:"truncated,omitempty"`
}

type BundlePayload struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Secrets     []BundleSecretItem `json:"secrets"`
	APIDocs     []BundleAPIDocItem `json:"api_docs"`
	Wiki        []BundleWikiItem   `json:"wiki"`
}

// RedeemMeta carries the anonymous network metadata captured when a bundle is
// redeemed. It identifies the request, never the requester (bundles are public
// links). Recorded best-effort into share_bundle_access_log on a successful
// reveal.
type RedeemMeta struct {
	RemoteIP  string
	UserAgent string
}

// BundleAccessEntry is one owner-facing access-log row. Action "redeem" is a
// guest opening the link: when, from where (best-effort IP), with what
// user-agent, and whether a passphrase gated it — no identity is recorded.
// Action "reveal" is an admin reading the link and passphrase back; ActorName
// says who.
type BundleAccessEntry struct {
	AccessedAt     time.Time `json:"accessed_at"`
	RemoteIP       string    `json:"remote_ip"`
	UserAgent      string    `json:"user_agent"`
	UsedPassphrase bool      `json:"used_passphrase"`
	Action         string    `json:"action"`
	ActorName      string    `json:"actor_name,omitempty"`
	SentTo         string    `json:"sent_to,omitempty"` // action 'send': the address mailed
}

// ownerScope is the WHERE fragment limiting share_bundles rows to the ones
// the actor may see and act on: admins all of them, everyone else their own.
func ownerScope(actor ActorContext) (string, []any) {
	if actor.Role == "admin" {
		return "TRUE", nil
	}
	return "created_by = ?", []any{actor.UserID}
}

// checkBundleScope answers ErrBundleNotFound when the bundle is missing or
// outside the actor's ownerScope.
func (r *SecretRepo) checkBundleScope(ctx context.Context, actor ActorContext, bundleID int64) error {
	scope, args := ownerScope(actor)
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM share_bundles WHERE id = ? AND `+scope, append([]any{bundleID}, args...)...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBundleNotFound
	}
	return err
}

// CreateBundle generates a fresh token and persists a new bundle. Item access
// rules and persistence live in buildBundle.
func (r *SecretRepo) CreateBundle(ctx context.Context, actor ActorContext, items []BundleItemInput, opts CreateBundleOpts) (rawToken string, view *BundleView, err error) {
	tok, hash, err := secretshare.GenerateToken()
	if err != nil {
		return "", nil, err
	}
	v, err := r.buildBundle(ctx, actor, tok, hash, items, opts)
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
	return r.buildBundle(ctx, actor, rawToken, hash, items, opts)
}

// buildBundle validates every item's access and persists the bundle + items
// under the given token hash. For secret items it enforces the SAME rule as a
// single-secret share: the actor must be allowed to reveal it. For api_doc
// items it only checks existence (catalog browse is open to any authenticated
// user). Shared by CreateBundle (random token) and ReissueBundle (supplied one).
// The raw token and passphrase are also stored encrypted so an admin can reveal
// them later (RevealBundle).
func (r *SecretRepo) buildBundle(ctx context.Context, actor ActorContext, rawToken string, hash []byte, items []BundleItemInput, opts CreateBundleOpts) (*BundleView, error) {
	// Validate access for every item and capture owner-facing labels (the view
	// echoes them back so the create UI can confirm what's in the link). Shared
	// with UpdateBundleItems; also enforces the non-empty rule.
	itemViews, err := r.validateBundleItems(ctx, actor, items)
	if err != nil {
		return nil, err
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
	var passphraseHash, passCipher, passNonce any
	hasPass := false
	if opts.Passphrase != "" {
		ph, err := hashPassphrase(opts.Passphrase)
		if err != nil {
			return nil, err
		}
		passphraseHash = ph
		hasPass = true
		ct, nonce, err := r.enc.Encrypt(opts.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("encrypt passphrase: %w", err)
		}
		passCipher, passNonce = ct, nonce
	}
	tokCipher, tokNonce, err := r.enc.Encrypt(rawToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	var recipientID any
	if opts.RecipientContactID != nil && *opts.RecipientContactID > 0 {
		var one int
		switch err := r.db.QueryRowContext(ctx,
			`SELECT 1 FROM contacts WHERE id = ?`, *opts.RecipientContactID).Scan(&one); {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrBundleRecipientInvalid
		case err != nil:
			return nil, err
		}
		recipientID = *opts.RecipientContactID
	}
	recipientLabel := strings.TrimSpace(opts.RecipientLabel)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id, err := database.InsertReturningID(tx,
		`INSERT INTO share_bundles
			(token_hash, title, description, expires_at, passphrase_hash, max_views, view_count, created_by,
			 token_cipher, token_nonce, passphrase_cipher, passphrase_nonce, recipient_contact_id, recipient_label)
		 VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?)`,
		hash, opts.Title, opts.Description, expiresArg, passphraseHash, maxViews, actor.UserID,
		tokCipher, tokNonce, passCipher, passNonce, recipientID, recipientLabel,
	)
	if err != nil {
		return nil, fmt.Errorf("insert share bundle: %w", err)
	}
	if err := insertBundleItems(ctx, tx, id, items); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	view := &BundleView{
		ID:             id,
		Title:          opts.Title,
		Description:    opts.Description,
		ExpiresAt:      expiresView,
		ViewCount:      0,
		CreatedBy:      actor.UserID,
		CreatedAt:      time.Now().UTC(),
		HasPassphrase:  hasPass,
		Items:          itemViews,
		RecipientLabel: recipientLabel,
		Recoverable:    true,
	}
	if recipientID != nil {
		cid := *opts.RecipientContactID
		view.RecipientContactID = &cid
	}
	if opts.MaxViews > 0 {
		mv := opts.MaxViews
		view.MaxViews = &mv
	}
	return view, nil
}

// validateBundleItems checks every requested item's access and returns the
// owner-facing labels. Enforces the non-empty rule and per-type ACL (secrets:
// revealable; api_doc: exists; wiki: inside a shareable collection). Shared by
// buildBundle (create/reissue) and UpdateBundleItems (edit).
func (r *SecretRepo) validateBundleItems(ctx context.Context, actor ActorContext, items []BundleItemInput) ([]BundleItemView, error) {
	if len(items) == 0 {
		return nil, ErrBundleEmpty
	}
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
			if !dec.canReveal || !revealPermitted(ctx, r.db, actor, v.Scope) {
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
		case BundleItemWikiDoc, BundleItemWikiCollection:
			// The actor may only bundle wiki content inside collections they can
			// legitimately share (common + project-linked). authorizeWikiRef
			// enforces that and returns the owner-facing label.
			label, err := r.authorizeWikiRef(ctx, actor, it.Type, it.RefKey)
			if err != nil {
				return nil, err
			}
			itemViews = append(itemViews, BundleItemView{Type: it.Type, RefKey: it.RefKey, Label: label})
		default:
			return nil, ErrBundleInvalidItem
		}
	}
	return itemViews, nil
}

// insertBundleItems writes the item rows for a bundle within an open tx (sort
// order follows slice order). Shared by create and edit.
func insertBundleItems(ctx context.Context, tx *sql.Tx, bundleID int64, items []BundleItemInput) error {
	for i, it := range items {
		var selector any
		if it.Type == BundleItemAPIDoc && it.Selector != nil {
			b, err := json.Marshal(it.Selector)
			if err != nil {
				return err
			}
			selector = string(b)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO share_bundle_items (bundle_id, item_type, ref_id, ref_key, selector, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			bundleID, it.Type, it.RefID, it.RefKey, selector, i,
		); err != nil {
			return fmt.Errorf("insert bundle item: %w", err)
		}
	}
	return nil
}

// UpdateBundleItems replaces the item set of an existing bundle WITHOUT changing
// the token, so a previously-issued /share/{token} URL keeps working and now
// exposes the new items (bundles resolve content live at redeem). Owner (or
// admin) only. Every new item is re-validated for access exactly as at
// creation; the bundle's expiry/passphrase/view_count/token are all preserved.
func (r *SecretRepo) UpdateBundleItems(ctx context.Context, actor ActorContext, bundleID int64, items []BundleItemInput) (*BundleView, error) {
	// Scope check first — also yields ErrBundleNotFound for a foreign/missing
	// bundle, so we never surface item errors for a bundle the actor can't see.
	if err := r.checkBundleScope(ctx, actor, bundleID); err != nil {
		return nil, err
	}

	if _, err := r.validateBundleItems(ctx, actor, items); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM share_bundle_items WHERE bundle_id = ?`, bundleID); err != nil {
		return nil, fmt.Errorf("clear bundle items: %w", err)
	}
	if err := insertBundleItems(ctx, tx, bundleID, items); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getBundleView(ctx, actor, bundleID)
}

// RedeemBundle is the public path: token-gated, no actor. Walks the same gate
// ladder as RedeemShareLink, then LIVE-RESOLVES each item against current
// data. Items whose source has since been deleted are skipped (graceful), not
// fatal. Increments view_count on success.
func (r *SecretRepo) RedeemBundle(ctx context.Context, token, passphrase string, meta RedeemMeta) (*BundlePayload, error) {
	hash := secretshare.HashToken(token)

	var (
		id          int64
		title       string
		description string
		passHash    []byte
		maxViews    sql.NullInt64
		viewCount   int
		expiresStr  sql.NullString
		revoked     sql.NullTime
		deleted     sql.NullTime
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, description, passphrase_hash, max_views, view_count, expires_at, revoked_at, deleted_at
		   FROM share_bundles WHERE token_hash = ?`, hash,
	).Scan(&id, &title, &description, &passHash, &maxViews, &viewCount, &expiresStr, &revoked, &deleted)
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

	payload := &BundlePayload{Title: title, Description: description, Secrets: []BundleSecretItem{}, APIDocs: []BundleAPIDocItem{}, Wiki: []BundleWikiItem{}}
	// Wiki items resolve live against Outline. Build one client on the first
	// wiki item (nil if the integration is down — those items then skip
	// gracefully, like a deleted secret/api_doc).
	var (
		wikiClient   *outlineclient.Client
		wikiSettings outlineclient.Settings
		wikiInit     bool
	)
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
				Name: v.Name, Type: string(v.Type), Payload: plain, Key: r.bundleKeyInfo(ctx, it.refID),
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
		case BundleItemWikiDoc, BundleItemWikiCollection:
			if !wikiInit {
				wikiInit = true
				if c, s, err := r.wikiClient(ctx); err == nil {
					wikiClient, wikiSettings = c, s
				}
			}
			if wikiClient == nil {
				continue // outline unavailable — skip gracefully
			}
			wi, err := r.resolveWikiItem(ctx, it.itemType, it.refKey, wikiClient, wikiSettings)
			if err != nil || wi == nil {
				continue // deleted/missing/timeout — skip gracefully
			}
			payload.Wiki = append(payload.Wiki, *wi)
		}
	}

	if _, err := r.db.ExecContext(ctx,
		`UPDATE share_bundles SET view_count = view_count + 1 WHERE id = ?`, id); err != nil {
		return nil, err
	}

	// Record the anonymous access (network metadata only). Best-effort: a log
	// failure must never block the reveal, so the error is swallowed.
	_, _ = r.db.ExecContext(ctx,
		`INSERT INTO share_bundle_access_log (bundle_id, remote_ip, user_agent, used_passphrase)
		 VALUES (?, ?, ?, ?)`,
		id, meta.RemoteIP, meta.UserAgent, len(passHash) > 0)

	return payload, nil
}

type bundleItemRow struct {
	itemType string
	refID    int64
	refKey   string
	selector string
}

func (r *SecretRepo) loadBundleItems(ctx context.Context, bundleID int64) ([]bundleItemRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT item_type, ref_id, ref_key, selector FROM share_bundle_items
		  WHERE bundle_id = ? ORDER BY sort_order, id`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bundleItemRow
	for rows.Next() {
		var it bundleItemRow
		var refKey, sel sql.NullString
		if err := rows.Scan(&it.itemType, &it.refID, &refKey, &sel); err != nil {
			return nil, err
		}
		if refKey.Valid {
			it.refKey = refKey.String
		}
		if sel.Valid {
			it.selector = sel.String
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// bundleViewColumns is the shared owner-facing projection used by every
// metadata read (list / list-for-item / single get), over share_bundles
// aliased `b`. Kept in one place so the scan order in scanBundleViews stays in
// lock-step with the SELECT. Names and the last redemption come from
// correlated subqueries, so a DISTINCT over a join with the items still works.
const bundleViewColumns = `b.id, b.title, b.description, b.expires_at, b.passphrase_hash, b.max_views, b.view_count,
	b.created_by, b.created_at, b.revoked_at, b.deleted_at,
	b.recipient_contact_id, b.recipient_label,
	COALESCE((SELECT c.name FROM contacts c WHERE c.id = b.recipient_contact_id), ''),
	COALESCE((SELECT c.email FROM contacts c WHERE c.id = b.recipient_contact_id), ''),
	COALESCE((SELECT COALESCE(NULLIF(u.display_name, ''), u.username) FROM users u WHERE u.id = b.created_by), ''),
	(SELECT MAX(l.accessed_at) FROM share_bundle_access_log l WHERE l.bundle_id = b.id AND l.action = 'redeem'),
	(b.token_cipher IS NOT NULL)`

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
			recID    sql.NullInt64
			lastAcc  sql.NullTime
		)
		if err := rows.Scan(&v.ID, &v.Title, &v.Description, &expires, &passHash, &maxViews,
			&v.ViewCount, &v.CreatedBy, &created, &revoked, &deleted,
			&recID, &v.RecipientLabel, &v.RecipientName, &v.RecipientEmail, &v.CreatedByName, &lastAcc, &v.Recoverable); err != nil {
			return nil, err
		}
		if recID.Valid {
			id := recID.Int64
			v.RecipientContactID = &id
		}
		if lastAcc.Valid {
			t := lastAcc.Time
			v.LastAccessAt = &t
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

// ListBundles returns the bundles in the actor's ownerScope (metadata + item
// labels): an admin's list is every bundle.
func (r *SecretRepo) ListBundles(ctx context.Context, actor ActorContext) ([]BundleView, error) {
	scope, args := ownerScope(actor)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bundleViewColumns+`
		   FROM share_bundles b WHERE `+scope+` ORDER BY b.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	return r.scanBundleViews(ctx, rows)
}

// ListBundlesForItem returns the bundles in the actor's ownerScope that CONTAIN an item of
// the given (itemType, refID) — e.g. every live link exposing a specific API
// doc or secret. It is the server-side, generalized form of the handler's
// ?secret_id= filter, and unlike that filter it also matches multi-item
// bundles (a link carrying the API doc plus attached secrets still counts).
func (r *SecretRepo) ListBundlesForItem(ctx context.Context, actor ActorContext, itemType string, refID int64) ([]BundleView, error) {
	scope, args := ownerScope(actor)
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT `+bundleViewColumns+`
		   FROM share_bundles b
		   JOIN share_bundle_items i ON i.bundle_id = b.id
		  WHERE `+scope+` AND i.item_type = ? AND i.ref_id = ?
		  ORDER BY b.created_at DESC`, append(args, itemType, refID)...)
	if err != nil {
		return nil, err
	}
	return r.scanBundleViews(ctx, rows)
}

// ListBundlesForItemKey is the string-keyed sibling of ListBundlesForItem, for
// wiki items whose Outline UUID lives in ref_key (ref_id is 0). Returns the
// bundles in the actor's ownerScope that contain an item matching (itemType, refKey).
func (r *SecretRepo) ListBundlesForItemKey(ctx context.Context, actor ActorContext, itemType, refKey string) ([]BundleView, error) {
	scope, args := ownerScope(actor)
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT `+bundleViewColumns+`
		   FROM share_bundles b
		   JOIN share_bundle_items i ON i.bundle_id = b.id
		  WHERE `+scope+` AND i.item_type = ? AND i.ref_key = ?
		  ORDER BY b.created_at DESC`, append(args, itemType, refKey)...)
	if err != nil {
		return nil, err
	}
	return r.scanBundleViews(ctx, rows)
}

// getBundleView reads a single bundle in the actor's ownerScope as a
// BundleView, or ErrBundleNotFound if it does not exist / is out of scope.
func (r *SecretRepo) getBundleView(ctx context.Context, actor ActorContext, bundleID int64) (*BundleView, error) {
	scope, args := ownerScope(actor)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bundleViewColumns+`
		   FROM share_bundles b WHERE b.id = ? AND `+scope, append([]any{bundleID}, args...)...)
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
// Owner (or admin) only. Optionally adjusts max_views. view_count is preserved
// so the usage audit stays honest.
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
	scope, args := ownerScope(actor)
	if opts.MaxViews == nil {
		res, err = r.db.ExecContext(ctx,
			`UPDATE share_bundles SET expires_at = ?, revoked_at = NULL, deleted_at = NULL
			  WHERE id = ? AND `+scope,
			append([]any{expiresArg, bundleID}, args...)...)
	} else {
		var mv any // NULL clears the cap (unlimited within TTL)
		if *opts.MaxViews > 0 {
			mv = *opts.MaxViews
		}
		res, err = r.db.ExecContext(ctx,
			`UPDATE share_bundles SET expires_at = ?, revoked_at = NULL, deleted_at = NULL, max_views = ?
			  WHERE id = ? AND `+scope,
			append([]any{expiresArg, mv, bundleID}, args...)...)
	}
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrBundleNotFound
	}
	return r.getBundleView(ctx, actor, bundleID)
}

// BundleDetails is a partial edit of a bundle's descriptive fields: a nil
// field keeps its value. RecipientContactID 0 clears the contact; Passphrase
// "" removes the passphrase. The token (and so the URL), items, expiry and
// view count are untouched.
type BundleDetails struct {
	Title              *string
	Description        *string
	RecipientContactID *int64
	RecipientLabel     *string
	Passphrase         *string
}

// UpdateBundleDetails edits a bundle's title, description, recipient and
// passphrase in place (owner, or admin). A new passphrase is stored like at
// creation: hashed for redeem, encrypted for the admin reveal.
func (r *SecretRepo) UpdateBundleDetails(ctx context.Context, actor ActorContext, bundleID int64, d BundleDetails) (*BundleView, error) {
	if err := r.checkBundleScope(ctx, actor, bundleID); err != nil {
		return nil, err
	}
	var sets []string
	var args []any
	set := func(col string, v any) { sets = append(sets, col+" = ?"); args = append(args, v) }
	if d.Title != nil {
		set("title", strings.TrimSpace(*d.Title))
	}
	if d.Description != nil {
		set("description", *d.Description)
	}
	if d.RecipientLabel != nil {
		set("recipient_label", strings.TrimSpace(*d.RecipientLabel))
	}
	if d.RecipientContactID != nil {
		if *d.RecipientContactID <= 0 {
			set("recipient_contact_id", nil)
		} else {
			var one int
			switch err := r.db.QueryRowContext(ctx, `SELECT 1 FROM contacts WHERE id = ?`, *d.RecipientContactID).Scan(&one); {
			case errors.Is(err, sql.ErrNoRows):
				return nil, ErrBundleRecipientInvalid
			case err != nil:
				return nil, err
			}
			set("recipient_contact_id", *d.RecipientContactID)
		}
	}
	if d.Passphrase != nil {
		if *d.Passphrase == "" {
			set("passphrase_hash", nil)
			set("passphrase_cipher", nil)
			set("passphrase_nonce", nil)
		} else {
			ph, err := hashPassphrase(*d.Passphrase)
			if err != nil {
				return nil, err
			}
			ct, nonce, err := r.enc.Encrypt(*d.Passphrase)
			if err != nil {
				return nil, fmt.Errorf("encrypt passphrase: %w", err)
			}
			set("passphrase_hash", ph)
			set("passphrase_cipher", ct)
			set("passphrase_nonce", nonce)
		}
	}
	if len(sets) > 0 {
		args = append(args, bundleID)
		if _, err := r.db.ExecContext(ctx, `UPDATE share_bundles SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
			return nil, err
		}
	}
	return r.getBundleView(ctx, actor, bundleID)
}

func (r *SecretRepo) labelItems(ctx context.Context, items []bundleItemRow) []BundleItemView {
	views := make([]BundleItemView, 0, len(items))
	// Outline client is built lazily on the first wiki item and reused for the
	// rest of this bundle's labels (nil if the integration is down — those items
	// then fall back to their "(deleted)" label).
	var (
		wikiClient *outlineclient.Client
		wikiInit   bool
	)
	for _, it := range items {
		bv := BundleItemView{Type: it.itemType, RefID: it.refID, RefKey: it.refKey, Label: "(deleted)"}
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
		case BundleItemWikiDoc, BundleItemWikiCollection:
			if !wikiInit {
				wikiInit = true
				if c, _, err := r.wikiClient(ctx); err == nil {
					wikiClient = c
				}
			}
			if wikiClient != nil && it.refKey != "" {
				lctx, cancel := context.WithTimeout(ctx, wikiAuthTimeout)
				if it.itemType == BundleItemWikiDoc {
					if d, err := wikiClient.DocumentInfo(lctx, it.refKey); err == nil {
						bv.Label = d.Title
					}
				} else {
					if c, err := wikiClient.CollectionInfo(lctx, it.refKey); err == nil {
						bv.Label = c.Name
					}
				}
				cancel()
			}
		}
		views = append(views, bv)
	}
	return views
}

// RevokeBundle marks a bundle revoked (owner or admin).
func (r *SecretRepo) RevokeBundle(ctx context.Context, actor ActorContext, bundleID int64) error {
	scope, args := ownerScope(actor)
	res, err := r.db.ExecContext(ctx,
		`UPDATE share_bundles SET revoked_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND revoked_at IS NULL AND `+scope,
		append([]any{bundleID}, args...)...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrBundleNotFound
	}
	return nil
}

// RevokeAllBundles revokes every bundle not yet revoked and answers how many.
// Admin only (anyone else gets ErrSecretForbidden).
func (r *SecretRepo) RevokeAllBundles(ctx context.Context, actor ActorContext) (int64, error) {
	if actor.Role != "admin" {
		return 0, ErrSecretForbidden
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE share_bundles SET revoked_at = CURRENT_TIMESTAMP WHERE revoked_at IS NULL`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RevealBundle decrypts a bundle's raw token and passphrase for an admin and
// records the reveal in the access log (action 'reveal', with the actor). A
// non-admin gets ErrBundleNotFound — the bundle's existence isn't leaked — and
// a bundle from before v105 (no stored cipher) ErrBundleNotRecoverable.
func (r *SecretRepo) RevealBundle(ctx context.Context, actor ActorContext, bundleID int64) (*BundleSecrets, error) {
	if actor.Role != "admin" {
		return nil, ErrBundleNotFound
	}
	var tokCT, tokNonce, passCT, passNonce []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT token_cipher, token_nonce, passphrase_cipher, passphrase_nonce
		   FROM share_bundles WHERE id = ?`, bundleID).Scan(&tokCT, &tokNonce, &passCT, &passNonce)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBundleNotFound
	}
	if err != nil {
		return nil, err
	}
	out, err := r.decryptBundleSecrets(tokCT, tokNonce, passCT, passNonce)
	if err != nil {
		return nil, err
	}
	// The audit row is not best-effort: no row, no reveal.
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO share_bundle_access_log (bundle_id, action, actor_user_id) VALUES (?, 'reveal', ?)`,
		bundleID, actor.UserID); err != nil {
		return nil, fmt.Errorf("audit reveal: %w", err)
	}
	return out, nil
}

// BundleAccessLog returns a bundle's access-log rows (guest redemptions, admin
// reveals and email sends), newest-first, hard-capped at the 500 most-recent.
// Scoped by ownerScope: a missing or out-of-scope bundle yields
// ErrBundleNotFound. Redemptions carry only network metadata; reveals and
// sends name the actor, sends also the address.
func (r *SecretRepo) BundleAccessLog(ctx context.Context, actor ActorContext, bundleID int64) ([]BundleAccessEntry, error) {
	if err := r.checkBundleScope(ctx, actor, bundleID); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT l.accessed_at, l.remote_ip, l.user_agent, l.used_passphrase, l.action, l.sent_to,
		        COALESCE(NULLIF(u.display_name, ''), u.username, '')
		   FROM share_bundle_access_log l
		   LEFT JOIN users u ON u.id = l.actor_user_id
		  WHERE l.bundle_id = ?
		  ORDER BY l.accessed_at DESC, l.id DESC
		  LIMIT 500`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BundleAccessEntry{}
	for rows.Next() {
		var e BundleAccessEntry
		if err := rows.Scan(&e.AccessedAt, &e.RemoteIP, &e.UserAgent, &e.UsedPassphrase, &e.Action, &e.SentTo, &e.ActorName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// bundleKeyInfo is the API key behind secretID, or nil when it isn't one (or
// the lookup fails — the secret is still shared, just without key details).
func (r *SecretRepo) bundleKeyInfo(ctx context.Context, secretID int64) *BundleKeyInfo {
	k, err := store.NewAPIKeyRepo(r.db).BySecret(ctx, secretID)
	if err != nil || k == nil {
		return nil
	}
	a, err := store.NewAPICatalogRepo(r.db).Get(ctx, k.APIID)
	if err != nil || a == nil {
		return nil
	}
	info := &BundleKeyInfo{APIID: a.ID, APIName: a.Name, APIBaseURL: a.BaseURL,
		Scopes: make([]BundleScopeInfo, 0, len(k.Scopes)), RateLimitPerMinute: k.RateLimitPerMinute}
	if k.ExternalLabel != nil {
		info.ClientID = *k.ExternalLabel
	}
	for _, s := range k.Scopes {
		info.Scopes = append(info.Scopes, BundleScopeInfo{Name: s})
	}
	return info
}
