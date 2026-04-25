// Package export plans the file/symlink layout for a recursive page export.
//
// The plan walks Notion's inline page hierarchy starting from a root page,
// stops at MaxDepth (or unlimited when MaxDepth == 0 and Recursive is true),
// and assigns each page a cache file path and a user-visible symlink path
// according to the chosen Layout.
package export

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/callmejustdodo/notion-link/internal/db"
	"github.com/callmejustdodo/notion-link/internal/link"
	"github.com/callmejustdodo/notion-link/internal/model"
)

// Layout names accepted by Build.
const (
	LayoutFlat = "flat"
	LayoutTree = "tree"
)

// Options configures Build.
type Options struct {
	Recursive bool
	MaxDepth  int    // 0 = unlimited (only meaningful when Recursive is true)
	Layout    string // LayoutFlat | LayoutTree
	LinkDir   string // user-supplied --dir, absolute
	CacheDir  string // root cache dir; cache files live in <CacheDir>/pages
}

// Plan is the resolved set of pages to export and their target paths.
type Plan struct {
	Pages    []*PagePlan          // BFS order, root first
	ByID     map[string]*PagePlan // quick lookup for cross-link resolution
	LayoutOf string               // echoes the chosen layout
}

// PagePlan describes one page's place in the export.
type PagePlan struct {
	Page        *model.Page
	Parent      *PagePlan
	Children    []*PagePlan
	Depth       int    // root = 0
	CachePath   string // absolute path of the rendered .md file
	SymlinkPath string // absolute path of the user-facing symlink
}

// Build walks the page hierarchy starting at root and assigns paths.
// The conn is read-only and used for SubPages lookups.
func Build(ctx context.Context, conn *db.Conn, root *model.Page, opt Options) (*Plan, error) {
	if opt.Layout == "" {
		opt.Layout = LayoutTree
	}
	if opt.Layout != LayoutFlat && opt.Layout != LayoutTree {
		return nil, fmt.Errorf("unknown layout %q (want flat or tree)", opt.Layout)
	}
	cacheRoot, err := filepath.Abs(filepath.Join(opt.CacheDir, "pages"))
	if err != nil {
		return nil, err
	}
	linkDir, err := filepath.Abs(opt.LinkDir)
	if err != nil {
		return nil, err
	}

	plan := &Plan{
		ByID:     map[string]*PagePlan{},
		LayoutOf: opt.Layout,
	}

	rp := &PagePlan{Page: root, Depth: 0}
	plan.Pages = append(plan.Pages, rp)
	plan.ByID[root.ID] = rp

	if opt.Recursive {
		if err := walkChildren(ctx, conn, plan, rp, opt); err != nil {
			return nil, err
		}
	}

	// Assign paths in a second pass so we know each page's children up-front.
	assignPaths(rp, linkDir, cacheRoot, opt.Layout, map[string]int{})

	return plan, nil
}

func walkChildren(ctx context.Context, conn *db.Conn, plan *Plan, parent *PagePlan, opt Options) error {
	if opt.MaxDepth > 0 && parent.Depth >= opt.MaxDepth {
		return nil
	}
	subs, err := conn.SubPages(ctx, parent.Page.ID)
	if err != nil {
		return fmt.Errorf("subpages of %s: %w", parent.Page.ID, err)
	}
	for _, sp := range subs {
		if _, dup := plan.ByID[sp.ID]; dup {
			continue
		}
		child := &PagePlan{Page: sp, Parent: parent, Depth: parent.Depth + 1}
		parent.Children = append(parent.Children, child)
		plan.Pages = append(plan.Pages, child)
		plan.ByID[sp.ID] = child
		if err := walkChildren(ctx, conn, plan, child, opt); err != nil {
			return err
		}
	}
	return nil
}

// assignPaths walks the plan tree depth-first and writes CachePath +
// SymlinkPath onto every node.
//
// Layout rules:
//
//	flat: every page lives at <linkDir>/<slug>.md (collisions get -<short-id>).
//	tree: a page with co-exported children lives at <base>/<slug>/index.md
//	      and its children take <base>/<slug>/ as their base; a leaf lives at
//	      <base>/<slug>.md.
//
// The seen map dedupes slug collisions across the whole plan.
func assignPaths(node *PagePlan, base, cacheRoot, layout string, seen map[string]int) {
	node.CachePath = filepath.Join(cacheRoot, node.Page.ID+".md")

	slug := uniqueSlug(node, seen)
	switch layout {
	case LayoutFlat:
		node.SymlinkPath = filepath.Join(base, slug+".md")
		for _, child := range node.Children {
			assignPaths(child, base, cacheRoot, layout, seen)
		}
	default: // tree
		if len(node.Children) == 0 {
			node.SymlinkPath = filepath.Join(base, slug+".md")
		} else {
			subBase := filepath.Join(base, slug)
			node.SymlinkPath = filepath.Join(subBase, "index.md")
			for _, child := range node.Children {
				assignPaths(child, subBase, cacheRoot, layout, seen)
			}
		}
	}
}

// uniqueSlug returns the page's slug, with a short-id suffix appended if
// the same slug was already used at this layout level.
func uniqueSlug(node *PagePlan, seen map[string]int) string {
	base := link.Slugify(node.Page.Title)
	key := strings.ToLower(base)
	if seen[key] == 0 {
		seen[key] = 1
		return base
	}
	seen[key]++
	short := strings.ReplaceAll(node.Page.ID, "-", "")
	if len(short) > 6 {
		short = short[len(short)-6:]
	}
	return base + " (" + short + ")"
}

// MarkdownPathFor returns a path to the cache .md file for id, expressed as
// a path relative to the cache directory of the *currently rendering* page.
// Since every cache file is a sibling of the others under <cache>/pages/,
// this is just "<id>.md" — but we expose it as a function so the plan owns
// the convention.
func (p *Plan) MarkdownPathFor(id string) string {
	if _, ok := p.ByID[id]; !ok {
		return ""
	}
	return id + ".md"
}
