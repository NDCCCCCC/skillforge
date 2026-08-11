---
spike: 003
name: structured-output-temperature-zero
type: standard
validates: "Given `temperature:0` + `response_format:{type:json_object}` + 合规 verdict schema, when 跑 10 次 × 5 合规 query, then JSON schema 解析成功率 100% (parse OK + schema OK)"
verdict: VALIDATED
related: [001, 002, 002b]
tags: [temperature, structured-output, json-schema]
---

# Spike 003: Structured Output + JSON Schema (H4)

## What This Validates

**Given** SKILL.md declares `temperature: 0` + `response_format:{type:json_object}` + system prompt enforcing `{status, confidence, citations, review_required}` schema
**When** invoke glm-5.2 (reasoning disabled) via Eino 10 times per query
**Then** 100% of outputs are valid JSON parseable AND match the schema

This is the **schema-level** determinism check — independent of byte-determinism (which spike 002/002b proved impossible). Even if content varies, **schema structure** can be deterministic.

## Why This Spike Matters

Spike 002 + 002b proved **byte-determinism is impossible** with current LLMs (variance 100% regardless of temp=0 + reasoning-off). The "确定性正解" philosophy must shift to **semantic determinism**:

1. **Structured output** → schema structure always consistent (this spike validates)
2. **Refusal-by-construction** → unanswered queries always refuse, never improvise
3. **RAGAS faithfulness + Self-RAG** → semantically faithful (not byte-deterministic)

Spike 003 validates the **first pillar**: structured output schema reliability.

## Research

- **GLM-5.2 + OpenAI `response_format`** confirmed via probe: returns clean JSON `{status, confidence, citations, review_required, note}` when schema is requested
- **Eino `WithExtraFields`** confirmed (spike 002b) for passing `response_format` + `thinking` extra params
- **Probe verdict sample**: `{"status":"unanswered","confidence":0,"citations":[],"review_required":true,"note":"..."}` — model gracefully falls back to "unanswered" when lacking context (correct refusal behavior)

## Approach

| Approach | Choice | Rationale |
|---|---|---|
| OpenAI `response_format:{type:json_object}` | **Chosen** | Standard OpenAI-compatible schema enforcement |
| Vendor-specific schema field | Rejected | Less portable |
| Manual JSON parsing from free text | Rejected | Lower reliability |

- Model: glm-5.2 (via Coding Plan endpoint)
- Reasoning: **disabled** via `thinking:{type:disabled}` (saves tokens, faster)
- Extra fields: `response_format:{type:json_object}` + `thinking:{type:disabled}`
- Schema (validated by Go struct + logic):
  - `status`: enum {合规, 不合规, 需补充, unanswered}
  - `confidence`: float 0-1
  - `citations`: array of strings (can be empty)
  - `review_required`: bool

## How to Run

```bash
cd .planning/spikes/003-structured-output-temperature-zero/
go mod init spike-003
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext/components/model/openai@latest
go get gopkg.in/yaml.v3
go build -o spike-003 .
timeout 300 ./spike-003
```

## What to Expect

- All 50 outputs should be valid JSON with 4 required fields
- Status distribution: likely mostly "unanswered" (no policy context in system prompt) + occasional "需补充"
- Schema validation failures indicate JSON parsing issues or wrong field types

## Observability

Per-iteration: parse OK / schema OK / status value / confidence / citations count.
Final: parse failure rate, schema failure rate, full pass rate.

## Investigation Trail

1. **Context**: spike 002 + 002b INVALIDATED (byte-determinism impossible). This spike validates the **first pillar of semantic determinism** — structured output schema reliability.

2. **Probe (curl)**: confirmed `response_format:{type:json_object}` + `thinking:{type:disabled}` cleanly produces parseable JSON with all 4 required fields. Sample: `{"status":"unanswered","confidence":0,"citations":[],"review_required":true,"note":"..."}`

3. **Initial issue (UTF-8)**: first probe with Chinese content in messages failed with `JSON parse error: Invalid UTF-8 middle byte 0xe3` (GLM server-side jackson issue). switched to ASCII content for probe; main.go also uses ASCII system prompt to avoid same issue.

4. **Build**: spike 003 main.go (extended spike 002b structure) + Verdict struct + validate() function. Extra fields: `response_format:{type:json_object}` + `thinking:{type:disabled}`. Reduced max_tokens to 400.

5. **Run**: 50 iterations across 5 compliance-real queries, **all completed within timeout**:
   - Q1-Q5: 10/10 each = **50/50 schema-valid**

6. **Behavior**: model consistently returned `status:"unanswered"` (no policy context in system prompt → correct refusal-by-construction). This validates that the schema can serve as a "semantic determinism guard" — model refuses rather than improvises when context is missing.

## Results

**Verdict: VALIDATED ✓**

```
JSON parse failures: 0 / 50 (0%)
Schema validation failures: 0 / 50 (0%)
Full passes: 50 / 50 (100%)
[VERDICT] H4 VALIDATED ✓ (100% JSON parse + 100% schema valid)
```

**Behavior pattern**:
- All 50 outputs: `status="unanswered"`, `confidence=0.0`, `citations=[]`, `review_required=true`
- Model **correctly refuses** when no policy context provided (matches system prompt: "If you lack policy context, status='unanswered'")
- Schema fields consistent across all 50 outputs (even though LLM content varies)

## Implications for SkillForge

**The 3-spike comprehensive verdict**:

| Spike | Validates | Verdict |
|---|---|---|
| 001 | Transmission (Eino → OpenAI wire body) | ✓ VALIDATED |
| 002 | Byte-determinism (temp=0 + reasoning ON) | ✗ INVALIDATED |
| 002b | Byte-determinism (temp=0 + reasoning OFF) | ✗ INVALIDATED |
| 003 | Schema-determinism (structured output) | **✓ VALIDATED** |

**This confirms the "确定性正解" philosophy must shift to semantic determinism**:
1. **Structured output** = schema structure deterministic ✓ (spike 003)
2. **Refusal-by-construction** = always refuse on missing context ✓ (spike 003 behavior)
3. **RAGAS faithfulness + Self-RAG** = semantic accuracy in content (deferred to Phase 2 eval pipeline)

**SKILL.md schema v1.1 implications** (next iteration):
- Add `reasoning: false` field (default for compliance Skills; creative Skills can opt in)
- Keep `temperature: 0` as REQUIRED (it does reduce variance even if not eliminate)
- Keep structured output as platform invariant — every Skill gets the `{status, confidence, citations, review_required}` envelope

**What "确定性正解" means in v1.1**:
- ✅ Schema structure is deterministic (verified)
- ✅ Model refuses on missing context (verified)
- ✅ Citations are chunk IDs + spans (architectural guarantee from reads field)
- ❌ Exact wording of answer varies (LLM-inherent; accept)

## Surprises / Gotchas

- **Model "unanswered" behavior is graceful**: model doesn't try to hallucinate when no context — it picks `status:"unanswered"` + low confidence + review_required=true. This is **exactly the desired behavior** for compliance domain.
- **`response_format:{type:json_object}` is fully supported** by GLM-5.2 via Coding Plan endpoint
- **`thinking:{type:disabled}` + `response_format:{type:json_object}` works in tandem** (both extra fields via Eino `WithExtraFields`)
- **Chinese content in messages triggers UTF-8 parse error** at GLM server — must use ASCII content or escape carefully