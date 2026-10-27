package service

import (
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

func byKey(rows []models.Dependency) map[string]models.Dependency {
	m := map[string]models.Dependency{}
	for _, r := range rows {
		m[r.Kind+":"+r.Name] = r
	}
	return m
}

func TestNormalize_OfficialNginxImage(t *testing.T) {
	// nginx's official image installs nginx from nginx.org on Debian and
	// says so in NGINX_VERSION: NVD for nginx, no Debian advisories for it.
	svc := int64(7)
	rows := byKey(NormalizeDependencies(sshtest.DepScope{
		Container: "web", Image: "nginx:1.25",
		OS:       sshtest.OSRelease{ID: "debian", VersionID: "12"},
		Runtimes: []sshtest.RuntimeVersion{{Name: "nginx", Version: "1.25.3"}, {Name: "openssl", Version: "3.0.11"}},
		Packages: []sshtest.DepPackage{
			{Name: "nginx", Version: "1.25.3-1~bookworm", SrcName: "nginx", SrcVersion: "1.25.3-1~bookworm"},
			{Name: "libssl3", Version: "3.0.11-1~deb12u2", SrcName: "openssl", SrcVersion: "3.0.11-1~deb12u2"},
		},
		ImageEnv: map[string]string{"NGINX_VERSION": "1.25.3", "NJS_VERSION": "0.8.2"},
	}, &svc))

	if r := rows["os:debian"]; r.Version != "12" || r.EOLProduct != "debian" || r.Lookup != "" {
		t.Fatalf("os = %+v", r)
	}
	if r := rows["runtime:nginx"]; r.VersionSource != models.DepSourceImageEnv || r.Lookup != models.DepLookupNVD ||
		r.LookupName != "f5:nginx,nginx:nginx" || r.LookupVersion != "1.25.3" || *r.ServiceID != 7 || r.Container != "web" {
		t.Fatalf("nginx runtime = %+v", r)
	}
	if r := rows["package:nginx"]; r.Lookup != "" {
		t.Fatalf("nginx package must not go to OSV: %+v", r)
	}
	// openssl: probed and shipped by Debian → the package carries the CVEs.
	if r := rows["package:libssl3"]; r.Lookup != models.DepLookupOSV || r.Ecosystem != "Debian:12" || r.LookupName != "openssl" {
		t.Fatalf("libssl3 = %+v", r)
	}
	if r := rows["runtime:openssl"]; r.Lookup != "" || r.EOLProduct != "openssl" {
		t.Fatalf("openssl runtime must be display + EOL only: %+v", r)
	}
}

func TestNormalize_HostDistroNginxAndSourcePHP(t *testing.T) {
	rows := byKey(NormalizeDependencies(sshtest.DepScope{
		OS:       sshtest.OSRelease{ID: "ubuntu", VersionID: "22.04"},
		Runtimes: []sshtest.RuntimeVersion{{Name: "nginx", Version: "1.18.0"}, {Name: "php", Version: "7.4.33"}},
		Packages: []sshtest.DepPackage{{Name: "nginx-core", Version: "1.18.0-6ubuntu14.4", SrcName: "nginx", SrcVersion: "1.18.0-6ubuntu14.4"}},
	}, nil))
	if r := rows["runtime:nginx"]; r.Lookup != "" {
		t.Fatalf("host nginx from apt goes to OSV via its package, not NVD: %+v", r)
	}
	if r := rows["package:nginx-core"]; r.Lookup != models.DepLookupOSV || r.Ecosystem != "Ubuntu:22.04" {
		t.Fatalf("nginx-core = %+v", r)
	}
	// php has no distro package here: built from source → NVD.
	if r := rows["runtime:php"]; r.Lookup != models.DepLookupNVD || r.LookupName != "php:php" || r.EOLProduct != "php" {
		t.Fatalf("php = %+v", r)
	}
}

func TestNormalize_PHPImageEnvAndUnknownDistro(t *testing.T) {
	rows := byKey(NormalizeDependencies(sshtest.DepScope{
		OS:       sshtest.OSRelease{ID: "wolfi", VersionID: "20230201"},
		Runtimes: []sshtest.RuntimeVersion{{Name: "php", Version: "7.4.33"}},
		Packages: []sshtest.DepPackage{{Name: "busybox", Version: "1.36", SrcName: "busybox", SrcVersion: "1.36"}},
		ImageEnv: map[string]string{"PHP_VERSION": "7.4.33"},
	}, nil))
	if len(rows) != 3 {
		t.Fatalf("rows = %+v; probe and env with the same version collapse into one", rows)
	}
	if r := rows["runtime:php"]; r.VersionSource != models.DepSourceImageEnv || r.Lookup != models.DepLookupNVD {
		t.Fatalf("php = %+v", r)
	}
	if r := rows["package:busybox"]; r.Lookup != "" {
		t.Fatalf("unknown distro: packages are shown only: %+v", r)
	}
	if r := rows["os:wolfi"]; r.EOLProduct != "" {
		t.Fatalf("unknown os has no EOL slug: %+v", r)
	}
}

// A container runs the host's kernel: kernel packages in its image (headers
// pulled in as build deps) carry every kernel CVE and none applies — in prod
// linux-libc-dev alone was 35% of all matches. On the host they stay.
func TestNormalize_KernelPackagesOnlyOnTheHost(t *testing.T) {
	pkgs := []sshtest.DepPackage{
		{Name: "linux-libc-dev", Version: "6.1.106-3", SrcName: "linux", SrcVersion: "6.1.106-3"},
		{Name: "linux-headers-6.1.0-25-amd64", Version: "6.1.106-3", SrcName: "linux", SrcVersion: "6.1.106-3"},
		{Name: "linux-image-6.1.0-25-amd64", Version: "6.1.106-3", SrcName: "linux-signed-amd64", SrcVersion: "6.1.106+3"},
		{Name: "libssl3", Version: "3.0.11-1~deb12u2", SrcName: "openssl", SrcVersion: "3.0.11-1~deb12u2"},
		{Name: "linuxdoc-tools", Version: "0.9.82-1", SrcName: "linuxdoc-tools", SrcVersion: "0.9.82-1"},
	}
	os := sshtest.OSRelease{ID: "debian", VersionID: "12"}
	ctr := byKey(NormalizeDependencies(sshtest.DepScope{Container: "app", OS: os, Packages: pkgs}, nil))
	for _, n := range []string{"linux-libc-dev", "linux-headers-6.1.0-25-amd64", "linux-image-6.1.0-25-amd64"} {
		if _, ok := ctr["package:"+n]; ok {
			t.Errorf("container kept kernel package %s", n)
		}
	}
	if _, ok := ctr["package:libssl3"]; !ok {
		t.Error("container lost libssl3")
	}
	if _, ok := ctr["package:linuxdoc-tools"]; !ok {
		t.Error("linuxdoc-tools is not a kernel package")
	}
	host := byKey(NormalizeDependencies(sshtest.DepScope{OS: os, Packages: pkgs}, nil))
	if d, ok := host["package:linux-image-6.1.0-25-amd64"]; !ok || d.Lookup != models.DepLookupOSV {
		t.Errorf("host kernel package = %+v %v; want kept with its OSV lookup", d, ok)
	}
}

func TestOSVEcosystemAndCleanUpstream(t *testing.T) {
	for in, want := range map[sshtest.OSRelease]string{
		{ID: "debian", VersionID: "12"}:      "Debian:12",
		{ID: "debian", VersionID: "11.9"}:    "Debian:11",
		{ID: "ubuntu", VersionID: "22.04"}:   "Ubuntu:22.04",
		{ID: "alpine", VersionID: "3.19.1"}:  "Alpine:v3.19",
		{ID: "rocky", VersionID: "9.3"}:      "Rocky Linux:9",
		{ID: "almalinux", VersionID: "8.10"}: "AlmaLinux:8",
		{ID: "rhel", VersionID: "9.4"}:       "",
		{ID: "debian"}:                       "",
	} {
		if got := osvEcosystem(in); got != want {
			t.Errorf("osvEcosystem(%+v) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"16.1-1.pgdg120+1": "16.1", "1.1.1w": "1.1.1w", "7.4.33": "7.4.33", "latest": ""} {
		if got := cleanUpstream(in); got != want {
			t.Errorf("cleanUpstream(%q) = %q, want %q", in, got, want)
		}
	}
}
