# Reprint guard: MCP main wires clone resources + in-session refresh

**Target (generated):** `cmd/municode-pp-mcp/main.go`
**Feature:** local-clone MCP surface (plan `docs/plans/2026-06-30-001-feat-local-clone-mcp-plan.md`, units U2 + U5)

## Intent

The generated MCP main only registers tools. The local-clone feature adds a
**resource** surface (an inventory resource plus one resource per cloned code
section) and needs that surface to stay current when a city is cloned mid-session.
On reprint, re-apply the three edits below or the resource surface silently
disappears from the server.

## Edits to re-apply in `main()`

1. Enable resource capabilities on the server:
   `server.WithResourceCapabilities(false, true)` in the `NewMCPServer` options.
2. Install the in-session refresh hook so a `clone` tool call re-lists resources
   without a restart. Because the hook is an option passed at construction time
   but must call back into the built server, declare the server first and let the
   hook read it lazily:
   ```go
   var s *server.MCPServer
   s = server.NewMCPServer(
       "Municode", version,
       server.WithToolCapabilities(false),
       server.WithResourceCapabilities(false, true),
       server.WithHooks(mcptools.RefreshHooks(func() *server.MCPServer { return s })),
   )
   ```
3. After `mcptools.RegisterTools(s)`, call `mcptools.RegisterResources(s)`.

All wiring beyond these lines lives in the hand-authored `internal/mcp/resources.go`
(`RegisterResources`, `RefreshHooks`, `RefreshCloneResources`), which reprint does
not overwrite — keep the main-side calls pointing at it.
