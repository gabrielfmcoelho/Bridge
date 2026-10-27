package models

import "time"

// Dependency is one OS, runtime or package version the SSH scan found on a
// host (Container "") or inside one of its containers. Lookup is where its
// advisories come from: "osv" (distro package), "nvd" (upstream build) or ""
// (display only). Persistence: internal/store.DependencyRepo.
type Dependency struct {
	ID            int64      `json:"id"`
	HostID        int64      `json:"host_id"`
	HostNickname  string     `json:"host_nickname"`
	HostSlug      string     `json:"host_slug"`
	ServiceID     *int64     `json:"service_id"`
	Container     string     `json:"container"`
	Image         string     `json:"image"`
	ImageID       string     `json:"image_id"`
	Kind          string     `json:"kind"` // os | runtime | package
	Name          string     `json:"name"`
	Version       string     `json:"version"`
	VersionSource string     `json:"version_source"` // probe | package | image_env | os_release
	Lookup        string     `json:"lookup"`
	Ecosystem     string     `json:"ecosystem"`
	LookupName    string     `json:"lookup_name"`
	LookupVersion string     `json:"lookup_version"`
	EOLProduct    string     `json:"eol_product"`
	CVECount      int        `json:"cve_count"`
	MaxSeverity   string     `json:"max_severity"`
	EOL           bool       `json:"eol"`
	EOLDate       string     `json:"eol_date"`
	EvaluatedAt   *time.Time `json:"evaluated_at"`
	FirstSeenAt   time.Time  `json:"first_seen_at"`
	LastSeenAt    time.Time  `json:"last_seen_at"`
}

// Dependency kinds and version sources (CHECKs of migration v115).
const (
	DepKindOS      = "os"
	DepKindRuntime = "runtime"
	DepKindPackage = "package"

	DepSourceProbe     = "probe"
	DepSourcePackage   = "package"
	DepSourceImageEnv  = "image_env"
	DepSourceOSRelease = "os_release"

	DepLookupOSV = "osv"
	DepLookupNVD = "nvd"
)

// DependencyFilter narrows DependencyRepo.List. Empty fields are ignored.
type DependencyFilter struct {
	HostID    int64
	ServiceID int64
	Kinds     []string // default: os + runtime
	HasCVEs   bool
	EOL       bool
	Search    string // name, version, container, image, host
	Page      int
	PerPage   int
}

// DependencyGroup is one component version across every place it is
// installed (GET /api/vulnerabilities/dependencies/groups). CVECount and
// MaxSeverity are the worst of its installations (an OSV answer can differ
// per distro release); Instances is capped at 50.
type DependencyGroup struct {
	Kind          string               `json:"kind"`
	Name          string               `json:"name"`
	Version       string               `json:"version"`
	Hosts         int                  `json:"hosts"`
	Installs      int                  `json:"installs"`
	CVECount      int                  `json:"cve_count"`
	MaxSeverity   string               `json:"max_severity"`
	EOL           bool                 `json:"eol"`
	EOLDate       string               `json:"eol_date"`
	Evaluated     int                  `json:"evaluated"` // installations with an answer
	Lookup        string               `json:"lookup"`
	VersionSource string               `json:"version_source"`
	Instances     []DependencyInstance `json:"instances"`
}

// DependencyInstance is one installation of a DependencyGroup.
type DependencyInstance struct {
	ID           int64  `json:"id"`
	HostID       int64  `json:"host_id"`
	HostNickname string `json:"host_nickname"`
	HostSlug     string `json:"host_slug"`
	Container    string `json:"container"`
	Image        string `json:"image"`
	ServiceID    *int64 `json:"service_id"`
	CVECount     int    `json:"cve_count"`
}

// DependencyVuln is one advisory matching a dependency.
type DependencyVuln struct {
	ID       string  `json:"id"` // CVE-… (or the OSV id when no CVE alias)
	Severity string  `json:"severity"`
	CVSS     float64 `json:"cvss"`
	Summary  string  `json:"summary"`
	Fixed    string  `json:"fixed,omitempty"`
	Source   string  `json:"source"` // osv | nvd
	// OSVID is the OSV advisory ("DEBIAN-CVE-…") when Source is osv.
	OSVID string `json:"osv_id,omitempty"`
}

// DependencyDetail is the drawer payload: the row, its advisories and the
// findings they produced.
type DependencyDetail struct {
	Dependency
	Vulns    []DependencyVuln `json:"vulns"`
	Findings []VulnFinding    `json:"findings"`
}
