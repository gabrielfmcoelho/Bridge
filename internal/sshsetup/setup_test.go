package sshsetup

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// A generated key must log in later: ssh.ParsePrivateKey reads PEM text only.
// An existing key installs the public half of the key it was given; with no
// key and no path it refuses instead of reading "".
func TestResolveKey(t *testing.T) {
	gen, err := resolveKey(Request{Host: "vm-a", Mode: "generate"})
	if err != nil || !gen.Generated {
		t.Fatalf("generate = %+v, %v", gen, err)
	}
	signer, err := ssh.ParsePrivateKey(gen.PrivKeyPEM)
	if err != nil {
		t.Fatalf("a generated key must parse back: %v", err)
	}
	pub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if !strings.HasPrefix(gen.PubKeyLine, pub+" ") {
		t.Fatalf("generated pub %q is not the key's %q", gen.PubKeyLine, pub)
	}

	got, err := resolveKey(Request{Mode: "existing", Key: gen.PrivKeyPEM})
	if err != nil || got.Generated || got.PubKeyLine != pub || string(got.PrivKeyPEM) != string(gen.PrivKeyPEM) {
		t.Fatalf("existing = %+v, %v", got, err)
	}
	if _, err := resolveKey(Request{Mode: "existing", Key: []byte("not a key")}); err == nil {
		t.Fatal("an unusable key must be refused")
	}
	if _, err := resolveKey(Request{Mode: "existing"}); err == nil || !strings.Contains(err.Error(), "no existing key") {
		t.Fatalf("no key, no path = %v", err)
	}
}
