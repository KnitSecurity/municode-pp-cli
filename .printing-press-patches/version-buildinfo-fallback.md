# Reprint guard: version falls back to go-install build info

**Targets (generated):** `internal/cli/version.go`, `cmd/municode-pp-mcp/main.go`
**Companion (hand-authored):** `internal/cliutil/buildversion.go` (+ test)

## Intent

Binaries built with `go install .../cmd/...@vX.Y.Z` do not receive the release
build's `-ldflags`, so the compiled-in `version` ("1.0.0" for the CLI,
"0.0.0-dev" for the MCP server) is what users see — not the installed tag. The
hand-authored `cliutil.ResolveVersion` reads `runtime/debug` build info and
prefers a concrete module version when present. On reprint, re-wire the two call
sites or `go install` builds mis-report their version again.

## Edits to re-apply

- `internal/cli/version.go`: import `.../internal/cliutil` and print
  `cliutil.ResolveVersion(version)` instead of the bare `version`.
- `cmd/municode-pp-mcp/main.go`: import `.../internal/cliutil` and pass
  `cliutil.ResolveVersion(version)` to `server.NewMCPServer`.

Ldflag-stamped and local (`(devel)`) builds keep their compiled version, so this
is purely additive.

## Upstream

Belongs in the Printing Press generator (the `version` command and MCP main
templates), not just this CLI. Tracked in docs/upstream-submission.md.
