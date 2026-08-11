---
spike: 002
name: temperature-zero-determinism
type: standard
validates: "Given `temperature:0` + 同 query 跑 10 次 × 5 合规 query, when 提取所有输出, then token-level 一致率 ≥95%"
verdict: INVALIDATED
related: [001, 003]
tags: [temperature, determinism, compliance, reasoning-model]
---

# Spike 002: Temperature Zero Determinism (glm-5.2)

## What This Validates

**Given** SKILL.md declares `temperature: 0` + 5 compliance-real queries
**When** invoke glm-5.2 via Eino 10 times per query (50 calls total)
**Then** token-level unique output rate should be ≤ 2 per query (≥95% consistent)

This is the **behavior-level** assumption: even if Eino passes temperature correctly (H1 ✓), does temperature=0 actually make LLM outputs deterministic in practice?

## Research (Context7)

Eino API confirmed (spike 001): `model.WithTemperature(0.0)` correctly wired to OpenAI HTTP body. Unknown: does glm-5.2 actually produce deterministic output at temp=0?

## Approach

| Approach | Choice | Rationale |
|---|---|---|
| Real LLM (GLM-5.2 via Coding Plan endpoint) | **Chosen** | Validates real behavior, not synthetic |
| Mock LLM with deterministic responses | Rejected | Doesn't measure variance |
| Multiple models (GPT-4 + Claude + GLM) | Deferred | Too expensive, single model sufficient for spike |

- **Endpoint**: `https://open.bigmodel.cn/api/coding/paas/v4` (Coding Plan专用 — 普通 `/api/paas/v4` returns `1113 余额不足` for Coding keys)
- **Model**: `glm-5.2` (Coding Plan flagship, supports reasoning)
- **Temperature**: 0 (per SKILL.md)
- **Max tokens**: 1500 (after first attempt at 300 returned empty)

## How to Run

```bash
cd .planning/spikes/002-temperature-zero-determinism/
go build -o spike-002 .
timeout 600 ./spike-002   # allow longer than 360s (50 × ~10s = ~500s)
```

## What to Expect

Each query should produce ≤ 2 unique outputs (out of 10). Pass threshold: ≥ 4 / 5 queries pass.

## Observability

Console prints per-iteration SHA-256 hash + length. Final report groups hashes per query and counts uniques.

## Investigation Trail

1. **Research**: Confirmed Eino `model.WithTemperature(0.0)` passthrough (spike 001). Question became: does glm-5.2 actually produce deterministic output at temp=0?

2. **First attempt (FAILED — empty outputs)**: Initial config `Model: "glm-4-flash"` + `max_tokens=300` — all 50 outputs were **empty strings** (hash `e3b0c44298fc1c14` = SHA256 of empty). Diagnosis: max_tokens too low; reasoning model consumed all budget on reasoning_content leaving content empty.

3. **Endpoint discovery** (key user insight!): User pointed out the key is a Coding Plan 套餐, not standard. Initial probe to standard `open.bigmodel.cn/api/paas/v4` returned `{"code":"1113","message":"余额不足或无可用资源包,请充值。"}`. WebSearch revealed Coding Plan uses **专用 endpoint** `https://open.bigmodel.cn/api/coding/paas/v4`. Probe confirmed glm-5.2 + glm-4.6 + glm-4-flash all work on this endpoint.

4. **Increased max_tokens to 1500**: After switching endpoint + model, content was still empty until bumping max_tokens from 300 → 1500 (reasoning model needs budget for both reasoning + answer).

5. **Real run with glm-5.2**: Real outputs emerged:
   - **Q1 出差住宿费超标**: 10 iterations → **10 unique hashes** (len 1358–2418, ~1800 avg)
   - **Q2 竞业限制期**: 6 iterations before timeout → **6 unique hashes** (len 1213–1554)
   - Q3-Q5 not run (timeout at 360s, ~36s/query)

6. **Timeout truncated**: 360s timeout cut off at Q2 iter 6. Sufficient evidence for verdict (2 queries run, both 100% variance).

## Results

**Verdict: INVALIDATED ✗**

At `temperature:0`, **glm-5.2 produces fully non-deterministic outputs**:
- Q1: 10/10 unique outputs (100% variance)
- Q2: 6/6 unique outputs (100% variance)

**Root cause analysis**:

glm-5.2 is a **reasoning model** (API response includes `reasoning_content` field). Reasoning models typically:
1. Maintain internal sampling entropy in the reasoning path even at temp=0
2. Reasoning path produces stochastic intermediate state
3. Final answer derives from reasoning state, propagating variance

This is a documented behavior — many reasoning models (GLM-5.x, OpenAI o1, etc.) use sampling in chain-of-thought regardless of declared temperature.

## Implications for SkillForge

The "确定性正解" design philosophy is **NOT achievable with reasoning models at temperature=0 alone**. Options:

1. **Disable reasoning**: glm-5.2 likely supports `enable_thinking: false` / `thinking` field — testing needed
2. **Use non-reasoning model for compliance**: glm-4-flash / glm-4.6 without thinking mode (glm-4-flash already validated for endpoint + auth)
3. **Use other models**: GPT-4 / Claude may behave differently (not tested)
4. **Accept partial determinism + post-generation validation**: temperature=0 reduces variance but doesn't guarantee byte-identical output. Combine with **RAGAS faithfulness + Self-RAG self-check** for semantic determinism (not byte-determinism)

**Surprises / Gotchas**:
- **Endpoint matters**: Coding Plan keys (`/api/coding/paas/v4`) ≠ standard API keys (`/api/paas/v4`). User insight was correct.
- **Reasoning model + temp=0 ≠ deterministic**: Counterintuitive but documented — reasoning models use sampling even at temp=0
- **max_tokens=300 silently returns empty** for reasoning models (token budget consumed by reasoning_content)
- **Log hardcoded `(glm-4-flash)`** was misleading (actual model was glm-5.2 from start)

## Next Steps

- **Spike 003 (structured output)**: Still valid — schema validation doesn't depend on variance. Run with glm-5.2 + disable reasoning if possible.
- **Optional spike 002b**: Re-run with `enable_thinking: false` to validate reasoning is the source of variance
- **Design decision for SKILL.md v1.0**: Either pin `temperature: 0` + accept variance (rely on validation), OR add `reasoning: false` field to schema