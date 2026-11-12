// Package minio is Bridge's S3 connection (the datalake's MinIO): only the
// connection lives here. What a feature stores and for how long is the
// feature's (host backups: service.BackupDefaults).
package minio

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	mc "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Settings is the minio_* group (app_settings; the secret key encrypted).
type Settings struct {
	Enabled   bool
	Endpoint  string // host:port Bridge reaches, e.g. 10.0.122.76:9000
	PublicURL string // optional, what browsers reach for download links (scheme://host:port)
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// LoadSettings reads the minio_* settings.
func LoadSettings(db *sql.DB, enc *database.Encryptor) (Settings, error) {
	repo := store.NewAppSettingsRepo(db)
	get := func(k string) string { return strings.TrimSpace(repo.Value(context.Background(), k)) }
	s := Settings{
		Enabled:   get("minio_enabled") == "true",
		Endpoint:  hostPort(get("minio_endpoint")),
		PublicURL: strings.TrimRight(get("minio_public_url"), "/"),
		AccessKey: get("minio_access_key"),
		Bucket:    get("minio_bucket"),
		UseSSL:    get("minio_use_ssl") == "true",
	}
	secret, _, err := store.NewAppSecretRepo(db).Reveal(context.Background(), enc, "minio_secret_key")
	if err != nil {
		return s, err
	}
	s.SecretKey = secret
	return s, nil
}

// Ready says whether the connection is usable: on, with an endpoint, keys and a bucket.
func (s Settings) Ready() bool {
	return s.Enabled && s.Endpoint != "" && s.AccessKey != "" && s.SecretKey != "" && s.Bucket != ""
}

// hostPort accepts "host:port" or a URL and returns host:port.
func hostPort(v string) string {
	if u, err := url.Parse(v); err == nil && u.Host != "" {
		return u.Host
	}
	return strings.TrimRight(v, "/")
}

// Client wraps a MinIO client on the settings' bucket.
type Client struct {
	s      Settings
	c      *mc.Client
	public *mc.Client // signs download links for PublicURL (nil = use c)
}

// region is fixed so presigning never asks the server for the bucket location.
const region = "us-east-1"

func New(s Settings) (*Client, error) {
	c, err := mc.New(s.Endpoint, &mc.Options{Creds: credentials.NewStaticV4(s.AccessKey, s.SecretKey, ""), Secure: s.UseSSL, Region: region})
	if err != nil {
		return nil, err
	}
	out := &Client{s: s, c: c}
	if s.PublicURL != "" {
		u, err := url.Parse(s.PublicURL)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("invalid public URL %q", s.PublicURL)
		}
		if out.public, err = mc.New(u.Host, &mc.Options{Creds: credentials.NewStaticV4(s.AccessKey, s.SecretKey, ""), Secure: u.Scheme == "https", Region: region}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Check confirms the credentials and that the bucket exists.
func (c *Client) Check(ctx context.Context) error {
	ok, err := c.c.BucketExists(ctx, c.s.Bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q does not exist", c.s.Bucket)
	}
	return nil
}

// partSize bounds memory per upload (one part buffered) and the object size
// (10 000 parts → 640 GiB).
const partSize = 64 << 20

// Put streams r (size unknown) to key and returns the stored size.
func (c *Client) Put(ctx context.Context, key string, r io.Reader, contentType string) (int64, error) {
	info, err := c.c.PutObject(ctx, c.s.Bucket, key, r, -1, mc.PutObjectOptions{ContentType: contentType, PartSize: partSize, NumThreads: 1})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

// Exists says whether key is in the bucket (false, nil when it isn't).
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if _, err := c.c.StatObject(ctx, c.s.Bucket, key, mc.StatObjectOptions{}); err != nil {
		if mc.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete removes key (no error when it's already gone).
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.c.RemoveObject(ctx, c.s.Bucket, key, mc.RemoveObjectOptions{})
}

// DownloadURL is a link to key, valid for ttl, signed for the public address
// when one is set.
func (c *Client) DownloadURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	signer := c.c
	if c.public != nil {
		signer = c.public
	}
	name := key[strings.LastIndex(key, "/")+1:]
	q := url.Values{"response-content-disposition": {`attachment; filename="` + name + `"`}}
	u, err := signer.PresignedGetObject(ctx, c.s.Bucket, key, ttl, q)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// withRule returns rules with the rule id set to expire objects under prefix
// after days (and drop the multipart uploads a cut upload left behind after
// a day); days 0 removes it. Every other rule is kept as is.
func withRule(rules []lifecycle.Rule, id, prefix string, days int) []lifecycle.Rule {
	out := make([]lifecycle.Rule, 0, len(rules)+1)
	for _, r := range rules {
		if r.ID != id {
			out = append(out, r)
		}
	}
	if days > 0 {
		out = append(out, lifecycle.Rule{
			ID:                             id,
			Status:                         "Enabled",
			RuleFilter:                     lifecycle.Filter{Prefix: strings.Trim(prefix, "/") + "/"},
			Expiration:                     lifecycle.Expiration{Days: lifecycle.ExpirationDays(days)},
			AbortIncompleteMultipartUpload: lifecycle.AbortIncompleteMultipartUpload{DaysAfterInitiation: 1},
		})
	}
	return out
}

// EnsureRule puts the lifecycle rule id (prefix, expire after days; 0
// removes it) on the bucket, leaving the bucket's other rules alone (the
// datalake's bucket may have its own). Idempotent.
func (c *Client) EnsureRule(ctx context.Context, id, prefix string, days int) error {
	cfg, err := c.c.GetBucketLifecycle(ctx, c.s.Bucket)
	if err != nil {
		if mc.ToErrorResponse(err).Code != "NoSuchLifecycleConfiguration" {
			return err
		}
		cfg = lifecycle.NewConfiguration()
	}
	cfg.Rules = withRule(cfg.Rules, id, prefix, days)
	return c.c.SetBucketLifecycle(ctx, c.s.Bucket, cfg)
}

// FetchURL is a link to key, valid for ttl, signed for the endpoint Bridge
// uses (the hosts reach MinIO there too): what a restore downloads from.
func (c *Client) FetchURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := c.c.PresignedGetObject(ctx, c.s.Bucket, key, ttl, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
