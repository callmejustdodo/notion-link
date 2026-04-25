// Package db opens the Notion desktop app's local SQLite database in
// read-only mode and exposes typed query helpers.
//
// notion.db lives at ~/Library/Application Support/Notion/notion.db on macOS.
// The Notion app holds a writer connection, so we always open with
// mode=ro and WAL journal mode to avoid lock contention.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by lookup helpers when no row matches.
var ErrNotFound = errors.New("not found")

// Conn wraps *sql.DB with our query helpers.
type Conn struct {
	sql *sql.DB
}

// DefaultPath returns the platform-standard location of notion.db.
// Falls back to the macOS path if the platform isn't recognized,
// since this tool is currently macOS-first.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Notion", "notion.db")
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Notion", "notion.db")
	case "linux":
		return filepath.Join(home, ".config", "Notion", "notion.db")
	default:
		return filepath.Join(home, "Library", "Application Support", "Notion", "notion.db")
	}
}

// Open opens notion.db in read-only mode with WAL so we don't fight
// the running Notion app for the writer lock.
func Open(path string) (*Conn, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("notion.db not readable at %s: %w", path, err)
	}
	// mode=ro opens read-only; WAL files (-wal/-shm) created by the running
	// Notion app are picked up automatically and we never attempt to write.
	dsn := "file:" + url.PathEscape(path) + "?mode=ro&_pragma=busy_timeout(5000)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		_ = d.Close()
		return nil, err
	}
	return &Conn{sql: d}, nil
}

// Close releases the underlying database handle.
func (c *Conn) Close() error {
	if c == nil || c.sql == nil {
		return nil
	}
	return c.sql.Close()
}

// withQuery is a helper that wires the context through a single query.
func (c *Conn) withQuery(ctx context.Context, query string, args ...any) *sql.Row {
	return c.sql.QueryRowContext(ctx, query, args...)
}
