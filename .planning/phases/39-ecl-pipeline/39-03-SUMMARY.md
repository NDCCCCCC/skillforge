---
phase: 39-ecl-pipeline
plan: 03
subsystem: knowledge
tags: [chunker, section-aware, regex-extraction, yaml-frontmatter, ecl, golden-set, sql-guides]

# Dependency graph
requires:
  - phase: 39-ecl-pipeline (repo substrate)
    provides: validated spike 005c artifacts (.planning/spikes/005c-ontology-aware-chunker/) + in-repo corpus knowledge/sql-guides (202 files); plans 39-01/39-02 landed the TBox/store and Milvus layer this package's downstream plans consume
provides:
  - internal/knowledge/ecl package: WalkAndMerge section-aware chunker (005c algorithm productionized per D-ECL-02)
  - IsSecFragment routing predicate ready for Plan 06's indexer.go branch (D-ECL-01)
  - ChunkMeta JSON contract byte-for-key with spike chunk_metadata.json (D-ECL-02)
  - StubExtract deterministic extractor + Extraction/ExtractedTable/RelatedNote types + ValidateExtraction kind-enum gate (D-Ext-01) — Plan 04's test double and Plan 09's golden-set drafter (D-GT-01)
  - Hermetic corpus-backed tests proving SC-1 (185/185 boundary, 26 notes, 69 sections, note 001 3/3 SQL targets)
affects: [39-04-extractor, 39-05-manager, 39-06-indexer-routing, 39-09-golden-set-verify]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies (yaml.v3 already in go.mod)
  patterns: [spike-verbatim productionization (D-ECL-02), line-anchored (?i:keyword)-scoped regexes keeping etl_ prefix strict, dynamic+pinned dual assertions with distinct failure messages, narrow deterministic drafter under D-GT-01]

key-files:
  created:
    - internal/knowledge/ecl/meta.go
    - internal/knowledge/ecl/merge.go
    - internal/knowledge/ecl/stub.go
    - internal/knowledge/ecl/merge_test.go
  modified: []

key-decisions:
  - "Related-notes detection pinned to strict 参考/见-word + 3-digit source_idx anchor; corpus survey found zero explicit numeric cross-references today (nearest non-reference lines documented in stub.go), so the drafter emits [] and human review owns relations per D-GT-01 — zero false positives by design"
  - "sort.Slice → sort.SliceStable in the merge port (plan pins a stable sort; output identical on the validated corpus where SecOrder is unique per note)"
  - "CRLF normalization applied before frontmatter split (plan-mandated, Pitfall 7): counts and structure match the spike exactly; body_bytes for CRLF-containing sections are shorter than spike chunk_metadata.json (the spike ran un-normalized), so byte-fidelity holds only for LF files — self-consistency (BodyBytes == len(Body)) is what the tests pin"

patterns-established:
  - "ECL fragment contract: __secNN filename AND non-empty section frontmatter — both required (IsSecFragment)"
  - "Dual assertion pattern: dynamic invariant (contribution completeness) + pinned SC numbers with a distinct 'corpus drift' failure message"
  - "Regex stub doubles as golden-set drafter; the LLM never touches golden-set construction (D-GT-01)"

requirements-completed: [ONT-10, ONT-11]

# Metrics
duration: 7 min
completed: 2026-09-22
---

# Phase 39 Plan 03: ECL Chunker Core Summary

**Section-aware walk+merge chunker productionized from validated spike 005c (D-ECL-02) with the IsSecFragment routing predicate, the ChunkMeta contract, and a deterministic regex stub extractor — corpus-backed tests prove SC-1: 185/185 boundary completeness, 185 fragments → 26 notes → 69 sections, note 001 contains 3/3 corpus SQL targets.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-09-22T01:41:34Z
- **Completed:** 2026-09-22T01:48:01Z
- **Tasks:** 3 (Task 2 via TDD RED→GREEN)
- **Files modified:** 4 (all new)

## Accomplishments
- `internal/knowledge/ecl` package with the 005c walk+merge core ported verbatim (secRegex, frontmatter line-scan, group-by source_idx → stable sort by SecOrder → same-section body concatenation in first-seen order → numeric source_idx note sort), plus subdir descent mirroring indexer.go walkTier and CRLF normalization before the frontmatter split (Pitfall 7)
- `IsSecFragment(base, frontmatter)` — the exact D-ECL-01 routing predicate Plan 06 wires into indexer.go: `__sec(\d{2})_(.+)\.md$` match AND non-empty `section` key; verified false on `030_承保清单.md` (no section key) and on secRegex-match-without-section
- Regex stub extractor with the D-Ext-01 kind enum (update|select|insert|create; DELETE FROM and MERGE INTO normalize to "update", SELECT…INTO → "select"), etl_-prefix gate with case preservation, positional first-seen dedupe, and exported `ValidateExtraction` for Plan 04's LLM output validation
- SC-1 proven hermetically against the real corpus: contribution completeness 185/185, 26 notes, 69 section records, note 001 3/3 (etl_payment_due_t / etl_policy_request_issue_t / etl_NONAUTO_POLICY_T), sec01 section==title pathology tolerated, ChunkMeta JSON key set exactly the spike contract

## Task Commits

Each task was committed atomically:

1. **Task 1: ChunkMeta contract + walk/merge core (005c port)** - `04dc0c1` (feat)
2. **Task 2: Regex stub extractor (TDD)** - `cf8415e` (test, RED) + `daaa563` (feat, GREEN)
3. **Task 3: Corpus-backed boundary tests (SC-1 proof)** - `37e4280` (test)

## Files Created/Modified
- `internal/knowledge/ecl/meta.go` — ChunkMeta (7 spike json tags verbatim) + Fragment/MergedSection/Note types
- `internal/knowledge/ecl/merge.go` — secRegex, IsSecFragment, WalkFragments, MergeFragments, ChunkMetas, WalkAndMerge; walk error posture per T-39-02 (unreadable file = error, never silent skip)
- `internal/knowledge/ecl/stub.go` — Extraction/ExtractedTable/RelatedNote, ValidateExtraction, StubExtract with pinned related-note anchor and corpus-survey evidence comment
- `internal/knowledge/ecl/merge_test.go` — 4 table-driven stub test groups + 4 corpus-backed SC-1 tests, all hermetic (file reads only)

## Decisions Made
- **Related-notes anchor pinned strict** (see key-decisions): plan instructed "grep the corpus and pin whatever regex you find"; the sweep found no `见 0NN`-style numeric references — only generic-noun collisions (041 uses 开门红 as a business term; 010 uses 定时任务 as a generic noun) and one title-based reference (002: "详情参见：Linux 中修改 JVM 内存"). The strict word+digit+boundary anchor emits zero relations on today's corpus (correct for a D-GT-01 drafter; human review supplies what regex cannot see), with the two near-miss lines cited in stub.go.
- **Stable sort in the port**: plan text pins "stable sort parts by SecOrder"; spike used unstable sort.Slice. SliceStable adopted — identical output on the validated corpus.
- **CRLF normalization vs spike body_bytes**: plan mandates CRLF normalization (Pitfall 7); the spike's chunk_metadata.json ran un-normalized, so byte counts for CRLF files differ. All pinned assertions are structural counts (185/26/69) plus self-consistency (BodyBytes == len(Body)); spike byte-fidelity holds for LF files (e.g. note 001's SQL section structure).

## Deviations from Plan

None - plan executed exactly as written. (TDD RED→GREEN test-fixture corrections during Task 2 are the documented TDD debug/iterate cycle, listed under Issues Encountered.)

**Total deviations:** 0 auto-fixed.
**Impact on plan:** None.

## Issues Encountered
- Task 2 GREEN: two test fixtures initially mis-encoded expectations — (1) the multi-section case hand-built a section body without the trailing "\n" MergeFragments always appends (fixed by asserting the real merge contract; an end-to-end MergeFragments→StubExtract case was added proving cross-fragment statement starts stay visible); (2) the related-notes fixture placed "041" after "与" instead of after a reference word, so the pinned regex correctly did not match it (fixture corrected). No implementation changes were needed.

## Known Stubs
- `StubExtract` in `internal/knowledge/ecl/stub.go` is intentionally a deterministic regex extractor, not a placeholder: D-GT-01 mandates the LLM never builds the golden set, and D-Ext-01's LLM extractor (Plan 04) implements the same Extraction contract separately. No unwired data paths.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 04 (LLM extractor/ABox) can import `Extraction`/`ValidateExtraction` and run `StubExtract` as its test double
- Plan 05 (manager) can consume `WalkAndMerge` and own the oversized-section split (deliberately not implemented here)
- Plan 06 (indexer routing) has `IsSecFragment` ready; indexer.go untouched per plan
- Plan 09 (golden set + verify gate) can drive `StubExtract` over all 26 notes and invoke the corpus tests directly
- Full package `go test ./internal/knowledge/ecl/ -count=1` exits 0 with no network or env dependencies

## Self-Check: PASSED

All 5 key files exist on disk; all 5 commit hashes (04dc0c1, cf8415e, daaa563, 37e4280, 6e42f57) present in git log; full package `go test ./internal/knowledge/ecl/ -count=1` green with SC numbers printed (185/185, 26 notes, 69 sections, note 001 3/3).

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
