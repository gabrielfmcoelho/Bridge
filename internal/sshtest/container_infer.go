package sshtest

import "strings"

// ContainerInference holds the inferred service classification from a container image.
type ContainerInference struct {
	ServiceType    string
	ServiceSubtype string
	Nickname       string
}

type imageRule struct {
	keyword     string
	serviceType string
	subtype     string
}

var imageRules = []imageRule{
	// Databases
	{"postgres", "database", "PostgreSQL"},
	{"mariadb", "database", "MySQL"},
	{"mysql", "database", "MySQL"},
	{"mongo", "database", "MongoDB"},
	{"redis", "database", "Redis"},
	{"elasticsearch", "database", "Elasticsearch"},
	{"opensearch", "database", "Elasticsearch"},
	{"sqlite", "database", "SQLite"},

	// Nginx
	{"nginx", "nginx", "Nginx"},

	// Infrastructure
	{"traefik", "infrastructure", "Traefik"},
	{"kong", "infrastructure", "Kong"},
	{"minio", "infrastructure", "MinIO"},
	{"keycloak", "infrastructure", "Keycloak"},
	{"portainer", "infrastructure", "Portainer"},
	{"coolify", "infrastructure", "Coolify"},
	{"apache", "infrastructure", "Apache"},
	{"haproxy", "infrastructure", ""},
	{"vault", "infrastructure", ""},
	{"consul", "infrastructure", ""},
	{"envoy", "infrastructure", ""},

	// Monitoring
	{"grafana", "monitoring", "Grafana"},
	{"metabase", "monitoring", "Metabase"},
	{"signoz", "monitoring", ""},
	{"prometheus", "monitoring", ""},
	{"loki", "monitoring", ""},
	{"jaeger", "monitoring", ""},

	// Workers / orchestration
	{"airflow", "workers", "Airflow"},
	{"prefect", "workers", "Prefect"},
	{"n8n", "workers", "n8n"},
	{"celery", "workers", ""},
	{"temporal", "workers", ""},
	{"dagster", "workers", ""},
	{"trino", "workers", "Trino"},

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

	for _, rule := range imageRules {
		if strings.Contains(lower, rule.keyword) {
			nickname := containerName
			if nickname == "" {
				nickname = base
			}
			return ContainerInference{
				ServiceType:    rule.serviceType,
				ServiceSubtype: rule.subtype,
				Nickname:       nickname,
			}
		}
	}

	nickname := containerName
	if nickname == "" {
		nickname = base
	}
	return ContainerInference{
		ServiceType:    "application",
		ServiceSubtype: "",
		Nickname:       nickname,
	}
}
