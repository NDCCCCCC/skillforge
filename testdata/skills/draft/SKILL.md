---
description: 草稿 tier compliance auditor. When user asks about draft policies under review, this skill reads only the 草稿 tier.
agent: ChatModelAgent
reads:
  - draft/**
writes: []
steps: |
  1. Read relevant draft chunks via tier filter
  2. Compose draft verdict with citations
  3. Refuse if no draft chunks retrieved
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from draft tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never modify draft tier content"
    - "Never treat draft content as authoritative"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    draft: ["read-only"]
temperature: 0.0
---

# Compliance Review Skill — Draft Tier

草稿 tier 的合规审查。当用户询问"草案"、"征求意见稿"、"草拟条款"时，限定读取 `草稿` tier 的未发布版本。所有草稿内容必须标注 `[DRAFT]` 前缀以避免被误用为权威条款。

## Sample Clause (草稿 tier — DRAFT, not yet effective)

> [DRAFT 2026-09-拟] 出差住宿费超标 300 元以内可由部门负责人审批。超过 300 元的部分需上级主管与财务部联合审批。**本条款尚未生效，正式实施前需经董事会审批。**