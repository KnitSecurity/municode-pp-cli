# Reprint guard: register the `clones` command on the CLI root

**Target (generated):** `internal/cli/root.go`
**Feature:** local-clone offline inventory (plan unit U3)

## Intent

`clones` is a hand-authored, offline-only command (`internal/cli/clones.go`,
`newClonesCmd`) that lists the municipalities present in the local clone store.
The command file survives reprint (it is hand-authored), but the generated root
command does not know to mount it. On reprint, re-add the registration or the
command becomes unreachable.

## Edit to re-apply

In `RootCmd()` (or wherever sibling commands are added to `rootCmd`), add:

```go
rootCmd.AddCommand(newClonesCmd(flags))
```

alongside the other `rootCmd.AddCommand(...)` calls. No other change to `root.go`
is required; the command's behavior lives entirely in the hand-authored file.
