# Phase 42: v1.4 Outstanding Closure - Context

**Gathered:** 2026-09-23
**Status:** Ready for planning
**Milestone:** v1.5 — v1.4 Outstanding Closure（NOT New Features）

<domain>
## Phase Boundary

把 v1.4（和 v1.3 inherited）shipped 时挂着的 outstanding 工作一次性收口。**不引入新功能**。本 phase 4 个子计划全部 closure 性质：

- **42-01 Audit-Open Triage Report** — 把 v1.4 close 时 acknowledged 的 45 项（7 debug + 14 UAT-gap + 8 verification-gap + 16 quick-task）+ v1.3 inherited 4 项 operator_gate/env_issue = 49 项总账。逐项决定 still-relevant（→ 42-02 code fix / 42-03 live verify）/ stale（→ close）/ superseded（→ close）；产出 `42-triage.md` 作为 v1.5 范围的事实基础
- **42-02 Deferred IN-* Code Fixes** — 应用 39-REVIEW.md 中 9 个 info findings (IN-01..IN-09) + 任何 42-01 triage 标记为 "needs code" 的项；每个 fix 配 regression test；`go vet` + `go test ./...` 全绿
- **42-03 Operator Live Verification Bundle** — 把 Phase 39 human_needed 4 项 + Phase 41 live hybrid ≥ 0.85 + 42-01 triage 标记的 live-only 项打包成单 runbook；operator 在可访问 intranet LLM + Milvus 的环境跑完签字 → 把 Phase 39 status 升 passed、Phase 41 全面 passed
- **42-04 v1.4 Closure Doc Update** — 修 milestone summary（`MILESTONE_SUMMARY-v1.4.md`）的 CR-01 framing（应描述为"已修于 ae84393，待 live verify"）+ 修正 Phase 41 live recall caveat 文案 + 产 v1.4 audit-open close report（最终 resolved vs still-deferred 列表）

**不包含**：新功能、新 phase、framework 升级、Eino v0.9.14 → 后续版本。所有这些留 v1.6+ 候选（且需用户驱动开 milestone）。

**为什么不直接关 milestone**：因为 (a) 49 项里有相当一部分需要 intranet live infra，operator 才能验证；(b) 9 个 IN-* fix 需要 code change + regression tests；(d) 一个 phase 装 4 个 plan 是 closure-friendly 的最小记录单位。

**与 v1.4 标准的兼容**：本 phase 完全遵循 v1.4 standards — 无新 UI（42-01/02/04 都是 backend + docs），handler ≤ 200 行（实为改动单文件 < 50 行），全 SDLC（每 plan 配 verify-42-NN.sh 或 equivalent）。

**三条护城河保留**：所有 42-02 的 IN-* fix 都尊重既有 trust boundary — T-3-AUDIT INSERT-only / KNOW-02 fail-closed / D-09 拒答构造 / Tier 隔离 不动。

</domain>

<decisions>
## Implementation Decisions

### 42-01 Triage methodology

- **D-Triage-01:** 45 + 4 = 49 项总账从 `.planning/debug/` + `.planning/quick/` + `.planning/phases/*-HUMAN-UAT.md` + `.planning/phases/*-VERIFICATION.md` 收集 frontmatter + summary 文件；不深入读每个文件全文 — 仅 frontmatter + 1 段 status 描述
- **D-Triage-02:** 每项归类到下列 bucket：
  - `stale`：v1.3/v1.4 已落地但 fix 没 close 文档；或最近 30 天无活动
  - `already-resolved`：frontmatter 已标 `closed` / `verified-encoding-fix-closed` 等明确 close 标记
  - `needs-code`：本可通过 42-02 IN-* 或新增 small fix 闭合；前置依赖：必须确认 closure 不会引入新功能
  - `needs-live`：需 operator 跑 live LLM/Milvus 才能 close；归到 42-03 runbook
  - `needs-human-judgment`：架构 / 优先级 决策；产出 decision card 而非 fix
  - `superseded`：v1.4 新代码已绕过该 issue；可 close
- **D-Triage-03:** Triage report 产出 `.planning/phases/42-v1-4-outstanding-closure/42-triage.md`；report 结构 = 49 项 table（item / category / closure path / owner）→ `[status]` bucket 计数 → `[42-02 scope]` 列表 → `[42-03 scope]` 列表 → `[needs-human-judgment]` 决策卡片 → `[resolved/stale/superseded]` close 列表
- **D-Triage-04:** Triage 不动代码（pure research）；不在 49 项 bucket 之外添加新项（避免 scope creep）

### 42-02 IN-* fix scope

- **D-IN-01:** 9 个 IN-* fix 必须在 v1.4 trust semantics 之内 — T-3-AUDIT INSERT-only / KNOW-02 fail-closed / Tier 隔离 不可削弱
- **D-IN-02:** 每个 IN-* fix 一个独立 commit + regression test；不在 fix 中 "顺手" 添加非 IN-* 范围改动
- **D-IN-03:** IN-05（panic recovery）优先 — 当前 detached goroutine 任何 panic 都会 kill server，转换为 full outage；其他 IN-* 按"风险/收益"排期
- **D-IN-04:** IN-02（section-cap sub-split）次优 — RESEARCH 测到 93,993-byte 单 section 会 oversize prompt → 3 retries → dead letter；fix = 复用 `subSplit` 对超 cap section 作 sub-split
- **D-IN-05:** 剩余 IN-01..04, IN-06..09 按 closure report 顺序排入 42-02 多 plan；若某项 fix 涉及架构变化（不是 agent 框改动），上报回 user 决定是否进 v1.5 还是 v2

### 42-03 Operator runbook shape

- **D-Op-01:** 5 项 live-only gate（Phase 39 LLM F1 / Milvus E2E / UI-SPEC walkthrough / CR-01 复现 / Phase 41 hybrid ≥ 0.85）+ 42-01 triage 标记的 live 项打包为单 runbook `.planning/phases/42-v1-4-outstanding-closure/42-operator-runbook.md`
- **D-Op-02:** 每项 gate 配 `[precheck] → [execute] → [verify] → [record]` 4 步模板；operator 完成签字 → 该项 status 升 passed
- **D-Op-03:** Runbook 不可由 Claude 直接执行（需 intranet）；产出但留 user 调度

### 42-04 Closure doc updates

- **D-Closure-01:** `MILESTONE_SUMMARY-v1.4.md` §6 Tech Debt 表的 CR-01 行改写为："已修于 `ae84393` (2026-09-22, 39-REVIEW-FIX iteration 1)；待 operator live verify（已 in 42-03 runbook）"
- **D-Closure-02:** Phase 41 verification gap 行同样细化（hybrid recall live verify 入 42-03）
- **D-Closure-03:** 产 `.planning/reports/v1.4-AUDIT-OPEN-CLOSE.md` — 最终 resolved vs still-deferred 列表（resolved = 49 项 - still-deferred - needs-human-judgment）
- **D-Closure-04:** STATE.md `## v1.4 Close — Acknowledged Open Items` 节替换为 `## v1.5 Closure — Open Items in Progress`，加入 42-02 / 42-03 / 42-04 范围描述

### Claude's Discretion

- 42-02 各 IN-* fix 的具体 regression test 设计（在 fix plan 中具体化）
- 42-04 修 milestone summary 的具体行号（在 doc fix 时定位）
- Triage report 中 `[needs-human-judgment]` 决策卡片的措辞（不替 user 决策，只描述选项）

</decisions>

<canonical_refs>
## Canonical References

### v1.4 close artifacts
- `.planning/phases/39-ecl-pipeline/39-REVIEW.md` — 16 findings (CR-01 + WR-01..06 + IN-01..09)
- `.planning/phases/39-ecl-pipeline/39-REVIEW-FIX.md` — 7/7 fixed in iteration 1; IN-* 明确被 fix scope 排除
- `.planning/phases/39-ecl-pipeline/39-VERIFICATION.md` — human_needed 4 项
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-VERIFICATION.md` — 17/17 passed
- `.planning/phases/41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval/41-06-SUMMARY.md` — 6/6 hermetic + live recall 待 full env
- `.planning/reports/MILESTONE_SUMMARY-v1.4.md` — milestone summary 报告（含需修的 CR-01 描述）
- `.planning/STATE.md` § `v1.4 Close — Acknowledged Open Items (2026-09-23)` — 49 项总账

### Open item locations（待 42-01 实际枚举）
- `.planning/debug/*.md` — 7 debug_session
- `.planning/phases/*-HUMAN-UAT.md` — uat_gap（per phase）
- `.planning/phases/*-VERIFICATION.md` frontmatter — verification_gap
- `.planning/quick/2408*.md` — 16 quick_task

### Project-level
- `.planning/PROJECT.md` — Tech stack locked; 无 v1.5 tech change
- `.planning/SKILL_SCHEMA.md` — 7-field frontmatter; IN-* fix 不触
- `.planning/MILESTONES.md` — v1.4 close entry

### Environment
- env-intranet-service-flakiness（已知 live LLM/Milvus 间歇断）— 42-03 runbook 必须 `precheck` 含 connectivity probe + env-STOP disposition
- env-wsl-backend-runbook（WSL Go 编译）— 42-02 regression tests 走 WSL
- env-ima-openapi-credentials（注册表 env 滞后）— 42-03 不绕开；如需，注册表 env 注入法

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/knowledge/ecl/manager.go` — Phase 39 已修 CR-01 + WR-01（ae84393 + 9a4e625）；IN-05 仍待 patch（panic recovery）
- `internal/knowledge/ecl/chunk.go::subSplit` — IN-02 可复用此 helper 对超 cap section 作 sub-split
- `internal/knowledge/ecl/extract.go` — IN-02 (extract.go:148-176) per-section path 不带 cap check；IN-07 watermark failure 行为
- `internal/server/admin_pipeline.go` — IN-05 detached goroutine at lines 112-116
- `.planning/phases/39-ecl-pipeline/39-REVIEW.md` — 16 findings 全文 + minimal-diff suggestion for every IN-*

### Established Patterns
- v1.4 standards: handler ≤ 200 行 / pipeline ≤ 100 行 / 无 cgo / 无静默吞错 — 42-02 fix 必须尊重
- T-3-AUDIT INSERT-only — IN-* fix 不可引入 UPDATE/DELETE 写路径
- v1.3 TRUST semantics (D-09 / D-11 / D-14 / HITL-07) — IN-* fix 不可削弱
- Phase 39 REVIEW-FIX 模式（named-error-return + defer guard + regression test）— 42-02 IN-* 沿用此 idiom

### Integration Points
- 42-02 fix touch 只在 `internal/knowledge/ecl/` + `internal/server/admin_pipeline.go` + `internal/store/sqlite/ecl_failures.go`（不跨 server contract）
- 42-04 doc fix 只在 `.planning/reports/` + `.planning/STATE.md`（纯 docs）
- 42-03 runbook 不改代码（只是 script/checklist）

</code_context>

<specifics>
## Specific Ideas

- 42-01 triage 应在 30-60 min 内完成（pure research, no code）— user 跑完即可进入 42-02
- 42-02 IN-* fix 6-9 个 commits（每个 IN 一个），estimate 2-3 h 总；IN-05 panic recovery 单一 format 0.5 h
- 42-03 runbook 是产物不执行；execute 需 user operator
- 42-04 doc fix 30 min；产 1 milestone summary patch + 1 close report + STATE.md 1 节替换
- **关键约束**：42-02 IN-* fix 不可 introduce 新功能；若 IN-08/IN-09 涉及架构变化，退出 v1.5 范围

</specifics>

<deferred>
## Deferred Ideas

- 任何 v1.6+ feature（新 transport / 完整 SPARQL 1.1 / 多 KB federation / Leiden community detection / OWL 2 RL/DL reasoning） — 留 v1.6 路线
- v1.3 inherited（BGE-M3 restore / Docker live / Phase 18 live E2E / MSYS2 gcc emutls linking）— 部分入 42-03 runbook（如有），其余 v1.6+ 候选
- Repo-wide 文档 pass（README architecture paragraph 错配 v1.1 vs v1.0 moats，per MILESTONE_SUMMARY-v1.1 §6 Gap 3） — 留 v1.6 候选

</deferred>

---

*Phase: 42-v1-4-outstanding-closure*
*Context gathered: 2026-09-23 via standard discuss-phase (philosophy: closure-first, no new features)*