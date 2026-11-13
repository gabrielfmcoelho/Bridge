package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/dbtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// compras1 is one of SAPASP-COMPRAS01's JBoss instances: the Redis password
// passed as a system property, next to a port that must stay visible.
const compras1 = "/opt/jdk1.7.0_79/bin/java -Dprogram.name=run.sh -DcacheServerPassword=abc123 -DcacheServerPort=6379 -classpath /opt/compras1/bin/run.jar:/opt/jdk1.7.0_79/lib/tools.jar org.jboss.Main -c default -b 0.0.0.0"

func TestFindSecrets(t *testing.T) {
	type kv struct{ Key, Value string }
	for _, c := range []struct {
		cmd  string
		want []kv
	}{
		{"java -DcacheServerPassword=abc123 -DcacheServerPort=6379", []kv{{"cacheServerPassword", "abc123"}}},
		{"app --password=s3cr3t", []kv{{"password", "s3cr3t"}}},
		{"app --db-password s3cr3t --db-host x", []kv{{"db-password", "s3cr3t"}}},
		{"PGPASSWORD=pw /usr/bin/pg_dump x", []kv{{"PGPASSWORD", "pw"}}},
		// A URL secret is named after its host only: the user part may itself be the credential.
		{"curl https://bob:pa55@git.example/x", []kv{{"url-git.example", "pa55"}}},
		// A token used as the URL's user, bare or with GitHub's placeholder password.
		{"git clone https://ghp_0123456789abcdefABCDEF0123456789abcd@github.com/o/r", []kv{{"url-github.com", "ghp_0123456789abcdefABCDEF0123456789abcd"}}},
		{"git clone https://glpat-AbCd123@gitlab.example/o/r.git", []kv{{"url-gitlab.example", "glpat-AbCd123"}}},
		{"curl https://0123456789abcdef0123456789abcdef:x-oauth-basic@api.github.com/x", []kv{{"url-api.github.com", "0123456789abcdef0123456789abcdef"}}},
		{"curl https://0123456789abcdef0123456789abcdef:hunter2@h/x", []kv{{"url-h", "0123456789abcdef0123456789abcdef"}, {"url-h", "hunter2"}}},
		// A login name is not a secret.
		{"git clone ssh://git@gitlab.example/o/r", nil},
		{"curl https://admin@intranet/x", nil},
		// curl -u user:password: the password, never the user.
		{"curl -s -u admin:S3cret https://x", []kv{{"curl-password", "S3cret"}}},
		{"curl --user admin:S3cret https://x", []kv{{"curl-password", "S3cret"}}},
		{"/usr/bin/curl --user=admin:S3cret https://x", []kv{{"curl-password", "S3cret"}}},
		{"curl -uadmin:S3cret https://x", []kv{{"curl-password", "S3cret"}}},
		{"curl -u admin https://x", nil},
		{"app -u admin:x", nil},
		{"mysqldump -uroot -pS3cret db", []kv{{"mysql-password", "S3cret"}}},
		// Go and Python split userinfo at the last "@": the whole password is the value.
		{"psql postgres://app:P@ssw0rd@db:5432/x", []kv{{"url-db", "P@ssw0rd"}}},
		{"java -Dpassword=${DB_PASS}", nil},
		{"app --token=$TOKEN", nil},
		{`PGPASSWORD="$PGPASS" pg_dump`, nil},
		{"java -DcacheServerPassword=", nil},
		// In a ps line "$" is literal: only $NAME / ${NAME} are references.
		{"java -Dpassword=$3cr3t", []kv{{"password", "$3cr3t"}}},
		{"app --password=$2y$10$abc", []kv{{"password", "$2y$10$abc"}}},
		// Already redacted: a backfill re-run must not take the token for a value.
		{"java -DcacheServerPassword={{secret:7}} --token {{secret}}", nil},
		{`x PASSWORD="{{secret:9}} d`, nil},
		// …but only a whole token is one.
		{"java -Dpassword={{secret}}realvalue", []kv{{"password", "{{secret}}realvalue"}}},
		// Assignments inside a quoted sh -c, a subshell, after ";" or in a query string.
		{`0 2 * * * bash -c "PGPASSWORD=secret pg_dump db"`, []kv{{"PGPASSWORD", "secret"}}},
		{`su - postgres -c 'PGPASSWORD=secret pg_dump db'`, []kv{{"PGPASSWORD", "secret"}}},
		{"(cd /x;A=1;DB_TOKEN=abc ./run)", []kv{{"DB_TOKEN", "abc"}}},
		{"curl https://api.example/x?a=1&token=abc", []kv{{"token", "abc"}}},
		// …and the first parameter of a query string, after "?".
		{"java -Ddb.url=jdbc:postgresql://db/x?password=p -Dx=1", []kv{{"password", "p"}}},
		{"curl https://api/x?access_token=abc", []kv{{"access_token", "abc"}}},
		{`sh -c "--token abc"`, []kv{{"token", "abc"}}},
		{`0 3 * * * bash -c "app --password=s3cr3t"`, []kv{{"password", "s3cr3t"}}},
		// A crontab line: the assignment follows the schedule.
		{"0 2 * * * PGPASSWORD=pw /usr/bin/pg_dump x", []kv{{"PGPASSWORD", "pw"}}},
		{`PGPASSWORD='pw' pg_dump`, []kv{{"PGPASSWORD", "pw"}}},
		{`DB_PASSWORD="two words" ./run.sh`, []kv{{"DB_PASSWORD", "two words"}}},
		{"app --password --token abc", []kv{{"token", "abc"}}},
		// A value starting inside another and ending past it: one span covers
		// both, its opening quote too, so no lone quote is left to re-parse.
		{`x PASSWORD="a --token=b"c d`, []kv{{"PASSWORD", `"a --token=b"c`}}},
		// Values that touch (one ends where the next starts) are one span too:
		// tokens side by side would re-parse as a value.
		{`-Dpassword="https://u:"|--token=@h`, []kv{{"password", `"https://u:"|--token=@h`}}},
		{"mysql -P3306 -p -h db", nil},
		{"tar -pczf x.tgz /srv", nil},
		{"app -Dhttp.port=8080 --user admin", nil},
	} {
		got := FindSecrets(c.cmd)
		var kvs []kv
		for _, f := range got {
			kvs = append(kvs, kv{f.Key, f.Value})
			if c.cmd[f.Start:f.End] != f.Value {
				t.Errorf("%q: span [%d:%d] = %q, value %q", c.cmd, f.Start, f.End, c.cmd[f.Start:f.End], f.Value)
			}
		}
		if !reflect.DeepEqual(kvs, c.want) {
			t.Errorf("FindSecrets(%q) = %v, want %v", c.cmd, kvs, c.want)
		}
		// Redacted, the line holds nothing a re-run would take for a value.
		red := redactSpans(c.cmd, got, func(FoundSecret) string { return "{{secret:9}}" })
		if again := FindSecrets(red); len(again) != 0 {
			t.Errorf("FindSecrets(%q) after redaction = %v", red, again)
		}
	}
}

func TestEnvVarName(t *testing.T) {
	for key, want := range map[string]string{
		"cacheServerPassword":              "CACHE_SERVER_PASSWORD",
		"javax.net.ssl.trustStorePassword": "JAVAX_NET_SSL_TRUST_STORE_PASSWORD",
		"db-password":                      "DB_PASSWORD",
		"url-git.example":                  "URL_GIT_EXAMPLE",
		"PGPASSWORD":                       "PGPASSWORD",
		"url--10.0.0.5":                    "URL_10_0_0_5",
		"2fa-token":                        "_2FA_TOKEN",
	} {
		if got := EnvVarName(key); got != want {
			t.Errorf("EnvVarName(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestSecretOrigin(t *testing.T) {
	for cmd, want := range map[string]string{
		compras1: "scan-opt-compras1",
		"/usr/lib/jvm/java-11/bin/java -Dcatalina.base=/srv/tomcat9 -Dcatalina.home=/usr/share/tomcat9 org.apache.catalina.startup.Bootstrap start": "scan-srv-tomcat9",
		"python3 /srv/app/manage.py runserver --password=x":  "scan-manage-py",
		"/usr/local/bin/foo --token=x":                       "scan-foo",
		"java -Xmx1g -jar /opt/api/api-1.2.jar --token=x":    "scan-api-1-2-jar",
		"PGPASSWORD=pw /usr/bin/pg_dump x":                   "scan-pg-dump",
		"/usr/bin/python3.11 -m gunicorn app:app --secret=x": "scan-gunicorn",
		// A value is never the origin.
		"node --secret Secr3t server.js":                 "scan-server-js",
		"php -d mysqli.default_pw=Secr3t /srv/app/x.php": "scan-x-php",
		`DB_PASSWORD="two words" ./run.sh`:               "scan-run-sh",
	} {
		if got := SecretOrigin(cmd); got != want {
			t.Errorf("SecretOrigin(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestRedactOnlyInsideFlag(t *testing.T) {
	cmd := "java -Dpassword=6379 -DcacheServerPort=6379"
	got := redactSpans(cmd, FindSecrets(cmd), func(FoundSecret) string { return "{{secret:1}}" })
	if want := "java -Dpassword={{secret:1}} -DcacheServerPort=6379"; got != want {
		t.Fatalf("redactSpans = %q, want %q", got, want)
	}
}

func newScanSecretsFixture(t *testing.T) (*database.DB, int64, int64) {
	t.Helper()
	d := dbtest.New(t)
	var uid, hid int64
	if err := d.SQL.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ('alice','x','admin') RETURNING id`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := d.SQL.QueryRow(`INSERT INTO hosts (nickname, oficial_slug, hostname) VALUES ('compras01','compras01','10.0.0.1') RETURNING id`).Scan(&hid); err != nil {
		t.Fatal(err)
	}
	return d, hid, uid
}

// scanSecretRows lists the live scan secrets as "group/NAME" → id.
func scanSecretRows(t *testing.T, d *database.DB) map[string]int64 {
	t.Helper()
	rows, err := d.SQL.Query(`SELECT id, group_label, name FROM secrets WHERE deleted_at IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var g, n string
		if err := rows.Scan(&id, &g, &n); err != nil {
			t.Fatal(err)
		}
		out[g+"/"+n] = id
	}
	return out
}

// planted builds the scan: the JBoss process, the same line cut short by
// resource_top (once after the value, once inside it), a resource_top line
// cut inside a URL's password (no "@" left), a root cron job and the legacy
// crontab lines.
func planted() *sshtest.VMInfo {
	i := strings.Index(compras1, "abc123")
	return &sshtest.VMInfo{
		ProcessDetails: []sshtest.ProcessDetail{{PID: "1", User: "jboss", Command: compras1, Unit: "compras1.service"}},
		ResourceTop: &sshtest.ResourceUsageSnapshot{
			TopCPU: []sshtest.ResourceProcess{
				{PID: 1, Command: compras1[:i+12] + "…"},
				{PID: 2, Command: "curl -s https://deploy:SuperSecretPassw…"},
			},
			TopMem: []sshtest.ResourceProcess{{PID: 1, Command: compras1[:i+4] + "…"}},
		},
		Cron: &sshtest.CronInfo{Jobs: []sshtest.CronJob{
			{Source: "system", Kind: "cron", Schedule: "0 2 * * *", User: "root", Command: "PGPASSWORD=pw /usr/bin/pg_dump x"},
		}},
		CronJobs: []string{"0 2 * * * PGPASSWORD=pw /usr/bin/pg_dump x", "DB_PASSWORD=s3cr3t"},
	}
}

// A resource_top line cut inside a token used as the URL's user (no "@"
// left) is blanked from the "//" on; a cut host name is not a secret.
func TestRecordScanSecrets_CutURLToken(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	info := &sshtest.VMInfo{ResourceTop: &sshtest.ResourceUsageSnapshot{TopCPU: []sshtest.ResourceProcess{
		{PID: 1, Command: "git clone https://ghp_0123456789abcd…"},
		{PID: 2, Command: "curl -s http://sistemas.pi.gov.b…"},
	}}}
	if _, err := RecordScanSecrets(context.Background(), d, hid, uid, info); err != nil {
		t.Fatal(err)
	}
	top := info.ResourceTop.TopCPU
	if got := top[0].Command; got != "git clone https://{{secret}}…" {
		t.Errorf("cut inside a URL token = %q", got)
	}
	if got := top[1].Command; got != "curl -s http://sistemas.pi.gov.b…" {
		t.Errorf("cut inside a host name = %q", got)
	}
}

func assertRedacted(t *testing.T, info *sshtest.VMInfo) string {
	t.Helper()
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, v := range []string{"abc123", "abc1", "pw ", "s3cr3t", "SuperSecret"} {
		if strings.Contains(out, v) {
			t.Errorf("scan JSON still holds %q: %s", v, out)
		}
	}
	return out
}

func TestRecordScanSecrets(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx := context.Background()

	info := planted()
	n, err := RecordScanSecrets(ctx, d, hid, uid, info)
	if err != nil || n != 2 {
		t.Fatalf("RecordScanSecrets = %d, %v; want 2, nil", n, err)
	}
	if !info.SecretsRedacted {
		t.Error("scan not marked secrets_redacted: the backfill would read it again")
	}
	rows := scanSecretRows(t, d)
	cache, pg := rows["scan-opt-compras1/CACHE_SERVER_PASSWORD"], rows["scan-cron-root/PGPASSWORD"]
	if len(rows) != 2 || cache == 0 || pg == 0 {
		t.Fatalf("vault rows = %v", rows)
	}
	var desc string
	_ = d.SQL.QueryRow(`SELECT description FROM secrets WHERE id = ?`, cache).Scan(&desc)
	if desc != "-DcacheServerPassword · java · /opt/compras1" {
		t.Errorf("description = %q", desc)
	}
	admin := vault.ActorContext{UserID: uid, Role: "admin"}
	if got, err := vault.NewSecretRepo(d).Reveal(ctx, admin, cache); err != nil || got != `{"value":"abc123"}` {
		t.Fatalf("reveal = %q, %v", got, err)
	}

	out := assertRedacted(t, info)
	if !strings.Contains(out, "{{secret:") {
		t.Fatalf("no token in %s", out)
	}
	tok := func(id int64) string { return fmt.Sprintf("{{secret:%d}}", id) }
	if got, want := info.ProcessDetails[0].Command, strings.Replace(compras1, "abc123", tok(cache), 1); got != want {
		t.Errorf("process = %q, want %q", got, want)
	}
	if got := info.ResourceTop.TopCPU[0].Command; !strings.Contains(got, "-DcacheServerPassword="+tok(cache)+" -Dcac…") {
		t.Errorf("top_cpu = %q", got)
	}
	if got := info.ResourceTop.TopMem[0].Command; !strings.HasSuffix(got, "-DcacheServerPassword={{secret}}") {
		t.Errorf("top_mem = %q", got)
	}
	if got := info.ResourceTop.TopCPU[1].Command; got != "curl -s https://deploy:{{secret}}…" {
		t.Errorf("top_cpu cut inside a URL = %q", got)
	}
	if got := info.Cron.Jobs[0].Command; got != "PGPASSWORD="+tok(pg)+" /usr/bin/pg_dump x" {
		t.Errorf("cron job = %q", got)
	}
	if want := []string{"0 2 * * * PGPASSWORD=" + tok(pg) + " /usr/bin/pg_dump x", "DB_PASSWORD={{secret}}"}; !reflect.DeepEqual(info.CronJobs, want) {
		t.Errorf("cron_jobs = %q, want %q", info.CronJobs, want)
	}

	// The next scan finds the same secrets: same rows, same tokens.
	again := planted()
	if n, err := RecordScanSecrets(ctx, d, hid, 0, again); err != nil || n != 2 {
		t.Fatalf("second scan = %d, %v", n, err)
	}
	if rows := scanSecretRows(t, d); len(rows) != 2 {
		t.Fatalf("vault rows after second scan = %v", rows)
	}
	if again.ProcessDetails[0].Command != info.ProcessDetails[0].Command {
		t.Errorf("second scan process = %q", again.ProcessDetails[0].Command)
	}
}

// Nothing reaches the vault (the database is gone): every value is still
// blanked before the scan is stored, and the error only counts the secrets
// (a group or name comes from the scanned line).
func TestRecordScanSecretsFailureStillBlanks(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	info := planted()
	n, err := RecordScanSecrets(ctx, d, hid, uid, info)
	if err == nil || n != 0 {
		t.Fatalf("RecordScanSecrets = %d, %v; want an error", n, err)
	}
	if regexp.MustCompile(`abc123|\bpw\b|scan-|PASSWORD`).MatchString(err.Error()) {
		t.Errorf("error names a secret: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %v does not wrap its cause", err)
	}
	out := assertRedacted(t, info)
	if strings.Contains(out, "{{secret:") || !strings.Contains(out, "{{secret}}") {
		t.Fatalf("want only blank tokens: %s", out)
	}
}

func TestScanSecretIdentityCollision(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx := context.Background()
	a := sshtest.ProcessDetail{Command: "python3 /a/manage.py --password=one"}
	b := sshtest.ProcessDetail{Command: "python3 /b/manage.py --password=two"}

	first := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{a, b}}
	if n, err := RecordScanSecrets(ctx, d, hid, uid, first); err != nil || n != 2 {
		t.Fatalf("first scan = %d, %v", n, err)
	}
	rows := scanSecretRows(t, d)
	one, two := rows["scan-manage-py/PASSWORD"], rows["scan-manage-py/PASSWORD_2"]
	if len(rows) != 2 || one == 0 || two == 0 {
		t.Fatalf("vault rows = %v", rows)
	}
	repo, admin := vault.NewSecretRepo(d), vault.ActorContext{UserID: uid, Role: "admin"}
	if got, _ := repo.Reveal(ctx, admin, one); got != `{"value":"one"}` {
		t.Fatalf("PASSWORD = %s", got)
	}
	if got, _ := repo.Reveal(ctx, admin, two); got != `{"value":"two"}` {
		t.Fatalf("PASSWORD_2 = %s", got)
	}

	// Listed the other way round, each value keeps its name.
	reversed := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{b, a}}
	if n, err := RecordScanSecrets(ctx, d, hid, uid, reversed); err != nil || n != 2 {
		t.Fatalf("reversed scan = %d, %v", n, err)
	}
	if got := scanSecretRows(t, d); !reflect.DeepEqual(got, rows) {
		t.Fatalf("rows after reversed scan = %v, want %v", got, rows)
	}
	if got, want := reversed.ProcessDetails[0].Command, fmt.Sprintf("python3 /b/manage.py --password={{secret:%d}}", two); got != want {
		t.Fatalf("reversed[0] = %q, want %q", got, want)
	}

	// The process holding "one" stops: "two" keeps PASSWORD_2 and PASSWORD
	// keeps "one" — a value no longer seen is kept.
	onlyB := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{b}}
	if n, err := RecordScanSecrets(ctx, d, hid, uid, onlyB); err != nil || n != 1 {
		t.Fatalf("scan with only b = %d, %v", n, err)
	}
	if got, want := onlyB.ProcessDetails[0].Command, fmt.Sprintf("python3 /b/manage.py --password={{secret:%d}}", two); got != want {
		t.Fatalf("only b = %q, want %q", got, want)
	}
	if got, _ := repo.Reveal(ctx, admin, one); got != `{"value":"one"}` {
		t.Fatalf("PASSWORD after one disappeared = %s", got)
	}
	if got, _ := repo.Reveal(ctx, admin, two); got != `{"value":"two"}` {
		t.Fatalf("PASSWORD_2 after one disappeared = %s", got)
	}

	// A single new value (one rotated) updates the first name this scan didn't match.
	rotated := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: "python3 /a/manage.py --password=three"}, b}}
	if n, err := RecordScanSecrets(ctx, d, hid, uid, rotated); err != nil || n != 2 {
		t.Fatalf("rotated scan = %d, %v", n, err)
	}
	if got := scanSecretRows(t, d); !reflect.DeepEqual(got, rows) {
		t.Fatalf("rows after rotation = %v, want %v", got, rows)
	}
	if got, _ := repo.Reveal(ctx, admin, one); got != `{"value":"three"}` {
		t.Fatalf("PASSWORD after rotation = %s", got)
	}
	if got, _ := repo.Reveal(ctx, admin, two); got != `{"value":"two"}` {
		t.Fatalf("PASSWORD_2 after rotation = %s", got)
	}
}

// The origin and the description come from the line with its values blanked:
// a value never names a group, a program or a place, and a rotated value
// keeps the same identity.
func TestScanSecretOriginIgnoresValues(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx := context.Background()
	info := &sshtest.VMInfo{
		ProcessDetails: []sshtest.ProcessDetail{
			{Command: "node --secret Secr3t server.js"},
			{Command: `/usr/bin/app A_PASSWORD="x1 y1"B_TOKEN=z1`},
		},
		Cron: &sshtest.CronInfo{Jobs: []sshtest.CronJob{
			{User: "root", Command: `DB_PASSWORD="two words" ./run.sh`},
			{User: "root", Command: `PGPASSWORD='p w' pg_dump db`},
		}},
	}
	if _, err := RecordScanSecrets(ctx, d, hid, uid, info); err != nil {
		t.Fatal(err)
	}
	rows, err := d.SQL.Query(`SELECT group_label, name, description FROM secrets ORDER BY group_label, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var g, n, desc string
		if err := rows.Scan(&g, &n, &desc); err != nil {
			t.Fatal(err)
		}
		got = append(got, g+"/"+n+" "+desc)
	}
	want := []string{
		`scan-app/A_PASSWORD A_PASSWORD · app · /usr/bin/app`,
		`scan-app/B_TOKEN A_PASSWORD="{{secret}}"B_TOKEN · app · /usr/bin/app`,
		`scan-cron-root/DB_PASSWORD DB_PASSWORD · run.sh · cron root`,
		`scan-cron-root/PGPASSWORD PGPASSWORD · pg_dump · cron root`,
		`scan-server-js/SECRET --secret · node · server.js`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// The same lines with other values: same identities, nothing new.
	rot := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: "node --secret Other1 server.js"}}}
	if _, err := RecordScanSecrets(ctx, d, hid, uid, rot); err != nil {
		t.Fatal(err)
	}
	if n := len(scanSecretRows(t, d)); n != 5 {
		t.Fatalf("%d rows after a rotated value, want 5", n)
	}
}

// A trashed scan secret is not recreated, and its value is still blanked.
func TestRecordScanSecretsTrashedStaysBlank(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx := context.Background()
	info := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: "/usr/local/bin/foo --token=s3cr3t"}}}
	if _, err := RecordScanSecrets(ctx, d, hid, uid, info); err != nil {
		t.Fatal(err)
	}
	id := scanSecretRows(t, d)["scan-foo/TOKEN"]
	if err := vault.NewSecretRepo(d).SoftDelete(ctx, vault.ActorContext{UserID: uid, Role: "admin"}, id); err != nil {
		t.Fatal(err)
	}
	again := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: "/usr/local/bin/foo --token=s3cr3t"}}}
	if n, err := RecordScanSecrets(ctx, d, hid, uid, again); err != nil || n != 0 {
		t.Fatalf("after trash = %d, %v; want 0, nil", n, err)
	}
	if got := again.ProcessDetails[0].Command; got != "/usr/local/bin/foo --token={{secret}}" {
		t.Fatalf("command = %q", got)
	}
	// Another value under the trashed name is that identity too: not recreated
	// as TOKEN_2 either.
	rotated := &sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: "/usr/local/bin/foo --token=n3w"}}}
	if n, err := RecordScanSecrets(ctx, d, hid, uid, rotated); err != nil || n != 0 {
		t.Fatalf("rotated after trash = %d, %v; want 0, nil", n, err)
	}
	if got := rotated.ProcessDetails[0].Command; got != "/usr/local/bin/foo --token={{secret}}" {
		t.Fatalf("rotated command = %q", got)
	}
	if rows := scanSecretRows(t, d); len(rows) != 0 {
		t.Fatalf("live rows = %v", rows)
	}
}

func TestBackfillScanSecretsTwice(t *testing.T) {
	d, hid, _ := newScanSecretsFixture(t)
	ctx := context.Background()
	defer func(n int) { scanBackfillBatch = n }(scanBackfillBatch)
	scanBackfillBatch = 1 // every row is its own batch

	repo := store.NewHostScanRepo(d.SQL)
	jbossScan, _ := json.Marshal(sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: compras1}}})
	// A field this Bridge no longer knows, and a number past float64 precision: kept as they are.
	legacy := strings.Replace(string(jbossScan), "{", `{"legacy_field":{"n":12345678901234567},`, 1)
	cronScan, _ := json.Marshal(sshtest.VMInfo{
		Cron:     &sshtest.CronInfo{Jobs: []sshtest.CronJob{{User: "root", Command: "PGPASSWORD=pw /usr/bin/pg_dump x"}}},
		CronJobs: []string{"0 2 * * * PGPASSWORD=pw /usr/bin/pg_dump x"},
	})
	for _, data := range []string{legacy, string(cronScan), `{"cpu":"4"}`} {
		if err := repo.Create(ctx, hid, data); err != nil {
			t.Fatal(err)
		}
	}

	rows, secrets, last, err := BackfillScanSecrets(ctx, d, 0, nil)
	if err != nil || rows != 2 || secrets != 2 {
		t.Fatalf("backfill = %d rows, %d secrets, %v; want 2, 2, nil", rows, secrets, err)
	}
	scans, err := repo.ListAfter(ctx, 0, 10)
	if err != nil || len(scans) != 3 {
		t.Fatalf("scans = %d, %v", len(scans), err)
	}
	if last != scans[2].ID {
		t.Fatalf("last = %d, want %d", last, scans[2].ID)
	}
	for _, s := range scans {
		if strings.Contains(s.Data, "abc123") || strings.Contains(s.Data, "pw ") {
			t.Errorf("row %d still holds a value: %s", s.ID, s.Data)
		}
	}
	if !strings.Contains(scans[0].Data, `"legacy_field":{"n":12345678901234567}`) || !strings.Contains(scans[0].Data, "{{secret:") {
		t.Errorf("row 1 = %s", scans[0].Data)
	}
	if scans[2].Data != `{"cpu":"4"}` {
		t.Errorf("row without secrets rewritten: %s", scans[2].Data)
	}
	vaultRows := scanSecretRows(t, d)
	if len(vaultRows) != 2 || vaultRows["scan-opt-compras1/CACHE_SERVER_PASSWORD"] == 0 || vaultRows["scan-cron-root/PGPASSWORD"] == 0 {
		t.Fatalf("vault rows = %v", vaultRows)
	}

	rows, secrets, _, err = BackfillScanSecrets(ctx, d, 0, nil)
	if err != nil || rows != 0 || secrets != 0 {
		t.Fatalf("second backfill = %d rows, %d secrets, %v; want 0, 0, nil", rows, secrets, err)
	}
	if again := scanSecretRows(t, d); !reflect.DeepEqual(again, vaultRows) {
		t.Fatalf("vault rows after second backfill = %v", again)
	}
	after, _ := repo.ListAfter(ctx, 0, 10)
	for i := range after {
		if after[i].Data != scans[i].Data {
			t.Errorf("row %d rewritten by the second run", after[i].ID)
		}
	}

	settings := store.NewAppSettingsRepo(d.SQL)
	if v := settings.Value(ctx, scanSecretsBackfillFlag); v != "" {
		t.Fatalf("flag set before the startup hook ran: %q", v)
	}
	RunScanSecretsBackfill(ctx, d)
	if v := settings.Value(ctx, scanSecretsBackfillFlag); v == "" {
		t.Fatal("startup hook did not set its flag")
	}
}

// The flag holds the last scan the backfill went through, and every start
// resumes after it: plaintext rows an older binary wrote after a finished walk
// (a rolling deploy) are redacted at the next start; rows already done are not
// read again.
func TestRunScanSecretsBackfillResumes(t *testing.T) {
	d, hid, _ := newScanSecretsFixture(t)
	ctx := context.Background()
	repo := store.NewHostScanRepo(d.SQL)
	plain := func(cmd string) string {
		b, _ := json.Marshal(sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: cmd}}})
		return string(b)
	}
	for _, data := range []string{plain("/usr/local/bin/foo --token=abc123"), `{"cpu":"4"}`} {
		if err := repo.Create(ctx, hid, data); err != nil {
			t.Fatal(err)
		}
	}
	RunScanSecretsBackfill(ctx, d)
	done, _ := repo.ListAfter(ctx, 0, 10)
	if len(done) != 2 || strings.Contains(done[0].Data, "abc123") {
		t.Fatalf("first start: %+v", done)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	if v := settings.Value(ctx, scanSecretsBackfillFlag); v != fmt.Sprint(done[1].ID) {
		t.Fatalf("flag = %q, want %d", v, done[1].ID)
	}

	// Plaintext put back in a row already done (to see whether it is read
	// again), and a new plaintext row from the old binary.
	if err := repo.UpdateData(ctx, done[0].ID, plain("/usr/local/bin/foo --token=abc123")); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, hid, plain("/usr/local/bin/bar --token=s3cr3t")); err != nil {
		t.Fatal(err)
	}
	RunScanSecretsBackfill(ctx, d)
	all, _ := repo.ListAfter(ctx, 0, 10)
	if len(all) != 3 || strings.Contains(all[2].Data, "s3cr3t") || !strings.Contains(all[2].Data, "{{secret:") {
		t.Fatalf("new row after the second start = %+v", all)
	}
	if !strings.Contains(all[0].Data, "abc123") {
		t.Fatalf("a row already done was read again: %s", all[0].Data)
	}
	if v := settings.Value(ctx, scanSecretsBackfillFlag); v != fmt.Sprint(all[2].ID) {
		t.Fatalf("flag = %q, want %d", v, all[2].ID)
	}
}

// A scan RecordScanSecrets marked is never read again: a redacted line can
// re-parse to a "value" that would overwrite a kept secret with token text. A
// row of the old binary has no mark: it is still redacted, and marked.
func TestBackfillSkipsRedactedScans(t *testing.T) {
	d, hid, _ := newScanSecretsFixture(t)
	ctx := context.Background()
	repo := store.NewHostScanRepo(d.SQL)
	scan := func(redacted bool, cmd string) string {
		b, _ := json.Marshal(sshtest.VMInfo{SecretsRedacted: redacted, ProcessDetails: []sshtest.ProcessDetail{{Command: cmd}}})
		return string(b)
	}
	// Plaintext under the mark only to see whether the row is read at all.
	marked := scan(true, "/usr/local/bin/foo --token=abc123")
	for _, data := range []string{marked, scan(false, "/usr/local/bin/bar --token=s3cr3t")} {
		if err := repo.Create(ctx, hid, data); err != nil {
			t.Fatal(err)
		}
	}
	RunScanSecretsBackfill(ctx, d)
	all, _ := repo.ListAfter(ctx, 0, 10)
	if len(all) != 2 || all[0].Data != marked {
		t.Fatalf("marked row was read: %+v", all)
	}
	if strings.Contains(all[1].Data, "s3cr3t") || !strings.Contains(all[1].Data, `"secrets_redacted":true`) {
		t.Fatalf("old row = %s", all[1].Data)
	}
	if rows := scanSecretRows(t, d); len(rows) != 1 || rows["scan-bar/TOKEN"] == 0 {
		t.Fatalf("vault rows = %v", rows)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// A start killed mid-walk (a slow first deploy) resumes after the last batch
// it finished: progress is saved, and its counts logged, batch by batch.
func TestRunScanSecretsBackfillSavesEachBatch(t *testing.T) {
	d, hid, _ := newScanSecretsFixture(t)
	repo := store.NewHostScanRepo(d.SQL)
	defer func(n int) { scanBackfillBatch = n }(scanBackfillBatch)
	scanBackfillBatch = 1
	for _, cmd := range []string{"/usr/local/bin/foo --token=abc123", "/usr/local/bin/bar --token=s3cr3t", "/usr/local/bin/baz --token=x"} {
		b, _ := json.Marshal(sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: cmd}}})
		if err := repo.Create(context.Background(), hid, string(b)); err != nil {
			t.Fatal(err)
		}
	}

	// The process dies as soon as the backfill logs anything.
	ctx, kill := context.WithCancel(context.Background())
	var logged bytes.Buffer
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		if bytes.Contains(p, []byte("secrets backfill")) {
			kill()
		}
		return logged.Write(p)
	}))
	defer log.SetOutput(os.Stderr)
	RunScanSecretsBackfill(ctx, d)

	all, _ := repo.ListAfter(context.Background(), 0, 10)
	settings := store.NewAppSettingsRepo(d.SQL)
	if v := settings.Value(context.Background(), scanSecretsBackfillFlag); len(all) != 3 || v != fmt.Sprint(all[0].ID) {
		t.Fatalf("flag after the killed start = %q, want %d (the first batch)", v, all[0].ID)
	}
	if strings.Contains(all[0].Data, "abc123") || !strings.Contains(all[1].Data, "s3cr3t") {
		t.Fatalf("killed start: %+v", all)
	}
	if out := logged.String(); !strings.Contains(out, "1 scan(s) rewritten") || strings.Contains(out, "abc123") {
		t.Fatalf("log = %q", out)
	}

	RunScanSecretsBackfill(context.Background(), d)
	all, _ = repo.ListAfter(context.Background(), 0, 10)
	for _, s := range all {
		if strings.Contains(s.Data, "s3cr3t") || strings.Contains(s.Data, "token=x") {
			t.Errorf("row %d not redacted after the next start: %s", s.ID, s.Data)
		}
	}
	if v := settings.Value(context.Background(), scanSecretsBackfillFlag); v != fmt.Sprint(all[2].ID) {
		t.Fatalf("flag = %q, want %d", v, all[2].ID)
	}
}

// A value the vault refuses for a moment (a DB error) is not lost: the row
// keeps it as it was (already plaintext there), later rows are still
// redacted, and the saved mark stays before the failed row, so the next start
// retries it — the log says from where.
func TestBackfillRetriesARefusedRow(t *testing.T) {
	d, hid, _ := newScanSecretsFixture(t)
	ctx := context.Background()
	repo := store.NewHostScanRepo(d.SQL)
	for _, cmd := range []string{"/usr/local/bin/foo --token=abc123", "/usr/local/bin/bar --token=s3cr3t", "/usr/local/bin/baz --token=xyz789"} {
		b, _ := json.Marshal(sshtest.VMInfo{ProcessDetails: []sshtest.ProcessDetail{{Command: cmd}}})
		if err := repo.Create(ctx, hid, string(b)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.SQL.Exec(`CREATE FUNCTION fail_scan_bar() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'vault down'; END $$;
		CREATE TRIGGER fail_scan_bar BEFORE INSERT ON secrets FOR EACH ROW WHEN (NEW.group_label = 'scan-bar') EXECUTE FUNCTION fail_scan_bar();`); err != nil {
		t.Fatal(err)
	}
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	RunScanSecretsBackfill(ctx, d)

	all, _ := repo.ListAfter(ctx, 0, 10)
	if len(all) != 3 || strings.Contains(all[0].Data, "abc123") || strings.Contains(all[2].Data, "xyz789") {
		t.Fatalf("rows around the refused one not redacted: %+v", all)
	}
	if !strings.Contains(all[1].Data, "s3cr3t") || strings.Contains(all[1].Data, "{{secret") {
		t.Fatalf("refused row was rewritten (its value lost): %s", all[1].Data)
	}
	settings := store.NewAppSettingsRepo(d.SQL)
	if v := settings.Value(ctx, scanSecretsBackfillFlag); v != fmt.Sprint(all[0].ID) {
		t.Fatalf("flag = %q, want %d (before the refused row)", v, all[0].ID)
	}
	if out := logged.String(); !strings.Contains(out, fmt.Sprintf("the next start resumes after scan %d", all[0].ID)) || strings.Contains(out, "s3cr3t") {
		t.Fatalf("log = %q", out)
	}

	if _, err := d.SQL.Exec(`DROP TRIGGER fail_scan_bar ON secrets`); err != nil {
		t.Fatal(err)
	}
	RunScanSecretsBackfill(ctx, d)
	all, _ = repo.ListAfter(ctx, 0, 10)
	if strings.Contains(all[1].Data, "s3cr3t") || !strings.Contains(all[1].Data, "{{secret:") {
		t.Fatalf("refused row not retried: %s", all[1].Data)
	}
	if rows := scanSecretRows(t, d); rows["scan-bar/TOKEN"] == 0 || len(rows) != 3 {
		t.Fatalf("vault rows = %v", rows)
	}
	if v := settings.Value(ctx, scanSecretsBackfillFlag); v != fmt.Sprint(all[2].ID) {
		t.Fatalf("flag = %q, want %d", v, all[2].ID)
	}
}
