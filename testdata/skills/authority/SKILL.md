---
name: compliance-authority
description: 权威 tier compliance auditor. When user asks about official policy, authoritative regulation, or current effective rules, this skill reads only the 权威 tier.
agent: ChatModelAgent
reads: ["权威"]
writes: []
temperature: 0
reasoning: false
guardrails:
  refuse_on_missing_tier: true
---

# Compliance Review Skill — Authority Tier

权威 tier 的合规审查。当用户询问"现行有效制度"、"权威发布版本"等条款时，限定读取 `权威` tier 的法规、政策与正式发布版本。

## Sample Clause (权威 tier)

> 出差住宿费超标 200 元以内可由部门负责人审批。超过 200 元的部分需上级主管与财务部联合审批。跨城市出差住宿标准按职级执行（A 类城市一线、B 类城市二线、C 类城市三线分别执行）。
