package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
)

// SAPASP-CONCURSO's release, as two scans: fail2ban died, logrotate
// started failing, proftpd left *.ucf-dist files, and php-cgi7.2 went away
// under the sites mapped to it.
func TestCompareScans(t *testing.T) {
	before := &sshtest.VMInfo{
		SystemdServices: []sshtest.SystemdService{{Unit: "apache2.service"}, {Unit: "fail2ban.service"}, {Unit: "proftpd.service"}},
		ReleaseRisks: &sshtest.ReleaseRisks{
			FailedUnits:      []string{"old.service"},
			PendingDistFiles: []string{"/etc/old.conf.dpkg-dist"},
			ForeignPackages:  []string{"php8.2-cli 8.2.6-1+ubuntu18.04.1+deb.sury.org+1"},
			PHPHandlers: []sshtest.PHPHandler{
				{Target: "/home/sasc/fcgi-bin/php7.2.fcgi", Runs: "/usr/bin/php-cgi7.2", State: "ok"},
				{Target: "/home/x/fcgi-bin/php5.fcgi", State: "missing"},
			},
		},
	}
	after := &sshtest.VMInfo{
		SystemdServices: []sshtest.SystemdService{{Unit: "apache2.service"}, {Unit: "proftpd.service"}},
		ReleaseRisks: &sshtest.ReleaseRisks{
			FailedUnits:      []string{"old.service", "fail2ban.service", "logrotate.service"},
			PendingDistFiles: []string{"/etc/old.conf.dpkg-dist", "/etc/proftpd/modules.conf.ucf-dist"},
			ForeignPackages:  []string{"libssl1.1 1.1.1f-1ubuntu2.20", "php8.2-cli 8.2.7-1+ubuntu18.04.1+deb.sury.org+1"},
			PHPHandlers: []sshtest.PHPHandler{
				{Source: "a.conf:5", Target: "/home/sasc/fcgi-bin/php7.2.fcgi", Runs: "/usr/bin/php-cgi7.2", State: "missing"},
				{Source: "a.conf:9", Target: "/home/sasc/fcgi-bin/php7.2.fcgi", Runs: "/usr/bin/php-cgi7.2", State: "missing"},
				{Target: "/home/x/fcgi-bin/php5.fcgi", State: "missing"},
			},
		},
	}
	want := ReleaseReport{
		StoppedServices:    []string{"fail2ban.service"},
		NewFailedUnits:     []string{"fail2ban.service", "logrotate.service"},
		NewDistFiles:       []string{"/etc/proftpd/modules.conf.ucf-dist"},
		NewForeignPackages: []string{"libssl1.1"},
		LostPHPHandlers:    []string{"/home/sasc/fcgi-bin/php7.2.fcgi → /usr/bin/php-cgi7.2"},
	}
	if got := CompareScans(before, after); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	// A scan from before the probe existed: only the services compare.
	before.ReleaseRisks = nil
	if got := CompareScans(before, after); !reflect.DeepEqual(got, ReleaseReport{StoppedServices: []string{"fail2ban.service"}}) {
		t.Fatalf("no probe before = %+v", got)
	}
	if !reflect.DeepEqual(CompareScans(nil, after), ReleaseReport{}) || !reflect.DeepEqual(CompareScans(before, nil), ReleaseReport{}) {
		t.Fatal("a missing scan compares to nothing")
	}
}

// The sites' "/" before and after: a 302 that stays a 302 is fine, a page
// that now prints PHP's deprecation notices (with html_errors) is broken
// although it answers 200, and a site that stopped answering is broken.
func TestProbeAndCompareSites(t *testing.T) {
	var after bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !after {
			w.Write([]byte("<html>ok</html>"))
			return
		}
		w.Write([]byte("<br />\n<b>Deprecated</b>:  Creation of dynamic property CI_URI::$config is deprecated in <b>/home/sasc/system/core/URI.php</b>"))
	}))
	defer srv.Close()
	redirect := httptest.NewServer(http.RedirectHandler("/login", http.StatusFound))
	defer redirect.Close()
	ctx := context.Background()
	app, login := strings.TrimPrefix(srv.URL, "http://"), strings.TrimPrefix(redirect.URL, "http://")

	beforeSites := ProbeSites(ctx, []string{app, login})
	if beforeSites[app] != (SiteState{Status: 200}) || beforeSites[login] != (SiteState{Status: 302}) {
		t.Fatalf("before = %+v", beforeSites)
	}
	after = true
	afterSites := ProbeSites(ctx, []string{app, login})
	afterSites["gone.example"] = SiteState{} // stopped answering
	beforeSites["gone.example"] = SiteState{Status: 200}
	got := CompareSites(beforeSites, afterSites)
	want := map[string]SiteCheck{
		app:            {Name: app, Before: 200, After: 200, Error: "Deprecated", Broken: true},
		login:          {Name: login, Before: 302, After: 302},
		"gone.example": {Name: "gone.example", Before: 200, Broken: true},
	}
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	for _, c := range got {
		if c != want[c.Name] {
			t.Errorf("%s = %+v, want %+v", c.Name, c, want[c.Name])
		}
	}
	if m := phpErrorText.FindString("<h4>A PHP Error was encountered</h4>"); m == "" {
		t.Fatal("CodeIgniter's error page not matched")
	}
	if got := SiteNames([]string{"A.example.gov.br", "*.example.gov.br", "a.example.gov.br", "", "x/y"}); !reflect.DeepEqual(got, []string{"a.example.gov.br"}) {
		t.Fatalf("names = %v", got)
	}
}
