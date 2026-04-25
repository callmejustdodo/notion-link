// Package resolve turns user-supplied page identifiers (URLs, UUIDs, compact ids)
// into the canonical dashed-UUID form Notion stores in notion.db.
package resolve

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	// dashedRe matches a canonical 8-4-4-4-12 UUID.
	dashedRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	// compactRe matches a 32-char hex id with no dashes (Notion URL form).
	compactRe = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	// trailingIDRe pulls the trailing 32-hex id off a Notion URL slug like
	// "https://www.notion.so/Some-Title-2bd2cfba780a806cb320e65c4f924ae7".
	trailingIDRe = regexp.MustCompile(`([0-9a-fA-F]{32})$`)

	// ErrInvalid is returned when no recognizable id can be extracted.
	ErrInvalid = errors.New("could not parse a Notion page id from input")
)

// PageID accepts a Notion URL, a dashed UUID, or a 32-char compact id and
// returns the canonical dashed-UUID form used in notion.db.
func PageID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", ErrInvalid
	}

	if dashedRe.MatchString(s) {
		return strings.ToLower(s), nil
	}
	if compactRe.MatchString(s) {
		return dashify(strings.ToLower(s)), nil
	}

	if u, err := url.Parse(s); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "notion") {
		// Try the path tail first.
		path := strings.TrimSuffix(u.Path, "/")
		segs := strings.Split(path, "/")
		tail := segs[len(segs)-1]
		if m := trailingIDRe.FindStringSubmatch(tail); m != nil {
			return dashify(strings.ToLower(m[1])), nil
		}
		// Fall back to query param ?p= used in some share URLs.
		if p := u.Query().Get("p"); p != "" {
			if compactRe.MatchString(p) {
				return dashify(strings.ToLower(p)), nil
			}
			if dashedRe.MatchString(p) {
				return strings.ToLower(p), nil
			}
		}
	}

	// Last resort: scan the whole string for a 32-hex run.
	if m := trailingIDRe.FindStringSubmatch(strings.ReplaceAll(s, "-", "")); m != nil {
		return dashify(strings.ToLower(m[1])), nil
	}

	return "", fmt.Errorf("%w: %q", ErrInvalid, input)
}

func dashify(compact string) string {
	// 8-4-4-4-12
	return compact[0:8] + "-" + compact[8:12] + "-" + compact[12:16] + "-" + compact[16:20] + "-" + compact[20:32]
}
