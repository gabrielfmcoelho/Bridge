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
	"regexp"
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

// envVarNameRe matches POSIX-style env var names (Task 2.1 / spec §4.2):
// must begin with an uppercase letter or underscore, then uppercase letters,
// digits, or underscores. Rejects lowercase to prevent the foot-gun where
// `db_url` and `DB_URL` are treated as distinct vars by the unique index.
var envVarNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// envVarGroupRe matches the kebab-case environment label used by env_var
// bundling (Task 2.1). Lowercase only so URLs and filter chips render
// uniformly, no underscores so it's visually distinct from var names.
var envVarGroupRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidateEnvVarName reports whether s satisfies the env_var.name contract.
// Exported so the bulk-upsert handler can validate each item in a batch
// before opening a transaction.
func ValidateEnvVarName(s string) error {
	if s == "" {
		return fmt.Errorf("env_var name required")
	}
	if !envVarNameRe.MatchString(s) {
		return fmt.Errorf("env_var name %q must match %s", s, envVarNameRe.String())
	}
	return nil
}

// ValidateEnvVarGroupLabel reports whether s satisfies env_var.group_label.
// Empty/nil is rejected — env_var rows are required to declare which
// environment they belong to (D5).
func ValidateEnvVarGroupLabel(s string) error {
	if s == "" {
		return fmt.Errorf("env_var group_label required")
	}
	if !envVarGroupRe.MatchString(s) {
		return fmt.Errorf("env_var group_label %q must match %s", s, envVarGroupRe.String())
	}
	return nil
}

// ValidateAppLoginPayload (Task 2.4) decodes the spec §4.2 app_login JSON
// shape `{app_name,url?,username,password,notes?}` and checks that the
// three required fields are non-empty. Called by the create/update handlers
// before encryption; the encrypted blob itself is opaque to the repo.
func ValidateAppLoginPayload(plaintext string) error {
	var p struct {
		AppName  string `json:"app_name"`
		Username string `json:"username"`
		Password string `json:"password"`
		// url + notes are optional — declared here so unknown-field strictness
		// (if ever enabled) wouldn't reject them; values themselves are not
		// validated beyond their absence in the required-set check.
		URL   string `json:"url,omitempty"`
		Notes string `json:"notes,omitempty"`
	}
	if err := json.Unmarshal([]byte(plaintext), &p); err != nil {
		return fmt.Errorf("app_login payload: invalid JSON: %w", err)
	}
	if p.AppName == "" {
		return fmt.Errorf("app_login payload: app_name required")
	}
	if p.Username == "" {
		return fmt.Errorf("app_login payload: username required")
	}
	if p.Password == "" {
		return fmt.Errorf("app_login payload: password required")
	}
	return nil
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
	// Type-specific metadata validation (Task 2.1).
	// The payload itself isn't visible here — handlers that encrypt and
	// pass the ciphertext are responsible for validating payload content
	// via ValidateEnvVarName/ValidateAppLoginPayload before they reach Create.
	if s.Type == SecretTypeEnvVar {
		if err := ValidateEnvVarName(s.Name); err != nil {
			return err
		}
		if s.GroupLabel == nil {
			return fmt.Errorf("env_var requires group_label")
		}
		if err := ValidateEnvVarGroupLabel(*s.GroupLabel); err != nil {
			return err
		}
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
