# Phase 12: 规划层（P8.3 — DeepAgent + write_todos + 可回放） - Research

**Researched:** 2026-08-20
**Domain:** Eino v0.9.14 DeepAgent (`adk/prebuilt/deep`) + write_todos middleware + 三层预算门 + 规划轨迹审计回放
**Confidence:** MEDIUM-HIGH (DeepAgent API verified against eino@v0.9.14 module cache; GLM-5.2 verdict-JSON parse rate is RF-5 LOW pending live measurement; apply-trust-stage refactor = HIGH)

## Summary

Phase 12 (P8.3) is the "planning layer" of the v0.8 Harness milestone: it adds an opt-in `agent: deep` archetype to SkillForge that gives a Skill **structured plan tracking** (write_todos tool → live todo stream), a **plan confirmation gate** (复用 Phase 11 HITL approval infrastructure), **three budget gates** (MaxIteration ≤10 + run-level token budget + loop detection), and an **auditable plan replay** (write_todos audit rows → UI 轨迹回放视图). The substrate is eino v0.9.14's `adk/prebuilt/deep.DeepAgent`, verified directly against the module cache at `C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/prebuilt/deep/`. DeepAgent returns an `adk.ResumableAgent` (implements `adk.TypedResumableAgent[*schema.Message]`) that fits the Phase 11 Runner pattern with no API churn beyond the `MaxIterations`/`MaxIteration` field rename. The trust floor / classifier / grounding ladder (D11-A2) must be hoisted from `internal/verdict` into `internal/runtime` so all archetypes (chat / deep / future sub-agent) flow through the same `ApplyTrustStage` exit (PLAN-01). Five new locks: `internal/runtime/{factory,deep_agent,trust_stage,budget}.go` + wire `data-plan-trace` chunk discriminator + sqlite `plan_todos` table (or extended audit table). RF-5 (GLM-5.2 verdict JSON parse rate on DeepAgent final-message) gates the design: the fallback = structured-output final step with `forced_stop` on JSON parse failure, mirrored from the Phase 3 emitForcedStop pattern.

**Primary recommendation:** Phase 12 is a vertical slice in the Phase 11 mold — **8 plans in 5 waves** (Wave 1: BE DeepAgent factory + trust-stage hoisting + budget gate; Wave 2: BE write_todos audit table + plan-approve HITL Tool + SkillManifest.Agent dispatch; Wave 3: BE plan confirm gate endpoint + loop-detect; Wave 4: FE PlanTracePart streaming + PlanApprovalCard + 轨迹回放视图; Wave 5: acceptance gate + frozen eval re-lock).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `internal/runtime` archetype factory dispatch (chat / deep / sub) | API / Backend | — | Single source of truth for `Manifest.Agent` → Eino archetype mapping (PLAN-01 contract); all archetypes funnel through `ApplyTrustStage` exit |
| `ApplyTrustStage` (floor→grounding→critique) exit point | API / Backend | — | Hoist from `internal/verdict` to `internal/runtime`; all archetypes must call it before verdict returns to chatHandler (PLAN-01 SC-1 regression lock) |
| DeepAgent construction (`deep.New`) | API / Backend | — | Lives in `internal/runtime/deep_agent.go`; closes over manifest + per-session store + retriever |
| write_todos tool (built-in via DeepAgent middleware) | API / Backend | — | Eino provides `write_todos` automatically when `WithoutWriteTodos=false` (default); SkillForge wraps it with audit emission + session-persistence (PLAN-02, PLAN-05) |
| Three budget gates (MaxIteration / Token / Loop) | API / Backend | — | `MaxIteration ≤10` via `adk/prebuilt/deep.Config.MaxIteration`; run-level token via ChatModel token counter accumulated in BeforeModelCallback; loop detection via rolling hash of last 3 tool calls |
| Plan confirmation gate | API / Backend | Browser / Client | Reuses Phase 11 HITL Tool pattern (`hitl_plan_approve.go`); FE renders plan preview card before execution begins (PLAN-03) |
| PlanTracePart SSE streaming | CDN / Static | API / Backend | New wire type `data-plan-trace` (mirrors `data-decision`); emitted by write_todos middleware's audit hook; consumed by FE PlanTracePart renderer (PLAN-02) |
| 规划轨迹回放视图 | Browser / Client | Database / Storage | FE fetches `GET /api/run/:id/plan` (PLAN-05 replay query); sqlite stores plan_todos audit rows for replay |
| Plan persistence (cross-refresh) | Database / Storage | API / Backend | New sqlite table `plan_todos` OR extended audit table with `event_type='plan_todo'` discriminator; queryable by run_id |

**Key boundary:** `internal/runtime` is the new package that owns archetype dispatch + `ApplyTrustStage` exit. `internal/agent` keeps ChatModelAgent construction (Phase 11) + HITL Tools (Phase 11) + callbacks (Phase 11 D11-A2). `internal/verdict` is downgraded to a library (floor, grounding, critique primitives) consumed by `internal/runtime.ApplyTrustStage`.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/cloudwego/eino/adk/prebuilt/deep` | v0.9.14 (locked 2026-08-17) | DeepAgent archetype (`adk/prebuilt/deep`) — prebuilt orchestration with write_todos, task tool, sub-agents | GA in v0.5.0; returns `adk.ResumableAgent` compatible with Phase 11 Runner path |
| `github.com/cloudwego/eino/adk` | v0.9.14 | Runner, ChatModelAgent, AsyncIterator events, CheckPointStore (`internal/core`) | Already in use (Phase 8-11) |
| `github.com/cloudwego/eino/components/tool/utils.InferTool` | v0.9.14 | Helper to wrap Go function as `tool.InvokableTool` with JSON schema inference (used by write_todos wrap for audit emission) | Built-in to eino; preferred over hand-rolled Tool impl |
| `github.com/cloudwego/eino/schema` | v0.9.14 | `schema.RegisterName[T]` for plan_todos custom payload (cross-process checkpoint round-trip) | Already used (D8-06 Phase 8 register pattern) |
| React 19 + Vercel AI SDK v5 | 19.2.x / v5 | PlanTracePart rendering + 规划轨迹回放视图 (UI hint yes per ROADMAP) | Locked since Phase 4 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/store/sqlite` | n/a (in-tree) | Phase 11 implementation of `compose.CheckPointStore` + `audit` table — Phase 12 adds `plan_todos` table OR extends `audit.action` enum with `plan_todo_*` values | Run-state persistence + audit replay query (PLAN-05) |
| `internal/agent/hitl/registry.go` | n/a (in-tree) | Phase 11's `AllowedWriteTargets` + `hitlHandlers` map — Phase 12 adds `plan_approve` target + `hitl_plan_approve.go` Tool file (D11-B3 single-source-of-truth) | Plan confirmation gate (PLAN-03) — reuse HITL Tool pattern + ApprovalDecision schema verbatim |
| `internal/wire` | n/a (in-tree) | Phase 3-11 wire types — Phase 12 adds `data-plan-trace` discriminator + `PlanTracePayload` struct (D15 lockstep) | SSE chunk for live todo streaming (PLAN-02) |
| `internal/skill` | n/a (in-tree) | Manifest + Validate — Phase 12 adds `deep` to `validAgentTypes` whitelist + `WithoutWriteTodos=false`/optional Tool pattern | `agent: deep` in SKILL.md |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `deep.New` (DeepAgent) | `react.Agent` + custom write_todos Tool | DeepAgent already ships write_todos + task tool + sub-agent slots; building it on react.Agent duplicates ~200 LOC and loses the `SessionKeyTodos` integration |
| `deep.New` | `planexecute.New` (PlanExecute) | PlanExecute is a planner/executor/replanner pattern (sequential); DeepAgent matches the "single agent with planning overlay" use case for SkillForge |
| New `data-plan-trace` SSE type | Reuse `data-decision` with new Decision string | D15 wire-type alarm requires new discriminator for type-safety; `data-decision.Step=0` already overloaded with 4 history events (D10-B4) — adding `plan_todo_*` would explode the enum; new discriminator is cleaner |
| Separate `plan_todos` table | Extend `audit` table with `action='plan_todo_*'` enum | Trade-off: separate table = clean PLAN-05 replay query (just `SELECT * FROM plan_todos WHERE run_id=? ORDER BY ts`), extended table = uniform audit query. **Recommended: separate table** — replay query is the hot path; audit uniformity is nicer for the operator but replay latency matters more (replay is per-render in the 轨迹回放视图) |
| Plan confirmation gate as separate Tool | Plan confirmation as a separate pre-execute interrupt (Runner-level, not Tool-level) | Phase 11 explicitly bound approval to Tool-level interrupts (D11-A2 architectural boundary — `tool.StatefulInterrupt` only callable from Tool fn); reusing the Tool pattern keeps the Runner migration intact. **Recommended: HITL Tool `plan_approve` with target=`plan_approve` in AllowedWriteTargets** |

**Installation:** No new packages — all Phase 12 dependencies are already on eino v0.9.14 + eino-ext openai v0.1.13 + sqlite v1.56.0 (locked Phase 8). The new `internal/runtime` package is in-tree.

**Version verification:**
```bash
# Verified at C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/adk/prebuilt/deep/deep.go
$ head -1 adk/prebuilt/deep/deep.go   # 2025 CloudWeGo
$ grep "adk/prebuilt/deep" C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/go.mod  # (not in go.mod, but in package source)
```

## Package Legitimacy Audit

> No new external packages installed this phase. All code is in-tree against eino v0.9.14 + eino-ext openai v0.1.13 (Phase 8 locked versions).

| Package | Registry | Age | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-------------|-----------|-------------|
| `github.com/cloudwego/eino@v0.9.14` | go.sum (locked) | Released 2026-08-13 (D8-01) | github.com/cloudwego/eino | [OK — Phase 8 verified] | Approved (no upgrade) |
| `github.com/cloudwego/eino/adk/prebuilt/deep` | (in v0.9.14) | (in v0.9.14) | github.com/cloudwego/eino | [OK — verified by direct file inspection] | Approved |
| `internal/store/sqlite` | n/a | n/a | in-tree (SkillForge) | n/a | Approved (Phase 11 in production) |
| `internal/agent/hitl` | n/a | n/a | in-tree | n/a | Approved (Phase 11 acceptance gate PASS) |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*slopcheck was not run because no new external packages are introduced this phase — the only new code paths are in `internal/runtime/*` and `internal/store/sqlite/*`, which consume the already-locked eino v0.9.14 + eino-ext openai v0.1.13.*

## Architecture Patterns

### System Architecture Diagram

```
                                ┌─────────────────────────────────────┐
                                │  SKILL.md  agent: deep              │
                                │  SKILL.md  agent: chat (existing)   │
                                └──────────┬──────────────────────────┘
                                           │ Loader.Scan → Manifest.Agent
                                           ▼
   ┌──────────────────────────┐    ┌─────────────────────────────────────┐
   │  internal/runtime        │◄───┤  factory.dispatchAgent(ctx, manifest)│
   │   factory.go             │    │   - chat  → NewChatModelAgent (P11) │
   │   (new)                  │    │   - deep  → deep.New + PlanApprove │
   └──────────┬───────────────┘    │   - future: ParallelAgent/Sequential│
              │                    └─────────────┬───────────────────────┘
              │                                  │
              ▼                                  ▼
   ┌──────────────────────────┐    ┌─────────────────────────────────────┐
   │  ApplyTrustStage (exit)  │◄───┤ ALL archetypes funnel here         │
   │  floor→grounding→critique│    │   (PLAN-01 SC-1)                   │
   │  (hoisted from verdict)  │    └─────────────┬───────────────────────┘
   └──────────┬───────────────┘                  │
              │                                  │
              ▼                                  ▼
   ┌──────────────────────────┐    ┌─────────────────────────────────────┐
   │  Budget Gate (forced_stop)│    │  adk.Runner.Run(ctx, msgs)         │
   │   - MaxIteration ≤10     │    │   with CheckPointStore +            │
   │   - Run-level tokens     │    │   emit → SSE                       │
   │   - Loop detection       │    └─────────────┬───────────────────────┘
   └──────────────────────────┘                  │
                                                ▼
   ┌─────────────────────────────────────────────────────────────────────┐
   │  DeepAgent (deep.New)                                               │
   │   ┌─────────────────────────────────────────────────────────────┐  │
   │   │  write_todos middleware (auto-injected)                      │  │
   │   │   - each tool call → audit row (PLAN-05)                     │  │
   │   │   - each tool call → data-plan-trace SSE chunk (PLAN-02)     │  │
   │   └─────────────────────────────────────────────────────────────┘  │
   │   ┌─────────────────────────────────────────────────────────────┐  │
   │   │  task tool (sub-agent spawner, unused in v0.8 P8.3)          │  │
   │   └─────────────────────────────────────────────────────────────┘  │
   │   ┌─────────────────────────────────────────────────────────────┐  │
   │   │  hitl_plan_approve Tool (PLAN-03, 复用 HITL Tool pattern)   │  │
   │   │   - interrupt before plan execution                          │  │
   │   │   - user approves/denies/rewrites                            │  │
   │   └─────────────────────────────────────────────────────────────┘  │
   │   ┌─────────────────────────────────────────────────────────────┐  │
   │   │  retrieve_compliance_chunks (per-tier closure, KNOW-03)      │  │
   │   └─────────────────────────────────────────────────────────────┘  │
   └────────────────────────────┬────────────────────────────────────────┘
                                │
                                ▼
   ┌─────────────────────────────────────────────────────────────────────┐
   │  Plan confirm interrupt                                             │
   │   - SSE: data-decision(forced_stop) + finish-message                │
   │   - sqlite checkpoint persisted                                     │
   │   - FE renders PlanApprovalCard                                     │
   │   - POST /api/run/:id/plan/approve  →  ResumeWithParams             │
   └─────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── runtime/                                # NEW — P8.3 archetype factory + trust stage exit
│   ├── factory.go                          # dispatchAgent(manifest) → ResumableAgent
│   ├── deep_agent.go                       # wraps deep.New + write_todos audit hook
│   ├── trust_stage.go                      # ApplyTrustStage(floor→grounding→critique) exit
│   ├── budget.go                           # MaxIteration + TokenBudget + LoopDetect callbacks
│   └── factory_test.go
├── agent/
│   ├── callbacks/
│   │   ├── budget.go                       # NEW — P12 token-budget + loop-detect middleware
│   │   ├── trust_floor.go                  # MOVED from trust_floor.go (D11-A2 floor)
│   │   └── classifier.go                   # D10-A1 (unchanged)
│   ├── hitl/
│   │   ├── registry.go                     # MODIFIED — add plan_approve to AllowedWriteTargets
│   │   ├── hitl_plan_approve.go            # NEW — Tool + interrupt for plan gate
│   │   └── audit_writer.go                 # MODIFIED — extend to emit plan_approve lifecycle rows
│   ├── deep_audit.go                       # NEW — wraps write_todos middleware with audit + SSE emission
│   └── (P11 files unchanged)
├── verdict/
│   ├── grounding.go                        # LIBRARY ONLY (consumed by runtime.ApplyTrustStage)
│   └── (no longer self-applies — D11-A2/PLAN-01 hoisted)
├── store/sqlite/
│   ├── plan_todos.go                       # NEW — INSERT-only audit table for write_todos replays
│   └── audit.go                            # MODIFIED — extended QueryAudit to JOIN plan_todos for /api/run/:id/plan
├── server/
│   ├── plan_api.go                         # NEW — GET /api/run/:id/plan, POST /api/run/:id/plan/approve
│   └── runner_setup.go                     # MODIFIED — call factory.dispatchAgent instead of inline NewChatModelAgent
└── wire/
    └── chunk.go                            # MODIFIED — add "data-plan-trace" to ValidTypes + PlanTracePayload struct
web/src/
├── components/
│   ├── PlanTracePart.tsx                   # NEW — live todo stream renderer (mirrors DataSubprogressPart pattern)
│   ├── PlanApprovalCard.tsx                # NEW — pre-execute plan confirmation (mirrors ApprovalCard)
│   └── PlanReplayView.tsx                  # NEW — 规划轨迹回放视图 (timeline of todos)
├── pages/
│   └── RunReplay.tsx                       # NEW — dedicated replay page (or CenterPanel tab)
└── types/wire-types.ts                     # MODIFIED — add data-plan-trace variant (D15 lockstep)
```

### Pattern 1: Eino v0.9.14 DeepAgent Construction

**What:** The `deep.New` constructor returns `adk.ResumableAgent` (which is `TypedResumableAgent[*schema.Message]`) — fully compatible with Phase 11's `adk.NewRunner(ctx, RunnerConfig{Agent: ...})` wiring. The default `WithoutWriteTodos=false` automatically registers the `write_todos` tool as a middleware that calls `adk.AddSessionValue(ctx, SessionKeyTodos, input.Todos)`.

**When to use:** Any Skill declaring `agent: deep` in SKILL.md. The factory (`internal/runtime/factory.go`) dispatches on `Manifest.Agent` — `chat` → existing Phase 11 ChatModelAgent path, `deep` → new DeepAgent path.

**Example (verified against eino@v0.9.14/adk/prebuilt/deep/deep.go:171-173):**
```go
// Source: github.com/cloudwego/eino@v0.9.14/adk/prebuilt/deep/deep.go:110-173
// (verified by direct module cache inspection)
import "github.com/cloudwego/eino/adk/prebuilt/deep"

agent, err := deep.New(ctx, &deep.Config{
    Name:        "skillforge-deep-" + sessionID,
    Description: "SkillForge compliance-review DeepAgent (P8.3 planning layer)",
    ChatModel:   cfg.ChatModel,
    Instruction: SystemPrompt(cfg.Manifest),  // render Manifest → system prompt
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}, // retrieve + plan_approve + write_todos (auto)
    },
    MaxIteration: 10,  // PLAN-04 budget gate (Pitfall 8)
    // WithoutWriteTodos=false is default → write_todos auto-injected
    // WithoutGeneralSubAgent=true — sub-agent not used in P8.3 (Phase 13 territory)
    Handlers: []adk.TypedChatModelAgentMiddleware[*schema.Message]{
        deepBudgetCallback(cfg.RunID),  // PLAN-04 token + loop detect
    },
})
// Returns adk.ResumableAgent — same type Phase 11 Runner accepts.
return adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true, CheckPointStore: store})
```

**Reference:** `eino@v0.9.14/adk/prebuilt/deep/deep.go:35-38` registers schema names via `schema.RegisterName[TODO]("_eino_adk_prebuilt_deep_todo")` — important for cross-process checkpoint round-trip (PLAN-05 replay).

### Pattern 2: write_todos Tool Auto-Injection

**What:** DeepAgent's `buildTypedBuiltinAgentMiddlewares` (deep.go:209-217) automatically creates a `write_todos` middleware via `typedNewWriteTodos[M]` (deep.go:244-267) when `WithoutWriteTodos=false`. The tool calls `adk.AddSessionValue(ctx, SessionKeyTodos, input.Todos)` — SkillForge needs to also (a) audit-log the call (PLAN-05) and (b) emit a `data-plan-trace` SSE chunk (PLAN-02).

**When to use:** Wrap the auto-injected write_todos middleware in `internal/agent/deep_audit.go` — intercept `BeforeAgent`/`AfterToolCalls` to emit audit + SSE side effects.

**Example (verified against deep.go:244-267):**
```go
// Source: github.com/cloudwego/eino@v0.9.14/adk/prebuilt/deep/deep.go:244-267
type TODO struct {
    Content    string `json:"content"`
    ActiveForm string `json:"activeForm"`
    Status     string `json:"status" jsonschema:"enum=pending,enum=in_progress,enum=completed"`
}
type writeTodosArguments struct {
    Todos []TODO `json:"todos"`
}

// Default write_todos auto-registers as middleware in buildTypedBuiltinAgentMiddlewares.
// To add audit + SSE emission, SkillForge wraps the DeepAgent with a custom ChatModelAgentMiddleware
// that intercepts the tool calls (BeforeToolCall / AfterToolCall) and emits the audit + SSE.
```

**Caveat:** The auto-injected write_todos is registered as a `TypedChatModelAgentMiddleware[M]` via `typedBuildAppendPromptTool` (deep.go:44-65). SkillForge cannot easily intercept the tool call's args from the outside (the middleware only sees `ChatModelAgentContext` with `Tools []tool.BaseTool`). **Two options:**
- **Option A (recommended):** Wrap `DeepAgent` inside a custom `adk.AgentTool` adapter — but this adds a wrapping layer for what should be a side-effect hook.
- **Option B (cleaner):** Use a `Handlers []TypedChatModelAgentMiddleware[M]` interceptor in `deep.New` config — the middleware wraps the model and sees tool calls. This is the eino v0.9+ pattern (chatmodel.go:325-326 explicitly documents this).
- **Option C (most direct):** Set `WithoutWriteTodos=true` and register our own `write_todos` Tool with audit + SSE hooks. This trades the auto-injection for full control — and is the cleanest path for audit + SSE emission.

**Recommended: Option C.** The audit + SSE emission is load-bearing for PLAN-02/PLAN-05 — losing the auto-injection's convenience is worth the explicit hook.

### Pattern 3: HITL Tool Approval Pattern (Reuse for Plan Approval Gate)

**What:** Phase 11 established the canonical HITL Tool pattern: `tool.StatefulInterrupt(ctx, &ApprovalInfo{...}, &ApprovalDecision{...})` (Phase 11 D11-A2 + D11-B2). The same pattern works for `hitl_plan_approve.go` — the plan is presented as `frozen_args` (the full plan JSON), the user approves/denies, and `ApprovalDecision.Decision == "approve"` causes ResumeWithParams to continue execution; `"deny"` triggers agent rewrite.

**When to use:** PLAN-03 plan confirmation gate.

**Example (mirrors Phase 11 hitl_memory_facts.go:71-78):**
```go
// Package hitl — internal/agent/hitl/hitl_plan_approve.go (NEW, P8.3)
func (t *planApproveTool) InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error) {
    var args planApproveArgs
    if err := json.Unmarshal([]byte(argsInJSON), &args); err != nil {
        return "", err
    }
    info := &ApprovalInfo{
        ToolName: "plan_approve",
        Target:   "plan_approve",
        What:     "执行计划",
        Args:     json.RawMessage(argsInJSON),  // full plan as frozen args
        Why:      fmt.Sprintf("Agent 提出了包含 %d 个步骤的计划", len(args.Plan.Steps)),
    }
    state := &ApprovalDecision{Decision: "pending", ApprovalID: args.PlanID}
    return "", tool.StatefulInterrupt(ctx, info, state)  // canonical interrupt
}
```

### Anti-Patterns to Avoid

- **Anti-pattern: Setting `MaxIteration: 50` (default).** DeepAgent's `MaxIteration` defaults to 50 (Pitfall 8: cost explosion + unbounded loops). **Hard-cap at 10** for PLAN-04.
- **Anti-pattern: Trust stage hoisting as a wrapper layer.** Don't wrap DeepAgent in another `adk.Agent` to apply trust floor/grounding. Use the `Handlers []TypedChatModelAgentMiddleware[M]` slot in `deep.New` config — it is the eino-blessed extension point (chatmodel.go:314-401).
- **Anti-pattern: write_todos audit via post-Run log scraping.** Don't wait until Run finishes to extract plan state from final message — emit each write_todos call as an audit row + SSE chunk as it fires, so the live stream is always authoritative (PLAN-02 requirement).
- **Anti-pattern: Reusing `data-decision` discriminator for plan todos.** D15 wire-type alarm + D10-B4 overload (4 history events already use `Step=0`) — add `data-plan-trace` discriminator instead. Cleaner + less audit-side ambiguity.
- **Anti-pattern: Plan approval as a separate Runner-level interrupt (not Tool-level).** Phase 11 explicitly bound approval to Tool-level interrupts (D11-A2 architectural boundary). Reusing the Tool pattern keeps the Runner migration intact and the audit lifecycle 3-row pattern uniform.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DeepAgent archetype | Custom `react.Agent` + custom write_todos Tool | `github.com/cloudwego/eino/adk/prebuilt/deep` | Eino provides write_todos, task tool, sub-agent slots, `SessionKeyTodos` integration, and returns `adk.ResumableAgent` (Runner-compatible). Building it duplicates ~200 LOC |
| Token budget callback | Custom token-counter middleware | `deepBudgetCallback` (P12 implementation, wraps ChatModel token counter — uses `model.WithGenerateConfig` options or hooks into response.Usage) | Token counting is HARD (per-model tokenizers, streaming chunk summation, retry overlap). Use the model's reported `Usage.TotalTokens` field (standard in OpenAI-compatible APIs including GLM-5.2) |
| Loop detection | Custom LLM-judge loop check | Hash-based rolling window of last 3 tool calls (`fnv64a(tool_name + args_hash)`); trigger if same hash 3 turns in a row | LLM-judge adds latency + cost; deterministic hash is provable + free |
| Plan approval gate | Custom Runner-level pre-execute hook | `hitl_plan_approve.go` Tool following Phase 11 HITL pattern (D11-B2 + D11-D2) | Reuses ApprovalDecision schema, fail-closed audit, rule auto-pass, etc. — already tested in Phase 11 acceptance gate |
| Schema registration for cross-process resume | Custom serialization | `schema.RegisterName[T](name)` for write_todos custom payload (`[]deep.TODO`) | Phase 8 SC-5 stub Hermetic test proved this pattern (`schema.RegisterName[*ApprovalDecision]("hitl.ApprovalDecision")`) |
| Verdict JSON parse for DeepAgent final message | Custom regex/JSON extractor | `llm.ExtractJSON` (shared in `internal/llm`) + `verdict.Verdict` struct | Already used in Phase 2-11; OOS-1 (debug: glm-verdict-fence-parse) + chunk-id L3 fix (debug: jpu) both validated this path |
| `MaxIterations` 名称差异 | Hand-mapping `MaxIterations` (chatmodel.go) vs `MaxIteration` (deep.go) — different field names on different config types | Document via code comment; both fields exist (deep.go:64 + chatmodel.go:308) | Verified by direct module cache inspection — NOT a typo, just inconsistent upstream naming |

**Key insight:** Almost all Phase 12 surface area is reusable from Phase 11 — the planning layer is the natural extension of the HITL harness. The only novel piece is the budget gate (which needs careful per-Skill `MaxIteration` calibration) and the write_todos audit hook (which needs Option C / custom write_todos Tool).

## Runtime State Inventory

> Phase 12 does NOT introduce persistent schema renames — it adds a new table (`plan_todos`) and new wire types (`data-plan-trace`, `PlanTracePayload`). Audit grep finds files; runtime state audit is for **newly created objects during execution**.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | New sqlite `plan_todos` table (run_id, todo_id, content, status, ts, op) — INSERT-only per T-3-AUDIT | Code edit (DDL) + new store interface; no data migration of existing tables |
| Live service config | N/A — no service config strings change | None |
| OS-registered state | N/A — Phase 12 doesn't register any OS-level resources | None |
| Secrets/env vars | N/A — no new env vars; existing `SKILLFORGE_RETENTION_DAYS` / `X-Admin-Token` carry over | None |
| Build artifacts | `cmd/server` binary re-linked (small change — new internal/runtime package) | Standard `go build ./...`; no special cleanup |

**Nothing found in category** (Live service config / OS-registered state / Secrets / Build artifacts): verified by direct inspection of `internal/server/*.go` + `cmd/server/main.go` (no env-name additions).

## Common Pitfalls

### Pitfall 1: DeepAgent MaxIteration Default = 50 (Cost Explosion)

**What goes wrong:** GLM-5.2 + DeepAgent with default `MaxIteration=50` runs 50 reasoning iterations → ~$5+ in API cost per query → v0.8 personal-brand deploys bankrupt the operator.

**Why it happens:** Eino's `deep.Config.MaxIteration` defaults to 0 → eino's internal default is 50 (verified `deep.go:64` `MaxIteration int` field; default behavior inherited from `chatmodel.go:307-308` `Optional. Defaults to 20` for ChatModelAgent — DeepAgent may differ, needs verify).

**How to avoid:** **Hard-code `MaxIteration: 10`** in the factory (`internal/runtime/deep_agent.go`) — the budget gate in PLAN-04 makes this an architectural invariant, not a config knob. Run-level token budget + loop detection are the second/third safety nets.

**Warning signs:** A live query spends >2 minutes in the LLM loop; `cost > $0.50` in GLM usage log for one query; `MaxIteration` not in `internal/runtime/deep_agent.go` at all.

### Pitfall 2: GLM-5.2 Verdict JSON Parse Failure (RF-5)

**What goes wrong:** DeepAgent final message embeds the verdict JSON inside a `<result>` tag or trailing prose; `llm.ExtractJSON` fails to parse; the agent's answer is lost; user sees an empty response.

**Why it happens:** DeepAgent's `instruction` prompt (deep.go:147-202) trains the model to use prose + tool calls, not strict JSON output. Unlike Phase 2's `systemPrompt` (which explicitly demands `Output ONLY a JSON object with this exact schema`), DeepAgent's instruction is prose-friendly.

**How to avoid:** **Two-pronged (RF-5 mitigation per ROADMAP):**
1. **Structured-output final step:** Render a final LLM call after DeepAgent.Run returns with `response_format={type:"json_object"}` (or GLM-5.2 equivalent structured output) + the verdict JSON schema. The final step's prompt is a verbatim copy of Phase 2's `systemPrompt` (verdict JSON schema).
2. **`forced_stop` fallback on parse failure:** If the structured-output final step fails to produce valid JSON, emit `data-decision(forced_stop)` SSE chunk and return the last-known assistant message verbatim (no verdict) — the user sees partial content + a "agent gave up" indicator (mirrors Phase 3 emitForcedStop).

**Warning signs:** Live query against GLM-5.2 + DeepAgent final message >2KB and no JSON parse success; verify-message test fails >30% of the time.

### Pitfall 3: write_todos Audit Not Persisted → Replay Broken

**What goes wrong:** PlanTracePart streams live todos during execution, but if the audit row isn't INSERTed synchronously (T-3-AUDIT) the 轨迹回放视图 can't reconstruct the plan evolution after refresh.

**Why it happens:** write_todos is a DeepAgent middleware — without intercepting the tool call, SkillForge never sees the todo list at audit time. The middleware calls `adk.AddSessionValue(ctx, SessionKeyTodos, input.Todos)` (deep.go:255) but this is in-memory only.

**How to avoid:** **Option C (recommended above)** — register a custom write_todos Tool that (a) calls `s.Store.PutPlanTodo(...)` synchronously (T-3-AUDIT fail-closed) AND (b) emits `data-plan-trace` SSE chunk via the per-Run emitter. Both must succeed before the Tool returns; failure = interrupt + audit row failure (mirrors Phase 11 HITL fail-closed posture).

**Warning signs:** After page refresh during a deep query, `GET /api/run/:id/plan` returns < the expected todo count; `data-plan-trace` SSE chunks arrive but no audit row in sqlite.

### Pitfall 4: Plan Approval Gate Timeout → Agent Hangs

**What goes wrong:** User opens plan approval card, walks away. Agent is interrupted mid-plan-approve; Runner.CheckPointStore persists the interrupt; 24 hours later, the user returns — but the Skill version has changed (hot reload), checkpoint is expired, agent can't resume.

**Why it happens:** Phase 11 already handles this for HITL approvals (D11-C2: snapshot dual-table落 + expired UI state + DELETE checkpoint row). Phase 12 plan approval must follow the same pattern.

**How to avoid:** Reuse `hitl_plan_approve.go` Tool pattern verbatim — `action_pending_approval` row snapshots `skill_version` + `prompt_hash`, Resume checks mismatch → DELETE checkpoint + UI "expired" state. The plan-specific layer only adds the `target='plan_approve'` discriminator in `AllowedWriteTargets` (D11-B3 single source of truth).

**Warning signs:** TestExpiredCheckpointUIState passes for HITL approvals but not for plan approvals; 24-hour-old plan approval resumes against a hot-reloaded Skill.

### Pitfall 5: Loop Detection False Positives on Legitimate Repetition

**What goes wrong:** A legitimate compliance audit query requires the agent to call `retrieve_compliance_chunks` 3 times in a row (with slightly refined queries each iteration). Loop detector triggers `forced_stop` mid-execution.

**Why it happens:** Naive loop detection hashes `tool_name + args` and triggers on any 3-in-a-row match. Real multi-hop retrieval uses different args each call.

**How to avoid:** **Hash only `tool_name + first 200 chars of args`** (truncation tolerates legitimate variation). OR **use Levenshtein distance** (fuzzier match) — but at the cost of complexity. Recommended: truncation + `tool_name`. Phase 3 retrieve tool calls with refined queries will hash differently because of the truncation moving the hash boundary.

**Warning signs:** Multi-hop compliance queries (Phase 3 golden set) trigger forced_stop >10% of the time; loop test fires on legitimate `retrieve → retrieve → retrieve` sequences.

### Pitfall 6: Plan Trace Streaming Buffering in AI SDK v5

**What goes wrong:** `data-plan-trace` SSE chunks arrive at the FE, but Vercel AI SDK v5's `useChat` parser buffers all custom `data-*` chunks into `message.parts` (per the UIMessageChunk discriminated union). PlanTracePart doesn't render live because the parts only flush at `finish-step` / `finish-message`.

**Why it happens:** Vercel AI SDK v5's stream parser batches custom parts at message boundaries by design (see `.messageMetadata` + `.data-*` accumulator pattern in `ai-sdk.dev` docs).

**How to avoid:** Two paths:
1. **Emit `data-plan-trace` chunks as synthetic `tool-input-*` + `tool-output-available` chunks** — these ARE rendered live in AI SDK v5 useChat (the tool-call lifecycle is the primary streaming surface). Use the tool name `update_plan` and pass the todos as the tool args.
2. **Use `addToolOutput` callback pattern** — same as Phase 11 ApprovalCard (D-04 transport-swap preserved). The tool-call-id-based routing makes live updates explicit.

**Recommended: Option 1** (synthetic tool-call pattern). PLAN-02's primary user-facing surface is a live todo list — tool-call lifecycle parts are the AI SDK v5-native way to render this. The actual write_todos Tool call ALSO emits a tool-call lifecycle, so we're not faking it; we're just routing the audit/SSE emission through the tool-call surface.

**Warning signs:** PlanTracePart doesn't render until message ends; useChat.messages parts only contain the final state.

### Pitfall 7: `data-plan-trace` Wire Type Drift (D15 Violation)

**What goes wrong:** `data-plan-trace` added to Go `internal/wire.ValidTypes` but the TS mirror `web/src/types/wire-types.ts` not updated → `AssertAssignable<SkillForgeChunk, UIMessageChunk>` fails (D15 alarm).

**Why it happens:** D15 two-PR lockstep rule (Phase 4) requires Go + TS changes in the same commit / PR.

**How to avoid:** **Every plan that touches wire types emits a paired Go + TS commit** with the same `D15:` prefix in the commit message. Already enforced in Phase 4-11 — Phase 12 plan-checker should grep `git log` for `D15:` commits covering `data-plan-trace` introduction.

**Warning signs:** `npx tsc --noEmit` fails in the same commit that adds `data-plan-trace` to `internal/wire/chunk.go`.

## Code Examples

Verified patterns from official sources:

### Example 1: DeepAgent Construction (Verified)

```go
// Source: github.com/cloudwego/eino@v0.9.14/adk/prebuilt/deep/deep.go:171-173 (verified by direct module cache inspection)
// + Phase 11 BuildRunner pattern (internal/agent/runner_setup.go:204-217)

import (
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/adk/prebuilt/deep"
    "github.com/cloudwego/eino/components/model"
)

func BuildDeepAgent(ctx context.Context, cfg DeepAgentBuildConfig) (adk.ResumableAgent, error) {
    if cfg.MaxIteration == 0 {
        cfg.MaxIteration = 10  // PLAN-04 hard cap (Pitfall 8)
    }
    // Handlers — wraps model with budget callback (token + loop detect)
    return deep.New(ctx, &deep.Config{
        Name:          "skillforge-deep-" + cfg.SessionID,
        Description:   "SkillForge compliance-review DeepAgent (P8.3 planning layer)",
        ChatModel:     cfg.ChatModel,
        Instruction:   renderDeepSystemPrompt(cfg.Manifest),
        ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: cfg.Tools}},
        MaxIteration:  cfg.MaxIteration,                              // PLAN-04 budget gate
        WithoutWriteTodos: false,                                    // auto-inject (default)
        // Custom Tool: option C — replace with our audited write_todos
        // Handlers: []adk.TypedChatModelAgentMiddleware{...},        // see budget.go for token+loop detect
    })
}
```

### Example 2: write_todos Custom Tool (Option C — Audit + SSE Hooks)

```go
// Mirrors Phase 11 hitl_memory_facts.go:71-78 pattern (D11-A2 + D11-B2)
// New: internal/agent/deep_audit.go

func (t *writeTodosAuditedTool) InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error) {
    var args writeTodosArguments  // mirrors deep.writeTodosArguments
    if err := json.Unmarshal([]byte(argsInJSON), &args); err != nil {
        return "", err
    }
    // (a) Audit row — INSERT-only, T-3-AUDIT
    for _, todo := range args.Todos {
        if err := t.store.PutPlanTodo(ctx, store.PlanTodoRow{
            RunID: t.runID, TodoID: hashID(todo.Content), Status: todo.Status,
            Content: todo.Content, Ts: time.Now(), Op: "upsert",
        }); err != nil {
            return "", fmt.Errorf("write_todos audit fail-closed: %w", err)  // mirror HITL-04
        }
    }
    // (b) SSE chunk — emit via per-Run emitter
    payload, _ := json.Marshal(wire.PlanTracePayload{
        RunID: t.runID, Todos: convertTODOs(args.Todos), Ts: time.Now(),
    })
    _ = t.emit(wire.UIMessageChunk{Type: "data-plan-trace", Data: payload})
    // (c) Default DeepAgent behavior — set session value
    adk.AddSessionValue(ctx, deep.SessionKeyTodos, args.Todos)
    return fmt.Sprintf("Updated todo list to %s", marshal(args.Todos)), nil
}
```

### Example 3: Budget Callback (PLAN-04)

```go
// Source: eino@v0.9.14/adk/chatmodel.go:325-326 documents Handlers as the v0.9+ extension point
// New: internal/agent/callbacks/budget.go

func NewBudgetCallback(cfg BudgetConfig) adk.TypedChatModelAgentMiddleware[*schema.Message] {
    var (
        totalTokens atomic.Int64
        recentCalls []string  // ring buffer of last 3 tool calls (fnv64a hash)
    )
    return &budgetMiddleware{
        TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.Message]{},
        onBeforeModel: func(ctx context.Context, rc *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
            if totalTokens.Load() > int64(cfg.MaxRunTokens) {
                return ctx, rc, errors.New("PLAN-04 token budget exceeded — forced_stop")
            }
            return ctx, rc, nil
        },
        onAfterModel: func(ctx context.Context, rc *adk.ChatModelAgentContext, msg *schema.Message) (context.Context, *adk.ChatModelAgentContext, *schema.Message, error) {
            if msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
                totalTokens.Add(int64(msg.ResponseMeta.Usage.TotalTokens))
            }
            return ctx, rc, msg, nil
        },
        // Loop detection on tool calls (BeforeToolCall or AfterToolCall)
    }
}
```

### Example 4: Plan Approval (PLAN-03) — HITL Tool Reuse

```go
// Source: internal/agent/hitl/hitl_memory_facts.go:71-78 (Phase 11 D11-A2 + D11-B2 verified pattern)
// New: internal/agent/hitl/hitl_plan_approve.go

// ApprovalDecision schema UNCHANGED (D11-B2 single source of truth) — frozen_args = plan JSON
// target = "plan_approve" added to AllowedWriteTargets (D11-B3 — single-file edit propagates)

func (t *planApproveTool) InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error) {
    var args planApproveArgs
    json.Unmarshal([]byte(argsInJSON), &args)  // assume valid; pre-validated upstream
    info := &ApprovalInfo{
        ToolName: "plan_approve",
        Target:   "plan_approve",
        What:     "执行计划批准",
        Args:     json.RawMessage(argsInJSON),
        Why:      fmt.Sprintf("Agent 提出了 %d 步计划", len(args.Plan.Steps)),
    }
    state := &ApprovalDecision{Decision: "pending", ApprovalID: args.PlanID}
    return "", tool.StatefulInterrupt(ctx, info, state)  // canonical interrupt (D11-A2)
}
```

### Example 5: data-plan-trace Wire Type (D15 Lockstep)

```go
// Source: internal/wire/chunk.go:280-298 (existing ValidTypes pattern)
// New: internal/wire/chunk.go addition (P12 plan)

// In ValidTypes:
"data-plan-trace": true,

// New type (mirrors SubprogressStep):
type PlanTracePayload struct {
    RunID string `json:"runId"`
    Todos []PlanTodo `json:"todos"`
    Ts    int64  `json:"ts"`
}
type PlanTodo struct {
    ID      string `json:"id"`
    Content string `json:"content"`
    Status  string `json:"status"` // pending | in_progress | completed
}
```

```typescript
// Source: web/src/types/wire-types.ts (D15 mirror)
// Add: P12 plan
export type PlanTracePayload = {
  runId: string;
  todos: PlanTodo[];
  ts: number;
};
export type PlanTodo = {
  id: string;
  content: string;
  status: 'pending' | 'in_progress' | 'completed';
};
// SkillForgeChunk union: add `| { type: 'data-plan-trace'; data: PlanTracePayload }`
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 11 ChatModelAgent (react.Agent under the hood) | DeepAgent with write_todos + sub-agent slots | eino v0.5.0 (June 2026) GA | DeepAgent provides planning layer + sub-agent composition (Phase 13) without re-implementation |
| `MaxStep` (react.Agent, internal to compose.Graph) | `MaxIterations` (ChatModelAgent) + `MaxIteration` (deep.Config) — naming inconsistency | eino v0.9 ChatModelAgent refactor | Document via code comment; both fields exist; pick one per archetype |
| `NewInterruptAndRerunErr` (legacy) | `tool.StatefulInterrupt(ctx, info, state)` (new) | eino v0.7+ | D8-04 invariant — legacy banned by `TestNoLegacyInterruptAPI` WalkDir gate |
| `chatmodel.go` Handlers field for middleware | `Handlers []TypedChatModelAgentMiddleware[M]` (v0.9+) | eino v0.9 ChatModelAgent refactor | v0.9+ extension point for budget/loop detection middleware |
| Phase 11 `audit.action` enum (`action_pending_approval` / `_approved` / `_rejected`) | P12 adds `action_plan_todo_create` / `_update` / `_complete` (extends existing enum) | P8.3 | Mirrors HITL-04 pattern; reuses T-3-AUDIT INSERT-only guarantee |
| Phase 4 wire 22 types | P12 adds `data-plan-trace` (23 total) | P8.3 | D15 lockstep with TS; required for live todo updates |

**Deprecated/outdated:**
- **`react.Agent` for write_todos scenarios:** Eino docs explicitly recommend DeepAgent (deep.go:18 comment: "Package deep provides a prebuilt agent with deep task orchestration."). Phase 12 plan should NOT use react.Agent for `agent: deep` Skills.
- **Custom write_todos implementations outside DeepAgent:** Eino's deep package provides it for free; only the audit/SSE hooks need to be SkillForge-specific.
- **`GetResumeContext[T](ctx)` for plan approval resume state:** Use the same Phase 11 `ApprovalDecision` schema (D11-B2 single source of truth) — don't introduce a new plan-specific resume state type.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `deep.Config.MaxIteration` defaults to 50 (not 20 like `ChatModelAgent.MaxIterations`) | Pitfall 1 | If default is 20, hard-cap at 10 is still right (cost discipline); but docs/cost-model may need adjustment |
| A2 | `deep.New` returns `adk.ResumableAgent` (= `TypedResumableAgent[*schema.Message]`) | Pattern 1 | Verified directly against deep.go:171-173; HIGH confidence |
| A3 | `eino@v0.9.14/adk/prebuilt/deep` is stable and won't change before P12 ships | Stack | Module is locked Phase 8 (D8-01); HIGH confidence |
| A4 | GLM-5.2 supports `response_format={type:"json_object"}` structured output | Pitfall 2 | GLM-5.2 is OpenAI-API-compatible (verified via Volcano Ark / Phase 3 spike); MEDIUM confidence on json_object specifically — may need json_schema mode |
| A5 | Vercel AI SDK v5 useChat routes `tool-input-*` + `tool-output-available` chunks as live-rendered parts | Pitfall 6 | Phase 11 ApprovalCard already uses this pattern (D-04 transport-swap invariant); HIGH confidence |
| A6 | Hot-reload of Skills with `agent: deep` triggers plan_todos audit table cleanup | Pitfall 4 | Phase 11 approval_rules hot-reload cleanup is deferred (CONTEXT §Deferred Ideas); P12 plan_todos likely same — MEDIUM confidence |
| A7 | `deep.go` Schema registration `schema.RegisterName[TODO]("_eino_adk_prebuilt_deep_todo")` is sufficient for cross-process resume | Pattern 1 | Eino auto-registers on package init; if SkillForge needs to register custom PlanTodo type, must call `schema.RegisterName[*PlanTodo]("skillforge.PlanTodo")` — MEDIUM confidence (depends on whether we use deep.TODO or our own type) |
| A8 | RF-5 verdict-parse rate is **unknown** — 50-case golden set must measure before locking design | Pitfall 2 | ROADMAP explicitly flags this as LOW confidence; planned spike in P12-01 acceptance gate |

**If this table is empty:** It is NOT empty — see A1-A8 above.

## Open Questions

1. **GLM-5.2 verdict-parse rate on DeepAgent final message (RF-5)**
   - What we know: GLM-5.2 follows OpenAI-compatible API; structured output (json_object / json_schema) is supported per Volcano Ark docs (assumed).
   - What's unclear: Actual parse rate on DeepAgent's prose-friendly final message format.
   - Recommendation: P12-01 spike measures 50 cases; if parse rate <90%, lock structured-output final step; if <70%, escalate to RF-5 fallback (verdict-JSON suffix + forced_stop).

2. **write_todos tool interception point (Option A/B/C)**
   - What we know: DeepAgent auto-registers write_todos via `typedBuildAppendPromptTool` (deep.go:44-65); SkillForge needs to intercept to emit audit + SSE.
   - What's unclear: Whether intercepting via `Handlers` (Option B) sees the tool args, or whether we must use Option C (custom write_todos Tool with `WithoutWriteTodos=true`).
   - Recommendation: Spike in P12-01 — build minimal handler, see if `RunToolsNode` callbacks expose write_todos args. If yes, use Option B (cleaner — preserves auto-injection). If no, use Option C.

3. **Plan approval reject semantics**
   - What we know: Phase 11 HITL approval supports approve / deny. Plan-specific deny could mean (a) "abort the agent run entirely" or (b) "agent rewrites plan and resubmits".
   - What's unclear: Which semantic matches user expectations; how to pass the rewritten plan back to the agent.
   - Recommendation: P12 discuss-phase with user; defer to Claude's discretion if not asked.

4. **Loop detection threshold (3 same calls = loop)**
   - What we know: Phase 3 multi-hop retrieval legitimately calls retrieve 2x (DefaultMaxHops=2).
   - What's unclear: What's the threshold beyond which "multi-hop" becomes "loop"? Is 3 same-hash calls safe, or should we look at 5?
   - Recommendation: Start at 3, configurable via env `SKILLFORGE_LOOP_THRESHOLD`; live measurement in acceptance gate.

5. **Token budget calibration (initial value)**
   - What we know: Phase 9 NO-GO revealed faithfulness/refusal context drift; RF-6 (tokenizer calibration) is a known unverified item.
   - What's unclear: What's the right run-level token cap for compliance Skill queries (typically 5-10 LLM calls)? For other Skills?
   - Recommendation: Start at 50,000 tokens per run (covers ~10 LLM calls with 5K context each); live tuning in acceptance gate. Per-Skill override possible via SKILL.md `token_budget: <int>` extension.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.23 | All BE code | ✓ | 1.23+ (per go.mod) | — |
| eino v0.9.14 | DeepAgent + Runner | ✓ (locked Phase 8) | v0.9.14 | — |
| eino-ext openai v0.1.13 | ChatModel (GLM-5.2 via OpenAI-compat) | ✓ (locked Phase 8) | v0.1.13 | — |
| SQLite (modernc.org v1.56.0) | CheckPointStore + audit + plan_todos | ✓ | v1.56.0 | — |
| Vercel AI SDK v5 | FE plan-trace streaming | ✓ (locked Phase 4) | v5 | — |
| React 19 + Vite 8 + Tailwind v4 + shadcn | FE PlanTracePart + PlanApprovalCard + PlanReplayView | ✓ (locked Phase 4-5) | 19.2.x | — |
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
| Quick run command | `go test ./internal/runtime/... ./internal/agent/... ./internal/store/... ./internal/verdict/... -count=1` (BE) + `cd web && npx vitest run --reporter=verbose` (FE) |
| Full suite command | `go test ./... -count=1 -tags=contract` + `cd web && npx vitest run` + `npx tsc --noEmit` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAN-01 | `agent: deep` Skill → DeepAgent archetype; all archetype → `ApplyTrustStage` exit | unit | `go test ./internal/runtime/... -run TestFactoryDispatch -v` | ❌ Wave 0 |
| PLAN-01 | Regression lock — Phase 11 ChatModelAgent still passes trust stage | unit | `go test ./internal/verdict/... -run TestTrustStageRegression -v` | ❌ Wave 0 |
| PLAN-02 | PlanTracePart streaming live | unit + integration | `go test ./internal/agent/... -run TestWriteTodosAuditedTool -v` + `cd web && npx vitest run plan-trace-part` | ❌ Wave 0 |
| PLAN-03 | Plan approval gate (approve / deny / rewrite) | unit | `go test ./internal/agent/hitl/... -run TestPlanApprove -v` | ❌ Wave 0 |
| PLAN-04 | Budget gate forced_stop on each layer | unit | `go test ./internal/agent/callbacks/... -run TestBudget -v` | ❌ Wave 0 |
| PLAN-05 | Replay view reconstructs plan from audit | unit + integration | `go test ./internal/store/sqlite/... -run TestPlanTodosReplay -v` + `cd web && npx vitest run plan-replay-view` | ❌ Wave 0 |
| PLAN-04 | Audit row on budget trigger | unit | `go test ./internal/store/sqlite/... -run TestAuditBudgetTriggered -v` | ❌ Wave 0 |
| D15 | TS mirror in sync with Go wire | compile-time | `cd web && npx tsc --noEmit` | ❌ Wave 0 |
| D8-04 | No legacy interrupt API | unit (WalkDir) | `go test ./... -run TestNoLegacyInterruptAPI -v` | ✅ Phase 8 (extended) |
| D11-C4 | approval_id == tool_call_id | unit | `go test ./internal/store/sqlite/... -run TestApprovalIDIdentityInvariant -v` | ✅ Phase 11 |
| HITL-04 | 3-row lifecycle + fail-closed | unit | `go test ./internal/agent/hitl/... -run TestAuditChainThreeRowLifecycle -v` | ✅ Phase 11 |
| RF-5 | GLM-5.2 verdict parse rate ≥ 90% on 50-case golden set | spike (live) | `go run ./cmd/spike-deep parse-rate --golden=eval/golden/deep.jsonl` | ❌ Wave 0 (needs golden set) |

### Sampling Rate
- **Per task commit:** `go test ./internal/runtime/... ./internal/agent/... ./internal/store/... ./internal/verdict/... ./internal/wire/... -count=1` + `cd web && npx vitest run`
- **Per wave merge:** Full suite above + `go test ./... -tags=contract` + `npx tsc --noEmit`
- **Phase gate:** Full suite green + live spike parse-rate + 50-case golden set acceptance + frozen eval re-lock (v0.8 milestone re-baseline)

### Wave 0 Gaps
- [ ] `internal/runtime/factory.go` + `factory_test.go` — archetype dispatch
- [ ] `internal/runtime/trust_stage.go` + `trust_stage_test.go` — ApplyTrustStage exit
- [ ] `internal/runtime/deep_agent.go` + `deep_agent_test.go` — DeepAgent wiring
- [ ] `internal/runtime/budget.go` + `budget_test.go` — budget gate callbacks
- [ ] `internal/agent/deep_audit.go` + `deep_audit_test.go` — audited write_todos Tool
- [ ] `internal/agent/hitl/hitl_plan_approve.go` + `hitl_plan_approve_test.go` — plan approval HITL Tool
- [ ] `internal/store/sqlite/plan_todos.go` + `plan_todos_test.go` — INSERT-only audit table
- [ ] `internal/store/sqlite/audit.go` — extended QueryAudit for /api/run/:id/plan JOIN
- [ ] `internal/server/plan_api.go` + `plan_api_test.go` — GET /api/run/:id/plan, POST /api/run/:id/plan/approve
- [ ] `internal/server/runner_setup.go` — refactor to call factory.dispatchAgent (PLAN-01)
- [ ] `internal/wire/chunk.go` — add data-plan-trace + PlanTracePayload
- [ ] `internal/skill/manifest.go` — add "deep" to validAgentTypes
- [ ] `examples/skills/compliance-deep/SKILL.md` — `agent: deep` showcase Skill
- [ ] `web/src/components/PlanTracePart.tsx` — live todo stream renderer
- [ ] `web/src/components/PlanApprovalCard.tsx` — plan confirmation card
- [ ] `web/src/components/PlanReplayView.tsx` — 规划轨迹回放视图
- [ ] `web/src/types/wire-types.ts` — D15 lockstep mirror
- [ ] `cmd/spike-deep/main.go` + `main_test.go` — RF-5 verdict parse rate spike
- [ ] `eval/golden/deep.jsonl` — 50-case golden set for RF-5 parse rate

## Security Domain

> `security_enforcement` is enabled (default — Phase 11 STRIDE-11 threat model). Required.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | N/A (P12 doesn't introduce auth changes) |
| V3 Session Management | yes | Per-session Agent + Runner (D11-B4); DeepAgent's `SessionKeyTodos` per-run |
| V4 Access Control | yes | Skill `agent:` field gated by manifest validator; `agent: deep` only allowed if Skill explicitly declares it (mirrors `validAgentTypes` whitelist) |
| V5 Input Validation | yes | plan_todos table INSERT only via audited write_todos Tool (T-3-AUDIT parameterized queries); budget gate enforced structurally (MaxIteration cap = code, not config) |
| V6 Cryptography | yes | Plan approval flow uses same `crypto/subtle.ConstantTimeCompare` (Phase 11 D-06) — no new crypto |
| V7 Error Handling | yes | Budget gate `forced_stop` must emit `data-decision` chunk (not silent abort); plan_todos audit fail-closed (mirrors HITL-04) |
| V8 Data Protection | yes | plan_todos audit row contents = todo text (could be sensitive — e.g. legal review plan); retention redaction (Phase 6 SC-5) applies via audit read-time projection |

### Known Threat Patterns for eino DeepAgent

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| DeepAgent sub-agent injection (P13 territory but start hardening now) | Tampering / Elevation of Privilege | `WithoutGeneralSubAgent: true` in P8.3 (Phase 13 builds the safe fan-out path) |
| write_todos audit bypass | Tampering / Repudiation | T-3-AUDIT INSERT-only; audited write_todos Tool fail-closed (mirrors HITL-04 audit_writer pattern) |
| Budget gate bypass via prompt injection | Denial of Service | `MaxIteration: 10` hardcoded in factory (not config-driven); callback enforces |
| Plan approval replay attack | Spoofing | approval_id == tool_call_id (D11-C4) + snapshot dual-table落 (D11-C2) |
| Verdict JSON parse manipulation | Tampering | RF-5 structured-output final step + forced_stop fallback (Pitfall 2) |
| Loop detection false positives | Denial of Service | Tunable threshold via env (default 3); legitimate multi-hop uses different args (Pitfall 5 mitigation) |

### Plan-Specific Threats (P12 only)

| Threat | Mitigation |
|--------|-----------|
| T-P12-1: Plan approval bypass via direct Runner.Resume without POST approval | ResumeWithParams path validates `tool.GetResumeContext[*ApprovalDecision]`; without proper Decision, Runner re-interrupts (canonical behavior) |
| T-P12-2: Plan replay UI reveals user-private plan content | Retention redaction (Phase 6 SC-5) applies to plan_todos rows older than retention window |
| T-P12-3: Budget gate DoS via cheap prompts (flood with low-effort queries) | Token budget + MaxIteration are PER-RUN — each run is independent; cannot accumulate across runs |
| T-P12-4: write_todos SSE chunk spoofing (claim a todo is "completed" when not) | SSE chunk payload is emitted by audited Tool call; recipient (FE) verifies by GET /api/run/:id/plan (T-3-AUDIT reads back from sqlite) |
| T-P12-5: Plan approval denial causes silent abort | "deny" decision emits `data-decision(forced_stop)` chunk (P12 budget gate) + returns assistant message verbatim — user sees the partial plan + abort indicator |

## Sources

### Primary (HIGH confidence)
- eino@v0.9.14 module cache (C:/Users/CPIC/go/pkg/mod/github.com/cloudwego/eino@v0.9.14/) — direct file inspection of `adk/prebuilt/deep/{deep,task_tool,types}.go`, `adk/prebuilt/planexecute/plan_execute.go`, `adk/{chatmodel,runner,runctx}.go`, `adk/middlewares/plantask/plantask.go`, `internal/core/interrupt.go` (CheckPointStore interface)
- Phase 11 D11-A*/B*/C*/D* locked decisions (CONTEXT.md)
- Phase 11 `internal/agent/{runner_setup,runner_sse,callbacks,hitl}` (existing patterns)
- Phase 11 `internal/store/sqlite/{schema,audit,approval_rules,checkpoint}.go` (DDL pattern)
- Phase 11 `internal/wire/chunk.go` (D15 wire-type discriminator pattern)

### Secondary (MEDIUM confidence)
- D15 wire-type compile alarm (Phase 4 D-04 — verified pattern)
- Phase 11 STRIDE-11 threat model (8 Critical + 4 High threats from `docs/threat-model/STRIDE-11.md`)
- Vercel AI SDK v5 `data-*` chunk routing (Phase 4-11 verified in D-04 transport-swap invariant)

### Tertiary (LOW confidence)
- RF-5 GLM-5.2 verdict parse rate on DeepAgent final message — pending live measurement
- `deep.Config.MaxIteration` default value (assumed 50; needs verify against ChatModelAgent pattern)
- Option A/B/C write_todos interception mechanism — needs spike
- Token budget initial value (50,000 per run) — needs live calibration

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — verified directly against eino@v0.9.14 module cache
- Architecture: HIGH — Phase 11 D11-* decisions are locked + DeepAgent API surface confirmed
- Pitfalls: MEDIUM-HIGH — 5/7 pitfalls are direct extensions of known Phase 11 patterns; Pitfall 1 (cost explosion) and Pitfall 2 (RF-5 parse rate) are the two MEDIUM items
- Validation Architecture: HIGH — Phase 11 11/11 acceptance gate pattern transfers directly; RF-5 spike is the only new item

**Research date:** 2026-08-20
**Valid until:** 30 days (Phase 12 plans should be written by 2026-09-19)