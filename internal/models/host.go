package models

import (
	"fmt"
	"time"
)

// Host represents a managed SSH target. The encrypted password / SSH-key blobs
// don't live on this struct — they're stored in the unified `secrets` table and
// accessed via vault.HostGetPassword / vault.HostGetSSHKey. The boolean flags
// (HasPassword, HasKey) remain as cached metadata so list/filter queries can
// answer "does this host have credentials" without a join.
//
// Host is a pure data type (DTO). All SQL lives in internal/store.HostRepo
// (R2 refactor).
type Host struct {
	ID                        int64     `json:"id"`
	Nickname                  string    `json:"nickname"`
	OficialSlug               string    `json:"oficial_slug"`
	Hostname                  string    `json:"hostname"`
	Hospedagem                string    `json:"hospedagem"`
	TipoMaquina               string    `json:"tipo_maquina"`
	User                      string    `json:"user"`
	HasPassword               bool      `json:"has_password"`
	HasKey                    bool      `json:"has_key"`
	KeyPath                   string    `json:"key_path"`
	Port                      string    `json:"port"`
	IdentitiesOnly            string    `json:"identities_only"`
	ProxyJump                 string    `json:"proxy_jump"`
	ForwardAgent              string    `json:"forward_agent"`
	Description               string    `json:"description"`
	SetorResponsavel          string    `json:"setor_responsavel"`
	ResponsavelInterno        string    `json:"responsavel_interno"`
	ContatoResponsavelInterno string    `json:"contato_responsavel_interno"`
	AcessoEmpresaExterna      bool      `json:"acesso_empresa_externa"`
	EmpresaResponsavel        string    `json:"empresa_responsavel"`
	ResponsavelExterno        string    `json:"responsavel_externo"`
	ContatoResponsavelExterno string    `json:"contato_responsavel_externo"`
	RecursoCPU                string    `json:"recurso_cpu"`
	RecursoRAM                string    `json:"recurso_ram"`
	RecursoArmazenamento      string    `json:"recurso_armazenamento"`
	Situacao                  string    `json:"situacao"`
	PrecisaManutencao         bool      `json:"precisa_manutencao"`
	PreferredAuth             string    `json:"preferred_auth"`
	ConnectionsFailed         int64     `json:"connections_failed"`
	PasswordTestStatus        *string   `json:"password_test_status"`
	KeyTestStatus             *string   `json:"key_test_status"`
	DockerGroupStatus         *string   `json:"docker_group_status"`
	CoolifyServerUUID         *string   `json:"coolify_server_uuid"`
	Observacoes               string    `json:"observacoes"`
	GrafanaDashboardUID       string    `json:"grafana_dashboard_uid"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// HostFilter is the value object describing list/count predicates, sort, and
// pagination. Consumed by store.HostRepo.List / Count.
type HostFilter struct {
	Situacao           string
	Tag                string
	Hospedagem         string
	Search             string
	EntidadeID         int64 // filter by entidade grant (creator or responsible)
	ResponsavelInterno string
	KeyTestStatus      string // "success" | "failed" | "untested"
	PasswordTestStatus string // "success" | "failed" | "untested"
	ScanResult         string // "failed" | "success" | "untested" — combined OR across both _test_status columns
	HasScan            string // "with" | "without"
	Page               int
	PerPage            int
	SortBy             string
	SortDir            string
}

// FormatPort returns the port or "22" if empty, for display purposes.
func (h *Host) FormatPort() string {
	if h.Port == "" {
		return "22"
	}
	return h.Port
}

// SSHConfigAlias returns the host alias for use in SSH config (oficial_slug).
func (h *Host) SSHConfigAlias() string {
	return h.OficialSlug
}

// DisplayLabel returns a formatted label like "nickname (hostname)" for UI display.
func (h *Host) DisplayLabel() string {
	if h.Hostname != "" {
		return fmt.Sprintf("%s (%s)", h.Nickname, h.Hostname)
	}
	return h.Nickname
}
