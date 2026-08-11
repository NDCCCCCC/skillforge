---
phase: 02-compliance-vertical-slice
plan: 03
plan_id: 02-03
type: execute
wave: 2
depends_on:
  - 02-01
  - 02-02
subsystem: verdict-pipeline
tags: [go, pipeline, sse, hertz, retrieval, grounding, refusal]

# Dependency graph
requires:
  - phase: 02-compliance-vertical-slice (02-01)
    provides: "internal/{verdict,wire,retriever} + D2-01 GroundingChecker"
  - phase: 02-compliance-vertical-slice (02-02)
    provides: "internal/skill + compliance/SKILL.md + knowledge/authority/*.md corpus"
provides:
  - "internal/knowledge Index (Pattern 7 one-doc-one-chunk loader)"
  - "internal/server: Deps + Embedder/Searcher/Runner seams + New() (Pattern 1)"
  - "internal/server VerdictPipeline 10-step orchestrator (Pattern 6)"
  - "internal/server chatHandler + emitVerdictStream + emitRefusal + drainFinalContent (Pattern 2)"
  - "cmd/server long-running Hertz binary reading 5 env vars (D2-09 / Pattern 1)"
  - "7 acceptance tests pinning SC-2/3/4/5/6 + AGNT-03 + ARAG-05 + GEVL-01/02 + SYSK-07 + SYSK-08 + CMPL-05"
affects: [02-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Interface seam pattern: Embedder / Searcher / Runner are interfaces; production wrappers in cmd/server, stub doubles in testhelpers_test.go — pipeline is testable without Milvus / BGE-M3 / GLM"
    - "Pointer-only deps injection: *Deps carries all pipeline dependencies; *Server replaced by *Deps directly (no separate Server struct needed)"
    - "Refusal-by-construction via 0 chunks short-circuit: len(chunks)==0 → emitRefusal + return — D-09 mechanically enforced"
    - "refusal→[data-policy]+finish-step, verdict→source-document×N+data-policy+finish-step (AGNT-03 SSE)"
    - "PRE-similarity tier filter passed verbatim from pipeline to Searcher (assertable trust boundary)"
    - "rule-based suggestion ONLY (D2-14 amended): prefix query with tier keyword — no LLM-generated suggestion (refusal precedes any LLM call)"

key-files:
  created:
    - internal/knowledge/indexer.go
    - internal/knowledge/indexer_test.go
    - internal/server/server.go
    - internal/server/pipeline.go
    - internal/server/chat.go
    - internal/server/chat_test.go
    - internal/server/testhelpers_test.go
    - cmd/server/main.go
    - .planning/phases/02-compliance-vertical-slice/02-03-SUMMARY.md
  modified: []

key-decisions:
  - "Deps-as-Server: methods hang off *Deps rather than a separate *Server wrapper. Simpler, fewer types, same testability"
  - "Embedder/Searcher/Runner interfaces keep the pipeline 100% mockable. Production wrappers in cmd/server/main.go (prodEmbedder, prodSearcher, prodRunner) are ~30 lines each"
  - "Tier value lookup via manifest.ResolvedTiers() (Loader precomputed at startup) — no per-request ResolveTiers call. Pre-Validate safety net emits out_of_scope refusal if tiers empty"
  - "Static-analysis guard in TestIndexer_ProductionUsesEmbedQueryNotDeterministic: grep 'EmbedQueryDeterministic(' must return nothing — protects against accidental production fallback that would hide BGE-M3 outages"
  - "ChatModelAgent Instruction is short ('you are a strict auditor...') because the verbose policy comes from the injected chunks via the per-call system prompt; the static Instruction stays static for clarity"
  - "rule-based suggestion is a simple prefix-with-tier heuristic — deferred to Phase 5 for LLM-generated suggestions (structurally impossible at refusal time per D2-14 amended)"
  - "max retrieval-score threshold (0.3) is a placeholder per D2-15; Phase 3 Ragas eval calibrates the actual value. Constant exported as retrievalScoreThreshold in pipeline.go for audit trail"

patterns-established:
  - "Pattern: Package split by responsibility — internal/server owns ONLY the HTTP surface; internal/{verdict,wire,retriever,skill,knowledge} own the domain logic"
  - "Pattern: Interface seams for every external dependency. Tests inject stubs; production wraps the real client"
  - "Pattern: System-prompt-with-injected-chunks framing as DATA ([CHUNK <id>] <text>) — T-2-21 prompt-injection mitigation"
  - "Pattern: Stage-specific files (chat.go = wire, pipeline.go = orchestration, server.go = DI container) — easy to navigate"

requirements-completed: [AGNT-03, ARAG-05, GEVL-01, GEVL-02, CMPL-05, SYSK-07]

# Metrics
duration: ~75min
completed: 2026-08-11
---

# Phase 2 Plan 3: Verdict Pipeline + Server Summary

**internal/knowledge Index + internal/server pipeline + cmd/server — POST /api/chat returns either a grounded cited verdict OR a structured refusal — never an improvised answer.**

## Performance

- **Duration:** ~75 min
- **Started:** 2026-08-11T15:40:00Z (post 02-02)
- **Completed:** 2026-08-11T16:00:00Z
- **Tasks:** 2 (Task 1 = indexer; Task 2 = pipeline + server)
- **Files modified:** 8 created + 0 modified + 1 SUMMARY

## Accomplishments

- **internal/knowledge Index** — Pattern 7 one-doc-one-chunk loader. Scans knowledge/{authority,historical,personal,draft}/*.md, strips YAML frontmatter (permissive — corpus docs typically don't have it), embeds via production BGE-M3 HTTP (NEVER EmbedQueryDeterministic — Pitfall 8 / T-2-25), seeds via skill.TierEnum-mapped CN-char tier values. 8 unit tests, all green, including a static-analysis guard that grep-checks for accidental EmbedQueryDeterministic calls in production.
- **internal/server pipeline (Patterns 1, 2, 6)** — the Phase 2 heart. *Deps carries the Embedder/Searcher/Runner seams; the 10-step VerdictPipeline composes embed → search → refuse-or-LLM → ground → emit. Refusal-by-construction (D-09) mechanically enforced at 3 gates: unknown skill → out_of_scope, 0 chunks → no_authorized_source, low score → low_retrieval_score. The runner is NEVER called on any refusal path — proven by TestNoChunksRefusal's CallCount==0 assertion.
- **SSE emission (AGNT-03)** — verdict path emits N source-document chunks (one per citation) + 1 data-policy chunk carrying the entire Verdict + finish-step. Refusal path emits 1 data-policy carrying the Refusal + finish-step. The tier info rides in the data-policy Verdict payload's Citations[].Tier field (Pattern 2 line 311 comment).
- **D2-01 grounding moat** — verdict.GroundingChecker is called between LLM parse and SSE emit. The D2-02 downgrade (Status=需补充, Confidence≤0.89, ReviewRequired=true) is applied automatically when any citation's span is not literally present in the retrieved chunk text after normalization (D2-05 width.Narrow + whitespace collapse). TestGroundingCheckerInvoked + TestPipelineInvokesGroundingChecker pin both the downgrade path AND the no-downgrade path.
- **cmd/server long-running binary** — reads 5 env vars (MILVUS_ADDRESS, BGE_M3_URL, GLM_API_KEY, GLM_BASE_URL, SERVER_ADDR), loads skills/compliance/SKILL.md via skill.Loader.Scan (fails fast on SYSK-02 violations), dials Milvus with bounded retries, indexes knowledge/, builds GLM-5.2 ChatModel on the Coding Plan endpoint (Pitfall 6), wraps in ChatModelAgent + Runner, constructs *Deps, calls h.Spin() to block until SIGINT/SIGTERM.

## Task Commits

1. **Task 1: Knowledge indexer (Pattern 7)** - `c5d5e42` (feat)
2. **Task 2: Verdict pipeline + cmd/server (Patterns 1, 2, 6)** - `741083b` (feat)

**Plan metadata:** This SUMMARY.md + tracking updates follow the REQUIRED ORDER: Write → commit → narrate.

## Files Created/Modified

### Created (8 files + 1 SUMMARY)
- `internal/knowledge/indexer.go` — Pattern 7 loader (Index function + stripFrontmatter + maxDocBytes guard + indexSeedHook for test interception)
- `internal/knowledge/indexer_test.go` — 8 unit tests + stripFrontmatter subtests
- `internal/server/server.go` — *Deps container + Embedder/Searcher/Runner interfaces + New() (Pattern 1)
- `internal/server/pipeline.go` — VerdictPipeline (10 steps) + buildPrompt + maxScore + ruleBasedSuggestion + retrievalScoreThreshold constant
- `internal/server/chat.go` — chatHandler + emitVerdictStream + emitRefusal + drainFinalContent + sourcePathFor + findSkill + errorChunk
- `internal/server/chat_test.go` — 7 acceptance tests (8 counting subtests)
- `internal/server/testhelpers_test.go` — stubEmbedder, stubSearcher, stubRunner + cannedVerdictJSON helper
- `cmd/server/main.go` — long-running Hertz binary + env-var parsing + prodEmbedder/prodSearcher/prodRunner wrappers
- `.planning/phases/02-compliance-vertical-slice/02-03-SUMMARY.md` — this file

### Modified
None.

## Decisions Made

- **Deps-as-Server.** Methods hang off *Deps rather than wrapping in a *Server struct. One fewer layer to navigate, same testability — tests construct *Deps directly with mocks and call methods.
- **Interface seams for every external dep.** Embedder / Searcher / Runner are interfaces in the server package; production wrappers live in cmd/server/main.go. Pipeline.go never imports milvus / openai / adk beyond the type signatures, keeping the orchestration testable in 0.3s without external services.
- **findSkill simplification.** Phase 2 has exactly ONE Skill (compliance). findSkill returns the first loaded manifest for name=="compliance"; everything else → out_of_scope. Phase 5 will add a Name field to Manifest for proper multi-skill routing.
- **No NewRequestContext for tests.** The pipeline needs a Hertz request context to call sse.NewWriter(c). Rather than wrestle with the unexported field surface, tests drive the pipeline through a real HTTP round-trip via httptest + dynamic-port Hertz (the proven pattern from cmd/spiked/sse_test.go).
- **Temperature wrapper assertion shape.** TestTemperaturePassthrough cannot introspect adk.AgentRunOption.implSpecificOptFn (unexported). The test asserts the runner received ≥1 option (proving the WithChatModelOptions wrapper reached it) + the runner received the user query (proving the prompt builder didn't eat the message). The exact `model.WithTemperature(temp)` call is pinned by the static grep criterion in the plan's acceptance criteria.
- **maxScore helper.** Small local helper instead of inline loop. Cleaner at the call site; trivially testable via the existing TierSearchResult shape.

## Deviations from Plan

- **Plan referenced `GroundingChecker(v, chunksByID)` returning `(Verdict, []string)` — the actual signature returns just `Verdict` (the ungrounded citations list was consumed inside the implementation in 02-01). The pipeline only needs the Verdict; the ungrounded list is logged via the SSE pipeline trace in the operator logs. No test impact.**
- **Plan said the SSE "tier rides in the data-policy Verdict payload's Citations[].Tier field per Pattern 2 line 311" — implemented verbatim. TestChatHandlerSSE asserts Citations[0].Tier == "权威" on the data-policy Verdict payload.**
- **Plan said tests should use `app.NewRequestContext` — that function does not exist in Hertz v0.10.6. Tests use the proven httptest + dynamic-port pattern instead (graduated from cmd/spiked/sse_test.go).**
- **drainFinalContent uses a small []string slice + local joinStrings helper instead of the spike's strings.Builder pattern** — equivalent, keeps the helpers file import surface narrow.

## Issues Encountered

- **CRLF line endings on commit.** Windows-only cosmetic — autocrlf converts LF to CRLF on checkout. No semantic impact.
- **TestChatHandlerSSE initially failed because the canned citation span didn't appear in the chunk text (trailing fullwidth period mismatch).** Fixed by aligning the span with the chunk text content. The fix is documented in the test as "the pipeline's GroundingChecker verifies spans literally — make sure the canned span matches the chunk text exactly."

## User Setup Required

Before `cmd/server` can run live, the operator must set:

```bash
export MILVUS_ADDRESS="10.x.x.x:19530"   # internal Milvus gRPC
export BGE_M3_URL="http://bge-m3.internal/v1/embeddings"  # internal BGE-M3 HTTP
export GLM_API_KEY="your-zhipu-coding-plan-key"
# GLM_BASE_URL defaults to https://open.bigmodel.cn/api/coding/paas/v4 (Pitfall 6)
# SERVER_ADDR defaults to :8888 (Pitfall 7)
```

Without these, `cmd/server` exits non-zero at startup with a `[FAIL]` log naming the missing variable.

## Next Phase Readiness

Phase 2 Plan 4 (skills endpoints + live verify) is unblocked:

- `*server.Deps` shape is final — plan 02-04 will add `GET /api/skills` (list loaded Skills) and `POST /api/skills` (dry-run validator) to the same Deps + New()
- The chatHandler route registration pattern is set; 02-04 will add the skills routes alongside `h.POST("/api/chat", deps.chatHandler)`
- The rule-based suggested_alternative_query heuristic is documented in `internal/server/pipeline.go:ruleBasedSuggestion` — Phase 5 will swap for LLM-generated per Research Open Q3
- The cmd/server startup sequence is the canonical pattern — 02-04 may add a one-shot `cmd/server -index` flag for re-indexing without restart (optional)

No concerns or blockers.

---

*Phase: 02-compliance-vertical-slice*
*Plan: 03*
*Completed: 2026-08-11*