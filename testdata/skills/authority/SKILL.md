---
description: 权威 tier compliance auditor. When user asks about official policy, authoritative regulation, or current effective rules, this skill reads only the 权威 tier.
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Read relevant policy chunks via tier filter
  2. Compose draft verdict with citations (chunk_id + verbatim span)
  3. Self-verify each claim (Go programmatic grounding check)
  4. Refuse if no authority chunks retrieved
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never modify authority tier content"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
---

# Compliance Review Skill — Authority Tier

权威 tier 的合规审查。当用户询问"现行有效制度"、"权威发布版本"等条款时，限定读取 `权威` tier 的法规、政策与正式发布版本。

## Sample Clause (权威 tier)

> 出差住宿费超标 200 元以内可由部门负责人审批。超过 200 元的部分需上级主管与财务部联合审批。跨城市出差住宿标准按职级执行（A 类城市一线、B 类城市二线、C 类城市三线分别执行）。