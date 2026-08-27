# Phase 17: KPI 仪表盘 Skill (P10.3) - Research

**Researched:** 2026-08-27
**Domain:** Skill-driven chat data-citation UX + multi-period KPI aggregation + eval-gated data-reference faithfulness
**Confidence:** HIGH

## Summary

Phase 17 ships the **kpi-dashboard Skill** that lets business users ask "上月贷款余额多少" in chat and receive a number with full provenance (value + KPI definition + window + as_of + per-point data-reference hover). The Skill extends the Phase 16 single-call kpi_call Tool with **multi-period periods array support** so a single invocation returns N rows tagged by period for sparkline rendering. The chat UI adds inline `<Sparkline>` (pure SVG, zero deps per D17-B1) + shared `DataReferencePopup` (single value vs sparkline-point reuses the same component per D17-B4) + drill-down `DrillChip` (re-invokes the same KPI with multi-period params per D17-C1). Backend eval gates GEVL-09 (50-case golden KPI-call accuracy, programmatic value matching) and GEVL-10 (data-reference consistency: each answer number matches the cited KPI snapshot row + as_of present), plus a separate `reid.jsonl` frozen subset locking Pitfall 11 (跨数据集 join) + Pitfall 22 (KPI 差分) attack expectations — the first data-eval lock in the codebase (Pitfall 19 — 每 phase 贡献 data-eval golden).

**Primary recommendation:** Mirror the existing compliance/fanout eval lock pattern (D17-D2/D4) end-to-end — `cmd/eval --frozen=kpi` driven by `eval/golden/kpi.jsonl` (50 normal cases) + `eval/frozen/reid.jsonl` (Pitfall 11/22 attacks, separate lock) + `eval/frozen/kpi.lock.json` (5-pinned fields + KPI registry SHA256). The kpi_dashboard SKILL.md lives at `examples/skills/kpi-dashboard/` (parallel to `compliance/`), reads `data:aggregated_only`, and declares its routing table inside the 7-field `steps:` as a fenced YAML block (no 8th frontmatter field — preserves SYSK-02 governance; loader parses the optional block only when present). Multi-period KPI functions are a NEW `ParamType` constant (`array`) in `internal/kpi/define.go` validated fail-closed by the loader; the executor resolves N period watermarks from the sqlite `datasets` registry, runs the engine N times sequentially, merges rows tagged with `period`, writes ONE audit row (kpi_calls), and emits ONE data-decision part with a new `DataReference.References[]` field (Go `internal/wire/chunk.go` + TS `web/src/types/wire-types.ts` lockstep add per D15 — additive field, NOT a new wire union variant, so ValidTypes stays at 24 and TS `AssertAssignable` alarm stays green).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Skill routing + 7-field validation | Frontend Server (manifest validator) | — | `internal/skill/loader.go` + `validate.go` own the 7-field schema; D16-C2 加载期交叉验证 already lives here |
| Cross-validation of Skill.steps.routes ↔ KPI registry | Frontend Server (loader boot) | API/Backend (registry) | New `Loader.ValidateAgainstKPI(reg)` method invoked in `cmd/server` after LoadKPIs; mirrors D16-C2 hermetic pattern |
| Multi-period KPI execution (engine loop) | API/Backend (executor) | Database/Storage (engine adapter) | `internal/kpi/executor.Call` extended to accept `periods: []string`; N engine calls over N period watermarks; existing QueryEngine interface isolates CGO |
| Period-watermark lookup (N historical periods) | API/Backend (datasets registry adapter) | Database/Storage (sqlite) | Extend `DatasetMetaSource` with `Periods(ctx, dataset) []string`; backed by `internal/store/sqlite.Datasets.ListDatasets` filtered by logical dataset |
| INSERT-only audit freeze (multi-period snapshot) | API/Backend (executor) | Database/Storage (sqlite kpi_calls) | One audit row per kpi_call; rows[] carries `period` column tagged at engine output merge; signature = registry content hash |
| Data-reference part emission (single part, N references) | Frontend Server (SSE boundary) | API/Backend (kpi_call closure) | `internal/agent/tools/kpi_tool.go` builds `[]wire.PointReference` from snap.Rows; closure passed to `internal/server` wires per-Runner `DataRefEmitFunc` (D16-B3 + D17-B2) |
| Wire lockstep (Go + TS DecisionPayload.DataRef.References) | Frontend Server (Go wire) | Browser/Client (TS types) | D15 two-commit rule; `wire.DecisionPayload.DataRef.References []PointReference` (Go) ↔ `DataReference.references?: PointReference[]` (TS); AssertAssignable alarm stays green because field addition, not union variant |
| Sparkline rendering | Browser/Client (React MessageBubble) | — | `<Sparkline>` hand-coded SVG (`<polyline>` + N `<circle>`); zero deps; `viewBox` auto-scale `[min*0.9, max*1.1]` |
| Per-point hover popup (single-value vs sparkline-point) | Browser/Client (React DataReferencePopup) | — | Same component instance (D17-B4); reads `data-decision.DataRef.References[i]` for sparkline index; reads `data-decision.DataRef` flat fields for single value |
| Drill-chip prefill → user confirmation → re-invoke | Browser/Client (React DrillChip + app-store + ChatInput) | — | New `app-store.pendingDraft: string` field; ChatInput initializer reads it; on submit clears it; chip onClick `setPendingDraft("请调 {kpi} periods=[…]")` |
| GEVL-09 golden scoring (50 cases, programmatic) | API/Backend (`cmd/eval --frozen=kpi`) | — | Extends existing frozen-eval runner; programmatic 3-step value match (tolerance ±0.5% + allowed_expressions array); no LLM judge |
| GEVL-10 faithfulness extension (answer vs snapshot rows) | API/Backend (`cmd/eval --frozen=kpi`) | — | Per-case assertions: each number in answer text matches a row value in `kpi_calls.result_snapshot`; `as_of` token present in answer text; `data:aggregated_only` token present |
| REID frozen subset (Pitfall 11/22 attack assertions) | API/Backend (`cmd/eval --frozen=kpi`) | — | `eval/frozen/reid.jsonl` independently scored; case shape `{query, expected_outcome: {should_refuse, min_suppression_count, all_values_suppressed}}` |

## User Constraints (from CONTEXT.md)

### Locked Decisions

> Copy verbatim from `.planning/phases/17-p10-3/17-CONTEXT.md ## decisions`. Planner MUST honor these.

#### A. KPI Skill 编排 + 路由策略 (DASH-01/02)

- **D17-A1**: Skill steps 显式路由表。`kpi-dashboard/SKILL.md` 的 `steps:` 写一个按业务域分组的路由表（loans / customers / risk）。Skill 作者控制映射点（diff 评审纪律），LLM 读 steps 自行选 KPI。拒绝：description 强化、router 子 agent。
- **D17-A2**: 单一 KPI Dashboard Skill。一个 Skill 服务所有数字问答。Phase 18 月报 Skill 同样适用单 Skill 形态。
- **D17-A3**: 歧义 = LLM 自决 + 标注不定可追问。answer 中加 `confidence_note` 字段（"该问题多个 KPI 可解，按 X 业务口径回答；若你想问 Y 可追问"）。零阻断交互。
- **D17-A4**: Answer shape = LLM 自然语言 + data-reference parts。不强制 verdict-like 结构化 envelope。文本含数值 + KPI 定义 + 时间窗 + as_of；每个数字挂一个独立 data-reference part。

#### B. Sparkline 渲染 + 多点 data-reference wire (DASH-03)

- **D17-B1**: 纯内联 SVG 手写。`<Sparkline points={...} onPointHover={...} />` 输出 SVG `<polyline>` + `<circle>` per point。零新增依赖；bundle 零增量。拒绝：recharts、react-sparklines。
- **D17-B2**: data-reference part 数组化。沿用 D16 data-decision 通道，不扩 UIMessageChunk union。新增 `references: [{point_index, period, value, k_min_applied?}, ...]` 字段，单 part 装载 sparkline 所有点引用。
- **D17-B3**: 单 KPI 多周期调用，一次 emit。`kpi_call` 调一次多周期 KPI 函数（参数 `periods: [...]`），返回 N 行，data-reference part 一次性 emit 包含 N 个 references。
- **D17-B4**: Per-point hover 复用 DASH-02 popup。同一引用 popup 显示 value + period + 共享 as_of + 通道名 + KPI 函数名 + args。

#### C. 下钻 UX + 多 KPI 组合编排 (DASH-04/05)

- **D17-C1**: 下钻 = 点 chip → Skill 重调多周期 KPI。数字下方内嵌 chip「看月度趋势」「看同比」，点 chip → 预填 chat 输入框（`请调 {kpi_name} periods=[最近12月]`）。Skill 决定 chip 语义，UI 只渲染。零新 SSE part 类型。
- **D17-C2**: 多 KPI 答案 = 文本 inline + hover。每个数字挂独立 data-reference part inline 高亮 + hover。
- **D17-C3**: Skill steps 模板提示 + LLM tool-call 编排。steps 列出常见业务组合模式提示，但不硬约束调用顺序。LLM 依靠 kpi_call tool call 实现多次调用。

#### D. GEVL-09/10 eval 实现 + 再识别断言

- **D17-D1**: 合成数据 + 合成 KPI。mini KPI 注册表（3-5 KPI）+ DuckDB 合成 1 万行伪贷款/客户数据进 `data/staging/testdata.duckdb`。零外部依赖。
- **D17-D2**: 3 步组合数值相等判定。Golden set 每例预定义 `expected_kpi` + `expected_value` + `allowed_expressions`。判定：1）去千分位/单位前缀后数值差 ≤ 0.5% ✓；2）如未通过看是否命中 allowed_expressions ✓；3）都不过 FAIL。
- **D17-D3**: 独立 reid 子集 + golden 互不干扰。`eval/frozen/reid.jsonl`，每例 = 攻击 query + 期望结果。
- **D17-D4**: 新独立 `kpi.lock.json`。pin 5 字段：`model_ver` / `prompt_hash` / `seed` / `eval_set_version` / `skill_name` + 额外 `kpi_registry_sha256`。

### Claude's Discretion

- 多周期 KPI 函数 `periods` 参数 schema（FEATURES Cap 3 落地：`{type: array, items: {type: string, pattern: <月份正则>}, minItems: 2, maxItems: 24}`；loader 静态校验） — recommended but planner owns shape
- Sparkline 维度/配色 token（SVG viewBox/width/height 默认值、stroke/circle 颜色与 Tailwind v4 token 映射）
- Chip 文案生成策略（KPI 定义文件可声明 `drill_chips: [{label, periods_template}]`；无声明则默认生成「看月度趋势」chip）
- Sparkline 维度自适应（viewBox 内自动 scale 还是固定坐标轴 + Y 轴动态 padding）
- `kpi_calls` 审计行的多周期快照格式（rows 数组按 period 排序 vs 按调用顺序）
- `eval/frozen/reid.jsonl` 与 `kpi.lock.json` 的关联方式（REID 子集引用主 KPI 注册表 sha256，单独 lock vs 合并主锁的 sub-section）
- `examples/skills/kpi-dashboard/SKILL.md` 与 `examples/skills/compliance/` 的差异（无 verdict envelope 字段、温度默认 0、guardrails 侧重"绝不暴露非聚合数字"）
- GEVL-09 50 例 query 文案构造（合成 vs LLM 改写 vs 模板化）
- Skill steps 路由表 schema（YAML 结构 vs Markdown 表）— recommended: fenced YAML inside `steps:` (preserves 7-field invariant)

### Deferred Ideas (OUT OF SCOPE)

- 多轮趋势组合（"趋势之上的趋势"） — v1.1+
- 真实企业脱敏数据驱动 eval — v1.1+
- 业务月报 Skill — Phase 18
- 静态报告门户 + cron 调度 — Phase 19
- 混合 Skill（tier 知识 + data 数据并读） — v1 互斥 (D16-C1)
- admin KPI 只读视图 + 实时 KPI dashboard 页面 — Phase 20 ADM-03
- data.lock.json 跨 phase 合并决策 — Phase 19 收尾
- Skill steps 加 verifier 子步骤调取多个 KPI 对照 — A3 拒绝项

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DASH-01 | 聊天内数字问答端到端（KPI 函数调用 + 数值 + 定义 + 时间窗 + as_of + 引用） | D17-A1/A2/A4 + §Architecture Patterns P-1 + §Code Examples E-1; multi-period kpi_call extension in `internal/kpi/executor.go` + `internal/agent/tools/kpi_tool.go` |
| DASH-02 | 每个数字带完整出处（hover 显示通道名 `data:aggregated_only`） | D17-B4 + DataReferencePopup shared component + DataRef closure path (`internal/agent/tools/kpi_tool.go:252-263`); UI-SPEC §Position 2 |
| DASH-03 | 答案内嵌 sparkline 趋势，每个数据点独立可审计 | D17-B1 + D17-B2 + §Code Examples E-3 + `web/src/components/chat/Sparkline.tsx` (new) |
| DASH-04 | 用户可点击数字下钻（chip 重调多周期 KPI，永不进原始行） | D17-C1 + §Code Examples E-4 + `web/src/store/app-store.ts` pendingDraft field + ChatInput initializer |
| DASH-05 | 多 KPI 组合答案由 Skill steps 编排，各自独立引用 | D17-C2 + D17-C3 + DataRef per-call emission (N data-decision parts = N inline hovers) |
| GEVL-09 | KPI 调用准确率 50 例 golden set（query → 期望 KPI 函数+参数），数值相等程序判定（非 LLM 判分） | D17-D1/D2 + §Architecture Patterns P-4 + `cmd/eval --frozen=kpi` extension + `eval/golden/kpi.jsonl` (50) + `eval/frozen/kpi.lock.json` |
| GEVL-10 | 引用一致性（答案数字与所引 KPI 快照逐个一致，as_of + 通道名必现） | D17-D2 (as_of token + channel token in answer) + kpi_calls audit row snapshot rows[] lookup + TestGEVL10ReferenceConsistency |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `cloudwego/eino` | v0.9.14 (locked) | Agent framework + ChatModelAgent + adk tool registration | Phase 8 lockstep; `internal/agent/tools/kpi_tool.go` mirrors `retrieve.go` |
| `cloudwego/hertz` | v0.10.6 (locked) | HTTP server + SSE (`pkg/protocol/sse`) | D16-B3 data-decision SSE channel; `internal/server` reuses for data-decision wiring |
| `mattn/go-sqlite3` | v1.14.50 (locked) | sqlite with CGO (audit + datasets registry) | D14-C1 lockstep; `internal/store/sqlite/kpi_calls.go` INSERT-only |
| `go-duckdb` | v2.4.3 (locked) | DuckDB engine adapter for KPI SQL execution | D16-06 + D16-08 CGO boundary; isolated behind `kpi.QueryEngine` interface |
| `internal/kpi` (in-tree) | Phase 16 | KPI registry / loader / executor / kmin / validate | Direct extension only — no replacement |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/skill` (in-tree) | Phase 2 | Skill 7-field validator + fsnotify + reads value domain | New `Loader.ValidateAgainstKPI(reg)` method for D17 cross-check |
| `internal/wire` (in-tree) | Phase 16 | Go wire types (DataReference, DecisionPayload, SubprogressStep) | Add `References []PointReference` field to DataReference |
| `internal/eval` (in-tree) | Phase 3 / D6 reversal | Ragas-faithfulness ports + per-Skill scoring | New `KPIReferenceConsistency` metric for GEVL-10 |
| `internal/agent/tools/kpi_tool.go` (in-tree) | Phase 16 | kpi_call Tool closure + DataRefEmitFunc | Extend DataRefEmitFunc signature to build References[] |
| `web/src/components/chat/MessageBubble.tsx` (in-tree) | Phase 4 | Assistant message part dispatcher | Add `data-decision` kpi_reference branch → DataReferenceChip + Sparkline |
| `web/src/store/app-store.ts` (in-tree) | Phase 5/10 | Zustand store (skills/sessions/draftStatus, NO `messages` per D14) | Add `pendingDraft: string` + setter for drill-chip prefill |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pure inline SVG (D17-B1) | `recharts` (already in use, Phase 6) | Per UI-SPEC + D17-B1: chat-internal sparkline is "杀鸡用牛刀" — recharts brings AxisLabel / Legend / Tooltip overhead that is overkill for 24-point inline SVG. **Decision locked** — do NOT use recharts for `<Sparkline>` (test gate `TestSparklineNoDeps` greps `web/src/components/chat/Sparkline.tsx` for `recharts\|react-sparklines\|chart.js`). |
| Pure inline SVG | `react-sparklines` | Adds a dependency for ~30 lines of code; cannot customize per-point hover / per-point data-reference binding. |
| Fenced YAML inside `steps:` (recommended) | Add 8th frontmatter field `routes:` to SKILL.md schema | **Schema governance risk**: SYSK-02 locks 7-field schema (`.planning/SKILL_SCHEMA.md` v1.0); adding an 8th field breaks governance. Fenced YAML inside `steps:` is parseable by a new optional loader pass while preserving the 7-field contract. |
| Fenced YAML inside `steps:` | Plain Markdown table inside `steps:` | Markdown table is LLM-readable but requires fragile parsing for TestKPISkillStepsRoutingTable (regex against prose). Fenced YAML is deterministic to parse (`json/gopkg.in/yaml.v3`) and cross-validatable against `kpi.Registry.Names()`. **Recommendation: fenced YAML.** |
| Single new `kpi.lock.json` (D17-D4) | Extend `compliance.lock.json` with kpi fields | Different Skills have different golden sets + signatures; mixing locks couples unrelated phases (Pitfall 19 — eval drift). |
| Extend `cmd/eval --frozen=kpi` (mirrors compliance) | New `cmd/spike-kpi` binary | Mirrors `cmd/eval --frozen=compliance` 50/50 case pattern; one fewer binary to maintain; the existing `--frozen` switch + lock convention is the path of least surprise. |
| App-store `pendingDraft` for chip prefill (D14-safe) | Prop-drill `setText` from App → ChatInput → MessageBubble | App-store is the established pattern for cross-component user-input state (D5-IA-01); one new field + setter keeps chip ↔ ChatInput in sync without prop trees. |
| React Router navigate for chip | In-place panel expand (Phase 12 deep planning style) | D17-C1 explicitly rejects in-place expand: "下钻 = 更多 KPI 调用，永不进原始行"; same Skill + same Tool path; chip is a chat shortcut, not a UI mode switch. |

**Installation:**

```bash
# Phase 17 introduces ZERO new Go dependencies and ZERO new npm packages.
# All assets are in-tree additions.
```

**Version verification:** Not applicable — no new external packages introduced. Existing pinned versions verified at `go.mod` (Phase 8 dep-upgrade) and `web/package.json` (Phase 4 init).

## Package Legitimacy Audit

> **Required per protocol but vacuously satisfied: Phase 17 introduces no new external packages.**

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*If slopcheck were required: graceful degradation would mark all packages [ASSUMED] and gate install behind `checkpoint:human-verify` — not applicable here.*

## Architecture Patterns

### System Architecture Diagram

```
                                       ┌─────────────────────┐
                                       │  User: business     │
                                       │  "上月贷款余额多少"  │
                                       └──────────┬──────────┘
                                                  │ chat input
                                                  ▼
        ┌─────────────────────────────────────────────────────────┐
   BROWSER │  React 19 + Vercel AI SDK v5                            │
        │  ├─ ChatInput (reads app-store.pendingDraft for prefill)  │
        │  ├─ useChat (Vercel AI SDK; SSE client via transport)     │
        │  └─ MessageBubble                                        │
        │      ├─ text part (LLM natural-language answer)          │
        │      ├─ data-decision (kpi_reference)                    │
        │      │     └─ DataReference.References[] (per-point)     │
        │      │           └─ DataReferencePopup (hover)          │
        │      ├─ <Sparkline> (pure SVG; per-point click)          │
        │      └─ <DrillChip> (onClick → setPendingDraft)          │
        └────────────────────────────┬────────────────────────────┘
                                     │  text/event-stream
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
 FE/BE  │  Hertz :8888  (pkg/protocol/sse)                          │
        │  ├─ POST /api/chat → ChatHandler                          │
        │  └─ internal/server: per-request DataRefEmitFunc          │
        └────────────────────────────┬────────────────────────────┘
                                     │  kpi_call(name, params{periods:[..]})
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
  BE   │  internal/agent (Eino ADK)                                 │
        │  ├─ BuildRunner (data channel → {kpi_call}; tier→retrieve)│
        │  ├─ ChatModelAgent (LLM tool-call)                        │
        │  └─ kpi_call Tool closure                                 │
        │       ├─ executor.Call(req with periods:[])              │
        │       │    ├─ ① Lookup KPI (registry)                    │
        │       │    ├─ ② validateParams (periods array shape)     │
        │       │    ├─ ③ resolve N period watermarks              │
        │       │    ├─ ④ engine.Query N times (one per period)    │
        │       │    ├─ ⑤ k_min suppression (per row)              │
        │       │    └─ ⑥ single Snapshot + audit row              │
        │       └─ emit DataRef (with References[])                │
        └──────────┬───────────────────────┬────────────────────────┘
                   │                       │
                   ▼                       ▼
        ┌──────────────────────┐  ┌──────────────────────┐
        │  internal/kpi        │  │  internal/store/     │
        │  ├─ Registry (immut) │  │  sqlite              │
        │  ├─ Executor         │  │  ├─ kpi_calls (audit)│
        │  └─ QueryEngine ─────│──│  ├─ datasets (period)│
        │      ↓               │  │  └─ chat_audit       │
        │  internal/duckdb     │  │                      │
        │  (CGO engine adapter)│  │                      │
        └──────────────────────┘  └──────────────────────┘

  EVAL:  cmd/eval --frozen=kpi
         ├─ eval/golden/kpi.jsonl  (50 cases, programmatic value match)
         ├─ eval/frozen/reid.jsonl (Pitfall 11 + 22 attacks, separate gate)
         ├─ eval/frozen/kpi.lock.json (5+1 pinned fields)
         └─ synthetic testdata.duckdb (1 万行伪贷款/客户数据)
```

### Recommended Project Structure (new files only)

```
SkillForge/
├── examples/skills/kpi-dashboard/
│   └── SKILL.md                          # NEW — 7-field YAML + fenced routes YAML inside steps
├── internal/kpi/
│   ├── define.go                         # MOD — add ParamTypeArray constant
│   ├── executor.go                       # MOD — multi-period Call branch + periods param validation
│   ├── kpi.go (Snapshot)                 # MOD — Snapshot already carries Rows[] (no shape change)
│   ├── loader.go                         # MOD — validate Array type items pattern + minItems/maxItems
│   ├── multi_period.go                   # NEW — ResolvePeriods(DatasetMetaSource, dataset, n) helper
│   ├── multi_period_test.go              # NEW — hermetic w/ stub meta source
│   └── wsl/
│       └── multi_period_wsl_test.go      # NEW — live WSL multi-period query test
├── internal/skill/
│   ├── loader.go                         # MOD — new ValidateAgainstKPI(reg *kpi.Registry) method
│   └── loader_kpi_test.go                # NEW — TestKPISkillStepsRoutingTable (fenced YAML + cross-check)
├── internal/agent/tools/
│   ├── kpi_tool.go                       # MOD — DataRefEmitFunc builds References[]; args.periods[] passthrough
│   └── kpi_tool_test.go                  # MOD — TestDataReferenceArrayPart
├── internal/wire/
│   ├── chunk.go                          # MOD — DataReference.References []PointReference field
│   └── chunk_test.go                     # MOD — TestDecisionDataRefWire (extended for References)
├── internal/store/sqlite/
│   ├── kpi_calls.go                      # UNTOUCHED — existing snapshot JSON already covers multi-period rows
│   └── kpi_calls_test.go                 # MOD — TestMultiPeriodKPIPeriodsArray (one call → N rows + 1 audit row)
├── internal/server/
│   ├── runner_setup.go                   # MOD — DataRefEmitFunc closure builds References[] from snap.Rows
│   └── server.go                         # UNTOUCHED
├── internal/eval/
│   ├── kpi_metrics.go                    # NEW — programmatic 3-step value match (D17-D2) + as_of/channel presence
│   └── kpi_metrics_test.go               # NEW — TestThreeStepValueMatch
├── cmd/
│   └── eval/main.go                      # MOD — --frozen=kpi mode + reid sub-gate + kpi_registry_sha256 pin
├── data/
│   ├── staging/testdata.duckdb           # NEW — synthetic 1 万行伪数据（导入通过 internal/data.Push）
│   └── testdata/kpis/testdata.yaml       # NEW — mini KPI 注册表（3-5 KPI；多周期 + 单周期各 2）
├── eval/
│   ├── golden/kpi.jsonl                  # NEW — 50 例（query → expected_kpi + expected_value + allowed_expressions）
│   └── frozen/
│       ├── kpi.lock.json                 # NEW — 5+1 pinned fields (D17-D4)
│       └── reid.jsonl                    # NEW — Pitfall 11/22 攻击用例
├── scripts/
│   ├── verify-phase-17.sh                # NEW — 11-stage hermetic gate
│   └── make-verify-phase-17              # NEW — Makefile target
├── web/src/
│   ├── types/wire-types.ts               # MOD — DataReference.references?: PointReference[] (D15 lockstep)
│   ├── components/chat/
│   │   ├── MessageBubble.tsx             # MOD — data-decision kpi_reference branch → DataReferenceChip
│   │   ├── parts/
│   │   │   ├── DataReferenceChip.tsx     # NEW — inline number hover popup + per-number wrap
│   │   │   ├── DataReferencePopup.tsx    # NEW — shared popup (single value + sparkline point)
│   │   │   ├── Sparkline.tsx             # NEW — pure SVG polyline + circles
│   │   │   ├── DrillChip.tsx             # NEW — Badge-based chip with onClick → setPendingDraft
│   │   │   └── Sparkline.test.tsx        # NEW — TestSparklinePureSVG / TestSparklineHoverPoint / TestSparklineAutoscale
│   │   └── ChatInput.tsx                 # MOD — read app-store.pendingDraft as initial value; clear on send
│   └── store/app-store.ts                # MOD — pendingDraft: string + setPendingDraft()
└── .planning/phases/17-p10-3/
    ├── 17-CONTEXT.md                      # (exists)
    ├── 17-UI-SPEC.md                      # (exists)
    └── 17-RESEARCH.md                     # (this file)
```

### Pattern 1: Multi-period KPI execution (executor extension)

**What:** Extend `internal/kpi/executor.Call` to accept `periods: []string` parameter; resolve N period watermarks from sqlite datasets registry; run engine.Query N times sequentially; merge rows tagged with `period` column; write ONE audit row with the merged `Rows[]`; build `DataReference.References[]` from the merged rows for SSE emission.

**When to use:** Single KPI definition has `params.periods: type: array`; caller passes N periods as `params.periods`. Replaces per-call single-period loop in the LLM driver.

**Example (1) — executor extension sketch:**

```go
// Source: internal/kpi/executor.go (Phase 17 extension of Phase 16 six-step flow)
//
// Step 2 (validateParams) extended: array-type param accepts []string; per-element
// validation against items.pattern; minItems/maxItems enforced; ordered ascending
// in audit args.
//
// Step 3 (as_of generation): when periods != nil, resolve N period watermarks via
// DatasetMetaSource.Periods(ctx, dataset). Each period must exist (otherwise
// ReasonDatasetNotAvailable). Stamped as_of.Period = max(periods); ts = now.
//
// Step 4 (engine.Query): loop periods; build EngineRequest per period with
// version.FilePath resolved for that period (existing LatestDatasetVersion takes
// one; extend to ListDatasetPeriods for historical).
//
// Step 5 (k_min suppression): applied per row across all periods (KMinApplied
// shared per snapshot, kpi-defines-file-dataset-global chain unchanged).
//
// Step 6 (single-Snapshot emission): snap.Rows = concatenated N result sets, each
// row gets a "period" column tagged at merge; snap.Args.periods = sorted []string;
// audit row ResultSnapshot JSON marshals the full N-row snapshot.
```

**Example (2) — wire chunk extension:**

```go
// Source: internal/wire/chunk.go (D15 lockstep with web/src/types/wire-types.ts)
type PointReference struct {
    PointIndex  int    `json:"pointIndex"`           // 0-based index in sparkline points[]
    Period      string `json:"period"`                // period the point came from
    Value       any    `json:"value"`                 // the numeric value (formattable)
    KMinApplied int    `json:"kMinApplied,omitempty"` // per-point suppression (rare for multi-period KPI)
}
type DataReference struct {
    Func        string           `json:"func"`        // existing
    Args        map[string]any   `json:"args,omitempty"`
    ASOfPeriod  string           `json:"asOfPeriod"`
    ASOfTs      string           `json:"asOfTs"`
    RowCount    int              `json:"rowCount"`
    Suppressed  int              `json:"suppressed"`
    KMinApplied int              `json:"kMinApplied"`
    // NEW (D17-B2): multi-point array (N = 1 for single-period calls; preserves wire compat).
    References  []PointReference `json:"references,omitempty"`
}
```

### Pattern 2: Skill steps routing table (fenced YAML inside 7-field schema)

**What:** Embed the KPI routing table inside the existing `steps:` field as a fenced YAML code block (```yaml``` + ```); the loader optionally parses it (when present, must be valid YAML + KPI names must exist in registry); the LLM reads the rendered markdown.

**When to use:** Any Skill that needs a structured routing table (kpi-dashboard; possibly future Skill flavors). Preserves SYSK-02 7-field schema governance.

**Example (3) — kpi-dashboard SKILL.md frontmatter + steps fragment:**

```yaml
---
description: 业务数字问答。触发短语：「贷款余额」「不良率」「客户数」「上月销售额」等聚合数字问题。返回数值 + KPI 定义 + 计算时间窗 + as_of + 数据引用。
agent: ChatModelAgent
reads:
  - data:aggregated_only
writes: []
steps: |
  # KPI 路由表（按业务域分组；LLM 读取后按业务词选 KPI）

  ```yaml
  routes:
    loans:
      - triggers: [贷款余额, 未偿还本金, loan balance, outstanding]
        kpi: KPI_LOAN_BALANCE
      - triggers: [不良率, NPL ratio, 逾期率]
        kpi: KPI_NPL_RATIO
    customers:
      - triggers: [客户数, 活跃客户, customer count]
        kpi: KPI_CUSTOMER_COUNT
  ```

  ## 调用流程
  1. 根据业务词查路由表 → 选 KPI 名 + 单 period 或 periods[]
  2. 调 kpi_call(name, params)，不得尝试 SQL 或 SELECT
  3. 自然语言答：数值 + KPI 定义 + 时间窗 + as_of（必须出现）+ 数据来源 `data:aggregated_only`
  4. 多 KPI 答案：每个数字独立 hover；可下钻「看月度趋势」「看同比」
guardrails:
  prohibitions:
    - "Never SELECT raw rows or attempt SQL"
    - "Never expose values < k_min suppression threshold"
    - "Never cite numbers not backed by a kpi_call snapshot row"
  required_refusal_phrases:
    - "no_matching_kpi"
    - "dataset_not_available"
  tier_limits:
    "data:aggregated_only": ["kpi_call only; no retrieve"]
temperature: 0.0
```

**Example (4) — loader optional parse + cross-check (sketch):**

```go
// Source: internal/skill/loader.go (Phase 17 extension)
//
// New method: ValidateAgainstKPI(reg *kpi.Registry) error
// Called at server boot AFTER LoadKPIs and AFTER Loader.Scan (D16-C2 加载期交叉验证
// pattern). Iterates cached manifests; for each, parses the optional fenced YAML
// block inside steps:; cross-checks every `kpi:` name appears in reg.Names().
// Failure = log.Fatalf (mirrors D16-08-D LoadKPIs failure mode).
```

### Pattern 3: Drill chip prefill (app-store + ChatInput)

**What:** New `pendingDraft: string` field in `web/src/store/app-store.ts`; ChatInput reads it as initial value via `useEffect`; on submit clears it. Chip onClick sets it to `请调 {kpi_name} periods=[最近 12 月]`. User must press Send (D17-C1).

**When to use:** Any chip/badge that needs to inject text into ChatInput without auto-sending.

**Example (5) — chip onClick sketch:**

```tsx
// Source: web/src/components/chat/parts/DrillChip.tsx (NEW)
import { Badge } from '@/components/ui/badge';
import { useAppStore } from '@/store/app-store';
export function DrillChip({ kpiName, periodsTemplate }: { kpiName: string; periodsTemplate: string }) {
  const setPendingDraft = useAppStore((s) => s.setPendingDraft);
  const pendingDraft = useAppStore((s) => s.pendingDraft);
  const handleClick = () => {
    setPendingDraft(`请调 ${kpiName} periods=${${periodsTemplate}}`);
    // Scroll to chat input; ChatInput.useEffect picks up pendingDraft
  };
  return (
    <Badge
      variant="secondary"
      className="kpi-chip cursor-pointer"
      data-testid="drill-chip"
      onClick={handleClick}
    >
      看月度趋势
    </Badge>
  );
}
```

```tsx
// Source: web/src/components/chat/ChatInput.tsx (Phase 17 mod)
//
// New useEffect: when pendingDraft changes (non-empty) and text is empty, set text
// to pendingDraft and focus textarea. On submit (Enter or Send click), call
// useAppStore.getState().setPendingDraft('') before invoking onSend.
```

### Pattern 4: GEVL-09/10 programmatic value match (no LLM judge)

**What:** Numeric consistency check is a pure-Go function — no LLM call. Step 1: parse answer text + extract all numbers (regex `\d[\d,]*\.?\d*` + unit-prefix variants "亿/万/%"); Step 2: normalize (strip commas, parse unit); Step 3: for each cited KPI call (from kpi_calls audit rows for this session), check each number in answer maps to a row value within ±0.5% OR hits `allowed_expressions[i]` exact match. as_of token presence in answer text is a simple substring check. channel-name token `data:aggregated_only` is a substring check.

**When to use:** Any eval that judges answer-vs-snapshot numeric consistency (GEVL-09/10; future data-eval Skills).

**Example (6) — three-step judge sketch:**

```go
// Source: internal/eval/kpi_metrics.go (NEW)
package eval
type KPIJudgment struct {
    CaseID    string
    Passed    bool
    Reason    string  // "value_mismatch", "missing_as_of", "missing_channel", "refused_correctly", "should_refuse"
    MatchedKPI string
}
func JudgeKPIReferenceConsistency(c Case, kpiCallRows []KPICallRow) KPIJudgment { ... }
```

### Anti-Patterns to Avoid

- **Adding a new wire union variant** for multi-point data-reference (e.g. `data-decision-sparkline`). D17-B2 explicitly forbids: D16-B3 union expansion already moved ValidTypes 17→24; adding again risks crossing the TS `AssertAssignable` alarm AND losing the single-channel `data-decision` discriminator. **Add field to `DataReference`, do NOT add variant.**
- **Chart library imports in `web/src/components/chat/Sparkline.tsx`.** Test gate `TestSparklineNoDeps` greps zero hits for `recharts|react-sparklines|chart.js`. If a future contributor reflexively adds recharts, the gate fails.
- **Routing table as an 8th frontmatter field.** SYSK-02 locks 7-field schema; adding `routes:` breaks schema governance and triggers a re-spec debate. Use fenced YAML inside `steps:` (Pattern 2).
- **Auto-sending on chip click.** D17-C1 explicitly requires user confirmation (Send). Auto-send violates "no YOLO approval" (Project Out-of-Scope table) and the user-confirmation discipline from HITL-01.
- **In-place drill-down panel** (in-place expand after answer text). D17-C1 rejects: requires new SSE part type + skill state machine + animation choreography. Chip re-invokes the same Skill + same Tool path.
- **Programmatic SQL for multi-period** (e.g. `WHERE period IN $periods` in a single SQL with multi-attached DuckDB). Engine adapter today handles one FilePath per call; cross-file ATTACH + UNION ALL would require AST walk changes to `validate_sql.go`. The recommended executor-loop approach keeps Phase 16 Layer 1+2 SQL validation intact.
- **LLM-judge for GEVL-09.** ROADMAP §SC-3 explicitly: "数值相等程序判定（非 LLM 判分）". LLM judge would introduce drift + cost; programmatic 3-step match is deterministic and hermetic-testable.
- **Per-Skill KPI subsets.** D16-C4 explicitly forbids — single global registry; singleton philosophy. Skill steps declare what to USE, not what to RESTRICT.
- **Runtime dynamic differential detection** (跨调用快照差集检测). D16-D2 rejected; eval asserts the attack-set is empty instead.
- **Multi-part SSE emission for N points** (one part per point). D17-B2 rejects: N SSE frames + ordering risk + DataRef per point multiplies callbacks. Single part with `References[]` array is the wire-minimal shape.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Multi-period KPI SQL with cross-file UNION ALL | Custom ATTACH logic + composite SQL template | Executor loop (Pattern 1): N `engine.Query` calls, each one period, merge rows tagged with `period` | Preserves Layer 1+2 SQL validation in `validate_sql.go` (each per-period SQL is the same validated template); no AST walk changes; no CGO adapter changes |
| Sparkline chart | Custom SVG + tooltip state machine | Hand-coded `<Sparkline points={...}>` (Pattern 1 code example); `<DataReferencePopup>` shared via shadcn `Tooltip` | D17-B1 explicitly: zero deps. recharts is 200+ KB and overkill for inline chat sparkline |
| Multi-point data-reference wire | New SSE chunk variant `data-decision-sparkline` | Extend `DataReference.References []PointReference` field (Pattern 2) | D16-B3 wire union has been the load-bearing constraint for 3 phases; adding a variant resets all assertions + transports |
| Chip → chat input prefill | Prop-drill setText from App → ChatColumn → ChatInput | App-store `pendingDraft` + ChatInput `useEffect` reader (Pattern 3) | Established cross-component pattern (D5-IA-01); one new field + setter |
| GEVL-09 numeric accuracy judge | LLM judge | Programmatic 3-step match in `internal/eval/kpi_metrics.go` (Pattern 4) | ROADMAP §SC-3 explicit; deterministic + hermetic-testable + $0 cost |
| Skill steps routing table | 8th frontmatter `routes:` field | Fenced YAML inside `steps:` (Pattern 2) | SYSK-02 7-field schema governance; loader optional parse |
| kpi-dashboard SKILL.md | New YAML schema / new Skill manifest type | Same 7-field compliance SKILL.md template; add fenced YAML inside steps | Compliance SKILL.md is the locked template (`examples/skills/compliance/SKILL.md` mirror); no schema work |
| Synthetic testdata | External real data source | DuckDB Push via `internal/data.Importer` to `data/staging/testdata.duckdb` | D17-D1 explicit: zero external deps; uses existing import pipeline + WSL CGO test pattern (D15-01-1) |

**Key insight:** Phase 17 is a **compose phase** — every component is already shipped in some form (registry, executor, Tool, SSE channel, eval runner, Skill loader, shadcn Tooltip). The work is composing them into the kpi-dashboard surface + extending 3 fields (DataReference.References, ParamSpec.Type=array, Skill steps YAML parser) + 2 eval assets (kpi.lock.json + reid.jsonl). **Zero new packages, zero new subsystems, zero new wire types.**

## Common Pitfalls

### Pitfall 1: Multi-period data layer — N historical periods must ALL exist in datasets registry

**What goes wrong:** Caller passes `periods: ["2025-08", "2025-09", "2026-07"]`; the engine queries only `2026-07` (latest watermark) and silently returns 1 row instead of N — OR fails because intermediate periods were never imported.

**Why it happens:** `DatasetMetaSource.LatestDatasetVersion` is the only seam today (one period). Multi-period requires a new seam `Periods(ctx, dataset) []string` that queries `internal/store/sqlite/datasets.ListDatasets(filter=dataset)` returning all registered periods sorted ascending.

**How to avoid:**
- Add `DatasetMetaSource.Periods(ctx, dataset) ([]PeriodRow, error)` interface method backed by existing `ListDatasets` filter.
- Executor step ③ fails fast: `if len(req.Periods) > 0 { for _, p := range periods { if !exists in datasets table { return ReasonDatasetNotAvailable } } }`.
- Test `TestMultiPeriodKPIPeriodsArray` in `internal/kpi/wsl/multi_period_wsl_test.go` covers: (a) all periods exist → N rows returned; (b) one missing → structured refusal; (c) empty array → invalid_params.

**Warning signs:** Golden-set answer shows `len(rows)==1` when expected 12; live eval answer has only the latest period; data-decision part has `references: []` empty.

### Pitfall 2: Wire lockstep D15 — Go + TS must commit in lockstep

**What goes wrong:** Go side adds `References []PointReference` field; TS side misses it; UI TypeScript build fails OR worse, UI silently renders no per-point hover because `references` is `undefined`.

**Why it happens:** D15 (TS `AssertAssignable` alarm) catches union-LEVEL drift but NOT nested-type field drift. `DataReference.References` is a NESTED field addition; the union member still matches because the property is optional.

**How to avoid:**
- **Two commits per D15 rule**: commit 1 BE adds field + test (`TestDecisionDataRefWire` extended); commit 2 FE mirrors (`web/src/types/wire-types.ts` `DataReference.references?: PointReference[]`). Doc the rule in `17-PLAN.md` upfront.
- **TS compile alarm test**: extend existing transport-swap or `wire-types.test.ts` to assert `PointReference` is defined AND `DataReference.references` is the optional array shape. If FE drifts, `npx tsc --noEmit` exits 1.
- **Phase 4 precedent**: 04-04 closed a similar 21-error TS drift debt; `npx tsc --noEmit` is the cheap detector.

**Warning signs:** `npx tsc --noEmit` exits 1; live UI shows references undefined; SSE stream has `references: [...]` but MessageBubble ignores it (silent mismatch).

### Pitfall 3: SYSK-02 7-field schema governance — don't add an 8th field

**What goes wrong:** Developer adds `routes:` as an 8th frontmatter field to kpi-dashboard SKILL.md; SYSK-02 validator rejects; OR validator is bypassed; OR the governance debate blocks the phase.

**Why it happens:** Routing table needs to be machine-parseable for TestKPISkillStepsRoutingTable. The reflex is to add a new frontmatter field.

**How to avoid:**
- **Fenced YAML inside `steps:`** (Pattern 2): the loader optionally parses the fenced block when present; routes are validated against KPI registry.
- **SYSK-02 unchanged**: `internal/skill/validate.go` continues to require exactly 7 fields; the fenced YAML is body content inside `steps:` (which is free-form Markdown).
- **Document in 17-PLAN**: state explicitly "no 8th field; routing table is fenced YAML inside steps; loader parses optionally".

**Warning signs:** Validate rejects kpi-dashboard; or worse, Validator is locally patched to accept routes — schema drift.

### Pitfall 4: CGO test binaries must run in WSL2 on Windows host

**What goes wrong:** Any new test file in `internal/kpi/` that imports `internal/duckdb` to exercise live multi-period queries will fail on Windows MSYS2 gcc linker limit (D15-01-1 + Phase 16 inherited wsl failures).

**Why it happens:** go-duckdb has a CGO dependency that Windows MSYS2 gcc cannot link (emutls ABI mismatch, duckdb-go-bindings#105).

**How to avoid:**
- **Hermetic tests** for executor/loader/define/validate_sql/registry stay in `internal/kpi/` (no CGO import). TestMultiPeriodKPIPeriodsArray and TestGEVL09/10 hermetic equivalents go in `internal/kpi/`.
- **Live WSL tests** for engine integration go in `internal/kpi/wsl/multi_period_wsl_test.go` (mirrors existing `snapshot_wsl_test.go`). Executor can call the engine adapter via the `QueryEngine` interface; the wsl test wires a real `internal/duckdb.EngineAdapter`.
- **`scripts/verify-phase-17.sh` Stage 8**: literal command runner that explicitly retries the wsl package, mirroring `verify-phase-16.sh` Stage 08.

**Warning signs:** `go test ./internal/kpi/... -count=1` on Windows host fails with emutls error; live eval cmd/eval stuck on DuckDB open.

### Pitfall 5: GEVL-09 live eval environment — needs GLM only (NOT BGE/Milvus)

**What goes wrong:** Operator tries to run `cmd/eval --frozen=kpi`; expects live stack; starts Milvus + BGE-M3; env-intranet flakiness memory note says these are intermittently unreachable; the eval sits at 0/50.

**Why it happens:** Past frozen eval gates (compliance, platform-floor) required Milvus + BGE-M3 for retrieval. KPI eval is different: it does NOT retrieve (data channel = kpi_call only). Only GLM_API_KEY + local DuckDB testdata are required.

**How to avoid:**
- **Document the live-stack prerequisite clearly** in `17-PLAN.md` and the operator runbook: only `GLM_API_KEY` + `data/staging/testdata.duckdb` + `data/testdata/kpis/testdata.yaml` + `cmd/server` bootable.
- **`scripts/verify-phase-17.sh` Stage 9** (live eval, operator-gated): mirror `verify-phase-13.sh` Stage 7 format; explicit preflight probe of GLM endpoint before starting 50 cases.
- **Hermetic fallback for CI**: the gate's primary acceptance is the hermetic 50-case numeric-match test (MockChatModel + fixture registry + fixture DuckDB). Live eval is the operator-signed kpi.lock.json update, NOT the gate.

**Warning signs:** Pre-flight probe fails; operator reaches for Milvus; testdata.duckdb missing.

### Pitfall 6: Synthetic testdata fixture must have ≥ k_min rows per (period, dimension) bucket

**What goes wrong:** Golden-set answers show `value=suppressed` (k_min suppression marker) for one or more periods; GEVL-10 fails because expected_value doesn't match the suppressed marker; the case is FAIL.

**Why it happens:** 1 万行 over 12 periods × 4 dimensions = ~200 rows per cell, well above KMinDefault=5; BUT if the synthetic distribution is skewed (e.g., one period has only 2 customers), suppression triggers.

**How to avoid:**
- **Distribution guarantee**: synthetic generator uses balanced stratified sampling (deterministic seed=42); each (period, dimension) bucket has ≥ 50 rows.
- **Test fixture**: `internal/kpi/wsl/multi_period_wsl_test.go` asserts zero suppression in expected output (per golden-set expected_values).
- **Golden-set design**: expected_values chosen from cells with high counts; reid.jsonl cases explicitly test suppressed cells.

**Warning signs:** GEVL-10 fails with "value mismatch: expected 4.23, got suppressed"; live eval shows `kpi_calls` audit row with `suppressed > 0` for golden-set expected periods.

### Pitfall 7: Pitfall 19 (eval audit drift) — this phase is the FIRST data-eval lock

**What goes wrong:** Future phases (18 / 19 / 20) build more Skills; each adds eval metrics; no discipline = the locks drift; kpi.lock.json sha256 fails byte-identity at milestone close.

**Why it happens:** Without per-phase discipline, the lock can drift silently; milestone close finds a lock that doesn't match the live eval report.

**How to avoid:**
- **`eval/frozen/README.md` row added**: `kpi.lock.json — Phase 17 (17-09) — KPI dashboard lock — 50-case eval/golden/kpi.jsonl + kpi_registry_sha256 + per-scenario counts + reid sub-lock`.
- **State.md closeout record**: at Phase 17 close, record `eval/frozen/kpi.lock.json` sha256 in STATE closeout block (mirrors 11-08 / 13-04 sign-off).
- **Phase 19 deferred decision (DATA-18)**: data.lock.json cross-phase merge decision deferred to Phase 19; this is the first data-eval lock to keep independent until that decision.

**Warning signs:** kpi.lock.json byte-identity check fails in verify-phase-17.sh Stage 11; future milestone close finds no record of kpi lock.

### Pitfall 8: App-store `pendingDraft` dual-source risk (D14 mirrors D14-`'messages'` ban)

**What goes wrong:** `pendingDraft` and `text` state in ChatInput both hold the input value; on first render, ChatInput initializes `text` from `useState('')` (empty) and then `useEffect` syncs `text = pendingDraft`; but if user has typed something while pendingDraft was set, the sync overwrites.

**Why it happens:** Two sources of truth for the same logical value.

**How to avoid:**
- **Single source**: `text` in ChatInput local state is the authoritative value; `pendingDraft` is a ONE-SHOT signal consumed and cleared in `useEffect`.
- **Consume-once pattern**: `useEffect(() => { if (pendingDraft) { setText(pendingDraft); setPendingDraft(''); } }, [pendingDraft])` — pendingDraft is cleared after consumption; subsequent typing is purely local state.
- **Test**: vitest covers (a) chip click → input prefilled; (b) typing over prefill → text updated, pendingDraft empty; (c) send → onSend called with prefilled text; (d) after send, next chip click prefills again.

**Warning signs:** User types over prefill, then chip is clicked, input reverts to previous prefill text.

### Pitfall 9: Streaming order — data-decision part arrives AFTER text body (Phase 16 pattern)

**What goes wrong:** `<Sparkline>` needs `references[]` to render; but the data-decision part with DataRef is emitted by the kpi_call closure AFTER the text streaming completes. If `<Sparkline>` mounts inside the text body, it has no refs until late.

**Why it happens:** Eino's Tool OnEnd fires after the LLM consumes the tool result; the closure's `emit(ref)` happens during this callback, which is partway through the answer's text streaming.

**How to avoid:**
- **Late-arriving data-decision handled**: `<Sparkline>` reads `references[]` from the data-decision part; on first render refs is `[]`, on next render refs is populated; React useState + useEffect handles this transparently.
- **Defer sparkline rendering**: `<Sparkline>` renders an empty container while refs empty; once refs arrive, re-render with circles. No layout shift (intrinsic height 32px reserved).
- **Text body independent**: the inline number hover popup ALSO waits for DataRef. If text contains "贷款余额 4,231.5 亿元" and DataRef hasn't arrived yet, hover is disabled (no popup). This is acceptable per Phase 4 streaming pattern.

**Warning signs:** Sparkline flickers; inline hover popups appear with 200-500ms delay; SSE stream shows text parts before data-decision part.

### Pitfall 10: kpi_calls audit row snapshot must capture multi-period rows verbatim (D16-D3)

**What goes wrong:** Executor merges N period rows in memory, then marshals `snap.Rows` to JSON for audit; but the JSON has rows with `period` column INCONSISTENTLY ordered (some periods come first alphabetically, some by execution order); audit replay produces a different number than the original call.

**Why it happens:** Map iteration order is non-deterministic in Go; `map[string]any` rows can serialize differently across runs.

**How to avoid:**
- **Sort rows by period ascending before marshaling**: executor step ⑥ sorts `snap.Rows` by `period` column ascending (stable secondary sort by remaining columns); deterministic JSON.
- **Test**: `TestMultiPeriodKPIPeriodsArray` runs the same query 10× and asserts byte-identical audit row JSON.
- **D16-D3 invariant preserved**: audit row is byte-stable across replays.

**Warning signs:** `kpi_calls` audit row sha256 changes across runs; live replay shows non-deterministic numbers.

## Runtime State Inventory

> **Not applicable: Phase 17 is not a rename/refactor/migration phase.** No existing strings or runtime state are renamed. The new artifacts (kpi-dashboard SKILL.md, eval/golden/kpi.jsonl, eval/frozen/kpi.lock.json, eval/frozen/reid.jsonl, data/staging/testdata.duckdb, web Sparkline + chip components) are additive; nothing existing is migrated.

## Code Examples

### Verified patterns from official sources (Phase 16 anchors + D17 extensions)

### Example E-1: kpi_call closure building DataRef with References[]

```go
// Source: internal/agent/tools/kpi_tool.go (Phase 17 extension of Phase 16 closure)
invoke := func(ctx context.Context, args kpiCallArgs) (kpiCallResponse, error) {
    // ... (steps 1-4 unchanged)
    snap := outcome.Snapshot
    if emit != nil && snap != nil {
        ref := wire.DataReference{
            Func:        snap.Func,
            Args:        snap.Args,
            ASOfPeriod:  snap.ASOf.Period,    // for multi-period = max(periods)
            ASOfTs:      snap.ASOf.Ts,
            RowCount:    len(snap.Rows),
            Suppressed:  snap.Suppressed,
            KMinApplied: snap.KMinApplied,
            References:  buildPointReferences(snap.Rows), // NEW (D17-B2): one entry per row
        }
        emit(ref)
    }
    return kpiCallResponse{Kind: "snapshot", Snapshot: snap}, nil
}

// buildPointReferences converts snap.Rows to per-point references for sparkline.
// Each row has a `period` column tagged at executor step ⑤; first column with
// numeric value becomes the Y axis; period is the X axis.
func buildPointReferences(rows []map[string]any) []wire.PointReference {
    out := make([]wire.PointReference, 0, len(rows))
    for i, row := range rows {
        p := wire.PointReference{
            PointIndex: i,
            Period:     fmt.Sprint(row["period"]),
        }
        // The KPI function's SELECT-list determines which column is "the value";
        // for v1 multi-period KPI, it's the LAST non-period column (after GROUP BY period).
        for k, v := range row {
            if k != "period" && k != "row_count" {
                p.Value = v
                break
            }
        }
        out = append(out, p)
    }
    return out
}
```

### Example E-2: Multi-period executor.Call (simplified)

```go
// Source: internal/kpi/executor.go (Phase 17 extension of Phase 16 step 3+4)
//
// For a KPI with params.periods type=array:
//
//   Step ③ resolved_periods = DatasetMetaSource.Periods(ctx, dataset) intersected
//          with req.Periods; each must exist (otherwise Refusal{DatasetNotAvailable}).
//   Step ④ loop: for each period, EngineRequest{FilePath = datasets.Period → FilePath,
//          Args = current params (periods replaced with single period for binding)}.
//          engine.Query returns 1 row; tag with period column; append to merged rows.
//   Step ⑤ k_min suppression on merged rows (per-row, KMinApplied shared).
//   Step ⑥ snap.Rows = merged; snap.Args.periods = sorted []string; audit row 1×.
```

### Example E-3: Pure SVG sparkline (TS)

```tsx
// Source: web/src/components/chat/parts/Sparkline.tsx (NEW — zero deps)
import { useMemo } from 'react';

export interface SparklinePoint {
  index: number;
  period: string;
  value: number;
}

export function Sparkline({ points, onPointHover }: {
  points: SparklinePoint[];
  onPointHover: (i: number) => void;
}) {
  const { polyline, circles } = useMemo(() => {
    const values = points.map((p) => p.value);
    const min = Math.min(...values);
    const max = Math.max(...values);
    const range = max - min || 1;
    const padding = range * 0.1; // ±10%
    const yMin = min - padding;
    const yMax = max + padding;
    const yRange = yMax - yMin;

    const w = 300, h = 32;
    const xStep = points.length > 1 ? w / (points.length - 1) : 0;

    const poly = points
      .map((p, i) => {
        const x = i * xStep;
        const y = h - ((p.value - yMin) / yRange) * h;
        return `${x},${y}`;
      })
      .join(' ');

    const cs = points.map((p, i) => {
      const x = i * xStep;
      const y = h - ((p.value - yMin) / yRange) * h;
      return { cx: x, cy: y, idx: i };
    });
    return { polyline: poly, circles: cs };
  }, [points]);

  if (points.length === 0) return null;

  return (
    <svg
      viewBox={`0 0 300 32`}
      width="100%"
      height="32"
      style={{ overflow: 'visible' }}
      data-testid="sparkline"
      role="img"
      aria-label={`sparkline with ${points.length} points`}
    >
      <polyline
        points={polyline}
        fill="none"
        stroke="var(--color-sparkline, var(--color-primary))"
        strokeWidth="1.5"
      />
      {circles.map((c) => (
        <circle
          key={c.idx}
          cx={c.cx}
          cy={c.cy}
          r="2.5"
          fill="var(--color-sparkline, var(--color-primary))"
          stroke="var(--color-surface-2)"
          strokeWidth="1"
          onMouseEnter={() => onPointHover(c.idx)}
          onClick={() => onPointHover(c.idx)}
          data-testid={`sparkline-point-${c.idx}`}
          style={{ cursor: 'pointer' }}
        />
      ))}
    </svg>
  );
}
```

### Example E-4: Data-decision kpi_reference branch in MessageBubble

```tsx
// Source: web/src/components/chat/MessageBubble.tsx (Phase 17 mod)
import { DataReferenceChip } from './parts/DataReferenceChip';
// ... existing imports

case 'data-decision': {
  const dp = (part as { data?: { decision?: string; dataRef?: DataReference } }).data;
  const decision = dp?.decision;
  if (decision === 'kpi_reference' && dp?.dataRef) {
    return <DataReferenceChip key={i} dataRef={dp.dataRef} />;
  }
  if (decision === 'counter_evidence') {
    return <CounterEvidenceCard key={i} conflictChunks={dp?.conflictChunks} />;
  }
  if (decision !== 'route') return null;
  return <RoutingTracePart key={i} part={...} />;
}
```

### Example E-5: GEVL-09 programmatic value match (Go)

```go
// Source: internal/eval/kpi_metrics.go (NEW)
package eval

// JudgeKPIReferenceConsistency implements GEVL-10 (faithfulness extension to data).
// Returns per-case verdict.
func JudgeKPIReferenceConsistency(c KPICase, answerText string, snapRows []map[string]any) (bool, string) {
    // Step 1: as_of token present
    if !containsASOf(answerText) {
        return false, "missing_as_of"
    }
    // Step 2: channel-name token present
    if !strings.Contains(answerText, "data:aggregated_only") {
        return false, "missing_channel"
    }
    // Step 3: every number in answer matches a snap row (±0.5% tolerance OR exact allowed_expressions)
    numbers := ExtractNumbers(answerText)
    for _, n := range numbers {
        if !matchesAnySnapshot(n, snapRows, c.AllowedExpressions, 0.005) {
            return false, fmt.Sprintf("value_mismatch: %v not in snapshot", n)
        }
    }
    return true, ""
}

// matchesAnySnapshot: 3-step judge per D17-D2.
func matchesAnySnapshot(num float64, rows []map[string]any, allowed []string, tol float64) bool {
    for _, row := range rows {
        for _, v := range row {
            f, ok := toFloat(v)
            if !ok { continue }
            if math.Abs(num - f) / math.Max(math.Abs(f), 1e-9) <= tol { return true }
        }
    }
    for _, expr := range allowed {
        f, ok := toFloat(expr)
        if ok && math.Abs(num - f) / math.Max(math.Abs(f), 1e-9) <= tol { return true }
    }
    return false
}
```

### Example E-6: kpi.lock.json shape (mirror compliance.lock.json + kpi_registry_sha256)

```json
// Source: eval/frozen/kpi.lock.json (NEW — Phase 17 v1.0 milestone re-baseline)
{
  "lock_id": "kpi-v1",
  "model_version": "v1.0",
  "phase": 17,
  "phase_slug": "17-p10-3-kpi-dashboard-skill",
  "schema_version": "v1",
  "created_at": "2026-08-XX",
  "locked_by": "Phase 17 acceptance gate (17-09 hermetic)",
  "eval_set": "eval/golden/kpi.jsonl",
  "eval_set_sha256": "<sha256>",
  "kpi_registry_sha256": "<sha256 of kpis/testdata.yaml content>",
  "model_ver": "glm-5.2",
  "prompt_hash": "<sha256 of rendered system prompt for kpi-dashboard>",
  "seed": 42,
  "case_count": 50,
  "category_counts": {
    "factual_loans": 20,
    "factual_customers": 10,
    "multi_kpi_combination": 10,
    "ambiguous_with_confidence_note": 5,
    "drill_down_to_multi_period": 5
  },
  "thresholds": {
    "kpi_call_accuracy_min": 0.9,
    "value_match_tolerance": 0.005,
    "as_of_required": true,
    "channel_required": true
  },
  "metrics": {
    "by_category_pass": {},
    "by_category_total": {},
    "kpi_call_accuracy": null,
    "refusal_accuracy": null,
    "run_at": null
  },
  "operator_signed_at": null,
  "operator_name": null,
  "lock_sha256": "<canonical body excluding self-field>"
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| recharts (Phase 6 chart library) | Pure inline SVG hand-coded | 2026-08-27 (D17-B1 lock) | Zero new deps; per-point hover + per-point data-reference binding possible; bundle delta 0 |
| Single-period KPI call (Phase 16) | Multi-period `periods: []string` array param + executor loop | 2026-08-27 (D17-B3 lock) | 1 kpi_call → N rows tagged by period; 1 audit row; 1 data-decision part with References[] |
| `wire.DataReference` flat struct (Phase 16) | `DataReference.References []PointReference` field | 2026-08-27 (D17-B2 lock) | Multi-point sparkline data-reference without union expansion; TS AssertAssignable alarm stays green |
| Compliance 50-case eval with LLM judge (Phase 8) | KPI 50-case eval with programmatic 3-step value match (no LLM) | 2026-08-27 (ROADMAP §SC-3) | Deterministic + hermetic-testable + $0 cost; closes eval drift risk |
| Compliance SKILL.md `steps:` as Markdown ordered list | kpi-dashboard SKILL.md `steps:` with embedded fenced YAML routing table | 2026-08-27 (Pattern 2) | Machine-parseable routes for cross-validation; 7-field schema preserved |
| Single `compliance.lock.json` (Phase 8) | Five independent locks (compliance/hitl/fanout/platform-floor) | 2026-08-21 (Phase 13) | Six locks (kpi added in Phase 17); per-Phase gate verifies byte-identity; DATA-18 cross-phase merge deferred to Phase 19 |

**Deprecated/outdated:**
- `wire.DataReference` without `References` field: NOT deprecated — the field is additive (omitempty); old consumers see no change.
- `ParamType` vocabulary (string/enum): extended with `array` (Phase 17). `array` is a NEW vocabulary item; the loader rejects unknown types fail-closed (D16-A3 vocabulary pattern), so adding it must include loader reject-path coverage.
- Tool catalog description `renderCatalog`: per Phase 16, when `array` is added, catalog params summary string should render as `periods:array(N)`.

## Assumptions Log

> All claims in this research are tagged [VERIFIED: code inspection], [CITED: doc], or [ASSUMED]. The planner uses this section to identify decisions needing user confirmation.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `kpi.Registry` content SHA can be computed deterministically from concatenated file contents sorted by filename (Phase 16 implementation already does this for `Signature()`) — extending to also be stored in `kpi.lock.json` is mechanical | Standard Stack / Code Example E-6 | If hash algorithm changes, lock pin invalidates |
| A2 | go-duckdb engine adapter binding `[]string` named parameter works without code changes (DuckDB treats `unnest()` or list-types natively; Phase 16 used `sql.Named(n, stringValue)` for enum-typed strings) | Pattern 1 / Code Example E-2 | If binding requires a new adapter method, executor change cascades to internal/duckdb |
| A3 | `internal/store/sqlite/datasets.go` already has `ListDatasets` returning rows with `Period`; new `Periods(ctx, dataset)` filter is a single query — no migration needed | Pattern 1 / Pitfall 1 | If ListDatasets lacks filtering, new query method needs DDL review |
| A4 | Synthetic 1 万行 DuckDB Push via `internal/data.Importer.Push` succeeds on Windows MSYS2 host (Hermetic CGO concerns are for QUERIES, not WRITES) | Recommended Project Structure / Pitfall 6 | If import also fails on Windows, fallback = pre-built testdata.duckdb committed to repo (adds repo weight, but mirrors Phase 16 WSL test fixtures pattern) |
| A5 | GEVL-09 live eval requires GLM_API_KEY only (NOT BGE/Milvus) | Pitfall 5 / Architecture Patterns | If the agent implicitly triggers retrieval, Milvus becomes required. Mitigated: kpi-dashboard Skill reads ONLY `data:aggregated_only` → Tool set = `{kpi_call}` per D16-C1 → no retrieval path |
| A6 | Skill `temperature: 0.0` for kpi-dashboard works with GLM-5.2 (Phase 8 + 16 precedent both used 0.0 successfully) | Code Example E-3 / Pattern 2 | If GLM-5.2 cannot handle temperature=0 with structured outputs, the prompt would need minor tuning |
| A7 | TestMultiPeriodKPIPeriodsArray running in `internal/kpi/wsl/` (CGO dependency) follows Phase 16 WSL test pattern (`internal/kpi/wsl/snapshot_wsl_test.go`) | Recommended Project Structure / Pitfall 4 | If the WSL test pattern requires additional fix-up, plan-time estimates shift |
| A8 | App-store `pendingDraft` field does not violate D14 dual-source ban (which applies to `messages`, not user-input state) | Pitfall 8 / Pattern 3 | If state-governance review disagrees, fallback = context prop from MessageBubble ancestor — more invasive but works |
| A9 | shadcn `Tooltip` + `Badge` blocks already present from Phase 4 (verified via `web/components.json` + Theme contract) | Code Example E-3 | Already verified by UI-SPEC §Design System; no new shadcn add needed |
| A10 | `eval/frozen/kpi.lock.json` + `reid.jsonl` are NOT merged into `compliance.lock.json` (D17-D4 explicit) | Standard Stack / Alternatives Considered | If the merge decision accelerates to Phase 17, scope expands significantly |

**If this table is empty:** All claims are verified or cited — no user confirmation needed.

> **Open for confirmation:** A2 (go-duckdb array binding semantics) and A4 (synthetic testdata import on Windows) are the two items most likely to bite at execution time. A spike is recommended in plan-time for A2 — see Open Questions §Q1.

## Open Questions (RESOLVED — execution-time spikes)

1. **Q1 (A2): How does go-duckdb bind `[]string` named parameters?**
   - **What we know:** Phase 16 binds `string` and enum values via `sql.Named(name, value)`; DuckDB accepts LIST/VARCHAR types natively. Multi-period KPI function will need to bind `[]string` to `$periods`. Possible patterns:
     - (a) `WHERE period IN (SELECT unnest($periods))` — bind `[]string` directly as a LIST.
     - (b) `WHERE period = ANY($periods)` — DuckDB's ANY syntax; bind as LIST.
     - (c) Multi-arg: `WHERE period IN ($p1, $p2, ...)` — variable-arity, ugly.
   - **What's unclear:** Which pattern go-duckdb supports without CGO adapter changes. (a) is most idiomatic but may require `duckdb.Composite[...]` type adapter.
   - **Recommendation:** Add to 17-01 plan as a 30-min spike: write a one-table hermetic DuckDB test binding `[]string` named param, try (a)/(b), assert one works. If neither, escalate to Plan 17-01b (CGO adapter extension).

2. **Q2 (A4): Can `internal/data.Importer.Push` run on Windows MSYS2 host for the 1 万行 fixture, or must the testdata.duckdb be pre-built in WSL and committed?**
   - **What we know:** Phase 15's `Importer.Push` is CGO-free Go code (writes via DuckDB Appender which is CGO-bound). Phase 16 WSL test pattern puts DuckDB-touching tests in `internal/kpi/wsl/` for WSL-only runs.
   - **What's unclear:** Whether `data/staging/testdata.duckdb` can be created once on WSL and then COMMITTED to the repo (binary file, +1 MB), or whether it must be generated at runtime via the importer (re-import every CI run, WSL-only).
   - **Recommendation:** Pre-build and commit the fixture as part of Plan 17-01 (committing a binary testdata.duckdb is acceptable; Phase 16 already commits `internal/kpi/wsl/testdata/`). Add `internal/data/.gitignore` exclusion if needed.

3. **Q3: GEVL-09 golden-set query naturalness — synthetic vs LLM-rewritten?**
   - **What we know:** CONTEXT §Claude's Discretion: "GEVL-09 50 例 query 文案构造（合成 vs LLM 改写 vs 模板化）". Phase 9 spike precedent: synthetic baseline 50 was operator-approved for adversarial/refusal/multi-hop.
   - **What's unclear:** Whether business-realistic query phrasing is important for GEVL-09 (deterministic routing) vs GEVL-10 (answer text patterns).
   - **Recommendation:** Hybrid: 30 cases are deterministic templates (high-confidence routing), 20 cases are LLM-rewritten (capture natural variance). LLM rewriting uses the same GLM-5.2 + seed=42; recorded in eval set metadata. This keeps GEVL-09 grounded while exercising routing variance.

4. **Q4: REID subset case shape — exact expected_outcome discriminator?**
   - **What we know:** CONTEXT §D17-D3 says "期望结果（`should_refuse: true` 或 `all_values_suppressed: true` 或 `min_suppression_count >= k_min`）". Three forms.
   - **What's unclear:** Whether one case can have multiple expected_outcome fields (e.g., `should_refuse=true` AND `all_values_suppressed=true`), or whether each case has exactly one form.
   - **Recommendation:** One form per case (simpler judge); cases are categorized `expect_refusal` / `expect_all_suppressed` / `expect_partial_with_count`. Mirrors `eval/golden/platform-floor.jsonl` 6-category structure.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All BE code | ✓ | 1.25.0 (win/amd64) | — |
| Node + npm | All FE code | ✓ | Node v24.19.0 / npm present | — |
| Go-duckdb CGO (internal/duckdb) | Live multi-period queries in `internal/kpi/wsl/` | ✗ on Windows | needs WSL2 gcc + libstdc++ | WSL2 Ubuntu-22.04 (operator); Phase 16 precedent `verify-phase-16.sh` Stage 8 literal wsl retry |
| mattn/go-sqlite3 CGO | sqlite audit + datasets registry | ✗ on Windows | needs WSL2 gcc | Same WSL2 fallback |
| GLM-5.2 (GLM_API_KEY) | Live GEVL-09/10 eval (operator-gated) | ⚠ intranet-flaky | per env-intranet-service-flakiness memory | Hermetic gate stays green; live lock re-sign deferred if endpoint down |
| Milvus + BGE-M3 | NOT required for KPI eval (kpi-dashboard reads data:aggregated_only only) | n/a | — | n/a |
| `examples/skills/kpi-dashboard/SKILL.md` | Skill loading | (to be created in Phase 17) | — | — |
| `data/staging/testdata.duckdb` | Eval fixtures | (to be created, likely WSL-only) | — | Pre-build + commit 1 万行 binary |
| `kpis/testdata.yaml` (mini registry) | Eval fixtures | (to be created in Phase 17) | — | — |

**Missing dependencies with no fallback:**
- Windows-host live multi-period tests (Pitfall 4): MUST run in WSL2 — operator responsibility per Phase 14 / 15 / 16 precedent.
- Live GEVL-09/10 lock re-sign (Pitfall 5): blocked by GLM endpoint availability (intranet flakiness memory).

**Missing dependencies with fallback:**
- Synthetic testdata generation: Hermetic in WSL via `internal/data.Importer.Push`; OR pre-build + commit fixture (Q2 Open Question).

## Validation Architecture

> Section included per `workflow.nyquist_validation: true` in `.planning/config.json`.

### Test Framework

| Property | Value |
|----------|-------|
| Go test framework | `testing` + `testify/assert` (Phase 16 precedent) |
| TS test framework | `vitest` + `@testing-library/react` (Phase 4 init) |
| Verify script | `scripts/verify-phase-17.sh` (NEW — 11-stage gate mirroring `verify-phase-16.sh` 11-stage) |
| Makefile target | `verify-phase-17` (NEW — mirrors `verify-phase-16`) |
| Quick run command (Go hermetic) | `go test ./internal/kpi/ ./internal/agent/tools/ ./internal/skill/ ./internal/wire/ ./internal/store/sqlite/ ./internal/eval/ -count=1` |
| Full suite command (Go) | `go test ./... -count=1` (Windows: WSL retry for `./internal/kpi/wsl/`) |
| Quick run command (TS) | `cd web && npx vitest run components/chat/parts/Sparkline.test.tsx components/chat/parts/DataReferenceChip.test.tsx` |
| Full suite command (TS) | `cd web && npx vitest run && npx tsc --noEmit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DASH-01 | 自然语言数字问题 → KPI 调用 → 答案含数值 + 定义 + 时间窗 + as_of | integration (WSL) | `go test ./internal/kpi/wsl/multi_period_wsl_test.go -run TestEndToEndNumericAnswer -count=1` | ❌ Wave 0 |
| DASH-02 | 每个数字带完整出处（hover 通道名） | unit | `go test ./internal/agent/tools/ -run TestDataRefHasChannelName -count=1` | ❌ Wave 0 |
| DASH-02 | TS popup renders channel name | unit | `npx vitest run components/chat/parts/DataReferenceChip.test.tsx -t "channel name"` | ❌ Wave 0 |
| DASH-03 | sparkline 渲染纯 SVG + N circles | unit | `npx vitest run components/chat/parts/Sparkline.test.tsx -t "pure SVG"` | ❌ Wave 0 |
| DASH-03 | sparkline 点 hover 拿对应 index 的引用展示 popup | unit | `npx vitest run components/chat/parts/Sparkline.test.tsx -t "hover"` | ❌ Wave 0 |
| DASH-03 | data-reference part references[] 数组 wire 序列化 | unit | `go test ./internal/wire/ -run TestDecisionDataRefWire -count=1` | ✅ extend existing (Phase 16) |
| DASH-04 | 点 chip → 预填 chat 输入框 | unit | `npx vitest run components/chat/parts/DrillChip.test.tsx -t "prefill"` | ❌ Wave 0 |
| DASH-04 | ChatInput 接收 pendingDraft | unit | `npx vitest run components/chat/ChatInput.test.tsx -t "pendingDraft"` | ❌ Wave 0 |
| DASH-04 | 重调多周期 KPI 走 kpi_call 工具 | integration (WSL) | `go test ./internal/agent/tools/ -run TestDrillChipReinvokesMultiPeriod -count=1` | ❌ Wave 0 |
| DASH-05 | 多 KPI 答案每个数字独立 hover | unit | `npx vitest run components/chat/MessageBubble.test.tsx -t "multi KPI"` | ❌ Wave 0 |
| DASH-05 | Skill steps 模板提示 + LLM tool-call 编排 | unit (string) | `go test ./internal/skill/ -run TestKPISkillStepsRoutingTable -count=1` | ❌ Wave 0 |
| GEVL-09 | 50 例 golden 全过 3 步判定 | integration (eval CLI) | `go run ./cmd/eval --frozen=kpi --archive` (operator-gated for live; hermetic equivalent for CI) | ❌ Wave 0 |
| GEVL-10 | 答案数字 = KPI 快照 rows 数值 + as_of 必出现 + 通道名必出现 | integration (eval CLI) | same as GEVL-09 | ❌ Wave 0 |
| GEVL-09/10 | frozen lock byte-identity | structural | `git diff --quiet eval/frozen/kpi.lock.json && git diff --quiet eval/frozen/reid.jsonl` | ❌ Wave 0 |
| Pitfall 11 | 跨数据集 join 再识别攻击用例 → 拒答 | integration (eval CLI) | `go run ./cmd/eval --frozen=kpi --subset=reid --attack=join` | ❌ Wave 0 |
| Pitfall 22 | KPI 差分再识别攻击用例 → 全 suppressed 或 min suppression count ≥ k_min | integration (eval CLI) | `go run ./cmd/eval --frozen=kpi --subset=reid --attack=differential` | ❌ Wave 0 |
| Channel guard | kpi-dashboard Skill reads 含 `data:aggregated_only` → Tool 集 = {kpi_call} | structural | `go test ./internal/agent/ -run TestReadsChannelGuardXOR -count=1` (D16-C1 mirror) | ❌ Wave 0 |
| No SQL surface | kpi-dashboard Skill 可达路径无 SQL 构造 | structural | `go test ./internal/agent/ -run TestNoSQLGenerationSurface -count=1` (D16 mirror) | ✅ exists (extend for kpi-dashboard roots) |
| Drill down never reaches raw | 下钻 chip 重调 KPI 必走 kpi_call 工具 | structural | `go test ./internal/agent/ -run TestDrillDownNeverReachesRaw -count=1` | ❌ Wave 0 |
| Sparkline no deps | `<Sparkline>` 不引 recharts/react-sparklines/chart.js | structural | `grep -rE "recharts\|react-sparklines\|chart.js" web/src/components/chat/Sparkline.tsx` returns 0 | ❌ Wave 0 |
| Multi-period kpi_calls | 1 调用 → N rows + 1 audit row + 1 data-decision part | integration (WSL) | `go test ./internal/store/sqlite/ -run TestMultiPeriodKPIPeriodsArray -count=1` | ❌ Wave 0 |
| kpi.lock byte-identity | `eval/frozen/kpi.lock.json` 与 TestShippedFrozenLockIsInSync 镜像模式 | structural | `git diff --quiet eval/frozen/kpi.lock.json && grep kpi_registry_sha256 eval/frozen/kpi.lock.json` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit (Hermetic tier):** `go test ./internal/kpi/ ./internal/agent/tools/ ./internal/skill/ ./internal/wire/ ./internal/store/sqlite/ ./internal/eval/ -count=1` + `cd web && npx vitest run components/chat/parts/Sparkline.test.tsx components/chat/parts/DataReferenceChip.test.tsx components/chat/parts/DrillChip.test.tsx` + `cd web && npx tsc --noEmit`
- **Per wave merge (Full suite):** `bash scripts/verify-phase-17.sh` (11-stage gate — mirror verify-phase-16.sh Stages 1-11)
- **Phase gate:** Full suite green before `/gsd:verify-work 17`

### Wave 0 Gaps

- [ ] `internal/kpi/multi_period.go` + `multi_period_test.go` — multi-period resolve + sort + assembly
- [ ] `internal/kpi/wsl/multi_period_wsl_test.go` — live engine integration for multi-period
- [ ] `internal/skill/loader_kpi_test.go` — TestKPISkillStepsRoutingTable (fenced YAML parse + cross-check)
- [ ] `internal/agent/tools/kpi_tool_test.go` extension — TestDataReferenceArrayPart + TestDrillChipReinvokesMultiPeriod
- [ ] `internal/wire/chunk_test.go` extension — TestDecisionDataRefWireReferences
- [ ] `internal/store/sqlite/kpi_calls_test.go` extension — TestMultiPeriodKPIPeriodsArray (1 call → N rows)
- [ ] `internal/eval/kpi_metrics.go` + `kpi_metrics_test.go` — TestThreeStepValueMatch + TestJudgeKPIReferenceConsistency
- [ ] `cmd/eval/main.go` extension — `--frozen=kpi` mode + kpi_registry_sha256 pin + reid sub-gate
- [ ] `examples/skills/kpi-dashboard/SKILL.md` — 7-field YAML + fenced routes YAML inside steps
- [ ] `data/staging/testdata.duckdb` — synthetic 1 万行 fixture (committed binary or generator)
- [ ] `kpis/testdata.yaml` — mini KPI 注册表 (3-5 KPI)
- [ ] `eval/golden/kpi.jsonl` — 50 cases (query → expected_kpi + expected_value + allowed_expressions + expected_period)
- [ ] `eval/frozen/kpi.lock.json` — 5+1 pinned fields (D17-D4)
- [ ] `eval/frozen/reid.jsonl` — Pitfall 11/22 attack cases (D17-D3)
- [ ] `web/src/types/wire-types.ts` lockstep — `DataReference.references?: PointReference[]`
- [ ] `web/src/components/chat/parts/Sparkline.tsx` — pure SVG (zero deps)
- [ ] `web/src/components/chat/parts/Sparkline.test.tsx` — TestSparklinePureSVG / HoverPoint / Autoscale / NoDeps
- [ ] `web/src/components/chat/parts/DataReferenceChip.tsx` — inline number hover wrapper
- [ ] `web/src/components/chat/parts/DataReferencePopup.tsx` — shared popup (single value + sparkline point)
- [ ] `web/src/components/chat/parts/DrillChip.tsx` — Badge-based chip
- [ ] `web/src/store/app-store.ts` extension — `pendingDraft` field + setter
- [ ] `web/src/components/chat/ChatInput.tsx` mod — useEffect reader
- [ ] `web/src/components/chat/MessageBubble.tsx` mod — data-decision kpi_reference branch
- [ ] `scripts/verify-phase-17.sh` — 11-stage hermetic gate
- [ ] `Makefile` — `verify-phase-17` target

## Security Domain

> Required per `security_enforcement` default-enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | n/a (chat auth unchanged from Phase 4) |
| V3 Session Management | yes | session_id stamped on `kpi_calls` audit row (existing Phase 16 pattern) |
| V4 Access Control | yes | `data:aggregated_only` reads channel + singleton registry (D16-C1/C4) |
| V5 Input Validation | yes | Programmatic 3-step numeric match in GEVL-09 (no LLM judge = no injection vector) |
| V6 Cryptography | no | n/a (no new hash algorithms) |
| V7 Error Handling | yes | Refusal taxonomy unchanged (no_matching_kpi / invalid_params / dataset_not_available / result_too_large) |
| V8 Data Protection | yes | INSERT-only audit + kpi_calls signature (KPI-02) + reid.jsonl frozen attack assertions |
| V9 Communication | yes | SSE chunk wire discipline (D16-B3 + D17-B2 — additive field, no union change) |
| V11 Business Logic | yes | D14 platform-floor composition re-asserted at chat + deep path for kpi-dashboard (mirrors Phase 14 tests) |

### Known Threat Patterns for kpi-dashboard stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Synthetic testdata leak via eval prompt | Information Disclosure | Golden set queries never reference real customer data; only synthetic (D17-D1) |
| KPI call returning >1 k_min row with sensitive dimension (Pitfall 11) | Information Disclosure | k_min row-level suppression (D16-D1) + reid.jsonl join-attack assertions + enumerator allowlist (D16-D2) |
| KPI differential attack — calling KPI(a) and KPI(a+b) and subtracting (Pitfall 22) | Information Disclosure | enum/pattern constraint on every param (D16-A3) + reid.jsonl differential assertions; dynamic detection deferred to Phase 18 |
| LLM hallucinating a number not in snapshot | Tampering | GEVL-10 programmatic judge + as_of token presence + channel name token presence |
| LLM fabricating KPI name | Tampering | Tool catalog Enum locks LLM choice to `reg.Names()` (D16-B1) |
| LLM attempting SELECT raw | Tampering | Platform floor item ① "永不编写/执行/建议 SQL" (D14-A1) + structural WalkDir gate TestNoSQLGenerationSurface |
| Routing table poisoned with bogus KPI names | Tampering | TestKPISkillStepsRoutingTable cross-validates against `kpi.Registry.Names()` at boot (Pattern 2) |
| Sparkline XSS via period string | Tampering | Periods are constrained to `^\d{4}-(0[1-9]|1[0-2])$` regex (loader static check); not user-controlled |
| Chip prefill injection | Tampering | `pendingDraft` is plain text; ChatInput renders via existing safe textarea; no HTML interpolation path |

### ASVS Verification by Phase 17 Tests

| ASVS | Verification |
|------|--------------|
| V3 (session) | `TestKPICallsAuditRowHasSessionID` (extend existing Phase 16 test) |
| V4 (access control) | `TestReadsChannelGuardXOR` + `TestSingletonRegistry` (extend existing) |
| V5 (input validation) | `TestGEVL09GoldenSet50` (programmatic 3-step match) + `TestThreeStepValueMatch` |
| V7 (error handling) | `TestRefusalTaxonomy` (extend existing Phase 16 for `periods` shape) |
| V8 (data protection) | `TestKPICallsInsertOnly` (existing) + `TestReidFrozenSubset` |
| V9 (communication) | `TestDecisionDataRefWire` (extend for References) + `TestWireTypeCompileAlarm` (existing) |
| V11 (business logic) | `TestPlatformFloorPrependBothPaths` (existing) — covers kpi-dashboard chat path |

## Sources

### Primary (HIGH confidence — code inspection)

- `internal/agent/tools/kpi_tool.go` — kpi_call Tool closure + DataRefEmitFunc (Phase 16 anchor)
- `internal/agent/tools/retrieve.go` — mirror factory pattern for Tool construction
- `internal/agent/callbacks.go` — two-level identity dispatch (D16-08-A); kpi_reference emission path
- `internal/agent/runner_setup.go` — channel branch in BuildRunner (D16-C3)
- `internal/agent/sql_surface_test.go` — TestNoSQLGenerationSurface + TestKPINarrowSurface
- `internal/kpi/define.go` — ParamSpec / KPIDef / ParamType vocabulary (extensibility point for `array`)
- `internal/kpi/executor.go` — six-step Call orchestration; Snapshot struct; Refusal taxonomy; as_of generation
- `internal/kpi/registry.go` — CompiledKPI / CompiledParam / CatalogEntry / Signature
- `internal/kpi/loader.go` — KPIFile validation flow
- `internal/kpi/wsl/snapshot_wsl_test.go` + `engine_wsl_test.go` + `fixture_wsl_test.go` — WSL test pattern + fixture generation
- `internal/store/sqlite/kpi_calls.go` — INSERT-only audit table (D16-D4)
- `internal/store/sqlite/datasets.go` — existing query surface for `ListDatasets` (multi-period needs extension)
- `internal/wire/chunk.go` lines 100-220 — DataReference + DecisionPayload shape (D17-B2 extension point)
- `internal/wire/chunk.go` lines 326-422 — ValidTypes constant 24 (must stay 24 after D17-B2)
- `internal/skill/loader.go` — Loader.Scan / NewLoader / parseAndValidate (Pattern 2 cross-validation insertion point)
- `internal/skill/manifest.go` — `ReadDataAggregatedOnly = "data:aggregated_only"` + HasDataChannel()
- `cmd/eval/main.go` lines 18-300 — frozen/locked eval runner pattern (cmd/eval --frozen=kpi extension)
- `web/src/types/wire-types.ts` lines 140-340 — DataReference TS mirror + SkillForgeChunk discriminated union + AssertAssignable alarm
- `web/src/components/chat/MessageBubble.tsx` — part dispatcher (data-decision branch insertion point)
- `web/src/components/chat/ChatInput.tsx` — local text state + onSend prop (Pattern 3 prefill reader insertion point)
- `web/src/store/app-store.ts` — draftStatus/draftText (no `messages` per D14); pendingDraft field insertion point
- `examples/skills/compliance/SKILL.md` — 7-field template + reads pattern
- `kpis/loans.yaml` — Phase 16 real KPI YAML (mirror for `kpis/testdata.yaml`)
- `eval/golden/compliance.jsonl` head — 50-case jsonl shape (mirror for `eval/golden/kpi.jsonl`)
- `eval/frozen/compliance.lock.json` — 5-field lock shape (mirror for `eval/frozen/kpi.lock.json`)
- `eval/frozen/platform-floor.lock.json` — schema_version v1 + 6-category attack shape (mirror for `eval/frozen/reid.jsonl` structure)
- `eval/frozen/README.md` — frozen lock convention + byte-identity verification

### Secondary (HIGH confidence — research docs + project planning)

- `.planning/phases/17-p10-3/17-CONTEXT.md` — 16 locked decisions D17-A1..D17-D4 + Claude's Discretion + canonical_refs
- `.planning/phases/17-p10-3/17-UI-SPEC.md` — 4 surfaces (Sparkline / DataReferencePopup / DrillChip / Multi-KPI inline); checker sign-off dimensions 1-6 PASS
- `.planning/phases/16-p10-2/16-CONTEXT.md` — 16 locked decisions D16-A1..D16-D4 inherited (the substrate Phase 17 consumes)
- `.planning/phases/16-p10-2/16-RESEARCH.md` line 698 — Pitfall 11 scenario cross-reference (跨数据集 join)
- `.planning/ROADMAP.md` lines 576-593 — Phase 17 SC 1-5 + Pitfalls 11/19/22 + Research flags (sparkline 渲染形态 + golden set 50 例构造来源)
- `.planning/SKILL_SCHEMA.md` — 7-field schema v1.0 governance (SYSK-02 lock — no 8th field)
- `.planning/REQUIREMENTS.md` lines 153-180, 209-216 — v1.0 DASH-01..05 + GEVL-09..10 + DATA-18 + trace table
- `.planning/STATE.md` Phase 16 closeout + Phase 14 deferred items + Pitfall 19 (eval audit drift) memo

### Tertiary (MEDIUM confidence — system memory + cross-phase precedent)

- env-intranet-service-flakiness memory — GLM/BGE/Milvus intermittent unreachability; live eval deferred if endpoint down
- D16-08-A — two-level identity dispatch + conservative default (Retrieve for unknown tools)
- D16-08-B — DataReference.Step = 0 (honest count, not fabricated)
- D16-08-C — KPI pipeline assembly not gated by SKILLFORGE_DATA_SALT (read-only)
- D16-08-D — LoadKPIs failure = log.Fatalf; empty dir = legal degraded state
- D16-08-E — admin KPI endpoint read-only structural; audit failure → 500
- D16-08-F — storeDatasetMeta in Go side (no new store method)
- D15-01-1 — internal/data + internal/kpi/wsl WSL-only on Windows (emutls ABI mismatch)
- D14-A1..A4 — platform floor 7 items + Chinese copy + prepend + unification
- D14-B4 — floor text change = full re-lock discipline
- D14-C1 — mattn/go-sqlite3 CGO driver + DSN pragma discipline
- D14-E1 — index-after-ALTER ordering contract
- D11-B1 — runtime dynamic Tool whitelist per Skill
- D8-04 — SC-3 ban-gate via WalkDir (no_bypass structural test)
- T-8-12 — compliance.lock.json byte-identity preserved at every phase close

## Metadata

**Confidence breakdown:**

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | All Phase 17 surface code already exists in some form (Phase 16 kpi channel + Phase 4 chat UI + Phase 3 eval runner); zero new external deps; in-tree additions only |
| Architecture | HIGH | System diagram anchored to existing components (kpi_call closure, DataRefEmitFunc, SSE channel, MessageBubble dispatcher); mirror patterns from compliance/fanout precedents |
| Multi-period implementation | HIGH for design, MEDIUM for go-duckdb binding specifics | Layer 1+2 SQL validation + executor extension pattern is locked (Pattern 1); the DuckDB binding mechanism for `[]string` named params is unverified in this session (Open Q1) |
| Sparkline + DataRef wire | HIGH | UI-SPEC §Position 1-4 + Component Contracts DASH-01..05 already locked; mirror data-decision discriminator pattern from Phase 16 |
| GEVL-09/10 eval | HIGH for programmatic match algorithm; MEDIUM for live env | 3-step value match is deterministic + hermetic-testable; live GLM-only environment is intranet-flaky (memory note) |
| Skill steps routing table | HIGH | Pattern 2 (fenced YAML inside steps) preserves SYSK-02 7-field schema governance; loader extension is mechanical |
| Common Pitfalls | HIGH | Each pitfall anchored to a verifiable failure mode from Phase 14-16 (wire lockstep, CGO environment, schema governance, app-store discipline, eval lock drift, streaming order, audit determinism) |
| Security Domain | HIGH | All ASVS controls inherited from Phase 14-16; new threats (routing table poisoning, chip prefill injection) addressed by structural gates |

**Research date:** 2026-08-27
**Valid until:** 2026-09-10 (14 days; Phase 17 ships within 1-2 weeks; no fast-moving external dependencies to monitor)