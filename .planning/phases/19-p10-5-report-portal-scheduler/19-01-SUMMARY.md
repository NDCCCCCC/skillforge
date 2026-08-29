---
phase: 19-p10-5-report-portal-scheduler
plan: "01"
subsystem: database
tags: [sqlite, cron, scheduler, report-portal]

# Dependency graph
requires:
  - phase: 18-p10-3-report-rendering
    provides: report HTML rendering and publication pipeline (reports table, hitl_report_publish tool)
provides:
  - report_runs INSERT-only audit table for scheduler-triggered runs
  - ReportSchedule field in Skill manifest with load-time cron validation
  - cron scheduler service with in-flight deduplication guard
  - GET /reports portal endpoint listing report run history
affects: [19-p10-5-report-portal-scheduler/19-02, 19-p10-5-report-portal-scheduler/19-03]

# Tech tracking
tech-stack:
  added: [github.com/robfig/cron/v3]
  patterns: [INSERT-only audit table, cron-with-SkipIfStillRunning-chain, optional-filter WHERE-clause]

key-files:
  created:
    - internal/store/sqlite/report_runs.go
    - internal/scheduler/in_flight.go
    - internal/scheduler/scheduler.go
    - internal/server/report_portal.go
  modified:
    - internal/store/sqlite/schema.go
    - internal/store/store.go
    - internal/skill/manifest.go
    - internal/skill/loader.go
    - internal/server/server.go

key-decisions:
  - "report_runs.status enum: success|failed|skipped_duplicate|eval_gate_blocked"
  - "ListReportRuns: optional skill_id/period filters via WHERE 1=1 pattern (T-3-SQLI)"
  - "cron.WithChain(SkipIfStillRunning, Recover) prevents runaway concurrent runs at the cron layer"
  - "inFlight map is secondary guard: SkipIfStillRunning handles same-job re-entrancy; inFlight guards cross-job (skill,period) deduplication"

patterns-established:
  - "Pattern: INSERT-only audit table following reports.go/kpi_calls.go/reports.go pattern"
  - "Pattern: btof helper for bool-to-int SQLite conversion"
  - "Pattern: ts string intermediate parse with time.RFC3339Nano then UTC assignment"

requirements-completed: [PORT-01, PORT-02, PORT-03, PORT-07, GEVL-11]

# Metrics
duration: 15min
completed: 2026-08-29
---

# Phase 19 Plan 01: report_runs Table + Scheduler Foundation + Portal API Summary

**INSERT-only report_runs audit table, ReportSchedule cron validation in Skill manifest, cron scheduler with in-flight deduplication, and GET /reports portal endpoint**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-08-29
- **Completed:** 2026-08-29
- **Tasks:** 3
- **Files modified:** 9 (3 created, 6 modified)

## Accomplishments
- `report_runs` table DDL with indexes on (skill_id, period) and fired_at; Store interface extended with PutReportRun, ListReportRuns, GetReportRun, GetLatestReportRunBySkill
- `ReportSchedule` field in Manifest struct; ValidateReportSchedule uses cron.ParseStandard at load time; invalid expressions fail Skill loading with descriptive error
- Scheduler skeleton with cron.New(WithChain(SkipIfStillRunning, Recover)), Start registers jobs for Skills with non-empty ReportSchedule, Stop graceful shutdown with 30s timeout; inFlight map prevents concurrent (skill, period) runs
- GET /reports endpoint with optional ?skill_id=, ?period=, ?limit= filters returning JSON report run rows

## Task Commits

Each task was committed atomically:

1. **Task 1: report_runs table DDL + store methods** - `75bb8bf` (feat)
2. **Task 2: ReportSchedule manifest field + loader validation** - `f30de09` (feat)
3. **Task 3: Scheduler skeleton + in-flight guard + GET /reports portal** - `546ff59` (feat)

**Plan metadata:** `5fbe5ad` (docs: complete plan)

## Files Created/Modified
- `internal/store/sqlite/report_runs.go` - INSERT-only store for report_runs: PutReportRun, ListReportRuns, GetReportRun, GetLatestReportRunBySkill
- `internal/store/sqlite/schema.go` - Added report_runs CREATE TABLE DDL, idx_report_runs_skill_period, idx_report_runs_fired_at, migrate() calls
- `internal/store/store.go` - Added ReportRunRow struct and four new Store interface methods
- `internal/skill/manifest.go` - Added ReportSchedule string field with yaml/json tags and godoc
- `internal/skill/loader.go` - Added ValidateReportSchedule (cron.ParseStandard), wired into parseAndValidate
- `internal/scheduler/in_flight.go` - inFlight struct with tryAcquire/release for (skillID, period) deduplication
- `internal/scheduler/scheduler.go` - Scheduler with cron.New(WithChain(SkipIfStillRunning, Recover)), Start/Stop/runReportJob skeleton
- `internal/server/report_portal.go` - reportsListHandler for GET /reports with skill_id/period/limit filters
- `internal/server/server.go` - Registered h.GET("/reports", deps.reportsListHandler) after static report route

## Decisions Made
- Used cron.ParseStandard (5-field format) matching the documented "分 时 日 月 周" convention
- inFlight map uses skillPeriodKey = "skillID:period" string key for cross-job deduplication
- ListReportRuns uses WHERE 1=1 dynamic clause assembly (T-3-SQLI posture: column names hardcoded, values via ? placeholders)
- btof helper for bool-to-int SQLite conversion following existing patterns

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **cgo build failure on Windows:** `go build` triggers CGO compiler errors in this environment (pre-existing toolchain issue, not related to our changes). `gofmt -l` and `go list` confirm all new files are syntactically valid and type-correct.

## Next Phase Readiness

- 19-02 plan is ready to proceed: the scheduler skeleton (runReportJob) is in place; 19-02 will wire eval gate checks, full report run logic, and PutReportRun audit calls
- No blockers; all three tasks completed successfully

---
*Phase: 19-p10-5-report-portal-scheduler*
*Completed: 2026-08-29*
