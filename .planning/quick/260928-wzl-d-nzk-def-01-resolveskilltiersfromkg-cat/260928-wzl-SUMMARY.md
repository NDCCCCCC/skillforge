---
task: quick-260928-wzl
title: D-NZK-DEF-01 — resolveSkillTiersFromKG normalization (cat_-shaped hasReadAccess)
date: 2026-09-28
status: complete
commits:
  - "71fe2b7 test(quick-260928-wzl): live-wiring regression (RED)"
  - "76b5290 fix(quick-260928-wzl): normalize hasReadAccess objects (GREEN)"
tags: [ontology, sparql, tier-isolation, D-TIER-03, fail-closed, regression-test]
---

# Quick 260928-wzl Summary: D-NZK-DEF-01 — resolveSkillTiersFromKG 归一化

**One-liner:** Normalized `hasReadAccess` object resolution (Tier-individual direct → keep; KBCategory → expand through `belongsToTier`; unknown → skip fail-closed), restoring live SPARQL ABox visibility for every skill (entity rows 0 → 65) while preserving fail-closed isolation (calculator: 0 ABox rows).

## Root Cause (as verified, fix applied)

Live KG wires skills as `skill_sql-guides hasReadAccess → cat_sql-guides` (KBCategory individual), tier membership on the category's `belongsToTier → sql-guides` edge. The resolver passed the raw object local name through `strings.TrimPrefix`, yielding tier `"cat_sql-guides"`, which `Store.QueryScoped` never matches against `belongsToTier` objects (Tier IRIs) → `visibleSubjects` empty → every ABox row fail-closed filtered for **all** skills. Phase 44's hermetic suite stayed green because tests hand-fed `allowedTiers: ["sql-guides"]` (`admin_platform_test.go:136`) and never exercised the live resolver wiring.

## Fix

`internal/server/admin_platform.go` — `resolveSkillTiersFromKG`: per access object O:

1. `(O, rdf:type, skf:Tier)` → keep stripped local name (legacy direct-Tier wiring backward compatible)
2. `(O, skf:belongsToTier, tierIRI)` → stripped tierIRI local name (KBCategory expansion; multi-tier categories expand to every tier)
3. neither → skipped; all-unknown → **nil** so callers 401

Nil-result (skill not found) semantics unchanged. Result deduped preserving first-seen order. New consts `skfTier` + `skfBelongsToTier`.

**Hardening note (fail-open guard):** the resolver can never return an empty **non-nil** slice — `Store.QueryScoped` treats empty `allowedTiers` as the unfiltered admin baseline, so an empty slice here would have failed OPEN at the handler level. Pinned by `TestResolveSkillTiersFromKG_FailClosedOnUnknownAccessObject`.

## Regression Tests (RED → GREEN)

New file `internal/server/admin_platform_def01_test.go` (package `server`, 6 tests, fixture mirrors the LIVE `cat_` wiring shape — the exact gap Phase 44 missed):

| Test | Pins |
|------|------|
| `LiveCategoryWiring` | cat_sql-guides expands to `["sql-guides"]`, raw `cat_` name never a tier |
| `DirectTierObjectBackwardCompat` | legacy Tier-object wiring unchanged |
| `UnknownSkillNil` | nil semantics preserved (incl. nil OntologyDeps) |
| `FailClosedOnUnknownAccessObject` | all-unknown → nil (never empty non-nil) |
| `DedupesPreservesOrder` | multi-path tiers dedupe, first-seen order |
| `SparqlQueryScoped_LiveCategoryWiring_ABoxVisible` | resolver output restores ABox visibility end-to-end through QueryScoped |

RED confirmed pre-fix (71fe2b7): resolver returned `["cat_sql-guides"]`, `note_live` invisible.

## Verification Gates

**Gate 1 — Hermetic (WSL Ubuntu-22.04, CGO_ENABLED=1, go1.25.5): PASS**

- `go build ./...` OK; `go test ./internal/server/... ./internal/ontology/... -count=1` all ok
- 6/6 new DEF-01 tests pass; Phase 44's 10 SPARQL tests all pass (they hand-feed tiers — resolver change does not affect `SparqlQueryScoped` itself, confirmed); 9/9 HybridSearchScoped tests pass (Phase 41 regression)

**Gate 2 — LIVE E2E (rebuilt `~/skillforge-server-44` with CGO, restarted via `sf44-restart.sh`, pid 2687 listening on `[::]:8888`): PASS**

| Probe | Before fix | After fix |
|-------|-----------|-----------|
| `X-Skill-Name: sql-guides`, `p=rdf:type o=InsuranceProduct` | count=23 (schema passthrough only, 0 ABox) | **count=88, 65 InsuranceProduct entity rows** (`ent_*`) |
| `X-Skill-Name: sql-guides`, `s=note_033` | 0 note rows | **3 rows**: `type NoteChunk` + `belongsToTier sql-guides` + `referencesTable tbl_etl_policy` |
| `X-Skill-Name: calculator` (no sql-guides access), same entity pattern | 23 (schema) | **23 — 0 ABox rows. Fail-closed HOLDS.** |
| `X-Skill-Name: calculator`, `s=note_033` | — | **0 note_033 rows** (23 schema only) |

**Admin KG query cross-check (`GET /api/admin/ontology/kg/query`, X-Admin-Token, unfiltered baseline): PASS**

- InsuranceProduct: admin 65 = skill-scoped 65 (exact match)
- note_033: admin 3 = skill-scoped 3 (exact match)

**Gate 3 — Stale process killed before rebuild/restart: PASS** (`pkill -x skillforge-serv`, port 8888 released in 1s, fresh boot log clean).

### Operational nuances observed (not defects of this fix)

- **limit=20 pagination nuance:** ~23 schema rows short-circuit ahead of the S/P/O pattern filter (documented Phase 44 D-TIER-03 behavior), so a `limit=20` first page is schema-only (`count=88, truncated=true`); entity rows sit past the schema wall. The task gate's `limit:20` body proves restoration via `count 23 → 88`; the >0-entity-row assertion was executed at `limit=200` (65 rows). Probes with small limits must page or raise the limit.
- `sf44-wait.sh` probes `/healthz` (404 — route doesn't exist) and tails the stale `/tmp/sf44.log` (start script now writes `~/sf44.log`). Cosmetic script drift for a future touch-up; the ss/log lines still confirm boot.
- wsl.exe/MSYS quoting mangles `$(...)` / `$VAR` expansion in inline `bash -c` strings — live-gate probes were run via temp script files with `MSYS_NO_PATHCONV=1` (deleted after use).

## Deviations from Plan

**1. [Rule 3 - Blocking] wsl inline-command quoting failures during live gate**
- **Found during:** Gate 2/3 execution
- **Issue:** multi-line `$(...)`/env expansion inside `wsl -- bash -c '...'` produced empty `SKILLFORGE_ADMIN_TOKEN` and mangled `/proc` paths
- **Fix:** temp script files under `.planning/quick/` executed with `MSYS_NO_PATHCONV=1`; deleted after gates
- **Files:** none committed (temp helpers only)

**2. [Scope discipline] Sibling resolver `(*Deps).resolveSkillTiers` NOT touched**
- `internal/server/admin_ontology.go:129` has the identical cat_-shaped defect feeding the admin KG query panel's `?skill=` param. Left as-is: admin-token-gated surface (admin can already see everything unfiltered), fails in the hide-not-leak direction, and outside this task's authoritative fix spec. **Recommended follow-up:** apply the same normalization (the fix body is directly portable) — candidate to fold into the D-SPARQL-05 v1.6 follow-up.
- Note: the admin cross-check gate above used the **unfiltered** admin path (no `?skill=`), so the exact-match evidence is unaffected by this deferred sibling.

**3. [Scope discipline] Admin `?skill=` panel behavior unchanged** — admin selecting a skill in the KG query panel still sees schema-only ABox rows for that skill (same sibling defect). Documented, not fixed.

## Threat Surface

No new surface. The change tightens correctness of the existing D-TIER trust-boundary resolver: unknown access objects now contribute no tier (fail-closed preserved and now unit-pinned), and the empty-slice→admin-baseline fail-open path is structurally unreachable.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/server/admin_platform_def01_test.go (committed, 71fe2b7)
- FOUND: internal/server/admin_platform.go fix (committed, 76b5290; no deletions in commit)
- FOUND: git log contains 71fe2b7 and 76b5290 on phase-20-p10-6-admin-console
- FOUND: live gate evidence above (65 entity rows / note_033 3 rows / calculator 0 ABox / admin exact-match)

## Metrics

- Duration: ~17 min (1039s)
- Commits: 2 (atomic RED → GREEN)
- Files: 1 created (test), 1 modified (resolver)
- Tests: 6 added, all green; full server+ontology hermetic gate green
