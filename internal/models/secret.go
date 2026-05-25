// Package models contains the database entity types for Bridge.
//
// Secret-related CRUD intentionally lives in internal/database/secret_repo.go
// rather than this file (departure from the per-model convention used by
// service_credential.go, host.go, etc.) because the secret repository carries
// visibility-aware filtering and encryption helpers that warrant a dedicated
// module. See docs/spec/secrets-manager.md §6.
package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// SecretType classifies what shape the encrypted payload carries.
// Payload JSON shapes are defined in docs/spec/secrets-manager.md §4.2.
type SecretType string

const (
	SecretTypeCred     SecretType = "cred"      // {username, password} against a host
	SecretTypeSSHKey   SecretType = "sshkey"    // {username, private_key_pem, public_key, passphrase?}
	SecretTypePassword SecretType = "password"  // {value} — standalone password
	SecretTypeAppLogin SecretType = "app_login" // {app_name, url?, username, password, notes?}
	SecretTypeEnvVar   SecretType = "env_var"   // {value, description?} — one var per row (D5)
)

// Valid reports whether t is one of the known SecretType values.
func (t SecretType) Valid() bool {
	switch t {
	case SecretTypeCred, SecretTypeSSHKey, SecretTypePassword, SecretTypeAppLogin, SecretTypeEnvVar:
		return true
	}
	return false
}

// SecretScope describes what the secret is attached to (D7).
// `avulso` means unattached (no parent row).
type SecretScope string

const (
	SecretScopeService SecretScope = "service"
	SecretScopeHost    SecretScope = "host"
	SecretScopeTool    SecretScope = "tool"
	SecretScopeAvulso  SecretScope = "avulso"
)

// Valid reports whether s is one of the known SecretScope values.
func (s SecretScope) Valid() bool {
	switch s {
	case SecretScopeService, SecretScopeHost, SecretScopeTool, SecretScopeAvulso:
		return true
	}
	return false
}

// SecretVisibility describes who can see the secret (D7), orthogonal to scope.
// `shared` falls under RBAC; `personal` is owner-only (admin sees metadata, see spec §5).
type SecretVisibility string

const (
	SecretVisibilityPersonal SecretVisibility = "personal"
	SecretVisibilityShared   SecretVisibility = "shared"
)

// Valid reports whether v is one of the known SecretVisibility values.
func (v SecretVisibility) Valid() bool {
	switch v {
	case SecretVisibilityPersonal, SecretVisibilityShared:
		return true
	}
	return false
}

// Secret is a single encrypted secret row.
//
// Uniqueness is enforced at the DB layer via partial unique indexes
// (see internal/database/migrations_*.go; spec §4.1):
//   - visibility='shared':   UNIQUE (scope, parent_id, name, group_label) WHERE deleted_at IS NULL
//   - visibility='personal': UNIQUE (scope, parent_id, owner_user_id, name, group_label) WHERE deleted_at IS NULL
//
// Two users may each own a personal secret with the same (scope, parent_id, name);
// only one team-shared secret with that key may exist at a time.
type Secret struct {
	ID                int64            `json:"id"`
	Type              SecretType       `json:"type"`
	Scope             SecretScope      `json:"scope"`
	Visibility        SecretVisibility `json:"visibility"`
	ParentID          *int64           `json:"parent_id,omitempty"` // service.id | host.id | external_tool.id; nil for avulso
	OwnerUserID       int64            `json:"owner_user_id"`       // creator; always set
	Name              string           `json:"name"`
	GroupLabel        *string          `json:"group_label,omitempty"` // environment ("prod", "staging", …) for env_var
	Description       *string          `json:"description,omitempty"`
	PayloadCiphertext []byte           `json:"-"`
	PayloadNonce      []byte           `json:"-"`
	KeyVersion        int              `json:"key_version"`
	CreatedBy         int64            `json:"created_by"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
	DeletedAt         *time.Time       `json:"deleted_at,omitempty"`
}

// Validate enforces the invariants from docs/spec/secrets-manager.md §4.1.
// Called by repository Create/Update before any DB write.
func (s *Secret) Validate() error {
	if !s.Type.Valid() {
		return fmt.Errorf("invalid type %q", s.Type)
	}
	if !s.Scope.Valid() {
		return fmt.Errorf("invalid scope %q", s.Scope)
	}
	if !s.Visibility.Valid() {
		return fmt.Errorf("invalid visibility %q", s.Visibility)
	}
	if s.Name == "" {
		return fmt.Errorf("name required")
	}
	if s.OwnerUserID == 0 {
		return fmt.Errorf("owner_user_id required")
	}
	// CHECK constraints on (scope, parent_id):
	if s.Scope == SecretScopeAvulso && s.ParentID != nil {
		return fmt.Errorf("scope %q (avulso) must not have parent_id", s.Scope)
	}
	if s.Scope != SecretScopeAvulso && s.ParentID == nil {
		return fmt.Errorf("scope %q requires parent_id", s.Scope)
	}
	if s.KeyVersion < 1 {
		return fmt.Errorf("key_version must be >= 1, got %d", s.KeyVersion)
	}
	return nil
}

// SecretShareLink is an external share-link record for a personal secret.
// Per spec D7/§5.3, share-link creation is rejected when the target secret
// has visibility != 'personal'.
//
// TokenHash stores sha256(raw_token); the raw token lives only in the URL and
// is also used as HKDF salt (D4) to derive a per-link AES key. DB compromise
// alone cannot decrypt the shared payload without the token.
type SecretShareLink struct {
	ID             int64      `json:"id"`
	SecretID       int64      `json:"secret_id"`
	TokenHash      []byte     `json:"-"` // sha256(raw_token); never stored unhashed
	ExpiresAt      time.Time  `json:"expires_at"`
	PassphraseHash []byte     `json:"-"` // argon2id(passphrase); nil = no passphrase gate
	MaxViews       *int       `json:"max_views,omitempty"`
	ViewCount      int        `json:"view_count"`
	CreatedBy      int64      `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
}

// SecretAuditAction enumerates the actions recorded in secret_audit_log.
type SecretAuditAction string

const (
	SecretAuditActionCreate      SecretAuditAction = "create"
	SecretAuditActionReveal      SecretAuditAction = "reveal"
	SecretAuditActionUpdate      SecretAuditAction = "update"
	SecretAuditActionDelete      SecretAuditAction = "delete"  // soft delete
	SecretAuditActionRestore     SecretAuditAction = "restore" // un-soft-delete
	SecretAuditActionShareCreate SecretAuditAction = "share_create"
	SecretAuditActionShareRedeem SecretAuditAction = "share_redeem"
	SecretAuditActionShareRevoke SecretAuditAction = "share_revoke"
)

// Valid reports whether a is one of the known SecretAuditAction values.
func (a SecretAuditAction) Valid() bool {
	switch a {
	case SecretAuditActionCreate, SecretAuditActionReveal, SecretAuditActionUpdate,
		SecretAuditActionDelete, SecretAuditActionRestore,
		SecretAuditActionShareCreate, SecretAuditActionShareRedeem, SecretAuditActionShareRevoke:
		return true
	}
	return false
}

// SecretAuditLog is a single audit entry. secret_id is intentionally NOT a
// foreign key — the log must survive hard deletion of the parent secret if a
// future hard-purge mechanism is added (out of scope per spec §9).
//
// Metadata uses json.RawMessage so JSON output preserves the JSONB blob as-is
// instead of base64-encoding it (which []byte would do).
type SecretAuditLog struct {
	ID          int64             `json:"id"`
	SecretID    int64             `json:"secret_id"`
	Action      SecretAuditAction `json:"action"`
	ActorUserID *int64            `json:"actor_user_id,omitempty"`
	ActorIP     *string           `json:"actor_ip,omitempty"`
	ShareLinkID *int64            `json:"share_link_id,omitempty"`
	Metadata    json.RawMessage   `json:"metadata,omitempty"`
	At          time.Time         `json:"at"`
}
