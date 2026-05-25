package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSecretType_Valid(t *testing.T) {
	cases := map[SecretType]bool{
		SecretTypeCred:     true,
		SecretTypeSSHKey:   true,
		SecretTypePassword: true,
		SecretTypeAppLogin: true,
		SecretTypeEnvVar:   true,
		SecretType("env_bundle"): false, // D5: removed in favor of per-var rows
		SecretType("user"):       false,
		SecretType(""):           false,
	}
	for in, want := range cases {
		if got := in.Valid(); got != want {
			t.Errorf("SecretType(%q).Valid() = %v, want %v", in, got, want)
		}
	}
}

func TestSecretScope_Valid(t *testing.T) {
	cases := map[SecretScope]bool{
		SecretScopeService: true,
		SecretScopeHost:    true,
		SecretScopeTool:    true,
		SecretScopeAvulso:  true,
		SecretScope("user"):       false, // D7: dropped
		SecretScope("standalone"): false, // D7: replaced by avulso
		SecretScope(""):           false,
	}
	for in, want := range cases {
		if got := in.Valid(); got != want {
			t.Errorf("SecretScope(%q).Valid() = %v, want %v", in, got, want)
		}
	}
}

func TestSecretVisibility_Valid(t *testing.T) {
	cases := map[SecretVisibility]bool{
		SecretVisibilityPersonal: true,
		SecretVisibilityShared:   true,
		SecretVisibility(""):       false,
		SecretVisibility("private"): false,
	}
	for in, want := range cases {
		if got := in.Valid(); got != want {
			t.Errorf("SecretVisibility(%q).Valid() = %v, want %v", in, got, want)
		}
	}
}

func TestSecretAuditAction_Valid(t *testing.T) {
	valid := []SecretAuditAction{
		SecretAuditActionCreate,
		SecretAuditActionReveal,
		SecretAuditActionUpdate,
		SecretAuditActionDelete,
		SecretAuditActionRestore,
		SecretAuditActionShareCreate,
		SecretAuditActionShareRedeem,
		SecretAuditActionShareRevoke,
	}
	for _, a := range valid {
		if !a.Valid() {
			t.Errorf("expected %q to be valid", a)
		}
	}
	for _, bad := range []SecretAuditAction{"", "list", "deleted"} {
		if bad.Valid() {
			t.Errorf("expected %q to be invalid", bad)
		}
	}
}

// validBase returns a minimal valid scoped Secret for table tests to mutate.
func validBase() Secret {
	parent := int64(42)
	return Secret{
		Type:              SecretTypeCred,
		Scope:             SecretScopeService,
		Visibility:        SecretVisibilityShared,
		ParentID:          &parent,
		OwnerUserID:       7,
		Name:              "db-root",
		PayloadCiphertext: []byte{0xde, 0xad},
		PayloadNonce:      []byte{0xbe, 0xef},
		KeyVersion:        1,
		CreatedBy:         7,
	}
}

func TestSecret_Validate_Valid(t *testing.T) {
	s := validBase()
	if err := s.Validate(); err != nil {
		t.Fatalf("baseline should be valid: %v", err)
	}

	// avulso with no parent is also valid
	avulso := validBase()
	avulso.Scope = SecretScopeAvulso
	avulso.ParentID = nil
	if err := avulso.Validate(); err != nil {
		t.Fatalf("avulso/no-parent should be valid: %v", err)
	}

	// personal visibility is valid for any scope (D7)
	personal := validBase()
	personal.Visibility = SecretVisibilityPersonal
	if err := personal.Validate(); err != nil {
		t.Fatalf("personal visibility on host scope should be valid: %v", err)
	}
}

func TestSecret_Validate_Invariants(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Secret)
		wantSub string
	}{
		{"empty type", func(s *Secret) { s.Type = "" }, "type"},
		{"unknown type", func(s *Secret) { s.Type = SecretType("nope") }, "type"},
		{"empty scope", func(s *Secret) { s.Scope = "" }, "scope"},
		{"unknown scope", func(s *Secret) { s.Scope = SecretScope("user") }, "scope"},
		{"empty visibility", func(s *Secret) { s.Visibility = "" }, "visibility"},
		{"missing name", func(s *Secret) { s.Name = "" }, "name"},
		{"missing owner", func(s *Secret) { s.OwnerUserID = 0 }, "owner"},
		{
			"avulso with parent",
			func(s *Secret) { s.Scope = SecretScopeAvulso /* keeps ParentID set */ },
			"avulso",
		},
		{
			"scoped without parent",
			func(s *Secret) { s.ParentID = nil /* scope still service */ },
			"parent_id",
		},
		{"zero key_version", func(s *Secret) { s.KeyVersion = 0 }, "key_version"},
		{"negative key_version", func(s *Secret) { s.KeyVersion = -1 }, "key_version"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := validBase()
			tc.mutate(&s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestSecret_JSON_HidesCiphertext(t *testing.T) {
	s := validBase()
	desc := "team postgres root"
	s.Description = &desc
	s.PayloadCiphertext = []byte("super-secret-bytes")
	s.PayloadNonce = []byte("nonce-bytes")
	s.CreatedAt = time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	s.UpdatedAt = s.CreatedAt

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	for _, banned := range []string{"super-secret-bytes", "nonce-bytes", "payload_ciphertext", "payload_nonce"} {
		if strings.Contains(got, banned) {
			t.Errorf("JSON output must not contain %q; got %s", banned, got)
		}
	}
	for _, must := range []string{`"name":"db-root"`, `"scope":"service"`, `"visibility":"shared"`, `"description":"team postgres root"`} {
		if !strings.Contains(got, must) {
			t.Errorf("JSON output missing %q; got %s", must, got)
		}
	}
}

func TestSecret_JSON_OmitsOptionals(t *testing.T) {
	// avulso secret with no parent, no description, no group_label, not deleted
	s := validBase()
	s.Scope = SecretScopeAvulso
	s.ParentID = nil

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, banned := range []string{"parent_id", "description", "group_label", "deleted_at"} {
		if strings.Contains(got, banned) {
			t.Errorf("JSON should omit optional field %q when nil; got %s", banned, got)
		}
	}
}

func TestSecretShareLink_JSON_HidesTokenHash(t *testing.T) {
	link := SecretShareLink{
		ID:             1,
		SecretID:       100,
		TokenHash:      []byte("hash-bytes"),
		PassphraseHash: []byte("argon2id-bytes"),
		ExpiresAt:      time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
		ViewCount:      0,
		CreatedBy:      7,
		CreatedAt:      time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(&link)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, banned := range []string{"hash-bytes", "argon2id-bytes", "token_hash", "passphrase_hash"} {
		if strings.Contains(got, banned) {
			t.Errorf("share-link JSON must not contain %q; got %s", banned, got)
		}
	}
}

func TestSecretAuditLog_MetadataRoundTrip(t *testing.T) {
	// Metadata should be preserved as raw JSON (not double-encoded as base64).
	entry := SecretAuditLog{
		ID:       1,
		SecretID: 100,
		Action:   SecretAuditActionUpdate,
		Metadata: json.RawMessage(`{"changed_fields":["name","payload"]}`),
		At:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(&entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"metadata":{"changed_fields":["name","payload"]}`) {
		t.Errorf("metadata should round-trip as raw JSON; got %s", got)
	}
}
