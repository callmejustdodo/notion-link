# notion-link

A CLI that turns Notion pages from your **local offline cache** (`notion.db`)
into Markdown files in any directory of your choice — via symlinks that stay
fresh as you re-sync.

```
$ notion-link link "https://www.notion.so/2026-Resolutions-3082...REBOOT" --dir ~/Documents/notes
~/Documents/notes/2026 Resolutions  REBOOT.md → ~/Library/Caches/notion-link/pages/3082....md
```

## Status

**Pre-alpha — PR #1 scaffolding.** `link` resolves the page id, opens
`notion.db` read-only, and prints the plan. No files are written yet.
Track progress in [`PLAN.md`](PLAN.md) (TBD).

## Requirements

- macOS (Linux/Windows TBD)
- Notion desktop app installed and at least one page marked for offline use
- Go 1.22+ to build

## Build

```bash
go build ./cmd/notion-link
./notion-link link 2bd2cfba780a806cb320e65c4f924ae7 --dir .
```

## CLI

```text
notion-link link <page> [--dir .] [--name X] [--cache auto|global|repo|<path>]
                        [--recursive] [--depth N] [--layout flat|tree]
                        [--force] [--dry-run]
```

`<page>` is a Notion URL, a dashed UUID, or a 32-char compact id.

## License

MIT
