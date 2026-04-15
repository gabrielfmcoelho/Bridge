package models

import (
	"database/sql"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	AuthProvider string    `json:"auth_provider"`
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func CreateUser(db *sql.DB, u *User) error {
	if u.AuthProvider == "" {
		u.AuthProvider = "local"
	}
	id, err := database.InsertReturningID(db,
		`INSERT INTO users (username, password_hash, display_name, role, auth_provider, email) VALUES (?, ?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.DisplayName, u.Role, u.AuthProvider, u.Email,
	)
	if err != nil {
		return err
	}
	u.ID = id
	return nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, password_hash, display_name, role, auth_provider, email, created_at, updated_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.AuthProvider, &u.Email, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func GetUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, password_hash, display_name, role, auth_provider, email, created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.AuthProvider, &u.Email, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`SELECT id, username, password_hash, display_name, role, auth_provider, email, created_at, updated_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.AuthProvider, &u.Email, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func UpdateUser(db *sql.DB, u *User) error {
	_, err := db.Exec(
		`UPDATE users SET username = ?, display_name = ?, role = ?, auth_provider = ?, email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		u.Username, u.DisplayName, u.Role, u.AuthProvider, u.Email, u.ID,
	)
	return err
}

func UpdateUserPassword(db *sql.DB, id int64, passwordHash string) error {
	_, err := db.Exec(
		`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		passwordHash, id,
	)
	return err
}

func DeleteUser(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func UserCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}
