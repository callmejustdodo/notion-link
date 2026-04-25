// Package model defines the data shapes notion-link passes around between
// the db reader and the markdown renderer.
package model

// Page is the slim view of a Notion page-typed block we use for resolving
// titles, parents, and child links. Body content is loaded separately as
// a tree of Block values rooted at this page.
type Page struct {
	ID                   string
	SpaceID              string
	Type                 string // always "page" today, kept for forward compat
	Title                string
	ParentID             string
	ParentTable          string // "block" | "space" | "collection"
	ChildIDs             []string
	LastEditedTimeMillis int64
	MetaUserID           string
}

// Block is the rendering-time view of a single Notion block.
//
// Properties and Format are kept as decoded maps so the renderer can pull
// type-specific fields (`title`, `checked`, `language`, ...) without us
// pre-modeling every Notion block schema.
type Block struct {
	ID         string
	Type       string
	Properties map[string]any
	Format     map[string]any
	Children   []*Block
}
