package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type dashboardHandlers struct {
	db *database.DB
}

func (h *dashboardHandlers) handleDashboard(w http.ResponseWriter, r *http.Request) {
	hostCount, _ := models.HostCount(h.db.SQL)
	hostBySituacao, _ := models.HostCountBySituacao(h.db.SQL)
	hostByHospedagem, _ := models.HostCountByHospedagem(h.db.SQL)
	hostsWithScans, _ := models.HostsWithScanCount(h.db.SQL)
	hostsMaintenance, _ := models.HostsNeedingMaintenanceCount(h.db.SQL)
	recentScans, _ := models.RecentScansWithHost(h.db.SQL, 5)
	dnsCount, _ := models.DNSRecordCount(h.db.SQL)
	projectCount, _ := models.ProjectCount(h.db.SQL)
	serviceCount, _ := models.ServiceCount(h.db.SQL)
	orchestratorCount, _ := models.OrchestratorCount(h.db.SQL)
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
