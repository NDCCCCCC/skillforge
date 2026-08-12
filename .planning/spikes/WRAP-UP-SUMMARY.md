# Spike Wrap-Up Summary

**Date:** 2026-08-11 (001–003), 2026-08-12 (004)
**Spikes processed:** 5
**Feature areas:** 2 (temperature-field-validation + structured-output; milvus tier-contract retrieval)
**Skill output:** `./.claude/skills/spike-findings-ai-agent/`

## Processed Spikes

| # | Name | Type | Verdict | Feature Area |
|---|------|------|---------|--------------|
| 001 | eino-temperature-passthrough | standard | ✓ VALIDATED | temperature-field-validation |
| 002 | temperature-zero-determinism | standard | ✗ INVALIDATED | temperature-field-validation |
| 002b | temperature-zero-no-reasoning | comparison | ✗ INVALIDATED | temperature-field-validation |
| 003 | structured-output-temperature-zero | standard | ✓ VALIDATED | temperature-field-validation |
| 004 | milvus-v3-milvusclient-tier-contract | standard | ✓ VALIDATED | milvus tier-contract retrieval |

## Key Findings

### Temperature + structured output (001–003)
1. **Eino OpenAI adapter passes `temperature` correctly** to HTTP wire body (spike 001 ✓)
2. **Byte-determinism impossible with reasoning models** — temperature=0 + reasoning control still produces 100% variance (spike 002 + 002b ✗)
3. **Structured output 100% schema-valid** via `response_format:{type:json_object}` + Go struct validation (spike 003 ✓)
4. **Refusal-by-construction works** — glm-5.2 gracefully returns `status:"unanswered"` when context missing (spike 003 observed behavior)
5. **GLM Coding Plan endpoint discovery** — `https://open.bigmodel.cn/api/coding/paas/v4` (not standard `/api/paas/v4`). User-provided key insight.
6. **Eino `WithExtraFields`** — passes OpenAI extras (`response_format`, `thinking`) cleanly to request body

### Milvus tier-contract retrieval (004)
7. **Tier PRE-filter survives the SDK migration** — `tier in ["权威"]` is applied server-side at index-scan time; 2 权威 returned, **0 个人 leaked** (a post-filter would leak scores — it does not). The trust moat holds under `milvus-sdk-go/v2` → `client/v2` (milvusclient) on Milvus 3.0.
8. **D-09 refusal-by-construction preserved** — empty tiers → literal `"false"` → 0 hits.
9. **`BuildMilvusFilter` ports UNCHANGED** — a filter is a server-side expression string; `WithFilter(string)` accepts any Milvus expression regardless of SDK version. `filter.go` stays; only config.go/client.go/search.go/seed.go are rewritten.
10. **VarChar column gotcha** — `column.NewColumnVarChar` (FieldType VarChar, enum 21), NOT `NewColumnString` (DataType String, enum 20, marked `/* NOT USED */`). Pair a `FieldTypeVarChar` schema field with `NewColumnVarChar`; otherwise Insert fails with a misleading "type string but is string" error.

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

### Retriever migration (unlocked by spike 004)
- Production `internal/retriever/milvus/` can migrate `config.go / client.go / search.go / seed.go` from `milvus-sdk-go/v2` (positional + entity-column) to `client/v2` (milvusclient fluent-option); `filter.go` is SDK-independent and stays.
- The PRE-filter + D-09 contract is the structural trust boundary — encode zero-leak + 0-hit-on-empty as regression gates.

## Operational Learnings

- All spikes in Go for consistency with denova reference
- Stdlib-only test approach (no extra deps) keeps temperature spikes minimal
- SHA-256 hashing of normalized output as deterministic identity works well for variance tracking
- Mock OpenAI server pattern (`net/http` stdlib) is sufficient for transmission tests
- `response_format:{type:json_object}` + Go struct validation gives 100% schema determinism
- GLM Coding Plan endpoint key scoping is non-obvious — must use `/api/coding/paas/v4` not `/api/paas/v4`
- Milvus spike needs a live Milvus 3.0 instance; deployment blockers (CPU x86-64-v2, SELinux, volume uid) are environment-only — keep them separate from contract logic

## Files Produced

- `.claude/skills/spike-findings-ai-agent/SKILL.md` — auto-loaded in future build conversations (covers temperature + Milvus)
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md` — temperature/structured-output blueprint
- `.claude/skills/spike-findings-ai-agent/references/milvus-tier-contract.md` — Milvus SDK-migration + tier-contract blueprint
- `.claude/skills/spike-findings-ai-agent/sources/` — original spike source files (main.go + skill.md per spike)
- `.planning/spikes/CONVENTIONS.md` — patterns for future spike sessions
- `.planning/spikes/WRAP-UP-SUMMARY.md` — this file

## Future Spike Recommendations

From MANIFEST analysis, frontier spikes worth considering:
1. **RAGAS faithfulness evaluation pipeline** (Phase 2 precursor) — validate that 50+ case golden set produces reliable faithfulness scores
2. **BGE-M3 + hybrid retrieval** — validate dense+sparse + reranker on top of the 004 tier-contract path (now that the moat is proven on the new SDK)
3. **Hertz SSE ↔ Vercel AI SDK v5 wire framing** (Phase 1 Integration Spike precursor) — validate streaming + tool-call wire format end-to-end
