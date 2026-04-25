package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/callmejustdodo/notion-link/internal/model"
)

// GetPage loads the metadata for a single page-typed block.
//
// Notion stores rich block state as a CRDT blob in `crdt_data`, but the
// legacy `properties` JSON column is still maintained as a materialized
// view of the title and inline rich text, which is enough for our MVP.
func (c *Conn) GetPage(ctx context.Context, pageID string) (*model.Page, error) {
	const q = `
SELECT id, space_id, type, properties, content, parent_id, parent_table,
       last_edited_time, meta_user_id
FROM block
WHERE id = ? AND alive = 1
LIMIT 1`

	var (
		p              model.Page
		properties     sql.NullString
		content        sql.NullString
		parentID       sql.NullString
		parentTable    sql.NullString
		lastEditedTime sql.NullFloat64
	)
	err := c.sql.QueryRowContext(ctx, q, pageID).Scan(
		&p.ID,
		&p.SpaceID,
		&p.Type,
		&properties,
		&content,
		&parentID,
		&parentTable,
		&lastEditedTime,
		&p.MetaUserID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.ParentID = parentID.String
	p.ParentTable = parentTable.String
	p.LastEditedTimeMillis = int64(lastEditedTime.Float64)

	if properties.Valid && properties.String != "" {
		var props map[string]any
		if err := json.Unmarshal([]byte(properties.String), &props); err == nil {
			p.Title = ExtractPlainTitle(props)
		}
	}
	if content.Valid && content.String != "" {
		var ids []string
		if err := json.Unmarshal([]byte(content.String), &ids); err == nil {
			p.ChildIDs = ids
		}
	}
	return &p, nil
}

// LoadTree loads a block (page or any other type) along with its descendants
// up to maxDepth. Pass 0 for unlimited depth.
//
// Pages are not recursed into when followPages is false — they appear as
// childless blocks the renderer can turn into a sub-page link. This is
// the M1 default; recursive sub-page export lands in M2.
func (c *Conn) LoadTree(ctx context.Context, rootID string, maxDepth int, followPages bool) (*model.Block, error) {
	return c.loadBlockRec(ctx, rootID, 0, maxDepth, followPages)
}

func (c *Conn) loadBlockRec(ctx context.Context, id string, depth, maxDepth int, followPages bool) (*model.Block, error) {
	blk, childIDs, err := c.getBlockRow(ctx, id)
	if err != nil {
		return nil, err
	}
	stop := !followPages && depth > 0 && blk.Type == "page"
	if stop || (maxDepth > 0 && depth >= maxDepth) {
		return blk, nil
	}
	for _, cid := range childIDs {
		child, err := c.loadBlockRec(ctx, cid, depth+1, maxDepth, followPages)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // missing offline child, skip silently
			}
			return nil, fmt.Errorf("load child %s: %w", cid, err)
		}
		blk.Children = append(blk.Children, child)
	}
	return blk, nil
}

func (c *Conn) getBlockRow(ctx context.Context, id string) (*model.Block, []string, error) {
	const q = `
SELECT type, properties, format, content
FROM block
WHERE id = ? AND alive = 1
LIMIT 1`

	var (
		blk        = &model.Block{ID: id}
		typ        string
		properties sql.NullString
		format     sql.NullString
		content    sql.NullString
	)
	err := c.sql.QueryRowContext(ctx, q, id).Scan(&typ, &properties, &format, &content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	blk.Type = typ
	if properties.Valid && properties.String != "" {
		_ = json.Unmarshal([]byte(properties.String), &blk.Properties)
	}
	if format.Valid && format.String != "" {
		_ = json.Unmarshal([]byte(format.String), &blk.Format)
	}
	var childIDs []string
	if content.Valid && content.String != "" {
		_ = json.Unmarshal([]byte(content.String), &childIDs)
	}
	return blk, childIDs, nil
}

// ExtractPlainTitle returns the concatenated plain text from a Notion
// rich-text `title` array (`[[ "text", [...annotations] ], ["more"]]`).
// Annotations are ignored.
func ExtractPlainTitle(props map[string]any) string {
	raw, ok := props["title"]
	if !ok {
		return ""
	}
	segs, ok := raw.([]any)
	if !ok {
		return ""
	}
	var out []byte
	for _, seg := range segs {
		row, ok := seg.([]any)
		if !ok || len(row) == 0 {
			continue
		}
		if s, ok := row[0].(string); ok {
			out = append(out, s...)
		}
	}
	return string(out)
}

// LookupTitle returns the plain title for the given block id, or "" if not
// found. Used by the renderer to resolve page-mention links.
func (c *Conn) LookupTitle(ctx context.Context, id string) string {
	const q = `SELECT properties FROM block WHERE id = ? AND alive = 1 LIMIT 1`
	var props sql.NullString
	if err := c.sql.QueryRowContext(ctx, q, id).Scan(&props); err != nil {
		return ""
	}
	if !props.Valid || props.String == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(props.String), &m); err != nil {
		return ""
	}
	return ExtractPlainTitle(m)
}

// SpaceName returns the workspace name for a given space id, or "" on miss.
func (c *Conn) SpaceName(ctx context.Context, spaceID string) string {
	const q = `SELECT name FROM space WHERE id = ? LIMIT 1`
	var name sql.NullString
	if err := c.sql.QueryRowContext(ctx, q, spaceID).Scan(&name); err != nil {
		return ""
	}
	return name.String
}
