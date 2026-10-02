---
description: 算术 / 计算 (arithmetic / calculation). Triggers when the user asks a pure numeric question such as "2+2", "monthly interest on 10000 at 5%", or "convert 17/25 to a percentage". Does NOT handle questions that require citing retrieved documents — calculator is a tool-only Skill and never reads from any knowledge tier.
agent: ReActAgent
reads:
  # PLACEHOLDER reads — draft-role KB (D7-10 + KBM-03 draft-role semantics).
  # `internal/skill.Validate` rejects empty `reads` (P-SKILL-3 default-deny)
  # so a pure tool-type Skill cannot legitimately declare `reads: []`. A
  # draft-role knowledge base is the semantically-correct placeholder: the
  # registry pins the draft role as "never enters retrieval" (KBM-03), and
  # `kb.ResolveReads` DROPS draft-role bases from the resolved set — the
  # declaration satisfies the non-empty contract while granting zero
  # retrievable and zero citable source. The `role:` wildcard syntax is
  # fail-closed here — `role: draft` expands to zero retrieval-eligible
  # KBs and is rejected — so the explicit `draft/**` slug spelling is the
  # required placeholder form. The validator is intentionally UNCHANGED —
  # the platform does not yet execute pure-tool Skills; this template
  # ships as a validate-only demonstration of Skill-format diversity
  # (DIST-08).
  - draft/**
writes:
  - skill_state
steps: |
  1. Parse the numeric expression out of the user query
  2. Invoke the calculator tool with the parsed expression
  3. Return the computed result; persist intermediates to `skill_state` for follow-up turns
guardrails:
  prohibitions:
    - "Never cite retrieved chunks — calculator reads no knowledge base (the draft-role KB placeholder never enters retrieval, KBM-03)"
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

The `reads: [draft/**]` entry is a PLACEHOLDER required by
`internal/skill.Validate` (P-SKILL-3 default-deny invariant: empty reads
is rejected). `draft` is a draft-role knowledge base: the registry pins
the role as "never enters retrieval" (KBM-03) and `kb.ResolveReads` drops
draft-role bases from the resolved set, so the declaration satisfies the
non-empty contract while granting zero retrievable — and zero citable —
source. (The `role:` wildcard is fail-closed for the draft role — it
expands to zero retrieval-eligible KBs and is rejected — so the explicit
`draft/**` slug spelling is the required form.) This keeps the validator
unchanged — no new capability is shipped under DIST-08.
