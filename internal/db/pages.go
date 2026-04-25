package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/callmejustdodo/notion-link/internal/model"
)

// SubPages returns the immediate page-typed block children of parentID,
// in insertion order. Only alive (non-deleted) pages are returned.
//
// Notion also exposes pages via collection_view (databases). Those are
// not followed here; recursive export covers the inline page hierarchy.
func (c *Conn) SubPages(ctx context.Context, parentID string) ([]*model.Page, error) {
	const q = `
SELECT id, space_id, properties, last_edited_time, parent_id, parent_table, meta_user_id
FROM block
WHERE parent_id = ? AND type = 'page' AND alive = 1
ORDER BY rowid`
	rows, err := c.sql.QueryContext(ctx, q, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Page
	for rows.Next() {
		var (
			p          model.Page
			properties sql.NullString
			lastEdited sql.NullFloat64
			parentID   sql.NullString
			parentTbl  sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.SpaceID, &properties, &lastEdited, &parentID, &parentTbl, &p.MetaUserID); err != nil {
			return nil, err
		}
		p.Type = "page"
		p.ParentID = parentID.String
		p.ParentTable = parentTbl.String
		p.LastEditedTimeMillis = int64(lastEdited.Float64)
		if properties.Valid && properties.String != "" {
			var props map[string]any
			if err := json.Unmarshal([]byte(properties.String), &props); err == nil {
				p.Title = ExtractPlainTitle(props)
			}
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// SearchPages returns pages whose title contains the given substring,
// case-insensitive. Limit caps the returned rows; pass 0 for no limit.
func (c *Conn) SearchPages(ctx context.Context, query string, spaceID string, limit int) ([]*model.Page, error) {
	const base = `
SELECT id, space_id, properties, last_edited_time
FROM block
WHERE type = 'page' AND alive = 1`
	args := []any{}
	q := base
	if query != "" {
		q += " AND lower(properties) LIKE ?"
		args = append(args, "%"+toLower(query)+"%")
	}
	if spaceID != "" {
		q += " AND space_id = ?"
		args = append(args, spaceID)
	}
	q += " ORDER BY last_edited_time DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := c.sql.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Page
	for rows.Next() {
		var (
			p          model.Page
			properties sql.NullString
			lastEdited sql.NullFloat64
		)
		if err := rows.Scan(&p.ID, &p.SpaceID, &properties, &lastEdited); err != nil {
			return nil, err
		}
		p.Type = "page"
		p.LastEditedTimeMillis = int64(lastEdited.Float64)
		if properties.Valid && properties.String != "" {
			var m map[string]any
			if err := json.Unmarshal([]byte(properties.String), &m); err == nil {
				p.Title = ExtractPlainTitle(m)
			}
		}
		// Defense: LIKE on raw JSON can match keys/punctuation; keep results
		// only if the title actually contains the query.
		if query == "" || containsFold(p.Title, query) {
			out = append(out, &p)
		}
	}
	return out, rows.Err()
}

// Spaces returns every space row known to the offline cache.
func (c *Conn) Spaces(ctx context.Context) ([]*model.Space, error) {
	const q = `SELECT id, name, plan_type, subscription_tier FROM space ORDER BY name`
	rows, err := c.sql.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Space
	for rows.Next() {
		var (
			s        model.Space
			name     sql.NullString
			planType sql.NullString
			subTier  sql.NullString
		)
		if err := rows.Scan(&s.ID, &name, &planType, &subTier); err != nil {
			return nil, err
		}
		s.Name = name.String
		s.PlanType = planType.String
		s.SubscriptionTier = subTier.String
		out = append(out, &s)
	}
	return out, rows.Err()
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsFold(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return indexFold(haystack, needle) >= 0
}

func indexFold(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	ls, lsub := len(s), len(sub)
	if lsub > ls {
		return -1
	}
	for i := 0; i <= ls-lsub; i++ {
		match := true
		for j := 0; j < lsub; j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
