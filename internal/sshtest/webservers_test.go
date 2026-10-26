package sshtest

import (
	"encoding/base64"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

const nginxTFixture = `nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
nginx: configuration file /etc/nginx/nginx.conf test is successful
# configuration file /etc/nginx/nginx.conf:
user www-data;
events { worker_connections 768; }
http {
    upstream app_backend {
        server 127.0.0.1:8080;
        server 127.0.0.1:8081 backup;
    }
    server {
        listen 80 default_server;
        server_name _;
        root /var/www/html;
    }
    server {
        listen 443 ssl http2;
        server_name portal.sead.pi.gov.br www.portal.sead.pi.gov.br;
        location / {
            proxy_pass http://app_backend;
        }
        location /api/ { proxy_pass http://127.0.0.1:3000/; }
    }
    # configuration file /etc/nginx/sites-enabled/gestor:
    server {
        listen 80;
        server_name gestor.sead.pi.gov.br ~^(?<sub>.+)\.example\.com$;
        root /banco/www/gestor;
        location ~ \.php$ {
            root /ignored/location/root;
            fastcgi_pass unix:/run/php/php8.1-fpm.sock;
        }
    }
}
`

func TestParseNginxSites(t *testing.T) {
	sites := parseNginxSites(nginxTFixture)
	if len(sites) != 3 {
		t.Fatalf("sites = %d, want 3: %+v", len(sites), sites)
	}
	want := []WebSite{
		{Names: []string{"_"}, Listen: []int{80}, Root: "/var/www/html"},
		{Names: []string{"portal.sead.pi.gov.br", "www.portal.sead.pi.gov.br"}, Listen: []int{443}, TLS: true,
			Proxy: []string{"127.0.0.1:8080", "127.0.0.1:8081", "http://127.0.0.1:3000/"}},
		{Names: []string{"gestor.sead.pi.gov.br", `~^(?<sub>.+)\.example\.com$`}, Listen: []int{80},
			Root: "/banco/www/gestor", FastCGI: []string{"unix:/run/php/php8.1-fpm.sock"}},
	}
	for i := range want {
		if !reflect.DeepEqual(sites[i], want[i]) {
			t.Errorf("site %d =\n %+v\nwant\n %+v", i, sites[i], want[i])
		}
	}
}

// The port → names map that labels listening ports keeps its old shape.
func TestParseNginxListens(t *testing.T) {
	got := parseNginxListens(nginxTFixture)
	want := map[int][]string{
		80:  {"_", "gestor.sead.pi.gov.br", `~^(?<sub>.+)\.example\.com$`},
		443: {"portal.sead.pi.gov.br", "www.portal.sead.pi.gov.br"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listens = %v, want %v", got, want)
	}
	if len(parseNginxListens("")) != 0 {
		t.Fatal("empty config should give an empty map")
	}
}

func TestParseApacheS(t *testing.T) {
	out := `AH00558: apache2: Could not reliably determine the server's fully qualified domain name
VirtualHost configuration:
*:443                  secure.example.gov.br (/etc/apache2/sites-enabled/default-ssl.conf:2)
*:80                   is a NameVirtualHost
         default server www.example.gov.br (/etc/apache2/sites-enabled/000-default.conf:1)
         port 80 namevhost www.example.gov.br (/etc/apache2/sites-enabled/000-default.conf:1)
                 alias example.gov.br
                 wild alias *.example.gov.br
         port 80 namevhost intranet.local (/etc/apache2/sites-enabled/intranet.conf:1)
ServerRoot: "/etc/apache2"
Main DocumentRoot: "/var/www/html"
`
	want := []WebSite{
		{Names: []string{"secure.example.gov.br"}, Listen: []int{443}, TLS: true, Source: "/etc/apache2/sites-enabled/default-ssl.conf:2"},
		{Names: []string{"www.example.gov.br", "example.gov.br", "*.example.gov.br"}, Listen: []int{80}, Source: "/etc/apache2/sites-enabled/000-default.conf:1"},
		{Names: []string{"intranet.local"}, Listen: []int{80}, Source: "/etc/apache2/sites-enabled/intranet.conf:1"},
	}
	if got := parseApacheS(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("apache sites =\n %+v\nwant\n %+v", got, want)
	}
}

func TestParseFPMPools(t *testing.T) {
	out := `/etc/php/8.1/fpm/pool.d/www.conf:[www]
/etc/php/8.1/fpm/pool.d/www.conf:listen = /run/php/php8.1-fpm.sock
/etc/php/7.4/fpm/pool.d/www.conf:[www]
/etc/php/7.4/fpm/pool.d/www.conf:listen = /run/php/php7.4-fpm.sock
/usr/local/etc/php-fpm.d/docker.conf:[global]
/usr/local/etc/php-fpm.d/docker.conf:[api]
/usr/local/etc/php-fpm.d/www.conf:[api]
/usr/local/etc/php-fpm.d/www.conf:listen = 127.0.0.1:9000
/usr/local/etc/php-fpm.d/zz-docker.conf:[global]
/usr/local/etc/php-fpm.d/zz-docker.conf:[api]
/usr/local/etc/php-fpm.d/zz-docker.conf:listen = 9000
`
	// One [api] split over three files (last listen wins); [www] of two PHP
	// versions stays two pools.
	want := []FPMPool{{Name: "api", Listen: "9000"}, {Name: "www", Listen: "/run/php/php8.1-fpm.sock"}, {Name: "www", Listen: "/run/php/php7.4-fpm.sock"}}
	if got := parseFPMPools(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("pools = %+v, want %+v", got, want)
	}
}

func TestParseWebProbe(t *testing.T) {
	raw := "@@nginx\n" + nginxTFixture +
		"@@CTR abc123def456 app nginx:1.25\n@@nginx\nhttp {\n server {\n  listen 80;\n  server_name app.sead.pi.gov.br;\n  location / { fastcgi_pass php:9000; }\n }\n}\n" +
		"@@CTR 0123456789ab php php:8.2-fpm\n@@fpm\n/usr/local/etc/php-fpm.d/www.conf:[www]\n/usr/local/etc/php-fpm.d/www.conf:listen = 9000\n" +
		"@@CTR deadbeef0000 quiet alpine:3\n\n" +
		"@@CTR nothex evil$(x) img\n@@nginx\nhttp { server { listen 80; server_name evil.example; } }\n" +
		"@@CTR 111111111111 locked nginx:1\n@@nginx\nnginx: [emerg] cannot load certificate \"/etc/ssl/x.pem\": (13: Permission denied)\n"
	servers, hostNginx := parseWebProbe(raw)
	if hostNginx != nginxTFixture {
		t.Fatalf("host nginx config not returned intact")
	}
	if len(servers) != 4 {
		t.Fatalf("servers = %d, want 4 (host nginx, app nginx, php fpm, locked error): %+v", len(servers), servers)
	}
	if s := servers[0]; s.Kind != "nginx" || s.Container != "" || len(s.Sites) != 3 {
		t.Errorf("host server = %+v", s)
	}
	if s := servers[1]; s.Kind != "nginx" || s.Container != "app" || s.ContainerID != "abc123def456" || s.ContainerImage != "nginx:1.25" ||
		len(s.Sites) != 1 || s.Sites[0].FastCGI[0] != "php:9000" {
		t.Errorf("app server = %+v", s)
	}
	if s := servers[2]; s.Kind != "php-fpm" || s.Container != "php" || len(s.Pools) != 1 || s.Pools[0].Listen != "9000" {
		t.Errorf("php server = %+v", s)
	}
	if s := servers[3]; s.Container != "locked" || !strings.Contains(s.Error, "Permission denied") || len(s.Sites) != 0 {
		t.Errorf("unreadable config should be an error entry: %+v", s)
	}
}

func TestMarkStoppedHostServers(t *testing.T) {
	servers := []WebServer{{Kind: "nginx"}, {Kind: "apache"}, {Kind: "php-fpm"}, {Kind: "nginx", Container: "app"}}
	markStoppedHostServers(servers, []SystemdService{{Unit: "nginx.service"}, {Unit: "php8.1-fpm.service"}})
	if servers[0].Stopped || !servers[1].Stopped || servers[2].Stopped || servers[3].Stopped {
		t.Fatalf("stopped flags = %v %v %v %v", servers[0].Stopped, servers[1].Stopped, servers[2].Stopped, servers[3].Stopped)
	}
	none := []WebServer{{Kind: "apache"}}
	markStoppedHostServers(none, nil)
	if none[0].Stopped {
		t.Fatal("no unit list must not mark anything stopped")
	}
}

// Both variants must be valid sh and carry the inner script intact; the root
// variant (run through runSudoCmd) retries nothing with sudo -n.
func TestWebProbeScript(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte(webInnerScript))
	for _, asRoot := range []bool{false, true} {
		s := webProbeScript(asRoot)
		if out, err := exec.Command("sh", "-n", "-c", s).CombinedOutput(); err != nil {
			t.Fatalf("root=%v sh -n: %v %s", asRoot, err, out)
		}
		for _, p := range []string{"{MAX}", "{B64}", "{CAP}", "{FALLBACK}"} {
			if strings.Contains(s, p) {
				t.Fatalf("root=%v placeholder %s left unreplaced", asRoot, p)
			}
		}
		if !strings.Contains(s, "printf %s "+b64+" | base64 -d") {
			t.Fatalf("root=%v container part does not carry the inner script", asRoot)
		}
		retries := strings.Contains(s, "SUDO=1") || strings.Contains(s, `D="sudo -n docker"`)
		if retries == asRoot {
			t.Fatalf("root=%v: sudo -n retries present = %v", asRoot, retries)
		}
	}
}
