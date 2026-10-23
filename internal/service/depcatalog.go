package service

import (
	"regexp"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// depProduct is one upstream component Bridge knows how to name for NVD
// (CPE vendor:product), endoflife.date (slug) and image env (*_VERSION),
// and which distro source packages ship it.
type depProduct struct {
	Name    string
	CPEs    []string // "vendor:product"; nil = no CVE lookup
	EOL     string   // endoflife.date slug; "" = none
	EnvKeys []string
	PkgSrc  *regexp.Regexp // distro source package names of this product
}

// depProducts: the runtimes the probe script reads (sshtest.runtimeRes).
// ponytail: java is shown only — its CPEs (oracle:jdk with update_N
// versions) are too messy to match without false hits.
var depProducts = []depProduct{
	{Name: "php", CPEs: []string{"php:php"}, EOL: "php", EnvKeys: []string{"PHP_VERSION"}, PkgSrc: regexp.MustCompile(`^php(\d+(\.\d+)?)?$`)},
	{Name: "nginx", CPEs: []string{"f5:nginx", "nginx:nginx"}, EOL: "nginx", EnvKeys: []string{"NGINX_VERSION"}, PkgSrc: regexp.MustCompile(`^nginx$`)},
	{Name: "openssl", CPEs: []string{"openssl:openssl"}, EOL: "openssl", EnvKeys: []string{"OPENSSL_VERSION"}, PkgSrc: regexp.MustCompile(`^openssl(\d+)?$`)},
	{Name: "node", CPEs: []string{"nodejs:node.js"}, EOL: "nodejs", EnvKeys: []string{"NODE_VERSION"}, PkgSrc: regexp.MustCompile(`^nodejs$`)},
	{Name: "python", CPEs: []string{"python:python"}, EOL: "python", EnvKeys: []string{"PYTHON_VERSION"}, PkgSrc: regexp.MustCompile(`^python3(\.\d+)?$`)},
	{Name: "apache", CPEs: []string{"apache:http_server"}, EOL: "apache-http-server", EnvKeys: []string{"HTTPD_VERSION"}, PkgSrc: regexp.MustCompile(`^(apache2|httpd)$`)},
	{Name: "postgresql", CPEs: []string{"postgresql:postgresql"}, EOL: "postgresql", EnvKeys: []string{"PG_VERSION"}, PkgSrc: regexp.MustCompile(`^postgresql(-?\d+)?$`)},
	{Name: "mysql", CPEs: []string{"oracle:mysql"}, EOL: "mysql", EnvKeys: []string{"MYSQL_VERSION"}, PkgSrc: regexp.MustCompile(`^mysql`)},
	{Name: "mariadb", CPEs: []string{"mariadb:mariadb"}, EOL: "mariadb", EnvKeys: []string{"MARIADB_VERSION"}, PkgSrc: regexp.MustCompile(`^mariadb`)},
	{Name: "redis", CPEs: []string{"redis:redis"}, EOL: "redis", EnvKeys: []string{"REDIS_VERSION"}, PkgSrc: regexp.MustCompile(`^redis$`)},
	{Name: "java", EnvKeys: []string{"JAVA_VERSION"}, PkgSrc: regexp.MustCompile(`^openjdk-`)},
}

// osEOL maps an os-release ID to its endoflife.date slug.
var osEOL = map[string]string{
	"debian": "debian", "ubuntu": "ubuntu", "alpine": "alpine-linux", "rhel": "rhel",
	"rocky": "rocky-linux", "almalinux": "almalinux", "centos": "centos",
}

// osvEcosystem names the OSV ecosystem of a distro release ("" = OSV can't
// answer for it). ponytail: RHEL is left out — OSV splits it per stream
// (Red Hat:enterprise_linux:9::baseos); add it when a RHEL host shows up.
func osvEcosystem(o sshtest.OSRelease) string {
	major := strings.SplitN(o.VersionID, ".", 2)[0]
	switch o.ID {
	case "debian":
		if major != "" {
			return "Debian:" + major
		}
	case "ubuntu":
		if o.VersionID != "" {
			return "Ubuntu:" + o.VersionID
		}
	case "alpine":
		if p := strings.SplitN(o.VersionID, ".", 3); len(p) >= 2 {
			return "Alpine:v" + p[0] + "." + p[1]
		}
	case "rocky":
		if major != "" {
			return "Rocky Linux:" + major
		}
	case "almalinux":
		if major != "" {
			return "AlmaLinux:" + major
		}
	}
	return ""
}

var upstreamRe = regexp.MustCompile(`^\d+(\.\d+)*[a-z]?`)

// cleanUpstream strips a packaging suffix from an upstream version
// ("16.1-1.pgdg120+1" → "16.1"); "" when it doesn't start with a number.
func cleanUpstream(v string) string { return upstreamRe.FindString(v) }

// NormalizeDependencies turns one scanned scope into inventory rows and
// decides, per row, where its advisories come from (one source per row, so
// a CVE is never reported twice):
//
//  1. the OS → endoflife only;
//  2. a product whose version comes from the image env (an official image
//     built from source) → NVD, and this scope's distro packages of that
//     product get no lookup (distro advisories don't apply to it);
//  3. a distro package of a product, when the distro is known → OSV, and the
//     probed runtime row is shown with its EOL only;
//  4. a probed runtime with no distro package (built from source) → NVD;
//  5. every other package → OSV on its source package, when the distro is
//     known; else shown only.
func NormalizeDependencies(s sshtest.DepScope, serviceID *int64) []models.Dependency {
	eco := osvEcosystem(s.OS)
	base := models.Dependency{ServiceID: serviceID, Container: s.Container, Image: s.Image, ImageID: s.ImageID}
	var out []models.Dependency
	add := func(d models.Dependency) {
		d.ServiceID, d.Container, d.Image, d.ImageID = base.ServiceID, base.Container, base.Image, base.ImageID
		out = append(out, d)
	}
	if s.OS.ID != "" && s.OS.VersionID != "" {
		add(models.Dependency{Kind: models.DepKindOS, Name: s.OS.ID, Version: s.OS.VersionID,
			VersionSource: models.DepSourceOSRelease, EOLProduct: osEOL[s.OS.ID]})
	}

	probed := map[string]string{}
	for _, r := range s.Runtimes {
		probed[r.Name] = r.Version
	}
	pkgOf := func(p depProduct) bool {
		if eco == "" {
			return false
		}
		for _, pk := range s.Packages {
			if p.PkgSrc.MatchString(pk.SrcName) {
				return true
			}
		}
		return false
	}
	suppressed := map[string]bool{} // source packages whose product came from the image env
	for _, p := range depProducts {
		envVer := ""
		for _, k := range p.EnvKeys {
			if v := cleanUpstream(s.ImageEnv[k]); v != "" {
				envVer = v
				break
			}
		}
		probeVer := probed[p.Name]
		nvd := func(d models.Dependency) models.Dependency {
			if len(p.CPEs) > 0 {
				d.Lookup, d.LookupName, d.LookupVersion = models.DepLookupNVD, strings.Join(p.CPEs, ","), cleanUpstream(d.Version)
			}
			return d
		}
		row := models.Dependency{Kind: models.DepKindRuntime, Name: p.Name, EOLProduct: p.EOL}
		switch {
		case envVer != "":
			for _, pk := range s.Packages {
				if p.PkgSrc.MatchString(pk.SrcName) {
					suppressed[pk.Name] = true
				}
			}
			row.Version, row.VersionSource = envVer, models.DepSourceImageEnv
			add(nvd(row))
			if probeVer != "" && cleanUpstream(probeVer) != envVer {
				add(models.Dependency{Kind: models.DepKindRuntime, Name: p.Name, Version: probeVer,
					VersionSource: models.DepSourceProbe, EOLProduct: p.EOL})
			}
		case probeVer != "" && pkgOf(p):
			row.Version, row.VersionSource = probeVer, models.DepSourceProbe
			add(row) // the distro package carries the CVEs (rule 3)
		case probeVer != "":
			row.Version, row.VersionSource = probeVer, models.DepSourceProbe
			add(nvd(row))
		}
	}

	for _, pk := range s.Packages {
		d := models.Dependency{Kind: models.DepKindPackage, Name: pk.Name, Version: pk.Version, VersionSource: models.DepSourcePackage}
		if eco != "" && !suppressed[pk.Name] {
			d.Lookup, d.Ecosystem, d.LookupName, d.LookupVersion = models.DepLookupOSV, eco, pk.SrcName, pk.SrcVersion
		}
		add(d)
	}
	return out
}
