package api

import (
	"fmt"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type graphHandlers struct {
	db *database.DB
}

type graphNode struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Label  string         `json:"label"`
	Status string         `json:"status,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

type graphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
}

func (h *graphHandlers) handleGraph(w http.ResponseWriter, r *http.Request) {
	nodes := []graphNode{}
	edges := []graphEdge{}

	// Hosts
	hosts, _ := models.ListHosts(h.db.SQL, models.HostFilter{})
	hostIDMap := make(map[int64]string)
	for _, host := range hosts {
		nid := fmt.Sprintf("host-%d", host.ID)
		hostIDMap[host.ID] = nid
		nodes = append(nodes, graphNode{
			ID:     nid,
			Type:   "host",
			Label:  host.Nickname,
			Status: host.Situacao,
			Data: map[string]any{
				"hostname":   host.Hostname,
				"hospedagem": host.Hospedagem,
				"slug":       host.OficialSlug,
			},
		})
	}

	// DNS records
	dnsRecords, _ := models.ListDNSRecords(h.db.SQL)
	dnsIDMap := make(map[int64]string)
	for _, dns := range dnsRecords {
		nid := fmt.Sprintf("dns-%d", dns.ID)
		dnsIDMap[dns.ID] = nid
		nodes = append(nodes, graphNode{
			ID:     nid,
			Type:   "dns",
			Label:  dns.Domain,
			Status: dns.Situacao,
			Data: map[string]any{
				"has_https": dns.HasHTTPS,
			},
		})

		// DNS -> Host edges
		hostIDs, _ := models.GetDNSHostIDs(h.db.SQL, dns.ID)
		for _, hid := range hostIDs {
			if target, ok := hostIDMap[hid]; ok {
				edges = append(edges, graphEdge{Source: nid, Target: target, Label: "points to"})
			}
		}
	}

	// Projects
	projects, _ := models.ListProjects(h.db.SQL)
	projectIDMap := make(map[int64]string)
	for _, p := range projects {
		nid := fmt.Sprintf("project-%d", p.ID)
		projectIDMap[p.ID] = nid
		nodes = append(nodes, graphNode{
			ID:     nid,
			Type:   "project",
			Label:  p.Name,
			Status: p.Situacao,
		})
	}

	// Services
	services, _ := models.ListServices(h.db.SQL)
	serviceIDMap := make(map[int64]string)
	for _, svc := range services {
		nid := fmt.Sprintf("service-%d", svc.ID)
		serviceIDMap[svc.ID] = nid
		nodes = append(nodes, graphNode{
			ID:    nid,
			Type:  "service",
			Label: svc.Nickname,
			Data: map[string]any{
				"technology_stack":       svc.TechnologyStack,
				"developed_by":           svc.DevelopedBy,
				"is_external_dependency": svc.IsExternalDependency,
			},
		})

		// Service -> Project edge
		if svc.ProjectID != nil {
			if target, ok := projectIDMap[*svc.ProjectID]; ok {
				edges = append(edges, graphEdge{Source: nid, Target: target, Label: "part of"})
			}
		}

		// Service -> Host edges
		hostIDs, _ := models.GetServiceHostIDs(h.db.SQL, svc.ID)
		for _, hid := range hostIDs {
			if target, ok := hostIDMap[hid]; ok {
				edges = append(edges, graphEdge{Source: nid, Target: target, Label: "runs on"})
			}
		}

		// Service -> DNS edges
		dnsIDs, _ := models.GetServiceDNSIDs(h.db.SQL, svc.ID)
		for _, did := range dnsIDs {
			if target, ok := dnsIDMap[did]; ok {
				edges = append(edges, graphEdge{Source: nid, Target: target, Label: "served at"})
			}
		}
	}

	// Direct Host -> Project edges
	for _, p := range projects {
		hostIDs, _ := models.GetProjectHostIDs(h.db.SQL, p.ID)
		for _, hid := range hostIDs {
			if target, ok := hostIDMap[hid]; ok {
				edges = append(edges, graphEdge{Source: target, Target: projectIDMap[p.ID], Label: "part of"})
			}
		}
	}

	// Service dependency edges
	for _, svc := range services {
		depIDs, _ := models.GetServiceDependencyIDs(h.db.SQL, svc.ID)
		for _, depID := range depIDs {
			src := serviceIDMap[svc.ID]
			if target, ok := serviceIDMap[depID]; ok {
				edges = append(edges, graphEdge{Source: src, Target: target, Label: "depends on"})
			}
		}
	}

	jsonOK(w, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}
