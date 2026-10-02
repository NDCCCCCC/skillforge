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
      - triggers: [贷款余额, 未偿还本金, loan balance, outstanding, 上月贷款余额]
        kpi: KPI_LOAN_BALANCE
      - triggers: [不良率, NPL ratio, 逾期率, 上月不良率]
        kpi: KPI_NPL_RATIO
      - triggers: [利息收入, interest income, 利息]
        kpi: KPI_INTEREST_INCOME
    customers:
      - triggers: [客户数, 活跃客户, customer count, 上月客户数]
        kpi: KPI_CUSTOMER_COUNT
  ```

  ## 调用流程
  1. 根据业务词查路由表 → 选 KPI 名
  2. 调 kpi_call(name, params)，不得尝试 SQL 或 SELECT
  3. 自然语言答：数值 + KPI 定义 + 时间窗 + as_of（必须出现）+ 数据来源 `data:aggregated_only`
  4. 多 KPI 答案：每个数字独立 hover；可下钻「看月度趋势」

  ## period 参数构造规则（重要）
  - period 必须是 `YYYY-MM` 格式的**具体月份**（如 `2026-07`），绝不传「上月」「本月」等相对词。
  - 相对表达按数据的最新已注册周期换算：「上月」= 最新周期的上一个月，「本月/最新」= 最新周期本身。
  - 若拒答提示周期不可用（dataset_not_available / invalid_params），从拒答信息中取可用周期重试一次；仍失败则如实告知用户可用周期范围。
  - 多周期趋势用 `periods` 数组参数（升序列出各月），如 `periods: ["2026-01","2026-02",...]`。

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
---
