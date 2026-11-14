package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// hitByLocator indexes a detector's hits for assertions.
func hitByLocator(hits []CredHit) map[string]CredHit {
	m := map[string]CredHit{}
	for _, h := range hits {
		m[h.Locator] = h
	}
	return m
}

// credSecret unmarshals a cred payload for assertions (tests only — production
// never decodes a stored secret outside an audited reveal).
func credSecret(t *testing.T, s string) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("cred payload %q: %v", s, err)
	}
	return m
}

func detect(path, content string) []CredHit {
	return detectCredFile(credFile{Path: path, Content: content}, nil, nil)
}

func TestDetectCI3Database(t *testing.T) {
	// 'default' in the $db['g']['k'] syntax (root → admin tier); 'second' in the
	// array syntax (normal user); 'env' with a dynamic password (indirect).
	content := `<?php
$db['default']['hostname'] = 'localhost';
$db['default']['username'] = 'root';
$db['default']['password'] = 's3cr3t';
$db['default']['database'] = 'app';
$db['default']['dbdriver'] = 'mysqli';
$db['default']['port']     = '3306';

$db['second'] = array(
    'hostname' => '10.0.0.5',
    'username' => 'reader',
    'password' => 'r3ad(er)',
    'database' => 'reports',
    'dbdriver' => 'postgre',
);

$db['env']['username'] = 'u';
$db['env']['password'] = getenv('DB_PASS');
`
	hits := hitByLocator(detect("/var/www/app/application/config/database.php", content))
	if len(hits) != 3 {
		t.Fatalf("groups = %d, want 3 (default, second, env)", len(hits))
	}
	d := hits["default"]
	if !d.Admin || d.Kind != "cred" || d.Detector != "ci3_database" {
		t.Fatalf("default = %+v, want admin cred", d)
	}
	if ds := credSecret(t, d.Secret); ds["username"] != "root" || ds["password"] != "s3cr3t" || ds["db"] != "app" || ds["port"] != "3306" {
		t.Fatalf("default payload = %v", ds)
	}
	if d.Fields["engine"] != "mysql" || d.Fields["host"] != "localhost" {
		t.Fatalf("default fields = %v", d.Fields)
	}
	s := hits["second"]
	if s.Admin {
		t.Fatalf("second should not be admin tier: %+v", s)
	}
	if ss := credSecret(t, s.Secret); ss["username"] != "reader" || ss["password"] != "r3ad(er)" || ss["host"] != "10.0.0.5" {
		t.Fatalf("second (array syntax) payload = %v", ss)
	}
	if s.Fields["engine"] != "postgresql" {
		t.Fatalf("second engine = %v", s.Fields)
	}
	if e := hits["env"]; e.Secret != "" {
		t.Fatalf("env group password is dynamic — want indirect (no secret), got %q", e.Secret)
	}
	// Site is shared by all groups of the one file, derived from the app dir.
	if d.Site != s.Site || !strings.HasPrefix(d.Site, "app-") {
		t.Fatalf("site = %q / %q, want a shared app-… slug", d.Site, s.Site)
	}
}

func TestDetectDotenv(t *testing.T) {
	content := `# app config
export APP_ENV=production
DB_PASSWORD="p@ss word"
MAIL_PASSWORD=mailpw # inline comment
API_KEY='ak-123'
REDIS_URL=redis://localhost:6379
DB_HOST=127.0.0.1
SECRET_FROM_VAULT=${VAULT_SECRET}
`
	hits := hitByLocator(detect("/srv/app/.env", content))
	// Allowlisted: DB_PASSWORD, MAIL_PASSWORD, API_KEY, SECRET_FROM_VAULT.
	// Not: APP_ENV, REDIS_URL (no _URL rule shipped), DB_HOST.
	for _, skip := range []string{"APP_ENV", "DB_HOST", "REDIS_URL"} {
		if _, ok := hits[skip]; ok {
			t.Fatalf("%s should not be captured", skip)
		}
	}
	if h := hits["DB_PASSWORD"]; h.Kind != "env_var" || h.Secret != "p@ss word" {
		t.Fatalf("DB_PASSWORD = %+v (want unquoted value)", h)
	}
	if h := hits["MAIL_PASSWORD"]; h.Secret != "mailpw" {
		t.Fatalf("MAIL_PASSWORD = %q (inline comment should be stripped)", h.Secret)
	}
	if h := hits["API_KEY"]; h.Secret != "ak-123" {
		t.Fatalf("API_KEY = %q", h.Secret)
	}
	if h := hits["SECRET_FROM_VAULT"]; h.Secret != "" {
		t.Fatalf("SECRET_FROM_VAULT is ${…} — want indirect, got %q", h.Secret)
	}
}

func TestDetectWPConfig(t *testing.T) {
	content := `<?php
define('DB_NAME', 'wordpress');
define( "DB_USER" , "root" );
define('DB_PASSWORD', 'wp-secret');
define('DB_HOST', 'localhost');
define('AUTH_KEY', 'should-be-ignored-salt');
`
	hits := detect("/var/www/html/wp-config.php", content)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1 cred", len(hits))
	}
	h := hits[0]
	if h.Detector != "wp_config" || !h.Admin {
		t.Fatalf("wp-config = %+v, want admin (DB_USER root)", h)
	}
	s := credSecret(t, h.Secret)
	if s["username"] != "root" || s["password"] != "wp-secret" || s["db"] != "wordpress" {
		t.Fatalf("payload = %v", s)
	}
	if strings.Contains(h.Secret, "should-be-ignored-salt") {
		t.Fatal("salt leaked into the payload")
	}
}

func TestDetectMysqlDebianCnf(t *testing.T) {
	content := `[client]
host     = localhost
user     = debian-sys-maint
password = Zxy123abc
socket   = /var/run/mysqld/mysqld.sock
[mysql_upgrade]
user = debian-sys-maint
password = Zxy123abc
`
	hits := detect("/etc/mysql/debian.cnf", content)
	if len(hits) != 1 || hits[0].Detector != "mysql_debian_cnf" || !hits[0].Admin {
		t.Fatalf("hits = %+v, want one admin cred", hits)
	}
	if s := credSecret(t, hits[0].Secret); s["username"] != "debian-sys-maint" || s["password"] != "Zxy123abc" {
		t.Fatalf("payload = %v", s)
	}
}

func TestDetectRootMyCnf(t *testing.T) {
	content := "[client]\nuser=root\npassword=TopSecret1\n"
	for _, p := range []string{"/root/.my.cnf", "/etc/my.cnf.d/client.cnf"} {
		hits := detect(p, content)
		if len(hits) != 1 || hits[0].Detector != "mysql_root_mycnf" || !hits[0].Admin {
			t.Fatalf("%s: hits = %+v", p, hits)
		}
	}
}

func TestDetectVirtualminDomain(t *testing.T) {
	content := `dom=example.com.br
user=exampleadm
pass=OwnerPw123
mysql_pass=DbPw456
mysql_user=example_u
db=example_db
`
	hits := hitByLocator(detect("/etc/webmin/virtual-server/domains/123456789", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want owner + mysql", len(hits))
	}
	o := hits["owner"]
	if !o.Admin || o.Fields["domain"] != "example.com.br" {
		t.Fatalf("owner = %+v", o)
	}
	if s := credSecret(t, o.Secret); s["username"] != "exampleadm" || s["password"] != "OwnerPw123" {
		t.Fatalf("owner payload = %v", s)
	}
	if s := credSecret(t, hits["mysql"].Secret); s["username"] != "example_u" || s["password"] != "DbPw456" {
		t.Fatalf("mysql payload = %v", s)
	}
	// Two domain files never share a group, so their OWNER names don't collide.
	other := hitByLocator(detect("/etc/webmin/virtual-server/domains/987654321", content))
	if other["owner"].Site == o.Site {
		t.Fatal("distinct domain files must get distinct sites")
	}
}

func TestDetectPostfixSasl(t *testing.T) {
	content := "[smtp.gmail.com]:587 relay@example.com:appPassword123\n# comment\nsmtp.other.com otheruser:otherpw\n"
	hits := hitByLocator(detect("/etc/postfix/sasl_passwd", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 relays", len(hits))
	}
	var gmail CredHit
	for _, h := range hits {
		if h.Fields["host"] == "[smtp.gmail.com]:587" {
			gmail = h
		}
	}
	if gmail.Admin || gmail.Detector != "postfix_sasl" {
		t.Fatalf("gmail relay = %+v (SMTP relay is the normal tier)", gmail)
	}
	if s := credSecret(t, gmail.Secret); s["username"] != "relay@example.com" || s["password"] != "appPassword123" {
		t.Fatalf("gmail payload = %v", s)
	}
}

func TestDetectCronEmbedded(t *testing.T) {
	script := `#!/bin/sh
PGPASSWORD=pgsecret pg_dump mydb > /tmp/d.sql
mysqldump -uroot -prootdumppw app > /tmp/a.sql
sshpass -p sshsecret scp /tmp/a.sql user@host:/bak/
`
	hits := hitByLocator(detectCredFile(credFile{Path: "/opt/bkp.sh", Content: script},
		map[string]bool{"/opt/bkp.sh": true}, nil))
	if h, ok := hits["pgpassword"]; !ok || h.Secret != "pgsecret" || h.Kind != "env_var" {
		t.Fatalf("pgpassword = %+v", h)
	}
	if h, ok := hits["mysql_password"]; !ok || h.Secret != "rootdumppw" || !h.Admin {
		t.Fatalf("mysql_password = %+v (root dump → admin tier)", h)
	}
	if h, ok := hits["sshpass"]; !ok || h.Secret != "sshsecret" {
		t.Fatalf("sshpass = %+v", h)
	}
}

func TestDetectRootSSHKey(t *testing.T) {
	// A real (unencrypted) OpenSSH key: the public half is derived, not encrypted.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(block))
	hits := detect("/root/.ssh/id_ed25519", pemKey)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1 sshkey", len(hits))
	}
	h := hits[0]
	if h.Detector != "root_ssh_keys" || h.Kind != "sshkey" || !h.Admin || h.Locator != "id_ed25519" {
		t.Fatalf("key hit = %+v", h)
	}
	var kp struct {
		Username, PrivateKeyPEM, PublicKey string
	}
	if err := json.Unmarshal([]byte(h.Secret), &struct {
		Username      *string `json:"username"`
		PrivateKeyPEM *string `json:"private_key_pem"`
		PublicKey     *string `json:"public_key"`
	}{&kp.Username, &kp.PrivateKeyPEM, &kp.PublicKey}); err != nil {
		t.Fatal(err)
	}
	if kp.Username != "root" || !strings.Contains(kp.PrivateKeyPEM, "PRIVATE KEY") || !strings.HasPrefix(kp.PublicKey, "ssh-ed25519 ") {
		t.Fatalf("payload user=%q pub=%q", kp.Username, kp.PublicKey)
	}
	if h.Fields["encrypted"] != "" {
		t.Fatalf("unencrypted key should not be flagged encrypted: %v", h.Fields)
	}
	// A .pub or non-key file is never captured as a key.
	if got := detect("/root/.ssh/id_rsa", "ssh-rsa AAAAB3Nz... user@host"); len(got) != 0 {
		t.Fatalf("a public key must not be captured: %+v", got)
	}
}

// An encrypted key is captured anyway (the passphrase is unknown), flagged.
func TestDetectRootSSHKeyEncrypted(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := gossh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("hunter2"))
	if err != nil {
		t.Skipf("MarshalPrivateKeyWithPassphrase unavailable: %v", err)
	}
	hits := detect("/root/.ssh/id_rsa", string(pem.EncodeToMemory(block)))
	if len(hits) != 1 || hits[0].Fields["encrypted"] != "true" {
		t.Fatalf("encrypted key = %+v, want encrypted=true and still stored", hits)
	}
	if !strings.Contains(hits[0].Secret, "PRIVATE KEY") {
		t.Fatal("encrypted key not stored")
	}
}
