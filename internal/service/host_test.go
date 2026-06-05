package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

func newHostService(t *testing.T) (*service.HostService, *database.DB) {
	t.Helper()
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return service.NewHostService(d), d
}

func TestHostService_GetComposesRelations(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	hostRepo := store.NewHostRepo(d.SQL)

	h := &models.Host{Nickname: "web", OficialSlug: "web-01", Hostname: "10.0.0.1", User: "root", Situacao: "active"}
	if err := hostRepo.Create(ctx, h); err != nil {
		t.Fatalf("create host: %v", err)
	}
	store.NewTagRepo(d.SQL).Set(ctx, "host", h.ID, []string{"prod"})

	// Link a service and a dns record to the host.
	sr, _ := d.SQL.Exec(`INSERT INTO services (nickname) VALUES ('svc')`)
	svcID, _ := sr.LastInsertId()
	d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svcID, h.ID)
	dr, _ := d.SQL.Exec(`INSERT INTO dns_records (domain) VALUES ('web.example.com')`)
	dnsID, _ := dr.LastInsertId()
	d.SQL.Exec(`INSERT INTO dns_host_links (dns_id, host_id) VALUES (?, ?)`, dnsID, h.ID)

	// A scan so last_scan is populated.
	store.NewHostScanRepo(d.SQL).Create(ctx, h.ID, `{"cpu":"4","services":[],"containers":[]}`)

	detail, err := svc.Get(ctx, "web-01")
	if err != nil || detail == nil {
		t.Fatalf("get = %+v, %v", detail, err)
	}
	if detail.Host.OficialSlug != "web-01" {
		t.Fatalf("host = %+v", detail.Host)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "prod" {
		t.Fatalf("tags = %v", detail.Tags)
	}
	if len(detail.Services) != 1 || len(detail.DNSRecords) != 1 {
		t.Fatalf("relations: services=%d dns=%d", len(detail.Services), len(detail.DNSRecords))
	}
	if detail.LastScan == nil {
		t.Fatal("last_scan should be populated")
	}

	// Missing host → (nil, nil).
	if got, err := svc.Get(ctx, "nope"); got != nil || err != nil {
		t.Fatalf("get(missing) = %+v, %v; want nil,nil", got, err)
	}
}

func TestHostService_ListEnrichesAndComputesAlerts(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	hostRepo := store.NewHostRepo(d.SQL)

	h := &models.Host{Nickname: "box", OficialSlug: "box-01", Hostname: "h", User: "root", Situacao: "active"}
	if err := hostRepo.Create(ctx, h); err != nil {
		t.Fatalf("create: %v", err)
	}
	store.NewTagRepo(d.SQL).Set(ctx, "host", h.ID, []string{"edge"})

	// Link a service so ServicesCount is enriched.
	sr, _ := d.SQL.Exec(`INSERT INTO services (nickname) VALUES ('s1')`)
	svcID, _ := sr.LastInsertId()
	d.SQL.Exec(`INSERT INTO service_host_links (service_id, host_id) VALUES (?, ?)`, svcID, h.ID)

	// Scan data with critical CPU usage (90% ≥ default critical 80%).
	scan := `{"cpu":"4 cores","cpu_usage":"90%","ram":"8G","ram_percent":"50%","storage":"100G","disk_percent":"40%","services":[],"containers":["c1"]}`
	store.NewHostScanRepo(d.SQL).Create(ctx, h.ID, scan)

	items, err := svc.List(ctx, models.HostFilter{})
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %d items, %v", len(items), err)
	}
	it := items[0]
	if it.OficialSlug != "box-01" {
		t.Fatalf("item host = %+v", it.Host)
	}
	if len(it.Tags) != 1 || it.Tags[0] != "edge" {
		t.Fatalf("tags = %v", it.Tags)
	}
	if !it.HasScan || it.ScanRes == nil {
		t.Fatalf("scan enrichment missing: hasScan=%v res=%v", it.HasScan, it.ScanRes)
	}
	if it.ServicesCount != 1 {
		t.Fatalf("services_count = %d, want 1", it.ServicesCount)
	}
	if it.ContainersCount != 1 {
		t.Fatalf("containers_count = %d, want 1", it.ContainersCount)
	}
	// A critical CPU alert should have been computed.
	var foundCPUCritical bool
	for _, a := range it.Alerts {
		if a.Type == "resource_cpu" && a.Level == "critical" {
			foundCPUCritical = true
		}
	}
	if !foundCPUCritical {
		t.Fatalf("expected a critical resource_cpu alert, got %+v", it.Alerts)
	}
}

func TestHostService_CreatePersistsVaultAndRelations(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	// Vault secret writes attribute to a user (the system actor); seed one.
	d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin')`)

	// Seed an ssh_keys row with an encrypted private payload to link.
	pc, pn, _ := d.Encryptor.Encrypt("-----BEGIN KEY-----priv-----END KEY-----")
	k := &models.SSHKey{Name: "k1", CredentialType: "key", PrivKeyCiphertext: pc, PrivKeyNonce: pn}
	if err := store.NewSSHKeyRepo(d.SQL).Create(ctx, k); err != nil {
		t.Fatalf("seed ssh key: %v", err)
	}

	tags := []string{"t1"}
	w := &service.HostWrite{
		Host:     models.Host{Nickname: "web", OficialSlug: "web-01", Hostname: "h", User: "root", HasPassword: true},
		Password: "pw",
		Tags:     &tags,
		SSHKeyID: k.ID,
	}
	host, err := svc.Create(ctx, 0, w)
	if err != nil || host == nil || host.ID == 0 {
		t.Fatalf("create = %+v, %v", host, err)
	}

	// Vault holds the password.
	pw, ok, err := vault.HostGetPassword(ctx, d, host.ID)
	if err != nil || !ok || pw != "pw" {
		t.Fatalf("vault password = %q ok=%v err=%v", pw, ok, err)
	}
	// SSH key link flipped has_key and stored the key in the vault.
	reread, _ := store.NewHostRepo(d.SQL).GetByID(ctx, host.ID)
	if !reread.HasKey {
		t.Fatalf("has_key not set after link: %+v", reread)
	}
	if _, ok, _ := vault.HostGetSSHKey(ctx, d, host.ID); !ok {
		t.Fatal("vault ssh key not stored after link")
	}
	// Tags synced.
	if got, _ := store.NewTagRepo(d.SQL).Get(ctx, "host", host.ID); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("tags = %v", got)
	}
}

func TestHostService_CreateKeyLinkFailureIsWrapped(t *testing.T) {
	ctx := context.Background()
	svc, _ := newHostService(t)
	w := &service.HostWrite{
		Host:     models.Host{Nickname: "x", OficialSlug: "x-01"},
		SSHKeyID: 9999, // nonexistent → link fails
	}
	host, err := svc.Create(ctx, 0, w)
	if !errors.Is(err, service.ErrHostKeyLink) {
		t.Fatalf("err = %v, want ErrHostKeyLink", err)
	}
	// Host was still created (partial success).
	if host == nil || host.ID == 0 {
		t.Fatalf("host should be created despite link failure: %+v", host)
	}
}

func TestHostService_UpdateRotatesPasswordAndClearsKey(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	d.SQL.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('u','x','admin')`)

	// Create a host with a password + linked key.
	pc, pn, _ := d.Encryptor.Encrypt("priv")
	k := &models.SSHKey{Name: "k", CredentialType: "key", PrivKeyCiphertext: pc, PrivKeyNonce: pn}
	store.NewSSHKeyRepo(d.SQL).Create(ctx, k)
	cw := &service.HostWrite{Host: models.Host{Nickname: "h", OficialSlug: "h-01", HasPassword: true}, Password: "old", SSHKeyID: k.ID}
	host, err := svc.Create(ctx, 0, cw)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Rotate password + clear key.
	uw := &service.HostWrite{Host: *host, Password: "new", ClearKey: true}
	uw.Host.HasKey = false
	if _, err := svc.Update(ctx, 0, uw); err != nil {
		t.Fatalf("update: %v", err)
	}
	if pw, ok, _ := vault.HostGetPassword(ctx, d, host.ID); !ok || pw != "new" {
		t.Fatalf("password after rotate = %q ok=%v, want new", pw, ok)
	}
	if _, ok, _ := vault.HostGetSSHKey(ctx, d, host.ID); ok {
		t.Fatal("ssh key should be cleared from vault after ClearKey")
	}
}

func TestHostService_DeleteCascades(t *testing.T) {
	ctx := context.Background()
	svc, d := newHostService(t)
	repo := store.NewHostRepo(d.SQL)

	h := &models.Host{Nickname: "doomed", OficialSlug: "doomed-01"}
	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("create: %v", err)
	}
	store.NewTagRepo(d.SQL).Set(ctx, "host", h.ID, []string{"x"})

	if err := svc.Delete(ctx, models.ActorContext{UserID: 1, Role: "admin"}, h.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := repo.GetByID(ctx, h.ID); got != nil {
		t.Fatalf("host should be gone, got %+v", got)
	}
}
