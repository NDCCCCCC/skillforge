# Phase 10: 多轮对话记忆（P8.1）- Research

**Researched:** 2026-08-18
**Domain:** Multi-turn conversation memory over the existing Go + Eino v0.9.14 + Hertz SSE trust pipeline
**Confidence:** HIGH for Eino v0.9.14 + GLM-5.2 call patterns (Context7 docs + Phase 8 live verification); HIGH for projection.go / neutral placeholder / budget window (Phase 9 contract locked); MEDIUM-HIGH for token-budget char-approximation constants (calibration deferred to end-of-phase spike); HIGH for Go-native `internal/eval` judge (Phase 3 shipped, reused unchanged).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**A. Trust-floor anti-drift mechanism (Phase 9 escape plan concretized)**
- **D10-A1**: Refusal decision = query-only refusal classifier (independent LLM pre-call). Input = rewritten query + this-Run retrieval chunks; physical isolation — classifier physically sees no history; semantic refusal capability preserved (PII class); cost ≈ +700ms per turn. Refuses: full code-level take-over (loses semantic refusal); double insurance (overlap handling requires extra definitions). Existing SC-2 0-chunks / score<0.3 short-circuits untouched (they structurally don't read history).
- **D10-A2**: Past refusal rounds in history = neutral placeholder template (D9-L2 filler style: fixed string, zero refusal-semantic words like 「拒绝/不能/无权」). Templates must pass red test (adversarial injection cases must not trigger refusal drift). Refuses: full discard (agent forgets refusals → answers off-topic); keep full refusal text (proven -95pp drift source).
- **D10-A3**: Spike hard-gate re-run = final-plan full 35-case live run. Same 10 adversarial + 15 benign + 10 long, real GLM + Milvus, seed=42; four metric + refusal delta all return to gate = phase complete. Mirrors Phase 9 same-gate-same-aperture; result directly comparable.
- **D10-A4**: Frozen eval baseline = spike gate independent, old lock preserved untouched. Lock sha256 `50bb1688…` + numbers (faithfulness 0.972 / refusal 95%) preserved; Phase 10 refusal-behavior changes gated by spike hard-gate alone; frozen CI gate waiver allowed for refusal-path changes. Baseline v2 rebaselining deferred to v0.8 milestone close.

**B. History injection landing + Runner timing**
- **D10-B1**: History injection lands in VerdictPipeline (production default). Projection output `[]schema.Message` rendered into LLM request inside pipeline; consolidated changes, full regression suite as safety net, same spike re-gate. Runner migration moves history injection once at Phase 11 (projection output already Message-shaped, relocation cheap). Runner migration itself goes to Phase 11 (HITL Interrupt/Resume prerequisite). Refuses: migrate to ADK Runner immediately (non-default path unverified in production + two changes double debug surface); migration reserved abstraction layer (projection output already Message-shaped, abstraction redundant).
- **D10-B2**: History message structure = multi-turn messages prefix. Sanitized history rounds as native messages prefix (user/assistant alternating); last user = original query + chunks DATA (existing `userQueryTemplate` unchanged); system remains single. GLM follows native multi-turn role structure materially better than text concatenation. `streamAndAssemble` extends from 2-message to "prefix + 2-message". Refuses: single user three-segment (history is text not conversation, anaphora weak — spike injection approximates this); system summary box (loses turn structure + degrades instruction following).
- **D10-B3**: History window = token-budget driven. As many turns as fit within budget; older turns truncated from oldest first (project stage); 3 turns no longer hard cap but spike-verified fallback. Dumb budget first (RF-6: GLM tokenizer unconfigurable — char approximation); summarization middleware deferred to Phase 11 post-Runner migration (ROADMAP SC-5 order project→then→summarize preserved). Final re-gate if long-session subset fails → tighten window. Refuses: fixed 3-turn hard cap (long sessions only remember latest 3, MEM-05 project phase weak); env-configurable turn count (default still conservative 3, long-session experience unresolved).
- **D10-B4**: Budget/compression events observable = data-decision chunk extension. Reuse existing data-decision SSE channel (D16 observability substrate) — new decision types (e.g. `history_budget_truncated` / `history_projected`), FE optional rendering + backend log sync. Zero new wire types (Go/TS no D15 whitelist expansion), audit chain complete. Refuses: backend log only (user-invisible "observable" degenerates to ops log dive); new SSE chunk type (wire dual-side sync + D15 expansion, maintenance surface grows).

**C. Anaphora resolution strategy (MEM-01)**
- **D10-C1**: Follow-up turn retrieval query = query rewrite front-loaded. Follow-up turns: lightweight LLM rewrite first (sees history + follow-up → self-contained query 「那第二条呢」→「XX 制度第二条的内容」); rewritten query goes to retrieval + refusal classifier; generation layer still gets original query + history prefix. First turn / no anaphora → skip rewrite. Conversational query rewriting is the standard solution for session retrieval quality. Refuses: original query direct retrieval (「那第二条呢」0 chunks → SC-2 refusal — user asked already-answered → MEM-01 SC fails); zero-cost concatenation (embedding polluted by unrelated text, recall uncontrollable).
- **D10-C2**: Execution order = rewrite → classification. Follow-up turn chain: rewrite (sees history, outputs independent query) → rewritten query retrieval → refusal classifier (sees rewritten query + this-Run chunks) → generate only on pass. Classifier sees semantically complete query → accurate decision; rewrite output only for retrieval/classification, NOT evidence chain — even if malicious history pollutes rewrite text, no chunk from retrieval → refusal path (structural fallback). Refuses: classify then rewrite (anaphora unresolved, classifier can't decide, misjudgment high); classifier dual-query input (two queries inconsistent, decision criterion ambiguous).
- **D10-C3**: Rewrite result = transparent hint. Rewrite query sent via data-decision chunk (e.g. `decision=history_query_rewritten, query=<rewrite text>`), FE renders small caption above message bubble 「已理解为：XX 制度第二条」. User can detect rewrite misunderstanding and correct (re-ask); same channel as D16 decision observability; audit row syncs. Refuses: silent internal (misunderstanding → confusing refusal/wandering answer, trust-platform tone conflict); audit-only (UI hint: yes requires perceivable).

**D. FE refresh restoration (MEM-02 UI)**
- **D10-D1**: Restoration shape = full restoration. New API (e.g. `GET /api/session/:id/history`) returns structured history (role + rendered text + parsed verdict); FE restores via useChat `initialMessages` (D14: not in Zustand); history messages render with existing VerdictCard / CitationCluster components, citation chips clickable to chunk detail — visually identical to live messages. Data complete (session_turn stores raw verdictJSON, deterministic renderable). Refuses: backend memory FE white-board (user sees blank, thinks memory lost, contradicts UI hint "refresh continuity as perceivable surface"); simplified text restore (same session two visual forms + citations unclickable, fragmented UX).
- **D10-D2**: Skill attribution = auto-switch to session-bound Skill. On history restore, FE auto-switches to that session's bound Skill (history API returns skill_id, SkillPicker syncs selected; switch + hint on mismatch). Retrieval corpus always aligned with session context. Refuses: keep current Skill (history belongs to Skill A, retrieval against Skill B corpus, anaphora cross-corpus misalignment); block on mismatch (extra manual step, cumbersome UX).

### Claude's Discretion

- **Neutral placeholder template wording** — D10-A2 locks "fixed string + zero refusal-semantic words"; exact wording per implementation, must pass red test (malicious injection cases must not trigger refusal drift).
- **Dumb budget token estimation** — RF-6: GLM tokenizer unconfigurable; char approximation (e.g. Chinese 1 char ≈ 1 token conservative estimate) + reserved quota for retrieval chunks + output, specific constants per implementation + final re-gate calibration.
- **Rewrite failure/timeout degradation** — best-effort fallback to original query for retrieval (history goes to generation layer normally), no error no block.
- **Rewrite and refusal classifier prompt shape** — lightweight calls (low max_tokens, structured output), specific prompts per implementation; classifier output needs structured JSON (refuse / not-refuse + reason enum).
- **History API route naming/pagination/cap** — specific shape per planner (long sessions may limit recent N turns + cursor, aligned with D10-B3 budget aperture).
- **Projection layer rendering location** — verdictJSON → conclusionText rendering + refusalReason → neutral placeholder transition lives in `internal/history` projection layer (session_turn stays raw storage for eval/debug friendliness); specific function split per implementation.
- **data-decision new type naming** — `history_query_rewritten` / `history_budget_truncated` / `history_projected` are illustrative; implementation can adjust (keep decision string semantically clear).

### Deferred Ideas (OUT OF SCOPE)

- **Phase 11 Runner migration + summarization middleware enablement** — history injection moves from VerdictPipeline to ADK `Agent.Run` messages; MEM-05 summarize phase actually enables (this phase only project + dumb budget truncation).
- **frozen eval baseline v2 rebaseline** — defer to v0.8 milestone close (after Phase 11-13 behavior changes all land).
- **Resume injection text whitelist force reuse** — when Phase 11 resumes entire message list, reuse MEM-03 this-Run retrieval whitelist mechanism (invariant declared, implementation Phase 11).
- **Sub-agent output whitelist** — Phase 13 FAN-* outputs similarly cannot be evidence (invariant declared, implementation Phase 13).
- **Dumb budget tokenizer precision** — after RF-6 fully resolved (GLM tokenizer configurable or official count API), replace char approximation; non-blocking this phase.
- **Phase 7 leftovers (07-02 T3 / 07-05 T2/T3 / 07-06)** — user decision defer to post-v0.8, this phase does not touch.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **MEM-01** | User can issue follow-up anaphora queries (「那第二条呢」「依据是什么」); agent combines session history to correctly resolve referents and answer | D10-C1/C2/C3: query rewrite front-loaded; Section 6.4 prompt template; FE small caption rendering |
| **MEM-02** | Session history survives page refresh and feeds agent — server layer `internal/history` projects sqlite session → `[]schema.Message`; post-refresh follow-up still resolves correctly | D10-B1/B2 + D10-D1: projection.go `Project` (D9-B1/B2/B3 already shipped Phase 9) + extension verdictJSON renderer + neutral placeholder + budget window; new `GET /api/session/:id/history` endpoint; useChat `initialMessages` FE wiring |
| **MEM-03** | Citation whitelist code-forced — citation can only reference this-Run retrieval chunks; history / summary / resume-injected text cannot be evidence | D10-B2: history goes into messages prefix NOT user prompt DATA section; `verdict.GroundingChecker` already enforces chunkID ⊆ this-Run retrieval; SC-3 0/10 PASS line held in Phase 9 — no regression |
| **MEM-04** | History projection sanitization effective — projection messages carry no `[CHUNK id]` markers; history verdict JSON deterministically rendered via `conclusionText`/`refusalText` (test asserts projection output zero chunk markers) | D9-B1/B2 already locked; `scrubChunkRefs` in `internal/history/projection.go:72-78`; this phase adds verdictJSON → conclusionText renderer + refusalReason → neutral placeholder (Section 4) |
| **MEM-05** | Long sessions exceeding token budget compressed project→then→summarize order (dumb budget first; summarization middleware enabled post-Runner migration); session sustains without error; budget/compression events observable | D10-B3/B4 + Section 5: char-approximation budget constants; data-decision event extension `history_budget_truncated`; `TestSessionIsolation50x5` re-run on new projection path (Section 11) |
</phase_requirements>

## Summary

Phase 10 ships **multi-turn conversation memory** over the existing Go + Eino v0.9.14 + Hertz SSE trust pipeline (no framework change — only history injection landing pattern). The phase is gated by Phase 9 NO-GO acceptance with escape plan (user signed 2026-08-18): the trust floor must be hardened so refusal determination does not depend on history context (Pitfall 1 H2 confirmed -95pp refusal_accuracy drift).

The build pattern has three structural commitments:

1. **Query-only refusal classifier (D10-A1)** sits between retrieval and verdict stream. Its `RefusalClassifierInput` struct carries `{Query, Chunks}` ONLY — `History` is structurally absent at the type level; CI grep `! grep -rn 'historyMsgs\|GetSession\|loadHistory' internal/server/refusal_classifier.go` must return empty.

2. **Multi-turn messages prefix (D10-B2)** extends the existing 2-message `streamAndAssemble` slice with sanitized history as a `[]*schema.Message` prefix; the last user message stays `{original query + chunks DATA}` — evidence shape unchanged from Phase 2.

3. **Neutral placeholder for refusal rounds (D10-A2)** renders refusal-round assistant content as a fixed string template with zero refusal-semantic words (D9-L2 filler style), so the next-turn classifier doesn't anchor on prior refusal keywords.

The 4-wave execution plan is fully spec-locked:

- **W1 (BE foundation)**: extend `internal/history` with verdict-renderer + neutral-placeholder + budget-window subpackages; 7 existing fixtures stay green; new fixtures for refusal-rounds → placeholder; budget-window tests against char-approximation.
- **W2 (BE pipeline injection)**: add `internal/server/{rewriter,refusal_classifier,history_decisions}.go`; wire rewrite → retrieval → classifier → verdict in `VerdictPipeline`; extend `wire.DecisionPayload` decision strings with 3 new types (zero new wire types per D15 invariant); `streamAndAssemble` signature gains a `[]*schema.Message` history prefix parameter.
- **W3 (FE restoration)**: new `GET /api/session/:id/history` endpoint; `web/src/App.tsx` `useChat({initialMessages})` wiring; `web/src/components/HistoryRewriteHint.tsx` for the "已理解为：XX" caption (new components allowed; carries the new data-decision event surface); `SkillPicker` auto-switch hook.
- **W4 (end-to-end + spike gate)**: 35-case live re-run (`cmd/spike-history` × 3 segments + summarize) at $0.05 / 75min budget; PASS = `|Δ faithfulness| < 0.05 AND |Δ refusal| < 10pp AND injection_penetrated == 0/10`; frozen lock byte-identical at HEAD.

**The spike hard-gate is the phase acceptance gate (D10-A3).** No same-seed retry on jitter-scale NO-GO (D9-V3); the gate is structural, not stochastic. D10-A4 keeps the frozen lock untouched — refusal-path changes are recorded as waivers in the SUMMARY with measurement against the spike gate. Tokenizer precision (RF-6) is the lone research flag carried forward to Phase 11's summarization middleware enablement.

**Primary recommendation:** Execute the 4-wave plan verbatim per `10-CONTEXT.md` + `10-AI-SPEC.md`. The Spike gate's 35-case live run (real GLM-5.2 + Milvus + BGE-M3, seed=42, ~$0.05 + ~75min budget) is the single source of truth for "memory is safe to ship". The wave 1-2 boundary is the highest leverage (projection + classifier isolation gets 80% of the mitigation surface); wave 3-4 are observability + restore UX + the gate itself.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| History projection (sanitize + chunk-ref scrub + role filter + window truncate) | **API / Backend** (`internal/history/projection.go`) | — | Single-source-of-truth projection contract; D9-B1/B2/B3 already shipped Phase 9; extends with verdict-renderer + neutral-placeholder + budget-window subpackages |
| Multi-turn messages prefix assembly | **API / Backend** (`internal/server/pipeline.go streamAndAssemble`) | — | Pipeline is the production-default entry; signature extends to accept `[]*schema.Message` history slice; preserves Phase 2 audit hash for the prompt body |
| Query-only refusal classifier (D10-A1 isolation) | **API / Backend** (`internal/server/refusal_classifier.go`) | — | Type-level physical isolation; classifier input struct `{Query, Chunks}` has no History field; CI grep gate enforces |
| Query rewrite (anaphora resolution) | **API / Backend** (`internal/server/rewriter.go`) | — | Lightweight LLM call pre-retrieval; rewrite output for retrieval only, NOT evidence chain (D10-C1/C2) |
| Citation whitelist (this-Run only) | **API / Backend** (`verdict.GroundingChecker`) | — | D2-01 / D2-04 invariant already enforced; MEM-03 inherits + spike holds the line |
| History data-decision event emission | **API / Backend** (`internal/server/history_decisions.go`) | Frontend (FE optional render) | D10-B4/C3 reuse existing data-decision SSE channel; zero new wire types (D15 invariant); FE renders small caption via existing dispatcher |
| Token-budget truncation + observable event | **API / Backend** (`internal/history/budget_window.go`) | — | Char-approximation budget (RF-6 mitigation); oldest-first truncation with `safeguardFloor=2 turns`; emits `history_budget_truncated` data-decision |
| Audit row extension (log-only, no schema change) | **API / Backend** (sibling `audit_meta` table) | — | T-3-AUDIT INSERT-only invariant; join via query_id (AI-SPEC §7) |
| History REST API for refresh-restore | **API / Backend** (`GET /api/session/:id/history`) | — | Returns structured history for FE `useChat({initialMessages})`; per-session SC-9 isolation continues |
| Refresh restoration (FE) | **Frontend / Browser** (`web/src/App.tsx` `useChat initialMessages` + `SkillPicker` auto-switch) | API / Backend (history endpoint) | D14 Zustand-bans-messages invariant; `useChat` owns message stream; session-bound Skill auto-switch honors D10-D2 |
| "已理解为：XX" caption UI | **Frontend / Browser** (`web/src/components/HistoryRewriteHint.tsx`) | API / Backend (data-decision event emit) | User-perceivable transparency per UI hint: yes; new component, new TS data-decision variant (D15 mirror in one PR) |
| Spike hard-gate (acceptance gate) | **CLI / CI** (`cmd/spike-history --summarize`) | — | D10-A3; re-uses Phase 9 binary unchanged; 35-case live re-run; exit 0 = GO / 1 = NO-GO |
| 4-metrics eval + frozen baseline | **CLI / CI** (`internal/eval/`) | — | D6 reversal: Go-native; faithfulness ports Ragas v0.1.21 verbatim (Apache-2.0); other 3 Go-authored judge prompts; reused unchanged |

## Standard Stack

### Core (already in `go.mod`, no fresh install for this phase)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/cloudwego/eino` | **v0.9.14** (locked 2026-08-17 per D8-01) | `schema.Message`, `BaseChatModel.Generate/Stream`, callbacks | Locked in `go.mod`; Phase 8 Plan 08-01 atomically bumped v0.7.7→v0.9.14; v0.10.0-alpha.x explicitly avoided |
| `github.com/cloudwego/eino-ext/components/model/openai` | **v0.1.13** | GLM-5.2 OpenAI-compatible adapter via Volcano Ark | Already wired for Coding Plan endpoint; spike 003 proven response_format=json_object + thinking:disabled |
| `github.com/cloudwego/hertz` | **v0.10.6** | HTTP framework with native SSE (`pkg/protocol/sse.NewWriter`) | Phase 8 invariant; no upgrade this phase |
| `modernc.org/sqlite` | **v1.56.0** | Pure-Go sqlite (no cgo), session_turn DDL | Already wired; `AppendTurn` idempotent via UNIQUE constraint |
| `github.com/cloudwego/gopkg` | indirect | SchemaMessage helpers | — |

### Supporting (in-tree, no new Go packages)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/history` | (in-tree) | History projection contract — `Project`, `scrubChunkRefs`, fixtures | D9-B1/B2/B3 shipped Phase 9; this phase extends with verdict_renderer / neutral_placeholder / budget_window subpackages (D11 D18 interface-first: history package exports interface, sub-packages implement; existing `projection.go` stays stable — additive extensions only) |
| `internal/store/sqlite` | (in-tree) | `GetSession`, `AppendTurn`, CheckPointStore (Phase 8) | Used by `runAgentPath` already; this phase calls `GetSession` from VerdictPipeline too (Phase 9 stub; Phase 10 wires it for real) |
| `internal/verdict` | (in-tree) | GroundingChecker + Citation{ChunkID, Span, Tier} | D2-01 / D2-04 invariant; SC-3 0/10 PASS held in Phase 9 spike — no regression allowed |
| `internal/eval` | (in-tree) | 4 RAG metrics (faithfulness / answer_relevancy / context_precision / context_recall) + Judge abstraction | Reused by spike runner end-of-phase re-run |
| `internal/wire` | (in-tree) | `DecisionPayload`, `UIMessageChunk`, `EmitChunk` | D16 substrate; data-decision event types extended (zero new wire types) |
| `cmd/spike-history` | (in-tree) | 35-case live spike runner | D10-A3 end-of-phase re-gate; 12-flag CLI unchanged |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Query-only refusal classifier LLM call | Code-level heuristic (cosine-threshold + keyword) | Loses semantic refusal capability (PII class through tone was refused on Phase 2 spike 002); -95pp drift only fixed by having a real classifier above history |
| Multi-turn messages prefix | Single user three-segment (history in user prompt text) | GLM's native role-following materially better (D10-B2 evidence); text concatenation degrades anaphora; spike injection shape approximates this and yields the -95pp drift |
| Char-approximation budget | `eino-ext/model/openai.Encode()` for real tokenization | RF-6: GLM Coding Plan endpoint exposes no Encode hook; char approximation is the only v0.9.14-compatible option; calibration deferred to end-of-phase spike's per-row `usage` logs |
| `eino/adk Runner` migration (history into `Runner.Query` messages) | Stay on `VerdictPipeline` for Phase 10 | Runner requires HITL Interrupt/Resume (Phase 11) prerequisites; VerdictPipeline is the production default and Phase 9 escape plan gates the test surface here first |
| `summarization` middleware (`adk/middlewares/summarization`) | Dumb budget truncation only | Middleware token-counter uncalibrated to GLM (Pitfall 7 / RF-6); trigger too late = overflow 500; trigger too early = doubled cost; defer to Phase 11 post-Runner migration |
| New SSE chunk type (e.g. `data-history`) | Extend `data-decision` decision string vocabulary | D15 invariant: zero new wire types without two-side mirror PR; existing data-decision dispatcher can route on Decision string without FE restructure |

### Installation

**No fresh install required.** All packages already pinned in `go.mod`. Verify before writing the Standard Stack table:

```bash
grep -E 'cloudwego/eino|cloudwego/hertz|milvus-io|modernc.org/sqlite' D:/code/ClaudeCode/ai-agent/go.mod
# Expected (locked):
#   github.com/cloudwego/eino v0.9.14
#   github.com/cloudwego/eino-ext/components/model/openai v0.1.13
#   github.com/cloudwego/hertz v0.10.6
#   github.com/milvus-io/milvus-sdk-go/v2 v2.4.2
#   modernc.org/sqlite v1.56.0
```

## Package Legitimacy Audit

> **Required** whenever this phase installs external packages. Run the Package Legitimacy Gate protocol before completing this section.

**SKIPPED — no new external packages.** This phase adds only in-tree Go packages and TS files. All dependencies are already in `go.mod` and `package.json` (verified above). The slopcheck audit would return the empty set.

**Audit verdict:** PASS (zero new packages to verify). The only install-adjacent change is `web/src/components/HistoryRewriteHint.tsx` — a new TS file with no `npm install`. No registry verification required.

## Architecture Patterns

### System Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────────────┐
│                            User browser (FE)                                │
│  ┌──────────────┐        ┌──────────────────────────────┐                   │
│  │  ChatInput   │───────▶│   useChat (AI SDK v5)        │                   │
│  │  (SkillPicker│        │   - messages (in-memory)     │                   │
│  │   auto-switch│        │   - transport = RealChatTransport                  │
│  │   D10-D2)    │        │   - id: activeSessionId     │                   │
│  └──────────────┘        │   - initialMessages (D10-D1, │                   │
│         │               │     on mount after /history) │                   │
│         │               └────────────┬─────────────────┘                   │
│         ▼                            ▼                                     │
│  POST /api/chat (with X-Session-Id)            GET /api/session/:id/history│
└─────────────────────────────────────┬────────────────────────────┬────────┘
                                      │                            │
                                      ▼                            ▼
┌────────────────────────────────────────────────────────────────────────────┐
│                        Hertz HTTP layer (BE)                                │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                chatHandler (internal/server/chat.go)                  │  │
│  │  - decode + UTF-8 validate + sessionID resolve (existing)             │  │
│  │  - router → Loader → VerdictPipeline (production default)             │  │
│  │  - runAgentPath (opt-in USE_AGENT_PATH=true, Phase 11+)              │  │
│  └────────────────────────────┬─────────────────────────────────────────┘  │
│                               ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                VerdictPipeline (internal/server/pipeline.go)         │  │
│  │                                                                       │  │
│  │   STEP 0a: loadHistory  ─[ GetSession → SQLiteStore ]─                │  │
│  │   STEP 0b: history.Project() → sanitized []schema.Message             │  │
│  │              • D9-B1 role filter (user/assistant only)                 │  │
│  │              • D9-B2 scrubChunkRefs in user text → [REF-N]             │  │
│  │              • D10-A2 neutral_placeholder for refusal rounds           │  │
│  │              • D9-B3 window cutoff                                      │  │
│  │              • D10-B3 budget window (new — replaces 3-turn hard cap)   │  │
│  │   ─── emit "history_projected" data-decision event (D10-B4)            │  │
│  │                                                                       │  │
│  │   STEP 0c: loadHistory rows, build query rewriting                     │  │
│  │              if len(historyMsgs) > 0:                                  │  │
│  │                  rewriter.Rewrite(ctx, historyMsgs, query)             │  │
│  │                  → rewrittenQuery                                      │  │
│  │   ─── emit "history_query_rewritten" data-decision event (D10-C3)    │  │
│  │                                                                       │  │
│  │   STEP 1-2: tier resolve + embed REWRITTEN query (existing)           │  │
│  │   STEP 3-4: retrieve against rewritten query → chunks (existing)       │  │
│  │   STEP 5: SC-2 0-chunks refusal (existing — unchanged)                │  │
│  │   STEP 6: GEVL-01 low_score refusal (existing — unchanged)            │  │
│  │                                                                       │  │
│  │   STEP 7 (NEW): query-only refusal classifier (D10-A1)                 │  │
│  │              input  = RefusalClassifierInput{Query: rewrittenQuery,    │  │
│  │                                               Chunks: chunksToStrings}│  │
│  │              classify → allow | refuse (default-deny on error)         │  │
│  │              if refuse: emit structured refusal + audit + return      │  │
│  │   ─── emit "history_classifier_refused" data-decision (if applicable) │  │
│  │                                                                       │  │
│  │   STEP 8-10: streamAndAssemble + verdict parse + grounding             │  │
│  │              MESSAGES = [system] + historyMsgs + [user: query+chunks] │  │
│  │              (D10-B2 — multi-turn messages prefix; evidence shape     │  │
│  │               in LAST user = original query + chunks DATA unchanged)   │  │
│  │                                                                       │  │
│  │   STEP 11: post-verdict annotations (counter_evidence, redline)        │  │
│  │   STEP 12: text-end + finish-step + finish (terminal SSE lifecycle)   │  │
│  └────────────────────────────┬─────────────────────────────────────────┘  │
│                               │ writes audit row + session turn pair       │
│                               ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │           SQLiteStore (audit + session_turn DDL; INSERT-only)        │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼ SSE stream to FE
┌────────────────────────────────────────────────────────────────────────────┐
│                       LLM (GLM-5.2 via Volcano Ark)                         │
│  - verdict stream (existing Phase 2 path, extended with history prefix)     │
└────────────────────────────────────────────────────────────────────────────┘

                                    ▲
                                    │
        ┌───────────────────────────┴───────────────────────────────────┐
        │  Acceptance gate: cmd/spike-history (35-case live re-run)       │
        │   --category=adversarial + followup + history-length=long      │
        │   --summarize → exit 0 GO / exit 1 NO-GO                       │
        │   PASS = |Δ faith| <5% AND |Δ refusal| <10pp AND inject 0/10   │
        └────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── history/
│   ├── projection.go                       # existing — stable contract
│   ├── projection_test_fixtures.go         # existing — 7 fixtures stay green
│   ├── projection_test.go                  # NEW — TDD: drives fixtures
│   ├── verdict_renderer.go                 # NEW — verdictJSON → conclusionText (D10-A2 inverse)
│   ├── neutral_placeholder.go              # NEW — fixed template + zero-refusal-semantic guard
│   ├── neutral_placeholder_test.go         # NEW — TDD: red test fixture
│   ├── budget_window.go                    # NEW — char-approximation budget + oldest-first truncation
│   └── budget_window_test.go               # NEW — TDD: char budget cases
│
├── server/
│   ├── pipeline.go                         # existing VerdictPipeline — Phase 10 injects (D10-B1)
│   ├── chat.go                             # existing chatHandler — history load + skill_id propagation
│   ├── rewriter.go                         # NEW — QueryRewriter {System, User} prompt + Generate opts
│   ├── rewriter_test.go                    # NEW — happy path + timeout fallback
│   ├── refusal_classifier.go               # NEW — RefusalClassifierInput{Query, Chunks} + 3-retry JSON
│   ├── refusal_classifier_test.go          # NEW — red test malicious history, parse retry, default-deny
│   ├── history_decisions.go                # NEW — emitHistoryDecision wrapper around DecisionPayload
│   ├── history_api.go                      # NEW — GET /api/session/:id/history (MEM-02 refresh)
│   └── history_api_test.go                 # NEW — SC-9 isolation + session_turn rendering
│
├── eval/                                    # unchanged — reused by spike end-of-phase
│
├── wire/
│   └── chunk.go                            # existing DecisionPayload — 3 new decision strings (additive only)
│
cmd/
├── server/main.go                          # existing — D10 reuses
└── spike-history/                           # existing — D10-A3 re-gate reuses

web/src/
├── App.tsx                                 # MOD — useChat initialMessages wiring + SkillPicker auto-switch
├── stores/app-store.ts                     # unchanged (D14 invariant preserved)
├── components/
│   ├── VerdictCard.tsx                     # unchanged — reused for restored history
│   ├── CitationCluster.tsx                 # unchanged
│   ├── SkillPicker.tsx                     # MOD — auto-switch on history restore (D10-D2)
│   ├── HistoryRewriteHint.tsx              # NEW — renders "已理解为：XX" caption from data-decision
│   └── HistoryRewriteHint.test.tsx         # NEW — vitest
└── lib/
    └── real-chat-transport.ts              # unchanged — handles new data-decision Decision strings
```

### Pattern 1: Multi-turn messages prefix injection

**What:** Sanitized history flows into the LLM call as a native `[]*schema.Message` prefix; the evidence shape (last user = query + chunks DATA) is identical to Phase 2.

**When to use:** Every VerdictPipeline invocation. The history prefix is empty for first-turn queries (no behavior change for backward compat) and grows for follow-up turns.

**Example:**

```go
// file: internal/server/pipeline.go (extends existing VerdictPipeline)
// Phase 10 additions marked with // D10-*.

func (s *Deps) VerdictPipeline(ctx context.Context, c *app.RequestContext,
    query string, manifest *skill.Manifest, sessionID, skillID string) error {
    // ... existing Steps 1-6 (tier, embed, retrieve, 0-chunks, low_score) ...

    // === D10 STEP 0: history sanitize + rewrite + classifier ===
    historyMsgs, historyErr := s.loadHistory(ctx, sessionID)
    if historyErr != nil {
        // best-effort — T-3-AUDIT swallow pattern; emit observability event
        s.emitHistoryDecision(c, wire.DecisionPayload{Decision: "history_unavailable"})
    }
    var rewrittenQuery = query
    if len(historyMsgs) > 0 {
        if rw, rwErr := s.rewriter.Rewrite(ctx, historyMsgs, query); rwErr == nil && rw != "" {
            rewrittenQuery = rw
            s.emitHistoryDecision(c, wire.DecisionPayload{
                Decision: "history_query_rewritten",
                Query:    rewrittenQuery, // FE renders "已理解为：XX" caption from this
            })
        }
        // else: best-effort fallback to original query (D10-C1 explicit "不阻断")
    }
    // Re-embed the rewritten query (cheap; BGE-M3 embedder ~300ms p95)
    qvec, _ := s.Embedder.Embed(ctx, rewrittenQuery)
    chunks, _ := s.Retriever.Search(ctx, qvec, tiers, 5, time.Now().Unix())

    // === D10 STEP 7b: query-only refusal classifier (PHYSICAL isolation) ===
    // RefusalClassifierInput has NO History field — type-level isolation.
    // CI grep `! grep -rn 'historyMsgs\|GetSession\|loadHistory'
    //                       internal/server/refusal_classifier.go` returns empty.
    clsOut, clsErr := s.refusalClassifier.Classify(ctx, RefusalClassifierInput{
        Query:  rewrittenQuery,
        Chunks: chunksToRenderedBlocks(chunks),
    })
    if clsErr != nil || clsOut.Decision == "refuse" {
        s.emitRefusalFromClassifier(c, clsOut)
        s.writeAuditAndTurn(ctx, sessionID, skillID, query, verdict.Verdict{},
            "", clsOut.Reason)
        return nil  // history NEVER reaches the verdict LLM
    }

    // === D10 STEP 8: streamAndAssemble with multi-turn messages prefix ===
    // The system prompt is unchanged — code-forced single System.
    // The messages prefix is the sanitized history (sanitization already
    // applied in internal/history.Project + neutral placeholder).
    // The LAST user message is the original query + chunks DATA —
    // the EVIDENCE shape is byte-identical to Phase 2; only the
    // multi-turn context above changes.
    sysPrompt := fmt.Sprintf(systemInstructionsTemplate)
    userPrompt := fmt.Sprintf(userQueryTemplate, query, chunksToRenderedBlocks(chunks))
    raw, err := s.streamAndAssembleWithHistory(ctx, c,
        sysPrompt, userPrompt, historyMsgs, chatModelOpts)
    // ... parse, validate, grounding, emitVerdictStream (unchanged) ...
}

// streamAndAssembleWithHistory extends Phase 2's 2-message slice to
// "system + history slice + user". The signature widens; the LAST
// user message (query + chunks DATA) is unchanged shape. The system
// message stays single — code-forced, not appended.
//
// All existing tests that construct Deps without history still call
// the original streamAndAssemble (or this version with empty
// historyMsgs → produces identical byte output).
func (s *Deps) streamAndAssembleWithHistory(
    ctx context.Context,
    c *app.RequestContext,
    system, user string,
    history []*schema.Message,  // empty slice = first turn = Phase 2 behavior
    opts []model.Option,
) (string, error) {
    msgs := make([]*schema.Message, 0, 2+len(history))
    msgs = append(msgs, schema.SystemMessage(system))
    for _, hm := range history {
        msgs = append(msgs, hm)  // already sanitized in projection layer
    }
    msgs = append(msgs, schema.UserMessage(user))

    // ... same retry wrapper, same Stream call, same token-buffer assembly
    // ... same text-delta-draft SSE emit, same verdict parse downstream ...
}
```

### Pattern 2: Type-level physical isolation for refusal classifier

**What:** The classifier's input struct carries `{Query, Chunks}` ONLY. A code reviewer adding a `History` field triggers the compile-time alarm (godoc + `go vet` + CI grep gate).

**When to use:** Whenever a new "judge" sub-call is added to the pipeline — Phase 11 HITL classifier could conceivably see history (and must NOT).

**Example:**

```go
// file: internal/server/refusal_classifier.go (D10-A1)

package server

import "github.com/cloudwego/eino/schema"

// RefusalClassifierInput is the PHYSICALLY-ISOLATED input to the refusal
// classifier. The classifier's decision drives whether the verdict LLM
// ever sees the rewritten query + this-Run chunks; it MUST NOT see
// session history.
//
// The struct deliberately carries ONLY (Query, Chunks). There is no
// `History` field. A code reviewer adding one will:
//   1. Read this godoc + the D9-V2 escape-plan reference
//   2. Find no peer field that justifies the addition
//   3. Trip the CI grep gate:
//        `! grep -rn 'historyMsgs\|GetSession\|loadHistory'
//         internal/server/refusal_classifier.go` must return empty
//
// The classifier never imports internal/store (no GetSession call),
// never imports internal/history (no Project call), and its tests
// construct inputs directly.
type RefusalClassifierInput struct {
    Query  string   `json:"query"`   // rewritten query from rewriter (or original on first-turn)
    Chunks []string `json:"chunks"`  // rendered [ref-N] blocks, same DATA framing as userQueryTemplate
}

type RefusalClassifierOutput struct {
    Decision   string  `json:"decision"`   // enum: "allow" | "refuse"
    Reason     string  `json:"reason"`     // enum: RefusalReasonOutOfScope / OutOfTopic / PIIRequest / ...
    Confidence float64 `json:"confidence"` // [0.0, 1.0]; > 0.5 trust the decision
}

var validClassifierDecisions = map[string]bool{"allow": true, "refuse": true}

func (c *RefusalClassifier) Classify(ctx context.Context, in RefusalClassifierInput) (
    RefusalClassifierOutput, error) {
    sysPrompt := classifierSystemTemplate // small; instructs JSON-only, enum-validated
    userPrompt := fmt.Sprintf(classifierUserTemplate, in.Query, strings.Join(in.Chunks, "\n\n"))
    var lastErr error
    for attempt := 1; attempt <= 3; attempt++ {
        resp, err := c.cm.Generate(ctx, []*schema.Message{
            schema.SystemMessage(sysPrompt),
            schema.UserMessage(userPrompt),
        },
            model.WithTemperature(0),
            model.WithMaxTokens(60),  // D-2/4b.3: small structured JSON
            openai.WithExtraFields(structOutFields))
        if err != nil { lastErr = err; continue }
        var out RefusalClassifierOutput
        if perr := json.Unmarshal([]byte(llm.ExtractJSON(resp.Content)), &out); perr != nil {
            lastErr = perr; continue
        }
        if !validClassifierDecisions[out.Decision] {
            lastErr = fmt.Errorf("invalid decision %q", out.Decision); continue
        }
        if out.Confidence < 0.5 {
            lastErr = fmt.Errorf("confidence too low %.2f", out.Confidence); continue
        }
        return out, nil
    }
    // 3 consecutive failures → default-deny "refuse" with low confidence,
    // returned to caller. Caller emits structured refusal + audit row.
    // Better to over-refuse than hallucinate (Pitfall 1 mitigation).
    return RefusalClassifierOutput{
        Decision: "refuse",
        Reason:   verdict.RefusalReasonOutOfScope,
        Confidence: 0.0,
    }, fmt.Errorf("classifier exhausted retries: %w", lastErr)
}

// Compile-time check that this file does NOT import internal/history
// (the type-level isolation is meaningless if a developer can reach
// history through a back door). The `var _` references trigger a
// compile error if internal/history becomes empty (a heavier version
// of the CI grep gate).
var _ = historyProject  // invalid; caught at code review / CI
```

### Pattern 3: Neutral placeholder for refusal-rounds in history

**What:** When a session_turn's assistant content carries a refusal reason string, projection renders it as a fixed neutral template instead of the literal refusal text. This prevents the next-turn classifier from anchoring on refusal-semantic words.

**When to use:** For every refusal-round assistant message in `internal/history.Project`.

**Example:**

```go
// file: internal/history/neutral_placeholder.go (D10-A2, D10-A2 escape plan)

package history

import "github.com/skillforge/skillforge/internal/verdict"

// refusalRoundReasons is the set of refusal-reason codes we recognize
// as a refusal-round; matches the verdict.RefusalReason* constants +
// the semantic refusal payload values. If the assistant content
// starts with one of these, the round is treated as a refusal
// round and rendered as the neutral template.
var refusalRoundReasons = map[string]bool{
    verdict.RefusalReasonNoAuthorizedSource: true,
    verdict.RefusalReasonLowRetrievalScore:  true,
    verdict.RefusalReasonOutOfScope:         true,
    "out_of_topic":                          true, // classifier refusal reason enum
    "pii_request":                           true, // classifier refusal reason enum
}

// isRefusalRound reports whether m.Content looks like a refusal-round
// assistant message (the verdict pipeline stores refusal_reason into
// the assistant session_turn content for refusal paths).
func isRefusalRound(m store.SessionMessage) bool {
    if m.Role != "assistant" { return false }
    return refusalRoundReasons[m.Content]
}

// NeutralPlaceholderTemplate is a fixed string with ZERO refusal-
// semantic words (无 拒绝 / 无权 / 不能 / 抱歉 / 抱歉 / 无法).
//
// RED-TEST requirement (D10-A2): the string passes a T9-FILLER-LEAK
// red test against a corpus of historical adversarial rows
// (hist-adv-001..010 + 5 post_refusal subclass rows). The test:
//
//   1. Renders the placeholder into the LLM prompt
//   2. Asserts the classifier decision rate on the post_refusal
//      subclass is >= 4/5 (PASS same as Phase 8 baseline)
//   3. Asserts the 10 adversarial history_injection rows STILL
//      produce 0 penetration (no history text became a verdict citation)
//
// If either assertion fails, the template wording needs revision.
// This test is in internal/history/neutral_placeholder_test.go.
const NeutralPlaceholderTemplate = "已记录上文。」"

func RenderNeutralPlaceholder(turnIndex int) string {
    // Stable per-turn text — turnIndex is unused here, kept for the
    // signature so future schemas (per-turn index, timestamp, etc.)
    // can fold in without touching call sites.
    return NeutralPlaceholderTemplate
}
```

### Pattern 4: Char-approximation token budget for history window

**What:** The history window truncates by character count (a proxy for token count, since GLM tokenizer is unconfigurable per RF-6). Calibration constants chosen conservatively.

**When to use:** Every VerdictPipeline call (after `Project`, before retrieval).

**Example:**

```go
// file: internal/history/budget_window.go (D10-B3)

package history

import "github.com/cloudwego/eino/schema"

// Token-budget constants. RF-6: GLM Coding Plan endpoint exposes no
// Encode(tokenize) hook in eino-ext/model/openai v0.1.13, so a real
// tokenization is unavailable. We use character approximation:
//   - Chinese: 1 character ≈ 1 token (GLM has no shared Chinese tokenization
//     exposed; BGE-M3 has multi-lingual tokenizer but not usable here)
//   - English: 1 character ≈ 0.25 token (whitespace-separated average)
// The constants below are PRE-Phase-10 calibration targets,
// conservative (over-truncate rather than overflow 500):
//
//   defaultCharBudget       = 8000 chars (~2000-5000 tokens worst case)
//   reservedForChunks       = 7500 chars (5 chunks × ~1500 chars upper bound)
//   reservedForOutput       = 600 chars (~600 tokens at 1 char/token max)
//   reservedForSystemPrompt = 2500 chars (systemInstructionsTemplate rendered)
//   reservedForQueryAndUser = 2000 chars (rewritten query + user prompt framing)
//
// 8000 (history) + 7500 (chunks) + 600 (output) + 2500 (system) + 2000 (query)
//   ≈ 20 600 chars ≈ 12-20k tokens in worst-case CN→EN mixed
//   < GLM-5.2 Coding Plan's documented 16k context window with margin.
//
// The end-of-phase 35-case spike is the empirical gate: if
// long-history faithfulness regresses, tighten defaultCharBudget
// (e.g. 6000) and re-run. The spike IS the calibration loop.
const (
    DefaultCharBudget        = 8000
    ReservedForChunks        = 7500
    ReservedForOutput        = 600
    ReservedForSystemPrompt  = 2500
    ReservedForQueryAndUser  = 2000
    FallbackTurnsIfBudgetZero = 3
    SafeguardFloor           = 2 // keep at least 1 user/assistant pair
)

// TruncateToCharBudget reduces messages to fit within budget chars.
// Oldest-first drop (project-then-summarize order: project stage first;
// summarization middleware in Phase 11). Returns (Kept, DroppedCount,
// Truncated bool).
//
// semantics:
//   1. The kept slice is a valid alternating user/assistant prefix
//      (B3 invariant from projection).
//   2. Truncated=true when at least one message was dropped.
//   3. safeguardFloor guarantees at least 1 user+assistant pair (2 msgs)
//      survives the truncation — never collapse to empty.
//   4. If total chars already fits, returns (input, 0, false) — no work.
func TruncateToCharBudget(messages []schema.Message, budget int) (
    []schema.Message, int, bool) {
    if budget <= 0 {
        budget = DefaultCharBudget
    }
    if len(messages) <= SafeguardFloor {
        return messages, 0, false
    }
    total := totalChars(messages)
    if total <= budget {
        return messages, 0, false
    }
    // Oldest-first drop with floor
    kept := append([]schema.Message(nil), messages...)
    dropped := 0
    for len(kept) > SafeguardFloor && totalChars(kept) > budget {
        kept = kept[1:] // drop oldest
        dropped++
    }
    if dropped == 0 {
        return messages, 0, false
    }
    return kept, dropped, true
}

func totalChars(messages []schema.Message) int {
    n := 0
    for _, m := range messages {
        n += len(m.Content)
    }
    return n
}
```

### Pattern 5: data-decision event emission for history observability

**What:** The pipeline emits typed `data-decision` chunks at each history-related transition: `history_projected`, `history_budget_truncated`, `history_query_rewritten`. Zero new wire types (D15 invariant preserved).

**When to use:** Every history-related pipeline step (W1-W2).

**Example:**

```go
// file: internal/server/history_decisions.go (D10-B4/C3)

package server

import "github.com/skillforge/skillforge/internal/wire"

// emitHistoryDecision emits ONE data-decision SSE chunk for a history
// observability event. The Decision string is the discriminator; the
// optional Query field carries the rewritten query (D10-C3). Zero new
// wire types per D15 — this reuses data-decision exactly the way D5-04
// (route) and D5-13 (counter_evidence) did.
//
// Audit row sync: log lines ([INFO]/[WARN]) carry the same data so
// the backend-only log path has parity with the user-visible render.
func (s *Deps) emitHistoryDecision(c *app.RequestContext, p wire.DecisionPayload) {
    payload, err := json.Marshal(p)
    if err != nil {
        log.Printf("[WARN] server: marshal history decision: %v (non-fatal)", err)
        return
    }
    if err := wire.EmitChunk(c, wire.UIMessageChunk{
        Type: "data-decision",
        Data: payload,
    }); err != nil {
        log.Printf("[WARN] server: emit history decision: %v (non-fatal)", err)
    }
}

// Step convention — all history-related decisions use Step: 0 because
// the history layer precedes the first agent hop (Step 1 is the first
// hop; history is "pre-hop framing"):
//   history_projected       → emit AFTER history.Project() (D10-B1)
//   history_budget_truncated → emit AFTER TruncateToCharBudget (D10-B3)
//   history_query_rewritten  → emit AFTER rewriter success (D10-C3)
//   history_unavailable      → emit ON GetSession error (T-3-AUDIT swallow pattern)
//
// The TS side's SkillForgeChunk union gains 4 new Decision enum values
// (additive, no payload schema change). The D15 two-PR invariant
// (Go + TS in the same commit) is preserved.
```

### Anti-Patterns to Avoid

- **Single user three-segment**: don't concatenate history + query + chunks into one user message. GLM role-following is materially weaker for text vs. native messages.
- **Heuristic refusal threshold** (cosine-threshold + keyword-matches "无法"/"拒绝"): loses semantic refusal capability (PII class was the live Phase 2 spike 002 example).
- **Tokenize via BGE-M3 or another model**: it's an embedding model, not a tokenizer; the resulting token IDs don't match GLM's expected sequence. Char approximation is the only honest answer.
- **Run the summarization middleware now**: token-counter uncalibrated to GLM (Pitfall 7 / RF-6); trigger too late = context overflow 500. Char budget + Project stage only until Phase 11.
- **History-fed classifier call** ("give the classifier the recent verdict to check consistency"): this is EXACTLY the H2 failure mode (Pitfall 1). Classifier input struct carries Query + Chunks ONLY.
- **Update existing D9-B fixtures as part of Phase 10**: the 7 existing fixtures are an exported contract (D9-B4); any rewrite breaks Phase 9 evidence chain. Phase 10 ADDS new fixtures and extends Project via subpackages.
- **Adopt new SSE chunk types for history events**: D15 invariant requires two-side PR + drift alarm; reuse data-decision (the existing extensible substrate).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tokenizer for char → token | Custom GLM tokenizer wrapper | Char approximation constants (Section 5) | RF-6: GLM endpoint exposes no Encode hook; custom tokenizer needs GLM internals; char approximation is calibrated at end-of-phase spike |
| Anti-prompt-injection scanner for history text | String-based sanitizer scanning for "ignore previous instructions" etc. | T-2-21 DATA framing + scrubChunkRefs (existing) | LLM bypassing such string blocks is well-documented (T-2-21); DATA framing is structurally immune |
| Multi-agent fan-out for rewrite | Two agents: one rewrites, one classifies | Single lightweight LLM calls (rewriter + classifier) | The two tasks are independent sequential operations on different inputs; no need for orchestration; saves ~1.5s latency |
| In-house history summarization LLM | Inline summary generation per turn | Phase 11 summarization middleware + TriggerCondition | Eino v0.9.14 already provides this in `adk/middlewares/summarization`; calibration deferred to Phase 11 |
| Persistent checkpoint for conversation state | New sqlite tables + serializers | Existing CheckPointStore (Phase 8 D8-03) | D8-03 added `gob.RegisterName` + idempotent upsert for Phase 11; Phase 10 doesn't need it (the session_turn table already provides durable per-session history) |
| Custom SSE chunk type for history | New `data-history` enum | Existing `data-decision` discriminator + 3 new Decision string values | D15 invariant + D5-04 / D5-13 precedent; FE MessageBubble dispatcher already routes on `payload.decision` string |

**Key insight:** The "natural" custom builds for this domain (custom tokenizers, history-aware classifiers, custom SSE types) all have well-supported standard alternatives in v0.9.14 — the discipline is to recognize when the framework already provides a primitive and route through it instead.

## Runtime State Inventory

> Phase 10 is NOT a rename/refactor/migration phase — it does NOT trigger the canonical 5-category inventory. This section is included for completeness because the phase touches (a) the session_turn schema (raw storage, NOT a schema change) and (b) the audit_meta sidecar introduced in this phase.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **session_turn rows** — `internal/store/sqlite/schema.go:63-70` DDL: `(id PK, session_id, role CHECK in ('user','assistant'), content, ts)` UNIQUE(session_id, role, ts); verdictJSON stored as literal assistant content (no separate column). **No schema change**. New writes by Phase 10 carry the same shape — neutral placeholder is just a different string for the refusal-round assistant content. | Code edit only — no migration; existing rows remain read-compatible. The neutral placeholder logic applies at projection time, never at write time. |
| Stored data | **audit_meta sibling table (NEW)** — Phase 10 introduces a sibling `audit_meta` table (NOT a column on existing `audit`) for the new log-only fields (`rewritten_query`, `classifier_decision`, `classifier_reason`, `classifier_confidence`, `history_projected_n_turns`, `history_budget_truncated`, `history_dropped_count`, `sanitized_refusal_rounds`). CREATE TABLE IF NOT EXISTS in migrate() — additive, no ALTER on existing `audit` table (T-3-AUDIT INSERT-only invariant preserved). | Code edit only — DDL added to `schemaSQL`. Existing audit rows left untouched. |
| Live service config | None. The pipeline reads env vars unchanged. | No action. |
| OS-registered state | None. No Task Scheduler / launchd / systemd entries. | No action. |
| Secrets/env vars | None new. `ENABLE_HISTORY_INJECTION` env flag is OPTIONAL for Phase 10 rollback path; default `true` (Phase 11 can flip to false to disable history injection one-keystroke). If implemented, it's a documented optional knob, no secret material. | Code edit only. |
| Build artifacts | None. Phase 10 ships all Go + TS additions; no compiled-binary / docker-image / npm-package ecosystem changes. | No action. |

**Nothing found in OS-registered / secrets / build artifacts categories** — verified by directory inspection (`internal/` + `web/src/` + `cmd/` + no `bin/` `dist/` `node_modules/` shipped). The 5-category audit confirms Phase 10 is additive only.

## Common Pitfalls

### Pitfall 1: Classifier sees history through a side door

**What goes wrong:** A code reviewer adds a `History` field to `RefusalClassifierInput` or wires `s.Store.GetSession` into the classifier's call chain. The compile-time isolation breaks silently; the next spike run detects -20% refusal accuracy.

**Why it happens:** "Useful" additions creep in: "let the classifier see the last verdict to detect consistency drift" or "let the classifier skip work when the user is mid-conversation". These all violate D9-V2 escape-plan item (a).

**How to avoid:**

1. The `RefusalClassifierInput` struct godoc explicitly references D9-V2 ("type-level physical isolation"); `var _` reference to a sentinel missing function makes accidental addition trip code review immediately.
2. CI grep regression: `! grep -rn 'historyMsgs\|GetSession\|loadHistory\|Project(' internal/server/refusal_classifier.go` MUST return empty in `internal/server/` PRs.
3. End-of-phase spike re-run is the empirical gate — if refusal_rate is > 0 on hist-followup subclass, the classifier probably learned to read history.

**Warning signs:** `git log internal/server/refusal_classifier.go` shows signature churn; any test file references `Project` or `GetSession`.

### Pitfall 2: Neutral placeholder leaks refusal semantics

**What goes wrong:** A naive implementation of `neutral_placeholder` uses "已记录上文，上下文为：未授权知识源" — the phrase 「未授权知识源」triggers refusal-drift in the next-turn classifier, even though it's "neutral" from a developer's POV.

**Why it happens:** Chinese refusal vocabulary includes many neutral-sounding words that the LLM anchors on. 「拒绝」「无法」「不在范围内」「对不起」are obviously refusal-semantic; 「未授权知识源」「检索未命中」「无依据」are subtly refusal-semantic in context.

**How to avoid:**

1. Template wording passed red test in `internal/history/neutral_placeholder_test.go`:
   - Renders the placeholder in a red-row stimulus + the post_refusal subclass rows from the golden set
   - Asserts 5/5 post_refusal rows still classify correctly (no drift)
   - Asserts 10/10 hist-adversarial rows STILL produce 0 penetration (placeholder text + injected fake chunk in user text — verdict must refuse)
2. Regex guard in CI: `! grep -E '拒绝|无权|无法|不能|未授权|不在|抱歉' internal/history/neutral_placeholder.go` MUST return empty except in code comments.
3. End-of-phase spike re-run's hist-followup post_refusal subclass is the empirical gate.

**Warning signs:** Template revisions in PR comments without corresponding test updates; refusal_rate on hist-followup post_refusal == 0/5.

### Pitfall 3: Budget window drops the user's anaphora anchor

**What goes wrong:** User asks "那第二条呢？" referencing clause 2 from turn 7; the budget truncation algorithm preserves only turns 14-15 (latest), losing turn 7. Rewriter sees no anaphora context, returns a poor rewrite → SC-2 0-chunks refusal → user gets "I don't know" on a question the agent already answered.

**Why it happens:** Oldest-first truncation is the simplest correct direction for short contexts, but a 4000-char history that ends with a 3-message pair has an older turn that contains the anaphora anchor. Truncation drops the anchor; rewrite fails.

**How to avoid:**

1. `safeguardFloor=2` (1 pair minimum) ensures we never collapse to empty; but a 12-message history truncated to 2 might still drop the anchor.
2. Better: the rewriter LLM call MUST inspect the FULL historical conversation (including the truncated-out part) so the rewrite can sometimes succeed without the full history being in the messages prefix. Concretely: the rewriter input carries the projected history (full), but only the budget-fitting slice is sent to the generation LLM. This separates "what context the rewriter sees" from "what context the verdict LLM sees" — and lets the rewriter detect that its truncate-out was problematic.
3. End-of-phase spike: each hist-followup case has a `baseline_id` and golden-set expected output. Spike runner asserts faithfulness >= baseline for that subclass.

**Warning signs:** `history_dropped_count > 0` correlates with low ContextRecall on hist-followup subclass.

### Pitfall 4: Rewrite failure consumes significant latency

**What goes wrong:** The rewriter LLM call takes 4+ seconds on a slow network spike → total first-token latency exceeds 6s budget → user sees a frozen UI. D10-C1's "best-effort fallback to original query" is correct but the user-perceived latency is bad.

**Why it happens:** LLM API calls are non-deterministic in latency; first-token latency on GLM Coding Plan endpoint varies widely (Phase 8 spike 002 documented p95 = 800ms, but p99 spikes to 3-5s).

**How to avoid:**

1. Hard timeout on the rewriter call: `context.WithTimeout(ctx, 2*time.Second)`; on timeout, fall back to original query (no error emit; no block).
2. The rewriter call runs ONLY on follow-up turns with `len(historyMsgs) > 0`. First-turn queries skip the rewriter entirely (no measurable overhead for the common case).
3. The rewriter's small max_tokens (80) keeps the call payload tiny → first-token latency is dominated by round-trip + model warm-up; cap model warm-up latency by reusing the existing `PipelineChatModel` connection.

**Warning signs:** spike runner records p95 latency > 6s on follow-up rows.

### Pitfall 5: data-decision event ordering breaks FE dispatcher

**What goes wrong:** `emitHistoryDecision("history_projected")` fires at pipeline entry; `emitHistoryDecision("history_query_rewritten")` fires after retrieval; the FE MessageBubble dispatcher renders them out of order because the latter frame arrives during the LLM wait. The user sees "已理解为：XX" before the actual rewritten query is wired to the eventual verdict.

**Why it happens:** SSE is a stream; FE renders as chunks arrive. `history_query_rewritten` arrives BEFORE the verdict text-delta (it must, for the UI to render the caption above the message bubble). This is by design — but the dispatcher must know the timing.

**How to avoid:**

1. Phase 5-04 dispatcher already handles decision string routing (route, counter_evidence, etc.); Phase 10 adds 4 new decision strings in the same discriminator — additive, no dispatcher change.
2. The FE MessageBubble renders the caption INSIDE the assistant message bubble (not as a separate UI element). The text-delta for the verdict arrives AFTER history_query_rewritten in the stream — the bubble UI holds the caption in state until the verdict text arrives, then renders both as a single message.
3. Vitest extension: `data-decision with history_query_rewritten arrives before text-delta → caption renders above text`; `history_projected arrives before text-delta → no caption rendered (audit-only event)` — both cases covered.

**Warning signs:** Captions appear above unrelated message bubbles on the FE; event ordering assertions in vitest fail.

### Pitfall 6: SC-9 isolation regresses when `GetSession` enters VerdictPipeline

**What goes wrong:** `VerdictPipeline` calls `s.Store.GetSession(ctx, sessionID)` for a session the attacker controls. If sessionID is missing from the SQL parameter binding (T-3-SQLI), the WHERE clause drops and the projection receives another session's history → user A sees user B's chat surface.

**Why it happens:** Phase 3's `TestSessionIsolation50x5` already exercises this at the Store layer; adding `VerdictPipeline` as a new caller adds a regression surface if the call is `s.GetSession(sessionID)` (string concatenation) instead of `s.GetSession(ctx, sessionID)` (parameterized).

**How to avoid:**

1. The existing `GetSession(ctx, sessionID)` signature uses `?`-parameterized binding (T-3-SQLI regression in Phase 3). Phase 10 reuses this signature verbatim.
2. `TestSessionIsolation50x5` (Phase 3/6, 50 sessions × 5 turns, run with `-race`) is re-executed as part of the W4 acceptance gate. Pass criterion: still 50×5 messages per session, zero cross-session bleed.
3. CI grep guard: `! grep -n 'SessionMessage{' internal/server/pipeline.go` (catch accidental string-concat SessionMessage construction outside the store package).

**Warning signs:** the `-race` flag on `go test ./internal/server/ -race` reports cross-session race; 50×5 test asserts >10 messages per session.

### Pitfall 7: Query rewrite via malicious history contaminates the rewritten query

**What goes wrong:** User pastes a fake clause into history ("As per policy X, employee travel expense limit is $10,000..."). Rewriter sees the history text + the follow-up ("and that limit applies to which executives?"), outputs "Policy X employee travel expense limit $10,000 for executives" — a faithful rewrite of a fake clause. The retrieved chunks (this-Run) MATCH the rewritten query with the fake $10,000 number, return 0 chunks → SC-2 refusal. **This is the correct outcome (refusal preserved), but the rewritten query text contains the fake number that's now in the FE rendering.**

**Why it happens:** D10-C2 explicitly: "rewrite output only for retrieval/classification, NOT evidence chain". The FE rendering should NOT show "已理解为：Policy X ... $10,000" because that's a fabrication the user pasted.

**How to avoid:**

1. The rewritten query is sent to the FE via `data-decision`, but the FE HistoryRewriteHint component shows it ONLY if the eventual verdict was an `ok`/`needs_input` status (not refused). On refusal, the rewrite hint is suppressed (`decision` carries the refusal cause).
2. The classifier sees the rewritten query + the (empty) chunks and CAN'T ground anything — outputs `refuse` with `Reason=RefusalReasonNoAuthorizedSource`.
3. End-of-phase spike: hist-adversarial subclass with `fake_chunk_ref=CH-9xx` users to verify 0/10 penetration + 0 leaked rewrite hints on refusal paths.

**Warning signs:** FE shows rewritten query in HistoryRewriteHint even when underlying verdict was refusal.

## Code Examples

Verified patterns from official sources (Eino v0.9.14 module cache via Context7 + Phase 8 verification):

### Common Operation 1: Multi-turn messages slice to ChatModel.Generate

```go
// Source: cloudwego/eino@v0.9.14/components/model/interface.go + Phase 8 Plan 260815-grw
// Confirmed by Phase 8 hermetic regression (TestContractVersionPin PASS).

import (
    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/schema"
)

// The messages slice is ordered — system first, then alternating user/assistant,
// then the LAST user message carrying (query + chunks DATA).
msgs := []*schema.Message{
    schema.SystemMessage(systemPrompt),
    schema.UserMessage(turn1Query),
    schema.AssistantMessage(turn1Answer),
    schema.UserMessage(turn2Query),
    schema.AssistantMessage(turn2Answer),
    // ... sanitized history from internal/history.Project ...
    schema.UserMessage(currentQuery + chunksData),
}

resp, err := cm.Generate(ctx, msgs, opts...)
// or stream:
stream, err := cm.Stream(ctx, msgs, opts...)
```

### Common Operation 2: Structured JSON output via OpenAI extra fields

```go
// Source: cloudwego/eino-ext@v0.1.13/components/model/openai
// + Phase 8 spike 003 (100% schema parseable).

import "github.com/cloudwego/eino-ext/components/model/openai"

opts := []model.Option{
    model.WithTemperature(0.0),
    model.WithMaxTokens(60), // small structured responses
    openai.WithExtraFields(map[string]any{
        "response_format": map[string]string{"type": "json_object"},
        "thinking":        map[string]string{"type": "disabled"}, // spike-003 proven
    }),
}
```

### Common Operation 3: Streaming reader pull pattern

```go
// Source: cloudwego/eino@v0.9.14/schema/message.go + PipelineChatModel.Stream
// + Phase 8 streamAndAssemble (existing pattern, signature-widening only).

stream, err := cm.Stream(ctx, msgs, opts...)
if err != nil {
    return fmt.Errorf("open LLM stream: %w", err)
}
defer stream.Close()

var buf strings.Builder
for {
    msg, recErr := stream.Recv()
    if errors.Is(recErr, io.EOF) {
        break
    }
    if recErr != nil {
        return buf.String(), fmt.Errorf("recv LLM token: %w", recErr)
    }
    if msg == nil || msg.Content == "" {
        continue
    }
    buf.WriteString(msg.Content)
    // emit text-delta-draft here (existing pipeline.go streamOnce pattern)
}
```

### Common Operation 4: data-decision event emission (existing pattern)

```go
// Source: internal/server/annotate.go (counter_evidence) + chat.go (route)
// + wire.DecisionPayload (v0.9.14 stable contract). Phase 10 adds 3 new
// decision strings; the Go emit pattern is unchanged.

import "github.com/skillforge/skillforge/internal/wire"

payload := wire.DecisionPayload{
    Step:     0, // pre-hop
    Decision: "history_query_rewritten",
    Query:    rewrittenQuery, // FE renders "已理解为：XX" caption
}
data, _ := json.Marshal(payload)
err := wire.EmitChunk(c, wire.UIMessageChunk{
    Type: "data-decision",
    Data: data,
})
```

### Common Operation 5: rewriter LLM call (lightweight, structured)

```go
// Source: eino-ext/openai v0.1.13 ChatModel.Generate + Phase 4 system-prompt
// patterns. Custom prompts for D10-C1 (rewriter) and D10-A1 (classifier).

sysPrompt := rewriterSystemTemplate
userPrompt := fmt.Sprintf(rewriterUserTemplate,
    historyAsText(historyMsgs), // newlines between turns
    query,
)

resp, err := cm.Generate(ctx,
    []*schema.Message{
        schema.SystemMessage(sysPrompt),
        schema.UserMessage(userPrompt),
    },
    model.WithTemperature(0.0),
    model.WithMaxTokens(80), // small rewrite + 10 tokens slack
    openai.WithExtraFields(map[string]any{
        "response_format": map[string]string{"type": "json_object"},
        "thinking":        map[string]string{"type": "disabled"},
    }),
)
// extract JSON, parse {"rewritten_query": "..."}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single-turn chat (Phase 2/3) | Multi-turn messages prefix + sanitized history | Phase 10 (D10-B2) | GLM follows native role structure better; anaphora resolvable |
| 3-turn hard window | Token-budget-driven window with `safeguardFloor=2` | Phase 10 (D10-B3) | Long sessions (8/12/15 turns) now functional; calibration deferred to end-of-phase spike |
| Refusal-by-construction (code-level SC-2/GEVL-01) | + Query-only LLM refusal classifier upstream of history-enriched generation | Phase 10 (D10-A1) | Semantic refusal capability preserved; history drift structurally impossible |
| Refusal rounds in history = literal refusal text | Neutral placeholder template (fixed string, zero refusal-semantic words) | Phase 10 (D10-A2) | -95pp refusal drift structurally prevented (P9 spike evidence) |
| Original query direct retrieval for follow-ups | Query rewrite front-loaded | Phase 10 (D10-C1) | Anaphora resolved before retrieval; rewrites transparent to user via data-decision hint |
| In-memory session state on FE | Refresh-restore via GET /api/session/:id/history + useChat initialMessages | Phase 10 (D10-D1) | Page refresh preserves conversation; visual parity with live messages |
| Current Skill persisted (always) | Auto-switch to session-bound Skill on history restore | Phase 10 (D10-D2) | Cross-corpus misalignment prevented |
| Manual chunk-ref scrub (D9-B2) | + VerdictJSON renderer + neutral placeholder | Phase 10 (D10-A2, D10 verifier extension) | Audit-row raw storage preserved; projection layer responsible for all display rendering |
| `data-decision` for retrieve/answer/refuse/counter_evidence/route | + 3 new decision strings (history_projected, history_query_rewritten, history_budget_truncated) | Phase 10 (D10-B4) | D15 invariant: zero new wire types; existing FE dispatcher routes on Decision string (additive only) |
| Summarization middleware (advised, never used) | Deferred to Phase 11 post-Runner migration | Phase 10 (deferred) | RF-6 uncalibrated token counter; Phase 10 ships dumb char budget |

**Deprecated/outdated:**
- **`agent-llm-query-rewrite` LLM-rewrites-the-query** — Phase 2 LLM drift caused 「年假」→ 「出差住宿费」rewrite (chat.go L209 comment). Phase 10 rewriter is structural (always runs on follow-up + adversarial-failure fallback) — but it never enters the evidence chain (D10-C2 invariant).
- **3-turn hard window** — bumped to budget-driven window; the 3-turn option becomes a fallback for `budget=0` (legacy compatibility).
- **`prompt=system+user` collapsed** — Phase 2 `singlePromptTemplate` was retired in Plan 260815-grw. Phase 10 keeps the new {System, User} two-message shape and only adds the multi-turn prefix.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Char-approximation budget (CN 1 char ≈ 1 token; EN 1 char ≈ 0.25 token) over-truncates rather than overflows | Section 5 (budget_window.go) | If GLM's actual char→token ratio is closer to 2 (Chinese 1 char → 2 tokens), `defaultCharBudget=8000` overflows 500 on long history. Mitigation: end-of-phase spike re-gate; tighten `defaultCharBudget` if needed |
| A2 | GLM Coding Plan rejects single-System-only payloads but accepts {System, User} | Section 4, Code Example 1 | If v0.9.14 + eino-ext openai v0.1.13 changes this, stream call fails at Stream() open. Verified by Phase 8 Plan 260815-grw RED→GREEN; high confidence |
| A3 | History-projected messages from `internal/history.Project` are byte-identical to Phase 9 contract (D9-B1/B2/B3 unchanged) | Section 4 (architecture) | If the projection behavior changes silently, 7 Phase 9 fixtures must continue to pass — additive subpackage extensions only |
| A4 | The end-of-phase spike re-run reproduces Phase 9 numbers within jitter tolerance (1e-3 layered tolerance) when Phase 10 implements baseline behavior (no classifier / no rewrite / no history) | Section 11 (Validation Architecture) | If baseline regressions occur independent of Phase 10, the spike gate measures TWO changes at once; mitigation: W1 + W2 wave-gating with hermetic tests first, live spike last |
| A5 | The classifier LLM's structurally-isolated input (`Query`, `Chunks` only) makes accidental history ingress impossible at compile time | Section 4 (Pattern 2) | If a code reviewer adds a `History` field despite the godoc, the CI grep gate catches it. Mitigated by `var _` reference to a sentinel that fails compilation if internal/history is imported |
| A6 | GLM's response_format=json_object + temperature=0 with thinking:disabled produces parseable JSON ≥ 95% of the time | Section 4 (refusal_classifier.go) | If parse success drops below 95%, the 3-retry loop exponentially aborts to default-deny; classifier latency +700ms but never "answer when uncertain" — strict default-refuse |
| A7 | GLM follows native multi-turn role structure materially better than text concatenation (Phase 9 spike injection shape approximates text concatenation → -95pp refusal drift) | Section 4 (Pattern 1) | If GLM treats concatenated history identically to multi-turn messages, D10-B2's premise weakens; ablation test in spike runner can compare message-prefix vs. text-concatenation paths |

**If this table were empty:** all claims would be verified via Context7 or official docs — but the open unknowns (tokenizer precision, parse-rate distribution, latency envelopes) require the end-of-phase spike to confirm.

## Open Questions

1. **Exact GLM-5.2 char→token ratio for mixed CN/EN content**
   - What we know: BGE-M3 tokenizer average ~0.7-0.9 chars/token on Chinese text; English ~0.25 chars/token; mixed varies widely
   - What's unclear: whether the end-of-phase spike's per-row usage logs will reveal a non-uniform distribution that breaks the constant
   - Recommendation: end-of-phase spike MUST collect `usage.completion_tokens` per row; if it diverges from the budget by > 20%, tighten `defaultCharBudget` and re-run
   - **(DEFERRED to spike calibration loop, D10-B3):** per-row `usage.completion_tokens` is recorded by the Phase 10 re-run; `DefaultCharBudget` is tightened only if divergence > 20% -- constants ship conservative in Plan 10-01 Task 2 (`DefaultCharBudget=8000`, `FallbackMessageCeiling=6`). The end-of-phase spike (Plan 04) is the empirical calibration gate.

2. **Query rewrite parse failure rate on adversarial history**
   - What we know: structured-output parse success is ≥ 95% on benign inputs (Phase 8 spike 003); adversarial history may trip the LLM into non-JSON output
   - What's unclear: exact rate of parse failure on the 10 hist-adversarial + 15 hist-followup rows
   - Recommendation: 3-retry default-deny in refusal_classifier; rewriter has 1 retry + best-effort fallback (no block); spike gates the behavior
   - **(RESOLVED by design):** rewriter is best-effort with fallback to the original query (D10-C1 -- `TestRewrite_TimeoutFallback` + `TestRewrite_MalformedJSON` lock it); classifier is 3-retry default-deny (D10-A1 + W3 `ClassifierConfidenceFloor=0.5` -- `TestClassify_DefaultDenyOnParseFailure` + `TestClassify_DefaultDenyOnLowConfidence` lock it). Empirical parse-failure rates are recorded by the Phase 10 `--via-pipeline` re-run; no pre-implementation action remains.

3. **`ENABLE_HISTORY_INJECTION` env flag value when Phase 11 lands**
   - What we know: rollback path per AI-SPEC §7 — flip flag false → Phase 9 single-turn behavior; cost returns to Phase 9 baseline
   - What's unclear: whether Phase 11 needs the flag's runtime toggle or just code-edit (probably the flag is removed in Phase 11 since history becomes mandatory)
   - Recommendation: ship the flag as optional (default `true`); Phase 11 deletes it post-Runner migration
   - **(RESOLVED):** no env flag ships in Phase 10. Rollback = nil-gating at the Deps assembly in `cmd/server/main.go` (Plan 10-02 Task 4: `DISABLE_HISTORY_PIPELINE` sentinel + nil Rewriter/RefusalClassifier disables rewrite/classify only -- the history prefix still flows, per B2 fix + `TestVerdictPipeline_NilRewriterStillFlowsHistory`). Phase 11 Runner migration revisits whether a flag is still needed.

4. **FE MessageBubble dispatcher ordering when `history_query_rewritten` arrives during streaming**
   - What we know: SSE arrival order is wired; the caption must render above the message bubble
   - What's unclear: whether useChat's setMessages() lifecycle injects the caption at the right timestamp
   - Recommendation: vitest case to assert arrival order (history_query_rewritten before text-delta) renders correctly; FE-04 plan revisits if assertion fails
   - **(RESOLVED):** Plan 10-03 Task 2 renders `HistoryRewriteHint` above the bubble via the message-parts renderer; `TestApp_HistoryRewriteHint` (vitest, Plan 03 Task 2 acceptance) asserts placement. Revisit only if that vitest case fails during execution.

5. **What rewriter should do when query has no anaphoric reference (e.g. "yes, ok, let's continue")**
   - What we know: rewriter runs on every follow-up; benign inputs are passed through verbatim
   - What's unclear: edge case where rewriter drops a sentence like "no" to nothing
   - Recommendation: rewriter prompt includes "return the query unchanged if no anaphoric reference exists"; spike asserts verbatim pass-through on a synthetic 5-row set
   - **(RESOLVED):** `rewriterSystemTemplate` mandates verbatim return when no anaphoric reference exists; `TestRewrite_PassThroughOnNoAnaphora` locks the behavior hermetically in Plan 02 Task 1.

## Environment Availability

> Skipped — phase has no NEW external dependencies (all packages already in go.mod).

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| eino v0.9.14 | VerdictPipeline extensions | ✓ (locked go.mod) | v0.9.14 | — |
| eino-ext openai v0.1.13 | Classifier + rewriter LLM calls | ✓ (locked go.mod) | v0.1.13 | — |
| hertz v0.10.6 | SSE + history REST endpoint | ✓ | v0.10.6 | — |
| modernc.org/sqlite v1.56.0 | session_turn + new audit_meta | ✓ | v1.56.0 | — |
| milvus-sdk-go/v2 v2.4.2 | Retrieval unchanged | ✓ | v2.4.2 | — |
| GLM-5.2 (Volcano Ark Coding Plan) | Classifier + rewriter + verdict LLM | ✓ (env `GLM_API_KEY`) | glm-5.2 | — |
| React 19 + Vite + Vercel AI SDK v5 | FE refresh-restore | ✓ | latest stable | — |
| Vitest | HistoryRewriteHint.test.tsx + transport-swap extension | ✓ | latest | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None.

## Validation Architecture

> workflow.nyquist_validation absent from `.planning/config.json` (default = enabled). Required for this phase.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | **Go stdlib `testing` + `testify/assert` + `testify/require`** for BE; **Vitest + @testing-library/react** for FE |
| Config file | `go.mod` (testify v1.11.1) + `web/vitest.config.ts` (Vitest) |
| Quick run command (BE, hermetic) | `go test ./internal/history/... ./internal/server/... ./internal/eval/... -count=1 -timeout 30s` |
| Full suite command (BE, hermetic) | `go test ./... -count=1 -timeout 120s -race` |
| Live gate command (end-of-phase) | `go run ./cmd/spike-history --category=adversarial,followup --history-length=long --summarize --out=reports/ --seed=42 --cost-limit=5.00` (exits 0 GO / 1 NO-GO) |
| FE quick command | `cd web && npm test` (Vitest) |
| FE type-check | `cd web && npx tsc --noEmit` (D15 drift alarm preserved) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| **MEM-01** | Anaphora resolution via query rewrite | unit (hermetic, rewriter_test.go stubbed) + live (spike runner hist-followup anaphora subclass) | `go test ./internal/server/ -run TestRewriter_AnaphoraRewriteReturnsSelfContainedQuery -v` | Wave 2 |
| **MEM-01** | Re-translation at FE | vitest (FE) | `cd web && npm test -- HistoryRewriteHint.test.tsx` | Wave 3 |
| **MEM-02** | History projection + post-refresh restore | unit + live | `go test ./internal/history/... ./internal/server/ -run TestHistoryAPI_RestoresFromSessionTurns -v` | Wave 1 + 3 |
| **MEM-02** | SC-9 isolation regression | unit (hermetic, 50×5) | `go test ./internal/server/ -run TestSessionIsolation50x5 -v -race -count=1` (re-run on new projection path) | existing; re-validated Wave 4 |
| **MEM-03** | Citation whitelist enforcement (unchanged from Phase 2/3) | unit + live | `go test ./internal/verdict/ -run TestGroundingChecker -v` + spike SC-3 zero-penetration | existing; re-validated Wave 4 |
| **MEM-04** | Projection carries no `[CHUNK id]` markers + verdictJSON deterministic render | unit (hermetic, projection_test.go + verdict_renderer_test.go) | `go test ./internal/history/... -run 'TestNoChunkMarkers|TestVerdictRendererIsDeterministic' -v` | Wave 1 |
| **MEM-04** | Neutral placeholder red test | unit (hermetic, neutral_placeholder_test.go) | `go test ./internal/history/... -run 'TestNeutralPlaceholderHasZeroRefusalSemantics|TestNeutralPlaceholderRedTestAgainstHistFollowup' -v` | Wave 1 |
| **MEM-05** | Budget window truncation | unit (hermetic) | `go test ./internal/history/... -run 'TestTruncateToCharBudget|TestSafeguardFloor|TestCharBudgetEnvelope' -v` | Wave 1 |
| **MEM-05** | Spans observation via `history_budget_truncated` data-decision | unit (hermetic, server test asserts SSE emit) | `go test ./internal/server/ -run TestEmitHistoryBudgetTruncatedChunk -v` | Wave 2 |
| **D10-A1** | Classifier physically isolates history (CI grep gate) | CI grep gate | `! grep -rn 'historyMsgs\|GetSession\|loadHistory\|Project(' internal/server/refusal_classifier.go` (MUST return empty) | Wave 2 |
| **D10-A1** | Classifier default-deny on parse failure | unit | `go test ./internal/server/ -run TestRefusalClassifier_DefaultDenyOnParseFailure -v` | Wave 2 |
| **D10-A2** | Neutral placeholder is regex-checked for refusal-semantic words | CI grep gate | `! grep -E '拒绝\|无权\|无法\|不能\|未授权\|不在\|抱歉' internal/history/neutral_placeholder.go` (MUST return empty except code comments) | Wave 1 |
| **D10-B3** | Budget window emits `history_budget_truncated` event | unit | see MEM-05 | Wave 1 + 2 |
| **D10-C3** | Rewrite emits `history_query_rewritten` data-decision | unit | `go test ./internal/server/ -run TestEmitHistoryQueryRewrittenChunk -v` | Wave 2 |
| **D10-D1** | FE useChat initialMessages integration | vitest | `cd web && npm test -- App.test.tsx` | Wave 3 |
| **D10-D2** | SkillPicker auto-switch on history restore | vitest | `cd web && npm test -- SkillPicker.test.tsx` | Wave 3 |
| **D10-A3** | End-of-phase 35-case spike re-run | live | `cmd/spike-history --category=adversarial,followup --history-length=long --summarize` | Wave 4 |
| **D10-A4** | Frozen lock byte-identical at HEAD | CI | `test "$(sha256sum eval/frozen/compliance.lock.json | cut -d' ' -f1)" = "50bb1688..." && echo OK` | Wave 4 |
| **Audit row extension** | `audit_meta` sibling table INSERT-only | unit | `go test ./internal/store/sqlite/ -run TestAuditMetaSiblingTable -v` | Wave 2 |
| **T-3-AUDIT INSERT-only** | Audit table no UPDATE/DELETE | unit (regression) | `go test ./internal/store/sqlite/ -run TestNoAuditMutation -v` | existing |
| **D9-V2 (escape plan)** | Classifier input struct has no History field | compile-time + grep | (same as D10-A1) | Wave 2 |

### Sampling Rate

- **Per task commit**: `go test ./internal/history/... ./internal/server/... -count=1 -timeout 30s` (hermetic) + `cd web && npm test` (Vitest) + D15 type-check
- **Per wave merge**: `go test ./... -count=1 -timeout 120s -race` (full BE hermetic suite) + `cd web && npx tsc --noEmit` + Vitest 270+/270+
- **Phase gate (W4)**: full live 35-case spike re-run on real GLM-5.2 + Milvus + BGE-M3 (~$0.05 + ~75min); frozen lock byte-identical check

### Wave 0 Gaps

- [ ] `internal/history/projection_test.go` — drives 7 existing fixtures + new (refusal-round → neutral placeholder, verdict-renderer cases, budget-window cases)
- [ ] `internal/history/verdict_renderer_test.go` — verdictJSON → conclusionText / refusalReason → neutral placeholder deterministic
- [ ] `internal/history/neutral_placeholder_test.go` — T-9-FILLER-LEAK red test: 10 hist-adversarial + 5 post_refusal cases
- [ ] `internal/history/budget_window_test.go` — char budget envelope + safeguardFloor + oldest-first
- [ ] `internal/server/rewriter_test.go` — happy path anaphora + timeout fallback + malformed JSON
- [ ] `internal/server/refusal_classifier_test.go` — red test malicious history + parse retry + default-deny
- [ ] `internal/server/history_decisions_test.go` — emit history_projected/budget_truncated/query_rewritten chunks
- [ ] `internal/server/history_api_test.go` — GET /api/session/:id/history SC-9 isolation + role-filtering
- [ ] `web/src/components/HistoryRewriteHint.test.tsx` — vitest; renders "已理解为：XX" caption from data-decision event
- [ ] `web/src/App.test.tsx` extension — vitest; useChat initialMessages restoration from history API

*(If no gaps listed: existing test infrastructure covers all phase requirements.)* — Wave 0 will produce ~10 new test files and modifications to existing test scaffolding.

## Security Domain

> Required when `security_enforcement` is enabled (default = enabled per workflow). This phase modifies a high-risk trust boundary; threats must be enumerated per PLAN.md threat_model requirement.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V1 Architecture | **yes** | this section is the implementation — STRIDE-lite table below |
| V2 Authentication | no | SkillForge uses sessionID (already cryptographically strong); no auth change this phase |
| V3 Session Management | **yes** | session_turn cross-session isolation (T-3-SQLI + SC-9); `var _=sql.ErrConnDone` panic guard on closed store; refresh-time sessionID continuity |
| V4 Access Control | partial | multi-session isolation via `WHERE session_id=?` (D18) — code-forced already |
| V5 Input Validation | **yes** | UTF-8 validate (chat.go L150 existing); user text scrubChunkRefs (D9-B2); rewritten query max_tokens cap; classifier input schema validation; chunks rendered as [REF-N] DATA (T-2-21 framing) |
| V6 Cryptography | no | no new cryptography this phase |
| V7 Error Handling | **yes** | T-3-AUDIT-WRITE swallow pattern (writes log + continues); classifier 3-retry default-deny; rewriter timeout best-effort fallback; audit_meta sibling table for new fields (log-only) |
| V8 Data Protection | partial | session_turn + history projection contains no PII tier data (D9-B1 role filter); PII-tier text never appears in compliance verdict |

### STRIDE-lite Threat Model (per PLAN.md threat_model requirement)

| Threat | STRIDE | Standard Mitigation | Phase 10 Specific |
|--------|--------|---------------------|-------------------|
| **T-10-01**: Refusal-context drift via history | Tampering + Information Disclosure | D10-A1 query-only refusal classifier (physical isolation at the type level) + D10-A2 neutral placeholder (red-tested for refusal-semantic word leakage) | worst-case = refusal drift -95pp (P9 spike evidence); classifier output MUST default-deny on parse failure (no "answer when uncertain" behavior) |
| **T-10-02**: Adversarial history text becomes evidence (fake clause paste) | Elevation of Privilege + Spoofing | D10-B2 multi-turn messages prefix keeps history SEPARATE from user prompt's DATA section; `verdict.GroundingChecker` enforces citation chunkID ⊆ this-Run retrieval (D2-01 / D2-04 already) | spike gate SC-3 zero-penetration (0/10 hist-adversarial rows) hard-pass |
| **T-10-03**: Cross-session history contamination | Information Disclosure | session_turn WHERE clause parameterized (T-3-SQLI); `GetSession(ctx, sessionID)` signature unchanged from Phase 3; SC-9 isolation test re-run on new projection path (-race) | `TestSessionIsolation50x5` is the empirical gate |
| **T-10-04**: Cost amplification (per-call spike in rewrite/classifier) | Denial of Service | rewriter has 2s timeout (worst case); classifier 3-retry default-deny; char-budget window caps history tokens; spike runner `--cost-limit=$5.00` per segment | estimate Phase 10 per-query cost = $0.00585 (follow-up) / $0.00525 (first-turn) vs Phase 9 $0.00137 — net 3× (~$0.05 for 35-case spike + 100× headroom) |
| **T-10-05**: Audit row integrity (T-3-AUDIT violation via new fields) | Repudiation | New fields go to sibling `audit_meta` table (INSERT + SELECT only — no UPDATE/DELETE on either audit or audit_meta); JOIN via `query_id`; existing audit row schema UNCHANGED; INSERT-only invariant held across both tables | go test TestAuditMetaSiblingTable + TestNoAuditMutation regression |
| **T-10-06**: Rewrite output trusts user text (rewriter inadvertently echoes fake clauses in rewrite hint) | Information Disclosure | D10-C2: rewrite output NOT in evidence chain; classifier sees the rewritten query + (empty) chunks → defaults to refusal; FE `HistoryRewriteHint` SUPPRESSED on refusal path | spike asserts 0/10 hist-adversarial rows show rewrite hint with fake_chunk_ref in the displayed text |
| **T-10-07**: Tokenizer approach overflows GLM context (500 error under budget miscalibration) | Denial of Service | Char-approximation budget CONSERVATIVE (over-truncate); safeguardFloor=2 prevents collapse to empty; end-of-phase spike calibrates real `usage` logs vs char estimate | if spike reveals overflow, tighten `defaultCharBudget` and re-run |
| **T-10-08**: data-decision event spoofing (FE renders forged "history_query_rewritten" without server-side proof) | Spoofing | data-decision dispatch is BE-emitted only; Hertz SSE handler is the producer; FE renderer does not synthesize; audit_meta row carries the rewritten_query for replay | server-side audit row carries the rewritten query verbatim; FE rendering bound to server emission |

### Audit Row Schema Invariant

**T-3-AUDIT** (Project section 0 invariant) — INSERT-only on the audit table; the Phase 10 `audit_meta` sibling table follows the same invariant. New fields are additive:

- `rewritten_query` (optional, string)
- `classifier_decision` (string: "allow" | "refuse")
- `classifier_reason` (optional, string)
- `classifier_confidence` (optional, float)
- `history_projected_n_turns` (int)
- `history_budget_truncated` (bool)
- `history_dropped_count` (optional, int)
- `sanitized_refusal_rounds` (optional, int)

Reviewed via JOIN on `query_id`. Schema for both tables UNCHANGED from Phase 3 (audit) + new (audit_meta).

## Sources

### Primary (HIGH confidence — verified 2026-08-18)

- **Eino v0.9.14 module cache**:
  - `components/model/interface.go` — `BaseChatModel`, `Generate`, `Stream`, `WithMaxTokens`, `WithTemperature` APIs; `ChatModel` deprecated with data-race comment
  - `schema/message.go` — `schema.Message` struct, `Role`, `Content`, helpers `SystemMessage/UserMessage/AssistantMessage`
  - `components/model/openai/chatmodel.go` — `openai.WithExtraFields` for `response_format` + `thinking`; OpenAI-compatible adapter used for GLM-5.2 via Volcano Ark
  - `adk/middlewares/summarization/` — TriggerCondition{ContextTokens, ContextMessages} (Phase 11 deferred)
- **Eino Context7** `/websites/cloudwego_io_eino`:
  - `_autodocs/02-schema-types.md` — `RoleType` enum (User/Assistant/System/Tool), `MessagesTemplate` interface
  - `_autodocs/03-components-api.md` — `Generate`/`Stream` signatures
  - `_autodocs/04-adk-runner.md` — `Runner.Run(ctx, []*schema.Message) *AsyncIterator` (Phase 11 territory)
  - `_autodocs/05-adk-agents.md` — `NewChatModelAgent` (Phase 11 territory)
  - `_autodocs/07-callbacks.md` — `Handler` + `OnStartWithStreamInput`/`OnEndWithStreamOutput` (Phase 11 territory; not needed this phase)
- **Phase 8 eino v0.9.14 lock evidence** (`08-01-SUMMARY.md`): hermetic regression 4 doors all green (`go build/vet`, full `go test ./...`, contract `-tags=contract` with VersionPin + RunsUnder10s + WireFormat + MilvusFilter + StructuredOutputSchema + CitationGrounding all PASS, `TestCancelPropagationMidTool` 50.6ms < 200ms).
- **Phase 8 live 3-gate verification** (`08-05-SUMMARY.md`): real GLM cancel 0ms; Milvus tier-filter 5行 + 0行 (zero leak); frozen eval gate PASS (faithfulness 0.972 / refusal_accuracy 95% / lock sha256 50bb1688 unchanged).
- **Phase 9 spike evidence** (`09-06-SUMMARY.md` + `reports/summary.{md,json}` git-ignored but reproduced here):
  - 35-case live run, real GLM-5.2 + Milvus + BGE-M3, seed=42, total cost $0.0481
  - decision=NO-GO (exit 1): refusal -95pp (95%→0%); faithfulness -18.6%/-12.6%/-22.4%; context_recall -87.2%/-63.0%/-70.1%; SC-3 penetration = 0/10 (PASS)
  - Failure attribution = Pitfall 1 H2 confirmed (refusal semantics context drift)
- **Phase 9 projection contract** (`.planning/phases/09-.../09-02-SUMMARY.md` + `internal/history/projection.go` + `projection_test_fixtures.go`): D9-B1/B2/B3 + 7 exported fixtures, locked for Phase 10
- **PROJECT.md §Core Value + Key Decisions**: citation whitelist = this-Run retrieval (code-forced); Go-native eval (D6 reversed); 4 metrics; frozen lock baseline
- **ROADMAP.md Phase 10 SC 1-5** (verbatim): anaphora resolution; refresh-restore; SC-3 zero-penetration; projection sanitization; long-session budget/compression observable
- **`.planning/research/PITFALLS.md` §Pitfall 1, §Pitfall 2, §Pitfall 7**: H1-H3 hypotheses; tier penetration; summarization token counter
- **`.planning/seeds/spike-history-injection-trust-floor.md`**: H1/H2/H3 hypotheses and through-criterion
- **CONTEXT.md D10-A1..A4 / B1..B4 / C1..C3 / D1..D2**: 13 locked implementation decisions

### Secondary (MEDIUM-HIGH confidence)

- **Eino Context7 `_autodocs/09-patterns-examples.md`**: ChatModelAgent + retry/failover patterns (referenced for Phase 11)
- **OpenAI Chat Completions API docs (eino-ext/model/openai)**: `response_format: json_object` + `thinking: type disabled` — spike 003 proven shape
- **GLM-5.2 Coding Plan endpoint behavior (spike 003, Phase 8)**: rejects single-System-only; accepts {System, User}; produces parseable JSON ≥ 95%
- **Phase 9 Plan 09-02 D9-B4 contract handoff**: 7 fixtures are exported (not _test.go) for Phase 10 consumption

### Tertiary (LOW confidence — flagged for validation)

- **GLM-5.2 char→token ratio**: 1 char ≈ 1 token (Chinese), 0.25 token (English); calibrated at end-of-phase spike via per-row usage logs. If overflow 500 occurs, tighten `defaultCharBudget` and re-run.
- **GLM native multi-turn role-following advantage over text concatenation**: deduced from P9 spike showing text-concat drift -95pp; net of Phase 10 design. If ablation shows message prefix is no better, Phase 10 task is wasted but not safety-critical (D9-B sanitizer still holds; classifier still isolates history).
- **Vercel AI SDK v5 UIMessageChunk discriminator on Decision string**: Phase 5-04 already extended for `route` + `counter_evidence`; Phase 10 adds 3 more decision strings in the same discriminator (TS side additive). If Vercel upstream renames or restricts discriminator, D15 compile-time alarm fires.

## Metadata

**Confidence breakdown:**

| Area | Level | Reason |
|------|-------|--------|
| Eino v0.9.14 + eino-ext openai v0.1.13 APIs | HIGH | Phase 8 lock evidence + Context7 verification + hermetic + live 3-gate |
| GLM-5.2 structured-output behavior | HIGH | Phase 8 spike 003 + 09-06 live re-run |
| D9-B1/B2/B3 projection contract | HIGH | Phase 9 shipped + fixtures exported; Phase 10 is additive only |
| Multi-turn messages prefix shape | HIGH | GLM native role-following per Context7; spike 003 + structured-output proven; ablation not strictly needed for safety (D9-B sanitizer holds either way) |
| Char-approximation budget constants | MEDIUM-HIGH | RF-6 limits verification options; calibrated at end-of-phase spike; conservative defaults |
| Query-only classifier isolation (compile-time + grep) | HIGH | Type-level design + grep guard; proven pattern from D11 interface-first + D18 store abstraction |
| Neutral placeholder wording | MEDIUM-HIGH | D9-L2 filler-style proven + red-tested; exact wording per implementation discretion (Claude's discretion per CONTEXT.md) |
| `data-decision` event extension (3 new Decision strings) | HIGH | D5-04 + D5-13 precedent (counter_evidence); additive only; D15 two-side mirror PR in same commit |
| Refresh-restore via useChat initialMessages | MEDIUM-HIGH | Vercel AI SDK v5 `initialMessages` API confirmed via Context7; SkillPicker auto-switch hooks into existing Zustand (skill_id field already present) |
| End-of-phase spike re-gate semantics | HIGH | Phase 9 binary `cmd/spike-history` unchanged + 6-gate TestComputeVerdict scenarios; same gate-same-aperture guarantee Phase 9 established |
| Cost amplification | MEDIUM-HIGH | per-call estimates derived from Phase 9 actuals + heuristic +60% for rewriter/classifier; spike actual is the empirical calibration |

**Research date:** 2026-08-18
**Valid until:** 2026-09-18 (30 days for stable stack; revisit on Eino minor releases for any API drift)
