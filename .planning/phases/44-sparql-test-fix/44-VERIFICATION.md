---
phase: "44-sparql-test-fix"
verified: 2026-09-28T12:35:00Z
status: passed
score: 7/7 must-haves verified
overrides_applied: 0
overrides: []
re_verification: false
gaps: []
deferred:
  - truth: "Skill hasReadAccess visibility decision (D-SPARQL-05)"
    addressed_in: "v1.6 backlog"
    evidence: "44-DECISIONS.md D-SPARQL-05: deferred to discuss-phase for v1.6 follow-up"
  - truth: "comment predicate hardening (D-SPARQL-07)"
    addressed_in: "v1.6 backlog"
    evidence: "44-DECISIONS.md v1.6 backlog net addition; no SPARQL test exercises comment in this path"
human_verification: []
---

# Phase 44: SPARQL Test Fix Verification Report

**Phase Goal:** Turn 8 failing SPARQL tests green without weakening D-TIER-03 fail-closed.
**Verified:** 2026-09-28T12:35:00Z
**Status:** **PASSED** (7/7 must-haves verified; hermetic gate green under CGO_ENABLED=1)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | All 8 SPARQL tests pass under CGO_ENABLED=1 build | VERIFIED | Runtime test (WSL Ubuntu-22.04, go1.25.5 linux/amd64): 10/10 SPARQL tests pass (`AllWildcard`, `SubjectFilter`, `Pagination`, `Truncated`, `TierFilter`, `FailClosedNoBelongsToTier`, `Filter`, `InvalidLimit`, `LimitCap`, `DefaultLimit`). The originally failing 8 (per Phase 43-01 discovery) are now green. |
| 2 | Store.QueryScoped returns deduped triples on the nil-tier path | VERIFIED | Code at `internal/ontology/store/store.go:503-515`: nil-tier + all-wildcard branch walks `s.all` with `seen := make(map[string]bool, len(s.all))` deduplicating by `S|P|O` key. Regression test `TestQueryScoped_NilTierDedupsDuplicates` passes (insert duplicate → 1 result). Also `TestQueryScoped_NilTierEmptyAllowedTiersDedups`, `TestQueryScoped_NilTierPatternSpecificDoesNotChangeNarrowResults`, `TestQueryScoped_NilTierNoDuplicatesIsUnchanged`. |
| 3 | isSchemaTriple no longer leaks ABox label/comment triples across tier boundaries (D-TIER-03 RESTORED) | VERIFIED | Code at `internal/ontology/store/store.go:626-629`: predicate gate now contains ONLY `domain`, `range`, `subClassOf`, `subPropertyOf`. `label`/`comment` REMOVED. Regression test `TestStore_LabelAndCommentNotSchemaPassthrough` passes (note_authority + note_oracle labels do NOT leak under `["sql-guides"]` filter). Trust boundary verified by `TestSparqlQueryScoped_TierFilter` (note_002, note_003 correctly excluded) and `TestSparqlQueryScoped_FailClosedNoBelongsToTier` (note_003 fail-closed). |
| 4 | Domain/range/subClassOf/subPropertyOf remain always-schema (OWL semantics preserved) | VERIFIED | Code at `internal/ontology/store/store.go:626-629`: predicate gate retains all four OWL/RDFS predicates. Regression test `TestIsSchemaTriple_LabelAndCommentAreABox` verifies `domain`, `range`, `subClassOf`, `subPropertyOf` all return `true`. |
| 5 | No regression in existing Phase 40 store hermetic gate | VERIFIED | Runtime test: `TestQueryScopedSchemaLayerNeverFiltered`, `TestQueryScopedFailClosedNoBelongsToTier`, `TestQueryScopedVisibleEntityAllEdgesIncluded` all PASS. Full Phase 40 hermetic gate (12 TestQueryScoped_* tests including TestQueryScopedABoxFilteredByTier, TestQueryScopedEmptyAllowedTiersNoFilter, TestQueryScopedOneTierOnly, TestQueryScopedMultiTierTable, TestQueryScopedSchemaTriplesAlwaysIncluded) green. |
| 6 | HybridRetrieval gracefully handles missing labels via entity-IRI fallback | VERIFIED | Code at `internal/server/admin_platform.go:441`: `label := entity // default label = entity IRI` — when no label triple is returned by `kg.QueryScoped`, the entity IRI is used as the default label. Runtime test: 9/9 `TestHybridSearchScoped_*` tests pass (OneHop, TwoHops, TierFilter, Rerank, NoRerank, EmptyKGExpansion, DefaultHops, RerankError, MaxHopsCap). |
| 7 | All 7+ test-author assertion miscounts documented with godoc rationale | VERIFIED | Code at `internal/server/admin_platform_test.go:39-114`: comprehensive 75-line godoc block above `fixtureKG()` documents the full 22-count breakdown across all 7 categories. Per-test inline godoc comments cross-reference the block. Plus 2 additional runtime-discovered miscounts (Truncated 2nd 0→2 + FailClosedNoBelongsToTier sql-guides 0→8) documented in `TestSparqlQueryScoped_Truncated` (lines 218-222) and `TestSparqlQueryScoped_FailClosedNoBelongsToTier` (lines 270-279) with godoc rationale. Total: 9 miscounts documented (7 from D-SPARQL-04b + 2 unanticipated per D-SPARQL-08). |

**Score:** 7/7 must-haves verified.

### Deferred Items

Items not yet met but explicitly addressed in later planning.

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | D-SPARQL-05: Skill `hasReadAccess` visibility in admin baseline | v1.6 backlog | 44-DECISIONS.md D-SPARQL-05: deferred to discuss-phase for v1.6 follow-up |
| 2 | D-SPARQL-07: `comment` predicate hardening | v1.6 backlog | 44-DECISIONS.md v1.6 backlog net addition: `comment` has same trust-leak risk as label; no SPARQL test exercises comment in this path |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/ontology/store/store.go` | QueryScoped nil-tier dedup + isSchemaTriple label/comment removal | VERIFIED | 738 lines; lines 492-515 (D-SPARQL-02 dedup); lines 593-602 (godoc: D-SPARQL-03 rationale); lines 619-629 (predicate gate after D-SPARQL-03); original lines 254/260 (SetShaclReport/GetShaclReport) untouched (pre-existing) |
| `internal/ontology/store/store_test.go` | Label-passthrough fail-closed regression test | VERIFIED | 50/50 existing Phase 40 tests pass; no modifications by Phase 44 (added new test files instead) |
| `internal/server/admin_platform_test.go` | SPARQL fixture IRI correctness + duplicate removal + 9 assertion miscount fixes | VERIFIED | 598 lines; lines 39-114 (fixtureKG godoc block); lines 119-122 (IRI fix owl:Class→Class, owl:ObjectProperty→ObjectProperty); line 122 (only 4 schema defs, duplicate at line 58 removed); lines 169-187 (assertion updates for AllWildcard + SubjectFilter); lines 213-229 (Pagination + Truncated); lines 252-258 (TierFilter); lines 268-286 (FailClosedNoBelongsToTier); lines 332-348 (LimitCap + DefaultLimit) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/ontology/store/store.go` (QueryScoped nil-tier path) | `internal/server/admin_platform_test.go` (TestSparqlQueryScoped_*) | Path A 6 tests (22-count) + TierFilter 1 test (15-count) | WIRED | Pattern `if len(allowedTiers) == 0 { ... seen := make(map[string]bool) ... }` verified at store.go:492-515 |
| `internal/ontology/store/store.go` (isSchemaTriple) | `internal/server/admin_platform_test.go` (TestSparqlQueryScoped_TierFilter, FailClosedNoBelongsToTier) | Bucket C 2 tests + 2 trust-boundary assertions | WIRED | Pattern `t.P == ns+"domain"` verified at store.go:626-629 (label/comment removed) |
| `internal/server/admin_platform_test.go` (fixture) | `internal/ontology/store/store.go` (isSchemaTriple schema detection) | owl:Class→Class IRI alignment | WIRED | Pattern `testNS+"owl:Class"` verified removed (replaced with `testNS+"Class"`); rdf:type arm now correctly classifies 4 schema defs |
| `internal/ontology/store/store_label_passthrough_test.go` (regression test) | `internal/ontology/store/store.go` (isSchemaTriple) | Local `testNSLocal` constant (cannot import from package server) | WIRED | Verified: file declares local `testNSLocal = "http://skillforge.local/ontology#"` (lines 28-34); D-SPARQL-06 regression test passes |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `TestSparqlQueryScoped_AllWildcard` | `resp.Count` | `kg.QueryScoped(S, P, O, allowedTiers)` via `SparqlQueryScoped` | YES | Verified: 22 unique triples returned from fixtureKG() with dedup (D-SPARQL-02) |
| `TestSparqlQueryScoped_TierFilter` | `resp.Results` | `kg.QueryScoped("", "", "", ["sql-guides"])` | YES | Verified: 15 triples returned (8 schema + 4 note_001 incl. label + 3 tbl_payment_t incl. label); note_002/note_003 correctly excluded |
| `TestStore_LabelAndCommentNotSchemaPassthrough` | `out`, `out2`, `out3` | `s.QueryScoped("", testNSLocal+"label", "", tiers)` | YES | Verified: empty for tier-filtered (fail-closed), 2 for nil-tier (admin baseline) |
| `HybridSearchScoped` (Phase 41) | `resp.KGNodes[].Label` | `kg.QueryScoped(entity, skfLabel, "", allowedTiers)` with IRI fallback at admin_platform.go:441 | YES | Verified: 9/9 HybridSearchScoped tests pass; IRI fallback present and works |

### Behavioral Spot-Checks (Runtime Verification under WSL Ubuntu-22.04, CGO_ENABLED=1)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 10/10 SPARQL tests pass | `go test ./internal/server/... -run TestSparqlQueryScoped -v -count=1` | All 10 PASS (AllWildcard, SubjectFilter, Pagination, Truncated, TierFilter, FailClosedNoBelongsToTier, Filter, InvalidLimit, LimitCap, DefaultLimit) | PASS |
| 9/9 HybridSearchScoped tests pass | `go test ./internal/server/... -run TestHybridSearchScoped -v -count=1` | All 9 PASS (OneHop, TwoHops, TierFilter, Rerank, NoRerank, EmptyKGExpansion, DefaultHops, RerankError, MaxHopsCap) | PASS |
| 12 Phase 40 store tests pass | `go test ./internal/ontology/store/... -run "TestQueryScoped" -v -count=1` | All 12 PASS (4 new D-SPARQL-02 dedup tests + 8 Phase 40 hermetic gate tests) | PASS |
| 2 label passthrough regression tests pass | `go test ./internal/ontology/store/... -run "TestIsSchemaTriple_LabelAndCommentAreABox\|TestStore_LabelAndCommentNotSchemaPassthrough" -v` | Both PASS | PASS |
| Full hermetic gate green | `go test ./internal/ontology/store/... ./internal/server/... -count=1` | All packages PASS; 16.254s for internal/server | PASS |

### Commit-by-Commit Verification

| Task | Commit | Hash | Claimed | Verified | Files Touched |
|------|--------|------|---------|----------|---------------|
| 1 | fix(44-01) | `7b69f45` | D-SPARQL-02: nil-tier path dedups | VERIFIED | store.go (+23), store_sparql_dedup_test.go (+103, NEW) |
| 2 | fix(44-02) | `2ac4855` | D-SPARQL-03: label/comment ABox | VERIFIED | store.go (+22/-5) — predicate gate narrowed to OWL predicates only |
| 3 | fix(44-03) | `9660177` | D-SPARQL-04: fixture IRI mismatch | VERIFIED | admin_platform_test.go (+4/-4) — 4 single-token edits owl:Class→Class |
| 4 | fix(44-04) | `788a06b` | D-SPARQL-04: fixture duplicate | VERIFIED | admin_platform_test.go (-1) — line 58 duplicate removed |
| 5 | test(44-05) | `49880f0` | D-SPARQL-04b + D-SPARQL-08: 7 assertion miscounts | VERIFIED | admin_platform_test.go (+118/-25) — godoc block + 7 assertion updates |
| 6 | chore(44-06) | `75c9fd1` | Hermetic gate + 2 runtime miscounts | VERIFIED | admin_platform_test.go (+23/-6) — Truncated 2nd 0→2, FailClosedNoBelongsToTier 0→8 |
| 7 | test(44-07) | `5fa5747` | D-SPARQL-06: label/comment passthrough regression test | VERIFIED | store_label_passthrough_test.go (+160, NEW) — 2 regression tests |

**All 7 commits exist with correct prefixes and hashes.**

### New Test Files

| File | Lines | Tests | Status |
|------|-------|-------|--------|
| `internal/ontology/store/store_sparql_dedup_test.go` | 102 | 4 (TestQueryScoped_NilTierDedupsDuplicates, TestQueryScoped_NilTierEmptyAllowedTiersDedups, TestQueryScoped_NilTierPatternSpecificDoesNotChangeNarrowResults, TestQueryScoped_NilTierNoDuplicatesIsUnchanged) | VERIFIED |
| `internal/ontology/store/store_label_passthrough_test.go` | 159 | 2 (TestIsSchemaTriple_LabelAndCommentAreABox, TestStore_LabelAndCommentNotSchemaPassthrough) | VERIFIED |

Both files exist with substantial content (102 and 159 lines respectively) and tests pass at runtime.

### Requirements Coverage

PLAN frontmatter declares `requirements: []` (closure phase, 0 new REQ IDs). The repo has no `.planning/REQUIREMENTS.md` file (file does not exist). Per closure phase contract, no new requirement IDs were introduced.

**Orphaned requirements:** N/A (no REQ IDs mapped to Phase 44).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/ontology/store/store.go` | 254 | `interface{}` (SetShaclReport) | INFO (pre-existing) | Pre-existing lint warning from Phase 40-06 commit `584b58b` — NOT introduced by Phase 44. Out of scope for this closure phase. Recommend follow-up to refactor to `any` (Go 1.18+) |
| `internal/ontology/store/store.go` | 260 | `interface{}` (GetShaclReport) | INFO (pre-existing) | Pre-existing lint warning from Phase 40-06 commit `584b58b` — NOT introduced by Phase 44. Out of scope for this closure phase. Recommend follow-up to refactor to `any` (Go 1.18+) |

**Debt-marker gate:** No `TBD`, `FIXME`, `XXX` markers introduced by Phase 44 in any modified file. PASS.

### Lint Diagnostics Assessment

The 2 lint diagnostics flagged on `store.go` lines 254:39 and 260:34 (`interface{} can be replaced by any`):

**Source investigation:**
- Line 254: `func (s *Store) SetShaclReport(report interface{})` — uses `interface{}` for the SHACL report parameter
- Line 260: `func (s *Store) GetShaclReport() interface{}` — uses `interface{}` for the SHACL report return

**Origin:** Both functions were introduced in commit `584b58b` (Phase 40-06: `feat(40-06): add quarantine state + boot-time SHACL validation`), which is the Phase 40 SPARQL/tier-filtering groundwork.

**Conclusion:** These are **PRE-EXISTING** lint warnings, NOT introduced by Phase 44. Phase 44 only modified lines 492-515 (QueryScoped) and 580-632 (isSchemaTriple + godoc). The SetShaclReport/GetShaclReport functions were not touched by any of the 7 Phase 44 commits.

**Recommendation:** Out of scope for Phase 44 closure. Log a v1.6 follow-up to refactor these functions to `any` (Go 1.18+), as the project migrated from `interface{}` to `any` idioms.

### Human Verification Required

None — all behavioral verification completed programmatically under WSL Ubuntu-22.04 with CGO_ENABLED=1.

## Summary

**All 7 must-haves verified via runtime tests (CGO_ENABLED=1, WSL Ubuntu-22.04, go1.25.5 linux/amd64) + code inspection.**

- **Goal:** Turn 8 failing SPARQL tests green without weakening D-TIER-03 fail-closed — ACHIEVED
- **All 10/10 SPARQL tests pass** (originally 8 failing, plus 2 already-green Filter/InvalidLimit)
- **D-TIER-03 fail-closed semantics RESTORED** (label/comment leak fixed; verified by `TestSparqlQueryScoped_TierFilter` + `TestSparqlQueryScoped_FailClosedNoBelongsToTier`)
- **OWL semantics preserved** (domain/range/subClassOf/subPropertyOf remain always-schema)
- **Phase 40 hermetic gate green** (12 TestQueryScoped_* tests pass; no regressions)
- **HybridRetrieval graceful degradation verified** (IRI fallback at admin_platform.go:441; 9/9 HybridSearchScoped tests pass)
- **9 assertion miscounts documented with godoc rationale** (7 from D-SPARQL-04b + 2 unanticipated runtime discoveries per D-SPARQL-08)
- **All 7 commits present with correct prefixes** (7b69f45, 2ac4855, 9660177, 788a06b, 49880f0, 75c9fd1, 5fa5747)
- **2 new test files created** (store_sparql_dedup_test.go, store_label_passthrough_test.go) with non-zero content and passing tests

**Lint diagnostics at store.go:254/260 are PRE-EXISTING** (from Phase 40-06 commit `584b58b`), not introduced by Phase 44. Recommend v1.6 follow-up.

**Status: PASSED** — Phase 44 goal achieved. v1.5 milestone SHIPPED gate cleared (pending operator 42-03 runbook execution per STATE.md § Phase 44).

---

_Verified: 2026-09-28T12:35:00Z_
_Verifier: Claude (gsd-verifier)_
_Build env: WSL Ubuntu-22.04, go1.25.5 linux/amd64, gcc 11.4.0, CGO_ENABLED=1_