package sshtest

import "testing"

func TestParseCronDaemon(t *testing.T) {
	cases := []struct {
		name, active, enabled string
		i, a, e               bool
	}{
		{"crond", "active", "enabled", true, true, true},
		{"", "", "", false, false, false},
		{"cron", "inactive", "disabled", true, false, false},
	}
	for _, c := range cases {
		i, a, e := parseCronDaemon(c.name, c.active, c.enabled)
		if i != c.i || a != c.a || e != c.e {
			t.Errorf("parseCronDaemon(%q,%q,%q) = %v,%v,%v", c.name, c.active, c.enabled, i, a, e)
		}
	}
}

func TestCronCommandBinary(t *testing.T) {
	cases := map[string]string{
		"/ust/bin/certbot renew --quiet --no-self-upgrade": "/ust/bin/certbot",
		"PGPASSWORD=x /usr/bin/pg_dump db":                 "/usr/bin/pg_dump",
		"cd /tmp && ./run.sh":                              "",
		"certbot renew":                                    "",
		"  /opt/x.sh>/dev/null 2>&1":                       "/opt/x.sh",
	}
	for in, want := range cases {
		if got := cronCommandBinary(in); got != want {
			t.Errorf("cronCommandBinary(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkMissingBinaries(t *testing.T) {
	jobs := []CronJob{
		{Command: "/ust/bin/certbot renew"},
		{Command: "/usr/bin/pg_dump db"},
		{Command: "certbot renew"},
	}
	markMissingBinaries(jobs, map[string]bool{"/ust/bin/certbot": true})
	if !jobs[0].MissingBinary || jobs[1].MissingBinary || jobs[2].MissingBinary {
		t.Errorf("got %+v", jobs)
	}
}
