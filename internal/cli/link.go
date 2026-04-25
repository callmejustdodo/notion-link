package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/callmejustdodo/notion-link/internal/cache"
	"github.com/callmejustdodo/notion-link/internal/db"
	"github.com/callmejustdodo/notion-link/internal/link"
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

PR #1 scaffolding: this currently runs in --dry-run mode only and
prints the resolved plan without touching the filesystem.`,
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
	f.BoolVarP(&o.recursive, "recursive", "r", false, "Also export descendant pages.")
	f.IntVar(&o.depth, "depth", 0, "Max recursion depth (0 = unlimited, only meaningful with --recursive).")
	f.StringVar(&o.layout, "layout", "tree", "Layout when --recursive: flat | tree.")
	f.BoolVarP(&o.force, "force", "f", false, "Overwrite an existing symlink at the target path.")
	f.BoolVar(&o.dryRun, "dry-run", true, "Print the plan without writing any files. (default true in scaffolding)")

	return cmd
}

func runLink(cmd *cobra.Command, pageArg string, o *linkOpts) error {
	out := cmd.OutOrStdout()

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

	page, err := conn.GetPage(cmd.Context(), pageID)
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

	fmt.Fprintf(out, "page         : %s\n", pageID)
	fmt.Fprintf(out, "title        : %s\n", page.Title)
	fmt.Fprintf(out, "space        : %s\n", page.SpaceID)
	fmt.Fprintf(out, "cache mode   : %s\n", mode)
	fmt.Fprintf(out, "cache file   : %s\n", cachePath)
	fmt.Fprintf(out, "symlink path : %s\n", linkPath)
	fmt.Fprintf(out, "recursive    : %t (depth=%d, layout=%s)\n", o.recursive, o.depth, o.layout)

	if o.dryRun {
		fmt.Fprintln(out, "\n(dry-run; no files written. PR #1 scaffolding only.)")
		_ = render.RenderPage // keep render import wired for the next PR
		return nil
	}

	return errors.New("non dry-run is not implemented yet (see PR #2)")
}
