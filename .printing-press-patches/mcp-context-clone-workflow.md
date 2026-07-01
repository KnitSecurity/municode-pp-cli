# Reprint guard: clone-first workflow guidance in the MCP `context` tool

**Target (generated):** `internal/mcp/tools.go` (`handleContext`)
**Feature:** clone-workflow discoverability (plan unit U6)

## Intent

The local-clone feature is only useful if an agent knows the loop: clone a city
once, then answer questions **offline** from the local mirror and the
`municode://clone/...` resources, reaching for the live API tools only to
discover a city or refresh a stale clone. `handleContext` is the one place that
front-loads domain knowledge to agents, so the guidance lives there. On reprint,
re-add the `clone_workflow` entry or the offline-first steering is lost and
agents default to live API calls.

## Edit to re-apply

In the `ctx` map returned by `handleContext`, add a `clone_workflow` key (placed
after `tool_surface`) carrying:

- `summary` — one line beginning "Clone once, then answer offline" that states the
  clone-first, answer-offline loop (the discoverability test asserts on the
  "Clone once" / "offline" substrings).
- `steps` — the ordered loop: discover (live) → `clone` once → answer offline
  (`search`, `read --data-source local`, `defs`, `history`, `xref`, `compare`,
  and the `municode://clones` / `municode://clone/{clientId}/{nodeId}` resources)
  → check freshness (`stale`, `diff`) → refresh (`clone`).
- `offline_tools` and `live_tools` — the split, with each tool on exactly one side
  (the test asserts no tool appears on both).
- `resources` — the two clone resource URIs so an agent can find them.
- `tip` — prefer offline tools/resources for "what does the code say" once cloned.

The corresponding `read` (`--data-source` semantics) and `clones` (offline-only)
`Long` help strings live in the hand-authored `internal/cli/read.go` and
`internal/cli/clones.go` and survive reprint on their own.

## Not re-applied on purpose

The optional `browse_local_code` intent (plan U6, marked optional) was **not**
added to the generated `internal/mcp/intents.go`; the `context` guidance plus the
resources deliver the steering without a second generated-file hand-edit to carry
forward.
