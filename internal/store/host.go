package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostRepo owns SQL for the hosts table. The encrypted password / SSH-key
// payloads live in the unified `secrets` table (vault); the host row only
// carries the cached has_password / has_key booleans so list/filter queries
// stay a single-table scan.
type HostRepo struct {
	db *sql.DB
}

// NewHostRepo constructs a HostRepo over the given DB handle.
func NewHostRepo(db *sql.DB) *HostRepo { return &HostRepo{db: db} }

// hostColumns / hostScanDest — secret-payload columns intentionally absent.
func hostColumns() string {
	return `id, nickname, oficial_slug, hostname, hospedagem, tipo_maquina,
		ssh_user, has_password,
		has_key, key_path, port, identities_only, proxy_jump, forward_agent,
		description, setor_responsavel, responsavel_interno, contato_responsavel_interno,
		acesso_empresa_externa, empresa_responsavel, responsavel_externo, contato_responsavel_externo,
		recurso_cpu, recurso_ram, recurso_armazenamento,
		situacao, precisa_manutencao, preferred_auth, connections_failed, password_test_status, key_test_status, docker_group_status, coolify_server_uuid, observacoes,
		grafana_dashboard_uid,
		created_at, updated_at`
}

func hostScanDest(h *models.Host) []any {
	return []any{
		&h.ID, &h.Nickname, &h.OficialSlug, &h.Hostname, &h.Hospedagem, &h.TipoMaquina,
		&h.User, &h.HasPassword,
		&h.HasKey, &h.KeyPath, &h.Port, &h.IdentitiesOnly, &h.ProxyJump, &h.ForwardAgent,
		&h.Description, &h.SetorResponsavel, &h.ResponsavelInterno, &h.ContatoResponsavelInterno,
		&h.AcessoEmpresaExterna, &h.EmpresaResponsavel, &h.ResponsavelExterno, &h.ContatoResponsavelExterno,
		&h.RecursoCPU, &h.RecursoRAM, &h.RecursoArmazenamento,
		&h.Situacao, &h.PrecisaManutencao, &h.PreferredAuth, &h.ConnectionsFailed, &h.PasswordTestStatus, &h.KeyTestStatus, &h.DockerGroupStatus, &h.CoolifyServerUUID, &h.Observacoes,
		&h.GrafanaDashboardUID,
		&h.CreatedAt, &h.UpdatedAt,
	}
}

// Create inserts a host and sets h.ID. Secret payloads are written separately
// via the vault; only the has_password / has_key flags live here.
func (r *HostRepo) Create(ctx context.Context, h *models.Host) error {
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO hosts (
			nickname, oficial_slug, hostname, hospedagem, tipo_maquina,
			ssh_user, has_password,
			has_key, key_path,
			port, identities_only, proxy_jump, forward_agent,
			description, setor_responsavel, responsavel_interno, contato_responsavel_interno,
			acesso_empresa_externa, empresa_responsavel, responsavel_externo, contato_responsavel_externo,
			recurso_cpu, recurso_ram, recurso_armazenamento,
			situacao, precisa_manutencao, preferred_auth, connections_failed, password_test_status, key_test_status, docker_group_status, observacoes,
			grafana_dashboard_uid
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		h.Nickname, h.OficialSlug, h.Hostname, h.Hospedagem, h.TipoMaquina,
		h.User, h.HasPassword,
		h.HasKey, h.KeyPath,
		h.Port, h.IdentitiesOnly, h.ProxyJump, h.ForwardAgent,
		h.Description, h.SetorResponsavel, h.ResponsavelInterno, h.ContatoResponsavelInterno,
		h.AcessoEmpresaExterna, h.EmpresaResponsavel, h.ResponsavelExterno, h.ContatoResponsavelExterno,
		h.RecursoCPU, h.RecursoRAM, h.RecursoArmazenamento,
		h.Situacao, h.PrecisaManutencao, h.PreferredAuth, h.ConnectionsFailed, h.PasswordTestStatus, h.KeyTestStatus, h.DockerGroupStatus, h.Observacoes,
		h.GrafanaDashboardUID,
	)
	if err != nil {
		return err
	}
	h.ID = id
	return nil
}

// GetBySlug returns a host by oficial_slug, or (nil, nil) if absent.
func (r *HostRepo) GetBySlug(ctx context.Context, slug string) (*models.Host, error) {
	h := &models.Host{}
	err := r.db.QueryRowContext(ctx, `SELECT `+hostColumns()+` FROM hosts WHERE oficial_slug = ? AND deleted_at IS NULL`, slug).Scan(hostScanDest(h)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

// GetByID returns a host by id, or (nil, nil) if absent.
func (r *HostRepo) GetByID(ctx context.Context, id int64) (*models.Host, error) {
	h := &models.Host{}
	err := r.db.QueryRowContext(ctx, `SELECT `+hostColumns()+` FROM hosts WHERE id = ? AND deleted_at IS NULL`, id).Scan(hostScanDest(h)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

// List returns hosts matching the filter, with sort + pagination applied.
func (r *HostRepo) List(ctx context.Context, f models.HostFilter) ([]models.Host, error) {
	query := `SELECT ` + hostColumns() + ` FROM hosts`
	var args []any
	where := []string{"deleted_at IS NULL"}

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
		where = append(where, "id IN (SELECT host_id FROM host_entidades WHERE entidade = ?)")
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
	// scan_result buckets by the *active* method (the one actually used to
	// connect to the host), not "any column matches". Mirrors the frontend's
	// activeMethodStatus helper.
	const activeStatusExpr = `CASE
		WHEN has_password AND has_key AND preferred_auth = 'password' THEN COALESCE(password_test_status,'')
		WHEN has_password AND has_key THEN COALESCE(key_test_status,'')
		WHEN has_key THEN COALESCE(key_test_status,'')
		ELSE COALESCE(password_test_status,'')
	END`
	switch f.ScanResult {
	case "failed":
		where = append(where, "("+activeStatusExpr+") = 'failed'")
	case "success":
		where = append(where, "("+activeStatusExpr+") = 'success'")
	case "untested":
		where = append(where, "("+activeStatusExpr+") = ''")
	}
	if f.HasScan == "with" {
		where = append(where, "id IN (SELECT DISTINCT host_id FROM host_scans)")
	} else if f.HasScan == "without" {
		where = append(where, "id NOT IN (SELECT DISTINCT host_id FROM host_scans)")
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

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

	if f.PerPage > 0 {
		offset := 0
		if f.Page > 1 {
			offset = (f.Page - 1) * f.PerPage
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PerPage, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		if err := rows.Scan(hostScanDest(&h)...); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

// ListForSSHConfig returns all active hosts with SSH-relevant fields populated.
func (r *HostRepo) ListForSSHConfig(ctx context.Context) ([]models.Host, error) {
	return r.List(ctx, models.HostFilter{Situacao: "active"})
}

// Count returns the number of hosts matching the (situacao/hospedagem/search/tag
// subset of the) filter — the same predicates the list view paginates over.
func (r *HostRepo) Count(ctx context.Context, f models.HostFilter) (int, error) {
	query := `SELECT COUNT(*) FROM hosts`
	var args []any
	where := []string{"deleted_at IS NULL"}

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
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// CountAll returns the total number of hosts.
func (r *HostRepo) CountAll(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CountBySituacao returns situacao → host count.
func (r *HostRepo) CountBySituacao(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT situacao, COUNT(*) FROM hosts WHERE deleted_at IS NULL GROUP BY situacao`)
	if err != nil {
		return nil, err
	}
	return scanStringCountMap(rows)
}

// NeedingMaintenanceCount returns the number of hosts flagged precisa_manutencao.
func (r *HostRepo) NeedingMaintenanceCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE precisa_manutencao AND deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CountByHospedagem returns hospedagem (Unknown when empty) → host count.
func (r *HostRepo) CountByHospedagem(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(NULLIF(hospedagem, ''), 'Unknown'), COUNT(*) FROM hosts WHERE deleted_at IS NULL GROUP BY COALESCE(NULLIF(hospedagem, ''), 'Unknown')`)
	if err != nil {
		return nil, err
	}
	return scanStringCountMap(rows)
}

// Update writes the mutable fields of a host by id (secret payloads excluded).
func (r *HostRepo) Update(ctx context.Context, h *models.Host) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE hosts SET
			nickname = ?, oficial_slug = ?, hostname = ?, hospedagem = ?, tipo_maquina = ?,
			ssh_user = ?, has_password = ?,
			has_key = ?, key_path = ?,
			port = ?, identities_only = ?, proxy_jump = ?, forward_agent = ?,
			description = ?, setor_responsavel = ?, responsavel_interno = ?, contato_responsavel_interno = ?,
			acesso_empresa_externa = ?, empresa_responsavel = ?, responsavel_externo = ?, contato_responsavel_externo = ?,
			recurso_cpu = ?, recurso_ram = ?, recurso_armazenamento = ?,
			situacao = ?, precisa_manutencao = ?, preferred_auth = ?, connections_failed = ?, password_test_status = ?, key_test_status = ?, docker_group_status = ?, observacoes = ?,
			grafana_dashboard_uid = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		h.Nickname, h.OficialSlug, h.Hostname, h.Hospedagem, h.TipoMaquina,
		h.User, h.HasPassword,
		h.HasKey, h.KeyPath,
		h.Port, h.IdentitiesOnly, h.ProxyJump, h.ForwardAgent,
		h.Description, h.SetorResponsavel, h.ResponsavelInterno, h.ContatoResponsavelInterno,
		h.AcessoEmpresaExterna, h.EmpresaResponsavel, h.ResponsavelExterno, h.ContatoResponsavelExterno,
		h.RecursoCPU, h.RecursoRAM, h.RecursoArmazenamento,
		h.Situacao, h.PrecisaManutencao, h.PreferredAuth, h.ConnectionsFailed, h.PasswordTestStatus, h.KeyTestStatus, h.DockerGroupStatus, h.Observacoes,
		h.GrafanaDashboardUID,
		h.ID,
	)
	return err
}

// SetCoolifyUUID updates only the coolify_server_uuid column.
func (r *HostRepo) SetCoolifyUUID(ctx context.Context, hostID int64, uuid *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE hosts SET coolify_server_uuid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, uuid, hostID)
	return err
}

// Delete removes a host row by id. (Vault cascade is handled separately via the
// cascade registry in the delete path.)
func (r *HostRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	return err
}

// SlugExists reports whether a slug is taken, optionally excluding one host id.
func (r *HostRepo) SlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var n int
	// NOTE: no deleted_at filter — a soft-deleted host keeps its slug reserved
	// (the unique index still covers it), so creating a new host with that slug
	// is correctly rejected; restore the soft-deleted host instead.
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE oficial_slug = ? AND id != ?`, slug, excludeID).Scan(&n)
	return n > 0, err
}

// UpdateHasPassword flips the host's cached has_password flag (the payload lives
// in the vault).
func (r *HostRepo) UpdateHasPassword(ctx context.Context, id int64, hasPassword bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE hosts SET has_password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hasPassword, id)
	return err
}

// UpdateKeyMeta sets has_key + key_path + identities_only without touching the
// encrypted payload (written via vault.HostSetSSHKey).
func (r *HostRepo) UpdateKeyMeta(ctx context.Context, id int64, hasKey bool, keyPath, identitiesOnly string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE hosts SET has_key = ?, key_path = ?, identities_only = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		hasKey, keyPath, identitiesOnly, id,
	)
	return err
}

// scanStringCountMap reads (string, count) rows into a map.
func scanStringCountMap(rows *sql.Rows) (map[string]int, error) {
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

// ListTrash returns soft-deleted hosts (deleted_at set), newest-slug order.
func (r *HostRepo) ListTrash(ctx context.Context) ([]models.Host, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+hostColumns()+` FROM hosts WHERE deleted_at IS NOT NULL ORDER BY oficial_slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		if err := rows.Scan(hostScanDest(&h)...); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}
