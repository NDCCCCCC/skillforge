---
phase: "42-v1-4-outstanding-closure"
plan: "03"
subsystem: closure-operator-runbook
status: plan-applied (operator-execution pending)
completed: 2026-09-23 (plan + runbook committed; awaiting operator execution)

# Phase 42 Plan 03: Operator Live Verification Bundle — Summary

**Pure doc plan：把 11 项 needs-live 项打包为单 operator runbook。Plan + runbook 已 committed；operator 跑完 L1..L11 签字后此项 plan 状态升 completed。**

## Performance

- **Duration:** ~20 min (doc authoring)
- **Started:** 2026-09-23T08:50:00Z
- **Completed:** 2026-09-23T09:10:00Z (plan + runbook drafted)
- **Operator execution:** PENDING (estimated 2-4 hours)

## Deliverables

| File | Status |
|------|--------|
| `.planning/phases/42-v1-4-outstanding-closure/42-03-PLAN.md` | CREATED + committed |
| `.planning/phases/42-v1-4-outstanding-closure/42-operator-runbook.md` | CREATED + committed |

## Runbook Structure (11 gates L1..L11)

| Gate | Title | Source | Operator-Only |
|------|-------|--------|---------------|
| L1 | Phase 39 SC-2 live LLM F1 | 39-VERIFICATION #1 + 39-HUMAN-UAT #1 | ✓ |
| L2 | Phase 39 SC-3 live Milvus E2E | 39-VERIFICATION #2 + 39-HUMAN-UAT #2 | ✓ |
| L3 | Phase 39 SC-4 UI-SPEC visual | 39-VERIFICATION #3 + 39-HUMAN-UAT #3 | ✓ |
| L4 | Phase 39 CR-01 fix live verify | 39-VERIFICATION #4 + 39-HUMAN-UAT #4 + 42-02 IN-05 | ✓ |
| L5 | Phase 41 hybrid recall ≥ 0.85 | 41-06-SUMMARY deferred | ✓ |
| L6 | v1.3 BGE-M3 endpoint restore | STATE.md Deferred #1 | ✓ |
| L7 | v1.3 Docker live build+run | STATE.md Deferred #2 | ✓ |
| L8 | v1.3 Phase 18 live E2E | STATE.md Deferred #3 + 18-HUMAN-UAT | ✓ |
| L9 | v1.3 MSYS2 gcc emutls | STATE.md Deferred #4 + env-wsl-backend-runbook | ✓ |
| L10 | D6 verdict-subject-mismatch | debug/verdict-subject-mismatch.md | ✓ |
| L11 | U7 Phase 29 visual walkthrough | 29-HUMAN-UAT Test 1-8 | ✓ |

## Per-gate protocol (4-step template)

每个 gate 都有 `[precheck] → [execute] → [verify] → [record]` 4 步模板：
- **Precheck**: 0.1 节 connectivity probe（GLM/BGE/Milvus/WSL Go/Docker）
- **Execute**: 具体命令 per source artifact
- **Verify**: 预期结果（assertion / 数值 threshold）
- **Record**: `[record] L{ID} operator-signed: ___ date: ____ PASS notes=___` 模板

## Precheck (0.1 connectivity probe)

Run once before any gate. 5 checks: GLM, BGE-3, Milvus, WSL Go build, Docker.

## Decisions

- 11 gates (not 12) — L12 verdict-stream-hang 已 close as stale per 42-decisions.md J-3
- L1..L4 顺序 = Phase 39 priority order
- L5 单独 (distinct SC: hybrid recall ≥ 0.85)
- L6..L9 = v1.3 inherited (BGE-3 → Docker → Phase 18 → MSYS2)
- L10 = D6 verdict (单独的 Phase 4 chat UI issue)
- L11 = U7 Phase 29 visual (最后 — least critical)
- Per-gate record format统一：`[record] L{ID} operator-signed: ___ date: ____ PASS  ...`

## Deviations from Plan

None — pure doc plan, executed exactly as written.

## Self-Check

- ✓ `42-03-PLAN.md` exists
- ✓ `42-operator-runbook.md` exists
- ✓ Runbook 有 11 gates with 4-step protocol
- ✓ Precheck 0.1 段含 5 connectivity checks
- ✓ Per-gate record 模板统一
- ✓ Operator-only execution flagged clearly

## Operator Execution Path

1. Read runbook fully
2. Run precheck 0.1 — 验证 5 connectivity checks
3. Execute L1..L11 sequentially
4. Per gate: precheck → execute → verify → record (签字)
5. After L11 complete: 11 records in runbook
6. Notify Claude (user) to apply 42-04 doc fix
7. `/gsd:execute-phase 42` 或 `42-04` 来 apply closure doc

## Closure Forecast (post-operator-execution)

| Metric | Before L1..L11 | After L1..L11 |
|---|---|---|
| Open items | 30 - 14 (42-01) - 9 (42-02) = 7 | 7 - 11 (live) = -4 (= 0, all closed) |
| Phase 39 status | human_needed (4 items) | passed |
| Phase 41 status | hermetic-only (混合) | passed (hybrid ≥ 0.85 verified) |
| v1.3 inherited | 4 deferred | 4 closed |
| v1.5 closure | 23 of 30 (77%) | 27 of 30 (90%) |
| v1.5 closure + decision cards resolved | + 4 (J-1..J-4 already LOCKED in 42-decisions.md) | 31 of 30 = 100% net |

---

*Phase: 42-v1-4-outstanding-closure*
*Plan: 03*
*Status: plan-applied; operator-execution pending*
*Closure-first philosophy: 0 new features; operator signature = closure evidence*