package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// UserRepo owns SQL for the users table. Referenced by api handlers and the
// auth package (session validation, login, setup); built inline at call sites
// until the Phase 2 container hoists it.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo constructs a UserRepo over the given DB handle.
func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

const userCols = `id, username, password_hash, display_name, role, auth_provider, email, created_at, updated_at`

func scanUser(scanner interface{ Scan(...any) error }, u *models.User) error {
	return scanner.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.AuthProvider, &u.Email, &u.CreatedAt, &u.UpdatedAt)
}

// Create inserts a user (defaulting auth_provider to local) and sets u.ID.
func (r *UserRepo) Create(ctx context.Context, u *models.User) error {
	if u.AuthProvider == "" {
		u.AuthProvider = "local"
	}
	id, err := database.InsertReturningID(r.db,
		`INSERT INTO users (username, password_hash, display_name, role, auth_provider, email) VALUES (?, ?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.DisplayName, u.Role, u.AuthProvider, u.Email,
	)
	if err != nil {
		return err
	}
	u.ID = id
	return nil
}

// GetByUsername returns the user with the given username, or (nil, nil).
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	u := &models.User{}
	err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username), u)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetByID returns the user with the given id, or (nil, nil).
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	u := &models.User{}
	err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id), u)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// List returns all users ordered by id.
func (r *UserRepo) List(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if err := scanUser(rows, &u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Update writes the editable profile fields (not the password) by id.
func (r *UserRepo) Update(ctx context.Context, u *models.User) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET username = ?, display_name = ?, role = ?, auth_provider = ?, email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		u.Username, u.DisplayName, u.Role, u.AuthProvider, u.Email, u.ID,
	)
	return err
}

// UpdatePassword sets a new password hash by id.
func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, passwordHash, id)
	return err
}

// Delete removes a user by id.
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// Count returns the total number of users.
func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}
