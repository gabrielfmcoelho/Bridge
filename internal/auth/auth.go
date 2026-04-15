package auth

import (
	"database/sql"
	"fmt"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SetupRequired returns true if no users exist yet (first-run state).
func SetupRequired(db *sql.DB) (bool, error) {
	n, err := models.UserCount(db)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

// SetupMasterUser creates the first admin user. Fails if any user already exists.
func SetupMasterUser(db *sql.DB, username, password, displayName string) (*models.User, error) {
	n, err := models.UserCount(db)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, fmt.Errorf("master user already exists")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	u := &models.User{
		Username:     username,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         "admin",
	}
	if err := models.CreateUser(db, u); err != nil {
		return nil, fmt.Errorf("create master user: %w", err)
	}
	return u, nil
}

// Login validates credentials and returns the user if successful.
func Login(db *sql.DB, username, password string) (*models.User, error) {
	u, err := models.GetUserByUsername(db, username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !CheckPassword(u.PasswordHash, password) {
		return nil, fmt.Errorf("invalid credentials")
	}
	return u, nil
}
