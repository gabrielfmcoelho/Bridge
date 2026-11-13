package sshtest

import "testing"

func TestUnitFromCgroup(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"v1 systemd", "1:name=systemd:/system.slice/compras1.service", "compras1.service"},
		{"v2", "0::/system.slice/nginx.service", "nginx.service"},
		{"session scope", "0::/user.slice/user-1008.slice/session-7.scope", ""},
		{"docker scope", "0::/system.slice/docker-ab12.scope", ""},
		{"sub-cgroup", "0::/system.slice/foo.service/sub", "foo.service"},
		{"v1 multi-line", "4:cpu,cpuacct:/system.slice/other.service\n3:memory:/system.slice/other.service\n1:name=systemd:/system.slice/compras1.service\n", "compras1.service"},
	}
	for _, c := range cases {
		if got := unitFromCgroup(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseProcessDetails(t *testing.T) {
	const s = "\x1f"
	raw := "1234" + s + "jboss" + s + "1.5" + s + "8.2" + s + "/opt/jboss/bin" + s + "/usr/bin/java" + s + "sh" + s +
		"1:name=systemd:/system.slice/compras1.service" + s + "8080,8083,1099" + s +
		"java -Dpass=x -jar run.jar | tee out\n" +
		"1235" + s + "jboss" + s + "0.5" + s + "3.0" + s + "/opt/jboss/bin" + s + "/usr/bin/java" + s + "sh" + s +
		"1:name=systemd:/system.slice/compras1.service" + s + "8080" + s + "java -jar b.jar\n" +
		"77" + s + "app" + s + "0.0" + s + "1.0" + s + "" + s + "" + s + "bash" + s + "" + s + "" + s + "python app.py\n"
	got := parseProcessDetails(raw)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	a := got[0]
	if a.PID != "1234" || a.StartedVia != "systemd" || a.Unit != "compras1.service" || a.Ports != "8080,8083,1099" ||
		a.Command != "java -Dpass=x -jar run.jar | tee out" || a.CPU != "1.5%" || a.CWD != "/opt/jboss/bin" {
		t.Errorf("first = %+v", a)
	}
	if got[2].StartedVia != "manual" || got[2].Unit != "" {
		t.Errorf("third = %+v", got[2])
	}
}
