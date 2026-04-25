// Package link creates the user-facing symlinks that point into the cache.
package link

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// CreateOptions controls Create.
type CreateOptions struct {
	// Force overwrites an existing symlink at linkPath. Real files are
	// never overwritten — Create returns an error in that case.
	Force bool
	// PreferRelative makes the symlink target a path relative to the
	// link's parent directory whenever both live under the same git/cache
	// root. Falls back to absolute if relativizing would escape the root.
	PreferRelative bool
}

// Create makes a symlink at linkPath pointing to target.
//
// Parent directories of linkPath are created if missing. If linkPath
// already exists as a symlink and Force is set, it is replaced. If it
// exists as a regular file or directory, Create refuses to overwrite.
func Create(target, linkPath string, opt CreateOptions) (string, error) {
	if target == "" || linkPath == "" {
		return "", errors.New("target and linkPath must both be set")
	}
	target = absOrSelf(target)
	linkPath = absOrSelf(linkPath)

	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir parent: %w", err)
	}

	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return "", fmt.Errorf("%s already exists and is not a symlink (refusing to overwrite)", linkPath)
		}
		if !opt.Force {
			return "", fmt.Errorf("%s already exists; pass --force to replace it", linkPath)
		}
		if err := os.Remove(linkPath); err != nil {
			return "", fmt.Errorf("remove existing symlink: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	linkTarget := target
	if opt.PreferRelative {
		if rel, ok := relIfReachable(filepath.Dir(linkPath), target); ok {
			linkTarget = rel
		}
	}

	if err := os.Symlink(linkTarget, linkPath); err != nil {
		return "", fmt.Errorf("create symlink: %w", err)
	}
	return linkTarget, nil
}

func absOrSelf(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// relIfReachable returns target as a path relative to fromDir when both
// share a common ancestor. Returns ok=false if relativizing would
// require traversing more than 8 parent directories — at that point
// keeping the absolute path is more robust.
func relIfReachable(fromDir, target string) (string, bool) {
	rel, err := filepath.Rel(fromDir, target)
	if err != nil {
		return "", false
	}
	if strings.Count(rel, ".."+string(filepath.Separator)) > 8 {
		return "", false
	}
	return rel, true
}
