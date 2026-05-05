package models

import (
	"database/sql"
	"fmt"
)

// HostEntidade is the junction between a host and an entidade
// (department/sector). A host can be allocated to multiple entidades; one is
// the main entidade shown on the host card.
type HostEntidade struct {
	ID       int64  `json:"id"`
	HostID   int64  `json:"host_id"`
	Entidade string `json:"entidade"`
	IsMain   bool   `json:"is_main"`
}

// HostEntidadeInput is the write-side payload.
type HostEntidadeInput struct {
	Entidade string `json:"entidade"`
	IsMain   bool   `json:"is_main"`
}

// ListHostEntidades returns all entidades linked to a host, main first.
func ListHostEntidades(db *sql.DB, hostID int64) ([]HostEntidade, error) {
	rows, err := db.Query(`
		SELECT id, host_id, entidade, is_main
		FROM host_entidades
		WHERE host_id = ?
		ORDER BY is_main DESC, entidade ASC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HostEntidade
	for rows.Next() {
		var e HostEntidade
		if err := rows.Scan(&e.ID, &e.HostID, &e.Entidade, &e.IsMain); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// GetMainEntidadeBulk returns a map of host_id → main entidade for all hosts
// in a single query. Used by the host list to render the card without N+1.
func GetMainEntidadeBulk(db *sql.DB) (map[int64]string, error) {
	rows, err := db.Query(`SELECT host_id, entidade FROM host_entidades WHERE is_main`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]string)
	for rows.Next() {
		var hostID int64
		var entidade string
		if err := rows.Scan(&hostID, &entidade); err != nil {
			return nil, err
		}
		m[hostID] = entidade
	}
	return m, rows.Err()
}

// SyncHostEntidades replaces all entidades for a host with the given inputs.
// Empty entidade strings are rejected. If multiple entries are flagged is_main,
// only the first is honored (callers should already enforce this on the UI).
func SyncHostEntidades(db *sql.DB, hostID int64, inputs []HostEntidadeInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM host_entidades WHERE host_id = ?`, hostID); err != nil {
		return err
	}

	mainSeen := false
	for _, inp := range inputs {
		if inp.Entidade == "" {
			return fmt.Errorf("entidade is required")
		}
		isMain := inp.IsMain && !mainSeen
		if isMain {
			mainSeen = true
		}
		if _, err := tx.Exec(
			`INSERT INTO host_entidades (host_id, entidade, is_main) VALUES (?, ?, ?)`,
			hostID, inp.Entidade, isMain,
		); err != nil {
			return err
		}
	}

	// If no entry was flagged main but there is at least one entidade, promote
	// the alphabetically-first row so the card always has something to show.
	if !mainSeen && len(inputs) > 0 {
		if _, err := tx.Exec(`
			UPDATE host_entidades SET is_main = TRUE
			WHERE id = (SELECT id FROM host_entidades WHERE host_id = ? ORDER BY entidade ASC LIMIT 1)`,
			hostID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
