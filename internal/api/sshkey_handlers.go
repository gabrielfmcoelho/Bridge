package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	gossh "golang.org/x/crypto/ssh"
)

type sshKeyHandlers struct {
	db *database.DB
}

func (h *sshKeyHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	keys, err := models.ListSSHKeys(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list credentials")
		return
	}
	type keyInfo struct {
		models.SSHKey
		HasPublicKey  bool `json:"has_public_key"`
		HasPrivateKey bool `json:"has_private_key"`
		HasPassword   bool `json:"has_password"`
	}
	result := make([]keyInfo, len(keys))
	for i, k := range keys {
		result[i] = keyInfo{
			SSHKey:        k,
			HasPublicKey:  len(k.PubKeyCiphertext) > 0,
			HasPrivateKey: len(k.PrivKeyCiphertext) > 0,
			HasPassword:   len(k.PasswordCiphertext) > 0,
		}
	}
	jsonOK(w, result)
}

func (h *sshKeyHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		CredentialType string `json:"credential_type"` // "key" or "password"
		Username       string `json:"username"`
		Description    string `json:"description"`
		PublicKey      string `json:"public_key"`
		PrivateKey     string `json:"private_key"`
		Password       string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.CredentialType == "" {
		req.CredentialType = "key"
	}
	if req.CredentialType == "key" && req.PublicKey == "" && req.PrivateKey == "" {
		jsonError(w, http.StatusBadRequest, "at least one key (public or private) is required")
		return
	}
	if req.CredentialType == "password" && req.Password == "" {
		jsonError(w, http.StatusBadRequest, "password is required")
		return
	}

	k := &models.SSHKey{Name: req.Name, CredentialType: req.CredentialType, Username: req.Username, Description: req.Description}

	if req.PublicKey != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.PublicKey)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt public key")
			return
		}
		k.PubKeyCiphertext = ct
		k.PubKeyNonce = nonce
		if pub, _, _, _, err := gossh.ParseAuthorizedKey([]byte(req.PublicKey)); err == nil {
			k.Fingerprint = gossh.FingerprintSHA256(pub)
		}
	}

	if req.PrivateKey != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.PrivateKey)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt private key")
			return
		}
		k.PrivKeyCiphertext = ct
		k.PrivKeyNonce = nonce

		// Derive public key and fingerprint from private key when no public key was provided
		if req.PublicKey == "" {
			if signer, err := gossh.ParsePrivateKey([]byte(req.PrivateKey)); err == nil {
				pub := signer.PublicKey()
				k.Fingerprint = gossh.FingerprintSHA256(pub)
				pubKeyStr := string(gossh.MarshalAuthorizedKey(pub))
				if ct, nonce, err := h.db.Encryptor.Encrypt(pubKeyStr); err == nil {
					k.PubKeyCiphertext = ct
					k.PubKeyNonce = nonce
				}
			}
		}
	}

	if req.Password != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.Password)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		k.PasswordCiphertext = ct
		k.PasswordNonce = nonce
	}

	if err := models.CreateSSHKey(h.db.SQL, k); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create credential")
		return
	}
	jsonCreated(w, k)
}

func (h *sshKeyHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	k, err := models.GetSSHKey(h.db.SQL, id)
	if err != nil || k == nil {
		jsonError(w, http.StatusNotFound, "key not found")
		return
	}

	var pubKey, privKey, password string
	if len(k.PubKeyCiphertext) > 0 {
		pubKey, _ = h.db.Encryptor.Decrypt(k.PubKeyCiphertext, k.PubKeyNonce)
	}
	if len(k.PrivKeyCiphertext) > 0 {
		privKey, _ = h.db.Encryptor.Decrypt(k.PrivKeyCiphertext, k.PrivKeyNonce)
	}
	if len(k.PasswordCiphertext) > 0 {
		password, _ = h.db.Encryptor.Decrypt(k.PasswordCiphertext, k.PasswordNonce)
	}

	jsonOK(w, map[string]any{
		"id":              k.ID,
		"name":            k.Name,
		"credential_type": k.CredentialType,
		"username":        k.Username,
		"description":     k.Description,
		"public_key":      pubKey,
		"private_key":     privKey,
		"password":        password,
		"fingerprint":     k.Fingerprint,
		"created_at":      k.CreatedAt,
	})
}

func (h *sshKeyHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := models.GetSSHKey(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "key not found")
		return
	}

	var req struct {
		Name           string `json:"name"`
		CredentialType string `json:"credential_type"`
		Username       string `json:"username"`
		Description    string `json:"description"`
		PublicKey      string `json:"public_key"`
		PrivateKey     string `json:"private_key"`
		Password       string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	k := existing
	if req.Name != "" {
		k.Name = req.Name
	}
	if req.CredentialType != "" {
		k.CredentialType = req.CredentialType
	}
	k.Username = req.Username
	k.Description = req.Description

	if req.PublicKey != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.PublicKey)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt public key")
			return
		}
		k.PubKeyCiphertext = ct
		k.PubKeyNonce = nonce
		if pub, _, _, _, err := gossh.ParseAuthorizedKey([]byte(req.PublicKey)); err == nil {
			k.Fingerprint = gossh.FingerprintSHA256(pub)
		}
	}

	if req.PrivateKey != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.PrivateKey)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt private key")
			return
		}
		k.PrivKeyCiphertext = ct
		k.PrivKeyNonce = nonce

		// Derive public key and fingerprint from private key when no public key was provided
		if req.PublicKey == "" {
			if signer, err := gossh.ParsePrivateKey([]byte(req.PrivateKey)); err == nil {
				pub := signer.PublicKey()
				k.Fingerprint = gossh.FingerprintSHA256(pub)
				pubKeyStr := string(gossh.MarshalAuthorizedKey(pub))
				if ct, nonce, err := h.db.Encryptor.Encrypt(pubKeyStr); err == nil {
					k.PubKeyCiphertext = ct
					k.PubKeyNonce = nonce
				}
			}
		}
	}

	if req.Password != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.Password)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		k.PasswordCiphertext = ct
		k.PasswordNonce = nonce
	}

	if err := models.UpdateSSHKey(h.db.SQL, k); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update credential")
		return
	}
	jsonOK(w, k)
}

func (h *sshKeyHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := models.DeleteSSHKey(h.db.SQL, id); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
