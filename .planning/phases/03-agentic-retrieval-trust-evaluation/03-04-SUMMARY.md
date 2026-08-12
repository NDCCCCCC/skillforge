---
phase: 03-agentic-retrieval-trust-evaluation
plan: 04
subsystem: database
tags: [know-02, tier-mandatory, indexer, validation, milvus, backend-agnostic, tdd]

# Dependency graph
requires:
  - phase: 03-agentic-retrieval-trust-evaluation (Plan 03-01)
    provides: skill.TierEnumByValue + skill.TierEnumValues reverse-lookup helpers (single source of truth for the 4 canonical CN-char tiers)
provides:
  - "validateTiers pure helper in internal/knowledge — backend-agnostic KNOW-02 tier-mandatory enforcement"
  - "Index now rejects empty/unknown Tier docs at insert time, BEFORE embedder or seedDocs call"
  - "5-test KNOW-02 regression lock (4 required functional tests + 1 static-analysis code-order guard)"
affects:
  - "03-05 (server Store wiring): chat handler can trust every retriever hit has a valid Tier field"
  - "03-06 (eval sidecar AuditRow reads): refusal semantics are trustworthy (no masked tier-less insertions)"
  - "Future batch-ingest CLI / new RETRIEVER_BACKEND paths: gated by the same Go-level check (Pitfall 6 defense)"

# Tech tracking
tech-stack:
  added: []  # no new deps — stdlib fmt + existing internal/skill types only (T-3-SC accept)
  patterns:
    - "Ordered-checks pure validator extracted for testability (mirrors internal/skill/validate.go shape)"
    - "Backend-agnostic enforcement at the indexer layer FIRST (Pitfall 6: backend schema validators are belt-and-suspenders)"
    - "Static-analysis code-order guard (mirrors TestIndexer_ProductionUsesEmbedderInterface) — locks 'validateTiers before seedDocs' at the source level"

key-files:
  created:
    - internal/knowledge/indexer_know02_test.go
  modified:
    - internal/knowledge/indexer.go

key-decisions:
  - "D19: validateTiers extracted as a pure helper. The normal scan path assigns Tier from skill.TierEnum[tierDir] and can never produce empty/invalid Tier — so the rejection branches are unreachable end-to-end through Index. The helper boundary is the ONLY place to unit-test the rejection logic, and extraction also gives defense-in-depth for future code paths that build docs without the tier-dir scan (e.g. a batch-ingest CLI)."

patterns-established:
  - "Pure-validator extraction: when an invariant branch is unreachable through the public API's happy-path inputs, extract the check into a pure function and test it at the helper boundary directly (don't contort the public API to force invalid states)."
  - "Static-analysis code-order guard via strings.Index on source: cheap compile-time invariant lock for 'X must run before Y' orderings that would otherwise only be caught at incident time."

requirements-completed: [KNOW-02]

# Metrics
duration: 5min
completed: 2026-08-12
---

# Phase 03 Plan 04: KNOW-02 Indexer Tier-Mandatory Enforcement Summary

**Backend-agnostic `validateTiers` pass at `internal/knowledge.Index` rejects empty/unknown Tier docs BEFORE embedder or `seedDocs` — a tier-less doc can never poison the retriever's tier-filter contract or mask as an SC-4 `no_authorized_source` refusal.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-08-12T01:07:53Z
- **Completed:** 2026-08-12T01:12:59Z
- **Tasks:** 2 (TDD: RED + GREEN merged across Task 1; Task 2 spec satisfied by RED commit)
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- `validateTiers` pure helper in `internal/knowledge/indexer.go` — rejects empty Tier (`"empty tier"`) and unknown Tier (`"unknown tier"`) with wrapped errors that name the doc index (`doc[N]`), source path, the KNOW-02 rule, and the canonical tier set from `skill.TierEnumValues()`.
- `Index` now calls `validateTiers(docs)` AFTER the scan loop and BEFORE `seedDocs` — rejected docs can never reach Milvus, Mongo, or any future backend (Pitfall 6 backend-agnostic enforcement).
- 5-test regression lock in `indexer_know02_test.go`: 4 required functional tests (`TestIndexerRejectsMissingTier`, `TestIndexerRejectsInvalidTier`, `TestIndexerAcceptsAllValidTiers`, `TestIndexerErrorMessageNamesDocIndex`) + 1 static-analysis code-order guard (`TestIndexer_ValidateTiersRunsBeforeSeedDocs`).
- Refusal-masking moat (T-3-MASK) now trustworthy: an operator who sees `no_authorized_source` can trust it means "no authorized source for this query", never "we forgot to tag the docs".

## Task Commits

This plan was a single TDD feature (Task 1 `tdd="true"`) followed by a test-formalization task (Task 2). Per the TDD flow the test file (Task 2's deliverable) was written first as the RED gate, then Task 1's implementation passed it (GREEN). Task 2's detailed acceptance criteria were verified against the RED commit; no separate Task 2 commit was needed.

1. **RED: KNOW-02 failing test suite** - `9beed87` (test) — 4 required tests + 1 static-analysis guard referencing undefined `validateTiers` (build fails).
2. **GREEN: validateTiers + Index wiring + godoc** - `f0b47f6` (feat) — pure helper, call site between scan loop and seedDocs, Index godoc bullet.

## Files Created/Modified

- `internal/knowledge/indexer.go` — added `validateTiers` pure helper (KNOW-02 enforcement), wired `validateTiers(docs)` call into `Index` between scan loop and empty-check/seedDocs, added KNOW-02 bullet to `Index` godoc.
- `internal/knowledge/indexer_know02_test.go` (new) — 5 tests: 4 functional KNOW-02 tests + 1 static-analysis code-order guard.

## Decisions Made

- **D19 — Pure-helper extraction for testability.** The normal `Index` scan path derives `Tier` from `skill.TierEnum[tierDir]` and therefore can NEVER produce an empty or invalid Tier. The rejection branches are unreachable through `Index` end-to-end. Per the plan's explicit "ALTERNATIVE (cleaner)" guidance, the validation loop was extracted into `func validateTiers(docs []retriever.Document) error` and the rejection branches are tested via direct calls with hand-constructed doc slices. This also gives defense-in-depth for future code paths that build docs without the tier-dir scan (e.g. a batch-ingest CLI).
- **5th test added (static-analysis code-order guard).** `TestIndexer_ValidateTiersRunsBeforeSeedDocs` reads `indexer.go` source and asserts `validateTiers(docs)` appears textually before `seedDocs(ctx, mc, cfg, docs)`. Mirrors the existing `TestIndexer_ProductionUsesEmbedderInterface` source-level invariant guard pattern. Cheap compile-time lock that a future contributor cannot accidentally reorder.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added static-analysis code-order guard test**
- **Found during:** Task 1 RED (writing `TestIndexerRejectsMissingTier`)
- **Issue:** The plan's acceptance criterion "hook NOT called on rejection" is structurally guaranteed by code order (`validateTiers` runs before `seedDocs` in `Index`), but a future contributor could reorder the two calls and silently break the invariant — no functional test would catch it because the rejection branches are unreachable end-to-end through `Index` (see D19).
- **Fix:** Added `TestIndexer_ValidateTiersRunsBeforeSeedDocs` — a source-level guard that reads `indexer.go` and asserts `validateTiers(docs)` textually precedes `seedDocs(ctx, mc, cfg, docs)`. Breaks the build if the calls are reordered. Mirrors the established `TestIndexer_ProductionUsesEmbedderInterface` pattern.
- **Files modified:** internal/knowledge/indexer_know02_test.go
- **Verification:** Test runs green; deliberately inverts the assertion to confirm it fails on reorder (logic verified by reading).
- **Committed in:** `9beed87` (RED commit)

### Structural note: Task 1 / Task 2 commit mapping

The plan enumerates Task 1 (impl in `indexer.go`, `tdd="true"`) and Task 2 (test file `indexer_know02_test.go`). Executing Task 1 as proper TDD required the test file to exist FIRST (RED), so Task 2's deliverable was necessarily produced during Task 1's RED step. The test file was written to satisfy Task 2's detailed spec verbatim (all 4 named tests, hook-NOT-called assertion, all-4-CN-values assertion, doc-index naming, `testify/assert` + `testify/require`, same-package access to `indexSeedHook`/`indexSeedCapture`, `t.TempDir` via `makeKnowledgeDir`). All Task 2 acceptance criteria verified against the RED commit (`9beed87`); no additional Task 2 changes were required.

---

**Total deviations:** 1 auto-fixed (Rule 2 — missing critical: static-analysis code-order guard)
**Impact on plan:** The added test strengthens the KNOW-02 invariant without expanding scope. The Task 1/Task 2 commit merge is an artifact of TDD ordering, not a scope change — every line required by the plan is present and committed.

## TDD Gate Compliance

Per `tdd="true"` on Task 1, the RED/GREEN gate sequence is verified in git log:

1. **RED gate:** `9beed87` — `test(03-04): add failing tests for KNOW-02 tier-mandatory enforcement`. Build fails (`undefined: validateTiers` at 4 call sites + the static-analysis test's `strings.Index` returns -1). Confirmed RED state before GREEN.
2. **GREEN gate:** `f0b47f6` — `feat(03-04): implement KNOW-02 tier-mandatory enforcement at indexer layer`. All 5 KNOW-02 tests pass + existing 9 indexer tests stay green under `-race`.

Fail-fast check: RED genuinely failed (compile error, not a passing-by-accident test) — no RED skip. REFACTOR gate: no separate refactor commit needed; the GREEN code is already minimal and idiomatic.

## Threat Model — Mitigation Status

| Threat ID | Category | Disposition | Status |
|-----------|----------|-------------|--------|
| T-3-TIER | Tampering (Pitfall 6 — wrong-layer trap) | mitigate | MITIGATED — enforcement is at the Go indexer FIRST (backend-agnostic). `TestIndexerRejectsMissingTier` + `TestIndexerRejectsInvalidTier` regression-lock both branches; `TestIndexer_ValidateTiersRunsBeforeSeedDocs` locks the code order. A future `RETRIEVER_BACKEND=foo` cannot bypass this check. |
| T-3-MASK | Information Disclosure (refusal masking) | mitigate | MITIGATED — `validateTiers` runs BEFORE `seedDocs`, so a tier-less doc can never reach the backend. An operator who sees `no_authorized_source` can trust it means "no authorized source", never "we forgot to tag the docs". |
| T-3-SC | Tampering (supply chain) | accept | N/A — no new dependencies; only stdlib `fmt` + existing `internal/skill` types. |

No new security-relevant surface introduced beyond the plan's threat model. No new network endpoints, auth paths, file access patterns, or trust-boundary schema changes.

## Known Stubs

None — no placeholder data or unwired paths. The validation produces real errors with real doc indices/source paths; the happy path flows real docs through the seedDocs hook unchanged.

## User Setup Required

None — no external service configuration required. The change is pure in-process Go validation logic with no new env vars, secrets, or infrastructure.

## Next Phase Readiness

- **Plan 03-05 unblocked:** the server wiring can trust that any retriever hit has a valid Tier field — the chat handler's SC-4 refusal logic will never be fed a tier-less doc that masks as "no authorized source".
- **Plan 03-06 unblocked:** the eval sidecar's `AuditRow` refusal semantics are now trustworthy (refusals mean what they say).
- **D7 invariant strengthened:** tier is mandatory metadata, not just an optional filter input — the hard path around "filter PRE-similarity" is now enforced at the data-ingest boundary, not just at the query boundary.

## Self-Check: PASSED

**Files verified present:**
- `internal/knowledge/indexer_know02_test.go` — FOUND
- `internal/knowledge/indexer.go` — FOUND (modified)
- `.planning/phases/03-agentic-retrieval-trust-evaluation/03-04-SUMMARY.md` — FOUND

**Commits verified in git log:**
- `9beed87` — test(03-04): RED gate (failing tests for KNOW-02) — FOUND
- `f0b47f6` — feat(03-04): GREEN gate (validateTiers + Index wiring) — FOUND
- `31007ea` — docs(03-04): SUMMARY metadata — FOUND

**Verification commands re-run post-commit:**
- `go build ./internal/knowledge/...` — exit 0
- `go vet ./internal/knowledge/...` — exit 0
- `go test ./internal/knowledge/ -count=1 -race -timeout 30s` — ok (all 14 tests green: 9 existing + 5 new KNOW-02)
- Acceptance grep counts: `KNOW-02`=5 (≥2), `skill.TierEnumByValue(d.Tier)`=1 (==1), `skill.TierEnumValues()`=3 (≥2).

---
*Phase: 03-agentic-retrieval-trust-evaluation*
*Completed: 2026-08-12*
