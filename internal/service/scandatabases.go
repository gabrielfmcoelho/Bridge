package service

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// scandatabases.go — Atlas database rows derived from the A5 DB-metadata probe
// plus Phase 1's credential_sources, the way RecordWebDNS / RecordGitLinks
// derive DNS and GitLab links inside recordScan. One row per non-system schema
// the probe found, keyed by the DB instance service on the host; the app
// services whose config files name that schema become its consumers. Only adds;
// never recreates a trashed row; copies the host's grants. The captured logins
// stay host-scoped — this links, it does not move a secret.

// ScanDatabaseSummary is what one scan's database-linking step did.
type ScanDatabaseSummary struct {
	Created   int `json:"created"`   // new Atlas database rows
	Consumers int `json:"consumers"` // consumer-service links added
	Skipped   int `json:"skipped"`   // schemas that already had a row (live or trashed)
}

// personalTableRe flags a schema whose table names read as personal data (the
// LGPD "pessoal" hint), beside the probe's own cpf/rg column flag.
var personalTableRe = regexp.MustCompile(`(?i)(candidat|pessoa|cliente|usuari|servidor|inscri|matricul|aluno|funcionari|cpf|contribuinte|eleitor|benefici|cidad|cadastr)`)

// localDBHosts are the Fields["host"] values that mean "the DB on this same VM".
var localDBHosts = map[string]bool{"": true, "localhost": true, "127.0.0.1": true, "::1": true, "0.0.0.0": true}

// RecordScanDatabases creates an Atlas databases row for each non-system schema
// the A5 probe found on host, when none exists yet (live or trashed). nil
// DBMeta (the probe didn't run) or no discovered DB instance service → a no-op.
func RecordScanDatabases(ctx context.Context, db *sql.DB, host *models.Host, info sshtest.VMInfo) (ScanDatabaseSummary, error) {
	var sum ScanDatabaseSummary
	if info.DBMeta == nil || len(info.DBMeta.Schemas) == 0 {
		return sum, nil
	}
	ctx = store.WithSystemScope(ctx)
	svcs, err := store.NewServiceRepo(db).ListDiscoveredByHost(ctx, host.ID)
	if err != nil {
		return sum, err
	}
	instanceID, engine, ok := dbInstanceService(svcs, info.DBMeta.Engine)
	if !ok {
		return sum, nil // no (or ambiguous) DB instance service → can't attribute the schemas
	}
	resolver := docrootResolver(svcs, info)
	appDBs := appDatabaseSources(info, host) // db name → config files that name it

	repo := store.NewDatabaseRepo(db)
	grants := store.NewAssetEntidadeRepo(db)
	for _, sc := range info.DBMeta.Schemas {
		exists, err := repo.ExistsByInstanceAndName(ctx, instanceID, sc.Name)
		if err != nil {
			return sum, err
		}
		if exists {
			sum.Skipped++
			continue
		}
		inst := instanceID
		hid := host.ID
		d := &models.Database{
			Name:              sc.Name + " @ " + hostLabel(host),
			DBName:            sc.Name,
			Engine:            engine,
			InstanceServiceID: &inst,
			ConnectionMode:    "ssh_tunnel",
			TunnelHostID:      &hid,
			Sensitivity:       sensitivityHint(sc),
		}
		if err := repo.CreateDiscovered(ctx, d); err != nil {
			if store.IsUniqueViolation(err) {
				sum.Skipped++ // a concurrent scan created it first
				continue
			}
			return sum, err
		}
		if err := grants.CopyFrom(ctx, db, store.AssetHost, host.ID, store.AssetDatabase, d.ID); err != nil {
			return sum, err
		}
		sum.Created++

		// Consumers: the service whose docroot holds a config file that names
		// this schema with a local host (the app that reads the DB).
		cset := map[int64]bool{}
		for _, path := range appDBs[sc.Name] {
			if sid := resolver(path); sid != 0 && sid != instanceID {
				cset[sid] = true
			}
		}
		if len(cset) > 0 {
			ids := sortedIDs(cset)
			if err := repo.SetLinks(ctx, d.ID, ids, nil); err != nil {
				return sum, err
			}
			sum.Consumers += len(ids)
		}
	}
	return sum, nil
}

// dbInstanceService returns the single DB instance service on the host that
// matches the probe's engine family (mysql/mariadb), its engine, and ok. Zero
// or several candidates → !ok: the schemas can't be attributed unambiguously.
func dbInstanceService(svcs []models.Service, probeEngine string) (int64, string, bool) {
	var id int64
	var engine string
	found := 0
	for _, s := range svcs {
		eng := serviceEngine(s)
		if eng == "" || !sameEngineFamily(eng, probeEngine) {
			continue
		}
		found++
		id, engine = s.ID, eng
	}
	if found != 1 {
		return 0, "", false
	}
	return id, engine, true
}

// serviceEngine is a service's DB engine, or "" when it isn't a database
// instance. ServiceKind "database" with an unknown image falls back to the
// probe family's generic engine via the caller.
func serviceEngine(s models.Service) string {
	eng := models.InferDatabaseEngine(s.ServiceSubtype, s.ContainerImage)
	if eng != "other" {
		return eng
	}
	if s.ServiceKind == "database" {
		return "mysql" // a DB service the inference couldn't name: assume the fleet's engine
	}
	return ""
}

// sameEngineFamily groups mysql and mariadb (one wire protocol, one probe).
func sameEngineFamily(a, b string) bool {
	norm := func(e string) string {
		if e == "mariadb" {
			return "mysql"
		}
		return e
	}
	return norm(a) == norm(b)
}

// docrootResolver returns a function mapping a config-file path to the service
// that serves the docroot containing it — the same site→service map RecordWebDNS
// builds (byHostKind for a host server, byContainer for a container). The
// longest matching docroot wins.
func docrootResolver(svcs []models.Service, info sshtest.VMInfo) func(path string) int64 {
	byContainer := map[string]int64{}
	byHostKind := map[string]int64{}
	for _, s := range svcs {
		if s.ContainerName != "" {
			byContainer[s.ContainerName] = s.ID
		}
		if s.DiscoveryKind == "host" {
			byHostKind[s.DiscoveryKey] = s.ID
		}
	}
	type root struct {
		prefix string
		svc    int64
	}
	var roots []root
	for _, ws := range info.WebServers {
		if ws.Stopped || ws.Error != "" {
			continue
		}
		svc := byHostKind[ws.Kind]
		if ws.Container != "" {
			svc = byContainer[ws.Container]
		}
		if svc == 0 {
			continue
		}
		for _, site := range ws.Sites {
			if r := strings.TrimRight(site.Root, "/"); r != "" {
				roots = append(roots, root{prefix: r, svc: svc})
			}
		}
	}
	sort.SliceStable(roots, func(i, j int) bool { return len(roots[i].prefix) > len(roots[j].prefix) })
	return func(p string) int64 {
		for _, r := range roots {
			if p == r.prefix || strings.HasPrefix(p, r.prefix+"/") {
				return r.svc
			}
		}
		return 0
	}
}

// appDatabaseSources groups the config files that name a database on this VM by
// the db name, from the Phase 1 credential_sources (a source with Fields["db"]
// set and a local Fields["host"]). The value is the config-file path, which
// docrootResolver maps to the consuming service.
func appDatabaseSources(info sshtest.VMInfo, host *models.Host) map[string][]string {
	out := map[string][]string{}
	hostAddr := strings.ToLower(strings.TrimSpace(host.Hostname))
	for _, cs := range info.CredentialSources {
		dbName := strings.TrimSpace(cs.Fields["db"])
		if dbName == "" || cs.Path == "" {
			continue
		}
		h := strings.ToLower(strings.TrimSpace(cs.Fields["host"]))
		if !localDBHosts[h] && h != hostAddr {
			continue // the config points at a DB on another host
		}
		out[dbName] = append(out[dbName], cs.Path)
	}
	return out
}

// sensitivityHint suggests "pessoal" when the probe flagged a cpf/rg column or a
// table name reads as personal data; "" otherwise (ApplyDefaults → "interno").
func sensitivityHint(sc sshtest.DBSchema) string {
	if sc.PII {
		return "pessoal"
	}
	for _, t := range sc.Names {
		if personalTableRe.MatchString(t) {
			return "pessoal"
		}
	}
	return ""
}

// hostLabel is the host's display name for the row's "<db> @ <host>" name.
func hostLabel(host *models.Host) string {
	if host.Nickname != "" {
		return host.Nickname
	}
	return host.OficialSlug
}

// ScanLoginsForHosts reads the latest scans of hostIDs and returns the
// config-file logins they found (Phase 1 credential_sources), as ScanLoginRef
// pointers for a database or service page — never a value. When dbName is set
// (a database page) only logins naming that db are kept; otherwise (a service
// page) every cred-kind login on the hosts is returned.
func ScanLoginsForHosts(ctx context.Context, db *sql.DB, hostIDs []int64, dbName string) ([]models.ScanLoginRef, error) {
	if len(hostIDs) == 0 {
		return nil, nil
	}
	srcs, err := store.NewHostScanRepo(db).LatestCredentialSources(ctx, hostIDs)
	if err != nil {
		return nil, err
	}
	var out []models.ScanLoginRef
	for _, s := range srcs {
		if dbName != "" {
			if !strings.EqualFold(strings.TrimSpace(s.DBName), strings.TrimSpace(dbName)) {
				continue
			}
		} else if s.Kind != "cred" {
			continue // a service page lists real logins, not .env keys or ssh keys
		}
		out = append(out, models.ScanLoginRef{
			HostID: s.HostID, HostNickname: s.HostNickname, HostSlug: s.HostSlug,
			Detector: s.Detector, Path: s.Path, Locator: s.Locator, User: s.User, DBName: s.DBName,
		})
	}
	return out, nil
}

func sortedIDs(set map[int64]bool) []int64 {
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
