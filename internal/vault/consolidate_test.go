package vault_test

import (
	"context"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Three VMs with the same deploy password and two with the same key: the plan
// shows both groups (the key reusing the library credential), applying links
// the hosts, drops the copies, and every host still logs in the same way.
func TestConsolidateHostCredentials(t *testing.T) {
	d, h1, uid := newHostSecretFixture(t)
	ctx := context.Background()
	mkHost := func(n string) int64 {
		var id int64
		d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname, ssh_user) VALUES (?,?,'x','deploy') RETURNING id`, n, n).Scan(&id)
		return id
	}
	h2, h3, h4 := mkHost("h2"), mkHost("h3"), mkHost("h4")
	for _, h := range []int64{h1, h2, h3} {
		if err := vault.HostSetPassword(ctx, d, h, uid, "same-pw"); err != nil {
			t.Fatal(err)
		}
	}
	vault.HostSetPassword(ctx, d, h4, uid, "other-pw")
	priv, _ := testKeyPEM(t)
	for _, h := range []int64{h2, h3} {
		vault.HostSetSSHKey(ctx, d, h, uid, vault.HostSSHKey{PrivateKeyPEM: priv})
	}
	// The same key already lives in the vault as a shared credential.
	lib := createSecret(t, vault.NewSecretRepo(d), vault.ActorContext{UserID: uid, Role: "admin"},
		"sshkey", "avulso", "shared", nil, "deploy-key", `{"username":"deploy","private_key_pem":`+jsonString(priv)+`}`)

	plan, err := vault.PlanHostCredentialConsolidation(ctx, d.SQL)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("plan = %+v; want password×3 and key×2", plan)
	}
	var keys []string
	for _, g := range plan {
		keys = append(keys, g.Key)
		if g.Type == "sshkey" && (g.TargetID == nil || *g.TargetID != lib) {
			t.Fatalf("key group should reuse the library key %d: %+v", lib, g)
		}
	}
	res, err := vault.ApplyHostCredentialConsolidation(ctx, d.SQL, d.Encryptor, uid, keys)
	if err != nil {
		t.Fatal(err)
	}
	if res.Groups != 2 || res.Created != 1 || res.Hosts != 5 {
		t.Fatalf("result = %+v", res)
	}
	for _, h := range []int64{h1, h2, h3} {
		if pw, ok, _ := vault.HostGetPassword(ctx, d, h); !ok || pw != "same-pw" {
			t.Fatalf("host %d password after = %q", h, pw)
		}
	}
	if pw, _, _ := vault.HostGetPassword(ctx, d, h4); pw != "other-pw" {
		t.Fatalf("untouched host changed: %q", pw)
	}
	for _, h := range []int64{h2, h3} {
		if k, ok, _ := vault.HostGetSSHKey(ctx, d, h); !ok || k.PrivateKeyPEM != priv {
			t.Fatalf("host %d key after consolidation lost", h)
		}
	}
	var copies int
	d.SQL.QueryRow(`SELECT COUNT(*) FROM secrets WHERE scope = 'host' AND deleted_at IS NULL AND parent_id <> ?`, h4).Scan(&copies)
	if copies != 0 {
		t.Fatalf("%d per-host copies left", copies)
	}
	if plan, _ := vault.PlanHostCredentialConsolidation(ctx, d.SQL); len(plan) != 0 {
		t.Fatalf("plan after apply = %+v, want empty", plan)
	}
}
