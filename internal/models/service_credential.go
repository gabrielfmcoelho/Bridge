package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type ServiceCredential struct {
	ID                   int64     `json:"id"`
	ServiceID            int64     `json:"service_id"`
	RoleName             string    `json:"role_name"`
	CredentialsCiphertext []byte   `json:"-"`
	CredentialsNonce     []byte    `json:"-"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func CreateServiceCredential(db *sql.DB, sc *ServiceCredential) error {
	id, err := database.InsertReturningID(db,
		`INSERT INTO service_credentials (service_id, role_name, credentials_ciphertext, credentials_nonce) VALUES (?, ?, ?, ?)`,
		sc.ServiceID, sc.RoleName, sc.CredentialsCiphertext, sc.CredentialsNonce,
	)
	if err != nil {
		return err
	}
	sc.ID = id
	return nil
}

func GetServiceCredential(db *sql.DB, id int64) (*ServiceCredential, error) {
	sc := &ServiceCredential{}
	err := db.QueryRow(
		`SELECT id, service_id, role_name, credentials_ciphertext, credentials_nonce, created_at, updated_at
		FROM service_credentials WHERE id = ?`, id,
	).Scan(&sc.ID, &sc.ServiceID, &sc.RoleName, &sc.CredentialsCiphertext, &sc.CredentialsNonce, &sc.CreatedAt, &sc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sc, err
}

func ListServiceCredentials(db *sql.DB, serviceID int64) ([]ServiceCredential, error) {
	rows, err := db.Query(
		`SELECT id, service_id, role_name, credentials_ciphertext, credentials_nonce, created_at, updated_at
		FROM service_credentials WHERE service_id = ? ORDER BY role_name`, serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []ServiceCredential
	for rows.Next() {
		var sc ServiceCredential
		if err := rows.Scan(&sc.ID, &sc.ServiceID, &sc.RoleName, &sc.CredentialsCiphertext, &sc.CredentialsNonce, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		creds = append(creds, sc)
	}
	return creds, rows.Err()
}

func UpdateServiceCredential(db *sql.DB, sc *ServiceCredential) error {
	_, err := db.Exec(
		`UPDATE service_credentials SET role_name = ?, credentials_ciphertext = ?, credentials_nonce = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		sc.RoleName, sc.CredentialsCiphertext, sc.CredentialsNonce, sc.ID,
	)
	return err
}

func DeleteServiceCredential(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM service_credentials WHERE id = ?`, id)
	return err
}
