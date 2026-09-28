# Phase 44: SPARQL Test Fix - Context

**Gathered:** 2026-09-28
**Revised:** 2026-09-28 (Bucket B claim corrected per plan-checker verdict)
**Status:** Ready for planning (pure closure)
**Milestone:** v1.5 — Outstanding Closure (NOT New Features)
**Discovered by:** Phase 43-01 CGO build verification (2026-09-28)

<domain>
## Phase Boundary

Phase 41 SPARQL endpoint shipped **without passing its own test suite** — 8 tests in `internal/server/admin_platform_test.go` failed under the CGO build that v1.4 never exercised (`CGO_ENABLED=0` masked this during normal development). Phase 43-01 finally brought the CGO build green, which surfaced the latent failures. This phase closes them.

**Three independent root causes** (per `44-RESEARCH.md`):

| Bucket | Cause | Tests affected | Fix location |
|--------|-------|----------------|--------------|
| **A** | `Store.Query` and the nil-tier early-return in `QueryScoped` have no dedup. Fixture inserts duplicate `(testNS+"Tier", rdfType, owl:Class)` at admin_platform_test.go:58 (identical to line 49). | 6 — `AllWildcard`, `Pagination`, `Truncated` (both branches), `LimitCap`, `DefaultLimit` (all expect 16, get 22 unique after fix) | `internal/ontology/store/store.go:492-494` |
| **B** | `TestSparqlQueryScoped_SubjectFilter` (S=note_001, nil allowedTiers) asserts `Count == 3` but `note_001` has 4 unique triples (rdf:type + belongsToTier + label + referencesTable). The test's inline comment at line 106 already says 4 — the `3` in the assertion is a typo/miscount. **NOTE (REVISION):** This test uses `nil` allowedTiers, so `QueryScoped` early-returns to `s.Query(note_001, "", "")` which NEVER consults `isSchemaTriple`. Bucket C's fix does NOT change the count — the assertion update is required independently. | 1 — `SubjectFilter` | `internal/server/admin_platform_test.go:106,107` (assertion update only, no code change) |
| **C** | `isSchemaTriple` (store.go:585-587) treats `label`/`comment`/`domain`/`range`/`subClassOf`/`subPropertyOf` predicates as **always schema** regardless of subject. This leaks `note_002` (authority tier) and `note_003` (no tier) `label` triples through `["sql-guides"]` tier filtering — **a real D-TIER-03 trust-boundary violation** masked by Phase 40 governance using `referencesTable` predicate (not affected by label leak). | 2 — `TierFilter` (expects 13, gets 15 after fix; negative assertions on note_002/note_003 labels STRONGER after fix), `FailClosedNoBelongsToTier` sql-guides branch (expects 0, gets 0 after fix) | `internal/ontology/store/store.go:585-587` |

**Path C — recommended fix**: both implementation AND fixture changes.

| Change | File | Lines | Rationale |
|--------|------|-------|-----------|
| Add dedup on nil-tier path | `internal/ontology/store/store.go` | 492-494 | `Insert` deliberately no-dedup (audit-trail-preserving); dedup belongs at query layer for admin baseline consistency |
| Remove `label`/`comment` from always-schema predicate gate; keep `domain`/`range`/`subClassOf`/`subPropertyOf` (OWL semantics) | `internal/ontology/store/store.go` | 585-587 | Restores D-TIER-03 fail-closed semantics; schema-subject label triples still pass via the `rdf:type skf:Skill` arm |
| Fix fixture IRI mismatch | `internal/server/admin_platform_test.go` | 46-49 | Replace `testNS+"owl:Class"` with `testNS+"Class"` (3x), `testNS+"owl:ObjectProperty"` with `testNS+"ObjectProperty"` (1x). Production TBox emitter uses `skf:Class`, not `owl:Class` |
| Remove duplicate fixture triple | `internal/server/admin_platform_test.go` | 58 | Delete the duplicate `(testNS+"Tier", rdfType, owl:Class)` (already covered by line 49) |
| Update 7 test-author assertion miscounts (D-SPARQL-04b extended + D-SPARQL-08) | `internal/server/admin_platform_test.go` | 94, 106-107, 121, 132, 140, 158, 224, 226, 236, 238 | Path A 16→22 (6 tests); SubjectFilter 3→4; TierFilter 13→15. See 44-01-PLAN.md Task 5 godoc block for full breakdown. |

**REVISION NOTE (2026-09-28, plan-checker BLOCK verdict):**

The previous version of this document claimed Bucket B (SubjectFilter 3→4) "Resolved as side-effect of Bucket C fix: with `label` no longer auto-schema, `note_001` returns 3 ABox triples, matching the existing assertion. No code change needed."

**This claim was WRONG.** The `SubjectFilter` test passes `nil` allowedTiers (line 104), so `QueryScoped` early-returns to `s.Query(note_001, "", "")` (store.go:286-289) which returns ALL 4 note_001 triples without consulting `isSchemaTriple` at all. Removing `label` from `isSchemaTriple`'s predicate gate (Bucket C fix) has ZERO effect on this path — it only affects Path B (tier-filtered queries). The assertion update from 3→4 is **required independently** of the Bucket C fix.

The revised Bucket B row above and the updated test plan (Task 5 row 2 in 44-01-PLAN.md) reflect this correction.

## What's NOT in scope

- **No new features.** Zero new API surface, zero new tests beyond regression coverage of these 8 fixes.
- **No `SPARQL 1.1` parser** (deferred to v1.6+ per Phase 41 backlog).
- **No skill `hasReadAccess` visibility decision** (D-SPARQL-05 deferred to discuss-phase follow-up; current implementation treats `hasReadAccess` as ABox, which matches Phase 40 `TestQueryScopedSchemaLayerNeverFiltered` line 571-590 scope).
- **No weakening of fail-closed semantics** (D-IN-01) — `D-TIER-03` is *restored*, not relaxed.
- **No SPARQL tier-filter behavior changes beyond what Bucket C requires.**

</domain>

<decisions>
## Implementation Decisions (Phase 44)

### Closure philosophy
- **D-SPARQL-01:** Phase 44 is pure closure. 0 new features. Per-fix atomic commits following Phase 42-02 pattern (one commit + regression test per fix).
- **D-SPARQL-02:** `Store.QueryScoped` with `allowedTiers=nil/empty` MUST dedup by `S|P|O` before returning. Closes Bucket A (6 tests: AllWildcard/Pagination/Truncated/LimitCap/DefaultLimit). Implementation = extend the early-return at `store.go:492-494` to walk `s.all` with a `seen` map when all of `subj,pred,obj` are empty; otherwise delegate to `s.Query` (which already has index-driven narrow returns that cannot dedup, but pattern-specific queries rarely have raw duplicates).
- **D-SPARQL-03:** `isSchemaTriple` predicate gate for `label`/`comment` is REMOVED. Labels are schema only when the subject is a schema individual (already covered by the `rdf:type`+`schemaClasses[O]` arm, lines 579-581). Keep `domain`/`range`/`subClassOf`/`subPropertyOf` (OWL semantics — these predicates legitimately only apply to schema definitions). Closes Bucket C (2 tests) AND restores D-TIER-03 fail-closed semantics. Reaffirms Phase 40-04 decision (ROADMAP.md:196).
- **D-SPARQL-04:** Fixture cleanup is non-semantic: fix `owl:Class`→`Class` IRI mismatch at admin_platform_test.go:46-49 and remove duplicate at line 58. After these fixes, `fixtureKG()` produces 22 unique triples (verified line-by-line in 44-RESEARCH.md §2). Closes Bucket A count mismatch when combined with D-SPARQL-02 dedup.
- **D-SPARQL-04b (REVISION EXTENDED):** **7** test-author assertion miscounts MUST be updated to match the actual fixture triple count after all code/fixture fixes:
  - `AllWildcard` line 94, 96: `16 → 22` (Path A returns all 22)
  - `SubjectFilter` line 106, 107: `3 → 4` (Path A with S=note_001 returns 4; **independent of Bucket C fix**)
  - `Pagination` line 121: `16 → 22` (Path A total)
  - `Truncated` line 132 (1st): `16 → 22`
  - `Truncated` line 140 (2nd): `16 → 22`
  - `TierFilter` line 158: `13 → 15` (Path B with sql-guides; 8 schema + 7 ABox)
  - `LimitCap` line 224, 226: `16 → 22` (Path A returns all 22)
  - `DefaultLimit` line 236, 238: `16 → 22` (Path A returns all 22)
- **D-SPARQL-05 (deferred):** Whether skill `hasReadAccess` triples are "schema layer visible to admin baseline" or "skill metadata excluded from SPARQL response". Phase 40 locked decision does not explicitly cover it; current implementation treats them as ABox (subject not in `visibleSubjects` set, predicate not in schema list). Path C as drafted excludes them from admin baseline. If maintainer wants them visible, switch to a subject-based schema detector (Option α). Out of scope for Phase 44 — defer to discuss-phase for v1.6 follow-up if surfaced.
- **D-SPARQL-06:** Phase 40 governance gate caught what it could (used `referencesTable` predicate, not affected by label leak); the SPARQL endpoint exposed the label-passthrough regression. A new regression test MUST be added to lock the fix. Per Revision 4, the new test file `store_label_passthrough_test.go` (in `package store`) declares a local `const testNS = "http://skillforge.local/ontology#"` because `testNS` is defined in `package server` (admin_platform_test.go:30) and cannot be imported.
- **D-SPARQL-08 (NEW — REVISION):** **Test-author assertion miscounts are valid closure-phase scope** when the assertion contradicts the actual fixture triple count (verified line-by-line in 44-RESEARCH.md §2). The correct fix is to update the assertion with godoc rationale documenting the count derivation. Do NOT silently change implementation behavior to match a miscounted assertion. This decision formalizes the policy underlying D-SPARQL-04b and prevents future cycles of "fix the assertion back to match buggy code".

### Fix ordering

| Priority | Fix | Risk | Commit prefix |
|----------|-----|------|---------------|
| 1 | D-SPARQL-02 Store.QueryScoped nil-tier dedup | Low — additive dedup | `fix(44-01)` |
| 2 | D-SPARQL-03 isSchemaTriple label/comment removal | Medium — semantic restoration of D-TIER-03 | `fix(44-02)` |
| 3 | D-SPARQL-04 Fixture IRI mismatch | Low — single-token edits | `fix(44-03)` |
| 4 | D-SPARQL-04 Duplicate fixture triple removal | Low | `fix(44-04)` |
| 5 | D-SPARQL-04b + D-SPARQL-08 7 test-author assertion corrections | Low — recorded rationale, single commit | `test(44-05)` |
| 6 | D-SPARQL-06 Hermetic gate (all 8 tests green + no regressions in store/governance/hybrid) | Gate, not fix | `chore(44-06)` |
| 7 | D-SPARQL-06 Label-passthrough fail-closed regression test (with local `testNS` per Revision 4) | Low — new test file | `test(44-07)` |

### Hard constraints (v1.5 closure rules)

- **Per-fix atomic commits** (per Phase 42-02 pattern): one commit + regression test per fix where applicable.
- **Trust semantics preserved**: T-3-AUDIT INSERT-only / KNOW-02 fail-closed / Tier 隔离 (D-TIER-03 RESTORED, not weakened).
- **No `Insert` dedup**: audit-trail-preserving contract preserved; dedup is a query-layer concern.
- **Test expectations aligned with actual fixture count**: per D-SPARQL-08, assertions MUST be updated (with godoc rationale) when they contradict the verified fixture triple count. "Preserve miscounted assertions" is NOT a hard constraint — implementation correctness is.

### Claude's Discretion

- Implementation idiom for the dedup walk (e.g., new helper function `dedupAllTriples()` or inline; both are acceptable).
- Exact regression-test file naming (`store_sparql_dedup_test.go` vs `store_query_dedup_test.go` vs adding to existing `store_test.go`).
- Whether to also remove the duplicate fixture line via `s.InsertUnique(...)` instead of literal deletion (functionally identical; pick whichever is more readable).
- Exact wording of godoc block above fixtureKG (the canonical "fixture triple inventory" block in Task 5 of 44-01-PLAN.md is the recommended structure).

</decisions>

<canonical_refs>
## Canonical References

### Research
- `.planning/phases/44-sparql-test-fix/44-RESEARCH.md` — full static-trace root cause analysis (math table redriven per Revision 3)
- `.planning/STATE.md` § "v1.5 Phase 43 Discovery" — 8 SPARQL failures list

### Phase 40 trust semantics (LOCKED)
- `.planning/ROADMAP.md:196` — `D-TIER-01/03/06/07` definitions
- `internal/ontology/store/store.go:487-566` — `QueryScoped` source-of-truth
- `internal/ontology/store/store.go:568-592` — `isSchemaTriple` source-of-truth
- `internal/ontology/store/store_test.go:569-744` — Phase 40 hermetic gate (must remain green after Phase 44)

### Phase 41 SPARQL endpoint
- `internal/server/admin_platform.go:189-252` — `SparqlQueryScoped` shared function
- `internal/server/admin_platform_test.go:42-84` — failing fixture
- `.planning/phases/41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval/41-02-SUMMARY.md` — SPARQL ship record (10 unit tests, 1 integration gap = CGO mask)

### Phase 42 closure patterns (template for Phase 44)
- `.planning/phases/42-v1-4-outstanding-closure/42-02-PLAN.md` — per-fix atomic commit pattern
- `.planning/phases/42-v1-4-outstanding-closure/42-decisions.md` — closure philosophy reference

### Environment constraints
- `env-wsl-backend-runbook` — WSL Go build path; CGO_ENABLED=1 requires Linux gcc with emutls ABI (Phase 43-01 path)
- `env-intranet-service-flakiness` — hermetic-only gate (no live LLM/Milvus dependency in Phase 44)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/ontology/store/store.go:517-524` — existing `seen` dedup pattern in tier-filtered path; Phase 44 D-SPARQL-02 mirrors this idiom for the nil-tier path
- `internal/ontology/store/store.go:579-581` — `schemaClasses[O]` subject-based schema detection (the path that makes label-of-schema-individual work via the rdf:type arm)
- `internal/ontology/store/store_test.go:615-634` — `TestQueryScopedFailClosedNoBelongsToTier` (existing) uses `referencesTable` predicate, NOT label — that's why Phase 40 gate didn't catch the leak
- `internal/knowledge/ecl/abox.go::BuildTriples` (per `STATE.md` § Decisions) — emits `label` triples for notes; Phase 44 fix changes their visibility under tier filter (CORRECT behavior)

### Established Patterns
- Per-fix atomic commit (D-IN-02 from Phase 42)
- Insert no-dedup, query dedupes (audit-trail-preserving contract per `store.go:117-150`)
- `isSchemaTriple` short-circuits before subject visibility check (`store.go:527-531`)

### Integration Points
- Phase 44 touches only `internal/ontology/store/store.go` (QueryScoped + isSchemaTriple) and `internal/server/admin_platform_test.go` (fixture + assertions).
- No contract changes (no public API changes).
- No DB schema changes.
- No frontend changes.
- Hybrid Retrieval (admin_platform.go:348-547) calls `kg.QueryScoped(...)` for label and tier collection. Removing label from `isSchemaTriple` means label triples for non-visible ABox subjects will NOT be returned by these calls — **correct behavior** (visible entities only). The current code (admin_platform.go:441) falls back to entity IRI as label when no label triple is found, so FE behavior is graceful.

### Files NOT touched
- `internal/knowledge/ecl/manager.go`, `internal/server/admin_pipeline.go` — no changes
- `cmd/server/main.go` — no changes
- `internal/retriever/milvus/*` — no changes
- Frontend (`web/`) — no changes
- Any other Phase 38-43 source files — no changes

</code_context>

<specifics>
## Specific Notes

- The `cgo.exe` Windows toolchain in this sandbox exits status 2 (per `44-RESEARCH.md` §1); runtime test invocation requires the Phase 43-01 build environment (Linux gcc with emutls ABI). The plan's hermetic gate MUST be executed by the operator in that environment.
- **`TestSparqlQueryScoped_SubjectFilter` REVISION (REPLACES previous note):** The previous version of this note claimed "After D-SPARQL-03, `note_001` returns 3 ABox triples (rdf:type + belongsToTier + referencesTable; label no longer schema), matching the existing `Count == 3` assertion — **no assertion change needed**." This claim was WRONG. The test passes `nil` allowedTiers (line 104), so `QueryScoped` early-returns to `s.Query(note_001, "", "")` which returns ALL 4 note_001 triples without ever consulting `isSchemaTriple`. The Bucket C fix has ZERO effect on this test path. The assertion update from 3→4 is **required independently**. See 44-01-PLAN.md Task 5 row 2 for the correction.
- **`TestSparqlQueryScoped_TierFilter` REVISION (unchanged from previous version, confirmed):** After D-SPARQL-03, label triples for subject-visible ABox entities ARE included (D-TIER-06: "if a subject is visible, ALL its edges are included"). The fixture has:
  - `note_001` label "SQL指南第一章" — subject visible → INCLUDED → +1
  - `tbl_payment_t` label "支付表" — subject visible → INCLUDED → +1
  - `note_002` label "合规政策V2" — subject NOT visible → EXCLUDED (correct, leak fixed)
  - `note_003` label "无归属笔记" — subject NOT visible → EXCLUDED (correct, fail-closed)

  **Detailed TierFilter breakdown after Path C**:
  - Schema triples (always visible): 4 rdf:type for Class/ObjectProperty/Skill/Tier definitions (after IRI fix) + 2 skill individuals (skill_sql, skill_compliance) rdf:type skf:Skill + 2 tier individuals (sql-guides, authority) rdf:type skf:Tier = **8**
  - ABox triples (subject must be visible = sql-guides tier membership):
    - note_001: rdf:type + belongsToTier + label + referencesTable = **4**
    - tbl_payment_t: rdf:type + belongsToTier + label = **3**
  - Total: 8 + 4 + 3 = **15**

  The test currently asserts 13. After Path C, the correct count is 15. Task 5 updates the assertion to 15 with godoc rationale.

- **Path A (nil allowedTiers) fixture count derivation (NEW per Revision):** After Tasks 1-4, `QueryScoped` with `allowedTiers=nil` returns ALL 22 unique fixture triples via the dedup-aware walk (`store.go:492-494` after Task 1). The breakdown:
  - 4 schema class definitions (after D-SPARQL-04 IRI fix)
  - 4 schema individuals (skill_sql + skill_compliance + sql-guides + authority rdf:type)
  - 2 skill hasReadAccess triples (skill_sql→sql-guides, skill_compliance→authority)
  - 4 note_001 triples (rdf:type + belongsToTier + label + referencesTable)
  - 3 note_002 triples (rdf:type + belongsToTier + label)
  - 2 note_003 triples (rdf:type + label — no belongsToTier)
  - 3 tbl_payment_t triples (rdf:type + belongsToTier + label)
  - = 22 unique triples

  All 6 nil-tier tests (`AllWildcard`, `Pagination`, `Truncated` x2, `LimitCap`, `DefaultLimit`) get this 22-count via `SparqlQueryScoped` (which calls `QueryScoped` with `nil` allowedTiers for these tests). The previous plan's "16 = 8 schema + 8 ABox" math silently excluded the 4 label triples + 2 hasReadAccess triples + 2 belongsToTier + 2 rdf:type for note_002/note_003 that Path A does NOT exclude. The 22-count is correct.

- **`TestSparqlQueryScoped_SubjectFilter` count derivation:** Path A with S=note_001 returns ALL 4 note_001 triples (rdf:type + belongsToTier + label + referencesTable). Path A delegates to `s.Query(note_001, "", "")` which returns 4. The test's inline comment at line 106 already says 4 — the `3` assertion is a typo/miscount.

- **Path B (S=note_003, allowedTiers=["sql-guides"]) count derivation:** Path B sees subject `note_003` NOT in `visibleSubjects` (no belongsToTier) → ALL its triples excluded by D-TIER-03 fail-closed. After D-SPARQL-03, label triple for note_003 is no longer auto-schema → still excluded. Result: 0 triples. Test asserts 0. Correct.

- **Path A (S=note_003, nil) count derivation:** Path A with S=note_003 returns 2 triples (rdf:type + label). No filtering applied. Test asserts 2. Correct.

</specifics>

<deferred>
## Deferred Items

- **D-SPARQL-05**: skill `hasReadAccess` visibility in admin baseline (currently ABox, excluded from admin baseline; could be argued to be schema-visible). Defer to discuss-phase for v1.6 follow-up if operator surfaces the question.
- **D-SPARQL-07 (new deferred)**: `comment` predicate has the same trust leak as `label`. Phase 44 fixes `label` but not `comment` (no test exercises comment in this path). Deferred for future hardening.
- **SPARQL 1.1 parser**: Phase 41 backlog item, v1.6+ candidate.
- **Skill `hasReadAccess` subject-based schema detection (Option α)**: if D-SPARQL-05 surfaces this need, switch to subject-based detector. Out of scope for Phase 44.

</deferred>

---

*Phase: 44-sparql-test-fix*
*Context gathered: 2026-09-28 via static-trace research (sandbox cannot run CGO tests; runtime confirmation deferred to operator under Phase 43-01 build environment)*
*Revised: 2026-09-28 per plan-checker BLOCK verdict — Bucket B claim corrected, 7 assertion updates enumerated, D-SPARQL-08 policy added*