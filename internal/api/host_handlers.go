package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type hostHandlers struct {
	db *database.DB
}

type scanResources struct {
	CPU        string `json:"cpu,omitempty"`
	CPUUsage   string `json:"cpu_usage,omitempty"`
	RAM        string `json:"ram,omitempty"`
	RAMPercent string `json:"ram_percent,omitempty"`
	Storage    string `json:"storage,omitempty"`
	DiskPct    string `json:"disk_percent,omitempty"`
}

type scanFull struct {
	scanResources
	Services   []string `json:"services"`
	Containers []string `json:"containers"`
}

type hostAlert struct {
	ID            int64  `json:"id"`
	Type          string `json:"type"`
	Level         string `json:"level"`
	Message       string `json:"message"`
	Description   string `json:"description,omitempty"`
	Source        string `json:"source"`
	Status        string `json:"status"`
	LinkedIssueID *int64 `json:"linked_issue_id,omitempty"`
}

func computeAlerts(_ *scanResources, sf *scanFull, host models.Host, t *models.AlertThresholds) []hostAlert {
	var alerts []hostAlert

	parseUsagePct := func(s string) (int, bool) {
		s = strings.TrimSuffix(strings.TrimSpace(s), "%")
		v, err := strconv.Atoi(s)
		return v, err == nil
	}

	if sf != nil {
		// Resource alerts
		for _, r := range []struct {
			typ, label, usage string
		}{
			{"resource_cpu", "CPU", sf.CPUUsage},
			{"resource_ram", "RAM", sf.RAMPercent},
			{"resource_disk", "Disk", sf.DiskPct},
		} {
			if r.usage == "" {
				continue
			}
			pct, ok := parseUsagePct(r.usage)
			if !ok {
				continue
			}
			var level, msg string
			switch {
			case pct >= t.ResourceCritical:
				level = "critical"
				msg = fmt.Sprintf("%s at %d%% (critical: %d%%)", r.label, pct, t.ResourceCritical)
			case pct >= t.ResourceWarning:
				level = "warning"
				msg = fmt.Sprintf("%s at %d%% (warning: %d%%)", r.label, pct, t.ResourceWarning)
			case pct <= t.ResourceInfoLow:
				level = "info"
				msg = fmt.Sprintf("%s at %d%% (sub-utilized)", r.label, pct)
			}
			if level != "" {
				alerts = append(alerts, hostAlert{Type: r.typ, Level: level, Message: msg, Source: "auto", Status: "active"})
			}
		}

		// /dev/null alert
		allFields := sf.CPU + sf.CPUUsage + sf.RAM + sf.RAMPercent + sf.Storage + sf.DiskPct
		lower := strings.ToLower(allFields)
		if strings.Contains(lower, "/dev/null") || strings.Contains(lower, "permission denied") {
			alerts = append(alerts, hostAlert{
				Type:    "dev_null",
				Level:   "warning",
				Message: "Scan returned /dev/null permission warning",
				Source:  "auto",
			})
		}
	}

	// Auth failed alert — only fires when all *tested* credentials failed
	pwdTestedFailed := host.HasPassword && host.PasswordTestStatus != nil && *host.PasswordTestStatus == "failed"
	keyTestedFailed := host.HasKey && host.KeyTestStatus != nil && *host.KeyTestStatus == "failed"
	pwdTestedOK := host.HasPassword && host.PasswordTestStatus != nil && *host.PasswordTestStatus == "success"
	keyTestedOK := host.HasKey && host.KeyTestStatus != nil && *host.KeyTestStatus == "success"

	if (pwdTestedFailed || keyTestedFailed) && !pwdTestedOK && !keyTestedOK {
		alerts = append(alerts, hostAlert{
			Type:    "auth_failed",
			Level:   "warning",
			Message: "No authentication method is working",
			Source:  "auto",
		})
	}

	return alerts
}

func (h *hostHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	f := models.HostFilter{
		Situacao:            r.URL.Query().Get("situacao"),
		Tag:                 r.URL.Query().Get("tag"),
		Hospedagem:          r.URL.Query().Get("hospedagem"),
		Search:              r.URL.Query().Get("search"),
		EntidadeResponsavel: r.URL.Query().Get("entidade_responsavel"),
		ResponsavelInterno:  r.URL.Query().Get("responsavel_interno"),
		KeyTestStatus:       r.URL.Query().Get("key_test_status"),
		PasswordTestStatus:  r.URL.Query().Get("password_test_status"),
		HasScan:             r.URL.Query().Get("has_scan"),
		SortBy:              r.URL.Query().Get("sort_by"),
		SortDir:             r.URL.Query().Get("sort_dir"),
	}

	// Parse pagination
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			f.Page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 && v <= 100 {
			f.PerPage = v
		}
	}

	hosts, err := models.ListHosts(h.db.SQL, f)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list hosts")
		return
	}

	// Attach tags, scan status, and scan resource summary for each host.
	tagMap, _ := models.GetAllTags(h.db.SQL, "host")
	scanStatuses, scanErr := models.GetHostScanStatuses(h.db.SQL)
	if scanErr != nil {
		log.Printf("[hosts] GetHostScanStatuses error: %v", scanErr)
	}
	scanDataBulk, bulkErr := models.GetLatestScanDataBulk(h.db.SQL)
	if bulkErr != nil {
		log.Printf("[hosts] GetLatestScanDataBulk error: %v", bulkErr)
	}
	log.Printf("[hosts] Found %d hosts, %d scan statuses, %d scan data entries", len(hosts), len(scanStatuses), len(scanDataBulk))

	thresholds, thErr := models.GetAlertThresholds(h.db.SQL)
	if thErr != nil {
		log.Printf("[hosts] GetAlertThresholds error: %v", thErr)
		thresholds = &models.AlertThresholds{ResourceCritical: 80, ResourceWarning: 60, ResourceInfoLow: 5}
	}

	type hostWithExtra struct {
		models.Host
		Tags                 []string       `json:"tags"`
		HasScan              bool           `json:"has_scan"`
		LastScanAt           *time.Time     `json:"last_scan_at,omitempty"`
		ScanRes              *scanResources `json:"scan_resources,omitempty"`
		ContainersCount      int            `json:"containers_count"`
		ProcessesCount       int            `json:"processes_count"`
		ServicesCount        int            `json:"services_count"`
		DNSCount             int            `json:"dns_count"`
		IssuesCount          int            `json:"issues_count"`
		CanCompile           bool           `json:"can_compile"`
		Alerts               []hostAlert    `json:"alerts"`
		MainResponsavelName  string         `json:"main_responsavel_name"`
		ChamadosCount        int            `json:"chamados_count"`
		ProjectsCount        int            `json:"projects_count"`
	}

	projCounts, _ := models.GetProjectCountsByHost(h.db.SQL)
	svcCounts, _ := models.GetServiceCountsByHost(h.db.SQL)
	dnsCounts, _ := models.GetDNSCountsByHost(h.db.SQL)
	issueCounts, _ := models.GetIssueCountsByEntity(h.db.SQL, "host")
	mainRespNames, _ := models.GetMainResponsavelNamesBulk(h.db.SQL)
	chamadosCounts, _ := models.GetChamadosCountsBulk(h.db.SQL)
	manualAlertsBulk, _ := models.ListHostAlertsBulk(h.db.SQL)
	alertLinkedIssues, _ := models.GetAlertLinkedIssueIDsBulk(h.db.SQL)

	result := make([]hostWithExtra, len(hosts))
	for i, host := range hosts {
		hwt := hostWithExtra{Host: host, Tags: tagMap[host.ID]}
		hwt.ServicesCount = svcCounts[host.ID]
		hwt.DNSCount = dnsCounts[host.ID]
		hwt.IssuesCount = issueCounts[host.ID]
		hwt.MainResponsavelName = mainRespNames[host.ID]
		hwt.ChamadosCount = chamadosCounts[host.ID]
		hwt.ProjectsCount = projCounts[host.ID]
		hwt.CanCompile = host.Hostname != "" && host.User != ""
		if scanTime, ok := scanStatuses[host.ID]; ok {
			hwt.HasScan = true
			t := scanTime
			hwt.LastScanAt = &t
		}
		var sfPtr *scanFull
		if data, ok := scanDataBulk[host.ID]; ok {
			var sf scanFull
			if err := json.Unmarshal([]byte(data), &sf); err != nil {
				log.Printf("[hosts] Host %d: JSON unmarshal error: %v", host.ID, err)
			} else {
				if sf.CPU != "" || sf.RAM != "" || sf.Storage != "" {
					sr := sf.scanResources
					hwt.ScanRes = &sr
				}
				hwt.ContainersCount = len(sf.Containers)
				hwt.ProcessesCount = len(sf.Services)
				sfPtr = &sf
			}
		}
		hwt.Alerts = computeAlerts(hwt.ScanRes, sfPtr, host, thresholds)
		// Build set of auto-computed alert types for dedup
		autoTypes := make(map[string]bool, len(hwt.Alerts))
		for _, a := range hwt.Alerts {
			autoTypes[a.Type] = true
		}
		// Merge DB alerts, enriched with linked issue IDs.
		// For source="auto" DB alerts that duplicate a computed alert of the
		// same type, replace the computed one (so the DB ID + issue link show up)
		// instead of appending a second copy.
		if manualAlerts, ok := manualAlertsBulk[host.ID]; ok {
			for _, ma := range manualAlerts {
				ha := hostAlert{
					ID:          ma.ID,
					Type:        ma.Type,
					Level:       ma.Level,
					Message:     ma.Message,
					Description: ma.Description,
					Source:      ma.Source,
					Status:      ma.Status,
				}
				if issueID, ok := alertLinkedIssues[ma.ID]; ok {
					id := issueID
					ha.LinkedIssueID = &id
				}
				if ma.Source == "auto" && autoTypes[ma.Type] {
					// Replace the computed alert with the persisted one
					for j, ca := range hwt.Alerts {
						if ca.Source == "auto" && ca.Type == ma.Type && ca.ID == 0 {
							hwt.Alerts[j] = ha
							break
						}
					}
				} else {
					hwt.Alerts = append(hwt.Alerts, ha)
				}
			}
		}
		result[i] = hwt
	}

	// Post-enrichment filter: alert_level
	alertLevelFilter := r.URL.Query().Get("alert_level")
	if alertLevelFilter != "" {
		filtered := result[:0]
		for _, hwt := range result {
			if alertLevelFilter == "none" {
				if len(hwt.Alerts) == 0 {
					filtered = append(filtered, hwt)
				}
			} else {
				for _, a := range hwt.Alerts {
					if a.Level == alertLevelFilter {
						filtered = append(filtered, hwt)
						break
					}
				}
			}
		}
		result = filtered
	}

	// If paginated, wrap in envelope with total count
	if f.PerPage > 0 {
		total, _ := models.CountHosts(h.db.SQL, f)
		totalPages := (total + f.PerPage - 1) / f.PerPage
		page := f.Page
		if page < 1 {
			page = 1
		}
		jsonOK(w, map[string]any{
			"data":        result,
			"total":       total,
			"page":        page,
			"per_page":    f.PerPage,
			"total_pages": totalPages,
		})
		return
	}

	jsonOK(w, result)
}

func (h *hostHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "database error")
		return
	}
	if host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	tags, _ := models.GetTags(h.db.SQL, "host", host.ID)
	orch, _ := models.GetOrchestratorByHost(h.db.SQL, host.ID)
	dns, _ := models.GetHostDNSRecords(h.db.SQL, host.ID)
	services, _ := models.ListServicesByHost(h.db.SQL, host.ID)
	projects, _ := models.ListProjectsByHost(h.db.SQL, host.ID)
	lastScan, _ := models.GetLatestHostScan(h.db.SQL, host.ID)
	responsaveis, _ := models.ListHostResponsaveis(h.db.SQL, host.ID)
	chamados, _ := models.ListHostChamados(h.db.SQL, host.ID)

	jsonOK(w, map[string]any{
		"host":         host,
		"tags":         tags,
		"orchestrator": orch,
		"dns_records":  dns,
		"services":     services,
		"projects":     projects,
		"last_scan":    lastScan,
		"responsaveis": responsaveis,
		"chamados":     chamados,
	})
}

func (h *hostHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Host
		Tags         []string                    `json:"tags"`
		Password     string                      `json:"password"`
		SSHKeyID     int64                       `json:"ssh_key_id"`
		Responsaveis []models.HostResponsavelInput `json:"responsaveis"`
		Chamados     []models.HostChamadoInput     `json:"chamados"`
		DNSIDs       []int64                     `json:"dns_ids"`
		ServiceIDs   []int64                     `json:"service_ids"`
		ProjectIDs   []int64                     `json:"project_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Nickname == "" || req.OficialSlug == "" {
		jsonError(w, http.StatusBadRequest, "nickname and oficial_slug are required")
		return
	}

	exists, _ := models.HostSlugExists(h.db.SQL, req.OficialSlug, 0)
	if exists {
		jsonError(w, http.StatusConflict, "slug already exists")
		return
	}

	// Key material only comes from the ssh_keys table via linkSSHKey. Callers
	// cannot set a filesystem path on the host row — the DB blob is the only
	// source of truth so backups restore cleanly onto any machine.
	req.Host.KeyPath = ""
	req.Host.HasKey = false
	req.Host.PubKeyCiphertext = nil
	req.Host.PubKeyNonce = nil
	req.Host.PrivKeyCiphertext = nil
	req.Host.PrivKeyNonce = nil

	// Handle password encryption.
	if req.Password != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.Password)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		req.Host.HasPassword = true
		req.Host.PasswordCiphertext = ct
		req.Host.PasswordNonce = nonce
	}

	preferredAuth, prefErr := normalizePreferredAuth(req.Host.HasPassword, req.Host.HasKey || req.SSHKeyID > 0, req.Host.PreferredAuth)
	if prefErr != nil {
		jsonError(w, http.StatusBadRequest, prefErr.Error())
		return
	}
	req.Host.PreferredAuth = preferredAuth

	if err := models.CreateHost(h.db.SQL, &req.Host); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create host")
		return
	}

	if len(req.Tags) > 0 {
		models.SetTags(h.db.SQL, "host", req.Host.ID, req.Tags)
	}

	// Sync responsaveis and chamados
	if len(req.Responsaveis) > 0 {
		if err := models.SyncHostResponsaveis(h.db.SQL, req.Host.ID, req.Responsaveis); err != nil {
			log.Printf("[hosts] SyncHostResponsaveis error on create: %v", err)
		}
	}
	if len(req.Chamados) > 0 {
		if err := models.SyncHostChamados(h.db.SQL, req.Host.ID, req.Chamados); err != nil {
			log.Printf("[hosts] SyncHostChamados error on create: %v", err)
		}
	}

	// Link SSH key from DB if provided.
	if req.SSHKeyID > 0 {
		h.linkSSHKey(req.Host.ID, req.SSHKeyID, req.OficialSlug)
	}

	// Link DNS records and services if provided.
	if len(req.DNSIDs) > 0 {
		if err := models.SetHostDNSLinks(h.db.SQL, req.Host.ID, req.DNSIDs); err != nil {
			log.Printf("[hosts] SetHostDNSLinks error on create: %v", err)
		}
	}
	if len(req.ServiceIDs) > 0 {
		if err := models.SetHostServiceLinks(h.db.SQL, req.Host.ID, req.ServiceIDs); err != nil {
			log.Printf("[hosts] SetHostServiceLinks error on create: %v", err)
		}
	}
	if len(req.ProjectIDs) > 0 {
		if err := models.SetHostProjectLinks(h.db.SQL, req.Host.ID, req.ProjectIDs); err != nil {
			log.Printf("[hosts] SetHostProjectLinks error on create: %v", err)
		}
	}

	jsonCreated(w, req.Host)
}

func (h *hostHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	existing, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	var req struct {
		models.Host
		Tags         []string                      `json:"tags"`
		Password     string                        `json:"password"`
		SSHKeyID     int64                         `json:"ssh_key_id"`
		ClearKey     bool                          `json:"clear_key"`
		Responsaveis *[]models.HostResponsavelInput `json:"responsaveis"`
		Chamados     *[]models.HostChamadoInput     `json:"chamados"`
		DNSIDs       *[]int64                      `json:"dns_ids"`
		ServiceIDs   *[]int64                      `json:"service_ids"`
		ProjectIDs   *[]int64                      `json:"project_ids"`
	}
	req.Host = *existing
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// If only ssh_key_id is provided (no host data), just link the key and return.
	if req.SSHKeyID > 0 && req.Nickname == "" {
		h.linkSSHKey(existing.ID, req.SSHKeyID, existing.OficialSlug)
		updatedHost, getErr := models.GetHostByID(h.db.SQL, existing.ID)
		if getErr == nil && updatedHost != nil {
			preferredAuth, normErr := normalizePreferredAuth(updatedHost.HasPassword, updatedHost.HasKey, updatedHost.PreferredAuth)
			if normErr == nil && preferredAuth != updatedHost.PreferredAuth {
				updatedHost.PreferredAuth = preferredAuth
				_ = models.UpdateHost(h.db.SQL, updatedHost)
			}
		}
		jsonOK(w, existing)
		return
	}

	req.Host.ID = existing.ID
	if req.Host.OficialSlug == "" {
		req.Host.OficialSlug = existing.OficialSlug
	}
	req.Host.OficialSlug = strings.TrimSpace(req.Host.OficialSlug)
	if req.Host.OficialSlug == "" {
		jsonError(w, http.StatusBadRequest, "oficial_slug is required")
		return
	}
	if req.Host.OficialSlug != existing.OficialSlug {
		exists, slugErr := models.HostSlugExists(h.db.SQL, req.Host.OficialSlug, existing.ID)
		if slugErr != nil {
			jsonError(w, http.StatusInternalServerError, "failed to validate slug")
			return
		}
		if exists {
			jsonError(w, http.StatusConflict, "slug already exists")
			return
		}
	}

	// Preserve existing password if not provided.
	if req.Password != "" {
		ct, nonce, err := h.db.Encryptor.Encrypt(req.Password)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		req.Host.HasPassword = true
		req.Host.PasswordCiphertext = ct
		req.Host.PasswordNonce = nonce
	} else {
		req.Host.HasPassword = existing.HasPassword
		req.Host.PasswordCiphertext = existing.PasswordCiphertext
		req.Host.PasswordNonce = existing.PasswordNonce
	}

	// Handle key data.
	if req.ClearKey {
		// Clear all key fields
		req.Host.HasKey = false
		req.Host.KeyPath = ""
		req.Host.IdentitiesOnly = ""
		req.Host.PubKeyCiphertext = nil
		req.Host.PubKeyNonce = nil
		req.Host.PrivKeyCiphertext = nil
		req.Host.PrivKeyNonce = nil
	} else {
		// Preserve existing key data (managed via setup-key endpoint, not direct edit).
		req.Host.HasKey = existing.HasKey
		req.Host.KeyPath = existing.KeyPath
		req.Host.IdentitiesOnly = existing.IdentitiesOnly
		req.Host.PubKeyCiphertext = existing.PubKeyCiphertext
		req.Host.PubKeyNonce = existing.PubKeyNonce
		req.Host.PrivKeyCiphertext = existing.PrivKeyCiphertext
		req.Host.PrivKeyNonce = existing.PrivKeyNonce
	}

	if req.Host.PreferredAuth == "" {
		req.Host.PreferredAuth = existing.PreferredAuth
	}
	preferredAuth, prefErr := normalizePreferredAuth(req.Host.HasPassword, req.Host.HasKey || req.SSHKeyID > 0, req.Host.PreferredAuth)
	if prefErr != nil {
		jsonError(w, http.StatusBadRequest, prefErr.Error())
		return
	}
	req.Host.PreferredAuth = preferredAuth

	if err := models.UpdateHost(h.db.SQL, &req.Host); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update host")
		return
	}

	if req.Tags != nil {
		models.SetTags(h.db.SQL, "host", existing.ID, req.Tags)
	}

	// Sync responsaveis and chamados if provided
	if req.Responsaveis != nil {
		if err := models.SyncHostResponsaveis(h.db.SQL, existing.ID, *req.Responsaveis); err != nil {
			log.Printf("[hosts] SyncHostResponsaveis error on update: %v", err)
		}
	}
	if req.Chamados != nil {
		if err := models.SyncHostChamados(h.db.SQL, existing.ID, *req.Chamados); err != nil {
			log.Printf("[hosts] SyncHostChamados error on update: %v", err)
		}
	}

	// Link SSH key from DB if provided.
	if req.SSHKeyID > 0 {
		h.linkSSHKey(existing.ID, req.SSHKeyID, existing.OficialSlug)
	}

	// Sync DNS and service links if provided.
	if req.DNSIDs != nil {
		if err := models.SetHostDNSLinks(h.db.SQL, existing.ID, *req.DNSIDs); err != nil {
			log.Printf("[hosts] SetHostDNSLinks error on update: %v", err)
		}
	}
	if req.ServiceIDs != nil {
		if err := models.SetHostServiceLinks(h.db.SQL, existing.ID, *req.ServiceIDs); err != nil {
			log.Printf("[hosts] SetHostServiceLinks error on update: %v", err)
		}
	}
	if req.ProjectIDs != nil {
		if err := models.SetHostProjectLinks(h.db.SQL, existing.ID, *req.ProjectIDs); err != nil {
			log.Printf("[hosts] SetHostProjectLinks error on update: %v", err)
		}
	}

	jsonOK(w, req.Host)
}

func normalizePreferredAuth(hasPassword, hasKey bool, preferredAuth string) (string, error) {
	switch {
	case hasPassword && hasKey:
		if preferredAuth != "password" && preferredAuth != "key" {
			return "", fmt.Errorf("preferred_auth must be 'password' or 'key' when both auth methods are configured")
		}
		return preferredAuth, nil
	case hasPassword:
		return "password", nil
	case hasKey:
		return "key", nil
	default:
		return "", nil
	}
}

func (h *hostHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}

	models.DeleteTags(h.db.SQL, "host", host.ID)
	if err := models.DeleteHost(h.db.SQL, host.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete host")
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *hostHandlers) handleGetPassword(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	host, err := models.GetHostBySlug(h.db.SQL, slug)
	if err != nil || host == nil {
		jsonError(w, http.StatusNotFound, "host not found")
		return
	}
	if !host.HasPassword {
		jsonError(w, http.StatusNotFound, "no password stored")
		return
	}
	password, err := h.db.Encryptor.Decrypt(host.PasswordCiphertext, host.PasswordNonce)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}
	jsonOK(w, map[string]string{"password": password})
}

// resolveHostKeyPEM returns the decrypted private-key PEM for a host, read
// directly from the encrypted blob stored on the host row. The filesystem is
// never touched — this is what keeps key-auth working after a DB backup is
// restored onto a machine where host.key_path no longer exists.
func resolveHostKeyPEM(db *database.DB, host *models.Host) ([]byte, error) {
	if host == nil || len(host.PrivKeyCiphertext) == 0 {
		return nil, fmt.Errorf("host has no stored private key — link an SSH key via the host editor")
	}
	plain, err := db.Encryptor.Decrypt(host.PrivKeyCiphertext, host.PrivKeyNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt host key: %w", err)
	}
	return []byte(plain), nil
}

// linkSSHKey copies the encrypted key blob from the ssh_keys table onto the
// host row. It does NOT materialize the key to the filesystem — key-auth SSH
// decrypts the blob in-memory at connection time via resolveHostKeyPEM.
func (h *hostHandlers) linkSSHKey(hostID, sshKeyID int64, slug string) {
	k, err := models.GetSSHKey(h.db.SQL, sshKeyID)
	if err != nil || k == nil {
		return
	}
	if len(k.PrivKeyCiphertext) == 0 {
		return
	}
	models.UpdateHostKey(h.db.SQL, hostID, true, "", "yes",
		k.PubKeyCiphertext, k.PubKeyNonce, k.PrivKeyCiphertext, k.PrivKeyNonce)
}
