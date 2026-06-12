package outline

import "strings"

// Excerpt renders a short, single-line preview of an Outline document's markdown
// body: leading heading markers stripped, newlines flattened, truncated to n
// runes with an ellipsis. Moved from the api handler (buildExcerpt) — it's
// Outline-content presentation.
func Excerpt(text string, n int) string {
	t := strings.TrimSpace(text)
	// Drop leading markdown heading markers.
	for strings.HasPrefix(t, "#") || strings.HasPrefix(t, " ") {
		t = strings.TrimLeft(t, "# ")
	}
	t = strings.ReplaceAll(t, "\n", " ")
	if len(t) <= n {
		return t
	}
	runes := []rune(t)
	if len(runes) <= n {
		return t
	}
	return string(runes[:n]) + "…"
}

// DisplayName is the nil-safe user label (was the api handler's userName).
func (u *User) DisplayName() string {
	if u == nil {
		return ""
	}
	return u.Name
}
