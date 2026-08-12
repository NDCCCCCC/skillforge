# SkillForge — Roadmap

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.
**Mode:** mvp (vertical slices, MVP-first delivery)
**Granularity:** standard
**Date:** 2026-08-10

---

## Phases

- [x] **Phase 1: Integration Spike & Trust Contract** — 1-2 day spike: validate Eino v0.7.7 + Hertz SSE + Vercel AI SDK v5 + Milvus tier-filter wire framing before committing to the data model
 (completed 2026-08-11)
- [x] **Phase 2: Compliance Vertical Slice** — one Skill / one authoritative tier / one endpoint / verifiable cited answer or structured refusal (proves the "Skill-as-platform" thesis) — 5/5 plans complete (incl. 02-04-MONGO extension)
- [x] **Phase 3: Agentic Retrieval & Trust Evaluation** — hybrid retrieval + BGE-M3 rerank + Self-RAG self-verification + 50–100-case golden set + Ragas quality gates
 (completed 2026-08-12)
- [x] **Phase 4: Streaming Trust UI** — React chat with visible reasoning chain + clickable citations + structured verdicts + explainable refusals — 3/3 waves complete (UAT 12/12, SECURITY verified / 0 open threats)
 (completed 2026-08-11)
- [x] **Phase 5: Skill Platform & Full Knowledge Boundaries** — semantic routing + hot reload + Skill Wizard + full 4-tier system + Agent state inspector + counter-evidence / redline (completed 2026-08-12)
- [ ] **Phase 6: Eval Dashboard & Production Hardening** — CI regression gates + tracing + metrics + queryable audits + session isolation + retention / redaction
- [ ] **Phase 7: Open-Source Release** — Docker Compose + sample docs + docs + live demo + showcase Skills + published eval results

---

## Phase Details

### Phase 1: Integration Spike & Trust Contract

**Goal:** Validate the core integration of Eino v0.7.7 + Hertz SSE + Vercel AI SDK v5 + Milvus tier filter before committing to the full data model and Skill schema.
**Mode:** mvp
**Depends on:** Nothing (first phase)
**Requirements:** None (spike — generates no v1 requirements)
**Success Criteria** (what must be TRUE):
  1. A minimal Go Hertz service streams a hardcoded agent response to a minimal React client using the agreed `UIMessageChunk` envelope (text / tool-invocation / source / reasoning parts) with no wire-format drift
  2. A Milvus tier-filter expression returns chunks ONLY when `tier == "权威"` and returns zero rows when queried for any other tier against a seeded 4-tier dataset
  3. An Eino `ChatModelAgent` + `SequentialAgent` two-step demo (one ChatModel call then one tool call) executes end-to-end and cancellation propagates from the React client to the agent run within 200ms
  4. A streaming "fake cited answer" demo reaches the UI with hardcoded chunk citations rendered as clickable elements (eventually Phase 4's `FEUI-03`)
  5. A version-pinned integration test (`go test ./integration/contract`) runs the above in under 10s and fails on any wire-format or filter-expression drift
**Pitfalls addressed:** Version pinning (P-EINO-1); stream contract drift (P-FE-1); upfront Eino API surface risk; Milvus filter syntax validation
**Plans**: 4 plans
**Plan list**:
- [x] 01-01-PLAN.md — Wave 0 scaffold + spike wire subcommand (SC-1 layer a) + Docker/API-key checkpoints
- [x] 01-02-PLAN.md — 12 fixtures + spike milvus subcommand + docker-compose (SC-2)
- [x] 01-03-PLAN.md — UIMessageChunk types + spike sse subcommand + React stub + vitest (SC-1 layers b+c, SC-4)
- [x] 01-04-PLAN.md — spike e2e subcommand + integration contract test (SC-3, SC-5)
**Wave structure**: W1={01-01} → W2={01-02, 01-03 parallel} → W3={01-04}
**UI hint**: no

### Phase 2: Compliance Vertical Slice

**Goal:** Prove the "Skill-as-platform" thesis by delivering one Skill (compliance), one authoritative tier, one endpoint, that yields either a verifiable cited answer or a structured refusal — never an improvised answer.
**Mode:** mvp
**Depends on:** Phase 1
**Requirements:** SYSK-01, SYSK-02, SYSK-07, SYSK-08, CMPL-01, CMPL-02, CMPL-03, CMPL-04, CMPL-05, AGNT-03, ARAG-04, ARAG-05, GEVL-01, GEVL-02
**Success Criteria** (what must be TRUE):
  1. Dropping a `SKILL.md` file into `skills/compliance/` registers it; `POST /api/skills` validates it against the 7-field schema (incl. `temperature` per D2-10 — ROADMAP originally said "6-field", corrected to 7-field) and lists it (SYSK-01, SYSK-02, SYSK-08)
  2. Asking a question with no 权威-tier chunk retrieved returns a structured refusal `{reason: "no_authorized_source", can_user_rephrase: true, suggested_alternative_query: "..."}` — never an answer (ARAG-05, GEVL-02)
  3. Asking a question WITH retrieved 权威 chunks returns a structured verdict `{status: 合规 | 不合规 | 需补充, confidence, citations:[{chunk_id, span, tier:"权威"}], review_required}` with every claim tied to a chunk ID + span, and the chunk supports the claim verbatim (CMPL-02, ARAG-04)
  4. Asking outside the Skill's declared `reads` (e.g., personal tier) produces zero tier-matched chunks and triggers refusal — no tier bleed, ever (SYSK-07, CMPL-04)
  5. A `review_required: true` verdict (confidence < 0.9 or novel claim) carries a verifiable event payload; the UI badge surfaces in Phase 4 (CMPL-03, CMPL-05)
  6. Every response carries an SSE `source` event with `tier="权威"` and chunk IDs streamed to the wire (AGNT-03, ARAG-04)
**Pitfalls addressed:** P-COMPL-1 (top risk: compliance hallucination with fake citations — every claim chunk-verified via D2-01 grounding), P-SKILL-1..6 (Skill spec design — 7-field validator), P-RAG-2 (tier bleed defense — PRE-similarity Milvus filter)
**Plans**: 4 plans
**Plan list**:
- [x] 02-01-PLAN.md — Wave 1: graduate spike → internal/{verdict,wire,retriever} + D2-06 Citation upgrade + D2-01 grounding moat + contract test (CMPL-02/03, ARAG-04) — completed 2026-08-11
- [x] 02-02-PLAN.md — Wave 1 (parallel): 7-field skill package + ResolveTiers + canonical skills/compliance/SKILL.md + knowledge/authority corpus (SYSK-01/02/08, CMPL-01/04) — completed 2026-08-11
- [x] 02-03-PLAN.md — Wave 2: knowledge indexer + verdict pipeline + POST /api/chat + cmd/server + SC-2/3/4/5/6 tests (AGNT-03, ARAG-05, GEVL-01/02, CMPL-05, SYSK-07)
- [x] 02-04-PLAN.md — Wave 3: GET/POST /api/skills endpoints (SC-1 HTTP layer) + live end-to-end verification of all 6 SCs against Milvus + GLM-5.2 (SYSK-01/02, CMPL-01 closure)
- [x] 02-04-MONGO-SUMMARY.md — Extension: MongoDB retriever backend (RETRIEVER_BACKEND=mongo). Adds internal/retriever/mongo implementing retriever.Retriever via {tier:{$in}} PRE-filter + in-Go cosine. Triggered by internal Milvus being broken; also proves the multi-backend abstraction. — completed 2026-08-11
**Wave structure**: W1={02-01, 02-02 parallel} → W2={02-03} → W3={02-04}
**UI hint**: no

### Phase 3: Agentic Retrieval & Trust Evaluation

**Goal:** Make the agent decide for itself (retrieve / refuse / ask / multi-hop) with Self-RAG self-verification, and stand up the Ragas-backed eval pipeline that becomes the project's measurement moat.
**Mode:** mvp
**Depends on:** Phase 2
**Requirements:** AGNT-01, AGNT-02, AGNT-04, AGNT-05, ARAG-01, ARAG-02, ARAG-03, KNOW-01, KNOW-02, KNOW-03, KNOW-04, GEVL-03, GEVL-04, GEVL-05
**Success Criteria** (what must be TRUE):
  1. The agent autonomously decides to retrieve / ask-clarifying / refuse / answer for the same query phrased three ways — observable via SSE stream events with `decision` payloads (ARAG-01)
  2. After a partial first-hop retrieval, the agent issues a second retrieval hop with a refined query and the final answer cites chunks from both hops (ARAG-02)
  3. Self-RAG self-verification re-reads retrieved docs and strips or refuses any claim not grounded in retrieved chunk text (ARAG-03, ARAG-04)
  4. Indexing a document without `tier` metadata into Milvus is rejected at the index layer — there is no path around it (KNOW-02)
  5. A query routed to a Skill whose `reads` list does not match any tier returns 0 chunks (default-deny), and the agent refuses rather than improvises (KNOW-04, GEVL-02)
  6. A 50-case Ragas golden set per Skill runs against the Python sidecar and emits faithfulness / context precision / answer relevance scores into a JSON report (GEVL-04, GEVL-05)
  7. Every query writes an audit row `{ts, user, query, skill, chunk_ids[], model_ver, prompt_hash, answer, refusal_reason}` queryable via SQL (GEVL-03)
  8. User can cancel an in-flight agent run mid-stream and the SSE connection closes cleanly (AGNT-04)
  9. Two concurrent sessions from the same user never see each other's conversation memory (AGNT-05)
**Pitfalls addressed:** P-RAG-1..4 (hybrid retrieval, tier bleed), P-AGENTIC-1,3,4 (autonomous decision, multi-hop, self-verify), P-EVAL-1..4 (Ragas pipeline, judge calibration), P-COMPL-1,3,6 (cited answer rigor, audit log)
**Plans**: 6 plans
**Plan list**:
- [x] 03-01-PLAN.md — Wave 1: data-decision wire discriminator (Go + TS in lockstep per D-04/D15) + TierEnumByValue/TierEnumValues reverse-lookup helpers (ARAG-01 substrate, KNOW-01)
- [x] 03-02-PLAN.md — Wave 1 (parallel): internal/store.Store interface + SQLite default impl + audit row + session memory + SQLi/isolation tests (GEVL-03, AGNT-05)
- [x] 03-03-PLAN.md — Wave 2: internal/agent — ReAct loop + retrieve tool + trust floor + layered critique + decision callbacks (AGNT-01/02, ARAG-01/02/03, KNOW-04)
- [x] 03-04-PLAN.md — Wave 2 (parallel): KNOW-02 tier-mandatory enforcement at internal/knowledge.Index (backend-agnostic)
- [x] 03-05-PLAN.md — Wave 3: server integration — chatHandler session_id + Agent.Run + audit/session writes + data-decision SSE + SC-8 cancel + SC-9 isolation (AGNT-04/05, ARAG-01, KNOW-03)
- [x] 03-06-PLAN.md — Wave 4: trust evaluator + 50-case golden set + calibration data retiring retrievalScoreThreshold=0.3 (GEVL-04/05). **D6 reversed during execution** — Python/Ragas sidecar eliminated in favor of Go-native evaluator (internal/eval + cmd/eval); faithfulness ports Ragas v0.1.21 prompts VERBATIM. See 03-06-SUMMARY.md.
**Wave structure**: W1={03-01, 03-02 parallel} → W2={03-03, 03-04 parallel} → W3={03-05} → W4={03-06}
**UI hint**: no

### Phase 4: Streaming Trust UI

**Goal:** Ship a React chat experience where every claim is auditable — visible reasoning chain, clickable citations with tier badges, structured verdicts, and distinguishably-explained refusals.
**Mode:** mvp
**Depends on:** Phase 3
**Requirements:** FEUI-01, FEUI-02, FEUI-03, FEUI-04, FEUI-05, FEUI-06, SYSK-03, KNOW-05
**Success Criteria** (what must be TRUE):
  1. Typing a question produces a streaming response with typed message parts (reasoning / tool-invocation / text / source) visible by default; reasoning is collapsible (FEUI-01, FEUI-02)
  2. Clicking a citation opens a side panel with the chunk verbatim text, tier badge (`[权威]` / `[历史]` / `[个人]` / `[草稿]`), version, and effective date (FEUI-03, KNOW-05)
  3. The skill picker lists installed Skills loaded from `/api/skills`; selecting a Skill scopes every subsequent query to that Skill (FEUI-04, SYSK-03)
  4. Refusal responses display "out-of-scope" vs "I don't know" distinctly with the structured `reason` and `suggested_alternative_query` surfaced in the UI (FEUI-05)
  5. While the agent works, a sub-progress indicator shows "Searching 权威 tier... Found 12 candidates... Re-ranking..." with step-level updates (FEUI-06)
  6. A compliance verdict with `review_required: true` shows a yellow "needs human review" badge over the message (CMPL-05)
**Pitfalls addressed:** P-FE-1..5 (UI integration, citation rendering, refusal UX), P-COMPL-3 (UI side-by-side), P-COMPL-5 (HITL badge visibility)
**Plans**: TBD
**UI hint**: yes

### Phase 5: Skill Platform & Full Knowledge Boundaries

**Goal:** Promote SkillForge from "one Skill" to a true Skill-driven platform — semantic routing across multiple Skills, hot reload of in-flight Skills, a Skill Wizard that generates a SKILL.md from natural language, the full 4-tier knowledge boundary system with conflict surfacing, and an Agent state inspector.
**Mode:** mvp
**Depends on:** Phase 4
**Requirements:** SYSK-04, SYSK-05, SYSK-06, AGNT-06, KNOW-06, CMPL-06, CMPL-07, CMPL-08
**Success Criteria** (what must be TRUE):
  1. With two Skills installed (compliance + general QA), an ambiguous query is routed to the right Skill purely by `description` semantic match — and the trace shows the score + selected Skill (SYSK-05)
  2. Editing an existing `SKILL.md` on disk triggers `fsnotify` reload; the next query after a 1s debounce window uses the new behavior without restart (SYSK-04)
  3. The Skill Wizard takes natural-language input (e.g., "审批流合规审计 Skill") and emits a validated `SKILL.md` that passes the same Phase 2 schema validator (SYSK-06)
  4. A live "Agent State Inspector" panel shows the current turn's tool calls, retrievals (with chunk IDs), and intermediate reasoning steps in real time (AGNT-06)
  5. The knowledge graph browser shows tier → doc → chunk relationships visually; clicking a chunk reveals its citations (KNOW-06)
  6. For a policy conflict between 权威 and 历史 tiers, the UI surfaces counter-evidence chunks next to the verdict (CMPL-06)
  7. The agent proposes a clause-level redline rewrite when its verdict flags a policy gap (CMPL-07)
  8. Querying chunks whose `effective_to < today` yields zero results (superseded policies are filtered out); the UI shows the active policy version (CMPL-08)
**Pitfalls addressed:** Full P-SKILL-* (semantic routing, hot reload, wizard generation), P-EINO-1..3 (multi-Skill orchestration, in-flight hot reload, cross-tier conflict policy), full tier system (P-RAG-1)
**Plans**: 6 plans
**Plan list**:
- [x] 05-01-PLAN.md - Wave 1: Wire extensions (data-redline 17->18 + DecisionPayload routing fields) + Skill Registry (atomic.Pointer + fsnotify + V12 symlink re-check + 1s debounce)
- [x] 05-02-PLAN.md - Wave 1: Skill Router (BGE-M3 cosine + ambiguity gate) + Chunk schema migration (Document.EffectiveFrom/To + Milvus + Mongo + indexer validateEffective + pipeline threads now)
- [x] 05-03-PLAN.md - Wave 2: SC-1 routing trace + ambiguity picker + SC-2 hot-reload UI (toasts + statusbar pinned-version) + Skill Manager menu + 5-tab CenterPanel wiring (inspector + graph placeholders)
- [x] 05-04-PLAN.md - Wave 2: SC-3 Skill Wizard (backend Generate + Commit endpoints with path-traversal guard; right-panel wizard mode + 5-step interview + preview + alert-dialog cancel)
- [x] 05-05-PLAN.md - Wave 2: SC-4 Agent State Inspector (Accordion tree from useChat.messages) + SC-5 Knowledge Graph Browser (GET /api/knowledge/graph + 3-level Accordion + search filter) + SC-8 version badges on ChunkSourceView
- [x] 05-06-PLAN.md - Wave 2: SC-6 Counter-evidence (BE post-verdict 历史 pass + FE teal annotation card with disclaimer) + SC-7 Redline (conditional LLM pass on 不合规/需补充 + FE diff view with copy/download)
**Wave structure**: W1={05-01, 05-02 sequential} -> W2={05-03, 05-04, 05-05, 05-06 parallel}
**UI hint**: yes

### Phase 6: Eval Dashboard & Production Hardening

**Goal:** Turn the eval pipeline into a CI gate, surface per-Skill metrics on a dashboard, and stand up the production-grade observability + privacy posture — audit retention, redaction, session isolation.
**Mode:** mvp
**Depends on:** Phase 5
**Requirements:** GEVL-06, GEVL-07
**Success Criteria** (what must be TRUE):
  1. A web dashboard surfaces per-Skill metrics — faithfulness, citation precision, refusal accuracy — with a trend line across runs (GEVL-06)
  2. Opening a pull request that drops a Skill's faithfulness below 0.9 fails the CI gate; the PR comment names the Skill and the threshold (GEVL-07)
  3. Audit records are queryable via a UI table filterable by user / skill / time range / `refusal_reason` with row-level redaction toggled by access policy
  4. Session isolation is proven by a concurrency test: 50 sessions × 5 turns each see only their own conversation history (no cross-session bleed)
  5. Retention / redaction policy is enforced: queries past the retention window have `query` and `answer` fields redacted in storage; access requires an explicit access policy
  6. Eval runs are reproducible: re-running against a frozen eval set + pinned `model_ver` + pinned `prompt_hash` returns numerically identical Ragas scores within tolerance
**Pitfalls addressed:** P-RAG-2 (monitoring + drift detection), P-COMPL-4 (audit trail integrity), P-EINO-2 (multi-tenant session isolation), P-OSS-5 (CI hygiene)
**Plans**: TBD
**UI hint**: yes

### Phase 7: Open-Source Release

**Goal:** Ship a polished, documented, demo-able open-source release that a stranger can fork, run locally, and benchmark — the artifact that earns the personal-brand.
**Mode:** mvp
**Depends on:** Phase 6
**Requirements:** DIST-01, DIST-02, DIST-03, DIST-04, DIST-05, DIST-06, DIST-07, DIST-08, DIST-09, DIST-10
**Success Criteria** (what must be TRUE):
  1. `git clone && docker compose up` brings up the full stack on a clean laptop — backend, frontend, Milvus, BGE-M3 (DIST-04)
  2. README renders badges, value prop, demo GIF, quick start, architecture diagram, contributing, and license at the repo root (DIST-01, DIST-02, DIST-07)
  3. `.env.example` lists every required variable with comments; missing values produce a clear startup error (DIST-03)
  4. Pre-bundled sample compliance docs ship under `examples/data/` and the demo works with no API key for retrieval (only LLM calls require one) (DIST-06)
  5. A live demo site on free-tier hosting responds to a sample compliance question and returns a cited verdict (DIST-05)
  6. `examples/` ships 2-3 Skill templates (compliance, simple QA, calculator) each passing the schema validator (DIST-08)
  7. README's "Eval Results" section shows published per-Skill faithfulness %, citation precision %, refusal accuracy % from the latest CI run (DIST-09)
  8. `eval/sample.jsonl` ships a 50-case sample of the per-Skill eval set in the repo (DIST-10)
**Pitfalls addressed:** P-OSS-1..6 (README, demo, Docker, contributing, license, eval results publication)
**Plans**: TBD
**UI hint**: yes

---

## Coverage

| Phase | Requirements | Count |
|-------|--------------|-------|
| 1 - Integration Spike & Trust Contract | (spike — none) | 0 |
| 2 - Compliance Vertical Slice | SYSK-01, SYSK-02, SYSK-07, SYSK-08; CMPL-01..05; AGNT-03; ARAG-04, ARAG-05; GEVL-01, GEVL-02 | 14 |
| 3 - Agentic Retrieval & Trust Evaluation | AGNT-01, AGNT-02, AGNT-04, AGNT-05; ARAG-01, ARAG-02, ARAG-03; KNOW-01..04; GEVL-03, GEVL-04, GEVL-05 | 14 |
| 4 - Streaming Trust UI | FEUI-01..06; SYSK-03; KNOW-05 | 8 |
| 5 - Skill Platform & Full Knowledge Boundaries | SYSK-04, SYSK-05, SYSK-06; AGNT-06; KNOW-06; CMPL-06, CMPL-07, CMPL-08 | 8 |
| 6 - Eval Dashboard & Production Hardening | GEVL-06, GEVL-07 | 2 |
| 7 - Open-Source Release | DIST-01..10 | 10 |
| **Total** | **55 of 55 v1 requirements mapped** | **55** |

**Unmapped:** 0 ✓
**Orphaned groups:** None.

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Integration Spike & Trust Contract | 4/4 | Complete | 2026-08-11 |
| 2. Compliance Vertical Slice | 5/5 (incl. 02-04-MONGO) | Complete | 2026-08-11 |
| 3. Agentic Retrieval & Trust Evaluation | 6/6 | Complete | 2026-08-12 |
| 4. Streaming Trust UI | 4/4 (incl. 04-04 gap-closure) | Complete | 2026-08-11 (04-04 debt-closure 2026-08-12) |
| 5. Skill Platform & Full Knowledge Boundaries | 0/6 | Not started | - |
| 6. Eval Dashboard & Production Hardening | 0/? | Not started | - |
| 7. Open-Source Release | 0/? | Not started | - |

---
*Last updated: 2026-08-12 — Phase 5 planned (6 plans: 2 in Wave 1 sequential; 4 in Wave 2 parallel). All 8 req IDs (SYSK-04/05/06, AGNT-06, KNOW-06, CMPL-06/07/08) covered; 15 locked D5-XX decisions implemented; 1 deferred item (DATA-*) explicitly out of scope per CONTEXT.md.*
