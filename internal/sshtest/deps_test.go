package sshtest

import (
	"strings"
	"testing"
)

const phpDebianProbe = `@@os
ID=debian
VERSION_ID="12"
VERSION_CODENAME=bookworm
@@rt:php
PHP 7.4.33 (cli) (built: Nov 15 2022 06:03:30) ( NTS )
Copyright (c) The PHP Group
@@rt:openssl
OpenSSL 3.0.11 19 Sep 2023 (Library: OpenSSL 3.0.11 19 Sep 2023)
@@pkg
dpkg
ii 	libssl3	3.0.11-1~deb12u2	openssl	3.0.11-1~deb12u2
ii 	bash	5.2.15-2+b2	bash	5.2.15-2+b2
rc 	oldpkg	1.0	oldpkg	1.0
ii 	evil$(rm -rf /)	1.0	x	1.0
`

func TestParseDepProbe_DebianPHPImage(t *testing.T) {
	s := parseDepProbe(phpDebianProbe)
	if s.OS != (OSRelease{ID: "debian", VersionID: "12", Codename: "bookworm"}) {
		t.Fatalf("os = %+v", s.OS)
	}
	want := map[string]string{"php": "7.4.33", "openssl": "3.0.11"}
	if len(s.Runtimes) != 2 {
		t.Fatalf("runtimes = %+v", s.Runtimes)
	}
	for _, r := range s.Runtimes {
		if want[r.Name] != r.Version {
			t.Errorf("runtime %s = %s, want %s", r.Name, r.Version, want[r.Name])
		}
	}
	if s.PkgManager != "dpkg" || len(s.Packages) != 2 {
		t.Fatalf("packages = %s %+v; want 2 installed (rc and hostile names dropped)", s.PkgManager, s.Packages)
	}
	if p := s.Packages[0]; p.Name != "libssl3" || p.SrcName != "openssl" || p.SrcVersion != "3.0.11-1~deb12u2" {
		t.Fatalf("libssl3 = %+v", p)
	}
}

func TestParseDepProbe_AlpineNodeAndMariaDB(t *testing.T) {
	s := parseDepProbe(`@@os
ID=alpine
VERSION_ID=3.19.1
@@rt:node
v18.19.0
@@rt:mysql
mysql  Ver 15.1 Distrib 10.11.6-MariaDB, for Linux (x86_64)
@@pkg
apk
ii	musl	1.2.4_git20230717-r4	musl	1.2.4_git20230717-r4
ii	libcrypto3	3.1.4-r5	openssl	3.1.4-r5
`)
	if s.OS.ID != "alpine" || s.OS.VersionID != "3.19.1" {
		t.Fatalf("os = %+v", s.OS)
	}
	got := map[string]string{}
	for _, r := range s.Runtimes {
		got[r.Name] = r.Version
	}
	if got["node"] != "18.19.0" || got["mariadb"] != "10.11.6" || got["mysql"] != "" {
		t.Fatalf("runtimes = %+v", got)
	}
	if s.PkgManager != "apk" || len(s.Packages) != 2 || s.Packages[1].SrcName != "openssl" {
		t.Fatalf("packages = %+v", s.Packages)
	}
}

func TestParseDepProbe_RPMAndHostRuntimes(t *testing.T) {
	s := parseDepProbe(`@@os
ID="rocky"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.3"
@@rt:nginx
nginx version: nginx/1.20.1
@@rt:java
openjdk version "17.0.9" 2023-10-17
@@rt:postgresql
psql (PostgreSQL) 15.5
@@rt:redis
Redis server v=7.2.3 sha=00000000:0 malloc=jemalloc-5.3.0 bits=64 build=1
@@pkg
rpm
ii	openssl	1:3.0.7-24.el9	openssl-3.0.7-24.el9.src.rpm
ii	nginx	0:1.20.1-14.el9_2.1	nginx-1.20.1-14.el9_2.1.src.rpm
`)
	if s.OS.ID != "rocky" || s.OS.IDLike != "rhel centos fedora" {
		t.Fatalf("os = %+v", s.OS)
	}
	got := map[string]string{}
	for _, r := range s.Runtimes {
		got[r.Name] = r.Version
	}
	if got["nginx"] != "1.20.1" || got["java"] != "17.0.9" || got["postgresql"] != "15.5" || got["redis"] != "7.2.3" {
		t.Fatalf("runtimes = %+v", got)
	}
	if s.Packages[0].Version != "1:3.0.7-24.el9" || s.Packages[0].SrcName != "openssl" {
		t.Fatalf("openssl rpm = %+v", s.Packages[0])
	}
	if s.Packages[1].Version != "1.20.1-14.el9_2.1" || s.Packages[1].SrcName != "nginx" {
		t.Fatalf("nginx rpm = %+v (epoch 0 dropped)", s.Packages[1])
	}
}

func TestParseDepProbe_HostileAndHuge(t *testing.T) {
	var b strings.Builder
	b.WriteString("@@rt:php\nPHP $(reboot) (cli)\n@@pkg\ndpkg\n")
	for i := 0; i < maxDepPackages+50; i++ {
		b.WriteString("ii \tpkg")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(strings.Repeat("y", i/26))
		b.WriteString("\t1.0\t\t\n")
	}
	s := parseDepProbe(b.String())
	if len(s.Runtimes) != 0 {
		t.Fatalf("hostile php line parsed: %+v", s.Runtimes)
	}
	if len(s.Packages) > maxDepPackages {
		t.Fatalf("packages = %d; cap %d", len(s.Packages), maxDepPackages)
	}
	if p := s.Packages[0]; p.SrcName != p.Name || p.SrcVersion != "1.0" {
		t.Fatalf("missing source falls back to the binary: %+v", p)
	}
}

func TestParseImageEnv_KeepsOnlyVersions(t *testing.T) {
	env := parseImageEnv(`sha256:aaa
PHP_VERSION=7.4.33
DB_PASSWORD=hunter2
PATH=/usr/local/bin:/usr/bin
NGINX_VERSION=1.25.3
org.opencontainers.image.version=1.2.0
WEIRD_VERSION=$(id)
sha256:bbb
NODE_VERSION=18.19.0
`)
	a := env["sha256:aaa"]
	if len(a) != 3 || a["PHP_VERSION"] != "7.4.33" || a["NGINX_VERSION"] != "1.25.3" || a[ociVersion] != "1.2.0" {
		t.Fatalf("image aaa = %+v", a)
	}
	if _, leaked := a["DB_PASSWORD"]; leaked {
		t.Fatal("secret env leaked")
	}
	if env["sha256:bbb"]["NODE_VERSION"] != "18.19.0" {
		t.Fatalf("image bbb = %+v", env["sha256:bbb"])
	}
}

func TestPickProbeTargets(t *testing.T) {
	cs := []runningContainer{
		{ID: "aaaaaaaaaaaa", ImageID: "sha256:1", Up: true},
		{ID: "bbbbbbbbbbbb", ImageID: "sha256:1", Up: true}, // same image: skipped
		{ID: "cccccccccccc", ImageID: "sha256:2", Up: false}, // exited
		{ID: "not-an-id;rm", ImageID: "sha256:3", Up: true}, // invalid id
		{ID: "dddddddddddd", ImageID: "sha256:4", Up: true},
	}
	got := pickProbeTargets(cs)
	if len(got) != 2 || got[0].ID != "aaaaaaaaaaaa" || got[1].ID != "dddddddddddd" {
		t.Fatalf("targets = %+v", got)
	}
	var many []runningContainer
	for i := 0; i < maxProbeImages+5; i++ {
		id := strings.Repeat(string(rune('a'+i%6)), 11) + string(rune('0'+i%10))
		many = append(many, runningContainer{ID: id, ImageID: "sha256:" + id, Up: true})
	}
	if n := len(pickProbeTargets(many)); n != maxProbeImages {
		t.Fatalf("cap = %d, want %d", n, maxProbeImages)
	}
}

func TestSplitExecLoopAndScript(t *testing.T) {
	out := splitExecLoop("@@CTR aaaaaaaaaaaa\n@@os\nID=debian\n\n@@CTR bbbbbbbbbbbb\n\n")
	if !strings.Contains(out["aaaaaaaaaaaa"], "ID=debian") || strings.Contains(out["bbbbbbbbbbbb"], "@@") {
		t.Fatalf("split = %+v", out)
	}
	s := depProbeScript([]string{"php*", "bad;name", "openssl"}, false)
	if !strings.Contains(s, "'php*'") || !strings.Contains(s, "'openssl'") || strings.Contains(s, "bad;name") {
		t.Fatalf("patterns not filtered: %s", s)
	}
	if strings.Contains(s, "java") {
		t.Fatal("java probed in a container script")
	}
	if !validContainerID("0123456789ab") || validContainerID("0123;rm -rf") || validContainerID("ABCDEF012345") {
		t.Fatal("validContainerID")
	}
}
