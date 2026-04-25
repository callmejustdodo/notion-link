# notion-link

A CLI that turns Notion pages from your **local offline cache**
(`notion.db`) into Markdown files in any directory of your choice — via
symlinks that stay fresh as you re-sync.

```
$ notion-link link "https://www.notion.so/2026-Resolutions-3082..." --dir ~/notes
~/notes/2026 Resolutions  REBOOT.md → ~/Library/Caches/notion-link/pages/3082....md

$ cat "~/notes/2026 Resolutions  REBOOT.md"   # transparently follows the symlink
---
notion_id: 3082cfba-780a-802e-8158-d0390b98bfaa
title: "2026 Resolutions : REBOOT"
space: DODO-SPACE
last_edited: 2026-02-18T18:24:01+09:00
url: https://www.notion.so/3082cfba780a802e8158d0390b98bfaa
exported_by: notion-link 0.1.0
exported_at: 2026-04-25T15:47:31+09:00
---

# 2026 Resolutions : REBOOT
...
```

## How it works

`notion-link` reads the Notion desktop app's local SQLite database
(`~/Library/Application Support/Notion/notion.db`) **read-only**, walks
the block tree of the page you ask for, renders it to Markdown into a
content-addressed cache, and creates a symlink at the path you choose.

Two cache modes:

- **global** (`~/Library/Caches/notion-link/`) — one cache per user; the
  symlinks in `--dir` use absolute targets.
- **repo** (`<git-root>/.notion/`) — chosen automatically when `--dir`
  is inside a git work tree. Symlinks use a **relative** target so both
  the cache and the symlinks can be committed and clone cleanly.

Pass `--cache global|repo|<path>|auto` to override; `auto` is the default.

## Commands

```text
notion-link link <page>     # render + symlink (the main verb)
notion-link sync            # re-render every cached page from notion.db
notion-link unlink <path>   # remove a symlink (--purge also removes the cache)
notion-link search <query>  # find pages by title (case-insensitive substring)
notion-link list            # most-recently-edited pages
notion-link spaces          # workspaces cached in notion.db
notion-link version
```

`<page>` accepts a Notion URL (`https://www.notion.so/...`), a dashed
UUID, or the 32-char compact id from a Notion URL.

### Recursive sub-page export

```
notion-link link <root> --dir ./notes -r [--depth N] [--layout flat|tree]
```

Walks the inline page hierarchy. Rich-text page mentions and sub-page
blocks are rewritten as **relative Markdown links** when the target is
co-exported, otherwise they fall back to a `notion://` deep-link.

Two layouts:

- `tree` (default) — a page with co-exported children lives at
  `<base>/<slug>/index.md` and its children are siblings of `index.md`.
- `flat` — every page is a sibling under `--dir`; collisions get a
  short-id suffix.

## Supported block types

text · header / sub_header / sub_sub_header · bulleted / numbered list
· to_do (with `[ ]` / `[x]`) · quote · code (with language) · divider
· callout (with icon) · toggle (as `<details>`) · equation · sub-page
link · column / column_list (flattened).

Inline annotations: bold, italic, inline code, strikethrough,
underline, external link, page mention.

Unsupported block types render as `<!-- TODO: unsupported block type "X" -->`
so the rest of the page is preserved.

## Status

Personal-use, macOS-first, **pre-alpha**. Things still on the roadmap:

- Tables (`table` / `table_row`) and column-header detection.
- Image inlining from `~/Library/Application Support/Notion/blob_storage/`.
- `gc` for orphaned cache files.
- `watch` mode (re-render on `notion.db` change).
- Linux/Windows portability (Linux probably works; Windows symlinks
  require Developer Mode).
- Notion API mode for pages **not** in the offline cache.

## Build

Requires Go 1.22+.

```bash
go build ./cmd/notion-link
./notion-link link 2bd2cfba780a806cb320e65c4f924ae7 --dir .
```

## License

MIT
