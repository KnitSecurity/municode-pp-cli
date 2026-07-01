# Municode CLI — Phase 5 Acceptance Report (Full Dogfood)

Level: Full Dogfood (live api.municode.com, Boulder, CO happy-args, read-only)
Tests: 82/82 passed, 55 skipped (auth-gated/no-op), 0 failed.
Gate: PASS

## Failures found & fixed inline (2 dogfood rounds)
1. **diff/history error_path** (CLI fix): malformed city arg returned graceful exit 0; now validates "City, ST" format → exit 2. Well-formed-not-cloned still exits 0 gracefully.
2. **sync happy_path/json_fidelity** (Printing Press issue → worked around): framework `sync` couldn't key Municode's PascalCase `StateID`/`ClientID`/`ProductID`; `genericIDFieldFallbacks` doesn't cover `<Type>ID`. Added `resourceIDFieldOverrides` for states/clients/products/codestoc → `sync states` now stores 51, exit 0. **Retro:** internal-YAML specs should support a per-resource `id_field`, and the fallback list should try `<Type>ID`/`<Type>Name`.
3. **versions happy_path** (CLI fix): the promoted command's `Example` referenced the pre-rename `jobs latest 10376` (stale — I authored it before renaming the resource to `versions`); dogfood couldn't build a runnable example. Fixed the example (and the spec) to `versions 10376`.

## Cross-library generalization fix (from user request to test Boulder, CO)
4. **history/ord_history parser** (CLI fix): Boulder uses spelled-out "Ordinance No. 4705" in footnote prose, not the abbreviated "(Ord. No. ...)" trailing parenthetical of Atlanta/Acworth. The parser missed Boulder's format → empty ord_history. Rewrote `mcParseOrdHistory` to handle both styles (scan-based, matches "Ord."/"Ordinance No." and "Code YYYY"/"YYYY Code"). Validated end-to-end: Boulder Chapter 1 now extracts Ord. 4705/3838, Code 1925. Added a Boulder-format unit test; Atlanta test still passes.

## Behavioral spot-checks (Boulder, CO — a different library than Acworth/Atlanta)
resolve (client 1357, "Municipal Code" product), toc (Boulder Titles/Charter), read (clean Title 1 text), clone (147 sections full), search (local FTS, real hits), defs (Definitions section), history (ordinance lineage), xref, compare, stale — all correct.

## PII: none — municipal codes are public records; no user/account data involved.
