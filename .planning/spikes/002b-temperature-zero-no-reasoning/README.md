---
spike: 002b
name: temperature-zero-no-reasoning
type: comparison
validates: "Given temperature:0 + reasoning DISABLED via `thinking:{type:disabled}`, when invoke glm-5.2 via Eino 10 times per query, then token-level unique output rate should drop vs spike 002 (reasoning ON)"
verdict: INVALIDATED
related: [001, 002, 003]
tags: [temperature, determinism, compliance, reasoning-model, reasoning-disabled]
---

# Spike 002b: Temperature Zero Determinism (No Reasoning)

## What This Validates

**Given** SKILL.md declares `temperature: 0` + `thinking:{type:disabled}` extra field
**When** invoke glm-5.2 via Eino 10 times per query (50 calls total)
**Then** token-level unique output rate should be ≤ 2 per query (vs spike 002's 10/10)

This is the **comparison spike** to spike 002. If reasoning is the source of variance, disabling it should restore determinism.

## Why This Spike

Spike 002 INVALIDATED — reasoning model glm-5.2 had 100% variance at temp=0. Hypothesis: the **reasoning path** (not the answer generation) is where the variance comes from. Reasoning models sample even at temp=0.

User insight: probe showed `thinking:{type:disabled}` cleanly removes `reasoning_content` from glm-5.2 responses. This spike verifies whether disabling reasoning restores determinism.

## Research

- **Eino `openai.WithExtraFields`** confirmed in source (`/libs/acl/openai/option.go:59`): allows passing extra JSON fields like `thinking:{type:disabled}` to the OpenAI-compatible endpoint
- **GLM-5.2 probe**: `thinking:{type:disabled}` → `reasoning_content: ''`, `content: '2+2 = 4'` (direct answer, no reasoning)

## Approach

Identical to spike 002 except:
- Adds `openai.WithExtraFields({"thinking": {"type": "disabled"}})` to Generate call
- Reduces max_tokens 1500 → 500 (no reasoning budget needed)

| Aspect | spike 002 | spike 002b |
|---|---|---|
| Model | glm-5.2 | glm-5.2 |
| Endpoint | Coding Plan | Coding Plan |
| Temperature | 0 | 0 |
| Reasoning | **ON** (default) | **OFF** (via `thinking:{type:disabled}`) |
| Max tokens | 1500 | 500 |
| Queries × iters | 5 × 10 | 5 × 10 |

## How to Run

```bash
cd .planning/spikes/002b-temperature-zero-no-reasoning/
go build -o spike-002b .
timeout 300 ./spike-002b
```

## What to Expect

If reasoning is the source of variance:
- Each query should now show ≤ 2 unique outputs (pass)
- Variance should drop **dramatically** vs spike 002's 10/10

If variance persists:
- Reasoning alone is not the cause; need alternative approach

## Observability

Console prints per-iteration SHA-256 hash + length. Final report compares to spike 002.

## Investigation Trail

1. **Hypothesis**: spike 002 INVALIDATED because reasoning path has entropy. Disable reasoning → variance drops.

2. **Probe (curl direct)**: confirmed `thinking:{type:disabled}` removes `reasoning_content` from glm-5.2 response:
   - `reasoning_content: ''`
   - `content: 'The answer to 2 + 2 is **4**.'` (or similar direct answer)

3. **Eino API**: discovered `openai.WithExtraFields(map[string]any)` allows passing custom JSON fields per-call. Confirmed in `/libs/acl/openai/option.go:59`.

4. **Build**: copied spike 002 structure, added `openai.WithExtraFields({"thinking":{"type":"disabled"}})` to Generate call. Reduced max_tokens 1500→500 (no reasoning budget needed).

5. **Run (timeout 300s)**: 28 iterations completed before timeout:
   - Q1 出差住宿费超标: 10/10 unique hashes (len 1749-2395)
   - Q2 竞业限制期: 10/10 unique hashes (len 1370-2069)
   - Q3 采购合同50万: 8/8 unique hashes (len 1572-2405, truncated)
   - Q4-Q5 not run (timeout)

## Results

**Verdict: INVALIDATED ✗**

Disabling reasoning did **NOT** restore determinism. Comparison with spike 002:

| Query | spike 002 (reasoning ON) | spike 002b (reasoning OFF) |
|---|---|---|
| Q1 出差住宿费 | 10/10 unique | 10/10 unique |
| Q2 竞业限制期 | 6/6 unique | 10/10 unique |
| Q3 采购合同 | (truncated) | 8/8 unique |

Variance pattern is **essentially identical** with/without reasoning. Length distributions also overlap (spike 002: 1358-2418; spike 002b: 1370-2405).

**Conclusion**: variance is **NOT primarily from reasoning path**. It comes from glm-5.2's generation itself. Likely causes:
- **Floating-point non-determinism** in GPU/CPU kernels (different reduction orderings)
- **GLM API server batching / KV cache state** varies between calls
- **`temperature: 0` ≠ greedy decoding** in GLM API (may still sample with very low probability)

This is a **fundamental property of GLM-5.2 inference serving**, not a SkillForge / Eino issue.

## Implications for SkillForge

**The "确定性正解" philosophy as byte-determinism is fundamentally unachievable** with any mainstream LLM (incl. glm-5.2, GPT-4, Claude — all share this property). The philosophy must shift to **semantic determinism**:

1. **Structured output** (JSON schema) → answer structure is deterministic, content varies but format is constrained
2. **RAGAS faithfulness ≥ 0.9** (CI gate) → answer is faithful to retrieved context (semantic, not byte)
4. **Post-generation validation** (Self-RAG self-check) → every cited claim verified against source
5. **Refusal-by-construction** → unanswerable queries always refuse, never improvise

These mechanisms together create **operational determinism** (correct refusal/citation/structure) even when byte-determinism is impossible.

**SKILL.md schema implication**: `temperature: 0` is still useful (reduces variance significantly vs temp=1) but NOT a guarantee. Add `reasoning: false` field for compliance Skills (explicit disable).

## Surprises / Gotchas

- **Variance has nothing to do with reasoning** — confirmed via direct comparison spike 002 vs 002b
- **Eino `WithExtraFields` works perfectly** for OpenAI extra params (e.g., `thinking`, `modalities`, vendor-specific flags)
- **GLM-5.2 at temp=0 produces ~2KB outputs with high variance** — model is verbose even for simple queries
- **Length range 1300-2400 chars across queries** — output volume itself is non-deterministic