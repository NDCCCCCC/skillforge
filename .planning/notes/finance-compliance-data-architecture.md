---
title: Finance Compliance Data Architecture
date: 2026-08-10
context: Added scenario during /gsd:explore — financial industry compliance data analysis
slug: finance-compliance-data-architecture
---

# 金融合规数据分析场景设计决策

## 触发
用户在 new-project 完成 + Phase 1 待启动时，追加讨论金融业合规场景：
业务部门需要数据决策支持 → 但敏感数据不能保存到本地 → 监管严格。

## 核心架构决策：方案 1+4 混合（双轨数据架构）

### 数据层定位
- **平台基础设施层**（A 定位，与 denova "资料库" 对齐）
- 多 Skill 共享
- Skill 只关心"分析逻辑"，不关心"数据从哪来 / 怎么脱敏"

### 双轨 Skill 路由
| Skill 类型 | 数据通道 | 脱敏策略 |
|---|---|---|
| **聚合分析类**（风控趋势 / 业务月报 / KPI 仪表盘） | SkillForge 内部 | 极致脱敏（方案 1）：导入即脱敏，原始数据 **不**进 SkillForge |
| **个体识别类**（异常客户定位 / 客户归因） | 外部授权通道 | 原始数据在用户数仓，agent 调外部 query interface；数据人员预授权 + 字段级二次脱敏 + 双重 audit |

### 字段分层（L1 / L2 / L3）
| 层级 | 字段类型 | 例子 | 监管依据 |
|---|---|---|---|
| L1 | PII（个人身份） | 姓名 / 身份证 / 银行卡号 / 手机号 | 《个人信息保护法》/ GDPR / PCI-DSS |
| L2 | 业务敏感 | 总资产 / 交易金额 / 信用分 / 持仓 | 公司内部 access control + 监管披露要求 |
| L3 | 业务字段（一般不脱） | 产品名 / 业务量 / 地区 / 时间 | —— |

脱敏粒度是**设计选择**（不是技术限制）：
- **极严**（L1+L2+L3 聚合）：合规第一，业务部门只能看趋势
- **中等**（L1 哈希，L2 限精度，L3 完整）：agent 能做财务分析
- **轻度**（只 L1 哈希）：业务部门能看大部分

用户选定：**极严（聚合分析 Skill）+ 双层授权（个体识别 Skill）**

## 与 SkillForge 现有架构的融合

### 真源分离的扩展
现有 `reads` 字段扩展支持：
- `reads: ["data:aggregated_only"]` → 聚合分析通道
- `reads: ["data:external:authorized"]` → 外部授权通道

### 6 个新 Skill（占位，待 Phase 5 落地）
1. **风控趋势 Skill** — 异常指标 / 同比 / 环比（聚合分析通道）
2. **业务月报 Skill** — 自动生成月报（聚合分析通道）
3. **KPI 仪表盘 Skill** — 实时指标（聚合分析通道）
4. **异常客户定位 Skill** — 走外部授权通道（个体识别）
5. **客户归因 Skill** — 走外部授权通道（个体识别）
6. **合规月报 Skill** — 自动生成合规报告（聚合分析通道）

## agent 能力边界（重要）

**agent 能做的**：
- 同比 / 环比 / 增长率
- 占比 / 趋势 / 漏斗 / KPI
- 异常指标告警
- 业务月报 / 季报自动生成

**agent 做不到的**：
- "哪些客户 / 账户异常"（个体识别）
- "这个客户为什么违约"（个体归因）

**业务部门工作模式变化**：从"自己定位异常"变为"agent 出指标 → 让人去定位"。这是真痛点，但很多金融监管强的企业就是这么走的。

## 与现有 7 阶段路线图的整合
- **Phase 2-3**（Compliance Vertical Slice / Agentic Retrieval）：现有架构已能覆盖基础；金融数据场景可在 Phase 5 集中落地
- **Phase 5**（Skill Platform）：**新增 6 个金融 Skill + 数据层（DATA-01..08）是核心扩展点**
- **Phase 7**（Open-Source Release）：金融场景作为"差异 demo" 提升品牌（金融垂直场景是 SkillForge 与通用 agent 平台的差异化护城河）

## 关键 trade-offs
- **合规 vs 业务灵活性**：选合规优先（方案 1 极严），业务灵活性让步
- **架构简洁 vs 业务深度**：选简洁（A 平台层），业务深度通过多 Skill 表达
- **NL2SQL 风险**：agent 不能写 SQL（防绕过脱敏），只能调预定义 KPI（research/questions.md Q1）