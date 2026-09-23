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

// Application is the subset of a Coolify application the DNS sync reads.
// FQDN is nullable and comma-separated ("https://a.x,http://b.x:8080/api").
type Application struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	FQDN        string `json:"fqdn"`
	Destination struct {
		Server ServerRef `json:"server"`
	} `json:"destination"`
}

// Service is the subset of a Coolify service (compose stack) the DNS sync
// reads: its server and each sub-application's fqdn.
type Service struct {
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Server       ServerRef `json:"server"`
	Applications []struct {
		Name string `json:"name"`
		FQDN string `json:"fqdn"`
	} `json:"applications"`
}

// ServerRef is the server embedded in an application/service. Kept to the
// fields the sync reads so an unexpected type elsewhere can't fail the decode.
type ServerRef struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}
