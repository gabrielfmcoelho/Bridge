package minio

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7/pkg/lifecycle"
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

// Bridge's rule is added, updated in place or removed; the bucket's other
// rules (the datalake's) are never touched.
func TestWithBackupRule(t *testing.T) {
	theirs := lifecycle.Rule{ID: "datalake-tmp", Status: "Enabled", RuleFilter: lifecycle.Filter{Prefix: "tmp/"}, Expiration: lifecycle.Expiration{Days: 7}}

	rules := withBackupRule([]lifecycle.Rule{theirs}, "/bridge/os-backups/", 30)
	if len(rules) != 2 || rules[0].ID != "datalake-tmp" || rules[0].Expiration.Days != 7 {
		t.Fatalf("their rule changed: %+v", rules)
	}
	mine := rules[1]
	if mine.ID != lifecycleRuleID || mine.Status != "Enabled" || mine.RuleFilter.Prefix != "bridge/os-backups/" ||
		mine.Expiration.Days != 30 || mine.AbortIncompleteMultipartUpload.DaysAfterInitiation != 1 {
		t.Fatalf("bridge rule = %+v", mine)
	}

	// A new retention replaces Bridge's rule, never duplicates it.
	rules = withBackupRule(rules, "bridge/os-backups", 7)
	if len(rules) != 2 || rules[1].Expiration.Days != 7 {
		t.Fatalf("updated = %+v", rules)
	}
	// 0 = keep forever: Bridge's rule goes, theirs stays.
	rules = withBackupRule(rules, "bridge/os-backups", 0)
	if len(rules) != 1 || rules[0].ID != "datalake-tmp" {
		t.Fatalf("removed = %+v", rules)
	}
}
