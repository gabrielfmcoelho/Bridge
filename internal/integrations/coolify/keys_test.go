package coolify

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// fakeKeys serves GET/POST /api/v1/security/keys with the given listing;
// a POST answers uuid "created".
func fakeKeys(t *testing.T, keys []PrivateKey) (*Client, *bool) {
	created := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/security/keys" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Method == "POST" {
			created = true
			json.NewEncoder(w).Encode(map[string]string{"uuid": "created"})
			return
		}
		json.NewEncoder(w).Encode(keys)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "tok"), &created
}

func TestEnsurePrivateKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	pemText := string(pem.EncodeToMemory(block))
	signer, _ := gossh.NewSignerFromKey(priv)
	fp := strings.TrimPrefix(gossh.FingerprintSHA256(signer.PublicKey()), "SHA256:")

	// A rotated vault key keeps its name: the stale same-name key must not match.
	c, created := fakeKeys(t, []PrivateKey{{UUID: "stale", Name: "sshcm-key-k", Fingerprint: "old"}})
	uuid, existed, err := c.EnsurePrivateKey(pemText, "sshcm-key-k", "")
	if err != nil || uuid != "created" || existed || !*created {
		t.Fatalf("rotated key: uuid=%q existed=%v created=%v err=%v", uuid, existed, *created, err)
	}

	// Same key under another (legacy) name is reused.
	c, created = fakeKeys(t, []PrivateKey{{UUID: "legacy", Name: "sshcm-k", Fingerprint: fp}})
	uuid, existed, err = c.EnsurePrivateKey(pemText, "sshcm-key-k", "")
	if err != nil || uuid != "legacy" || !existed || *created {
		t.Fatalf("fingerprint match: uuid=%q existed=%v created=%v err=%v", uuid, existed, *created, err)
	}
}
