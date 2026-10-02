---
description: 通用问答 / general Q&A. Triggers when the user asks a factual question that should be answered from the authority knowledge tier (e.g. "what does the policy say about X", "summarize the rule on Y"). Does NOT handle compliance verdicts with citation spans (use the `compliance` Skill for that), arithmetic / calculator queries, or any request involving personal information about a specific individual.
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Retrieve the most relevant chunks from the authority tier via the `reads.tier` filter
  2. Compose a concise answer grounded ONLY in the retrieved chunks; cite each claim with chunk_id
  3. If no authority chunks are retrieved, refuse with the `no_relevant_source` refusal phrase
guardrails:
  prohibitions:
    - "Never answer without at least one retrieved citation"
    - "Never fabricate sources or chunk ids"
    - "Never reveal personal/contact information of any individual"
  required_refusal_phrases:
    - "no_relevant_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.3
---

# Simple QA Skill

A minimal RAG Skill template that proves the 7-field schema works for a
generic question-answering use case — the same kernel serves a focused
compliance verdict (see `examples/skills/compliance/`) and a tool-only
Skill (see `examples/skills/calculator/`).

Reads only the authority tier (`authority/**`), writes nothing (read-only),
and refuses by default when retrieval returns zero chunks. The Skill
declares a moderate (non-zero) temperature so answers can paraphrase the
grounded facts slightly more naturally than the frozen-deterministic
compliance Skill, while still staying tight enough for factual Q&A.
