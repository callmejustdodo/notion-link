package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/callmejustdodo/notion-link/internal/cache"
	"github.com/callmejustdodo/notion-link/internal/db"
	"github.com/callmejustdodo/notion-link/internal/link"
	"github.com/callmejustdodo/notion-link/internal/model"
	"github.com/callmejustdodo/notion-link/internal/render"
	"github.com/callmejustdodo/notion-link/internal/resolve"
)

type linkOpts struct {
	dir       string
	name      string
	dbPath    string
	cacheMode string // global | repo | <explicit-path>
	recursive bool
	depth     int
	layout    string // flat | tree
	force     bool
	dryRun    bool
}

func newLinkCmd() *cobra.Command {
	o := &linkOpts{}
	cmd := &cobra.Command{
		Use:   "link <page>",
		Short: "Render a Notion page to Markdown and symlink it into a directory.",
		Long: `Resolve a Notion page (URL, dashed UUID, or 32-char compact id),
render it to Markdown into a cache, and create a symlink in --dir
pointing at the cache file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLink(cmd, args[0], o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.dir, "dir", "d", ".", "Directory in which to create the symlink.")
	f.StringVar(&o.name, "name", "", "Override the symlink filename (defaults to page title).")
	f.StringVar(&o.dbPath, "db", "", "Path to notion.db (defaults to the platform-standard Notion install).")
	f.StringVar(&o.cacheMode, "cache", "auto", "Cache location: auto | global | repo | <path>.")
	f.BoolVarP(&o.recursive, "recursive", "r", false, "Also export descendant pages (M2; not yet implemented).")
	f.IntVar(&o.depth, "depth", 0, "Max recursion depth (0 = unlimited, only with --recursive; M2).")
	f.StringVar(&o.layout, "layout", "tree", "Layout when --recursive: flat | tree (M2).")
	f.BoolVarP(&o.force, "force", "f", false, "Overwrite an existing symlink at the target path.")
	f.BoolVar(&o.dryRun, "dry-run", false, "Print the plan without writing any files.")

	return cmd
}

func runLink(cmd *cobra.Command, pageArg string, o *linkOpts) error {
	out := cmd.OutOrStdout()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	pageID, err := resolve.PageID(pageArg)
	if err != nil {
		return fmt.Errorf("resolve page id: %w", err)
	}

	dbPath := o.dbPath
	if dbPath == "" {
		dbPath = db.DefaultPath()
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open notion db: %w", err)
	}
	defer conn.Close()

	page, err := conn.GetPage(ctx, pageID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("page %s not found in notion.db (is it synced for offline use?)", pageID)
		}
		return fmt.Errorf("load page: %w", err)
	}

	cacheDir, mode, err := cache.Resolve(o.cacheMode, o.dir)
	if err != nil {
		return fmt.Errorf("resolve cache dir: %w", err)
	}
	cachePath := filepath.Join(cacheDir, "pages", pageID+".md")

	linkName := o.name
	if linkName == "" {
		linkName = link.Slugify(page.Title) + ".md"
	}
	linkPath := filepath.Join(o.dir, linkName)

	if o.recursive {
		fmt.Fprintln(out, "warning: --recursive is M2; rendering only the requested page for now.")
	}

	fmt.Fprintf(out, "page         : %s\n", pageID)
	fmt.Fprintf(out, "title        : %s\n", page.Title)
	fmt.Fprintf(out, "space        : %s\n", page.SpaceID)
	fmt.Fprintf(out, "cache mode   : %s\n", mode)
	fmt.Fprintf(out, "cache file   : %s\n", cachePath)
	fmt.Fprintf(out, "symlink path : %s\n", linkPath)

	if o.dryRun {
		fmt.Fprintln(out, "\n(dry-run; no files written.)")
		return nil
	}

	root, err := conn.LoadTree(ctx, pageID, 0, false)
	if err != nil {
		return fmt.Errorf("load page tree: %w", err)
	}

	md := render.Page(page, root, render.Options{
		SpaceName:       conn.SpaceName(ctx, page.SpaceID),
		LastEdited:      timeFromMillis(page.LastEditedTimeMillis),
		SourceURL:       notionURL(page),
		LookupPageTitle: func(id string) string { return conn.LookupTitle(ctx, id) },
		ExportedAt:      time.Now(),
		Tool:            "notion-link " + Version,
	})

	if err := cache.WriteAtomic(cachePath, []byte(md)); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}

	resolvedTarget, err := link.Create(cachePath, linkPath, link.CreateOptions{
		Force:          o.force,
		PreferRelative: mode == cache.ModeRepo,
	})
	if err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	fmt.Fprintf(out, "\nwrote        : %s (%d bytes)\n", cachePath, len(md))
	fmt.Fprintf(out, "symlink      : %s -> %s\n", linkPath, resolvedTarget)
	return nil
}

func timeFromMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// notionURL builds a "best-effort" canonical URL for a page. Notion URLs
// carry a slug + the 32-char id; we omit the slug since we don't know
// the workspace subdomain.
func notionURL(p *model.Page) string {
	if p == nil {
		return ""
	}
	return "https://www.notion.so/" + strings.ReplaceAll(p.ID, "-", "")
}
