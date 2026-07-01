---
name: pp-municode
description: "Every Municode feature, plus an offline local mirror, real full-text search, change tracking, and citation tooling no other Municode tool has. Trigger phrases: `look up Atlanta zoning code`, `what does the city code say about short-term rentals`, `compare noise ordinances across cities`, `find the definition of dwelling unit in the municipal code`, `clone a municipal code for offline search`, `use municode`, `run municode`."
author: "Clu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - municode-pp-cli
    install:
      - kind: go
        bins: [municode-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/government/municode/cmd/municode-pp-cli
---

# Municode — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `municode-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install municode --cli-only
   ```
2. Verify: `municode-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/government/municode/cmd/municode-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Browse and read 3,300+ US municipal codes from the command line, then clone any city's code into a local SQLite database for offline reading and FTS5 search that works even though Municode's own full-text search is a paid MuniPro feature. On top of that local store it adds clone-vs-live drift detection (diff), side-by-side cross-city topic comparison (compare), structured ordinance-history extraction (history), an intra-code cross-reference graph (xref), and authoritative defined-term lookup (defs).

## When to Use This CLI

Use this CLI when an agent or user needs to read, search, or analyze the actual text of US municipal codes and ordinances — zoning, building, licensing, definitions — across one or many cities. It is the right tool for offline legal-text research, citation building, comparing how different jurisdictions regulate a topic, and tracking how a code changed between codification supplements.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for federal or state statutes — it only covers municipal/county codes hosted on Municode.
- Do not use it to obtain paid MuniPro full-text search results; the free tier has no upstream search index.
- Do not use it as legal advice or an authoritative legal citation source without verifying against the official adopted code.
- Do not use it for cities not hosted on library.municode.com (many use American Legal, eCode360, or Code Publishing instead).

## Unique Capabilities

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

## Command Reference

**clients** — Municipalities (clients) hosted on Municode

- `municode-pp-cli clients by-name` — Resolve a municipality by name and state
- `municode-pp-cli clients by-state` — List all municipalities in a state

**codestoc** — Code table-of-contents tree navigation

- `municode-pp-cli codestoc breadcrumb` — Get the ancestry breadcrumb path of a TOC node
- `municode-pp-cli codestoc children` — List the child TOC nodes of a node (use nodeId=productId for the root)

**content** — Section content (HTML chunks) of a code

- `municode-pp-cli content` — Get the content chunk for a TOC node (Docs[] of section HTML)

**products** — Publications (Code of Ordinances, charters) for a municipality

- `municode-pp-cli products by-client` — List products for a municipality by client id
- `municode-pp-cli products by-name` — Resolve a product by client id and product name

**states** — US states available in Municode

- `municode-pp-cli states by-abbr` — Get a single state by two-letter abbreviation
- `municode-pp-cli states list` — List all states with Municode-hosted codes

**versions** — Codification jobs (dated versions of a publication)

- `municode-pp-cli versions <productId>` — Get the latest codification job (version) for a product


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
municode-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Compare a regulation across cities (agent-friendly, narrowed)

```bash
municode-pp-cli compare "noise ordinance" --city "Atlanta, GA" --city "Savannah, GA" --agent --select cities.city,cities.section.citation,cities.section.heading
```

After cloning both cities, aligns the controlling section per city and narrows the deeply nested response to just city, citation, and heading.

### Read a section as clean text

```bash
municode-pp-cli read "Atlanta, GA" PTIICOORENOR_CH1GEPR --json
```

Fetches a chapter/section chunk and returns each section's title and HTML-stripped plaintext.

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

## Auth Setup

No authentication required — Municode's public API at api.municode.com serves municipal codes, ordinances, and table-of-contents data to anyone. Note: upstream full-text search is a paid 'MuniPro' tier and returns no results for free/anonymous users, which is exactly why this CLI builds its own local FTS index over cloned content.

Run `municode-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  municode-pp-cli content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `MUNICODE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `MUNICODE_CONFIG_DIR`, `MUNICODE_DATA_DIR`, `MUNICODE_STATE_DIR`, `MUNICODE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `MUNICODE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `municode-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `MUNICODE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `MUNICODE_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
municode-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
municode-pp-cli feedback --stdin < notes.txt
municode-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `MUNICODE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MUNICODE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
municode-pp-cli profile save briefing --json
municode-pp-cli --profile briefing content --product-id 42 --job-id 42 --node-id 550e8400-e29b-41d4-a716-446655440000
municode-pp-cli profile list --json
municode-pp-cli profile show briefing
municode-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `municode-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/government/municode/cmd/municode-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add municode-pp-mcp -- municode-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which municode-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   municode-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `municode-pp-cli <command> --help`.
