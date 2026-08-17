# Technology Stack — v0.8 Harness Milestone (可信×行动融合)

**Project:** SkillForge
**Researched:** 2026-08-17
**Scope:** Stack additions/changes for the 4 new harness capabilities only (multi-turn memory, write tools + approval gate, DeepAgent planning, sub-agent fan-out). Existing stack (Go/Hertz/Milvus/Mongo/BGE-M3/GLM/React 19/ai@7/Zustand/sqlite) is NOT re-researched.

## TL;DR Decisions

| Question | Answer | Confidence |
|----------|--------|------------|
| (a) Min eino version for summarization middleware + stable HITL | **v0.9.13** (latest stable tag). Summarization middleware landed in v0.8.x; v0.9.x ("agentic-runtime") is the current stable line. Do NOT use v0.10.0-alpha.x | HIGH |
| (b) Official persistent checkpoint store | **None exists** in eino-ext. Only `store.NewInMemoryStore()` ships. Build a ~100-line sqlite `CheckPointStore` on the existing sqlite store | HIGH |
| (c) ai@7 approval-interrupt support | **First-class.** `tool-approval-request`/`tool-approval-response` are UIMessageChunk wire types; `addToolApprovalResponse` + `approval-requested`/`output-denied` tool part states in `@ai-sdk/react`. Zero new FE deps | HIGH |
| (d) Sub-agent fan-out observability | **Built-in.** `AgentEvent.RunPath`/`AgentName` auto-filled by framework + `HandlerBuilder` agent callbacks. No new library needed; defer Langfuse/CozeLoop | HIGH |

## (a) Eino Upgrade: v0.7.7 → v0.9.13

### Version landscape (verified 2026-08-17)

| Line | Status | Theme | Relevance |
|------|--------|-------|-----------|
| v0.7.* (current pin v0.7.7) | superseded | interrupt/resume refactor | Already have: `compose.Interrupt`, CheckPointStore |
| **v0.8.\*** | superseded | **ADK middlewares** | **Adds `adk/middlewares/summarization` — required for multi-turn memory compression** |
| **v0.9.\* (latest v0.9.13)** | **CURRENT STABLE — upgrade target** | agentic-runtime | Agent Cancel, TurnLoop, AgenticMessage, HITL refinements, 13 patch releases of ADK fixes (checkpoint-aware cancel scopes, streaming tool output reduction, nested agent cancel isolation) |
| v0.10.0-alpha.13 | pre-release, AVOID | durable runtime, background exec, automemory | Maintainers warn: "expect many breaking changes before v0.10 final... avoid treating alpha APIs as stable production contracts." Stable targeted Q3 2026 |

**Upgrade command:**
```bash
go get github.com/cloudwego/eino@v0.9.13
```

### Migration notes v0.7.7 → v0.9.13 (only what touches SkillForge)

**From v0.8 (ADK middlewares):**
- Breaking changes are concentrated in the `adk/prebuilt/deep` filesystem/tool surface, which we are about to adopt (not yet in our code): `ShellBackend` interface renamed to `Shell`; `ReadRequest.Offset` 0-based → 1-based; `GrepRequest.Pattern` literal → regex (escape special chars); `WriteRequest` now overwrites existing files; `FileInfo.Path` no longer guaranteed absolute. **Impact on us: read the current docs when wiring DeepAgent; do not copy v0.7-era examples.**
- Custom agent decorators/wrappers must migrate to the `ChatModelAgentMiddleware` interface.

**From v0.9 (agentic-runtime):**
- `AgentMiddleware` struct + `ChatModelAgentConfig.Middlewares` field **deprecated** → migrate to interface-based `ChatModelAgentMiddleware`; embed `*adk.BaseChatModelAgentMiddleware` for no-op defaults. Since we haven't written middlewares yet, write them the new way from day one.
- `AgenticMessage` introduced alongside `*schema.Message` — foundation for server-side tool calling. **Not needed for our 4 features; stay on `*schema.Message`.**
- New Agent Cancel (`CancelMode` bitmask, `CancelHandle.Wait()`) integrates with our existing cancellation SC (<200ms cancel from Phase 1) — re-verify that SC after upgrade.

**Upgrade risk hotspots for our existing code:**
1. Phase 1 cancellation SC (ChatModelAgent + cancel <200ms) — v0.9 changed cancel scopes (#1143, #1151); re-run live verify.
2. SSE wire contract (`integration/contract/` drift suite) — run full contract suite post-upgrade before touching any feature code.
3. `eino-ext` component versions must be bumped in lockstep; v0.9 release notes pair eino v0.9.0 with e.g. `callbacks/cozeloop@v0.3.0`. Check each eino-ext component README for its eino compat floor (components are independently versioned).

## (b) Checkpoint Store: Build sqlite, Not Redis

### What exists

| Option | Status | Verdict |
|--------|--------|---------|
| `adk` in-memory store (`store.NewInMemoryStore()`) | Official, ships in eino | Dev/tests only — loses pending approvals on restart |
| Official redis/sqlite CheckPointStore in eino-ext | **Does not exist** (verified: only `components/embedding/cache/redis` exists; HITL docs say "use Redis in production" but ship no implementation) | — |
| Community `RedisCheckPointStore` (e.g. xyncra-server, RAGFlow Go port) | Third-party patterns: KV bytes + key prefix + TTL + fail-closed error propagation | Reference patterns only |
| **Custom sqlite store on existing `internal/store/sqlite`** | Interface is trivial: `Get(ctx, id) ([]byte, bool, error)` / `Set(ctx, id, []byte) error` (+ optional `Delete` via `CheckPointDeleter`) | **RECOMMENDED** |

### Why sqlite over Redis

- SkillForge deployment is local + Docker Compose single-node (PROJECT.md constraint) — a Redis sidecar violates "易上手" for zero benefit.
- We already operate an sqlite session/audit store with proven `WHERE session_id=?` isolation (SC-4, 50×5 test). Checkpoint rows key naturally by `checkpoint_id` (session-scoped), co-locating with the audit trail — the "可信×行动" story: approval interrupt → checkpoint row + audit row in one DB, replayable.
- Matches the INSERT-only posture: checkpoints are immutable blobs; resume reads, never mutates history.
- Production-grade precedent patterns (xyncra/RAGFlow) confirm the interface is just bytes-KV; the serialization is `encoding/gob` — **must `gob.RegisterName(...)` every custom type placed in agent state** or resume fails at decode time. Add a unit test that round-trips a real interrupted run through the store.

**Gotcha (from official docs):** gob serialization of custom state types requires registration; the v0.8+ migration also nudges middleware state to `adk.SetRunLocalValue`/`GetRunLocalValue` (serialization-safe) instead of `State.Extra`.

## (c) Frontend: Approval Interrupts Are Native in ai@7 / @ai-sdk/react

**No new dependencies. No transport redesign.** Our D-04 transport-swap invariant survives.

### Wire-level (UIMessageChunk) — what our Hertz SSE endpoint must emit

The `UIMessageChunk` discriminated union already contains:
- `tool-approval-request` (`{ type, approvalId, toolCallId }`) — emitted when Eino interrupts before a write tool
- `tool-approval-response` (`{ type, approvalId, approved }`) — carries the user's decision back
- Tool part terminal states: `output-available` (approved+executed) and `output-denied` (rejected)

### Client-level (@ai-sdk/react)

```tsx
const { messages, sendMessage, addToolApprovalResponse } = useChat({
  transport: new DefaultChatTransport({ api: '/api/chat' }),
  sendAutomaticallyWhen: lastAssistantMessageIsCompleteWithApprovalResponses,
});
// part.state === 'approval-requested' → render Approve/Deny →
// addToolApprovalResponse({ id: part.approval.id, approved: true|false })
```

### Integration mapping (Eino ↔ UIMessageChunk)

| Eino side | SSE chunk emitted | React side |
|-----------|-------------------|------------|
| Write tool wrapped with interrupt (`compose.WithInterruptBeforeNodes` or ADK tool-level interrupt) | `tool-input-available` + `tool-approval-request` | Tool part renders `approval-requested` state with Approve/Deny buttons |
| User clicks → `addToolApprovalResponse` | Client re-POSTs messages including approval response part | `sendAutomaticallyWhen: lastAssistantMessageIsCompleteWithApprovalResponses` auto-submits |
| Server extracts approval → `Runner.ResumeWithParams(ctx, checkpointID, ...)` | Stream resumes; on completion emit `tool-output-available` or `tool-output-denied` | Part transitions to `output-available`/`output-denied` |
| Audit row written (existing `store.QueryAudit` path) | optional `data-*` chunk with audit id | AuditTable already renders |

**New FE work is presentational only:** an `ApprovalCard` part component (mirror of VerdictCard/RefusalCard patterns from Phase 4) + one new branch in the transport mock fixtures. Update `transport-swap.test.tsx` fixtures to include approval chunks so the D-04 byte-level invariant covers the new chunk types.

## (d) Sub-Agent Fan-Out Observability: Framework Gives It Free

### What Eino provides (no new deps)

1. **`AgentEvent.RunPath []RunStep` + `AgentEvent.AgentName`** — auto-filled by the framework on every event from every sub-agent. This IS the fan-out trace: serialize `RunPath` into a `data-*` chunk → new `RoutingTracePart`-style UI already exists from Phase 4 (`DataSubprogressPart`, `RoutingTracePart`) to render per-sub-agent progress.
2. **`callbacks.HandlerBuilder` with `info.Component == adk.ComponentOfAgent`** — per-agent start/end/resume hooks; `adk.ConvAgentCallbackInput/Output` distinguishes fresh run vs resume-from-interrupt. Attach our existing audit writer here: every sub-agent invocation = one audit row (agent name, run path, parent session).
3. **Transfer events**: `AgentAction.TransferToAgent` / `Interrupted *InterruptInfo` are first-class on `AgentEvent.Action` — the planning-layer replay requirement (规划轨迹可回放) is served by persisting these action events per session.

### What to defer (explicitly NOT in this milestone)

| Deferred | Why |
|----------|-----|
| Langfuse / CozeLoop callback handlers (`eino-ext/callbacks/langfuse`, `callbacks/cozeloop`) | Both exist and are one-liner installs, but they require an external server (Langfuse) or a Volcano Engine account (CozeLoop) — violates local-first open-source posture. Our sqlite audit + RunPath chunks cover the demo story. Revisit post-open-source if users ask for OTLP. |
| OTel collector + Jaeger in compose | Same reason — ops weight without demo payoff. The `callbacks.HandlerBuilder` seam means adding OTLP later is additive, not a refactor. |

## Recommended Stack Additions (complete delta)

| Layer | Addition | Version | Why |
|-------|----------|---------|-----|
| BE core | `github.com/cloudwego/eino` | **v0.7.7 → v0.9.13** | Summarization middleware (v0.8+) + stable HITL + 13 ADK patch fixes |
| BE ext | `github.com/cloudwego/eino-ext` components in use | bump per-component to versions whose README declares eino ≥v0.9 compat | Lockstep with core bump |
| BE internal | `internal/store/sqlite/checkpoint.go` (new, ~100 LOC) | — | Implements `compose.CheckPointStore`; zero new external deps |
| BE internal | `internal/hitl/` (new): Eino interrupt → UIMessageChunk `tool-approval-request` mapper + resume handler | — | Bridges ADK HITL to existing SSE envelope |
| FE | `web/src/components/chat/parts/ApprovalPart.tsx` (new) | — | Approve/Deny card; reuses Phase-4 part-component pattern |
| FE deps | **none** | — | `addToolApprovalResponse` + approval chunk types ship in installed ai@7 |
| Observability | **none new** | — | RunPath/AgentName + HandlerBuilder → existing audit store |

## What NOT to Add

| Anti-choice | Why not | Instead |
|-------------|---------|---------|
| **eino v0.10.0-alpha.x** | Maintainers explicitly warn of breaking changes pre-final; durable runtime/automemory not needed for our 4 features | Pin v0.9.13; revisit v0.10 after GA (Q3 2026 target) |
| **Redis checkpoint store** | No official impl exists; Redis sidecar breaks single-node Compose story | sqlite CheckPointStore |
| **Langfuse/CozeLoop now** | External server/account dependency contradicts local-first OSS goal | sqlite audit + RunPath data-chunks |
| **agentic* model components (`eino-ext/components/model/agenticark` etc.)** | For `AgenticMessage` server-side tool calling; we stay on `*schema.Message` | Keep current model components, just version-bump |
| **Frontend HITL library / custom approval protocol** | ai@7 approval flow is native and matches our UIMessageChunk transport | `addToolApprovalResponse` |
| **Python eval sidecar revival for trajectory eval** | D6-Reversed decision stands (Go-native eval) | Extend `internal/eval` with approval-rate/trajectory metrics in Go |

## Validation Checklist for the Upgrade Phase (前置 phase)

1. `go get github.com/cloudwego/eino@v0.9.13` + bump eino-ext components → `go build ./...` + full unit suite
2. `integration/contract/` drift suite green (SSE envelope unchanged)
3. Re-run Phase-1 live verifies: cancellation <200ms SC, Milvus tier-filter SC
4. New: gob round-trip test for a real interrupted ChatModelAgent state through sqlite CheckPointStore
5. New: mock-transport fixture with `tool-approval-request`/`tool-approval-response` chunks passes `transport-swap.test.tsx`

## Sources

- [Eino v0.8 release notes — ADK middlewares (summarization middleware intro + breaking changes)](https://www.cloudwego.io/docs/eino/release_notes_and_migration/eino_v0.8._-adk_middlewares) — HIGH (official docs via Context7)
- [Eino v0.9 release notes — agentic-runtime + stable upgrade commands](https://www.cloudwego.io/docs/eino/release_notes_and_migration/eino_v0.9._agentic-runtime) — HIGH
- [Eino v0.9 migration notes — AgentMiddleware deprecation](https://www.cloudwego.io/docs/eino/release_notes_and_migration/eino_v0.9._agentic-runtime/eino_v0.9_migration_notes) — HIGH
- [Eino release notes index — v0.1..v0.9 line map](https://www.cloudwego.io/docs/eino/release_notes_and_migration) — HIGH
- [cloudwego/eino releases — v0.9.13 latest stable, v0.10.0-alpha.13 pre-release](https://github.com/cloudwego/eino/releases) — MEDIUM-HIGH (WebSearch of releases page)
- [Eino Discussion #1159 — v0.10 roadmap + alpha instability warning](https://github.com/cloudwego/eino/discussions/1159) — MEDIUM-HIGH
- [Summarization middleware config (TriggerCondition, TranscriptFilePath, Retry, Failover)](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/middleware_summarization) — HIGH
- [Eino ADK HITL — CheckPointStore interface, in-memory store, gob registration](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_hitl/) — HIGH
- [eino-ext HITL reference — "Use Redis in production", no official redis impl](https://github.com/cloudwego/eino-ext/blob/main/skills/eino-agent/reference/human-in-the-loop.md) — HIGH
- [xyncra-server RedisCheckPointStore (community pattern: bytes-KV, TTL, fail-closed)](https://pkg.go.dev/github.com/PineappleBond/xyncra-server/internal/agent) — LOW-MEDIUM (third-party reference only)
- [Vercel AI SDK — human-in-the-loop cookbook (`needsApproval`, `addToolApprovalResponse`, part states)](https://github.com/vercel/ai/blob/main/content/cookbook/01-next/75-human-in-the-loop.mdx) — HIGH
- [Vercel AI SDK — tool approvals agent docs](https://github.com/vercel/ai/blob/main/content/docs/03-agents/06-tool-approvals.mdx) — HIGH
- [UIMessageChunk union — `tool-approval-request`/`tool-approval-response` chunk types](https://github.com/vercel/ai) — HIGH (via Context7 type extraction)
- [Eino ADK agent callbacks — HandlerBuilder, ComponentOfAgent, ConvAgentCallbackInput](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/adk_agent_callback) — HIGH
- [Eino AgentEvent structure — AgentName, RunPath, Action.TransferToAgent/Interrupted](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_interface) — HIGH
- [eino-ext langfuse callback](https://github.com/cloudwego/eino-ext/blob/main/callbacks/langfuse/README.md) / [cozeloop callback](https://github.com/cloudwego/eino-ext/blob/main/components/prompt/cozeloop/README.md) — HIGH (existence), deferred by decision

## Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| eino upgrade target (v0.9.13) | HIGH | Official release notes index + releases page agree; alpha warning is from maintainers themselves |
| Checkpoint store (build sqlite) | HIGH | Official interface docs + confirmed absence of official persistent impl; interface is 2 methods |
| FE approval support | HIGH | Context7-extracted official cookbook + UIMessageChunk type union both confirm |
| Fan-out observability | HIGH | Official AgentEvent/callback docs; no external claims needed |
| Exact v0.7.7→v0.9.13 compile breakage surface in OUR code | MEDIUM | Breaking-change lists verified, but which ones bite requires the upgrade phase build — flagged as phase research/validation task |
