package service

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// SiteServiceSummary is what one scan's site-service step did.
type SiteServiceSummary struct {
	Sites int `json:"sites"` // distinct host web apps (vhost + docroot) reconciled
}

// RecordSiteServices turns the scan's host web apps into "site" services
// (C.3, discovery_kind='site'): one per vhost docroot, keyed by the vhost's
// primary (DNS-linkable) name + docroot, carrying the framework/version the A7
// code fingerprint found. A site service is the natural app parent for a
// host's captured DB logins and discovered Atlas databases. It derives from
// web_servers — present in every scan — so a sync scan keeps a live site
// online; the framework only enriches it in a background scan. Only adds,
// refreshes and offlines through store.ReconcileSites; a trashed site is never
// recreated. Runs before RecordWebDNS, which links each one to its DNS record.
//
// Scoped to host (non-container) nginx vhosts with a document root and a
// DNS-linkable name — Apache vhosts carry no DocumentRoot through apachectl -S,
// so they have no site service until that gap is closed.
func RecordSiteServices(ctx context.Context, db *sql.DB, host *models.Host, info sshtest.VMInfo) (SiteServiceSummary, error) {
	ctx = store.WithSystemScope(ctx)
	fw := frameworkByDocroot(info)
	var sites []store.DiscoveredSite
	seen := map[string]bool{}
	for _, ws := range info.WebServers {
		if ws.Container != "" || ws.Stopped || ws.Error != "" {
			continue
		}
		for _, site := range ws.Sites {
			if site.Disabled || site.Root == "" {
				continue
			}
			name, ok := primarySiteName(site)
			if !ok {
				continue
			}
			key := store.SiteDiscoveryKey(name, site.Root)
			if seen[key] {
				continue
			}
			seen[key] = true
			f := fw[site.Root]
			sites = append(sites, store.DiscoveredSite{Name: name, Docroot: site.Root, Framework: f.framework, Version: f.version})
		}
	}
	if err := store.NewServiceRepo(db).ReconcileSites(ctx, host.ID, sites); err != nil {
		return SiteServiceSummary{}, err
	}
	return SiteServiceSummary{Sites: len(sites)}, nil
}

type docFramework struct{ framework, version string }

// frameworkByDocroot maps each A7 docroot to its primary app's framework and
// version: the root app (Path "") when present, else the first app found.
// Empty when the job-mode code fingerprint didn't run (DeepFacts nil).
func frameworkByDocroot(info sshtest.VMInfo) map[string]docFramework {
	out := map[string]docFramework{}
	if info.DeepFacts == nil {
		return out
	}
	for _, c := range info.DeepFacts.Code {
		var pick *sshtest.CodeApp
		for i := range c.Apps {
			if c.Apps[i].Path == "" {
				pick = &c.Apps[i]
				break
			}
			if pick == nil {
				pick = &c.Apps[i]
			}
		}
		if pick != nil {
			out[c.Docroot] = docFramework{framework: pick.Framework, version: pick.Version}
		}
	}
	return out
}

// primarySiteName is the vhost's first DNS-linkable server name — the stable
// half of a site service's identity and the name RecordWebDNS links to it.
func primarySiteName(site sshtest.WebSite) (string, bool) {
	for _, raw := range site.Names {
		if n, ok := LinkableName(raw); ok {
			return n, true
		}
	}
	return "", false
}
