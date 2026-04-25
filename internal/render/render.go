// Package render converts a tree of Notion blocks into Markdown.
//
// The renderer drives off `block.Type` and consumes `properties` /
// `format` as decoded JSON maps. It is intentionally tolerant: unknown
// or empty blocks fall through to a placeholder comment so the output
// always reflects what was on the page even if a type is unsupported.
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/callmejustdodo/notion-link/internal/model"
)

// PageRef carries the resolution of a page-id mention.
//
// When Internal is true, Link is a Markdown-relative path (relative to
// the *cache* file currently being rendered) that an editor can follow.
// When Internal is false, Link is a notion:// deep-link to the Notion app.
type PageRef struct {
	Title    string
	Link     string
	Internal bool
}

// Options controls renderer behavior.
type Options struct {
	// SpaceName is the workspace display name; emitted in frontmatter.
	SpaceName string
	// LastEdited is the page's last_edited_time. Zero is rendered as "".
	LastEdited time.Time
	// SourceURL is the canonical Notion URL for this page (optional).
	SourceURL string
	// LookupPageTitle resolves a page-id mention to a title only.
	// Used as a fallback when ResolvePageRef is nil. Return "" if unknown.
	LookupPageTitle func(id string) string
	// ResolvePageRef resolves a page-id mention to a (title, link, internal)
	// triple. Set this when exporting multiple pages so cross-page links can
	// be relative paths between co-exported files. When nil, the renderer
	// falls back to LookupPageTitle and emits a notion:// deep-link.
	ResolvePageRef func(id string) PageRef
	// ExportedAt stamps the frontmatter; defaults to time.Now() if zero.
	ExportedAt time.Time
	// Tool name shown in frontmatter (e.g. "notion-link 0.1.0").
	Tool string
}

// Page renders a full page (frontmatter + heading + body) to Markdown.
func Page(page *model.Page, root *model.Block, opt Options) string {
	var b strings.Builder
	writeFrontmatter(&b, page, opt)
	b.WriteString("# ")
	b.WriteString(escapeHeading(page.Title))
	b.WriteString("\n\n")

	r := &renderer{opt: opt}
	for _, child := range root.Children {
		r.block(&b, child, "")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeFrontmatter(b *strings.Builder, p *model.Page, opt Options) {
	b.WriteString("---\n")
	fmt.Fprintf(b, "notion_id: %s\n", p.ID)
	if p.Title != "" {
		fmt.Fprintf(b, "title: %s\n", yamlString(p.Title))
	}
	if opt.SpaceName != "" {
		fmt.Fprintf(b, "space: %s\n", yamlString(opt.SpaceName))
	}
	if !opt.LastEdited.IsZero() {
		fmt.Fprintf(b, "last_edited: %s\n", opt.LastEdited.Format(time.RFC3339))
	}
	if opt.SourceURL != "" {
		fmt.Fprintf(b, "url: %s\n", opt.SourceURL)
	}
	if opt.Tool != "" {
		fmt.Fprintf(b, "exported_by: %s\n", opt.Tool)
	}
	exportedAt := opt.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now()
	}
	fmt.Fprintf(b, "exported_at: %s\n", exportedAt.Format(time.RFC3339))
	b.WriteString("---\n\n")
}

type renderer struct {
	opt        Options
	numbering  []int // counter stack per nested numbered_list
}

func (r *renderer) block(b *strings.Builder, blk *model.Block, indent string) {
	switch blk.Type {
	case "text":
		r.paragraph(b, blk, indent)
	case "header":
		r.heading(b, blk, "##")
	case "sub_header":
		r.heading(b, blk, "###")
	case "sub_sub_header":
		r.heading(b, blk, "####")
	case "bulleted_list":
		r.listItem(b, blk, indent, "- ")
	case "numbered_list":
		n := r.nextNumber(indent)
		r.listItem(b, blk, indent, fmt.Sprintf("%d. ", n))
	case "to_do":
		mark := "- [ ] "
		if r.checked(blk) {
			mark = "- [x] "
		}
		r.listItem(b, blk, indent, mark)
	case "quote":
		r.quote(b, blk, indent)
	case "code":
		r.code(b, blk, indent)
	case "divider":
		b.WriteString(indent)
		b.WriteString("---\n\n")
	case "callout":
		r.callout(b, blk, indent)
	case "toggle":
		r.toggle(b, blk, indent)
	case "equation":
		r.equation(b, blk, indent)
	case "page":
		r.subPageLink(b, blk, indent)
	case "column_list", "column":
		// Flatten — columns don't translate to Markdown.
		for _, child := range blk.Children {
			r.block(b, child, indent)
		}
	case "":
		return
	default:
		b.WriteString(indent)
		fmt.Fprintf(b, "<!-- TODO: unsupported block type %q -->\n\n", blk.Type)
	}

	// Reset the numbered-list counter when we leave a numbered run.
	if blk.Type != "numbered_list" {
		r.resetNumberingAt(indent)
	}
}

func (r *renderer) paragraph(b *strings.Builder, blk *model.Block, indent string) {
	text := r.richText(blk)
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	r.renderChildren(b, blk, indent+"  ")
}

func (r *renderer) heading(b *strings.Builder, blk *model.Block, prefix string) {
	text := r.richText(blk)
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(b, "%s %s\n", prefix, line)
	}
	b.WriteString("\n")
	r.renderChildren(b, blk, "")
}

func (r *renderer) listItem(b *strings.Builder, blk *model.Block, indent, marker string) {
	text := r.richText(blk)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	b.WriteString(indent)
	b.WriteString(marker)
	b.WriteString(lines[0])
	b.WriteString("\n")
	cont := indent + strings.Repeat(" ", len(marker))
	for _, line := range lines[1:] {
		b.WriteString(cont)
		b.WriteString(line)
		b.WriteString("\n")
	}
	r.renderChildren(b, blk, indent+"  ")
}

func (r *renderer) quote(b *strings.Builder, blk *model.Block, indent string) {
	text := r.richText(blk)
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(indent)
		b.WriteString("> ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	r.renderChildren(b, blk, indent)
}

func (r *renderer) code(b *strings.Builder, blk *model.Block, indent string) {
	text := r.richText(blk)
	lang := r.codeLang(blk)
	b.WriteString(indent)
	b.WriteString("```")
	if lang != "" {
		b.WriteString(lang)
	}
	b.WriteString("\n")
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(indent)
	b.WriteString("```\n\n")
}

func (r *renderer) callout(b *strings.Builder, blk *model.Block, indent string) {
	icon := r.calloutIcon(blk)
	text := r.richText(blk)
	if text == "" && len(blk.Children) == 0 {
		return
	}
	prefix := "> "
	if icon != "" {
		prefix = "> " + icon + " "
	}
	first := true
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(indent)
		if first {
			b.WriteString(prefix)
			first = false
		} else {
			b.WriteString("> ")
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	r.renderChildren(b, blk, indent)
}

func (r *renderer) toggle(b *strings.Builder, blk *model.Block, indent string) {
	summary := r.richText(blk)
	b.WriteString(indent)
	b.WriteString("<details>\n")
	if summary != "" {
		b.WriteString(indent)
		fmt.Fprintf(b, "<summary>%s</summary>\n\n", summary)
	}
	r.renderChildren(b, blk, indent)
	b.WriteString(indent)
	b.WriteString("</details>\n\n")
}

func (r *renderer) equation(b *strings.Builder, blk *model.Block, indent string) {
	text := r.richText(blk)
	if text == "" {
		return
	}
	b.WriteString(indent)
	b.WriteString("$$\n")
	b.WriteString(indent)
	b.WriteString(text)
	b.WriteString("\n")
	b.WriteString(indent)
	b.WriteString("$$\n\n")
}

func (r *renderer) subPageLink(b *strings.Builder, blk *model.Block, indent string) {
	title := plainTitle(blk)
	if title == "" {
		title = "(untitled page)"
	}
	ref := r.resolvePageRef(blk.ID)
	if ref.Title != "" {
		title = ref.Title
	}
	link := ref.Link
	if link == "" {
		link = "notion://www.notion.so/" + strings.ReplaceAll(blk.ID, "-", "")
	}
	b.WriteString(indent)
	fmt.Fprintf(b, "- [%s](%s)\n", title, link)
}

// formatPageMention turns a page-id mention into a Markdown link, preferring
// an internal cross-link when the page is co-exported.
func (r *renderer) formatPageMention(id string) string {
	ref := r.resolvePageRef(id)
	title := ref.Title
	if title == "" {
		title = "(page)"
	}
	link := ref.Link
	if link == "" {
		link = "notion://www.notion.so/" + strings.ReplaceAll(id, "-", "")
	}
	return fmt.Sprintf("[%s](%s)", title, link)
}

// resolvePageRef centralizes the precedence between ResolvePageRef and the
// title-only fallback.
func (r *renderer) resolvePageRef(id string) PageRef {
	if r.opt.ResolvePageRef != nil {
		ref := r.opt.ResolvePageRef(id)
		if ref.Title == "" && r.opt.LookupPageTitle != nil {
			ref.Title = r.opt.LookupPageTitle(id)
		}
		return ref
	}
	if r.opt.LookupPageTitle != nil {
		return PageRef{Title: r.opt.LookupPageTitle(id)}
	}
	return PageRef{}
}

func (r *renderer) renderChildren(b *strings.Builder, blk *model.Block, indent string) {
	for _, child := range blk.Children {
		r.block(b, child, indent)
	}
}

// nextNumber/resetNumberingAt give numbered_list a sibling counter that
// resets when the run is broken by a non-numbered block at the same level.
func (r *renderer) nextNumber(indent string) int {
	depth := len(indent) / 2
	for len(r.numbering) <= depth {
		r.numbering = append(r.numbering, 0)
	}
	r.numbering[depth]++
	// Trim deeper counters so re-entering a deeper list restarts at 1.
	for len(r.numbering) > depth+1 {
		r.numbering = r.numbering[:len(r.numbering)-1]
	}
	return r.numbering[depth]
}

func (r *renderer) resetNumberingAt(indent string) {
	depth := len(indent) / 2
	if depth < len(r.numbering) {
		r.numbering = r.numbering[:depth]
	}
}

// ---- properties helpers -----------------------------------------------------

func (r *renderer) richText(blk *model.Block) string {
	if blk == nil || blk.Properties == nil {
		return ""
	}
	raw, ok := blk.Properties["title"]
	if !ok {
		return ""
	}
	return r.renderRichText(raw)
}

// renderRichText handles the [[text, [annotations]], ...] shape Notion uses.
func (r *renderer) renderRichText(raw any) string {
	segs, ok := raw.([]any)
	if !ok {
		return ""
	}
	var out strings.Builder
	for _, seg := range segs {
		row, ok := seg.([]any)
		if !ok || len(row) == 0 {
			continue
		}
		text, _ := row[0].(string)
		var anns [][]any
		if len(row) > 1 {
			if a, ok := row[1].([]any); ok {
				for _, x := range a {
					if ann, ok := x.([]any); ok {
						anns = append(anns, ann)
					}
				}
			}
		}
		out.WriteString(r.applyAnnotations(text, anns))
	}
	return out.String()
}

func (r *renderer) applyAnnotations(text string, anns [][]any) string {
	if len(anns) == 0 {
		return text
	}
	// Page mentions: text == "‣" with a `["p", id, space]` annotation.
	for _, a := range anns {
		if len(a) >= 2 {
			if k, _ := a[0].(string); k == "p" {
				if id, _ := a[1].(string); id != "" {
					return r.formatPageMention(id)
				}
			}
		}
	}

	// External link: ["a", "https://..."]. Apply last so it wraps any inline marks.
	var linkURL string
	for _, a := range anns {
		if len(a) >= 2 {
			if k, _ := a[0].(string); k == "a" {
				if u, _ := a[1].(string); u != "" {
					linkURL = u
				}
			}
		}
	}

	out := text
	for _, a := range anns {
		if len(a) == 0 {
			continue
		}
		k, _ := a[0].(string)
		switch k {
		case "b":
			out = "**" + out + "**"
		case "i":
			out = "*" + out + "*"
		case "c":
			out = "`" + out + "`"
		case "s":
			out = "~~" + out + "~~"
		case "_":
			out = "<u>" + out + "</u>"
		}
	}
	if linkURL != "" {
		out = fmt.Sprintf("[%s](%s)", out, linkURL)
	}
	return out
}

func (r *renderer) checked(blk *model.Block) bool {
	if blk.Properties == nil {
		return false
	}
	raw, ok := blk.Properties["checked"]
	if !ok {
		return false
	}
	segs, ok := raw.([]any)
	if !ok || len(segs) == 0 {
		return false
	}
	row, ok := segs[0].([]any)
	if !ok || len(row) == 0 {
		return false
	}
	v, _ := row[0].(string)
	return strings.EqualFold(v, "Yes")
}

func (r *renderer) codeLang(blk *model.Block) string {
	if blk.Properties == nil {
		return ""
	}
	raw, ok := blk.Properties["language"]
	if !ok {
		return ""
	}
	segs, ok := raw.([]any)
	if !ok || len(segs) == 0 {
		return ""
	}
	row, ok := segs[0].([]any)
	if !ok || len(row) == 0 {
		return ""
	}
	v, _ := row[0].(string)
	return normalizeLang(v)
}

func (r *renderer) calloutIcon(blk *model.Block) string {
	if blk.Format == nil {
		return ""
	}
	icon, ok := blk.Format["page_icon"].(string)
	if !ok {
		return ""
	}
	return icon
}

func plainTitle(blk *model.Block) string {
	if blk == nil || blk.Properties == nil {
		return ""
	}
	raw, ok := blk.Properties["title"]
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

func escapeHeading(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}

// yamlString quotes a YAML scalar when it contains characters that would
// otherwise break parsing. Plain scalars are emitted bare for readability.
func yamlString(s string) string {
	if s == "" {
		return `""`
	}
	needs := strings.ContainsAny(s, ":#&*!|>'\"%@`?[]{},\n") ||
		strings.HasPrefix(s, "- ") ||
		strings.HasPrefix(s, " ") ||
		strings.HasSuffix(s, " ")
	if !needs {
		return s
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return `"` + escaped + `"`
}

// normalizeLang maps Notion's display-name code languages onto the short
// identifiers Markdown highlighters expect. Unknown values pass through.
func normalizeLang(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "plain text":
		return ""
	case "javascript":
		return "js"
	case "typescript":
		return "ts"
	case "python":
		return "python"
	case "go":
		return "go"
	case "rust":
		return "rust"
	case "shell", "bash":
		return "bash"
	case "html":
		return "html"
	case "css":
		return "css"
	case "json":
		return "json"
	case "yaml":
		return "yaml"
	case "sql":
		return "sql"
	case "markdown":
		return "markdown"
	default:
		return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}
}
