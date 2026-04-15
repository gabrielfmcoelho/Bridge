package models

import (
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// HostChamado represents a chamado (ticket/request) associated with a host.
type HostChamado struct {
	ID              int64  `json:"id"`
	HostID          int64  `json:"host_id"`
	ChamadoID       string `json:"chamado_id"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	UserID          int64  `json:"user_id"`
	UserDisplayName string `json:"user_display_name"`
	Date            string `json:"date"`
}

// HostChamadoInput is the input for creating/syncing host chamados.
type HostChamadoInput struct {
	ChamadoID string `json:"chamado_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	UserID    int64  `json:"user_id"`
	Date      string `json:"date"`
}

const chamadoCols = `hc.id, hc.host_id, hc.chamado_id, hc.title, hc.status, hc.user_id, COALESCE(u.display_name, '') AS user_display_name, hc.date`

func scanChamado(scanner interface{ Scan(...any) error }, c *HostChamado) error {
	return scanner.Scan(&c.ID, &c.HostID, &c.ChamadoID, &c.Title, &c.Status, &c.UserID, &c.UserDisplayName, &c.Date)
}

// ListHostChamados returns all chamados for a host, joined with user display_name.
func ListHostChamados(db *sql.DB, hostID int64) ([]HostChamado, error) {
	rows, err := db.Query(`
		SELECT `+chamadoCols+`
		FROM host_chamados hc
		LEFT JOIN users u ON u.id = hc.user_id
		WHERE hc.host_id = ?
		ORDER BY hc.date DESC, hc.id DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HostChamado
	for rows.Next() {
		var c HostChamado
		if err := scanChamado(rows, &c); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// GetHostChamado returns a single chamado by ID.
func GetHostChamado(db *sql.DB, id int64) (*HostChamado, error) {
	c := &HostChamado{}
	err := scanChamado(db.QueryRow(`
		SELECT `+chamadoCols+`
		FROM host_chamados hc
		LEFT JOIN users u ON u.id = hc.user_id
		WHERE hc.id = ?`, id), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// CreateHostChamado inserts a single chamado.
func CreateHostChamado(db *sql.DB, hostID int64, inp *HostChamadoInput) (int64, error) {
	if inp.Status == "" {
		inp.Status = "in_execution"
	}
	return database.InsertReturningID(db,
		`INSERT INTO host_chamados (host_id, chamado_id, title, status, user_id, date) VALUES (?, ?, ?, ?, ?, ?)`,
		hostID, inp.ChamadoID, inp.Title, inp.Status, inp.UserID, inp.Date,
	)
}

// UpdateHostChamado updates a single chamado.
func UpdateHostChamado(db *sql.DB, id int64, inp *HostChamadoInput) error {
	_, err := db.Exec(
		`UPDATE host_chamados SET chamado_id = ?, title = ?, status = ?, user_id = ?, date = ? WHERE id = ?`,
		inp.ChamadoID, inp.Title, inp.Status, inp.UserID, inp.Date, id,
	)
	return err
}

// DeleteHostChamado deletes a single chamado.
func DeleteHostChamado(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM host_chamados WHERE id = ?`, id)
	return err
}

// SyncHostChamados replaces all chamados for a host with the given inputs.
func SyncHostChamados(db *sql.DB, hostID int64, inputs []HostChamadoInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM host_chamados WHERE host_id = ?`, hostID); err != nil {
		return err
	}

	for _, inp := range inputs {
		status := inp.Status
		if status == "" {
			status = "in_execution"
		}
		if _, err := tx.Exec(
			`INSERT INTO host_chamados (host_id, chamado_id, title, status, user_id, date) VALUES (?, ?, ?, ?, ?, ?)`,
			hostID, inp.ChamadoID, inp.Title, status, inp.UserID, inp.Date,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
