---
title: SkillForge Design Philosophy (Product Layer + Agent Layer)
date: 2026-08-10
context: Established during /gsd:explore — SKILL.md 6 fields → 7 fields (added `temperature`); "智能体给出确定的唯一的正解" as agent-layer philosophy
slug: skillforge-design-philosophy
---

# SkillForge 设计哲学

## 两层哲学（产品 + Agent）

### 产品层（Core Value in PROJECT.md）

> "同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码"

→ 描述**企业怎么用** SkillForge（Skill 驱动 / 平台化）

### Agent 层（本文件新增）

> **"智能体给出确定的唯一的正解"**

→ 描述 **agent 怎么输出**（确定性 / 可审计 / 可拒绝）

两层互补：**产品层（how to use）+ agent 层（how to act）**。

## "确定性正解"的工程化（最小可行集合）

不是"什么都是正解"，而是"**该确定的就确定，该创作的就承认创作**"。

| 机制 | 角色 | 来源 |
|---|---|---|
| `temperature` 字段（per-Skill） | 最大化生成确定性 | 新增 v1.0（第7字段） |
| Structured output | 输出可审计 / 可消费 | Anthropic / OpenAI |
| 真源分离（`reads` 字段硬过滤） | 输出空间被约束 | denova 灵感 |
| 操作边界（`writes` 字段） | 行动空间被约束 | denova 灵感 |
| 拒答护栏（`guardrails` 字段） | 边界外严格拒绝 | SkillForge 关键 |
| Self-RAG 自检 | 起草后自评每个声明 | ARAG-03 |
| Ragas faithfulness ≥0.9 (CI gate) | 量化"忠于率" | GEVL-07 |

## 创作场景的承接

"确定性正解"哲学**不是普适** —— 创作场景（营销文案 / 战略建议 / 月报叙事）没有唯一正解。

承接方式：每个 Skill 自带 `temperature` 字段（SKILL.md v1.0 第7字段）：

- 合规 / 数据 / 引用 Skill → `temperature: 0.0`（完全确定）
- 月报 / 报告 Skill → `temperature: 0.3–0.5`（中间值）
- 营销 / 创作 Skill → `temperature: 0.7–1.0`（高创意）

→ **同一平台，多种 agent 行为**，每个 Skill 自己定义"我是确定性的还是创作性的"。

## 关键 design tension 已解决

| Tension | 解决方式 |
|---|---|
| "确定性正解" vs LLM 概率本质 | 工程层对抗（温度 0 + 真源分离 + 拒答 + Self-RAG + Eval gating） |
| "工程化完整性" vs "Skill 难写" | 6 字段 → 7 字段，全部 REQUIRED → 强制明确，但**Skill Wizard 可自动生成** |
| "合规" vs "业务灵活性" | 双轨数据架构（DATA-01..08，详见 finance-compliance-data-architecture.md Note） |

## 历史决策

| Date | Decision | Rationale |
|---|---|---|
| 2026-08-10 | SKILL.md 6 字段 → 7 字段（+ `temperature`） | 用户"最工程化"+"确定性正解"哲学；让平台同时支持确定性与创作性 Skill |
| 2026-08-10 | `temperature` 是 REQUIRED 字段（无默认） | 强制每个 Skill 显式声明 determinism level |
| 2026-08-10 | "智能体给出确定的唯一的正解" as agent-layer design philosophy | 用户在 /gsd:explore 中明确 |
| 2026-08-10 | 数据层 = 平台基础设施 + 双轨（聚合 + 个体识别） | 金融合规场景 explore（finance-compliance-data-architecture.md Note） |

## 关联 artifacts

- `.planning/PROJECT.md` — Core Value（产品层哲学）
- `.planning/SKILL_SCHEMA.md` — 7 字段 schema 官方规范
- `.planning/REQUIREMENTS.md` — SYSK-01..08（schema 系统需求）
- `.planning/notes/finance-compliance-data-architecture.md` — 数据层架构
- `.planning/seeds/finance-data-skills.md` — 6 个金融 Skill 触发条件