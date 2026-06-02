package apicatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

const swagger20JSON = `{
  "swagger": "2.0",
  "info": {"title": "Petstore", "version": "1.0.0"},
  "host": "api.example.com",
  "basePath": "/v1",
  "schemes": ["https"],
  "paths": {
    "/pets": {
      "get":  {"operationId": "listPets", "summary": "List pets", "tags": ["pets"]},
      "post": {"summary": "Create a pet", "tags": ["pets"]}
    }
  }
}`

const openapi30YAML = `openapi: 3.0.1
info:
  title: Orders API
  version: 2.3.0
servers:
  - url: https://orders.example.com
paths:
  /orders:
    get:
      operationId: listOrders
      tags: [orders]
  /orders/{id}:
    get:
      summary: Get order
      tags: [orders]
`

const openapi31JSON = `{
  "openapi": "3.1.0",
  "info": {"title": "Billing", "version": "0.9.0"},
  "servers": [{"url": "https://billing.example.com/api"}],
  "paths": {
    "/invoices": {"get": {"operationId": "listInvoices", "tags": ["billing"]}}
  }
}`

func TestParse_Swagger20(t *testing.T) {
	ps, err := Parse([]byte(swagger20JSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(ps.SpecVersion, "swagger-") {
		t.Errorf("SpecVersion = %q, want swagger- prefix", ps.SpecVersion)
	}
	if ps.Title != "Petstore" || ps.VersionLabel != "1.0.0" {
		t.Errorf("title/version = %q/%q", ps.Title, ps.VersionLabel)
	}
	if ps.ExternalURL != "https://api.example.com/v1" {
		t.Errorf("ExternalURL = %q", ps.ExternalURL)
	}
	if len(ps.Operations) != 2 {
		t.Fatalf("got %d operations, want 2", len(ps.Operations))
	}
	keys := opKeySet(ps.Operations)
	if !keys["listPets"] {
		t.Errorf("expected operationId-based op_key 'listPets', got %v", keys)
	}
	if !keys["POST /pets"] {
		t.Errorf("expected method+path op_key 'POST /pets' for the op without operationId, got %v", keys)
	}
	// Canonical JSON must be valid JSON regardless of input format.
	assertValidJSON(t, ps.SpecJSON)
}

func TestParse_OpenAPI30_YAML(t *testing.T) {
	ps, err := Parse([]byte(openapi30YAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(ps.SpecVersion, "openapi-3") {
		t.Errorf("SpecVersion = %q, want openapi-3 prefix", ps.SpecVersion)
	}
	if ps.Title != "Orders API" || ps.VersionLabel != "2.3.0" {
		t.Errorf("title/version = %q/%q", ps.Title, ps.VersionLabel)
	}
	if ps.ExternalURL != "https://orders.example.com" {
		t.Errorf("ExternalURL = %q", ps.ExternalURL)
	}
	if len(ps.Operations) != 2 {
		t.Fatalf("got %d operations, want 2", len(ps.Operations))
	}
	keys := opKeySet(ps.Operations)
	if !keys["listOrders"] || !keys["GET /orders/{id}"] {
		t.Errorf("unexpected op_keys: %v", keys)
	}
	assertValidJSON(t, ps.SpecJSON)
}

func TestParse_OpenAPI31_JSON(t *testing.T) {
	ps, err := Parse([]byte(openapi31JSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(ps.SpecVersion, "openapi-3.1") {
		t.Errorf("SpecVersion = %q, want openapi-3.1 prefix", ps.SpecVersion)
	}
	if ps.ExternalURL != "https://billing.example.com/api" {
		t.Errorf("ExternalURL = %q", ps.ExternalURL)
	}
	if len(ps.Operations) != 1 || ps.Operations[0].OpKey != "listInvoices" {
		t.Errorf("operations = %+v", ps.Operations)
	}
	if ps.SpecHash == "" {
		t.Error("SpecHash not computed")
	}
}

func TestParse_Empty(t *testing.T) {
	if _, err := Parse([]byte("   ")); err == nil {
		t.Error("expected error for empty spec")
	}
}

func TestParse_Garbage(t *testing.T) {
	if _, err := Parse([]byte("this is not a spec: : :")); err == nil {
		t.Error("expected error for unparseable spec")
	}
}

func opKeySet(ops []Operation) map[string]bool {
	set := make(map[string]bool, len(ops))
	for _, o := range ops {
		set[o.OpKey] = true
	}
	return set
}

func assertValidJSON(t *testing.T, b []byte) {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("canonical spec is not valid JSON: %v", err)
	}
}
