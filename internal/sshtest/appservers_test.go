package sshtest

import (
	"reflect"
	"strings"
	"testing"
)

const jbossCmd = "/opt/jdk1.7.0_79/bin/java -Dprogram.name=run.sh -Djavax.net.ssl.trustStorePassword=x -classpath /opt/compras1/bin/run.jar:/opt/jdk1.7.0_79/lib/tools.jar org.jboss.Main -c default -b 0.0.0.0"

func TestDetectAppServers(t *testing.T) {
	procs := []ProcessDetail{
		{PID: "1", Command: jbossCmd, Unit: "compras1.service", Ports: "1090,1098,1099,4444,8009,8080,8083"},
		{PID: "2", Command: strings.ReplaceAll(jbossCmd, "compras1", "compras2"), Unit: "compras2.service", Ports: "1190,1198,1199,4544,8109,8180,8183"},
		{PID: "3", Command: "java -Dcatalina.base=/srv/tomcat9 -classpath /srv/tomcat9/bin/bootstrap.jar org.apache.catalina.startup.Bootstrap start", Ports: "8080"},
		{PID: "4", Command: "java -jar app.jar"},
	}
	got := detectAppServers(procs)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(got), got)
	}
	for i, want := range []struct {
		inst string
		port int
	}{{"compras1.service", 8080}, {"compras2.service", 8180}} {
		s := got[i]
		if s.Name != "jboss" || s.Label != "JBoss AS" || s.Instance != want.inst || !s.HostRunning ||
			len(s.HostPorts) == 0 || s.HostPorts[0] != want.port || !reflect.DeepEqual(s.Sources, []string{"process"}) {
			t.Errorf("jboss[%d] = %+v", i, s)
		}
	}
	if got[2].Name != "tomcat" || got[2].Instance != "/srv/tomcat9" {
		t.Errorf("tomcat = %+v", got[2])
	}
}

func TestAppHome(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{jbossCmd, "/opt/compras1"},
		{"java -Djboss.home.dir=/opt/wildfly -jar /opt/wildfly/jboss-modules.jar org.jboss.modules.Main", "/opt/wildfly"},
		{"java -Dcatalina.base=/srv/t org.apache.catalina.startup.Bootstrap start", "/srv/t"},
	}
	for _, c := range cases {
		if got := appHome(c.cmd); got != c.want {
			t.Errorf("appHome(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestAppDeployDirs(t *testing.T) {
	cases := []struct {
		cat, home, cmd string
		want           []string
	}{
		{"jboss", "/opt/compras1", jbossCmd, []string{"/opt/compras1/server/default/deploy"}},
		{"wildfly", "/opt/w", "", []string{"/opt/w/standalone/deployments"}},
		{"tomcat", "/srv/t", "", []string{"/srv/t/webapps"}},
	}
	for _, c := range cases {
		if got := appDeployDirs(c.cat, c.home, c.cmd); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.cat, got, c.want)
		}
	}
}

func TestParseAppsListing(t *testing.T) {
	raw := "@@dir /opt/compras1/server/default/deploy\nsgc.war\nsigacomum.ear\njbossweb.sar\njmx-console.war\nadmin-console.war\nROOT.war\nmail-service.xml\nx.rar\nsigatr-api-1.jar\n"
	want := map[string][]string{"/opt/compras1/server/default/deploy": {"sgc.war", "sigacomum.ear"}}
	if got := parseAppsListing(raw); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestHttpPortFirst(t *testing.T) {
	cases := []struct{ in, want []int }{
		{[]int{1090, 8009, 8080, 8083}, []int{8080, 1090, 8009, 8083}},
		{[]int{1190, 8109, 8183, 8180}, []int{8180, 1190, 8109, 8183}},
		{[]int{1090, 8009, 8083}, []int{8009, 1090, 8083}},
		{[]int{5000}, []int{5000}},
	}
	for _, c := range cases {
		if got := httpPortFirst(append([]int(nil), c.in...)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%v: got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestProcessScriptAwkFilterAnchored(t *testing.T) {
	s := processDetailsScript()
	if strings.Contains(s, "!~ /grep|ps|awk/") || !strings.Contains(s, `/(^|\/)(grep|ps|awk)$/`) {
		t.Errorf("awk filter not anchored to the basename")
	}
}
