---
phase: 39-ecl-pipeline
plan: 01
subsystem: ontology
tags: [rdf, triple-store, tbox, owl, knowledge-graph, dedupe, go]

# Dependency graph
requires:
  - phase: 38-foundation-tbox-kg
    provides: "internal/ontology/store (Insert/Query/indices/snapshot) + internal/ontology/emitter (6-class TBox render)"
provides:
  - "store.DeleteSubject — O(n) subject-scoped delete with full byS/byP/byO index rebuild (note-level replace primitive, D-Boot-01)"
  - "store.InsertUnique — single-lock dedupe insert guard (boot TBox→KG merge anti-double-count)"
  - "TBox v2 schema: 8 classes / 8 object properties (+NoteChunk, +TableSchema, +belongsToNote, +referencesTable per D-TBox2-01)"
  - "NoteChunk sentinel string test (`skf:NoteChunk a owl:Class`) — Plan 06 boot upgrade trigger keys on it"
affects: ["39 Plan 04 (ecl/abox.go KG replace)", "39 Plan 05/06 (manager + boot wiring, upgrade trigger)", "phase-40 governance"]

# Tech tracking
tech-stack:
  added: []  # zero new dependencies (T-39-SC accepted)
  patterns:
    - "Delete-then-full-index-rebuild: positions in byS/byP/byO point into s.all, so any removal invalidates all three — filter + rebuild in one write-lock pass"
    - "Check-then-insert under one lock acquisition (TOCTOU-free dedupe via byS candidate scan)"

key-files:
  created: []
  modified:
    - internal/ontology/store/store.go
    - internal/ontology/store/store_test.go
    - internal/ontology/emitter/emitter.go
    - internal/ontology/emitter/emitter_test.go

key-decisions:
  - "DeleteSubject rebuilds all three indices from scratch after filtering (planner-pinned approach; O(n) fine at <10k triples)"
  - "InsertUnique inlines the candidate scan under the write lock (byS[subject] positions + exact P/O compare) instead of calling Query — avoids RLock-inside-write-lock awkwardness and TOCTOU"
  - "belongsToNote emitted declared-only (zero ABox instances, open-question resolution #1 — reserved for Phase 40+ section-level ABox); test asserts no ABox usage leaks into emitter output"
  - "Godoc class/property counts updated 6→8 alongside tests (package, Emit, render comments) so docs match the schema source"
  - "requirements ONT-12/ONT-13 intentionally NOT marked complete — they are phase-level outcomes (LLM extraction, Milvus dual-write) delivered by Plans 04-06, not by this contract plan"

patterns-established:
  - "Index-symmetric mutation: every store mutation that shifts positions must rebuild byS/byP/byO together (proven by phantom-entry tests)"
  - "Sentinel-line test pattern: emitted schema contracts pinned as exact Turtle substrings for downstream boot triggers"

requirements-completed: [ONT-12, ONT-13]  # copied verbatim from plan frontmatter — see note below; phase-level outcomes remain open until Plans 04-06

# Metrics
duration: 5min
completed: 2026-09-22
---

# Phase 39 Plan 01: Store Delete/Dedupe Primitives + TBox 6→8 Summary

**KG store gains DeleteSubject (index-rebuilding note-level replace) + InsertUnique (TOCTOU-free dedupe merge); TBox emitter extended to the locked 8-class/8-objProp D-TBox2-01 schema with NoteChunk sentinel tests.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-09-22T01:05:06Z
- **Completed:** 2026-09-22T01:10:11Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- `store.DeleteSubject(subject) int` — removes all triples of a subject, rebuilds byS/byP/byO from scratch in one write-lock pass; no-op on unknown subject
- `store.InsertUnique(t) bool` — dedupe-guarded insert under a single lock acquisition; boot-merge replay test proves snapshot restore + TBox reload does not double-count
- TBox emitter 6→8 classes (+NoteChunk, +TableSchema) and 6→8 object properties (+belongsToNote, +referencesTable) exactly per D-TBox2-01; emitted triple count 176→190 (+14)
- Sentinel test pins `skf:NoteChunk a owl:Class` / `skf:TableSchema a owl:Class` as exact substrings — the Phase 39 boot upgrade trigger contract for Plan 06

## Task Commits

Each task was committed atomically:

1. **Task 1: Add DeleteSubject + InsertUnique primitives to the KG store** - `e11af8e` (feat)
2. **Task 2: Extend TBox emitter 6→8 classes (D-TBox2-01)** - `5aa2e11` (feat)

## Files Created/Modified
- `internal/ontology/store/store.go` — InsertUnique + DeleteSubject primitives; Insert godoc now points at InsertUnique
- `internal/ontology/store/store_test.go` — 4 new tests: InsertUnique, InsertUniqueBootMerge (no double-count), DeleteSubject (index symmetry/phantom-entry), DeleteSubjectUnknown (no-op purity)
- `internal/ontology/emitter/emitter.go` — classes/objProps lists extended with the 4 D-TBox2-01 entries; godoc counts 6→8
- `internal/ontology/emitter/emitter_test.go` — TestEmitSchemaCounts 8/8/8; new TestEmitECLSchemaDeclarations (sentinels + domain/range blocks + zero-ABox-usage assertions)

## Decisions Made
- DeleteSubject uses filter + full index rebuild (planner-pinned; partial compaction would corrupt the other two indices since all hold positions into s.all)
- InsertUnique inlines its existence scan under the write lock rather than calling Query (Query takes RLock — nesting under the write lock is not possible; inline scan over byS[t.S] candidates is semantically identical to Query(t.S, t.P, t.O))
- belongsToNote is declared-only this phase (CONTEXT open-question resolution #1); emitter test forbids any ABox usage of it
- Godoc counts (package reference, Emit, render comments) updated in the same pass as tests so the "single schema source" lists and their documentation agree
- ONT-12/ONT-13 left unchecked in REQUIREMENTS.md: this plan delivers the store/emitter contracts only; the requirements' outcomes (LLM extraction, ABox dual-write, Milvus metadata) arrive with Plans 04-06

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## Verification

- `go test ./internal/ontology/store/ -count=1` — 16/16 PASS (12 pre-existing green + 4 new)
- `go test ./internal/ontology/emitter/ -count=1` — 7/7 PASS (roundtrip lossless at 190 triples)
- `go test ./internal/ontology/... -count=1` — all 3 packages ok (emitter, induction, store)
- DeleteSubject symmetry proven: no phantom entries via byS/byP/byO after delete; wildcard enumeration consistent with Len; no-op delete byte-identical snapshot
- Emitted TTL contains all 4 sentinel declarations with correct domain/range wiring; zero ABox instances for NoteChunk/TableSchema/belongsToNote

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Contracts exported for downstream plans: `store.DeleteSubject`, `store.InsertUnique` (Plan 04 abox.go note-level replace), emitter NoteChunk sentinel (Plan 06 boot upgrade trigger: pointer TTL lacking `skf:NoteChunk a owl:Class` → re-emit v2)
- Zero new dependencies; hermetic tests only (no Milvus/BGE/GLM contact)
- Ready for 39-02

## Self-Check: PASSED

- All 4 key-files exist on disk (store.go, store_test.go, emitter.go, emitter_test.go)
- Commit e11af8e (Task 1) and 5aa2e11 (Task 2) present in git log
- All task acceptance criteria re-run and passing (signatures present; 23 tests green across store+emitter; full ./internal/ontology/... ok)

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
