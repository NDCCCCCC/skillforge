---
phase: 03-agentic-retrieval-trust-evaluation
plan: 03
subsystem: backend/agent
tags: [agent, react, trust-floor, critique, callbacks, sse, arag, d3-01, d3-04, d3-05]
requires:
  - "03-01 (wire.DecisionPayload + data-decision discriminator)"
  - "02-03 (verdict.Verdict / Citation / Refusal / GroundingChecker / grounding floor)"
  - "02-02 (skill.Manifest.ResolvedTiers)"
  - "02-01 (retriever.Retriever interface)"
  - "spike-003 (structured-output option slice — T-2-21 framing + json_object envelope)"
provides:
  - "internal/agent.Agent + AgentConfig + NewAgent + (*Agent).Run — Plan 03-05 chat handler entry point"
  - "internal/agent.BuildRetrieveTool — InvokableTool closing over manifest.ResolvedTiers (KNOW-03)"
  - "internal/agent.ChunkAccumulator + ApplyTrustFloor — D3-01 / D-09 / KNOW-04 refusal-by-construction moat"
  - "internal/agent.(*Agent).MaybeCritique — D3-04 / D3-05 layered Self-RAG critique (cannot raise Confidence)"
  - "internal/agent.newDecisionCallback — SC-1 data-decision SSE emission via Eino callbacks"
  - "skill.Manifest.SetResolvedTiersForTest — cross-package test affordance (NEW)"
affects:
  - "Plan 03-05 (chat handler) — wires (*Agent).Run into /api/chat with Emit func"
  - "Plan 03-06 (eval sidecar) — calls (*Agent).Run with Emit=nil in batch mode"
  - "internal/server/pipeline.go — superseded by Agent.Run in Plan 03-05; stays compileable until then"
tech_stack:
  added:
    - "github.com/cloudwego/eino/flow/agent/react (ReAct agent primitive)"
    - "github.com/cloudwego/eino/flow/agent (AgentOption / WithComposeOptions)"
    - "github.com/cloudwego/eino/compose (ToolsNodeConfig / WithCallbacks)"
    - "github.com/cloudwego/eino/callbacks (Handler / RunInfo — parent callbacks pkg)"
    - "github.com/cloudwego/eino/utils/callbacks aliased as template (HandlerHelper / ModelCallbackHandler / ToolCallbackHandler)"
    - "github.com/cloudwego/eino/components/tool/utils (NewTool generic helper)"
    - "github.com/cloudwego/eino/components/model (ToolCallingChatModel interface — preferred over deprecated ChatModel)"
  patterns:
    - "ReAct loop bounded by MaxStep=N*2+1 (Rule 1 deviation from plan's +2 — see ComputeMaxStep godoc)"
    - "Per-Run state threaded via context.Value (accCtxKey) — avoids rebuilding react.Agent per Run"
    - "Mutex-guarded chunkAccumulator (Pitfall 2 race prevention; 100×10 concurrent Add() test green under -race)"
    - "Structural enforcement via grep assertion: critique.go has 0 writes to v.Confidence/Status/Citations"
    - "Layered trust: Trust floor (Go deterministic) > GroundingChecker (Phase 2 unchanged) > MaybeCritique (LLM additive-only)"
key_files:
  created:
    - path: internal/agent/agent.go
      lines: 432
      contains: "react.NewAgent + ComputeMaxStep + (*Agent).Run"
    - path: internal/agent/retrieve_tool.go
      lines: 222
      contains: "BuildRetrieveTool + formatChunksAsData (T-2-21 [CHUNK %d] framing)"
    - path: internal/agent/trust_floor.go
      lines: 215
      contains: "ChunkAccumulator + ApplyTrustFloor + ruleBasedSuggestion"
    - path: internal/agent/critique.go
      lines: 233
      contains: "MaybeCritique + runCritiqueLLMCall + D3-04 INVARIANT block"
    - path: internal/agent/callbacks.go
      lines: 227
      contains: "newDecisionCallback + stepCounter + DecisionRecord"
  modified:
    - path: internal/skill/manifest.go
      change: "+SetResolvedTiersForTest test affordance (15 lines)"
    - path: internal/agent/agent_test.go
      change: "All 18 unit tests + multiTurnStubChatModel + blockingStubChatModel + decisionCapturer"
decisions:
  - "Used react.Agent.Generate directly (synchronous *schema.Message) instead of plan's adk.NewRunner+iterator drain — Rule 3 API adaptation (react.Agent does NOT satisfy adk.Agent interface in eino@v0.7.7)"
  - "AgentConfig.ChatModel typed as model.ToolCallingChatModel (not the wider model.ChatModel) — BindTools-only models cannot drive a ReAct loop; type-level rejection at construction is honest"
  - "ComputeMaxStep formula corrected from MaxHops*2+2 to MaxHops*2+1 — plan's formula actually admitted N+1 retrieve calls, defeating the cap (Rule 1 bug; locked by TestMaxStepBlocksThirdHop)"
  - "Per-Run chunkAccumulator threaded via context.Value instead of rebuilding react.Agent per Run — compose.Graph construction is expensive; ctx-injection lets us reuse the cached react.Agent"
  - "skill.Manifest.SetResolvedTiersForTest exported as test affordance — Phase 2's disk-I/O newTestManifest pattern is too heavyweight for Phase 3 unit tests; one exported method with *ForTest suffix is the minimum-pollution alternative"
  - "CritiqueLowConfidence=0.6 / CritiqueHighConfidence=0.9 — illustrative v1 band; Plan 03-06 calibrates from golden-set Ragas numbers"
metrics:
  duration: "~29 minutes (executor wall clock)"
  completed: "2026-08-12T01:40:04Z"
  tasks_complete: 3
  files_created: 5
  files_modified: 2
  tests_added: 18
  commits: 6
---

# Phase 3 Plan 03: ReAct Agent + Trust Floor + Self-RAG Critique Summary

**One-liner:** Eino ReAct agent bounded by MaxStep=5 (Rule 1 deviation from plan's MaxStep=6), with mutex-guarded chunkAccumulator feeding a Go-only trust floor that forces refusal on 0 tier-matched chunks (D-09/KNOW-04), a layered Self-RAG critique that structurally cannot raise Confidence (D3-04 regression-locked), and SC-1 decision-observability SSE via Eino callbacks.

## What Was Built

### 1. `internal/agent/agent.go` (Task 1 + Task 3)

- **`AgentConfig`** — Manifest / Embedder / Retriever / ChatModel (`model.ToolCallingChatModel`) / MaxHops / Emit.
- **`Agent`** wrapper around `*react.Agent` plus cfg + cached retrieve tool.
- **`NewAgent`** validates non-nil deps, defaults `MaxHops=2`, calls `BuildRetrieveTool`, builds `react.AgentConfig{ToolCallingModel, ToolsConfig, MaxStep}`, calls `react.NewAgent`.
- **`ComputeMaxStep(maxHops)`** — corrected D3-02 formula `MaxHops*2 + 1` (=5 for default). See deviations.
- **`(*Agent).Run`** — production entry point: stashes per-Run `ChunkAccumulator` in ctx; constructs system+user msgs; calls `reactAgent.Generate` with optional `compose.WithCallbacks`; trust floor → parse → GroundingChecker → MaybeCritique.
- **`buildAgentMessages` + `systemPrompt`** — lifts T-2-21 prompt from `pipeline.go:60-81` (migration target: shared `prompt.go` in Plan 03-05).

### 2. `internal/agent/retrieve_tool.go` (Task 1, augmented Task 3)

- **`BuildRetrieveTool(emb, ret, m)`** — InvokableTool via `utils.NewTool`. Closes over `m.ResolvedTiers()` ONCE (KNOW-03 — agent LLM cannot override tiers).
- **Empty-query guard** — LLM hallucination of `{"query":""}` produces a clean error the ReAct loop can feed back.
- **`formatChunksAsData`** — character-for-character copy of `pipeline.go:234-241` `[CHUNK %d] tier=%s source=%s\n%s\n---\n` envelope (T-2-21 prompt-injection mitigation).
- **Task 3 augmentation** — reads `*ChunkAccumulator` from ctx via `accCtxKey{}` and calls `acc.Add(hits)` before returning. When no accumulator is present (Task 1 unit tests), the call is a no-op.

### 3. `internal/agent/trust_floor.go` (Task 2)

- **`ChunkAccumulator`** — mutex-guarded; `seen map[string]struct{}` for dedup-by-chunk-ID; `byID map[string]Hit` for chunk content lookup; `callCount int` for retrieve-call counting; `closed bool` for EOF discipline.
- **`Add`** dedups by chunk ID (D3-01 open Q2 — same chunk from 2 hops counts once); no-ops after `MarkClosed`.
- **`TierMatched`** / **`ChunksByID`** return caller-owned slices/maps; safe to iterate without the lock.
- **`HasMultipleRetrieveCalls`** — backs Run's critique gating (D3-05).
- **`ApplyTrustFloor`** — D3-01 / D-09 / KNOW-04 refusal-by-construction moat. Returns `nil` when accumulator has ≥1 chunk; returns `&Refusal{Reason: RefusalReasonNoAuthorizedSource, CanRephrase: true, SuggestedAlternativeQuery: ...}` when empty.
- **`ruleBasedSuggestion`** — logic-equivalent copy of `pipeline.go:267-278` (anti-pattern #1 drift guard).

### 4. `internal/agent/critique.go` (Task 2)

- **`(*Agent).MaybeCritique`** — D3-04 / D3-05 / ARAG-03 layered Self-RAG critique. Gated by `multiHop || status==需补充 || confidence∈[0.6,0.9]`. Errors are non-fatal — verdict returned UNCHANGED on critique LLM failure (Pitfall 3).
- **`runCritiqueLLMCall`** — structured-output JSON parse (`{"semantic_reversal_detected": bool}`); uses `verdict.Normalize` on both chunk and span text for parity with `GroundingChecker`'s substring check.
- **`buildCritiqueUserPrompt`** — formats verdict + chunks with both raw and normalized forms.
- **D3-04 INVARIANT** comment block at top of `MaybeCritique`; the structural enforcement is grep-asserted (`v.Confidence =|v.Status =|v.Citations =` returns 0 in critique.go).

### 5. `internal/agent/callbacks.go` (Task 3)

- **`newDecisionCallback(emit)`** — returns `(callbacks.Handler, *stepCounter)`. Uses `template.NewHandlerHelper().ChatModel(h).Tool(h).Handler()` (canonical Eino pattern).
  - `ModelCallbackHandler.OnEnd` emits `DecisionAnswer` when ChatModel returns no tool calls (final answer).
  - `ToolCallbackHandler.OnStart` increments stepCounter, emits `DecisionRetrieve` with parsed query.
  - Returns `(nil, nil)` when emit==nil — Run skips `compose.WithCallbacks` (batch mode for Plan 03-06).
- **`DecisionRecord`** — in-memory payload shape; converts to `wire.DecisionPayload` at the chat handler boundary (Plan 03-05). Decouples `internal/agent` from `internal/wire` (which is Hertz-aware).
- **`tryParseConfidence`** / **`tryParseQuery`** — best-effort JSON extraction (non-fatal).
- **`stripJSONDelim`** — trims ```` ```json ```` fences some shims add despite `response_format=json_object`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ComputeMaxStep formula off by 1 (cap admitted N+1 retrieves)**
- **Found during:** Task 3 (`TestMaxStepBlocksThirdHop` failed: got 3 retrieves, expected 2)
- **Issue:** Plan specified `MaxStep = MaxHops*2 + 2`. Empirically verified against `eino@v0.7.7/flow/agent/react/option_test.go:71` (MaxStep=3 produces single-tool-call + final answer). The correct formula for "cap at N retrieves + final chat" is `MaxHops*2 + 1`. The plan's "+2 for safety" actually admitted one more tool call — at MaxStep=6 with MaxHops=2, a 3rd retrieve fired before the loop terminated, defeating the cap.
- **Fix:** Changed `ComputeMaxStep` to `maxHops*2 + 1`; updated `TestComputeMaxStepFormula` expectations to `{1,3},{2,5},{3,7}`; updated `DefaultMaxHops` godoc.
- **Files modified:** `internal/agent/agent.go`, `internal/agent/agent_test.go`
- **Commit:** `b30a87f` (Task 3 GREEN)
- **Regression-locked by:** `TestMaxStepBlocksThirdHop` (asserts retriever.CallCount==2 for MaxHops=2)

**2. [Rule 3 - Blocking issue] react.Agent ≠ adk.Agent — plan's adk.NewRunner wiring does not compile**
- **Found during:** Task 1 GREEN attempt
- **Issue:** Plan assumed `adk.NewRunner(ctx, adk.RunnerConfig{Agent: reactAgent})` + iterator drain. Verified at compile time against eino@v0.7.7: `react.Agent` has `Generate(ctx, msgs, opts...) (*schema.Message, error)` while `adk.Agent` expects `Query(...) iterator`. The two interfaces are incompatible — `adk.NewRunner` cannot wrap `react.Agent`.
- **Fix:** Used `react.NewAgent` directly; call `reactAgent.Generate(ctx, msgs, agent.WithComposeOptions(...))` for synchronous single-message result. No iterator drain needed.
- **Files modified:** `internal/agent/agent.go`
- **Commit:** `e8b4c80` (Task 1 GREEN)

**3. [Rule 3 - Blocking issue] AgentConfig.ChatModel typed too widely**
- **Found during:** Task 1 GREEN attempt
- **Issue:** Plan specified `ChatModel model.ChatModel`. The `model.ChatModel` interface has `BindTools`; `model.ToolCallingChatModel` has `WithTools` instead. They are SIBLING interfaces — neither is a subtype of the other. `react.AgentConfig.ToolCallingModel` requires the `WithTools` variant.
- **Fix:** Typed `AgentConfig.ChatModel` as `model.ToolCallingChatModel`. This is honest (the type system now rejects BindTools-only models at construction, not at first use).
- **Files modified:** `internal/agent/agent.go`
- **Commit:** `e8b4c80` (Task 1 GREEN)

**4. [Rule 3 - Blocking issue] Callback signatures differ from plan/research**
- **Found during:** Task 3 GREEN implementation
- **Issue:** Plan / 03-RESEARCH.md §Code Examples showed callbacks with signature `func(ctx, output, _) error`. Verified at compile time against `eino@v0.7.7/utils/callbacks/template.go:430-433`: actual signature is `func(ctx context.Context, runInfo *callbacks.RunInfo, output *model.CallbackOutput) context.Context` — returns `context.Context`, NOT `error`. Takes a typed `*callbacks.RunInfo` parameter.
- **Fix:** Used the actual signatures. Documented the import-graph fact (two `callbacks` packages in Eino: parent `eino/callbacks` for `Handler`/`RunInfo`, `eino/utils/callbacks` aliased as `template` for `HandlerHelper`/`*CallbackHandler`).
- **Files modified:** `internal/agent/callbacks.go`
- **Commit:** `b30a87f` (Task 3 GREEN)

**5. [Rule 3 - Blocking issue] `compose.ToolsNodeConfig` field name**
- **Found during:** Task 1 GREEN attempt
- **Issue:** Plan / research note A2 specified `InvokableTools []tool.InvokableTool`. Verified at `eino@v0.7.7/compose/tool_node.go:121`: actual field is `Tools []tool.BaseTool`. (`tool.InvokableTool` satisfies `tool.BaseTool` since `BaseTool` is the minimal `Info()` interface.)
- **Fix:** Used `Tools: []tool.BaseTool{retrieveTool}`.
- **Files modified:** `internal/agent/agent.go`
- **Commit:** `e8b4c80` (Task 1 GREEN)

**6. [Rule 3 - Blocking issue] `schema.ParamsOneOf` construction**
- **Found during:** Task 1 GREEN attempt
- **Issue:** Plan showed `schema.ParamsOneOf{ParamsOneOfByParams: ...}` literal. Verified at `eino@v0.7.7/schema/tool.go:95`: `ParamsOneOf` has unexported fields only — cannot be constructed via struct literal. Must use `schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{...})`. Additionally, `ParameterInfo` uses `Desc` (not `Description`) and `Type` is `DataType` (`schema.String`).
- **Fix:** Used `schema.NewParamsOneOfByParams(...)` with correct `ParameterInfo` field names.
- **Files modified:** `internal/agent/retrieve_tool.go`
- **Commit:** `e8b4c80` (Task 1 GREEN)

**7. [Rule 3 - Blocking issue] `verdict.StatusNeedsMore` does not exist**
- **Found during:** Task 2 implementation
- **Issue:** Plan referenced `verdict.StatusNeedsMore`. No such constant exists in `internal/verdict/`.
- **Fix:** Used `verdict.DowngradeStatus` (same value `"需补充"`, already exported, already used by `GroundingChecker`).
- **Files modified:** `internal/agent/critique.go`
- **Commit:** `5e16c53` (Task 2 GREEN)

**8. [Rule 3 - Blocking issue] Cross-package test construction of `*skill.Manifest` with resolved tiers**
- **Found during:** Task 1 RED attempt
- **Issue:** `skill.Manifest.resolvedTiers` is unexported. Phase 2's `internal/server/chat_test.go:newTestManifest` writes a SKILL.md to a temp dir + invokes `Loader.Scan` — too heavyweight for Phase 3 unit tests. Reflection cannot set unexported fields without `unsafe`.
- **Fix:** Added `skill.Manifest.SetResolvedTiersForTest(tiers []string)` exported method. The `ForTest` suffix flags any non-test caller in review. 03-04 (KNOW-02 enforcement) does NOT touch `internal/skill/manifest.go` (verified via 03-04-PLAN.md `files_modified`), so no parallel-execution conflict.
- **Files modified:** `internal/skill/manifest.go`
- **Commit:** `4a5a503` (Task 1 RED)

## Threat Mitigation Summary

| Threat | Mitigation | Regression Lock |
|--------|------------|-----------------|
| T-3-INJECT (T-2-21 prompt injection) | `[CHUNK %d] ... \n---\n` envelope in `formatChunksAsData` byte-identical to `pipeline.go:234-241` | `TestRetrieveToolFramesChunksAsData` |
| T-3-FLOOR (D3-01 bypass) | `ApplyTrustFloor` runs after `MarkClosed` and reads `TierMatched()` under mutex; agent LLM has no API to write to the accumulator | `TestTrustFloorForcesRefusal`, `TestTrustFloorPassesWhenChunksExist` |
| T-3-CRIT (D3-04 critique overriding floor) | Structural: only `v.ReviewRequired = true` is a permitted write in critique.go; grep assertion `v\.(Confidence\|Status\|Citations) =` returns 0 | `TestCritiqueCannotRaiseConfidence` |
| T-3-RACE (Pitfall 2 accumulator race) | `ChunkAccumulator` mutex-guarded; `Add`, `TierMatched`, `ChunksByID`, `MarkClosed`, `HasMultipleRetrieveCalls` all take the lock | `TestAccumulatorConcurrent` (100 goroutines × 10 hits, -race clean) |
| T-3-HOP (MaxStep miscounting) | `ComputeMaxStep` formula corrected to `MaxHops*2+1` (Rule 1 deviation); TestComputeMaxStepFormula locks the formula | `TestMaxStepBlocksThirdHop` (asserts retriever.CallCount==2) |
| T-3-CANCEL-SRC (Pitfall 4 cancel propagation) | ctx flows through `reactAgent.Generate` → `compose.Graph`; blockingStubChatModel returns on `<-ctx.Done()` | `TestAgentRunCancelMidTool` (<250ms responsiveness) |
| T-3-SC (supply chain) | No new dependencies; all imports from existing `go.mod` | n/a |

## Test Results

```
$ go test -race -count=1 -timeout 60s ./internal/agent/...
=== RUN   TestComputeMaxStepFormula
--- PASS: TestComputeMaxStepFormula (0.00s)
=== RUN   TestBuildRetrieveTool
--- PASS: TestBuildRetrieveTool (0.02s)
=== RUN   TestRetrieveToolFramesChunksAsData
--- PASS: TestRetrieveToolFramesChunksAsData (0.00s)
=== RUN   TestRetrieveToolEnforcesResolvedTiers
--- PASS: TestRetrieveToolEnforcesResolvedTiers (0.00s)
=== RUN   TestNewAgentConfig
--- PASS: TestNewAgentConfig (0.00s)
=== RUN   TestTrustFloorForcesRefusal
--- PASS: TestTrustFloorForcesRefusal (0.00s)
=== RUN   TestTrustFloorPassesWhenChunksExist
--- PASS: TestTrustFloorPassesWhenChunksExist (0.00s)
=== RUN   TestTrustFloorDedup
--- PASS: TestTrustFloorDedup (0.00s)
=== RUN   TestAccumulatorConcurrent
--- PASS: TestAccumulatorConcurrent (0.01s)
=== RUN   TestCritiqueSemanticReversal
--- PASS: TestCritiqueSemanticReversal (0.00s)
=== RUN   TestCritiqueCannotRaiseConfidence
--- PASS: TestCritiqueCannotRaiseConfidence (0.00s)
=== RUN   TestCritiqueGating
--- PASS: TestCritiqueGating (0.00s)
=== RUN   TestCritiqueErrorNonFatal
--- PASS: TestCritiqueErrorNonFatal (0.00s)
=== RUN   TestAgentRetrievesAndAnswers
--- PASS: TestAgentRetrievesAndAnswers (0.00s)
=== RUN   TestMultiHopCitationAggregation
--- PASS: TestMultiHopCitationAggregation (0.00s)
=== RUN   TestMaxStepBlocksThirdHop
--- PASS: TestMaxStepBlocksThirdHop (0.00s)
=== RUN   TestDecisionSSEEmission
--- PASS: TestDecisionSSEEmission (0.00s)
=== RUN   TestAgentRunCancelMidTool
--- PASS: TestAgentRunCancelMidTool (0.05s)
PASS
ok  	github.com/skillforge/skillforge/internal/agent	1.303s
```

Full-suite regression check: `go test ./...` green across all packages (`internal/server`, `internal/skill`, `internal/verdict`, `internal/wire`, `internal/store`, `internal/knowledge`, `internal/retriever/mongo`, `cmd/spiked`).

## TDD Gate Compliance

| Task | RED commit | GREEN commit | REFACTOR |
|------|------------|--------------|----------|
| 1 — agent.go + retrieve_tool.go | `4a5a503` (build failed: undefined `agent.BuildRetrieveTool` / `agent.NewAgent` / `agent.AgentConfig`) | `e8b4c80` | n/a (clean from first pass) |
| 2 — trust_floor.go + critique.go | `1ac77e7` (build failed: undefined `agent.NewChunkAccumulator` / `agent.ApplyTrustFloor` / `a.MaybeCritique`) | `5e16c53` | n/a |
| 3 — callbacks.go + Run loop | `e04fce9` (build failed: undefined `a.Run` / `agent.DecisionRecord`) | `b30a87f` | n/a |

All three RED gates failed as expected (undefined symbols). All three GREEN gates green under `-race`. No TDD gate violations.

## Known Stubs / Deferred Items

- **`internal/agent/agent.go:systemPrompt`** is a *partial* lift of `pipeline.go:60-81` systemPromptTemplate — the two `%s` slots (tier-keyword hints, refusal-phrase hints) are inlined as constants rather than Skill-driven. Plan 03-05 will introduce `internal/agent/prompt.go` shared between this package and `internal/server/pipeline.go` and re-introduce the slots if a Skill needs to override.
- **`ruleBasedSuggestion` (trust_floor.go) and `truncate` (critique.go)** are logic-equivalent copies of `pipeline.go:267-278` and `pipeline.go:283-288`. Marked as anti-pattern #1 drift guards; Plan 03-05 lifts both into `internal/agent/prompt.go` so both packages import from one source.
- **Critique uncertain-band constants** (`CritiqueLowConfidence=0.6`, `CritiqueHighConfidence=0.9`) are illustrative v1 values. Plan 03-06 (Ragas eval sidecar) calibrates from the golden-set's per-Skill faithfulness numbers; the constants live in `critique.go` so the calibration is a one-line change.
- **No structured-output option slice wired into `(*Agent).Run`'s `reactAgent.Generate` call yet** — the spike-003-proven `model.WithTemperature + model.WithMaxTokens + openai.WithExtraFields({"response_format":{"type":"json_object"}, "thinking":{"type":"disabled"}})` option slice is documented in `agent.go`'s package godoc but not passed because the stubChatModel does not require it. Plan 03-05's chat handler injects the slice via `flowagent.WithChatModelOptions(...)` when wiring against a real `openai.NewChatModel`.

## Threat Flags

None — no security-relevant surface introduced beyond what the plan's `<threat_model>` already enumerated and mitigated (T-3-INJECT / T-3-FLOOR / T-3-CRIT / T-3-RACE / T-3-HOP / T-3-CANCEL-SRC / T-3-SC). Verified via grep: no new network endpoints, no new auth paths, no new file-access patterns, no new schema changes at trust boundaries.

## Self-Check: PASSED

Files created:
- `internal/agent/agent.go` — FOUND
- `internal/agent/retrieve_tool.go` — FOUND
- `internal/agent/trust_floor.go` — FOUND
- `internal/agent/critique.go` — FOUND
- `internal/agent/callbacks.go` — FOUND

Files modified:
- `internal/skill/manifest.go` — FOUND (+`SetResolvedTiersForTest`)
- `internal/agent/agent_test.go` — FOUND (18 tests)

Commits in `git log --oneline`:
- `4a5a503` test(03-03): add failing tests for ReAct agent + retrieve tool — FOUND
- `e8b4c80` feat(03-03): implement ReAct agent + retrieve tool — FOUND
- `1ac77e7` test(03-03): add failing tests for trust floor + critique (Task 2) — FOUND
- `5e16c53` feat(03-03): implement trust floor + gated Self-RAG critique — FOUND
- `e04fce9` test(03-03): add failing tests for full Run loop + SC-1 callbacks (Task 3) — FOUND
- `b30a87f` feat(03-03): implement Run + SC-1 decision callbacks — FOUND
