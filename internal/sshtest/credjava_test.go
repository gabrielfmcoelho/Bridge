package sshtest

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---- Spring Boot ----

func TestDetectSpringProperties(t *testing.T) {
	content := `spring.datasource.url=jdbc:mysql://localhost:3306/app
spring.datasource.username=root
spring.datasource.password=springRoot1
spring.datasource.hikari.maximum-pool-size=10
spring.mail.host=smtp.local
spring.mail.username=notify
spring.mail.password=mailPw3
spring.redis.host=127.0.0.1
spring.redis.port=6379
spring.redis.password=redisPw4
`
	hits := hitByLocator(detect("/opt/app/application.properties", content))
	ds := hits["datasource"]
	if !ds.Admin || ds.Detector != "spring_props" {
		t.Fatalf("datasource = %+v, want admin spring_props (root user)", ds)
	}
	if s := credSecret(t, ds.Secret); s["username"] != "root" || s["password"] != "springRoot1" {
		t.Fatalf("datasource payload = %v", s)
	}
	if ds.Fields["engine"] != "mysql" {
		t.Fatalf("datasource engine = %v", ds.Fields)
	}
	if m := hits["mail"]; m.Admin || credSecret(t, m.Secret)["password"] != "mailPw3" {
		t.Fatalf("mail = %+v", m)
	}
	if r := hits["redis"]; credSecret(t, r.Secret)["password"] != "redisPw4" || r.Fields["host"] != "127.0.0.1" {
		t.Fatalf("redis = %+v", r)
	}
}

func TestDetectSpringYAML(t *testing.T) {
	content := `spring:
  datasource:
    url: jdbc:postgresql://db.local:5432/app
    username: appuser
    password: "yamlSecret1"
  mail:
    host: smtp.local
    username: mailer
    password: mailYaml2 # inline comment
`
	hits := hitByLocator(detect("/srv/svc/config/application.yml", content))
	ds := hits["datasource"]
	if ds.Fields["engine"] != "postgresql" {
		t.Fatalf("yaml datasource engine = %v", ds.Fields)
	}
	if s := credSecret(t, ds.Secret); s["username"] != "appuser" || s["password"] != "yamlSecret1" {
		t.Fatalf("yaml datasource payload = %v (quotes should be stripped)", s)
	}
	if s := credSecret(t, hits["mail"].Secret); s["password"] != "mailYaml2" {
		t.Fatalf("yaml mail password = %q (inline comment should be stripped)", s["password"])
	}
}

func TestDetectSpringProfileAndIndirect(t *testing.T) {
	content := "spring.datasource.url=jdbc:mysql://h/app\nspring.datasource.username=app\nspring.datasource.password=${DB_PASSWORD}\n"
	hits := detect("/opt/app/application-prod.properties", content)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	h := hits[0]
	if h.Locator != "datasource-prod" {
		t.Fatalf("locator = %q, want datasource-prod (profile in locator)", h.Locator)
	}
	if h.Secret != "" {
		t.Fatalf("placeholder password must be indirect, got %q", h.Secret)
	}
	if h.Fields["note"] == "" {
		t.Fatalf("indirect password should carry a note: %v", h.Fields)
	}
}

// ---- Tomcat ----

func TestDetectTomcatResource(t *testing.T) {
	content := `<Context>
  <Resource name="jdbc/appDS" auth="Container" type="javax.sql.DataSource"
    driverClassName="com.mysql.jdbc.Driver"
    url="jdbc:mysql://localhost:3306/app"
    username="appuser" password="tomcatPw1" maxTotal="20"/>
</Context>`
	hits := detect("/opt/tomcat/conf/context.xml", content)
	if len(hits) != 1 || hits[0].Detector != "tomcat_resource" {
		t.Fatalf("hits = %+v, want one tomcat_resource", hits)
	}
	h := hits[0]
	if h.Fields["engine"] != "mysql" || h.Fields["user"] != "appuser" {
		t.Fatalf("fields = %v", h.Fields)
	}
	if s := credSecret(t, h.Secret); s["password"] != "tomcatPw1" {
		t.Fatalf("payload = %v", s)
	}
}

func TestDetectTomcatUsers(t *testing.T) {
	content := `<tomcat-users>
  <role rolename="manager-gui"/>
  <user username="mgr" password="mgrPw2" roles="manager-gui,admin-gui"/>
  <user username="guest" password="guestPw" roles="tomcat"/>
</tomcat-users>`
	hits := hitByLocator(detect("/opt/tomcat/conf/tomcat-users.xml", content))
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want only the privileged user", len(hits))
	}
	h := hits["mgr"]
	if !h.Admin || h.Detector != "tomcat_users" {
		t.Fatalf("mgr = %+v, want admin tier", h)
	}
	if s := credSecret(t, h.Secret); s["password"] != "mgrPw2" {
		t.Fatalf("mgr payload = %v", s)
	}
	if _, ok := hits["guest"]; ok {
		t.Fatal("a user without a manager/admin role must not be captured")
	}
}

// ---- WildFly ----

func TestDetectWildflyDS(t *testing.T) {
	content := `<datasources>
  <datasource jndi-name="java:/AppDS" pool-name="AppDS">
    <connection-url>jdbc:mysql://localhost:3306/app</connection-url>
    <driver>mysql</driver>
    <security>
      <user-name>wfuser</user-name>
      <password>wildflyPw1</password>
    </security>
  </datasource>
  <datasource jndi-name="java:/VaultDS" pool-name="VaultDS">
    <connection-url>jdbc:postgresql://db:5432/v</connection-url>
    <security>
      <user-name>vuser</user-name>
      <password>${VAULT::ds::pw::abc}</password>
    </security>
  </datasource>
</datasources>`
	hits := hitByLocator(detect("/opt/wildfly/standalone/configuration/standalone.xml", content))
	app := hits["appds"]
	if app.Detector != "wildfly_ds" || credSecret(t, app.Secret)["password"] != "wildflyPw1" {
		t.Fatalf("AppDS = %+v", app)
	}
	v := hits["vaultds"]
	if v.Secret != "" {
		t.Fatalf("vault-expression password must not be resolved, got %q", v.Secret)
	}
	if !strings.Contains(v.Fields["note"], "vault expression") {
		t.Fatalf("VaultDS note = %v, want vault expression", v.Fields)
	}
}

// ---- WebLogic: {AES} recorded, never decoded ----

func TestDetectWeblogicJDBC(t *testing.T) {
	content := `<jdbc-data-source>
  <name>AppJDBC</name>
  <jdbc-driver-params>
    <url>jdbc:oracle:thin:@db:1521:ORCL</url>
    <password-encrypted>{AES}abc123encryptedBlob==</password-encrypted>
    <properties>
      <property><name>user</name><value>oracleapp</value></property>
    </properties>
  </jdbc-driver-params>
</jdbc-data-source>`
	hits := detect("/u01/domains/base/config/jdbc/App-jdbc.xml", content)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	h := hits[0]
	if h.Detector != "weblogic_jdbc" || h.Fields["engine"] != "oracle" || h.Fields["user"] != "oracleapp" {
		t.Fatalf("weblogic = %+v", h)
	}
	if h.Fields["encrypted"] != "true" {
		t.Fatalf("AES password must be flagged encrypted: %v", h.Fields)
	}
	// Recorded verbatim, not decoded.
	if s := credSecret(t, h.Secret); s["password"] != "{AES}abc123encryptedBlob==" {
		t.Fatalf("AES blob should be stored as-is, got %q", s["password"])
	}
}

func TestDetectWeblogicBoot(t *testing.T) {
	content := "username={AES}encUserBlob==\npassword={AES}encPassBlob==\n"
	hits := detect("/u01/domains/base/servers/AdminServer/security/boot.properties", content)
	if len(hits) != 1 || hits[0].Locator != "boot" || !hits[0].Admin {
		t.Fatalf("boot = %+v, want one admin hit", hits)
	}
	if hits[0].Fields["encrypted"] != "true" {
		t.Fatalf("boot.properties AES should be flagged: %v", hits[0].Fields)
	}
	if s := credSecret(t, hits[0].Secret); s["password"] != "{AES}encPassBlob==" {
		t.Fatalf("boot payload = %v", s)
	}
}

// ---- Hibernate / JDBC properties ----

func TestDetectHibernateXML(t *testing.T) {
	content := `<hibernate-configuration><session-factory>
  <property name="hibernate.connection.url">jdbc:mysql://localhost/app</property>
  <property name="hibernate.connection.username">hbuser</property>
  <property name="hibernate.connection.password">hibernatePw1</property>
</session-factory></hibernate-configuration>`
	hits := detect("/opt/app/hibernate.cfg.xml", content)
	if len(hits) != 1 || hits[0].Detector != "hibernate_jdbc_props" {
		t.Fatalf("hits = %+v", hits)
	}
	if s := credSecret(t, hits[0].Secret); s["username"] != "hbuser" || s["password"] != "hibernatePw1" {
		t.Fatalf("payload = %v", s)
	}
}

func TestDetectPersistenceXML(t *testing.T) {
	content := `<persistence><persistence-unit name="pu"><properties>
  <property name="javax.persistence.jdbc.url" value="jdbc:postgresql://db/app"/>
  <property name="javax.persistence.jdbc.user" value="jpauser"/>
  <property name="javax.persistence.jdbc.password" value="jpaPw2"/>
</properties></persistence-unit></persistence>`
	hits := detect("/opt/app/META-INF/persistence.xml", content)
	if len(hits) != 1 {
		t.Fatalf("hits = %d", len(hits))
	}
	if s := credSecret(t, hits[0].Secret); s["username"] != "jpauser" || s["password"] != "jpaPw2" {
		t.Fatalf("payload = %v (inline value form)", s)
	}
}

func TestDetectJDBCProperties(t *testing.T) {
	content := "jdbc.url=jdbc:mariadb://localhost/app\njdbc.username=jp\njdbc.password=propsPw3\n"
	hits := detect("/opt/app/jdbc.properties", content)
	if len(hits) != 1 || hits[0].Fields["engine"] != "mysql" {
		t.Fatalf("hits = %+v (mariadb → mysql engine)", hits)
	}
	if credSecret(t, hits[0].Secret)["password"] != "propsPw3" {
		t.Fatalf("payload = %v", credSecret(t, hits[0].Secret))
	}
}

// ---- Oracle tnsnames: metadata only, never a secret ----

func TestDetectOracleTNS(t *testing.T) {
	content := `ORCL =
  (DESCRIPTION =
    (ADDRESS = (PROTOCOL = TCP)(HOST = oradb.local)(PORT = 1521))
    (CONNECT_DATA = (SERVICE_NAME = orcl.local))
  )
PROD.WORLD =
  (DESCRIPTION =
    (ADDRESS = (PROTOCOL = TCP)(HOST = 10.0.0.9)(PORT = 1522))
    (CONNECT_DATA = (SID = PRODSID))
  )
`
	hits := hitByLocator(detect("/u01/app/oracle/product/19.0.0/dbhome_1/network/admin/tnsnames.ora", content))
	if len(hits) != 2 {
		t.Fatalf("aliases = %d, want 2", len(hits))
	}
	o := hits["orcl"]
	if o.Secret != "" {
		t.Fatalf("tnsnames is metadata, never a secret: %q", o.Secret)
	}
	if o.Fields["host"] != "oradb.local" || o.Fields["port"] != "1521" || o.Fields["service"] != "orcl.local" {
		t.Fatalf("ORCL fields = %v", o.Fields)
	}
	if p := hits["prod-world"]; p.Fields["host"] != "10.0.0.9" || p.Fields["service"] != "PRODSID" {
		t.Fatalf("PROD.WORLD fields = %v", p.Fields)
	}
}

// ---- Leak discipline: no captured password reaches the stored provenance ----

// A fixture password lives only in CredHit.Secret (transient). The stored shape
// — CredentialSource (detector/path/locator/kind/admin/fields/token) — must
// never contain it; recordCredentialSources stores exactly that.
func TestJavaDetectorsNoPasswordInProvenance(t *testing.T) {
	cases := []struct{ path, content, password string }{
		{"/opt/app/application.properties",
			"spring.datasource.username=root\nspring.datasource.password=leakSpring1\n", "leakSpring1"},
		{"/opt/tomcat/conf/context.xml",
			`<Resource name="jdbc/x" username="u" password="leakTomcat2" url="jdbc:mysql://h/d"/>`, "leakTomcat2"},
		{"/opt/tomcat/conf/tomcat-users.xml",
			`<user username="mgr" password="leakUsers3" roles="manager-gui"/>`, "leakUsers3"},
		{"/opt/wf/standalone/configuration/standalone.xml",
			`<datasource pool-name="D"><connection-url>jdbc:mysql://h/d</connection-url><security><user-name>u</user-name><password>leakWildfly4</password></security></datasource>`, "leakWildfly4"},
		{"/opt/app/hibernate.cfg.xml",
			`<property name="hibernate.connection.password">leakHib5</property>`, "leakHib5"},
	}
	for _, c := range cases {
		hits := detect(c.path, c.content)
		if len(hits) == 0 {
			t.Fatalf("%s: no hit", c.path)
		}
		inSecret := false
		for _, h := range hits {
			if strings.Contains(h.Secret, c.password) {
				inSecret = true
			}
		}
		if !inSecret {
			t.Fatalf("%s: password not captured into CredHit.Secret (leak test would be meaningless)", c.path)
		}
		// The provenance stored in the scan JSON: everything but Secret.
		prov := make([]CredentialSource, len(hits))
		for i, h := range hits {
			prov[i] = CredentialSource{Detector: h.Detector, Path: h.Path, Locator: h.Locator, Kind: h.Kind, Admin: h.Admin, Fields: h.Fields}
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

// ---- Fake-root probe layouts (Oracle Linux, CentOS 7) ----

func TestCredProbeOracleLinuxLayout(t *testing.T) {
	root := t.TempDir()
	// Oracle DB home tnsnames (static glob), and a WebLogic domain (process cwd).
	plant(t, root, "u01/app/oracle/product/19.0.0/dbhome_1/network/admin/tnsnames.ora",
		"ORCL = (DESCRIPTION = (ADDRESS=(HOST=oradb)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=orcl)))\n")
	plant(t, root, "u01/domains/base/config/jdbc/App-jdbc.xml",
		"<jdbc-data-source><jdbc-driver-params><url>jdbc:oracle:thin:@db:1521:ORCL</url><password-encrypted>{AES}encBlob==</password-encrypted><properties><property><name>user</name><value>oracleapp</value></property></properties></jdbc-driver-params></jdbc-data-source>\n")
	plant(t, root, "u01/domains/base/servers/AdminServer/security/boot.properties",
		"username={AES}u==\npassword={AES}p==\n")

	files := runCredProbe(t, root, []string{"/u01/domains/base"}, nil)
	hits := detectorSet(detectCreds(files, nil, nil))
	for _, d := range []string{"oracle_tns", "weblogic_jdbc"} {
		if !hits[d] {
			t.Errorf("Oracle Linux layout: detector %s produced no hit (got %v)", d, hits)
		}
	}
}

func TestCredProbeCentos7Layout(t *testing.T) {
	root := t.TempDir()
	plant(t, root, "opt/tomcat/conf/context.xml",
		`<Context><Resource name="jdbc/app" username="u" password="centosPw1" url="jdbc:mysql://h/d"/></Context>`)
	plant(t, root, "opt/tomcat/conf/tomcat-users.xml",
		`<tomcat-users><user username="mgr" password="centosMgr2" roles="manager-gui"/></tomcat-users>`)
	plant(t, root, "opt/springapp/application.properties",
		"spring.datasource.username=app\nspring.datasource.password=centosSpring3\n")

	files := runCredProbe(t, root, []string{"/opt/tomcat", "/opt/springapp"}, nil)
	hits := detectorSet(detectCreds(files, nil, nil))
	for _, d := range []string{"tomcat_resource", "tomcat_users", "spring_props"} {
		if !hits[d] {
			t.Errorf("CentOS 7 layout: detector %s produced no hit (got %v)", d, hits)
		}
	}
	// The passwords travel in the transient file content but never in the stored
	// provenance (checked end-to-end here).
	var prov []CredentialSource
	for _, h := range detectCreds(files, nil, nil) {
		prov = append(prov, CredentialSource{Detector: h.Detector, Path: h.Path, Locator: h.Locator, Kind: h.Kind, Admin: h.Admin, Fields: h.Fields})
	}
	b, _ := json.Marshal(prov)
	for _, pw := range []string{"centosPw1", "centosMgr2", "centosSpring3"} {
		if strings.Contains(string(b), pw) {
			t.Fatalf("password %s leaked into provenance", pw)
		}
	}
}

// Spring Boot fat jar: the probe reads BOOT-INF/classes/application.* with unzip.
func TestCredProbeFatJar(t *testing.T) {
	unzip, err := exec.LookPath("unzip")
	if err != nil {
		t.Skip("unzip not available")
	}
	bin := credFakeHost(t)
	if err := os.Symlink(unzip, filepath.Join(bin, "unzip")); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	jarRel := "opt/app/app.jar"
	writeJar(t, filepath.Join(root, jarRel), map[string]string{
		"BOOT-INF/classes/application.yml": "spring:\n  datasource:\n    username: jaruser\n    password: jarSecret1\n    url: jdbc:mysql://h/d\n",
	})

	files := runCredProbeJars(t, root, nil, nil, []string{"/" + jarRel}, bin)
	var member *credFile
	for i := range files {
		if strings.Contains(files[i].Path, "!/BOOT-INF/classes/application.yml") {
			member = &files[i]
		}
	}
	if member == nil {
		t.Fatalf("fat jar member not streamed; got %v", pathsOf(files))
	}
	hits := detectCreds([]credFile{*member}, nil, nil)
	if len(hits) == 0 || hits[0].Detector != "spring_props" {
		t.Fatalf("fat jar member not detected as spring_props: %+v", hits)
	}
	if credSecret(t, hits[0].Secret)["password"] != "jarSecret1" {
		t.Fatalf("fat jar payload = %v", credSecret(t, hits[0].Secret))
	}
}

// Probe portability: the script is POSIX sh (no awk); it must run under busybox
// sh / dash too, not just the dev machine's /bin/sh.
func TestCredProbePortableShell(t *testing.T) {
	ran := false
	for _, sh := range []string{"dash", "busybox"} {
		p, err := exec.LookPath(sh)
		if err != nil {
			continue
		}
		ran = true
		root := t.TempDir()
		plant(t, root, "opt/app/application.properties", "spring.datasource.username=app\nspring.datasource.password=portPw1\n")
		args := []string{"-c", credProbeScript([]string{"/opt/app"}, nil, nil)}
		if sh == "busybox" {
			args = append([]string{"sh"}, args...)
		}
		cmd := exec.Command(p, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: probe failed: %v\n%s", sh, err, out)
		}
		if !strings.Contains(string(out), "@@CSBEGIN") {
			t.Fatalf("%s: no @@CSBEGIN", sh)
		}
		hits := detectorSet(detectCreds(parseCredProbe(string(out)), nil, nil))
		if !hits["spring_props"] {
			t.Fatalf("%s: spring_props not detected under %s", sh, sh)
		}
	}
	if !ran {
		t.Skip("neither dash nor busybox available")
	}
}

func detectorSet(hits []CredHit) map[string]bool {
	m := map[string]bool{}
	for _, h := range hits {
		m[h.Detector] = true
	}
	return m
}

func pathsOf(files []credFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}

func writeJar(t *testing.T, name string, members map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for n, c := range members {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
