package coolify

import (
	"log"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// EnsurePrivateKey finds an existing Coolify private key matching keyName or the
// key's SHA256 fingerprint, otherwise uploads it. Returns the key UUID. Moved
// verbatim from the api handler (resolvePrivateKeyUUID) — the find-or-create
// dance against Coolify's /security/keys is protocol logic that belongs here.
func (c *Client) EnsurePrivateKey(privKey, keyName, description string) (string, error) {
	existingKeys, listErr := c.ListPrivateKeys()
	if listErr != nil {
		log.Printf("[coolify] failed to list keys: %v", listErr)
	}
	log.Printf("[coolify] found %d existing keys in Coolify", len(existingKeys))

	for _, k := range existingKeys {
		log.Printf("[coolify] key: uuid=%s name=%q fingerprint=%q", k.UUID, k.Name, k.Fingerprint)
		if k.Name == keyName {
			log.Printf("[coolify] matched by name: %s", k.UUID)
			return k.UUID, nil
		}
	}

	ourFingerprint := ""
	if signer, err := gossh.ParsePrivateKey([]byte(privKey)); err == nil {
		ourFingerprint = strings.TrimPrefix(gossh.FingerprintSHA256(signer.PublicKey()), "SHA256:")
		log.Printf("[coolify] our key fingerprint: %s", ourFingerprint)
	} else {
		log.Printf("[coolify] failed to parse private key for fingerprint: %v", err)
	}

	if ourFingerprint != "" {
		for _, k := range existingKeys {
			if k.Fingerprint != "" && k.Fingerprint == ourFingerprint {
				log.Printf("[coolify] matched by fingerprint: %s (name=%q)", k.UUID, k.Name)
				return k.UUID, nil
			}
		}
	}

	uuid, createErr := c.CreatePrivateKey(CreateKeyRequest{
		Name:        keyName,
		Description: description,
		PrivateKey:  privKey,
	})
	if createErr == nil {
		log.Printf("[coolify] key created: %s", uuid)
		return uuid, nil
	}

	log.Printf("[coolify] key create failed: %v", createErr)
	if !strings.Contains(createErr.Error(), "422") && !strings.Contains(createErr.Error(), "already exists") {
		return "", createErr
	}
	log.Printf("[coolify] 422 duplicate — re-listing for fingerprint match, our fp=%q", ourFingerprint)
	freshKeys, _ := c.ListPrivateKeys()
	for _, k := range freshKeys {
		log.Printf("[coolify] re-list key: uuid=%s name=%q fingerprint=%q", k.UUID, k.Name, k.Fingerprint)
		if ourFingerprint != "" && k.Fingerprint == ourFingerprint {
			log.Printf("[coolify] matched on re-list by fingerprint: %s", k.UUID)
			return k.UUID, nil
		}
	}
	return "", createErr
}
