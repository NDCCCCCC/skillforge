---
description: 个人 tier compliance auditor. When user asks about individual records, personal leave, or single-employee matters, this skill reads only the 个人 tier.
agent: ChatModelAgent
reads:
  - personal/**
writes: []
steps: |
  1. Read relevant individual-record chunks via tier filter
  2. Compose draft response with citations
  3. Refuse if no personal chunks retrieved
guardrails:
  prohibitions:
    - "Never cite records not retrieved from personal tier"
    - "Never reveal records belonging to other employees"
    - "Never modify personal tier content"
  required_refusal_phrases:
    - "no_authorized_source"
    - "out_of_scope"
  tier_limits:
    personal: ["read-only"]
temperature: 0.0
---

# Compliance Review Skill — Personal Tier

个人 tier 的合规审查。当用户询问"我的年假"、"我的报销"、"某员工记录"等个体记录相关问题时，限定读取 `个人` tier 的个体数据。

## Sample Clause (个人 tier — individual record only)

> [个人记录] 员工 E-1042 累计年假 12.5 天，其中去年转入 3 天（按公司制度，过期转入最多保留 5 天）。本年度已使用 5 天，剩余 7.5 天。不得用于回答其他员工问题。