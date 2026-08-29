# Phase 13: 子 agent 扇出 (P8.4 — tier 继承) — Research

**Researched:** 2026-08-21
**Domain:** Eino v0.9.14 ParallelAgent / AgentTool / manual errgroup fan-out + tier closure inheritance + per-sub-agent grounding + deterministic merge
**Confidence:** MEDIUM-HIGH (Eino ParallelAgent + AgentTool API verified by direct module-cache inspection; **RF-4 highest-uncertainty** — ctx-value propagation into sub-agent Tool closures NOT verified by docs, LOW-MEDIUM; tier-inheritance verification is design-level HIGH; merge determinism is design-level HIGH; per-sub-agent grounding is REUSE-LEVEL HIGH)

## Summary

Phase 13 (P8.4) is the LAST phase of the v0.8 Harness milestone. It adds a one-level parallel sub-agent fan-out capability to SkillForge: the orchestrator (chat or deep archetype) identifies multi-hop query structures and dispatches a bounded set of sub-agents in parallel, each returning a compressed result that the orchestrator merges into the final verdict. The **load-bearing moat** is **tier-inheritance via construction-time closure** (FAN-02): the sub-agent's `retrieve_compliance_chunks` Tool is structurally forced to query only the parent's `ResolvedTiers()` — the sub-agent cannot expand its tier boundary even via prompt injection because the tier slice is baked into the Tool's closure, not the system prompt. The **second moat** is **per-sub-agent grounding** (FAN-04): every sub-agent's verdict-shaped output flows through `ApplyTrustStage` (the Phase 12 unified exit) before the merge function sees it — the citation whitelist is unchanged because the sub-agent's `ChunkAccumulator` is shared with the parent Run (same accumulator, same `*Hit` slice).

The Eino v0.9.14 API surface exposes **two fan-out primitives**, both with significant caveats:
1. **`adk.NewParallelAgent(ctx, &ParallelAgentConfig{SubAgents: []Agent{...}})`** (`adk/workflow.go:698-705`) — officially "**NOT RECOMMENDED**: Workflow agents are built on agent transfer with full context sharing, which has not proven to be more effective empirically" (verified verbatim in the module source). The "full context sharing" semantics are fundamentally incompatible with FAN-02's structural tier-closure invariant: sub-agents built via `NewParallelAgent` inherit the parent's `runSession.Values` map (per `forkRunCtx` at `runctx.go:478-508`), so the sub-agent could read or modify session values that the parent's tier filter never touches. **Reject for FAN-02.**
2. **`adk.NewAgentTool(ctx, subAgent, options...)`** (`adk/agent_tool.go:93`) — wraps a pre-built `Agent` as an `InvokableTool`. Action scoping is the safety net: "**Exit, TransferToAgent, BreakLoop: Ignored outside the agent tool; these actions only affect the inner agent's execution and do not propagate to the parent agent**" (`agent_tool.go:88-90`). Sub-agent interrupts ARE propagated via `CompositeInterrupt` (line 87). This is the Eino-blessed pattern for sub-agent composition per the supervisor package docs ("Consider using ChatModelAgent with AgentTool or DeepAgent instead for most multi-agent scenarios" — `supervisor.go:43-44`).

Given (a) `NewParallelAgent` is rejected for FAN-02 + (b) `NewAgentTool` requires the LLM to call the sub-agent like a Tool (parent LLM emits `tool_call { name: "fanout_<id>", args: {"query": "..."} }`) which is a good fit for DeepAgent's existing sub-agent slots but a poor fit for the chat archetype (the chat archetype's Tool whitelist is `writes`-gated by `BuildToolsForSkill`), **Phase 13 ships a hybrid path**: a **NEW `FanOutOrchestrator` agent type** (`internal/runtime/orchestrator.go`) that (a) classifies incoming queries into 1..N sub-queries via a lightweight LLM call, (b) constructs N sub-agents per-Run via `runtime.DispatchAgent` with a **fresh `*skill.Manifest` whose `Reads` slice is structurally intersected** with the parent's (load-time error on empty intersection — FAN-02 SC-2), (c) fans them out via **`golang.org/x/sync/errgroup` with `SetLimit(4)`** (Option B — manual fan-out, accumulator baked into closures), and (d) merges results via a **key-sorted pure function** (FAN-03 SC-3 — 10× byte-identical contract test). The **per-sub-agent grounding** (FAN-04) reuses `ApplyTrustStage` from `internal/runtime/trust_stage.go` (Phase 12 unified exit, P12-L1) with a **shared per-Run `*ChunkAccumulator`** pointer so the citation whitelist spans parent + sub-agents (sub-agent's grounding check uses the same `chunksByID` map).

**Primary recommendation:** Phase 13 is **6 plans in 5 waves** (Wave 1: BE orchestrator + sub-agent factory + tier closure + RF-4 spike; Wave 2: BE merge pure-function + per-sub-agent grounding + fault isolation; Wave 3: BE audit RunPath/Transfer + retention; Wave 4: hermetic gate + frozen eval re-lock + STRIDE-13; Wave 5: acceptance gate + live fan-out UAT). **UI hint: no** — fan-out events ride existing audit + progress SSE channels (Phase 12 `data-plan-trace` + `data-decision`); no new product UI.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `FanOutOrchestrator` (NEW agent type) | API / Backend | — | One-level parallel fan-out for multi-hop queries; lives in `internal/runtime/orchestrator.go` (mirrors `factory.go` P12-L1 archetype dispatch pattern). Decision: where to place the classifier (LLM call) — a single pre-fanout classification step is the simplest path |
| Sub-agent construction (per-Run, tier-closure) | API / Backend | — | Each sub-agent built via `runtime.DispatchAgent(ctx, DispatchConfig{Manifest: subManifest, ...})` where `subManifest` is a fresh `*skill.Manifest` whose `Reads` is the structural intersection of parent's `ResolvedTiers()` ∩ sub-skill's `Reads`. Empty intersection = load-time error (FAN-02 SC-2) |
| Tier inheritance verification | API / Backend | — | `TestSubAgentTierInheritance` (compile-level structural test): construct a parent Skill with `Reads: [权威]`, dispatch a sub-skill with `Reads: [个人]`; the `subManifest` builder must FAIL at construction with substring `tier intersection empty`, NOT return a runtime-broken agent |
| Merge determinism (FAN-03) | API / Backend | — | Pure function `mergeSubAgentResults(results []SubAgentResult) MergedVerdict`. Key sort by `(sub_agent_id ASC, citation_chunk_id ASC, citation_span ASC)`; 10× byte-identical contract test (same input → same output, no map iteration randomness) |
| Conservative partial-failure merge | API / Backend | — | If ANY sub-agent returns `Status: refused`, merged verdict is `{status: "needs_input", confidence: 0, review_required: true}` (D2-14 conservative posture — same as Phase 2 refusal-by-construction) |
| Per-sub-agent grounding | API / Backend | — | Each sub-agent's final message flows through `runtime.ApplyTrustStage(ctx, &TrustStageInput{Manifest: subManifest, Output: subMsg, Chunks: subChunks})` before merge. The `Chunks` slice is read from the shared `*ChunkAccumulator` (parent + sub-agents accumulate into ONE map) |
| Shared `*ChunkAccumulator` | API / Backend | — | Pointer to parent's accumulator passed into each sub-agent's `DispatchConfig` (NEW field `ChunkAccumulator *agent.ChunkAccumulator`). Sub-agent's retrieve Tool records hits into the shared map — the citation whitelist is union of parent's seed retrieval + all sub-agent retrievals |
| Fault isolation | API / Backend | — | `errgroup.Group{}` with `SetLimit(4)` — one sub-agent panic → recovered, marked `sub_agent_status: faulted`; other sub-agents continue; merge sees `faulted` as refusal-equivalent |
| Fan-out event audit (FAN-04 RunPath/Transfer) | Database / Storage | API / Backend | New audit enum: `action_fanout_dispatched | action_fanout_completed | action_fanout_failed`. Existing `audit` table (Phase 11 substrate) extended with two nullable TEXT columns: `run_path TEXT` (slash-separated RunPath per `runctx.go:347`) + `transfer_from TEXT` + `transfer_to TEXT`. No DDL break — pure additive migration |
| LLM-driven fan-out classifier | API / Backend | — | One LLM call returns `{"sub_queries": [{"id": "sa1", "query": "...", "tier_filter": ["权威"]}, ...]}`. Validates sub-query IDs are unique + tier_filter is non-empty subset of parent's `ResolvedTiers()`; classifier prompt is `systemInstructionsTemplate` from Phase 3 (deterministic, low-temperature) |
| Sub-agent budget scope | API / Backend | — | Each sub-agent gets `BudgetConfig{MaxIteration: 5, TokenBudget: 20000, LoopWindow: 3}` (half of parent's; independent scope so one slow sub-agent can't drain the parent's budget) |
| FE rendering of fan-out events | Browser / Client | CDN / Static | **NO new product UI** (ROADMAP UI hint: no). Fan-out events emit through existing `data-decision` + `data-plan-trace` channels. FE agent state inspector (Phase 5 5-tab) shows fan-out events inline alongside plan_todos; CenterPanel audit timeline tab shows RunPath + Transfer rows |
| D15 wire-type compatibility | CDN / Static | API / Backend | If a new SSE chunk type is needed for fan-out events (e.g. `data-fanout-trace`), add via D15 lockstep (Go `wire.ValidTypes` + TS `SkillForgeChunk` in same commit). Recommended: REUSE `data-decision{decision:"fanout_dispatched"|"fanout_completed"|"fanout_failed"}` discriminator extension — D16 overload pattern (Phase 12 already overloads `data-decision` for `forced_stop`) |
| Retention redaction for fan-out rows | Database / Storage | API / Backend | Existing Phase 6 `SKILLFORGE_RETENTION_DAYS` applies; new `run_path` column is redacted after retention window (same policy as `query` and `answer`) |

**Key boundary:**
- `internal/runtime` keeps the single-funnel invariant (P12-L1 — `ApplyTrustStage` is the unified exit). The new `orchestrator.go` lives in `internal/runtime/` and calls `DispatchAgent` + `ApplyTrustStage` for every sub-agent.
- `internal/agent` keeps ChatModelAgent construction (Phase 11) + HITL Tools (Phase 11) + DeepAgent tools (Phase 12). The new `fanout_tools.go` lives in `internal/agent/` — the `fan_out_dispatch` Tool that the orchestrator's LLM uses to declare sub-queries (mirrors Phase 12 `write_todos` Tool audit pattern).
- `internal/agent/hitl/` is UNTOUCHED (Phase 11 closure). Sub-agents do NOT call HITL Tools (FAN-01 SC-1 says "compressed results, no HITL inside sub-agents").
- `internal/store/sqlite/audit.go` gets additive DDL (3 new nullable TEXT columns) + additive enum values (3 new `action_fanout_*` strings). No INSERT-only posture changes.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/cloudwego/eino/adk` | v0.9.14 (locked 2026-08-17) | `NewAgentTool` (`agent_tool.go:93`) — wraps a sub-Agent as a Tool for the orchestrator LLM; action scoping guarantees sub-agent cannot Exit/TransferToAgent/BreakLoop parent | Eino-blessed multi-agent pattern per supervisor.go docs ("Consider using ChatModelAgent with AgentTool ... instead") |
| `github.com/cloudwego/eino/adk` | v0.9.14 | `DispatchAgent` (Phase 12 `internal/runtime/factory.go`) — sub-agent construction via the unified factory | Reuse Phase 12 archetype dispatch (chat / deep) |
| `github.com/cloudwego/eino/adk` | v0.9.14 | `RunPath` (runContext.RunPath — `runctx.go:347`) — natural anchor for FAN-04 audit RunPath column | Already exposes the per-Run agent path |
| `github.com/cloudwego/eino/schema` | v0.9.14 | `RegisterName[T](name)` — register custom sub-agent result payload type for cross-process resume (D8-06 pattern) | Phase 8 SC-5 stub pattern proven; needed for `*FanOutSubResult` checkpoint round-trip |
| `golang.org/x/sync/errgroup` | latest (Go stdlib-adjacent) | `errgroup.Group{}` + `SetLimit(N)` for parallelism cap + panic recovery | Standard Go pattern; one stdlib + one Go-team-maintained module |
| `internal/runtime.ApplyTrustStage` | n/a (in-tree, Phase 12) | Per-sub-agent grounding via the unified exit (P12-L1) | Already hoisted from `internal/verdict` in Phase 12; FAN-04 reuse = zero new code path |
| `internal/runtime.DispatchAgent` | n/a (in-tree, Phase 12) | Per-Run sub-agent construction via unified factory | Phase 12 P12-L1 substrate; sub-agents instantiate through the same dispatcher |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/store/sqlite` | n/a (in-tree) | Audit table DDL extension (3 nullable TEXT columns: `run_path`, `transfer_from`, `transfer_to`); new enum values: `action_fanout_dispatched`, `action_fanout_completed`, `action_fanout_failed` | FAN-04 audit substrate |
| `internal/agent/callbacks` | n/a (in-tree, Phase 12) | `BudgetAuditStore` + `BudgetEmitter` — per-sub-agent budget middleware (smaller scope than parent's) | Each sub-agent gets `MaxIteration: 5` (half of parent's 10) |
| `internal/wire` | n/a (in-tree, Phase 3+) | D16 overload — extend `data-decision.decision` enum with `fanout_dispatched` / `fanout_completed` / `fanout_failed` (mirrors Phase 12 `forced_stop` overload) | SSE chunk for live fan-out trace |
| `internal/skill` | n/a (in-tree) | `ManifestBuilder.WithReads(intersection)` — NEW helper for tier-closure structural subset | FAN-02 tier inheritance |
| `internal/agent/ChunkAccumulator` | n/a (in-tree, Phase 3+) | Shared `*ChunkAccumulator` pointer — sub-agents append to parent's accumulator; citation whitelist spans parent + sub-agents | FAN-04 per-sub-agent grounding reuses parent's chunks map |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual `errgroup` fan-out (Option B) | `adk.NewParallelAgent` (Option A — `adk/workflow.go:698-705`) | `NewParallelAgent` is documented as "NOT RECOMMENDED ... built on agent transfer with full context sharing" — directly conflicts with FAN-02 tier-closure invariant. Sub-agent could read parent's `runSession.Values` via `adk.GetSessionValue(ctx, key)` (`runctx.go:233-241`); would require duplicating the closure at the `runSession.Values` level (architectural impedance mismatch). **Reject for FAN-02.** |
| Manual `errgroup` fan-out (Option B) | `adk.NewAgentTool` per sub-agent in a DeepAgent orchestrator | `AgentTool` is the Eino-blessed multi-agent pattern (supervisor.go:43), and DeepAgent has the auto-injected `task` tool for sub-agent invocation. BUT: (a) DeepAgent's `task` tool is auto-injected via `WithoutGeneralSubAgent=false`, which Phase 12 deliberately set to `true` (P12-L2 hardened against sub-agent injection T-P12-1); (b) `AgentTool` requires the LLM to drive the fan-out (parent LLM emits tool_call), but the chat archetype's tool whitelist is `writes`-gated — fan-out is NOT a write tool, but adding it to the LLM surface changes the prompt semantics. **Recommended: keep chat archetype unchanged; introduce a new `FanOutOrchestrator` archetype that owns the fan-out lifecycle, leaving chat/deep to operate on a single-vertex per-Run model** |
| Shared `*ChunkAccumulator` pointer | Per-sub-agent separate accumulator + merge at sub-agent output | Per-sub-agent separate accumulators → sub-agent grounding only sees sub-agent's chunks; if sub-agent's verdict cites a chunk retrieved by the PARENT (not itself), the citation is ungrounded. Per Phase 2 D2-01 invariant: "逐字引用验证在每个 agent 输出前" — citation MUST be grounded by the emitting agent's chunks. **Recommended: shared accumulator — but document the invariant in the API doc string** (sub-agent can only ground against the parent's seed chunks + its own retrieved chunks; the LLM schema must not let sub-agent cite chunks it never retrieved) |
| New `data-fanout-trace` SSE chunk discriminator | D16 overload — `data-decision{decision:"fanout_dispatched"}` | New discriminator = cleaner type safety, no enum growth on `data-decision`. Overload = fewer wire types + cheaper FE rendering (FE already routes `data-decision` for `forced_stop`). **Recommended: D16 overload** — matches Phase 12 P12-L3 precedent; 3 enum additions (`fanout_dispatched` / `fanout_completed` / `fanout_failed`) |
| Audit table ADD COLUMN (3 nullable TEXT columns) | New `fanout_events` table (separate, like plan_todos) | Add columns = same audit query path + retention redaction applies uniformly. New table = cleaner SELECT but needs JOIN for AuditTimelineTab. **Recommended: ADD COLUMN** — additive migration is cheap; Phase 6 retention redaction already touches all audit rows |

**Installation:**
```bash
go get golang.org/x/sync/errgroup@latest   # already in go.sum per Phase 11 transitive deps
# Verify: go list -m golang.org/x/sync
```

**Version verification:**
```bash
# Verified by direct module-cache inspection:
$ ls C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/workflow.go  # workflow.go:639-705 defines ParallelAgentConfig + NewParallelAgent
$ grep "NOT RECOMMENDED" C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/workflow.go  # supervisor.go:43-44 + workflow.go:630-714
$ grep "func NewAgentTool" C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/agent_tool.go  # line 93
$ grep "Action Scoping" C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/agent_tool.go  # line 86
$ grep "RegisterName" C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/schema/serialization.go  # line 83
$ grep "forkRunCtx" C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/runctx.go  # line 478
```

## Package Legitimacy Audit

> Phase 13 does not install new external packages. The only NEW external module is `golang.org/x/sync` (already a transitive dep of `modernc.org/sqlite` via Phase 3 store work — verify with `go list -m golang.org/x/sync`).

| Package | Registry | Age | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-------------|-----------|-------------|
| `github.com/cloudwego/eino@v0.9.14` | go.sum (locked) | Released 2026-08-13 (D8-01) | github.com/cloudwego/eino | [OK — Phase 8 verified] | Approved (no upgrade) |
| `golang.org/x/sync/errgroup` | go.sum (transitive) | Multi-year stable | github.com/golang/sync | [OK — Go team module] | Approved (transitive dep already present) |
| `internal/runtime` | n/a | n/a | in-tree | n/a | Approved (Phase 12 in production; new orchestrator.go extends) |
| `internal/agent` | n/a | n/a | in-tree | n/a | Approved (Phase 11-12 in production) |
| `internal/store/sqlite` | n/a | n/a | in-tree | n/a | Approved (Phase 3-12 in production) |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*slopcheck was not run because no new external packages are introduced — Phase 13 is pure extension against eino v0.9.14 + errgroup (transitive dep). The slopcheck protocol's "graceful degradation" applies: if slopcheck were unavailable, all packages above would be tagged `[OK]` based on direct module-cache + go.sum verification, NOT `[ASSUMED]`.*

## Architecture Patterns

### System Architecture Diagram

```
                                ┌─────────────────────────────────────┐
                                │  POST /api/chat  (query + skill_id) │
                                └──────────┬──────────────────────────┘
                                           │ chatHandler (Phase 11/12)
                                           ▼
   ┌──────────────────────────────────────────────────────────────────────┐
   │  FanOutOrchestrator  (NEW — internal/runtime/orchestrator.go)         │
   │   ┌─────────────────────────────────────────────────────────────┐    │
   │   │  Step 1: Classify (single LLM call)                          │    │
   │   │   input:  parent query + skill_manifest + resolved_tiers     │    │
   │   │   output: sub_queries[] = [{id, query, tier_filter}, ...]    │    │
   │   │   contract: tier_filter ⊆ parent.ResolvedTiers()             │    │
   │   └─────────────────────────────────────────────────────────────┘    │
   │   ┌─────────────────────────────────────────────────────────────┐    │
   │   │  Step 2: Construct sub-agents (per-Run)                      │    │
   │   │   for each sub_query in sub_queries:                         │    │
   │   │     subManifest = ManifestBuilder(parent.Manifest)           │    │
   │   │                     .WithReads(intersection)                  │    │
   │   │     → subAgent = runtime.DispatchAgent(ctx, {Manifest: subM..│  │
   │   │                              Tools: [retrieve_compliance..  │  │
   │   │                                     ]  // closes over subM..│  │
   │   │                              ChunkAccumulator: parent.acc})  │    │
   │   │     // tier-closure: subAgent's retrieve Tool is forced to    │    │
   │   │     //   query only subManifest.ResolvedTiers() = intersec..  │    │
   │   └─────────────────────────────────────────────────────────────┘    │
   │   ┌─────────────────────────────────────────────────────────────┐    │
   │   │  Step 3: Fan out (errgroup with SetLimit(4))                  │    │
   │   │   for each subAgent:                                         │    │
   │   │     go func() {                                              │    │
   │   │       defer recover()  // panic → sub_agent_status=faulted   │    │
   │   │       audit(action_fanout_dispatched, run_path, ...)         │    │
   │   │       msg := subAgent.Run(ctx, sub_query)                    │    │
   │   │       verdict := runtime.ApplyTrustStage(ctx, &TrustStageI...│  │
   │   │                              {Manifest: subManifest,        │    │
   │   │                               Output: msg,                  │    │
   │   │                               Chunks: parent.acc.Chunk...})  │    │
   │   │       result := compressedResult(sub_query.id, verdict)      │    │
   │   │       audit(action_fanout_completed, ...)                    │    │
   │   │       results <- result                                      │    │
   │   │     }()                                                     │    │
   │   └─────────────────────────────────────────────────────────────┘    │
   │   ┌─────────────────────────────────────────────────────────────┐    │
   │   │  Step 4: Merge (key-sorted pure function)                    │    │
   │   │   merged := mergeSubAgentResults(results)                   │    │
   │   │   // FAN-03: key sort by (sub_agent_id, chunk_id, span)      │    │
   │   │   // FAN-03: any result.Status=refused → merged.needs_input  │    │
   │   │   // 10× byte-identical contract test                        │    │
   │   └─────────────────────────────────────────────────────────────┘    │
   │   ┌─────────────────────────────────────────────────────────────┐    │
   │   │  Step 5: Apply Trust Stage to merged verdict                 │    │
   │   │   (unified exit — P12-L1 invariant; FAN-04 reuse)            │    │
   │   └─────────────────────────────────────────────────────────────┘    │
   └────────────────────┬─────────────────────────────────────────────────┘
                        │ emit merged verdict to SSE
                        ▼
   ┌─────────────────────────────────────────────────────────────────────┐
   │  chatHandler SSE  (Phase 11-12 wire)                                  │
   │   - data-decision{decision: "fanout_dispatched", run_path, ...}       │
   │   - data-decision{decision: "fanout_completed", run_path, ...}         │
   │   - data-decision{decision: "fanout_failed", run_path, ...}           │
   │   - existing data-verdict / data-policy / data-plan-trace chunks      │
   └─────────────────────────────────────────────────────────────────────┘
                        │ audit (action_fanout_* rows + run_path / transfer_* cols)
                        ▼
   ┌─────────────────────────────────────────────────────────────────────┐
   │  sqlite audit table  (Phase 3 schema + Phase 12 budget_layer + P13)   │
   │   - 3 new nullable TEXT columns: run_path, transfer_from, transfer_to│
   │   - 3 new action enum values: action_fanout_dispatched/completed/... │
   │   - retention redaction applies to all fanout rows (Phase 6 SC-5)     │
   └─────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── runtime/                                # EXTENDED — P8.4 orchestrator
│   ├── factory.go                          # P12-01 — UNCHANGED (DispatchAgent used by orchestrator)
│   ├── deep_agent.go                       # P12-01 — UNCHANGED (deep archetype)
│   ├── trust_stage.go                      # P12-01 — UNCHANGED (FAN-04 per-sub-agent grounding reuse)
│   ├── budget.go                           # P12-05 — UNCHANGED (BudgetConfig reused with smaller per-sub-agent scope)
│   ├── orchestrator.go                     # NEW — FanOutOrchestrator agent type + 5-step pipeline
│   ├── sub_agent_builder.go                # NEW — per-Run sub-agent construction with tier-closure
│   ├── merge.go                            # NEW — mergeSubAgentResults pure function (FAN-03)
│   ├── orchestrator_test.go                # NEW
│   └── merge_test.go                       # NEW (10× byte-identical contract test)
│
├── agent/
│   ├── callbacks/
│   │   └── budget.go                       # UNCHANGED (P12-05 reuse)
│   ├── fanout_tools.go                     # NEW — fan_out_dispatch Tool (orchestrator LLM-facing)
│   └── (Phase 11-12 files unchanged)
│
├── skill/
│   ├── manifest.go                         # MODIFIED — add ManifestBuilder.WithReads(intersection) helper
│   └── manifest_builder.go                 # NEW — fluent builder for tier-closure subset construction
│
├── store/sqlite/
│   ├── schema.go                           # MODIFIED — ADD COLUMN run_path TEXT, transfer_from TEXT, transfer_to TEXT
│   ├── audit.go                            # MODIFIED — extend QueryAudit to filter by run_path; add 3 fanout action enum constants
│   ├── audit.go (D16 overload)             # action_fanout_dispatched / _completed / _failed
│   └── fanout_events.go                    # NEW — Optional: separate query helper for fanout-specific audit queries
│
├── server/
│   ├── chat_handler.go                     # MODIFIED — wire orchestrator into the per-Run dispatch path
│   └── runner_setup.go                     # MODIFIED — call runtime.NewOrchestrator when manifest.Agent == "orchestrator"
│
└── wire/
    └── chunk.go                            # MODIFIED — D16 overload: data-decision.decision extends with fanout_* values
                                            #             (NO new SSE chunk type — see Alternatives table)

web/src/
├── types/wire-types.ts                     # MODIFIED — D15 lockstep mirror of decision enum extension
└── lib/real-chat-transport.ts              # MODIFIED — route fanout_* decisions into agent state inspector timeline (Phase 5 substrate)
```

### Pattern 1: Per-Run Sub-Agent Construction with Tier Closure (FAN-02)

**What:** Each sub-agent is a fresh `adk.Agent` constructed via `runtime.DispatchAgent(ctx, DispatchConfig{Manifest: subManifest, ...})`. The `subManifest` is a structural subset of the parent's manifest — `subManifest.Reads` = `parent.ResolvedTiers()` ∩ `sub_skill.Reads` (the intersection). Empty intersection = load-time error at sub-agent construction (NOT runtime).

**When to use:** Every sub-agent in the orchestrator. The parent's `ResolvedTiers()` is the closure's input; the sub-skill declares its own `Reads` field; the intersection is computed ONCE at construction time.

**Example (mirrors Phase 12 `runtime/factory.go:156` DispatchAgent pattern):**
```go
// Source: internal/runtime/sub_agent_builder.go (NEW, Phase 13 / Plan 13-01)
// Mirrors: internal/runtime/factory.go:104-115 (DispatchConfig shape)

package runtime

import (
    "context"
    "fmt"

    "github.com/skillforge/skillforge/internal/skill"
)

// SubAgentConfig is the per-Run construction envelope for a single
// sub-agent. Mirrors DispatchConfig (Phase 12 / runtime/factory.go:104)
// but adds the sub-agent-specific tier-closure fields.
type SubAgentConfig struct {
    ParentManifest   *skill.Manifest    // source of truth for ResolvedTiers()
    SubSkillManifest *skill.Manifest    // sub-skill's declared Reads (the requested tier filter)
    SubQueryID       string             // "sa1", "sa2", ... (unique per orchestrator Run)
    ChatModel        model.ToolCallingChatModel
    Embedder         embedder.Embedder
    Retriever        retriever.Retriever
    SessionID        string
    ChunkAccumulator *agent.ChunkAccumulator // shared with parent
    Instruction      string
}

// BuildSubAgent constructs the per-Run sub-agent with tier-closure
// inheritance. FAN-02 SC-2: empty intersection = load-time error.
func BuildSubAgent(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error) {
    if cfg.ParentManifest == nil || cfg.SubSkillManifest == nil {
        return nil, fmt.Errorf("runtime.BuildSubAgent: both manifests required")
    }

    // Tier-closure: structural intersection — the sub-agent's
    // ResolvedTiers is the SUBSET of parent's that the sub-skill
    // declares. Computed ONCE at construction; sub-agent's Tool
    // closure reads ONLY from subManifest.ResolvedTiers() (not
    // from any ctx value or system prompt).
    parentTiers := cfg.ParentManifest.ResolvedTiers()
    subRequested := cfg.SubSkillManifest.Reads
    if len(subRequested) == 0 {
        return nil, fmt.Errorf("runtime.BuildSubAgent: sub_skill %q has empty Reads (FAN-02 tier inheritance requires explicit declaration)",
            cfg.SubSkillManifest.Name)
    }
    intersection := intersectTiers(parentTiers, subRequested)
    if len(intersection) == 0 {
        // FAN-02 SC-2: empty intersection = load-time error, NOT runtime.
        return nil, fmt.Errorf("runtime.BuildSubAgent: tier intersection empty (parent=%v, sub_skill=%v) — load-time validation failure per FAN-02 SC-2",
            parentTiers, subRequested)
    }

    // Construct subManifest with the intersection baked into Reads.
    subManifest := skill.NewManifestBuilder(cfg.SubSkillManifest).
        WithReads(intersection).
        Build()
    // subManifest.ResolvedTiers() == intersection (structural — the
    // Loader's tier resolution is a pure function of Reads)

    // Build sub-agent via the Phase 12 unified dispatcher. The Tool
    // whitelist is built by BuildToolsForSkill using subManifest —
    // the retrieve Tool closes over subManifest.ResolvedTiers()
    // (= intersection), so the LLM cannot expand tiers via Tool args.
    subTools, err := agent.BuildToolsForSkill(ctx, subManifest, cfg.SessionID,
        cfg.SubSkillManifest.Name, hitl.NoopRuleMatcher{})
    if err != nil {
        return nil, fmt.Errorf("runtime.BuildSubAgent: BuildToolsForSkill: %w", err)
    }
    subRetrieveTool := agenttools.BuildRetrieveTool(cfg.Embedder, cfg.Retriever, subManifest)
    subTools = append(subTools, subRetrieveTool)

    return DispatchAgent(ctx, DispatchConfig{
        Manifest:           subManifest,
        ChatModel:          cfg.ChatModel,
        Embedder:           cfg.Embedder,
        Retriever:          cfg.Retriever,
        SessionID:          cfg.SessionID + ":" + cfg.SubQueryID, // sub-session ID for audit RunPath
        Tools:              subTools,
        Instruction:        cfg.Instruction,
        // NEW (Phase 13): shared ChunkAccumulator pointer
        ChunkAccumulator:   cfg.ChunkAccumulator,
    })
}

// intersectTiers returns the set intersection of two tier slices.
// Order-preserving on the LEFT operand (parent's tiers first).
// O(n*m) — fine for small N (4 tiers max).
func intersectTiers(parent, sub []string) []string {
    seen := make(map[string]bool, len(sub))
    for _, t := range sub {
        seen[t] = true
    }
    var out []string
    for _, t := range parent {
        if seen[t] {
            out = append(out, t)
        }
    }
    return out
}
```

### Pattern 2: Per-Sub-Agent Grounding via ApplyTrustStage (FAN-04)

**What:** Each sub-agent's final message flows through `runtime.ApplyTrustStage(ctx, &TrustStageInput{Manifest: subManifest, Output: subMsg, Chunks: sharedAccumulator.Hits()})` BEFORE the merge function sees it. The `Chunks` slice is the union of parent's seed retrieval + all sub-agent retrievals (read from the shared `*ChunkAccumulator`).

**When to use:** Step 3 of the orchestrator pipeline (every sub-agent's output). Reuses Phase 12 unified exit (P12-L1) — zero new code in `trust_stage.go`.

**Example (mirrors Phase 12 `runtime/trust_stage.go:144` ApplyTrustStage pattern):**
```go
// Source: internal/runtime/orchestrator.go (NEW, Phase 13 / Plan 13-02)
// Mirrors: internal/runtime/trust_stage.go:144-222

// runSubAgent executes ONE sub-agent with per-sub-agent grounding.
// Returns a SubAgentResult that the merge function consumes.
// FAN-04: every sub-agent output flows through ApplyTrustStage.
func (o *FanOutOrchestrator) runSubAgent(
    ctx context.Context,
    subAgent adk.Agent,
    subManifest *skill.Manifest,
    subQueryID string,
    subQuery string,
    sharedAcc *agent.ChunkAccumulator,
    auditStore *sqlite.SQLiteStore,
) SubAgentResult {
    defer func() {
        if r := recover(); r != nil {
            // Fault isolation — panic → sub_agent_status=faulted.
            // The errgroup fan-out catches this; the merge sees
            // faulted as refusal-equivalent.
            auditStore.PutAudit(ctx, store.AuditRow{
                Action:       sqlite.ActionFanoutFailed,
                RunPath:      parentRunPath + "/" + subQueryID,
                TransferFrom: parentRunID,
                TransferTo:   subQueryID,
                Reason:       fmt.Sprintf("panic: %v", r),
                Ts:           time.Now().UTC(),
            })
        }
    }()

    // (a) Fanout dispatched audit (FAN-04 RunPath + Transfer)
    auditStore.PutAudit(ctx, store.AuditRow{
        Action:       sqlite.ActionFanoutDispatched,
        RunPath:      parentRunPath + "/" + subQueryID,
        TransferFrom: parentRunID,
        TransferTo:   subQueryID,
        SubQuery:     subQuery,
    })

    // (b) Execute sub-agent. The Tool's closure reads from
    // sharedAcc (shared *ChunkAccumulator); sub-agent's retrieve
    // Tool appends hits to sharedAcc as it runs.
    subMsg, err := subAgent.Run(ctx, subQuery)
    if err != nil {
        // Audit + return as faulted (FAN-01 SC-1 fault isolation)
        auditStore.PutAudit(ctx, store.AuditRow{Action: sqlite.ActionFanoutFailed, ...})
        return SubAgentResult{ID: subQueryID, Status: "faulted", Reason: err.Error()}
    }

    // (c) Per-sub-agent grounding — reuse Phase 12 unified exit (FAN-04).
    // The shared *ChunkAccumulator gives ApplyTrustStage access to ALL
    // chunks retrieved in this Run (parent + sub-agents), so the
    // citation entailment check works against the union. D2-01
    // invariant: citation must appear verbatim in chunk text.
    trustOut, err := runtime.ApplyTrustStage(ctx, &runtime.TrustStageInput{
        Manifest: subManifest,
        Output:   subMsg,
        Chunks:   sharedAcc.Hits(), // SHARED with parent
        History:  nil,             // D9-V2: history doesn't influence refusal
    })
    if err != nil {
        return SubAgentResult{ID: subQueryID, Status: "faulted", Reason: err.Error()}
    }

    if trustOut.Refusal != nil {
        return SubAgentResult{ID: subQueryID, Status: "refused", Refusal: trustOut.Refusal}
    }

    // (d) Compress verdict to a sub-agent-friendly shape (FAN-01 SC-1).
    return SubAgentResult{
        ID:         subQueryID,
        Status:     "ok",
        Verdict:    trustOut.Verdict,
        Confidence: trustOut.Verdict.Confidence,
        Citations:  trustOut.Verdict.Citations,
    }
}
```

### Pattern 3: Deterministic Merge Pure Function (FAN-03)

**What:** `mergeSubAgentResults(results []SubAgentResult) MergedVerdict` is a PURE function with key-sorted output. Same input → byte-identical output (10× contract test). Conservative partial-failure: any `Status: refused` or `Status: faulted` → merged verdict is `needs_input` with confidence 0.

**When to use:** Step 4 of the orchestrator pipeline.

**Example:**
```go
// Source: internal/runtime/merge.go (NEW, Phase 13 / Plan 13-02)

// SubAgentResult is the per-sub-agent envelope consumed by the merge.
// JSON tag order MUST match the sort key order (FAN-03 SC-3 contract
// test pin).
type SubAgentResult struct {
    ID         string             `json:"id"`
    Status     string             `json:"status"` // ok | refused | faulted
    SubQuery   string             `json:"sub_query"`
    Verdict    *verdict.Verdict   `json:"verdict,omitempty"`
    Refusal    *verdict.Refusal   `json:"refusal,omitempty"`
    Confidence float64            `json:"confidence"`
    Citations  []verdict.Citation `json:"citations"`
}

// MergedVerdict is the post-merge envelope the orchestrator emits.
type MergedVerdict struct {
    Status         verdict.ComplianceStatus `json:"status"`
    Confidence     float64                  `json:"confidence"`
    Citations      []verdict.Citation       `json:"citations"`
    ReviewRequired bool                     `json:"review_required"`
    SubResults     []SubAgentResult         `json:"sub_results"` // sorted by ID
}

// mergeSubAgentResults is FAN-03 SC-3's load-bearing pure function.
// Key sort: (sub_agent_id ASC, citation_chunk_id ASC, citation_span ASC).
// Conservative partial-failure: any refused/faulted → needs_input + 0
// confidence. D2-14 posture (same as Phase 2 refusal-by-construction).
func mergeSubAgentResults(results []SubAgentResult) MergedVerdict {
    // Defensive copy + key sort (FAN-03 determinism).
    sorted := make([]SubAgentResult, len(results))
    copy(sorted, results)
    sort.SliceStable(sorted, func(i, j int) bool {
        return sorted[i].ID < sorted[j].ID
    })

    // Conservative partial-failure merge (D2-14).
    var refused []SubAgentResult
    var faulted []SubAgentResult
    var ok []SubAgentResult
    for _, r := range sorted {
        switch r.Status {
        case "refused":
            refused = append(refused, r)
        case "faulted":
            faulted = append(faulted, r)
        default:
            ok = append(ok, r)
        }
    }
    if len(refused) > 0 || len(faulted) > 0 {
        // FAN-03: any refused/faulted → needs_input + 0 confidence.
        return MergedVerdict{
            Status:         verdict.DowngradeStatus, // "needs_input"
            Confidence:     0,
            Citations:      mergeCitations(nil),     // empty
            ReviewRequired: true,
            SubResults:     sorted,                  // include for FE audit timeline
        }
    }

    // Merge OK results: union citations, dedup by (chunk_id, span),
    // first-wins (FAN-03 conflict resolution).
    seenCitation := make(map[string]bool)
    var mergedCitations []verdict.Citation
    for _, r := range ok {
        for _, c := range r.Citations {
            key := c.ChunkID + ":" + c.Span
            if !seenCitation[key] {
                seenCitation[key] = true
                mergedCitations = append(mergedCitations, c)
            }
        }
    }
    // Sort citations by (chunk_id, span) for byte-identity (FAN-03).
    sort.SliceStable(mergedCitations, func(i, j int) bool {
        if mergedCitations[i].ChunkID != mergedCitations[j].ChunkID {
            return mergedCitations[i].ChunkID < mergedCitations[j].ChunkID
        }
        return mergedCitations[i].Span < mergedCitations[j].Span
    })

    // Aggregate confidence: min across sub-agents (conservative).
    var minConf float64 = 1.0
    for _, r := range ok {
        if r.Confidence < minConf {
            minConf = r.Confidence
        }
    }

    return MergedVerdict{
        Status:         verdict.ConfirmedStatus, // "confirmed" — multi-source agreement
        Confidence:     minConf,
        Citations:      mergedCitations,
        ReviewRequired: minConf < 0.9, // D2-13 calibration
        SubResults:     sorted,
    }
}
```

### Anti-Patterns to Avoid

- **Anti-pattern: Using `adk.NewParallelAgent` for fan-out.** `adk/workflow.go:639-714` documents it as "NOT RECOMMENDED ... built on agent transfer with full context sharing" — directly conflicts with FAN-02 structural tier inheritance. Sub-agent could read parent's `runSession.Values` via `adk.GetSessionValue(ctx, key)` (`runctx.go:233-241`). **Use the new `FanOutOrchestrator` with manual `errgroup` instead.**
- **Anti-pattern: Per-sub-agent separate `ChunkAccumulator`.** If sub-agents have their own accumulators, the per-sub-agent grounding check only sees the sub-agent's chunks. A sub-agent verdict that cites a chunk retrieved by the PARENT (not itself) would be ungrounded → citation stripping. **Share the parent's accumulator pointer with all sub-agents** (Pattern 2 above).
- **Anti-pattern: Adding a new SSE chunk discriminator for fan-out.** The `data-decision` discriminator already exists + is the AI SDK v5 native routing path for orchestrator events (Phase 12 P12-L3 D16 overload precedent). **Overload `data-decision{decision:"fanout_dispatched|completed|failed"}` instead** — no D15 lockstep needed beyond enum extension.
- **Anti-pattern: Sub-agent calling HITL Tools (plan_approve, memory:facts, review_report).** FAN-01 SC-1 says "compressed results, no HITL inside sub-agents". Sub-agent's Tool whitelist excludes all HITL Tools (the `BuildToolsForSkill` pure function does NOT add them because subManifest.Writes is empty — only the parent's manifest has `plan_approve` in writes for the planning layer). **Sub-agents are read-only by structural construction.**
- **Anti-pattern: Map iteration in merge function.** Go map iteration is randomized; merge MUST use sorted slice iteration only. `sort.SliceStable` on sub-results + citations is required for FAN-03 10× byte-identical contract.
- **Anti-pattern: Trust stage called only on the merged verdict.** FAN-04 requires per-sub-agent grounding BEFORE merge. If the merge sees ungrounded sub-agent verdicts, the merged verdict is contaminated. **Each sub-agent's output is grounded independently** via `ApplyTrustStage` (Pattern 2).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Sub-agent construction | Custom `chatmodel.NewChatModelAgent` wrapper for sub-agents | `runtime.DispatchAgent(ctx, DispatchConfig{Manifest: subManifest, ...})` | Phase 12 unified dispatcher (P12-L1) handles all archetype dispatch; reusing it ensures sub-agents inherit the same tool whitelist + budget + trust staircase + audit substrate as the parent's chat/deep archetype |
| Tier intersection | Hand-coded `for _, p := range parent { for _, s := range sub { ... } }` | `skill.NewManifestBuilder(subManifest).WithReads(intersectTiers(parent.ResolvedTiers(), subManifest.Reads)).Build()` | Builder pattern enforces the Manifest invariants (`Reads` must be subset of declared tiers, validator must pass); hand-rolled slicing could produce a manifest the validator rejects |
| Per-sub-agent grounding | Custom grounding call inside the sub-agent runner | `runtime.ApplyTrustStage(ctx, &TrustStageInput{Manifest: subManifest, Output: subMsg, Chunks: sharedAcc.Hits()})` | Phase 12 unified exit (P12-L1) — same function used by parent chatHandler; D2-01 + D9-V2 invariants are enforced uniformly |
| Audit INSERT for fan-out events | Custom SQL in `internal/runtime/orchestrator.go` | `store.SQLiteStore.PutAudit(ctx, store.AuditRow{Action: sqlite.ActionFanoutDispatched, RunPath: ..., ...})` | T-3-AUDIT INSERT-only + T-3-SQLI `?`-placeholder invariants enforced uniformly; reuses Phase 11 audit_writer fail-closed posture |
| Parallel fan-out | Custom `sync.WaitGroup` + manual panic recovery | `errgroup.Group{}` + `SetLimit(4)` + `defer recover()` per goroutine | errgroup is the standard Go pattern for bounded parallel execution with error aggregation; manual `sync.WaitGroup` would lose the error-aggregation + panic-recovery boilerplate |
| Schema registration for sub-agent result | Custom serialization for cross-process resume | `schema.RegisterName[*SubAgentResult]("skillforge.SubAgentResult")` | Phase 8 SC-5 stub pattern proven (D8-06); bare gob registration fails Phase 8 TestNoBareGobRegistration; needed if sub-agent result is checkpointed (it isn't yet, but register preemptively for future interrupt support) |

**Key insight:** Phase 13 is a thin orchestration layer over Phase 11/12 substrate. The bulk of the code is REUSE — `DispatchAgent`, `ApplyTrustStage`, `BudgetConfig`, audit_writer, `BuildRetrieveTool` (with tier closure). The NEW surface is the **5-step pipeline** (`Classify → Build → Fan-out → Merge → TrustStage`) + the **3 audit columns** + the **3 enum values**. No new agent primitives are built.

## Runtime State Inventory

> Phase 13 adds 3 nullable TEXT columns to the audit table + 3 enum values. Audit grep finds files; runtime state audit is for **newly created objects during execution**.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | 3 new audit columns: `run_path TEXT`, `transfer_from TEXT`, `transfer_to TEXT` (all nullable, additive DDL) | Code edit (DDL ALTER TABLE) + new audit_writer fields; no data migration of existing rows |
| Live service config | N/A — no service config strings change | None |
| OS-registered state | N/A — Phase 13 doesn't register any OS-level resources | None |
| Secrets/env vars | N/A — no new env vars | None |
| Build artifacts | `cmd/server` binary re-linked (small change — new internal/runtime/orchestrator.go + internal/agent/fanout_tools.go) | Standard `go build ./...`; no special cleanup |

**Nothing found in category** (Live service config / OS-registered state / Secrets / Build artifacts): verified by direct inspection of `internal/server/*.go` + `cmd/server/main.go` (no env-name additions).

## Common Pitfalls

### Pitfall 1: NewParallelAgent Incompatible with FAN-02 (Reject Option A)

**What goes wrong:** Sub-agents built via `adk.NewParallelAgent` inherit the parent's `runSession.Values` map (per `forkRunCtx` at `runctx.go:478-508` + `runSession.addValue` at `runctx.go:324-328`). The sub-agent LLM can read any session value (including ones that encode tier hints, system prompt fragments, or future values). A prompt-injection attack on the sub-agent could expose the parent's session state.

**Why it happens:** Eino's parallel workflow agents share the parent's session for event ordering + resume support. This is a sensible default for most use cases — but for SkillForge's tier-closure invariant, sharing ANY state across the parent/sub boundary breaks the "structural" guarantee.

**How to avoid:** **Use Option B — manual `errgroup` fan-out with per-Run agent construction.** Each sub-agent is a fresh `adk.Agent` constructed via `runtime.DispatchAgent(ctx, DispatchConfig{Manifest: subManifest, ...})` where `subManifest.Reads` is the structural intersection computed AT CONSTRUCTION (not at runtime, not via ctx-value, not via prompt). The sub-agent's `retrieve_compliance_chunks` Tool closes over `subManifest.ResolvedTiers()` — physically cannot query outside.

**Warning signs:** Sub-agent's `internal/agent/tools/retrieve.go:53` `RetrieveToolName = "retrieve_compliance_chunks"` is the SAME Tool for both parent and sub-agent — only the closed-over manifest differs. If you see a path where the sub-agent uses the PARENT's manifest, FAN-02 is broken.

### Pitfall 2: Sub-Agent Tier Inheritance — Empty Intersection = Runtime, Not Load-Time

**What goes wrong:** Parent Skill declares `reads: [权威]`. Sub-skill declares `reads: [个人]`. The intersection is empty. If the orchestrator's classifier emits this combination, the sub-agent should FAIL FAST at construction. If the orchestrator silently drops the sub-agent or proceeds with an empty tier filter, the user sees zero chunks → trust floor refusal → empty response.

**Why it happens:** Naive implementations might defer tier validation to runtime (the sub-agent's retrieve Tool returns zero chunks → trust floor fires). But this masks the structural mismatch — a load-time error would have prevented the classifier from generating the bad sub-query in the first place.

**How to avoid:** `runtime.BuildSubAgent` returns `fmt.Errorf("tier intersection empty (parent=%v, sub_skill=%v) — load-time validation failure per FAN-02 SC-2", ...)` when intersection is empty. The orchestrator's classifier prompt explicitly enumerates the parent's `ResolvedTiers()` as the ONLY valid tier_filter values; an LLM-emitted invalid combination is rejected at the orchestrator's pre-fanout validation step (BEFORE sub-agent construction). **Two-layer defense: classifier prompt + load-time error.**

**Warning signs:** A live query with a sub-skill having non-overlapping reads returns an empty response with no obvious error; Phase 12 SC-5 log shows zero chunks retrieved; user reports "I asked a question and got nothing".

### Pitfall 3: Non-Deterministic Merge (Map Iteration Randomness)

**What goes wrong:** `mergeSubAgentResults` iterates over a `map[string]SubAgentResult` to build the merged citations. Go map iteration is randomized → the merged citation list varies across calls → the contract test `TestMergeSubAgentResultsByteIdentity` (10× same input → same output) fails.

**Why it happens:** Go's `range` over maps intentionally randomizes iteration order (runtime hash seed). The merge function must use SLICE iteration only.

**How to avoid:** Sort sub-results by `ID` (stable sort) + iterate the sorted slice. Sort citations by `(ChunkID, Span)` before returning the merged verdict. The FAN-03 contract test runs `mergeSubAgentResults` 10 times with the same input + asserts byte-identical JSON output via `json.Marshal` + `bytes.Equal`. **The test is the regression lock** — it would catch any accidental map iteration introduction.

**Warning signs:** Flaky test in CI: 9/10 runs pass, 1/10 fails; the failed run shows different citation order in the assertion message.

### Pitfall 4: Sub-Agent Fault Propagation Kills Parent Run

**What goes wrong:** A sub-agent panics (LLM timeout, OOM, retrieval backend error). The unhandled panic propagates up through the orchestrator's errgroup → parent Run is killed → user sees a 500 error with no partial result.

**Why it happens:** Without `defer recover()` per goroutine + `errgroup.SetLimit(N)`, a single sub-agent's panic tears down the whole fan-out. The parent Run has no idea that sub-agents are independent.

**How to avoid:** Each goroutine in the fan-out wraps with `defer func() { if r := recover(); r != nil { audit(faulted); result.Status = "faulted" } }()`. The merge function treats `faulted` as refusal-equivalent (FAN-03 conservative partial-failure). `errgroup.SetLimit(4)` caps parallelism so one slow sub-agent doesn't starve the others. **Sub-agent faults MUST NEVER propagate to parent.**

**Warning signs:** Live multi-hop query with one flaky sub-skill kills the entire response; the audit table shows zero fanout rows (panic before audit INSERT).

### Pitfall 5: Sub-Agent's `RetrieveTool` Re-Instantiated Per-Run (Tier Closure Drift)

**What goes wrong:** The sub-agent's `BuildRetrieveTool` is constructed with the PARENT's manifest (not the subManifest) due to a wiring mistake. The sub-agent's retrieve Tool closes over the parent's ResolvedTiers → FAN-02 structural tier inheritance is broken silently.

**Why it happens:** `internal/agent/tools/retrieve.go:100-102` documents that the Tool closes over `m *skill.Manifest`. If the orchestrator passes `cfg.ParentManifest` instead of `cfg.SubManifest`, the closure is wrong. Easy to make this mistake in the wiring code.

**How to avoid:** **Compile-time type safety via the `SubAgentConfig` shape** — `SubAgentConfig` has BOTH `ParentManifest` and `SubSkillManifest` fields. The `BuildSubAgent` function uses ONLY `SubSkillManifest` (after the intersection bake) when constructing the retrieve Tool. **Add a regression test `TestSubAgentRetrieveToolClosesOverSubManifest`** that constructs a parent with `Reads: [权威]` + sub-skill with `Reads: [个人]` → asserts the sub-agent's retrieve Tool's `manifest` field is the subManifest, NOT the parentManifest.

**Warning signs:** Sub-agent retrieves chunks from a tier outside the sub-skill's declared reads; audit table shows the sub-agent's `chunk_ids` includes tiers the sub-skill didn't declare.

### Pitfall 6: Sub-Agent Exceeds Budget (Cost Explosion Recursive)

**What goes wrong:** Each sub-agent runs `MaxIteration: 5` + `TokenBudget: 20000`. Three sub-agents running in parallel = `MaxIteration: 15` total + `TokenBudget: 60000` total. The parent's budget is `MaxIteration: 10` + `TokenBudget: 50000`. A multi-hop query with 3 sub-agents can cost 6× the parent's budget ceiling.

**Why it happens:** Sub-agents are scoped INDEPENDENTLY but share the parent's accumulator + LLM pool. The LLM cost is multiplicative; the budget is per-vertex, not per-Run.

**How to avoid:** The orchestrator's pre-fanout budget check sums the projected sub-agent budgets + the parent's remaining budget; if the total exceeds the Run's hard cap, the orchestrator emits `data-decision{decision:"fanout_rejected", reason:"budget would exceed Run cap"}` + returns a refusal (NOT a partial merge). **The cap is structural** (constant in `internal/runtime/orchestrator.go`), NOT config-driven.

**Warning signs:** Live multi-hop query on compliance Skill costs $0.30 (vs $0.05 for single-vertex); GLM usage log shows 6 sub-agent LLM calls per query.

### Pitfall 7: Sub-Agent HITL Tool Accidentally Whitelisted

**What goes wrong:** The sub-skill's manifest has `writes: [plan_approve, memory:facts]` (inheriting from a parent that had them). The orchestrator's `BuildToolsForSkill(ctx, subManifest, ...)` returns the HITL Tools as part of the sub-agent's whitelist. The sub-agent calls `plan_approve` → interrupts → user is asked to approve a sub-agent's plan → sub-agent's Tool is intended for compressed results, NOT interactive approval (FAN-01 SC-1).

**Why it happens:** `BuildToolsForSkill` is the Phase 11 per-turn Tool whitelist pure function. It intersects `manifest.Writes` with `AllowedWriteTargets`. If the subManifest has writes that match HITL targets, the whitelist includes them.

**How to avoid:** **`runtime.BuildSubAgent` explicitly sets `subManifest.Writes = []string{}`** before calling `BuildToolsForSkill`. Sub-agents are read-only by structural construction. The regression test `TestSubAgentIsReadOnly` asserts: (a) sub-agent's Tool whitelist contains NO HITL Tools, (b) sub-agent's Info() schema does not include write tool schemas.

**Warning signs:** Sub-agent emits `tool_call{function.name: "plan_approve"}`; audit table shows `action_pending_approval` rows with `sub_query` = the sub-agent's compressed query (suggests sub-agent invoked HITL).

### Pitfall 8: Wire-Type Drift (D15 Violation on data-decision Enum Extension)

**What goes wrong:** `data-decision.decision` is extended with `fanout_dispatched` / `fanout_completed` / `fanout_failed` in Go `internal/wire/chunk.go` but the TS mirror `web/src/types/wire-types.ts` is not updated → `AssertAssignable<SkillForgeChunk, UIMessageChunk>` fails (D15 alarm).

**Why it happens:** D15 two-PR lockstep rule (Phase 4) requires Go + TS changes in the same commit / PR.

**How to avoid:** **Every plan that extends the `data-decision.decision` enum emits a paired Go + TS commit** with the same `D15:` prefix in the commit message. Phase 12 P12-L6 precedent + Phase 11 P11-D15: enforced via grep gate in 13-04 (hermetic gate).

**Warning signs:** `npx tsc --noEmit` fails in the same commit that extends `data-decision.decision` in `internal/wire/chunk.go`.

## Code Examples

Verified patterns from official sources:

### Example 1: NewAgentTool Action Scoping (Verified)

```go
// Source: github.com/cloudwego/eino@v0.9.14/adk/agent_tool.go:84-92 (verified by direct module cache inspection)
// Phase 13 uses AgentTool ONLY for the DeepAgent orchestrator path;
// for the chat archetype's manual fan-out, the orchestrator builds
// the sub-agent Tool list directly.

// "Action Scoping:
//  Actions emitted by the inner agent are scoped to the agent tool boundary:
//   - Interrupted: Propagated via CompositeInterrupt to allow proper interrupt/resume across boundaries
//   - Exit, TransferToAgent, BreakLoop: Ignored outside the agent tool; these actions only affect
//     the inner agent's execution and do not propagate to the parent agent"
//
// Phase 13 design implication: AgentTool IS safe for the orchestrator's
// sub-agent composition IF the orchestrator is a DeepAgent. The chat
// archetype's manual fan-out via errgroup is the cleaner path for
// MVP because it doesn't require re-introducing the deep archetype's
// auto-injected `task` tool (which Phase 12 deliberately disabled via
// WithoutGeneralSubAgent=true).
```

### Example 2: forkRunCtx Lane Session (Verified)

```go
// Source: github.com/cloudwego/eino@v0.9.14/adk/runctx.go:478-508 (verified)
// This is the eino-internal mechanism for parallel workflows.
// Phase 13 does NOT use this directly (we're using errgroup + manual
// dispatch), but the pattern validates that parallel sub-agents each
// get their own lane-local session — confirming that Option A
// (NewParallelAgent) would share values via the parent's
// runSession.Values map (the very behavior we want to AVOID for
// FAN-02 tier inheritance).
//
// "Create a new session for the child lane by manually copying the parent's session fields.
//  This is crucial to ensure a new mutex is created and that the LaneEvents pointer is unique.
//  childSession := &runSession{
//      Events:    parentRunCtx.Session.Events, // Share the committed history
//      Values:    parentRunCtx.Session.Values, // Share the values map
//      valuesMtx: parentRunCtx.Session.valuesMtx,
//  }"
//
// KEY OBSERVATION (lines 489): Values are SHARED. This is the eino
// behavior we cannot use for tier inheritance — sub-agents would
// read parent's session values.
```

### Example 3: ApplyTrustStage Reuse for Per-Sub-Agent Grounding

```go
// Source: internal/runtime/trust_stage.go:144-222 (Phase 12 P12-L1)
// Phase 13 FAN-04 uses this function unchanged — every sub-agent's
// final message is grounded via ApplyTrustStage BEFORE merge.
//
// Key invariant (D2-01): citation's Span must appear verbatim in
// ChunkID's chunk text (after Normalize). The shared
// *ChunkAccumulator gives ApplyTrustStage access to ALL chunks
// retrieved in this Run (parent + sub-agents).
```

### Example 4: Tier Closure SubManifest Builder

```go
// Source: internal/skill/manifest_builder.go (NEW, Phase 13 / Plan 13-01)
// Mirrors Phase 11 skill.Manifest shape but adds fluent builder API.

// NewManifestBuilder returns a fluent builder seeded with base.
func NewManifestBuilder(base *Manifest) *ManifestBuilder {
    return &ManifestBuilder{base: base.Clone()}
}

// WithReads sets the Reads slice to the structural intersection.
// MUST be a subset of base.ResolvedTiers() — the builder validates
// at Build() time and returns an error if not.
func (b *ManifestBuilder) WithReads(intersection []string) *ManifestBuilder {
    b.base.Reads = intersection
    return b
}

// Build validates the manifest against the 7-field schema (SYSK-02)
// and returns the typed result. Empty intersection → validation
// error (FAN-02 SC-2).
func (b *ManifestBuilder) Build() (*Manifest, error) {
    if len(b.base.Reads) == 0 {
        return nil, errors.New("skill.ManifestBuilder: empty Reads (FAN-02 tier inheritance requires non-empty intersection)")
    }
    if err := Validate(b.base); err != nil {
        return nil, fmt.Errorf("skill.ManifestBuilder: Validate: %w", err)
    }
    return b.base, nil
}
```

### Example 5: D16 Overload on data-decision

```go
// Source: internal/wire/chunk.go (existing DecisionPayload pattern from Phase 3 03-01)
// Phase 13 extends the decision enum with 3 fan-out values.

type DecisionPayload struct {
    Step      int     `json:"step"`
    Decision  string  `json:"decision"`
    Reason    string  `json:"reason,omitempty"`
    Confidence float64 `json:"confidence,omitempty"`
    RunPath   string  `json:"run_path,omitempty"`         // NEW: eino runctx.RunPath (runctx.go:347)
    TransferFrom string `json:"transfer_from,omitempty"`  // NEW: sub-agent invocation source
    TransferTo   string `json:"transfer_to,omitempty"`    // NEW: sub-agent invocation target
}

// New enum values (mirrors Phase 12 forced_stop overload):
//   "fanout_dispatched"
//   "fanout_completed"
//   "fanout_failed"
//
// Existing values retained: "retrieve" | "answer" | "refuse" |
// "forced_stop" (Phase 12) | "plan_approved" | "plan_denied" (Phase 12).
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 12 DeepAgent `WithoutGeneralSubAgent: true` (no sub-agent invocation) | Phase 13 introduces `FanOutOrchestrator` agent type that explicitly constructs sub-agents per-Run with tier-closure | eino v0.9.14 DeepAgent + Phase 13 design | Sub-agent composition is now EXPLICIT (orchestrator owns the lifecycle) not IMPLICIT (DeepAgent auto-injects the `task` tool) |
| `adk.NewParallelAgent` (full context sharing) | Manual `errgroup` fan-out with `runtime.DispatchAgent` per sub-agent | Phase 13 design | FAN-02 structural tier inheritance is now possible (sub-agent's manifest is independent from parent's) |
| Phase 11 `audit.action` enum (pending/approved/rejected + plan_todo_* + budget_triggered) | Phase 13 adds `action_fanout_dispatched | action_fanout_completed | action_fanout_failed` | P8.4 | Mirrors HITL-04 + PLAN-04 enum extension patterns; reuses T-3-AUDIT INSERT-only guarantee |
| Phase 12 `data-decision` discriminator (10 events: retrieve/answer/refuse/forced_stop/plan_*) | Phase 13 D16 overload adds `fanout_dispatched | fanout_completed | fanout_failed` | P8.4 | Reuses D15 lockstep; no new wire chunk type |
| Per-archetype Tool whitelist (D11-B1) | Sub-agent Tool whitelist with EMPTY writes (FAN-01 SC-1: read-only by construction) | P8.4 | Sub-agents cannot invoke HITL Tools (plan_approve/memory:facts/review_report) |
| Phase 3 `*agent.ChunkAccumulator` (per-Run accumulator) | Shared `*ChunkAccumulator` pointer across parent + sub-agents | P8.4 | Citation whitelist spans parent + sub-agents; per-sub-agent grounding (FAN-04) works against the union |
| Phase 12 `BudgetConfig` per-vertex | Sub-agent `BudgetConfig{MaxIteration: 5, TokenBudget: 20000}` (half of parent's 10/50000) | P8.4 | One slow sub-agent cannot drain the parent's budget; per-sub-agent cost is bounded |

**Deprecated/outdated:**
- **`adk.NewParallelAgent` for tier-inheritance use cases**: Eino docs explicitly say NOT RECOMMENDED + the "full context sharing" semantics are incompatible with FAN-02 structural inheritance. Phase 13 uses manual `errgroup` fan-out instead.
- **DeepAgent's auto-injected `task` tool (via `WithoutGeneralSubAgent: false`)**: Phase 12 deliberately disabled it (`WithoutGeneralSubAgent: true`). Phase 13 keeps this disabled + introduces explicit orchestrator. Re-enabling would require sub-agent grounding + tier-closure + budget scoping to be handled implicitly by DeepAgent — far more error-prone than the explicit orchestrator.
- **Sub-agent HITL Tool capability**: FAN-01 SC-1 explicitly excludes HITL inside sub-agents. `BuildSubAgent` zeroes out `subManifest.Writes` to enforce this structurally.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `adk.NewParallelAgent` shares parent's `runSession.Values` with sub-agents (per `forkRunCtx` at `runctx.go:478-508`) | Pitfall 1 | HIGH — if eino doesn't share Values, Option A (NewParallelAgent) would be viable for FAN-02; but the verified `forkRunCtx` code explicitly shares. **Verified by direct inspection of eino@v0.9.14/adk/runctx.go:489** |
| A2 | `runtime.ApplyTrustStage` (Phase 12 P12-L1) is reusable for per-sub-agent grounding without modification | Pattern 2 | LOW — function signature is `(ctx, *TrustStageInput) (*TrustStageOutput, error)`; passing `subManifest` + `subMsg` + `sharedAcc.Hits()` works without any internal change |
| A3 | `*agent.ChunkAccumulator` exists in the codebase with a `Hits() []retriever.Hit` accessor | Pattern 2 | MEDIUM — needs verification in `internal/agent/agent.go` (Phase 3 substrate); if accessor is named differently or returns `map[int64]Hit`, the orchestrator's per-sub-agent grounding call needs a thin wrapper |
| A4 | `skill.Manifest.ResolvedTiers()` is a method on the existing Manifest type | Pattern 1 | HIGH — must verify the exact signature; if it's a function `ResolveTiers(m)` instead of a method, the builder pattern needs adjustment |
| A5 | Phase 11 `BuildToolsForSkill(ctx, manifest, sessionID, skillID, ruleMatcher)` signature is stable | Pattern 1 + Pitfall 7 | LOW — Phase 11 acceptance gate (11-08) froze the signature; Phase 12 reused it for chat + deep archetypes |
| A6 | `internal/store.AuditRow` has all fields needed for fan-out events (RunPath / TransferFrom / TransferTo) without DDL change | Pitfall 8 + Audit substrate | HIGH — if the existing AuditRow struct doesn't have these fields, ADD COLUMN DDL is required (acceptable, planned) |
| A7 | `golang.org/x/sync/errgroup` is already a transitive dep (verified in go.sum from Phase 3 sqlite work) | Standard Stack | LOW — `go list -m golang.org/x/sync` confirms transitive presence; Phase 13 may need explicit `go get` if used at the top-level |
| A8 | Vercel AI SDK v5 routes `data-decision{decision:"fanout_*"}` chunks through the existing transport | Pattern 5 | LOW — Phase 11 + Phase 12 already route `data-decision` chunks (forced_stop, plan_approved, plan_denied); the FE agent state inspector timeline can display fanout events without new wire type |
| A9 | Sub-agent's LLM cost is bounded at MaxIteration=5 + TokenBudget=20000 (half of parent's) | Pitfall 6 | MEDIUM — calibration via Phase 13 acceptance gate; sub-agent cost depends on the sub-query complexity + the chunk-count + the LLM model. 20K tokens ≈ 4 LLM calls with 5K context each |
| A10 | Phase 9 escape plan (D9-V2) does not apply to sub-agent grounding — sub-agents see no History | Pattern 2 | LOW — `runtime.ApplyTrustStage` already handles `History: nil` correctly per D9-V2; the orchestrator passes nil for sub-agents |

**If this table is empty:** It is NOT empty — see A1-A10 above. Items A1 + A3 + A4 + A6 are HIGH priority for plan-level spike verification (RF-4 closes; structural tier-inheritance verification is a regression test).

## Open Questions

1. **RF-4 (HIGHEST UNCERTAINTY): Does `runtime.ApplyTrustStage` + `runtime.DispatchAgent` + shared `*ChunkAccumulator` work as designed for sub-agent per-Run construction?**
   - What we know: All three are Phase 11/12 in production; the signature is stable; the closure pattern is documented.
   - What's unclear: Whether sub-agent's `RetrieveTool` (with `subManifest` closure) actually gets called by Eino's ChatModelAgent in the way we expect. End-to-end smoke test needed.
   - Recommendation: P13-01 spike (TDD — write `TestBuildSubAgentTierInheritance` + `TestSubAgentRetrieveToolClosesOverSubManifest` BEFORE implementation; let the tests drive the design).

2. **Sub-agent LLM cost calibration**
   - What we know: Phase 12 budget defaults are `MaxIteration: 10`, `TokenBudget: 50000`. Sub-agent is half: 5 + 20000.
   - What's unclear: Whether 5 iterations + 20K tokens is enough for typical sub-queries (compliance chunk retrieval + verdict parsing + grounding).
   - Recommendation: P13-02 acceptance gate measures 20 cases; if any sub-agent trips the budget, escalate to 8 + 30K (still below parent's 10 + 50K).

3. **Classifier LLM call cost**
   - What we know: The orchestrator's pre-fanout classifier is ONE LLM call (returns `{"sub_queries": [...]}`).
   - What's unclear: Whether the classifier call is worth the latency cost for SIMPLE queries that don't benefit from fan-out. A single LLM call ≈ 1-2 seconds + $0.01.
   - Recommendation: P13-01 decision — for v0.8 MVP, ALWAYS run the classifier (the cost is bounded + the value of multi-hop on ambiguous queries is high). Future optimization: heuristic short-circuit (e.g., single-keyword query → skip classifier).

4. **RunPath audit column data type (TEXT vs JSON)**
   - What we know: Eino's `runctx.RunPath` is `[]RunStep{agentName: "..."}` — a list.
   - What's unclear: Whether to store as slash-separated TEXT (`parent_run_id/sa1/sa2`) or JSON array (`["parent", "sa1", "sa2"]`).
   - Recommendation: TEXT slash-separated — simpler query (`LIKE 'parent_run_id/%'`) + smaller storage + sufficient for AuditTimelineTab rendering.

5. **Sub-agent timeout**
   - What we know: Each sub-agent runs in its own goroutine via `errgroup.SetLimit(4)`. No per-sub-agent timeout currently.
   - What's unclear: Whether a single slow sub-agent (e.g., retrieval backend stall) can starve the parent's overall response time.
   - Recommendation: P13-02 adds `context.WithTimeout(ctx, 30*time.Second)` per sub-agent goroutine. Timeout → audit `action_fanout_failed` + sub-agent marked `faulted` (same as panic). Parent's overall latency cap = 30s + overhead.

6. **Sub-agent's chunk accumulator growth**
   - What we know: Each sub-agent's retrieve calls append to the shared accumulator. N sub-agents + 5 chunks each = N×5 chunks in the union.
   - What's unclear: Whether this explodes memory / token budget for downstream sub-agents (sub-agent 2's context includes sub-agent 1's retrieved chunks).
   - Recommendation: P13-02 limits shared accumulator to 20 chunks max (LRU eviction). Sub-agent 2 sees only the top-20 chunks across all retrievals.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.23 | All BE code | ✓ | 1.23+ (per go.mod) | — |
| eino v0.9.14 | DispatchAgent + ApplyTrustStage + AgentTool | ✓ (locked Phase 8) | v0.9.14 | — |
| eino-ext openai v0.1.13 | ChatModel (GLM-5.2 via OpenAI-compat) | ✓ (locked Phase 8) | v0.1.13 | — |
| SQLite (modernc.org v1.56.0) | Audit table ADD COLUMN DDL + extended QueryAudit | ✓ | v1.56.0 | — |
| golang.org/x/sync/errgroup | Parallel fan-out | ✓ (transitive dep of sqlite; verify with `go list -m`) | latest | — |
| Vercel AI SDK v5 | FE agent state inspector timeline (Phase 5 substrate) | ✓ (locked Phase 4) | v5 | — |
| React 19 + Vite 8 + Tailwind v4 + shadcn | FE — minimal (only D15 mirror + transport parse update) | ✓ (locked Phase 4-5) | 19.2.x | — |
| GLM-5.2 (live) | Live spike + acceptance gate | ✗ (execution env) | — | `.env` + `docker compose up` (Phase 7 pattern) — operator gates |

**Missing dependencies with no fallback:**
- None blocking — all in-process deps are present.

**Missing dependencies with fallback:**
- GLM-5.2 live API key + Milvus + BGE-M3 (live stack) — falls back to hermetic-only verification per Phase 7-11 pattern; acceptance gate = operator-runs-live.

## Validation Architecture

> workflow.nyquist_validation is enabled (default — Phase 4 acceptance gate evidence). Required.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | go test (stdlib) + testify/assert + vitest (web) |
| Config file | go.mod (BE), vitest.config (FE) |
| Quick run command | `go test ./internal/runtime/... ./internal/agent/... ./internal/store/... ./internal/skill/... -count=1` (BE) + `cd web && npx vitest run` (FE) |
| Full suite command | `go test ./... -count=1 -tags=contract` + `cd web && npx vitest run` + `npx tsc --noEmit` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FAN-01 | Multi-hop triggers parallel fan-out; sub-agent returns compressed result; sub-agent faults isolated | unit + integration | `go test ./internal/runtime/... -run TestFanOutOrchestrator -v` | ❌ Wave 0 |
| FAN-01 | Sub-agent never calls HITL Tool (read-only structural) | unit | `go test ./internal/runtime/... -run TestSubAgentIsReadOnly -v` | ❌ Wave 0 |
| FAN-01 | Sub-agent panic → other sub-agents continue; merge sees faulted | unit | `go test ./internal/runtime/... -run TestSubAgentFaultIsolation -v` | ❌ Wave 0 |
| FAN-02 | Sub-agent's retrieve Tool closes over subManifest.ResolvedTiers() (NOT parent's) | unit | `go test ./internal/runtime/... -run TestSubAgentRetrieveToolClosesOverSubManifest -v` | ❌ Wave 0 |
| FAN-02 | Empty tier intersection → load-time error (NOT runtime) | unit | `go test ./internal/runtime/... -run TestSubAgentTierInheritance -v` | ❌ Wave 0 |
| FAN-02 | Sub-agent cannot query parent's unauthorized tier (structural, not prompt) | unit | `go test ./internal/runtime/... -run TestSubAgentCannotExpandTierBoundary -v` | ❌ Wave 0 |
| FAN-03 | 10× byte-identical merge (key sort pure function) | unit | `go test ./internal/runtime/... -run TestMergeSubAgentResultsByteIdentity -v` | ❌ Wave 0 |
| FAN-03 | Any refused/faulted → merged verdict needs_input (D2-14) | unit | `go test ./internal/runtime/... -run TestMergeConservativePartialFailure -v` | ❌ Wave 0 |
| FAN-03 | Same chunk cited by 2 sub-agents → dedup first-wins | unit | `go test ./internal/runtime/... -run TestMergeCitationDedup -v` | ❌ Wave 0 |
| FAN-04 | Per-sub-agent grounding reuses ApplyTrustStage (citation entailment per sub-agent) | unit | `go test ./internal/runtime/... -run TestSubAgentGroundingReuse -v` | ❌ Wave 0 |
| FAN-04 | Shared ChunkAccumulator across parent + sub-agents | unit | `go test ./internal/runtime/... -run TestSharedChunkAccumulator -v` | ❌ Wave 0 |
| FAN-04 | Fan-out event audit rows (action_fanout_dispatched/completed/failed) | unit | `go test ./internal/store/sqlite/... -run TestFanoutAuditRows -v` | ❌ Wave 0 |
| FAN-04 | RunPath + TransferFrom + TransferTo columns populated | unit | `go test ./internal/store/sqlite/... -run TestFanoutRunPathTransferColumns -v` | ❌ Wave 0 |
| D15 | TS mirror in sync with Go wire (data-decision fanout_* values) | compile-time | `cd web && npx tsc --noEmit` | ❌ Wave 0 |
| D8-04 | No legacy interrupt API (WalkDir extended to internal/runtime/orchestrator.go) | unit (WalkDir) | `go test ./... -run TestNoLegacyInterruptAPI -v` | ✅ Phase 8 (extended) |
| D11-B1 | Per-turn Tool whitelist — sub-agent excludes HITL Tools | unit | `go test ./internal/runtime/... -run TestSubAgentToolWhitelistExcludesHITL -v` | ❌ Wave 0 |
| P12-L1 | ApplyTrustStage exit point — sub-agents funnel through (regression lock) | unit | `go test ./internal/runtime/... -run TestSubAgentAppliesTrustStage -v` | ❌ Wave 0 |
| P12-L2 | MaxIteration ≤10 parent + ≤5 sub-agent (hardcoded literals) | unit (source-grep) | `go test ./internal/runtime/... -run TestSubAgentMaxIterationHardCap -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/runtime/... ./internal/agent/... ./internal/store/... ./internal/skill/... ./internal/wire/... -count=1` + `cd web && npx vitest run`
- **Per wave merge:** Full suite above + `go test ./... -tags=contract` + `npx tsc --noEmit`
- **Phase gate:** Full suite green + live fan-out 20-case UAT + frozen eval re-lock (v0.8 milestone re-baseline including fan-out behaviors)

### Wave 0 Gaps
- [ ] `internal/runtime/orchestrator.go` + `orchestrator_test.go` — FanOutOrchestrator agent type
- [ ] `internal/runtime/sub_agent_builder.go` + `sub_agent_builder_test.go` — per-Run sub-agent construction with tier-closure
- [ ] `internal/runtime/merge.go` + `merge_test.go` — merge pure function (FAN-03 10× byte-identical)
- [ ] `internal/agent/fanout_tools.go` + `fanout_tools_test.go` — fan_out_dispatch Tool (orchestrator LLM-facing)
- [ ] `internal/agent/ChunkAccumulator.Hits()` accessor — verify or add (Phase 3 substrate)
- [ ] `internal/skill/manifest_builder.go` + `manifest_builder_test.go` — fluent ManifestBuilder.WithReads(intersection)
- [ ] `internal/skill/manifest.go` — verify Manifest.ResolvedTiers() method signature (Phase 3 substrate)
- [ ] `internal/store/sqlite/schema.go` — ADD COLUMN run_path TEXT, transfer_from TEXT, transfer_to TEXT
- [ ] `internal/store/sqlite/audit.go` — extend QueryAudit with RunPath filter + 3 fanout action enum constants
- [ ] `internal/store/sqlite/audit.go` — AuditRow struct adds RunPath/TransferFrom/TransferTo fields (nullable)
- [ ] `internal/server/chat_handler.go` — wire orchestrator into per-Run dispatch (NEW archetype: "orchestrator")
- [ ] `internal/wire/chunk.go` — D16 overload: data-decision.decision extends with fanout_* values
- [ ] `web/src/types/wire-types.ts` — D15 lockstep mirror of decision enum extension
- [ ] `web/src/lib/real-chat-transport.ts` — route fanout_* decisions into agent state inspector timeline
- [ ] `examples/skills/compliance-orchestrator/SKILL.md` — `agent: orchestrator` showcase Skill
- [ ] `cmd/spike-fanout/main.go` + `main_test.go` — RF-4 spike binary (sub-agent smoke test)
- [ ] `eval/golden/fanout.jsonl` — 20-case golden set for fan-out UAT

## Security Domain

> `security_enforcement` is enabled (default — Phase 11 STRIDE-11 + Phase 12 STRIDE-12 threat models). Required.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | N/A (P13 doesn't introduce auth changes) |
| V3 Session Management | yes | Per-session Agent + Runner (D11-B4); sub-agents share parent's sessionID + add sub-suffix |
| V4 Access Control | yes | FAN-02 tier-closure structural inheritance (sub-agent's manifest.Reads = intersection); `BuildSubAgent` zeros `subManifest.Writes` (read-only by construction) |
| V5 Input Validation | yes | Classifier LLM call validates sub-query IDs unique + tier_filter ⊆ parent.ResolvedTiers(); Load-time tier intersection check rejects empty intersection (FAN-02 SC-2); audit INSERT only via audited PutAudit (T-3-AUDIT parameterized queries) |
| V6 Cryptography | yes | N/A (P13 doesn't introduce crypto) |
| V7 Error Handling | yes | Sub-agent panic → `defer recover()` → `sub_agent_status=faulted` (Pitfall 4); merge treats faulted as refusal-equivalent; timeout via `context.WithTimeout(30s)` |
| V8 Data Protection | yes | Fan-out audit rows redacted after retention window (Phase 6 SC-5); `run_path` / `transfer_*` columns carry session-level metadata (no PII beyond sessionID) |

### Known Threat Patterns for Eino ADK Fan-Out

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Sub-agent tier boundary expansion (tier leak) | Tampering / Elevation of Privilege | FAN-02 structural tier-closure via `BuildSubAgent` (Pitfall 1 + 5); `TestSubAgentRetrieveToolClosesOverSubManifest` regression lock; `TestSubAgentCannotExpandTierBoundary` integration test |
| Sub-agent HITL Tool access | Tampering / Elevation of Privilege | FAN-01 SC-1 read-only-by-construction (`subManifest.Writes = []`); `TestSubAgentIsReadOnly` regression lock |
| Sub-agent panic kills parent | Denial of Service | Pitfall 4 — `defer recover()` + `errgroup.SetLimit(4)`; `TestSubAgentFaultIsolation` regression lock |
| Non-deterministic merge (audit replay drift) | Repudiation | FAN-03 key-sorted pure function; `TestMergeSubAgentResultsByteIdentity` 10× regression lock |
| Classifier prompt injection → invalid tier_filter | Tampering | Two-layer defense: classifier prompt enumerates valid tiers + `BuildSubAgent` rejects empty intersection at load time |
| Audit row spoofing (claim fanout_completed when actually faulted) | Repudiation | `TestFanoutAuditRows` regression lock + `TestFanoutRunPathTransferColumns` integration test |
| Wire-type drift (data-decision enum extension missing from TS) | Tampering | D15 lockstep gate in 13-04 hermetic phase; `npx tsc --noEmit` must exit 0 |

### Plan-Specific Threats (P13 only)

| Threat | Mitigation |
|--------|-----------|
| T-P13-1: Tier intersection mismatch allows sub-agent to query parent's unauthorized tier | FAN-02 SC-2 load-time error (Pitfall 2) — `BuildSubAgent` returns error on empty intersection; classifier prompt restricts valid tier_filter values; `TestSubAgentTierInheritance` regression lock |
| T-P13-2: Sub-agent's Tool whitelist inherits HITL Tools from parent Skill | FAN-01 SC-1 — `BuildSubAgent` zeros `subManifest.Writes`; `TestSubAgentToolWhitelistExcludesHITL` regression lock |
| T-P13-3: Faulty sub-agent (LLM timeout, retrieval backend stall) blocks parent's overall response | Pitfall 4 — `context.WithTimeout(30s)` per sub-agent + `defer recover()` + errgroup parallelism cap; `TestSubAgentFaultIsolation` regression lock |
| T-P13-4: Merge function non-determinism (map iteration) breaks audit replay | FAN-03 SC-3 — key-sorted pure function; `TestMergeSubAgentResultsByteIdentity` 10× regression lock |
| T-P13-5: Classifier LLM call cost explosion (always run, even for trivial queries) | P13-02 budget check sums projected sub-agent budgets; reject fan-out if total exceeds Run cap; `TestFanOutBudgetRejection` regression lock |
| T-P13-6: Sub-agent shared ChunkAccumulator grows unbounded (memory + token budget) | P13-02 — LRU cap at 20 chunks; `TestSharedChunkAccumulatorCap` regression lock |
| T-P13-7: Sub-agent's Tool closure accidentally binds parent manifest (typo in wiring) | Pitfall 5 — `TestSubAgentRetrieveToolClosesOverSubManifest` regression lock; compile-time type safety via `SubAgentConfig` shape |
| T-P13-8: NewParallelAgent accidentally used instead of manual errgroup (full context sharing breaks FAN-02) | Static analysis: `TestNoNewParallelAgentUsage` grep gate (similar to D8-04 TestNoLegacyInterruptAPI); package-level comment in `orchestrator.go` warns explicitly |

## Sources

### Primary (HIGH confidence)
- eino@v0.9.14 module cache (`C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/`) — direct file inspection:
  - `adk/workflow.go:639-714` — `ParallelAgentConfig` + `NewSequentialAgent` + `NewParallelAgent` + `NewLoopAgent` (verified NOT RECOMMENDED docs verbatim)
  - `adk/prebuilt/supervisor/supervisor.go:33-120` — supervisor pattern (verified NOT RECOMMENDED docs verbatim + AgentTool recommendation)
  - `adk/agent_tool.go:84-92` — Action Scoping (Exit/TransferToAgent/BreakLoop ignored outside agent tool boundary; Interrupted propagated via CompositeInterrupt)
  - `adk/agent_tool.go:93` — `NewAgentTool(ctx, agent, options...)` signature
  - `adk/runctx.go:213-241` — `AddSessionValue` + `GetSessionValue` (sub-agent can read parent's session values if shared)
  - `adk/runctx.go:478-508` — `forkRunCtx` (parallel workflows share parent's `runSession.Values`)
  - `adk/runctx.go:347` — `runContext.RunPath` (natural anchor for FAN-04 audit)
  - `schema/serialization.go:83` — `RegisterName[T any](name string)` (cross-process resume pattern)
- Phase 12 substrate (HIGH confidence — Phase 12 acceptance gate passed):
  - `internal/runtime/factory.go:104-179` — `DispatchConfig` + `DispatchAgent` (P12-L1 unified dispatch)
  - `internal/runtime/deep_agent.go:65-113` — `BuildDeepAgent` (P12-L2 MaxIteration: 10 hardcoded literal; P12 WithoutGeneralSubAgent: true)
  - `internal/runtime/trust_stage.go:144-222` — `ApplyTrustStage` (P12-L1 unified exit; FAN-04 reuse)
  - `internal/runtime/budget.go:142-153` — `NewBudgetCallbacks` (P12-L9 loop detection; FAN-13 per-sub-agent budget scope)
  - `internal/agent/deep_audit.go:208-270` — `writeTodosAuditedTool` 3-effect contract (audit INSERT + SSE emit + session value; FAN-13 audit_writer extension pattern)
  - `internal/agent/hitl/registry.go:69-101` — `AllowedWriteTargets` + `hitlHandlers` (D11-B3 single source of truth; FAN-13 doesn't add write targets)
  - `internal/store/sqlite/audit.go:419-454` — `ActionPendingApproval` + `ActionApproved` + `ActionRejected` + `ActionPlanTodoUpsert` + `ActionPlanTodoComplete` + `ActionBudgetTriggered` (FAN-13 adds 3 fanout enum values)
  - `internal/store/sqlite/plan_todos.go:131-153` — `PutPlanTodo` INSERT-only fail-closed (FAN-13 audit_writer extension pattern)

### Secondary (MEDIUM confidence)
- D15 wire-type compile alarm (Phase 4 D-04 — verified pattern; FAN-13 D16 overload uses same mechanism)
- Phase 11 STRIDE-11 + Phase 12 STRIDE-12 threat models (8 Critical + 4 High threats; FAN-13 inherits the same STRIDE process)
- Vercel AI SDK v5 `data-decision` chunk routing (Phase 4-12 verified in D-04 transport-swap invariant; FAN-13 D16 overload on `fanout_*` follows the same path)

### Tertiary (LOW confidence)
- RF-4 highest-uncertainty (Eino sub-agent ctx-value propagation) — **pending spike verification in P13-01** (mitigation: manual errgroup + per-Run agent construction bakes tier closure into Tool closures; sub-agent cannot read parent's session values because sub-agent is constructed fresh, not shared)
- Sub-agent LLM cost calibration (MaxIteration: 5 + TokenBudget: 20000) — **pending acceptance gate measurement** in P13-02 (initial values per A9 assumption)
- Sub-agent timeout value (30s) — **pending acceptance gate tuning** in P13-02
- Classifier LLM call cost-vs-value trade-off — **pending P13-01 design decision** (initial: ALWAYS run; future: heuristic short-circuit)
- `*agent.ChunkAccumulator.Hits()` accessor existence — **pending verification** in `internal/agent/agent.go` (A3 assumption)

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — verified directly against eino@v0.9.14 module cache; no new external packages
- Architecture: MEDIUM-HIGH — Phase 12 P12-L1 substrate is locked + verified; Phase 13 NEW surface (orchestrator + sub-agent builder + merge) is design-level HIGH but P13-01 spike will TDD-verify the wiring
- Pitfalls: MEDIUM-HIGH — 8 pitfalls catalogued; 4 are direct extensions of known Phase 11/12 patterns (fault isolation, HITL whitelist, wire-type drift, audit INSERT-only); 4 are Phase 13-specific (Pitfall 1 ParallelAgent rejection, Pitfall 2 empty intersection, Pitfall 3 map iteration, Pitfall 5 sub-manifest wiring)
- Validation Architecture: HIGH — Phase 11 + 12 acceptance gate patterns transfer directly; 17 NEW tests in Wave 0 gap list; all hermetic except live fan-out 20-case UAT (operator gate, same pattern as Phase 7/9/11)
- RF-4 Mitigation: MEDIUM-HIGH — recommended Option B (manual errgroup + tier-closure) has HIGH design confidence but LOW empirical confidence (no live eino sub-agent examples); P13-01 spike is the verification mechanism

**Research date:** 2026-08-21
**Valid until:** 30 days (Phase 13 plans should be written by 2026-09-20)
