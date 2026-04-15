package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshconfig"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshkeys"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshsetup"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

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

// handlePreviewConfig renders the SSH config that would be generated.
func (h *sshHandlers) handlePreviewConfig(w http.ResponseWriter, r *http.Request) {
	hosts, err := models.ListHostsForSSHConfig(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load hosts")
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
		jsonError(w, http.StatusInternalServerError, "failed to load hosts")
		return
	}

	entries := hostsToEntries(hosts)
	if err := sshconfig.WriteFile(h.configPath, entries); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to write config: "+err.Error())
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
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	var req struct {
		Method  string `json:"method"`  // "password" or "key"
		Capture bool   `json:"capture"` // capture VM specs
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user := host.User
	if user == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}

	result := map[string]any{}

	if req.Capture {
		// Use capture variants that return VM info
		var vmInfo *sshtest.VMInfo
		var testErr error
		switch req.Method {
		case "password":
			if !host.HasPassword {
				jsonError(w, http.StatusBadRequest, "no password stored for this host")
				return
			}
			password, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
				return
			}
			vmInfo, testErr = sshtest.TestWithPasswordCapture(host.Hostname, host.Port, user, password)
		case "key":
			if host.KeyPath == "" {
				jsonError(w, http.StatusBadRequest, "no key configured for this host")
				return
			}
			vmInfo, testErr = sshtest.TestWithKeyCapture(host.Hostname, host.Port, user, host.KeyPath)
		default:
			jsonError(w, http.StatusBadRequest, "method must be 'password' or 'key'")
			return
		}
		if testErr != nil {
			// Mark connection failure
			host.ConnectionsFailed++
			// Update test status for the failed method
			if req.Method == "password" {
				status := "failed"
				host.PasswordTestStatus = &status
			} else if req.Method == "key" {
				status := "failed"
				host.KeyTestStatus = &status
			}
			models.UpdateHost(h.db.SQL, host)
			h.logOperation(r, host.ID, "test", &req.Method, "failed", testErr.Error())
			jsonOK(w, map[string]any{"success": false, "error": testErr.Error()})
			return
		}
		result["success"] = true
		result["vm_info"] = vmInfo
		// Reset failure counter on successful connection
		if host.ConnectionsFailed > 0 {
			host.ConnectionsFailed = 0
		}
		// Update test status for the successful method
		if req.Method == "password" {
			status := "success"
			host.PasswordTestStatus = &status
		} else if req.Method == "key" {
			status := "success"
			host.KeyTestStatus = &status
		}
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

		// Alert: any resource > 80%
		if cpuPct > 80 || ramPct > 80 || diskPct > 80 {
			host.PrecisaManutencao = true
			updated = true
			result["resource_alert"] = true
			// Auto-tag alerta-recursos, remove sub-utilizado
			models.AddTag(h.db.SQL, "host", host.ID, "alerta-recursos")
			models.RemoveTag(h.db.SQL, "host", host.ID, "sub-utilizado")
		} else if cpuPct > 0 && cpuPct < 5 && ramPct > 0 && ramPct < 5 && diskPct > 0 && diskPct < 5 {
			// Sub-utilized: all resources < 5%
			result["sub_utilized"] = true
			models.AddTag(h.db.SQL, "host", host.ID, "sub-utilizado")
			models.RemoveTag(h.db.SQL, "host", host.ID, "alerta-recursos")
			// Clear maintenance flag if it was set by resource alert
			if host.PrecisaManutencao {
				host.PrecisaManutencao = false
				updated = true
			}
		} else {
			// Normal range — remove both auto-tags
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
		h.logOperation(r, host.ID, "test", &req.Method, "success", "")
		// Annotate scanned SSH keys with managed status from DB
		if len(vmInfo.SSHKeys) > 0 {
			dbKeys, _ := models.ListSSHKeys(h.db.SQL)
			for i, sk := range vmInfo.SSHKeys {
				for _, dk := range dbKeys {
					if dk.Fingerprint != "" && dk.Fingerprint == sk.Fingerprint {
						vmInfo.SSHKeys[i].Managed = true
						vmInfo.SSHKeys[i].ManagedName = dk.Name
						break
					}
				}
			}
		}
		// Store scan snapshot in DB
		if scanJSON, err := json.Marshal(vmInfo); err == nil {
			if dbErr := models.CreateHostScan(h.db.SQL, host.ID, string(scanJSON)); dbErr != nil {
				// Include error in response for debugging
				result["scan_save_error"] = dbErr.Error()
			} else {
				result["scan_saved"] = true
			}
		}
	} else {
		var testErr error
		switch req.Method {
		case "password":
			if !host.HasPassword {
				jsonError(w, http.StatusBadRequest, "no password stored for this host")
				return
			}
			password, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
				return
			}
			testErr = sshtest.TestWithPassword(host.Hostname, host.Port, user, password)
		case "key":
			if host.KeyPath == "" {
				jsonError(w, http.StatusBadRequest, "no key configured for this host")
				return
			}
			testErr = sshtest.TestWithKey(host.Hostname, host.Port, user, host.KeyPath)
		default:
			jsonError(w, http.StatusBadRequest, "method must be 'password' or 'key'")
			return
		}
		if testErr != nil {
			// Mark connection failure
			host.ConnectionsFailed++
			// Update test status for the failed method
			if req.Method == "password" {
				status := "failed"
				host.PasswordTestStatus = &status
			} else if req.Method == "key" {
				status := "failed"
				host.KeyTestStatus = &status
			}
			models.UpdateHost(h.db.SQL, host)
			h.logOperation(r, host.ID, "test", &req.Method, "failed", testErr.Error())
			jsonOK(w, map[string]any{"success": false, "error": testErr.Error()})
			return
		}
		result["success"] = true
		// Reset failure counter on successful connection and update test status
		if host.ConnectionsFailed > 0 {
			host.ConnectionsFailed = 0
		}
		// Update test status for the successful method
		if req.Method == "password" {
			status := "success"
			host.PasswordTestStatus = &status
		} else if req.Method == "key" {
			status := "success"
			host.KeyTestStatus = &status
		}
		models.UpdateHost(h.db.SQL, host)
		h.logOperation(r, host.ID, "test", &req.Method, "success", "")
	}

	jsonOK(w, result)
}

// handleFixDevNull attempts to repair /dev/null permissions on remote host.
func (h *sshHandlers) handleFixDevNull(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	var req struct {
		Method string `json:"method"` // "password" or "key"
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user := host.User
	if user == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}

	method := strings.TrimSpace(req.Method)
	if method == "" {
		switch {
		case host.HasPassword && host.KeyPath != "":
			if host.PreferredAuth == "password" || host.PreferredAuth == "key" {
				method = host.PreferredAuth
			} else {
				jsonError(w, http.StatusBadRequest, "host has both auth methods; set preferred auth or choose one")
				return
			}
		case host.HasPassword:
			method = "password"
		case host.KeyPath != "":
			method = "key"
		default:
			jsonError(w, http.StatusBadRequest, "host has no password or key configured")
			return
		}
	}

	var output string
	var fixErr error

	switch method {
	case "password":
		if !host.HasPassword {
			jsonError(w, http.StatusBadRequest, "no password stored for this host")
			return
		}
		password, decErr := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
		if decErr != nil {
			jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
			return
		}
		output, fixErr = sshtest.FixDevNullWithPassword(host.Hostname, host.Port, user, password)
	case "key":
		if host.KeyPath == "" {
			jsonError(w, http.StatusBadRequest, "no key configured for this host")
			return
		}
		output, fixErr = sshtest.FixDevNullWithKey(host.Hostname, host.Port, user, host.KeyPath)
	default:
		jsonError(w, http.StatusBadRequest, "method must be 'password' or 'key'")
		return
	}

	if fixErr != nil {
		h.logOperation(r, host.ID, "fix-dev-null", &method, "failed", fixErr.Error()+"\n"+output)
		jsonOK(w, map[string]any{
			"success": false,
			"error":   fixErr.Error(),
			"output":  output,
		})
		return
	}

	h.logOperation(r, host.ID, "fix-dev-null", &method, "success", output)
	jsonOK(w, map[string]any{
		"success": true,
		"method":  method,
		"output":  output,
		"message": "Remote /dev/null was validated and is now in expected state.",
	})
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
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	password := req.Password
	if req.UseSavedPassword && host.HasPassword {
		pw, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
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

	setupReq := sshsetup.Request{
		Host:            host.OficialSlug,
		HostName:        host.Hostname,
		Port:            host.Port,
		User:            user,
		Password:        password,
		Mode:            req.Mode,
		ExistingKeyPath: existingKeyPath,
	}

	result, err := sshsetup.Execute(setupReq)
	if err != nil {
		pwMethod := "password"
		h.logOperation(r, host.ID, "setup-key", &pwMethod, "failed", err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pwMethodOK := "password"
	h.logOperation(r, host.ID, "setup-key", &pwMethodOK, "success", fmt.Sprintf("mode=%s key=%s", req.Mode, result.PublicKeyPath))
	// Read key file contents and store encrypted in DB as backup.
	var pubKeyCT, pubKeyNonce, privKeyCT, privKeyNonce []byte
	if privBytes, err := os.ReadFile(result.PrivateKeyPath); err == nil {
		privKeyCT, privKeyNonce, _ = h.db.Encryptor.Encrypt(string(privBytes))
	}
	if pubBytes, err := os.ReadFile(result.PublicKeyPath); err == nil {
		pubKeyCT, pubKeyNonce, _ = h.db.Encryptor.Encrypt(string(pubBytes))
	}

	// Update host key fields.
	models.UpdateHostKey(h.db.SQL, host.ID, true, result.PrivateKeyPath, "yes",
		pubKeyCT, pubKeyNonce, privKeyCT, privKeyNonce)

	jsonOK(w, map[string]any{
		"success":    true,
		"generated":  result.Generated,
		"public_key": result.PublicKeyPath,
	})
}

// handleListKeys returns available SSH keys.
func (h *sshHandlers) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := sshkeys.ListKeys()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}
	jsonOK(w, keys)
}

// handleDownloadConfig returns the SSH config as a downloadable file.
func (h *sshHandlers) handleDownloadConfig(w http.ResponseWriter, r *http.Request) {
	hosts, err := models.ListHostsForSSHConfig(h.db.SQL)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load hosts")
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

// handleSetupSudoNopasswd configures passwordless sudo for the remote user.
// Connects via password or key (based on available credentials), but always
// needs the stored password for `sudo -S`.
func (h *sshHandlers) handleSetupSudoNopasswd(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	user := host.User
	if user == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}
	if !host.HasPassword {
		jsonError(w, http.StatusBadRequest, "stored password required for sudo -S")
		return
	}

	password, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}

	// Pick auth method: prefer key if available, fall back to password
	var method string
	var output string
	var setupErr error

	if host.HasKey && host.KeyPath != "" {
		method = "key"
		output, setupErr = sshtest.SetupSudoNopasswdWithKey(host.Hostname, host.Port, user, host.KeyPath, password)
	} else {
		method = "password"
		output, setupErr = sshtest.SetupSudoNopasswdWithPassword(host.Hostname, host.Port, user, password)
	}

	if setupErr != nil {
		h.logOperation(r, host.ID, "setup-sudo-nopasswd", &method, "failed", setupErr.Error()+"\n"+output)
		jsonOK(w, map[string]any{
			"success": false,
			"error":   setupErr.Error(),
			"output":  output,
		})
		return
	}

	h.logOperation(r, host.ID, "setup-sudo-nopasswd", &method, "success", output)
	jsonOK(w, map[string]any{
		"success": true,
		"output":  output,
		"message": "NOPASSWD sudo configured for " + user,
	})
}

// handleCreateRemoteUser creates a new user on the remote host with an authorized
// SSH key and passwordless sudo. The username and public key are provided in the request body.
func (h *sshHandlers) handleCreateRemoteUser(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	var req struct {
		Username string `json:"username"`
		PubKey   string `json:"pub_key"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.PubKey == "" {
		jsonError(w, http.StatusBadRequest, "username and pub_key are required")
		return
	}

	loginUser := host.User
	if loginUser == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}
	if !host.HasPassword {
		jsonError(w, http.StatusBadRequest, "stored password required for sudo")
		return
	}

	password, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}

	var method string
	var output string
	var setupErr error

	if host.HasKey && host.KeyPath != "" {
		method = "key"
		output, setupErr = sshtest.CreateRemoteUserWithKey(host.Hostname, host.Port, loginUser, host.KeyPath, password, req.Username, req.PubKey)
	} else {
		method = "password"
		output, setupErr = sshtest.CreateRemoteUserWithPassword(host.Hostname, host.Port, loginUser, password, req.Username, req.PubKey)
	}

	if setupErr != nil {
		h.logOperation(r, host.ID, "create-remote-user", &method, "failed", setupErr.Error()+"\n"+output)
		jsonOK(w, map[string]any{
			"success": false,
			"error":   setupErr.Error(),
			"output":  output,
		})
		return
	}

	h.logOperation(r, host.ID, "create-remote-user", &method, "success", output)
	jsonOK(w, map[string]any{
		"success": true,
		"output":  output,
		"message": "User " + req.Username + " created with sudo NOPASSWD",
	})
}

// handleDockerSetup checks docker status and optionally adds user to docker group.
func (h *sshHandlers) handleDockerSetup(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	user := host.User
	if user == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}

	var req struct {
		Fix bool `json:"fix"`
	}
	decodeJSON(r, &req)

	var password string
	if host.HasPassword {
		password, _ = h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
	}

	var status *sshtest.DockerStatus
	var method string
	var opErr error

	if host.HasKey && host.KeyPath != "" {
		method = "key"
		status, opErr = sshtest.CheckAndFixDockerGroupWithKey(host.Hostname, host.Port, user, host.KeyPath, password, req.Fix)
	} else if host.HasPassword {
		method = "password"
		status, opErr = sshtest.CheckAndFixDockerGroupWithPassword(host.Hostname, host.Port, user, password, req.Fix)
	} else {
		jsonError(w, http.StatusBadRequest, "no credentials configured")
		return
	}

	if opErr != nil {
		s := "failed"
		host.DockerGroupStatus = &s
		models.UpdateHost(h.db.SQL, host)
		h.logOperation(r, host.ID, "docker-setup", &method, "failed", opErr.Error())
		jsonOK(w, map[string]any{"success": false, "error": opErr.Error()})
		return
	}

	// Persist docker group status
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

// handleListRemoteKeys lists SSH keys found on the remote host.
func (h *sshHandlers) handleListRemoteKeys(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	user := host.User
	if user == "" {
		jsonError(w, http.StatusBadRequest, "host has no user configured")
		return
	}

	var keys []sshtest.RemoteKeyInfo
	var method string
	var listErr error

	if host.HasKey && host.KeyPath != "" {
		method = "key"
		keys, listErr = sshtest.ListRemoteKeysWithKey(host.Hostname, host.Port, user, host.KeyPath)
	} else if host.HasPassword {
		method = "password"
		password, decErr := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
		if decErr != nil {
			jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
			return
		}
		keys, listErr = sshtest.ListRemoteKeysWithPassword(host.Hostname, host.Port, user, password)
	} else {
		jsonError(w, http.StatusBadRequest, "no credentials configured")
		return
	}

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
	if includeKey && host.KeyPath != "" {
		entry.IdentityFile = host.KeyPath
		entry.IdentitiesOnly = host.IdentitiesOnly
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
		jsonError(w, http.StatusInternalServerError, "failed to load operation logs")
		return
	}
	if logs == nil {
		logs = []models.OperationLog{}
	}
	jsonOK(w, logs)
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
