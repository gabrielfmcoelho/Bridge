// Package mailer sends plain-text email over SMTP (net/smtp), configured from
// the smtp_* integration settings. One message, one recipient per Send: share
// links go to each address separately, so recipients never see each other.
package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// ErrNotConfigured means SMTP is disabled or lacks host, sender or link base URL.
var ErrNotConfigured = errors.New("smtp is not enabled or not configured")

// Settings is the resolved SMTP configuration. TLS is "starttls" (default,
// usually port 587), "tls" (implicit, usually 465) or "none" (a local relay;
// net/smtp then refuses to send a password unless the host is localhost).
// LinkBaseURL is Bridge's public address, prefixed to /share/{token}.
type Settings struct {
	Enabled     bool
	Host        string
	Port        string
	Username    string
	Password    string
	From        string
	TLS         string
	LinkBaseURL string
}

// LoadSettings reads the smtp_* settings, decrypting the password, and
// answers ErrNotConfigured when sending isn't possible.
func LoadSettings(ctx context.Context, db *sql.DB, enc *database.Encryptor) (Settings, error) {
	get := func(k string) string { return strings.TrimSpace(store.NewAppSettingsRepo(db).Value(ctx, k)) }
	s := Settings{
		Enabled:     get("smtp_enabled") == "true",
		Host:        get("smtp_host"),
		Port:        get("smtp_port"),
		Username:    get("smtp_username"),
		From:        get("smtp_from"),
		TLS:         get("smtp_tls"),
		LinkBaseURL: strings.TrimRight(get("smtp_link_base_url"), "/"),
	}
	pw, _, err := store.NewAppSecretRepo(db).Reveal(ctx, enc, "smtp_password")
	if err != nil {
		return s, err
	}
	s.Password = pw
	if !s.Enabled || s.Host == "" || s.From == "" || s.LinkBaseURL == "" {
		return s, ErrNotConfigured
	}
	return s, nil
}

// Message is one plain-text email to one address.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Send delivers m through the configured server.
func Send(ctx context.Context, s Settings, m Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("smtp_from: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	raw, err := buildMessage(from, to, m.Subject, m.Body)
	if err != nil {
		return err
	}

	port := s.Port
	if port == "" {
		port = map[string]string{"tls": "465", "none": "25"}[s.TLS]
		if port == "" {
			port = "587"
		}
	}
	addr := net.JoinHostPort(s.Host, port)
	d := &net.Dialer{Timeout: 15 * time.Second}
	tlsCfg := &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	if s.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connect %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer c.Close()
	if s.TLS != "tls" && s.TLS != "none" {
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if s.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return c.Quit()
}

// buildMessage renders headers + a quoted-printable UTF-8 body. The subject
// goes through mime.QEncoding, which encodes any CR/LF, so a bundle title
// can't inject headers; addresses come from mail.ParseAddress.
func buildMessage(from, to *mail.Address, subject, body string) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
