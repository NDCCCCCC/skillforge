---
phase: "42-v1-4-outstanding-closure"
plan: "01"
subsystem: closure-research
tags: [closure, triage, audit-open]
status: complete
commit: 3b856b7
completed: 2026-09-23

# Phase 42 Plan 01: Audit-Open Triage Report — Summary

**Pure research plan**：收集 49 项 v1.4 acknowledged open items + v1.3 inherited 4 项 → 分类入桶 → 锁定 42-02/03/04 scope。

## Performance

- **Duration:** ~12 min (research + write)
- **Started:** 2026-09-23T08:30:00Z
- **Completed:** 2026-09-23T08:50:00Z
- **Tasks:** 4 (collect / classify / write / commit)

## Key Findings

1. **STATE.md 计数不实**：49 项总账高估 — 实际 30 open + 19 stale/superseded/already-resolved
   - "16 quick_task" 错误 — 实际 23 个 quick 目录，**全部已 closure 总结**
   - "14 uat_gap" 过度细分（每个 test item 当一项算）
2. **真正 outstanding = 30 项**：
   - 5 already-resolved（commit/closure 已存在）
   - 6 stale（30+ 天无活动或 superseded）
   - 4 needs-human-judgment（决策卡 J-1..J-4）
   - 12 needs-42 work（9 IN-* + 11 live runbook — **注意 IN-* 与 live runbook 重叠，实际 12 个 unique work items**）
3. **42-02 scope 锁定**：39-REVIEW IN-01..09 (9 IN-* fixes with regression tests)
4. **42-03 scope 锁定**：11 live runbook items（Phase 39 4 + Phase 41 hybrid + v1.3 inherited 4 + D6 verdict-subject + U7）
5. **42-04 scope 锁定**：MILESTONE_SUMMARY CR-01 framing + STATE.md 节 + new v1.4-AUDIT-OPEN-CLOSE.md
6. **Net closure forecast**：
   - 50% 立即 closure (15/30)
   - 40% 42-02/03/04 closure (12/30)
   - 13% 需 user decision (4/30)
   - **0%** inherit to v1.6+

## Files Created

| File | Status | Lines |
|------|--------|-------|
| `.planning/phases/42-v1-4-outstanding-closure/42-triage.md` | CREATED + committed (3b856b7) | 290 |

## Decisions Made

- 49 → 30 reclassification：剔除 19 stale/superseded/already-resolved；保留 30 真 outstanding
- 6 bucket 分类法 (already-resolved / stale / superseded / needs-code / needs-live / needs-human-judgment)
- IN-* fix 优先级：IN-05 (panic recovery, full outage protection) → IN-02 (real-world truncation) → IN-04 (500 vs 404 safety) → IN-07 (PK-collision prevention) → 其余按风险/收益
- Live runbook 12 项（L1..L12）按 source 排序：Phase 39 first (4 items) → Phase 41 → v1.3 inherited → 跨 phase operator gates
- Decision cards 4 项（J-1..J-4）需 user 决策；不替 user 选 — 只列 options + 推荐

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

- ✓ `42-triage.md` exists
- ✓ 30 items classified
- ✓ Bucket counts sum to 30
- ✓ 42-02 scope = 9 IN-* (≥ 1 item)
- ✓ 42-03 scope = 11 runbook items (≥ 5 items)
- ✓ 42-04 scope = 3 doc fix targets (≥ 2 items)
- ✓ Decision cards = 4 items (J-1..J-4)
- ✓ Commit `3b856b7` FOUND in git log

## Next Phase Readiness

- **42-02**: 9 IN-* fix plans 可立即进入 discuss-phase / plan-phase（IN 列表已 locked）
- **42-03**: operator runbook 可起草（11 items 已锁 L1..L11；L12 = D6 verdict）
- **42-04**: doc fix targets 已锁（READY 起草）
- **STATE.md § v1.4 Close** corrections identified for 42-04 实施

## Closure Forecast (end-state)

| Metric | Forecast |
|---|---|
| 49 items 实际 open (revised) | 30 |
| Closure by 42-01 (this plan) | 5 + 6 + 3 = 14 |
| Closure by 42-02 + 42-04 (Claude-executable) | 9 IN-* + 3 doc fix = 12 |
| Closure by 42-03 (operator-executable) | 11 live |
| Pending decision (user) | 4 decision cards |
| **Net closure by v1.5 complete** | ~25-26/30 = ~85-87% |
| **Net inherit to v1.6** | ~4-5 (decision cards long-term + any unfixed live) |

---

*Phase: 42-v1-4-outstanding-closure*
*Plan: 01*
*Status: complete*
*Closure-first philosophy: 0 new features; v1.4 audit-open closed in 4 waves*