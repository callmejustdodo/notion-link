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
	"github.com/callmejustdodo/notion-link/internal/export"
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
pointing at the cache file.

Pass --recursive to also export descendant sub-pages. Cross-references
between co-exported pages become relative Markdown links so editors can
follow them; references to pages outside the export set fall back to a
notion:// deep-link.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLink(cmd, args[0], o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.dir, "dir", "d", ".", "Directory in which to create the symlink.")
	f.StringVar(&o.name, "name", "", "Override the symlink filename for the root page.")
	f.StringVar(&o.dbPath, "db", "", "Path to notion.db (defaults to the platform-standard install).")
	f.StringVar(&o.cacheMode, "cache", "auto", "Cache location: auto | global | repo | <path>.")
	f.BoolVarP(&o.recursive, "recursive", "r", false, "Also export descendant sub-pages.")
	f.IntVar(&o.depth, "depth", 0, "Max recursion depth (0 = unlimited, only with --recursive).")
	f.StringVar(&o.layout, "layout", "tree", "Layout when --recursive: flat | tree.")
	f.BoolVarP(&o.force, "force", "f", false, "Overwrite existing symlinks at the target paths.")
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

	root, err := conn.GetPage(ctx, pageID)
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

	plan, err := export.Build(ctx, conn, root, export.Options{
		Recursive: o.recursive,
		MaxDepth:  o.depth,
		Layout:    o.layout,
		LinkDir:   o.dir,
		CacheDir:  cacheDir,
	})
	if err != nil {
		return fmt.Errorf("build export plan: %w", err)
	}

	// Allow the user to override the root page's symlink filename.
	if o.name != "" && len(plan.Pages) > 0 {
		plan.Pages[0].SymlinkPath = filepath.Join(filepath.Dir(plan.Pages[0].SymlinkPath), o.name)
	}

	fmt.Fprintf(out, "root page    : %s (%s)\n", root.Title, pageID)
	fmt.Fprintf(out, "space        : %s\n", root.SpaceID)
	fmt.Fprintf(out, "cache mode   : %s\n", mode)
	fmt.Fprintf(out, "cache dir    : %s\n", cacheDir)
	fmt.Fprintf(out, "exporting    : %d page(s)\n", len(plan.Pages))
	if o.recursive {
		fmt.Fprintf(out, "layout       : %s, depth=%d\n", plan.LayoutOf, o.depth)
	}
	fmt.Fprintln(out)

	if o.dryRun {
		for _, p := range plan.Pages {
			fmt.Fprintf(out, "  %s\n    -> %s\n", p.SymlinkPath, p.CachePath)
		}
		fmt.Fprintln(out, "\n(dry-run; no files written.)")
		return nil
	}

	resolveRef := makeResolver(ctx, conn, plan)
	spaceName := conn.SpaceName(ctx, root.SpaceID)
	now := time.Now()
	tool := "notion-link " + Version

	written := 0
	for _, p := range plan.Pages {
		root, err := conn.LoadTree(ctx, p.Page.ID, 0, false)
		if err != nil {
			return fmt.Errorf("load tree for %s: %w", p.Page.ID, err)
		}
		md := render.Page(p.Page, root, render.Options{
			SpaceName:       spaceName,
			LastEdited:      timeFromMillis(p.Page.LastEditedTimeMillis),
			SourceURL:       notionURL(p.Page),
			ResolvePageRef:  resolveRef,
			LookupPageTitle: func(id string) string { return conn.LookupTitle(ctx, id) },
			ExportedAt:      now,
			Tool:            tool,
		})
		if err := cache.WriteAtomic(p.CachePath, []byte(md)); err != nil {
			return fmt.Errorf("write cache for %s: %w", p.Page.ID, err)
		}
		target, err := link.Create(p.CachePath, p.SymlinkPath, link.CreateOptions{
			Force:          o.force,
			PreferRelative: mode == cache.ModeRepo,
		})
		if err != nil {
			return fmt.Errorf("create symlink for %s: %w", p.Page.ID, err)
		}
		fmt.Fprintf(out, "  %s\n    -> %s\n", p.SymlinkPath, target)
		written++
	}

	fmt.Fprintf(out, "\nwrote %d page(s) into %s\n", written, cacheDir)
	return nil
}

// makeResolver wires the renderer's PageRef hook to the export plan plus
// a DB title lookup for pages outside the plan.
func makeResolver(ctx context.Context, conn *db.Conn, plan *export.Plan) func(string) render.PageRef {
	return func(id string) render.PageRef {
		if pp, ok := plan.ByID[id]; ok {
			return render.PageRef{
				Title:    pp.Page.Title,
				Link:     plan.MarkdownPathFor(id),
				Internal: true,
			}
		}
		return render.PageRef{
			Title: conn.LookupTitle(ctx, id),
		}
	}
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
