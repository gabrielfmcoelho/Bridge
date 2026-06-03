package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// UserIdentityRepo owns SQL for user_external_identities (links a local user to
// an external auth provider account).
type UserIdentityRepo struct {
	db *sql.DB
}

// NewUserIdentityRepo constructs a UserIdentityRepo over the given DB handle.
func NewUserIdentityRepo(db *sql.DB) *UserIdentityRepo { return &UserIdentityRepo{db: db} }

const userIdentityCols = `id, user_id, provider_name, external_id, external_data, created_at, updated_at`

func scanUserIdentity(scanner interface{ Scan(...any) error }, i *models.UserExternalIdentity) error {
	return scanner.Scan(&i.ID, &i.UserID, &i.ProviderName, &i.ExternalID, &i.ExternalData, &i.CreatedAt, &i.UpdatedAt)
}

// Create inserts an identity (defaulting external_data to "{}") and sets i.ID.
func (r *UserIdentityRepo) Create(ctx context.Context, i *models.UserExternalIdentity) error {
	if i.ExternalData == "" {
		i.ExternalData = "{}"
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO user_external_identities (user_id, provider_name, external_id, external_data) VALUES (?, ?, ?, ?)`,
		i.UserID, i.ProviderName, i.ExternalID, i.ExternalData,
	)
	if err != nil {
		return err
	}
	i.ID = id
	return nil
}

// GetByProviderAndExternalID returns the identity for (provider, external_id),
// or (nil, nil) if none.
func (r *UserIdentityRepo) GetByProviderAndExternalID(ctx context.Context, providerName, externalID string) (*models.UserExternalIdentity, error) {
	i := &models.UserExternalIdentity{}
	err := scanUserIdentity(r.db.QueryRowContext(ctx,
		`SELECT `+userIdentityCols+` FROM user_external_identities WHERE provider_name = ? AND external_id = ?`,
		providerName, externalID), i)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return i, err
}

// ListByUser returns all external identities for a user.
func (r *UserIdentityRepo) ListByUser(ctx context.Context, userID int64) ([]models.UserExternalIdentity, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+userIdentityCols+` FROM user_external_identities WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var identities []models.UserExternalIdentity
	for rows.Next() {
		var i models.UserExternalIdentity
		if err := scanUserIdentity(rows, &i); err != nil {
			return nil, err
		}
		identities = append(identities, i)
	}
	return identities, rows.Err()
}
