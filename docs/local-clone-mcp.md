# Local clone & the MCP clone surface

This guide covers the offline workflow that lets an AI (or a person) work with a
city's municipal code from a **local clone** instead of hitting the Municode API
on every question: the `clone` / `clones` commands, offline `read`, and the MCP
resource surface that exposes the clone to an agent.

It is the authoritative reference for the feature shipped in
[`docs/plans/2026-06-30-001-feat-local-clone-mcp-plan.md`](plans/2026-06-30-001-feat-local-clone-mcp-plan.md).
It lives under `docs/` (hand-authored) so it survives a CLI reprint; the
generated `README.md` / `SKILL.md` carry only pointers to it.

---

## The idea: clone once, then answer offline

Municode's public API serves table-of-contents and section content to anyone,
but its **full-text search is a paid MuniPro tier** that returns nothing for
anonymous users. Rather than call the API repeatedly, you clone a city's whole
code into a local SQLite store once, then answer questions from that store with
no further network calls.

```
                clone "<City, ST>"                 (one live pull)
  Municode API ───────────────────────────────▶  local SQLite store (~/.local/share/municode-pp-cli/data.db)
                                                        │
        offline, zero network:                          ▼
        search · read --data-source local · defs · history · xref · compare · clones · sql
        MCP resources: municode://clones, municode://clone/{clientId}/{nodeId}
```

Reach for the **live** API tools only to *discover* a city (`clients`, `states`,
`products`) or to *check freshness* of a clone (`stale`, `diff`). Everything
that answers "what does the code say" reads the local clone.

---

## CLI workflow

### 1. Clone a city

```bash
# Mirror the whole code (TOC + content + FTS index + ordinance-history lineage)
municode-pp-cli clone "Boulder, CO"

# Also write an AI-ready Markdown/text tree to disk, timestamped for later
# version comparison, with a clone-manifest.json:
municode-pp-cli clone "Boulder, CO" --export ./boulder-code --agent
```

A clone is resumable and bounded (a ~15-minute budget; partial-on-timeout), so a
large code can be cloned across a couple of runs. Each stored section records its
place in the table of contents (`parent_id` + `depth`), which is what makes the
clone navigable both on the CLI and through the MCP resources.

### 2. See what you have offline

```bash
municode-pp-cli clones            # human table
municode-pp-cli clones --json     # machine-readable
```

`clones` lists every municipality in the local store with its codification
version (`job_id`), section count, and last-synced timestamp. It reads **only**
the local store and makes no API call — use it to decide whether to `clone` a new
city or re-clone a stale one. It intentionally exposes **no `--db` flag** (see
[Security](#security-the-clone-surface-is-pinned-to-the-default-store)).

### 3. Read a section offline

`read` chooses its data source with `--data-source`:

| `--data-source` | Behavior |
|-----------------|----------|
| `auto` (default) | Read the local clone when the section is present; **fall back to a live API call** otherwise (or when the local store is present but degraded). |
| `local` | Read **only** the clone; makes no network call. Prints a `clone` hint and exits 0 (empty result) when the city isn't cloned. |
| `live` | Always fetch from the API. |

```bash
# Offline, no network — reads Boulder from the clone:
municode-pp-cli read "Boulder, CO" TIT1GEAD --data-source local

# Offline-first with automatic live fallback (the default):
municode-pp-cli read "Boulder, CO" TIT1GEAD
```

**Grouping note.** The offline read returns the requested section **plus its
direct child sections** from the clone. A live read returns the API's *content
chunk*, which for a container node (a title or chapter) may span more levels. For
a **leaf** section both return the same text and citation. Don't treat offline
`read` of a container node as byte-identical to a live read of the same node.

### 4. Check freshness, see drift, refresh

```bash
municode-pp-cli stale                 # which local mirrors are behind upstream
municode-pp-cli diff "Boulder, CO"    # section-level drift since you cloned
municode-pp-cli clone "Boulder, CO"   # re-run to refresh the mirror
```

---

## MCP workflow

The companion MCP server (`municode-pp-mcp`) exposes the same clone offline to an
agent, in two forms. For a plain-language, human-facing guide to installing,
connecting, and driving the MCP server, see [mcp-manual.md](mcp-manual.md).

### Tools

Every CLI command is mirrored as an MCP tool, plus a `context` tool that
front-loads the domain model. **Call `context` first** — its `clone_workflow`
block states the clone-first loop and splits the tools into offline vs live:

- **offline** (read the local clone, no network): `search`, `read`
  (`--data-source local|auto`), `defs`, `history`, `xref`, `compare`, `clones`,
  `sql`
- **live** (hit the API): `clients`, `states`, `products`, `codestoc`,
  `content`, `versions`, `diff`, `stale`

### Resources

The clone is also exposed as MCP **resources**, so an agent can browse and read
it as a corpus:

| Resource URI | What it is |
|--------------|------------|
| `municode://clones` | JSON inventory of cloned cities (state, ids, codification version, section count, last synced) — the resource twin of the `clones` command. |
| `municode://clone/{clientId}/{nodeId}` | One cloned code section as plain text (title + body + `Source:` citation). Read via the resource template; also listed, one entry per stored section. |

Resource reads are **offline** — they only open the local store and query
SQLite; they never call the network.

### In-session clones appear without a restart

If an agent clones a city **during** a live MCP session, the new city's sections
show up in `resources/list` immediately — no server restart. This is wired with
an `OnAfterCallTool` hook (mcp-go): after a `clone` tool call completes, the
resource list is rebuilt from the current store contents. The rebuild is
idempotent and uses an atomic upsert, so a concurrent `resources/list` never sees
a half-populated list.

---

## Security: the clone surface is pinned to the default store

The MCP resource handlers and the offline CLI `read`/`clones` paths **always**
resolve the store path from the default data dir
(`~/.local/share/municode-pp-cli/data.db`, or the platform equivalent). They do
**not** honor a client-supplied database path:

- The `clones` command exposes no `--db` flag.
- The MCP surface never accepts a filesystem path from the client; a section URI
  only yields an integer `clientId` and a string `nodeId`, both bound as SQL
  parameters (never interpolated).

This means an MCP client cannot point the server at an arbitrary file on the host
or smuggle SQL through a resource URI. See the plan's KTD6/R7 for the rationale.

---

## Where the data lives

| Path | Contents |
|------|----------|
| `~/.local/share/municode-pp-cli/data.db` | The default clone store (SQLite): sections in a generic `resources` table + an FTS5 index. |
| `./<export-dir>/` (with `clone --export`) | AI-ready Markdown/text tree + `clone-manifest.json`, timestamped for version comparison. |

Run `municode-pp-cli doctor --json` to print the resolved paths on your machine.
