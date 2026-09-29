package api

import (
	"fmt"
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
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
	hosts, _ := store.NewHostRepo(h.db.SQL).List(r.Context(), models.HostFilter{})
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
	dnsRecords, _ := store.NewDNSRepo(h.db.SQL).List(r.Context())
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

	}

	// Projects
	projects, _ := store.NewProjectRepo(h.db.SQL).List(r.Context())
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
	services, _ := store.NewServiceRepo(h.db.SQL).List(r.Context())
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

	}

	// Link-table edges: one query per table. An edge is drawn only when both
	// ends are in the scoped node maps above, so invisible assets never leak.
	links, err := store.NewGraphRepo(h.db.SQL).Links(r.Context())
	if err != nil {
		jsonErrorLogged(w, r, http.StatusInternalServerError, "failed to load graph", err)
		return
	}
	addEdges := func(pairs []store.LinkPair, from, to map[int64]string, label string) {
		for _, p := range pairs {
			src, ok1 := from[p.From]
			dst, ok2 := to[p.To]
			if ok1 && ok2 {
				edges = append(edges, graphEdge{Source: src, Target: dst, Label: label})
			}
		}
	}
	addEdges(links.DNSHost, dnsIDMap, hostIDMap, "points to")
	addEdges(links.ServiceHost, serviceIDMap, hostIDMap, "runs on")
	addEdges(links.ServiceDNS, serviceIDMap, dnsIDMap, "served at")
	// Stored project -> host; drawn host -> project, as before.
	for _, p := range links.ProjectHost {
		src, ok1 := hostIDMap[p.To]
		dst, ok2 := projectIDMap[p.From]
		if ok1 && ok2 {
			edges = append(edges, graphEdge{Source: src, Target: dst, Label: "part of"})
		}
	}
	addEdges(links.ServiceDepends, serviceIDMap, serviceIDMap, "depends on")

	jsonOK(w, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *graphHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/graph", h.handleGraph)
}
