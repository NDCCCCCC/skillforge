---
phase: 39-ecl-pipeline
plan: 07
subsystem: api
tags: [hertz, admin-api, ecl-pipeline, dead-letter, sse-free-polling, kg, milvus]

# Dependency graph
requires:
  - phase: 39-05-ecl-manager
    provides: ecl.Manager (Run/CASRunning/Status/RetryNote), ErrPipelineBusy/ErrUnknownNote sentinels, StatusDTO + Failure json tags, FailureStore seam
  - phase: 39-06-ecl-boot-integration
    provides: Deps.ECL field pre-added in server.Deps + ecl import, armed manager at boot
  - phase: 38-foundation-tbox-kg
    provides: admin_ontology.go gate/handler/envelope pattern, requireAdmin, graphResponseError
provides:
  - POST /api/admin/pipeline/run — token-gated fire-and-forget trigger (409 double-run via CASRunning, detached goroutine on context.Background, WARN on failed run, immediate 200 {"status":"started"})
  - GET /api/admin/pipeline/status — D-Prog-01 polling feed, StatusDTO marshaled verbatim
  - POST /api/admin/pipeline/retry/:note_id — dead-letter manual trigger (400 format gate / 404 unknown-or-non-dead / 409 busy / 200 queued)
  - requireECL gate with LOCKED 403-before-503 order (admin_ontology pattern)
  - TestAdminPipeline* suite (5 groups) — the regression guard Plan 09's verify script greps
affects: [39-08-admin-pipeline-tab, 39-09-verify-script, phase-40-governance]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "requireECL: requireAdmin 403 fires BEFORE the nil-manager 503 (locked order, D-Route-01/T-39-19)"
    - "fire-and-forget admin run: CAS claim → detached goroutine on context.Background → immediate 200; progress via status polling only (D-Prog-01)"
    - "wire DTOs serialized verbatim across package seams (owner package keeps the json tags — no re-mapping)"
    - "defense-in-depth path-param format gate (^[0-9]{3}$) before semantic manager lookup (T-39-10)"

key-files:
  created:
    - internal/server/admin_pipeline.go
    - internal/server/admin_pipeline_test.go
  modified:
    - internal/server/server.go

key-decisions:
  - "39-07 run handler: CASRunning claim in-handler → goroutine on context.Background (run outlives request lifecycle; failed run WARN-logs via log.Printf, never silent) → immediate 200 started"
  - "39-07 status handler marshals ecl.StatusDTO verbatim — Plan 05 owns the json tags; no field-by-field re-mapping (single source of truth)"
  - "39-07 retry: ^[0-9]{3}$ format gate answers 400 BEFORE the manager is consulted; then errors.Is maps ErrPipelineBusy→409 / ErrUnknownNote→404 with pinned machine-stable strings (FE owns the zh copy)"
  - "39-07 all three routes body-less — no BindAndValidate anywhere (D-Route-01; no invented body schema)"
  - "39-07 test fakes rebuilt in the external server test package (unexported manager backoff untouched; all-succeeding stubs mean zero backoff sleeps); seed signals a channel as the deterministic run-executed witness"

patterns-established:
  - "Pattern: ECL admin gate = requireAdmin → nil-ECL 503 → handler logic; reuse for any future ecl-adjacent admin route"
  - "Pattern: external-package hermetic manager tests = temp-dir corpus + map FailureStore + stub Extractor + channel-signaling SeedFunc + SetIDAllocator sequence"

requirements-completed: [ONT-13]

# Metrics
duration: 6min
completed: 2026-09-22
---

# Phase 39 Plan 07: ECL Admin API Surface Summary

**Three token-gated /api/admin/pipeline/* routes (run/status/retry) with the locked 403-before-503 order, CAS 409 double-run semantics, dead-letter retry validation, and the D-Prog-01 status DTO — the server contract Plan 08's FE tab consumes verbatim.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-09-22T03:34:05Z
- **Completed:** 2026-09-22T03:40:28Z
- **Tasks:** 2
- **Files modified:** 3 (1 handler file + 1 test file created, server.go extended)

## Accomplishments
- POST /api/admin/pipeline/run: requireECL → CASRunning (409 on double-run, T-39-20) → detached goroutine on context.Background (run outlives the request; failed run WARN-logs) → immediate 200 {"status":"started"} (D-Prog-01: progress via polling only)
- GET /api/admin/pipeline/status: StatusDTO serialized verbatim — exact locked key set {stage, current_file, done, total, queue_len, failures[]} with failure objects {note_id, stage, attempt_count, last_error, status, updated_at}
- POST /api/admin/pipeline/retry/:note_id: ^[0-9]{3}$ format gate (400, T-39-10 layer 2) before the manager; ErrPipelineBusy → 409, ErrUnknownNote → 404, success → 200 queued — full-note idempotent re-run per D-Run-01
- 5-group hermetic test suite (TestAdminPipeline*) including the 403-with-nil-ECL regression guard and exact-key-set shape assertions; race detector + full server suite green

## Task Commits

Each task was committed atomically:

1. **Task 1: admin_pipeline.go — gate + 3 handlers + route registration** - `6785575` (feat)
2. **Task 2: Handler test suite (gate order, DTO shape, 409/404)** - `425b10f` (test)

## Files Created/Modified
- `internal/server/admin_pipeline.go` — requireECL gate + run/status/retry handlers + route godoc wire contract
- `internal/server/server.go` — 3 route registrations in the admin block beside the ontology routes (Deps.ECL field pre-added by 39-06, reused not redeclared)
- `internal/server/admin_pipeline_test.go` — 5 test groups over a 3-note temp corpus with map-backed FailureStore, stub extractor, channel-signaling seed

## Decisions Made
- Run handler claims CASRunning inside the handler and launches Run on context.Background — the request ctx would cancel the run when the HTTP response flushes; a failed background run WARN-logs via the server package's log.Printf convention
- Status handler returns mgr.Status() directly — re-mapping fields would create a second source of truth for the UI-SPEC wire contract
- Retry validation is two-layer: the regex gate (400) kills malformed/traversal-shaped ids at the door; the manager's DEAD-row requirement (404) stays the semantic authority — consistent with T-39-10's layering
- Test fakes replicated in the external test package rather than exported from ecl: the manager's unexported backoff never needs touching because all-succeeding stubs produce zero retry sleeps
- Fixture corpus carries notes 001/002/007 — 007 must exist on disk so the dead-letter retry happy path can walk it back out of the corpus

## Deviations from Plan

None - plan executed exactly as written. (Deps.ECL was already pre-added to server.go by the 39-06 executor per its recorded deviation — this plan used the existing field as instructed and only added the route registrations.)

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 08 (FE PipelineTab) can code against this exact contract: pinned error strings ("admin token required" / "ECL pipeline not wired" / "pipeline already running" / "invalid note id" / "unknown or non-dead note") map 1:1 to the UI-SPEC zh copy; stage sentinels idle/merge/extract/kg-load/milvus-load/done flow through verbatim
- Plan 09's verify script can grep TestAdminPipeline{GateOrder,RunConflict,StatusShape,RetryValidation,RunStarts}
- SC-4/SC-5 server half complete; the run/status/retry semantics are regression-guarded

## Self-Check: PASSED

- internal/server/admin_pipeline.go exists (committed 6785575)
- internal/server/admin_pipeline_test.go exists (committed 425b10f)
- server.go route registrations present (grep /api/admin/pipeline/run → hit)
- go build ./... + go vet ./internal/server/ clean
- go test ./internal/server/ -count=1 → ok (full suite, no regressions)
- TestAdminPipeline* suite: 5/5 PASS; -race → ok

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
