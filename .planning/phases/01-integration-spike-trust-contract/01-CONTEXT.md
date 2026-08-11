# Phase 1: Integration Spike & Trust Contract - Context

**Gathered:** 2026-08-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 1 is a **1-2 day validation spike** — verify that Eino v0.7.7 + Hertz SSE + Vercel AI SDK v5 + Milvus tier-filter wire framing works end-to-end on the production stack (GLM-5.2 via Coding Plan endpoint + Milvus + BGE-M3). Output: a runnable Go skeleton + complete E2E trace log, serving as the "trust contract" that Phase 2-7 can build on.

**This is validation, not product feature construction.** No new capabilities — only confirmation that the foundation works.
</domain>

<decisions>
## Implementation Decisions

### Mock vs Real API

- **D-01**: API validation = **real API only (GLM-5.2 via Coding Plan endpoint `https://open.bigmodel.cn/api/coding/paas/v4`)**. Rationale: spike 001 already used a mock OpenAI server to validate wire format. Phase 1 must validate **end-to-end behavior** with a real LLM to be the "trust contract". User-provided GLM Coding Plan key covers cost (glm-4-flash free tier + glm-5.2 paid tier).
- **D-02**: Mock OpenAI server pattern from spike 001 is **reused as a wire-format validation tool alongside real API** (parallel, not exclusive) — to catch wire regressions quickly without burning API quota.

### SSE Streaming Verification Depth

- **D-03**: SSE verification = **validate all 3 layers** — (a) wire format (text/event-stream + Content-Type + data: frames), (b) typed parts (reasoning / tool-calls / sources can be encoded in frames), (c) UIMessageChunk alignment (Vercel AI SDK v5 client can parse). Rationale: Phase 4 (Streaming Trust UI) depends on this end-to-end alignment; a wire-only check would leave the UI integration risky.
- **D-04**: Typed parts to validate = `reasoning` (when `thinking:{type:disabled}` is OFF, vs ON), `tool-calls` (Eino tool-calling via `WithTools`), `sources` (citation chips in chat). All three must serialize correctly through Hertz SSE.
- **D-05**: `WithExtraFields(map[string]any{...})` (Eino v0.1.13+) is the API for passing OpenAI extras (`response_format`, `thinking`, etc.) — already proven in spike 002b + 003.

### Milvus Tier-filter Verification Scope

- **D-06**: Milvus scope = **+ BGE-M3 embedding + real retrieval E2E** (heaviest, most production-like). Rationale: tier-filter is SkillForge's core differentiator (no other vector DB has this). The spike must prove the full tier pipeline works, not just filter syntax.
- **D-07**: Test data = 4 tiers (权威 / 历史 / 个人 / 草稿), each tier with ≥3 documents. Query with `tier in ["权威"]` filter must return **zero** personal/草稿 chunks (the bleeding-prevention guarantee).
- **D-08**: BGE-M3 embedding model is the locked choice (C-MTEB leader, multilingual, 8K context, self-hostable via Docker). Self-host via Docker for spike reproducibility.
- **D-09**: Milvus instance = local Docker container (Milvus v2.5+ standalone) for spike — no remote/cloud dependency. Validates full ops path (start container → connect → insert → query → tear down).

### Spike Output

- **D-10**: Output = **Runnable Go skeleton + complete E2E trace log**. Rationale: skeleton is the starting point for Phase 2 (Eino + Hertz integration); trace log is the acceptance evidence + future regression reference.
- **D-11**: Skeleton must be runnable end-to-end on a fresh laptop with one command (`docker compose up` + `go run .`), not stubbed or partial. Phase 2-7 builds on this as the foundation.
- **D-12**: E2E trace log = 1 sample query that walks through ALL components (skill parse → Eino → Coding Plan → GLM-5.2 → SSE → mock React client → Milvus → BGE-M3 → tier filter → final response). Each step timestamped with input/output/decision.

### Claude's Discretion

- Specific 5 query list for the spike (suggest: same compliance-real queries as spike 002/003 for direct comparability)
- Whether to use `eino-ext/adk` (multi-agent) or just `eino` (single ChatModel) for the spike (suggest: single ChatModel for spike; ADK added in Phase 3)
- React frontend in the spike — minimal stub vs full Phase-4-style client (suggest: minimal stub that can parse SSE; real React client built in Phase 4)
- Container orchestration — `docker compose` vs `docker run` (suggest: `docker compose` for Phase-2 forward compatibility)

### Folded Todos

- **`spike-temperature-field-validation.md`** (Phase 1 scope match, score 0.6) — H1+H2+H4 spike validation. **Status: ALREADY DONE**. spike 001 (Eino passthrough ✓), spike 002 (variance ✗ → semantic-determinism), spike 002b (variance ✗ → reasoning NOT source), spike 003 (structured output ✓ 100% schema). Results packaged in `spike-findings-ai-agent` skill + 4 spike directories under `.planning/spikes/`. **No re-execution needed in Phase 1.** Phase 1 spike focuses on the **integration** (Eino + Hertz + AI SDK + Milvus + BGE-M3 together), not re-validating temperature field.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level (from PROJECT.md, REQUIREMENTS.md, STATE.md)

- `.planning/PROJECT.md` — Core Value (Skill 可插拔平台) + 10 Decisions (D1-D10) + audience (OSS personal brand)
- `.planning/REQUIREMENTS.md` — 70 v1 requirements (55 base + 15 added via /gsd:explore: 8 DATA + 1 SYSK-08 temperature + 6 DATA-09..14 NL2SQL security)
- `.planning/STATE.md` — Decision log + open questions
- `.planning/ROADMAP.md` — Phase 1 reference + 7-phase structure

### Research (validated choices)

- `.planning/research/STACK.md` — Locked stack: Go + Eino v0.7.7+ + Eino ADK + Hertz SSE + OpenAI compat + Milvus + BGE-M3
- `.planning/research/ARCHITECTURE.md` — 6-layer system; Eino ADK mapping (ChatModelAgent / SequentialAgent / ParallelAgent)
- `.planning/research/SUMMARY.md` — Consolidated research findings; 7-phase order
- `.planning/research/FEATURES.md` — Table stakes / differentiators / anti-features

### Schema (locked contract)

- `.planning/SKILL_SCHEMA.md` — SKILL.md v1.0 7-field spec (description / agent / reads / writes / steps / guardrails / **temperature**). v1.1 adds `reasoning: false` field (compliance Skills default disable reasoning path)

### Spike findings (validated patterns — auto-loads)

- `.claude/skills/spike-findings-ai-agent/SKILL.md` — Implementation blueprint index
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md` — Coding Plan endpoint + WithExtraFields + structured output patterns (Eino v0.1.13+)
- `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go` — Proven Eino OpenAI adapter + mock server pattern (reuse for wire validation)
- `.claude/skills/spike-findings-ai-agent/sources/003-structured-output-temperature-zero/main.go` — Proven structured output pattern (reuse for E2E validation)
- `.claude/skills/spike-findings-ai-agent/SOURCES_INDEX.md` (if exists) — Index of all 4 spike source files

### Sketch findings (UI prep — auto-loads)

- `.claude/skills/sketch-findings-ai-agent/SKILL.md` — Frontend decisions index
- `.claude/skills/sketch-findings-ai-agent/references/layout-three-panel.md` — Phase 4 layout blueprint
- `.claude/skills/sketch-findings-ai-agent/references/chat-input-attach-skill.md` — Phase 4 chat input blueprint

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **spike 001 `main.go`** (`.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go`) — Eino OpenAI adapter + mock HTTP server pattern. **Reuse** the mock server side-by-side with real API for wire-format regression testing. Pattern includes:
  - `openai.NewChatModel(ctx, &openai.ChatModelConfig{BaseURL, APIKey, Model})`
  - `model.WithTemperature(float32)`, `model.WithMaxTokens(int)`, `model.WithExtraFields(map[string]any)`
  - Local mock server at `:18080` with `text/event-stream` responses
- **spike 003 `main.go`** (`.claude/skills/spike-findings-ai-agent/sources/003-structured-output-temperature-zero/main.go`) — structured output via `response_format:{type:json_object}` + `thinking:{type:disabled}`. **Reuse** the Go struct + `validate()` pattern for compliance verdicts (`{status, confidence, citations, review_required}`)
- **GLM-5.2 + Coding Plan endpoint** (`https://open.bigmodel.cn/api/coding/paas/v4`) — proven model + endpoint combo (validated in spike 002/002b/003). NOT standard `/api/paas/v4`.

### Established Patterns

- **Coding Plan API key handling** — env var `GLM_API_KEY` (or hardcode fallback for spike). User provided two: `42d522...` (no balance) and `c23710516a6d4a12a24863d0d06be917.SFsK8jprfDgnxv7G` (has balance — use this).
- **Mock server + real API parallel** — pattern from spike 001: mock for fast iteration, real for final validation. Phase 1 spike applies this in BOTH directions (mock for wire regression, real for E2E).
- **SHA-256 of normalized output** as deterministic identity for variance tracking (spike 002/002b method). Reusable for any future variance testing.
- **localStorage persistence for user preferences** (from sketch 001) — pattern for any client-side state.

### Integration Points

- **Eino ChatModel → OpenAI adapter** (proven) → wraps GLM-5.2
- **Hertz `pkg/protocol/sse`** (locked stack) → wraps Eino `StreamReader[*schema.Message]` → emits to Vercel AI SDK v5 client
- **Vercel AI SDK v5 UIMessageChunk** (FE) → parses `data: {...}` frames; typed parts map to `text` / `reasoning` / `tool-invocation` / `source` UIMessage parts
- **Milvus v2.5+** + **BGE-M3** (Docker self-host) → tier-filtered retrieval
- **Eino retriever abstraction** (`eino-ext/components/retriever/milvus`) → wraps Milvus + tier filter

### Creative Options

- Eino ADK's `WithChatModelOptions(...)` lets you pass per-agent chat model options — useful for Phase 3 multi-agent (Phase 1 spike may defer ADK)
- Milvus collection-level vs metadata-filter isolation — spike validates metadata-filter approach (lighter, scales better per `research/SUMMARY.md`)
- Eino's `RequestBodyModifier` (deprecated in v0.1.17, use `WithExtraFields`) — avoid

</code_context>

<specifics>
## Specific Ideas

- **Phase 1 spike should be a single Go program** with subcommands: `spike wire`, `spike sse`, `spike milvus`, `spike e2e` (each subcommand runs a focused check). Output is a single pass/fail + trace log per subcommand. Cleaner than multiple separate spike binaries.
- **Use the SAME 5 compliance queries from spike 002/003** for the E2E test — enables direct comparability with prior spike results.
- **Phase 1's runnable Go skeleton** should be the seed for `cmd/spiked/` (Phase 2's first real code) — reuse the directory structure.

No specific UI/UX requirements — Phase 1 is backend integration, no UI work.

</specifics>

<deferred>
## Deferred Ideas

- **Multi-agent orchestration** (Eino ADK `SequentialAgent` / `ParallelAgent` / `LoopAgent`) — Phase 3 (Agentic Retrieval & Trust Evaluation). Not Phase 1.
- **Real React frontend with Vercel AI SDK v5** — Phase 4 (Streaming Trust UI). Phase 1 spike can use a minimal stub client.
- **Ragas faithfulness eval pipeline** (Python sidecar) — Phase 6 (Eval Dashboard & Production Hardening). Not Phase 1.
- **Skill Wizard UI** (NL → SKILL.md generation) — Phase 5 (Skill Platform). Not Phase 1.

</deferred>

---

*Phase: 1-Integration-Spike-Trust-Contract*
*Context gathered: 2026-08-11*
