# Municode CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 7 built. (clone, diff, compare, history, xref, defs, stale)

## Built
- **Foundation (Priority 0):** `internal/cli/municode_api.go` (resolve chain, TOC/content/breadcrumb fetch, HTML→text, ordinance-history + xref parsers), `municode_store.go` (BFS code walk → generic `resources` store as `document` type, query helpers), `municode_localcmd.go` (local-store open + FTS helpers). Data persisted into the generated framework store (`resources` + `resources_fts`), so framework `search`/`sql` work for free.
- **Absorbed / friendly commands (Priority 1):** `resolve`, `cities`, `toc`, `read` (hand-written, name-based, live). Generated endpoint commands: `states`, `clients`, `products`, `versions`, `codestoc`, `content`.
- **Local FTS search:** framework `search` made local-first by removing the dead upstream `/search` endpoint from the spec (it is paid MuniPro — 401 / 0 hits anonymously). Default `search <query>` now hits the local FTS5 index.
- **Transcendence (Priority 2):** all 7 implemented as hand-written Go.
  - `clone` (flagship): full TOC walk → store + FTS + ordinance-history index; `--export` writes an AI-ready Markdown tree. Resumable (Upsert skips stored), partial-on-timeout, 15-min budget, dogfood-curtailed.
  - `diff`: **reframed (user-approved)** from two-supplement diff to clone-vs-live drift detection. Walks current code, diffs vs local clone → added/removed/reworded.
  - `compare`, `history`, `xref`, `defs`, `stale`: local-store queries (stale re-checks live job versions).
- **Tests:** `municode_parse_test.go` (city parse, HTML clean, ordinance-history incl. nested-paren, xref dedup, library URL). Caught and fixed 2 real parser bugs (nested-paren ords, whitespace).

## Deferred / out of scope (honest)
- Verbatim text of **superseded code versions** (Municode CodeBank): paid feature, 401 anonymous. Disclosed to and accepted by the user. `clone` captures current text + full ordinance lineage; `diff` reframed to drift-vs-live.
- Upstream full-text `/search`: dropped (paid MuniPro, returns nothing for free users). Replaced by local FTS.

## Generator notes
- `jobs` resource name collided with a framework command → renamed `versions`.
- Single-endpoint resources (`versions`, `content`) collapse to `versions <productId>` / `content` (cosmetic).
- regen `--force` preserves whole hand-authored files but drops hand-added `root.go` AddCommand wiring for non-scaffold commands (resolve/cities/toc/read) — re-injected manually after each regen.
