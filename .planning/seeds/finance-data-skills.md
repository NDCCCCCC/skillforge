---
title: 金融数据分析 Skill 一组（6 个）
trigger_condition: 当 Phase 5 (Skill Platform & Full Knowledge Boundaries) 实施时
planted_date: 2026-08-10
---

# Seed: 金融数据分析 Skill 一组

## 想法
Phase 5 实施 Skill Platform 时，同步落地 6 个金融数据分析 Skill：

1. **风控趋势 Skill** — 异常指标 / 同比 / 环比（聚合分析通道）
2. **业务月报 Skill** — 自动生成月报（聚合分析通道）
3. **KPI 仪表盘 Skill** — 实时指标（聚合分析通道）
4. **异常客户定位 Skill** — 走外部授权通道（个体识别）
5. **客户归因 Skill** — 走外部授权通道（个体识别）
6. **合规月报 Skill** — 自动生成合规报告（聚合分析通道）

## 与现有架构的依赖

- 需要 `reads` 字段扩展支持 `data:aggregated_only` 和 `data:external:authorized`（DATA-07）
- 需要数据层（DATA-01..08 requirements）落地
- 需要外部授权通道的具体接口协议（research/questions.md Q3）
- 需要 NL2SQL 安全机制（research/questions.md Q1）
- 需要双层 audit（DATA-06）

## 触发时建议

- **优先落地 1-2 个聚合分析 Skill**（方案 1 通道），验证数据层架构
- 外部授权通道作为 Phase 5+ 增量（更复杂）
- **合规月报 Skill 可作为 "showcase"**（金融垂直场景 + 完整方案演示）

## 价值

- **显著区分 SkillForge 与通用 Agent 平台**（金融垂直场景是差异化护城河）
- 让 Phase 7 (Open-Source Release) 有更丰富的 demo 内容
- 让个人品牌定位更清晰（"做金融级合规 Agent"而非"通用 Agent"）

## 关联 artifacts

- `.planning/notes/finance-compliance-data-architecture.md` — 架构决策详情
- `.planning/REQUIREMENTS.md` — DATA-01..08 新增需求
- `.planning/research/questions.md` — 5 个未决研究问题