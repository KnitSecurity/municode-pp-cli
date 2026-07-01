# Municode MCP — User Manual

A plain-language guide to installing, connecting, and using the **Municode MCP
server** (`municode-pp-mcp`) — the tool that lets an AI assistant look up,
search, and read US municipal codes, and keep a local offline copy of any city's
code so it can answer without hitting the network every time.

This manual is written for a person driving the server through an AI assistant
(Claude Desktop, Claude Code, or any MCP-capable host). You mostly type ordinary
requests to your assistant; the assistant picks the tools. Each section also
shows the underlying tool so you know what's happening.

**Contents**
1. [What this is](#1-what-this-is)
2. [Install & connect](#2-install--connect)
3. [The mental model: clone once, then answer offline](#3-the-mental-model-clone-once-then-answer-offline)
4. [Your first session](#4-your-first-session)
5. [What you can ask (tool reference)](#5-what-you-can-ask-tool-reference)
6. [The local clone as resources](#6-the-local-clone-as-resources)
7. [Common recipes](#7-common-recipes)
8. [Where your data lives](#8-where-your-data-lives)
9. [Troubleshooting](#9-troubleshooting)
10. [Privacy & safety](#10-privacy--safety)

---

## 1. What this is

Municode hosts the official code of ordinances for 3,300+ US municipalities. Its
public data (table of contents, section text) is free to read, but its
**full-text search is a paid tier** — anonymous search returns nothing.

This MCP server gives your assistant:

- **Live lookup** of any Municode-hosted city's code (browse, resolve, read).
- **A local clone** — pull a whole city's code into a local database once, then
  search and read it offline with no further API calls.
- **Tools no official Municode tool has**: real full-text search over the clone,
  change/drift detection, ordinance-history extraction, cross-reference graphs,
  authoritative defined-term lookup, and cross-city comparison.

Everything is **read-only**. The server never edits, publishes, or deletes
anything on Municode — it only reads public data and writes to your local copy.

---

## 2. Install & connect

You need two things: the server binary, and an entry in your MCP host's config.

### Option A — Claude Desktop (one-click)

1. Download the `.mcpb` bundle for your platform from the project's latest
   release.
2. Double-click it. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0+. Pre-built bundles ship for macOS Apple Silicon
and Windows; on other platforms use Option B.

### Option B — Claude Desktop (manual config)

Install the binary:

```bash
go install github.com/mvanhorn/printing-press-library/library/government/municode/cmd/municode-pp-mcp@latest
```

Add it to `~/Library/Application Support/Claude/claude_desktop_config.json`
(macOS) or the equivalent on your OS:

```json
{
  "mcpServers": {
    "municode": {
      "command": "municode-pp-mcp"
    }
  }
}
```

Restart Claude Desktop. You should see "municode" in the tools list.

### Option C — Claude Code

```bash
claude mcp add municode-pp-mcp -- municode-pp-mcp
claude mcp list      # verify it's connected
```

### Option D — any other MCP host

Run the binary with the stdio transport (the default) and point your host at it:

```bash
municode-pp-mcp                     # stdio (default)
municode-pp-mcp --transport http --addr :7777   # streamable HTTP, for remote hosts
```

### First thing to do once connected

Ask your assistant to **"call the municode context tool."** That returns the
domain model and, importantly, the *clone-first workflow* and the list of which
tools are offline vs live. A good assistant calls this on its own; asking once
makes sure it has the map.

---

## 3. The mental model: clone once, then answer offline

There are two kinds of tools, and knowing the difference is the whole game:

| | **Live tools** (hit the Municode API) | **Offline tools** (read your local clone) |
|---|---|---|
| **Use for** | Discovering a city, first-time browse, checking freshness | Everything else once a city is cloned |
| **Tools** | `cities`, `states_*`, `clients_*`, `products_*`, `codestoc_*`, `content_get`, `versions_latest`, `resolve`, `toc`, `diff`, `stale` | `search`, `read` (local), `defs`, `history`, `xref`, `compare`, `clones`, `sql` |
| **Network** | Yes | No |

The recommended loop:

```
  1. Discover   →  "Is Boulder, CO on Municode?"          (live)
  2. Clone      →  "Clone Boulder's code."                (one live pull, minutes)
  3. Answer     →  "What does Boulder say about X?"        (offline, instant)
  4. Freshness  →  "Is my Boulder clone still current?"    (live check)
  5. Refresh    →  "Re-clone Boulder."                     (live pull)
```

You only pay the slow live clone once. After that, searching and reading are
instant and work with no network.

---

## 4. Your first session

Type these to your assistant, in order. The tool each maps to is in brackets.

> **"Is Boulder, CO available on Municode?"** — confirms the city exists and
> resolves its ids. *[resolve]*

> **"Clone Boulder, Colorado's code so we can work offline."** — pulls the whole
> code into your local store. This takes a few minutes; it walks the entire table
> of contents and fetches every section. *[clone]*

> **"What cities do I have cloned?"** — shows your local library with each city's
> version and section count. *[clones]*

> **"What does Boulder's code say about the legislative intent of chapter 1?"**
> — searches and reads the clone, offline. *[search + read]*

> **"Give me the exact definition of 'dwelling unit' in Boulder's code, with a
> citation."** *[defs]*

> **"Has Boulder published a newer version of the code since I cloned it?"**
> *[stale, then diff]*

---

## 5. What you can ask (tool reference)

You rarely name tools yourself — you describe what you want. This table maps
intents to tools so you can steer when needed. `*` marks required inputs.

### Find & browse (live)

| Ask for… | Tool | Inputs |
|----------|------|--------|
| Cities in a state that have a code | `cities` | `*ST` |
| Resolve "City, ST" to its ids | `resolve` | `*city` |
| Browse the table of contents | `toc` | `*city`, `node`, `depth` |
| Latest published version of a code | `versions_latest` | `*productId` |

### Clone & manage the local copy

| Ask for… | Tool | Inputs |
|----------|------|--------|
| Pull a whole city's code offline | `clone` | `*city`, `export`, `max-nodes` |
| See what's cloned locally | `clones` | *(none)* |
| Which clones are behind upstream | `stale` | *(none)* |
| What changed since you cloned | `diff` | `*city` |

- **`clone … export=<dir>`** also writes a clean Markdown/text tree to disk (with
  a timestamped `clone-manifest.json`) that an assistant can read as files.
- **`clone … max-nodes=<n>`** caps how many content chunks are fetched — handy for
  a quick partial pull while testing.

### Read & search the code (offline once cloned)

| Ask for… | Tool | Inputs |
|----------|------|--------|
| Full-text search the code | `search` | `*query`, `limit` |
| Read a section as clean text | `read` | `*city`, `*node-id`, `data-source`, `citation` |
| The controlling definition of a term | `defs` | `*city`, `*term` |
| A section's enacting-ordinance history | `history` | `*city`, `section`, `by-ordinance` |
| What a section cites / what cites it | `xref` | `*city`, `section`, `inbound` |
| One topic across several cities | `compare` | `*topic`, `city` (repeatable), `state` |
| Ad-hoc analysis over the local store | `sql` | `*query` (read-only SELECT) |

**About `read` and `data-source`:** `read` takes `data-source` = `auto` (default),
`local`, or `live`.
- `auto` — read the clone if the section is there, otherwise fall back to a live
  API call. Best default.
- `local` — clone only, never the network. If the city isn't cloned you get an
  empty result and a hint to clone it.
- `live` — always fetch fresh from Municode.

> Grouping note: an offline `read` returns the section **plus its direct child
> sections**; a live `read` returns Municode's content chunk, which for a big
> container node (a whole title or chapter) can span more levels. For an
> individual section, both give the same text and citation.

### The `context` tool

`context` returns the taxonomy, query tips, and the clone-first workflow guidance.
It's the one tool to call **first** in a session — it tells the assistant which
tools are offline vs live and how the pieces fit.

> Tools you can ignore: `sync`, `workflow`, `workflow_archive`, `workflow_status`,
> and `read_a_section_as_clean_text` are generic framework helpers. The
> purpose-built tools above cover municipal-code work better; `context` steers the
> assistant to them.

---

## 6. The local clone as resources

Besides tools, the server exposes your clone as MCP **resources** — a browsable
corpus the assistant (or you, in a host that shows resources) can open directly.

| Resource | What it is |
|----------|------------|
| `municode://clones` | A JSON inventory of every cloned city (state, ids, version, section count, last synced). The resource form of the `clones` tool. |
| `municode://clone/{clientId}/{nodeId}` | One code section as plain text: title, body, and a `Source:` citation link. |

- Reading a resource is **offline** — it only opens the local store.
- After the first clone, the resource list is populated with one entry per stored
  section. There's one such entry per section, so a large city adds a lot of
  entries.
- **In-session refresh:** if you clone a city *while the assistant is connected*,
  the new city's sections appear in the resource list right away — no need to
  restart the server.

Ask your assistant: *"List the municode resources"* or *"Open the municode
resource for Boulder section TIT1GEAD_CH1COIN_1-1-1LEIN."*

---

## 7. Common recipes

Copy-paste these as prompts to your assistant.

**Compare a regulation across cities**
> "Clone Atlanta GA and Savannah GA, then compare how each regulates short-term
> rentals, and give me the controlling section and citation for each."
> *(clone ×2 → compare)*

**Cite the ordinances behind a rule**
> "For Atlanta section 16-28.001, list the ordinances that enacted or amended it,
> with dates." *(history)*

**Trace how sections reference each other**
> "In Atlanta's code, what does section 16-28.001 cite, and what cites it?"
> *(xref, add `inbound` for what-cites-it)*

**Get an authoritative definition, not search hits**
> "What is the code's exact definition of 'accessory dwelling unit' in Boulder,
> with the citation?" *(defs)*

**Keep a clone current**
> "Check if any of my cloned cities are out of date, and for any that are, show me
> what changed." *(stale → diff)*

**Snapshot for versioning**
> "Clone Boulder and export it to ~/codes/boulder-2026-06 so I have a timestamped
> copy." *(clone with export)*

---

## 8. Where your data lives

| Path | What |
|------|------|
| `~/.local/share/municode-pp-cli/data.db` | Your default clone store (SQLite). All clones live here unless relocated. |
| `<export-dir>/` | The Markdown/text tree + `clone-manifest.json` written by `clone … export=<dir>`. |

**Relocating the store.** The MCP server does **not** take command-line flags from
the host. To move where it reads/writes, set an environment variable in the host's
`env` block:

```json
{
  "mcpServers": {
    "municode": {
      "command": "municode-pp-mcp",
      "env": { "MUNICODE_DATA_DIR": "/path/to/your/store" }
    }
  }
}
```

The command-line CLI (`municode-pp-cli`) shares the same default store, so a city
you clone from the terminal is immediately available to the MCP server, and vice
versa.

---

## 9. Troubleshooting

**"Search / read says the city isn't cloned (or returns nothing)."**
Clone it first — offline tools read the local store, not the live site. Ask:
"Clone <City, ST>." Then retry.

**"`database disk image is malformed`."**
A clone was interrupted (the process was killed mid-write), leaving stale
SQLite sidecar files. Delete them and retry:
```bash
rm -f ~/.local/share/municode-pp-cli/data.db-wal ~/.local/share/municode-pp-cli/data.db-shm
```
Your committed `data.db` is intact; this only clears the leftover journal.

**"A clone is taking a long time."**
That's expected — a full clone walks the entire table of contents and fetches
every section (minutes for a mid-size city). It's resumable and bounded by an
internal time budget; if it stops early, run the clone again to continue, or use
a smaller city to test. For a quick partial test, ask to clone with
`max-nodes=40`.

**"The assistant keeps calling the live API instead of the clone."**
Ask it to "call the context tool first," or say "use the local clone / offline
tools." `context` carries the clone-first guidance that steers tool choice.

**"Nothing shows up in the resource list."**
The list is empty until you clone at least one city. After a clone it populates
automatically (no restart needed).

**"Is the server actually connected?"**
In Claude Code: `claude mcp list`. In Claude Desktop: check the tools/plug icon in
a chat — "municode" should appear.

---

## 10. Privacy & safety

- **Read-only.** The server only reads public Municode data and writes to your
  local store. It never creates, edits, publishes, or deletes anything upstream.
- **No account or key needed.** Municode's public API is unauthenticated.
- **The clone stays local.** Cloned code lives in your local SQLite store; nothing
  is uploaded.
- **The server can't be pointed at arbitrary files.** The offline tools and
  resources are pinned to your default store — an MCP client cannot make the
  server read some other file on your disk or smuggle a query through a resource
  link. Section addresses are treated strictly as data.

---

*For the CLI-side workflow and implementation detail, see
[local-clone-mcp.md](local-clone-mcp.md).*
