package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type Orchestrator struct {
	ID          int64     `json:"id"`
	HostID      int64     `json:"host_id"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	Observacoes string    `json:"observacoes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func CreateOrchestrator(db *sql.DB, o *Orchestrator) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO orchestrators (host_id, type, version, observacoes) VALUES (?, ?, ?, ?)`,
		o.HostID, o.Type, o.Version, o.Observacoes,
	)
	if err != nil {
		return err
	}
	o.ID = id
	return nil
}

func GetOrchestrator(db *sql.DB, id int64) (*Orchestrator, error) {
	o := &Orchestrator{}
	err := db.QueryRow(
		`SELECT id, host_id, type, version, observacoes, created_at, updated_at FROM orchestrators WHERE id = ?`, id,
	).Scan(&o.ID, &o.HostID, &o.Type, &o.Version, &o.Observacoes, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

func GetOrchestratorByHost(db *sql.DB, hostID int64) (*Orchestrator, error) {
	o := &Orchestrator{}
	err := db.QueryRow(
		`SELECT id, host_id, type, version, observacoes, created_at, updated_at FROM orchestrators WHERE host_id = ?`, hostID,
	).Scan(&o.ID, &o.HostID, &o.Type, &o.Version, &o.Observacoes, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

func ListOrchestrators(db *sql.DB) ([]Orchestrator, error) {
	rows, err := db.Query(`SELECT id, host_id, type, version, observacoes, created_at, updated_at FROM orchestrators ORDER BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orchestrators []Orchestrator
	for rows.Next() {
		var o Orchestrator
		if err := rows.Scan(&o.ID, &o.HostID, &o.Type, &o.Version, &o.Observacoes, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orchestrators = append(orchestrators, o)
	}
	return orchestrators, rows.Err()
}

func UpdateOrchestrator(db *sql.DB, o *Orchestrator) error {
	_, err := db.Exec(
		`UPDATE orchestrators SET type = ?, version = ?, observacoes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		o.Type, o.Version, o.Observacoes, o.ID,
	)
	return err
}

func DeleteOrchestrator(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM orchestrators WHERE id = ?`, id)
	return err
}

func OrchestratorCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM orchestrators`).Scan(&n)
	return n, err
}
