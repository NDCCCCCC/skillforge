# Phase 23 — Plan Outline (4 waves)

## Wave 1 — README 重排（核心交付）
- §2 "Why SkillForge?" 三段改写为 ROADMAP 指定的三段：
  - **approve-and-remember 审批门一等公民原语**（Phase 11 落地）
  - **信任契约内核**（Phase 1-5 落地）
  - **评测门禁**（Phase 6 + 22 落地）
- 降权/收窄的叙事改为"支撑能力"段（不删，留为事实）
- 新增 §"SkillForge vs WeKnora vs Semantica"三分位对照表
- 新增 §"What is a Skill" 去混淆段（WeKnora Skill vs SkillForge Skill）
- README 顶部 tagline 摘要："SkillForge = execution-time trust boundaries
  for AI agents: 什么不许答、什么必须批。"——区分查得准 (WeKnora) /
  为什么 (Semantica) / 不许答/必须批 (SkillForge)

## Wave 2 — CHANGELOG.md v1.1.0
- `## v1.1.0 — 2026-09-06` 节
- 列出 Phase 25 / 24 / 22 / 21 的可观察成果
- 不变式 / 向后兼容声明（全部 additive + omitempty；Wire 形状不破）
- 标注升级路径：v1.0 → v1.1 是配置层增量，schema/store DDL 自动迁移

## Wave 3 — RELEASE-NOTES.md
- 用户面向：3 条 headline + 一张新能力清单
- Migration notes：env 增量（`LOCAL_API_KEY` / `LOCAL_BASE_URL` /
  `LOCAL_LLM_THINK` / `HISTORY_LAYER_TIMEOUT` / `VERDICT_STREAM_TIMEOUT`
  / `EVAL_CASE_TIMEOUT`）
- Breaking changes：NONE（v1.1 全部 additive）
- 竞品定位摘要（一句）：SkillForge 是唯一一个在执行时绑 tier 隔离 /
  拒答构造 / 审批门 / 评测门禁 4 个信任原语的开源 Agent 平台。

## Wave 4 — CONTRIBUTING.md 定位指南 + 闭环
- CONTRIBUTING.md 顶部加一段"How to position SkillForge"——指引后续
  贡献者在 PR 描述 / docs / talks 中引用三分位定位，而不是回到旧三段
- 写 `.planning/uat/phase23-oss-2026-09-06.md` 闭环报告
- commit
