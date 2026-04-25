// Package model defines the data shapes notion-link passes around between
// the db reader and the markdown renderer.
package model

// Page is the slim view of a Notion page-typed block we use for resolving
// titles, parents, and child links. Body content is loaded separately as
// a tree of Block values.
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
// Populated incrementally as render support for each type lands.
type Block struct {
	ID       string
	Type     string
	Title    string   // flattened plain text from properties.title
	Children []*Block // resolved child blocks
}
