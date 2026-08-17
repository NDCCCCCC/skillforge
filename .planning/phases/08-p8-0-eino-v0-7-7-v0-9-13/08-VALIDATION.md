---
phase: 8
slug: p8-0-eino-v0-9-14
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-17
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> **Source:** Derived from `08-RESEARCH.md` § Validation Architecture (verified against eino v0.9.14 source).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go 标准 `testing` + `testify`（后端）；vitest（前端，本 phase 不触碰） |
| **Config file** | 无独立配置（Go 惯例；`go vet` 作为静态门） |
| **Quick run command** | `go build ./... && go vet ./... && go test ./internal/... ./cmd/... ./scripts/...` |
| **Full suite command** | `go test ./...`（含 `./integration/contract`；`.claude/worktrees/*` 自带 go.mod 被天然排除） |
| **Estimated runtime** | ~120 秒（全量） |

---

## Sampling Rate

- **After every task commit:** Run `go build ./... && go vet ./... && go test ./internal/... ./cmd/... ./scripts/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | PLAT-04 | — | N/A | regression | `go build ./... && go vet ./... && go test ./...` | ✅ | ⬜ pending |
| 08-01-02 | 01 | 1 | PLAT-04 | — | N/A | contract | `go test ./integration/contract -v`（`TestContractVersionPin` 改写断言后绿） | ✅ | ⬜ pending |
| 08-02-01 | 02 | 1 | PLAT-05 | — | checkpoint payload 完整落库（不落明文敏感字段以外内容） | unit | `go test ./internal/store/sqlite -run TestCheckpoint -v` | ❌ W0 | ⬜ pending |
| 08-02-02 | 02 | 1 | PLAT-05 | — | N/A | integration | `go test ./internal/... -run TestInterruptResumeRoundTrip -v`（新 DB handle + 新 Runner） | ❌ W0 | ⬜ pending |
| 08-03-01 | 03 | 1 | PLAT-04 | — | N/A | lint | legacy interrupt API 禁令命中即 fail（grep/analyze 检查器） | ❌ W0 | ⬜ pending |
| 08-04-01 | 04 | 2 | PLAT-04 | — | N/A | manual | live cancel <200ms + Milvus tier-filter 零行 + 冻结 eval 重跑（human-verify checkpoint） | — | — | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

> 注：Task ID 为规划前的预估映射；planner 产出 PLAN.md 后以实际任务编号为准回填。

---

## Wave 0 Requirements

- [ ] `internal/store/sqlite/checkpoint.go` — sqlite `CheckPointStore` 实现（~100 LOC，接口断言 `var _ compose.CheckPointStore = (*CheckpointStore)(nil)`）
- [ ] `internal/store/sqlite/checkpoint_test.go` — Get/Set round-trip 测试 stub（含 `schema.RegisterName[T]` 注册面）
- [ ] 中断往返测试 stub（一次 Run 中断 → checkpoint 落库 → 新 DB handle + 新 Runner → Resume 恢复）
- [ ] legacy interrupt API lint 禁令（`NewInterruptAndRerunErr` 等无地址旧语义 grep/analyze 检查器）

*既有基础设施（`go build`/`go test`/`integration/contract` 套件）覆盖升级回归部分，无需新建。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| agent run cancel 从客户端到 agent <200ms（live） | PLAT-04 | 需真实 LLM 端点 + 起栈 | 起本地栈 → 发起 agent run → 客户端 cancel → 计时 <200ms；对照 Phase-1 live verify 步骤 |
| Milvus tier-filter 对声明外 tier 返回零行（live） | PLAT-04 | 需 Docker Milvus + 已导入语料 | 起 Milvus → 用声明外 skill tier 发查询 → 断言零行返回 |
| 冻结 eval 基线重新运行全绿 | PLAT-04 | 需真实 LLM 端点（GLM） | 运行冻结 eval 套件 → 与 lock 基线同容差比对；漂移超限按 blocking 归因处理 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
