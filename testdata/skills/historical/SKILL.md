---
description: 历史 tier compliance auditor. When user asks about superseded rules, prior versions, or historical policy changes, this skill reads only the 历史 tier.
agent: ChatModelAgent
reads:
  - historical/**
writes: []
steps: |
  1. Read relevant archived chunks via tier filter
  2. Compose draft verdict with citations
  3. Refuse if no historical chunks retrieved
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from historical tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never modify historical tier content"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    historical: ["read-only"]
temperature: 0.0
---

# Compliance Review Skill — Historical Tier

历史 tier 的合规审查。当用户询问"历史版本"、"旧版制度"、"修订前条款"时，限定读取 `历史` tier 的归档版本与变更记录。

## Sample Clause (历史 tier — superseded)

> [历史归档 v2023-04] 出差住宿费超标 100 元以内可由部门负责人审批；超过 100 元的部分需上级主管审批。本条款自 2024-Q1 起废止，被 "超标 200 元以内由部门负责人审批" 替代（见权威 tier v2024-01）。