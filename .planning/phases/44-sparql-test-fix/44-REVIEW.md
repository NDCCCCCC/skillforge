---
phase: 44-sparql-test-fix
reviewed: 2026-09-28T12:55:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/ontology/store/store.go
  - internal/ontology/store/store_sparql_dedup_test.go
  - internal/ontology/store/store_label_passthrough_test.go
  - internal/server/admin_platform_test.go
findings:
  critical: 0
  warning: 3
  info: 5
  total: 8
status: issues_found
---

# Phase 44: Code Review Report

**Reviewed:** 2026-09-28
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Phase 44 is pure closure work fixing 8 SPARQL test failures via 2 code fixes (QueryScoped nil-tier dedup + isSchemaTriple label/comment removal), 2 fixture fixes (IRI alignment + duplicate removal), and 9 test-author assertion corrections. All 10 SPARQL tests now pass and D-TIER-03 fail-closed semantics are restored.

**Functional correctness: verified.** I traced the dedup key (S|P|O), the schema short-circuit ordering (before pattern filter), the visibleSubjects computation (only subjects whose belongsToTier is in allowedTiers), and the new isSchemaTriple predicate gate. The math in the test assertions (22 path A, 15 path B, 8 FailClosedNoBelongsToTier, 2 Truncated offset) all check out against the fixture's 22 unique triples. The Bucket C fix correctly moves label/comment back to ABox — the trust-boundary restoration is real, not weakened.

**Test coverage: solid.** 4 new nil-tier dedup tests + 2 label-passthrough regression tests lock the contracts at both unit and integration layers. The 4 dedup tests also probe the early-return-to-Query behavior (pattern-specific queries bypass dedup, consistent with documented design).

**Documentation quality: mixed.** The 75-line fixtureKG godoc block is comprehensive and the per-test inline rationale is excellent. However, three documentation issues surfaced:

1. Cross-file line-number references in the fixtureKG godoc are stale (referenced lines shifted after Phase 44 additions).
2. The "visibleSubjects" enumeration conflates two paths (visibleSubjects map vs schema short-circuit) and mislabels schema individuals as ABox.
3. The isSchemaTriple godoc's "are caught by the rdf:type+schemaClasses[O] arm" wording can be misread to suggest label triples themselves are caught.

No BLOCKER findings. No security vulnerabilities. No logic errors. The 3 WARNINGs are all documentation accuracy; the 5 INFOs are minor quality/consistency items.

## Critical Issues

None.

## Warnings

### WR-01: Stale line-number references in fixtureKG godoc

**File:** `internal/server/admin_platform_test.go:53` and `:58`

**Issue:** The cross-file line-number references in the 75-line fixtureKG godoc are stale. After Phase 44 added 27/-7 lines to `store.go`, the `isSchemaTriple` function moved from approximately lines 570-580 to lines 603-632. The godoc still references:

- Line 53: `Caught by isSchemaTriple's rdf:type arm (store.go:570-575)` — actual lines are 605-617
- Line 58: `Caught by isSchemaTriple's schemaClasses arm (store.go:579-581)` — actual lines are 614-616

This is a Phase-44-introduced documentation drift. The narrative descriptions (which arm catches which case) are still correct, but the cited line numbers will mislead any maintainer navigating to verify the claim.

**Fix:** Update the line-number references to the post-Phase-44 values:

```go
// 1. Schema class definitions (4 rdf:type triples): testNS+Class,
//    testNS+ObjectProperty, testNS+Skill, testNS+Tier -> rdf:type with
//    object = Class/ObjectProperty. Caught by isSchemaTriple's rdf:type
//    arm (store.go:605-617) + the owl-IRI fix (D-SPARQL-04).
//
// 2. Schema individual types (4 rdf:type triples): skill_sql,
//    skill_compliance -> rdf:type skf:Skill; sql-guides, authority ->
//    rdf:type skf:Tier. Caught by isSchemaTriple's schemaClasses arm
//    (store.go:614-616).
```

### WR-02: Incorrect visibleSubjects membership enumeration in fixtureKG godoc

**File:** `internal/server/admin_platform_test.go:97-98`

**Issue:** The Path B breakdown godoc states:

```
ABox (subject in visibleSubjects={note_001, tbl_payment_t,
      sql-guides, authority, skill_sql, skill_compliance}):
```

This is technically wrong. Per the implementation at `store.go:528-536`, `visibleSubjects` is built exclusively from `belongsToTier` triples whose object is in `allowedTierSet`. Looking at the fixture:

- `(note_001, belongsToTier, sql-guides)` → added (subject note_001)
- `(note_002, belongsToTier, authority)` → NOT added (tierName=authority not in {"sql-guides"})
- `(tbl_payment_t, belongsToTier, sql-guides)` → added (subject tbl_payment_t)

So `visibleSubjects = {note_001, tbl_payment_t}` — full stop. The schema individuals `skill_sql`, `skill_compliance`, `sql-guides`, `authority` have NO `belongsToTier` triple and are NOT in `visibleSubjects`. Their triples appear in the Path B output via the `isSchemaTriple` schema short-circuit (`store.go:550-554`), which runs BEFORE the visibleSubjects check.

The final count (15) is mathematically correct because of this two-path accounting (8 schema + 4 note_001 + 3 tbl_payment_t). But labeling the 4 schema individuals as "ABox ... subject in visibleSubjects" mischaracterizes how they appear in the output and contradicts the implementation. The list also conflates "entities whose triples appear in the result" with "entities in visibleSubjects".

**Fix:** Rewrite the Path B breakdown to clearly separate the two mechanisms:

```go
//     Path B (allowedTiers=["sql-guides"]):
//       Schema (always visible via isSchemaTriple short-circuit at
//               store.go:550-554, D-TIER-03):
//         - 4 schema class defs (rdf:type+Class/ObjectProperty arm)
//         - 4 schema individual types (rdf:type+schemaClasses arm)
//         = 8
//       ABox (subject in visibleSubjects={note_001, tbl_payment_t}
//             computed at store.go:528-536 from belongsToTier triples):
//         - note_001: 4 triples (subject visible, D-TIER-06 includes all edges)
//         - tbl_payment_t: 3 triples (subject visible)
//       = 8 schema + 4 + 3 = 15
//
//       Subjects NOT in visibleSubjects (excluded by ABox path):
//         - skill_sql, skill_compliance (no belongsToTier) — their
//           hasReadAccess triples are ABox and excluded
//         - note_002 (authority tier not in allowedTiers) — fail-closed
//         - note_003 (no belongsToTier) — fail-closed (D-TIER-07)
```

### WR-03: Wording ambiguity in isSchemaTriple godoc

**File:** `internal/ontology/store/store.go:597-600`

**Issue:** The D-SPARQL-03 godoc block says:

```
Schema-subject label/comment triples (e.g., skill_sql rdfs:label "...") are
caught by the rdf:type+schemaClasses[O] arm above for the rdf:type triple;
their label/comment triples are returned via the ABox path when the subject
is visible and excluded when fail-closed (correct D-TIER-06 + D-TIER-07
behaviour).
```

The phrase "are caught by the rdf:type+schemaClasses[O] arm above for the rdf:type triple" can be misread to suggest that the label/comment triples themselves are caught by the schema arm. They are not — only the `rdf:type` triple (e.g., `(skill_sql, rdf:type, skf:Skill)`) is caught. The label/comment triples always fall through to the ABox path. The second clause ("returned via the ABox path when the subject is visible") correctly contradicts this misreading, but the first clause invites it.

The intent is: "The rdf:type triple that declares a schema individual is caught (kept visible unconditionally); the same subject's label/comment triples fall through to ABox and are subject to the standard visibility check."

**Fix:** Reword for clarity:

```go
// D-SPARQL-03: rdfs:label and rdfs:comment are intentionally EXCLUDED from
// the always-schema predicate gate. They are ABox annotations subject to
// D-TIER-03 fail-closed semantics — a label/comment triple for an
// invisible subject (e.g., note_002 in authority tier when allowedTiers is
// ["sql-guides"]) MUST NOT be returned.
//
// Note: the rdf:type triple that declares a subject as a schema individual
// (e.g., (skill_sql, rdf:type, skf:Skill)) IS schema (caught by the
// rdf:type+schemaClasses[O] arm at store.go:605-617). But the SAME
// subject's label/comment triples are NOT schema — they fall through to
// the ABox path and are returned only when the subject is in
// visibleSubjects, otherwise excluded (correct D-TIER-06 + D-TIER-07).
```

## Info

### IN-01: Capacity hint inconsistency between Path B and nil-tier dedup

**File:** `internal/ontology/store/store.go:540` vs `:504`

**Issue:** The new nil-tier all-wildcards dedup pre-allocates the seen map with a capacity hint (`seen := make(map[string]bool, len(s.all))`), but the pre-existing Path B main loop's seen map has no hint (`seen := make(map[string]bool)`). Both maps are populated by walking `s.all` once. Minor consistency issue.

**Fix:** Add the same capacity hint to Path B's seen map at line 540:

```go
seen := make(map[string]bool, len(s.all)) // deduplicate by S+P+O string key
```

### IN-02: Dedup key uses '|' as separator — theoretical collision risk

**File:** `internal/ontology/store/store.go:507` and `:544`

**Issue:** Both dedup sites construct the key as `t.S + "|" + t.P + "|" + t.O"`. If any S/P/O value contains the `|` character, a collision is theoretically possible (e.g., `(S="a|b", P="c", O="d")` and `(S="a", P="b|c", O="d")` both produce `"a|b|c|d"`). In practice, IRIs don't contain `|`, so this is a theoretical risk only.

**Fix:** If paranoia warranted, use a less-common separator (e.g., NUL byte via `"\x00"`) or a struct key (Go map with struct key is fine for hash-tuple comparisons). Not blocking.

### IN-03: Pre-existing `_ = i` workaround in Path B main loop

**File:** `internal/ontology/store/store.go:543-575`

**Issue:** The Path B main loop uses `for i, t := range s.all` and the index `i` is never referenced in the body, requiring `_ = i // silence unused variable` at line 574. The cleaner form is `for _, t := range s.all`. Pre-existing code (not introduced by Phase 44), but worth noting because the new nil-tier path at lines 506-513 uses the cleaner form, highlighting the inconsistency.

**Fix:** Drop `i` and the `_ = i` line:

```go
for _, t := range s.all {
    key := t.S + "|" + t.P + "|" + t.O
    if seen[key] {
        continue
    }
    ...
}
```

### IN-04: Integration test lacks schema-subject label case

**File:** `internal/ontology/store/store_label_passthrough_test.go:133-160`

**Issue:** The unit test `TestIsSchemaTriple_LabelAndCommentAreABox` covers the schema-subject label case (asserts `isSchemaTriple((skill_a, label, ...))` returns false). But the integration test `TestStore_LabelAndCommentNotSchemaPassthrough` only includes ABox subjects (`note_authority` and `note_oracle`). A scenario where a schema individual has a label triple (no belongsToTier) would verify end-to-end that the label is visible on Path A but excluded on Path B.

**Fix:** Add a third subject to the integration test fixture:

```go
// skill_sql: schema individual with a label, NO belongsToTier
s.Insert(Triple{S: testNSLocal + "skill_sql", P: rdfTypeLocal, O: skfSkillLocal})
s.Insert(Triple{S: testNSLocal + "skill_sql", P: testNSLocal + "label", O: `"SQL Skill"`})

// In the test body, add assertions:
// Path A: skill_sql label is visible (no tier filter)
out4 := s.QueryScoped("", testNSLocal+"label", "", nil)
assert.Len(t, out4, 3, "admin baseline sees all 3 labels (note_authority + note_oracle + skill_sql)")

// Path B: skill_sql label is NOT visible (schema individual without belongsToTier)
out5 := s.QueryScoped(testNSLocal+"skill_sql", testNSLocal+"label", "", []string{"sql-guides"})
assert.Empty(t, out5, "schema individual label excluded under tier filter (D-SPARQL-03)")
```

### IN-05: `schemaClasses` map declared as var and initialized in init()

**File:** `internal/ontology/store/store.go:637-659`

**Issue:** `schemaClasses` is declared as `var schemaClasses map[string]bool` (nil map) and then populated in `init()`. There's a brief window where `schemaClasses` is nil between package load and `init()` completion. In practice `init()` runs before any test or production code calls `isSchemaTriple`, so this is safe. Pre-existing pattern, not Phase 44.

**Fix:** Could be made a compile-time `var schemaClasses = map[string]bool{...}` to remove the two-step init. Low priority.

---

_Reviewed: 2026-09-28T12:55:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

## REVIEW COMPLETE