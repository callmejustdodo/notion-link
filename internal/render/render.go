// Package render converts a tree of Notion blocks into Markdown.
//
// PR #1 ships a stub so the cli/db packages can compile against the
// final shape; the per-block-type rendering lands in PR #2 (M1).
package render

import (
	"errors"

	"github.com/callmejustdodo/notion-link/internal/model"
)

// ErrUnimplemented is returned by RenderPage until PR #2 lands.
var ErrUnimplemented = errors.New("render: not implemented yet (PR #2)")

// RenderPage produces the Markdown body (frontmatter + content) for a page.
func RenderPage(page *model.Page, blocks []*model.Block) (string, error) {
	_ = page
	_ = blocks
	return "", ErrUnimplemented
}
