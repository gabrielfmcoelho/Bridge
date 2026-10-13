package coolify

import (
	"log"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// EnsurePrivateKey finds the Coolify private key with privKey's SHA256
// fingerprint, otherwise uploads it as keyName. Returns the key UUID and
// whether it already existed. A key is matched by name only when privKey's
// fingerprint can't be computed: a vault key rotated under the same name must
// not resolve to the stale copy in Coolify.
func (c *Client) EnsurePrivateKey(privKey, keyName, description string) (string, bool, error) {
	fp := ""
	if signer, err := gossh.ParsePrivateKey([]byte(privKey)); err == nil {
		fp = strings.TrimPrefix(gossh.FingerprintSHA256(signer.PublicKey()), "SHA256:")
	} else {
		log.Printf("[coolify] failed to parse private key for fingerprint: %v", err)
	}
	match := func(keys []PrivateKey) string {
		for _, k := range keys {
			if (fp != "" && k.Fingerprint == fp) || (fp == "" && k.Name == keyName) {
				return k.UUID
			}
		}
		return ""
	}

	existing, listErr := c.ListPrivateKeys()
	if listErr != nil {
		log.Printf("[coolify] failed to list keys: %v", listErr)
	}
	if uuid := match(existing); uuid != "" {
		log.Printf("[coolify] reusing key %s (fp=%q)", uuid, fp)
		return uuid, true, nil
	}

	uuid, createErr := c.CreatePrivateKey(CreateKeyRequest{
		Name:        keyName,
		Description: description,
		PrivateKey:  privKey,
	})
	if createErr == nil {
		log.Printf("[coolify] key created: %s", uuid)
		return uuid, false, nil
	}
	if !strings.Contains(createErr.Error(), "422") && !strings.Contains(createErr.Error(), "already exists") {
		return "", false, createErr
	}
	// 422 duplicate: the first list failed or raced with another upload.
	fresh, _ := c.ListPrivateKeys()
	if uuid := match(fresh); uuid != "" {
		return uuid, true, nil
	}
	return "", false, createErr
}
