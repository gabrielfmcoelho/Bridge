package api

import (
	"encoding/json"
	"testing"
)

// TestListEnvelope_ShapeAndPagination locks the R4 list contract: every list
// endpoint returns { "data": [...], "meta": {page,per_page,total} }, and the
// shared PageParams windowing works (per_page limits the page; total is the
// full count; per_page=0 returns all). Exercised through the real
// secrets handler + jsonPaged helper via the secret API env (alice=admin sees
// all three seeded secrets).
func TestListEnvelope_ShapeAndPagination(t *testing.T) {
	env := newSecretAPIEnv(t)

	type envelope struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"meta"`
	}
	get := func(path string) envelope {
		t.Helper()
		rec := env.do(env.alice, "GET", path, "")
		if rec.Code != 200 {
			t.Fatalf("GET %s: status %d body=%s", path, rec.Code, rec.Body)
		}
		// Reject a bare array: the body MUST be an object with data+meta.
		if b := rec.Body.Bytes(); len(b) == 0 || b[0] != '{' {
			t.Fatalf("GET %s: expected envelope object, got %s", path, rec.Body)
		}
		var e envelope
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
			t.Fatalf("GET %s: decode envelope: %v (body=%s)", path, err, rec.Body)
		}
		return e
	}

	// Unbounded: meta.total equals the number of rows returned.
	full := get("/api/secrets")
	if full.Meta.Total != len(full.Data) {
		t.Fatalf("unbounded: meta.total=%d but len(data)=%d", full.Meta.Total, len(full.Data))
	}
	if full.Meta.Total < 2 {
		t.Fatalf("fixture should seed >=2 secrets visible to admin, got %d", full.Meta.Total)
	}

	// per_page=1 → exactly one row, but total is still the full count.
	p1 := get("/api/secrets?per_page=1")
	if len(p1.Data) != 1 {
		t.Errorf("per_page=1: len(data)=%d, want 1", len(p1.Data))
	}
	if p1.Meta.PerPage != 1 || p1.Meta.Page != 1 || p1.Meta.Total != full.Meta.Total {
		t.Errorf("per_page=1 meta = %+v, want {page:1 per_page:1 total:%d}", p1.Meta, full.Meta.Total)
	}

	// page 2 returns a different row than page 1 (windowing actually advances).
	p2 := get("/api/secrets?per_page=1&page=2")
	if len(p2.Data) != 1 {
		t.Fatalf("page 2: len(data)=%d, want 1", len(p2.Data))
	}
	if p1.Data[0]["id"] == p2.Data[0]["id"] {
		t.Errorf("page 1 and page 2 returned the same row id=%v", p1.Data[0]["id"])
	}

	// per_page=0 is the unbounded sentinel: returns everything.
	all := get("/api/secrets?per_page=0")
	if len(all.Data) != full.Meta.Total {
		t.Errorf("per_page=0: len(data)=%d, want all %d", len(all.Data), full.Meta.Total)
	}
}
