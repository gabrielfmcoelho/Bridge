package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/email.html
var templateFS embed.FS

// views holds one template set per message: the shared layout, with
// "content" bound to that message's block. html/template escapes every value
// (titles and descriptions are user-written).
var views = func() map[string]*template.Template {
	base := template.Must(template.ParseFS(templateFS, "templates/email.html"))
	out := map[string]*template.Template{}
	for _, name := range []string{"share", "test"} {
		t := template.Must(base.Clone())
		template.Must(t.Parse(`{{define "content"}}{{template "` + name + `-content" .}}{{end}}`))
		out[name] = t
	}
	return out
}()

func renderHTML(view string, data any) (string, error) {
	var b bytes.Buffer
	if err := views[view].ExecuteTemplate(&b, "layout", data); err != nil {
		return "", fmt.Errorf("render %s email: %w", view, err)
	}
	return b.String(), nil
}

// ShareData is what a share-link email shows. ExpiresAt is already formatted
// ("" = never expires); Name is the recipient contact's, "" for others.
type ShareData struct {
	Name        string
	Sender      string
	Title       string
	Description string
	URL         string
	Passphrase  string
	ExpiresAt   string
}

// ShareEmail renders the share-link email (HTML + plain-text fallback), pt-BR:
// recipients are outside Bridge, so it isn't in the sender's UI language. The
// link and passphrase travel together by design; the warning says why the
// mail must not be forwarded. To is left for the caller.
func ShareEmail(d ShareData) (Message, error) {
	subject := "Compartilhamento: " + d.Title
	html, err := renderHTML("share", struct {
		ShareData
		Subject, Preheader string
	}{d, subject, d.Sender + " compartilhou \"" + d.Title + "\" com você pelo Bridge."})
	if err != nil {
		return Message{}, err
	}

	var b strings.Builder
	if d.Name != "" {
		fmt.Fprintf(&b, "Olá, %s,\n\n", d.Name)
	} else {
		b.WriteString("Olá,\n\n")
	}
	fmt.Fprintf(&b, "%s compartilhou com você \"%s\" pelo Bridge (SEAD-PI).\n\n", d.Sender, d.Title)
	if s := strings.TrimSpace(d.Description); s != "" {
		b.WriteString(s + "\n\n")
	}
	fmt.Fprintf(&b, "Link de acesso: %s\n", d.URL)
	if d.Passphrase != "" {
		fmt.Fprintf(&b, "Senha: %s\n", d.Passphrase)
	}
	if d.ExpiresAt != "" {
		fmt.Fprintf(&b, "Válido até: %s (horário de Brasília)\n", d.ExpiresAt)
	} else {
		b.WriteString("Validade: sem data de expiração\n")
	}
	b.WriteString("\nNão encaminhe este e-mail: quem tiver o link")
	if d.Passphrase != "" {
		b.WriteString(" e a senha")
	}
	b.WriteString(" acessa o conteúdo.\nMensagem automática, não responda.\n")
	return Message{Subject: subject, Body: b.String(), HTML: html}, nil
}

// TestEmail renders the SMTP settings test message.
func TestEmail(linkBaseURL string) (Message, error) {
	subject := "Bridge: teste de e-mail"
	html, err := renderHTML("test", struct{ Subject, Preheader, LinkBaseURL string }{
		subject, "O envio de e-mail do Bridge está funcionando.", linkBaseURL,
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		Subject: subject,
		Body:    "Este é um teste da configuração de SMTP do Bridge.\nLinks de compartilhamento usarão: " + linkBaseURL + "/share/…\n",
		HTML:    html,
	}, nil
}
