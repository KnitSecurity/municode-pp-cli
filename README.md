# Municode CLI

**Every Municode feature, plus an offline local mirror, real full-text search, change tracking, and citation tooling no other Municode tool has.**

Browse and read 3,300+ US municipal codes from the command line, then clone any city's code into a local SQLite database for offline reading and FTS5 search that works even though Municode's own full-text search is a paid MuniPro feature. On top of that local store it adds clone-vs-live drift detection (diff), side-by-side cross-city topic comparison (compare), structured ordinance-history extraction (history), an intra-code cross-reference graph (xref), and authoritative defined-term lookup (defs).

## Install

The recommended path installs both the `municode-pp-cli` binary and the `pp-municode` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install municode
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install municode --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install municode --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install municode --agent claude-code
npx -y @mvanhorn/printing-press-library install municode --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/government/municode/cmd/municode-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/municode-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install municode --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-municode --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-municode --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install municode --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/municode-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

Once connected, call the `context` tool first — it front-loads the clone-first workflow and the offline-vs-live tool split. The server also exposes the local clone as MCP **resources**: `municode://clones` (inventory) and `municode://clone/{clientId}/{nodeId}` (one section as plain text). Resource reads are offline, and a city cloned mid-session appears in `resources/list` without a restart. See the **[MCP User Manual](docs/mcp-manual.md)** for a plain-language guide, and [docs/local-clone-mcp.md](docs/local-clone-mcp.md) for the offline workflow detail.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/government/municode/cmd/municode-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`). **Use the binary's absolute path** — `go install` places it in `~/go/bin`, which is usually not on the PATH the MCP host uses to launch servers, so a bare `municode-pp-mcp` shows as *not connected*. Find it with `command -v municode-pp-mcp`:

```json
{
  "mcpServers": {
    "municode": {
      "command": "/home/you/go/bin/municode-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required — Municode's public API at api.municode.com serves municipal codes, ordinances, and table-of-contents data to anyone. Note: upstream full-text search is a paid 'MuniPro' tier and returns no results for free/anonymous users, which is exactly why this CLI builds its own local FTS index over cloned content.

## Quick Start

```bash
# Confirm the CLI and api.municode.com are reachable before anything else.
municode-pp-cli doctor --dry-run

# Resolve a city name to its addressable code (clientId, productId, latest jobId).
municode-pp-cli resolve "Atlanta, GA"

# Browse the code's table of contents to find the chapter you need.
municode-pp-cli toc "Atlanta, GA"

# Mirror the whole code into local SQLite so search and the novel commands work offline.
municode-pp-cli clone "Atlanta, GA"

# Full-text search the cloned code locally — works without the paid MuniPro tier.
municode-pp-cli search "short-term rental" --type document

# Pull the authoritative definition of a term with its citation.
municode-pp-cli defs "Atlanta, GA" "dwelling unit"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`clone`** — Pull a municipality's entire code into a local store and an AI-ready Markdown/text tree in one command, so an agent can reference it offline with no further API calls.

  _Reach for this first when an agent needs to read or search a whole city's code repeatedly without hitting the live API each time._

  ```bash
  municode-pp-cli clone "Atlanta, GA" --export ./atlanta-code --agent
  ```
- **`diff`** — See which sections of a city's code changed in the live code since you cloned it: added, removed, or reworded.

  _Reach for this to detect exactly what changed in a code since you last cloned it._

  ```bash
  municode-pp-cli diff "Atlanta, GA" --agent
  ```
- **`stale`** — List synced codes whose upstream codification is newer than your local copy, so you know which mirrors to re-sync.

  _Reach for this to find out-of-date local mirrors before trusting or publishing their text._

  ```bash
  municode-pp-cli stale --agent
  ```
- **`clones`** — List the municipalities already cloned into the local store, with codification version, section count, and last-synced time. Offline only; makes no API call.

  _Reach for this to see what an agent can answer offline before deciding whether to `clone` a new city or re-clone a stale one._

  ```bash
  municode-pp-cli clones --json
  ```

### Cross-city intelligence
- **`compare`** — Lay one topic (e.g. short-term rentals) side by side across several cities, aligning each city's controlling section.

  _Reach for this to compare how different jurisdictions regulate the same activity in one shot._

  ```bash
  municode-pp-cli compare "short-term rental" --city "Atlanta, GA" --city "Savannah, GA" --agent
  ```

### Legal-content extraction
- **`history`** — Turn a section's '(Ord. No. ..., date)' history annotations into structured ordinance / section / date rows.

  _Reach for this to cite the enacting ordinances for a section, or invert with --by-ordinance to see everything an ordinance changed._

  ```bash
  municode-pp-cli history "Atlanta, GA" --section 16-28.001 --agent
  ```
- **`xref`** — List what a section cites and what cites it, building an inbound/outbound reference graph for a city's code.

  _Reach for this to trace how sections of a code reference each other before relying on a single section in isolation._

  ```bash
  municode-pp-cli xref "Atlanta, GA" --section 16-28.001 --inbound --agent
  ```
- **`defs`** — Return the one authoritative definition of a term from a code's Definitions sections, with its citation.

  _Reach for this when you need the controlling definition of a term, not general full-text hits._

  ```bash
  municode-pp-cli defs "Atlanta, GA" "dwelling unit" --agent
  ```

## Recipes

### Compare a regulation across cities (agent-friendly, narrowed)

```bash
municode-pp-cli compare "noise ordinance" --city "Atlanta, GA" --city "Savannah, GA" --agent --select cities.city,cities.section.citation,cities.section.heading
```

After cloning both cities, aligns the controlling section per city and narrows the deeply nested response to just city, citation, and heading.

### Read a section as clean text (offline when cloned)

```bash
municode-pp-cli read "Atlanta, GA" PTIICOORENOR_CH1GEPR --json
```

Fetches a chapter/section chunk and returns each section's title and HTML-stripped plaintext. Choose the data source with `--data-source`: `auto` (default) reads the local clone when the section is present and falls back to a live API call otherwise; `local` reads only the clone and makes no network call; `live` always fetches from the API. See [docs/local-clone-mcp.md](docs/local-clone-mcp.md) for the offline workflow.

### Detect what changed since you cloned

```bash
municode-pp-cli diff "Atlanta, GA"
```

Compares your local clone against the current live code and reports added, removed, and reworded sections with citations.

### Find stale local mirrors

```bash
municode-pp-cli stale
```

Lists every cloned city whose upstream codification is newer than your local copy so you know what to re-clone.

## Usage

Run `municode-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MUNICODE_CONFIG_DIR`, `MUNICODE_DATA_DIR`, `MUNICODE_STATE_DIR`, or `MUNICODE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MUNICODE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MUNICODE_HOME=/srv/municode
municode-pp-cli doctor
```

Under `MUNICODE_HOME=/srv/municode`, the four dirs resolve to `/srv/municode/config`, `/srv/municode/data`, `/srv/municode/state`, and `/srv/municode/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "municode": {
      "command": "municode-pp-mcp",
      "env": {
        "MUNICODE_HOME": "/srv/municode"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MUNICODE_DATA_DIR` overrides an explicit `--home` for that kind. Use `MUNICODE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MUNICODE_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `municode-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### clients

Municipalities (clients) hosted on Municode

- **`municode-pp-cli clients by-name`** - Resolve a municipality by name and state
- **`municode-pp-cli clients by-state`** - List all municipalities in a state

### codestoc

Code table-of-contents tree navigation

- **`municode-pp-cli codestoc breadcrumb`** - Get the ancestry breadcrumb path of a TOC node
- **`municode-pp-cli codestoc children`** - List the child TOC nodes of a node (use nodeId=productId for the root)

### content

Section content (HTML chunks) of a code

- **`municode-pp-cli content`** - Get the content chunk for a TOC node (Docs[] of section HTML)

### products

Publications (Code of Ordinances, charters) for a municipality

- **`municode-pp-cli products by-client`** - List products for a municipality by client id
- **`municode-pp-cli products by-name`** - Resolve a product by client id and product name

### states

US states available in Municode

- **`municode-pp-cli states by-abbr`** - Get a single state by two-letter abbreviation
- **`municode-pp-cli states list`** - List all states with Municode-hosted codes

### versions

Codification jobs (dated versions of a publication)

- **`municode-pp-cli versions <productId>`** - Get the latest codification job (version) for a product


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
municode-pp-cli content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
municode-pp-cli content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
municode-pp-cli content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
municode-pp-cli content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
municode-pp-cli content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
municode-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `municode-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/municode-pp-cli/config.toml`; `--home`, `MUNICODE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns nothing** — Run 'municode-pp-cli clone "<City, ST>"' first — search reads the local FTS index, not the paid upstream search.
- **a local command says the city is not cloned** — Run 'municode-pp-cli clone "<City, ST>"' to mirror that city's code before querying it offline.
- **city not found by name** — Use the exact municipality name and two-letter state, e.g. "St. Petersburg, FL"; list candidates with 'municode-pp-cli cities --state FL'.
- **diff or history shows nothing** — These read the local store — clone the city first. diff reports changes since you cloned, so a freshly cloned code shows no drift.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**noclocks/municode-scraper**](https://github.com/noclocks/municode-scraper) — Python (31 stars)
- [**wcurrangroome/municoder**](https://github.com/wcurrangroome/municoder) — R (3 stars)
- [**opengovfoundation/lasvegas-parser**](https://github.com/opengovfoundation/lasvegas-parser) — PHP (2 stars)
- [**TIDYAPP/open-legal-codes**](https://github.com/TIDYAPP/open-legal-codes) — TypeScript
- [**RoryStolzenberg/municode-dump**](https://github.com/RoryStolzenberg/municode-dump) — JavaScript
- [**Skatterbrainz/MunicipalMCP**](https://github.com/Skatterbrainz/MunicipalMCP) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
