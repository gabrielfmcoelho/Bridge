package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// ErrHostKeyLink wraps an SSH-key link failure so the handler can surface it as
// a partial-success (the host row was already created/updated).
var ErrHostKeyLink = errors.New("ssh key link failed")

// HostService owns host read enrichment: the list view's bulk-loaded
// composition (tags + scan summary + counts + alerts) and the single-host
// detail view. Write paths (create/update with vault dual-writes, SSH-key
// resealing, Grafana provisioning) remain in the handler pending a later phase.
type HostService struct {
	db           *database.DB // needed for vault dual-writes + the encryptor (ssh-key reseal)
	sqlDB        *sql.DB
	hosts        *store.HostRepo
	tags         *store.TagRepo
	scans        *store.HostScanRepo
	alerts       *store.HostAlertRepo
	alertCfg     *store.AlertSettingsRepo
	services     *store.ServiceRepo
	dns          *store.DNSRepo
	projects     *store.ProjectRepo
	grants       *store.AssetEntidadeRepo
	chamados     *store.HostChamadoRepo
	orch         *store.OrchestratorRepo
	sshKeys      *store.SSHKeyRepo
	responsaveis *store.ResponsavelRepo
}

// NewHostService constructs a HostService over the given DB handle.
func NewHostService(db *database.DB) *HostService {
	return &HostService{
		db:           db,
		sqlDB:        db.SQL,
		hosts:        store.NewHostRepo(db.SQL),
		tags:         store.NewTagRepo(db.SQL),
		scans:        store.NewHostScanRepo(db.SQL),
		alerts:       store.NewHostAlertRepo(db.SQL),
		alertCfg:     store.NewAlertSettingsRepo(db.SQL),
		services:     store.NewServiceRepo(db.SQL),
		dns:          store.NewDNSRepo(db.SQL),
		projects:     store.NewProjectRepo(db.SQL),
		grants:       store.NewAssetEntidadeRepo(db.SQL),
		chamados:     store.NewHostChamadoRepo(db.SQL),
		orch:         store.NewOrchestratorRepo(db.SQL),
		sshKeys:      store.NewSSHKeyRepo(db.SQL),
		responsaveis: store.NewResponsavelRepo(db.SQL),
	}
}

// ScanResources is the compact resource snapshot surfaced on the host list.
type ScanResources struct {
	CPU        string `json:"cpu,omitempty"`
	CPUUsage   string `json:"cpu_usage,omitempty"`
	RAM        string `json:"ram,omitempty"`
	RAMPercent string `json:"ram_percent,omitempty"`
	Storage    string `json:"storage,omitempty"`
	DiskPct    string `json:"disk_percent,omitempty"`
}

type scanFull struct {
	ScanResources
	Services   []string            `json:"services"`
	Containers []string            `json:"containers"`
	Profile    *hostProfileSummary `json:"profile,omitempty"`
}

// hostProfileSummary mirrors the relevant subset of sshtest.HostProfile — only
// the fields needed to surface the idle verdict (and the operator's "why"
// trail) on the host list.
type hostProfileSummary struct {
	Idle         bool     `json:"idle"`
	Reasons      []string `json:"reasons,omitempty"`
	Counterfacts []string `json:"counterfacts,omitempty"`
}

// AlertView is a computed-or-persisted host alert as rendered in the list.
type AlertView struct {
	ID            int64  `json:"id"`
	Type          string `json:"type"`
	Level         string `json:"level"`
	Message       string `json:"message"`
	Description   string `json:"description,omitempty"`
	Source        string `json:"source"`
	Status        string `json:"status"`
	LinkedIssueID *int64 `json:"linked_issue_id,omitempty"`
}

// HostListItem is an enriched host for the list view.
type HostListItem struct {
	models.Host
	Tags                []string       `json:"tags"`
	HasScan             bool           `json:"has_scan"`
	LastScanAt          *time.Time     `json:"last_scan_at,omitempty"`
	ScanRes             *ScanResources `json:"scan_resources,omitempty"`
	ContainersCount     int            `json:"containers_count"`
	ProcessesCount      int            `json:"processes_count"`
	ServicesCount       int            `json:"services_count"`
	DNSCount            int            `json:"dns_count"`
	IssuesCount         int            `json:"issues_count"`
	CanCompile          bool           `json:"can_compile"`
	Idle                bool           `json:"idle,omitempty"`
	IdleReasons         []string       `json:"idle_reasons,omitempty"`
	IdleCounterfacts    []string       `json:"idle_counterfacts,omitempty"`
	Alerts              []AlertView    `json:"alerts"`
	MainResponsavelName string         `json:"main_responsavel_name"`
	MainEntidade        string         `json:"main_entidade"`
	ChamadosCount       int            `json:"chamados_count"`
	ProjectsCount       int            `json:"projects_count"`
}

// HostDetail is the full single-host view (host + relations).
type HostDetail struct {
	Host         *models.Host         `json:"host"`
	Tags         []string             `json:"tags"`
	Orchestrator *models.Orchestrator `json:"orchestrator"`
	DNSRecords   []models.DNSRecord   `json:"dns_records"`
	Services     []models.Service     `json:"services"`
	Projects     []models.Project     `json:"projects"`
	LastScan     *models.HostScan     `json:"last_scan"`
	Responsaveis []models.Responsavel `json:"responsaveis"`
	Chamados     []models.HostChamado `json:"chamados"`
	Entidades    models.AssetGrants   `json:"entidades"`
}

// Count returns the number of hosts matching the filter (for pagination).
func (s *HostService) Count(ctx context.Context, f models.HostFilter) (int, error) {
	return s.hosts.Count(ctx, f)
}

// Get returns the enriched single-host detail by slug, or (nil, nil) if absent.
func (s *HostService) Get(ctx context.Context, slug string) (*HostDetail, error) {
	host, err := s.hosts.GetBySlug(ctx, slug)
	if err != nil || host == nil {
		return nil, err
	}
	tags, _ := s.tags.Get(ctx, "host", host.ID)
	orch, _ := s.orch.GetByHost(ctx, host.ID)
	dns, _ := s.dns.RecordsByHost(ctx, host.ID)
	services, _ := s.services.ListByHost(ctx, host.ID)
	projects, _ := s.projects.ProjectsByHost(ctx, host.ID)
	lastScan, _ := s.scans.GetLatest(ctx, host.ID)
	responsaveis, _ := s.responsaveis.List(ctx, "host", host.ID)
	chamados, _ := s.chamados.ListByHost(ctx, host.ID)
	entidades, _ := s.grants.Get(ctx, store.AssetHost, host.ID)

	return &HostDetail{
		Host:         host,
		Tags:         tags,
		Orchestrator: orch,
		DNSRecords:   dns,
		Services:     services,
		Projects:     projects,
		LastScan:     lastScan,
		Responsaveis: responsaveis,
		Chamados:     chamados,
		Entidades:    entidades,
	}, nil
}

// List returns hosts matching the filter, each enriched with tags, scan
// summary, relation counts, responsável/entidade names, and computed+persisted
// alerts. Bulk enrichment is best-effort (a failed auxiliary load degrades to
// empty rather than failing the whole list, matching the prior handler).
// Post-enrichment filters (alert_level, idle) and the pagination envelope are
// applied by the caller.
func (s *HostService) List(ctx context.Context, f models.HostFilter) ([]HostListItem, error) {
	hosts, err := s.hosts.List(ctx, f)
	if err != nil {
		return nil, err
	}

	tagMap, _ := s.tags.GetAll(ctx, "host")
	scanStatuses, _ := s.scans.Statuses(ctx)
	scanDataBulk, _ := s.scans.LatestDataBulk(ctx)

	thresholds, thErr := s.alertCfg.GetThresholds(ctx)
	if thErr != nil || thresholds == nil {
		thresholds = &models.AlertThresholds{ResourceCritical: 80, ResourceWarning: 60, ResourceInfoLow: 5}
	}

	projCounts, _ := s.services.ProjectCountsByHost(ctx)
	svcCounts, _ := s.services.CountsByHost(ctx)
	dnsCounts, _ := s.dns.CountsByHost(ctx)
	issueCounts, _ := models.GetIssueCountsByEntity(s.sqlDB, "host")
	mainRespNames, _ := s.responsaveis.MainNamesBulk(ctx, "host")
	mainEntidades, _ := s.grants.CreatorNamesBulk(ctx, store.AssetHost)
	chamadosCounts, _ := s.chamados.CountsBulk(ctx)
	manualAlertsBulk, _ := s.alerts.ListBulk(ctx)
	alertLinkedIssues, _ := s.alerts.LinkedIssueIDsBulk(ctx)

	result := make([]HostListItem, len(hosts))
	for i, host := range hosts {
		hwt := HostListItem{Host: host, Tags: tagMap[host.ID]}
		hwt.ServicesCount = svcCounts[host.ID]
		hwt.DNSCount = dnsCounts[host.ID]
		hwt.IssuesCount = issueCounts[host.ID]
		hwt.MainResponsavelName = mainRespNames[host.ID]
		hwt.MainEntidade = mainEntidades[host.ID]
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
			if err := json.Unmarshal([]byte(data), &sf); err == nil {
				if sf.CPU != "" || sf.RAM != "" || sf.Storage != "" {
					sr := sf.ScanResources
					hwt.ScanRes = &sr
				}
				hwt.ContainersCount = len(sf.Containers)
				hwt.ProcessesCount = len(sf.Services)
				if sf.Profile != nil {
					hwt.Idle = sf.Profile.Idle
					hwt.IdleReasons = sf.Profile.Reasons
					hwt.IdleCounterfacts = sf.Profile.Counterfacts
				}
				sfPtr = &sf
			}
		}
		hwt.Alerts = computeAlerts(hwt.ScanRes, sfPtr, host, thresholds)

		// Merge persisted alerts; an auto-sourced DB alert that duplicates a
		// computed one of the same type replaces it (so the DB id + issue link
		// surface) instead of appending a second copy.
		autoTypes := make(map[string]bool, len(hwt.Alerts))
		for _, a := range hwt.Alerts {
			autoTypes[a.Type] = true
		}
		if manualAlerts, ok := manualAlertsBulk[host.ID]; ok {
			for _, ma := range manualAlerts {
				ha := AlertView{
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
	return result, nil
}

// computeAlerts derives resource/auth alerts from a host's scan summary and
// credential test status, bucketed against the configured thresholds.
func computeAlerts(_ *ScanResources, sf *scanFull, host models.Host, t *models.AlertThresholds) []AlertView {
	var alerts []AlertView

	parseUsagePct := func(s string) (int, bool) {
		s = strings.TrimSuffix(strings.TrimSpace(s), "%")
		v, err := strconv.Atoi(s)
		return v, err == nil
	}

	if sf != nil {
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
				alerts = append(alerts, AlertView{Type: r.typ, Level: level, Message: msg, Source: "auto", Status: "active"})
			}
		}

		allFields := sf.CPU + sf.CPUUsage + sf.RAM + sf.RAMPercent + sf.Storage + sf.DiskPct
		lower := strings.ToLower(allFields)
		if strings.Contains(lower, "/dev/null") || strings.Contains(lower, "permission denied") {
			alerts = append(alerts, AlertView{
				Type:    "dev_null",
				Level:   "warning",
				Message: "Scan returned /dev/null permission warning",
				Source:  "auto",
			})
		}
	}

	// Auth-failed alert — only when all *tested* credentials failed.
	pwdTestedFailed := host.HasPassword && host.PasswordTestStatus != nil && *host.PasswordTestStatus == "failed"
	keyTestedFailed := host.HasKey && host.KeyTestStatus != nil && *host.KeyTestStatus == "failed"
	pwdTestedOK := host.HasPassword && host.PasswordTestStatus != nil && *host.PasswordTestStatus == "success"
	keyTestedOK := host.HasKey && host.KeyTestStatus != nil && *host.KeyTestStatus == "success"
	if (pwdTestedFailed || keyTestedFailed) && !pwdTestedOK && !keyTestedOK {
		alerts = append(alerts, AlertView{
			Type:    "auth_failed",
			Level:   "warning",
			Message: "No authentication method is working",
			Source:  "auto",
		})
	}

	return alerts
}

// HostWrite carries a host create/update payload: the host row plus secrets
// (password / linked SSH key) and relation sets. Relation pointers distinguish
// "absent" from "present" — Create applies only non-empty sets; Update applies
// any non-nil set (nil = leave unchanged).
type HostWrite struct {
	Host         models.Host
	Tags         *[]string
	Password     string
	SSHKeyID     int64
	ClearKey     bool
	Responsaveis *[]models.ResponsavelInput
	Chamados     *[]models.HostChamadoInput
	Grants       *models.AssetGrants // resolved entidade grants; nil = leave unchanged
	DNSIDs       *[]int64
	ServiceIDs   *[]int64
	ProjectIDs   *[]int64
}

func present[T any](p *[]T, requireNonEmpty bool) bool {
	if p == nil {
		return false
	}
	if requireNonEmpty {
		return len(*p) > 0
	}
	return true
}

// Create persists a new host (already flag-normalized by the caller): the row,
// the vault password dual-write (best-effort), relation sets, and the linked
// SSH key. A key-link failure is returned wrapped in ErrHostKeyLink with the
// host still created. w.Host.ID is set on success.
func (s *HostService) Create(ctx context.Context, actorID int64, w *HostWrite) (*models.Host, error) {
	if err := s.hosts.Create(ctx, &w.Host); err != nil {
		return nil, err
	}
	if w.Password != "" {
		if err := vault.HostSetPassword(ctx, s.db, w.Host.ID, actorID, w.Password); err != nil {
			log.Printf("[hosts] vault password dual-write on create slug=%s: %v", w.Host.OficialSlug, err)
		}
	}
	s.applyMetaRelations(ctx, w.Host.ID, w, true)
	if w.SSHKeyID > 0 {
		if err := s.LinkSSHKey(ctx, w.Host.ID, w.SSHKeyID, w.Host.OficialSlug); err != nil {
			return &w.Host, fmt.Errorf("%w: %w", ErrHostKeyLink, err)
		}
	}
	s.applyLinkRelations(ctx, w.Host.ID, w, true)
	return &w.Host, nil
}

// Update persists host changes (already flag-normalized by the caller): the row,
// vault password rotation / key clearing (best-effort), relation sets, and an
// optional SSH-key relink. A relink failure is returned wrapped in
// ErrHostKeyLink with the host already updated.
func (s *HostService) Update(ctx context.Context, actorID int64, w *HostWrite) (*models.Host, error) {
	if err := s.hosts.Update(ctx, &w.Host); err != nil {
		return nil, err
	}
	if w.Password != "" {
		if err := vault.HostSetPassword(ctx, s.db, w.Host.ID, actorID, w.Password); err != nil {
			log.Printf("[hosts] vault password dual-write on update slug=%s: %v", w.Host.OficialSlug, err)
		}
	}
	if w.ClearKey {
		if err := vault.HostSetSSHKey(ctx, s.db, w.Host.ID, actorID, vault.HostSSHKey{}); err != nil {
			log.Printf("[hosts] vault clear-key on update slug=%s: %v", w.Host.OficialSlug, err)
		}
	}
	s.applyMetaRelations(ctx, w.Host.ID, w, false)
	if w.SSHKeyID > 0 {
		if err := s.LinkSSHKey(ctx, w.Host.ID, w.SSHKeyID, w.Host.OficialSlug); err != nil {
			return &w.Host, fmt.Errorf("%w: %w", ErrHostKeyLink, err)
		}
	}
	s.applyLinkRelations(ctx, w.Host.ID, w, false)
	return &w.Host, nil
}

// Delete removes the host's tags then cascade-deletes the host (and any vault
// secrets scoped to it) via the store cascade registry, atomically.
func (s *HostService) Delete(ctx context.Context, actor models.ActorContext, hostID int64) error {
	if err := s.tags.Delete(ctx, "host", hostID); err != nil {
		return err
	}
	return store.DeleteParent(ctx, s.sqlDB, actor, models.SecretScopeHost, hostID)
}

// LinkSSHKey copies the encrypted key blob from the ssh_keys table into the
// host's vault entry and flips the host's has_key flag. The key is never
// materialized to the filesystem — key-auth decrypts the vault blob in-memory at
// connection time. actor_user_id=0 falls through to the vault's system actor
// (linkSSHKey runs from create/update/coolify-sync where the caller's user
// isn't threaded down).
func (s *HostService) LinkSSHKey(ctx context.Context, hostID, sshKeyID int64, slug string) error {
	k, err := s.sshKeys.Get(ctx, sshKeyID)
	if err != nil {
		return fmt.Errorf("load ssh key %d: %w", sshKeyID, err)
	}
	if k == nil {
		return fmt.Errorf("ssh key %d not found", sshKeyID)
	}
	if len(k.PrivKeyCiphertext) == 0 {
		return fmt.Errorf("ssh key %q has no stored private key — add a private key to the entry or pick a different key", k.Name)
	}
	if err := s.hosts.UpdateKeyMeta(ctx, hostID, true, "", "yes"); err != nil {
		return fmt.Errorf("update host key flags: %w", err)
	}
	priv, perr := s.db.Encryptor.Decrypt(k.PrivKeyCiphertext, k.PrivKeyNonce)
	if perr != nil {
		return fmt.Errorf("decrypt ssh key %d private payload: %w", sshKeyID, perr)
	}
	pub := ""
	if len(k.PubKeyCiphertext) > 0 {
		if p, derr := s.db.Encryptor.Decrypt(k.PubKeyCiphertext, k.PubKeyNonce); derr == nil {
			pub = p
		}
	}
	if vErr := vault.HostSetSSHKey(ctx, s.db, hostID, 0, vault.HostSSHKey{PrivateKeyPEM: priv, PublicKey: pub}); vErr != nil {
		return fmt.Errorf("store host ssh key in vault: %w", vErr)
	}
	return nil
}

// applyMetaRelations syncs tags + responsáveis + chamados + entidade grants.
// Best-effort: an individual sync failure is logged, not fatal (matching prior
// handler behavior). requireNonEmpty selects create (len>0) vs update (non-nil)
// semantics.
func (s *HostService) applyMetaRelations(ctx context.Context, hostID int64, w *HostWrite, requireNonEmpty bool) {
	if present(w.Tags, requireNonEmpty) {
		if err := s.tags.Set(ctx, "host", hostID, *w.Tags); err != nil {
			log.Printf("[hosts] set tags id=%d: %v", hostID, err)
		}
	}
	if present(w.Responsaveis, requireNonEmpty) {
		if err := s.responsaveis.Sync(ctx, "host", hostID, *w.Responsaveis); err != nil {
			log.Printf("[hosts] SyncHostResponsaveis id=%d: %v", hostID, err)
		}
	}
	if present(w.Chamados, requireNonEmpty) {
		if err := s.chamados.Sync(ctx, hostID, *w.Chamados); err != nil {
			log.Printf("[hosts] SyncHostChamados id=%d: %v", hostID, err)
		}
	}
	if w.Grants != nil {
		if err := s.grants.Replace(ctx, s.sqlDB, store.AssetHost, hostID, *w.Grants); err != nil {
			log.Printf("[hosts] set entidades id=%d: %v", hostID, err)
		}
	}
}

// applyLinkRelations syncs the host's dns / service / project link sets.
func (s *HostService) applyLinkRelations(ctx context.Context, hostID int64, w *HostWrite, requireNonEmpty bool) {
	if present(w.DNSIDs, requireNonEmpty) {
		if err := s.dns.SetLinksForHost(ctx, hostID, *w.DNSIDs); err != nil {
			log.Printf("[hosts] SetHostDNSLinks id=%d: %v", hostID, err)
		}
	}
	if present(w.ServiceIDs, requireNonEmpty) {
		if err := s.services.SetServicesForHost(ctx, hostID, *w.ServiceIDs); err != nil {
			log.Printf("[hosts] SetServicesForHost id=%d: %v", hostID, err)
		}
	}
	if present(w.ProjectIDs, requireNonEmpty) {
		if err := s.projects.SetProjectsForHost(ctx, hostID, *w.ProjectIDs); err != nil {
			log.Printf("[hosts] SetProjectsForHost id=%d: %v", hostID, err)
		}
	}
}

// ListTrash returns soft-deleted hosts (the trash view).
func (s *HostService) ListTrash(ctx context.Context) ([]models.Host, error) {
	return s.hosts.ListTrash(ctx)
}

// Restore un-deletes a host (and cascade-restores its secrets) by id.
func (s *HostService) Restore(ctx context.Context, actor models.ActorContext, id int64) error {
	return store.RestoreParent(ctx, s.sqlDB, actor, models.SecretScopeHost, id)
}
