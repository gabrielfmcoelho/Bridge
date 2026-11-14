package sshtest

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---- CodeIgniter 4 ----

func TestDetectCI4Database(t *testing.T) {
	content := `<?php
namespace Config;
class Database extends \CodeIgniter\Database\Config {
	public array $default = [
		'DSN'      => '',
		'hostname' => 'localhost',
		'username' => 'root',
		'password' => 'ci4Secret1',
		'database' => 'ci4app',
		'DBDriver' => 'MySQLi',
		'port'     => 3306,
	];
	public array $tests = [
		'hostname' => 'localhost',
		'username' => 'ignored',
		'password' => 'shouldNotLeak',
	];
}`
	hits := detect("/var/www/ci4/app/Config/Database.php", content)
	if len(hits) != 1 || hits[0].Detector != "ci4_database" {
		t.Fatalf("hits = %+v, want one ci4_database (only $default, not $tests)", hits)
	}
	h := hits[0]
	if !h.Admin {
		t.Fatalf("root login should be admin tier: %+v", h)
	}
	if h.Fields["engine"] != "mysql" || h.Fields["db"] != "ci4app" || h.Fields["port"] != "3306" {
		t.Fatalf("fields = %v", h.Fields)
	}
	if s := credSecret(t, h.Secret); s["username"] != "root" || s["password"] != "ci4Secret1" {
		t.Fatalf("payload = %v", s)
	}
	if strings.Contains(h.Secret, "shouldNotLeak") {
		t.Fatal("the $tests array must not be read")
	}
}

func TestDetectCI4DatabaseEnvIndirect(t *testing.T) {
	content := `<?php
class Database {
	public array $default = [
		'hostname' => 'localhost',
		'username' => 'app',
		'password' => env('database.default.password'),
	];
}`
	hits := detect("/var/www/ci4/app/Config/Database.php", content)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if hits[0].Secret != "" {
		t.Fatalf("env() password must be indirect (no secret), got %q", hits[0].Secret)
	}
}

// ---- CodeIgniter email ----

func TestDetectCIEmail(t *testing.T) {
	content := `<?php
$config['protocol'] = 'smtp';
$config['smtp_host'] = 'smtp.local';
$config['smtp_user'] = 'notify@local';
$config['smtp_pass'] = 'ciMail2';
$config['smtp_port'] = 587;
`
	hits := detect("/var/www/ci3/application/config/email.php", content)
	if len(hits) != 1 || hits[0].Detector != "ci_email" {
		t.Fatalf("hits = %+v", hits)
	}
	h := hits[0]
	if h.Admin {
		t.Fatalf("SMTP login is the normal tier: %+v", h)
	}
	if h.Fields["protocol"] != "smtp" || h.Fields["host"] != "smtp.local" || h.Fields["port"] != "587" {
		t.Fatalf("fields = %v", h.Fields)
	}
	if s := credSecret(t, h.Secret); s["username"] != "notify@local" || s["password"] != "ciMail2" {
		t.Fatalf("payload = %v", s)
	}
}

// ---- Joomla ----

func TestDetectJoomlaConfig(t *testing.T) {
	content := `<?php
class JConfig {
	public $host = 'localhost';
	public $user = 'joomla_u';
	public $password = 'joomlaDb3';
	public $db = 'joomla';
	public $dbtype = 'mysqli';
	public $smtphost = 'smtp.local';
	public $smtpuser = 'mailer';
	public $smtppass = 'joomlaSmtp4';
	public $smtpport = '465';
}`
	hits := hitByLocator(detect("/var/www/joomla/configuration.php", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want database + smtp", len(hits))
	}
	db := hits["database"]
	if db.Admin || db.Fields["engine"] != "mysql" {
		t.Fatalf("database = %+v (joomla_u is not a superuser)", db)
	}
	if s := credSecret(t, db.Secret); s["username"] != "joomla_u" || s["password"] != "joomlaDb3" || s["db"] != "joomla" {
		t.Fatalf("database payload = %v", s)
	}
	sm := hits["smtp"]
	if sm.Fields["protocol"] != "smtp" || sm.Fields["port"] != "465" {
		t.Fatalf("smtp fields = %v", sm.Fields)
	}
	if s := credSecret(t, sm.Secret); s["password"] != "joomlaSmtp4" {
		t.Fatalf("smtp payload = %v", s)
	}
}

// ---- Drupal (both array syntaxes, several connections) ----

func TestDetectDrupalSettings(t *testing.T) {
	content := `<?php
$databases['default']['default'] = array(
	'database' => 'drupal',
	'username' => 'drupal_u',
	'password' => 'drupalDb5',
	'host'     => 'localhost',
	'port'     => '3306',
	'driver'   => 'mysql',
	'prefix'   => '',
);
$databases['migrate']['default'] = [
	'database' => 'legacy',
	'username' => 'root',
	'password' => 'drupalMig6',
	'host'     => 'db2',
	'driver'   => 'pgsql',
];
`
	hits := hitByLocator(detect("/var/www/drupal/sites/default/settings.php", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want two connections", len(hits))
	}
	d := hits["default-default"]
	if d.Admin || d.Fields["engine"] != "mysql" {
		t.Fatalf("default = %+v", d)
	}
	if s := credSecret(t, d.Secret); s["password"] != "drupalDb5" || s["db"] != "drupal" {
		t.Fatalf("default payload = %v (array() syntax)", s)
	}
	m := hits["migrate-default"]
	if !m.Admin || m.Fields["engine"] != "postgresql" {
		t.Fatalf("migrate = %+v, want admin (root) + postgresql", m)
	}
	if s := credSecret(t, m.Secret); s["password"] != "drupalMig6" {
		t.Fatalf("migrate payload = %v (short [] syntax)", s)
	}
}

// ---- Moodle ----

func TestDetectMoodleConfig(t *testing.T) {
	content := `<?php
$CFG = new stdClass();
$CFG->dbtype    = 'mariadb';
$CFG->dbhost    = 'localhost';
$CFG->dbname    = 'moodle';
$CFG->dbuser    = 'moodle_u';
$CFG->dbpass    = 'moodleDb7';
$CFG->prefix    = 'mdl_';
`
	hits := detect("/var/www/moodle/config.php", content)
	if len(hits) != 1 || hits[0].Detector != "moodle_config" {
		t.Fatalf("hits = %+v", hits)
	}
	h := hits[0]
	if h.Fields["engine"] != "mysql" || h.Fields["db"] != "moodle" {
		t.Fatalf("fields = %v (mariadb → mysql)", h.Fields)
	}
	if s := credSecret(t, h.Secret); s["username"] != "moodle_u" || s["password"] != "moodleDb7" {
		t.Fatalf("payload = %v", s)
	}
}

func TestMoodleConfigIgnoresNonMoodle(t *testing.T) {
	// config.php is a generic name; a non-Moodle one yields nothing.
	if hits := detect("/var/www/x/config.php", "<?php return array('base_url' => 'http://x');"); len(hits) != 0 {
		t.Fatalf("non-Moodle config.php must not match: %+v", hits)
	}
}

// ---- ScriptCase: encoded, not decoded ----

func TestDetectScriptcaseConn(t *testing.T) {
	content := `[cvi]
tp_conexao = "MySQL"
host = "localhost"
user = "root"
pass = "Sc3nc0d3dBlob8=="
nome_base = "cvi"
`
	hits := detect("/var/www/sc/cvi/conexao.ini", content)
	if len(hits) != 1 || hits[0].Detector != "scriptcase_conn" {
		t.Fatalf("hits = %+v", hits)
	}
	h := hits[0]
	if !h.Admin || h.Fields["encoded"] != "true" || h.Fields["engine"] != "mysql" {
		t.Fatalf("hit = %+v, want admin (root) + encoded flag", h)
	}
	// Stored verbatim — the reversible encoding is NOT decoded.
	if s := credSecret(t, h.Secret); s["password"] != "Sc3nc0d3dBlob8==" || s["db"] != "cvi" {
		t.Fatalf("payload = %v (encoded blob must be stored as-is)", s)
	}
}

// ---- Webmin / Plesk (DB admin) ----

func TestDetectWebminDB(t *testing.T) {
	content := "login=root\npass=WebminDb9\nhost=localhost\nport=3306\n"
	hits := detect("/etc/webmin/mysql/config", content)
	if len(hits) != 1 || hits[0].Detector != "webmin_mysql" || !hits[0].Admin {
		t.Fatalf("hits = %+v, want one admin webmin_mysql", hits)
	}
	if s := credSecret(t, hits[0].Secret); s["username"] != "root" || s["password"] != "WebminDb9" {
		t.Fatalf("payload = %v", s)
	}
	if got := detect("/etc/webmin/postgresql/config", "login=postgres\npass=WebminPg\n"); got[0].Detector != "webmin_pgsql" || got[0].Fields["engine"] != "postgresql" {
		t.Fatalf("pgsql = %+v", got)
	}
}

func TestDetectPleskAdmin(t *testing.T) {
	hits := detect("/etc/psa/.psa.shadow", "PleskAdmin10\n")
	if len(hits) != 1 || hits[0].Detector != "plesk_admin" || !hits[0].Admin {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Fields["encrypted"] != "" {
		t.Fatalf("plaintext shadow should not be flagged encrypted: %v", hits[0].Fields)
	}
	if s := credSecret(t, hits[0].Secret); s["username"] != "admin" || s["password"] != "PleskAdmin10" {
		t.Fatalf("payload = %v", s)
	}
	enc := detect("/etc/psa/.psa.shadow", "$AES-128-CBC$abc$def\n")
	if enc[0].Fields["encrypted"] != "true" {
		t.Fatalf("$AES shadow should be flagged encrypted: %v", enc[0].Fields)
	}
}

// ---- .pgpass: one login per line, escapes ----

func TestDetectPgpass(t *testing.T) {
	content := "localhost:5432:appdb:appuser:PgAppPw11\n" +
		"*:*:*:postgres:PgRootPw12\n" +
		`db\:host:5432:d\\b:u:PgEsc13` + "\n# comment\n"
	hits := detect("/root/.pgpass", content)
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want 3 lines (comment skipped)", len(hits))
	}
	byPass := map[string]CredHit{}
	for _, h := range hits {
		if !h.Admin {
			t.Fatalf("every .pgpass login is admin tier: %+v", h)
		}
		byPass[credSecret(t, h.Secret)["password"]] = h
	}
	if _, ok := byPass["PgAppPw11"]; !ok {
		t.Fatal("appuser line not captured")
	}
	esc := byPass["PgEsc13"]
	if s := credSecret(t, esc.Secret); s["host"] != "db:host" || s["db"] != `d\b` {
		t.Fatalf("escaped line parsed wrong: %v", s)
	}
}

// ---- Redis: requirepass / masterauth / ACL ----

func TestDetectRedisConf(t *testing.T) {
	content := "# Redis\nrequirepass RedisAuth14\nmasterauth RedisMaster15\nmaxmemory 256mb\n"
	hits := hitByLocator(detect("/etc/redis/redis.conf", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want requirepass + masterauth", len(hits))
	}
	if h := hits["requirepass"]; h.Kind != "env_var" || h.Secret != "RedisAuth14" || h.Admin {
		t.Fatalf("requirepass = %+v (env_var value, normal tier)", h)
	}
	if hits["masterauth"].Secret != "RedisMaster15" {
		t.Fatalf("masterauth = %+v", hits["masterauth"])
	}
}

func TestDetectRedisACL(t *testing.T) {
	content := "user default on >RedisAcl16 ~* +@all\nuser readonly on #deadbeefhash ~* +@read\n"
	hits := detect("/etc/redis/users.acl", content)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want only the >plaintext user (hash skipped)", len(hits))
	}
	if hits[0].Secret != "RedisAcl16" || hits[0].Fields["user"] != "default" {
		t.Fatalf("acl hit = %+v", hits[0])
	}
}

// ---- literal PHP connect calls ----

func TestDetectPHPLiteralConnect(t *testing.T) {
	content := `<?php
$conn = mysqli_connect('localhost', 'root', 'LiteralMy17', 'legacydb');
$pdo = new PDO('mysql:host=db2;dbname=app2', 'appuser', 'LiteralPdo18');
$bad = mysqli_connect($host, $user, $pass);
`
	hits := detect("/var/www/legacy/db.php", content)
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 literal calls (variable call skipped)", len(hits))
	}
	var my, pdo CredHit
	for _, h := range hits {
		switch credSecret(t, h.Secret)["password"] {
		case "LiteralMy17":
			my = h
		case "LiteralPdo18":
			pdo = h
		}
	}
	if !my.Admin || my.Fields["confidence"] != "low" {
		t.Fatalf("mysqli_connect = %+v, want admin (root) + low confidence", my)
	}
	if s := credSecret(t, my.Secret); s["host"] != "localhost" || s["db"] != "legacydb" || s["username"] != "root" {
		t.Fatalf("mysqli payload = %v", s)
	}
	if pdo.Admin || pdo.Fields["engine"] != "mysql" {
		t.Fatalf("PDO = %+v", pdo)
	}
	if s := credSecret(t, pdo.Secret); s["host"] != "db2" || s["db"] != "app2" || s["username"] != "appuser" {
		t.Fatalf("PDO payload = %v (host/dbname from DSN)", s)
	}
}

// ---- Leak discipline: no captured password reaches the stored provenance ----

func TestPHPDetectorsNoPasswordInProvenance(t *testing.T) {
	cases := []struct{ path, content, password string }{
		{"/var/www/ci4/app/Config/Database.php",
			"<?php class Database { public array $default = ['username' => 'root', 'password' => 'leakCi4', 'hostname' => 'h']; }", "leakCi4"},
		{"/var/www/ci3/application/config/email.php",
			"<?php $config['smtp_host']='h'; $config['smtp_user']='u'; $config['smtp_pass']='leakEmail';", "leakEmail"},
		{"/var/www/joomla/configuration.php",
			"<?php class JConfig { public $user='u'; public $password='leakJoomla'; public $host='h'; public $db='d'; }", "leakJoomla"},
		{"/var/www/drupal/sites/default/settings.php",
			"<?php $databases['default']['default'] = array('username'=>'u','password'=>'leakDrupal','host'=>'h','database'=>'d','driver'=>'mysql');", "leakDrupal"},
		{"/var/www/moodle/config.php",
			"<?php $CFG->dbuser='u'; $CFG->dbpass='leakMoodle'; $CFG->dbhost='h'; $CFG->dbname='d';", "leakMoodle"},
		{"/var/www/sc/cvi/conexao.ini", "[cvi]\nuser=\"u\"\npass=\"leakScript\"\nhost=\"h\"\n", "leakScript"},
		{"/etc/webmin/mysql/config", "login=root\npass=leakWebmin\n", "leakWebmin"},
		{"/etc/psa/.psa.shadow", "leakPlesk\n", "leakPlesk"},
		{"/root/.pgpass", "h:5432:d:u:leakPgpass\n", "leakPgpass"},
		{"/etc/redis/redis.conf", "requirepass leakRedis\n", "leakRedis"},
		{"/var/www/legacy/db.php", "<?php mysqli_connect('h','u','leakLiteral','d');", "leakLiteral"},
	}
	for _, c := range cases {
		hits := detect(c.path, c.content)
		if len(hits) == 0 {
			t.Fatalf("%s: no hit", c.path)
		}
		inSecret := false
		prov := make([]CredentialSource, len(hits))
		for i, h := range hits {
			if strings.Contains(h.Secret, c.password) {
				inSecret = true
			}
			prov[i] = CredentialSource{Detector: h.Detector, Path: h.Path, Locator: h.Locator, Kind: h.Kind, Admin: h.Admin, Fields: h.Fields}
		}
		if !inSecret {
			t.Fatalf("%s: password not captured into CredHit.Secret (leak test would be meaningless)", c.path)
		}
		b, err := json.Marshal(prov)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), c.password) {
			t.Fatalf("%s: password leaked into stored provenance: %s", c.path, b)
		}
	}
}

// ---- Fake-root probe layout (PHP apps + DB-admin/host files) ----

func TestCredProbePHPHostLayout(t *testing.T) {
	root := t.TempDir()
	// Host-level stores (static allowlist paths).
	plant(t, root, "etc/webmin/mysql/config", "login=root\npass=webPw1\n")
	plant(t, root, "etc/psa/.psa.shadow", "pleskPw2\n")
	plant(t, root, "root/.pgpass", "localhost:5432:app:u:pgPw3\n")
	plant(t, root, "etc/redis/redis.conf", "requirepass redisPw4\n")
	plant(t, root, "etc/redis/users.acl", "user default on >aclPw5 ~* +@all\n")
	// App docroots (docroot-relative allowlist).
	plant(t, root, "var/www/joomla/configuration.php",
		"<?php class JConfig { public $user='ju'; public $password='joomPw6'; public $host='h'; public $db='d'; }")
	plant(t, root, "var/www/moodle/config.php",
		"<?php $CFG->dbuser='mu'; $CFG->dbpass='moodPw7'; $CFG->dbhost='h'; $CFG->dbname='d';")
	plant(t, root, "var/www/drupal/sites/default/settings.php",
		"<?php $databases['default']['default']=array('username'=>'du','password'=>'drupPw8','host'=>'h','database'=>'d','driver'=>'mysql');")
	plant(t, root, "var/www/ci4/app/Config/Database.php",
		"<?php class Database { public array $default=['username'=>'ci4u','password'=>'ci4Pw9','hostname'=>'h','database'=>'d','DBDriver'=>'MySQLi']; }")
	plant(t, root, "var/www/ci3/application/config/email.php",
		"<?php $config['smtp_host']='h'; $config['smtp_user']='eu'; $config['smtp_pass']='emailPw10';")
	plant(t, root, "var/www/sc/cvi/conexao.ini", "[cvi]\nuser=\"su\"\npass=\"scPw11\"\nhost=\"h\"\n")
	plant(t, root, "var/www/legacy/db.php", "<?php mysqli_connect('h','lu','litPw12','d');")

	roots := []string{"/var/www/joomla", "/var/www/moodle", "/var/www/drupal", "/var/www/ci4", "/var/www/ci3", "/var/www/sc", "/var/www/legacy"}
	files := runCredProbe(t, root, roots, nil)
	got := detectCreds(files, nil, nil)

	want := []string{"webmin_mysql", "plesk_admin", "pgpass", "redis_conf", "joomla_config",
		"moodle_config", "drupal_settings", "ci4_database", "ci_email", "scriptcase_conn", "php_literal_connect"}
	set := detectorSet(got)
	for _, d := range want {
		if !set[d] {
			t.Errorf("detector %s produced no hit from the probe output (got %v)", d, keysOfBool(set))
		}
	}
	// redis_conf must fire for BOTH redis.conf and the ACL file.
	acl := false
	for _, h := range got {
		if h.Detector == "redis_conf" && h.Secret == "aclPw5" {
			acl = true
		}
	}
	if !acl {
		t.Error("redis ACL file password not captured")
	}

	// No fixture password reaches the stored provenance.
	var prov []CredentialSource
	for _, h := range got {
		prov = append(prov, CredentialSource{Detector: h.Detector, Path: h.Path, Locator: h.Locator, Kind: h.Kind, Admin: h.Admin, Fields: h.Fields})
	}
	b, _ := json.Marshal(prov)
	for _, pw := range []string{"webPw1", "pleskPw2", "pgPw3", "redisPw4", "aclPw5", "joomPw6", "moodPw7", "drupPw8", "ci4Pw9", "emailPw10", "scPw11", "litPw12"} {
		if strings.Contains(string(b), pw) {
			t.Fatalf("password %s leaked into provenance", pw)
		}
	}
}

func keysOfBool(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
