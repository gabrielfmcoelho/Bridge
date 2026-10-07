package mailer

import (
	"bufio"
	"context"
	"net"
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
