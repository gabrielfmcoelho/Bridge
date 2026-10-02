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
	// Derived edges are not stored: host/DNS → project reached through one of
	// the project's services (the same rule the project page uses).
	Derived bool `json:"derived,omitempty"`
}

// handleGraph godoc
//
//	@Summary		Inventory graph
//	@Description	Any role. {"nodes": [graphNode], "edges": [graphEdge]} over visible hosts, DNS records, projects, services and APIs; edges only between visible nodes.
//	@Tags			graph
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/graph [get]
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
				"container_name":         svc.ContainerName,
				"source":                 svc.Source,
				"coolify_project":        svc.CoolifyProject,
				"coolify_environment":    svc.CoolifyEnvironment,
				"coolify_stack":          svc.CoolifyStack,
				"project_id":             svc.ProjectID,
			},
		})

		// Service -> Project edge
		if svc.ProjectID != nil {
			if target, ok := projectIDMap[*svc.ProjectID]; ok {
				edges = append(edges, graphEdge{Source: nid, Target: target, Label: "part of"})
			}
		}

	}

	// APIs (the Atlas catalog). Listed through VisibleExpr like the rest, so
	// only visible APIs become nodes; List already drops trashed ones.
	apis, _ := store.NewAPICatalogRepo(h.db.SQL).List(r.Context(), models.APICatalogFilter{})
	apiIDMap := make(map[int64]string)
	for _, a := range apis {
		nid := fmt.Sprintf("api-%d", a.ID)
		apiIDMap[a.ID] = nid
		nodes = append(nodes, graphNode{
			ID:    nid,
			Type:  "api",
			Label: a.Name,
			Data: map[string]any{
				"operation_count": a.OperationCount,
				"base_url":        a.BaseURL,
			},
		})
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
	// Stored project -> dns; drawn dns -> project, like hosts.
	for _, p := range links.ProjectDNS {
		src, ok1 := dnsIDMap[p.To]
		dst, ok2 := projectIDMap[p.From]
		if ok1 && ok2 {
			edges = append(edges, graphEdge{Source: src, Target: dst, Label: "part of"})
		}
	}
	addEdges(links.ServiceDepends, serviceIDMap, serviceIDMap, "depends on")
	addEdges(links.APIService, apiIDMap, serviceIDMap, "served by")
	addEdges(links.APIProject, apiIDMap, projectIDMap, "part of")
	edges = append(edges, derivedProjectEdges(services, links, hostIDMap, dnsIDMap, projectIDMap)...)

	jsonOK(w, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

// derivedProjectEdges draws host → project and dns → project for the hosts
// and DNS records of a project's services that have no direct link to it —
// what the project page already lists as the project's hosts/DNS.
func derivedProjectEdges(services []models.Service, links store.GraphLinks, hostIDs, dnsIDs, projectIDs map[int64]string) []graphEdge {
	direct := map[[2]string]bool{}
	for _, p := range links.ProjectHost {
		direct[[2]string{hostIDs[p.To], projectIDs[p.From]}] = true
	}
	for _, p := range links.ProjectDNS {
		direct[[2]string{dnsIDs[p.To], projectIDs[p.From]}] = true
	}
	svcHosts, svcDNS := store.ByFrom(links.ServiceHost), store.ByFrom(links.ServiceDNS)
	var out []graphEdge
	add := func(src string, ok bool, dst string) {
		if !ok || direct[[2]string{src, dst}] {
			return
		}
		direct[[2]string{src, dst}] = true
		out = append(out, graphEdge{Source: src, Target: dst, Label: "via service", Derived: true})
	}
	for _, svc := range services {
		if svc.ProjectID == nil {
			continue
		}
		dst, ok := projectIDs[*svc.ProjectID]
		if !ok {
			continue
		}
		for _, h := range svcHosts[svc.ID] {
			src, okh := hostIDs[h]
			add(src, okh, dst)
		}
		for _, d := range svcDNS[svc.ID] {
			src, okd := dnsIDs[d]
			add(src, okd, dst)
		}
	}
	return out
}

// registerRoutes wires this group's routes (self-registration, R2).
func (h *graphHandlers) registerRoutes(rr routeRegistrar) {
	rr.auth("GET /api/graph", h.handleGraph)
}
