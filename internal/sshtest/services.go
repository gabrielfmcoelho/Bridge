package sshtest

import (
	"sort"
	"strings"
)

// serviceSpec is the detection pattern for one application service. Same
// shape as agentSpec but with a "Kind" field instead of "Category" so the
// UI can group rows by service type (web/database/cache/…) rather than by
// vendor function (monitoring/security/…).
type serviceSpec struct {
	Name        string
	Label       string
	Vendor      string
	Kind        string
	Units       []string
	Binaries    []string
	Packages    []string
	ImageHints  []string // case-insensitive substrings to match against container image names
	// SignaturePorts are well-known TCP ports for this service. Used as a
	// fallback for the listening-ports panel when `ss -tlnp` can't read
	// the owning process (e.g., postgres listening as the postgres user
	// while the scan runs as a non-root account). Catalog ports are
	// claimed first by agentCatalog; service entries that overlap with
	// already-claimed ports won't double-attribute.
	SignaturePorts []int
}

// serviceCatalog enumerates well-known application services. The catalog
// is intentionally curated to be disjoint from agentCatalog — these are
// the workloads users *run on* their hosts (web servers, databases,
// queues), not the agents that *manage* the host (monitoring, security).
var serviceCatalog = []serviceSpec{
	// ── Web servers / reverse proxies ──
	{Name: "nginx", Label: "Nginx", Vendor: "F5/NGINX", Kind: "web",
		Units: []string{"nginx"}, Binaries: []string{"nginx"},
		Packages:   []string{"nginx", "nginx-core", "nginx-full", "nginx-extras", "nginx-light"},
		ImageHints: []string{"nginx"}},
	{Name: "apache", Label: "Apache HTTP Server", Vendor: "Apache Software Foundation", Kind: "web",
		Units: []string{"apache2", "httpd"}, Binaries: []string{"apache2", "httpd"},
		Packages:   []string{"apache2", "httpd"},
		ImageHints: []string{"httpd", "apache"}},
	{Name: "caddy", Label: "Caddy", Vendor: "Caddy", Kind: "web",
		Units: []string{"caddy"}, Binaries: []string{"caddy"},
		Packages:   []string{"caddy"},
		ImageHints: []string{"caddy"}},
	{Name: "lighttpd", Label: "lighttpd", Vendor: "lighttpd", Kind: "web",
		Units: []string{"lighttpd"}, Binaries: []string{"lighttpd"},
		Packages: []string{"lighttpd"}},
	{Name: "haproxy", Label: "HAProxy", Vendor: "HAProxy Technologies", Kind: "proxy",
		Units: []string{"haproxy"}, Binaries: []string{"haproxy"},
		Packages:   []string{"haproxy"},
		ImageHints: []string{"haproxy"}},
	{Name: "traefik", Label: "Traefik", Vendor: "Traefik Labs", Kind: "proxy",
		Units: []string{"traefik"}, Binaries: []string{"traefik"},
		Packages:   []string{"traefik"},
		ImageHints: []string{"traefik"}},
	{Name: "envoy", Label: "Envoy Proxy", Vendor: "CNCF", Kind: "proxy",
		Units: []string{"envoy"}, Binaries: []string{"envoy"},
		Packages:   []string{"envoy"},
		ImageHints: []string{"envoyproxy/envoy"}},
	{Name: "squid", Label: "Squid", Vendor: "Squid", Kind: "proxy",
		Units: []string{"squid"}, Binaries: []string{"squid"},
		Packages: []string{"squid"}},

	// ── Relational databases ──
	{Name: "postgresql", Label: "PostgreSQL", Vendor: "PostgreSQL Global Development Group", Kind: "database",
		Units: []string{"postgresql", "postgresql@"}, Binaries: []string{"postgres"},
		Packages: []string{
			"postgresql", "postgresql-server", "postgresql-15", "postgresql-14",
			"postgresql-13", "postgresql-12", "postgresql-11", "postgresql-16", "postgresql-17",
		},
		ImageHints:     []string{"postgres"},
		SignaturePorts: []int{5432}},
	{Name: "mysql", Label: "MySQL", Vendor: "Oracle", Kind: "database",
		Units: []string{"mysql", "mysqld"}, Binaries: []string{"mysqld"},
		Packages:       []string{"mysql-server", "mysql-community-server", "mysql"},
		ImageHints:     []string{"mysql"},
		SignaturePorts: []int{3306}},
	{Name: "mariadb", Label: "MariaDB", Vendor: "MariaDB Foundation", Kind: "database",
		Units: []string{"mariadb"}, Binaries: []string{"mariadbd"},
		Packages:   []string{"mariadb-server"},
		ImageHints: []string{"mariadb"}},
	{Name: "mssql", Label: "Microsoft SQL Server", Vendor: "Microsoft", Kind: "database",
		Units: []string{"mssql-server"}, Binaries: []string{"sqlservr"},
		Packages:       []string{"mssql-server"},
		ImageHints:     []string{"mcr.microsoft.com/mssql"},
		SignaturePorts: []int{1433}},
	{Name: "oracle-db", Label: "Oracle Database", Vendor: "Oracle", Kind: "database",
		Binaries: []string{"oracle"},
		Packages: []string{"oracle-database-ee", "oracle-instantclient"}},

	// ── Document / search / time-series databases ──
	{Name: "mongodb", Label: "MongoDB", Vendor: "MongoDB Inc.", Kind: "database",
		Units: []string{"mongod"}, Binaries: []string{"mongod"},
		Packages:       []string{"mongodb-org-server", "mongodb-server", "mongodb"},
		ImageHints:     []string{"mongo"},
		SignaturePorts: []int{27017}},
	{Name: "elasticsearch", Label: "Elasticsearch", Vendor: "Elastic", Kind: "database",
		Units: []string{"elasticsearch"}, Binaries: []string{"elasticsearch"},
		Packages:       []string{"elasticsearch"},
		ImageHints:     []string{"elasticsearch"},
		SignaturePorts: []int{9200}},
	{Name: "opensearch", Label: "OpenSearch", Vendor: "OpenSearch", Kind: "database",
		Units: []string{"opensearch"}, Binaries: []string{"opensearch"},
		Packages:   []string{"opensearch"},
		ImageHints: []string{"opensearch"}},
	{Name: "cassandra", Label: "Apache Cassandra", Vendor: "Apache Software Foundation", Kind: "database",
		Units: []string{"cassandra"}, Binaries: []string{"cassandra"},
		Packages:       []string{"cassandra"},
		ImageHints:     []string{"cassandra"},
		SignaturePorts: []int{9042}},
	{Name: "couchdb", Label: "Apache CouchDB", Vendor: "Apache Software Foundation", Kind: "database",
		Units: []string{"couchdb"}, Binaries: []string{"couchdb"},
		Packages:       []string{"couchdb"},
		ImageHints:     []string{"couchdb"},
		SignaturePorts: []int{5984}},
	{Name: "influxdb", Label: "InfluxDB", Vendor: "InfluxData", Kind: "database",
		Units: []string{"influxdb"}, Binaries: []string{"influxd"},
		Packages:       []string{"influxdb"},
		ImageHints:     []string{"influxdb"},
		SignaturePorts: []int{8086}},
	{Name: "clickhouse", Label: "ClickHouse", Vendor: "ClickHouse Inc.", Kind: "database",
		Units: []string{"clickhouse-server"}, Binaries: []string{"clickhouse-server"},
		Packages:       []string{"clickhouse-server", "clickhouse-common-static"},
		ImageHints:     []string{"clickhouse"},
		SignaturePorts: []int{8123}},

	// ── Caches / KV ──
	{Name: "redis", Label: "Redis", Vendor: "Redis Ltd.", Kind: "cache",
		Units: []string{"redis", "redis-server", "redis@"}, Binaries: []string{"redis-server"},
		Packages:       []string{"redis-server", "redis"},
		ImageHints:     []string{"redis"},
		SignaturePorts: []int{6379}},
	{Name: "memcached", Label: "Memcached", Vendor: "Memcached", Kind: "cache",
		Units: []string{"memcached"}, Binaries: []string{"memcached"},
		Packages:       []string{"memcached"},
		ImageHints:     []string{"memcached"},
		SignaturePorts: []int{11211}},
	{Name: "valkey", Label: "Valkey", Vendor: "Valkey", Kind: "cache",
		Units: []string{"valkey", "valkey-server"}, Binaries: []string{"valkey-server"},
		Packages:   []string{"valkey", "valkey-server"},
		ImageHints: []string{"valkey"}},
	{Name: "dragonfly", Label: "DragonflyDB", Vendor: "DragonflyDB", Kind: "cache",
		Binaries:   []string{"dragonfly"},
		Packages:   []string{"dragonfly"},
		ImageHints: []string{"dragonflydb/dragonfly"}},
	{Name: "etcd", Label: "etcd", Vendor: "CNCF", Kind: "cache",
		Units: []string{"etcd"}, Binaries: []string{"etcd"},
		Packages:   []string{"etcd"},
		ImageHints: []string{"etcd"}},

	// ── Message queues / brokers ──
	{Name: "rabbitmq", Label: "RabbitMQ", Vendor: "VMware", Kind: "queue",
		Units: []string{"rabbitmq-server"}, Binaries: []string{"rabbitmq", "beam.smp"},
		Packages:       []string{"rabbitmq-server"},
		ImageHints:     []string{"rabbitmq"},
		SignaturePorts: []int{5672, 15672}},
	{Name: "kafka", Label: "Apache Kafka", Vendor: "Apache Software Foundation", Kind: "queue",
		Units: []string{"kafka"}, Binaries: []string{"kafka"},
		Packages:       []string{"kafka"},
		ImageHints:     []string{"kafka"},
		SignaturePorts: []int{9092}},
	{Name: "nats", Label: "NATS", Vendor: "Synadia", Kind: "queue",
		Units: []string{"nats-server"}, Binaries: []string{"nats-server"},
		Packages:       []string{"nats-server"},
		ImageHints:     []string{"nats"},
		SignaturePorts: []int{4222}},
	{Name: "mosquitto", Label: "Eclipse Mosquitto", Vendor: "Eclipse Foundation", Kind: "queue",
		Units: []string{"mosquitto"}, Binaries: []string{"mosquitto"},
		Packages:       []string{"mosquitto"},
		ImageHints:     []string{"mosquitto"},
		SignaturePorts: []int{1883, 8883}},
	{Name: "activemq", Label: "Apache ActiveMQ", Vendor: "Apache Software Foundation", Kind: "queue",
		Units: []string{"activemq"}, Binaries: []string{"activemq"},
		Packages: []string{"activemq"}},

	// ── Application runtimes / process managers ──
	{Name: "supervisord", Label: "Supervisor", Vendor: "Supervisor", Kind: "runtime",
		Units: []string{"supervisor", "supervisord"}, Binaries: []string{"supervisord"},
		Packages: []string{"supervisor"}},
	{Name: "uwsgi", Label: "uWSGI", Vendor: "uWSGI", Kind: "runtime",
		Units: []string{"uwsgi"}, Binaries: []string{"uwsgi"},
		Packages: []string{"uwsgi", "uwsgi-core"}},
	{Name: "gunicorn", Label: "Gunicorn", Vendor: "Benoit Chesneau", Kind: "runtime",
		Binaries: []string{"gunicorn"},
		Packages: []string{"gunicorn"}},
	{Name: "php-fpm", Label: "PHP-FPM", Vendor: "PHP Group", Kind: "runtime",
		Units: []string{
			"php-fpm",
			"php8.0-fpm", "php8.1-fpm", "php8.2-fpm", "php8.3-fpm", "php8.4-fpm",
			"php7.0-fpm", "php7.1-fpm", "php7.2-fpm", "php7.3-fpm", "php7.4-fpm",
		},
		Binaries: []string{"php-fpm", "php-fpm8", "php-fpm7"},
		Packages: []string{"php-fpm", "php8.1-fpm", "php8.2-fpm", "php8.3-fpm", "php7.4-fpm", "php-cli"},
		ImageHints: []string{"php-fpm"}},

	// ── DNS ──
	{Name: "named", Label: "BIND (named)", Vendor: "ISC", Kind: "dns",
		Units: []string{"named", "bind9"}, Binaries: []string{"named"},
		Packages: []string{"bind9", "bind"}},
	{Name: "dnsmasq", Label: "Dnsmasq", Vendor: "dnsmasq", Kind: "dns",
		Units: []string{"dnsmasq"}, Binaries: []string{"dnsmasq"},
		Packages: []string{"dnsmasq"}},
	{Name: "unbound", Label: "Unbound", Vendor: "NLnet Labs", Kind: "dns",
		Units: []string{"unbound"}, Binaries: []string{"unbound"},
		Packages: []string{"unbound"}},
	{Name: "pihole", Label: "Pi-hole", Vendor: "Pi-hole", Kind: "dns",
		Units: []string{"pihole-FTL"}, Binaries: []string{"pihole-FTL"},
		Packages:   []string{"pihole"},
		ImageHints: []string{"pihole/pihole"}},

	// ── Mail ──
	{Name: "postfix", Label: "Postfix", Vendor: "Postfix", Kind: "mail",
		Units: []string{"postfix"}, Binaries: []string{"master"},
		Packages: []string{"postfix"}},
	{Name: "exim", Label: "Exim", Vendor: "Exim", Kind: "mail",
		Units: []string{"exim4", "exim"}, Binaries: []string{"exim4", "exim"},
		Packages: []string{"exim4", "exim"}},
	{Name: "dovecot", Label: "Dovecot", Vendor: "Dovecot", Kind: "mail",
		Units: []string{"dovecot"}, Binaries: []string{"dovecot"},
		Packages: []string{"dovecot-core", "dovecot"}},

	// ── File sharing / network storage ──
	{Name: "samba", Label: "Samba", Vendor: "Samba Team", Kind: "file-sharing",
		Units: []string{"smbd", "nmbd"}, Binaries: []string{"smbd", "nmbd"},
		Packages: []string{"samba"}},
	{Name: "vsftpd", Label: "vsftpd", Vendor: "vsftpd", Kind: "file-sharing",
		Units: []string{"vsftpd"}, Binaries: []string{"vsftpd"},
		Packages: []string{"vsftpd"}},
	{Name: "proftpd", Label: "ProFTPD", Vendor: "ProFTPD", Kind: "file-sharing",
		Units: []string{"proftpd"}, Binaries: []string{"proftpd"},
		Packages: []string{"proftpd-basic", "proftpd"}},
	{Name: "nfs-server", Label: "NFS Server", Vendor: "Linux NFS", Kind: "file-sharing",
		Units: []string{"nfs-server", "nfs-kernel-server"}, Binaries: []string{"nfsd"},
		Packages: []string{"nfs-kernel-server", "nfs-utils"}},
	{Name: "minio", Label: "MinIO", Vendor: "MinIO", Kind: "file-sharing",
		Units: []string{"minio"}, Binaries: []string{"minio"},
		Packages:   []string{"minio"},
		ImageHints: []string{"minio/minio"}},

	// ── Identity / directory ──
	{Name: "openldap", Label: "OpenLDAP", Vendor: "OpenLDAP Foundation", Kind: "directory",
		Units: []string{"slapd"}, Binaries: []string{"slapd"},
		Packages: []string{"slapd"}},
	{Name: "freeipa", Label: "FreeIPA", Vendor: "Red Hat", Kind: "directory",
		Units: []string{"ipa"}, Binaries: []string{"ipa"},
		Packages: []string{"freeipa-server"}},
	{Name: "keycloak", Label: "Keycloak", Vendor: "Red Hat", Kind: "directory",
		Units: []string{"keycloak"}, Binaries: []string{"keycloak"},
		ImageHints: []string{"keycloak"}},

	// ── Analytics / search frontends / business apps ──
	{Name: "kibana", Label: "Kibana", Vendor: "Elastic", Kind: "analytics",
		Units: []string{"kibana"}, Binaries: []string{"kibana"},
		Packages:       []string{"kibana"},
		ImageHints:     []string{"kibana"},
		SignaturePorts: []int{5601}},
	{Name: "logstash", Label: "Logstash", Vendor: "Elastic", Kind: "logging",
		Units: []string{"logstash"}, Binaries: []string{"logstash"},
		Packages:   []string{"logstash"},
		ImageHints: []string{"logstash"}},
	{Name: "graylog", Label: "Graylog", Vendor: "Graylog", Kind: "logging",
		Units: []string{"graylog-server"}, Binaries: []string{"graylog"},
		Packages:   []string{"graylog-server"},
		ImageHints: []string{"graylog"}},
	{Name: "sonarqube", Label: "SonarQube", Vendor: "SonarSource", Kind: "analytics",
		Units: []string{"sonar"}, Binaries: []string{"sonar"},
		ImageHints: []string{"sonarqube"}},
	{Name: "gitlab", Label: "GitLab", Vendor: "GitLab Inc.", Kind: "analytics",
		Units: []string{"gitlab-runsvdir"}, Binaries: []string{"gitlab-mon"},
		Packages:   []string{"gitlab-ce", "gitlab-ee"},
		ImageHints: []string{"gitlab/gitlab"}},
	{Name: "jenkins", Label: "Jenkins", Vendor: "Jenkins", Kind: "analytics",
		Units: []string{"jenkins"}, Binaries: []string{"jenkins"},
		Packages:   []string{"jenkins"},
		ImageHints: []string{"jenkins"}},
}

// captureServices intersects hostInventory with serviceCatalog and folds in
// container-derived signals. agentNames is the set of catalog keys already
// classified as agents (so we never double-list grafana-server etc. — both
// catalogs are curated disjoint, this is a safety net). agentPortClaims is
// the set of ports already claimed by agentCatalog SignaturePort matches;
// service entries whose SignaturePorts overlap with those won't re-claim.
func captureServices(inv *hostInventory, containers []ContainerInfo, agentNames map[string]bool, agentPortClaims map[int]bool) []DiscoveredService {
	// Service-side port claims accumulate as we iterate so two services
	// with the same well-known port (e.g. mysql + mariadb both on 3306)
	// don't both flag a host that has neither installed.
	servicePortClaims := map[int]bool{}
	var services []DiscoveredService
	for _, spec := range serviceCatalog {
		if agentNames[spec.Name] {
			continue
		}
		svc, ok := detectService(spec, inv, containers, agentPortClaims, servicePortClaims)
		if !ok {
			continue
		}
		for _, p := range spec.SignaturePorts {
			if inv.ListenPorts[p] {
				servicePortClaims[p] = true
			}
		}
		services = append(services, svc)
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].Kind != services[j].Kind {
			return services[i].Kind < services[j].Kind
		}
		return services[i].Label < services[j].Label
	})
	return services
}

// detectService is the per-spec detector. Mirrors detectAgent's structure
// but additionally inspects container images so a postgres-in-docker host
// shows up as "postgresql" alongside a host-installed postgres.
//
// agentPortClaims and servicePortClaims are sets of ports already claimed
// by earlier catalog entries (agents first, then in-progress services).
// Signature-port detection skips any port already in either set so two
// catalog rows can't both flag the same listener.
func detectService(spec serviceSpec, inv *hostInventory, containers []ContainerInfo, agentPortClaims, servicePortClaims map[int]bool) (DiscoveredService, bool) {
	svc := DiscoveredService{Name: spec.Name, Label: spec.Label, Vendor: spec.Vendor, Kind: spec.Kind}
	sources := map[string]bool{}

	// systemd unit-files (installed); drives Enabled state.
	for _, want := range spec.Units {
		matched := false
		for unit, state := range inv.UnitFiles {
			if unitMatches(want, unit) {
				svc.Unit = stripUnitSuffix(unit)
				svc.Enabled = state == "enabled" || state == "static" || state == "alias" || state == "enabled-runtime"
				sources["systemd"] = true
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}
	// systemd loaded units (active state).
	for _, want := range spec.Units {
		matched := false
		for unit, active := range inv.LoadedUnits {
			if unitMatches(want, unit) {
				if svc.Unit == "" {
					svc.Unit = stripUnitSuffix(unit)
				}
				svc.State = active
				sources["systemd"] = true
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}

	// Running processes.
	for _, p := range inv.Procs {
		matched := false
		for _, b := range spec.Binaries {
			if binaryMatches(b, p.comm, p.args) {
				if svc.PID == 0 {
					svc.PID = p.pid
				}
				if svc.State == "" {
					svc.State = "running"
				}
				sources["process"] = true
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}

	// Packages — supplies version when available.
	for _, pkg := range spec.Packages {
		if v, ok := inv.Packages[pkg]; ok {
			svc.Package = pkg
			svc.Version = v
			sources["package"] = true
			break
		}
	}

	// Listen ports — match owner process basename, then fall back to
	// SignaturePorts for hosts where ss couldn't read the owning process
	// (e.g. postgres listening as the postgres user). Ports already
	// claimed by an agent or a previous service are skipped to prevent
	// double-attribution (e.g., mysql + mariadb both target :3306).
	portSet := map[int]bool{}
	for _, b := range spec.Binaries {
		for _, port := range inv.PortsByProc[b] {
			portSet[port] = true
			sources["port"] = true
		}
	}
	for _, p := range spec.SignaturePorts {
		if !inv.ListenPorts[p] {
			continue
		}
		if agentPortClaims[p] || servicePortClaims[p] {
			continue
		}
		portSet[p] = true
		sources["port"] = true
	}
	if len(portSet) > 0 {
		for p := range portSet {
			svc.Ports = append(svc.Ports, p)
		}
		sort.Ints(svc.Ports)
	}

	// Containers — match image name. A container counts as a separate
	// signal so e.g. "postgres in docker" is detected even when there's no
	// host-installed postgres binary or systemd unit.
	for _, c := range containers {
		image := strings.ToLower(c.Image)
		for _, hint := range spec.ImageHints {
			if hint != "" && strings.Contains(image, strings.ToLower(hint)) {
				if svc.ContainerID == "" {
					svc.ContainerID = c.ID
					svc.ContainerImage = c.Image
					if svc.State == "" {
						svc.State = "running"
					}
				}
				sources["container"] = true
				break
			}
		}
	}

	if len(sources) == 0 {
		return DiscoveredService{}, false
	}

	if svc.State == "" && sources["package"] {
		svc.State = "stopped"
	}
	for src := range sources {
		svc.Sources = append(svc.Sources, src)
	}
	sort.Strings(svc.Sources)
	return svc, true
}
