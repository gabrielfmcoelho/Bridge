package api

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/mailer"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// maxSendRecipients caps one send request; a share link is for a few people.
const maxSendRecipients = 20

// brt is Brasília time (no DST since 2019), fixed so the binary needs no tzdata.
var brt = time.FixedZone("BRT", -3*60*60)

type sendBundleRequest struct {
	// Emails are the addresses to mail; empty = the recipient contact's email.
	Emails []string `json:"emails"`
}

type sendBundleResult struct {
	Email string `json:"email"`
	Sent  bool   `json:"sent"`
	Error string `json:"error,omitempty"`
}

// handleSend godoc
//
//	@Summary		Email a share bundle's link and passphrase
//	@Description	Any role; owner or admin (others get 404). Mails the link and its passphrase, one message per address, to "emails" or, when omitted, the recipient contact's email. Each address mailed is recorded in the access log (action "send"). Always 200 once sending starts, with a per-address {email, sent, error?}. 400 when no valid address; 409 when the link is not live or predates stored links; 503 when SMTP is not configured.
//	@Tags			share-bundles
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Bundle ID"
//	@Param			body	body		sendBundleRequest	false	"Addresses (optional)"
//	@Success		200		{object}	map[string][]sendBundleResult
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Failure		503		{object}	httpx.ErrorResponse
//	@Router			/api/share-bundles/{id}/send [post]
func (h *bundleHandlers) handleSend(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid bundle id", err)
		return
	}
	var req sendBundleRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	d, err := h.repo.BundleForSend(r.Context(), actor, id)
	switch {
	case err == nil:
	case errors.Is(err, vault.ErrBundleNotFound):
		jsonError(w, http.StatusNotFound, "share bundle not found")
		return
	case errors.Is(err, vault.ErrBundleNotLive):
		jsonError(w, http.StatusConflict, "this link is revoked, archived, expired or out of views")
		return
	case errors.Is(err, vault.ErrBundleNotRecoverable):
		jsonError(w, http.StatusConflict, "this link was created before links were stored and cannot be sent")
		return
	default:
		jsonServerError(w, r, "load share bundle", err)
		return
	}

	raw := req.Emails
	if len(raw) == 0 && d.RecipientEmail != "" {
		raw = []string{d.RecipientEmail}
	}
	addrs, err := parseRecipients(raw)
	if err != nil {
		jsonBadRequest(w, r, err.Error(), nil)
		return
	}

	settings, err := mailer.LoadSettings(r.Context(), h.db.SQL, h.db.Encryptor)
	if errors.Is(err, mailer.ErrNotConfigured) {
		jsonError(w, http.StatusServiceUnavailable, "email (SMTP) is not configured")
		return
	}
	if err != nil {
		jsonServerError(w, r, "load smtp settings", err)
		return
	}
	send := h.send
	if send == nil {
		send = mailer.Send
	}

	sender := "Alguém"
	if u := auth.UserFromContext(r.Context()); u != nil {
		sender = cmp.Or(u.DisplayName, u.Username)
	}
	results := make([]sendBundleResult, 0, len(addrs))
	for _, a := range addrs {
		res := sendBundleResult{Email: a.Address}
		undo, err := h.repo.RecordBundleSend(r.Context(), actor, id, a.Address)
		if err != nil {
			jsonServerError(w, r, "audit share send", err)
			return
		}
		name := ""
		if strings.EqualFold(a.Address, d.RecipientEmail) {
			name = d.RecipientName
		}
		msg := shareEmail(d, settings.LinkBaseURL, sender, name)
		msg.To = a.String()
		if err := send(r.Context(), settings, msg); err != nil {
			log.Printf("[share-send] bundle %d to %s: %v", id, a.Address, err)
			if uerr := undo(); uerr != nil {
				log.Printf("[share-send] bundle %d: undo audit row: %v", id, uerr)
			}
			res.Error = err.Error()
		} else {
			res.Sent = true
		}
		results = append(results, res)
	}
	jsonOK(w, map[string]any{"results": results})
}

// parseRecipients validates and de-duplicates addresses (case-insensitive).
func parseRecipients(raw []string) ([]*mail.Address, error) {
	seen := map[string]bool{}
	var out []*mail.Address
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		a, err := mail.ParseAddress(s)
		if err != nil {
			return nil, fmt.Errorf("invalid email address: %q", s)
		}
		if k := strings.ToLower(a.Address); !seen[k] {
			seen[k] = true
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no email address: pass emails or link a recipient contact that has one")
	}
	if len(out) > maxSendRecipients {
		return nil, fmt.Errorf("at most %d addresses per send", maxSendRecipients)
	}
	return out, nil
}

// shareEmail renders the pt-BR message (recipients are outside Bridge, so it
// isn't localised to the sender's UI language). Link and passphrase travel
// together by design; the warning says why the mail must not be forwarded.
func shareEmail(d *vault.BundleDelivery, baseURL, sender, name string) mailer.Message {
	var b strings.Builder
	if name != "" {
		fmt.Fprintf(&b, "Olá, %s,\n\n", name)
	} else {
		b.WriteString("Olá,\n\n")
	}
	title := cmp.Or(d.Title, "Compartilhamento")
	fmt.Fprintf(&b, "%s compartilhou com você \"%s\" pelo Bridge (SEAD-PI).\n\n", sender, title)
	if s := strings.TrimSpace(d.Description); s != "" {
		b.WriteString(s + "\n\n")
	}
	fmt.Fprintf(&b, "Link de acesso: %s/share/%s\n", baseURL, d.Token)
	if d.Passphrase != "" {
		fmt.Fprintf(&b, "Senha: %s\n", d.Passphrase)
	}
	if d.ExpiresAt != nil {
		fmt.Fprintf(&b, "Válido até: %s (horário de Brasília)\n", d.ExpiresAt.In(brt).Format("02/01/2006 15:04"))
	} else {
		b.WriteString("Validade: sem data de expiração\n")
	}
	b.WriteString("\nNão encaminhe este e-mail: quem tiver o link")
	if d.Passphrase != "" {
		b.WriteString(" e a senha")
	}
	b.WriteString(" acessa o conteúdo.\nMensagem automática, não responda.\n")
	return mailer.Message{Subject: "Compartilhamento: " + title, Body: b.String()}
}
