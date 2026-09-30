package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

type dashboardHandlers struct {
	db *database.DB
}

// handleDashboard godoc
//
//	@Summary		Dashboard counters
//	@Description	Any role. Host totals (by situação, by hospedagem, with scans, needing maintenance), the 5 most recent scans and counts of DNS records, projects, services, orchestrators and visible open issues.
//	@Tags			dashboard
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/dashboard [get]
func (h *dashboardHandlers) handleDashboard(w http.ResponseWriter, r *http.Request) {
	hostCount, _ := store.NewHostRepo(h.db.SQL).CountAll(r.Context())
	hostBySituacao, _ := store.NewHostRepo(h.db.SQL).CountBySituacao(r.Context())
	hostByHospedagem, _ := store.NewHostRepo(h.db.SQL).CountByHospedagem(r.Context())
	hostsWithScans, _ := store.NewHostScanRepo(h.db.SQL).CountHostsWithScans(r.Context())
	hostsMaintenance, _ := store.NewHostRepo(h.db.SQL).NeedingMaintenanceCount(r.Context())
	recentScans, _ := store.NewHostScanRepo(h.db.SQL).RecentWithHost(r.Context(), 5)
	dnsCount, _ := store.NewDNSRepo(h.db.SQL).Count(r.Context())
	projectCount, _ := store.NewProjectRepo(h.db.SQL).Count(r.Context())
	serviceCount, _ := store.NewServiceRepo(h.db.SQL).Count(r.Context())
	orchestratorCount, _ := store.NewOrchestratorRepo(h.db.SQL).Count(r.Context())
	visSQL, visArgs := store.VisibleExprDyn(r.Context(), "entity_type", "entity_id")
	openIssues, _ := models.OpenIssueCountVisible(h.db.SQL, visSQL, visArgs)

	jsonOK(w, map[string]any{
		"hosts": map[string]any{
			"total":         hostCount,
			"by_situacao":   hostBySituacao,
			"by_hospedagem": hostByHospedagem,
			"with_scans":    hostsWithScans,
			"maintenance":   hostsMaintenance,
		},
		"recent_scans":  recentScans,
		"dns_records":   dnsCount,
		"projects":      projectCount,
		"services":      serviceCount,
		"orchestrators": orchestratorCount,
		"open_issues":   openIssues,
	})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *dashboardHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/dashboard", h.handleDashboard)
}
