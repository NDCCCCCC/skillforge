# Architecture: SkillForge v0.8 Harness — 可信×行动融合

**Project:** SkillForge — Skill-driven Enterprise Agent Platform
**Researched:** 2026-08-17
**Milestone:** v0.8 Harness (multi-turn memory · write tools + approval gate · DeepAgent planning · sub-agent fan-out)
**Base:** v0.7.7 codebase (single-skill ReAct agent, trust floor + critique, Hertz SSE + ai@7 UIMessageChunk, sqlite session/audit)
**Confidence:** HIGH on layering decisions and trust-floor containment; MEDIUM on Eino ADK sub-agent ctx propagation and CheckpointStore sqlite specifics (flagged for phase spikes)

> This document EXTENDS `.planning/research/ARCHITECTURE.md` (v0.7.7 research). It does not replace it. Section numbers here are scoped to the v0.8 milestone.

---

## 1. Executive Summary

The v0.8 milestone adds four harness capabilities onto a trust-anchored Q&A kernel. The single most important architectural fact: **the existing trust anchors are already structurally history-safe and sub-agent-safe, because they are keyed to a per-Run `ChunkAccumulator`, not to the message list.** `ApplyTrustFloor(acc, query, tiers)` and `GroundingChecker(v, acc.ChunksByID())` (`internal/agent/agent.go:532,553`) only know about chunks retrieved **during the current Run**. Any citation a history-polluted LLM invents from prior turns fails grounding and gets stripped; any turn with zero current retrievals is forced to refusal. History therefore becomes a *behavioral* risk (LLM confusion, refusal-rate drift) — which the planned golden-set spike measures — not a *structural* hole.

The four capabilities map onto four new seams:

| Capability | Architectural seam | New vs modified |
|---|---|---|
| Multi-turn memory | Server-layer history assembler feeding `[]*schema.Message` into agent; summarization middleware inside the runner | NEW `internal/history/`; MODIFY `internal/server/chat.go`, `internal/agent/agent.go` |
| Write tools + approval gate | Eino Interrupt/Resume + sqlite CheckPointStore + resume endpoint streaming the same UIMessageChunk protocol | NEW `internal/approval/`, `internal/store/sqlite/checkpoint.go`, `internal/server/resume.go`; MODIFY `internal/wire/chunk.go`, `web/` parts |
| Planning layer (`agent: deep`) | Per-Skill agent factory dispatching on `Manifest.Agent` (field already exists, currently ignored); shared post-run trust pipeline | NEW `internal/runtime/` (factory); REFACTOR trust anchors out of `(*Agent).Run` into shared stage |
| Sub-agent fan-out | Sub-agents constructed at build time from the parent manifest; retrieve tools close over the same resolved tiers and the same accumulator | MODIFY factory + retrieve tool construction; NO new tier-enforcement mechanism needed |

**Build order (dependency-driven):** deps upgrade → history spike → multi-turn memory → approval gate → DeepAgent → sub-agents. This matches the user-decided pillar order in `.planning/notes/harness-roadmap-decision.md` with the eino upgrade inserted as phase 0 (summarization middleware + ADK HITL require v0.8.x-era eino).

---

## 2. Current State → Target State

### 2.1 Current state (verified against code)

```
POST /api/chat (internal/server/chat.go chatHandler)
  ├─ decode → lastUserQuery (ai@7 {messages} body)
  ├─ session_id resolve (X-Session-Id header / crypto/rand)
  ├─ route: Router (Plan 05-02) OR findSkill fallback
  ├─ branch:
  │   ├─ s.Agent != nil  → runAgentPath (OPT-IN, USE_AGENT_PATH)
  │   │     → Agent.Run(ctx, sessionID, query)      ← SINGLE-TURN
  │   │        buildAgentMessages = [system, user]  ← (agent.go:623)
  │   │        pre-retrieve → ReAct loop → trust floor → grounding → critique
  │   └─ default         → VerdictPipeline (production default)
  └─ writeAuditAndTurn → audit row + session_turn (user, assistant) pair
         assistant content = verdictJSON or refusal reason  ← NOT human-readable
```

Key facts that constrain v0.8:

- `internal/server/chat.go:399` — session history is loaded for the SC-9 isolation test but **deliberately not fed** to the agent ("multi-turn conversation memory feeding is Phase 5+", deferred to this milestone).
- `internal/store/sqlite` — `session_turn` table stores `role CHECK(role IN ('user','assistant'))` + content + ts; assistant content is **verdict JSON or refusal reason string**, not the rendered conclusion sentence.
- `internal/agent/agent.go` — `Run(ctx, sessionID, query)` builds exactly `[system, user]` per call; the trust anchors (floor, grounding, critique) all read the per-Run `ChunkAccumulator`.
- `Manifest.Agent` field (`internal/skill/manifest.go:59`) already exists and validates as an enum slot, but nothing dispatches on it — `NewAgent` hardcodes the ReAct archetype.
- eino pinned at **v0.7.7** in `go.mod`; ADK Runner is NOT used on the agent path (documented API adaptation in agent.go package godoc).

### 2.2 Target state (v0.8)

```
POST /api/chat                          POST /api/chat/resume   (NEW)
  │                                        │
  ▼                                        ▼
chatHandler                          resumeHandler
  │ 1. history assemble                    │ 1. load checkpoint (sqlite)
  │    (internal/history)                  │ 2. Runner.ResumeWithParams
  ▼                                        │    (+ HistoryModifier)
runtime.RunnerFor(manifest)  (NEW)         ▼
  │  switch manifest.Agent:                adk.Runner ──► interrupt? ──► data-approval
  │    "chat" → react agent  ──────────────┤     (both endpoints)        SSE + checkpoint save
  │    "deep" → deep agent  ───────────────┤                            + audit row
  ▼                                        ▼
shared post-run trust pipeline (REFACTORED out of agent.go)
  trust floor → grounding → critique     (input: per-Run ChunkAccumulator ONLY)
  │
  ▼
SSE UIMessageChunk stream (+ data-approval part, NEW wire type)
```

---

## 3. Question (a): History Assembly — Server Layer, Trust Floor Containment

### 3.1 Decision: history assembly lives in the SERVER layer, not the agent layer

**Recommendation: NEW package `internal/history`, owned by the server layer.**

```
internal/server/chat.go
  └─► history.Assemble(ctx, store, sessionID, budget) []schema.Message   (NEW)
        └─► passed INTO the agent as a parameter
internal/agent/agent.go
  └─► Run(ctx, sessionID, query, history []schema.Message)               (MODIFIED signature)
```

Rationale:

1. **The app owns the session store** — this is the confirmed Eino fact ("Runner.Run takes full message list per turn; app owns session store"). The sqlite `session_turn` table is already that store. Assembly is a store-projection concern (which rows, which format, how much budget), not an agent-reasoning concern.
2. **The agent stays a pure function of (manifest, tools, messages).** This preserves the Plan 03-06 eval sidecar path (batch, no history) and the `USE_AGENT_PATH` test matrix: callers that pass `nil` history get exactly today's single-turn behavior. History becomes additive, never load-bearing.
3. **The prompt-hash invariant stays computable server-side.** `computePromptHash(skillName, modelVersion, renderedPromptBody)` (`chat.go:605`) hashes the *system prompt* only. History must NOT enter the hash — it varies per turn and would destroy the frozen-pin semantics (SC-6). Server-side assembly makes the separation explicit: hash covers `systemPrompt(manifest)`; history is conversation state, not configuration.

### 3.2 The assistant-content projection problem (must-fix)

`writeAuditAndTurn` (`chat.go:522`) currently stores the assistant turn as **raw verdict JSON** (or a refusal reason string). Feeding that verbatim into history would give the LLM a JSON-shaped conversation that (a) teaches it to converse in JSON, (b) wastes tokens, (c) leaks `[CHUNK id]` markers from prior turns into the context — the single biggest history-pollution vector (see 3.3).

**Fix: project at assembly time, not at write time (preserves T-3-AUDIT INSERT-only).** `history.Assemble` converts each stored turn:

| Stored assistant content | History projection |
|---|---|
| Verdict JSON | `conclusionText(v)` — the deterministic renderer ALREADY in `chat.go:793` (lift it to `internal/verdict` or `internal/history` to avoid a server↔history import tangle) |
| Refusal reason | `refusalText(Refusal{Reason: reason})` — already deterministic (`chat.go:859`) |
| Approval-gated action rows (new in v0.8) | `审批已通过/已拒绝：{action summary}` — see §4 |

**Rule: `[CHUNK id]` markers and chunk spans NEVER appear in projected history.** The projection emits the conclusion sentence and citation count ("依据权威 tier 2 条"), not the spans. This is the cheap, deterministic 80% of the pollution mitigation — the LLM physically cannot copy a citation it never sees.

### 3.3 How the trust floor stays history-safe

The containment is **structural, already present**, and needs no new mechanism — but it must be *preserved* deliberately:

| Anchor | Input today | History safety |
|---|---|---|
| Trust floor `ApplyTrustFloor` (agent.go:532) | per-Run `ChunkAccumulator` | History rows are never `acc.Add`-ed. The pre-retrieve + agent retrieve calls are the ONLY accumulator writers. A turn with zero current retrieval → forced refusal, no matter what history says. **Invariant: `accCtxKey` accumulator is created fresh per Run (agent.go:465) — do NOT refactor it into session scope.** |
| Grounding `GroundingChecker(v, acc.ChunksByID())` (agent.go:553) | current-Run chunks by ID | A citation whose chunk_id is not in the current accumulator fails grounding → span stripped → confidence downgrade. History-carried citations die here automatically. |
| Critique `MaybeCritique` (agent.go:557) | current-Run `ChunksByID` | Critique compares the verdict against current chunks only. Additive-strictness-only property (TestCritiqueCannotRaiseConfidence) means history cannot inflate confidence through this path. |

The residual risk is **behavioral**: the LLM sees prior turns and may answer from memory, produce well-formed verdicts with no current grounding (floor fires → refusal-rate inflation), or merge prior-turn facts into the conclusion (grounding strips citations → confidence collapse → `review_required` spam). Both failure modes are *measurable on the golden set* — exactly what the planned spike (`spike-history-injection-trust-floor`) compares against baseline on the four frozen metrics.

**Spike success criteria (proposed):** faithfulness / answer_relevancy / context_precision / context_recall drift < ±2 points, and refusal-rate delta on the 50-question frozen set < 10%, comparing `history=nil` vs `history=projected`. If the spike fails, the escape hatches (in order of preference): (1) shrink history budget to last-3-turns; (2) system-prompt addendum "prior turns are CONTEXT not EVIDENCE — every verdict must cite chunks retrieved THIS turn"; (3) drop assistant turns from history entirely (user-turns-only memory).

### 3.4 Summarization middleware placement

Confirmed fact: `adk/middlewares/summarization` (token-threshold-triggered) requires eino v0.8.x. Two integration points:

- **Short term (multi-turn phase):** server-layer `history.Assemble` applies a dumb token budget (last-N turns, hard cap) — no middleware needed. Ship multi-turn on the dumb budget first; it is deterministic, testable, and keeps the spike clean (one variable at a time).
- **Medium term:** when the runner migration lands (§5), the summarization middleware attaches **inside the runner**, between the assembled history and the model. A summarized prefix is still projected content (no chunk markers survive summarization if the projection in 3.2 ran first — order matters: **project → then summarize**, never summarize raw JSON turns).

**HistoryModifier caution:** the same HistoryModifier mechanism is used on the approval-resume path (§4.4). One modifier pipeline, two call sites — define it once in `internal/history` and inject it into both `Run` and `ResumeWithParams`.

---

## 4. Question (b): Approval Interrupt Round-Trip over Hertz SSE + ai@7

### 4.1 The round-trip shape

```
Turn N (POST /api/chat):
  user query → agent runs → write tool invoked
    → tool calls compose.Interrupt(ctx, approvalInfo)         [confirmed: v0.7.0+]
    → compose graph suspends; checkpoint SAVED (CheckPointStore)
    → server catches ExtractInterruptInfo(err)
    → SSE emits: data-approval {action_id, checkpoint_id, tool, args_summary, tier_target}
    → SSE emits: finish-step / finish                          [stream CLOSES cleanly]
    → audit row: refusal_reason="action_pending_approval", action JSON in new column

User clicks 批准 / 拒绝 in ApprovalCard (NEW web part):
  POST /api/chat/resume {session_id, checkpoint_id, action_id, decision, comment?}
    → resumeHandler loads checkpoint from sqlite
    → Runner.ResumeWithParams(ctx, checkpointID, &ResumeParams{...})  [confirmed]
    → agent CONTINUES from the interrupt point
    → response streams as a NEW SSE response (same UIMessageChunk protocol)
    → audit row: "action_approved" / "action_rejected" + executor result
```

Two deliberate design choices:

1. **The interrupt stream CLOSES; resume is a NEW HTTP request.** Do not hold the SSE socket open across the human approval (minutes-to-hours). The FE renders an ApprovalCard from the `data-approval` part; the card's buttons POST to `/api/chat/resume` and consume the resume response **through the same custom transport** so the continuation lands in the same `useChat` message list. ai@7's transport abstraction supports this: the resume fetch is just another streaming response appended to the conversation. (Validate in phase spike: whether the custom transport needs a `resume` method or can reuse the chat endpoint URL with a different body shape. The cleanest is a separate endpoint + transport branch on request body kind.)
2. **`checkpoint_id` is server-generated and session-scoped.** The client never constructs it. Format: `{sessionID}:{actionULID}` — this makes "list pending approvals for this session" a trivial `WHERE checkpoint_id LIKE 'sess:%'` query and gives the SC-9 isolation boundary for free.

### 4.2 Checkpoint store: sqlite, same DB file, new table

**Decision: implement `compose.CheckPointStore` against the existing sqlite store. Do NOT use an in-memory store, do NOT add Redis.**

```sql
-- internal/store/sqlite/schema.go (MODIFIED — additive table)
CREATE TABLE IF NOT EXISTS checkpoint (
    id          TEXT PRIMARY KEY,            -- {session_id}:{action_id}
    session_id  TEXT NOT NULL,
    skill_id    TEXT NOT NULL,
    payload     BLOB NOT NULL,               -- serialized compose checkpoint
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at  TEXT NOT NULL                -- checkpoint TTL (e.g. 24h)
);
CREATE INDEX IF NOT EXISTS idx_checkpoint_session ON checkpoint(session_id);
```

Rationale: single-file ops story (matches the "zero-ops SQLite default" posture of `internal/store/store.go:125`), checkpoints survive server restart (an approval pending across a deploy must not vanish — that is the whole point of an auditable action gate), and the `session_id` column reuses the SC-9 partition discipline. Confidence: HIGH on the choice; MEDIUM on the exact `CheckPointStore` interface method signatures (Save/Load shape) — phase task should open with a 30-minute compile-check spike against the upgraded eino version, mirroring how Phase 3 locked `react.AgentConfig.ToolCallingModel` by file:line citation.

**TTL + cleanup:** expired checkpoints are NOT deleted on a schedule (INSERT-only posture on audit does not extend to checkpoints — checkpoints are mutable workflow state, not audit records). A lazy-delete on read (`WHERE expires_at < now` → treat as absent) plus a startup sweep is sufficient at this scale.

### 4.3 Audit: one row per action, three-row lifecycle

The milestone demands "audit row per action". Recommended lifecycle, all INSERT-only (T-3-AUDIT preserved):

| Event | `refusal_reason` value | New column content |
|---|---|---|
| Tool interrupted, approval requested | `action_pending_approval` | action JSON: `{action_id, tool, args_redacted, checkpoint_id}` |
| User approved, action executed | `action_approved` | same action_id + executor result summary |
| User rejected | `action_rejected` | same action_id + comment |

Add ONE column rather than a new table: `ALTER TABLE audit ADD COLUMN action_json TEXT` (nullable; NULL for all pre-v0.8 rows — sqlite ALTER ADD COLUMN is append-safe and keeps the denormalized-query posture of `AuditRow`). The `/api/audit` UI gains an "action" filter value. Reusing `refusal_reason` as the lifecycle discriminator keeps `AuditFilter` unchanged; a dedicated `action_id` index column is a Phase-9 optimization, not v0.8 scope.

### 4.4 New wire part + FE part

- **NEW wire type `data-approval`** in `internal/wire/chunk.go`, added to the `SkillForgeChunk` union — the compile-time drift alarm (`AssertAssignable<SkillForgeChunk, UIMessageChunk>`) guards it exactly like `data-decision` and `data-policy`.
- **NEW FE part** `web/src/components/chat/parts/ApprovalPart.tsx` — renders action summary + tier target + approve/reject buttons; on click, POSTs to `/api/chat/resume` through the transport; renders the resumed stream's verdict inline. Pitfall 7 invariant (`useChat` owns the message stream; Zustand never stores `messages`) is preserved: pending-approval state is *derived from the message's parts*, not stored in the app store.
- **HistoryModifier on resume:** when the user rejects, the resumed run must not see the tool call as "executed". Use `ChatModelAgentResumeData.HistoryModifier` [confirmed] to rewrite the pending tool call into "user rejected this action — do not retry; explain the rejection" so the agent produces a clean terminal answer instead of looping the interrupt.

---

## 5. Question (c): `agent: chat` vs `agent: deep` Co-existence

### 5.1 Decision: factory dispatch on the existing `Manifest.Agent` field

The field already exists and validates (`internal/skill/manifest.go:59`, enum slot 2). Nothing dispatches on it today — `NewAgent` hardcodes ReAct. The v0.8 change:

```
NEW internal/runtime/factory.go:

  func BuildAgent(ctx, manifest, deps) (RunnableAgent, error) {
      switch manifest.Agent {
      case "chat", "":  return agent.NewAgent(ctx, agentCfg)        // existing path
      case "deep":      return deep.New(ctx, deepCfg(manifest, deps)) // NEW
      default:          return nil, ErrUnknownAgentType              // loader Validate already bounds this
      }
  }
```

`RunnableAgent` is a narrow interface (`.Name()`, `.Run/Generate` adapted) so `chatHandler` holds one field, not two. **Do not branch on `manifest.Agent` inside chatHandler** — the handler already has two branches (Agent vs VerdictPipeline); adding a third dimension there repeats the Plan 03-05 closure-capture wiring twice.

### 5.2 The trust anchors must be LIFTED out of `(*Agent).Run`

Today floor + grounding + critique live *inside* `agent.go` Run (steps 7/9/10). A DeepAgent run does not pass through that code. v0.8 refactor:

```
NEW internal/agent/truststage.go (or trust_pipeline.go):

  type TrustInput struct {
      Acc      *ChunkAccumulator   // per-Run, shared across sub-agents (§6)
      Manifest *skill.Manifest
      Raw      string              // final model message
      Query    string
  }
  func ApplyTrustStage(ctx, in TrustInput) (verdict.Verdict, *verdict.Refusal, error)
      // = today's ApplyTrustFloor → parse → GroundingChecker → MaybeCritique
```

`(*Agent).Run` becomes a thin caller of `ApplyTrustStage`; the deep-agent runner calls the same stage on its final message. **The regression locks move with the code**: `TestTrustFloorForcesRefusal`, `TestCritiqueCannotRaiseConfidence` retarget the stage. This is the load-bearing refactor of the milestone — everything else hangs off "every archetype exits through the same trust stage".

### 5.3 Verdict contract on the deep path

DeepAgent (confirmed: `adk/prebuilt/deep`, `write_todos` built in) produces free-form final text. SkillForge's wire contract is a `verdict.Verdict` JSON. Bridge:

- The deep agent's system prompt gets the same verdict-JSON suffix as `systemPrompt(m)` (agent.go:690) — plus its planning preamble. Skills with `agent: deep` must keep `guardrails.require_explicit_citations: true` so the contract holds.
- If the deep run's final message fails verdict parse, map to the existing `forced_stop` path (today: `emitForcedStop` + error at agent.go:548) rather than inventing a new failure mode.
- **Replayable plan trace:** `write_todos` state transitions (pending/in_progress/completed) are ADK events — surface them as `data-decision` chunks with `decision: "plan"` and the todo payload (reuses D5-04's "extend data-decision, no new wire type" precedent). The audit row gains the final todo list inside `action_json` or `verdict_json` so a plan is replayable from the audit trail alone. This is the "规划轨迹可回放" deliverable at minimal wire cost.

### 5.4 Runner migration: do it ONCE, at the deep phase

The confirmed facts point everything at `adk.NewRunner` (HITL + checkpoints + summarization middleware all hang off Runner). But today's agent path deliberately bypasses Runner (agent.go package godoc: `react.Agent` did not satisfy `adk.Agent` at v0.7.7). Recommended sequencing:

- Multi-turn phase: keep `react.Generate` — history is just a longer `msgs` slice; Runner is NOT required for memory. (Avoids coupling the riskiest behavioral spike to a framework migration.)
- Approval phase: Runner becomes mandatory (RunnerConfig.CheckPointStore + ResumeWithParams). Migrate the chat archetype onto Runner HERE, under the post-upgrade eino version, with `TestCancelPropagationMidTool` (<200ms) re-locked.
- Deep phase: deep agent slots into the already-Runner-shaped harness.

---

## 6. Question (d): Sub-Agent Tier Inheritance

### 6.1 Decision: build-time closure inheritance — the existing pattern, applied recursively

The current tier boundary is enforced by **closure**, not by runtime checks: `BuildRetrieveTool(embedder, retriever, manifest)` closes over `manifest.ResolvedTiers()` (agent.go:223; KNOW-03 "the agent LLM cannot override tiers"). Sub-agents inherit by receiving **the same closure recipe**:

```go
// In the per-Skill agent factory (internal/runtime):
subAgentCfgs := manifest.SubAgents()           // v0.8: from SKILL.md body or defaults
for _, sc := range subAgentCfgs {
    tool := agent.BuildRetrieveTool(embedder, retriever, manifest) // PARENT manifest
    // child never gets its own manifest.ResolvedTiers() — it gets the parent's tool
    sub := adk.NewChatModelAgent(ctx, adk.ChatModelAgentConfig{
        Model: cfg.ChatModel, Tools: []tool.BaseTool{tool}, ...
    })
}
```

**Rules:**

1. **Exact inheritance, no narrowing, no widening (v0.8).** Child tiers == parent `ResolvedTiers()`. Per-sub-agent tier narrowing is a later refinement; widening is structurally impossible because the child never sees a manifest — it sees a tool whose tier slice is already closed.
2. **Sub-agent declarations live in SKILL.md but resolve through the parent.** If a SKILL.md body names sub-skills, the factory intersects their `reads` with the parent's before building; an intersection that is empty = load-time validation error (fail fast, mirroring `NewAgent`'s nil-field posture).
3. **One accumulator per Run, shared across sub-agents.** The `accCtxKey` pattern (agent.go:368) threads the per-Run `ChunkAccumulator` through ctx. Sub-agent retrieve tools must read the SAME accumulator so the trust floor sees the union of all hops — parent and children. **RESEARCH FLAG (MEDIUM confidence): verify that ADK transfer/handoff propagates ctx values into sub-agent tool invocation.** If it does not, the fallback is explicit: pass the accumulator into the sub-agent tool closures at construction time (build agents per-Run instead of per-process — more expensive; `react.NewAgent` graph construction cost is why the ctx pattern exists, so prefer ctx and validate early).
4. **Audit attribution:** sub-agent retrieve calls log under the parent's `skill_id` with the hop/decision metadata already flowing through `data-decision`. No new audit shape needed; the `data-decision` payload's `Step` counter just counts higher.

### 6.2 What sub-agents are FOR in v0.8

Keep the first use narrow: **parallel multi-hop retrieval** (fan-out N query rewrites → N retrieve calls → merge), which maps to the confirmed `NewParallelAgent` primitive and exercises exactly one new thing (ctx/accumulator propagation). Do not combine the first sub-agent phase with deep planning; sequential composition after fan-out is already proven compositional ground.

---

## 7. New vs Modified — File-Level Inventory

### NEW files

| File | Responsibility |
|---|---|
| `internal/history/assemble.go` | session_turn → `[]schema.Message` projection (verdict JSON → `conclusionText`; refusal → `refusalText`); token budget; chunk-marker stripping |
| `internal/history/assemble_test.go` | projection table tests; budget tests; "no [CHUNK in history" invariant test |
| `internal/runtime/factory.go` | `Manifest.Agent` dispatch (chat / deep); shared deps bundle |
| `internal/agent/truststage.go` | lifted trust floor + grounding + critique stage (moved FROM agent.go Run steps 7/9/10) |
| `internal/approval/tool.go` | write-tool wrapper emitting `compose.Interrupt` with ApprovalInfo; executor for post-approval completion |
| `internal/store/sqlite/checkpoint.go` | `compose.CheckPointStore` impl + `checkpoint` table |
| `internal/server/resume.go` | `POST /api/chat/resume` handler |
| `web/src/components/chat/parts/ApprovalPart.tsx` (+test) | approval card; resume POST through transport |
| `web/src/components/chat/parts/PlanTracePart.tsx` (+test) | replayable plan/todo rendering from `data-decision{decision:"plan"}` |

### MODIFIED files

| File | Change |
|---|---|
| `internal/agent/agent.go` | `Run` gains `history []schema.Message` param; steps 7/9/10 extracted to truststage; buildAgentMessages splices history between system and current user turn |
| `internal/server/chat.go` | call `history.Assemble`; route resume-capable runs through Runner (approval phase); lift `conclusionText`/`refusalText` to a shared package (history imports them) |
| `internal/server/server.go` | Deps gains `CheckpointStore`, `ApprovalExecutor`; route registration for `/api/chat/resume` |
| `internal/store/store.go` + `internal/store/sqlite/audit.go` | `AuditRow.ActionJSON` column; checkpoint CRUD on Store interface |
| `internal/wire/chunk.go` | `data-approval` chunk type; `data-decision` plan payload variant |
| `internal/skill/validate.go` | `agent: deep` allowed value (if not already); sub-agent declaration validation |
| `skills/compliance/SKILL.md` | first `writes` declaration (memory:facts / report generation) + approval guardrail entry |
| `web/src/types/` + transport | `SkillForgeChunk` union + `AssertAssignable` extended; transport resume branch |
| `go.mod` | eino v0.7.7 → v0.8.x-era (summarization + ADK HITL); re-lock all file:line API citations in godocs |

---

## 8. Suggested Build Order (Dependency-Ordered)

| # | Phase | Deliverable | Unblocks | Depends on |
|---|---|---|---|---|
| 0 | **Deps upgrade** | eino v0.8.x-era + eino-ext refresh; all godoc file:line citations re-verified; green suite | everything (summarization, HITL) | — |
| 1 | **History spike** | golden-set A/B: `history=nil` vs projected history; four-metric drift + refusal-rate delta | go/no-go on phase 2; escape-hatch selection | 0 |
| 2 | **Multi-turn memory** | `internal/history` + `Run` history param + `chat.go` wiring; dumb token budget | approval UX (resume replays full turns), deep planning (plans span turns) | 1 (spike pass) |
| 3 | **Approval gate** | Runner migration (chat archetype) + sqlite CheckPointStore + interrupt/resume endpoints + ApprovalPart + 3-row audit lifecycle | deep agent (Runner-shaped harness), any write tool | 0, 2 |
| 4 | **DeepAgent planning** | factory dispatch + truststage refactor + deep verdict bridge + plan trace as `data-decision` | sub-agents (composition patterns) | 3 |
| 5 | **Sub-agent fan-out** | parallel multi-hop retrieval; ctx/accumulator propagation spike first; audit attribution | — | 4 (or 3, if scoped to parallel retrieval only) |

**Ordering rationale:**

- Upgrade first because two confirmed capabilities (summarization middleware, ADK HITL ergonomics) simply do not exist at v0.7.7, and every subsequent phase would pay a double-migration tax.
- History before approval because the resume round-trip replays the full message list — shipping approval on single-turn memory would build the resume machinery on a foundation that immediately gets rebuilt.
- Approval before deep because the Runner migration is the riskiest framework change; doing it on the simple ReAct archetype (with the full existing regression suite as a safety net) is strictly safer than landing it together with a new agent type.
- Deep before sub-agents because deep's `write_todos` + sub-agent slots are the natural composition host; parallel-retrieval fan-out without planning is the fallback if deep slips.
- Every phase ends with a "可信税" artifact: history→spike report; approval→3-row audit lifecycle; deep→replayable plan in audit row; sub-agents→tier-inheritance proof test.

---

## 9. Risks / Research Flags for Phases

| Flag | Phase | Question | Fallback |
|---|---|---|---|
| RF-1 (spike planned) | 1 | History injection refusal-rate drift on frozen set | last-3-turns budget → prompt addendum → user-turns-only memory |
| RF-2 | 3 | Exact `compose.CheckPointStore` interface shape post-upgrade | 30-min compile-check spike, cite file:line like Phase 3 did |
| RF-3 | 3 | ai@7 custom transport: can the resume response reuse the same message list without a synthetic user message? | FE appends a hidden "审批结果" user turn to anchor the resumed assistant message |
| RF-4 | 5 | Does ADK transfer/handoff propagate `context.Value` into sub-agent tool closures? | Per-Run agent construction with accumulator baked into closures |
| RF-5 | 4 | DeepAgent final-message verdict-parse rate on GLM-5.2 | structured-output model option (spike-003 shape) on the final step |
| RF-6 | 2 | Summarization middleware behavior on projected history (does it re-introduce raw JSON?) | project-then-summarize ordering; middleware config audit in phase plan |

---

## 10. Sources

- Codebase (HIGH): `internal/agent/agent.go` (trust anchors, accCtxKey/emitCtxKey patterns, single-turn buildAgentMessages), `internal/server/chat.go` (chatHandler, writeAuditAndTurn, conclusionText/refusalText renderers), `internal/server/server.go` (Deps wiring posture), `internal/store/store.go` (INSERT-only audit, SessionMessage shape), `internal/skill/manifest.go` (Agent field, ResolvedTiers closure seam), `internal/store/sqlite/schema.go` (session_turn CHECK constraint)
- `.planning/notes/harness-roadmap-decision.md` (HIGH): confirmed Eino facts — Interrupt/Resume/CheckPointStore v0.7.0+; adk.NewRunner HITL + ResumeWithParams + HistoryModifier; deep.New + write_todos; summarization middleware v0.8.x-era; Runner.Run full-message-list-per-turn
- `.planning/PROJECT.md` (HIGH): milestone goal, 可信税 principle, open-source deferral
- `.planning/research/ARCHITECTURE.md` (MEDIUM — v0.7.7-era research): six-layer model, Skill→ADK mapping table (chat/deep/router), approval middleware sketch (§3.7); note its WebSocket recommendation was superseded by the shipped SSE-only transport — v0.8 stays SSE + REST resume
- RF-3/RF-4 marked MEDIUM/LOW pending phase spikes; everything labeled "confirmed" traces to harness-roadmap-decision.md's 2026-08-17 verification

---

*Last updated: 2026-08-17 — v0.8 Harness milestone research*
