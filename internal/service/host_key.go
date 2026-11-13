package service

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// HostKeyChangedError is a host that showed another SSH host key than the
// one pinned for it: a reinstalled VM, or someone else at its address. An
// admin resets the pin once the change is known to be legitimate.
type HostKeyChangedError struct{ Expected, Got string }

func (e *HostKeyChangedError) Error() string {
	return fmt.Sprintf("host key changed (expected %s, got %s)", e.Expected, e.Got)
}

// HostKeyFingerprint is the SHA256 fingerprint and type of an
// authorized_keys line, "" when it doesn't parse.
func HostKeyFingerprint(line string) (fingerprint, keyType string) {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return "", ""
	}
	return ssh.FingerprintSHA256(key), key.Type()
}

// HostKeyCheck builds how a dial to host checks its SSH host key. A pinned
// key must match, and its algorithms are offered first so the server shows
// that key whatever x/crypto prefers. With no pin any key passes and pin
// saves it: call pin only after the login succeeded (trust on first use), so
// an address that refuses the credentials never becomes the pin.
func HostKeyCheck(ctx context.Context, db *database.DB, host *models.Host) (hk sshtest.HostKey, pin func(context.Context) error, err error) {
	repo := store.NewHostRepo(db.SQL)
	stored, _, err := repo.SSHHostKey(ctx, host.ID)
	if err != nil {
		return hk, nil, fmt.Errorf("load pinned host key: %w", err)
	}
	var want ssh.PublicKey
	if stored != "" {
		if want, _, _, _, err = ssh.ParseAuthorizedKey([]byte(stored)); err != nil {
			return hk, nil, fmt.Errorf("parse pinned host key: %w", err)
		}
		first := []string{want.Type()}
		if want.Type() == ssh.KeyAlgoRSA { // one RSA key, three signature algorithms
			first = []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA}
		}
		// The rest after it: a host that lost that key type then fails as
		// "host key changed", not "no common algorithm".
		hk.Algorithms = first
		for _, a := range append(ssh.SupportedAlgorithms().HostKeys, ssh.InsecureAlgorithms().HostKeys...) {
			if !slices.Contains(hk.Algorithms, a) {
				hk.Algorithms = append(hk.Algorithms, a)
			}
		}
	}
	var seen ssh.PublicKey
	hk.Check = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if want != nil && !bytes.Equal(key.Marshal(), want.Marshal()) {
			return &HostKeyChangedError{Expected: ssh.FingerprintSHA256(want), Got: ssh.FingerprintSHA256(key)}
		}
		seen = key
		return nil
	}
	pin = func(ctx context.Context) error {
		if want != nil || seen == nil {
			return nil
		}
		line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(seen)))
		pinned, err := repo.PinSSHHostKey(ctx, host.ID, line)
		if err != nil {
			return fmt.Errorf("pin host key: %w", err)
		}
		if pinned != line { // a concurrent login pinned another key first
			fp, _ := HostKeyFingerprint(pinned)
			return &HostKeyChangedError{Expected: fp, Got: ssh.FingerprintSHA256(seen)}
		}
		return nil
	}
	return hk, pin, nil
}

// DialHost logs in to host as user, checking its SSH host key
// (HostKeyCheck) and pinning it on the first successful login.
func DialHost(ctx context.Context, db *database.DB, host *models.Host, user string, auth sshtest.Auth) (*ssh.Client, error) {
	hk, pin, err := HostKeyCheck(ctx, db, host)
	if err != nil {
		return nil, err
	}
	client, err := sshtest.Dial(host.Hostname, host.Port, user, auth, hk)
	if err != nil {
		return nil, err
	}
	if err := pin(ctx); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}
