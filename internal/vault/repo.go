package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// ErrSecretNotFound is returned when a secret does not exist OR the caller
// has no visibility into it. Per spec §5.2 personal secrets owned by another
// user look like 404 to non-admins, so the same error covers both cases.
var ErrSecretNotFound = errors.New("secret not found")

// ErrSecretForbidden is returned when the caller can see the secret's
// existence but is not allowed to perform the requested action
// (e.g. admin trying to Reveal another user's personal secret per D1).
var ErrSecretForbidden = errors.New("forbidden")

// ActorContext is the acting principal for a repo call. The canonical type now
// lives in models (so store + vault can share it without an import cycle); this
// alias keeps the historical vault.ActorContext spelling working for existing
// callers.
type ActorContext = models.ActorContext

// SecretFilter narrows List / ListTrash results. Empty fields are wildcards.
type SecretFilter struct {
	Scope          models.SecretScope
	Type           models.SecretType
	Visibility     models.SecretVisibility
	ParentID       *int64
	OwnerUserID    *int64
	GroupLabel     string
	IncludeDeleted bool // include soft-deleted rows
	OnlyDeleted    bool // restrict to soft-deleted rows (trash view)
}

// SecretView is a metadata-only projection of a Secret. PayloadCiphertext
// is intentionally always nil from this repo's read paths; callers that need
// the plaintext use Reveal(). Description may be nil when an admin views
// another user's personal secret (D6).
type SecretView struct {
	ID                int64                   `json:"id"`
	Type              models.SecretType       `json:"type"`
	Scope             models.SecretScope      `json:"scope"`
	Visibility        models.SecretVisibility `json:"visibility"`
	ParentID          *int64                  `json:"parent_id,omitempty"`
	OwnerUserID       int64                   `json:"owner_user_id"`
	Name              string                  `json:"name"`
	GroupLabel        *string                 `json:"group_label,omitempty"`
	Description       *string                 `json:"description,omitempty"`
	KeyVersion        int                     `json:"key_version"`
	CreatedBy         int64                   `json:"created_by"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	DeletedAt         *time.Time              `json:"deleted_at,omitempty"`
	PayloadCiphertext []byte                  `json:"-"` // always nil; sentinel
}

// SecretPatch is a partial update. Nil fields are unchanged. Visibility is
// intentionally not patchable (spec §6: immutable on PUT).
type SecretPatch struct {
	Name        *string
	Description *string
	GroupLabel  *string
	Payload     *string // re-encrypted before storage
}

// SecretRepo handles persistence + ACL for the unified secrets manager.
// See docs/spec/secrets-manager.md §5 for the access-control rules baked in.
type SecretRepo struct {
	db  *sql.DB
	enc  *database.Encryptor
}

// NewSecretRepo wires a repo from a *DB.
func NewSecretRepo(d *database.DB) *SecretRepo {
	return &SecretRepo{db: d.SQL, enc: d.Encryptor}
}

// --- ACL ---------------------------------------------------------------

type accessDecision struct {
	canSeeMetadata    bool
	canReveal         bool
	canWrite          bool // create + update
	canDelete         bool // soft-delete + restore
	redactDescription bool // admin viewing others' personal (D6)
}

// decideAccess applies spec §5.1 (shared/RBAC) and §5.2 (personal/ownership).
func decideAccess(actor ActorContext, vis models.SecretVisibility, ownerUserID int64) accessDecision {
	isAdmin := actor.Role == "admin"
	isEditor := actor.Role == "editor" || isAdmin
	isOwner := actor.UserID == ownerUserID

	if vis == models.SecretVisibilityShared {
		return accessDecision{
			canSeeMetadata: true,
			canReveal:      true, // viewer+
			canWrite:       isEditor,
			canDelete:      isAdmin,
		}
	}
	// personal
	if isOwner {
		return accessDecision{
			canSeeMetadata: true,
			canReveal:      true,
			canWrite:       true,
			canDelete:      true,
		}
	}
	if isAdmin {
		// D6: admin sees name/type/scope of others' personal; values + share not.
		return accessDecision{
			canSeeMetadata:    true,
			redactDescription: true,
		}
	}
	return accessDecision{} // 404-equivalent
}

// --- Mutations ---------------------------------------------------------

// Create inserts a new secret, encrypting the plaintext payload, and writes
// a create audit row in the same transaction.
//
// Authz: a personal secret can only be created with OwnerUserID == actor.
// A shared secret requires actor to be editor or admin.
func (r *SecretRepo) Create(ctx context.Context, actor ActorContext, s *models.Secret, plaintext string) (int64, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	if s.Visibility == models.SecretVisibilityPersonal && s.OwnerUserID != actor.UserID {
		return 0, ErrSecretForbidden
	}
	if s.Visibility == models.SecretVisibilityShared {
		if actor.Role != "editor" && actor.Role != "admin" {
			return 0, ErrSecretForbidden
		}
	}

	ct, nonce, err := r.enc.Encrypt(plaintext)
	if err != nil {
		return 0, err
	}
	s.PayloadCiphertext = ct
	s.PayloadNonce = nonce

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	id, err := database.InsertReturningID(tx,
		`INSERT INTO secrets
			(type, scope, visibility, parent_id, owner_user_id, name, group_label,
			 description, payload_ciphertext, payload_nonce, key_version, created_by)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		string(s.Type), string(s.Scope), string(s.Visibility),
		s.ParentID, s.OwnerUserID, s.Name, s.GroupLabel,
		s.Description, s.PayloadCiphertext, s.PayloadNonce, s.KeyVersion, s.CreatedBy,
	)
	if err != nil {
		return 0, err
	}
	if err := writeAuditTx(tx, id, models.SecretAuditActionCreate, actor, nil, nil); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.ID = id
	return id, nil
}

// Update applies a partial patch and records the changed fields in audit
// metadata (`{"changed_fields":["name","payload",...]}`). Visibility is
// immutable here per spec §6.
func (r *SecretRepo) Update(ctx context.Context, actor ActorContext, id int64, patch SecretPatch) error {
	v, err := r.loadView(ctx, id, false)
	if err != nil {
		return err
	}
	dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
	if !dec.canSeeMetadata {
		return ErrSecretNotFound
	}
	if !dec.canWrite {
		return ErrSecretForbidden
	}

	var (
		changed []string
		sets    []string
		args    []any
	)
	if patch.Name != nil && v.Name != *patch.Name {
		sets = append(sets, "name = ?")
		args = append(args, *patch.Name)
		changed = append(changed, "name")
	}
	if patch.Description != nil {
		cur := ""
		if v.Description != nil {
			cur = *v.Description
		}
		if cur != *patch.Description {
			sets = append(sets, "description = ?")
			args = append(args, *patch.Description)
			changed = append(changed, "description")
		}
	}
	if patch.GroupLabel != nil {
		cur := ""
		if v.GroupLabel != nil {
			cur = *v.GroupLabel
		}
		if cur != *patch.GroupLabel {
			sets = append(sets, "group_label = ?")
			args = append(args, *patch.GroupLabel)
			changed = append(changed, "group_label")
		}
	}
	if patch.Payload != nil {
		ct, nonce, err := r.enc.Encrypt(*patch.Payload)
		if err != nil {
			return err
		}
		sets = append(sets, "payload_ciphertext = ?", "payload_nonce = ?")
		args = append(args, ct, nonce)
		changed = append(changed, "payload")
	}
	if len(sets) == 0 {
		return nil // no-op patch
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE secrets SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string][]string{"changed_fields": changed})
	if err := writeAuditTx(tx, id, models.SecretAuditActionUpdate, actor, nil, meta); err != nil {
		return err
	}
	return tx.Commit()
}

// SoftDelete sets deleted_at and writes an audit row. The row is retained
// so Restore can bring it back (spec D3).
func (r *SecretRepo) SoftDelete(ctx context.Context, actor ActorContext, id int64) error {
	return r.softDeleteOrRestore(ctx, actor, id, true)
}

// Restore un-deletes a soft-deleted secret.
func (r *SecretRepo) Restore(ctx context.Context, actor ActorContext, id int64) error {
	return r.softDeleteOrRestore(ctx, actor, id, false)
}

func (r *SecretRepo) softDeleteOrRestore(ctx context.Context, actor ActorContext, id int64, doDelete bool) error {
	v, err := r.loadView(ctx, id, true)
	if err != nil {
		return err
	}
	dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
	if !dec.canSeeMetadata {
		return ErrSecretNotFound
	}
	if !dec.canDelete {
		return ErrSecretForbidden
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if doDelete {
		if _, err := tx.Exec(
			`UPDATE secrets SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ? AND deleted_at IS NULL`, id,
		); err != nil {
			return err
		}
		if err := writeAuditTx(tx, id, models.SecretAuditActionDelete, actor, nil, nil); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(
			`UPDATE secrets SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ? AND deleted_at IS NOT NULL`, id,
		); err != nil {
			return err
		}
		if err := writeAuditTx(tx, id, models.SecretAuditActionRestore, actor, nil, nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Reads -------------------------------------------------------------

// GetMetadata returns one secret's metadata, applying ACL redaction.
func (r *SecretRepo) GetMetadata(ctx context.Context, actor ActorContext, id int64) (*SecretView, error) {
	v, err := r.loadView(ctx, id, true)
	if err != nil {
		return nil, err
	}
	dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
	if !dec.canSeeMetadata {
		return nil, ErrSecretNotFound
	}
	if dec.redactDescription {
		v.Description = nil
	}
	return v, nil
}

// Reveal decrypts and returns the plaintext, writing a reveal audit row.
// Personal secrets owned by another user always 403, even for admins (D1).
func (r *SecretRepo) Reveal(ctx context.Context, actor ActorContext, id int64) (string, error) {
	var (
		typeStr, scopeStr, visStr string
		s                         models.Secret
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, type, scope, visibility, parent_id, owner_user_id, name,
		        payload_ciphertext, payload_nonce, key_version, created_by
		 FROM secrets WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&s.ID, &typeStr, &scopeStr, &visStr, &s.ParentID, &s.OwnerUserID,
		&s.Name, &s.PayloadCiphertext, &s.PayloadNonce, &s.KeyVersion, &s.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrSecretNotFound
	}
	if err != nil {
		return "", err
	}
	s.Type = models.SecretType(typeStr)
	s.Scope = models.SecretScope(scopeStr)
	s.Visibility = models.SecretVisibility(visStr)

	dec := decideAccess(actor, s.Visibility, s.OwnerUserID)
	if !dec.canSeeMetadata {
		return "", ErrSecretNotFound
	}
	if !dec.canReveal {
		return "", ErrSecretForbidden
	}

	plain, err := r.enc.Decrypt(s.PayloadCiphertext, s.PayloadNonce)
	if err != nil {
		return "", err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := writeAuditTx(tx, id, models.SecretAuditActionReveal, actor, nil, nil); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return plain, nil
}

// List returns metadata views the caller is permitted to see.
func (r *SecretRepo) List(ctx context.Context, actor ActorContext, f SecretFilter) ([]SecretView, error) {
	return r.runList(ctx, actor, f)
}

// ListTrash returns soft-deleted secrets the caller could restore (their own
// personal + any shared if admin). Items the caller cannot restore are
// filtered out so the trash UI doesn't show unactionable rows.
func (r *SecretRepo) ListTrash(ctx context.Context, actor ActorContext) ([]SecretView, error) {
	views, err := r.runList(ctx, actor, SecretFilter{IncludeDeleted: true, OnlyDeleted: true})
	if err != nil {
		return nil, err
	}
	out := views[:0]
	for _, v := range views {
		if decideAccess(actor, v.Visibility, v.OwnerUserID).canDelete {
			out = append(out, v)
		}
	}
	return out, nil
}

// History returns the audit log for a secret. Owners + admins-on-shared see
// the full log; admin-on-others'-personal returns ErrSecretForbidden so audit
// trails of personal items don't leak to admins (spec D6, conservative read).
func (r *SecretRepo) History(ctx context.Context, actor ActorContext, id int64) ([]models.SecretAuditLog, error) {
	v, err := r.loadView(ctx, id, true)
	if err != nil {
		return nil, err
	}
	dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
	if !dec.canSeeMetadata {
		return nil, ErrSecretNotFound
	}
	if v.Visibility == models.SecretVisibilityPersonal && v.OwnerUserID != actor.UserID {
		return nil, ErrSecretForbidden
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, secret_id, action, actor_user_id, actor_ip, share_link_id, metadata, at
		 FROM secret_audit_log WHERE secret_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.SecretAuditLog
	for rows.Next() {
		var (
			e         models.SecretAuditLog
			actionStr string
			meta      sql.NullString
		)
		if err := rows.Scan(&e.ID, &e.SecretID, &actionStr, &e.ActorUserID,
			&e.ActorIP, &e.ShareLinkID, &meta, &e.At); err != nil {
			return nil, err
		}
		e.Action = models.SecretAuditAction(actionStr)
		if meta.Valid {
			e.Metadata = json.RawMessage(meta.String)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- internals ---------------------------------------------------------

func (r *SecretRepo) runList(ctx context.Context, actor ActorContext, f SecretFilter) ([]SecretView, error) {
	var (
		conds []string
		args  []any
	)
	switch {
	case f.OnlyDeleted:
		conds = append(conds, "deleted_at IS NOT NULL")
	case !f.IncludeDeleted:
		conds = append(conds, "deleted_at IS NULL")
	}
	if f.Scope != "" {
		conds = append(conds, "scope = ?")
		args = append(args, string(f.Scope))
	}
	if f.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, string(f.Type))
	}
	if f.Visibility != "" {
		conds = append(conds, "visibility = ?")
		args = append(args, string(f.Visibility))
	}
	if f.ParentID != nil {
		conds = append(conds, "parent_id = ?")
		args = append(args, *f.ParentID)
	}
	if f.OwnerUserID != nil {
		conds = append(conds, "owner_user_id = ?")
		args = append(args, *f.OwnerUserID)
	}
	if f.GroupLabel != "" {
		conds = append(conds, "group_label = ?")
		args = append(args, f.GroupLabel)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	query := `SELECT id, type, scope, visibility, parent_id, owner_user_id, name,
	                 group_label, description, key_version, created_by,
	                 created_at, updated_at, deleted_at
	          FROM secrets` + where + ` ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SecretView
	for rows.Next() {
		var (
			v                         SecretView
			typeStr, scopeStr, visStr string
		)
		if err := rows.Scan(&v.ID, &typeStr, &scopeStr, &visStr, &v.ParentID,
			&v.OwnerUserID, &v.Name, &v.GroupLabel, &v.Description, &v.KeyVersion,
			&v.CreatedBy, &v.CreatedAt, &v.UpdatedAt, &v.DeletedAt); err != nil {
			return nil, err
		}
		v.Type = models.SecretType(typeStr)
		v.Scope = models.SecretScope(scopeStr)
		v.Visibility = models.SecretVisibility(visStr)

		dec := decideAccess(actor, v.Visibility, v.OwnerUserID)
		if !dec.canSeeMetadata {
			continue
		}
		if dec.redactDescription {
			v.Description = nil
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *SecretRepo) loadView(ctx context.Context, id int64, includeDeleted bool) (*SecretView, error) {
	where := "id = ?"
	if !includeDeleted {
		where += " AND deleted_at IS NULL"
	}
	var (
		v                         SecretView
		typeStr, scopeStr, visStr string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, type, scope, visibility, parent_id, owner_user_id, name,
		        group_label, description, key_version, created_by,
		        created_at, updated_at, deleted_at
		 FROM secrets WHERE `+where, id,
	).Scan(&v.ID, &typeStr, &scopeStr, &visStr, &v.ParentID, &v.OwnerUserID,
		&v.Name, &v.GroupLabel, &v.Description, &v.KeyVersion, &v.CreatedBy,
		&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSecretNotFound
	}
	if err != nil {
		return nil, err
	}
	v.Type = models.SecretType(typeStr)
	v.Scope = models.SecretScope(scopeStr)
	v.Visibility = models.SecretVisibility(visStr)
	return &v, nil
}

func writeAuditTx(tx *sql.Tx, secretID int64, action models.SecretAuditAction, actor ActorContext, shareLinkID *int64, metadata json.RawMessage) error {
	var actorID *int64
	if actor.UserID != 0 {
		a := actor.UserID
		actorID = &a
	}
	var metaParam any // nil for NULL, *string for the JSON blob
	if len(metadata) > 0 {
		s := string(metadata)
		metaParam = s
	}
	_, err := tx.Exec(
		`INSERT INTO secret_audit_log (secret_id, action, actor_user_id, share_link_id, metadata)
		 VALUES (?,?,?,?,?)`,
		secretID, string(action), actorID, shareLinkID, metaParam,
	)
	return err
}
