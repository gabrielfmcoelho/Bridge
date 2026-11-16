package sshtest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// ---- .netrc ----

func TestDetectNetrc(t *testing.T) {
	content := `machine git.example.org login gituser password netrcMy1
default login defuser password netrcDef2
macdef init
  password NOT_A_CRED_macro_body
`
	hits := hitByLocator(detect("/root/.netrc", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 (macdef body skipped): %+v", len(hits), hits)
	}
	m := hits["git-example-org"]
	if m.Admin || m.Detector != "netrc" || m.Fields["host"] != "git.example.org" {
		t.Fatalf("machine entry = %+v", m)
	}
	if s := credSecret(t, m.Secret); s["username"] != "gituser" || s["password"] != "netrcMy1" {
		t.Fatalf("machine payload = %v", s)
	}
	if s := credSecret(t, hits["default"].Secret); s["password"] != "netrcDef2" {
		t.Fatalf("default payload = %v", s)
	}
	for _, h := range hits {
		if strings.Contains(h.Secret, "NOT_A_CRED") {
			t.Fatal("macdef macro body must not be parsed as a credential")
		}
	}
}

// ---- docker / registry auth ----

func TestDetectDockerConfig(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("dockeruser:dockerSecret1"))
	content := `{"auths":{"registry.example.org":{"auth":"` + auth + `"},"other.io":{"username":"ou","password":"dockerSecret2"}}}`
	hits := hitByLocator(detect("/root/.docker/config.json", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 registries: %+v", len(hits), hits)
	}
	reg := hits["registry-example-org"]
	if !reg.Admin || reg.Detector != "docker_config" || reg.Fields["registry"] != "registry.example.org" {
		t.Fatalf("registry = %+v (registry push cred is admin tier)", reg)
	}
	if s := credSecret(t, reg.Secret); s["username"] != "dockeruser" || s["password"] != "dockerSecret1" {
		t.Fatalf("decoded auth payload = %v", s)
	}
	if s := credSecret(t, hits["other-io"].Secret); s["password"] != "dockerSecret2" {
		t.Fatalf("explicit password payload = %v", s)
	}
}

// ---- git-credentials ----

func TestDetectGitCredentials(t *testing.T) {
	content := "https://ghuser:gitSecret1@github.com\nhttp://u2:gitSecret2@gitlab.local:8080\n# comment\n"
	hits := detect("/root/.git-credentials", content)
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 (comment skipped): %+v", len(hits), hits)
	}
	byPass := map[string]CredHit{}
	for _, h := range hits {
		if h.Admin {
			t.Fatalf("git host credential is the normal tier: %+v", h)
		}
		byPass[credSecret(t, h.Secret)["password"]] = h
	}
	if h := byPass["gitSecret1"]; h.Fields["host"] != "github.com" || h.Fields["scheme"] != "https" {
		t.Fatalf("github entry = %+v", h)
	}
	if h := byPass["gitSecret2"]; h.Fields["host"] != "gitlab.local:8080" {
		t.Fatalf("gitlab entry = %+v", h)
	}
}

// ---- npm ----

func TestDetectNpmrc(t *testing.T) {
	content := "//registry.npmjs.org/:_authToken=npmSecret1\n_auth=dXNlcjpwYXNz\n//scoped.example.org/:_password=npmSecret3\nmetadata_token=${NPM_TOKEN}\nregistry=https://registry.npmjs.org/\n"
	hits := detect("/home/deploy/.npmrc", content)
	// _authToken, _auth, _password captured; registry= and the ${…} token skipped.
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want 3: %+v", len(hits), hits)
	}
	byVal := map[string]CredHit{}
	for _, h := range hits {
		if !h.Admin || h.Kind != "env_var" || h.Detector != "npmrc" {
			t.Fatalf("npmrc hit not an admin env_var: %+v", h)
		}
		byVal[h.Secret] = h
	}
	if _, ok := byVal["npmSecret1"]; !ok {
		t.Fatal("_authToken not captured")
	}
	if _, ok := byVal["dXNlcjpwYXNz"]; !ok {
		t.Fatal("_auth blob not captured verbatim")
	}
	if _, ok := byVal["npmSecret3"]; !ok {
		t.Fatal("_password not captured")
	}
	for _, h := range hits {
		if strings.Contains(h.Secret, "NPM_TOKEN") {
			t.Fatal("a ${NPM_TOKEN} placeholder must be indirect, not captured")
		}
	}
}

// ---- composer ----

func TestDetectComposerAuth(t *testing.T) {
	content := `{
		"http-basic": { "repo.example.org": { "username": "cu", "password": "composerSecret1" } },
		"gitlab-token": { "gitlab.example.org": "composerSecret2" },
		"github-oauth": { "github.com": "composerSecret3" }
	}`
	hits := detect("/root/.composer/auth.json", content)
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want http-basic + gitlab-token + github-oauth: %+v", len(hits), hits)
	}
	byVal := map[string]CredHit{}
	for _, h := range hits {
		if !h.Admin || h.Detector != "composer_auth" {
			t.Fatalf("composer hit not admin tier: %+v", h)
		}
		switch h.Kind {
		case "cred":
			byVal[credSecret(t, h.Secret)["password"]] = h
		case "env_var":
			byVal[h.Secret] = h
		}
	}
	if h := byVal["composerSecret1"]; h.Fields["kind"] != "http-basic" || h.Fields["host"] != "repo.example.org" {
		t.Fatalf("http-basic entry = %+v", h)
	}
	if h := byVal["composerSecret2"]; h.Fields["kind"] != "gitlab-token" {
		t.Fatalf("gitlab-token entry = %+v", h)
	}
	if _, ok := byVal["composerSecret3"]; !ok {
		t.Fatal("github-oauth token not captured")
	}
}

// ---- maven ----

func TestDetectMavenSettings(t *testing.T) {
	content := `<settings>
	<servers>
		<server><id>releases</id><username>deploy</username><password>mavenSecret1</password></server>
		<server><id>encrypted</id><username>u2</username><password>{COQLCE6DU6GtcS5P=}</password></server>
		<server><id>fromenv</id><password>${env.MVN_PASS}</password></server>
	</servers>
</settings>`
	hits := hitByLocator(detect("/root/.m2/settings.xml", content))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 literal passwords (${env} indirect): %+v", len(hits), hits)
	}
	r := hits["releases"]
	if r.Admin || r.Detector != "maven_settings" || r.Fields["encrypted"] != "" {
		t.Fatalf("releases = %+v (plaintext, normal tier)", r)
	}
	if s := credSecret(t, r.Secret); s["username"] != "deploy" || s["password"] != "mavenSecret1" {
		t.Fatalf("releases payload = %v", s)
	}
	e := hits["encrypted"]
	if e.Fields["encrypted"] != "true" {
		t.Fatalf("Maven-encrypted {…} password must be flagged: %+v", e)
	}
	if s := credSecret(t, e.Secret); s["password"] != "{COQLCE6DU6GtcS5P=}" {
		t.Fatalf("encrypted blob must be stored verbatim: %v", s)
	}
	if _, ok := hits["fromenv"]; ok {
		t.Fatal("a ${env.*} password must be indirect, not captured")
	}
}

// ---- pip ----

func TestDetectPipConf(t *testing.T) {
	content := "[global]\nindex-url = https://pipuser:pipSecret1@pypi.example.org/simple\nextra-index-url = https://pypi.org/simple\n"
	hits := detect("/etc/pip.conf", content)
	if len(hits) != 1 || hits[0].Detector != "pip_conf" {
		t.Fatalf("hits = %+v, want one pip_conf (the public extra index has no userinfo)", hits)
	}
	h := hits[0]
	if h.Admin || h.Fields["host"] != "pypi.example.org" {
		t.Fatalf("pip hit = %+v", h)
	}
	if s := credSecret(t, h.Secret); s["username"] != "pipuser" || s["password"] != "pipSecret1" {
		t.Fatalf("payload = %v", s)
	}
}

// ---- kubeconfig ----

func TestDetectKubeconfig(t *testing.T) {
	content := `apiVersion: v1
users:
- name: admin
  user:
    token: kubeSecret1
    client-key-data: PRIVATEKEYDATASHOULDNOTLEAK==
- name: basic
  user:
    username: bu
    password: kubeSecret2
`
	hits := detect("/root/.kube/config", content)
	byLoc := map[string]CredHit{}
	gotToken, gotPass, gotKeyNote := false, false, false
	for _, h := range hits {
		if !h.Admin || h.Detector != "kubeconfig" {
			t.Fatalf("kubeconfig hit not admin tier: %+v", h)
		}
		byLoc[h.Locator] = h
		switch {
		case h.Secret == "kubeSecret1":
			gotToken = true
		case h.Secret == "kubeSecret2":
			gotPass = true
		case h.Locator == "client-key-data":
			gotKeyNote = true
			if h.Secret != "" {
				t.Fatalf("client-key-data must have no secret: %+v", h)
			}
		}
		if strings.Contains(h.Secret, "PRIVATEKEYDATA") {
			t.Fatal("client-key-data (a private key) must never be captured")
		}
	}
	if !gotToken || !gotPass {
		t.Fatalf("token/password not both captured: %+v", hits)
	}
	if !gotKeyNote {
		t.Fatal("client-key-data presence should be recorded as not-captured provenance")
	}
}

// ---- systemd EnvironmentFile ----

func TestDetectSystemdEnv(t *testing.T) {
	// The parser relabels the .env allowlist under the systemd_env detector.
	env := credFile{Path: "/etc/app/db.env", Content: "DB_PASSWORD=systemdSecret1\nAPP_ENV=prod\nDB_HOST=localhost\n"}
	hits := systemdEnv(env)
	if len(hits) != 1 || hits[0].Detector != "systemd_env" || hits[0].Secret != "systemdSecret1" {
		t.Fatalf("parser = %+v, want one systemd_env for DB_PASSWORD only", hits)
	}
	// Routing: detectCreds reads EnvironmentFile= from the unit and routes its target.
	unit := credFile{Path: "/etc/systemd/system/app.service", Content: "[Service]\nEnvironmentFile=-/etc/app/db.env\n"}
	got := detectCreds([]credFile{unit, env}, nil, nil)
	if set := detectorSet(got); !set["systemd_env"] {
		t.Fatalf("EnvironmentFile target not routed to systemd_env: %+v", got)
	}
	// The unit file itself yields nothing.
	if got2 := detectCreds([]credFile{unit}, nil, nil); len(got2) != 0 {
		t.Fatalf("the unit file alone must produce no hit: %+v", got2)
	}
}

// ---- leak discipline: no captured secret reaches the stored provenance ----

func TestCredStoresNoPasswordInProvenance(t *testing.T) {
	dockerAuth := base64.StdEncoding.EncodeToString([]byte("u:leakDocker"))
	cases := []struct{ path, content, secret string }{
		{"/root/.netrc", "machine h login u password leakNetrc\n", "leakNetrc"},
		{"/root/.docker/config.json", `{"auths":{"reg":{"auth":"` + dockerAuth + `"}}}`, "leakDocker"},
		{"/root/.git-credentials", "https://u:leakGit@github.com\n", "leakGit"},
		{"/root/.npmrc", "//r/:_authToken=leakNpm\n", "leakNpm"},
		{"/root/.composer/auth.json", `{"gitlab-token":{"gl":"leakComposer"}}`, "leakComposer"},
		{"/root/.m2/settings.xml", "<settings><servers><server><id>s</id><password>leakMaven</password></server></servers></settings>", "leakMaven"},
		{"/etc/pip.conf", "[global]\nindex-url = https://u:leakPip@pypi.local/simple\n", "leakPip"},
		{"/root/.kube/config", "users:\n- user:\n    token: leakKube\n", "leakKube"},
	}
	for _, c := range cases {
		hits := detect(c.path, c.content)
		if len(hits) == 0 {
			t.Fatalf("%s: no hit", c.path)
		}
		inSecret := false
		var prov []CredentialSource
		for _, h := range hits {
			if strings.Contains(h.Secret, c.secret) {
				inSecret = true
			}
			prov = append(prov, CredentialSource{Detector: h.Detector, Path: h.Path, Locator: h.Locator, Kind: h.Kind, Admin: h.Admin, Fields: h.Fields})
		}
		if !inSecret {
			t.Fatalf("%s: secret not captured into CredHit.Secret (leak test would be meaningless)", c.path)
		}
		b, err := json.Marshal(prov)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), c.secret) {
			t.Fatalf("%s: secret leaked into stored provenance: %s", c.path, b)
		}
	}
}

// ---- fake-root probe layout (home-relative, static, systemd, project) ----

func TestCredProbeStoresLayout(t *testing.T) {
	root := t.TempDir()
	dockerAuth := base64.StdEncoding.EncodeToString([]byte("dockeruser:dockerPw2"))
	// Home-relative stores: root, a /home user, /var/www.
	plant(t, root, "root/.netrc", "machine git.example.org login gu password netrcPw1\n")
	plant(t, root, "root/.docker/config.json", `{"auths":{"registry.example.org":{"auth":"`+dockerAuth+`"}}}`)
	plant(t, root, "root/.git-credentials", "https://ghu:gitCredPw3@github.com\n")
	plant(t, root, "home/deploy/.npmrc", "//registry.npmjs.org/:_authToken=npmPw4\n")
	plant(t, root, "home/deploy/.composer/auth.json", `{"http-basic":{"repo.example.org":{"username":"cu","password":"composerPw5"}}}`)
	plant(t, root, "root/.m2/settings.xml", "<settings><servers><server><id>central</id><username>mu</username><password>mavenPw6</password></server></servers></settings>")
	plant(t, root, "root/.config/pip/pip.conf", "[global]\nindex-url = https://pu:pipPw7@pypi.example.org/simple\n")
	plant(t, root, "root/.kube/config", "users:\n- name: admin\n  user:\n    token: kubePw8\n    client-key-data: FAKEKEYDATA==\n")
	// Static system paths.
	plant(t, root, "etc/pip.conf", "[global]\nindex-url = https://su:pipSysPw9@pypi.example.org/simple\n")
	plant(t, root, "etc/kubernetes/admin.conf", "users:\n- name: kubernetes-admin\n  user:\n    token: adminConfPw10\n")
	// systemd EnvironmentFile.
	plant(t, root, "etc/systemd/system/app.service", "[Service]\nEnvironmentFile=-/etc/app/db.env\nExecStart=/usr/bin/app\n")
	plant(t, root, "etc/app/db.env", "DB_PASSWORD=systemdPw11\nAPP_ENV=prod\n")
	// Project-level npm / composer under a docroot.
	plant(t, root, "var/www/site/.npmrc", "//npm.example.org/:_authToken=projNpmPw12\n")
	plant(t, root, "var/www/site/auth.json", `{"gitlab-token":{"gitlab.example.org":"projComposerPw13"}}`)

	files := runCredProbe(t, root, []string{"/var/www/site"}, nil)
	got := detectCreds(files, nil, nil)

	want := []string{"netrc", "docker_config", "git_credentials", "npmrc", "composer_auth",
		"maven_settings", "pip_conf", "kubeconfig", "systemd_env"}
	set := detectorSet(got)
	for _, d := range want {
		if !set[d] {
			t.Errorf("detector %s produced no hit from the probe output (got %v)", d, keysOfBool(set))
		}
	}
	// systemd routed the EnvironmentFile target, not the unit.
	sysEnv := false
	for _, h := range got {
		if h.Detector == "systemd_env" && h.Secret == "systemdPw11" {
			sysEnv = true
		}
		if strings.Contains(h.Secret, "FAKEKEYDATA") {
			t.Fatal("kubeconfig client-key-data must never be captured")
		}
	}
	if !sysEnv {
		t.Error("systemd EnvironmentFile secret not captured")
	}
	// No fixture secret reaches the stored provenance.
	var prov []CredentialSource
	for _, h := range got {
		prov = append(prov, CredentialSource{Detector: h.Detector, Path: h.Path, Locator: h.Locator, Kind: h.Kind, Admin: h.Admin, Fields: h.Fields})
	}
	b, _ := json.Marshal(prov)
	for _, pw := range []string{"netrcPw1", "dockerPw2", "gitCredPw3", "npmPw4", "composerPw5", "mavenPw6", "pipPw7", "kubePw8", "pipSysPw9", "adminConfPw10", "systemdPw11", "projNpmPw12", "projComposerPw13", "FAKEKEYDATA"} {
		if strings.Contains(string(b), pw) {
			t.Fatalf("secret %s leaked into provenance", pw)
		}
	}
}

// Probe portability: the sed-based systemd step and the home-dir glob must run
// under busybox sh / dash too, not just the dev machine's /bin/sh.
func TestCredProbeStoresPortableShell(t *testing.T) {
	ran := false
	for _, sh := range []string{"dash", "busybox"} {
		p, err := exec.LookPath(sh)
		if err != nil {
			continue
		}
		ran = true
		root := t.TempDir()
		plant(t, root, "root/.netrc", "machine h login u password portNetrc1\n")
		plant(t, root, "etc/systemd/system/app.service", "[Service]\nEnvironmentFile=/etc/app/db.env\n")
		plant(t, root, "etc/app/db.env", "DB_PASSWORD=portSystemd2\n")
		args := []string{"-c", credProbeScript(nil, nil, nil)}
		if sh == "busybox" {
			args = append([]string{"sh"}, args...)
		}
		cmd := exec.Command(p, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "BRIDGE_ROOT=" + root}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: probe failed: %v\n%s", sh, err, out)
		}
		set := detectorSet(detectCreds(parseCredProbe(string(out)), nil, nil))
		if !set["netrc"] {
			t.Fatalf("%s: netrc (home-dir glob) not detected", sh)
		}
		if !set["systemd_env"] {
			t.Fatalf("%s: systemd_env (sed step) not detected", sh)
		}
	}
	if !ran {
		t.Skip("neither dash nor busybox available")
	}
}
