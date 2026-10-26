package models

import (
	"fmt"
	"strings"
	"time"
)

// Database is one logical database in the Atlas catalogue (sei_prod, a Trino
// catalog, an Oracle service name). It runs on an instance — a services row
// the scan or Coolify found (InstanceServiceID) — or is external, run by
// ExternalProvider. Persistence lives in internal/store.DatabaseRepo.
type Database struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`    // display name ("SEI produção")
	DBName            string `json:"db_name"` // real name in the engine
	Description       string `json:"description"`
	UseCases          string `json:"use_cases"` // Markdown: who uses it and for what
	Engine            string `json:"engine"`
	InstanceServiceID *int64 `json:"instance_service_id"` // nil = external
	ExternalProvider  string `json:"external_provider"`
	// Host/Port override the endpoint derived from the instance; "" / nil =
	// derive. Setting a free host is admin-only (it is where Bridge will
	// connect to).
	Host                string     `json:"host"`
	Port                *int       `json:"port"`
	ConnectionMode      string     `json:"connection_mode"` // none | direct | ssh_tunnel
	TunnelHostID        *int64     `json:"tunnel_host_id"`
	TunnelTarget        string     `json:"tunnel_target"` // container name or 127.0.0.1 on the tunnel host
	TLSMode             string     `json:"tls_mode"`
	HasCredential       bool       `json:"has_credential"` // the credential itself never leaves the backend
	Sensitivity         string     `json:"sensitivity"`    // LGPD: publico | interno | pessoal | sensivel
	Environment         string     `json:"environment"`    // prod | homolog | dev
	BackupPolicy        string     `json:"backup_policy"`
	BackupRetentionDays *int       `json:"backup_retention_days"`
	LastBackupAt        *time.Time `json:"last_backup_at"`
	CreatedBy           *int64     `json:"created_by"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`

	// Instance is a summary of the engine row, nil when external or when
	// the caller can't see it.
	Instance *DatabaseInstanceRef `json:"instance,omitempty"`
	// ConsumerServiceIDs / ProjectIDs: always set, [] when none.
	ConsumerServiceIDs []int64 `json:"consumer_service_ids"`
	ProjectIDs         []int64 `json:"project_ids"`
	// MainResponsavelName is the main internal responsável (list responses).
	MainResponsavelName string `json:"main_responsavel_name,omitempty"`
	// Responsaveis and Entidades are loaded on detail responses only.
	Responsaveis []Responsavel `json:"responsaveis,omitempty"`
	Entidades    *AssetGrants  `json:"entidades,omitempty"`
}

// DatabaseInstanceRef summarizes the services row a database runs on.
type DatabaseInstanceRef struct {
	ID              int64  `json:"id"`
	Nickname        string `json:"nickname"`
	Subtype         string `json:"service_subtype"`
	ContainerName   string `json:"container_name"`
	ContainerImage  string `json:"container_image"`
	ContainerStatus string `json:"container_status"`
	Port            string `json:"port"`
	HostID          *int64 `json:"host_id"`
	HostNickname    string `json:"host_nickname"`
	HostSlug        string `json:"host_slug"`
}

// DatabaseInstance is an engine found by the scan or Coolify (services of
// kind database/cache, or a Coolify database resource), with how many
// catalogued databases point at it.
type DatabaseInstance struct {
	DatabaseInstanceRef
	Engine              string `json:"engine"` // inferred; "other" when unknown
	Kind                string `json:"service_kind"`
	CoolifyResourceType string `json:"coolify_resource_type"`
	RegisteredCount     int    `json:"registered_count"`
}

// DatabaseFilter narrows DatabaseRepo.List. Empty fields are ignored.
type DatabaseFilter struct {
	Query             string
	Engine            string
	Environment       string
	Sensitivity       string
	InstanceServiceID int64
	ConsumerServiceID int64
	ProjectID         int64 // directly, or through the instance's / a consumer's project
}

// Database enums. Values match the CHECK constraints of migration v115.
var (
	DatabaseEngines         = []string{"postgres", "mysql", "mariadb", "oracle", "mssql", "trino", "mongodb", "redis", "other"}
	DatabaseConnectionModes = []string{"none", "direct", "ssh_tunnel"}
	DatabaseTLSModes        = []string{"disable", "prefer", "require", "verify"}
	DatabaseSensitivities   = []string{"publico", "interno", "pessoal", "sensivel"}
	DatabaseEnvironments    = []string{"prod", "homolog", "dev"}
)

func oneOf(v string, set []string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// ApplyDefaults fills the enum columns' defaults.
func (d *Database) ApplyDefaults() {
	if d.ConnectionMode == "" {
		d.ConnectionMode = "none"
	}
	if d.TLSMode == "" {
		d.TLSMode = "prefer"
	}
	if d.Sensitivity == "" {
		d.Sensitivity = "interno"
	}
	if d.Environment == "" {
		d.Environment = "prod"
	}
}

// Validate checks the invariants the DB CHECKs also guard, plus the ones
// they can't express (a tunnel needs a host; a database needs somewhere to
// live).
func (d *Database) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("name is required")
	}
	for _, c := range []struct {
		field, v string
		set      []string
	}{
		{"engine", d.Engine, DatabaseEngines},
		{"connection_mode", d.ConnectionMode, DatabaseConnectionModes},
		{"tls_mode", d.TLSMode, DatabaseTLSModes},
		{"sensitivity", d.Sensitivity, DatabaseSensitivities},
		{"environment", d.Environment, DatabaseEnvironments},
	} {
		if !oneOf(c.v, c.set) {
			return fmt.Errorf("invalid %s %q", c.field, c.v)
		}
	}
	if d.Port != nil && (*d.Port <= 0 || *d.Port > 65535) {
		return fmt.Errorf("invalid port %d", *d.Port)
	}
	if d.BackupRetentionDays != nil && *d.BackupRetentionDays < 0 {
		return fmt.Errorf("backup_retention_days must be >= 0")
	}
	if d.ConnectionMode == "ssh_tunnel" && d.TunnelHostID == nil {
		return fmt.Errorf("ssh_tunnel needs tunnel_host_id")
	}
	if d.InstanceServiceID == nil && strings.TrimSpace(d.ExternalProvider) == "" && strings.TrimSpace(d.Host) == "" {
		return fmt.Errorf("an external database needs external_provider or host")
	}
	return nil
}

// InferDatabaseEngine maps a services row's subtype / image to an engine
// ("other" when nothing matches). Order matters: mariadb before mysql.
func InferDatabaseEngine(subtype, image string) string {
	s := strings.ToLower(subtype + " " + image)
	for _, m := range []struct{ needle, engine string }{
		{"postgres", "postgres"}, {"postgis", "postgres"}, {"timescale", "postgres"},
		{"mariadb", "mariadb"}, {"mysql", "mysql"},
		{"oracle", "oracle"}, {"mssql", "mssql"}, {"sqlserver", "mssql"}, {"sql server", "mssql"},
		{"trino", "trino"}, {"mongo", "mongodb"},
		{"redis", "redis"}, {"valkey", "redis"}, {"dragonfly", "redis"}, {"keydb", "redis"},
	} {
		if strings.Contains(s, m.needle) {
			return m.engine
		}
	}
	return "other"
}
