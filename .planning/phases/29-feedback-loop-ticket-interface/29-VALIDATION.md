---
phase: 29
slug: feedback-loop-ticket-interface
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-08
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Draft created at plan-phase (before planner defines tasks) — per-task map below is a skeleton to be filled/rewritten once plans land (prior-phase convention: 27/28 rewrote this file post-planning).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`（hermetic，零外部依赖）+ vitest（FE） |
| **Config file** | `go.mod` / `web/package.json`（both exist） |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ` + `cd web && npx vitest run src/__tests__` |
| **Full suite command** | `go test ./...` + `cd web && npx vitest run && npx tsc --noEmit` |
| **Estimated runtime** | ~60-120 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick run command (scoped to touched packages)
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (pending planner) | — | — | FBK-01 | — | INSERT-only + UNIQUE(session_id, turn_id) 幂等 | unit (Go) | `go test ./internal/store/sqlite/ -run TestFeedback` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | FBK-02 | — | 三处置均终态 + 审计行 | unit (Go) | `go test ./internal/server/ -run TestReview` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | FBK-03 | — | 全票分母帮助率 SQL | unit (Go) | `go test ./internal/server/ -run TestFeedbackMetrics` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | FBK-04 | — | history voted_verdict 回显 + 失败不锁定 | vitest | `npx vitest run src/__tests__/FeedbackRow` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | FBK-05 | — | 反馈→retriever/KB 零写路径 source-grep 结构锁 | source-grep | verify script grep gate | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | TKT-01..04 | — | provider 注册表 + 外发不阻塞 + 结果审计 | unit (Go) | `go test ./internal/ticket/... ./internal/server/ -run TestTicket` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] feedback/tickets 表的 InsertOnly 测试族（per-table，store 包内惯例）
- [ ] history 端点 voted_verdict/turn id 的 handler 测试
- [ ] FE FeedbackRow 真实 API 调用的 vitest mock

*Existing infrastructure covers framework needs — no new framework install.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| webhook provider 真实外发 | TKT-01/04 | 需要外部端点（内网间歇断，env-intranet-service-flakiness） | 配置 webhook URL 指向可观测端点，建单观察外发与重试日志、审计行 |
| FE 锁定态真实回显 | FBK-04 | 跨请求 UI 状态 | 投票 → 刷新 → 从 history 回显已投态 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
