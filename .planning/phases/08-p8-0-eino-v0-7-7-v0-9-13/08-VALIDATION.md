---
phase: 8
slug: p8-0-eino-v0-7-7-v0-9-13
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-17
updated: 2026-08-18
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> **Source:** Derived from `08-RESEARCH.md` § Validation Architecture (verified against eino v0.9.14 source).
> **Post-execution audit:** 2026-08-18 — task map backfilled to actual plan structure, statuses verified green on final HEAD (f137d7f + review fixes).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go 标准 `testing` + `testify`（后端）；vitest（前端，本 phase 不触碰） |
| **Config file** | 无独立配置（Go 惯例；`go vet` 作为静态门） |
| **Quick run command** | `go build ./... && go vet ./... && go test ./internal/... ./cmd/... ./scripts/...` |
| **Full suite command** | `go test ./...` + **单独** `go test -tags=contract ./integration/contract/ -count=1 -timeout 60s`（contract 套件带 `//go:build contract` 门，默认构建不编译——规划期「`go test ./...` 含 contract」的表述有误，本审计修正） |
| **Estimated runtime** | ~120 秒（全量）+ ~4 秒（contract，热缓存） |

---

## Sampling Rate

- **After every task commit:** Run `go build ./... && go vet ./... && go test ./internal/... ./cmd/... ./scripts/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 秒

---

## Per-Task Verification Map

*回填自实际 PLAN.md 任务结构（规划期预估映射已被 5-plan 结构取代）：*

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | PLAT-04 | regression | `go build ./... && go vet ./... && go test ./...` | ✅ | ✅ green（执行后 + review-fix 后两次复跑 EXIT=0） |
| 08-01-02 | 01 | 1 | PLAT-04 | contract | `go test -tags=contract ./integration/contract/ -count=1`（`TestContractVersionPin` 重写为行级锚定 `assertExactPin`，拒绝 `-rc1`/`-alpha` 后缀——IN-03 修复强化） | ✅ | ✅ green（3.7s） |
| 08-02-01 | 02 | 2 | PLAT-05 | unit | `go test ./internal/store/sqlite -run TestCheckpoint -v -count=1` | ✅ | ✅ green（六面：round-trip/重开读回/upsert/unknown-id/SQL 注入存活/closed 守卫） |
| 08-02-02 | 02 | 2 | PLAT-05 | unit | 同上（GREEN 提交 `e6a580a` 实现 `var _ compose.CheckPointStore` 编译期断言 + 87 LOC） | ✅ | ✅ green |
| 08-03-01 | 03 | 2 | PLAT-04 | lint | `go test -tags=contract ./integration/contract/ -run TestNoLegacyInterruptAPI -v` | ✅ | ✅ green（负向证明 ×2：executor 探针 + verifier 对抗探针，均 FAIL→删→PASS） |
| 08-03-02 | 03 | 2 | PLAT-04 | docs | 文档对齐验证（REQUIREMENTS/ROADMAP/CLAUDE.md v0.9.14） | — | ✅ green（grep 复核残留仅决策注记） |
| 08-04-01 | 04 | 3 | PLAT-05 | integration | `go test ./internal/store/sqlite -run TestInterruptResumeRoundTrip -v -count=1` | ✅ | ✅ green（0.17s；三重建 + 三重证据断言） |
| 08-04-02 | 04 | 3 | PLAT-04 | regression | 阶段封闭五项门（build/vet/全量/contract/store 七面） | — | ✅ green |
| 08-05-01..03 | 05 | 4 | PLAT-04 | manual | live 三门（详见 Manual-Only 表） | — | ✅ human-approved 2026-08-18 |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/store/sqlite/checkpoint.go` — sqlite `CheckPointStore` 实现（87 LOC，`var _ compose.CheckPointStore = (*SQLiteStore)(nil)` 编译期断言，`e6a580a`）
- [x] `internal/store/sqlite/checkpoint_test.go` — 六面测试（185 LOC，`package sqlite_test`，TDD RED `9a557b1` 先行）
- [x] `internal/store/sqlite/checkpoint_resume_test.go` — 中断往返测试（440 行，`3c42c20`，history-keyed stub 模型 + schema.RegisterName 双注册）
- [x] `integration/contract/no_legacy_interrupt_test.go` — legacy interrupt API 禁令门（124 行，`fea6114`，含防绕行内部名）

*既有基础设施（`go build`/`go test`/`integration/contract` 套件）覆盖升级回归部分，无需新建。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions | Result |
|----------|-------------|------------|-------------------|--------|
| agent run cancel 从客户端到 agent <200ms（live） | PLAT-04 | 需真实 LLM 端点 + 起栈 | 起本地栈 → 发起 agent run → 客户端 cancel → 计时 <200ms | ✅ 实测 0ms ×2 稳定，human-approved 2026-08-18（`spike-e2e-trace.json` 留档） |
| Milvus tier-filter 对声明外 tier 返回零行（live） | PLAT-04 | 需内网 Milvus + 已导入语料 | 声明外 skill tier 发查询 → 断言零行返回 | ✅ 声明 tier 5 行带引用 / 声明外 0 行结构化拒答，human-approved 2026-08-18 |
| 冻结 eval 基线重新运行全绿 | PLAT-04 | 需真实 LLM 端点（GLM） | 运行冻结 eval 套件 → 与 lock 基线同容差比对 | ✅ 50/50 case，gate PASS（faithfulness 0.972 ≥ 0.9），lock sha256 一致，human-approved 2026-08-18 |

---

## Validation Audit 2026-08-18

| Metric | Count |
|--------|-------|
| Gaps found | 0（全部任务有自动化验证或已完成的 manual-only 门） |
| Resolved | 0（无需补救——规划期 Wave 0 依赖在执行期全部落地） |
| Escalated | 0 |
| 文档修正 | 1（Test Infrastructure 表：contract 套件需显式 `-tags=contract`，默认 `go test ./...` 不编译该套件——无 tag 运行实测 setup failed） |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-08-18
