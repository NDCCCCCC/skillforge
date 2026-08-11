---
description: 企业合规/制度审查。当用户问"这个报销合规吗"、"这个条款怎么解读"、"出差住宿费超标"、"竞业限制期"、"年假折算"、"合同审批权限"等企业内部合规问题时触发。不处理外部法律法规咨询、销售话术、营销文案或个人隐私信息查询。
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Read relevant policy chunks via `reads.tier` filter (tier=权威 only)
  2. Compose draft verdict with citations (chunk_id + verbatim span)
  3. Self-verify each claim against retrieved text (Go programmatic grounding check — D2-01)
  4. Refuse if no authority chunks retrieved (no_authorized_source); otherwise output structured verdict
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal/contact information of any individual"
    - "Never modify authority tier content"
  required_refusal_phrases:
    - "no_authorized_source"
    - "out_of_scope"
    - "low_retrieval_score"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
---

# Compliance Review Skill

企业合规/制度审查。仅依据权威 tier（`authority/**`）回答现行有效制度问题。

所有引用必须为 chunk_id + 逐字 span，由 Go 程序化校验（grounding check, D2-01）。任何引用若在原文检索不到，将触发 `需补充` 降级并标记 `review_required=true`（D2-02）。