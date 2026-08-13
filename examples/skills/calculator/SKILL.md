---
description: 算术 / 计算 (arithmetic / calculation). Triggers when the user asks a pure numeric question such as "2+2", "monthly interest on 10000 at 5%", or "convert 17/25 to a percentage". Does NOT handle questions that require citing retrieved documents — calculator is a tool-only Skill and never reads from any knowledge tier.
agent: ReActAgent
reads:
  # PLACEHOLDER tier (D7-10 + 07-RESEARCH Open Question Q2 resolution).
  # `internal/skill.Validate` rejects empty `reads` (P-SKILL-3 default-deny)
  # so a pure tool-type Skill cannot legitimately declare `reads: []`. The
  # `draft/**` tier is the most semantically-appropriate placeholder: it is
  # the scratch / working tier, signalling "no authoritative knowledge
  # source". The validator is intentionally UNCHANGED — the platform does
  # not yet execute pure-tool Skills; this template ships as a
  # validate-only demonstration of Skill-format diversity (DIST-08).
  - draft/**
writes:
  - skill_state
steps: |
  1. Parse the numeric expression out of the user query
  2. Invoke the calculator tool with the parsed expression
  3. Return the computed result; persist intermediates to `skill_state` for follow-up turns
guardrails:
  prohibitions:
    - "Never cite retrieved chunks — calculator reads no knowledge tier (placeholder draft tier is not a real source)"
    - "Never approximate — use exact arithmetic and only round when the user asks"
    - "Never persist raw user input beyond the current turn"
  required_refusal_phrases:
    - "non_arithmetic_query"
  tier_limits: {}
temperature: 0.0
---

# Calculator Skill (tool-type, validate-only per D7-10)

A tool-type Skill template that demonstrates the Skill schema's range: the
same 7-field declarative format serves RAG Skills (`compliance`,
`simple-qa`) AND pure-tool Skills (`calculator`).

**calculator is a tool-type Skill; it validates under the 7-field schema
but the runtime does not yet execute pure-tool Skills (D7-10).** The
SkillForge runtime is currently a RAG pipeline; tool-only execution is a
future capability. This template ships so contributors can see the
stateful tool-Skill pattern (a `writes: [skill_state]` declaration paired
with a frozen-deterministic temperature) and so the schema validator's
coverage is self-documenting.

The `reads: [draft/**]` entry is a PLACEHOLDER tier required by
`internal/skill.Validate` (P-SKILL-3 default-deny invariant: empty reads
is rejected). `draft` was chosen because it is the scratch / working tier
and most accurately signals "no authoritative source". This keeps the
validator unchanged — no new capability is shipped under DIST-08.
