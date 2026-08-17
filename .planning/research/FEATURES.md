# Feature Research

**Domain:** Trusted-action agent harness (multi-turn memory, HITL write approval, planning, sub-agent fan-out) for enterprise compliance Q&A
**Researched:** 2026-08-17
**Confidence:** MEDIUM-HIGH (primary sources: Anthropic engineering blog, LangChain official docs, Dify official blog/docs, Claude Code permission guides; MEDIUM on ChatGPT/Gemini UI specifics via secondary sources)

## Feature Landscape

Cross-product baseline: Claude Code (permission model, TodoWrite/Tasks, Task sub-agents), LangGraph (interrupt/Resume HITL middleware, checkpointer), ChatGPT Deep Research / agent mode (plan narration, confirmation before consequential actions), Gemini Deep Research (plan approval gate before execution), Dify/Coze (memory windows, weak HITL — a gap, not a bar).

---

## Capability 1: Multi-Turn Conversation Memory

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| ChatGPT / Claude.ai | Full history in-window; automatic summarization/compaction for long conversations; "new chat" = clean slate; anaphora ("the second one") just works |
| Dify | `TokenBufferMemory` sliding window, session-scoped, hard backend caps (2,000 tokens / 500 messages); no built-in summarization; long-term via knowledge base or Conversation Variables |
| Coze | Basic memory only — weakest of the set |
| Claude Code | Session history + auto-compaction at token threshold; persistent memory via files (CLAUDE.md), not chat history |
| LangGraph | Checkpointer for in-thread state; Store for cross-thread long-term memory; official summarization middleware |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Anaphora/follow-up resolution ("那第二条呢") | ChatGPT-grade baseline; broken follow-ups feel like a toy | MEDIUM | History must reach the ReAct agent; fixes `chat.go` single-turn limit |
| History survives page refresh | Every chat product does this | LOW | sqlite session table already exists (Phase 6 SC-4 proved 50×5 isolation) |
| Context-window management (window + summarization, never silent truncation) | Users punish "loses the thread" behavior | MEDIUM | Eino `adk/middlewares/summarization` (v0.8.x) — requires dep upgrade |
| "New conversation" affordance | Mental model for clean scope / privacy | LOW | SessionPicker already exists in UI |

### Differentiators (Competitive Advantage — trusted angle)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Trust-floor regression gate on history injection | Nobody else proves memory doesn't degrade refusal/citation behavior; golden-set A/B (baseline vs history-injected four metrics) | MEDIUM | This IS the milestone's spike seed; uses existing frozen eval set + gate |
| Compaction transparency marker | UI shows "earlier turns summarized" — auditable context | LOW | Natural extension of DataSubprogressPart |
| Tier-aware memory | Personal-tier facts enter memory only via approved writes; memory reads respect tier boundary | MEDIUM | Ties Capability 1 to Capability 2; leverages existing `reads`/`writes` spec |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Cross-session semantic long-term memory (Mem0-style user profile) | "Remember me across chats" | Enterprise data-residency/privacy risk; scope creep; Dify delegates this to plugins for a reason | Defer to v2; session-scoped memory is enough for the milestone |
| Vectorizing chat history into knowledge tiers | "Search past conversations like docs" | Pollutes authority tier; breaks true-source separation (the moat) | Memory stays in session store, never in Milvus/Mongo tiers |
| History as citation evidence | Agent "remembers" a fact from turn 3 and cites it | Hallucination amplifier; grounding checker can't verify against chunks | History = context only; citations still require retrieval; grounding checker unchanged |

---

## Capability 2: Write Tools + Human Approval Gate

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| Claude Code | Three-rule model: `allow` / `ask` / `deny`, deny always wins, first match wins; modes (`default`, `acceptEdits`, `plan`, `bypassPermissions`); prompt shows tool + key args; options: Allow once / Always allow (scope) / Deny |
| LangGraph HITL middleware | Four decision types: **approve** / **edit** (modify args before execution) / **reject** (feedback to agent) / **respond** (synthetic tool result — only for ask-user tools); checkpointer makes interrupts survive refresh; UX guidance: show full args + why, make approve one click, validate edits, log all decisions, set timeouts, keep resolved cards on screen |
| ChatGPT agent mode | Live "watch it work" view; confirmation before consequential/irreversible actions |
| Dify / Coze | Tool on/off toggles, no fine-grained per-call approval UX — **this is the gap SkillForge exploits** |

Known weaknesses to avoid (from Claude Code issue tracker): whole-string matching lets compound commands bypass allow-patterns; "Always allow" as default is a dangerous UX regression; "don't ask again" persistence bugs erode trust. LangGraph practitioner gotcha: interrupt placed **after** a non-deterministic step means resume re-runs it and executes something different from what was approved — gate **before** the action, persist inputs.

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Approval card shows what/tool/full-args/why | LangChain docs #1 UX rule; Claude Code issue consensus | LOW | Reuses ToolPart.tsx rendering patterns |
| Three decisions minimum: approve once / reject (with reason) / approve-and-remember | Claude Code baseline | LOW-MEDIUM | "Remember" = scoped rule, not global |
| Approval state survives refresh (checkpoint) | LangGraph checkpointer baseline; users refresh mid-approval constantly | MEDIUM | Eino `compose.WithCheckPointStore` + `WithCheckPointID` |
| Immutable audit row per decision (timestamp, actor, args, decision, reason) | Enterprise table stakes (EU AI Act Art. 12 record-keeping direction; every enterprise audit-trail guide) | LOW | Extends existing T-3-AUDIT INSERT-only store — read-time redaction pattern already proven |
| Resume continues from checkpoint (idempotent), never re-runs the gated action | LangGraph idempotency pitfall is the canonical HITL bug | MEDIUM | Gate before the side effect; persist tool inputs in checkpoint |
| Timeout/cancel affordance | Don't block the session forever | LOW | |

### Differentiators (Competitive Advantage — trusted angle)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Edit-before-approve (modify args in card, validated) | LangGraph's `edit` decision; almost no CN product has it; "可信×行动" made tangible | MEDIUM | Validate edited JSON before resume; conservative-edit warning |
| Skill-declared write boundary as deterministic gate | Agent **cannot** invoke a write tool outside its Skill's `writes:` frontmatter — code-enforced, not prompt-enforced | LOW-MEDIUM | The 6-field spec already has `writes`; enforcement is new — this is platform-level differentiation |
| Refusable writes | Trust floor applies to actions: "我不能生成该报告，缺少权威依据" with structured refusal | LOW | Reuses existing RefusalCard + verdict pipeline |
| Approval→audit→eval linkage | Approval row links to plan step (Capability 3) and appears in existing `/eval/audit` UI with redaction | LOW | Audit UI + redaction already shipped (Phase 6 SC-3/SC-5) |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Bypass/YOLO mode ("approve everything") | Power users hate prompts | Directly contradicts the compliance positioning; Claude Code's `dangerously-skip-permissions` is a sandboxed-dev tool, not an enterprise feature | "Allow for session" per tool+args pattern at most |
| Gating reads behind approval | "Maximum control" | Approval fatigue is the #1 HITL failure mode; Claude Code learned to gate only writes/irreversibles | Gate writes only; reads stay free |
| LLM self-classifies whether approval is needed | "Smart gating" | Non-deterministic trust boundary = no trust boundary | Declarative: `writes:` frontmatter decides |
| Async/webhook approval delegation (mobile push, Telegram) | "Approve from phone" | Real feature request in Claude Code ecosystem, but v2 scope; adds identity/delivery complexity | Defer; in-app queue is enough |
| Compound-action auto-approval via string matching | Convenience | Claude Code's known bypass: `git status && rm -rf /` matches allow pattern for `git status` | Approval binds to structured tool call (name + arg hash), never raw strings |

### Approval UX During Streaming Chat (answers the milestone's key question)

Synthesized from LangGraph frontend HITL guidance + Vercel AI SDK part model + SkillForge's existing chunk pipeline:

1. **Inline card, not modal.** The approval request renders as a message part in the chat stream at the point of interruption (a `data-*` UIMessageChunk, matching existing DataSubprogressPart/RoutingTracePart precedent). Streamed reasoning above the card stays visible — the user sees *why* the agent wants to act.
2. **Stream lifecycle.** SSE emits the interrupt payload and cleanly ends (or idles) with a checkpoint ID. On decision, a new SSE connection resumes with that checkpoint ID; tokens continue appending into the **same** assistant message — no duplicate bubble, no flicker.
3. **Resolved card persists.** After decide, the card stays in the transcript with a stamped state ("已批准 · 14:32 · admin" / "已拒绝 · 原因： …"), committed atomically to state (LangGraph: `stream.respond(decision, {update: …})` pattern) so it survives refresh and doubles as the audit-visible record.
4. **Input area state.** While awaiting approval, the chat input switches to approval context (disabled free-text or "reply to agent" mode feeding the `reject` reason); approve is the one-click/Enter path; reject requires a reason (feedback to the agent).
5. **Queue, never stack.** Multiple pending approvals render as an ordered queue resolved one at a time — deterministic interrupt order, no stacked modals.
6. **Edit mode.** Args render as an editable form inside the card; invalid JSON blocks submission inline.

---

## Capability 3: Planning Layer (Task Decomposition + Todos + Replayable Trace)

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| Claude Code | TodoWrite (flat, in-context scratchpad) → Tasks system (persistent, dependency graph with blocking semantics, hierarchy, multi-session shared board) |
| Gemini Deep Research | Proposes a research plan and asks user **confirmation before executing** — the plan-approval gate |
| ChatGPT Deep Research / agent mode | Live plan + step narration during execution |
| LangChain Deep Agents | Four ingredients: detailed system prompt, planning tool (write_todos), sub-agents, file system |
| Eino | `adk/prebuilt/deep` ships `write_todos` middleware built-in (pending/in_progress/completed) |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Visible live checklist (pending → in_progress → completed) | Every serious agent product shows this; invisible planning reads as flailing | LOW-MEDIUM | write_todos state maps directly onto existing subprogress UI |
| Current-step highlighting, updates stream in real time | Progress transparency during long runs | LOW | SSE data-chunk; precedent: DataSubprogressPart five-step copy |
| Agent re-planning is visible, not silent | Silent plan changes break trust | LOW | Emit plan-revision event |
| Plan persists with the session | Refresh/resume continuity | MEDIUM | Same store as Capability 1 history |

### Differentiators (Competitive Advantage — trusted angle)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Plan confirmation gate before execution (Gemini pattern) | "Agent 先报计划再动手" — for compliance buyers this is the single most legible trust feature | MEDIUM | Reuses Capability 2's approval card + checkpoint machinery; approve/edit/re-plan decisions |
| Replayable plan trace | Each step links to its tool calls, retrievals, verdicts, and approval rows; inspectable after the fact | MEDIUM-HIGH | The "可回放" third of 可审计/可拒答/可回放； builds on reasoning-trace + routing-trace UI already shipped |
| Step-completion evidence integrity | Agent cannot mark a step completed without a corresponding tool output/verdict | MEDIUM | Deterministic check in plan middleware, not prompt discipline |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Hidden reasoning-only plans | "Less UI noise" | Contradicts transparency positioning; indistinguishable from no planning | Always materialize plan as todos |
| 20+ micro-step auto-plans | "Thoroughness" | Noise; users tune out; Anthropic's own guidance is a few meaningful steps | Cap at 3–7 steps; agent may nest detail inside a step |
| Direct user editing of the plan DAG | "Control" | Dependency-graph editing UX is a rabbit hole (Claude Code Tasks needs a whole tool family for it) | approve / edit-as-text / re-plan only |
| Multi-session shared task boards (Agent Teams Kanban) | Cross-session collaboration | Far outside v1; coordination protocol complexity | Defer to v2+ |

---

## Capability 4: Sub-Agent Fan-Out (Parallel Multi-Hop Retrieval, Tier-Boundary Inheritance)

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| Anthropic multi-agent research system | Orchestrator-worker: lead agent decomposes → parallel sub-agents, each isolated context → sub-agents return **compressed** findings (intelligent filters, not raw dumps) → synthesis (+ CitationAgent); ~15× tokens for ~90% quality gain on research tasks; plan persisted to survive truncation |
| OpenAI Deep Research | Single-agent RL-trained model (the contrast case — not reproducible without training) |
| Gemini Deep Research | Splits into sub-problems, multiple parallel browsing agents per sub-problem |
| Claude Code | Task tool spawns ephemeral sub-agents: isolated context, self-contained prompts, parallelism + isolation + specialization; parallelism pays only when tasks share no state and have no inter-dependencies |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| True parallel fan-out (not sequential in disguise) | Latency is the entire point | MEDIUM | Go goroutines + Eino ADK are natural fits |
| Compressed sub-agent returns | Raw dumps blow up orchestrator context (Anthropic's core finding) | MEDIUM | Sub-agent summarizes + cites; orchestrator synthesizes |
| Aggregate progress indicator ("3 个子任务并行检索中") | Long multi-hop runs need legibility | LOW | Extends DataSubprogressPart |
| Failure isolation (one sub-agent fails → graceful degradation, not total failure) | Parallel systems must degrade | MEDIUM | Partial-failure verdict state ("部分来源未能检索") |

### Differentiators (Competitive Advantage — trusted angle; unique to SkillForge)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Tier-boundary inheritance enforced at retriever level | Sub-agent inherits parent Skill's `reads` tiers and **cannot** exceed them — deterministic filter, not prompt instruction. Nobody else does this | MEDIUM | Reuses existing Milvus/Mongo tier-filter (Phase 1 SC-2 proven); enforcement is in fan-out construction |
| Per-sub-agent grounding | Grounding checker runs on each sub-agent's output before merge, not just the final answer | MEDIUM | Reuses the D2-01 moat (verbatim citation-span check) |
| Provenance-preserving merge | Citations carry sub-agent identity through orchestration; citation UI can show "via 子任务： 历史制度对比" | MEDIUM | Extends existing citation/ChunkSourceView |
| Auditable fan-out decision | Routing trace records why orchestrator split into N sub-tasks | LOW-MEDIUM | Extends RoutingTracePart |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Recursive spawning (sub-agents spawning sub-agents) | "Depth" | Unbounded cost, unauditable traces, timeout cascades | One fan-out level max in v0.8 |
| Fan-out for simple single-hop questions | "Use the architecture" | 15× token cost for zero gain; slower answers | Orchestrator classifies 1 vs N; single-hop stays on the current ReAct path |
| Shared mutable state between parallel sub-agents | "Coordination" | Races, non-determinism, audit impossibility; isolation is the point of the pattern | Isolated contexts; merge only via orchestrator |
| Raw sub-agent transcripts in main chat | "Full transparency" | Context explosion in the UI | Compressed summary inline; full trace behind expand (replay view) |

---

## Feature Dependencies

```
[Eino dep upgrade to v0.8.x+]
    └──required by──> [C1 Memory: summarization middleware]
    └──required by──> [C2 Approval: ADK HITL Interrupt/Resume maturity]

[C1 Multi-turn memory]
    └──required by──> [C3 Planning]          (plan persists across turns within session)
    └──enhances────> [C2 Approval]           (approval context references conversation)

[C2 Write tools + approval]
    └──requires────> [audit store]            (EXISTS: T-3-AUDIT INSERT-only, Phase 6)
    └──requires────> [checkpoint store]       (NEW: Eino WithCheckPointStore)
    └──requires────> [skill writes: spec]     (EXISTS: 6-field spec, enforcement NEW)

[C3 Planning]
    └──enhances────> [C4 Fan-out]            (plan steps drive fan-out decomposition)
    └──requires────> [subprogress UI]         (EXISTS: DataSubprogressPart)

[C4 Sub-agent fan-out]
    └──requires────> [tier reads + retriever filter]  (EXISTS: Phase 1/2)
    └──requires────> [grounding checker]               (EXISTS: D2-01)
    └──conflicts──> [single-hop ReAct path]            (must coexist, not replace)

[ALL FOUR] ──gated by──> [frozen eval set + gate]     (EXISTS: trust-floor metrics must not drift)
```

### Dependency Notes

- **C1 requires dep upgrade:** summarization middleware and ADK HITL are v0.8.x-era; the milestone's "前置： 全量依赖升级" is a hard blocker, not hygiene.
- **C1 requires the trust-floor spike:** history injection can contaminate refusal judgment (the milestone's only identified substantive risk). Golden-set A/B must run **before** memory ships, and the gate threshold stays the promotion criterion.
- **C2 requires checkpoint persistence:** approval-survives-refresh is table stakes; without `WithCheckPointStore` the approval card is a toy.
- **C2's write-boundary enforcement leverages an existing spec field:** `writes:` was designed into the 6-field spec (Key Decision: explicit reads/writes) — v0.8 finally enforces it. Cheap, high-credibility win.
- **C3 enhances C4 but isn't required by it:** fan-out can ship with classifier-driven decomposition; plan-driven decomposition is the better UX. Keep C3 before C4 (matches user's pillar order).
- **C4 conflicts with the single-hop path if not gated:** fan-out must be orchestrator-decided per query; never force every question through multi-agent.
- **All four gated by the existing eval harness:** every capability ships with a frozen-set regression showing faithfulness/refusal metrics did not drift. This is the "可信税" made mechanical.

## MVP Definition (milestone scope)

### Launch With (v0.8)

- [ ] C1: session history → agent + summarization + trust-floor spike gate — foundation for everything
- [ ] C2: two write tools (memory:facts write, review-report generation) + inline approval card + audit row + checkpoint resume — the moat's first positive proof
- [ ] C3: write_todos plan checklist + plan confirmation gate + basic replay view — "先报计划再动手"
- [ ] C4: one-level parallel retrieval fan-out with tier inheritance + per-sub-agent grounding — the architectural flex

### Add After Validation (v0.8.x)

- [ ] Edit-before-approve (after base approval UX is validated with real users)
- [ ] Step-completion evidence integrity checks
- [ ] Partial-failure verdict states for fan-out

### Future Consideration (v2+)

- [ ] Cross-session long-term memory — privacy design needed first
- [ ] Async/webhook approval delegation — needs identity + delivery infra
- [ ] Multi-session task boards / agent teams — coordination protocol scope
- [ ] Recursive sub-agents — cost/audit model needed first

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| C1 history → ReAct agent | HIGH (broken follow-ups = broken product) | MEDIUM | P1 |
| C1 trust-floor spike (history A/B) | HIGH (de-risks the whole milestone) | LOW-MEDIUM | P1 |
| C2 approval card + checkpoint + audit row | HIGH (the "可信×行动" headline) | MEDIUM | P1 |
| C2 skill `writes:` enforcement | HIGH (platform credibility) | LOW-MEDIUM | P1 |
| C3 plan checklist + confirmation gate | MEDIUM-HIGH | MEDIUM | P2 |
| C3 replayable plan trace | MEDIUM (demo gold, daily-use less critical) | MEDIUM-HIGH | P2 |
| C4 parallel fan-out + tier inheritance | MEDIUM-HIGH | HIGH | P2 |
| C2 edit-before-approve | MEDIUM | MEDIUM | P3 |
| C4 provenance-preserving citation merge | MEDIUM | MEDIUM | P3 |

## Competitor Feature Analysis

| Capability | Claude Code | LangGraph | ChatGPT/Gemini DR | Dify/Coze | SkillForge v0.8 Approach |
|-----------|-------------|-----------|-------------------|-----------|--------------------------|
| Memory | Session + compaction + file memory | Checkpointer + Store + summarization mw | Auto-compaction | Token window (weak) | Session memory + summarization + **trust-floor regression gate** |
| Approval | allow/ask/deny rules, modes | approve/edit/reject/respond + checkpoint | Confirm before irreversible | Tool toggles (weak) | Inline streaming card + **skill-declared write boundary + audit row + refusable writes** |
| Planning | TodoWrite → Tasks (persistent, deps) | write_todos (Deep Agents) | Live plan; Gemini: plan approval gate | Workflow DAG (design-time) | write_todos + **plan confirmation gate + replayable trace linked to audit** |
| Fan-out | Task tool, isolated ephemeral sub-agents | Orchestrator-worker patterns | Anthropic: parallel compressed sub-agents | Workflow parallel branches | One-level fan-out + **tier inheritance + per-sub-agent grounding + provenance merge** |

The rightmost column is the differentiation statement: every competitor ships the capability; nobody ships the capability **with the trust tax applied**. Dify/Coze's weak HITL is the clearest market gap for a compliance-positioned product.

## Sources

**HIGH confidence (official/primary):**
- LangChain docs — [Human-in-the-loop](https://docs.langchain.com/oss/python/langchain/human-in-the-loop), [Frontend HITL](https://docs.langchain.com/oss/python/langchain/frontend/human-in-the-loop), [Interrupts](https://docs.langchain.com/oss/python/langgraph/interrupts): four decision types (approve/edit/reject/respond), checkpointer persistence, resolved-card persistence, idempotency-before-interrupt guidance
- Anthropic engineering — [How we built our multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system): orchestrator-worker, compressed sub-agent returns, ~15× tokens / ~90% quality, plan persistence
- Dify official — [Conversation Variables blog](https://dify.ai/blog/enhancing-llm-memory-with-conversation-variables-and-variable-assigners), [Agent docs](https://docs.dify.ai/en/cloud/use-dify/nodes/agent), [GitHub Discussion #22907](https://github.com/langgenius/dify/discussions/22907): TokenBufferMemory, 2,000-token/500-message caps, session-scoped
- LangChain blog — [Deep Agents](https://www.langchain.com/blog/deep-agents): four ingredients (system prompt, planning tool, sub-agents, file system)
- OpenAI — [Introducing deep research](https://openai.com/index/introducing-deep-research/), [Deep research help article](https://help.openai.com/en/articles/10500283-deep-research-in-chatgpt)

**MEDIUM confidence (multiple secondary sources agree):**
- Claude Code permission model — [claudedirectory.org permissions guide](https://claudedirectory.org/blog/claude-code-permissions-guide), [arte.itlibra.com rules guide](https://arte.itlibra.com/en/articles/claude-code-permission-rules-settings), [stackademic permission system walkthrough](https://blog.stackademic.com/stop-fighting-claude-codes-permission-prompts-here-s-how-the-system-actually-works-ae594e59fb13): deny→ask→allow order, deny absolute, modes
- Claude Code Tasks/TodoWrite — [aibuilderclub comparison](https://www.aibuilderclub.com/blog/claude-code-todowrite-vs-task), [echofold Tasks overview](https://echofold.ai/news/claude-code-tasks-multi-session-collaboration), [mindstudio agent teams](https://www.mindstudio.ai/blog/what-is-claude-code-agent-teams)
- HITL workflow practices — [Towards Data Science HITL workflows](https://towardsdatascience.com/building-human-in-the-loop-agentic-workflows/)
- Deep Research architecture surveys — [waylandz AI Agent Architecture ch.27](https://waylandz.com/ai-agent-book-en/chapter-27-deep-research/), [ai-blacksmith architecture series](https://ai-blacksmith.com/blog/deep-research-agent-architecture), [tritonite patterns](https://tritonite.io/stories/building-deep-research-agents-patterns)

**LOW confidence (single-source / issue-tracker, flagged):**
- Claude Code "Always allow" default regression — [GitHub issue #48550](https://github.com/anthropics/claude-code/issues/48550) (single-user report; treat as UX hazard to design against, not established fact)
- Compound-command allow-bypass — [claude-hooks community hook](https://github.com/liberzon/claude-hooks) (community analysis; directionally credible)
- Webhook approval demand — [GitHub issue #26000](https://github.com/anthropics/claude-code/issues/26000) (feature request; used only as v2 signal)
- Enterprise audit-trail direction — vendor blogs ([prompthalo](https://www.prompthalo.ai/feeds/blog/ai-audit-trail), [runlayer](https://www.runlayer.com/blog/ai-agent-audit-trails-dos-and-donts), [miniorange](https://www.miniorange.com/blog/ai-agent-audit-trail/)); EU AI Act Art. 12 record-keeping reference via [inkog glossary](https://inkog.io/glossary/ai-agent-logging). Vendor content — use for direction, not specifics.

---
*Feature research for: trusted-action agent harness (SkillForge v0.8)*
*Researched: 2026-08-17*
