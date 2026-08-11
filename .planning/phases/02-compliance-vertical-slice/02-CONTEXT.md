# Phase 2: Compliance Vertical Slice - Context

**Gathered:** 2026-08-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 2 delivers one complete compliance-review vertical slice: **one Skill (compliance), one authoritative knowledge tier (权威), one streaming endpoint (`POST /api/chat`)**, that for any query returns either a **verifiable cited verdict** or a **structured refusal** — never an improvised answer.

**This is where real retrieval plugs into the verdict pipeline.** Phase 1's spike e2e kept verdicts and retrieval disconnected — verdicts used synthetic citations, and Milvus retrieval was a separate SC-2 subcommand. Phase 2 connects them: query → embed → tier-filtered Milvus retrieve → inject chunks → LLM verdict → Go programmatic grounding check → emit via SSE.

**Scope anchor:** the "Skill-as-platform" thesis proof. NOT multi-skill routing (Phase 5), NOT autonomous retrieval decisions (Phase 3), NOT UI (Phase 4).

**Requirement coverage:** SYSK-01, SYSK-02, SYSK-07, SYSK-08, CMPL-01..05, AGNT-03, ARAG-04, ARAG-05, GEVL-01, GEVL-02.
</domain>

<decisions>
## Implementation Decisions

### Verdict Generation & Grounding (SC-3, ARAG-04 — P-COMPL-1 moat)
- **D2-01: Go programmatic grounding check.** Retrieval is a pipeline PRE-step (Go embeds query → tier-filtered Milvus search → inject chunks into prompt), NOT an agent-decided tool call (that is Phase 3 ARAG-01). After the LLM emits a JSON verdict, Go code verifies each citation's `span` literally appears in the retrieved chunk text (normalized substring match — see D2-05); ungrounded claims are stripped. Rationale: deterministic, independently unit-testable, and aligns with the "语义确定性正解" philosophy (semantic-determinism via structured output + programmatic grounding + refusal-by-construction). The hallucination moat must NOT rely on the LLM policing itself.
- **D2-02: Ungrounded-claim handling = downgrade to 需补充.** When a claim's citation fails the grounding check, keep the grounded 合规/不合规 conclusion, set `review_required: true`, and cap `confidence < 0.9`. Reuses the existing 需补充 status + review_required flag (CMPL-03). Does NOT silently drop (hides model hallucination attempts — bad for audit) and does NOT nuke the whole verdict on one weak claim (overly aggressive).

### Citation Contract (SC-3, ARAG-04)
- **D2-03: `span` = verbatim quote.** The exact substring from the chunk that supports the claim. Locked by D2-01 (programmatic match requires verbatim). Enables the Phase 4 side-by-side citation panel (CMPL-03/FEUI-03).
- **D2-04: citation object = minimal set `{chunk_id, span, tier}`.** Exactly meets SC-3. `score` and `effective_date` deferred to Phase 3/5 (current Milvus chunk schema `{id, tier, content, source, embedding}` has no `effective_date`; `TierSearchResult.Score` is available but not surfaced in the v2 citation).
- **D2-05: normalized substring match.** The Go verifier normalizes (collapse whitespace + 全角→半角 punctuation + trim) both span and chunk text, then substring-contains. Avoids false-negatives from CN 全角/半角 punctuation mixing while remaining a true exact-substring check — NOT fuzzy/edit-distance (that would erode the moat).
- **D2-06 (schema evolution): upgrade `Verdict.Citations` from `[]string` to structured `[]Citation`.** The Phase 1 `internal/spike/verdict.go` `Citations []string` (clause IDs) becomes `[]Citation{{ChunkID, Span, Tier}}`. `ValidStatuses` (合规/不合规/需补充/unanswered) and `Validate()` are reused.

### Skill Registration & API (SC-1, SYSK-01/02)
- **D2-07: file-driven registration.** `skills/*/SKILL.md` is the single source of truth. Loader scans on startup; `GET /api/skills` lists; `POST /api/skills` is a dry-run validator (validates against the 7-field schema, does NOT persist). Makes "drop a file → it registers" literally true and demo-able; Phase 5 fsnotify hot-reload extends this naturally; matches denova file-based Skills + the SKILL_SCHEMA "pure declarative Markdown" philosophy.
- **D2-08: query endpoint = `POST /api/chat`.** Skill specified in request body (Phase 2 default = compliance). Future-proofs Phase 5 SYSK-05 semantic routing (URL stable, routing layer added in front). Aligns with Vercel AI SDK v5 `useChat` default endpoint + AGNT-03 SSE streaming.
- **D2-09: new `cmd/server` Hertz long-running service.** `cmd/spiked` retained as the regression/spike binary. Phase 2 introduces the real server. Reusable spike assets graduate into production packages (see code_context).
- **D2-10: 7-field schema (with `temperature`).** ROADMAP SC-1 text says "6-field" (stale). Phase 2 implements `.planning/SKILL_SCHEMA.md` v1.0 **7-field** schema incl. `temperature` (SYSK-08). The 4 spike-era `testdata/skills/*/SKILL.md` fixtures (with `reasoning: false`, inline `reads: ["权威"]`, `guardrails.refuse_on_missing_tier`) must be reconciled to the 7-field spec.

### Compliance Corpus (enables SC-2/SC-3)
- **D2-11: hand-authored small authoritative corpus.** 5–10 policy docs / ~30 chunks covering the spike's 5 queries (差旅 / 竞业 / 合同审批 / 年假 / 联系方式拒答), including the intentional refusal case (contact-info query → out-of-scope / personal tier → SC-2/SC-4 refusal path). Deterministic → eval reproducibility (personal-brand "measurable" goal), safe (no real-regulation licensing/accuracy risk), demo-able. Phase 3 expands to 50 cases (GEVL-05). Reuse the existing `auth_001.md` travel-expense clause.
- **D2-12: authoritative corpus at `knowledge/authority/*.md` (tier = top-level dir).** `skills/compliance/` holds only `SKILL.md` and declares `reads: authority/**`. Indexer scans `knowledge/{tier}/` into Milvus with `tier` metadata. Continues denova true-source separation: multiple Skills sharing one 权威 tier don't duplicate. Phase 7 DIST-06 packages samples from here. Migrate/reconcile existing `testdata/docs/` into `knowledge/`.

### Refusal Taxonomy & Thresholds (ARAG-05, GEVL-01) — Claude's discretion
- **D2-13 (discretion): refusal reason enum** = `{no_authorized_source, low_retrieval_score, out_of_scope}`. `no_authorized_source` pinned by SC-2; `low_retrieval_score` from GEVL-01; `out_of_scope` for queries outside the Skill's domain (e.g., contact-info). Structured refusal shape `{reason, can_user_rephrase: bool, suggested_alternative_query: string}` per ARAG-05.
- **D2-14 (discretion, AMENDED during planning): `suggested_alternative_query` = rule-based only in Phase 2** (rephrase to include the tier keyword, e.g. 权威制度). The original "LLM-generated + rule-based fallback" is structurally impossible on the refusal path: SC-2 / ARAG-05 refuses BEFORE any LLM call (refusal-by-construction, D-09), so there is no LLM in-flight to generate a suggestion. LLM-generated suggestions for non-refusal cases are deferred to Phase 5 polish. *(Plan-checker warning #3, resolved.)*
- **D2-15 (discretion): retrieval-score refusal threshold** — STATE.md open question #4 flags thresholds as needing empirical calibration. For Phase 2 use a conservative placeholder (e.g., cosine < 0.3 → `low_retrieval_score` refusal) and mark it for Phase 3 eval calibration. The `review_required` confidence threshold is pinned at `< 0.9` by SC-5.

### Claude's Discretion
- Exact package layout for graduating spike assets (e.g., `internal/verdict`, `internal/wire`, `internal/retriever`, `internal/skill`, `internal/server`) — planner decides; keep `cmd/spiked` runnable.
- How `reads: authority/**` resolves to the Milvus tier value `权威` (loader mapping) — planner/researcher decides; the tier enum is 权威/历史/个人/草稿.
- Whether the grounding check also treats a "novel claim" (claim with no citation at all) via the same D2-02 downgrade path (recommend: yes).
- Exact 5-query compliance set (suggest: reuse spike 002/003's 5 queries for comparability).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level
- `.planning/PROJECT.md` — Core Value (Skill 可插拔平台) + Key Decisions D1–D10 (esp. D7 Milvus tier metadata, D8 citation = chunk ID + span, D9 refusal-by-construction)
- `.planning/REQUIREMENTS.md` — Phase 2 requirements: SYSK-01/02/07/08, CMPL-01..05, AGNT-03, ARAG-04/05, GEVL-01/02
- `.planning/ROADMAP.md` — Phase 2 goal + 6 Success Criteria (⚠️ SC-1 "6-field" is stale → implement 7-field per D2-10)
- `.planning/STATE.md` — Open questions #2 (CN compliance dataset), #4 (thresholds need calibration)

### Schema (locked contract)
- `.planning/SKILL_SCHEMA.md` — SKILL.md v1.0 **7-field** spec (description / agent / reads / writes / steps / guardrails / **temperature**). The Phase 2 validator (SYSK-02) enforces this; `reads` patterns map to Milvus tier values.

### Research (validated choices)
- `.planning/research/STACK.md` — locked stack
- `.planning/research/ARCHITECTURE.md` — 6-layer system; Eino ADK mapping
- `.planning/research/PITFALLS.md` — P-COMPL-1 (compliance hallucination with fake citations — TOP risk, addressed by D2-01/D2-05), P-SKILL-1..6, P-RAG-2 (tier bleed)

### Prior phase context
- `.planning/phases/01-integration-spike-trust-contract/01-CONTEXT.md` — Phase 1 decisions D-01..D-12 (esp. D-08 BGE-M3, D-09 Milvus default-deny, verdict schema proven in spike 003, refusal-by-construction proven, semantic-determinism philosophy)

### Spike findings (validated patterns — auto-loads)
- `.claude/skills/spike-findings-ai-agent/SKILL.md` — findings index
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md` — Coding Plan endpoint + `WithExtraFields` + structured-output patterns (Eino v0.1.13+)
- `.claude/skills/spike-findings-ai-agent/sources/003-structured-output-temperature-zero/main.go` — proven structured-output verdict pattern (reuse for Phase 2 verdict generation)

### Existing code (reuse targets — see code_context)
- `internal/spike/verdict.go` — Verdict struct + ValidStatuses + Validate() (upgrade Citations per D2-06)
- `internal/spike/uimsg.go` — UIMessageChunk 16-type wire format (reuse for AGNT-03 SSE)
- `internal/spike/milvus_client.go` — BuildMilvusFilter + SearchWithTierFilter + EnsureCollection + SeedDocuments + EmbedQuery (reuse for SYSK-07/GEVL-02)
- `cmd/spiked/e2e.go` — proven ChatModelAgent + Runner + structured-output pipeline (the template for the production verdict path)
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (graduate from `internal/spike/` to production packages)
- `internal/spike/verdict.go` → production verdict package: `Verdict` struct, `ValidStatuses` (合规/不合规/需补充/unanswered), `Validate()`. **Upgrade `Citations []string` → `[]Citation{ChunkID, Span, Tier}` per D2-06.**
- `internal/spike/uimsg.go` → production wire package: `UIMessageChunk` (16 kebab-case types incl. `data-policy`, `source-document`), `ValidTypes`, `IsValidType()`. Reuse directly for AGNT-03 SSE; the `source-document` chunk type carries each citation `{chunk_id, span, tier}` to the wire, `data-policy` carries the Verdict.
- `internal/spike/milvus_client.go` → production retriever package: `BuildMilvusFilter` (default-deny `"false"` on empty reads — SYSK-07), `SearchWithTierFilter` (PRE-similarity filter), `EnsureCollection`, `SeedDocuments`, `EmbedQuery` / `EmbedQueryDeterministic`. Collection schema `{id, tier, content, source, embedding}` is the Phase 2 chunk schema (no `effective_date` — deferred to Phase 5 CMPL-08).
- `cmd/spiked/e2e.go` — the proven verdict pipeline template: `openai.NewChatModel` → `adk.NewChatModelAgent` → `adk.Runner.Query` → drain iterator → parse Verdict JSON → `Validate`. `WithChatModelOptions(model.WithTemperature(0), WithExtraFields{response_format:json_object, thinking:disabled})` is the exact call shape (eino v0.7.7/adk/call_option.go:54). Phase 2 inserts the retrieval pre-step + the Go grounding check.

### Established Patterns
- **temperature 0.0 + `response_format:{type:json_object}` + `thinking:{type:disabled}`** via `WithExtraFields` + `WithChatModelOptions` — proven in spike 003/e2e. Compliance Skill MUST pass these (SYSK-08).
- **default-deny tier filter**: `BuildMilvusFilter([])` returns `"false"` → Milvus returns 0 rows → refusal-by-construction (D-09). The compliance Skill `reads: [authority]` → filter `tier in ["权威"]` (loader resolves `authority/**` → tier `权威`).
- **GLM-5.2 graceful refusal**: returns `status:"unanswered"`, confidence=0, citations=[] when no policy context (spike 002/003) — aligns with the SC-2 refusal path.
- **Internal-network strategy**: Milvus + BGE-M3 via `MILVUS_ADDRESS` / `BGE_M3_URL` (内网), GLM via public Coding Plan endpoint `https://open.bigmodel.cn/api/coding/paas/v4`, `GLM_API_KEY` env var. No Docker (Phase 7 adds docker-compose).

### Integration Points
- **New `cmd/server`** (Hertz) → routes `POST /api/chat` (stream SSE), `GET /api/skills` (list), `POST /api/skills` (dry-run validate). Reuses `pkg/protocol/sse` + `UIMessageChunk` emission.
- **Skill loader** → scans `skills/*/SKILL.md` at startup, parses 7-field YAML frontmatter, validates (SYSK-02), resolves `reads` patterns → tier values for the retriever.
- **Indexer** → scans `knowledge/{authority,historical,personal,draft}/*.md`, embeds via BGE-M3, inserts into Milvus with `tier` metadata (migrates `testdata/docs/` → `knowledge/`).
- **Verdict pipeline** (the Phase 2 core): `POST /api/chat` → embed query → `SearchWithTierFilter(tier in ["权威"])` → 0 chunks ⇒ structured refusal (SC-2) → else inject chunks into prompt → ChatModelAgent verdict (temp=0, json_object) → Go grounding check (D2-01) → downgrade ungrounded (D2-02) → emit SSE (text / source-document / data-policy chunks, AGNT-03).

### Creative Options
- The `data-policy` UIMessageChunk custom type (already whitelisted) is the canonical wire carrier for the compliance Verdict — distinct from `source-document` (carries each citation).
- The grounding check can additionally catch "novel claim" (a claim with no citation at all) via the same D2-02 downgrade path.
</code_context>

<specifics>
## Specific Ideas

- Cover the spike's exact 5 queries (差旅报销超标 / 竞业限制 / 合同审批权限 / 年假折算 / 联系方式索取) for direct comparability with spike 002/003 results — the contact-info query is the intentional refusal case (SC-2/SC-4).
- The hand-authored corpus should include **overlapping clauses** (e.g., two policies giving different standards for the same expense) to stress-test the grounding + `review_required` path — but counter-evidence surfacing itself is Phase 5 CMPL-06.
- Keep `cmd/spiked` runnable for regression: the `wire` / `milvus` / `sse` / `e2e` subcommands remain the fast contract-check path alongside the new server.
</specifics>

<deferred>
## Deferred Ideas

- **Multi-skill semantic routing** (SYSK-05) — Phase 5. Phase 2 has one Skill; `POST /api/chat` default-skills to compliance, routing layer added later.
- **Autonomous retrieval decisions / multi-hop / full Self-RAG** (ARAG-01/02/03) — Phase 3. Phase 2 retrieval is a fixed pre-step, not agent-decided.
- **Streaming Trust UI** (FEUI-*) — Phase 4. Phase 2 is curl/test-driven (no UI); SSE events are emitted to the wire but no React consumer beyond the spike stub.
- **Policy version tracking / counter-evidence / redline** (CMPL-06/07/08) — Phase 5. Chunk schema stays `{id, tier, content, source, embedding}` (no `effective_from/to`) in Phase 2.
- **Ragas eval pipeline + 50-case golden set + audit SQL** (GEVL-03/04/05) — Phase 3/6. Phase 2 has only the refusal-by-construction gate (GEVL-01/02), not the full eval sidecar.
- **Hot-reload** (SYSK-04, fsnotify) — Phase 5. Phase 2 loads Skills at startup only.

### Reviewed Todos (not folded)
- **`spike-temperature-field-validation.md`** (score 0.6 — keyword false-positive on field/status/phase/skill/api) — H1+H2+H4 temperature-field validation. **Already complete** (folded + executed in Phase 1 per `01-CONTEXT.md`). Not Phase 2 work. SYSK-08 (temperature required, passed via `WithTemperature`) is the Phase 2 carry-forward — implemented as part of the 7-field validator + verdict pipeline, not re-spiked.
</deferred>

---
*Phase: 2-Compliance-Vertical-Slice*
*Context gathered: 2026-08-11*
