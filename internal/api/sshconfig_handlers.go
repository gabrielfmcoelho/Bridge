package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	grafanaclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/grafana"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshconfig"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshkeys"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshsetup"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// validLinuxUsername matches POSIX portable usernames: starts with lowercase
// letter or underscore, followed by up to 31 lowercase alphanums, hyphens, or
// underscores. This prevents shell metacharacter injection when the name is
// interpolated into remote commands.
var validLinuxUsername = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// validSSHPubKeyPrefix matches the type prefix of an OpenSSH public key line.
var validSSHPubKeyPrefix = regexp.MustCompile(`^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp256|ecdsa-sha2-nistp384|ecdsa-sha2-nistp521|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com)\s+[A-Za-z0-9+/=]+`)

type sshHandlers struct {
	db         *database.DB
	configPath string
}

func (h *sshHandlers) logOperation(r *http.Request, hostID int64, opType string, method *string, status string, output string) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return
	}
	ol := &models.OperationLog{
		HostID:        hostID,
		UserID:        user.ID,
		OperationType: opType,
		AuthMethod:    method,
		Status:        status,
		Output:        output,
	}
	if err := models.CreateOperationLog(h.db.SQL, ol); err != nil {
		log.Printf("[ssh] failed to log operation: %v", err)
	}
}

// requireHost loads a host by slug from the request path. Returns nil and
// writes an HTTP error if not found.
func (h *sshHandlers) requireHost(w http.ResponseWriter, r *http.Request) *models.Host {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return nil
	}
	return host
}

// requireUser returns the host's configured SSH user, or writes an HTTP error.
func (h *sshHandlers) requireUser(w http.ResponseWriter, host *models.Host) string {
	if host.User == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return ""
	}
	return host.User
}


// resolveAuth builds an Auth for the given method. If method is empty, it picks
// the best available method. Returns the resolved method name and Auth, or
// writes an HTTP error and returns false.
func (h *sshHandlers) resolveAuth(w http.ResponseWriter, host *models.Host, method string) (string, sshtest.Auth, bool) {
	// Auto-select method if not specified
	if method == "" {
		switch {
		case host.HasPassword && host.HasKey:
			if host.PreferredAuth == "password" || host.PreferredAuth == "key" {
				method = host.PreferredAuth
			} else {
				jsonError(w, http.StatusBadRequest, "host has both auth methods; set preferred auth or choose one")
				return "", sshtest.Auth{}, false
			}
		case host.HasKey:
			method = "key"
		case host.HasPassword:
			method = "password"
		default:
			jsonError(w, http.StatusBadRequest, "host has no password or key configured")
			return "", sshtest.Auth{}, false
		}
	}

	switch method {
	case "password":
		if !host.HasPassword {
			jsonError(w, http.StatusBadRequest, "no password stored for this host")
			return "", sshtest.Auth{}, false
		}
		pw, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
			return "", sshtest.Auth{}, false
		}
		return "password", sshtest.PasswordAuth(pw), true

	case "key":
		if !host.HasKey {
			jsonError(w, http.StatusBadRequest, "no key configured for this host")
			return "", sshtest.Auth{}, false
		}
		keyPEM, err := resolveHostKeyPEM(h.db, host)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return "", sshtest.Auth{}, false
		}
		// Carry the password for sudo operations even with key auth
		var pw string
		if host.HasPassword {
			pw, _ = h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
		}
		return "key", sshtest.KeyAuth(keyPEM, pw), true

	default:
		jsonError(w, http.StatusBadRequest, "method must be 'password' or 'key'")
		return "", sshtest.Auth{}, false
	}
}

// dial opens an SSH connection using the resolved auth. Returns the client or
// writes an HTTP error and returns nil.
func (h *sshHandlers) dial(w http.ResponseWriter, host *models.Host, user string, auth sshtest.Auth) *ssh.Client {
	client, err := sshtest.Dial(host.Hostname, host.Port, user, auth)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "SSH connect: "+err.Error())
		return nil
	}
	return client
}

// handlePreviewConfig renders the SSH config that would be generated.
func (h *sshHandlers) handlePreviewConfig(w http.ResponseWriter, r *http.Request) {
	hosts, err := models.ListHostsForSSHConfig(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load hosts", err)
		return
	}

	entries := hostsToEntries(hosts)
	content := sshconfig.RenderConfig(entries)

	jsonOK(w, map[string]string{"content": content})
}

// handleGenerateConfig writes the SSH config file from DB data.
func (h *sshHandlers) handleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	hosts, err := models.ListHostsForSSHConfig(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load hosts", err)
		return
	}

	entries := hostsToEntries(hosts)
	if err := sshconfig.WriteFile(h.configPath, entries); err != nil {
		jsonServerError(w, r, "failed to write config: "+err.Error(), err)
		return
	}

	jsonOK(w, map[string]any{
		"status":     "generated",
		"host_count": len(entries),
		"path":       h.configPath,
	})
}

// handleTestConnection tests SSH connectivity to a host.
func (h *sshHandlers) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	var req struct {
		Method  string `json:"method"`  // "password" or "key"
		Capture bool   `json:"capture"` // capture VM specs
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	method, auth, ok := h.resolveAuth(w, host, req.Method)
	if !ok {
		return
	}

	// Helper to update test status on the host record
	setTestStatus := func(status string) {
		if method == "password" {
			host.PasswordTestStatus = &status
		} else {
			host.KeyTestStatus = &status
		}
	}

	// Dial inline (don't use h.dial helper) so we can persist
	// "failed" test status to the DB on connection failures. The helper
	// returns 502 without touching the DB, which leaves rows stuck as
	// "untested" forever even though the user clearly attempted a scan.
	client, dialErr := sshtest.Dial(host.Hostname, host.Port, user, auth)
	if dialErr != nil {
		host.ConnectionsFailed++
		setTestStatus("failed")
		models.UpdateHost(h.db.SQL, host)
		h.logOperation(r, host.ID, "test", &method, "failed", dialErr.Error())
		jsonOK(w, map[string]any{"success": false, "error": "SSH connect: " + dialErr.Error()})
		return
	}
	defer client.Close()

	result := map[string]any{}

	if req.Capture {
		vmInfo, testErr := sshtest.TestCapture(client, auth.Password())
		if testErr != nil {
			host.ConnectionsFailed++
			setTestStatus("failed")
			models.UpdateHost(h.db.SQL, host)
			h.logOperation(r, host.ID, "test", &method, "failed", testErr.Error())
			jsonOK(w, map[string]any{"success": false, "error": testErr.Error()})
			return
		}
		result["success"] = true
		result["vm_info"] = vmInfo
		if host.ConnectionsFailed > 0 {
			host.ConnectionsFailed = 0
		}
		setTestStatus("success")

		// Auto-update host resource fields if empty
		updated := false
		if host.RecursoCPU == "" && vmInfo.CPU != "" {
			host.RecursoCPU = vmInfo.CPU
			updated = true
		}
		if host.RecursoRAM == "" && vmInfo.RAM != "" {
			host.RecursoRAM = vmInfo.RAM
			updated = true
		}
		if host.RecursoArmazenamento == "" && vmInfo.Storage != "" {
			host.RecursoArmazenamento = vmInfo.Storage
			updated = true
		}
		// Check resource usage thresholds
		cpuPct := parsePercent(vmInfo.CPUUsage)
		ramPct := parsePercent(vmInfo.RAMPercent)
		diskPct := parsePercent(vmInfo.DiskPercent)

		if cpuPct > 80 || ramPct > 80 || diskPct > 80 {
			host.PrecisaManutencao = true
			updated = true
			result["resource_alert"] = true
			models.AddTag(h.db.SQL, "host", host.ID, "alerta-recursos")
			models.RemoveTag(h.db.SQL, "host", host.ID, "sub-utilizado")
		} else if cpuPct > 0 && cpuPct < 5 && ramPct > 0 && ramPct < 5 && diskPct > 0 && diskPct < 5 {
			result["sub_utilized"] = true
			models.AddTag(h.db.SQL, "host", host.ID, "sub-utilizado")
			models.RemoveTag(h.db.SQL, "host", host.ID, "alerta-recursos")
			if host.PrecisaManutencao {
				host.PrecisaManutencao = false
				updated = true
			}
		} else {
			models.RemoveTag(h.db.SQL, "host", host.ID, "alerta-recursos")
			models.RemoveTag(h.db.SQL, "host", host.ID, "sub-utilizado")
			if host.PrecisaManutencao {
				host.PrecisaManutencao = false
				updated = true
			}
		}

		if updated {
			models.UpdateHost(h.db.SQL, host)
		}
		h.logOperation(r, host.ID, "test", &method, "success", "")
		// Annotate scanned SSH keys with managed status from DB
		if len(vmInfo.SSHKeys) > 0 {
			dbKeys, _ := models.ListSSHKeys(h.db.SQL)
			matched := 0
			for i, sk := range vmInfo.SSHKeys {
				for _, dk := range dbKeys {
					if dk.Fingerprint != "" && dk.Fingerprint == sk.Fingerprint {
						vmInfo.SSHKeys[i].Managed = true
						vmInfo.SSHKeys[i].ManagedName = dk.Name
						matched++
						break
					}
				}
				if !vmInfo.SSHKeys[i].Managed {
					log.Printf("[ssh] scan-annotate host=%s unmatched user=%s source=%s fp=%s (no DB key with this fingerprint)",
						host.OficialSlug, sk.User, sk.Source, sk.Fingerprint)
				}
			}
			log.Printf("[ssh] scan-annotate host=%s scanned=%d matched=%d db_keys=%d",
				host.OficialSlug, len(vmInfo.SSHKeys), matched, len(dbKeys))
		}
		// Store scan snapshot in DB
		if scanJSON, err := json.Marshal(vmInfo); err == nil {
			if dbErr := models.CreateHostScan(h.db.SQL, host.ID, string(scanJSON)); dbErr != nil {
				result["scan_save_error"] = dbErr.Error()
			} else {
				result["scan_saved"] = true
			}
		}

		// Reconcile container-based services.
		if len(vmInfo.ParsedContainers) > 0 {
			if reconcileErr := models.ReconcileContainerServices(h.db.SQL, host.ID, vmInfo.ParsedContainers); reconcileErr != nil {
				result["reconcile_error"] = reconcileErr.Error()
			} else {
				result["services_reconciled"] = true
			}
		}
	} else {
		testErr := sshtest.Test(client)
		if testErr != nil {
			host.ConnectionsFailed++
			setTestStatus("failed")
			models.UpdateHost(h.db.SQL, host)
			h.logOperation(r, host.ID, "test", &method, "failed", testErr.Error())
			jsonOK(w, map[string]any{"success": false, "error": testErr.Error()})
			return
		}
		result["success"] = true
		if host.ConnectionsFailed > 0 {
			host.ConnectionsFailed = 0
		}
		setTestStatus("success")
		models.UpdateHost(h.db.SQL, host)
		h.logOperation(r, host.ID, "test", &method, "success", "")
	}

	jsonOK(w, result)
}

// handleFixDevNull attempts to repair /dev/null permissions on remote host.
func (h *sshHandlers) handleFixDevNull(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	var req struct {
		Method string `json:"method"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	method, auth, ok := h.resolveAuth(w, host, strings.TrimSpace(req.Method))
	if !ok {
		return
	}
	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	output, fixErr := sshtest.FixDevNull(client)
	if fixErr != nil {
		h.logOperation(r, host.ID, "fix-dev-null", &method, "failed", fixErr.Error()+"\n"+output)
		jsonOK(w, map[string]any{"success": false, "error": fixErr.Error(), "output": output})
		return
	}

	h.logOperation(r, host.ID, "fix-dev-null", &method, "success", output)
	jsonOK(w, map[string]any{"success": true, "method": method, "output": output, "message": "Remote /dev/null was validated and is now in expected state."})
}

// handleSetupKey sets up an SSH key for a host.
func (h *sshHandlers) handleSetupKey(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	var req struct {
		User             string `json:"user"`
		Password         string `json:"password"`
		UseSavedPassword bool   `json:"use_saved_password"`
		Mode             string `json:"mode"` // "generate" or "existing"
		ExistingKeyPath  string `json:"existing_key_path"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	password := req.Password
	if req.UseSavedPassword && host.HasPassword {
		pw, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
		if err != nil {
			jsonServerError(w, r, "failed to decrypt password", err)
			return
		}
		password = pw
	}

	user := req.User
	if user == "" {
		user = host.User
	}
	if user == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}

	existingKeyPath := req.ExistingKeyPath
	// If using "existing" mode and no explicit path was given, use the host's
	// current key_path (already set by linkSSHKey when the key was linked).
	if req.Mode == "existing" && existingKeyPath == "" && host.KeyPath != "" {
		existingKeyPath = host.KeyPath
	}

	// Dial with password auth for the initial key copy
	auth := sshtest.PasswordAuth(password)
	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	setupReq := sshsetup.Request{
		Host:            host.OficialSlug,
		Mode:            req.Mode,
		ExistingKeyPath: existingKeyPath,
	}

	result, err := sshsetup.Execute(client, setupReq)
	if err != nil {
		method := auth.Method()
		h.logOperation(r, host.ID, "setup-key", &method, "failed", err.Error())
		jsonServerError(w, r, err.Error(), err)
		return
	}

	method := auth.Method()
	h.logOperation(r, host.ID, "setup-key", &method, "success", fmt.Sprintf("mode=%s generated=%v", req.Mode, result.Generated))

	// Encrypt key material and store in DB — nothing touches the filesystem.
	privKeyCT, privKeyNonce, _ := h.db.Encryptor.Encrypt(string(result.PrivKeyPEM))
	pubKeyCT, pubKeyNonce, _ := h.db.Encryptor.Encrypt(result.PubKeyLine)

	// key_path is empty — keys live only in the encrypted DB blob.
	models.UpdateHostKey(h.db.SQL, host.ID, true, "", "yes",
		pubKeyCT, pubKeyNonce, privKeyCT, privKeyNonce)

	jsonOK(w, map[string]any{
		"success":   true,
		"generated": result.Generated,
	})
}

// handleListKeys returns available SSH keys.
func (h *sshHandlers) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := sshkeys.ListKeys()
	if err != nil {
		jsonServerError(w, r, "failed to list keys", err)
		return
	}
	jsonOK(w, keys)
}

// handleDownloadConfig returns the SSH config as a downloadable file.
func (h *sshHandlers) handleDownloadConfig(w http.ResponseWriter, r *http.Request) {
	hosts, err := models.ListHostsForSSHConfig(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to load hosts", err)
		return
	}

	entries := hostsToEntries(hosts)
	content := sshconfig.RenderConfig(entries)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=config")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}

// handleServerInfo returns information about the server running SSHCM.
func (h *sshHandlers) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	home, _ := os.UserHomeDir()

	isLocal := false
	remoteAddr := r.RemoteAddr
	if remoteAddr == "127.0.0.1" || remoteAddr == "::1" || remoteAddr == "[::1]" {
		isLocal = true
	}
	// Check X-Forwarded-For as well
	if xff := r.Header.Get("X-Forwarded-For"); xff == "" && isLocal {
		isLocal = true
	}

	jsonOK(w, map[string]any{
		"hostname":    hostname,
		"user":        user,
		"home":        home,
		"config_path": h.configPath,
		"is_local":    isLocal,
		"message":     fmt.Sprintf("SSH operations run from: %s@%s", user, hostname),
	})
}

// handleSetupSudoNopasswd configures passwordless sudo for the remote host's user.
func (h *sshHandlers) handleSetupSudoNopasswd(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	password := auth.Password()
	if password == "" {
		jsonError(w, http.StatusBadRequest, "stored password required for sudo -S")
		return
	}

	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	output, setupErr := sshtest.SetupSudoNopasswd(client, user, password)
	if setupErr != nil {
		h.logOperation(r, host.ID, "setup-sudo-nopasswd", &method, "failed", setupErr.Error()+"\n"+output)
		jsonOK(w, map[string]any{"success": false, "error": setupErr.Error(), "output": output})
		return
	}

	h.logOperation(r, host.ID, "setup-sudo-nopasswd", &method, "success", output)
	jsonOK(w, map[string]any{"success": true, "output": output, "message": "NOPASSWD sudo configured for " + user})
}

// handleCreateRemoteUser creates a new user on the remote host with an authorized
// SSH key and passwordless sudo.
func (h *sshHandlers) handleCreateRemoteUser(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}

	var req struct {
		Username string `json:"username"`
		PubKey   string `json:"pub_key"`
		SSHKeyID int64  `json:"ssh_key_id"`
		Force    bool   `json:"force"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.PubKey == "" {
		jsonError(w, http.StatusBadRequest, "username and pub_key are required")
		return
	}
	if !validLinuxUsername.MatchString(req.Username) {
		jsonError(w, http.StatusBadRequest, "invalid username: must be 1-32 lowercase alphanumeric characters, hyphens, or underscores, starting with a letter or underscore")
		return
	}
	if !validSSHPubKeyPrefix.MatchString(req.PubKey) {
		jsonError(w, http.StatusBadRequest, "invalid public key format: must be a valid OpenSSH public key")
		return
	}

	loginUser := h.requireUser(w, host)
	if loginUser == "" {
		return
	}

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	password := auth.Password()
	if password == "" {
		jsonError(w, http.StatusBadRequest, "stored password required for sudo")
		return
	}

	client := h.dial(w, host, loginUser, auth)
	if client == nil {
		return
	}
	defer client.Close()

	output, setupErr := sshtest.CreateRemoteUser(client, password, req.Username, req.PubKey, req.Force)
	if setupErr != nil {
		resp := map[string]any{"success": false, "error": setupErr.Error(), "output": output}
		if errors.Is(setupErr, sshtest.ErrUserExists) {
			resp["user_exists"] = true
		}
		h.logOperation(r, host.ID, "create-remote-user", &method, "failed", setupErr.Error()+"\n"+output)
		jsonOK(w, resp)
		return
	}

	// Record the linkage so the Coolify integration can auto-pick this key
	// when registering/syncing the server. Non-fatal on failure: the remote
	// account was created successfully; only the convenience lookup is lost.
	var keyIDArg *int64
	if req.SSHKeyID > 0 {
		id := req.SSHKeyID
		keyIDArg = &id
	}
	if linkErr := models.CreateOrUpdateHostRemoteUser(h.db.SQL, host.ID, req.Username, keyIDArg); linkErr != nil {
		log.Printf("[sshcm] create-remote-user host=%d user=%s link-persist error=%v", host.ID, req.Username, linkErr)
	}

	h.logOperation(r, host.ID, "create-remote-user", &method, "success", output)
	jsonOK(w, map[string]any{"success": true, "output": output, "message": "User " + req.Username + " created with sudo NOPASSWD"})
}

// handleDeleteRemoteUser removes a non-system user from the remote host and
// cleans up its NOPASSWD sudoers drop-in. Safety rails live in sshtest.
// DeleteRemoteUser (root, UID<1000, and SSH login user are refused).
func (h *sshHandlers) handleDeleteRemoteUser(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}

	var req struct {
		Username   string `json:"username"`
		RemoveHome bool   `json:"remove_home"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		jsonError(w, http.StatusBadRequest, "username is required")
		return
	}
	if !validLinuxUsername.MatchString(req.Username) {
		jsonError(w, http.StatusBadRequest, "invalid username: must be 1-32 lowercase alphanumeric characters, hyphens, or underscores, starting with a letter or underscore")
		return
	}

	loginUser := h.requireUser(w, host)
	if loginUser == "" {
		return
	}

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	password := auth.Password()
	if password == "" {
		jsonError(w, http.StatusBadRequest, "stored password required for sudo")
		return
	}

	client := h.dial(w, host, loginUser, auth)
	if client == nil {
		return
	}
	defer client.Close()

	output, delErr := sshtest.DeleteRemoteUser(client, password, loginUser, req.Username, req.RemoveHome)
	if delErr != nil {
		resp := map[string]any{"success": false, "error": delErr.Error(), "output": output}
		switch {
		case errors.Is(delErr, sshtest.ErrUserMissing):
			resp["user_missing"] = true
		case errors.Is(delErr, sshtest.ErrUserProtected):
			resp["user_protected"] = true
		}
		h.logOperation(r, host.ID, "delete-remote-user", &method, "failed", delErr.Error()+"\n"+output)
		jsonOK(w, resp)
		return
	}

	if linkErr := models.DeleteHostRemoteUser(h.db.SQL, host.ID, req.Username); linkErr != nil {
		log.Printf("[sshcm] delete-remote-user host=%d user=%s link-cleanup error=%v", host.ID, req.Username, linkErr)
	}

	h.logOperation(r, host.ID, "delete-remote-user", &method, "success", output)
	jsonOK(w, map[string]any{"success": true, "output": output, "message": "User " + req.Username + " deleted"})
}

// handleDockerSetup checks docker status and optionally adds user to docker group.
func (h *sshHandlers) handleDockerSetup(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	var req struct {
		Fix bool `json:"fix"`
	}
	decodeJSON(r, &req)

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	status, opErr := sshtest.CheckAndFixDockerGroup(client, user, auth.Password(), req.Fix)
	if opErr != nil {
		s := "failed"
		host.DockerGroupStatus = &s
		models.UpdateHost(h.db.SQL, host)
		h.logOperation(r, host.ID, "docker-setup", &method, "failed", opErr.Error())
		jsonOK(w, map[string]any{"success": false, "error": opErr.Error()})
		return
	}

	var dockerStatus string
	if !status.Installed {
		dockerStatus = "not_installed"
	} else if !status.NeedsSudo {
		dockerStatus = "ok"
	} else if status.GroupFixApplied {
		dockerStatus = "fixed"
	} else if status.UserInGroup {
		dockerStatus = "needs_relogin"
	} else {
		dockerStatus = "needs_sudo"
	}
	host.DockerGroupStatus = &dockerStatus
	models.UpdateHost(h.db.SQL, host)

	logStatus := "success"
	if status.NeedsSudo && !status.GroupFixApplied {
		logStatus = "info"
	}
	h.logOperation(r, host.ID, "docker-setup", &method, logStatus, status.Message)
	jsonOK(w, map[string]any{"success": true, "status": status})
}

// handleNginxCleanup detects and removes non-containerized nginx from a remote host.
func (h *sshHandlers) handleNginxCleanup(w http.ResponseWriter, r *http.Request) {
	// Recover from any panic to return a proper JSON error.
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[ssh] nginx-cleanup panic: %v", rv)
			jsonServerError(w, r, fmt.Sprintf("internal error: %v", rv), fmt.Errorf("panic: %v", rv))
		}
	}()

	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	var req struct {
		Purge bool `json:"purge"`
	}
	if err := decodeJSON(r, &req); err != nil {
		// Body might be empty; purge defaults to false — that's fine.
		req.Purge = false
	}

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	password := auth.Password()
	if password == "" {
		jsonError(w, http.StatusBadRequest, "stored password required for sudo operations")
		return
	}

	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	status, opErr := sshtest.CleanupNginx(client, password, req.Purge)
	if opErr != nil {
		h.logOperation(r, host.ID, "nginx-cleanup", &method, "failed", opErr.Error())
		jsonOK(w, map[string]any{"success": false, "error": opErr.Error()})
		return
	}

	logStatus := "success"
	if !status.Found || status.IsContainer {
		logStatus = "info"
	}
	h.logOperation(r, host.ID, "nginx-cleanup", &method, logStatus, status.Message)
	jsonOK(w, map[string]any{"success": true, "status": status})
}

// handleGrafanaAgentSetup installs and starts grafana-agent on the target host,
// configured to scrape node_exporter locally and remote_write to the Prometheus
// endpoint from Grafana integration settings. Mirrors handleDockerSetup's shape.
func (h *sshHandlers) handleGrafanaAgentSetup(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[ssh] grafana-agent-setup panic: %v", rv)
			jsonServerError(w, r, fmt.Sprintf("internal error: %v", rv), fmt.Errorf("panic: %v", rv))
		}
	}()

	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	// Load Grafana settings and validate the pieces the Agent actually needs.
	settings, err := grafanaclient.LoadSettings(h.db.SQL, h.db.Encryptor)
	if err != nil {
		jsonServerError(w, r, "failed to load grafana settings", err)
		return
	}
	if !settings.Enabled {
		jsonError(w, http.StatusBadRequest, "Grafana integration is disabled in settings")
		return
	}
	if settings.PromRemoteWriteURL == "" {
		jsonError(w, http.StatusBadRequest,
			"Prometheus remote_write URL is not configured — set it in Settings → Integrations → Grafana")
		return
	}

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	password := auth.Password()
	if password == "" {
		jsonError(w, http.StatusBadRequest, "stored password required for agent install (uses sudo)")
		return
	}

	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	script := grafanaclient.RenderInstallScript(grafanaclient.AgentInstallParams{
		HostLabel:           host.OficialSlug,
		RemoteWriteURL:      settings.PromRemoteWriteURL,
		RemoteWriteUsername: settings.PromRemoteWriteUsername,
		RemoteWritePassword: settings.PromRemoteWritePassword,
	})

	output, runErr := sshtest.RunPrivilegedScript(client, password, script)
	if runErr != nil {
		errMsg := runErr.Error()
		if output != "" {
			errMsg = output
		}
		h.logOperation(r, host.ID, "grafana-agent-setup", &method, "failed", errMsg)
		jsonOK(w, map[string]any{"success": false, "error": errMsg, "output": output})
		return
	}

	h.logOperation(r, host.ID, "grafana-agent-setup", &method, "success", output)
	jsonOK(w, map[string]any{
		"success": true,
		"output":  output,
		"message": "grafana-agent installed and running. Metrics labelled host=" + host.OficialSlug + " will appear within ~1 minute.",
	})
}

// handleListRemoteKeys lists SSH keys found on the remote host.
func (h *sshHandlers) handleListRemoteKeys(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}

	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	keys, listErr := sshtest.ListRemoteKeys(client)
	if listErr != nil {
		h.logOperation(r, host.ID, "list-remote-keys", &method, "failed", listErr.Error())
		jsonOK(w, map[string]any{"success": false, "error": listErr.Error()})
		return
	}

	if keys == nil {
		keys = []sshtest.RemoteKeyInfo{}
	}
	h.logOperation(r, host.ID, "list-remote-keys", &method, "success", fmt.Sprintf("%d keys found", len(keys)))
	jsonOK(w, map[string]any{"success": true, "keys": keys})
}

// identityFileNameSanitizer strips characters that would be awkward in a
// filename while preserving readability for ssh_keys names the user typed.
var identityFileNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// resolveIdentityFile builds the ~/.ssh/<name> path the user is expected to
// save the private key at. It matches the host's stored pub key against the
// centralized ssh_keys table by fingerprint; if no match is found (e.g. a key
// generated directly on the host), it falls back to a slug-based name.
func (h *sshHandlers) resolveIdentityFile(host *models.Host) string {
	fallback := "~/.ssh/id_ed25519_" + identityFileNameSanitizer.ReplaceAllString(host.OficialSlug, "_")

	if len(host.PubKeyCiphertext) == 0 {
		return fallback
	}
	pubPEM, err := h.db.Encryptor.Decrypt(host.PubKeyCiphertext, host.PubKeyNonce)
	if err != nil {
		return fallback
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubPEM))
	if err != nil {
		return fallback
	}
	fp := ssh.FingerprintSHA256(pub)

	keys, err := models.ListSSHKeys(h.db.SQL)
	if err != nil {
		return fallback
	}
	for _, k := range keys {
		if k.Fingerprint != "" && k.Fingerprint == fp && k.Name != "" {
			return "~/.ssh/" + identityFileNameSanitizer.ReplaceAllString(k.Name, "_")
		}
	}
	return fallback
}

// handleHostSSHConfig returns the SSH config snippet for a single host.
func (h *sshHandlers) handleHostSSHConfig(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	includeKey := r.URL.Query().Get("include_key") == "true"

	entry := sshconfig.HostEntry{
		Host:     host.OficialSlug,
		HostName: host.Hostname,
		User:     host.User,
		Port:     host.Port,
	}
	if includeKey && host.HasKey {
		entry.IdentityFile = h.resolveIdentityFile(host)
		entry.IdentitiesOnly = "yes"
	}
	if host.ProxyJump != "" {
		entry.ProxyJump = host.ProxyJump
	}
	if host.ForwardAgent != "" {
		entry.ForwardAgent = host.ForwardAgent
	}

	config := sshconfig.RenderConfig([]sshconfig.HostEntry{entry})
	jsonOK(w, map[string]any{"config": config})
}

// handleOperationLogs returns operation logs for a host.
func (h *sshHandlers) handleOperationLogs(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	logs, err := models.ListOperationLogs(h.db.SQL, host.ID, limit)
	if err != nil {
		jsonServerError(w, r, "failed to load operation logs", err)
		return
	}
	if logs == nil {
		logs = []models.OperationLog{}
	}
	jsonOK(w, logs)
}

// portCheckResult is the per-port outcome of a TCP connect probe.
type portCheckResult struct {
	Port      int    `json:"port"`
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// pingResult is the outcome of an ICMP ping probe. ICMP requires extra
// privileges that aren't available everywhere (containers, etc.), so
// Skipped flags the case where the check couldn't run rather than failed.
type pingResult struct {
	OK        bool   `json:"ok"`
	Skipped   bool   `json:"skipped"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

// runPing shells out to the system `ping` binary. We don't open raw ICMP
// sockets ourselves because that needs CAP_NET_RAW (or root) — the binary
// is usually setuid and works without elevating the Go process.
func runPing(host string) pingResult {
	bin, err := exec.LookPath("ping")
	if err != nil {
		return pingResult{Skipped: true, Error: "ping binary not available on this server"}
	}
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"-n", "3", "-w", "2000", host}
	} else {
		args = []string{"-c", "3", "-W", "2", host}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	latency := time.Since(start).Milliseconds()
	output := strings.TrimSpace(string(out))
	if err != nil {
		// `ping` returns non-zero on packet loss; surface as a normal failure
		// rather than a "skipped" so the user sees the failure clearly.
		return pingResult{OK: false, LatencyMS: latency, Output: output, Error: err.Error()}
	}
	return pingResult{OK: true, LatencyMS: latency, Output: output}
}

// runTCPConnect attempts a TCP handshake to host:port within timeout. We
// only need the connect to succeed — anything we'd write would be protocol
// specific (and might confuse the server).
func runTCPConnect(host string, port int, timeout time.Duration) portCheckResult {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return portCheckResult{Port: port, OK: false, LatencyMS: latency, Error: err.Error()}
	}
	_ = conn.Close()
	return portCheckResult{Port: port, OK: true, LatencyMS: latency}
}

// handleNetworkTest probes a host's reachability without using SSH. It
// always pings (best-effort) and TCP-connects to the configured SSH port,
// plus an optional custom port supplied by the caller. This is the
// "is it even on the network" check before debugging SSH itself.
func (h *sshHandlers) handleNetworkTest(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}

	var req struct {
		Port int `json:"port"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			jsonBadRequest(w, r, "invalid request body", err)
			return
		}
	}
	if req.Port < 0 || req.Port > 65535 {
		jsonError(w, http.StatusBadRequest, "port must be between 1 and 65535")
		return
	}

	hostname := strings.TrimSpace(host.Hostname)
	if hostname == "" {
		jsonError(w, http.StatusBadRequest, "host has no hostname configured")
		return
	}

	ping := runPing(hostname)

	sshPort, err := strconv.Atoi(strings.TrimSpace(host.Port))
	if err != nil || sshPort <= 0 {
		sshPort = 22
	}
	sshTCP := runTCPConnect(hostname, sshPort, 5*time.Second)

	var customTCP *portCheckResult
	if req.Port > 0 && req.Port != sshPort {
		res := runTCPConnect(hostname, req.Port, 5*time.Second)
		customTCP = &res
	}

	// "Success" means at least one probe got through. Pure ICMP success
	// (without any TCP) still counts because it answers "is the box up?".
	success := ping.OK || sshTCP.OK || (customTCP != nil && customTCP.OK)

	// Build a compact log line so operators can scan history quickly.
	var sb strings.Builder
	if ping.Skipped {
		sb.WriteString("ping: skipped")
	} else if ping.OK {
		fmt.Fprintf(&sb, "ping: ok (%dms)", ping.LatencyMS)
	} else {
		fmt.Fprintf(&sb, "ping: failed (%s)", strings.TrimSpace(ping.Error))
	}
	fmt.Fprintf(&sb, "; tcp/%d: ", sshTCP.Port)
	if sshTCP.OK {
		fmt.Fprintf(&sb, "ok (%dms)", sshTCP.LatencyMS)
	} else {
		fmt.Fprintf(&sb, "failed (%s)", strings.TrimSpace(sshTCP.Error))
	}
	if customTCP != nil {
		fmt.Fprintf(&sb, "; tcp/%d: ", customTCP.Port)
		if customTCP.OK {
			fmt.Fprintf(&sb, "ok (%dms)", customTCP.LatencyMS)
		} else {
			fmt.Fprintf(&sb, "failed (%s)", strings.TrimSpace(customTCP.Error))
		}
	}

	logStatus := "success"
	if !success {
		logStatus = "failed"
	}
	h.logOperation(r, host.ID, "network-test", nil, logStatus, sb.String())

	resp := map[string]any{
		"success":  success,
		"hostname": hostname,
		"ping":     ping,
		"ssh_port": sshTCP,
	}
	if customTCP != nil {
		resp["custom_port"] = customTCP
	}
	jsonOK(w, resp)
}

// hostsToEntries converts DB hosts to SSH config HostEntry slice.
func hostsToEntries(hosts []models.Host) []sshconfig.HostEntry {
	entries := make([]sshconfig.HostEntry, len(hosts))
	for i, h := range hosts {
		entries[i] = sshconfig.HostEntry{
			Host:           h.OficialSlug,
			HostName:       h.Hostname,
			User:           h.User,
			Port:           h.Port,
			IdentityFile:   h.KeyPath,
			IdentitiesOnly: h.IdentitiesOnly,
			ProxyJump:      h.ProxyJump,
			ForwardAgent:   h.ForwardAgent,
		}
	}
	return entries
}

// parsePercent extracts a numeric percentage from strings like "45%", "45.2%", "45".
func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
