# Municode CLI Brief

## API Identity
- **Domain:** Municipal legal codes & ordinances. `library.municode.com` (MunicodeNEXT, a CivicPlus product) hosts 3,300+ municipal/county codes of ordinances, charters, and related laws for US local governments.
- **Users:** Attorneys & paralegals (land-use, zoning, code enforcement), city clerks & municipal staff, planners, journalists, civic-tech/open-data developers, researchers studying local regulation, compliance teams.
- **Data profile:** Hierarchical legal documents. State → Municipality (client) → Product (e.g. "Code of Ordinances") → Job (a dated codification version) → TOC tree of nodes → leaf content (HTML chunks: titles, chapters, articles, sections). Deeply nested, large per-code (a city code is tens of thousands of nodes).

## Reachability Risk
- **None.** `cli-printing-press probe-reachability https://api.municode.com/States` → `mode: standard_http`, confidence 0.95. Both stdlib and Surf-Chrome probes returned HTTP 200 `application/json`. No bot protection, no clearance cookie, no auth. Ship plain stdlib HTTP transport.
- No tier/permission gating observed (public records).

## Verified API Contract (reverse-engineered; no official OpenAPI exists)
Base: `https://api.municode.com` — ASP.NET Core, JSON, **no authentication**. (Backend of the AngularJS `mcc.library_desktop` app.)

| Method | Path | Params | Returns |
|---|---|---|---|
| GET | `/States` | — | `[{StateID, StateName, StateAbbreviation}]` |
| GET | `/Clients/stateAbbr` | `?stateAbbr=GA` | `[{ClientID, ClientName, State{...}, Address, City, ZipCode, Website, ...}]` (municipalities) |
| GET | `/Clients/name` | `?name=&stateAbbr=` | client lookup by name |
| GET | `/Products/clientId/{clientId}` | path | `[{ProductID, ProductName}]` (e.g. "Code of Ordinances") |
| GET | `/Jobs/latest/{productId}` | path | `{Id, ...}` — latest codification job; `Id` is the `jobId` |
| GET | `/codesToc/children` | `?productId=&jobId=&nodeId=` | `[{Id, Heading}]` — TOC children (root: `nodeId=productId`) |
| GET | `/CodesContent` | `?productId=&jobId=&nodeId=` | `{Docs:[{Id, TitleHtml, Content}]}` — section HTML |
| GET | `/search` | `?productId=&jobId=&query=&pageNum=&pageSize=` | `{NumberOfHits, Hits[], ProductFacets, ContentTypeFacets, ...}` |

**Resolve chain** (slug → addressable code): `stateAbbr` → `/Clients/stateAbbr` (pick ClientID) → `/Products/clientId` (pick "Code of Ordinances" ProductID) → `/Jobs/latest/{productId}` (jobId) → TOC/content/search all keyed by `(productId, jobId, nodeId)`.

Public library URL pattern: `https://library.municode.com/{stateabbr}/{client-slug}/codes/code_of_ordinances`.

### RESOLVED (browser-sniff): upstream search is a PAID feature (MuniPro)
Net-log capture of the live site proved why direct `/search` returns 0 hits: **full-text search is the paid "MuniPro" tier, not available to free/anonymous users.**
- `localapi/IpBasedAuthentication/HasMuniproSearch` → `hasMuniproSearch:false`
- frontend `library.municode.com/api/search` → **HTTP 401**
- public `api.municode.com/search` → 200 but `NumberOfHits:0` for every query (free tier has no index access)

**Decision:** Local FTS5 over synced content is the **only viable search** and becomes the headline differentiator (offline, regex, no MuniPro). A thin upstream `/search` passthrough may exist but must honestly report MuniPro gating. See `discovery/browser-sniff-report.md`.

### Bonus endpoints surfaced by capture (verified on public api.municode.com)
- `GET /States/abbr?stateAbbr=GA` → single state object
- `GET /Clients/name?clientName=atlanta&stateAbbr=ga` → resolve a city directly by name (no list+filter)
- `GET /Products/name?clientId=&productName=code of ordinances` → product with `ContentType{IsSearchable}` + `Features{CodeBank,OrdBank,CodeBankCompare,...}`
- `GET /codesToc/breadcrumb?jobId=&nodeId=&productId=` → node ancestry path (citation/permalink building)

## Top Workflows
1. **Find a municipality's code** — "open Atlanta, GA's code of ordinances" (state → city → resolve to addressable code).
2. **Browse the code tree** — walk TOC chapters/articles/sections (zoning, building, business licensing…).
3. **Read a specific section** — pull the HTML/plaintext of a chapter or section by node id or heading.
4. **Search within a code** — "where does Atlanta regulate short-term rentals / fences / noise?"
5. **Sync a whole code locally** — mirror a city's code into SQLite for offline reading, grep/FTS, and diffing across codification versions.

## Table Stakes (from existing tools)
- Resolve city → code (every scraper does this): `noclocks/municode-scraper`, `TIDYAPP/open-legal-codes` (TS), `RoryStolzenberg/municode-dump`, `wcurrangroome/municoder` (R), the two MCP servers.
- Fetch TOC tree + recurse children.
- Fetch section content (HTML → text).
- Dump/export an entire code.
- List states / list municipalities in a state.
- Search (the MCP servers expose it, though param shape is stale).

## Data Layer
- **Primary entities:** `states`, `clients` (municipalities), `products`, `jobs` (codification versions), `toc_nodes` (tree: id, parent, heading, depth), `documents` (leaf content: node_id, title, html, text).
- **Sync cursor:** per (clientId, productId) the latest `jobId` (codification version/date). Re-sync detects a new job → new version.
- **FTS/search:** FTS5 over `documents.text` + `toc_nodes.heading` — offline, regex-capable, the headline differentiator.

## Codebase Intelligence
- Source: GitHub scraper/MCP corpus. `TIDYAPP/open-legal-codes/src/crawlers/municode.ts` documents the exact resolve chain; `Skatterbrainz/MunicipalMCP/municode-mcp-server.py` mirrors the API as 7 MCP tools; `RoryStolzenberg/municode-dump` documents `/CodesContent` + `/codesToc`.
- Auth: **none** — public `api.municode.com`, no token, no cookie.
- Data model: state→client→product→job→toc→content (verified live).
- Rate limiting: none observed; community scrapers use polite ~500ms delays. Be courteous (adaptive limiter).
- Architecture: AngularJS SPA over an ASP.NET Core JSON API; URLs are id-keyed, not slug-keyed (slug only in the human library URL).

## Product Thesis
- **Name:** `municode` (binary `municode-pp-cli`).
- **Why it should exist:** Today the only ways to programmatically read a municipal code are brittle one-off scrapers or a click-through SPA. No tool gives you: resolve-by-name, offline sync of an entire code into SQLite, FTS/regex search that actually works, plaintext extraction from the HTML chunks, cross-version diffing of codifications, and agent-native (`--json`/`--select`) output. This is the agent-and-power-user front door to 3,300+ US municipal codes.

## Build Priorities
1. **Resolve + browse core** (states, cities, resolve city→code, TOC walk, read section) — fully verified, the foundation.
2. **Local sync + offline FTS search** — sync a code into SQLite, plaintext extraction, FTS5 search (fixes the broken upstream search; the headline differentiator).
3. **Transcendence** — cross-version diff, "find-ordinance" semantic locate, cross-city compare of a topic, citation/section permalink resolver, export (markdown/text).
