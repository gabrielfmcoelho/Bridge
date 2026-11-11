package minio

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHostPortAndReady(t *testing.T) {
	for in, want := range map[string]string{"10.0.122.76:9000": "10.0.122.76:9000", "http://10.0.122.76:9000/": "10.0.122.76:9000", "https://s3.example.gov.br": "s3.example.gov.br"} {
		if got := hostPort(in); got != want {
			t.Errorf("hostPort(%q) = %q, want %q", in, got, want)
		}
	}
	s := Settings{Enabled: true, Endpoint: "e:9000", AccessKey: "a", SecretKey: "s", Bucket: "b"}
	if !s.Ready() {
		t.Fatal("complete settings should be ready")
	}
	s.SecretKey = ""
	if s.Ready() {
		t.Fatal("no secret key: not ready")
	}
}

// Download links are signed offline (fixed region), for the public address
// when one is set, and make the browser save the file under its own name.
func TestDownloadURLAndKey(t *testing.T) {
	c, err := New(Settings{Endpoint: "10.0.122.76:9000", PublicURL: "https://s3.example.gov.br", AccessKey: "ak", SecretKey: "sk", Bucket: "ops", Prefix: DefaultPrefix})
	if err != nil {
		t.Fatal(err)
	}
	key := c.BackupKey("vm-web-01", 42, "tar.zst", time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC))
	if key != "bridge/os-backups/vm-web-01/20261112-093000-run42.tar.zst" {
		t.Fatalf("key = %q", key)
	}
	raw, err := c.DownloadURL(context.Background(), key, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	if u.Scheme != "https" || u.Host != "s3.example.gov.br" || !strings.HasSuffix(u.Path, "/ops/"+key) || u.Query().Get("X-Amz-Expires") != "3600" ||
		!strings.Contains(u.Query().Get("response-content-disposition"), "20261112-093000-run42.tar.zst") {
		t.Fatalf("url = %s", raw)
	}

	// No public address: signed for the endpoint Bridge uses.
	c, _ = New(Settings{Endpoint: "10.0.122.76:9000", AccessKey: "ak", SecretKey: "sk", Bucket: "ops"})
	raw, _ = c.DownloadURL(context.Background(), key, time.Hour)
	if u, _ := url.Parse(raw); u.Host != "10.0.122.76:9000" || u.Scheme != "http" {
		t.Fatalf("url = %s", raw)
	}
	if _, err := New(Settings{Endpoint: "e:9000", PublicURL: "::bad", AccessKey: "a", SecretKey: "s"}); err == nil {
		t.Fatal("a bad public URL must error")
	}
}
