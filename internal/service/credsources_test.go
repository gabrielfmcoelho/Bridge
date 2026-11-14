package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Distinctive fixture secrets (never real) so a leak is grep-able.
const (
	leakDBPW  = "LEAKPW-dbadmin-9f3a"
	leakEnvPW = "LEAKPW-dotenv-7c21"
	leakKey   = "LEAKKEY-rootssh-4b88"
)

func credScanFixture() *sshtest.VMInfo {
	return &sshtest.VMInfo{
		CredSourcesScanned: true,
		CredHits: []sshtest.CredHit{
			{ // admin-tier DB-admin login
				Detector: "mysql_debian_cnf", Path: "/etc/mysql/debian.cnf", Locator: "client",
				Kind: "cred", Admin: true, Site: "mysql-aaa111",
				Fields: map[string]string{"engine": "mysql", "user": "debian-sys-maint"},
				Secret: `{"username":"debian-sys-maint","password":"` + leakDBPW + `"}`,
			},
			{ // normal-tier .env key
				Detector: "dotenv", Path: "/srv/app/.env", Locator: "DB_PASSWORD",
				Kind: "env_var", Admin: false, Site: "app-bbb222",
				Secret: leakEnvPW,
			},
			{ // admin-tier root key
				Detector: "root_ssh_keys", Path: "/root/.ssh/id_rsa", Locator: "id_rsa",
				Kind: "sshkey", Admin: true, Site: "ssh-ccc333",
				Fields: map[string]string{"user": "root"},
				Secret: `{"username":"root","private_key_pem":"-----BEGIN OPENSSH PRIVATE KEY-----\n` + leakKey + `\n-----END OPENSSH PRIVATE KEY-----","public_key":""}`,
			},
			{ // an indirect reference: no value stored
				Detector: "dotenv", Path: "/srv/app/.env", Locator: "MAIL_PASSWORD",
				Kind: "env_var", Admin: false, Site: "app-bbb222", Secret: "",
			},
		},
	}
}

// The whole point of the feature: a captured value reaches the vault and
// nowhere else. It must not appear in the stored scan JSON (host_scans.data),
// the returned error, the credential_sources provenance, or the logs — only an
// audited reveal returns it, and the admin tier is admin-only. The fix-plan
// tools read the stored scan JSON, so a clean scan JSON keeps values away from
// them too.
func TestCredentialSourcesNoLeak(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx := context.Background()

	var logBuf bytes.Buffer
	origLog := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(origLog)

	info := credScanFixture()
	n, err := RecordScanSecrets(ctx, d, hid, uid, info)
	if err != nil {
		t.Fatalf("RecordScanSecrets: %v", err)
	}
	if n != 3 {
		t.Fatalf("stored %d values, want 3 (the indirect one stores nothing)", n)
	}

	// The plaintext is dropped from memory.
	if info.CredHits != nil {
		t.Fatal("CredHits not cleared after recording")
	}
	// The stored scan JSON (host_scans.data + the API response are built from it).
	scanJSON, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	leaks := []string{leakDBPW, leakEnvPW, leakKey}
	for _, v := range leaks {
		if strings.Contains(string(scanJSON), v) {
			t.Errorf("scan JSON leaks %q", v)
		}
		if strings.Contains(logBuf.String(), v) {
			t.Errorf("logs leak %q", v)
		}
	}

	// credential_sources holds tokens + field names only, one per hit (indirect
	// included), the indirect one with no token.
	if len(info.CredentialSources) != 4 {
		t.Fatalf("credential_sources = %d, want 4", len(info.CredentialSources))
	}
	tokens := map[string]string{} // locator → token
	for _, cs := range info.CredentialSources {
		csJSON, _ := json.Marshal(cs)
		for _, v := range leaks {
			if strings.Contains(string(csJSON), v) {
				t.Errorf("credential_source %s leaks %q", cs.Locator, v)
			}
		}
		tokens[cs.Locator] = cs.Token
	}
	if tokens["MAIL_PASSWORD"] != "" {
		t.Errorf("indirect reference got a token: %q", tokens["MAIL_PASSWORD"])
	}
	for _, loc := range []string{"client", "DB_PASSWORD", "id_rsa"} {
		if !strings.HasPrefix(tokens[loc], "{{secret:") {
			t.Errorf("%s has no vault token: %q", loc, tokens[loc])
		}
	}

	// The value is retrievable — by an admin, through the audited reveal — which
	// proves it was stored, not dropped. The admin tier is admin-only.
	repo := vault.NewSecretRepo(d)
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	var eid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('ed','x','editor') RETURNING id`).Scan(&eid); err != nil {
		t.Fatal(err)
	}
	editor := vault.ActorContext{UserID: eid, Role: "editor"}

	secretID := func(token string) int64 {
		id, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(token, "{{secret:"), "}}"), 10, 64)
		return id
	}
	envID, dbID := secretID(tokens["DB_PASSWORD"]), secretID(tokens["client"])

	// Normal tier: editor (seeded hosts.secrets.reveal) may reveal.
	if got, err := repo.Reveal(ctx, editor, envID); err != nil || got != `{"value":"`+leakEnvPW+`"}` {
		t.Fatalf("editor reveal .env = %q, %v", got, err)
	}
	// Admin tier: editor refused, admin allowed.
	if _, err := repo.Reveal(ctx, editor, dbID); !errors.Is(err, vault.ErrSecretForbidden) {
		t.Fatalf("editor reveal admin-tier: err = %v, want forbidden", err)
	}
	if got, err := repo.Reveal(ctx, admin, dbID); err != nil || !strings.Contains(got, leakDBPW) {
		t.Fatalf("admin reveal admin-tier = %q, %v", got, err)
	}

	// The admin-tier rows live under scan-admin-; the normal row does not.
	for _, c := range []struct {
		id    int64
		admin bool
	}{{dbID, true}, {envID, false}} {
		var group string
		if err := d.SQL.QueryRow(`SELECT group_label FROM secrets WHERE id = ?`, c.id).Scan(&group); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(group, "scan-admin-") != c.admin {
			t.Errorf("secret %d group = %q, admin-tier want %v", c.id, group, c.admin)
		}
	}
}
