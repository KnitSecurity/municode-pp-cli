# Municode Browser-Sniff Discovery Report

**Goal:** Capture the exact `/search` request the Municode UI fires, to resolve why direct `/search` returns 0 hits.

**Method:** `browser-use` could not launch its managed browser on this headless host (daemon start timeout). Pivoted to a direct **Chromium net-log capture** (`--log-net-log`, `--virtual-time-budget`) against the confirmed-working Playwright Chromium — a capture method, not tool-debugging. Navigated `library.municode.com/ga/atlanta/codes/code_of_ordinances?searchText=zoning` and recorded all network traffic, then probed the discovered endpoints directly.

## Key Findings

### 1. Frontend uses a same-origin proxy; public API is the CLI target
The Angular SPA calls **`https://library.municode.com/api/...`** (a BFF proxy), not `api.municode.com` directly. The public host **`https://api.municode.com`** serves the same data unauthenticated and is the correct CLI target (it's what every community scraper uses; no proxy, no cookies). Runtime: `standard_http` (no browser capture or clearance cookie at runtime).

### 2. Full-text search is a PAID feature (MuniPro) — not available to free/anonymous users
This is the decisive finding and the reason direct `/search` returns 0 hits:
- `GET library.municode.com/localapi/IpBasedAuthentication/HasMuniproSearch` → `{"hasMuniproSearch":false,...}`
- Frontend `GET library.municode.com/api/search?...` → **HTTP 401**
- Public `GET api.municode.com/search?productId=&jobId=&query=` → HTTP 200 but `{"NumberOfHits":0,"Hits":[]}` for every query/city (free tier has no search-index access)

**Implication:** Upstream full-text search cannot be a headline CLI feature — it is empty for unauthenticated users. **Local FTS5 over synced content is the only viable search** and becomes the headline differentiator (offline, regex, works without MuniPro). A thin upstream `/search` passthrough may be retained but must honestly report the MuniPro gating.

### 3. Name-based resolution endpoints (verified on public api.municode.com)
The net-log surfaced cleaner resolution endpoints than the list-and-filter forms:
- `GET /States/abbr?stateAbbr=GA` → single `{StateID,StateName,StateAbbreviation}`
- `GET /Clients/name?clientName=atlanta&stateAbbr=ga` → `{ClientID, ClientName, State, Address, City, ZipCode, Website, ...}` (resolve a city directly by name — no list+filter)
- `GET /Products/name?clientId=1093&productName=code of ordinances` → product with rich `ContentType{Id,Name,IsSearchable}` + `Features{CodeBank,OrdBank,CodeBankCompare,...}`
- `GET /codesToc/breadcrumb?jobId=&nodeId=&productId=` → ancestry path of a node (citation/permalink building)

## Replayability
All target endpoints replay over plain HTTP with a Chrome UA + `Referer: https://library.municode.com/`. No browser sidecar, no cookies, no clearance. Capture artifacts: `captured-urls.txt` (23 unique URLs), `traffic-analysis.json`.
