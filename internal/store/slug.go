package store

import "strings"

// Slugify lowercases and collapses anything that isn't [a-z0-9] into '-'.
// Admins can always hand-edit the slug; this only provides the default.
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, c := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteRune(c)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
