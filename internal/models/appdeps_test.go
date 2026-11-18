package models

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseAppDepCatalogExamples(t *testing.T) {
	data, err := os.ReadFile("testdata/appdeps/shop-api.yml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseAppDepCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	if c.CI != "shop-api" || len(c.Components) != 3 || c.Git == nil || c.Git.Path != "example-corp/shop-api" {
		t.Fatalf("got %+v", c)
	}
	if api := c.Components[0]; api.Name != "api" || api.Protocol != "https" || api.Port != 8443 || api.Criticality != "high" {
		t.Errorf("component api = %+v", api)
	}
	up := c.Dependencies.Upstream
	if len(up) != 1 || up[0].CI != "payment-gateway" || up[0].Component != "" || up[0].Technology != "rest-api" || up[0].Port != 443 {
		t.Errorf("upstream = %+v", up)
	}
	down := c.Dependencies.Downstream
	if len(down) != 2 || down[0].Component != "frontend" || down[0].LocalComponent != "api" || down[1].CI != "bi-dashboard" {
		t.Errorf("downstream = %+v", down)
	}

	data, _ = os.ReadFile("testdata/appdeps/shop-web.yml")
	w, err := ParseAppDepCatalog(data)
	if err != nil || w.CI != "shop-web" || len(w.Dependencies.Downstream) != 0 {
		t.Fatalf("shop-web: %+v, %v", w, err)
	}
}

func TestParseAppDepCatalogMinimalHasNoNullArrays(t *testing.T) {
	c, err := ParseAppDepCatalog([]byte("ci: lonely\nunknown_key: ignored\nmetadata:\n  1: one\n  nested: {2: two}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("metadata with non-string keys must encode: %v", err)
	}
	for _, s := range []string{`"owners":[]`, `"components":[]`, `"upstream":[]`, `"downstream":[]`, `"1":"one"`, `"nested":{"2":"two"}`} {
		if !strings.Contains(string(b), s) {
			t.Errorf("%s missing in %s", s, b)
		}
	}
}

func TestParseAppDepCatalogErrors(t *testing.T) {
	cases := map[string]string{
		"invalid yaml":  "ci: [unclosed",
		"not a mapping": "- ci: a\n- ci: b\n",
		"empty file":    "",
		"no ci":         "description: nobody\n",
		"wrong type":    "ci: a\ncomponents:\n  - name: api\n    port: abc\n",
	}
	for name, in := range cases {
		if _, err := ParseAppDepCatalog([]byte(in)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestAppDepFileURL(t *testing.T) {
	if got := AppDepFileURL("https://gitlab.example/g/r/", "main"); got != "https://gitlab.example/g/r/-/blob/main/catalog/dependencies.yml" {
		t.Errorf("got %q", got)
	}
	if AppDepFileURL("", "main") != "" {
		t.Error("no web url → no link")
	}
}
