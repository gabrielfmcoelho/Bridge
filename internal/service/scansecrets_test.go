package service

import (
	"context"
	"encoding/json"
	"fmt"
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
		{"curl https://bob:pa55@git.example/x", []kv{{"url-bob-git.example", "pa55"}}},
		{"mysqldump -uroot -pS3cret db", []kv{{"mysql-password", "S3cret"}}},
		{"java -Dpassword=${DB_PASS}", nil},
		{"app --token=$TOKEN", nil},
		{"java -DcacheServerPassword=", nil},
		// Already redacted: a backfill re-run must not take the token for a value.
		{"java -DcacheServerPassword={{secret:7}} --token {{secret}}", nil},
		// A crontab line: the assignment follows the schedule.
		{"0 2 * * * PGPASSWORD=pw /usr/bin/pg_dump x", []kv{{"PGPASSWORD", "pw"}}},
		{`PGPASSWORD='pw' pg_dump`, []kv{{"PGPASSWORD", "pw"}}},
		{`DB_PASSWORD="two words" ./run.sh`, []kv{{"DB_PASSWORD", "two words"}}},
		{"app --password --token abc", []kv{{"token", "abc"}}},
		// A value starting inside another and ending past it: one span covers both.
		{`x PASSWORD="a --token=b"c d`, []kv{{"PASSWORD", `a --token=b"c`}}},
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
	}
}

func TestEnvVarName(t *testing.T) {
	for key, want := range map[string]string{
		"cacheServerPassword":              "CACHE_SERVER_PASSWORD",
		"javax.net.ssl.trustStorePassword": "JAVAX_NET_SSL_TRUST_STORE_PASSWORD",
		"db-password":                      "DB_PASSWORD",
		"url-bob-git.example":              "URL_BOB_GIT_EXAMPLE",
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
// resource_top (once after the value, once inside it), a root cron job and
// the legacy crontab lines.
func planted() *sshtest.VMInfo {
	i := strings.Index(compras1, "abc123")
	return &sshtest.VMInfo{
		ProcessDetails: []sshtest.ProcessDetail{{PID: "1", User: "jboss", Command: compras1, Unit: "compras1.service"}},
		ResourceTop: &sshtest.ResourceUsageSnapshot{
			TopCPU: []sshtest.ResourceProcess{{PID: 1, Command: compras1[:i+12] + "…"}},
			TopMem: []sshtest.ResourceProcess{{PID: 1, Command: compras1[:i+4] + "…"}},
		},
		Cron: &sshtest.CronInfo{Jobs: []sshtest.CronJob{
			{Source: "system", Kind: "cron", Schedule: "0 2 * * *", User: "root", Command: "PGPASSWORD=pw /usr/bin/pg_dump x"},
		}},
		CronJobs: []string{"0 2 * * * PGPASSWORD=pw /usr/bin/pg_dump x", "DB_PASSWORD=s3cr3t"},
	}
}

func assertRedacted(t *testing.T, info *sshtest.VMInfo) string {
	t.Helper()
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, v := range []string{"abc123", "abc1", "pw ", "s3cr3t"} {
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
// blanked before the scan is stored, and the error says which secret failed.
func TestRecordScanSecretsFailureStillBlanks(t *testing.T) {
	d, hid, uid := newScanSecretsFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	info := planted()
	n, err := RecordScanSecrets(ctx, d, hid, uid, info)
	if err == nil || n != 0 {
		t.Fatalf("RecordScanSecrets = %d, %v; want an error", n, err)
	}
	if regexp.MustCompile(`abc123|\bpw\b`).MatchString(err.Error()) {
		t.Errorf("error carries a value: %v", err)
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

	rows, secrets, err := BackfillScanSecrets(ctx, d)
	if err != nil || rows != 2 || secrets != 2 {
		t.Fatalf("backfill = %d rows, %d secrets, %v; want 2, 2, nil", rows, secrets, err)
	}
	scans, err := repo.ListAfter(ctx, 0, 10)
	if err != nil || len(scans) != 3 {
		t.Fatalf("scans = %d, %v", len(scans), err)
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

	rows, secrets, err = BackfillScanSecrets(ctx, d)
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
