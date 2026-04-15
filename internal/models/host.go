package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Host struct {
	ID                        int64     `json:"id"`
	Nickname                  string    `json:"nickname"`
	OficialSlug               string    `json:"oficial_slug"`
	Hostname                  string    `json:"hostname"`
	Hospedagem                string    `json:"hospedagem"`
	TipoMaquina               string    `json:"tipo_maquina"`
	User                      string    `json:"user"`
	HasPassword               bool      `json:"has_password"`
	PasswordCiphertext        []byte    `json:"-"`
	PasswordNonce             []byte    `json:"-"`
	HasKey                    bool      `json:"has_key"`
	KeyPath                   string    `json:"key_path"`
	PubKeyCiphertext          []byte    `json:"-"`
	PubKeyNonce               []byte    `json:"-"`
	PrivKeyCiphertext         []byte    `json:"-"`
	PrivKeyNonce              []byte    `json:"-"`
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
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

type HostFilter struct {
	Situacao            string
	Tag                 string
	Hospedagem          string
	Search              string
	EntidadeResponsavel string
	ResponsavelInterno  string
	KeyTestStatus       string // "success" | "failed" | "untested"
	PasswordTestStatus  string // "success" | "failed" | "untested"
	HasScan             string // "with" | "without"
	Page                int
	PerPage             int
	SortBy              string
	SortDir             string
}

func CreateHost(db *sql.DB, h *Host) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO hosts (
			nickname, oficial_slug, hostname, hospedagem, tipo_maquina,
			ssh_user, has_password, password_ciphertext, password_nonce,
			has_key, key_path, pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce,
			port, identities_only, proxy_jump, forward_agent,
			description, setor_responsavel, responsavel_interno, contato_responsavel_interno,
			acesso_empresa_externa, empresa_responsavel, responsavel_externo, contato_responsavel_externo,
			recurso_cpu, recurso_ram, recurso_armazenamento,
			situacao, precisa_manutencao, preferred_auth, connections_failed, password_test_status, key_test_status, docker_group_status, observacoes
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		h.Nickname, h.OficialSlug, h.Hostname, h.Hospedagem, h.TipoMaquina,
		h.User, h.HasPassword, h.PasswordCiphertext, h.PasswordNonce,
		h.HasKey, h.KeyPath, h.PubKeyCiphertext, h.PubKeyNonce, h.PrivKeyCiphertext, h.PrivKeyNonce,
		h.Port, h.IdentitiesOnly, h.ProxyJump, h.ForwardAgent,
		h.Description, h.SetorResponsavel, h.ResponsavelInterno, h.ContatoResponsavelInterno,
		h.AcessoEmpresaExterna, h.EmpresaResponsavel, h.ResponsavelExterno, h.ContatoResponsavelExterno,
		h.RecursoCPU, h.RecursoRAM, h.RecursoArmazenamento,
		h.Situacao, h.PrecisaManutencao, h.PreferredAuth, h.ConnectionsFailed, h.PasswordTestStatus, h.KeyTestStatus, h.DockerGroupStatus, h.Observacoes,
	)
	if err != nil {
		return err
	}
	h.ID = id
	return nil
}

func GetHostBySlug(db *sql.DB, slug string) (*Host, error) {
	h := &Host{}
	err := db.QueryRow(
		`SELECT `+hostColumns()+` FROM hosts WHERE oficial_slug = ?`, slug,
	).Scan(hostScanDest(h)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

func GetHostByID(db *sql.DB, id int64) (*Host, error) {
	h := &Host{}
	err := db.QueryRow(
		`SELECT `+hostColumns()+` FROM hosts WHERE id = ?`, id,
	).Scan(hostScanDest(h)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

func ListHosts(db *sql.DB, f HostFilter) ([]Host, error) {
	query := `SELECT ` + hostColumns() + ` FROM hosts`
	var args []any
	var where []string

	if f.Situacao != "" {
		where = append(where, "situacao = ?")
		args = append(args, f.Situacao)
	}
	if f.Hospedagem != "" {
		where = append(where, "hospedagem = ?")
		args = append(args, f.Hospedagem)
	}
	if f.Search != "" {
		op := database.LikeOp()
		where = append(where, "(nickname "+op+" ? OR hostname "+op+" ? OR oficial_slug "+op+" ? OR description "+op+" ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s, s, s)
	}
	if f.Tag != "" {
		where = append(where, "id IN (SELECT entity_id FROM tags WHERE entity_type = 'host' AND tag = ?)")
		args = append(args, f.Tag)
	}
	if f.EntidadeResponsavel != "" {
		where = append(where, "setor_responsavel = ?")
		args = append(args, f.EntidadeResponsavel)
	}
	if f.ResponsavelInterno != "" {
		where = append(where, "responsavel_interno = ?")
		args = append(args, f.ResponsavelInterno)
	}
	if f.KeyTestStatus == "untested" {
		where = append(where, "(key_test_status IS NULL OR key_test_status = '')")
	} else if f.KeyTestStatus != "" {
		where = append(where, "key_test_status = ?")
		args = append(args, f.KeyTestStatus)
	}
	if f.PasswordTestStatus == "untested" {
		where = append(where, "(password_test_status IS NULL OR password_test_status = '')")
	} else if f.PasswordTestStatus != "" {
		where = append(where, "password_test_status = ?")
		args = append(args, f.PasswordTestStatus)
	}
	if f.HasScan == "with" {
		where = append(where, "id IN (SELECT DISTINCT host_id FROM host_scans)")
	} else if f.HasScan == "without" {
		where = append(where, "id NOT IN (SELECT DISTINCT host_id FROM host_scans)")
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	// Sort
	allowedSorts := map[string]string{
		"nickname": "nickname", "hostname": "hostname", "hospedagem": "hospedagem",
		"situacao": "situacao", "user": "ssh_user", "tipo_maquina": "tipo_maquina",
	}
	sortCol := "nickname"
	if col, ok := allowedSorts[f.SortBy]; ok {
		sortCol = col
	}
	sortDir := "ASC"
	if f.SortDir == "desc" {
		sortDir = "DESC"
	}
	query += " ORDER BY " + sortCol + " " + sortDir

	// Pagination
	if f.PerPage > 0 {
		offset := 0
		if f.Page > 1 {
			offset = (f.Page - 1) * f.PerPage
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PerPage, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(hostScanDest(&h)...); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func CountHosts(db *sql.DB, f HostFilter) (int, error) {
	query := `SELECT COUNT(*) FROM hosts`
	var args []any
	var where []string

	if f.Situacao != "" {
		where = append(where, "situacao = ?")
		args = append(args, f.Situacao)
	}
	if f.Hospedagem != "" {
		where = append(where, "hospedagem = ?")
		args = append(args, f.Hospedagem)
	}
	if f.Search != "" {
		op := database.LikeOp()
		where = append(where, "(nickname "+op+" ? OR hostname "+op+" ? OR oficial_slug "+op+" ? OR description "+op+" ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s, s, s)
	}
	if f.Tag != "" {
		where = append(where, "id IN (SELECT entity_id FROM tags WHERE entity_type = 'host' AND tag = ?)")
		args = append(args, f.Tag)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func UpdateHost(db *sql.DB, h *Host) error {
	_, err := db.Exec(
		`UPDATE hosts SET
			nickname = ?, oficial_slug = ?, hostname = ?, hospedagem = ?, tipo_maquina = ?,
			ssh_user = ?, has_password = ?, password_ciphertext = ?, password_nonce = ?,
			has_key = ?, key_path = ?, pub_key_ciphertext = ?, pub_key_nonce = ?,
			priv_key_ciphertext = ?, priv_key_nonce = ?,
			port = ?, identities_only = ?, proxy_jump = ?, forward_agent = ?,
			description = ?, setor_responsavel = ?, responsavel_interno = ?, contato_responsavel_interno = ?,
			acesso_empresa_externa = ?, empresa_responsavel = ?, responsavel_externo = ?, contato_responsavel_externo = ?,
			recurso_cpu = ?, recurso_ram = ?, recurso_armazenamento = ?,
			situacao = ?, precisa_manutencao = ?, preferred_auth = ?, connections_failed = ?, password_test_status = ?, key_test_status = ?, docker_group_status = ?, observacoes = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		h.Nickname, h.OficialSlug, h.Hostname, h.Hospedagem, h.TipoMaquina,
		h.User, h.HasPassword, h.PasswordCiphertext, h.PasswordNonce,
		h.HasKey, h.KeyPath, h.PubKeyCiphertext, h.PubKeyNonce,
		h.PrivKeyCiphertext, h.PrivKeyNonce,
		h.Port, h.IdentitiesOnly, h.ProxyJump, h.ForwardAgent,
		h.Description, h.SetorResponsavel, h.ResponsavelInterno, h.ContatoResponsavelInterno,
		h.AcessoEmpresaExterna, h.EmpresaResponsavel, h.ResponsavelExterno, h.ContatoResponsavelExterno,
		h.RecursoCPU, h.RecursoRAM, h.RecursoArmazenamento,
		h.Situacao, h.PrecisaManutencao, h.PreferredAuth, h.ConnectionsFailed, h.PasswordTestStatus, h.KeyTestStatus, h.DockerGroupStatus, h.Observacoes,
		h.ID,
	)
	return err
}

// SetHostCoolifyUUID updates only the coolify_server_uuid column.
func SetHostCoolifyUUID(db *sql.DB, hostID int64, uuid *string) error {
	_, err := db.Exec(`UPDATE hosts SET coolify_server_uuid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, uuid, hostID)
	return err
}

func DeleteHost(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	return err
}

func HostCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&n)
	return n, err
}

func HostCountBySituacao(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT situacao, COUNT(*) FROM hosts GROUP BY situacao`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int)
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		m[s] = n
	}
	return m, rows.Err()
}

func HostsNeedingMaintenanceCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE precisa_manutencao`).Scan(&n)
	return n, err
}

func HostCountByHospedagem(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT COALESCE(NULLIF(hospedagem, ''), 'Unknown'), COUNT(*) FROM hosts GROUP BY COALESCE(NULLIF(hospedagem, ''), 'Unknown')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int)
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		m[s] = n
	}
	return m, rows.Err()
}

func hostColumns() string {
	return `id, nickname, oficial_slug, hostname, hospedagem, tipo_maquina,
		ssh_user, has_password, password_ciphertext, password_nonce,
		has_key, key_path, port, identities_only, proxy_jump, forward_agent,
		description, setor_responsavel, responsavel_interno, contato_responsavel_interno,
		acesso_empresa_externa, empresa_responsavel, responsavel_externo, contato_responsavel_externo,
		recurso_cpu, recurso_ram, recurso_armazenamento,
		situacao, precisa_manutencao, preferred_auth, connections_failed, password_test_status, key_test_status, docker_group_status, coolify_server_uuid, observacoes,
		created_at, updated_at,
		pub_key_ciphertext, pub_key_nonce, priv_key_ciphertext, priv_key_nonce`
}

func hostScanDest(h *Host) []any {
	return []any{
		&h.ID, &h.Nickname, &h.OficialSlug, &h.Hostname, &h.Hospedagem, &h.TipoMaquina,
		&h.User, &h.HasPassword, &h.PasswordCiphertext, &h.PasswordNonce,
		&h.HasKey, &h.KeyPath, &h.Port, &h.IdentitiesOnly, &h.ProxyJump, &h.ForwardAgent,
		&h.Description, &h.SetorResponsavel, &h.ResponsavelInterno, &h.ContatoResponsavelInterno,
		&h.AcessoEmpresaExterna, &h.EmpresaResponsavel, &h.ResponsavelExterno, &h.ContatoResponsavelExterno,
		&h.RecursoCPU, &h.RecursoRAM, &h.RecursoArmazenamento,
		&h.Situacao, &h.PrecisaManutencao, &h.PreferredAuth, &h.ConnectionsFailed, &h.PasswordTestStatus, &h.KeyTestStatus, &h.DockerGroupStatus, &h.CoolifyServerUUID, &h.Observacoes,
		&h.CreatedAt, &h.UpdatedAt,
		&h.PubKeyCiphertext, &h.PubKeyNonce, &h.PrivKeyCiphertext, &h.PrivKeyNonce,
	}
}

// HostSlugExists checks if a slug is already taken, optionally excluding a specific host ID.
func HostSlugExists(db *sql.DB, slug string, excludeID int64) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM hosts WHERE oficial_slug = ? AND id != ?`,
		slug, excludeID,
	).Scan(&n)
	return n > 0, err
}

// ListHostsForSSHConfig returns all active hosts with SSH-relevant fields populated.
func ListHostsForSSHConfig(db *sql.DB) ([]Host, error) {
	return ListHosts(db, HostFilter{Situacao: "active"})
}

// UpdateHostPassword updates only the password fields for a host.
func UpdateHostPassword(db *sql.DB, id int64, hasPassword bool, ciphertext, nonce []byte) error {
	_, err := db.Exec(
		`UPDATE hosts SET has_password = ?, password_ciphertext = ?, password_nonce = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		hasPassword, ciphertext, nonce, id,
	)
	return err
}

// UpdateHostKey updates only the key fields for a host, including encrypted key content.
func UpdateHostKey(db *sql.DB, id int64, hasKey bool, keyPath, identitiesOnly string,
	pubKeyCT, pubKeyNonce, privKeyCT, privKeyNonce []byte) error {
	_, err := db.Exec(
		`UPDATE hosts SET has_key = ?, key_path = ?, identities_only = ?,
		 pub_key_ciphertext = ?, pub_key_nonce = ?,
		 priv_key_ciphertext = ?, priv_key_nonce = ?,
		 updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		hasKey, keyPath, identitiesOnly,
		pubKeyCT, pubKeyNonce, privKeyCT, privKeyNonce,
		id,
	)
	return err
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
