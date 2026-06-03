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

func (h *dashboardHandlers) handleDashboard(w http.ResponseWriter, r *http.Request) {
	hostCount, _ := models.HostCount(h.db.SQL)
	hostBySituacao, _ := models.HostCountBySituacao(h.db.SQL)
	hostByHospedagem, _ := models.HostCountByHospedagem(h.db.SQL)
	hostsWithScans, _ := store.NewHostScanRepo(h.db.SQL).CountHostsWithScans(r.Context())
	hostsMaintenance, _ := models.HostsNeedingMaintenanceCount(h.db.SQL)
	recentScans, _ := store.NewHostScanRepo(h.db.SQL).RecentWithHost(r.Context(), 5)
	dnsCount, _ := store.NewDNSRepo(h.db.SQL).Count(r.Context())
	projectCount, _ := models.ProjectCount(h.db.SQL)
	serviceCount, _ := models.ServiceCount(h.db.SQL)
	orchestratorCount, _ := store.NewOrchestratorRepo(h.db.SQL).Count(r.Context())
	openIssues, _ := models.OpenIssueCount(h.db.SQL)

	jsonOK(w, map[string]any{
		"hosts": map[string]any{
			"total":          hostCount,
			"by_situacao":    hostBySituacao,
			"by_hospedagem":  hostByHospedagem,
			"with_scans":     hostsWithScans,
			"maintenance":    hostsMaintenance,
		},
		"recent_scans":   recentScans,
		"dns_records":    dnsCount,
		"projects":       projectCount,
		"services":       serviceCount,
		"orchestrators":  orchestratorCount,
		"open_issues":    openIssues,
	})
}
