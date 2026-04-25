package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

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
			p.Title = extractTitle(props)
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

// extractTitle pulls the title text out of a Notion properties JSON map.
// `properties.title` is a `[[ "text", [["b"], ...] ]]` rich-text array;
// for MVP we just concatenate the leading string of each segment.
func extractTitle(props map[string]any) string {
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
