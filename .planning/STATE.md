# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-10-13)

**Core value:** 同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码
**Current focus:** Phase 47 清债与可复现基线（v1.7 Open-Source Readiness 首个 phase）

## Current Milestone

**v1.7 Open-Source Readiness** — 外部用户 clone→跑通→看懂→信任。
Phases: 47 清债基线 → (48 CI 矩阵 ∥ 49 Hermetic Demo) → 50 开源门面 → 51 发布工程
Coverage: 14/14 REQ（FACE×3 / CICD×3 / DEMO×3 / REL×4）

## Current Position

Phase: 47 of 51 (清债与可复现基线)
Plan: 0 of TBD
Status: Context gathered — ready to plan
Last activity: 2026-10-19 — Phase 47 context gathered（47-CONTEXT.md + 47-DISCUSSION-LOG.md；advisor 模式 4 灰区并行研究后全部按推荐锁定）

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0 (v1.7)
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
**历史 phase 决策（v1.4–v1.6，64 条，含 Phase 39/40/41 planner-pinned）**: `.planning/reports/v1.4-v1.6-DECISIONS-ARCHIVE.md` § Decisions —— 该清单原在本 STATE.md 中，被 v1.7 roadmap 会话的紧凑重写删除前未迁移，2026-10-19 归档保存。
Recent decisions affecting current work:

- **v1.7 roadmap（2026-10-13）**: REL 拆两处——清债（REL-01/02/04）前置 Phase 47，goreleaser（REL-03）收官 Phase 51；flaky 修复必须先于 CI 全量门与 hermetic 套件（否则闪红）
- **CI 约束**: live 门禁不进 CI（eval-gate.yml frozen-replay 是既有 hermetic 先例）；ubuntu runner 原生 gcc = WSL CGO_ENABLED=1 惯例的 CI 等价物
- **REL-04 对象**: v1.7 Traceability 100% 填充（roadmap 时已填）+ v1.6 遗留警告处置记录；不改 `.planning/milestones/` 归档
- **文档与构建现实一致**: CGO_ENABLED=1 是 go-duckdb 生产构建现实路径，README/goreleaser 表述必须如实（v1.4 前的"无 cgo 单 binary"标准已被现实路径取代）
- **Phase 46 遗留（D-45/46-01..04）**: severity 归位 shapes 数据 + ShapesPathsFromEnv 单点（详见 PROJECT.md Key Decisions）

### Pending Todos

None yet.

### Blockers/Concerns

- **内网服务间歇断**（Milvus/BGE/GLM）: live verify 沿用"探测先行、断线记 env-SKIP"惯例；v1.7 的 hermetic 化正是为降低此依赖
- **Phase 49 是最大工作量 phase**: 真实代码改动（embedded 向量兜底 + stub LLM chat），建议 plan-phase 时重点拆分

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| tech_debt | D-46-DEF-02 TestDeriveEdgesDeterministic flaky | → Phase 47 修复 (REL-01) | v1.6 |
| tech_debt | audit WARNING-1 rollback SHACL 缓存刷新缺口 | → Phase 47 修复 (REL-02) | v1.6 |
| tech_debt | BL-01..04 Traceability 补录 | → Phase 47 处置 (REL-04) | v1.6 |
| tech_debt | D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2 | → Phase 47 逐项处置记录 | v1.6 |
| future | 身份体系/RBAC、多租户、OTel、OpenAPI、rate limiting | v1.8+ 商业分期 | v1.7 规划 |

## Session Continuity

Last session: 2026-10-19
Stopped at: Phase 47 context gathered（4 灰区决议锁定：REL-01 实现确定化+测试加固 / REL-02 域层刷新+FE refetch / 单文件处置主表 / 全量门归零边界）
Resume file: .planning/phases/47-debt-cleanup-reproducible-baseline/47-CONTEXT.md
