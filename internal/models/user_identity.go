package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// UserExternalIdentity links a local user to an external auth provider account.
type UserExternalIdentity struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	ProviderName string    `json:"provider_name"`
	ExternalID   string    `json:"external_id"`
	ExternalData string    `json:"external_data"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func CreateUserExternalIdentity(db *sql.DB, i *UserExternalIdentity) error {
	if i.ExternalData == "" {
		i.ExternalData = "{}"
	}
	id, err := database.InsertReturningID(db,
		`INSERT INTO user_external_identities (user_id, provider_name, external_id, external_data) VALUES (?, ?, ?, ?)`,
		i.UserID, i.ProviderName, i.ExternalID, i.ExternalData,
	)
	if err != nil {
		return err
	}
	i.ID = id
	return nil
}

func GetIdentityByProviderAndExternalID(db *sql.DB, providerName, externalID string) (*UserExternalIdentity, error) {
	i := &UserExternalIdentity{}
	err := db.QueryRow(
		`SELECT id, user_id, provider_name, external_id, external_data, created_at, updated_at
		 FROM user_external_identities WHERE provider_name = ? AND external_id = ?`,
		providerName, externalID,
	).Scan(&i.ID, &i.UserID, &i.ProviderName, &i.ExternalID, &i.ExternalData, &i.CreatedAt, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return i, err
}

func ListIdentitiesByUser(db *sql.DB, userID int64) ([]UserExternalIdentity, error) {
	rows, err := db.Query(
		`SELECT id, user_id, provider_name, external_id, external_data, created_at, updated_at
		 FROM user_external_identities WHERE user_id = ? ORDER BY id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []UserExternalIdentity
	for rows.Next() {
		var i UserExternalIdentity
		if err := rows.Scan(&i.ID, &i.UserID, &i.ProviderName, &i.ExternalID, &i.ExternalData, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		identities = append(identities, i)
	}
	return identities, rows.Err()
}

func DeleteUserExternalIdentity(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM user_external_identities WHERE id = ?`, id)
	return err
}

func DeleteIdentitiesByUser(db *sql.DB, userID int64) error {
	_, err := db.Exec(`DELETE FROM user_external_identities WHERE user_id = ?`, userID)
	return err
}
