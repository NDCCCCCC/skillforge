---
phase: 19
slug: report-portal-scheduler
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-29
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` + `testify/assert` |
| **Config file** | None — pure unit tests |
| **Quick run command** | `go test ./internal/scheduler/... ./internal/store/sqlite/... ./internal/skill/... -count=1 -short` |
| **Full suite command** | `go test ./internal/scheduler/... ./internal/store/sqlite/... ./internal/skill/... ./internal/report/... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/scheduler/... -count=1 -run Test<task> -v`
- **After every plan wave:** Run `go test ./internal/scheduler/... ./internal/store/sqlite/... ./internal/skill/... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | PORT-03 | T-19-02 | Concurrent guard prevents double-run | unit | `go test ./internal/store/sqlite/... -run TestReportRun -v` | W0 | pending |
| 19-01-02 | 01 | 1 | PORT-02 | T-19-01 | Cron expr validated at load time | unit | `go test ./internal/skill/... -run TestReportSchedule -v` | W0 | pending |
| 19-01-03 | 01 | 1 | PORT-01, PORT-07 | T-19-01, T-19-03 | GET /reports unauth'd; cron expr safe | unit | `go test ./internal/scheduler/... -run TestScheduler -v` | W0 | pending |
| 19-02-01 | 02 | 2 | GEVL-11 | T-19-04 | Eval gate blocks degraded skill | unit | `go test ./internal/scheduler/... -run TestEvalGate -v` | W0 | pending |
| 19-02-02 | 02 | 2 | PORT-06 | T-19-05 | Drift detection no data leak | unit | `go test ./internal/scheduler/... -run TestDrift -v` | W0 | pending |
| 19-02-03 | 02 | 2 | PORT-05 | T-19-04 | Manual rerender returns 202 (no 501) | unit | `go build ./internal/server/...` | W0 | pending |
| 19-03-01 | 03 | 3 | DATA-18 | T-19-06 | Only anonymized datasets deleted | unit | `go test ./internal/scheduler/... -run TestTTLCleanup -v` | W0 | pending |
| 19-03-02 | 03 | 3 | PORT-01 | T-19-08 | Portal shows runs without auth | integration | `cd web && npx tsc --noEmit` | W0 | pending |
| 19-03-03 | 03 | 3 | PORT-05 | T-19-07 | Rerun creates new version | integration | `go build ./cmd/server/...` | W0 | pending |

*Status: pending · green · red · flaky*

---

## Wave 0 Requirements

- [ ] `internal/store/sqlite/report_runs_test.go` — stubs for PutReportRun, ListReportRuns, GetReportRun
- [ ] `internal/skill/loader_test.go` — stubs for ValidateReportSchedule (valid/invalid cron)
- [ ] `internal/scheduler/scheduler_test.go` — stubs for Start, Stop, runReportJob skeleton
- [ ] `internal/scheduler/eval_gate_test.go` — stubs for EvalGate.Check (pass/block/no-eval)
- [ ] `internal/scheduler/drift_test.go` — stubs for DetectDrift (first run, no drift, drift detected)
- [ ] `go get github.com/robfig/cron/v3` (if not already in go.mod)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| DETACH-before-delete on Windows | DATA-18 | Windows file lock is OS-level race; cannot reproduce in unit test | 1. Attach a DuckDB dataset. 2. Run TTL cleanup. 3. Verify file deleted and no "file is locked" error in logs. |
| Drift badge renders in FE | PORT-06 | Visual UI verification | Visit /reports portal, trigger a drifted run, verify amber DRIFT badge appears. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
