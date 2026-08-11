# Phase 4: Streaming Trust UI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-11
**Phase:** 4-Streaming Trust UI
**Areas discussed:** Center panel & citation IA, Mock transport strategy, Verdict & refusal visual, Reasoning display

**Context note:** This discussion opened as `/gsd-discuss-phase 3`, then pivoted to Phase 4 after a dependency analysis (user goal: parallelize a phase that doesn't depend on the in-flight Phase 2). Mock policy was confirmed mid-discussion: mock/fixture allowed during development; product demo + Ragas eval use real data. Under that policy, Phase 4 is nearly independent of Phase 2 (mock-driven UI) and was selected as the parallel track.

---

## Center Panel & Citation Information Architecture

| Option | Description | Selected |
|--------|-------------|----------|
| 载入中面板 | citation → chunk 原文载入中 panel 文档预览区；tier badge/version/effective date 置顶；符合 denova 模式，不引入第 4 panel | ✓ |
| 弹出右侧抽屉 | 右边缘滑出 overlay 抽屉；字面贴合 SC-2 "side panel"，但右列已是 chat，会挤压 | |
| 内联展开 | 消息内 citation 下方展开；最轻量，但长原文打断对话流 | |

**User's choice:** 载入中面板
**Notes:** Reconciles sketch "center=doc preview" with SC-2 "side panel" — center IS the side panel relative to right chat.

| Option (default content) | Description | Selected |
|--------------------------|-------------|----------|
| Verdict card | 默认显示最新合规判定 card；点 citation 切 chunk 原文；顶部 tab 可切 SKILL.md | ✓ |
| SKILL.md 预览 | 默认显示当前 Skill 定义；强化 Skill 驱动叙事 | |
| 空状态占位 | 占位提示；最简，但浪费核心视觉位 | |

**User's choice:** Verdict card

---

## Mock Transport Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| 内嵌 mock transport | 自定义 AI SDK Transport 从 fixture 回放 UIMessageChunk 序列；零网络纯前端，vitest 可断言；真实接线只换 transport 实例 | ✓ |
| mock Hertz 端点 | Go 侧/Vite middleware 加 mock SSE 端点；更接近 wire，但要写后端 mock，可能与 P2 会话冲突 | |
| 录制回放真实流 | 等 P2 真实 agent 录流再回放；最真实，但要等 P2，违背并行初衷 | |

**User's choice:** 内嵌 mock transport
**Notes:** Fixture set is dual-use (vitest + dev), with configurable streaming delay. Coverage: 4 verdicts / 3 refusals / multi-part / sub-progress / citation+tier.

---

## Verdict & Refusal Visual

| Option (verdict card) | Description | Selected |
|-----------------------|-------------|----------|
| 色头+置信度条 | status 主色 header + confidence 数字&进度条 + review_required 黄角标 + citations 计数 | ✓ |
| 极简色块 | 图标主导，confidence 折叠；最直观但置信度不显眼 | |
| 仪表盘 gauge | 半圆 gauge；视觉重，占空间 | |

**User's choice:** 色头+置信度条

| Option (refusal distinction) | Description | Selected |
|------------------------------|-------------|----------|
| 蓝灰 vs 灰 | out-of-scope=蓝灰边框+🚫；I-don't-know=灰边框+❓；结构化 reason/suggested query | ✓ |
| 统一灰仅文案 | 简单，但两种 refusal 难一眼区分 | |
| 黄 vs 灰 | out-of-scope 用黄；与"需补充黄/review_required 黄"冲突 | |

**User's choice:** 蓝灰 vs 灰
**Notes:** sketch's status palette had only one gray (unanswered); this adds blue-gray for out-of-scope to keep the two refusal semantics visually distinct.

---

## Reasoning Display

| Option | Description | Selected |
|--------|-------------|----------|
| 渐进披露 | 默认展开限高预览（前几行+暗色可滚动区）置于 answer 上方；一键全展开；实时流式填充 | ✓ |
| 默认折叠摘要 | 默认折叠显示首行摘要；最省空间，但"默认可见"打折 | |
| 默认全展开 | 完全展开；最透明，但长链（1000+ token）淹没 answer | |

**User's choice:** 渐进披露
**Notes:** Specifically guards against GLM-5.2 reasoning-model long chains burying the answer while still satisfying SC-1 "visible by default, collapsible".

---

## Claude's Discretion

- Left sidebar (session/file-tree/skill-list) = mock skeleton in P4; real management → Phase 5.
- shadcn/ui + Tailwind v4 theme落地, Zustand state boundary, source-url sanitization (T-1-08), sub-progress data-* events — captured in CONTEXT.md D-series / Discretion.

## Deferred Ideas

- Real multi-session mgmt, knowledge graph browser, counter-evidence/redline, policy-version filter UI → Phase 5.
- Real sub-progress decision events → wire after Phase 3.
