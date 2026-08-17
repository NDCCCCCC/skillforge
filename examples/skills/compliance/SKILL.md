---
description: 企业合规/制度审查。当用户问"这个报销合规吗"、"这个条款怎么解读"、"出差住宿费超标"、"竞业限制期"、"年假折算"、"合同审批权限"等企业内部合规问题时触发。事实型查询（"X 由谁审批"、"X 有几天"、"X 多久"）也通过此 Skill 处理 —— 答案放在 `answer` 字段。不处理外部法律法规咨询、销售话术、营销文案或个人隐私信息查询。
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Classify the query type FIRST (STEP 0 in the system prompt): factual lookup vs compliance review.
     - Factual (「X 由谁审批」「X 有几天」「X 多久」「X 标准是什么」): populate `answer` with the
       direct answer extracted from the cited chunk; set `compliance_status` to "not_applicable";
       platform `status` to "ok".
     - Compliance review (「X 合规吗」「是否违反」「是否符合制度」): leave `answer` empty; set
       `compliance_status` to "compliant" / "non_compliant" / "not_applicable"; platform `status`
       is "ok" / "needs_input" accordingly.
  2. Read relevant policy chunks via `reads.tier` filter (tier=权威 only)
  3. Compose draft verdict with citations (chunk_id + verbatim span)
  4. Self-verify each claim against retrieved text (Go programmatic grounding check — D2-01)
  5. Refuse if no authority chunks retrieved (no_authorized_source); otherwise output structured verdict
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal/contact information of any individual"
    - "Never modify authority tier content"
    - "Never populate `answer` with a paraphrase of the citation span — the answer must be a SHORT factual sentence, not a requote"
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

所有引用必须为 chunk_id + 逐字 span，由 Go 程序化校验（grounding check, D2-01）。任何引用若在原文检索不到，将触发平台 `status="needs_input"` 降级并标记 `review_required=true`（D2-02）。

## Wire schema (260817-vsm)

`verdict.Verdict` 是平台信封，本 Skill 通过两个可选字段扩展：

- `answer` (string, optional)：事实型查询的直接答案（如「由部门负责人审批」）。
  仅在 STEP 0 判为事实型查询时填充；合规审查查询必须留空。
- `compliance_status` (string, optional)：合规审查裁决。
  - `"compliant"` — 主体符合所引条款
  - `"non_compliant"` — 主体违反所引条款（会触发 SC-7 redline 改写建议）
  - `"not_applicable"` — 该查询是事实型 lookup（与 `answer` 配对；UI 隐藏 verdict badge）

平台 `status` 字段（`ok` / `needs_input` / `refused` / `error`）由系统统一维护，Skill 不直接控制（除 `compliance_status="non_compliant"` 或 grounding 降级 → 平台会同步 `status="needs_input"`）。