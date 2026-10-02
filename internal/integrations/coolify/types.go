package coolify

// Server represents a Coolify server resource.
type Server struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IP          string `json:"ip"`
	User        string `json:"user"`
	Port        int    `json:"port"`
	IsReachable bool   `json:"is_reachable"`
	IsUsable    bool   `json:"is_usable"`
}

// PrivateKey represents a Coolify private key resource.
type PrivateKey struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// CreateServerRequest is the body for POST /servers.
type CreateServerRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	IP              string `json:"ip"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	PrivateKeyUUID  string `json:"private_key_uuid"`
	InstantValidate bool   `json:"instant_validate,omitempty"`
}

// UpdateServerRequest is the body for PATCH /servers/{uuid}.
type UpdateServerRequest struct {
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	IP             string `json:"ip,omitempty"`
	Port           int    `json:"port,omitempty"`
	User           string `json:"user,omitempty"`
	PrivateKeyUUID string `json:"private_key_uuid,omitempty"`
}

// CreateKeyRequest is the body for POST /security/keys.
type CreateKeyRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PrivateKey  string `json:"private_key"`
}

// Application is the subset of a Coolify application the syncs read.
// FQDN is nullable and comma-separated ("https://a.x,http://b.x:8080/api").
// GitRepository can embed credentials (https://user:token@host/…): never log
// or store it without SanitizeRepoURL.
type Application struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	FQDN          string `json:"fqdn"`
	EnvironmentID int64  `json:"environment_id"`
	GitRepository string `json:"git_repository"`
	GitBranch     string `json:"git_branch"`
	Destination   struct {
		Server ServerRef `json:"server"`
	} `json:"destination"`
}

// Service is the subset of a Coolify service (compose stack) the syncs read:
// its server and each member's name/fqdn. Members run as `<name>-<uuid>`
// containers, uuid being the service's.
type Service struct {
	UUID          string    `json:"uuid"`
	Name          string    `json:"name"`
	EnvironmentID int64     `json:"environment_id"`
	Server        ServerRef `json:"server"`
	Applications  []struct {
		Name string `json:"name"`
		FQDN string `json:"fqdn"`
	} `json:"applications"`
	Databases []struct {
		Name string `json:"name"`
	} `json:"databases"`
}

// Database is a standalone Coolify database (one `<uuid>` container).
type Database struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	EnvironmentID int64  `json:"environment_id"`
	Destination   struct {
		Server ServerRef `json:"server"`
	} `json:"destination"`
}

// Project is a Coolify project with its environments (GET /projects/{uuid}).
type Project struct {
	UUID         string        `json:"uuid"`
	Name         string        `json:"name"`
	Environments []Environment `json:"environments"`
}

// Environment is one environment of a Coolify project.
type Environment struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ServerRef is the server embedded in an application/service. Kept to the
// fields the sync reads so an unexpected type elsewhere can't fail the decode.
type ServerRef struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}
