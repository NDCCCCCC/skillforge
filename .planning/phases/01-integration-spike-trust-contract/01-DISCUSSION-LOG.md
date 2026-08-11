# Phase 1 Discussion Log

**Phase:** 1 — Integration Spike & Trust Contract
**Date:** 2026-08-11
**Mode:** discuss (default, no flags)
**Areas discussed:** 4 (all selected by user)

## Participants

- User (SkillForge project owner) — visionary, executor
- Claude (discuss-phase) — thinking partner, captured decisions

## Areas Selected (4 of 4)

All 4 gray areas selected via multiSelect:
1. Mock vs Real API
2. SSE streaming verification depth
3. Milvus tier-filter verification scope
4. Spike output

## Area 1: Mock vs Real API

**Question:** Phase 1 的 API 验证策略？

**Options presented:**
- A. Mock server only (spike 001 模式) — 离线/可重复/快速，但只能验 wire format
- B. Real API only (GLM-5.2 via Coding Plan) — 能验真 behavior (reasoning/tool call/schema)，但 rate limit + 不可重复
- C. 双轨 (Mock wire + 真 API spot-check) — 平衡深度与成本
- D. 分层 (Mock 走 SSE/structured + 真 API 推 Milvus 检索) — 分工明确，跨 3 个验证点

**User selected:** **B (Real API only)** — but with implicit reuse of mock pattern from spike 001 (D-02)

**Decisions captured:**
- **D-01**: API validation = real API only (GLM-5.2 via Coding Plan endpoint)
- **D-02**: Mock OpenAI server pattern from spike 001 reused as a wire-format regression tool alongside real API (parallel, not exclusive)

**Rationale (user):** spike 001 already validated wire format with mock; Phase 1 must validate end-to-end behavior with real LLM to be a "trust contract". Mock reuse is for fast iteration / regression catching.

**Discretion items deferred to Claude:**
- Specific 5 query list (suggest: same compliance-real queries as spike 002/003 for comparability)
- Whether to use eino-ext/adk (multi-agent) or just eino (single ChatModel)

## Area 2: SSE Streaming Verification Depth

**Question:** SSE streaming 验证到哪一层？

**Options presented:**
- A. Wire format only (text/event-stream + Content-Type + data: frames)
- B. + Typed parts (reasoning / tool-calls / sources in frames)
- C. + UIMessageChunk 对齐 (Vercel AI SDK v5 client can parse)

**User selected:** **All 3 (wire + typed + UIMessageChunk)** — interpreted from "圈验" = 全选

**Decisions captured:**
- **D-03**: SSE verification = validate all 3 layers
- **D-04**: Typed parts to validate = `reasoning` / `tool-calls` / `sources`
- **D-05**: `WithExtraFields(map[string]any{...})` is the API for OpenAI extras (Eino v0.1.13+)

**Rationale (user):** Phase 4 (Streaming Trust UI) depends on this end-to-end alignment; a wire-only check would leave UI integration risky.

**Discretion items deferred to Claude:**
- Whether to write a minimal Vercel AI SDK v5 test client or use an existing tool (e.g., `curl` with `--no-buffer` for SSE inspection)

## Area 3: Milvus Tier-filter Verification Scope

**Question:** Milvus tier-filter 验证范围？

**Options presented:**
- A. Filter 表达式语法 only (tier == "权威")
- B. + 本地 Milvus 容器 + 4 tier 数据
- C. + BGE-M3 嵌入 + 真检索 E2E

**User selected:** **C (BGE-M3 + E2E)**

**Decisions captured:**
- **D-06**: Milvus scope = + BGE-M3 embedding + real retrieval E2E (heaviest, most production-like)
- **D-07**: Test data = 4 tiers (权威/历史/个人/草稿), each ≥3 documents; `tier in ["权威"]` must return zero 个人/草稿 chunks
- **D-08**: BGE-M3 = locked (C-MTEB leader, multilingual, 8K, self-hostable)
- **D-09**: Milvus = local Docker container (Milvus v2.5+ standalone)

**Rationale (user):** tier-filter is SkillForge's core differentiator; spike must prove the full pipeline, not just syntax.

**Discretion items deferred to Claude:**
- BGE-M3 self-host via Docker vs API endpoint (suggest: Docker for reproducibility)
- Docker compose vs docker run (suggest: compose for Phase-2 forward compatibility)

## Area 4: Spike Output

**Question:** Phase 1 spike 产出物？

**Options presented:**
- A. Runnable Go skeleton
- B. + Wire-contract doc
- C. + E2E trace log
- D. Contract doc only

**User selected:** **A + C (Runnable Go skeleton + E2E trace log)** — interpreted from "+ E2E trace log"

**Decisions captured:**
- **D-10**: Output = runnable Go skeleton + complete E2E trace log
- **D-11**: Skeleton must be runnable end-to-end on a fresh laptop with one command
- **D-12**: E2E trace log = 1 sample query walking through ALL components, each step timestamped

**Rationale (user):** Skeleton is Phase 2 starting point; trace log is acceptance evidence + future regression reference.

**Discretion items deferred to Claude:**
- Skeleton structure (suggest: `cmd/spike1/{wire,sse,milvus,e2e}.go` subcommands)
- Trace log format (suggest: structured JSON or human-readable log with `[ts] [component] [action] input → output`)

## Claude's Discretion Summary

- 5 query list = same as spike 002/003 (compliance-real)
- Eino usage = single ChatModel (no ADK in Phase 1)
- BGE-M3 = self-host Docker
- Milvus = docker compose
- Skeleton = subcommand-based (wire / sse / milvus / e2e)
- Trace log = structured with timestamp + component

## Folded Todos

- `spike-temperature-field-validation.md` (H1+H2+H4) — already completed via spike 001/002/002b/003; results in `spike-findings-ai-agent` skill. No re-execution in Phase 1.

## Next Up

`/gsd:plan-phase 1` — generate the Phase 1 spike plan based on these decisions.

---

*Discussion log: Phase 1-Integration-Spike-Trust-Contract / 2026-08-11*
