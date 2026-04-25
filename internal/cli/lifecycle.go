package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/callmejustdodo/notion-link/internal/cache"
	"github.com/callmejustdodo/notion-link/internal/db"
	"github.com/callmejustdodo/notion-link/internal/render"
)

// pagesDirRe matches `<cache>/pages/<dashed-uuid>.md`.
var pagesDirRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.md$`)

func newSyncCmd() *cobra.Command {
	var (
		dbPath string
		cmode  string
		dir    string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-render every page already present in the cache.",
		Long: `Walks <cache>/pages/ and re-renders every <id>.md from notion.db.
Use this after editing pages in Notion to refresh the symlinks pointing
into the cache. The set of cross-link resolutions is restricted to pages
that already exist in the cache.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer conn.Close()

			cacheDir, _, err := cache.Resolve(cmode, dir)
			if err != nil {
				return fmt.Errorf("resolve cache dir: %w", err)
			}
			pagesDir := filepath.Join(cacheDir, "pages")
			entries, err := os.ReadDir(pagesDir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Fprintln(cmd.OutOrStdout(), "(no cached pages)")
					return nil
				}
				return err
			}

			var ids []string
			for _, e := range entries {
				if e.IsDir() || !pagesDirRe.MatchString(e.Name()) {
					continue
				}
				ids = append(ids, strings.TrimSuffix(e.Name(), ".md"))
			}
			if len(ids) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no cached pages)")
				return nil
			}

			ctx := cmd.Context()
			knownIDs := make(map[string]bool, len(ids))
			for _, id := range ids {
				knownIDs[id] = true
			}
			tool := "notion-link " + Version
			now := time.Now()

			updated, missing := 0, 0
			for _, id := range ids {
				page, err := conn.GetPage(ctx, id)
				if err != nil {
					if errors.Is(err, db.ErrNotFound) {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s: removed in Notion (skipped)\n", id)
						missing++
						continue
					}
					return fmt.Errorf("load %s: %w", id, err)
				}
				root, err := conn.LoadTree(ctx, id, 0, false)
				if err != nil {
					return fmt.Errorf("load tree %s: %w", id, err)
				}
				md := render.Page(page, root, render.Options{
					SpaceName:       conn.SpaceName(ctx, page.SpaceID),
					LastEdited:      timeFromMillis(page.LastEditedTimeMillis),
					SourceURL:       "https://www.notion.so/" + strings.ReplaceAll(page.ID, "-", ""),
					ResolvePageRef:  syncResolver(ctx, conn, knownIDs),
					LookupPageTitle: func(id string) string { return conn.LookupTitle(ctx, id) },
					ExportedAt:      now,
					Tool:            tool,
				})
				dst := filepath.Join(pagesDir, id+".md")
				if err := cache.WriteAtomic(dst, []byte(md)); err != nil {
					return fmt.Errorf("write %s: %w", dst, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", id, oneLine(page.Title))
				updated++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nrefreshed %d page(s)", updated)
			if missing > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), ", skipped %d missing", missing)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to notion.db.")
	cmd.Flags().StringVar(&cmode, "cache", "auto", "Cache location: auto | global | repo | <path>.")
	cmd.Flags().StringVar(&dir, "dir", ".", "Hint dir used to detect repo cache mode.")
	return cmd
}

func syncResolver(ctx context.Context, conn *db.Conn, known map[string]bool) func(string) render.PageRef {
	return func(id string) render.PageRef {
		if known[id] {
			return render.PageRef{
				Title:    conn.LookupTitle(ctx, id),
				Link:     id + ".md",
				Internal: true,
			}
		}
		return render.PageRef{Title: conn.LookupTitle(ctx, id)}
	}
}

func newUnlinkCmd() *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "unlink <path>...",
		Short: "Remove a symlink created by notion-link (and optionally its cache file).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			for _, p := range args {
				info, err := os.Lstat(p)
				if err != nil {
					return fmt.Errorf("stat %s: %w", p, err)
				}
				if info.Mode()&os.ModeSymlink == 0 {
					return fmt.Errorf("%s is not a symlink (refusing to remove)", p)
				}
				target, err := os.Readlink(p)
				if err != nil {
					return fmt.Errorf("readlink %s: %w", p, err)
				}
				abs := target
				if !filepath.IsAbs(target) {
					abs = filepath.Join(filepath.Dir(p), target)
				}
				if err := os.Remove(p); err != nil {
					return fmt.Errorf("remove symlink: %w", err)
				}
				fmt.Fprintf(out, "removed symlink %s\n", p)
				if purge {
					if err := os.Remove(abs); err != nil {
						if !errors.Is(err, os.ErrNotExist) {
							return fmt.Errorf("purge cache: %w", err)
						}
					} else {
						fmt.Fprintf(out, "purged cache    %s\n", abs)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "Also delete the cache file the symlink pointed to.")
	return cmd
}
