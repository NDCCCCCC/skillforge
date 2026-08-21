# eval/frozen — Frozen Baseline Locks

This directory holds the **frozen baselines** the acceptance gates
verify byte-identity against. The lock files are SACRED — do not edit
them in place; a drift in the lock file's sha256 breaks the
acceptance gate and forces an explicit rebaseline decision at the
milestone close.

## Files

| File | Phase | Purpose |
|------|-------|---------|
| `compliance.lock.json` | Phase 8 (SC-5) | Compliance golden set lock — model_ver + prompt_hash + seed for the 75-case `eval/golden/compliance.jsonl` |
| `hitl.lock.json` | Phase 11 (11-08) | HITL golden set lock — 75-case `eval/golden/hitl.jsonl` sha256 + per-category counts + acceptance-gate thresholds |
| `deep.lock.json` | **Phase 12 (12-08)** | **Deep agentic planning layer lock** — 50-case `eval/golden/deep.jsonl` + v0.8 milestone metrics (faithfulness + refusal_accuracy + budget_triggered_count + plan_todo_completeness) + RF-5 parse rate + P12-L10 verdict |
| `fanout.lock.json` | **Phase 13 (13-04 / 13-05)** | **Sub-agent fan-out (P8.4 — tier 继承) lock** — 20-case `eval/golden/fanout.jsonl` + per-scenario counts (8 multi-hop + 6 tier-mismatch + 4 partial-failure + 2 single-vertex) + gate thresholds (fanout_dispatched_count_floor + conservative_merge_rate_max + tier_closure_regression_rate_max) + sub-agent budget frozen (MaxIteration=5 + TokenBudget=20000) + shared accumulator cap (20) |

## Byte-Identity Verification

The acceptance gate (`scripts/verify-phase-12.sh` Stage 11) verifies
ALL THREE locks are present + readable:

```bash
sha256sum eval/frozen/compliance.lock.json eval/frozen/hitl.lock.json eval/frozen/deep.lock.json
```

Phase 13 adds a FOURTH lock file (`fanout.lock.json`). The Phase 13
acceptance gate (`scripts/verify-phase-13.sh` Stage 7) verifies BOTH
`compliance.lock.json` (Phase 8 pin — UNCHANGED; T-8-12) AND
`fanout.lock.json` (Phase 13 v0.8 milestone re-baseline). Use:

```bash
sha256sum eval/frozen/compliance.lock.json eval/frozen/fanout.lock.json
```

The expected sha256 values are recorded in `.planning/STATE.md` at
the phase closeout; a drift between the on-disk sha256 and the
recorded pin means either (a) the lock file was edited in place —
FORBIDDEN — or (b) the phase is being rebaselined (operator-driven,
requires a milestone-close waiver).

## compliance.lock.json (Phase 8)

Schema (frozen at Phase 8 closeout; do not extend):

```json
{
  "model_ver": "glm-5.2",
  "prompt_hash": "<sha256 of rendered system prompt>",
  "seed": 42,
  "eval_set_version": "compliance-2026-08-13-v1",
  "skill_name": "compliance"
}
```

The Phase 11 acceptance gate (`cmd/spike-history --frozen --archive`)
reads this lock and computes its sha256 at run-time; the value is
recorded into `reports/frozen_lock_<date>.sha256` as ship evidence.

## hitl.lock.json (Phase 11)

Schema (v1, Phase 11 introduction):

```json
{
  "schema_version": "v1",
  "created_at": "2026-08-20",
  "locked_by": "Phase 11 acceptance gate (11-08)",
  "golden_sha256": "<sha256 of eval/golden/hitl.jsonl bytes>",
  "model_ver": "hitl-frozen-v1",
  "prompt_hash": "<sha256 of testdata/skills/hitl-smoke/SKILL.md>",
  "case_count": 75,
  "category_counts": { "should-interrupt": 20, "...": "..." },
  "gate_thresholds": { "...": "..." }
}
```

The `golden_sha256` field is the byte-identity anchor for
`eval/golden/hitl.jsonl`. A change to the golden set requires
updating `golden_sha256` AND `created_at` + `locked_by` in the same
commit — a silent golden edit surfaces as a lock mismatch in the
acceptance gate.

## deep.lock.json (Phase 12)

Schema (v1, Phase 12 introduction; mirrors `compliance.lock.json` + adds deep-specific fields):

```json
{
  "version": "v0.8-deep-frozen-v2",
  "createdAt": "2026-08-20",
  "phase": "12-p8-3-planning-layer",
  "schema_version": "v1",
  "model_ver": "glm-5.2",
  "prompt_hash": "<sha256 of rendered DeepAgent system prompt — recorded at lock time>",
  "seed": 42,
  "eval_set_version": "deep-2026-08-20-v1",
  "skill_name": "deep",
  "agent_archetype": "deep",
  "max_iterations": 10,
  "budget_layer_frozen": true,
  "case_count": 50,
  "category_counts": {
    "multi-hop": 25,
    "plan-style": 15,
    "refusal-required": 10
  },
  "metrics": {
    "faithfulness": "<operator-measured at lock time>",
    "refusal_accuracy": "<operator-measured at lock time>",
    "budget_triggered_count": "<operator-measured at lock time>",
    "plan_todo_completeness": "<operator-measured at lock time>"
  },
  "rf5_parse_rate_without_structured_output": "<operator-measured at lock time — RF-5>",
  "rf5_parse_rate_with_structured_output": "<operator-measured at lock time — RF-5>",
  "rf5_p12l10_verdict": "<OK | MANDATORY | ESCALATE>",
  "lock_sha256": "<recorded at lock time — must match git-tracked file>"
}
```

### What is locked

The v0.8 milestone baseline including the **5 new behaviors** Phase 12
introduces (PLAN-01..05):

1. **Sub-agent dispatch via ApplyTrustStage** (PLAN-01) — verified
   by `TestFactoryDispatch` + `TestApplyTrustStageHoist`.
2. **write_todos audit + plan trace** (PLAN-02) — verified by
   `TestWriteTodosAuditedTool` + `TestPlanTracePartStreaming`.
3. **plan approval gate** (PLAN-03) — verified by
   `TestPlanApproveFirstPassInterrupts` + `TestPlanApprovalCardRendering`.
4. **3-layer budget gate** (PLAN-04) — verified by
   `TestBudgetMaxIterationCap` + `TestBudgetTokenCounterAccumulation` +
   `TestBudgetLoopDetection3InRow`.
5. **plan_todos replay + UI** (PLAN-05) — verified by
   `TestPlanTodosReplay` + `TestPlanReplayViewReconstruction`.

The metrics recorded at lock time (`metrics.faithfulness` +
`metrics.refusal_accuracy` + `metrics.budget_triggered_count` +
`metrics.plan_todo_completeness`) anchor the v0.8 milestone's
quality posture; subsequent phases (13+) measure against this baseline.

### When to lock

- **Phase 12 acceptance gate (12-08)** — initial lock with
  operator-measured values.
- **Subsequent behavior changes** — Phase 13 sub-agent fan-out,
  Phase 14 advanced verification, etc. each rebaseline.
- **v1.0 release** — final rebaseline at v1.0 milestone.

### Reproduce command (operator-runs-live)

```bash
GLM_API_KEY=$GLM_API_KEY \
  MILVUS_ADDRESS=$MILVUS_ADDRESS \
  BGE_M3_URL=$BGE_M3_URL \
  go run ./cmd/spike-deep parse-rate \
    --golden=eval/golden/deep.jsonl \
    --output=reports/frozen-eval-v2.jsonl \
    --cost-limit=10.00
```

After running, the operator:

1. Reads the JSONL summary row (last line).
2. Populates `metrics.*` + `rf5_*` fields in `deep.lock.json` from
   the summary + any side-channel measurements (faithfulness +
   refusal_accuracy + budget_triggered_count + plan_todo_completeness
   come from the per-case rows).
3. Computes `lock_sha256 = sha256sum deep.lock.json` BEFORE
   populating `lock_sha256` (the sha256 is of the file WITH the
   null `lock_sha256` placeholder — the placeholder is replaced,
   then the new sha256 is recorded).

   Alternative: set `lock_sha256 = "pending"` initially, populate
   the rest, then `sha256sum` → update `lock_sha256` → commit.
4. Re-runs `bash scripts/verify-phase-12.sh` — Stage 11 verifies
   the new sha256 matches the recorded value.

### Byte-identity check

```bash
git diff --quiet eval/frozen/deep.lock.json  # exit 0 = unchanged
sha256sum eval/frozen/deep.lock.json          # compare to STATE.md pin
```

A silent edit to the lock file (without re-running the spike)
surfaces as a sha256 drift → the acceptance gate exits 1.

## RF-5 Spike Protocol

### What is measured

GLM-5.2 + DeepAgent final message **verdict-JSON parse rate**:

- **Without structured output** (direct path): the model emits
  free-form text; `llm.ExtractJSON` strips code fences + trailing
  prose, then attempts JSON parse.
- **With structured output** (fallback): on direct-path miss,
  `llm.ExtractVerdictJSONWithStructuredOutput` propagates the
  recorded OpenAI-compatible `response_format=json_object` signal
  to the model; the chat model emits strict JSON; the fallback
  parse rate is the structured-output rescue rate.

### P12-L10 lock evaluation

The spike's `summary.p12l10_verdict` field encodes the lock:

- parse rate ≥ 90%: `OK (structured-output optional)` — recommend
  enabling for safety, but not blocking.
- parse rate 70-89%: `MANDATORY: structured-output final step
  required (P12-L10)` — the structured-output fallback MUST be
  enabled in production.
- parse rate < 70%: `ESCALATE: verdict-suffix fallback + forced_stop
  on parse failure` — escalate to user; verdict-suffix fallback
  + forced_stop on parse failure is the next-step mitigation.

### How to measure

```bash
go run ./cmd/spike-deep parse-rate \
  --golden=eval/golden/deep.jsonl \
  --output=reports/rf5-parse-rate.jsonl \
  --cost-limit=10.00
```

The JSONL output schema is locked by
`TestSpikeDeepParseRateOutputJSONLSchema` (12-08 Task 2):

```json
{
  "type": "case" | "summary",
  "id": "<case_id>",
  "category": "multi-hop" | "plan-style" | "refusal-required",
  "direct_parse": <bool>,
  "structured_rescue": <bool>,
  "parse_failed": <bool>,
  "model_ver": "glm-5.2",
  "prompt_hash": "<sha256>",
  "seed": 42,
  "max_iterations": 10,
  "model_calls": <int>,
  "cost_usd": <float64>,
  // summary-only:
  "total_cases": 50,
  "direct_ok": <int>,
  "structured_ok": <int>,
  "both_failed": <int>,
  "direct_rate": <float [0,1]>,
  "total_rate": <float [0,1]>,
  "p12l10_verdict": "<OK | MANDATORY | ESCALATE>"
}
```

### How to lock

The parse rate is recorded in `eval/frozen/deep.lock.json` fields:

- `rf5_parse_rate_without_structured_output` = `summary.direct_rate`
- `rf5_parse_rate_with_structured_output` = `summary.total_rate`
- `rf5_p12l10_verdict` = `summary.p12l10_verdict`

## Rebaseline Cadence

- **Phase 8 pin (`compliance.lock.json`)**: rebaselined only at v0.8
  milestone close. Phase 11–13 behavior changes must NOT touch this
  file.
- **Phase 11 pin (`hitl.lock.json`)**: rebaselined at Phase 11
  acceptance sign-off. Subsequent HITL changes (Phase 12 plan_approve,
  etc.) rebaseline at their own phase close.
- **Phase 12 pin (`deep.lock.json`)**: rebaselined at Phase 12
  acceptance sign-off (12-08) + any subsequent behavior change
  (Phase 13 sub-agent fan-out, Phase 14 advanced verification, etc.).
- **Phase 13 pin (`fanout.lock.json`)**: rebaselined at Phase 13
  acceptance sign-off (13-05 live UAT operator-gated). The Phase 13
  13-04 hermetic gate ships a placeholder with null `eval_set_sha256`
  + null metrics; 13-05 fills the actual values after operator runs
  the live stack and signs off. T-13-04-1 + T-P13 invariant: a drift
  between on-disk sha256 and the recorded pin surfaces as a hard
  FAIL in `scripts/verify-phase-13.sh` Stage 7.

## fanout.lock.json (Phase 13 v0.8 milestone re-baseline)

Locked by `scripts/verify-phase-13.sh` Stage 7 (hermetic sha256 check) +
`cmd/spike-fanout --live --golden=eval/golden/fanout.jsonl --summarize`
(Phase 13 / Plan 13-05 — operator-gated live UAT).

### Schema (v1, Phase 13 introduction; mirrors `deep.lock.json` pattern)

```json
{
  "lock_id": "fanout-2026-08-21",
  "model_version": "v0.8",
  "phase": 13,
  "eval_set": "eval/golden/fanout.jsonl",
  "eval_set_sha256": "<sha256 of fanout.jsonl bytes — recorded at 13-05 lock time>",
  "case_count": 20,
  "category_counts": {
    "multi_hop": 8,
    "tier_mismatch_rejection": 6,
    "partial_failure_conservative_merge": 4,
    "single_vertex_fallback": 2
  },
  "thresholds": {
    "fanout_dispatched_count_floor": 20,
    "conservative_merge_rate_max": 0.0,
    "tier_closure_regression_rate_max": 0.0,
    "tier_mismatch_rejection_rate_min": 1.0
  },
  "agent_archetype": "orchestrator",
  "sub_agent_budget_frozen": true,
  "sub_agent_max_iteration": 5,
  "sub_agent_token_budget": 20000,
  "shared_chunk_accumulator_max": 20,
  "phase_8_pin_unchanged": "5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded",
  "metrics": { "...": "<operator-measured at lock time>" },
  "operator_signed_at": "<2026-08-21>",
  "operator_name": "<operator name>",
  "live_fanout_md_sha256": "<sha256 of reports/live_fanout.md>",
  "lock_sha256": "<recorded at lock time — must match git-tracked file>"
}
```

### What is locked

The v0.8 milestone fan-out behavior delta from Phase 13:

1. **Tier-closure structural inheritance** (P13-L2) — verified by
   `TestSubAgentTierInheritance` + `TestSubAgentCannotExpandTierBoundary`.
2. **Deterministic 10× byte-identity merge** (P13-L4) — verified by
   `TestMergeSubAgentResultsByteIdentity`.
3. **Conservative partial-failure merge** (P13-L5) — verified by
   `TestMergeConservativePartialFailure`.
4. **Per-sub-agent grounding via ApplyTrustStage** (P13-L9) — verified
   by `TestFanOutOrchestratorAppliesTrustStagePerSubAgent` +
   `TestSharedChunkAccumulatorLRUCap`.
5. **D16 data-decision overload** (P13-L7) — verified by
   `TestDataDecisionFanoutEnum` (D15 lockstep) +
   `TestTransportSwapWithFanoutChunks` (D-04 byte-identity).

### **`compliance.lock.json` (Phase 8 pin) is UNCHANGED** — T-8-12
regression lock: re-generating the Phase 8 pin = tampering baseline.
Phase 13 ADDS `fanout.lock.json` as a NEW v0.8 milestone baseline; it
does NOT modify `compliance.lock.json`. The Phase 13 acceptance gate
Stage 7 verifies BOTH sha256 values — neither can drift without
operator detection.

### Byte-identity verification

```bash
sha256sum eval/frozen/compliance.lock.json eval/frozen/fanout.lock.json
# Compare to the values recorded in .planning/STATE.md after Phase 13
# acceptance (Phase 8 pin: 5cf700f3…; Phase 13 pin: <recorded at 13-05>).
```

A silent edit to either lock file (without re-running the spike and
operator sign-off) surfaces as a sha256 drift → the acceptance gate
exits 1.

### Reproduce command (operator-runs-live, Plan 13-05)

```bash
GLM_API_KEY=$GLM_API_KEY \
  MILVUS_ADDRESS=$MILVUS_ADDRESS \
  BGE_M3_URL=$BGE_M3_URL \
  go run ./cmd/spike-fanout --live \
    --golden=eval/golden/fanout.jsonl \
    --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md \
    --cost-limit=2.00 \
    --seed=42 \
    --summarize
```

After running, the operator populates `metrics.*` fields + actual
`eval_set_sha256` + `operator_signed_at` + `operator_name` +
`live_fanout_md_sha256` in `fanout.lock.json`, computes `lock_sha256`
(file with null `lock_sha256` placeholder first; replace then
re-verify), and commits the lock. The Phase 13 acceptance gate
Stage 7 then verifies the new sha256 against the recorded value.