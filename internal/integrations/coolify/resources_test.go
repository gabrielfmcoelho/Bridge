package coolify

import "testing"

func TestContainerKey(t *testing.T) {
	cases := []struct{ name, key, uuid string }{
		{"a1rwyyxk4rby396m4ooq4ubu-143303502430", "a1rwyyxk4rby396m4ooq4ubu", "a1rwyyxk4rby396m4ooq4ubu"},
		{"airflow-scheduler-rtb37u69bbkepzukr9vclrd7-123344967269", "airflow-scheduler-rtb37u69bbkepzukr9vclrd7", "rtb37u69bbkepzukr9vclrd7"},
		{"keycloak-rsul83d5es2xd6dxdvup5d98", "keycloak-rsul83d5es2xd6dxdvup5d98", "rsul83d5es2xd6dxdvup5d98"},
		{"zwkg80s48g4gogko44o8gg4g", "zwkg80s48g4gogko44o8gg4g", "zwkg80s48g4gogko44o8gg4g"},
		{"coolify-proxy", "coolify-proxy", ""},
		{"postgres-15", "postgres-15", ""},
	}
	for _, c := range cases {
		if k, u := ContainerKey(c.name); k != c.key || u != c.uuid {
			t.Errorf("ContainerKey(%q) = %q, %q; want %q, %q", c.name, k, u, c.key, c.uuid)
		}
	}
}

func TestSanitizeRepoURL(t *testing.T) {
	cases := map[string]string{
		"https://gitlab+deploy-token-12:gldt-SECRET@gitlab.example.com/g/r.git": "https://gitlab.example.com/g/r.git",
		"https://gitlab.ati.pi.gov.br/seadpi/x.git":                             "https://gitlab.ati.pi.gov.br/seadpi/x.git",
		"git@gitlab.example.com:g/r.git":                                        "git@gitlab.example.com:g/r.git",
		"https://user:p%zz@host/r":                                              "https://host/r",
		"":                                                                      "",
	}
	for in, want := range cases {
		if got := SanitizeRepoURL(in); got != want {
			t.Errorf("SanitizeRepoURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResourcesResolvesEnvironmentAndMembers(t *testing.T) {
	apps := []Application{{UUID: "a", Name: "API", EnvironmentID: 7, FQDN: "https://api.x,http://b.y:8080/p",
		GitRepository: "https://u:t@h/r.git"}}
	apps[0].Destination.Server = ServerRef{UUID: "s1", IP: masterIP}
	svcs := []Service{{UUID: "svcuuid", Name: "gateway", EnvironmentID: 8}}
	svcs[0].Applications = append(svcs[0].Applications, struct {
		Name string `json:"name"`
		FQDN string `json:"fqdn"`
	}{"apisix", "http://gw.x"})
	projects := []Project{{Name: "Gestor", Environments: []Environment{{ID: 7, Name: "production"}}},
		{Name: "API Gateway", Environments: []Environment{{ID: 8, Name: "production"}}}}
	rs := Resources(apps, svcs, nil, projects, "10.0.0.1")
	if len(rs) != 2 {
		t.Fatalf("len = %d", len(rs))
	}
	a := rs[0]
	if a.Project != "Gestor" || a.Environment != "production" || a.GitRepository != "https://h/r.git" || a.Server.IP != "10.0.0.1" {
		t.Errorf("app = %+v", a)
	}
	if h := a.Hosts(); len(h) != 2 || h[0] != "api.x" || h[1] != "b.y" {
		t.Errorf("hosts = %v", h)
	}
	if s := rs[1]; s.Project != "API Gateway" || len(s.Members) != 1 || s.Members[0] != "apisix-svcuuid" {
		t.Errorf("service = %+v", s)
	}
}
