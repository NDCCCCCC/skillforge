# Phase 5: Skill Platform & Full Knowledge Boundaries - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning
**Scope locked:** ROADMAP 8 SC only — DATA-* (金融数据层) explicitly deferred (user-confirmed).

<domain>
## Phase Boundary

Promote SkillForge from "one compliance Skill" to a true **Skill-driven platform**. Eight success criteria across five capability themes:

1. **Multi-Skill semantic routing (SC-1, SYSK-05)** — ambiguous query routed to the right Skill purely by `description` semantic match; trace shows score + selected Skill.
2. **fsnotify hot reload (SC-2, SYSK-04)** — editing `SKILL.md` on disk reloads without restart; next query after 1s debounce uses new behavior.
3. **Skill Wizard (SC-3, SYSK-06)** — natural-language input → validated `SKILL.md` passing the Phase-2 7-field validator.
4. **Agent State Inspector (SC-4, AGNT-06)** — live panel: current turn's tool calls, retrievals (chunk IDs), intermediate reasoning.
5. **Knowledge graph browser (SC-5, KNOW-06)** — visual tier → doc → chunk; click chunk reveals citations.
6. **Counter-evidence (SC-6, CMPL-06)** — surface 权威 vs 历史 conflict chunks next to the verdict.
7. **Redline rewrite (SC-7, CMPL-07)** — propose clause-level rewrite when verdict flags a policy gap.
8. **Policy version filtering (SC-8, CMPL-08)** — `effective_to < today` chunks filtered out; UI shows active version.

**In scope:** SYSK-04, SYSK-05, SYSK-06, AGNT-06, KNOW-06, CMPL-06, CMPL-07, CMPL-08.
**Out of scope:** DATA-* financial data layer (deferred — see `<deferred>`), CI gate / eval dashboard (Phase 6 GEVL-06/07), open-source packaging (Phase 7 DIST-*).
**UI hint:** yes — four new UI surfaces (inspector / graph browser / counter-evidence annotation / redline) PLUS the Phase-4-deferred UI items (LeftSidebar real file tree, `@`-mention fuzzy search, Skill manager UI, bidirectional tab nav) which **highly overlap** Phase 5 (file tree ↔ SC-5 graph; Skill manager ↔ SC-3 wizard; `@`-mention ↔ multi-Skill routing).

**Requirement coverage (8):** SYSK-04, SYSK-05, SYSK-06, AGNT-06, KNOW-06, CMPL-06, CMPL-07, CMPL-08.

**Sequencing note:** Phase 5 depends on Phase 4 (shipped UI) + Phase 3 (agent loop, D16 decision SSE, store). All agent-loop + wire invariants (D-04/D12/D15/D16) MUST be preserved so the shipped UI keeps rendering.
</domain>

<decisions>
## Implementation Decisions

### Semantic Routing (SC-1, SYSK-05) — D5-01..05
- **D5-01: BGE-M3 embedding routing.** Reuse `internal/embedder/bge` to embed each Skill's `name`+`description`; query embedding cosine-similarity picks the Skill. Score = cosine — directly satisfies SC-1's "trace shows score". Chosen over LLM intent-classifier (extra call/cost, score must be fabricated) and hybrid (highest complexity, two code paths).
- **D5-02: in-memory map for Skill embeddings.** Embed all Skills at startup; hot-reload (SC-2) re-embeds only the changed Skill. N<100 makes Milvus overkill. Pairs naturally with fsnotify.
- **D5-03: ambiguity → ask the user.** When top-1 score is low OR top-1/top-2 are close, UI surfaces candidate Skills (with scores) for user selection — reuses the Phase-4 `SkillPicker` pattern. Most rigorous interpretation of SC-1's "ambiguous query routed to the right Skill".
- **D5-04: routing trace = extend D16 `data-decision`.** Add `skill_scores[]` + `selected` fields to the existing `data-decision` chunk. Does NOT touch the wire type table — preserves D-04/D12 transport-swap invariant and D15 drift alarm. Reserves the "new chunk type" budget for redline (D5-14).
- **D5-05: routing sits BEFORE ADK.** Go code dispatches (selects Skill via D5-01) then runs that Skill's pipeline/agent. `internal/server/pipeline.go` (currently single-Skill) evolves into "route-then-run". Chosen over `NewConditionalAgent` wrapping routing into the ADK graph (heavier refactor, less debuggable).

### Hot Reload (SC-2, SYSK-04) — D5-06..09
- **D5-06: in-flight runs are version-pinned.** A run in flight at the moment of file change keeps using the pre-change Skill snapshot to completion; fsnotify only updates the version used by the *next* new run. Matches SC-2's literal "next query uses new behavior" with zero interruption. (STATE.md flagged "in-flight hot reload" as the Phase-5 research item — resolved.)
- **D5-07: each turn resolves the current latest version.** Within a session, every turn parses the current Skill version (no session-level version pinning). Directly honors SC-2; a session may span versions, which is acceptable post-debounce.
- **D5-08: validation failure keeps the old version + reports.** A broken `SKILL.md` (7-field validation fail) leaves the prior version available and emits an SSE/log error (UI can show "Skill X load failed, kept old version"). Never silently drops a working Skill.
- **D5-09: reload scope = Skill definition only.** Only `SKILL.md` fields reload. `reads` changes are absorbed by per-run tier resolution. Knowledge-tier reindex (new docs) is a separate mechanism, NOT in SC-2.

### Skill Wizard (SC-3, SYSK-06) — D5-10..12
- **D5-10: guided multi-turn interview.** The agent walks the user through key decisions (purpose description / which `reads` tiers / compliance-vs-creative type / guardrails / temperature), then emits the `SKILL.md`. Highest quality, non-technical-user friendly, mirrors denova's `skills-creator` (the PROJECT.md-referenced inspiration). Chosen over one-shot generation (key fields like `reads` may be mis-guessed) and template-fill (low flexibility).
- **D5-11: preview → confirm → write to `skills/<name>/SKILL.md` + register.** File-driven registration (D2-07) takes effect immediately and is handed to fsnotify (SC-2) — closed loop. Prevents dirty files / accidental overwrites.
- **D5-12: temperature recommended by Skill type.** Compliance/review → 0.0; QA → ~0.2; creative/brainstorm → 0.7+. User can override. Reuses the SYSK-08 semantics and reinforces the "Skill drives determinism level" narrative.

### Counter-evidence + Redline + Policy Version (SC-6/7/8, CMPL-06/07/08) — D5-13..15
- **D5-13 (SC-6): counter-evidence = platform-level post-verdict pass.** The verdict itself STILL reads only 权威 (CMPL-04 / D-09 trust floor unchanged — authority remains the sole ground truth). After the verdict, an independent retrieval pass over the 历史 tier surfaces contradicting chunks as **annotation** alongside the verdict; this NEVER participates in grounding (D2-01 GroundingCheck). Chosen over extending `reads` to `[权威, 历史]` (mixes 历史 into grounding → D-09 tension) and metadata-tagged conflict relations (extra maintenance). The semantic boundary "counter-evidence is annotation, not grounding" is the trust-moat crux.
- **D5-14 (SC-7): redline = independent LLM pass on policy-gap + new `data-redline` chunk.** Only when the verdict flags a gap (`不合规` / `需补充`) does a separate LLM pass generate a clause-level diff rewrite. Wire = new `data-redline` chunk (`original_clause` + `proposed_rewrite` + `rationale`); wire `ValidTypes` 17→18 + TS `SkillForgeChunk` mirror per D15. No gap → no call (cost-controlled).
- **D5-15 (SC-8): extend chunk schema + retriever PRE-filter.** Chunk schema gains optional `effective_from`/`effective_to` (null `effective_to` = perpetually valid — backward-compatible with existing versionless chunks). Retriever PRE-filter `effective_to IS NULL OR effective_to >= today` sits at the SAME layer as the tier filter (D7 extension) — superseded policies filtered before similarity. All backends updated (Milvus + Mongo); KNOW-02 mandatory enforcement extends to `effective` fields. SC-8's "effective_to < today yields zero results" is satisfied at the filter layer.

### Claude's Discretion
- **SC-4 (AGNT-06) Agent State Inspector data source:** reuse the existing D16 `data-decision` + tool-call + reasoning SSE events already emitted by `internal/agent/callbacks.go` — the inspector is a new UI panel that re-renders these as a structured tree (tool calls / retrievals-with-chunk-IDs / intermediate reasoning), NOT a new instrumentation channel. Avoids duplicate state sourcing.
- **SC-5 (KNOW-06) Knowledge graph browser:** render tier → doc → chunk hierarchy within the locked three-panel layout (sketch-findings) using `store.Store` + knowledge metadata. Visualization engine (lightweight tree/list vs force-directed graph) → planner decides; lean toward a lightweight tree/list to avoid a heavy d3/react-flow dependency unless the user wants a true graph.
- **Routing/ambiguity score thresholds:** researcher calibrates from the golden set (analogous to D2-15 / D3-08).
- **Wizard interview step count / field order:** planner designs.
- **`data-redline` exact field set + diff format:** planner defines.
- **`effective_from`/`effective_to` index strategy per backend** (Milvus expr syntax vs Mongo query): researcher validates (open question — same class as the Phase-1 Milvus tier-filter syntax spike).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level (locked decisions that constrain this phase)
- `.planning/PROJECT.md` — Core Value (Skill-driven platform) + Key Decisions (D6 Go-native, D7 Milvus tier PRE-filter, D-09 refusal-by-construction)
- `.planning/REQUIREMENTS.md` — Phase 5 requirements: SYSK-04/05/06, AGNT-06, KNOW-06, CMPL-06/07/08. ⚠️ DATA-01..14 are traced to Phase 5 here but are **out of scope** this phase (ROADMAP-authoritative; see `<deferred>`).
- `.planning/ROADMAP.md` — Phase 5 goal + 8 Success Criteria + pitfalls (P-SKILL-*, P-EINO-1..3, P-RAG-1)
- `.planning/STATE.md` — open question #6 (cross-tier conflict policy → resolved by D5-13); research flag "Phase 5: hot reload for in-flight conversations, cross-tier conflict policy, historical tier storage"

### Schema & Research
- `.planning/SKILL_SCHEMA.md` — SKILL.md v1.0 7-field spec (the SC-3 wizard output gate; `temperature` = SYSK-08)
- `.planning/research/PITFALLS.md` — P-SKILL-* (semantic routing, hot reload, wizard generation), P-EINO-1..3 (multi-Skill orchestration, in-flight hot reload, cross-tier conflict policy), P-RAG-1 (full tier system)
- `.planning/research/STACK.md` + `.planning/research/ARCHITECTURE.md` — locked stack, 6-layer architecture, Eino ADK mapping

### Prior phase context (carry-forward)
- `.planning/phases/02-compliance-vertical-slice/02-CONTEXT.md` — D2-01 grounding, D2-06 Citation struct, D2-07 file-driven registration, D2-13/14 refusal taxonomy, 7-field validator, chunk schema `{id,tier,content,source,embedding}` (SC-8 extends this), refusal-by-construction
- `.planning/phases/03-agentic-retrieval-trust-evaluation/03-CONTEXT.md` — D16 `data-decision` SSE (SC-1 trace + SC-4 inspector substrate), D17 TierEnum helpers, agent loop (`internal/agent`), store.Store + SQLite, D3-08 calibration pattern
- `.planning/phases/04-streaming-trust-ui/04-CONTEXT.md` — D-04/D12 transport-swap invariant, D15 wire drift alarm, `<deferred>` = exactly the Phase-5-overlapping UI items; SkillPicker (reused for D5-03 ambiguity UI); Zustand-bans-messages state boundary

### Spike/sketch findings (auto-loads)
- `.claude/skills/spike-findings-ai-agent/SKILL.md` — temperature + structured-output + refusal-by-construction findings (informs SC-3 wizard temperature logic, D5-12)
- `.claude/skills/sketch-findings-ai-agent/SKILL.md` — three-panel locked layout (SC-4/5 new panels live within it); chat-input `@`-mention (overlaps multi-Skill routing)

### Existing code (reuse targets — see code_context)
- `internal/skill/{loader,manifest,tier,validate}.go` — Skill registration / parsing / 7-field validation (the base SC-1/SC-2/SC-3 extend)
- `internal/embedder/embedder.go` + `internal/embedder/bge/bge.go` — `Embedder` (D5-01 embeds Skill descriptions)
- `internal/agent/{agent,callbacks,retrieve_tool,trust_floor,critique}.go` — Phase-3 agent loop + callbacks (D16 decision emit); SC-4 inspector data source; SC-6 post-verdict pass + SC-7 redline pass insertion points
- `internal/server/{chat,pipeline,server,skills}.go` — `POST /api/chat` + `VerdictPipeline` (D5-05 route-then-run evolution); `/api/skills` (SC-3 wizard endpoint base)
- `internal/wire/{chunk,sse}.go` — `UIMessageChunk` 17-type wire (D5-04 extend data-decision; D5-14 add data-redline → 18; D-04/D12/D15 invariant)
- `internal/retriever/retriever.go` + `internal/retriever/{milvus,mongo}` — tier PRE-filter (D5-15 effective_to filter co-layer); all backends must implement
- `internal/knowledge/indexer.go` — KNOW-02 tier-mandatory enforcement (extends to effective fields)
- `internal/store/store.go` + SQLite — SC-5 graph browser data source
- `web/src/components/**` + `web/src/transport/**` — Phase-4 component tree + transport (SC-4/5/6/7 new panels reuse SkillPicker / VerdictCard / ChunkSourceView)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/skill/{loader,manifest,tier,validate}.go`** — `Manifest` + `ResolvedTiers()` + 7-field validator already exist. SC-1 routing adds description-embedding; SC-2 adds fsnotify watcher; SC-3 wizard's output passes `validate.go`. The Skill model is the single extension nucleus.
- **`internal/embedder/bge`** — proven BGE-M3 `Embedder`; D5-01 reuses it for Skill-description vectors (query embedding path already exists).
- **`internal/agent/callbacks.go`** — already emits `data-decision` (D16) + tool-call + reasoning events. SC-4 inspector re-renders these (no new instrumentation). SC-1 routing trace extends the same chunk (D5-04).
- **`internal/server/pipeline.go`** — `VerdictPipeline` is the route-then-run evolution point (D5-05). SC-6 post-verdict counter-evidence pass + SC-7 conditional redline pass insert here, AFTER the verdict, BEFORE final emit.
- **`internal/wire/{chunk,sse}.go`** — the wire contract. D5-04 extends `data-decision` fields; D5-14 adds `data-redline` (18th type). Both must mirror in TS (`web/src/types`) per D15.
- **`internal/retriever/{milvus,mongo}`** — tier PRE-filter is the D5-15 effective_to filter's home; both backends updated (D11 abstraction).
- **Phase-4 frontend (`web/src/components`)** — `SkillPicker` (→ D5-03 ambiguity picker), `VerdictCard` (→ counter-evidence annotation host), `ChunkSourceView` (→ graph-browser chunk detail). Three-panel layout (sketch-findings) hosts the new panels.

### Established Patterns
- **D-04/D12 transport-swap invariant + D15 wire drift alarm** — every new/changed wire chunk must obey (D5-04, D5-14).
- **D7 tier PRE-similarity metadata filter + D-09 refusal-by-construction** — D5-13 (counter-evidence as annotation) and D5-15 (effective_to filter) must not weaken these. Counter-evidence never enters grounding.
- **D11 / D3-09 interface-first multi-backend** — effective_to filter must cover Milvus + Mongo (and the retriever abstraction's contract).
- **D6 Go-native (no Python sidecar)** — wizard + redline use Go + GLM single-language stack.
- **D2-07 file-driven registration** — wizard write → immediate registration → fsnotify hot-reload = closed loop.
- **D2-15 / D3-08 calibration pattern** — routing/ambiguity thresholds calibrated from the golden set.

### Integration Points
- **`POST /api/chat`** → route-dispatch (SC-1, D5-05) → Skill pipeline → post-verdict counter-evidence pass (SC-6, D5-13) → conditional redline pass (SC-7, D5-14) → emit.
- **New fsnotify watcher** on `skills/*/SKILL.md` (D5-06..09) → 1s debounce → reload `Manifest` → in-flight runs pinned, next run uses new version; validation failure keeps old + reports.
- **New wizard endpoint** (`POST /api/skills/wizard` or extension of `/api/skills`) → guided interview (D5-10) → preview → confirm → write `skills/<name>/SKILL.md` (D5-11).
- **Chunk schema migration** (D5-15): `{id,tier,content,source,embedding}` → `+effective_from/effective_to`. Touches Milvus collection schema, Mongo docs, `internal/knowledge/indexer.go`, retriever PRE-filter expr in both backends.

### Creative Options
- SC-1 "trace shows score + selected Skill" is satisfied for free by D5-04 (cosine IS the score) — no extra instrumentation.
- The "new wire type" budget is deliberately spent on `data-redline` (D5-14) rather than routing (D5-04 extends) — one new type instead of two.
- Counter-evidence (D5-13) and redline (D5-14) are both *post-verdict passes* — they compose naturally as two optional stages after the existing verdict, leaving the verdict's trust floor untouched.
</code_context>

<specifics>
## Specific Ideas

- **Counter-evidence annotation ≠ grounding** is the single most important semantic line this phase draws: 历史-tier contradicting chunks are shown next to the verdict but NEVER feed `GroundingCheck` (D2-01). This is how SC-6 stays compatible with CMPL-04 (reads only 权威) and D-09 (refusal-by-construction).
- **Hot reload closed loop:** wizard (SC-3) writes file → file-driven registration (D2-07) → fsnotify (SC-2) picks it up → next query routes to it (SC-1). Four SCs form a single platform loop.
- **Versioning backward-compat:** `effective_to IS NULL` = perpetually valid, so the existing versionless corpus needs no migration — only new/versioned policy docs populate the fields.
- **Wizard ↔ Phase 7 templates:** the SC-3 wizard and DIST-08's 2-3 Skill templates (compliance/QA/calculator) are same-source — templates can seed the interview.
- **Inspector is a re-presentation, not new plumbing:** SC-4 deliberately reuses the D16 + tool/reasoning events the agent already streams — the work is UI, not a new channel.
</specifics>

<deferred>
## Deferred Ideas

- **DATA-* financial data layer (DATA-01..14)** — user confirmed OUT of scope this phase. Data import / desensitization / aggregation API / external authorized channel / NL2SQL 3-layer safety / approval flow / dual audit. Recommend its own phase or v2. ⚠️ Action: fix the REQUIREMENTS.md traceability drift (DATA-* currently mapped to Phase 5) and the ROADMAP Coverage table inconsistency (55 vs 70 total) — these are documentation drift, not Phase 5 work.
- **CI promotion gate (GEVL-07) / eval dashboard (GEVL-06)** — Phase 6.
- **Open-source packaging (DIST-*)** — Phase 7.
- **Routing/ambiguity score threshold values** — researcher calibrates from golden set (D2-15 pattern).
- **Counter-evidence beyond 权威 vs 历史** (e.g., 个人 / 草稿 conflicts) — v2; this phase is 权威 vs 历史 only.
- **Knowledge-tier reindex hot-reload** (new docs triggering reindex) — NOT SC-2; separate mechanism, Phase 6+.
- **Graph browser viz engine choice** (d3/react-flow vs lightweight tree) — planner decides; lean lightweight.

### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md` (todo.match-phase score 0.6) — reviewed in Phases 2, 3, 4, and now 5: keyword false-match (date/phase/skill/eino) + work already complete in Phase 1. SYSK-08 (temperature required, passed via `WithTemperature`) was implemented in Phase 2's 7-field validator. For Phase 5 it is only a constraint on the SC-3 wizard's output (must pass the validator incl. `temperature`). Not folded — not Phase 5 work.
</deferred>

---
*Phase: 5-Skill-Platform-Full-Knowledge-Boundaries*
*Context gathered: 2026-08-12*
