# Municode CLI — Absorb Manifest

Sources surveyed: `noclocks/municode-scraper` (31★ Py), `TIDYAPP/open-legal-codes` (TS crawler — documents full resolve chain), `RoryStolzenberg/municode-dump` (JS), `hbruce11216/municode-scraper`, `macarah/Municode`, `opengovfoundation/lasvegas-parser` (PHP), `wcurrangroome/municoder` (R — "interface to the Municode API"), `Skatterbrainz/MunicipalMCP` (MCP, 7 tools), `dominickdupuy/Swamphacks` municode-mcp (MCP).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List all states | MunicipalMCP `get_states_info` | (generated endpoint) `states list` | offline cache, `--json`/`--select` |
| 2 | Get state by abbreviation | net-log `/States/abbr` | (generated endpoint) `states get` | direct lookup, no client-side filter |
| 3 | List municipalities in a state | MunicipalMCP `list_municipalities` | `municode-pp-cli cities --state GA` | offline, FTS over names, `--json` |
| 4 | Resolve municipality by name → addressable code | MunicipalMCP `get_municipality_info`, `municoder` | `municode-pp-cli resolve "Atlanta, GA"` | one-shot name→(clientId,productId,jobId)+metadata (address, website, zip) |
| 5 | Get municipality public library URL | MunicipalMCP `get_municipality_url` | (behavior in `municode-pp-cli resolve --url`) | permalink/citation building |
| 6 | List products for a municipality | scrapers `/Products/clientId` | (generated endpoint) `products list` | surfaces `Features`/`ContentType` |
| 7 | Resolve product by name + feature flags | net-log `/Products/name` | (behavior in `municode-pp-cli resolve`) | `IsSearchable`/`CodeBank`/`OrdBank` flags |
| 8 | Get latest codification job/version | scrapers `/Jobs/latest` | (behavior in `municode-pp-cli resolve`) | version/date tracking for diffs |
| 9 | Browse code table-of-contents (root) | MunicipalMCP `get_code_structure` | `municode-pp-cli toc "Atlanta, GA"` | tree view, offline after sync |
| 10 | Browse TOC children of a node | scrapers `/codesToc/children` | (behavior in `municode-pp-cli toc --node`) | recurse the tree, offline |
| 11 | Get node breadcrumb/ancestry | net-log `/codesToc/breadcrumb` | (behavior in `municode-pp-cli read --citation`) | citation/permalink construction |
| 12 | Read a section's content (HTML → clean text) | MunicipalMCP `get_code_section`, all scrapers | `municode-pp-cli read "Atlanta, GA" <nodeId>` | clean plaintext extraction, `--json`, chunked Docs handling |
| 13 | Sync/export an entire code to local store | RoryStolzenberg/municode-dump | `municode-pp-cli sync "Atlanta, GA"` | SQLite mirror, resumable, FTS-indexed |
| 14 | Full-text search | MunicipalMCP `search_municipal_codes`, Swamphacks MCP | `municode-pp-cli search <query>` | **Replaced upstream search with local FTS5.** Upstream `/search` is paid MuniPro (401 / 0 hits for free users — verified), so shipping it would return nothing. `search` defaults to the local FTS index over cloned content — the only working search for free users, and offline + regex-capable. |

Every absorbed row works offline (post-sync), emits `--json`, supports `--select`/`--compact`, returns typed exit codes, and persists to SQLite. The community tools are one-off scrapers or click-through MCPs; none offer an offline local store, real (local-FTS) search, plaintext extraction, version diffing, or cross-city comparison.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Clone-vs-live drift diff | diff | hand-code | Compares the local clone against the current live code to emit added/removed/reworded section citations — no API returns a cross-version delta. (Reframed from two-supplement diff: Municode's historical version archive is paid CodeBank, 401 for free users; drift-vs-live is the feasible, valuable form. User-approved reframe.) | none |
| 2 | Cross-city topic comparison | compare | hand-code | FTS5-matches a topic across `documents` from multiple synced `clients` and aligns the top section per city — a cross-client join no single API call provides | Use to lay ONE topic across MULTIPLE synced cities side by side. Do NOT use for ranked hits in one city (use 'search') or to fetch a known section by id (use 'read'). --state GA widens to every synced client in a state. |
| 3 | Ordinance-history extraction | history | hand-code | Regex-extracts "(Ord. No., §, date)" annotations from `documents.text` in local SQLite into structured rows; nothing upstream exposes this | Use to extract enacting-ordinance citations for a section. --by-ordinance 2019-45 inverts the index to list every section an ordinance enacted. |
| 4 | Intra-code cross-reference graph | xref | hand-code | Regex-extracts "see § X"/"Chapter Y" references and joins to `toc_nodes` to build an inbound/outbound edge graph in local SQLite | none |
| 5 | Defined-term lookup | defs | hand-code | FTS5 scoped to Definitions-family headings over local `documents`, returning the defining clause + citation rather than a hit list | Use to fetch the ONE controlling definition of a term. Do NOT use for general full-text hits across the code (use 'search'); 'defs' is scoped to Definitions sections and returns the defining clause. |
| 6 | Mirror freshness check | stale | hand-code | Re-calls `/Jobs/latest/{productId}` per synced (clientId,productId) and compares against the stored `jobId` to flag out-of-date local mirrors | none |
| 7 | **Full offline AI-referenceable clone** (flagship; user-requested) | clone | hand-code | One command: full TOC walk + all section content → SQLite + FTS, builds the complete ordinance-history lineage index from embedded "(Ord. No. …, date)" annotations, and exports an AI-ready Markdown/text tree so an agent references the whole municipal code locally with zero further API calls. Scope: current authoritative text + ordinance-change lineage; verbatim superseded-version snapshots are Municode paid-CodeBank (401 anonymous) and explicitly out of scope. | Use to pull an entire municipality's code for offline/AI reference in one command. Builds on 'sync' and 'history'. Do NOT expect verbatim past-version text (paid Municode CodeBank); 'clone' captures current text plus full ordinance lineage. |

All 7 are **hand-code** (≈50–150 LoC each + `root.go` wiring). Full audit trail (customer model, 12 candidates, 6 kills) in `2026-06-30-152538-novel-features-brainstorm.md`.

### Local FTS search (Priority 0/1 foundation, not counted as transcendence)
Because upstream `/search` is paid-MuniPro-gated (empty for free users), the CLI ships its own FTS5 index over synced `documents.text` + `toc_nodes.heading` as the real `search` command. This is foundation, not a transcendence row, but it's the headline differentiator vs every existing scraper/MCP.
