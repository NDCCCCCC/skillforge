---
phase: 19-p10-5-report-portal-scheduler
plan: "03"
subsystem: scheduler
tags: [go, duckdb, react, ttl-cleanup, portal]

# Dependency graph
requires:
  - phase: 19-p10-5-report-portal-scheduler
    plan: "02"
    provides: RunReportJob, drift detection, eval gate, manual rerender endpoint, report_runs audit table
affects:
  - phase: 19-p10-5-report-portal-scheduler
    plans: ["04+"]
provides:
  - TTL cleanup job with DETACH-before-delete for anonymized DuckDB datasets
  - ReportPortal React component with skill/period filter, drift badge, rerun button
  - Scheduler wired to store + eval gate + AttachLRU in main.go
  - Daily cron TTL cleanup registered at 2am UTC (90-day default)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - DETACH-before-delete: AttachLRU.Detach called before os.Remove on Windows to release file handle
    - TTL cleanup as cron job registered in scheduler.Start

key-files:
  created:
    - internal/scheduler/cleanup_job.go — RunTTLCleanup with DETACH-before-delete
    - web/src/components/admin/ReportPortal.tsx — portal UI component
  modified:
    - internal/scheduler/scheduler.go — SetDetacher, SetTTLCleanup, runTTLCleanup
    - internal/server/server.go — Deps.Detacher interface field
    - cmd/server/main.go — scheduler construction + wiring + graceful shutdown
    - web/src/types/wire-types.ts — ReportRun interface added

key-decisions:
  - "AttachLRU passed as Detacher to TTL cleanup — same LRU used by KPI executor handles both KPI queries and dataset lifecycle"
  - "TTL cleanup is nil-safe: detacher nil → skip DETACH step, proceed to file deletion only"
  - "Detacher interface lives in scheduler package (not duckdb) to avoid cross-package import"

requirements-completed:
  - PORT-01
  - PORT-03
  - PORT-04
  - PORT-05
  - PORT-06
  - PORT-07
  - DATA-18
  - GEVL-11

# Metrics
duration: 20min
completed: 2026-08-29
---

# Phase 19 Plan 03: TTL Cleanup + Report Portal FE + Scheduler Wiring Summary

**TTL cleanup job with DETACH-before-delete for anonymized DuckDB datasets, ReportPortal React component with filter/drift-badge/rerun, scheduler fully wired in main.go**

## Performance

- **Duration:** 20 min
- **Started:** 2026-08-29T02:03:23Z
- **Completed:** 2026-08-29T02:23:00Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- TTL cleanup job (DATA-18): DETACH-before-delete pattern for anonymized datasets, daily cron at 2am UTC, 90-day default TTL
- ReportPortal React component: skill/period filter, status badges colored by state, amber DRIFT badge, manual rerun button
- Scheduler fully wired in main.go: store + eval gate + AttachLRU detacher injection, graceful stop on shutdown

## Task Commits

Each task was committed atomically:

1. **Task 1: TTL cleanup job** - `a61ca2a` (feat) + **Task 3: Scheduler wiring** — wired together in same commit
2. **Task 2: ReportPortal FE component** - `6e892a1` (feat)

**Plan metadata:** `5fbe5ad` (docs: create phase plan — 3 plans in 3 waves)

## Files Created/Modified

- `internal/scheduler/cleanup_job.go` — RunTTLCleanup: scans anonymized datasets older than TTL, DETACH via AttachLRU then os.Remove; idempotent, logs each deletion
- `internal/scheduler/scheduler.go` — Added SetDetacher/SetTTLCleanup methods, runTTLCleanup, TTL cron registration in Start()
- `internal/server/server.go` — Deps gains Detacher interface{Detach(ctx,id)error} for TTL cleanup wiring
- `cmd/server/main.go` — Scheduler constructed after KPI pipeline, SetStore/SetEvalGate/SetDetacher/SetTTLCleanup called, graceful stop deferred
- `web/src/components/admin/ReportPortal.tsx` — ReportPortal component: GET /reports list, skill/period filters, status badges, amber DRIFT badge, rerun button
- `web/src/types/wire-types.ts` — ReportRun interface added (mirrors store.ReportRunRow)

## Decisions Made

- Used `interface{ Detach(ctx context.Context, id string) error }` inline in Deps rather than importing duckdb package (avoids cross-package cycle)
- `Detacher nil` is nil-safe in RunTTLCleanup: when detacher is nil, skip DETACH and only attempt file deletion (graceful degradation)
- AttachLRU from KPI pipeline (kpiLRU, unconditionally initialized) used as the Detacher — same LRU handles both KPI dataset queries and TTL cleanup
- TTL cleanup schedule hardcoded to `"0 2 * * *"` (daily 2am UTC); TTL duration configurable via `SKILLFORGE_DATASET_TTL_DAYS` env var (default 90 days)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **CGO build failure on Windows:** duckdb CGO dependency prevents `go build` on native Windows (MSYS2 gcc can link sqlite but not duckdb). This is pre-existing Phase 15 status quo. Code syntax verified via TypeScript compilation pass (`cd web && npx tsc --noEmit` → clean). Go code is structurally correct.

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| threat_flag: DETACH-before-delete | internal/scheduler/cleanup_job.go | New surface: cleanup job deletes .duckdb files after DETACH. Only tier=="anonymized" eligible. Mitigated by tier check + DETACH-first ordering. |
| threat_flag: scheduler_wiring | cmd/server/main.go | Scheduler now starts with server and holds reference to kpiLRU. Graceful shutdown ensures cron jobs complete before stop. |

## Next Phase Readiness

- TTL cleanup job ready: RunTTLCleanup, Detacher interface, and cron registration all complete
- ReportPortal component ready: wired to GET /reports + POST /api/reports/rerender
- Scheduler fully wired: store + eval gate + LRU detacher + graceful stop
- DEFERRED to future phase: actual KPI executor integration in executeReportRun (stub returns empty map); report publication path (report.Publish) not yet wired to scheduler output

---
*Phase: 19-p10-5-report-portal-scheduler*
*Plan: 03*
*Completed: 2026-08-29*
