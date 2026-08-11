# Phase 3: Agentic Retrieval & Trust Evaluation - Context

**Gathered:** 2026-08-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 3 delivers two things, picking up exactly what Phase 2 deferred (see `02-CONTEXT.md <deferred>`):

1. **Make the agent autonomous in its retrieval decision (ARAG-01/02/03).** Phase 2's retrieval is a *fixed pipeline pre-step* (D2-01) — `VerdictPipeline` runs embed → tier-filtered search → verdict as a linear 10-step path, and refusal-by-construction refuses *before any LLM call*. Phase 3 turns this into an **agent loop**: the agent autonomously decides retrieve / ask-clarifying / refuse / answer, performs bounded multi-hop retrieval, and Self-RAG-verifies every claim before the final answer. This is observable via SSE `decision` payloads (SC-1).

2. **Stand up the measurement moat (GEVL-03/04/05).** The Ragas-backed eval pipeline (50-case golden set per Skill, Python sidecar, faithfulness / context_precision / answer_relevance → JSON report) + a SQL-queryable audit row on every query. This is the project's defensible quality bar ("可衡量效果" — the OSS personal-brand differentiator vs. a pure interview project).

**Scope anchor:** autonomy + measurement + trust hardening. NOT multi-skill routing (Phase 5 SYSK-05), NOT UI (Phase 4 — already shipped), NOT CI gating / dashboard (Phase 6 GEVL-06/07), NOT counter-evidence/redline/policy-version (Phase 5 CMPL-06/07/08).

**Requirement coverage (14):** AGNT-01, AGNT-02, AGNT-04, AGNT-05, ARAG-01, ARAG-02, ARAG-03, KNOW-01, KNOW-02, KNOW-03, KNOW-04, GEVL-03, GEVL-04, GEVL-05. (ARAG-04 citation=chunk_id+span and ARAG-05 structured refusal were already delivered in Phase 2 and are carried forward as locked.)

**Sequencing note:** Phase 3 was deferred under MVP vertical-slice reordering (Phase 2 → Phase 4 shipped first). Phase 4 (Streaming Trust UI) is already complete and consumes the SSE events this phase enriches with `decision` payloads. The agent-loop refactor of `VerdictPipeline` MUST preserve the UIMessageChunk wire contract (D-04 / D15 Phase 4 invariant) so the shipped UI keeps rendering correctly.
</domain>

<decisions>
## Implementation Decisions

### Agent Autonomy Architecture (ARAG-01 ↔ D-09 — the core tension)
- **D3-01: Hybrid tool-calling agent + Go "trust floor" (option C).** Retrieval becomes an Eino ADK **tool** the agent calls (also satisfies **AGNT-02 function calling**); the agent autonomously decides retrieve / ask-clarifying / refuse / answer. The deterministic refusal moat is preserved by a Go-side **trust floor** wrapping the whole agent run: after the run completes, if the **total tier-matched chunks across all hops == 0**, **force a structured refusal regardless of agent output**. The existing `GroundingChecker` (D2-01) continues to strip per-citation ungrounded claims on top. Rationale: maximally agentic AND keeps D-09's "refusal-by-construction, never rely on the LLM policing itself" — agent autonomy on top, deterministic floor underneath. Chosen over A (router-on-deterministic-core, underuses Eino ADK — the project's agent-engineering showcase) and B (pure tool-calling, moat depends on LLM choosing to refuse — exactly the D-01 risk).
- **D3-02: Multi-hop = agent-self-judged + hard cap `max_hops=2` (1 initial + 1 re-hop).** The ADK agent decides "information insufficient" and calls retrieve again with a naturally-rewritten query; Go caps at 2 hops total (satisfies SC-2's literal "a second retrieval hop"). At cap, the agent MUST answer or refuse with the chunks it has — no open-ended looping. Query refinement is the agent's own rewrite (not Go keyword extraction) — the agentic answer. Chosen over Go-coverage-signal-forced re-hop (conflicts with agent-autonomy philosophy) and dual-trigger (over-engineering for Phase 3).
- **D3-03: SC-1 decision observability = tool-call events → SSE `decision` frames.** The agent's retrieve/ask/refuse/answer decisions ARE its tool calls / structured outputs; Eino `callbacks.Handler` (OnStart/OnEnd per tool/LLM) maps these to a `decision` SSE payload on the wire (extending `wire.UIMessageChunk`, preserving D-04/D15). Researcher confirms the exact Eino callback hook + chunk discriminator; planner wires it.

### Self-RAG Self-Verification (ARAG-03 + ARAG-04)
- **D3-04: Layered verification — Go grounding hard floor + optional LLM critique (option C).** The existing deterministic `GroundingCheck` (substring match after D2-05 normalize, D2-02 downgrade) is the **hard floor**: always runs, can strip/downgrade, cannot be weakened. An optional **LLM critique** pass re-reads retrieved docs + draft verdict and flags claims that pass the substring check but are **semantically misused** (the known blind spot: chunk says X, agent cites the same span to support ¬X — verbatim-present, meaning-reversed). The critique can ONLY append `review_required` flags; it **MUST NOT override the floor or raise confidence.** Same shape as D3-01's trust floor — deterministic floor + "only-adds-strictness" LLM layer. Chosen over pure-Go (misses the semantic-reversal blind spot) and pure-LLM-judge (D-01: moat must not rely on LLM judging itself).
- **D3-05: LLM critique is gated, not always-on.** Runs only when (Go floor PASSED i.e. grounded) AND (confidence in an uncertain band, e.g. 0.6–0.9, OR status == 需补充, OR multi-hop verdict). High-confidence single-hop 合规/不合规 skips critique (save cost); refusals skip (no claim to critique). Exact band thresholds are Claude/researcher discretion, calibrated by the eval data.

### Eval Pipeline & Golden Set (GEVL-04/05 + D2-15 threshold calibration)
- **D3-06: Golden set = hand-authored synthetic, extending the Phase 2 corpus.** ~50 cases per Skill, deliberately constructed to include the **three GEVL-05 categories** (adversarial / refusal-required / multi-hop). Deterministic, reproducible, zero licensing/PII risk — fits the OSS personal-brand "measurable" goal. Reuses `knowledge/authority/` + the spike's 5 queries for comparability. Real-user sampling may be added in Phase 6.
- **D3-07: Ragas judge = GLM default + configurable `judge_model` slot + human spot-check.** GLM is the only model reachable under the internal-network strategy (Milvus + BGE-M3 内网, GLM 公网). Self-evaluation bias (faithfulness skewing optimistic) is countered two ways: (a) the deterministic Go grounding floor already catches the worst hallucinations regardless of judge, (b) human spot-check on a sample. `judge_model` is a config slot so Phase 6 / users can swap in a second model (e.g. DeepSeek public) for cross-evaluation. Chosen over dual-model-by-default (extra dependency + cost + reachability) and pure-GLM-no-slot (inflexible).
- **D3-08: Sidecar = CLI batch + produce calibration data; CI gate deferred to Phase 6.** Go writes a JSONL of (query, answer, retrieved context, refusal_reason) from a golden-set run → Python offline Ragas → JSON report (satisfies SC-6's "emits … into a JSON report" exactly). This phase ALSO produces the **calibration data** that gives the `retrievalScoreThreshold = 0.3` placeholder (`pipeline.go:43`, D2-15) a measured recommended value from the score distribution. The CI promotion gate (GEVL-07 faithfulness < 0.9 blocks) is explicitly Phase 6. Chosen over always-on HTTP service (over-engineering for Phase 3; the dashboard that needs it is Phase 6).

### Audit Log + Session Memory Storage (GEVL-03 + AGNT-05)
- **D3-09: SQLite default + `store.Store` abstraction (PG-ready), mirroring D11.** A new `internal/store` package defines a `store.Store` interface for audit rows (GEVL-03) + session memory (AGNT-05); SQLite is the v1 default implementation (embedded, zero-ops, single file, ships in repo, SQL-native so GEVL-03 "queryable via SQL" is satisfied literally); PostgreSQL is a Phase 6/production drop-in behind the same interface. Rationale: this mirrors the proven D11 pattern (`retriever.Retriever` + Milvus/Mongo), keeps v1 zero-ops (OSS fork-and-run), AND the abstraction itself signals engineering maturity to reviewers — the narrative is "we designed for production switchover", not "we picked a toy DB". SQLite WAL handles the Phase 6 concurrency test (50 sessions × 5 turns) trivially. Not PostgreSQL-by-default (heavy dep + ops, conflicts with local+Docker positioning) and not MongoDB-reuse (not SQL — directly conflicts with GEVL-03).
- **D3-10: Session memory = persistent to the store, partitioned by `session_id` (Claude discretion).** Conversation memory is written to the same store (keyed by `session_id`); cross-session isolation is enforced by row-level `session_id` partitioning, proven by the SC-9 concurrency test (two concurrent same-user sessions see only their own history). Restart-safe; audit rows correlate to sessions by `session_id`; sets up Phase 6 retention/redaction. Chosen over in-memory map (lost on restart + diverges from audit store) and TTL-ephemeral (extra mechanism for Phase 3).

### Claude's Discretion
- **Trust-floor counting detail (D3-01):** the exact definition of "total tier-matched chunks across all hops == 0" (e.g. dedup across hops, whether low-score-but-nonzero counts). Researcher/planner nails this; the invariant is "no tier-matched evidence → forced refusal, period".
- **Critique uncertain-band thresholds (D3-05):** the 0.6–0.9 band is illustrative; researcher calibrates from the golden-set score distribution produced in this phase.
- **Golden-set 50-case category split (D3-06):** the adversarial / refusal-required / multi-hop ratio. Researcher proposes; should over-weight refusal-required (the moat's headline behavior).
- **`prompt_hash` computation (GEVL-03 audit row):** hash of the rendered system prompt + skill id + model version (deterministic, reproducible). Researcher defines the exact canon.
- **KNOW-02 tier-mandatory enforcement layer:** enforce in the Go indexer (`internal/knowledge/indexer.go`) — backend-agnostic, consistent with D7's "tier is metadata, filter PRE-similarity" and the retriever abstraction. Reject at insert if `tier` missing/invalid. (Backend-specific schema validators are belt-and-suspenders, not primary.)
- **`store.Store` interface method set:** planner designs (`PutAudit`, `GetSession`/`AppendTurn`, query helpers for GEVL-03 SQL filter by user/skill/time/refusal_reason). Follow the D11 interface-first pattern.
- **`max_hops=2` cap semantics:** at cap, the agent must emit a final verdict or refusal using accumulated chunks — no third hop, no "I need more" loop escape. SC-1 `decision` payload marks the forced-stop case.

### Reviewed Todos (not folded)
- **`spike-temperature-field-validation.md`** (score 0.4 — keyword false-positive on phase/skill/field) — H1+H2+H4 temperature-field validation. **Already complete** in Phase 1 (per `01-CONTEXT.md`); SYSK-08 carry-forward was implemented in Phase 2's 7-field validator + verdict pipeline. Not Phase 3 work.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level (locked decisions that constrain this phase)
- `.planning/PROJECT.md` — Core Value (Skill-driven platform) + Key Decisions D1–D15 (esp. **D6** Ragas Python sidecar, **D7** Milvus tier mandatory metadata + PRE-similarity filter, **D8** citation = chunk_id + span, **D9** refusal-by-construction, **D10** Self-RAG/CRAG, **D11** MongoDB backend + the abstraction pattern Phase 3 mirrors for `store.Store`, D12–D15 Phase 4 wire invariants)
- `.planning/REQUIREMENTS.md` — Phase 3 requirements: AGNT-01/02/04/05, ARAG-01/02/03, KNOW-01..04, GEVL-03/04/05
- `.planning/ROADMAP.md` — Phase 3 goal + 9 Success Criteria + pitfalls addressed (P-RAG-1..4, P-AGENTIC-1/3/4, P-EVAL-1..4, P-COMPL-1/3/6)
- `.planning/STATE.md` — open questions #4 (threshold calibration), #8 (eval set composition); research flag "Phase 3: Chinese eval dataset sourcing, BGE-M3 reranker, Ragas judge calibration"

### Schema & Research
- `.planning/SKILL_SCHEMA.md` — SKILL.md v1.0 7-field spec (`reads` → tier value mapping; `temperature` SYSK-08)
- `.planning/research/PITFALLS.md` — P-RAG-1..4 (hybrid retrieval, tier bleed), P-AGENTIC-1/3/4 (autonomous decision, multi-hop, self-verify), P-EVAL-1..4 (Ragas pipeline, judge calibration), P-COMPL-1/3/6 (cited-answer rigor, audit log)
- `.planning/research/STACK.md` + `.planning/research/ARCHITECTURE.md` — locked stack (Eino ADK GA, Ragas, BGE-M3), 6-layer architecture, Eino ADK mapping
- `.planning/research/SUMMARY.md` — stack locked, recommended phase order

### Prior phase context
- `.planning/phases/02-compliance-vertical-slice/02-CONTEXT.md` — D2-01..D2-15 (esp. **D2-01** retrieval-is-a-pre-step → Phase 3 changes this; **D2-15** 0.3 threshold placeholder → Phase 3 calibrates; **D2-06** Citation struct; **D2-13/14** refusal taxonomy + rule-based suggestion; the `<deferred>` list is exactly Phase 3's pickup list)
- `.planning/phases/01-integration-spike-trust-contract/01-CONTEXT.md` — D-01..D-12 (esp. **D-08** BGE-M3, **D-09** Milvus default-deny, semantic-determinism philosophy, **cancel < 200ms** SC-3 proven for AGNT-04)
- `.planning/phases/04-streaming-trust-ui/04-CONTEXT.md` — D-04 transport-swap invariant + D15 wire-type drift alarm: the agent-loop refactor MUST preserve the UIMessageChunk contract so the shipped Phase 4 UI keeps rendering

### Spike/sketch findings (auto-loads)
- `.claude/skills/spike-findings-ai-agent/SKILL.md` — findings index (Eino v0.7.7 structured-output + temperature + ADK patterns; ChatModelAgent + Runner proven shape)
- `.claude/skills/sketch-findings-ai-agent/SKILL.md` — frontend layout findings (Phase 4 consumed; Phase 3 backend-only but SSE decision payloads must feed the existing chat input / parts)

### Existing code (reuse targets — see code_context)
- `internal/retriever/retriever.go` — `Retriever` interface (multi-hop = repeated `Search`; tier PRE-filter contract; `Hit.Score` available for D3-08 calibration)
- `internal/server/pipeline.go` — `VerdictPipeline` 10-step (Phase 3 refactors into agent loop; `retrievalScoreThreshold = 0.3` @ line 43 is the D2-15 placeholder to calibrate)
- `internal/server/chat.go` — `POST /api/chat` handler + `emitRefusal`/SSE writers; the cancel (AGNT-04) + session (AGNT-05) + `decision`-payload (SC-1) integration points
- `internal/verdict/grounding.go` — `GroundingCheck` / `ApplyDowngrade` / `GroundingChecker` (the D3-04 hard floor; pure + idempotent, unit-tested)
- `internal/verdict/verdict.go` + `internal/verdict/citation.go` — `Verdict` / `Citation{ChunkID,Span,Tier}` / `Refusal` + `RefusalReason` enum (no_authorized_source / low_retrieval_score / out_of_scope — ARAG-05 already delivered)
- `internal/knowledge/indexer.go` — indexer (KNOW-02 tier-mandatory enforcement point, D3 discretion)
- `internal/embedder/embedder.go` + `internal/embedder/bge/bge.go` — `Embedder` (query + doc embedding; multi-hop re-embeds the refined query)
- `internal/skill/{loader,manifest,tier,validate}.go` — `Manifest` + `ResolvedTiers()` + 7-field validator (SYSK-07/08; the agent tool's tier scope comes from here)
- `internal/wire/{chunk,sse}.go` — `UIMessageChunk` 16-type wire format (SC-1 `decision` payload extension point; D-04/D15 invariant preserved)
- `cmd/spiked/e2e.go` — proven `adk.NewChatModelAgent` + `Runner.Query` + structured-output pipeline (the template the agent loop builds on)
- `cmd/server/main.go` — server bootstrap + `Deps` wiring (where `store.Store` + eval sidecar get assembled)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`retriever.Retriever` interface** — `Search(ctx, queryVec, tiers, topK)` is stateless and one-shot; multi-hop (D3-02) is literally calling `Search` again with the agent's rewritten query (re-embedded). The tier PRE-filter contract + empty-tiers→0-hits guarantee is the trust floor's foundation. `Hit.Score` (float32) feeds the D3-08 calibration distribution. Both Milvus and MongoDB impls exist (D11).
- **`internal/server/pipeline.go` `VerdictPipeline`** — the current linear 10-step orchestration (resolve tiers → embed → search → 0-chunk refusal → low-score refusal → prompt → runner → parse → grounding → emit). Phase 3 refactors steps 3–10 into an agent loop while preserving steps 1–2 (tier resolution) and the refusal-by-construction short-circuits as the trust floor.
- **`internal/verdict/grounding.go`** — `GroundingCheck` (pure predicate), `ApplyDowngrade` (D2-02 idempotent policy), `GroundingChecker` (combined). The D3-04 Self-RAG hard floor is this, formalized as a pre-emit self-verification pass. Already unit-tested.
- **`verdict.Refusal` + `RefusalReason` enum** — ARAG-05 structured refusal already shipped in Phase 2; the trust floor (D3-01) and Self-RAG refusal (D3-04) reuse `RefusalReasonNoAuthorizedSource` etc.
- **`cmd/spiked/e2e.go`** — the proven `adk.NewChatModelAgent` → `adk.Runner.Query` → drain iterator → parse Verdict JSON shape (temp=0, json_object, thinking:disabled via `WithExtraFields`). Phase 3's tool-calling agent builds directly on this; the spike's SC-3 two-step (ChatModel → tool) demo is the seed for the retrieve-tool loop.
- **`wire.UIMessageChunk`** 16-type wire format — the SC-1 `decision` payload is a new (or data-*) chunk type on this wire; `source-document` carries citations, `data-policy` carries the Verdict (established in Phase 2).

### Established Patterns
- **Refusal-by-construction (D-09 / D2-01):** 0 tier-matched chunks → refusal with NO LLM call. The trust floor (D3-01) preserves the *spirit* (no-evidence → forced refusal) even though the agent loop means an LLM is now in-flight — the floor overrides post-hoc.
- **Semantic-determinism philosophy:** structured output + programmatic grounding + refusal-by-construction. Every Phase 3 decision (D3-01 hybrid, D3-04 layered, D3-07 judge) defers to this — deterministic floors, LLM only adds strictness.
- **T-2-21 prompt-injection mitigation:** chunks framed as `[CHUNK <id>]` DATA not instructions. The retrieve-tool's returned chunks MUST keep this framing when fed back to the agent (multi-hop injection resistance).
- **D11 abstraction pattern (interface + multi-backend):** the `store.Store` design (D3-09) copies this exactly — interface-first, SQLite default, PG drop-in. Proven migration path.
- **Internal-network strategy:** Milvus + BGE-M3 via 内网, GLM via public Coding Plan endpoint. The eval sidecar (D3-08) and judge (D3-07) inherit this — GLM is the only reachable judge, hence D3-07's GLM-default + config slot.

### Integration Points
- **`POST /api/chat` (`chat.go`)** → currently calls `VerdictPipeline`. Phase 3 inserts the agent loop + emits `decision` SSE frames (SC-1) + wires cancel propagation (AGNT-04 — spike SC-3 proved < 200ms React → agent cancel; extend to mid-loop) + session scoping (AGNT-05 — currently fully stateless, `session_id` is new plumbing through the handler).
- **New `internal/store`** (`store.Store` + SQLite impl) — audit table (GEVL-03 row schema) + session-memory table, both keyed/filtered by `session_id`. Assembled in `cmd/server/main.go` `Deps`.
- **New `eval/` Python sidecar** (CLI batch) — reads JSONL emitted by a Go golden-set runner, runs Ragas (faithfulness / context_precision / answer_relevance), writes JSON report + calibration data.
- **`internal/knowledge/indexer.go`** — add tier-mandatory enforcement at insert (KNOW-02, D3 discretion) — backend-agnostic, the hard path around D7.
- **Eino ADK retrieve-tool** — wraps `Retriever.Search`, enforces the Skill's `reads` tiers (from `Manifest.ResolvedTiers()`), returns `[CHUNK id]`-framed DATA to the agent; the agent loop's max-hops counter (D3-02) lives here or in the runner wrapper.
</code_context>

<specifics>
## Specific Ideas

- **`max_hops=2` = 1 initial + 1 re-hop** satisfies SC-2's literal wording ("a second retrieval hop with a refined query"); the final answer citing chunks from both hops is the multi-hop citation aggregation across the agent's tool calls.
- **The Self-RAG critique's target blind spot is explicit:** "逐字命中却语义反转" — chunk says X, agent cites the same verbatim span to support ¬X. Substring GroundingCheck passes it; the LLM critique catches it. This is the concrete technical motivation for D3-04's layered approach (not just flexibility).
- **`store.Store` reviewer narrative:** "designed for production switchover at the interface boundary, SQLite ships by default for zero-ops, PostgreSQL is a config switch" — this is the enterprise-positioning answer the user weighed in Area 4 (better senior-reviewer signal than either bare SQLite or premature PostgreSQL).
- **Golden-set comparability:** reuse the Phase 2 `knowledge/authority/` corpus + the spike's 5 compliance queries so eval results are directly comparable to the spike 002/003 baseline.
- **Judge-bias double-counter:** GLM self-evaluation optimism is countered by (a) the deterministic Go grounding floor catching worst-case hallucinations regardless of judge, and (b) human spot-check on a sample — so D3-07's GLM-default is defensible despite self-eval bias.
- **Trust-floor invariant phrasing:** "no tier-matched evidence across all hops → forced refusal, period" — the agent literally cannot fabricate an answer when the tool returned nothing usable, regardless of what it emits.
</specifics>

<deferred>
## Deferred Ideas

- **CI promotion gate (GEVL-07):** faithfulness < 0.9 blocks Skill promotion — Phase 6. Phase 3 produces the calibration data; Phase 6 turns it into a gate.
- **Eval dashboard (GEVL-06):** per-Skill metrics with trend line — Phase 6. Phase 3's CLI batch + JSON report is its data source.
- **HTTP always-on eval service:** for the dashboard — Phase 6 (D3-08 chose CLI batch for Phase 3).
- **PostgreSQL production backend impl:** Phase 6 (D3-09 leaves the `store.Store` PG-ready slot; SQLite ships now).
- **Retention / redaction / access-control policy:** Phase 6 SC-5 (queries past retention window redacted). D3-10's persistent session memory sets up the substrate for it.
- **Real-user-sampled golden-set augmentation:** Phase 6 (D3-06 ships synthetic now).
- **BGE-reranker (BGE-M3 reranker self-host vs API):** STATE research flag — if retrieval precision on the golden set is insufficient, a reranker stage sits between Milvus/Mongo top-K and the agent tool. Researcher validates in Phase 3 research; implementation may land here or defer.
- **LLM-generated `suggested_alternative_query` for non-refusal cases:** Phase 5 polish (D2-14 deferred — Phase 2 is rule-based only on the refusal path).
- **Multi-skill semantic routing (SYSK-05):** Phase 5.
- **Counter-evidence / redline / policy-version tracking (CMPL-06/07/08):** Phase 5. Chunk schema stays `{id, tier, content, source, embedding}` (no `effective_from/to`) through Phase 3.

</deferred>

---
*Phase: 3-Agentic-Retrieval-Trust-Evaluation*
*Context gathered: 2026-08-11*
