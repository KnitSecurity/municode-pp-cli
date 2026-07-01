# Upstream submission notes

This CLI began as a CLI Printing Press print and was then extended and fixed by
hand. This document is the hand-off for two upstreams:

1. Submitting the **Municode CLI** as a new item in
   [`mvanhorn/printing-press-library`](https://github.com/mvanhorn/printing-press-library).
2. Folding the **framework-level fixes** back into
   [`mvanhorn/cli-printing-press`](https://github.com/mvanhorn/cli-printing-press)
   (the generator), because they are not Municode-specific — they affect every
   printed CLI.

Public source of truth for the standalone build:
<https://github.com/KnitSecurity/municode-pp-cli> (tag `v1.0.2`).

---

## Part 1 — Framework fixes (raise against `cli-printing-press`)

These should be issues/PRs on the **generator**, not just carried as
`.printing-press-patches/` in this CLI. Ordered by blast radius.

### 1. `.gitignore` binary patterns must be root-anchored — **HIGH**
The generated `.gitignore` ignores the built binary by bare name (e.g.
`municode-pp-cli`). With no leading slash, that pattern also matches the
**`cmd/municode-pp-cli/` source directory**, so `cmd/<cli>/main.go` is silently
excluded from the repo. Local builds still work (the file is in the working
tree), but **no fresh clone can build the CLI**, and
`go install .../cmd/<cli>@latest` fails with "module found but does not contain
package". This affects every printed CLI, since the binary name always equals its
`cmd/` subdirectory.
**Fix:** emit root-anchored ignores (`/municode-pp-cli`, `/municode-pp-mcp`) or
ignore a dedicated build output dir instead.
**Evidence here:** `.gitignore` + the `fix: commit cmd/municode-pp-cli source`
change.

### 2. Back off before retrying transient network errors — **MEDIUM**
The client template retries 429 and 5xx with exponential backoff, but retries
network-level failures (connection reset, DNS blip, request timeout) with a bare
`continue` and no backoff — three retries burn out in a tight millisecond loop,
so a brief blip mid-walk aborts a long operation (e.g. a full clone).
**Fix:** apply the same `2^attempt`-second backoff on the `HTTPClient.Do` error
path, honoring `ctx` cancellation via `sleepContext`.
**Evidence here:** `.printing-press-patches/client-network-retry-backoff.md` and
`internal/client/client_network_retry_test.go`.

### 3. Use a 2-part `go` directive in the generated `go.mod` — **MEDIUM**
Generated `go.mod` pins `go 1.26.4` (3-part patch). Go treats that as a toolchain
requirement and tries to download `golang.org/toolchain@...go1.26.4` during
`go install`, which fails for a fresh user. A 2-part `go 1.26` uses the user's
local toolchain when it is new enough.
**Fix:** emit `go 1.26` (major.minor).

### 4. Version fallback to build info — **LOW**
`go install`-built binaries do not get release `-ldflags`, so the `version`
command and MCP `serverInfo` report the compiled default instead of the installed
tag. A `runtime/debug.ReadBuildInfo()` fallback fixes this.
**Fix:** in the generated `version` command and MCP main, resolve the version via
build info when the compiled value is a placeholder.
**Evidence here:** `internal/cliutil/buildversion.go` +
`.printing-press-patches/version-buildinfo-fallback.md`.

---

## Part 2 — The Municode library item

Submit via the Printing Press publish flow (`/printing-press-publish` or the
`publish` sub-skill) as a new item, e.g. `library/government/municode`.

### What it adds beyond a stock print
A **local-clone + offline-AI** surface for US municipal codes:

- `clone` — pull a city's entire code into local SQLite (TOC + content + FTS +
  ordinance-history lineage), and by default a per-city Markdown tree. Runs to
  completion in one set-and-forget pass; resilient to transient network blips.
- Offline commands over the clone: `search` (real FTS — upstream search is a paid
  tier), `read --data-source local|auto|live`, `defs`, `history`, `xref`,
  `compare`, `clones`, `stale`, `diff`.
- **MCP surface:** the clone exposed as resources (`municode://clones`,
  `municode://clone/{clientId}/{nodeId}`) plus a `context` tool that front-loads a
  clone-first, offline-vs-live workflow; in-session clones refresh the resource
  list without a restart.

Design/plan: `docs/plans/2026-06-30-001-feat-local-clone-mcp-plan.md`.
User docs: `docs/local-clone-mcp.md`, `docs/mcp-manual.md`.

### Municode-specific fix worth noting for the generic sync
Municode delivers a large subtree as a **chunk-group TOC**: the requested node's
body is inlined, but its descendants come back as content-less pointer docs
(`Content: null`) whose bodies live in a deeper per-node chunk-group fetch. The
sync walk must **not** mark a content-less pointer as "covered" — doing so
suppresses the deeper fetch and leaves stub sections. This is specific to
Municode's content model, but the general lesson (never treat a content-less
placeholder as fully synced) may be worth encoding in the generic sync template
for other chunked APIs.
**Evidence here:** `internal/cli/municode_store.go` +
`internal/cli/municode_clone_completeness_test.go`.

### CLI-specific patches that stay with the item
The remaining `.printing-press-patches/` entries (`cli-root-register-clones`,
`mcp-context-clone-workflow`, `mcp-main-resources-and-refresh`,
`docs-local-clone-mcp-surface`) are wiring for the novel commands/resources and
travel with the library item, not the generator.
