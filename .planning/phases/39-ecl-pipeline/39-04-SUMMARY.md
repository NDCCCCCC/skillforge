---
phase: 39-ecl-pipeline
plan: 04
subsystem: knowledge-pipeline
tags: [ecl, ontology, abox, rdf, llm-extraction, f1, golden-set, eino]

# Dependency graph
requires:
  - phase: 39-01
    provides: store.DeleteSubject / store.InsertUnique (note-level replace primitives) + emitter TBox v2 (NoteChunk / TableSchema / referencesTable declarations)
  - phase: 39-02
    provides: Milvus 4 native ECL fields, DeleteByExpr, GetMaxIDCollection (not consumed by this plan; consumed by Plans 05/06)
  - phase: 39-03
    provides: ecl.Note / MergedSection types, Extraction/ExtractedTable/RelatedNote types, ValidateExtraction kind-enum gate, StubExtract regex drafter
provides:
  - ecl.Extractor interface with dual implementations: LLMExtractor (temp-0, response_format json_object, llm.ExtractJSON, ValidateExtraction, 40K-rune per-section union guardrail) and StubExtractFunc adapter
  - pinned cross-plan constructor ecl.NewLLMExtractor(chat, sectionCap ...int) — Plan 06 Task 3 calls ecl.NewLLMExtractor(cm)
  - internal/llm.JSONObjectResponseFormat exported option (one-line wrapper, no second WrapImplSpecificOptFn copy)
  - F1 evaluation math: LoadGolden (D-GT-01 jsonl loader), ScoreTables, ScoreRelated, NoteScore/MacroF1 — pure, reusable by Plan 09 harness and verify script unmodified
  - ABox write path: NoteIRI / TableIRI / BuildTriples (D-Ext-01 triple forms) / ApplyNote (validate-before-mutate → DeleteSubject → orphan sweep → Insert/InsertUnique → single onChange snapshot trigger)
  - emitter.SanitizeLocal exported wrapper (single IRI local-name character policy)
affects: [39-05 manager orchestrator, 39-06 boot wiring, 39-09 golden harness + verify script]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Narrow Extractor interface (induction.Generator seam shape) with LLM + stub behind one contract"
    - "LLM call posture: SystemMessage('You output only valid JSON.') + model.WithTemperature(0.0) + llm.JSONObjectResponseFormat() + llm.ExtractJSON + strict unmarshal into private mirror shape + ValidateExtraction enum gate"
    - "Prompt-size guardrail: whole-note call under 40K runes; per-section calls with union (first kind wins, related deduped by source_idx+relation) above it"
    - "Note-level KG replace ordering: validate → capture → DeleteSubject → orphan sweep → Insert/InsertUnique → exactly one onChange"
    - "Zero-denominator → 0.0 convention for P/R/F1; macro-average counts absent notes as zero rows"

key-files:
  created:
    - internal/knowledge/ecl/extract.go
    - internal/knowledge/ecl/extract_test.go
    - internal/knowledge/ecl/f1.go
    - internal/knowledge/ecl/f1_test.go
    - internal/knowledge/ecl/abox.go
    - internal/knowledge/ecl/abox_test.go
  modified:
    - internal/llm/extract.go
    - internal/llm/extract_test.go
    - internal/ontology/emitter/emitter.go

key-decisions:
  - "39-04 JSONObjectResponseFormat placed end-of-file in internal/llm/extract.go, below ExtractVerdictJSONWithStructuredOutput — keeps the T-P12-7 sha256 line-range baseline (threat_mitigations_test.go:425, lines 213-265) byte-stable; inserting above tripped the tamper guard and was relocated"
  - "39-04 ONT-13 NOT marked complete: this plan delivers the KG half (ABox write path); the Milvus indexing half lands in Plans 05/06 — REQUIREMENTS checkbox stays unchecked until the pipeline writes to both stores"
  - "39-04 Oversized-note union asserts N section calls (one per section), not the acceptance text's literal 'N+1' — the action spec (per-section calls, no full-note call) is authoritative"
  - "39-04 F1 conventions pinned: zero denominator yields 0.0 for that metric; kind secondary check bites only when both sides specify kind; related_notes scored on source_idx sets, relation tag unscored"

patterns-established:
  - "Extractor seam: ecl.Extractor interface; new extractors implement ExtractNote(ctx, Note) (Extraction, error)"
  - "ecl error voice: 'ecl: extract:' / 'ecl: abox:' prefixes, zero partial results on error"
  - "White-box option introspection in tests via model.GetCommonOptions / model.GetImplSpecificOptions[map[string]any]"

requirements-completed: [ONT-12]

# Metrics
duration: 23 min
completed: 2026-09-22
---

# Phase 39 Plan 4: Extraction Layer + ABox Write Path Summary

**Extractor seam (LLM temp-0/json_object + regex stub) with 40K-rune per-section union guardrail, pure macro-F1 math over the D-GT-01 golden jsonl, and idempotent note-level ABox replace with orphan sweep and snapshot trigger**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-22T01:55:53Z
- **Completed:** 2026-09-22T02:18:07Z
- **Tasks:** 3 (all TDD: RED → GREEN)
- **Files modified:** 9 (6 created, 3 modified)

## Accomplishments
- Extraction seam complete: `ecl.Extractor` interface with `LLMExtractor` (induction call posture: temp-0, response_format json_object via newly exported `llm.JSONObjectResponseFormat`, `llm.ExtractJSON` defensive parse, strict unmarshal, `ValidateExtraction` gate) and the Plan 03 regex stub adapted via `StubExtractFunc` — compile-time pinned. `NewLLMExtractor(chat, sectionCap ...int)` signature frozen for Plan 06.
- Prompt-size guardrail proven: bodies over the 40K-rune default cap extract per-section (exactly N calls for N sections — no full-note call) and union results with first-kind-wins table dedupe and source_idx+relation related dedupe.
- SC-2 harness math pinned pure: `LoadGolden` (filename:line errors, never silent), `ScoreTables` (name sets, kind secondary check), `ScoreRelated`, `MacroF1` (absent notes score zero, never skipped) — Plan 09 calls these unmodified.
- ONT-13 KG half complete: `BuildTriples` emits exactly the D-Ext-01 forms (no belongsToNote / related_notes triples); `ApplyNote` is idempotent (Len stable across re-runs), isolated per note (other notes' triples untouched), orphan-cleaning (solely-referenced tables fully removed incl. TableSchema type triple; shared tables survive via `InsertUnique`), and fires the debounced-snapshot `onChange` exactly once per successful apply.

## Task Commits

Each task was committed atomically (TDD: test commit → feat commit):

1. **Task 1: Extractor interface + LLM impl** — `2dda7e9` (test) + `32ba95f` (feat); guard fix `4c0e52f` (fix)
2. **Task 2: F1 math + golden loader** — `fb1e1a7` (test) + `f251615` (feat)
3. **Task 3: ABox build + note-level replace** — `7a5290d` (test) + `5e7540f` (feat)

## Files Created/Modified
- `internal/knowledge/ecl/extract.go` — Extractor interface, LLMExtractor, StubExtractFunc, D-Ext-01 prompt
- `internal/knowledge/ecl/extract_test.go` — fake Generator with option-introspection posture asserts
- `internal/knowledge/ecl/f1.go` — golden loader + pure set-similarity math
- `internal/knowledge/ecl/f1_test.go` — table-driven math + loader error cases
- `internal/knowledge/ecl/abox.go` — NoteIRI/TableIRI/BuildTriples/ApplyNote
- `internal/knowledge/ecl/abox_test.go` — idempotency/isolation/orphan/error-path tests on real store.Store
- `internal/llm/extract.go` — +JSONObjectResponseFormat export (end-of-file placement, see decisions)
- `internal/llm/extract_test.go` — +TestJSONObjectResponseFormatOption
- `internal/ontology/emitter/emitter.go` — +SanitizeLocal exported wrapper

## Decisions Made
- ONT-13 left unchecked in REQUIREMENTS.md — this plan delivers its KG half only; Milvus half is Plans 05/06. ONT-12 checked (extraction machinery complete and hermetically tested).
- JSONObjectResponseFormat relocated below ExtractVerdictJSONWithStructuredOutput rather than updating the T-P12-7 baseline hash — zero churn to a security tamper baseline beats re-baselining it.
- Oversized-note test asserts N per-section calls (the action spec's contract) rather than the acceptance text's "N+1" phrasing, which does not match the specified behavior.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] T-P12-7 sha256 guard tripped by extract.go insertion**
- **Found during:** Task 1 (full-suite regression sweep after GREEN)
- **Issue:** Adding JSONObjectResponseFormat above ExtractVerdictJSONWithStructuredOutput shifted the function out of the guard's pinned line range (213-265 of internal/llm/extract.go); TestThreatModelCriticalMitigations/T-P12-7 failed with a baseline-hash mismatch.
- **Fix:** Relocated the wrapper to end-of-file, restoring the guarded span byte-identically; placement rationale documented in the godoc. No baseline hash touched.
- **Files modified:** internal/llm/extract.go
- **Verification:** TestThreatModelCriticalMitigations passes; full `go test ./...` suite green (single fixed failure — no other package affected)
- **Committed in:** 4c0e52f

---

**Total deviations:** 1 auto-fixed (1 bug, Rule 1)
**Impact on plan:** None on scope — the fix restores a pre-existing guard, does not alter it.

## Issues Encountered
- `.planning/config.json` carried an orchestrator-side `_auto_chain_active: true→false` residue on disk; left uncommitted (outside this plan's declared files).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 05 (manager orchestrator) can call ExtractNote + ApplyNote per note and wire its own debounce fn into ApplyNote's onChange parameter; ecl_failures ledger and stage orchestration remain.
- Plan 06 calls `ecl.NewLLMExtractor(cm)` — signature frozen and tested.
- Plan 09's golden harness calls LoadGolden/ScoreTables/ScoreRelated/MacroF1 unmodified; golden jsonl does not exist yet (Plan 09 Task 1 drafts it).

## Self-Check: PASSED

- All 6 created files verified on disk (`[ -f ]`)
- All 8 commits verified in git log (`2dda7e9 32ba95f 4c0e52f fb1e1a7 f251615 7a5290d 5e7540f ed2059a`)
- Plan verification re-run green: `go test ./internal/knowledge/ecl/ ./internal/llm/ ./internal/ontology/... -count=1` all ok
- Full-suite sweep `go test ./... -count=1`: zero failures (post 4c0e52f guard fix)

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
