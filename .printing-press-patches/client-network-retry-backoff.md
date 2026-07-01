# Reprint guard: back off before retrying transient network errors

**Target (generated):** `internal/client/client.go` (the `do`/`doInternal` retry loop)
**Companion test (hand-authored):** `internal/client/client_network_retry_test.go`

## Intent

The client already retries 429 and 5xx with exponential backoff, and retries
network-level failures (`HTTPClient.Do` returning an error: connection reset, DNS
blip, request timeout). But the network-error branch retried with a **bare
`continue` and no backoff** — three retries burned out in a tight loop within
milliseconds, so a brief outage mid-walk aborted a long-running `clone`. That
undermines the set-and-forget clone (a single flaky request must not kill a
multi-thousand-section clone).

On reprint, re-apply the backoff or the tight-loop behavior returns.

## Edit to re-apply

In the `HTTPClient.Do(req)` error branch (after the `ctx.Err()` early return and
setting `lastErr`), before `continue`, add the same exponential backoff the 5xx
branch uses:

```go
if attempt < maxRetries {
    wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
    fmt.Fprintf(os.Stderr, "network error, retrying in %s (attempt %d/%d)\n", wait, attempt+1, maxRetries)
    if serr := sleepContext(ctx, wait); serr != nil {
        return nil, 0, serr
    }
}
```

`math`, `time`, `os`, and `sleepContext` are already used in the file. Backoff
schedule is 1s, 2s, 4s (matching the 5xx path); `sleepContext` returns the ctx
error on Ctrl-C so cancellation still breaks out immediately.

## Upstream

This is a framework-level fix and belongs upstream in CLI Printing Press's client
template (per AGENTS.md, systemic fixes are upstream fixes first). Until that
lands, this guard keeps the intent across local reprints.
