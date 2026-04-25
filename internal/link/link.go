// Package link creates the user-facing symlinks that point into the cache.
package link

import (
	"strings"
	"unicode"
)

// Slugify turns a Notion page title into a filesystem-safe filename stem.
// It does not touch a file extension; callers append `.md` themselves.
func Slugify(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return "untitled"
	}

	var b strings.Builder
	b.Grow(len(t))
	for _, r := range t {
		switch {
		case r == '/' || r == '\\' || r == ':':
			b.WriteRune(' ')
		case unicode.IsControl(r):
			// drop
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if out == "" {
		return "untitled"
	}
	return out
}
