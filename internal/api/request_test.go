package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeBody(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/x", strings.NewReader(`{"name":"a"}`))
		var body struct {
			Name string `json:"name"`
		}
		if !decodeBody(rec, req, &body) {
			t.Fatalf("decodeBody returned false for valid JSON")
		}
		if body.Name != "a" {
			t.Fatalf("name = %q, want a", body.Name)
		}
	})

	t.Run("malformed -> 400", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/x", strings.NewReader(`{not json`))
		var body map[string]any
		if decodeBody(rec, req, &body) {
			t.Fatalf("decodeBody returned true for malformed JSON")
		}
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}

func TestRequireFields(t *testing.T) {
	t.Run("all present", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if !requireFields(rec, map[string]string{"a": "x", "b": "y"}) {
			t.Fatalf("requireFields returned false when all present")
		}
	})

	t.Run("blank field -> 400 naming it", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if requireFields(rec, map[string]string{"source_url": "   "}) {
			t.Fatalf("requireFields returned true for blank field")
		}
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "source_url") {
			t.Fatalf("body %q should name the missing field", rec.Body.String())
		}
	})
}
