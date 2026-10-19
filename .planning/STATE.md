---
gsd_state_version: "1.0"
milestone: v1.7
milestone_name: Open-Source Readiness
current_phase: 48
current_phase_name: CI 矩阵
status: Ready for discuss/plan
stopped_at: context exhaustion at 76% (2026-10-19)
last_updated: "2026-10-19T08:01:50.377Z"
last_activity: 2026-10-19
last_activity_desc: Phase 47 verified 26/28 must-haves (0 FAILED, 2 PARTIAL)
state_head: ad24bc75a750804b1b5a3702423c8e276ca76ef8
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 12
  completed_plans: 9
  percent: 20
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-10-13)

**Core value:** 同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码
**Current focus:** Phase 47 清债与可复现基线（v1.7 Open-Source Readiness 首个 phase）

## Current Milestone

**v1.7 Open-Source Readiness** — 外部用户 clone→跑通→看懂→信任。
Phases: 47 清债基线 → (48 CI 矩阵 ∥ 49 Hermetic Demo) → 50 开源门面 → 51 发布工程
Coverage: 13/13 REQ（FACE×3 / CICD×3 / DEMO×3 / REL×4）

## Current Position

Phase: 48 (CI 矩阵) — READY TO EXECUTE
Plan: —
Status: Ready for discuss/plan
Last activity: 2026-10-19 -- Phase 47 verified 26/28 must-haves (0 FAILED, 2 PARTIAL)

Progress: [██░░░░░░░░] 20%

## Performance Metrics

**Velocity:**

- Total plans completed: 8 (v1.7, Phase 47)
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 47 清债与可复现基线 | 8 | — | — |

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
- **【A2 → Phase 48 必读】`verify-phase-47.sh` 前端分区被跳过时仍 exit 0**: 输出已无歧义（`PARTIAL PASS — FRONTEND PARTITION NOT VERIFIED`、P6 不在已验证列表、零 `ALL PASSED`），但退出码仍是 0。Phase 48 的 CI **不得**只看退出码，必须同时 grep `PARTIAL PASS`。runner 上没有 node 的那一刻就会踩到。
- **【A3】P4 覆盖 `./internal/...`，但仓库另有 18 个已跟踪测试文件在其外**: `go test ./...` 实测 exit 0（8 包全绿），所以**当前没有任何东西被藏起来**；但这是偶然而非结构性保证——明天在 `cmd/` 下加一个测试，它会被 CI 跑却不被本门覆盖。非 SC-4 违规（SC-4 字面定义 P4 为 `./internal/...`）。
- **MEDIUM-2（已裁定接受，`graph_edges.go:232-241`）**: 边方向已确定化，但 `keyword_cooccur` 边的**挂载**仍取 `docs` 的首命中 per-source 代表 chunk；Milvus `ListMetadata` 返回行序未定义且不去重 → 同一语料可能挂到不同节点。验证者复核确认这是**输入置换敏感**（不是固定切片上的跑间不确定），因此 `-count=100`/`-shuffle=on` 门的作用域是诚实的。
- **LOW-1（已裁定接受，`GovernanceTab.tsx:167`）**: REL-02 的 FE 连线本身无直测（删掉 prop 三个新测试照样过）。
- **D-05 已知限制（明确接受，非隐式修复）**: 启动校验会 quarantine 坏实体、rollback 不会，故回滚确认框与横幅的 `N 个实体已隔离` 都夸大了 rollback 的实际行为。三处显式记录，从未作为已修复呈现。
- **既有 flake（非 Phase 47 引入，`audit_writer_test.go:59`）**: `TestAuditWriterPreconditions` 对文件型 SQLite store 开库时套 5s `context.WithTimeout`，CPU 超卖下会超时。串行全量 8/8 绿（Phase 47 验证者自行复跑 3/3 绿），8 份并发则全红。`git log -S` 定源于 `9613d4d`（2026-08-20, Phase 11）。Phase 48 每线单 job 不会触发。

## Deferred Items

Items acknowledged and carried forward from previous milestone close.

**本表是索引，不是权威记录（D-03c）。** 每个遗留项的处置与证据的唯一权威来源是
[`.planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md`](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md)；
下表每行一个 ID、状态格内链回该主表，不复制证据。本表曾在本 STATE.md 的紧凑重写中丢过一次内容，故索引必须保持可解析（每 ID 一行，不合并成区间行）。

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| tech_debt | D-46-DEF-02 `TestDeriveEdgesDeterministic` flaky | `v1.7 修复`（Phase 47 / REL-01，已闭合；根因 = 边方向 map 迭代序，非浮点 Weight）— 证据见 [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-46-DEF-02` | v1.6 |
| tech_debt | WARNING-1 rollback SHACL 缓存刷新缺口 | `v1.7 修复`（Phase 47 / REL-02，已闭合；BE `e7ca19a` + FE `197421e`/`bf1c1e0`）— 证据见 [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `WARNING-1` | v1.6 |
| tech_debt | BL-01 双 resolver 去重（`resolveSkillTiers` vs `resolveSkillTiersFromKG`） | `延期+理由`（带行为语义的重构，无当前缺陷，须单独立项）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-01` | v1.6 |
| tech_debt | BL-02 server 包 lint 清扫 | `延期+理由`（需 Phase 48 的 `golangci-lint` 报告才有可定义范围）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-02` | v1.6 |
| tech_debt | BL-03 live 抽取 prompt 对齐 writes-only 的残余微调 | `延期+理由`（needs-live 内网 LLM/BGE）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-03` | v1.6 |
| tech_debt | BL-04 DetailPanel / emitter 两个 deferred 测试 | `v1.7 修复`（Phase 47 已闭合：emitter = plans 47-03，DetailPanel = plan 47-05）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `BL-04` | v1.6 |
| tech_debt | D-45-DEF-01 emitter `TestEmitSubCategoryInstanceCount` 6 vs 7 | `v1.7 修复`（Phase 47 已闭合；修断言非实现，`emitter.go` 未改）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-45-DEF-01` | v1.6 |
| tech_debt | D-45-DEF-02 未跟踪的 runtime-stale `verify_main.go` | `v1.7 修复`（清理工作树由 plan 47-08 执行；harness 重写未立项，因无消费者）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-45-DEF-02` | v1.6 |
| tech_debt | D-45-DEF-03 45-REVIEW 的 WR-01 / WR-02 / IN-02 | `延期+理由`（count 语义变更、fail-open 姿态对齐、IRI 族对齐，各需独立决策记录）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-45-DEF-03` | v1.6 |
| tech_debt | D-46-DEF-01 DetailPanel SC-5 空态 hero 测试确定性失败 | `v1.7 修复`（Phase 47 已闭合；断言重定向到「文档」mode，生产组件未改）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-46-DEF-01` | v1.6 |
| tech_debt | WARNING-2 `owl:Class`-vs-`skf:Class` 声明 quirk（skill 视图 tier-invisible） | `延期+理由`（schemaClasses 可见性语义决策，影响面覆盖两条路径；admin baseline 不受影响）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `WARNING-2` | v1.6 |
| known_limit | D-05 rollback 不执行 quarantine → banner「N 个实体已隔离」在回滚路径失真 | `延期+理由`（已知限制，本 phase 明确不修；修它会改变 rollback 副作用面）— [47-DEBT-DISPOSITION.md](phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md) § `D-05` | Phase 47 |
| future | 身份体系/RBAC、多租户、OTel、OpenAPI、rate limiting | v1.8+ 商业分期 | v1.7 规划 |

## Session Continuity

Last session: 2026-10-19T08:01:50.377Z
Stopped at: Phase 48 planning complete — checker PASSED (iteration 3), commit `68149ae`
Resume file: .planning/phases/48-ci/48-CONTEXT.md
Resume command: `/gsd-execute-phase 48` — 4 plans / 3 waves (48-01 tracer → 48-02 ∥ 48-03 → 48-04). Iteration-3 issue list is closed; see `48-PLAN-REVISION-3.md` for the audit trail.
