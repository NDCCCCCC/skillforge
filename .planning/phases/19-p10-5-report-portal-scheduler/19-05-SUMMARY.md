---
phase: 19-p10-5-report-portal-scheduler
plan: 05
subsystem: scheduler
tags: [gap-closure, port-03, port-04, cr-04-audit-sink, hermetic, tdd]
requires:
  - 19-04 (GetAllKPISnapshots + executeReportRun real impl — root causes moved, not removed)
provides:
  - "RunReportJob → executeReportRun → report.Publish audit-sink threading (store.Store satisfies ReportAuditSink, compile-time asserted)"
  - "cmd/server boot wiring: sched.SetKPIExecutor(kpiExecutor)"
  - "TestExecuteReportRunWiresAuditSink + TestExecuteReportRunNilAuditFailsClosed (first scheduler report-path coverage)"
affects:
  - "PORT-04 closure: automated runs produce vN.html + reports row + non-empty output_url"
  - "PORT-03 closure: cron path captures real KPI snapshots; drift detection operates on real data"
key-files:
  created:
    - internal/scheduler/report_job_test.go
  modified:
    - internal/scheduler/report_job.go
    - cmd/server/main.go
decisions:
  - "Supersedes 19-04's 'pass nil audit for automated runs' decision — that decision violated the CR-04 fail-closed contract (publish.go returns error on nil sink); the reports row IS written for automated runs with approvalAuditID empty (no HITL approval, but version/digest/template_version persisted)"
  - "Narrow-interface threading without a new adapter: store.Store's method set already contains PutReport(ctx, store.ReportRow) error — `var _ report.ReportAuditSink = (store.Store)(nil)` compile-time lock documents the reliance"
metrics:
  duration: ~20 min
  completed_date: 2026-08-29
---

# Phase 19 Plan 05 Summary: 审计接收器 + KPI 执行器接线补全

**Plan:** 19-05 — PORT-03/PORT-04 gap closure 第二轮
**Status:** COMPLETE
**Executed:** 2026-08-29
**Commit:** `ccd8824`

## One-liner

19-04 复验暴露两处断点——`report.Publish` 被传入 nil 审计接收器（CR-04 fail-closed 保证报错，自动化运行全败）和 `SetKPIExecutor` 无调用点（cron 路径空快照）——本轮以 storeDB 直通 threading + 一行 boot 装配关闭，TDD 双测试锁定。

## Tasks Completed

| Task | Description | Commit | Status |
|------|-------------|--------|--------|
| Task 1 | report_job.go：executeReportRun 增加 audit 参数；RunReportJob 传 storeDB；Publish 调用传实 sink | `ccd8824` | PASS |
| Task 2 | main.go：sched.SetKPIExecutor(kpiExecutor) boot 装配 | `ccd8824` | PASS |
| Task 3 | report_job_test.go：双测试（wires-audit-sink + nil-fails-closed）+ 编译期断言 | `ccd8824` | PASS (RED→GREEN) |

## Verification

- `CGO_ENABLED=0 go test ./internal/scheduler/ -count=1` → ok（新增 2 测试 + 既有 drift/eval_gate 零回归）
- `CGO_ENABLED=0 go build ./internal/report/ ./internal/kpi/ ./internal/scheduler/` → OK
- cmd/server 类型检查：本机 CGO vet 被 D15-01-1 阻断（emutls ABI，既有环境约束）→ WSL operator leg；改动为单行 setter 调用，参数类型由同段 375 行 `kpi.NewExecutor` 返回值保证

## Deviations

None — 计划即本轮复验产物，无范围外变更。
