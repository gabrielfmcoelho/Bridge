package api

import "testing"

func TestScrubPath(t *testing.T) {
	cases := map[string]string{
		"/api/share/abc123":              "/api/share/[REDACTED]",
		"/api/share/longer-token-here":   "/api/share/[REDACTED]",
		"/api/share/":                    "/api/share/[REDACTED]",
		"/api/secrets/123":               "/api/secrets/123",
		"/api/secrets/123/share-links":   "/api/secrets/123/share-links",
		"/api/share-anything-else/no":    "/api/share-anything-else/no", // prefix must match exactly
		"":                               "",
		"/api/share/with/extra/segments": "/api/share/[REDACTED]",
	}
	for in, want := range cases {
		if got := scrubPath(in); got != want {
			t.Errorf("scrubPath(%q) = %q, want %q", in, got, want)
		}
	}
}
