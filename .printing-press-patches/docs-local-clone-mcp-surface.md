# Reprint guard: document the local-clone / MCP-resource surface

**Targets (generated):** `README.md`, `SKILL.md`
**Companions (hand-authored, reprint-safe):** `docs/local-clone-mcp.md`, `docs/mcp-manual.md` (plain-language MCP user manual)
**Feature:** local-clone offline + MCP resources (plan `docs/plans/2026-06-30-001-feat-local-clone-mcp-plan.md`)

## Intent

The local-clone feature adds surfaces the generated docs don't describe: the
offline `clones` command, `read --data-source auto|local|live`, and the MCP
resource surface (`municode://clones`, `municode://clone/{clientId}/{nodeId}`)
with in-session refresh. On reprint the generated `README.md`/`SKILL.md` are
regenerated and lose these additions. Re-apply the edits below. The authoritative
long-form guide lives in `docs/local-clone-mcp.md`, which is hand-authored and
survives reprint on its own — the generated docs only need pointers to it.

## Edits to re-apply

**README.md**
- In the "Local state that compounds" feature list, add a `clones` bullet
  (offline inventory of cloned cities; no API call; `municode-pp-cli clones --json`).
- Retitle the "Read a section as clean text" recipe to note it is offline when
  cloned, and describe the `--data-source auto|local|live` flag; link
  `docs/local-clone-mcp.md`.
- In the Claude Desktop / MCP section, add a paragraph: call `context` first;
  the server exposes the clone as resources `municode://clones` and
  `municode://clone/{clientId}/{nodeId}` (offline reads, mid-session refresh);
  link the MCP user manual `docs/mcp-manual.md` and `docs/local-clone-mcp.md`.

**SKILL.md**
- Same `clones` bullet in "Local state that compounds".
- Same `--data-source` note on the "Read a section as clean text" recipe, linking
  `docs/local-clone-mcp.md`.
- In "MCP Server Installation", add the same `context`-first + resources
  paragraph, linking `docs/local-clone-mcp.md`.

Do NOT hand-edit `CHANGELOG.md` — it is maintained by the library release
automation (see AGENTS.md).
