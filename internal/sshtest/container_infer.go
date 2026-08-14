package sshtest

import "strings"

// ContainerInference holds the inferred service classification from a container
// image or a catalog service name.
type ContainerInference struct {
	ServiceType    string
	ServiceSubtype string
	Nickname       string
}

// kindToServiceType maps a serviceCatalog Kind onto the `service_type` enum the
// services page offers (see enum_options seeded in migrations). A spec can
// override the mapping with serviceSpec.ServiceType when its Kind is too coarse
// (nginx has its own type; GitLab/Jenkins are infra, not monitoring).
var kindToServiceType = map[string]string{
	"web":          "infrastructure",
	"proxy":        "infrastructure",
	"database":     "database",
	"cache":        "database",
	"queue":        "worker",
	"runtime":      "application",
	"dns":          "infrastructure",
	"mail":         "infrastructure",
	"file-sharing": "infrastructure",
	"directory":    "infrastructure",
	"analytics":    "monitoring",
	"logging":      "monitoring",
}

type imageRule struct {
	keyword     string
	serviceType string
	subtype     string
}

// imageRules covers container images with no serviceCatalog entry — app
// frameworks, SaaS agents, and orchestration tools that aren't host services
// anyone installs from a package. Anything the catalog already knows (nginx,
// postgres, redis, traefik, minio, keycloak, …) is deliberately absent: the
// catalog is the single source for those, so the scan panel and the services
// page can't disagree about what a container is.
var imageRules = []imageRule{
	// Databases with no host-service catalog entry
	{"sqlite", "database", "SQLite"},

	// Infrastructure
	{"kong", "infrastructure", "Kong"},
	{"portainer", "infrastructure", "Portainer"},
	{"coolify", "infrastructure", "Coolify"},
	{"vault", "infrastructure", ""},
	{"consul", "infrastructure", ""},

	// Monitoring
	{"grafana", "monitoring", "Grafana"},
	{"metabase", "monitoring", "Metabase"},
	{"signoz", "monitoring", ""},
	{"prometheus", "monitoring", ""},
	{"loki", "monitoring", ""},
	{"jaeger", "monitoring", ""},

	// Workers / orchestration
	{"airflow", "worker", "Airflow"},
	{"prefect", "worker", "Prefect"},
	{"n8n", "worker", "n8n"},
	{"celery", "worker", ""},
	{"temporal", "worker", ""},
	{"dagster", "worker", ""},
	{"trino", "worker", "Trino"},

	// Agents
	{"watchtower", "agents", ""},
	{"datadog", "agents", ""},
	{"newrelic", "agents", ""},
	{"telegraf", "agents", ""},
	{"fluentd", "agents", ""},
	{"filebeat", "agents", ""},

	// Fullstack
	{"next", "app-fullstack", ""},
	{"nuxt", "app-fullstack", ""},
	{"remix", "app-fullstack", ""},

	// API
	{"fastapi", "app-api", ""},
	{"flask", "app-api", ""},
	{"django", "app-api", ""},
	{"express", "app-api", ""},
	{"spring", "app-api", ""},
	{"gin", "app-api", ""},

	// Frontend
	{"react", "app-frontend", ""},
	{"angular", "app-frontend", ""},
	{"vue", "app-frontend", ""},
	{"svelte", "app-frontend", ""},
}

// InferFromImage maps a Docker image name to a service type/subtype.
// containerName is used as the fallback nickname.
//
// The serviceCatalog is consulted first (matching ImageHints against the full
// image reference exactly like detectService does, so registry-qualified hints
// such as "minio/minio" still match), then imageRules for images the catalog
// doesn't model. Unknown images land on "application".
func InferFromImage(imageName, containerName string) ContainerInference {
	// Normalize: strip registry prefix and tag
	// e.g. "ghcr.io/org/postgres:15-alpine" -> "postgres"
	base := imageName
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.Index(base, ":"); idx >= 0 {
		base = base[:idx]
	}
	lower := strings.ToLower(base)

	nickname := containerName
	if nickname == "" {
		nickname = base
	}

	full := strings.ToLower(imageName)
	for _, spec := range serviceCatalog {
		for _, hint := range spec.ImageHints {
			if hint != "" && strings.Contains(full, strings.ToLower(hint)) {
				return ContainerInference{
					ServiceType:    ServiceTypeForCatalog(spec.Name),
					ServiceSubtype: spec.Label,
					Nickname:       nickname,
				}
			}
		}
	}

	for _, rule := range imageRules {
		if strings.Contains(lower, rule.keyword) {
			return ContainerInference{
				ServiceType:    rule.serviceType,
				ServiceSubtype: rule.subtype,
				Nickname:       nickname,
			}
		}
	}

	return ContainerInference{
		ServiceType:    "application",
		ServiceSubtype: "",
		Nickname:       nickname,
	}
}

// ServiceTypeForCatalog returns the `service_type` enum value for a
// serviceCatalog entry key (DiscoveredService.Name). Unknown keys fall back to
// "application", matching InferFromImage's default.
func ServiceTypeForCatalog(name string) string {
	for _, spec := range serviceCatalog {
		if spec.Name != name {
			continue
		}
		if spec.ServiceType != "" {
			return spec.ServiceType
		}
		if t, ok := kindToServiceType[spec.Kind]; ok {
			return t
		}
		break
	}
	return "application"
}
