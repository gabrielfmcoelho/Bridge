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
