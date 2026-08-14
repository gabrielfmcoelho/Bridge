package models

import "time"

// Service is a pure data type (DTO). All SQL for services and its junctions
// (service_host_links, service_dns_links, service_dependencies) lives in
// internal/store.ServiceRepo; enrichment, lifecycle rules, and container
// reconciliation live in internal/service.ServiceService / store.ServiceRepo
// (R2 refactor).
type Service struct {
	ID                   int64  `json:"id"`
	Nickname             string `json:"nickname"`
	ProjectID            *int64 `json:"project_id"`
	Description          string `json:"description"`
	ServiceType          string `json:"service_type"`
	ServiceSubtype       string `json:"service_subtype"`
	TechnologyStack      string `json:"technology_stack"`
	DeployApproach       string `json:"deploy_approach"`
	OrchestratorTool     string `json:"orchestrator_tool"`
	Environment          string `json:"environment"`
	Port                 string `json:"port"`
	Version              string `json:"version"`
	OrchestratorManaged  bool   `json:"orchestrator_managed"`
	IsDirectlyManaged    bool   `json:"is_directly_managed"`
	IsResponsible        bool   `json:"is_responsible"`
	DevelopedBy          string `json:"developed_by"`
	IsExternalDependency bool   `json:"is_external_dependency"`
	ExternalProvider     string `json:"external_provider"`
	ExternalURL          string `json:"external_url"`
	ExternalContact      string `json:"external_contact"`
	RepositoryURL        string `json:"repository_url"`
	GitlabURL            string `json:"gitlab_url"`
	DocumentationURL     string `json:"documentation_url"`
	Source               string `json:"source"`
	// DiscoveryKind is "container" (from `docker ps`) or "host" (a catalog
	// hit backed by systemd/process/port evidence); empty for manual services.
	// DiscoveryKey is the per-host identity the scan reconciles on — the
	// container name or the catalog service name.
	DiscoveryKind string `json:"discovery_kind"`
	DiscoveryKey  string `json:"discovery_key"`
	// ContainerStatus is the online/offline lifecycle flag for both discovery
	// kinds (the name predates host-service discovery).
	ContainerStatus     string     `json:"container_status"`
	ContainerID         string     `json:"container_id"`
	ContainerName       string     `json:"container_name"`
	ContainerImage      string     `json:"container_image"`
	ContainerPorts      string     `json:"container_ports"`
	DiscoveredAt        *time.Time `json:"discovered_at"`
	LastSeenAt          *time.Time `json:"last_seen_at"`
	GrafanaDashboardUID string     `json:"grafana_dashboard_uid"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ServiceFilter is the value object describing list/count predicates, sort, and
// pagination for services. Consumed by store.ServiceRepo.ListFiltered /
// CountFiltered. IsExternalDependency and OrchestratorManaged are tri-state
// strings: "" (any), "yes" (true), or "no" (false).
type ServiceFilter struct {
	Search               string
	Tag                  string
	DevelopedBy          string
	IsExternalDependency string
	OrchestratorManaged  string
	SortBy               string
	SortDir              string
	Page                 int
	PerPage              int
}
