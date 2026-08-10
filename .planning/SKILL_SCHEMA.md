# SKILL.md Schema (v1.0)

**Status:** Official specification
**Date:** 2026-08-10 (added `temperature` field — 7th field)
**Project:** SkillForge

The SkillForge `SKILL.md` schema extends Anthropic Skills baseline (2 fields) + denova's implicit patterns into a 7-field declarative spec. Each field is machine-parseable, lint-able, and runtime-validated.

## Fields

### 1. `description` (REQUIRED — Anthropic baseline)

- **Purpose:** Intent routing (when to trigger this Skill)
- **Format:** Plain text sentence
- **Rules:**
  - Must include trigger phrases (queries that should activate this Skill)
  - Must include negative scope (what's NOT this Skill)
  - Must be distinguishable from other Skills' descriptions
- **Example:**
  ```yaml
  description: 企业合规/制度审查。当用户问"这个报销合规吗"、"这个条款怎么解读"等合规问题时触发。不处理销售话术、营销文案等非合规问题。
  ```

### 2. `agent` (REQUIRED — SkillForge extension)

- **Purpose:** Bind this Skill to an Eino ADK agent role
- **Format:** Eino agent type name (`ChatModelAgent`, `ReActAgent`, `SequentialAgent`, `ParallelAgent`, `LoopAgent`, `ConditionalAgent`)
- **Rules:** Must be one of the agent types supported by Eino ADK
- **Example:** `agent: ChatModelAgent`

### 3. `reads` (REQUIRED — denova-inspired)

- **Purpose:** Knowledge boundary (what this Skill can retrieve)
- **Format:** Array of paths/tier patterns
- **Patterns:**
  - Tier patterns: `authority/**`, `historical/**`, `personal/**`, `draft/**`
  - Data channel patterns (financial): `data:aggregated_only`, `data:external:authorized`
- **Rules:**
  - Empty array → agent must refuse (default-deny)
  - `*` wildcard rejected at load time (anti-pattern, see P-SKILL-3)
- **Example:**
  ```yaml
  reads:
    - authority/policies/**
    - authority/regulations/**
  ```

### 4. `writes` (REQUIRED — denova-inspired)

- **Purpose:** Operation boundary (what this Skill can mutate)
- **Format:** Array of operation patterns
- **Rules:**
  - Compliance / data Skills SHOULD have `writes: []` (read-only)
  - Empty array → read-only
- **Example:** `writes: []`

### 5. `steps` (REQUIRED — denova-inspired, explicit)

- **Purpose:** Execution flow (how the Skill behaves)
- **Format:** Markdown ordered list
- **Rules:**
  - Declarative (intent), not imperative (control flow)
  - Reference data via `reads` patterns, not literal filenames
- **Example:**
  ```yaml
  steps:
    1. Read relevant policy chunks via `reads.tier` filter
    2. Compose draft verdict with citations (chunk_id + span)
    3. Self-verify each claim against retrieved text (Self-RAG)
    4. Refuse if any claim unsupported; otherwise output structured verdict
  ```

### 6. `guardrails` (REQUIRED — SkillForge-critical)

- **Purpose:** Refusal triggers + safety constraints
- **Format:** Structured object
- **Fields:**
  - `prohibitions`: array of forbidden actions
  - `required_refusal_phrases`: array of phrases that MUST appear in refusal messages
  - `tier_limits`: array of tier-specific limits
- **Rules:**
  - Must be non-empty
  - Must include ≥3 negative examples
- **Example:**
  ```yaml
  guardrails:
    prohibitions:
      - "Never cite clauses not retrieved"
      - "Never paraphrase critical clauses (quote verbatim)"
      - "Never modify authority tier content"
    required_refusal_phrases:
      - "out-of-scope"
      - "no_authorized_source"
    tier_limits:
      authority: ["read-only"]
  ```

### 7. `temperature` (REQUIRED — SkillForge extension, added 2026-08-10)

- **Purpose:** LLM generation determinism level for this Skill
- **Format:** Float `0.0`–`1.0`
- **Recommended values:**
  - `0.0` — full determinism (same input → same output; **mandatory for compliance / data / citation Skills**)
  - `0.3–0.5` — light variance (structure stable, phrasing varies; good for reports)
  - `0.7–1.0` — high creativity (multiple plausible outputs; for marketing / strategy / draft)
- **Rules:**
  - Must be specified (no default — every Skill explicitly declares its determinism level)
  - Skill runtime MUST pass it to LLM calls via Eino `GenerateOption(temperature)`
  - Compliance / data / citation Skills SHOULD use `0.0`
  - Creative Skills (marketing, strategy, draft) SHOULD use `0.7+`
- **Rationale:** SkillForge's "deterministic 正解" design philosophy requires per-Skill temperature control to support both compliance (deterministic) and creative (varied) use cases on the same platform.
- **Example:**
  ```yaml
  # 合规 Skill — 完全确定
  temperature: 0.0

  # 月报生成 Skill — 中间值
  temperature: 0.4

  # 营销文案 Skill — 高创意
  temperature: 0.8
  ```

## Validation

All 7 fields are REQUIRED. Schema validator (`SYSK-02`) rejects SKILL.md files with:

- Missing fields
- Invalid types (e.g., `temperature` not a float, `reads` not an array)
- Wildcard `*` in `reads` (anti-pattern, P-SKILL-3)
- Empty `guardrails.prohibitions` (<3 entries, P-SKILL-2)
- Missing `temperature` value (no implicit default)

## Example: Complete SKILL.md (compliance review)

```markdown
---
name: compliance-review
description: 企业合规/制度审查。当用户问"这个报销合规吗"、"这个条款怎么解读"等合规问题时触发。
agent: ChatModelAgent
reads:
  - authority/policies/**
  - authority/regulations/**
writes: []
temperature: 0.0
---

# Compliance Review Skill

guardrails:
  prohibitions:
    - Never cite clauses not retrieved
    - Never paraphrase critical clauses
    - Never modify authority tier content
  required_refusal_phrases:
    - "out-of-scope"
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]

steps:
  1. Read relevant policy chunks via tier filter
  2. Compose draft verdict with citations (chunk_id + span)
  3. Self-verify each claim (Self-RAG)
  4. Output structured verdict or structured refusal
```

## Versioning

- **v1.0** (2026-08-10): Initial 7-field schema (Anthropic `name`+`description` + 5 SkillForge extensions: `agent`, `reads`, `writes`, `steps`, `guardrails`, `temperature`)
- Future minor versions may add optional fields without breaking compat

## Compatibility

- **Anthropic Skills spec**: `description` field is a superset (Anthropic Skills with just `name`+`description` will be accepted; SkillForge will treat missing extensions as default-deny for `reads` / `writes` / require explicit `temperature`)
- **denova `SKILL.md`**: denova's implicit `steps` / reads / writes / guardrails are now **explicit + machine-parseable** in SkillForge; denova Skills can be migrated by extracting body text into fields
- **Pure declarative Markdown**: no code-compile Skills (kills hot-reload promise, see P-OSS anti-stack)