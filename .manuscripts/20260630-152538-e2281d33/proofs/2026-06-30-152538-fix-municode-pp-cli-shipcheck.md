# Municode CLI — Shipcheck Report

## Final verdict: PASS (7/7 legs), scorecard 86/100 Grade A → ship

| Leg | Result |
|---|---|
| verify | PASS (100%, 26/26, 0 critical) |
| validate-narrative | PASS (10 narrative commands + full examples) |
| dogfood | PASS (7/7 novel features; 0 dead flags; 1 dead-func WARN) |
| workflow-verify | workflow-pass |
| verify-skill | PASS (after Use-string fix) |
| scorecard | PASS — 86/100 Grade A |

## Blockers found & fixed
1. **search hit dead upstream `/search`** (paid MuniPro, 0 hits) → removed the `remote` search endpoint from the spec and regenerated; framework `search` is now local-FTS-first.
2. **clone errored on timeout** → made the BFS walk return partial results on deadline; clone uses a 15-min budget, is resumable, and reports `partial`.
3. **diff infeasible as two-supplement diff** (paid CodeBank, 401) → user-approved reframe to clone-vs-live drift detection.
4. **2 parser bugs** (nested-paren ordinances, double whitespace) caught by unit tests → fixed.
5. **verify-skill: 16 positional-arg errors** from `<City, ST>` placeholders in `Use:` (comma/space broke the parser) → switched to single-token `<city>` placeholders.

## Behavioral correctness (tested live)
resolve, cities, toc, read, clone (1162 sections for Acworth), search (local FTS), defs, history (structured ord rows), xref (outbound/inbound), compare (aligned per city), stale (live job check) — all return correct, relevant output.

## Known scope (disclosed & user-approved)
- Verbatim superseded-version text (paid Municode CodeBank, 401 anonymous) is out of scope. `clone` = current text + full ordinance lineage; `diff` = drift-vs-live.

Final ship recommendation: **ship**
