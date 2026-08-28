---
description: 业务月报生成。当用户说「出月报」「2026-08 月报」「生成月度报告」时触发。
agent: deep
reads:
  - data:aggregated_only
writes:
  - report:publish
  - plan_approve
steps: |
  1. 解析用户请求的 period（支持「2026-08」「2026年8月」「上月」「本月」等格式）。
  2. 调用 plan_approve 确认报告结构（章节 + KPI 集 + 叙事槽位类型）。
  3. 使用 fan_out_dispatch 决定是否并行取数：
     - should_fan_out=true：每个章节并行调用 kpi_call 取数。
     - should_fan_out=false 或分类失败：顺序逐章节取数。
     - task_boundaries 必须与模板 sections.title 逐字匹配。
  4. 基于取数结果填写叙事槽位：
     - narrative：业务描述文字（LLM 自由叙事，不引用具体数字）。
     - insight：基于 KPI 值的分析点评（数字必须来自 KPI 快照）。
     - risk_note：风险提示，必须引用具体 KPI 数字（如「NPL 率 1.23%」）。
  5. 调用 report:publish 提交审批（附完整参数快照）。
  6. 审批通过后发布报告，提供「带修改重生成」chip。
guardrails:
  prohibitions:
    - "Never write a number that is not server-injected from a KPI snapshot"
    - "Never SELECT raw rows or attempt SQL"
    - "Never overwrite an existing report version file"
    - "Never cite external knowledge outside the KPI data"
  required_refusal_phrases:
    - "period_not_found"
    - "insufficient_kpi_data"
temperature: 0.0
template:
  title: "{period} 业务月报"
  sections:
    - title: 业务概览
      kpis:
        - KPI_LOAN_BALANCE
        - KPI_CUSTOMER_COUNT
      slots:
        - type: narrative
        - type: insight
    - title: 贷款质量
      kpis:
        - KPI_NPL_RATIO
        - KPI_INTEREST_INCOME
      slots:
        - type: narrative
        - type: risk_note
    - title: 风险提示
      kpis:
        - KPI_NPL_RATIO
        - KPI_INTEREST_INCOME
      slots:
        - type: risk_note
    - title: 客户情况
      kpis:
        - KPI_CUSTOMER_COUNT
        - KPI_INTEREST_INCOME
      slots:
        - type: narrative
        - type: insight
---

# 业务月报 Skill

业务月报生成工具。读取聚合数据（`data:aggregated_only`），生成固定结构的月度报告。

## 模板（template:）

月报模板定义报告结构和叙事槽位：

```yaml
template:
  title: "{period} 业务月报"
  sections:
    - title: 业务概览
      kpis: [KPI_LOAN_BALANCE, KPI_CUSTOMER_COUNT]
      slots:
        - type: narrative
        - type: insight
    - title: 贷款质量
      kpis: [KPI_NPL_RATIO, KPI_INTEREST_INCOME]
      slots:
        - type: narrative
        - type: risk_note
    - title: 风险提示
      kpis: [KPI_NPL_RATIO, KPI_INTEREST_INCOME]
      slots:
        - type: risk_note
    - title: 客户情况
      kpis: [KPI_CUSTOMER_COUNT, KPI_INTEREST_INCOME]
      slots:
        - type: narrative
        - type: insight
```

## 槽位填写规则

- **narrative**: LLM 自由撰写业务描述，不引用具体数字。
- **insight**: LLM 基于 KPI 快照值撰写分析点评；数字必须来自快照。
- **risk_note**: LLM 必须引用具体 KPI 数字（如「NPL 率 1.23%」），不得自由发挥。

## 发布

发布成功后聊天内显示报告卡片，含「查看报告」链接（新 tab 打开）和「带修改重生成」chip。
