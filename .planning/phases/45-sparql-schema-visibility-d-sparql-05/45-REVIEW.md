---
phase: 45-sparql-schema-visibility-d-sparql-05
reviewed: 2026-10-03T03:16:19Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/ontology/store/store.go
  - internal/ontology/store/store_emitter_conformance_test.go
  - internal/ontology/store/store_export_test.go
  - internal/ontology/store/store_label_passthrough_test.go
  - internal/ontology/store/store_schema_subject_test.go
  - internal/server/admin_ontology_test.go
  - internal/server/admin_platform_test.go
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 45: Code Review Report

**Reviewed:** 2026-10-03T03:16:19Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 45's core change (commit 0c3f398) is correct. The β arm
(`schemaSubjects[t.S] && metaPredicates[t.P]`) is nil-map-safe, the 13-predicate
`metaPredicates` allowlist maps exactly 13/13 onto the emitter's schema-individual
predicate inventory (emitter.go render: tierName, categoryName, subCategoryName,
agentTypeName, skillName, skillDescription, skillTemperature, poweredBy,
hasReadAccess, hasWriteAccess, requiresRefusal, refusalPhraseText, belongsToTier),
and all four LOCKED red lines hold:

- **Negative assertions unchanged** — note_002/note_003 subjects carry
  `rdf:type → skf:NoteChunk` (not in `schemaClasses`), so the β arm cannot fire
  for them; their label triples fall to the fail-closed ABox path. Verified by
  hand against fixtureKG math (Path A=22, Path B sql-guides=17, Path B
  S=note_003=10) and independently re-derived — all pinned counts are correct.
- **label/comment excluded** — not in `metaPredicates`; 4 negative pins across
  store_label_passthrough_test.go and the conformance test cover both skf: and
  rdfs: IRI forms.
- **Fail-closed D-TIER-03 preserved** — QueryScoped Path B structure untouched
  except the `isSchemaTriple` signature; visibleSubjects computation unchanged.
- **Nil-tier early-return untouched** — diff confirms Path A returns at
  store.go:516, before the `schemaSubjectSet()` derivation at store.go:543;
  Path A never consults `isSchemaTriple`.

Query-time derivation (D-45-02) is sound: both `schemaSubjectSet` and the single
production `isSchemaTriple` call site (store.go:557) run under RLock, and
derivation-from-indices is invariant across Insert/InsertUnique/DeleteSubject/
Quarantine/LoadSnapshot. Test seam hygiene is acceptable (all seam symbols live
in _test.go — zero production surface; verified). No symbol collisions in either
test package (stubLLM/writeFileStr/fixtureKG defined exactly once in package
server; testNSLocal* once in package store). The external-test-package cycle
handling (store_test → emitter) is legitimate Go. New store tests compile and
pass (`CGO_ENABLED=0 go test` on the three new/changed test functions).

The three warnings below are real but none is a Phase 45 red-line violation:
WR-01/WR-02 are pre-existing behaviors that Phase 45 amplifies or documents
around, WR-03 is a doc-drift regression of a Phase 44 review fix.

## Warnings

### WR-01: Path B schema passthrough bypasses the S/P/O pattern filter — queries return non-matching rows and an inflated Count

**File:** `internal/ontology/store/store.go:550-561` (pinned at
`internal/server/admin_platform_test.go:308-310`,
`internal/ontology/store/store_schema_subject_test.go:92-104`)
**Issue:** In QueryScoped Path B, `isSchemaTriple` short-circuits BEFORE the
subj/pred/obj pattern filter, so every pattern-scoped tier query returns ALL
schema triples regardless of the pattern. Concretely: `QueryScoped("",
hasReadAccess, "", ["sql-guides"])` on fixtureKG returns 10 rows of which only 2
match `p=hasReadAccess`; `S=note_003` returns 10 rows whose subjects are all
≠ note_003. `SparqlQueryScoped` then reports `Count=10` for a pattern matched by
2 triples. This is pre-existing (Phase 44), but Phase 45's β arm amplifies the
phantom-row surface (8→10 per query) and the new tests pin it as intended
("schema passthrough precedes the S/P/O filter"). Consumer-facing risk: both
/api/sparql and the MCP skf_query tool feed from this path — a caller doing
existence checks or graph construction from results gets false positives
(e.g. "does note_003 have triples?" → Count=10) and pages polluted with
unrelated rdf:type/hasReadAccess rows. No data leak (all phantom rows are
non-sensitive schema layer), but it is a query-contract correctness defect.
**Fix:** Apply the S/P/O pattern filter to schema-passthrough rows as well
(schema visibility policy is orthogonal to pattern matching; all fail-closed red
lines are unaffected — only the Path B row-count pins 17→15 / 10→1 would change,
which is a deliberate re-pin, not a red-line break). If the current behavior is
confirmed as product semantics instead, document the "pattern does not apply to
schema rows" contract explicitly on `SparqlPattern` and in the /api/sparql
handler godoc so clients can defend themselves.

### WR-02: GET /api/admin/ontology/kg/query?skill=<unresolvable> fails OPEN to the unfiltered admin baseline

**File:** `internal/server/admin_ontology.go:214-216` (discovered via the parity
claims in reviewed file `internal/server/admin_ontology_test.go:308-394`)
**Issue:** `adminOntologyKGQueryHandler` calls `s.resolveSkillTiers(skill)` and
passes the result straight to `QueryScoped`. An unknown/non-resolving skill
returns nil, and `QueryScoped(..., nil)` is Path A — the unfiltered admin
baseline. The sibling surface handles this correctly:
`adminPlatformSparqlHandler` explicitly 401s on nil tiers
(admin_platform.go:277-281), and `resolveSkillTiersFromKG`'s own godoc
(admin_platform.go:616-617) states nil "callers reject with 401". The kg/query
handler is the one caller that does not — `?skill=bogus` silently returns the
entire KG while implying skill scoping. Admin-token-gated, so no privilege
escalation, but it breaks the fail-closed posture (D-TIER) and means the new
parity test's guarantee ("identical schema/ABox judgment on both surfaces") only
holds for resolvable skills — the unresolvable-skill paths diverge completely.
Pre-existing, not a Phase 45 regression; flagged because the phase's parity test
documents this handler pair as judgment-identical.
**Fix:**

```go
if skill != "" {
    allowedTiers := s.resolveSkillTiers(skill)
    if allowedTiers == nil {
        c.JSON(http.StatusUnauthorized, graphResponseError{Error: "skill not found in KG"})
        return
    }
    matches = dep.KG.QueryScoped(subj, pred, obj, allowedTiers)
}
```

(Or 404/empty-matches per admin-API convention — any explicit fail-closed
outcome; never nil-tier passthrough.)

### WR-03: Phase 45 insertions re-staled the store.go line-number cross-references (regression of 44-review WR-01)

**File:** `internal/server/admin_platform_test.go:54-55, 59, 101, 121, 304`;
`internal/ontology/store/store.go:607`
**Issue:** Commit 0c3f398 inserted 7 lines into QueryScoped and ~60 lines around
`isSchemaTriple`/`metaPredicates` without updating the line-number references in
comments — the exact defect class Phase 44 review WR-01 already fixed once
(commit a69a774). Now stale: "short-circuit at store.go:550-554" → actual
556-561; "schemaClasses arm (store.go:614-616)" → actual 633-635; "rdf:type arm
at store.go:605-617" (three sites) → actual 624-636; "precedes the S/P/O filter
at store.go:543-554" → actual 550-561 (line 543 is now the D-45-02 comment).
Only "store.go:528-536" (visibleSubjects) survives. Misleading refs send
maintainers to the wrong arms during the next security-sensitive edit of this
gate — costly in precisely the file where visibility semantics live.
**Fix:** Re-pin the references to current lines (556-561, 633-635, 624-636,
550-561), or better: replace hard line numbers with anchor names
("the isSchemaTriple short-circuit in QueryScoped", "the rdf:type+schemaClasses
arm") so future insertions cannot stale them again.

## Info

### IN-01: Test seam hygiene — export-for-test seam returns mutable aliases and skips the documented lock

**File:** `internal/ontology/store/store_export_test.go:42-49`
**Issue:** Three seam functions leak more than needed: `ConformanceMetaPredicates()`
returns the live package-level allowlist map (a test bug could mutate the
allowlist for the whole test binary, silently weakening the D-SPARQL-03 gate);
`ConformanceAll(s)` returns the internal `s.all` slice by alias;
`ConformanceSchemaSubjectSet(s)` calls `schemaSubjectSet()` without RLock,
violating that method's "Caller must hold at least RLock" contract. All safe
today (test-only, single-threaded, verified read-only usage in
store_emitter_conformance_test.go), and the zero-production-surface property is
correct (symbols live only in _test.go).
**Fix:** Return copies (`for k := range metaPredicates { out[k] = true }`;
`append([]Triple(nil), s.all...)`) and take `s.mu.RLock()` inside
`ConformanceSchemaSubjectSet`.

### IN-02: isSchemaTriple keys on skf: IRIs while the emitter emits owl:/rdfs: IRIs — declaration layer is tier-invisible, and the mismatch is only half-documented

**File:** `internal/ontology/store/store.go:625-627, 645-647` vs
`internal/ontology/emitter/emitter.go:219, 237, 252`
**Issue:** The conformance test's PRE-EXISTING QUIRK comment
(store_emitter_conformance_test.go:63-69) documents that class declarations emit
`a owl:Class` while the rdf:type arm checks `ns+"Class"` (skf:Class), so
production declaration triples are tier-invisible. The review adds the
undocumented half of the same defect: the always-schema metadata arm checks
`ns+"domain"` / `ns+"range"` / `ns+"subClassOf"` / `ns+"subPropertyOf"`
(skf: IRIs), but the emitter emits `rdfs:domain` / `rdfs:range`
(W3C RDFS IRIs, emitter.go:237/252) — so that arm never matches production
output either. Additionally `ns+"Property"` and `ns+"NamedIndividual"`
(store.go:626-627) are never emitted (dead checks). Net effect is
over-filtering in the fail-closed direction (no leak): skill-scoped Path B
views never see the TBox declaration layer. Correctly deferred out of Phase 45
scope; recorded here so Phase 46 fixes the whole IRI-alignment family at once
rather than only the owl:Class half.
**Fix:** Phase 46: normalize the gate to the IRIs the emitter actually emits
(`owl:Class`/`owl:ObjectProperty`/`owl:DatatypeProperty`,
`rdfs:domain`/`rdfs:range`/`rdfs:subClassOf`/`rdfs:subPropertyOf`) and delete or
justify the dead skf:Property/skf:NamedIndividual checks; extend the emitter
conformance test to pin declaration-layer visibility when fixed.

### IN-03: β arm discloses cross-skill tier topology to every skill-scoped view (intended per D-SPARQL-05, noted for contract visibility)

**File:** `internal/server/admin_platform_test.go:61-70` (behavior originates in
`internal/ontology/store/store.go:653-655`)
**Issue:** Because schema passthrough is tier-agnostic, a sql-guides-scoped
caller now receives `skill_compliance hasReadAccess authority` — i.e. which
OTHER skills exist and which restricted tiers they can read. The fixture godoc
pins this explicitly ("regardless of allowedTiers") and the data disclosed is
platform configuration metadata, not ABox user data, so this is the intended
D-SPARQL-05 semantics — no red line broken. Recorded so the information-disclosure
surface change is a visible, accepted product decision rather than an
unexamined side effect.
**Fix:** None required if accepted. If cross-skill topology is later judged
sensitive, scope the β arm's passthrough to hasReadAccess edges whose object
resolves to an allowed tier (that would change the pinned 17/10 counts).

---

## Verification Performed

- Diff-scoped the phase: 224a7b6 → f5c7c06 across the 7 files (0c3f398, 7931106,
  1479534, 417ef99, 118eebd).
- Hand-derived fixtureKG visibility math for Path A (22), Path B sql-guides (17),
  Path B S=note_003 (10) — all pinned counts correct.
- Verified allowlist completeness against emitter.go render output: 13/13.
- Traced all `isSchemaTriple` (1 production call site, store.go:557, RLock held)
  and `schemaSubjectSet` (1 production call site, store.go:543, RLock held) callers.
- Verified both resolvers resolve ["sql-guides"] for skill "sql" on fixtureKG
  (Case-1 direct-Tier), confirming the parity test's resolver-parity precondition.
- Checked test-package symbol uniqueness (no redeclarations of stubLLM,
  writeFileStr, fixtureKG, testNSLocal consts).
- `CGO_ENABLED=0 go test -run 'TestEmitterPredicatesWithinAllowlist|
  TestStoreSchemaSubject|TestIsSchemaTriple_Label' ./internal/ontology/store/`
  → ok. (`go vet` cgo failure is the known Windows toolchain issue; project
  builds via WSL per runbook — not a code defect.)
- Pre-existing deferred failure TestEmitSubCategoryInstanceCount (D-45-DEF-01)
  not flagged, per phase context.

_Reviewed: 2026-10-03T03:16:19Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
