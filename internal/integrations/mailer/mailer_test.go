package mailer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSMTP answers one plaintext SMTP session and hands back the envelope and
// DATA it received.
func fakeSMTP(t *testing.T) (addr string, got <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		rd := bufio.NewReader(c)
		var log strings.Builder
		say := func(s string) { c.Write([]byte(s + "\r\n")) }
		say("220 fake")
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				ch <- log.String()
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
				log.WriteString(strings.TrimSpace(line) + "\n")
				say("250 ok")
			case cmd == "DATA":
				say("354 go")
				for {
					l, _ := rd.ReadString('\n')
					if l == ".\r\n" || l == "" {
						break
					}
					log.WriteString(l)
				}
				say("250 queued")
			case cmd == "QUIT":
				say("221 bye")
				ch <- log.String()
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().String(), ch
}

func TestSend_PlaintextRelay(t *testing.T) {
	addr, got := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	s := Settings{Enabled: true, Host: host, Port: port, TLS: "none", From: "Bridge <bridge@sead.pi.gov.br>"}
	err := Send(context.Background(), s, Message{
		To:      "Fulana <fulana@example.com>",
		Subject: "Compartilhamento: título\r\nBcc: evil@example.com",
		Body:    "Senha: ção\n",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	out := <-got
	for _, want := range []string{"MAIL FROM:<bridge@sead.pi.gov.br>", "RCPT TO:<fulana@example.com>", "Subject: =?utf-8?q?", "Senha: =C3=A7=C3=A3o"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\r\nBcc:") {
		t.Errorf("subject CR/LF injected a header:\n%s", out)
	}
}

func TestShareEmail_RendersAndEscapes(t *testing.T) {
	m, err := ShareEmail(ShareData{
		Name: "Fulana", Sender: "Gabriel", Title: `Acesso <script>alert(1)</script>`,
		Description: "Chaves da API", URL: "https://bridge.example/share/abc_DEF-123",
		Passphrase: "Xk9-senha", ExpiresAt: "08/10/2026 15:04",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="https://bridge.example/share/abc_DEF-123"`, "Xk9-senha", "08/10/2026 15:04", "Fulana", "&lt;script&gt;"} {
		if !strings.Contains(m.HTML, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	if strings.Contains(m.HTML, "<script>") {
		t.Error("title not escaped in HTML")
	}
	for _, want := range []string{"Link de acesso: https://bridge.example/share/abc_DEF-123", "Senha: Xk9-senha", "Olá, Fulana"} {
		if !strings.Contains(m.Body, want) {
			t.Errorf("text missing %q", want)
		}
	}

	noPass, _ := ShareEmail(ShareData{Sender: "G", Title: "T", URL: "https://b/share/x"})
	if strings.Contains(noPass.HTML, "Senha de acesso") || !strings.Contains(noPass.HTML, "sem data de expira") {
		t.Error("no-passphrase / no-expiry variant rendered wrong")
	}
}

func TestBuildMessage_Multipart(t *testing.T) {
	from, _ := mail.ParseAddress("bridge@example.com")
	to, _ := mail.ParseAddress("a@example.com")
	raw, err := buildMessage(from, to, Message{Subject: "s", Body: "texto", HTML: "<p>html</p>"})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	mt, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if mt != "multipart/alternative" {
		t.Fatalf("content-type = %s", mt)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types []string
	for {
		p, err := mr.NextPart() // decodes quoted-printable
		if err != nil {
			break
		}
		body, _ := io.ReadAll(p)
		types = append(types, p.Header.Get("Content-Type")+"="+string(body))
	}
	if strings.Join(types, "|") != "text/plain; charset=UTF-8=texto|text/html; charset=UTF-8=<p>html</p>" {
		t.Errorf("parts = %v", types)
	}

	plain, _ := buildMessage(from, to, Message{Subject: "s", Body: "só texto"})
	if !bytes.Contains(plain, []byte("s=C3=B3 texto")) {
		t.Errorf("plain-only body missing:\n%s", plain)
	}
}

// TestPreview writes the rendered emails to $MAILER_PREVIEW_DIR to look at in
// a browser: MAILER_PREVIEW_DIR=/tmp/mail go test -run Preview ./internal/integrations/mailer
func TestPreview(t *testing.T) {
	dir := os.Getenv("MAILER_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set MAILER_PREVIEW_DIR to write previews")
	}
	share, _ := ShareEmail(ShareData{
		Name: "Maria Souza", Sender: "Gabriel Coelho", Title: "API de Servidores — chaves de acesso",
		Description: "Credenciais do ambiente de homologação.\nQualquer dúvida, fale com a equipe de infraestrutura.",
		URL: "https://bridge.sead.pi.gov.br/share/q3Zt8xVb2LkP0wNcR7mYfH4sJdA1eGuT9oXiWnQv5Ck", Passphrase: "Tq7-Vena-82Lx", ExpiresAt: "14/10/2026 18:00",
	})
	test, _ := TestEmail("https://bridge.sead.pi.gov.br")
	for name, html := range map[string]string{"share.html": share.HTML, "test.html": test.HTML} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
