# Municode — Novel Features Brainstorm (audit trail)

Subagent: general-purpose, 3-pass (customer model → candidates → adversarial cut). First print (no prior research). Brief had `## Codebase Intelligence` (source f eligible) but no `## User Vision` (source e off).

## Customer model

- **Dana — land-use paralegal** (regional zoning firm, ~12 GA/FL client cities). Weekly: re-verifies zoning/setback language across 3 cities, copies exact section text + citation into memos, confirms current codification supplement. Frustration: no side-by-side topic comparison across cities, no trustworthy search, ordinance-history line must be transcribed by hand.
- **Marcus — investigative reporter** (housing policy). Weekly: monitors cities for newly adopted ordinances, reconstructs which sections each rewrote. Frustration: site shows only current code; supplements are dated but no diff exists.
- **Priya — civic-tech developer** (open-data mirror). Weekly: re-runs sync jobs, spot-checks mirrors, tries to detect which cities re-codified. Frustration: nothing flags which mirrors are stale vs upstream latest job; paid MuniPro gate blocks upstream search for QA.
- **Tom — code-enforcement officer / clerk support**. Weekly: repeated defined-term and controlling-section lookups with exact citation for enforcement notices. Frustration: wants the one authoritative definition/section, not a 50-hit list; free-tier search broken.

## Survivors (6, all hand-code, >=5/10)

| # | Feature | Command | Score | How it works |
|---|---------|---------|-------|--------------|
| 1 | Cross-version codification diff | `diff` | 9/10 | Join `documents` across two `jobs`, hash section text by node/heading → added/removed/changed citations |
| 2 | Cross-city topic comparison | `compare` | 9/10 | FTS5-match a topic across `documents` from multiple synced `clients`, align top section per city; `--state` widens |
| 3 | Ordinance-history extraction | `history` | 8/10 | Regex "(Ord. No., §, date)" from `documents.text`; `--by-ordinance` inverts the index |
| 4 | Intra-code cross-reference graph | `xref` | 7/10 | Regex "see § X"/"Chapter Y" refs joined to `toc_nodes` → inbound/outbound edges |
| 5 | Defined-term lookup | `defs` | 8/10 | FTS5 scoped to Definitions-family headings → defining clause + citation |
| 6 | Mirror freshness check | `stale` | 8/10 | Re-call `/Jobs/latest` per synced (clientId,productId), diff vs stored jobId |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| locate | Thin re-rank of framework `search` | defs |
| cite | Wrapper — breadcrumb already absorbed (`read --citation`); permalink is a fixed template | xref |
| families | Low pain; re-presents TOC already in `toc` | compare |
| survey | Subset of `compare` output; folded into `compare --state` | compare |
| outline | Thin re-presentation of `toc` | stale |
| ordinance | Same engine as `history`; folded into `history --by-ordinance` | history |
