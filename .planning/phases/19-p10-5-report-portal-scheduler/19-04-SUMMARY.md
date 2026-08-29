# Phase 19 Plan 04 Summary: Wire report.Publish into Scheduler (Gap Closure)

**Phase:** 19-p10-5-report-portal-scheduler
**Plan:** 04
**Type:** execute
**Wave:** 1
**Completed:** 2026-08-29
**Requirement:** PORT-04

## One-Liner
KPI executor snapshots and report.Publish now wired into the scheduler; automated cron-triggered and manual rerenders produce real vN.html reports with output_url populated in report_runs.

## What Was Done

### Task 1: Add GetAllKPISnapshots to KPI Executor
- **File:** `internal/kpi/executor.go`
- **Change:** Added `GetAllKPISnapshots(ctx context.Context) (map[string]any, error)` to `Executor`
- Iterates all KPIs from `e.reg.Catalog()`, calls `Call(ctx, CallRequest{Name, Params: nil})` for each, captures `outcome.Snapshot.Rows[0]` per KPI
- Individual KPI failures logged and skipped (does not block all snapshots)
- Package remains CGO-free (no duckdb import introduced)

### Task 2: Wire executeReportRun to KPI executor + report.Publish
- **File:** `internal/scheduler/scheduler.go`
  - Added `kpiExecutor *kpi.Executor` field to `Scheduler` struct
  - Added `SetKPIExecutor(exec *kpi.Executor)` setter
  - Updated `runReportJob` to pass `s.kpiExecutor` and `s.loader` to `RunReportJob`
- **File:** `internal/scheduler/report_job.go`
  - Replaced stub `executeReportRun` with real implementation:
    1. Calls `kpiExec.GetAllKPISnapshots(ctx)` for real KPI values
    2. Loads skill manifest via `loader.Skills()` to get `Template`
    3. Parses template via `report.ParseTemplate(m.Template)`
    4. Builds empty `SlotTexts` (`map[int]map[int]string`) for automated runs
    5. Builds `report.MapSnapshot(snapshots)` for rendering
    6. Calls `report.Publish(ctx, tmpl, slotTexts, period, snap, "", nil, skillID, nil)` (nil audit = no reports table row for automated runs)
    7. Returns `(snapshots, result.URL, nil)`
  - `RunReportJob` updated to receive `kpiExec *kpi.Executor` and `loader *skill.Loader`
  - `outputURL` now properly set from `executeReportRun` return value and stored in `report_runs.OutputURL`
- **File:** `internal/server/rerender.go`
  - Updated `RunReportJob` call in manual rerender path to pass `deps.KPIExecutor` and `nil` for loader

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Pass `nil` for `loader` in rerender handler | Manual rerender may not have a loaded skill (no cron schedule); template resolution gracefully falls back to minimal template |
| Pass `nil` for `audit` to `report.Publish` | Automated runs have no HITL approval row; the reports table row is written by the HITL tool path, not by the scheduler |
| Skip individual KPI failures in GetAllKPISnapshots | A single failing KPI should not block all snapshot capture; error is logged and other KPIs proceed |

## Files Modified

| File | Change |
|------|--------|
| `internal/kpi/executor.go` | Added `GetAllKPISnapshots` method |
| `internal/scheduler/scheduler.go` | Added `kpiExecutor` field + `SetKPIExecutor` setter; updated `runReportJob` call |
| `internal/scheduler/report_job.go` | Rewrote `executeReportRun` stub; updated `RunReportJob` signature; wired KPI executor + report.Publish |
| `internal/server/rerender.go` | Updated `RunReportJob` call to pass KPIExecutor |

## Verification

```bash
CGO_ENABLED=0 go build ./internal/scheduler/...  # OK
CGO_ENABLED=0 go build ./internal/kpi/...       # OK
CGO_ENABLED=0 go build ./internal/report/...    # OK
CGO_ENABLED=0 go test ./internal/scheduler/... -count=1  # ok
```

## Commits

- `c28ca46` — feat(19-04): wire KPI executor and report.Publish into scheduler to close PORT-04

## Self-Check: PASSED

- [x] `internal/kpi/executor.go` — `GetAllKPISnapshots` added
- [x] `internal/scheduler/scheduler.go` — `kpiExecutor` field + setter + `runReportJob` updated
- [x] `internal/scheduler/report_job.go` — `executeReportRun` stub replaced with real implementation
- [x] `internal/server/rerender.go` — `RunReportJob` call updated with KPIExecutor
- [x] `go build ./internal/scheduler/...` passes
- [x] `go build ./internal/kpi/...` passes
- [x] `go test ./internal/scheduler/...` passes
- [x] Commit `c28ca46` exists
