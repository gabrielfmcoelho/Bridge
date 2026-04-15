package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const (
	sessionCookieName = "sshcm_session"
	sessionDuration   = 7 * 24 * time.Hour // 7 days
)

// CreateSession generates a new session token for the user and stores it.
func CreateSession(db *sql.DB, userID int64) (string, time.Time, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(sessionDuration)

	_, err := db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("store session: %w", err)
	}
	return token, expiresAt, nil
}

// ValidateSession checks a token and returns the user ID if valid.
func ValidateSession(db *sql.DB, token string) (int64, error) {
	var userID int64
	var expiresAt time.Time
	err := db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("invalid session")
	}
	if err != nil {
		return 0, err
	}
	if time.Now().After(expiresAt) {
		// Clean up expired session.
		db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return 0, fmt.Errorf("session expired")
	}
	return userID, nil
}

// DeleteSession invalidates a session token.
func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// CleanExpiredSessions removes all expired sessions.
func CleanExpiredSessions(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now())
	return err
}

// SetSessionCookie sets the session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionToken extracts the session token from the request cookie.
func GetSessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
