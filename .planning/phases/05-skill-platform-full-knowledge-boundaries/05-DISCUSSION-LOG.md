# Phase 5: Skill Platform & Full Knowledge Boundaries - Discussion Log

**Gathered:** 2026-08-12
**Mode:** default (interactive, AskUserQuestion)
**Human reference only** — not consumed by researcher/planner/executor (see `05-CONTEXT.md` for the canonical decisions).

---

## Scope gate (pre-discussion)

- **Q: Phase 5 范围** — DATA-* 金融数据层（14 需求）在 REQUIREMENTS traceability 映射到 Phase 5，但 ROADMAP 只列 8 SC。
  - **A: 仅 ROADMAP 8 SC（推荐）** — DATA-* 延期到独立 phase / v2；建议修 traceability + Coverage（55 vs 70）漂移。

## Area 1 — 语义路由机制 (SC-1, SYSK-05)

| Q | Options presented | Selection |
|---|---|---|
| 主路由机制 | BGE-M3 embedding 索引 / LLM 意图分类器 / embedding 粗筛+LLM 精排 / Claude 裁量 | **BGE-M3 embedding 索引** |
| embedding 存哪 | 内存 map / Milvus 单独 collection / Claude 裁量 | **内存 map** |
| 歧义行为 | 反问用户澄清 / 跑 top-1+标低置信 / 默认 fallback Skill / Claude 裁量 | **反问用户澄清** |
| trace wire | 扩展 D16 data-decision / 新增 data-routing chunk / Claude 裁量 | **扩展 D16 data-decision** |
| 路由层级 | 路由在 ADK 之前 / ADK NewConditionalAgent 包进图 / Claude 裁量 | **路由在 ADK 之前** |

## Area 2 — 热重载会话内语义 (SC-2, SYSK-04)

| Q | Options presented | Selection |
|---|---|---|
| in-flight run | 版本钉死 / 立即生效（中断重跑）/ Claude 裁量 | **版本钉死** |
| session 版本 | 每 turn 解析最新版 / session 绑定首版本 / Claude 裁量 | **每 turn 解析最新版** |
| reload 失败 | 保留旧版+报错 / 卸载该 Skill / Claude 裁量 | **保留旧版+报错** |

Assumption (not separately asked): reload scope = Skill definition only; knowledge reindex ∉ SC-2.

## Area 3 — Skill 向导交互 (SC-3, SYSK-06)

| Q | Options presented | Selection |
|---|---|---|
| 交互模型 | 引导式多轮访谈 / 一键单次生成 / 模板+LLM 填充 / Claude 裁量 | **引导式多轮访谈** |
| 产物落点 | 预览+确认后落盘 / 仅预览+下载 / 自动落盘+注册 / Claude 裁量 | **预览+确认后落盘** |
| temperature | 按类型推荐 / 用户自填 / Claude 裁量 | **按类型推荐** |

## Area 4 — 反证 + 红线 + 版本 (SC-6/7/8, CMPL-06/07/08)

| Q | Options presented | Selection |
|---|---|---|
| SC-6 反证架构 | 平台层后置 pass / reads 扩到 [权威,历史] / chunk 元数据标冲突 / Claude 裁量 | **平台层后置 pass** |
| SC-7 红线 wire | gap 时独立 pass+新 data-redline chunk / verdict 同次附带 / Claude 裁量 | **gap 时独立 pass+新 chunk** |
| SC-8 版本过滤 | 扩展 schema+PRE-filter / store 版本表+join / Claude 裁量 | **扩展 schema+PRE-filter** |

## Claude's Discretion (captured without user questions)

- **SC-4 (AGNT-06) Agent State Inspector** — reuse existing D16 + tool/reasoning SSE events as data source; new UI panel renders structured tree; no new instrumentation channel.
- **SC-5 (KNOW-06) Knowledge graph browser** — tier→doc→chunk within locked three-panel layout; store + knowledge metadata; lean lightweight tree/list over heavy d3.

## Not discussed (flagged for research/planning)

- Routing / ambiguity threshold values → researcher calibrates from golden set (D2-15 pattern).
- Wizard interview step count / field order → planner.
- `data-redline` exact fields + diff format → planner.
- `effective_from/to` per-backend index syntax → researcher validates (Milvus expr / Mongo query).

---
*All four areas resolved on 2026-08-12. All user selections were the recommended option.*
