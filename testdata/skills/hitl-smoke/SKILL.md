---
description: HITL smoke witness Skill for cmd/spike-hitl (Plan 11-08 Task 2). Exercises the memory:facts write path under the approval gate.
agent: ChatModelAgent
reads:
  - authority/**
writes:
  - memory:facts
steps: |
  1. Receive a fact-write request from the user
  2. Emit the memory_facts_write tool call with the fact payload
  3. Interrupt for operator approval (Pass 1)
  4. On approve, execute the write; on deny, surface the operator's reason
guardrails:
  prohibitions:
    - "Never execute memory:facts writes without operator approval"
    - "Never mutate frozen_args between approval and execution"
  required_refusal_phrases:
    - "operator_denied"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
---

# HITL Smoke Witness Skill

This Skill is the **witness** for the Plan 11-08 acceptance gate's
single-binary E2E smoke (`cmd/spike-hitl`). It declares exactly one
write target (`memory:facts`) so the agent's per-turn Tool whitelist
contains exactly one HITL Tool.

The binary `cmd/spike-hitl/main.go` constructs an in-memory manifest
equivalent to this frontmatter (it does NOT parse the file at runtime
— hermetic requirement); the file exists so reviewers can see the
manifest the E2E smoke exercises in a readable form.

## Why a witness Skill (not the production compliance Skill)

- The production compliance Skill carries `writes: []` and cannot
  trigger the HITL flow.
- The acceptance gate must be hermetic — a scripted ChatModel stub
  emits the memory_facts_write tool call; no live LLM is involved.
- A separate witness keeps the production Skills unchanged while
  proving the HITL contract end-to-end under one binary.
