package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteError_ShapeAndContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 403, "forbidden")

	if got := rec.Code; got != 403 {
		t.Fatalf("status = %d, want 403", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (raw=%q)", err, rec.Body.String())
	}
	if body["error"] != "forbidden" {
		t.Fatalf(`body = %v, want {"error":"forbidden"}`, body)
	}
}

func TestWriteJSON_EncodesValue(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, 201, map[string]any{"id": 7})
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["id"].(float64) != 7 {
		t.Fatalf("id = %v, want 7", body["id"])
	}
}
