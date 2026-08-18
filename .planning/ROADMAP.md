# SkillForge — Roadmap

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.
**Mode:** mvp (vertical slices, MVP-first delivery)
**Granularity:** standard
**Date:** 2026-08-10

---

## Phases

### Milestone v0.7.7 — 可信问答内核 (phases 1-7)

- [x] **Phase 1: Integration Spike & Trust Contract** — 1-2 day spike: validate Eino v0.7.7 + Hertz SSE + Vercel AI SDK v5 + Milvus tier-filter wire framing before committing to the data model
 (completed 2026-08-11)

- [x] **Phase 2: Compliance Vertical Slice** — one Skill / one authoritative tier / one endpoint / verifiable cited answer or structured refusal (proves the "Skill-as-platform" thesis) — 5/5 plans complete (incl. 02-04-MONGO extension)
- [x] **Phase 3: Agentic Retrieval & Trust Evaluation** — hybrid retrieval + BGE-M3 rerank + Self-RAG self-verification + 50–100-case golden set + Ragas quality gates
 (completed 2026-08-12)

- [x] **Phase 4: Streaming Trust UI** — React chat with visible reasoning chain + clickable citations + structured verdicts + explainable refusals — 3/3 waves complete (UAT 12/12, SECURITY verified / 0 open threats)
 (completed 2026-08-11)

- [x] **Phase 5: Skill Platform & Full Knowledge Boundaries** — semantic routing + hot reload + Skill Wizard + full 4-tier system + Agent state inspector + counter-evidence / redline
 (completed 2026-08-12)

- [x] **Phase 6: Eval Dashboard & Production Hardening — Code-Side (6a)** — dashboard + gate logic + queryable audits + session isolation + retention / redaction (CI workflow wiring deferred → Phase 999.2)
 (code-side complete 2026-08-13; split decision 2026-08-14)

- [ ] **Phase 7: Open-Source Release** — Docker Compose + sample docs + docs + live demo + showcase Skills + published eval results

### Milestone v0.8 — Harness（可信×行动融合）(phases 8-13)

贯穿性不变量：**citation 白名单 = 本轮检索结果，代码强制** —— 历史/摘要/子 agent 输出/resume 注入的文本永远不能成为证据。每个 phase 以「可信税」产物收尾：spike 报告 → 消毒规则 → 3 行审计生命周期 → 可回放规划 → tier 继承证明测试。依赖为严格链：8 → 9 → 10 → 11 → 12 → 13。

- [x] **Phase 8: 全量依赖升级（P8.0）** — eino v0.7.7 → v0.9.14 + lockstep eino-ext 联动升级 + sqlite CheckPointStore 骨架；回归套件全绿，legacy interrupt API lint 禁用 (completed 2026-08-18 — live 三门 human-verify approved；PLAT-04/PLAT-05 勾选)
- [ ] **Phase 9: 历史注入 trust-floor spike 硬门（P8.S）** — golden-set A/B 证明历史投影不漂移拒答/引用行为（四指标漂移 <5%、refusal delta <10%）；不过门则记忆不得发布
- [ ] **Phase 10: 多轮对话记忆（P8.1）** — sqlite 历史投影喂 agent + 投影消毒 + citation 白名单代码强制 + token 预算/摘要压缩
- [ ] **Phase 11: 写工具 + 审批门（P8.2）** — Eino Interrupt/Resume HITL + 流内审批卡 + 跨刷新审批态 + 3 行 INSERT-only 审计生命周期 + `writes:` 代码级强制
- [ ] **Phase 12: 规划层（P8.3）** — DeepAgent（write_todos）+ 计划确认门 + 三层预算门 + 审计可回放的规划轨迹
- [ ] **Phase 13: 子 agent 扇出（P8.4）** — 一层并行多跳扇出 + tier 构造期闭包继承 + 确定性/保守合并 + 子 agent 独立 grounding

---

## Phase Details — Milestone v0.7.7

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
**Plans**: 4 plans (3 + 1 gap-closure; 04-01 was folded into 04-02 execution — PLAN file superseded by SUMMARY)
**Plan list**:

- [x] 04-01-PLAN.md — Wave 1: Foundation — wire types + Mock Transport + Parts Renderer
- [x] 04-02-PLAN.md — Wave 2: VerdictCard + RefusalCard + ReasoningPart + CenterPanel + locked-file reconciliation
- [x] 04-03-PLAN.md — Wave 3: Three-Panel + SkillPicker + ChunkSourceView wire + transport-swap integration test
- [x] 04-04-PLAN.md — Wave 4: gap-closure — AI SDK v7 drift alarm (D15) + tsc clean

**Wave structure**: W1={04-01} → W2={04-02} → W3={04-03} → W4={04-04 gap-closure}
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
**Plans**: 7 plans (6 + 1 gap-closure)
**Plan list**:

- [x] 05-01-PLAN.md - Wave 1: Wire extensions (data-redline 17->18 + DecisionPayload routing fields) + Skill Registry (atomic.Pointer + fsnotify + V12 symlink re-check + 1s debounce)
- [x] 05-02-PLAN.md - Wave 1: Skill Router (BGE-M3 cosine + ambiguity gate) + Chunk schema migration (Document.EffectiveFrom/To + Milvus + Mongo + indexer validateEffective + pipeline threads now)
- [x] 05-03-PLAN.md - Wave 2: SC-1 routing trace + ambiguity picker + SC-2 hot-reload UI (toasts + statusbar pinned-version) + Skill Manager menu + 5-tab CenterPanel wiring (inspector + graph placeholders)
- [x] 05-04-PLAN.md - Wave 2: SC-3 Skill Wizard (backend Generate + Commit endpoints with path-traversal guard; right-panel wizard mode + 5-step interview + preview + alert-dialog cancel)
- [x] 05-05-PLAN.md - Wave 2: SC-4 Agent State Inspector (Accordion tree from useChat.messages) + SC-5 Knowledge Graph Browser (GET /api/knowledge/graph + 3-level Accordion + search filter) + SC-8 version badges on ChunkSourceView
- [x] 05-06-PLAN.md - Wave 2: SC-6 Counter-evidence (BE post-verdict 历史 pass + FE teal annotation card with disclaimer) + SC-7 Redline (conditional LLM pass on 不合规/需补充 + FE diff view with copy/download)
- [x] 05-07-PLAN.md - Wave 3 (gap-closure, UAT #2): left-sidebar IA — sessions + skills in LEFT sidebar, chat-input Skill selector, SkillManagerMenu wizard-only, all wired to LIVE /api/skills (item.manifest.*)

**Wave structure**: W1={05-01, 05-02 sequential} -> W2={05-03, 05-04, 05-05, 05-06 parallel} -> W3={05-07 gap-closure}
**UI hint**: yes

### Phase 6: Eval Dashboard & Production Hardening — Code-Side (6a)

**Goal:** Surface per-Skill metrics on a dashboard and stand up the production-grade observability + privacy posture — audit retention, redaction, session isolation. CI-gate WORKFLOW wiring is deferred to Phase 999.2 (user decision 2026-08-14: 先跳过 CI). Gate LOGIC (gateDecision / runGateCheck / eval-gate.json) is complete and unit-tested on master.
**Mode:** mvp
**Depends on:** Phase 5
**Requirements:** GEVL-06, GEVL-07
**Success Criteria** (what must be TRUE):

  1. A web dashboard surfaces per-Skill metrics — faithfulness, citation precision, refusal accuracy — with a trend line across runs (GEVL-06)
  2. The gate DECISION logic is pure, unit-tested, and shipped (`internal/eval/gate.go::gateDecision` + `cmd/eval runGateCheck` + `skills/compliance/eval-gate.json` with `faithfulness_threshold: 0.9`); the GitHub Actions workflow wiring lives in Phase 999.2 (GEVL-07 end-to-end deferred 2026-08-14)
  3. Audit records are queryable via a UI table filterable by user / skill / time range / `refusal_reason` with row-level redaction toggled by access policy
  4. Session isolation is proven by a concurrency test: 50 sessions × 5 turns each see only their own conversation history (no cross-session bleed)
  5. Retention / redaction policy is enforced: queries past the retention window have `query` and `answer` fields redacted in storage; access requires an explicit access policy
  6. Eval runs are reproducible: re-running against a frozen eval set + pinned `model_ver` + pinned `prompt_hash` returns numerically identical Ragas scores within tolerance

**Pitfalls addressed:** P-RAG-2 (monitoring + drift detection), P-COMPL-4 (audit trail integrity), P-EINO-2 (multi-tenant session isolation), P-OSS-5 (CI hygiene)
**Plans**: 3 active + 1 deferred (06-03 → 999.2)
**Plan list**:

- [x] 06-01-PLAN.md — Wave 1: Reproducibility substrate + gate logic (SC-6 + Q1/Q2 + GEVL-07 gateDecision)
- [x] 06-02-PLAN.md — Wave 2: Backend API surface — audit/redaction, /api/eval/*, SC-4 concurrency
- [→] 06-03-PLAN.md — DEFERRED to Phase 999.2 (2026-08-14, user decision: 先跳过 CI; workflow file exists only on unmerged worktree branch `worktree-agent-a1bfd67bedf21ad40` commit fc26d12)
- [x] 06-04-PLAN.md — Wave 3: Dashboard frontend — SkillsTable, TrendPanel, AuditTable (GEVL-06)

**Wave structure**: W1={06-01} → W2={06-02, 06-03} → W3={06-04}
**UI hint**: yes

### Phase 7: Open-Source Release

**Goal:** Ship a polished, documented, demo-able open-source release that a stranger can fork, run locally, and benchmark — the artifact that earns the personal-brand.
**Mode:** mvp
**Depends on:** Phase 6
**Requirements:** DIST-01, DIST-02, DIST-03, DIST-04, DIST-05, DIST-06, DIST-07, DIST-08, DIST-09, DIST-10
**Success Criteria** (what must be TRUE):

  1. `git clone && docker compose up` brings up the full stack on a clean laptop — backend, frontend, Milvus, pluggable embedding endpoint (DIST-04)
  2. README renders badges, value prop, demo GIF, quick start, architecture diagram, contributing, and license at the repo root (DIST-01, DIST-02, DIST-07)
  3. `.env.example` lists every required variable with comments; missing values produce a clear startup error (DIST-03)
  4. Pre-bundled sample compliance docs ship under `examples/data/` and the demo works with no API key for retrieval (only LLM calls require one) (DIST-06)
  5. A recorded demo GIF in the README hero walks through a sample compliance question and returns a cited verdict + structured refusal (DIST-05)
  6. `examples/` ships 2-3 Skill templates (compliance, simple QA, calculator) each passing the schema validator (DIST-08)
  7. README's "Eval Results" section shows published per-Skill faithfulness %, citation precision %, refusal accuracy % from the latest CI run (DIST-09)
  8. `eval/sample.jsonl` ships a 50-case sample of the per-Skill eval set in the repo (DIST-10)

**Pitfalls addressed:** P-OSS-1..6 (README, demo, Docker, contributing, license, eval results publication)
**Plans**: 6 plans
**Plan list**:

- [x] 07-01-PLAN.md — Wave 1: Release-meta foundations (LICENSE + .env.example internal-cleanup + .gitignore/.dockerignore hygiene + ROADMAP/REQUIREMENTS SC-1/SC-5 + DIST-04/DIST-05 revisions per D7-03/D7-06)
- [~] 07-02-PLAN.md — Wave 1 (parallel): Containerization (docker-compose + Dockerfile.backend + Dockerfile.frontend; Milvus v2.4.x pinned to SDK v2.4.2; SSE proxy_buffering off; autonomous:false — Docker verify gate) — **2/3 tasks committed (`798248d`+`3d9f590`); Task 3 docker-build human-verify BLOCKING (Docker not in PATH)**
- [x] 07-03-PLAN.md — Wave 2: Docs (README hero+quick-start+Mermaid architecture+eval placeholder+license; CONTRIBUTING write-a-Skill/add-a-tool/add-a-knowledge-tier)
- [x] 07-04-PLAN.md — Wave 2 (parallel): Examples + eval sample (3 SKILL.md templates passing internal/skill.Validate via examples_validate_test.go + 7 sample docs + eval/sample.jsonl 50-case copy)
- [~] 07-05-PLAN.md — Wave 3: Eval snapshot + demo GIF (scripts/eval_snapshot derives refusal_accuracy from Report.Cases; autonomous:false — needs live Milvus+BGE-M3+GLM + screen capture) — **1/3 tasks committed (`abf2f4d` RED + `4873213` GREEN — `scripts/eval_snapshot/{main.go,main_test.go}` shipped via TDD, 6/6 hermetic tests PASS); Tasks 2 & 3 `checkpoint:human-verify gate=blocking` BLOCKING — Task 2 needs live stack for `cmd/eval --frozen=compliance` → README snapshot; Task 3 needs screen capture for `docs/demo.gif`**
- [ ] 07-06-PLAN.md — Wave 4: Publication governance (single combined git filter-repo: strip .planning/ + scrub internal IPs/domains; 4 post-rewrite audit gates zero-match; force-push to public skillforge remote; autonomous:false — destructive, git-filter-repo not installed, no remote)

**Wave structure**: W1={07-01, 07-02 parallel} → W2={07-03, 07-04 parallel} → W3={07-05} → W4={07-06}
**UI hint**: no (packaging/docs/release-engineering phase — no new product UI)

---

## Phase Details — Milestone v0.8（Harness — 可信×行动融合）

> 依赖链（严格顺序）：8 → 9 → 10 → 11 → 12 → 13。Rationale（research SUMMARY.md §Implications for Roadmap）：升级先行（summarization 中间件与成熟 HITL 在 v0.7.7 不存在，混做会让回归归因不可能）→ spike 硬门（历史注入是唯一可能静默破坏已校准 trust floor 的行为风险，冻结基线自身抓不到漂移）→ 记忆先于审批（resume 回放整条消息列表，单轮记忆上做审批是返工）→ 审批先于 Deep（Runner 迁移落在简单 ReAct archetype 上最安全，全套回归套件做安全网）→ Deep 先于扇出（write_todos + 子 agent 槽位是天然的组合宿主，扇出继承 P8.3 的预算门）。

### Phase 8: 全量依赖升级（P8.0 — eino v0.7.7 → v0.9.14）

**Goal:** As a SkillForge 二次开发者, I want to 把依赖栈升级到 eino v0.9.14 并就位 CheckPointStore 骨架, so that v0.8 的记忆与审批能力解锁且升级回归归因干净.
**Mode:** mvp
**Depends on:** Phase 7（v0.7.7 里程碑结束态；不阻塞 07-02/07-05 人工 checkpoint）
**Requirements:** PLAT-04, PLAT-05
**Success Criteria** (what must be TRUE):

  1. `go build ./...` 与全部单元测试在 eino v0.9.14 + 联动升级的 eino-ext 下全绿；`integration/contract` 漂移套件与冻结 eval 基线重新运行全绿（PLAT-04）
  2. Phase-1 live verify 重新锁定：agent run cancel 从客户端到 agent <200ms；Milvus tier-filter 对声明外 tier 返回零行（PLAT-04）
  3. legacy interrupt API（`NewInterruptAndRerunErr` 等无地址旧语义）在代码库出现即 lint fail——Interrupt/Resume 双语义不并存（PLAT-04，Pitfall 5）
  4. sqlite `CheckPointStore`（复用 `internal/store/sqlite`，~100 LOC）Get/Set round-trip：所有自定义状态类型经 `schema.RegisterName[T](name)` 注册后可序列化写库、进程重启后读回等值（PLAT-05）
  5. 真实中断运行往返测试通过：一次 Run 中断 → checkpoint 持久化到 sqlite → 进程重启后 Resume 恢复到中断点继续执行（PLAT-05，Pitfall 4）

**Pitfalls addressed:** Pitfall 4（内存 checkpoint 丢中断态 — store 骨架），Pitfall 5（Interrupt/Resume 双语义并存 — 一次性切换 + lint 禁用）
**Research flag:** RF-2 — 已在研究期解决（`CheckPointStore` 接口 = Get/Set 两方法，eino@v0.9.14 internal/core/interrupt.go:27-30；编译 spike 实证升级零源码改动）
**Plans**: 5 plans
**Plan list**:

- [x] 08-01-PLAN.md — Wave 1: 升级主提交（eino v0.9.14 + ext openai v0.1.13 + TestContractVersionPin 改写 + 8 处 godoc 重核）+ hermetic 回归重锁（全量单测 / contract 套件 / cancel <200ms）(completed 2026-08-17, commit 36845c0 — 四道门全绿零修复提交)
- [x] 08-02-PLAN.md — Wave 2: sqlite CheckPointStore 骨架（schema DDL 追加 + SQLiteStore Get/Set + 编译期断言）+ 六面测试 TDD（round-trip / 重开 DB / upsert / unknown-id / SQLi / closed）(completed 2026-08-17, commits 9a557b1 RED + e6a580a GREEN — 六面全绿，internal/store 零回归，SC-4 store 层语义锁定)
- [x] 08-03-PLAN.md — Wave 2（parallel）: legacy interrupt API 禁令门（test-as-gate，含触发负向证明）+ 版本叙事文档对齐（REQUIREMENTS/ROADMAP/CLAUDE.md → v0.9.14）
- [x] 08-04-PLAN.md — Wave 3: SC-5 真实中断往返测试（schema.RegisterName 注册 + stub 剧本 + adk Runner + 重开 DB 新 Runner Resume）+ phase hermetic 全量收口
- [x] 08-05-PLAN.md — Wave 4: live 重锁 human-verify 三门（cancel <200ms 真 GLM / Milvus tier-filter 声明外零行 / 冻结 eval + gate 重跑）(completed 2026-08-18 — 三门 approved：cancel 实测 0ms；tier 5 行/0 行零泄漏；frozen eval 50/50 gate PASS faithfulness 0.9720、refusal_accuracy 95%、lock 50bb1688… 未动，指标留档 v0.8 基线 D8-07；零代码 plan)

**Wave structure**: W1={08-01} → W2={08-02, 08-03 parallel} → W3={08-04} → W4={08-05}
**UI hint**: no

### Phase 9: 历史注入 trust-floor spike 硬门（P8.S — SPIKE / HARD GATE）

**Goal:** 用 golden-set A/B 实验证明：把投影后的会话历史注入 prompt 不会静默漂移已校准的 trust floor（拒答率、faithfulness、引用精度）。**这是硬门——不过门，Phase 10 记忆不得发布。** 本 phase 是 spike，产出是报告 + 消毒规则，不是产品代码。
**Mode:** mvp
**Depends on:** Phase 8（冻结 eval 基线需在新依赖下重新锁定）
**Requirements:** GEVL-08
**Success Criteria** (what must be TRUE):

  1. Spike 报告落盘：golden set 在 `history=nil`（baseline）vs 投影历史下逐指标对比，覆盖三组场景——善意追问 / 恶意注入（用户粘贴伪制度文本进历史）/ 旧拒答上下文；四指标（faithfulness / answer_relevancy / context_precision / context_recall）漂移数字逐项可见（GEVL-08）
  2. Go/no-go 结论明确且有记录：四指标漂移 <5% 且 refusal delta <10% → go；不过门则产出逃逸方案选型（last-3-turns 预算 → prompt 附录 → 仅用户轮记忆）及理由（GEVL-08）
  3. 恶意注入用例全部被拒答或标注「未验证用户陈述」——零例让历史中的伪文本穿透成 citation（GEVL-08，Pitfall 2 早期暴露）
  4. 消毒规则作为 spike 主产物成文：历史投影剥离 `[CHUNK id]` 标记、历史 verdict JSON 经 `conclusionText`/`refusalText` 确定性渲染为纯文本——LLM 物理上复制不到它从未见过的引用标记（GEVL-08，供 Phase 10 实现）

**Pitfalls addressed:** Pitfall 1（历史注入污染 trust floor，H1–H3），Pitfall 2（用户引用文本穿透 tier 边界 — 早期暴露）
**Spike seed:** `spike-history-injection-trust-floor`（已预写；冻结 eval 集 + gate 已存在，无需 research-phase）
**Plans**: 6 plans
**Plan list**:
- [x] 09-01-PLAN.md — Wave 1: golden set 75 行口径（50 baseline + 10 恶意 + 15 善意三小类 5/5/5）+ GEVL-05 契约门升级
- [ ] 09-02-PLAN.md — Wave 1: internal/history/projection.go + projection_test_fixtures.go（D9-B1/B2/B3 三规则 + 6+ fixtures）
- [ ] 09-03-PLAN.md — Wave 2: cmd/spike-history spike-only 二进制（spike_row + runner + main；短历史 A/B 路径；dry-run + 成本守卫）
- [ ] 09-04-PLAN.md — Wave 3: long.go 长会话用例合成器（10 例 × 8/12/15 长度 × seeded shuffle；Plan 09-03 stub gate 替换）
- [ ] 09-05-PLAN.md — Wave 3: summary.go + summary_test.go（computeVerdict 6+ case hermetic + renderSummary + runSummarize；Plan 09-03 stub gate 替换）
- [ ] 09-06-PLAN.md — Wave 4: live UAT 端到端（GLM-5.2 + Milvus + BGE-M3）+ 4 SC 静态验证 + human-verify go/no-go 决策签字

**Wave structure**: W1={09-01, 09-02} → W2={09-03} → W3={09-04, 09-05} → W4={09-06}
**UI hint**: no

### Phase 10: 多轮对话记忆（P8.1）

**Goal:** 用户可跨页面刷新进行多轮追问——agent 结合消毒后的会话历史正确解析指代并作答；citation 白名单代码级锁定为本轮检索，历史/摘要文本永远不能成为证据。
**Mode:** mvp
**Depends on:** Phase 9（spike 硬门必须 go；消毒规则与逃逸方案选型是本 phase 的输入）
**Requirements:** MEM-01, MEM-02, MEM-03, MEM-04, MEM-05
**Success Criteria** (what must be TRUE):

  1. 用户在收到回答后追问「那第二条呢」「依据是什么」，agent 结合会话历史正确解析指代对象，并给出带本轮检索引用的回答（MEM-01）
  2. 页面刷新后会话历史仍存活并喂给 agent——server 层 `internal/history` 将 sqlite session 投影为 `[]schema.Message`；刷新后的追问仍正确解析（MEM-02）
  3. Citation 白名单代码强制：构造历史中含旧 chunk 引用的会话，凡本轮未检索到的 chunk 出现在答案中即被 grounding 剥离或触发拒答——无代码路径可让历史/摘要/resume 注入文本成为证据（MEM-03）
  4. 历史投影消毒生效：投影消息中不出现 `[CHUNK id]` 标记；历史 verdict JSON 经 `conclusionText`/`refusalText` 确定性渲染为纯文本（测试断言投影输出零 chunk 标记）（MEM-04）
  5. 长会话超 token 预算时按 project→then→summarize 顺序压缩（先哑预算；summarization 中间件在 Runner 迁移后启用），会话可持续不报错，预算/压缩事件可观测（MEM-05）；`TestSessionIsolation50x5` 在新投影路径下重新跑绿

**Pitfalls addressed:** Pitfall 1（消毒规则落地），Pitfall 2（白名单 + red-team tier 穿透用例入 eval），Pitfall 7（token 阈值对 GLM 真实用量校准）
**Research flag:** RF-6 — summarization 中间件 token 计数器对 GLM 用量日志校准；tokenizer 可配置性未验证（启用前用哑预算兜底）
**Plans**: TBD
**UI hint**: yes（多轮聊天 UX — 历史恢复、刷新后连续性为用户可感知界面行为）

### Phase 11: 写工具 + 审批门（P8.2）

**Goal:** agent 调用写工具前必须经人在环审批——流内审批卡、跨刷新/重启存活的审批态、INSERT-only 三行审计生命周期、「批的 A 不会执行成 B」。「可信 × 行动」首次正面落地。Runner 迁移（Interrupt/Resume 新语义）落在简单 ReAct archetype 上，全套既有回归套件做安全网。**FE 与 BE HITL 同 phase 交付，不可拆。**
**Mode:** mvp
**Depends on:** Phase 10（resume 回放整条消息列表——审批不能建在单轮记忆上）
**Requirements:** HITL-01, HITL-02, HITL-03, HITL-04, HITL-05, HITL-06, HITL-07, HITL-08
**Success Criteria** (what must be TRUE):

  1. agent 触发写工具时中断（Eino Interrupt），SSE 流干净关闭，聊天流内呈现内联审批卡：what（动作）/ tool（工具名）/ 完整参数快照 / why（理由）；用户可批准一次或拒绝（拒绝必填理由），审批绑定结构化工具调用（name+args），永不绑定原始字符串（HITL-01, HITL-02）
  2. 中断后刷新页面或重启后端，审批态经 sqlite checkpoint + GET resume 端点恢复，审批卡回到待决态；checkpoint 绑定 skill 版本 + prompt_hash，二者变更后的过期 checkpoint 显示显式过期 UI 态且不可 resume（HITL-03）
  3. 每次审批决策产生 INSERT-only 审计三行生命周期（`action_pending_approval` → `action_approved` / `action_rejected`）；interrupt 时冻结参数快照进 pending 行，Resume 执行的参数与快照逐字节一致（批的 A 不会执行成 B）；审计写入失败则写工具拒绝执行（fail-closed，与读路径 swallow 相反的显式 Key Decision）；tool_call_id 幂等键防重复执行（HITL-04）
  4. `writes:` frontmatter 代码级强制——为未声明对应 writes 的 Skill 构造写工具调用被平台层拒绝（agent 无法绕过）；首批两个写工具可用：memory:facts 写入 + 审查报告生成（HITL-05, HITL-06）
  5. 用户可在批准前修改工具参数（edit-before-approve），修改后参数进入审批审计行；用户可「批准并记住」，规则按会话/Skill 级作用域存储，可查看、可撤销，后续同类调用按规则自动放行（HITL-07, HITL-08）

**Pitfalls addressed:** Pitfall 3（审批门破坏审计完整性 — write-ahead pending 行 + fail-closed），Pitfall 4（持久 checkpoint + GET resume 刷新恢复），Pitfall 6（checkpoint 绑定版本/prompt_hash 拒过期 resume），Pitfall 10（FE 审批状态机：审批态派生自 message parts，永不入 Zustand；D-04 transport-swap fixtures 覆盖 approve/deny 回路）；`TestCancelPropagationMidTool` <200ms 在 Runner 迁移后重新锁定
**Research flags:** RF-3 — ai@7 custom transport resume 形状（独立端点 + transport 分支 vs 合成 user message）；RF-2 — 升级后 CheckPointStore 接口精确形状（廉价 compile-check）
**Plans**: TBD
**UI hint**: yes（流内审批卡 ApprovalPart + 过期/待决状态 UI）

### Phase 12: 规划层（P8.3 — DeepAgent + write_todos + 可回放）

**Goal:** Skill 声明 `agent: deep` 即获得 DeepAgent 规划能力——实时计划清单流、执行前计划确认门、三层预算门防成本爆炸、规划轨迹仅凭审计行即可完整回放。信任锚（floor→grounding→critique）在此 phase 提升为所有 archetype 统一经过的 `ApplyTrustStage`。
**Mode:** mvp
**Depends on:** Phase 11（Deep 落进已 Runner 化的 harness；复用审批机制做计划确认门；复用审计/checkpoint 基建）
**Requirements:** PLAN-01, PLAN-02, PLAN-03, PLAN-04, PLAN-05
**Success Criteria** (what must be TRUE):

  1. SKILL.md 写 `agent: deep` 后该 Skill 经 `internal/runtime` 工厂按 `Manifest.Agent` 分发到 DeepAgent archetype（`adk/prebuilt/deep` + write_todos）；所有 archetype（chat / deep / 后续 sub-agent）统一经 `ApplyTrustStage`（floor→grounding→critique）出口，回归锁随重构迁移保持绿（PLAN-01）
  2. 用户在执行期间看到计划清单实时流式更新（todo pending / in_progress / completed），计划随会话持久化——刷新后计划状态仍在（PLAN-02）
  3. DeepAgent 执行前用户看到完整计划并可批准/拒绝（计划确认门复用 Phase 11 审批机制）；拒绝后 agent 修改计划重新提请（PLAN-03）
  4. 三层预算门任一触发即保守收尾（`forced_stop`，绝不硬跑到底）：MaxIteration ≤10 + run 级 token 预算 + 回路检测；预算事件写入审计可查（PLAN-04）
  5. 仅凭审计表中的 write_todos 行可完整重建某次运行的计划演变序列；UI 提供规划轨迹回放视图（PLAN-05）

**Pitfalls addressed:** Pitfall 8（DeepAgent 默认 MaxIteration=50 成本爆炸 / 无界回路 — 三层预算门从第一天起在）
**Research flag:** RF-5 — DeepAgent 最终消息 verdict JSON 解析率在 GLM-5.2 上实测；兜底 = 最终步 structured-output 选项（verdict-JSON 后缀 + `forced_stop` fallback 已定）
**Plans**: TBD
**UI hint**: yes（计划清单流 PlanTracePart + 轨迹回放视图 + 计划确认门 UI）

### Phase 13: 子 agent 扇出（P8.4 — tier 继承）

**Goal:** orchestrator 将多跳检索扇出为一层并行子 agent——子 agent 的 tier 知识边界在构造期以闭包继承（结构性无法扩权）、合并结果确定性且对部分失败保守、每个子 agent 结论合并前独立过逐字引用验证。首次用途保持窄：一层并行多跳检索。
**Mode:** mvp
**Depends on:** Phase 12（复用 P8.1 citation-ID 机制、P8.2 审计/checkpoint 基建、P8.3 预算门）
**Requirements:** FAN-01, FAN-02, FAN-03, FAN-04
**Success Criteria** (what must be TRUE):

  1. 多跳问题触发 orchestrator 并行扇出子 agent（一层为限——子 agent 无扇出能力，禁止递归）；子 agent 返回压缩结果；任一子 agent 故障被隔离，主流程继续（FAN-01）
  2. Tier 边界构造期闭包继承：子 agent 的 retrieve 工具闭包自父 manifest 的 `ResolvedTiers()`——测试证明子 agent 无法查询父未授权的 tier（结构性，非 prompt 约束）；父子 tier 交集为空 = 加载期校验错误而非运行时行为（FAN-02）
  3. 合并确定性：同一组子 agent 结果合并 10 次输出字节一致（key 排序纯函数，契约测试把关）；保守部分失败合并——任一子 agent 拒答时合并结论最多「需补充」，绝不产出肯定裁决（FAN-03）
  4. 每个子 agent 结论在合并前独立过 D2-01 逐字引用验证（per-sub-agent grounding，共享 per-Run ChunkAccumulator）；扇出事件（RunPath / Transfer）写入审计可按 run 查询（FAN-04）

**Pitfalls addressed:** Pitfall 9（tier 泄漏 + 非确定性合并）
**Research flag:** RF-4（最高不确定性）— ADK transfer/handoff ctx-value 传递到子 agent 工具闭包：本 phase 以此 spike 开场；已知兜底 = per-Run agent 构造（accumulator 烘进闭包），成本更高。子 agent tier 继承精确 API 形态在文档未验证（LOW-MEDIUM），需 plan 级研究
**Plans**: TBD
**UI hint**: no（扇出事件走既有审计/进度通道，无新产品 UI；如 plan 阶段发现需要子 agent 进度可视化再补注）

---

## Coverage

### Milestone v0.7.7 (v1 requirements)

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

### Milestone v0.8 (Harness requirements)

| Phase | Requirements | Count |
|-------|--------------|-------|
| 8 - 全量依赖升级 (P8.0) | PLAT-04, PLAT-05 | 2 |
| 9 - 历史注入 spike 硬门 (P8.S) | GEVL-08 | 1 |
| 10 - 多轮对话记忆 (P8.1) | MEM-01..05 | 5 |
| 11 - 写工具 + 审批门 (P8.2) | HITL-01..08 | 8 |
| 12 - 规划层 (P8.3) | PLAN-01..05 | 5 |
| 13 - 子 agent 扇出 (P8.4) | FAN-01..04 | 4 |
| **Total** | **25 of 25 v0.8 requirements mapped** | **25** |

**Unmapped:** 0 ✓ (v1 + v0.8)
**Orphaned groups:** None.

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Integration Spike & Trust Contract | 4/4 | Complete | 2026-08-11 |
| 2. Compliance Vertical Slice | 5/5 (incl. 02-04-MONGO) | Complete | 2026-08-11 |
| 3. Agentic Retrieval & Trust Evaluation | 6/6 | Complete | 2026-08-12 |
| 4. Streaming Trust UI | 4/4 (incl. 04-04 gap-closure) | Complete | 2026-08-11 (04-04 debt-closure 2026-08-12) |
| 5. Skill Platform & Full Knowledge Boundaries | 8/8 (incl. 05-08 gap-closure) | Complete | 2026-08-12 (05-08 debt-closure 2026-08-13) |
| 6. Eval Dashboard & Production Hardening — Code-Side | 3/3 active (+06-03 deferred→999.2) | Complete (code-side) | 2026-08-13 |
| 7. Open-Source Release | 5/6 (07-02 2/3 tasks · 07-05 1/3 tasks · 07-06 not started) | In Progress — 2 human-verify checkpoints blocking (07-02 T3 docker-build, 07-05 T2/T3 live stack + GIF) | - |
| 8. 全量依赖升级 (P8.0) | 5/5 | Complete — live 三门 human-verify approved 2026-08-18（cancel 0ms / tier 5行+0行 / frozen eval gate PASS 0.972，v0.8 基线留档）；ready for verify-work | 2026-08-18 |
| 9. 历史注入 spike 硬门 (P8.S) | 1/6 | Wave 1 partial — 09-01 spike 数据地基 DONE (75-row golden set 50 baseline + 10 adversarial + 15 followup 5/5/5; GEVL-05 契约门 + spike row schema 门 green); 09-02..09-06 pending | 2026-08-18 |
| 10. 多轮对话记忆 (P8.1) | 0/? | Not started — gated by Phase 9 | - |
| 11. 写工具 + 审批门 (P8.2) | 0/? | Not started | - |
| 12. 规划层 (P8.3) | 0/? | Not started | - |
| 13. 子 agent 扇出 (P8.4) | 0/? | Not started | - |

---
*Last updated: 2026-08-18 — Phase 8 COMPLETE (5/5)：08-05 live 三门重锁全部 human-verify approved（真 GLM cancel 0ms / Milvus tier-filter 5行+0行零泄漏 / 冻结 eval gate PASS faithfulness 0.972 + refusal_accuracy 95%，lock 未动，指标留档 v0.8 基线 D8-07）；PLAT-04 + PLAT-05 勾选。Next: Phase 9 (GEVL-08 spike 硬门). Prior: 2026-08-17 — Milestone v0.8 roadmap appended (phases 8-13, strict chain per research SUMMARY.md; 25/25 v0.8 requirements mapped; v0.7.7 phases 1-7 and backlog 999.1/999.2 unchanged).*

---

## Backlog

### Phase 999.1: 分层分块器决策 — hierarchical markdown splitter (BACKLOG)

**Goal:** Decide the fate of the markdown hierarchical splitter that was silently dropped from scope. Two options: (a) formally record one-doc-one-chunk + maxDocBytes(4000) guardrail as the permanent design (fine if knowledge corpus stays short-clause .md), or (b) implement eino-ext markdown header splitter to support long policy docs / PDFs. Origin: stale comments in internal/knowledge/indexer.go claimed "deferred to Phase 3 (CMPL-06)" — corrected in quick task 260814-ofn; no phase ever owned this work.
**Requirements:** TBD
**Plans:** 5/5 plans complete

### Phase 999.2: Eval CI Wiring + Live UAT (BACKLOG) — Phase 6b

**Goal:** Complete the deferred CI-gate half of Phase 6 (split decision 2026-08-14: code-side scope completed as Phase 6a, this backlog holds the runtime-dependent remainder). Items: (1) merge `.github/workflows/eval-gate.yml` from unmerged worktree branch `worktree-agent-a1bfd67bedf21ad40` (commit fc26d12, 138 lines) to master; (2) register `GLM_API_KEY` (required) + optional `BGE_M3_URL` / `MILVUS_ADDRESS` as GitHub repository secrets; (3) sacrificial PR proving the gate blocks a faithfulness < 0.9 regression with `gh pr comment` naming skill + threshold + ❌; (4) the 4 pending `06-HUMAN-UAT.md` items — dashboard visual check (`--archive` → `/eval` + `/eval/audit`), `SKILLFORGE_ADMIN_TOKEN` redaction toggle, `go test -tags liveeval` jitter layer with `GLM_API_KEY`, archive→dashboard walkthrough.
**Requirements:** GEVL-07 (end-to-end)
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd:review-backlog when ready)
