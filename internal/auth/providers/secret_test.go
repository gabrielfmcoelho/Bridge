package providers

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// TestProviders_DecryptSetting_RoundTrip is the safety net for the R3 move of
// integration secrets from app_settings into app_secrets: each SSO provider must
// still resolve its stored secret to plaintext through the new AppSecretRepo.
// (No provider-path coverage existed before this change.)
func TestProviders_DecryptSetting_RoundTrip(t *testing.T) {
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	secrets := store.NewAppSecretRepo(d.SQL)

	// Seed each provider's secret the same way the settings handler does.
	must := func(key, val string) {
		if err := secrets.Store(ctx, d.Encryptor, key, val); err != nil {
			t.Fatalf("store %s: %v", key, err)
		}
	}
	must("auth_ldap_bind_password", "ldap-bind-pw")
	must("auth_keycloak_client_secret", "kc-client-secret")
	must("auth_gitlab_client_secret", "gl-client-secret")

	cases := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"ldap", func() (string, error) {
			return NewLDAPProvider(d.SQL, d.Encryptor).decryptSetting("auth_ldap_bind_password")
		}, "ldap-bind-pw"},
		{"keycloak", func() (string, error) {
			return NewKeycloakProvider(d.SQL, d.Encryptor).decryptSetting("auth_keycloak_client_secret")
		}, "kc-client-secret"},
		{"gitlab", func() (string, error) {
			return NewGitLabProvider(d.SQL, d.Encryptor).decryptSetting("auth_gitlab_client_secret")
		}, "gl-client-secret"},
	}
	for _, c := range cases {
		got, err := c.got()
		if err != nil || got != c.want {
			t.Errorf("%s decryptSetting = %q, %v; want %q, nil", c.name, got, err, c.want)
		}
	}

	// An unconfigured secret resolves to "" with no error (the "not configured"
	// contract every provider relies on).
	if v, err := NewLDAPProvider(d.SQL, d.Encryptor).decryptSetting("auth_ldap_bind_password_absent"); v != "" || err != nil {
		t.Fatalf("absent secret = %q, %v; want \"\", nil", v, err)
	}
}
