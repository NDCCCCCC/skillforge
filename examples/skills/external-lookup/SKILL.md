---
description: 外部授权数据查询（个体级）。触发场景：需要查某个员工/客户的个体级记录（如上月消费明细、账号、联系方式），且数据在预编译视图内。先经数据人员审批，再返回按视图掩码规则脱敏的行。
agent: chat
reads:
  - data:external:authorized
writes: []
steps: |
  # 外部授权通道查询流程（approve-before-query）

  ## 调用流程
  1. 识别用户请求是否指向个体级数据；确认请求能落到一个已登记的预编译视图（见 external_views/ 受控文件），否则如实拒绝。
  2. 调 needs_external_query(query, business_reason)：
     - 工具只生成只读 SQL 预览并进入人工审批，绝不直接返回行数据。
     - 返回 status=pending 时，向用户呈现「待数据人员审批」卡片，并告知审批单号（query_id）。
  3. 审批是持续等待状态：不要轮询、不要重复提交等价请求、不要自行判定超时。数据人员批准或拒绝是唯一的状态变更方式。
  4. 数据人员批准后，再次调用 needs_external_query(query, query_id) 取回结果：
     - 返回的行已按视图掩码规则脱敏（如账号仅保留后 4 位、邮箱仅保留前 2 位）。
     - 如返回 token_expired，说明授权凭证已到期，须重新走一遍审批，不得重试旧 query_id。
  5. 数据人员拒绝时，向用户如实转达拒绝结果，不得换措辞绕过审批再次提交。
  6. 结论必须说明：数据来自外部授权通道、字段已脱敏、以及审批单号（可追溯）。

guardrails:
  prohibitions:
    - "Never request raw rows outside the approved precompiled view, and never attempt raw-table access"
    - "Never attempt to re-identify or un-mask a masked field, and never present masked values as complete data"
    - "Never claim real-time warehouse data; state that rows reflect the approved query at approval time"
    - "Never bypass or re-word around a rejected approval to resubmit an equivalent request"
  required_refusal_phrases:
    - "no_data"
  tier_limits:
    "data:external:authorized": ["needs_external_query only; approval required before any row is returned"]
temperature: 0.3
---
