// Package cache decides where rendered Markdown files for Notion pages live.
//
// Two location modes are supported:
//
//   - "global": $XDG_CACHE_HOME/notion-link (or ~/Library/Caches/notion-link
//     on macOS). One cache per user, shared across all symlink directories.
//
//   - "repo": <git-root>/.notion. Used when the symlink directory is inside
//     a git work tree, so both the cache files and the (relative) symlinks
//     can be committed and remain valid on every clone.
//
// "auto" picks "repo" when --dir is inside a git work tree, "global" otherwise.
// An explicit path overrides everything.
package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Mode is the resolved cache strategy reported back to the caller.
type Mode string

const (
	ModeGlobal Mode = "global"
	ModeRepo   Mode = "repo"
	ModeCustom Mode = "custom"
)

// Resolve returns the cache directory and the resolved mode for the given
// user-supplied --cache flag value and --dir.
func Resolve(flag, dir string) (string, Mode, error) {
	switch flag {
	case "", "auto":
		if root, ok := findGitRoot(dir); ok {
			return filepath.Join(root, ".notion"), ModeRepo, nil
		}
		p, err := globalCacheDir()
		return p, ModeGlobal, err
	case "global":
		p, err := globalCacheDir()
		return p, ModeGlobal, err
	case "repo":
		root, ok := findGitRoot(dir)
		if !ok {
			return "", "", fmt.Errorf("--cache=repo but %s is not inside a git work tree", dir)
		}
		return filepath.Join(root, ".notion"), ModeRepo, nil
	default:
		// Treat as an explicit path.
		abs, err := filepath.Abs(flag)
		if err != nil {
			return "", "", err
		}
		return abs, ModeCustom, nil
	}
}

func globalCacheDir() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Caches", "notion-link"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "notion-link"), nil
}

// findGitRoot walks up from start looking for a directory containing .git.
// Returns the directory and true if found.
func findGitRoot(start string) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	cur := abs
	for {
		info, err := os.Stat(filepath.Join(cur, ".git"))
		if err == nil && (info.IsDir() || !info.IsDir()) {
			return cur, true
		}
		if !errors.Is(err, os.ErrNotExist) && err != nil {
			return "", false
		}
		parent := filepath.Dir(cur)
		if parent == cur || strings.TrimSpace(parent) == "" {
			return "", false
		}
		cur = parent
	}
}
