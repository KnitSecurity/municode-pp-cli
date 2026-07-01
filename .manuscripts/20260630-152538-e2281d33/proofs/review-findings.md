# Municode CLI — Agentic Review Findings (Phase 4.8 / 4.85 / 4.9 / 4.95)

## Phase 4.95 Local Code Review (hand-written Go) — verdict PASS (no blocking)
Clean: SQL safety (parameterized), drain-first rows, NULL-safe scans, no resource leaks, sequential (no races).
4 warnings — ALL FIXED in-session:
1. diff.go: silent-timeout `mcCollectCode` → removed-section pass falsely flagged unfetched sections as "removed". FIXED: guard removed-pass on `!walkPartial`; set `partial`+note.
2. defs.go: `--limit` registered but discarded. FIXED: dropped the flag (defs returns the one controlling definition by design).
3. xref.go: inbound match `Contains(text,"1-2")` false-matched "1-20"/"1-2-09"/dates. FIXED: added `mcMentionsSection` (full-token section-citation match) + unit test.
4. history.go: `--by-ordinance` prefix-matched (2006-4 → 2006-45). FIXED: exact `EqualFold` only.

## Phase 4.8 / 4.9 SKILL/README/output review — verdict PASS (no blocking)
- Output plausibility PASS: resolve/toc/read return real, clean, well-formed output (no HTML tags/garbage).
- Anti-triggers + trigger phrases PASS.
2 warnings:
1. "version diffing" headline implies the disclaimed paid historical-version feature → FIXED via research.json headline ("change tracking") + final regen propagating to all 8 surfaces.
2. Generated `content` cookbook examples use synthetic UUID node-ids + id/name/status fields (don't match Municode's slug ids). ACCEPTED as documented warning: `content` is a secondary raw-endpoint command; the friendly `read` command (the one users/agents actually use) has correct Municode-id examples. Generator-template cosmetic; routed as retro candidate.

## Phase 4.85 Output review — folded into the 4.8/4.9 output-plausibility pass (PASS).
