# Project Research Summary

**Project:** SkillForge — v0.8 Harness Milestone (可信×行动融合)
**Domain:** Trusted-action agent harness for enterprise compliance Q&A (multi-turn memory, HITL write approval, DeepAgent planning, sub-agent fan-out)
**Researched:** 2026-08-17
**Confidence:** HIGH

## Executive Summary

SkillForge v0.8 adds four harness-class capabilities — multi-turn memory, write tools behind a human approval gate, DeepAgent planning with replayable traces, and sub-agent fan-out — onto an existing trusted-Q&A kernel whose moat is trust-floor refusal, verbatim citations, and INSERT-only audit. The research converges on one load-bearing fact: **the existing trust anchors are already structurally safe under every new capability, because they are keyed to a per-Run `ChunkAccumulator`, not to the message list.** The milestone's single unifying trust invariant is: **citation whitelist = current-Run retrieval only, code-enforced.** History, summaries, sub-agent outputs, and resumed-after-approval runs can all inject text into the prompt, but no citation survives unless its chunk was retrieved during the current Run. Every capability inherits this invariant; every pitfall prevention strategy reduces to preserving it.

The recommended approach is a strict dependency-ordered build: upgrade eino v0.7.7 → **v0.9.13** first (summarization middleware and mature ADK HITL simply do not exist at v0.7.7; v0.10.0-alpha.x is explicitly warned against by maintainers), then run the history-injection trust-floor spike as a hard gate before memory ships, then memory, then the approval gate (which forces the Runner migration onto the well-tested ReAct archetype rather than a new agent type), then DeepAgent planning, then sub-agent fan-out. The frontend needs zero new dependencies — ai@7's `tool-approval-request`/`tool-approval-response` UIMessageChunk types and `addToolApprovalResponse` are first-class. The only new infrastructure is a ~100-line sqlite `CheckPointStore` (no official persistent store exists in eino-ext; Redis would break the single-node Compose posture).

Key risks are behavioral, not structural: history injection can silently drift refusal rates and faithfulness on the frozen golden set (mitigated by the P8.S spike gate with a <5% drift threshold and sanitization rules); approval can break the audit invariant via partial writes (mitigated by write-ahead pending rows and fail-closed semantics — the opposite of the read path's swallow policy); and DeepAgent's default MaxIteration=50 can explode cost (mitigated by a three-layer budget gate from day one). Every phase ends with a "可信税" artifact: spike report, 3-row audit lifecycle, replayable plan in audit, tier-inheritance proof test.

## Key Findings

### Recommended Stack

Details: .planning/research/STACK.md

**Core decisions:**
- **eino v0.7.7 → v0.9.13**: agent framework upgrade — v0.8.x line adds `adk/middlewares/summarization` (required for memory); v0.9.x is the current stable "agentic-runtime" line with 13 ADK patch releases (checkpoint-aware cancel, nested agent cancel isolation). NOT v0.10.0-alpha.x (maintainers warn of breaking changes).
- **Custom sqlite `CheckPointStore`** (~100 LOC on existing `internal/store/sqlite`): no official persistent checkpoint store exists in eino-ext — only `store.NewInMemoryStore()` ships, which loses pending approvals on restart. Interface is trivial (`Get`/`Set` []byte). Must `gob.RegisterName` every custom state type.
- **ai@7 approval interrupts — zero new FE deps**: `tool-approval-request`/`tool-approval-response` are UIMessageChunk wire types; `addToolApprovalResponse` + `approval-requested`/`output-denied` part states ship in installed `@ai-sdk/react`. D-04 transport-swap invariant survives.
- **Sub-agent observability — framework-built-in**: `AgentEvent.RunPath`/`AgentName` auto-filled + `HandlerBuilder` agent callbacks → existing sqlite audit. Defer Langfuse/CozeLoop/OTel (external server/account contradicts local-first OSS posture).
- **eino-ext components**: bump in lockstep per-component to versions declaring eino ≥v0.9 compat (components are independently versioned).

### Expected Features

Details: .planning/research/FEATURES.md

**Must have (table stakes):**
- Anaphora/follow-up resolution — broken follow-ups read as a toy; history must reach the agent
- History survives page refresh — sqlite session store already proven (SC-4, 50×5 isolation)
- Approval card with what/tool/full-args/why; approve-once / reject-with-reason / approve-and-remember — Claude Code baseline
- Approval state survives refresh via persistent checkpoint — LangGraph checkpointer baseline
- Immutable audit row per decision — enterprise table stakes (EU AI Act Art. 12 direction)
- Visible live plan checklist streaming in real time; plan persists with session
- True parallel fan-out with compressed sub-agent returns and failure isolation

**Should have (competitive — the trust-tax differentiators):**
- Trust-floor regression gate on history injection — nobody else proves memory doesn't degrade refusal/citation behavior; this is the milestone's spike seed
- Skill-declared write boundary (`writes:` frontmatter) as **code-enforced** gate — agent cannot invoke a write tool its Skill doesn't declare; platform-level differentiation
- Edit-before-approve — LangGraph's `edit` decision; almost no CN product has it
- Plan confirmation gate before execution (Gemini pattern) — the most legible trust feature for compliance buyers
- Replayable plan trace linked to audit rows — the "可回放" third of 可审计/可拒答/可回放
- Tier-boundary inheritance enforced at retriever level for sub-agents — deterministic filter, not prompt instruction; nobody else does this
- Per-sub-agent grounding before merge — reuses the D2-01 verbatim citation-span moat

**Defer (v2+):**
- Cross-session semantic long-term memory (privacy design needed first)
- Async/webhook approval delegation (identity + delivery infra)
- Multi-session task boards; recursive sub-agents (cost/audit model needed)
- Bypass/YOLO approval mode — contradicts compliance positioning, never ship

### Architecture Approach

Details: .planning/research/ARCHITECTURE-v08.md

Four capabilities map onto four new seams, all hanging off one load-bearing refactor: **lifting the trust anchors (floor + grounding + critique) out of `(*Agent).Run` into a shared `ApplyTrustStage`** that every agent archetype (chat ReAct, deep, sub-agent) exits through. History assembly lives in a new server-layer `internal/history` package (projects stored verdict JSON → `conclusionText`, strips `[CHUNK id]` markers — the LLM physically cannot copy a citation it never sees), keeping the agent a pure function of (manifest, tools, messages). Approval uses Eino Interrupt/Resume with the interrupt SSE stream closing cleanly and resume as a NEW HTTP request through the same transport; `checkpoint_id` is server-generated, session-scoped (`{sessionID}:{actionULID}`). Planning dispatches on the existing-but-ignored `Manifest.Agent` field via a new `internal/runtime` factory. Sub-agents inherit tiers by **build-time closure** — they receive retrieve tools closed over the parent's `ResolvedTiers()`, so widening is structurally impossible.

**Major components:**
1. `internal/history` — session_turn → `[]schema.Message` projection; token budget; chunk-marker stripping; one HistoryModifier pipeline serving both Run and ResumeWithParams
2. `internal/agent/truststage.go` — lifted trust floor + grounding + critique (THE load-bearing refactor; regression locks move with it)
3. `internal/approval` + `internal/store/sqlite/checkpoint.go` + `internal/server/resume.go` — interrupt/resume round-trip; 3-row audit lifecycle (pending → approved/rejected) via one new `action_json` column (INSERT-only preserved)
4. `internal/runtime/factory.go` — `Manifest.Agent` dispatch (chat/deep); sub-agents built with parent-closed retrieve tools
5. FE: `ApprovalPart.tsx` + `PlanTracePart.tsx` — presentational only; approval state derived from message parts, never Zustand

### Critical Pitfalls

Details: .planning/research/PITFALLS.md

1. **历史注入污染 trust floor (Pitfall 1)** — history lets the LLM skip retrieval or drift refusal rates silently; frozen baseline won't catch it. Avoid: P8.S spike as hard gate (four-metric drift <5%, refusal delta <10%), history sanitization (strip chunk refs/data parts), per-Run re-grounding, "context not evidence" prompt contract.
2. **用户引用文本穿透 tier 边界 (Pitfall 2)** — user-pasted text in history becomes unverified evidence via the prompt path, bypassing the retriever-level Milvus filter. Avoid: the citation-whitelist invariant (citations must hit current-Run authority-tier retrieval, code-enforced), "未验证用户陈述" labels, red-team injection cases in the golden set.
3. **审批门破坏审计完整性 (Pitfall 3)** — partial writes without audit rows bankrupt the differentiation claim. Avoid: write-ahead `status=pending` row at interrupt time bound to checkpoint ID, fail-closed audit writes (opposite of read-path swallow — a D-level Key Decision), idempotency keys on tool_call_id.
4. **SSE 断连 + 内存 checkpoint = 中断态丢失 (Pitfall 4)** — official examples default to InMemoryStore; copying them is the canonical production trap. Avoid: sqlite CheckPointStore from P8.0, gob.RegisterName all custom types, idempotency keys, GET resume endpoint for page-refresh recovery, checkpoint TTL with explicit expired UI.
5. **Interrupt/Resume 迁移双语义并存 (Pitfall 5)** — legacy `NewInterruptAndRerunErr` lacks addresses; mixing old/new APIs makes resume unpredictable. Avoid: P8.0 cuts over once with a lint ban on legacy API; HistoryModifier may only inject approve/reject+comment, never rewrite history.

## Implications for Roadmap

Based on research, suggested phase structure (6 phases — PITFALLS.md proposal, agreed by ARCHITECTURE-v08.md and FEATURES.md):

### Phase P8.0: 前置 — 全量依赖升级 (eino v0.7.7 → v0.9.13)
**Rationale:** Summarization middleware (v0.8.x), mature ADK HITL ergonomics, and 13 ADK patch fixes do not exist at v0.7.7. Doing the upgrade standalone — not mixed with feature work — keeps regression attribution clean (Pitfall 5). ARCHITECTURE-v08.md's "v0.8.x-era" build-order note predates the stack finding; the unified target is **v0.9.13**.
**Delivers:** eino v0.9.13 + lockstep eino-ext bumps; sqlite CheckPointStore skeleton + gob round-trip test; green `integration/contract` drift suite; re-locked Phase-1 live verifies (cancel <200ms, Milvus tier-filter); legacy-interrupt API lint ban.
**Avoids:** Pitfalls 4 (store skeleton), 5 (migration traps).
**Research flag:** 30-min compile-check spike against upgraded eino for exact `CheckPointStore` interface shape (RF-2).

### Phase P8.S: Spike — 历史注入对 trust floor 的影响 (HARD GATE)
**Rationale:** The milestone's only identified substantive behavioral risk. Memory must not ship until the gate passes; this is the "可信税" made mechanical.
**Delivers:** Golden-set A/B (`history=nil` vs projected history × benign follow-up / malicious injection / old-refusal-context); go/no-go on P8.1; sanitization rules as the spike's primary artifact; escape-hatch selection if failed (last-3-turns budget → prompt addendum → user-turns-only memory).
**Avoids:** Pitfall 1 (H1–H3), exposes Pitfall 2 early.
**Research flag:** none — frozen eval set + gate already exist; spike seed `spike-history-injection-trust-floor` is pre-written.

### Phase P8.1: 多轮对话记忆
**Rationale:** Resume round-trips replay the full message list — shipping approval on single-turn memory would build resume machinery on a foundation that immediately gets rebuilt. Gated by P8.S.
**Delivers:** `internal/history` projection (verdict JSON → conclusionText, refusal → refusalText, chunk-marker stripping); `Run` gains history param; dumb token budget first (summarization middleware deferred until runner migration; project-then-summarize ordering when it lands); red-team tier-penetration cases in eval; `TestSessionIsolation50x5` re-run.
**Avoids:** Pitfalls 1 (sanitization), 2 (whitelist + red team), 7 (threshold calibration vs real GLM usage).
**Research flag:** summarization middleware token-counter calibration against GLM usage logs (RF-6); exact tokenizer configurability unverified.

### Phase P8.2: 写工具 + 审批门
**Rationale:** Runner migration is the riskiest framework change — landing it on the simple ReAct archetype with the full existing regression suite as safety net is strictly safer than combining it with a new agent type. Audit + checkpoint infrastructure built here is reused by P8.3 (todo audit, budget aborts).
**Delivers:** Chat-archetype Runner migration (`TestCancelPropagationMidTool` <200ms re-locked); interrupt/resume endpoints (`POST /api/chat/resume` + GET resume for refresh recovery); 3-row audit lifecycle via `action_json` column, fail-closed writes (Key Decision); idempotency keys; checkpoint skill-version/prompt_hash binding (reject stale resume); `ApprovalPart.tsx` with full part.state enum + `sendAutomaticallyWhen`; transport-swap fixtures covering approve/deny loops; two write tools (memory:facts, report generation); skill `writes:` frontmatter enforcement.
**Avoids:** Pitfalls 3 (write-ahead audit), 4 (persistent checkpoint + resume), 6 (version-bound checkpoints), 10 (FE state machine).
**Research flags:** ai@7 custom transport resume shape — separate endpoint + transport branch vs synthetic user message (RF-3); FE and backend HITL must ship in the same phase (不可拆).

### Phase P8.3: 规划层 (DeepAgent + write_todos + 可回放)
**Rationale:** Deep slots into the already-Runner-shaped harness from P8.2. Plan-driven decomposition is the better host for fan-out than classifier-only.
**Delivers:** `internal/runtime` factory dispatch on `Manifest.Agent`; truststage refactor (the load-bearing move); deep verdict bridge (verdict-JSON suffix + `forced_stop` fallback); plan trace as `data-decision{decision:"plan"}` chunks (no new wire type); plan confirmation gate reusing P8.2 approval machinery; three-layer budget gate (MaxIteration ≤10, run token budget, loop detection) from day one; write_todos audit rows → replayable plan from audit alone.
**Avoids:** Pitfall 8 (cost explosion / unbounded loops).
**Research flag:** DeepAgent final-message verdict-parse rate on GLM-5.2 (RF-5); fallback is structured-output option on the final step.

### Phase P8.4: 子 agent 扇出 (tier 继承)
**Rationale:** Last because it reuses P8.1's citation-ID mechanism, P8.2's audit/checkpoint infra, and P8.3's budget gates. Keep first use narrow: one-level parallel multi-hop retrieval.
**Delivers:** Build-time closure tier inheritance (sub-agents receive parent-closed retrieve tools; empty intersection = load-time validation error); shared per-Run ChunkAccumulator across sub-agents; deterministic merge (key-sorted pure function, contract-tested 10×); conservative partial-failure merge (any sub-agent refusal → at most "需补充", never affirmative); run-level retrieval cache; per-sub-agent grounding before merge.
**Avoids:** Pitfall 9 (tier leakage + non-deterministic merge).
**Research flag:** ADK transfer/handoff ctx-value propagation into sub-agent tool closures (RF-4) — validate early; fallback is per-Run agent construction with accumulator baked into closures. Sub-agent tier-inheritance exact API form unverified in docs (LOW-MEDIUM) — needs plan-level research.

### Phase Ordering Rationale

- **Upgrade first** because two confirmed capabilities simply don't exist at v0.7.7; every subsequent phase would pay a double-migration tax, and mixing upgrade with features makes regression attribution impossible.
- **Spike gates memory** because history injection is the only risk that can silently break the already-calibrated trust floor; the frozen baseline won't catch drift on its own.
- **Memory before approval** because resume replays the full message list — approval on single-turn memory is throwaway work.
- **Approval before deep** because the Runner migration lands safest on the simple archetype with the full regression suite as net.
- **Deep before fan-out** because write_todos + sub-agent slots are the natural composition host, and fan-out inherits the budget gates built in P8.3.
- **Every phase ends with a 可信税 artifact**: spike report → sanitization rules → 3-row audit lifecycle → replayable plan in audit → tier-inheritance proof test.

### Research Flags

Phases likely needing deeper research during planning (`--research-phase`):
- **P8.1:** summarization middleware token-counter calibration + tokenizer configurability (RF-6)
- **P8.2:** ai@7 custom transport resume shape (RF-3); exact CheckPointStore interface post-upgrade (RF-2, cheap compile-check)
- **P8.3:** DeepAgent verdict-parse rate on GLM-5.2 (RF-5)
- **P8.4:** ADK ctx propagation into sub-agent closures (RF-4) — highest-uncertainty flag; sub-agent tier-inheritance API form unverified in docs

Phases with standard patterns (skip research-phase):
- **P8.0:** upgrade checklist + breaking-changes list fully enumerated in STACK.md/PITFALLS.md
- **P8.S:** spike seed pre-written; eval harness exists

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official release notes + Context7-verified docs; alpha warning from maintainers themselves; absence of official persistent CheckPointStore confirmed |
| Features | MEDIUM-HIGH | Primary sources for LangGraph/Anthropic/Dify; MEDIUM on Claude Code permission specifics (secondary sources); LOW items flagged as design hazards only |
| Architecture | HIGH on layering + trust containment; MEDIUM on sub-agent ctx propagation and CheckPointStore signature specifics | Trust-anchor containment verified against actual code (agent.go file:line); MEDIUM items are spiked in-phase |
| Pitfalls | HIGH for Eino/ai@7 facts (Context7-verified); MEDIUM-HIGH for integration pitfalls (synthesized from system invariants) | Two items explicitly unverified: sub-agent tier-inheritance API form, summarization tokenizer details |

**Overall confidence:** HIGH

### Gaps to Address

- **Exact v0.7.7→v0.9.13 compile breakage surface in OUR code:** breaking-change lists verified, but which bite requires the P8.0 build — accepted as phase validation task, not a blocker.
- **ADK ctx-value propagation to sub-agent tool closures (RF-4):** docs confirm context-isolation but not value propagation; P8.4 opens with this spike; fallback (per-Run agent construction) is known but costlier.
- **ai@7 resume transport shape (RF-3):** whether resume reuses the chat endpoint with a different body or needs a dedicated transport branch; P8.2 FE spike.
- **Summarization token counter vs GLM tokenizer (RF-6):** calibrate against real usage logs in P8.1 before enabling; until then, dumb token budget only.
- **DeepAgent verdict-parse rate on GLM-5.2 (RF-5):** measure in P8.3; structured-output final step is the known fallback.

## Sources

### Primary (HIGH confidence)
- Context7 `/websites/cloudwego_io_eino` — v0.8/v0.9 release notes + migration guides; ADK HITL (CheckPointStore, ResumeWithParams, HistoryModifier, legacy interrupt wrapping); summarization middleware config; deep agent docs (MaxIteration, write_todos); agent callbacks (HandlerBuilder, RunPath); gob registration requirement
- Context7 `/vercel/ai` — UIMessageChunk union (tool-approval-request/response); tool-approvals docs (`addToolApprovalResponse`, `sendAutomaticallyWhen`); chatbot-resume-streams (`useChat({resume:true})` server requirements)
- cloudwego/eino releases page + Discussion #1159 — v0.9.13 latest stable; v0.10 alpha instability warning
- LangChain official docs — HITL four decision types, checkpointer persistence, frontend HITL guidance, Deep Agents four ingredients
- Anthropic engineering blog — multi-agent research system (orchestrator-worker, compressed returns, ~15× tokens / ~90% quality)
- This repo — `internal/agent/agent.go`, `internal/server/chat.go`, `internal/store/`, `internal/skill/manifest.go` (file:line verified); `.planning/notes/harness-roadmap-decision.md`; PROJECT.md Key Decisions

### Secondary (MEDIUM confidence)
- Claude Code permission model guides (claudedirectory, stackademic) — deny→ask→allow, modes
- Claude Code Tasks/TodoWrite comparisons (aibuilderclub, echofold)
- Deep Research architecture surveys (waylandz, ai-blacksmith, tritonite)
- xyncra-server RedisCheckPointStore — community bytes-KV pattern reference only

### Tertiary (LOW confidence — flagged, not load-bearing)
- Claude Code GitHub issues #48550 / #26000 — single-user reports; used as UX hazard signals only
- Enterprise audit-trail vendor blogs — direction only, not specifics
- Height.app closure / 2026 benchmark blog posts — context, not decisions

---
*Research completed: 2026-08-17*
*Ready for roadmap: yes*
