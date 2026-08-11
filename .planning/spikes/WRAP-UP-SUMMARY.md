# Spike Wrap-Up Summary

**Date:** 2026-08-11
**Spikes processed:** 4
**Feature areas:** 1 (temperature-field-validation + structured-output)
**Skill output:** `./.claude/skills/spike-findings-ai-agent/`

## Processed Spikes

| # | Name | Type | Verdict | Feature Area |
|---|------|------|---------|--------------|
| 001 | eino-temperature-passthrough | standard | ✓ VALIDATED | temperature-field-validation |
| 002 | temperature-zero-determinism | standard | ✗ INVALIDATED | temperature-field-validation |
| 002b | temperature-zero-no-reasoning | comparison | ✗ INVALIDATED | temperature-field-validation |
| 003 | structured-output-temperature-zero | standard | ✓ VALIDATED | temperature-field-validation |

## Key Findings

1. **Eino OpenAI adapter passes `temperature` correctly** to HTTP wire body (spike 001 ✓)
2. **Byte-determinism impossible with reasoning models** — temperature=0 + reasoning control still produces 100% variance (spike 002 + 002b ✗)
3. **Structured output 100% schema-valid** via `response_format:{type:json_object}` + Go struct validation (spike 003 ✓)
4. **Refusal-by-construction works** — glm-5.2 gracefully returns `status:"unanswered"` when context missing (spike 003 observed behavior)
5. **GLM Coding Plan endpoint discovery** — `https://open.bigmodel.cn/api/coding/paas/v4` (not standard `/api/paas/v4`). User-provided key insight.
6. **Eino `WithExtraFields`** — passes OpenAI extras (`response_format`, `thinking`) cleanly to request body

## Design Implications

### SKILL.md v1.1 update
- Add `reasoning: false` field (compliance Skills default disable reasoning path)
- Keep `temperature: 0` REQUIRED (reduces variance even if not eliminate)
- Structured output envelope is platform invariant
- Eval gate (RAGAS faithfulness ≥0.9) = operational determinism guarantee

### "确定性正解" philosophy shift
From byte-determinism → semantic-determinism:
- ✅ Structured output (schema structure) — spike 003
- ✅ Refusal-by-construction — spike 003 observed
- 🔜 RAGAS faithfulness — Phase 2 eval pipeline
- 🔜 Self-RAG self-check — Phase 2

## Operational Learnings

- All 4 spikes in Go for consistency with denova reference
- Stdlib-only test approach (no extra deps) keeps spikes minimal
- SHA-256 hashing of normalized output as deterministic identity works well for variance tracking
- Mock OpenAI server pattern (`net/http` stdlib) is sufficient for transmission tests
- `response_format:{type:json_object}` + Go struct validation gives 100% schema determinism
- GLM Coding Plan endpoint key scoping is non-obvious — must use `/api/coding/paas/v4` not `/api/paas/v4`

## Files Produced

- `.claude/skills/spike-findings-ai-agent/SKILL.md` — auto-loaded in future build conversations
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md` — implementation blueprint
- `.claude/skills/spike-findings-ai-agent/sources/` — original spike source files (preserved)
- `.planning/spikes/CONVENTIONS.md` — patterns for future spike sessions
- `.planning/spikes/WRAP-UP-SUMMARY.md` — this file

## Future Spike Recommendations

From MANIFEST analysis, frontier spikes worth considering:
1. **RAGAS faithfulness evaluation pipeline** (Phase 2 precursor) — validate that 50+ case golden set produces reliable faithfulness scores
2. **Milvus + BGE-M3 hybrid retrieval** (denova's knowledge tier mechanism) — validate true-source separation at the retrieval layer
3. **Hertz SSE ↔ Vercel AI SDK v5 wire framing** (Phase 1 Integration Spike precursor) — validate streaming + tool-call wire format end-to-end