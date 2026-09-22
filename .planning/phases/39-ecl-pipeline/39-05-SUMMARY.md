---
phase: 39-ecl-pipeline
plan: 05
subsystem: database
tags: [sqlite, dead-letter, milvus, chunking, eino-ext, splitter, orchestrator, retry, kg]

# Dependency graph
requires:
  - phase: 39-02
    provides: retriever.Document 11-field schema (+source_idx/section/section_idx/section_count), milvus DeleteByExpr/GetMaxIDCollection watermark
  - phase: 39-04
    provides: ecl Extractor seam + NewLLMExtractor, abox ApplyNote note-level KG replace with onChange hook, f1 golden loader
provides:
  - "SQLite ecl_failures dead-letter ledger (idempotent DDL + narrow FailureStore: UpsertFailure/ListFailures/GetFailure, restart-durable)"
  - "ecl.BuildDocuments: merged sections → Milvus-ready Documents with recursive sub-split at MinSplitRunes=3500 (Pitfall 2 designed out)"
  - "ecl.Manager stage orchestrator: Run/Status/RetryNote/CASRunning/SetIDAllocator, stage sentinels idle|merge|extract|kg-load|milvus-load|done, per-note retry ×3 → dead-letter, locked StatusDTO wire shape"
  - "SeedFunc seam (ecl pkg has zero milvus imports) — Plan 06 injects purge+watermark+embed+SeedDocuments"
affects: [39-06 (boot wiring), 39-07 (admin HTTP surface), 39-08 (FE PipelineTab), 39-09 (verify script)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Narrow FailureStore interface implemented by sqlite store, map-backed fake in manager_test (induction.Generator precedent)"
    - "Injectable seams for hermetic stage-failure tests: applyNote func field (kg-load), SeedFunc (milvus-load), backoff slice, idAlloc"
    - "Splitter separator ladder terminating with \"\" so eino recursive splitter can never pass through an oversized chunk"
    - "Mutable-status SQLite table (ecl_failures) — sanctioned non-audit exception with DDL-anchored rationale comment"

key-files:
  created:
    - internal/knowledge/ecl/failures.go
    - internal/knowledge/ecl/chunk.go
    - internal/knowledge/ecl/manager.go
    - internal/store/sqlite/ecl_failures.go
    - internal/knowledge/ecl/failures_test.go
    - internal/knowledge/ecl/chunk_test.go
    - internal/knowledge/ecl/manager_test.go
  modified:
    - internal/store/sqlite/schema.go
    - go.mod
    - go.sum

key-decisions:
  - "Document.SectionIdx emitted 1-BASED (plan text read literally would copy merge's 0-based index) — Plan 39-02's retriever.Document contract is the downstream lock: 1-based for ECL rows, 0 = legacy zero-fill sentinel (seed.go:43)"
  - "done/total/queue_len are PER-STAGE counters: done resets at each stage entry, total stays len(notes); queue_len = stage universe minus done"
  - "RetryNote claims the running flag for its synchronous execution — a concurrent Run's CASRunning then correctly answers busy (409)"
  - "RetryNote on a ledger row whose note vanished from the corpus: parks the row dead with the explanation and returns ErrUnknownNote"
  - "milvus-load retry writes per-note pending rows per failed batch attempt; final failure parks all batch notes dead at milvus-load; success resolves (ledger stays per-note per planner pin)"
  - "Splitter config: rune-counting LenFunc, CJK-aware separator ladder, \"\" final separator = hard cap guarantee; BuildDocuments post-condition errors on any part > 3500 runes"
  - "failures_test.go lives in external ecl_test package: sqlite imports ecl for the compile-time conformance check, and only an external test package can import it back without a cycle"

patterns-established:
  - "Dead-letter ledger pattern: one row per work item, mutable status transitions via single upsert write path, restart-durable via schemaSQL"
  - "Stage orchestrator pattern: CAS running flag + stage sentinels + per-stage status counters + defer release"

requirements-completed: [ONT-12]

# Metrics
duration: 28 min
completed: 2026-09-22
---

# Phase 39 Plan 05: ECL Pipeline Core (ledger + chunker + orchestrator) Summary

**SQLite ecl_failures dead-letter ledger behind a narrow FailureStore, the section→Document chunker with recursive sub-split of oversized sections (3500-rune cap), and the Manager state machine running merge → extract → kg-load → milvus-load with per-note retry ×3, batch isolation, idempotent re-enqueue, and the locked status DTO.**

## Performance

- **Duration:** 28 min
- **Started:** 2026-09-22T02:23:58Z
- **Completed:** 2026-09-22T02:52:03Z
- **Tasks:** 3
- **Files modified:** 11 (7 created, 4 modified)

## Accomplishments
- ecl_failures DDL (note_id PK / stage / attempt_count / last_error / status / updated_at) in schemaSQL with the D-DL-01 mutable-status sanction documented at the DDL; sqlite impl (single upsert write path, updated_at set by caller) proven restart-durable and migrate-idempotent
- BuildDocuments turns merged sections into Milvus-ready Documents: identical section metadata across split parts, rune-counting recursive sub-split whose separator ladder terminates with "" (the splitter's pass-through branch is unreachable), hard post-condition error over 3500 runes — Pitfall 2's silent truncateRunes loss is designed out (T-39-13)
- Manager orchestrator: CAS running flag, LOCKED stage sentinels, extract worker pool capped at 3 concurrent LLM calls, per-note retry ×3 (1s/4s/16s backoff, injectable) → dead rows while the batch continues across ALL THREE retry stages (extract / kg-load via injectable applyNote seam / milvus-load via SeedFunc), full-note idempotent RetryNote, per-stage status counters, StatusDTO marshaling the exact 39-UI-SPEC wire keys

## Task Commits

Each task was committed atomically:

1. **Task 1: ecl_failures DDL + FailureStore interface + sqlite impl** - `5e7d1ff` (feat)
2. **Task 2: chunk.go — sections → Documents with recursive sub-split** - `07bca5f` (feat)
3. **Task 3: Manager — stage orchestrator with retry, dead-letter, status** - `365f1f6` (feat)

## Files Created/Modified
- `internal/store/sqlite/schema.go` — ecl_failures DDL appended to schemaSQL (additive, idempotent)
- `internal/store/sqlite/ecl_failures.go` — FailureStore impl: upsert / list (updated_at DESC) / get (wrapped ErrNoFailureRecord) + compile-time conformance check
- `internal/knowledge/ecl/failures.go` — Failure struct with the LOCKED wire json tags + narrow 3-method FailureStore
- `internal/knowledge/ecl/chunk.go` — BuildDocuments + MinSplitRunes=3500 + recursive sub-split
- `internal/knowledge/ecl/manager.go` — Manager, stage sentinels, retry loops, recordFailure, Status/StatusDTO, RetryNote, SeedFunc type
- `internal/knowledge/ecl/failures_test.go` — schema column lock, status lifecycle, list ordering, sentinel miss, restart persistence (external ecl_test package)
- `internal/knowledge/ecl/chunk_test.go` — short/mixed/oversized synthetic notes, cap post-condition, zero `update etl_*` loss across parts, guard rails
- `internal/knowledge/ecl/manager_test.go` — 9 hermetic orchestrator tests incl. all three dead-letter legs, RetryNote guards, DTO wire shape, ≤3 concurrency cap
- `go.mod` / `go.sum` — splitter/recursive promoted indirect→direct via go mod tidy (same version; tidy also corrected other mis-marked indirect→direct entries of already-imported packages and dropped unused indirect requires)

## Decisions Made
- **SectionIdx is 1-based in Documents** — the plan's literal "SectionIdx = section's SectionIdx" would copy merge's 0-based index; Plan 39-02's retriever.Document contract (and milvus seed.go DDL comment: "1-based section position (0 = legacy)") is the downstream lock, so chunk.go emits `int64(s.SectionIdx) + 1`.
- **done/total/queue_len are per-stage counters** — done resets at each stage entry (the FE progress bar tracks the active stage); total stays len(notes) for the whole run. Not pinned by the plan; chosen for coherent progress UX and documented in Status godoc.
- **RetryNote claims the running flag** during its synchronous execution so a concurrent admin Run gets the 409 path (plan specified only "requires !running" at entry).
- **Corpus-missing retry target** → dead row with explanatory error + ErrUnknownNote return (ledger never left pending on a note that cannot re-run).
- **Empty-bodied sections emit no Document** (mirrors legacy indexer's empty-body skip — blank rows would only pollute retrieval).
- **ONT-13 left unchecked in REQUIREMENTS.md** (39-04 precedent): this plan delivers the Milvus doc-build half (consistent-metadata Documents, watermark IDs, SeedFunc seam) but actual seeding into Milvus lands when Plan 06 wires the boot SeedFunc; ONT-12 stays checked from 39-04 (the Manager completes the "system runs extraction" machinery).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Document.SectionIdx emitted 1-based instead of copying merge's 0-based index**
- **Found during:** Task 2 (chunk.go)
- **Issue:** Plan interface text said "SectionIdx = section's SectionIdx" (0-based from MergeFragments); copying literally would collide with D-Milvus-02's zero-fill sentinel semantics and violate the Plan 39-02 retriever.Document contract ("1-based position; 0 for legacy rows", seed.go:43)
- **Fix:** Emit `int64(s.SectionIdx) + 1` with contract-comment; tests assert 1-based values
- **Files modified:** internal/knowledge/ecl/chunk.go, internal/knowledge/ecl/chunk_test.go
- **Verification:** TestBuildDocumentsShortSection / TestBuildDocumentsMixedNotes assert SectionIdx 1 and 2
- **Committed in:** 07bca5f

**2. [Rule 2 - Missing Critical] RetryNote leaves the pipeline visibly busy during its synchronous run**
- **Found during:** Task 3 (manager.go)
- **Issue:** Plan said RetryNote "requires !running" at entry but said nothing about holding the flag; without claiming it, a concurrent admin Run's CASRunning would succeed mid-retry (double-pipeline race on the same KG/store)
- **Fix:** RetryNote claims via CASRunning (ErrPipelineBusy when held) and releases via defer; tests exercise busy + release
- **Files modified:** internal/knowledge/ecl/manager.go
- **Verification:** TestManagerRetryNoteGuards, TestManagerCASRunning
- **Committed in:** 365f1f6

**3. [Rule 3 - Blocking] failures_test.go needed the external ecl_test package**
- **Found during:** Task 1 (test construction)
- **Issue:** The sqlite impl imports ecl (compile-time conformance check); an internal `package ecl` test importing sqlite would form an import cycle in the test binary
- **Fix:** failures_test.go uses `package ecl_test` (the legal import direction); manager_test.go stays internal `package ecl` for recordFailure/seam access — the two coexist in one test binary
- **Files modified:** internal/knowledge/ecl/failures_test.go
- **Verification:** `go test ./internal/knowledge/ecl/` green
- **Committed in:** 5e7d1ff

**Total deviations:** 3 auto-fixed (1 bug, 1 missing-critical, 1 blocking)
**Impact on plan:** All three were correctness/necessity fixes inside the plan's own file list; no scope creep, no API drift from the Plan 06/07 contracts.

## Issues Encountered
- `go doc` for the splitter package could not resolve the document package path initially (GOPATH-mode error under `bash -lc`); resolved by reading the splitter source directly from the module cache — confirmed the pass-through branch (`len(newSeparators) == 0`) that motivates the "" terminal separator.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 06 (boot wiring) binds: `ecl.NewLLMExtractor(cm)`, `ecl.NewManager(ns, kbDir, kg, ex, failureStore, seedFn)`, `SetOnChange(debounce)`, `SetIDAllocator(GetMaxIDCollection counter)`, `SetIDAllocator` + SeedFunc impl (DeleteByExpr marker purge → embed → SeedDocuments), and `sqlite.New` already auto-migrates ecl_failures
- Plan 07 (HTTP) binds: `CASRunning()` → 409, `Status()` → GET status verbatim, `RetryNote` with `ErrPipelineBusy`/`ErrUnknownNote` → 409/404, `Enabled()` gate
- No blockers; full ecl + store suites green, `go build ./...` clean, -race green on Manager tests

## Self-Check: PASSED

- All 7 created files exist on disk; 4 modified files committed
- Commits verified: 5e7d1ff, 07bca5f, 365f1f6 in `git log`
- Plan verification re-run: `go test ./internal/knowledge/ecl/ ./internal/store/... -count=1` green; `go build ./...` clean; `go test -race -run TestManager` green; `grep milvus internal/knowledge/ecl/*.go` shows no package import (sentinel strings only)

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
