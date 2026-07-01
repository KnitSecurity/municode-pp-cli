# Upstream contributions to CLI Printing Press

This CLI began as a CLI Printing Press print and is now maintained as a
**standalone project** at <https://github.com/KnitSecurity/municode-pp-cli>. It
is not published to `mvanhorn/printing-press-library` (that submission was
intentionally set aside). This document tracks only the **framework-level fixes**
worth folding back into
[`mvanhorn/cli-printing-press`](https://github.com/mvanhorn/cli-printing-press)
(the generator), because they are not Municode-specific — they affect every
printed CLI.

Status:

- **Network-retry backoff — SUBMITTED.** Issue
  [mvanhorn/cli-printing-press#3423](https://github.com/mvanhorn/cli-printing-press/issues/3423),
  PR [#3424](https://github.com/mvanhorn/cli-printing-press/pull/3424).
- The remaining fixes below are not yet submitted.

---

## Framework fixes (raise against `cli-printing-press`)

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

### 2. Back off before retrying transient network errors — **MEDIUM** — ✅ SUBMITTED (#3424)
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

### 5. Chunk-group completeness in the generic sync — note only
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

---

## Library submission — set aside

Publishing this CLI to `mvanhorn/printing-press-library` was intentionally not
pursued. This repo is maintained as a standalone project instead. If a library
submission is ever revisited, do it via the Printing Press publish flow
(`/printing-press-publish`), which regenerates a library-shaped tree — do **not**
fork this repo into a parallel library version.

The `.printing-press-patches/` entries remain as reprint guards documenting the
generated-file hand-edits, in case this source is ever re-run through the
generator.
