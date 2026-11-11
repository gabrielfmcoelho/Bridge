// Package minio is Bridge's S3 store (the datalake's MinIO): where OS update
// runs put each host's backup before touching it.
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

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// Default backup paths: the whole root; pseudo and network filesystems are
// left out by the backup script itself, these on top of them.
const (
	DefaultInclude = "/"
	DefaultExclude = "/tmp /var/tmp /var/cache /var/lib/docker /var/lib/containerd /swapfile /swap.img /var/lib/bridge-osupdate /lost+found"
	DefaultPrefix  = "bridge/os-backups"
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
	Prefix    string
	Include   []string // paths backed up (osbackup_include)
	Exclude   []string // paths left out (osbackup_exclude)
}

// LoadSettings reads the minio_* and osbackup_* settings.
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
		Prefix:    strings.Trim(get("osbackup_prefix"), "/"),
		Include:   strings.Fields(get("osbackup_include")),
		Exclude:   strings.Fields(get("osbackup_exclude")),
	}
	if s.Prefix == "" {
		s.Prefix = DefaultPrefix
	}
	if len(s.Include) == 0 {
		s.Include = strings.Fields(DefaultInclude)
	}
	if get("osbackup_exclude") == "" {
		s.Exclude = strings.Fields(DefaultExclude)
	}
	secret, _, err := store.NewAppSecretRepo(db).Reveal(context.Background(), enc, "minio_secret_key")
	if err != nil {
		return s, err
	}
	s.SecretKey = secret
	return s, nil
}

// Ready says whether backups can run: on, with an endpoint, keys and a bucket.
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

// BackupKey is where a host's backup of a run goes.
func (c *Client) BackupKey(hostSlug string, runID int64, ext string, now time.Time) string {
	return fmt.Sprintf("%s/%s/%s-run%d.%s", c.s.Prefix, hostSlug, now.UTC().Format("20060102-150405"), runID, ext)
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
