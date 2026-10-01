// Package kcfake is an in-memory Keycloak (token endpoint + the slice of the
// admin REST API kcadmin uses) and a SEAD-style API (/escopos, /admin/uso)
// for tests. Every Keycloak request must carry Host; one that doesn't is
// recorded in BadHost.
package kcfake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Client is a stored client.
type Client struct {
	ID       string
	ClientID string
	Desc     string
	Enabled  bool
	Secret   string
	Optional []string // scope names
	Mappers  map[string]map[string]any
}

// Admin and usage service accounts the fake knows (client id → secret).
const (
	AdminClient = "kc-bridge-admin"
	AdminSecret = "s3cret"
	UsageClient = "kc-bridge-usage"
	UsageSecret = "u5age"
)

// Keycloak is the fake. Lock Mu before reading its maps from a test.
// Tokens are "tok-<client id>|<scope>"; admin endpoints only take
// AdminClient's (or Admins'), /admin/uso only UsageClient's.
type Keycloak struct {
	// Creds are the client credentials the token endpoint accepts.
	Creds map[string]string
	// Admins are the client ids whose tokens the admin API accepts.
	Admins map[string]bool
	// UsageBearers records the Authorization of every /admin/uso call.
	UsageBearers []string
	Mu           sync.Mutex
	Host         string
	Realm        string
	Clients      map[string]*Client // by internal id
	Scopes       map[string]string  // name → id
	BadHost      []string           // requests that came without Host
	Usage        map[string]int64   // clientId → lifetime, for the fake API
	// UsageScopes records the scope parameter of each token request.
	TokenScopes []string
	seq         int
	t           *testing.T
}

// New starts a fake Keycloak expecting host and a fake API. kc is the
// Keycloak server, api the API (its /escopos answers apiScopes).
func New(t *testing.T, host, realm string, apiScopes []map[string]any) (*Keycloak, *httptest.Server, *httptest.Server) {
	f := &Keycloak{Host: host, Realm: realm, Clients: map[string]*Client{},
		Creds:  map[string]string{AdminClient: AdminSecret, UsageClient: UsageSecret},
		Admins: map[string]bool{AdminClient: true},
		Scopes: map[string]string{
			"offline_access": "s-offline", "profile": "s-profile",
		}, Usage: map[string]int64{}, t: t}
	kc := httptest.NewServer(http.HandlerFunc(f.serve))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/escopos":
			_ = json.NewEncoder(w).Encode(apiScopes)
		case r.URL.Path == "/admin/uso":
			f.Mu.Lock()
			f.UsageBearers = append(f.UsageBearers, r.Header.Get("Authorization"))
			n := f.Usage[r.URL.Query().Get("cliente")]
			f.Mu.Unlock()
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok-"+UsageClient+"|") ||
				!strings.HasSuffix(r.Header.Get("Authorization"), ":admin.uso") {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":{"code":"AUTH_SCOPE_DENIED","message":"escopo admin.uso ausente"}}`)
				return
			}
			c := r.URL.Query().Get("cliente")
			_ = json.NewEncoder(w).Encode(map[string]any{"cliente": c, "lifetime": n, "last_used_at": nil, "daily": map[string]int64{"2026-09-30": n}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(kc.Close)
	t.Cleanup(api.Close)
	return f, kc, api
}

func (f *Keycloak) id(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

func (f *Keycloak) rep(c *Client) map[string]any {
	mappers := []map[string]any{}
	for _, m := range c.Mappers {
		mappers = append(mappers, m)
	}
	return map[string]any{"id": c.ID, "clientId": c.ClientID, "description": c.Desc, "enabled": c.Enabled,
		"optionalClientScopes": append([]string{"offline_access"}, c.Optional...), "protocolMappers": mappers}
}

func (f *Keycloak) serve(w http.ResponseWriter, r *http.Request) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if r.Host != f.Host {
		f.BadHost = append(f.BadHost, r.Method+" "+r.URL.Path+" Host="+r.Host)
	}
	realm := "/realms/" + f.Realm
	admin := "/admin/realms/" + f.Realm
	p := r.URL.Path
	write := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if v != nil {
			_ = json.NewEncoder(w).Encode(v)
		}
	}
	if p == realm+"/protocol/openid-connect/token" {
		_ = r.ParseForm()
		cid := r.PostForm.Get("client_id")
		if sec, ok := f.Creds[cid]; !ok || sec != r.PostForm.Get("client_secret") {
			write(http.StatusUnauthorized, map[string]string{"error": "unauthorized_client", "error_description": "Invalid client credentials"})
			return
		}
		f.TokenScopes = append(f.TokenScopes, r.PostForm.Get("scope"))
		write(http.StatusOK, map[string]any{"access_token": "tok-" + cid + "|" + r.PostForm.Get("scope"), "expires_in": 300})
		return
	}
	bearerClient, _, _ := strings.Cut(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer tok-"), "|")
	if !strings.HasPrefix(p, admin+"/") || !f.Admins[bearerClient] {
		write(http.StatusUnauthorized, map[string]string{"error": "HTTP 401 Unauthorized"})
		return
	}
	segs := strings.Split(strings.TrimPrefix(p, admin+"/"), "/")
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	switch {
	case segs[0] == "client-scopes" && len(segs) == 1 && r.Method == http.MethodGet:
		out := []map[string]string{}
		for n, id := range f.Scopes {
			out = append(out, map[string]string{"id": id, "name": n})
		}
		write(http.StatusOK, out)
	case segs[0] == "client-scopes" && len(segs) == 1 && r.Method == http.MethodPost:
		name, _ := body["name"].(string)
		if f.Scopes[name] != "" {
			write(http.StatusConflict, map[string]string{"errorMessage": "Client Scope " + name + " already exists"})
			return
		}
		f.Scopes[name] = f.id("s")
		write(http.StatusCreated, nil)
	case segs[0] != "clients":
		write(http.StatusNotFound, map[string]string{"error": "unknown " + p})
	case len(segs) == 1 && r.Method == http.MethodGet:
		q, search := r.URL.Query().Get("clientId"), r.URL.Query().Get("search") == "true"
		out := []map[string]any{}
		for _, c := range f.Clients {
			if q == "" || c.ClientID == q || (search && strings.Contains(c.ClientID, q)) {
				out = append(out, f.rep(c))
			}
		}
		write(http.StatusOK, out)
	case len(segs) == 1 && r.Method == http.MethodPost:
		cid, _ := body["clientId"].(string)
		for _, c := range f.Clients {
			if c.ClientID == cid {
				write(http.StatusConflict, map[string]string{"errorMessage": "Client " + cid + " already exists"})
				return
			}
		}
		if body["serviceAccountsEnabled"] != true || body["publicClient"] != false || body["standardFlowEnabled"] != false {
			f.t.Errorf("kcfake: client %s created with the wrong flags: %v", cid, body)
		}
		c := &Client{ID: f.id("c"), ClientID: cid, Enabled: true, Secret: f.id("secret"), Mappers: map[string]map[string]any{}}
		c.Desc, _ = body["description"].(string)
		if ms, ok := body["protocolMappers"].([]any); ok {
			for _, m := range ms {
				mm := m.(map[string]any)
				mm["id"] = f.id("m")
				c.Mappers[mm["id"].(string)] = mm
			}
		}
		f.Clients[c.ID] = c
		w.Header().Set("Location", "http://"+r.Host+admin+"/clients/"+c.ID)
		write(http.StatusCreated, nil)
	default:
		c := f.Clients[segs[1]]
		if c == nil {
			write(http.StatusNotFound, map[string]string{"error": "Could not find client"})
			return
		}
		f.client(c, segs[2:], r.Method, body, write)
	}
}

func (f *Keycloak) client(c *Client, rest []string, method string, body map[string]any, write func(int, any)) {
	switch {
	case len(rest) == 0 && method == http.MethodGet:
		write(http.StatusOK, f.rep(c))
	case len(rest) == 0 && method == http.MethodPut:
		if body["clientId"] != c.ClientID {
			f.t.Errorf("kcfake: PUT client without its full representation: %v", body)
		}
		c.Enabled, _ = body["enabled"].(bool)
		write(http.StatusNoContent, nil)
	case len(rest) == 0 && method == http.MethodDelete:
		delete(f.Clients, c.ID)
		write(http.StatusNoContent, nil)
	case rest[0] == "client-secret" && method == http.MethodGet:
		write(http.StatusOK, map[string]string{"type": "secret", "value": c.Secret})
	case rest[0] == "client-secret" && method == http.MethodPost:
		c.Secret = f.id("secret")
		write(http.StatusOK, map[string]string{"type": "secret", "value": c.Secret})
	case rest[0] == "optional-client-scopes" && len(rest) == 2:
		name := ""
		for n, id := range f.Scopes {
			if id == rest[1] {
				name = n
			}
		}
		if name == "" {
			write(http.StatusNotFound, map[string]string{"error": "Client scope not found"})
			return
		}
		out := []string{}
		for _, s := range c.Optional {
			if s != name {
				out = append(out, s)
			}
		}
		if method == http.MethodPut {
			out = append(out, name)
		}
		c.Optional = out
		write(http.StatusNoContent, nil)
	case rest[0] == "protocol-mappers" && len(rest) == 2 && method == http.MethodGet:
		out := []map[string]any{}
		for _, m := range c.Mappers {
			out = append(out, m)
		}
		write(http.StatusOK, out)
	case rest[0] == "protocol-mappers" && len(rest) == 2 && method == http.MethodPost:
		body["id"] = f.id("m")
		c.Mappers[body["id"].(string)] = body
		write(http.StatusCreated, nil)
	case rest[0] == "protocol-mappers" && len(rest) == 3 && method == http.MethodPut:
		c.Mappers[rest[2]] = body
		write(http.StatusNoContent, nil)
	case rest[0] == "protocol-mappers" && len(rest) == 3 && method == http.MethodDelete:
		delete(c.Mappers, rest[2])
		write(http.StatusNoContent, nil)
	default:
		write(http.StatusNotFound, map[string]string{"error": "unknown"})
	}
}

// ByClientID returns the stored client with clientID (lock Mu first).
func (f *Keycloak) ByClientID(clientID string) *Client {
	for _, c := range f.Clients {
		if c.ClientID == clientID {
			return c
		}
	}
	return nil
}
