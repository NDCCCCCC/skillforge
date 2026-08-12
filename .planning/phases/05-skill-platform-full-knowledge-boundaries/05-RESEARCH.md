# Phase 5: Skill Platform & Full Knowledge Boundaries - Research

**Researched:** 2026-08-12
**Domain:** Multi-Skill semantic routing + fsnotify hot-reload + Skill Wizard generation + post-verdict annotation (counter-evidence / redline) + policy-version PRE-filter + agent-state UI surfacing
**Confidence:** HIGH (most findings verified against the live codebase + official Milvus/fsnotify docs; one Milvus nullable-field edge case is MEDIUM and mitigated by a sentinel-value recommendation)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Semantic Routing (SC-1, SYSK-05) — D5-01..05**
- **D5-01:** BGE-M3 embedding routing. Reuse `internal/embedder/bge` to embed each Skill's `name`+`description`; query embedding cosine-similarity picks the Skill. Score = cosine. Chosen over LLM intent-classifier and hybrid.
- **D5-02:** in-memory map for Skill embeddings. Embed all Skills at startup; hot-reload re-embeds only the changed Skill. N<100 makes Milvus overkill.
- **D5-03:** ambiguity → ask the user. top-1 low OR top-1/top-2 close → UI surfaces candidates (reuses Phase-4 `SkillPicker`).
- **D5-04:** routing trace = extend D16 `data-decision`. Add `skill_scores[]` + `selected` fields. Does NOT touch the wire type count (preserves D-04/D12 + D15). New-type budget reserved for redline (D5-14).
- **D5-05:** routing sits BEFORE ADK. Go code dispatches then runs the Skill's pipeline. `internal/server/pipeline.go` evolves into "route-then-run". Chosen over `NewConditionalAgent` (heavier refactor).

**Hot Reload (SC-2, SYSK-04) — D5-06..09**
- **D5-06:** in-flight runs are version-pinned. A run in flight at file-change keeps the pre-change snapshot; fsnotify only updates the version used by the *next* new run.
- **D5-07:** each turn resolves the current latest version (no session-level pinning).
- **D5-08:** validation failure keeps the old version + reports (SSE/log error). Never silently drops a working Skill.
- **D5-09:** reload scope = Skill definition only. `reads` changes absorbed by per-run tier resolution. Knowledge reindex is NOT SC-2.

**Skill Wizard (SC-3, SYSK-06) — D5-10..12**
- **D5-10:** guided multi-turn interview. Walks user through purpose / `reads` tiers / type / guardrails / temperature, then emits `SKILL.md`. Chosen over one-shot and template-fill.
- **D5-11:** preview → confirm → write to `skills/<name>/SKILL.md` + register. File-driven registration (D2-07) → handed to fsnotify (SC-2) — closed loop.
- **D5-12:** temperature recommended by Skill type. Compliance/review → 0.0; QA → ~0.2; creative → 0.7+. User can override. Reuses SYSK-08 semantics.

**Counter-evidence + Redline + Policy Version (SC-6/7/8, CMPL-06/07/08) — D5-13..15**
- **D5-13 (SC-6):** counter-evidence = platform-level post-verdict pass. Verdict STILL reads only 权威 (CMPL-04 / D-09 unchanged). After verdict, an independent retrieval pass over 历史 tier surfaces contradicting chunks as **annotation**; NEVER participates in grounding (D2-01).
- **D5-14 (SC-7):** redline = independent LLM pass on policy-gap + new `data-redline` chunk. Only when verdict flags a gap (`不合规`/`需补充`). Wire = new `data-redline` chunk (`original_clause` + `proposed_rewrite` + `rationale`); wire `ValidTypes` 17→18 + TS mirror per D15. No gap → no call.
- **D5-15 (SC-8):** extend chunk schema + retriever PRE-filter. Chunk schema gains optional `effective_from`/`effective_to` (null `effective_to` = perpetually valid). Retriever PRE-filter sits at the SAME layer as the tier filter (D7 extension). All backends updated (Milvus + Mongo).

### Claude's Discretion
- **SC-4 (AGNT-06) Agent State Inspector data source:** reuse the existing D16 `data-decision` + tool-call + reasoning SSE events emitted by `internal/agent/callbacks.go` — the inspector is a new UI panel that re-renders these as a structured tree, NOT a new instrumentation channel.
- **SC-5 (KNOW-06) Knowledge graph browser:** render tier → doc → chunk within the locked three-panel layout using `store.Store` + knowledge metadata. Visualization engine → planner decides; lean lightweight tree/list (avoid d3/react-flow).
- **Routing/ambiguity score thresholds:** researcher calibrates from the golden set (analogous to D2-15 / D3-08).
- **Wizard interview step count / field order:** planner designs.
- **`data-redline` exact field set + diff format:** planner defines.
- **`effective_from`/`effective_to` index strategy per backend** (Milvus expr vs Mongo query): researcher validates.

### Deferred Ideas (OUT OF SCOPE)
- **DATA-* financial data layer (DATA-01..14)** — user confirmed OUT of scope. ⚠️ Action: fix REQUIREMENTS.md traceability drift (DATA-* mapped to Phase 5) + ROADMAP Coverage table inconsistency (55 vs 70). Documentation drift, not Phase 5 work.
- **CI promotion gate (GEVL-07) / eval dashboard (GEVL-06)** — Phase 6.
- **Open-source packaging (DIST-*)** — Phase 7.
- **Routing/ambiguity threshold values** — calibrated from golden set (researcher recommends mechanism; values empirical).
- **Counter-evidence beyond 权威 vs 历史** — v2; this phase is 权威 vs 历史 only.
- **Knowledge-tier reindex hot-reload** — NOT SC-2; Phase 6+.
- **Graph browser viz engine choice** — planner decides; lean lightweight.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SYSK-04 | System hot-reloads Skills on file change (fsnotify) | §Hot-Reload Pattern — fsnotify v1.5.4 already transitive in go.mod; debounce + atomic.Pointer swap; in-flight version pinning via `atomic.Pointer[*Manifest]` (D5-06). Loader docstring already says "Phase 5 extends this with fsnotify hot-reload". |
| SYSK-05 | System routes queries to the right Skill via intent matching on `description` | §Routing Pattern — reuse `internal/embedder/bge` (D5-01) + the existing `cosine()` from `internal/retriever/mongo/rank.go:63`. In-memory map (D5-02). Trace extends `data-decision` (D5-04) — `skill_scores[]` + `selected` fields already within DecisionPayload's extension budget. |
| SYSK-06 | Skill Wizard auto-generates a compliant `SKILL.md` from NL | §Wizard Pattern — guided multi-turn interview (D5-10); output MUST pass existing `skill.Validate()` (7-field). Temperature passthrough + structured output validated in spikes 001/003. Write → D2-07 → fsnotify closed loop (D5-11). |
| AGNT-06 | User can inspect live agent state via the UI | §Inspector Pattern — pure UI re-render of D16 decision + tool/reasoning events already emitted by `internal/agent/callbacks.go` `newDecisionCallback`. No new instrumentation channel (Claude's discretion, confirmed). |
| KNOW-06 | User can browse the knowledge graph (tier+doc+chunk) in the UI | §Graph Browser Pattern — `store.Store` + retriever Hit metadata + `skill.TierEnum` (D17). Lightweight tree within three-panel layout. `source-document` wire chunks already carry chunk content. |
| CMPL-06 | Counter-evidence highlighting — surface chunks that contradict the verdict | §Counter-evidence Pattern — POST-verdict pass (D5-13) inserts after `emitVerdictStream` in `pipeline.go:232`. Second `Retriever.Search(ctx, qvec, ["历史"], k)` call. NEVER enters `GroundingChecker`. Reuse `source-document` wire type (no new type). |
| CMPL-07 | Redline suggestion — clause-level rewrites on policy gap | §Redline Pattern — conditional LLM pass only on `不合规`/`需补充` (D5-14). New `data-redline` wire chunk (17→18, D15 TS mirror). Cost-controlled (no gap → no call). |
| CMPL-08 | Policy version tracking — `effective_from`/`effective_to`; superseded filtered | §Policy-Version Pattern — chunk schema gains `EffectiveFrom`/`EffectiveTo` (D5-15). **Sentinel `0` = perpetually valid** (recommendation; see Milvus nullable-field pitfall). PRE-filter co-layered with tier filter in both `milvus/filter.go` and `mongo/mongo.go`. |
</phase_requirements>

## Summary

Phase 5 promotes SkillForge from a single-Skill compliance demo into a true Skill-driven platform. Eight success criteria span five capability themes, but they decompose into a surprisingly small set of extension points on already-shipped code: (1) the `SKILL.md` loader/manifest/validator (`internal/skill/*`), (2) the wire contract (`internal/wire/chunk.go` + TS mirror), (3) the verdict pipeline (`internal/server/pipeline.go`), (4) the retriever abstraction + two backends (`internal/retriever/*`), and (5) the agent callbacks (`internal/agent/callbacks.go`). The phase is **extension, not greenfield** — every SC has a named reuse target in the existing codebase.

The single most important architectural line this phase draws is **"counter-evidence is annotation, not grounding"** (D5-13). The verdict's trust floor (CMPL-04 reads-only-权威 + D-09 refusal-by-construction + D2-01 GroundingChecker) MUST remain untouched. The 历史-tier contradicting chunks are surfaced *next to* the verdict via a separate post-verdict retriever pass that is structurally forbidden from feeding `GroundingChecker`. This is how SC-6 stays compatible with the existing trust moat — and the planner must encode this boundary in code structure (separate function, separate chunk map), not in prompt discipline.

The one genuine technical landmine is D5-15's "null `effective_to` = perpetually valid" design. **Milvus documents that NULL values always evaluate to FALSE in filter expressions** `[CITED: milvus.io/docs/nullable-and-default.md]`, which breaks the naive `effective_to IS NULL OR effective_to >= today` expression (NULL rows get filtered out by the `>= today` arm before the `IS NULL` arm can rescue them, and nullable-field OR-rewrite has documented edge cases). The safe, cross-backend recommendation is a **sentinel value** (`effective_to == 0` = perpetually valid, stored as Int64 epoch seconds): it avoids nullable fields entirely on both Milvus and Mongo, is fully backward-compatible with the existing versionless corpus (default 0), and makes the PRE-filter a clean boolean `and (effective_to == 0 or effective_to >= today)`. This is a deviation from CONTEXT.md's literal "null" wording but preserves its *semantic* intent (perpetually valid) at lower risk; it should be flagged to the planner as a researcher recommendation.

**Primary recommendation:** Treat the phase as four composable loops bolted onto existing substrates — (A) route-then-run evolution of `VerdictPipeline`, (B) fsnotify watcher + atomic-pointer registry on `Loader`, (C) post-verdict annotation passes (counter-evidence + conditional redline) inserted after `emitVerdictStream`, and (D) chunk-schema + PRE-filter extension for effective dates. No new external packages are required (fsnotify is already transitive; cosine for routing is already implemented in mongo/rank.go).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Semantic Skill routing (SC-1) | API / Backend (Go) | — | Routing selects the Skill BEFORE ADK runs (D5-05). Pure Go: embed query + cosine against in-memory Skill-description map. Must sit in front of `VerdictPipeline`. |
| Routing trace surfacing (SC-1 score) | API / Backend → Wire → Browser | — | Score is the cosine (D5-04 extends D16 `data-decision`). Go emits; React renders. No browser-side computation. |
| fsnotify hot-reload (SC-2) | API / Backend (Go) | OS (filesystem) | Watcher is a server-side goroutine. Version pinning is a Go concurrency concern (`atomic.Pointer`). Browser only sees the result via `/api/skills` + SSE errors (D5-08). |
| Skill Wizard generation (SC-3) | API / Backend (Go + LLM) | Browser (interview UI) | The interview is multi-turn chat (D5-10); the LLM call + `SKILL.md` write + validator gate are all backend. Browser is the interview surface. |
| Agent State Inspector (SC-4) | Browser | API / Backend (already emits) | Pure re-presentation of D16/tool/reasoning events already streaming. No new backend instrumentation (Claude's discretion). |
| Knowledge graph browser (SC-5) | Browser | API / Backend (data source) | Tree rendered in the locked three-panel layout from `store.Store` + retriever metadata. Backend exposes data; browser renders hierarchy. |
| Counter-evidence annotation (SC-6) | API / Backend (Go) | Browser (annotation card) | The post-verdict 历史-tier retrieval pass is backend logic (D5-13); the annotation card is browser. **Critical:** the retrieval happens backend-side but the result is annotation-only, structurally segregated from grounding. |
| Redline rewrite (SC-7) | API / Backend (Go + LLM) | Browser (redline card) | Conditional LLM pass backend-side (D5-14); `data-redline` wire chunk streams to browser. No gap → no call (cost control is backend). |
| Policy-version PRE-filter (SC-8) | Database / Storage | API / Backend (filter builder) | The `effective_to` filter is applied at the Milvus/Mongo SDK layer PRE-similarity (D5-15), same layer as the tier filter. Backend builds the expr; storage evaluates it. |
| Chunk schema migration (SC-8) | Database / Storage | API / Backend (indexer) | `retriever.Document` gains fields; Milvus collection schema + Mongo docs + `knowledge.Index` all change. KNOW-02 enforcement extends to effective fields. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/fsnotify/fsnotify` | v1.5.4 (already transitive in go.mod) | Filesystem watcher for SC-2 hot-reload | Cross-platform (incl. Windows — verified `[CITED: pkg.go.dev/github.com/fsnotify/fsnotify]`); the canonical Go file-watcher; already pulled in by hertz. **Promote from indirect to direct require.** `[VERIFIED: go.mod line "github.com/fsnotify/fsnotify v1.5.4 // indirect"]` |
| `github.com/cloudwego/eino` | v0.7.7 (locked) | Agent framework — Wizard LLM call, redline LLM pass | Already the agent substrate. Wizard + redline reuse the same `ChatModel` + `GenerateOption(temperature)` path validated in spike 001. `[VERIFIED: go.mod]` |
| `github.com/cloudwego/hertz` | v0.10.6 (locked) | HTTP + SSE emission | Already the server. New endpoints (`/api/skills/wizard`) + new wire chunks reuse `pkg/protocol/sse`. `[VERIFIED: go.mod]` |
| `github.com/milvus-io/milvus-sdk-go/v2` | v2.4.2 (locked) | Vector DB client (effective_to PRE-filter) | Already the Milvus backend. D5-15 extends the filter expr, not the SDK. `[VERIFIED: go.mod]` |
| `go.mongodb.org/mongo-driver/v2` | v2.8.0 (locked) | Mongo client (effective_to PRE-filter) | Already the Mongo backend (the working one — internal Milvus is broken per D11). D5-15 adds `$or` clause. `[VERIFIED: go.mod]` |
| `internal/embedder/bge` | (in-repo) | BGE-M3 embedder for Skill-description vectors (D5-01) | Proven in Phase 2. Query-embedding path already exists. `[VERIFIED: internal/embedder/bge/bge.go]` |
| `internal/retriever/mongo` `cosine()` | (in-repo, rank.go:63) | Cosine similarity for routing score | Hand-rolled float32 cosine, already battle-tested for the Mongo backend ranking. Reuse verbatim for D5-01 — N<100 Skills makes gonum/BLAS overkill. `[VERIFIED: internal/retriever/mongo/rank.go lines 63-82]` |
| React 19 + Vite + TS + Vercel AI SDK v5 + shadcn/Tailwind v4 | (locked, web/) | New UI panels (inspector / graph browser / counter-evidence / redline) | Already shipped in Phase 4. New panels live within the locked three-panel layout (sketch-findings). `[VERIFIED: web/src/components/layout/]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sync/atomic` (`atomic.Pointer[T]`) | Go stdlib (1.19+, project is 1.25) | Lock-free publication of reloaded Skill manifests to concurrent readers (D5-06) | The in-flight version-pinning mechanism. Readers Load() the snapshot once per run; the watcher Store()s the new version after debounce. `[VERIFIED: Go stdlib]` |
| `time.Timer` (reset-on-event) | Go stdlib | Debounce fsnotify events (SC-2's 1s window) | Canonical fsnotify debounce (issue #553): reset the timer on each event; fire reload only when the timer finally expires. `[CITED: github.com/fsnotify/fsnotify/issues/553]` |
| `internal/store.Store` + SQLite | (in-repo, D18) | SC-5 graph browser data source (tier/doc/chunk metadata) | Already ships audit + session. Graph browser reads knowledge metadata through the same abstraction. `[VERIFIED: internal/store/]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `fsnotify` (SC-2) | `watcher`/`hawk`/polling | fsnotify is already transitive + the de facto Go standard + Windows-supported. No reason to swap. |
| hand-rolled `cosine()` (D5-01) | `gonum`/BLAS | gonum adds a heavy dep for N<100 vectors. The existing float32 cosine is bit-exact across platforms and already in use. `[VERIFIED: internal/retriever/mongo/rank.go:63]` |
| in-memory Skill-embedding map (D5-02) | Milvus for Skill vectors | N<100 makes Milvus operationally overkill (CONTEXT.md D5-02 locked this). |
| Lightweight tree (SC-5 graph browser) | d3 / react-flow | CONTEXT.md leans lightweight (Claude's discretion). d3/react-flow add bundle weight + complexity for a 3-level hierarchy. Planner decides — flag for user if a true graph is desired. |
| Sentinel `effective_to == 0` (D5-15) | Nullable field `effective_to IS NULL` | **Researcher recommends sentinel** — Milvus nullable fields have a documented "NULL evaluates to FALSE" rule that complicates the OR-filter, plus nullable-OR-rewrite edge cases. Sentinel avoids all of it. See §Pitfall 2. `[CITED: milvus.io/docs/nullable-and-default.md]` |

**Installation:**
```bash
# No new packages to install. fsnotify v1.5.4 is already in go.mod (indirect via hertz).
# Action: promote fsnotify from indirect to direct require (edit go.mod require block),
# then `go mod tidy`. Verify with:
go list -m github.com/fsnotify/fsnotify
```

**Version verification:** (all confirmed via `go list -m` against the project's go.mod)
```bash
go list -m github.com/fsnotify/fsnotify       # v1.5.4 (already present, indirect)
go list -m github.com/cloudwego/eino          # v0.7.7
go list -m github.com/cloudwego/hertz         # v0.10.6
go list -m go.mongodb.org/mongo-driver/v2     # v2.8.0
go version                                     # go1.25.0 windows/amd64
```

## Package Legitimacy Audit

> **slopcheck was unavailable** at research time (`pip install slopcheck` failed — no network or package not installable in this env). Per the Package Legitimacy Protocol's graceful-degradation rule, every recommended package below is tagged `[ASSUMED]` and the planner MUST gate each *new* install behind a `checkpoint:human-verify` task. **However, this phase installs ZERO new external packages** — every recommendation is already present in `go.mod` (verified). The only action is promoting `fsnotify` from `indirect` to direct, which does not require downloading anything new.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `github.com/fsnotify/fsnotify` | go (Go module proxy) | ~10 yrs (v1.5.4) | massive (std-grade) | github.com/fsnotify/fsnotify | n/a (unavailable) | **Already in go.mod (indirect)** — no install needed. Tagged `[ASSUMED]` per slopcheck-unavailable rule, but provenance is the project's own lockfile. Promote to direct require. |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
**New external packages introduced by this phase:** **ZERO** — all recommendations reuse existing go.mod entries or in-repo code.

*If slopcheck was unavailable at research time, all packages above are tagged `[ASSUMED]` and the planner must gate each install behind a `checkpoint:human-verify` task.* → The only candidate (fsnotify) is already a transitive dependency verified present in the project's own `go.mod`, so the human-verify gate is satisfied by `go list -m github.com/fsnotify/fsnotify` returning `v1.5.4`.

## Architecture Patterns

### System Architecture Diagram

The diagram traces the primary use case (a multi-Skill routed query with counter-evidence + redline) from request to streamed response. The four SC-loops (A: route-then-run, B: hot-reload, C: post-verdict annotation, D: policy-version filter) are shown as numbered overlays on the existing pipeline.

```
                          ┌─────────────────────────────┐
  POST /api/chat ───────▶ │  Hertz chat handler         │
  {query, session_id}     │  (internal/server/chat.go)  │
                          └──────────────┬──────────────┘
                                         │
                          ┌──────────────▼──────────────┐
                          │  ROUTE (SC-1, D5-05) ◄── Loop A
                          │  embed query (BGE-M3)        │
                          │  cosine vs Skill desc map    │ ◄─ (D5-02 in-mem map:
                          │  → selected Skill + scores   │     atomic.Pointer refresh
                          └──────────────┬───────────────┘     by fsnotify watcher)
                                  ambiguous? ──yes──▶ SkillPicker UI (D5-03)
                                         │ no
                          ┌──────────────▼──────────────┐
                          │  VerdictPipeline (existing) │
                          │  tier PRE-filter (D7)       │
                          │  + effective_to PRE-filter  │ ◄── Loop D (SC-8, D5-15)
                          │    (effective_to==0 OR      │     sentinel 0 = perpetual
                          │     effective_to>=today)    │
                          │  retrieve → ground → verdict│
                          └──────────────┬──────────────┘
                                         │ verdict (合规/不合规/需补充)
                          ┌──────────────▼──────────────┐
                          │  emitVerdictStream (exists) │
                          │  source-doc + data-policy   │
                          └──────────────┬──────────────┘
                                         │
                          ┌──────────────▼──────────────┐
                          │  POST-VERDICT PASSES ◄── Loop C
                          │  ┌────────────────────────┐ │
                          │  │ Counter-evidence (SC-6)│ │  2nd Search(tiers=["历史"])
                          │  │  D5-13: annotation,    │ │  → source-doc chunks
                          │  │  NOT grounding         │ │  (NEVER enters
                          │  │  → emit annotation     │ │   GroundingChecker)
                          │  └────────────────────────┘ │
                          │  ┌────────────────────────┐ │  only if verdict flags gap
                          │  │ Redline (SC-7, D5-14)  │ │  (不合规/需补充):
                          │  │  LLM pass → data-      │ │  independent LLM call,
                          │  │  redline chunk (18th   │ │  cost-controlled
                          │  │  wire type)            │ │  (no gap → no call)
                          │  └────────────────────────┘ │
                          └──────────────┬──────────────┘
                                         │ SSE stream of UIMessageChunks
                          ┌──────────────▼──────────────┐
                          │  React UI (Phase-4 tree)    │
                          │  + new panels:              │
                          │    Inspector (SC-4)         │ ◄── reads D16 data-decision
                          │    Graph browser (SC-5)     │     + tool/reasoning events
                          │    Counter-evidence card    │     already in the stream
                          │    Redline card             │
                          └─────────────────────────────┘

  LOOP B (SC-2, hot-reload, parallel):               LOOP A (routing trace):
  ┌────────────────────────────────┐                 ┌──────────────────────────┐
  │ fsnotify watcher goroutine     │                 │ data-decision chunk      │
  │  watches skills/*/  (DIR!)     │                 │  extended (D5-04):       │
  │  → time.Timer 1s debounce      │                 │  + skill_scores[]        │
  │  → re-parse + Validate         │                 │  + selected              │
  │  → atomic.Pointer.Store(new)   │                 │ (no new wire type —      │
  │  in-flight runs keep old snap  │                 │  preserves D-04/D12/D15) │
  │  next run Loads() new version  │                 └──────────────────────────┘
  └────────────────────────────────┘
```

### Recommended Project Structure
```
internal/
├── skill/
│   ├── loader.go          # EXTEND: add Watch(ctx) (fsnotify goroutine + debounce)
│   ├── registry.go        # NEW: atomic.Pointer[map[string]*Manifest] + versioning (D5-06)
│   ├── router.go          # NEW: embed query vs Skill-desc map, cosine, ambiguity (D5-01/02/03)
│   ├── wizard.go          # NEW: guided interview → Validate → write SKILL.md (D5-10/11/12)
│   ├── manifest.go        # EXTEND: DescriptionVec []float32 (cached) + version tag
│   └── validate.go        # UNCHANGED (the 7-field gate the wizard output must pass)
├── embedder/bge/          # UNCHANGED (D5-01 reuses Embed())
├── retriever/
│   ├── retriever.go       # EXTEND: Document gains EffectiveFrom/EffectiveTo (D5-15)
│   ├── milvus/filter.go   # EXTEND: BuildMilvusFilter (+effective_to clause) (D5-15)
│   ├── milvus/search.go   # EXTEND: output fields + effective cols
│   └── mongo/mongo.go     # EXTEND: filter $or clause + Insert effective fields (D5-15)
├── agent/callbacks.go     # UNCHANGED (SC-4 reuses data-decision as-is)
├── server/
│   ├── pipeline.go        # EVOLVE: route-then-run (D5-05) + post-verdict passes (SC-6/7)
│   ├── annotate.go        # NEW: counter-evidence pass (D5-13) — separate from grounding
│   ├── redline.go         # NEW: conditional redline LLM pass (D5-14)
│   └── skills.go          # EXTEND: /api/skills/wizard endpoint (SC-3)
├── knowledge/indexer.go   # EXTEND: parse effective_from/to from frontmatter; validate (KNOW-02)
├── wire/
│   ├── chunk.go           # EXTEND: DecisionPayload +skill_scores/+selected; add data-redline (D5-04/14)
│   └── redline.go         # NEW: RedlinePayload struct {original_clause, proposed_rewrite, rationale}
└── store/                 # UNCHANGED (SC-5 reads through existing abstraction)
web/src/
├── types/wire-types.ts    # EXTEND: DecisionPayload + fields; add data-redline variant (D15 mirror)
├── components/
│   ├── inspector/         # NEW: AgentStateInspector panel (SC-4)
│   ├── graph/             # NEW: KnowledgeGraphBrowser tree (SC-5)
│   └── verdict/           # EXTEND: CounterEvidenceCard + RedlineCard (SC-6/7)
└── transport/             # UNCHANGED (D-04/D12 invariant holds — new chunk types auto-flow)
```

### Pattern 1: fsnotify Hot-Reload with In-Flight Version Pinning (SC-2, D5-06..09)
**What:** A watcher goroutine reloads `SKILL.md` on change after a 1s debounce; an `atomic.Pointer` publishes the new version to concurrent readers; in-flight runs keep the snapshot they started with.
**When to use:** SYSK-04. Every Skill file under `skills/*/SKILL.md`.
**Critical detail:** **Watch the parent directory (`skills/`), NOT individual files.** Editors (and the SC-3 wizard) save atomically via write-temp + rename, which invalidates file-level watches. Directory watches survive renames `[CITED: github.com/fsnotify/fsnotify/issues/553, deepin-community/golang-fsnotify]`.
**Example:**
```go
// Source: pattern adapted from github.com/fsnotify/fsnotify issue #553 (canonical debounce)
// + Go sync/atomic.Pointer docs. NOT yet in the codebase — planned for internal/skill/registry.go.

func (r *Registry) Watch(ctx context.Context, skillsDir string) error {
    w, err := fsnotify.NewWatcher()
    if err != nil { return err }
    defer w.Close()
    // Watch the directory (NOT individual files) — atomic save+rename safe.
    if err := w.Add(skillsDir); err != nil { return fmt.Errorf("watch %s: %w", skillsDir, err) }

    var debounce *time.Timer
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event, ok := <-w.Events:
            if !ok { return nil }
            // Only react to SKILL.md changes (create/write/rename) in immediate subdirs.
            if !strings.HasSuffix(event.Name, "/SKILL.md") { continue }
            // D5-06 debounce: reset the timer on every event; fire only after 1s of quiet.
            if debounce != nil { debounce.Stop() }
            debounce = time.AfterFunc(time.Second, func() {
                r.reloadOne(event.Name) // re-parse → Validate → atomic.Store (D5-08: fail keeps old)
            })
        case err, ok := <-w.Errors:
            if !ok { return nil }
            // Log + continue; a transient watcher error must NOT kill the server.
            slog.Warn("fsnotify error", "err", err)
        }
    }
}

// reloadOne implements D5-08: validation failure keeps the old version + reports.
func (r *Registry) reloadOne(path string) {
    m, err := parseAndValidate(path) // reuses skill.Validate — the 7-field gate
    if err != nil {
        // D5-08: NEVER silently drop a working Skill. Old version stays; emit error.
        emitSkillLoadError(path, err) // SSE/log + /api/skills status
        return
    }
    desc, _ := r.embedder.Embed(ctx, m.Name + " " + m.Description) // D5-01 re-embed
    snap := r.snapshot.Load() // copy-on-write the map
    next := snap.clone(); next[m.Name] = &Entry{Manifest: m, DescVec: desc, Version: bump()}
    r.snapshot.Store(next) // atomic publication — in-flight runs keep their old pointer (D5-06)
}
```

### Pattern 2: Route-then-Run (SC-1, D5-05)
**What:** Go dispatches (selects a Skill via cosine) THEN runs that Skill's `VerdictPipeline`. The router is plain Go code BEFORE the ADK agent — not a `ConditionalAgent` in the graph.
**When to use:** SYSK-05. The entry to `POST /api/chat`.
**Example:**
```go
// Source: extends internal/server/pipeline.go VerdictPipeline (D5-05). Router reuses the
// existing cosine() from internal/retriever/mongo/rank.go:63 — verbatim.

func (s *Deps) RouteThenRun(ctx context.Context, c *app.RequestContext, query string) error {
    qvec, err := s.Embedder.Embed(ctx, query)
    if err != nil { return fmt.Errorf("route: embed query: %w", err) }

    reg := s.Registry.Snapshot() // atomic.Pointer load — the current latest version
    type scored struct { name string; score float32 }
    scores := make([]scored, 0, len(reg))
    for name, entry := range reg {
        // reuse mongo.cosine(queryVec, entry.DescVec) — D5-01
        scores = append(scores, scored{name, cosine(qvec, entry.DescVec)})
    }
    sort.SliceStable(scores, func(i,j int) bool { return scores[i].score > scores[j].score })

    selected, ambiguous := pickSkill(scores) // D5-03: top-1<T_low OR (top1-top2)<margin → ambiguous
    // D5-04: extend the existing data-decision chunk with skill_scores + selected
    emitRoutingTrace(c, scores, selected, ambiguous)

    if ambiguous {
        return emitAmbiguityPicker(c, scores) // reuse Phase-4 SkillPicker
    }
    manifest := reg[selected].Manifest
    return s.VerdictPipeline(ctx, c, query, manifest) // unchanged existing pipeline
}
```

### Pattern 3: Counter-evidence as Annotation, NOT Grounding (SC-6, D5-13)
**What:** After the verdict, a SECOND retriever call over the 历史 tier surfaces contradicting chunks. These chunks are emitted as annotation but **structurally never reach `GroundingChecker`**.
**When to use:** CMPL-06. Every verdict (or every non-refusal verdict — planner decides).
**The trust boundary (load-bearing):**
```go
// Source: NEW file internal/server/annotate.go. The critical invariant: the counter-evidence
// chunk map is a DIFFERENT variable from the one passed to GroundingChecker in pipeline.go.

func (s *Deps) emitCounterEvidence(ctx context.Context, c *app.RequestContext, qvec []float32) error {
    // SECOND retriever call — tiers=["历史"]. This is the ONLY place 历史 enters the flow.
    histChunks, err := s.Retriever.Search(ctx, qvec, []string{"历史"}, 5)
    if err != nil { return err }
    // CRITICAL: these chunks are NEVER added to chunksByID (the map GroundingChecker reads).
    // They are annotation-only. Verdict's grounding floor (权威-only) is untouched (D-09/D2-01).
    for _, ch := range histChunks {
        emitSourceDoc(c, ch, tier="历史", marker="counter-evidence") // reuse source-document wire type
    }
    return nil
}
// Insertion point (pipeline.go, after line 232 emitVerdictStream):
//   return s.emitCounterEvidence(ctx, c, qvec) // annotation pass; verdict already emitted
```

### Anti-Patterns to Avoid
- **Watching individual `SKILL.md` files instead of the `skills/` directory.** Editors and the wizard write atomically (temp + rename); the inode-level watch dies on rename. **Watch the directory; filter by `HasSuffix("/SKILL.md")`.** `[CITED: fsnotify issue #553]`
- **Routing via `NewConditionalAgent` inside the ADK graph.** D5-05 explicitly rejected this — it entangles routing with the agent loop, makes the routing trace harder to emit, and is a heavier refactor. Route in plain Go BEFORE ADK.
- **Adding 历史-tier chunks to the `chunksByID` map that feeds `GroundingChecker` (D2-01).** This is the single most dangerous slip: it would mix counter-evidence into grounding and silently weaken D-09. The counter-evidence pass must use a **different variable** in a **different function**.
- **Making `data-redline` a `data-policy` sub-field instead of its own wire type.** D5-14 explicitly spends the "new wire type" budget (17→18) so redline has its own lifecycle + TS variant (D15). Stuffing it into data-policy muddies the verdict contract.
- **Storing `effective_to` as a nullable Milvus field and filtering with `effective_to IS NULL OR effective_to >= today`.** Milvus evaluates NULL as FALSE in comparisons, breaking the naive OR. Use a sentinel (`0` = perpetual). See §Pitfall 2.
- **Hand-rolling cosine for routing.** It already exists at `internal/retriever/mongo/rank.go:63` — reuse it. A second copy invites drift.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cosine similarity for Skill routing | A new vector-math helper | `internal/retriever/mongo/rank.go:63` `cosine()` | Already in-repo, bit-exact, battle-tested. `[VERIFIED]` |
| File-watch + debounce | A polling loop or raw OS hooks | `github.com/fsnotify/fsnotify` + `time.AfterFunc` debounce | Cross-platform (Windows!), handles atomic rename, already transitive in go.mod. `[VERIFIED: go.mod]` |
| Lock-free config publication | `sync.Mutex` on every read | `atomic.Pointer[T]` | Readers (every `/api/chat`) pay zero contention; watcher Store()s infrequently. Go 1.19+ stdlib. |
| Skill validation (wizard output) | A new validator | `skill.Validate(*Manifest)` (the Phase-2 7-field gate) | The wizard's output MUST pass the same gate (SC-3). Reusing it keeps the contract singular. `[VERIFIED: internal/skill/validate.go]` |
| Routing-trace wire chunk | A new `data-routing` wire type | Extend the existing `data-decision` DecisionPayload (+`skill_scores[]`, +`selected`) | D5-04 — preserves the wire type count + D-04/D12 transport invariant + D15 drift alarm. The "new type budget" is reserved for redline. |
| Counter-evidence wire chunk | A new `data-counter` type | Reuse `source-document` chunks with `tier="历史"` + a marker | source-document already carries chunk content + tier; no new wire type needed (keeps the count at 18 after redline). |
| The Verdict pipeline | A parallel "routing pipeline" | Evolve `VerdictPipeline` into route-then-run (D5-05) | The existing 10-step pipeline (embed → retrieve → ground → emit) is preserved; routing is a thin prelude. `[VERIFIED: internal/server/pipeline.go:120]` |

**Key insight:** Every SC in this phase has a named reuse target. The phase is an *extension* of shipped code, not a new product. The planner should resist any task that builds a parallel system to one that already exists.

## Runtime State Inventory

> This phase modifies the chunk schema (D5-15), extends two wire types (D5-04, D5-14), and changes the retriever PRE-filter. That makes it a partial migration — the corpus + wire must keep working across the change.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **Mongo `knowledge` collection docs** currently have `{id, tier, content, source, embedding}` (no effective fields) `[VERIFIED: internal/retriever/mongo/mongo.go:285 Insert]`. **Milvus collection schema** same fields `[VERIFIED: internal/retriever/milvus/seed.go + search.go:52 output field list]`. | **Code edit + data default.** New `effective_from`/`effective_to` fields default to sentinel `0` (perpetual). Existing docs need NO migration — the indexer writes the default on re-seed; the filter treats absent/0 as perpetual. Operator re-seeds via `cmd/server` startup (the existing `knowledge.Index` path). |
| Live service config | **In-memory Skill registry** (`Loader.cached []*Manifest` `[VERIFIED: internal/skill/loader.go:53]`) — currently startup-only, no runtime state beyond the cache. fsnotify adds a long-lived goroutine. | **Code edit.** Replace `Loader.cached` with an `atomic.Pointer[map[string]*Entry]` registry (D5-06). The watcher goroutine is new runtime state — must be ctx-cancelable + survive SIGHUP. |
| OS-registered state | None found. No Windows Task Scheduler / pm2 / launchd entries reference Skill names. Skills are filesystem-driven only (`skills/<name>/SKILL.md`). | None. |
| Secrets/env vars | No new secrets. Wizard + redline reuse the existing `GLM_API_KEY` / `ARK_API_KEY` (OpenAI-compatible) already wired in `cmd/server`. | None. |
| Build artifacts | **Milvus collection schema is immutable post-create** — adding `effective_to` requires either a new collection or `ALTER` (Milvus 2.5+ supports add-field). Mongo is schemaless (no migration). | **Code edit.** Milvus: `Ensure` must add the new scalar columns (sentinel default 0) — drop+recreate is acceptable for the dev corpus (the existing `Reset`/re-seed pattern). The Mongo `Ensure` already creates a `{tier:1}` index; extend to add an `{effective_to:1}` index. |

**Migration backward-compat:** The sentinel-`0` design means the existing versionless corpus needs **zero data migration** — only the code (Document struct, filter expr, Insert) changes. Re-seeding writes `0` for old docs. This is the lowest-risk path and directly satisfies CONTEXT.md's "backward-compatible with existing versionless chunks" intent.

## Common Pitfalls

### Pitfall 1: Watching the file instead of the directory (fsnotify silent death)
**What goes wrong:** The watcher adds `skills/compliance/SKILL.md` directly. The SC-3 wizard (or an editor) saves via write-temp + atomic rename. The rename invalidates the inode watch; subsequent edits silently never fire `Watch.Events`.
**Why it happens:** File-level watches feel more precise. But POSIX/Windows atomic save is the norm for editors and write helpers.
**How to avoid:** `w.Add(skillsDir)` on the **directory**; filter events server-side with `strings.HasSuffix(event.Name, "/SKILL.md")`. Re-add the directory watch if it's removed. `[CITED: github.com/fsnotify/fsnotify/issues/553]`
**Warning signs:** "I edited the Skill but the change never took effect without a restart" — the literal regression SC-2 exists to fix.

### Pitfall 2: Milvus nullable `effective_to` evaluates NULL as FALSE (D5-15 landmine)
**What goes wrong:** CONTEXT.md's literal design stores `effective_to` as nullable with semantics "NULL = perpetually valid" and filters `effective_to IS NULL OR effective_to >= today`. **Milvus documents that NULL scalar values always evaluate to FALSE in filter expressions** `[CITED: milvus.io/docs/nullable-and-default.md]`. In the OR expression, a NULL row reaches the `effective_to >= today` arm where it evaluates FALSE; whether the `IS NULL` arm rescues it depends on Milvus's OR-rewrite path, which has documented edge cases (nullable OR-rewrite bugs in the Milvus issue tracker around nullable scalar filters). The net risk: perpetually-valid policies get filtered OUT, and SC-8 ("effective_to < today yields zero results") fails inverted — the current policy disappears.
**Why it happens:** Milvus's nullable support is relatively recent (v2.5+) and the filter-rewrite semantics around `OR` + nullable are subtle. Mongo handles nullable cleanly, so the bug is Milvus-specific — but the retriever abstraction must work on both.
**How to avoid:** **Store `effective_to` as Int64 epoch-seconds with sentinel `0` = perpetually valid.** Filter becomes `effective_to == 0 or effective_to >= today` — no nullable field, no NULL-semantics dependency, no OR-rewrite edge case. Works identically on Milvus (`and (effective_to == 0 or effective_to >= <ts>)`) and Mongo (`$or: [{effective_to: 0}, {effective_to: {$gte: <ts>}}]`). Fully backward-compatible (existing chunks default to 0). **This is a deviation from CONTEXT.md's literal "null" wording but preserves its semantic intent at lower risk — planner should confirm.** `[CITED: milvus.io/docs/nullable-and-default.md]` + `[ASSUMED]` on the specific Milvus bug numbers (the "NULL evaluates FALSE" rule is documented; the exact OR-rewrite issue numbers are harder to pin down and may be version-specific).
**Warning signs:** After enabling effective_to filtering, current/valid policies stop returning in retrieval; eval faithfulness drops; users see spurious `no_authorized_source` refusals on queries that used to work.

### Pitfall 3: Mixing counter-evidence into the grounding chunk map (D5-13 moat erosion)
**What goes wrong:** The counter-evidence 历史-tier chunks get appended to the same `chunksByID` map that `GroundingChecker` (D2-01) reads. Now the verdict's citations can be "grounded" against 历史-tier text — destroying the 权威-only trust floor (CMPL-04) and weakening refusal-by-construction (D-09).
**Why it happens:** It feels efficient to reuse one chunk map. The semantic boundary "annotation ≠ grounding" is invisible unless encoded in code structure.
**How to avoid:** The counter-evidence pass runs in a **separate function** (`emitCounterEvidence`) that takes only `qvec` and emits directly to the wire — it never receives the verdict's `chunksByID` map and never returns chunks to the caller. The insertion point in `pipeline.go` is AFTER `emitVerdictStream` (line 232), so the verdict is already finalized + grounded before 历史 enters the picture.
**Warning signs:** A citation's `tier` field ever reads `"历史"` in a compliance verdict (CMPL-04 says citations MUST be 权威-only). Add an assertion in the verdict-emit path.

### Pitfall 4: Wizard output that fails the 7-field validator (SC-3 closed-loop break)
**What goes wrong:** The SC-3 wizard generates a `SKILL.md` that looks valid but is missing `temperature` (SYSK-08 — no default) or has an empty `reads` (default-deny). The validator rejects it; fsnotify never picks it up; the user sees no new Skill. The "closed loop" (D5-11) silently breaks.
**Why it happens:** LLMs commonly omit fields they consider "obvious defaults." GLM-5.2 may emit `temperature: 0.0` for compliance but omit it for a "general QA" Skill, assuming a default.
**How to avoid:** The wizard's confirmation step MUST run `skill.Validate(parsedManifest)` BEFORE writing the file and show validation errors inline. Spike 003 confirmed structured-output (`response_format:json_object`) gives 100% schema-valid output — use it for the wizard's final-emit step. `temperature` is explicitly recommended by Skill type (D5-12) so the wizard's interview elicits it.
**Warning signs:** Wizard preview shows a `SKILL.md` but the Skill never appears in `/api/skills`; server log shows a validation error on the written file.

### Pitfall 5: Redline call on every verdict (SC-7 cost blowup)
**What goes wrong:** The redline LLM pass fires unconditionally on every verdict, doubling token cost per query even for `合规` verdicts where no rewrite is needed.
**Why it happens:** The conditional gate ("only when verdict flags a gap") is easy to forget when wiring the post-verdict passes.
**How to avoid:** The redline pass is strictly conditional on `verdict.Status == "不合规" || verdict.Status == "需补充"`. A `合规` verdict emits NO redline chunk and makes NO LLM call. D5-14 is explicit: "No gap → no call (cost-controlled)." Encode the gate as the first line of the redline function.
**Warning signs:** Token cost per query roughly doubles after SC-7 ships; redline chunks appear on compliant verdicts.

### Pitfall 6: TS wire mirror drift after adding data-redline (D15 alarm)
**What goes wrong:** The Go `ValidTypes` map gains `data-redline` (17→18) but the TS `SkillForgeChunk` union doesn't get the new variant. The D15 compile-time `AssertAssignable<SkillForgeChunk, UIMessageChunk>` catches the drift — but only if the change is in the same commit/PR.
**Why it happens:** Go and TS changes happen in different worktrees/commits. D15's two-PR rule exists for exactly this.
**How to avoid:** Every wire change (D5-04 DecisionPayload field additions + D5-14 data-redline) MUST mirror in `web/src/types/wire-types.ts` in the same commit. Run `cd web && npx tsc --noEmit` (exit 0 is the gate — Phase 4 closed this debt in 04-04). `[VERIFIED: web/src/types/wire-types.ts _driftCheck]`

## Code Examples

### Example 1: Extending the retriever abstraction + Document schema (D5-15)
```go
// Source: extends internal/retriever/retriever.go (verified) + internal/retriever/milvus/filter.go.
// The retriever.Document gains two optional effective-date fields. Sentinel 0 = perpetual.

type Document struct {
    ID             int64
    Tier           string
    Content        string
    Source         string
    Embedding      []float32
    EffectiveFrom  int64  // epoch seconds; 0 = unspecified (treated as perpetual)
    EffectiveTo    int64  // epoch seconds; 0 = perpetually valid (sentinel — see Pitfall 2)
}

// milvus/filter.go — extend BuildMilvusFilter to co-layer the effective PRE-filter (D5-15).
// today is passed in (not computed inside) so the function stays pure + testable.
func BuildMilvusFilter(tiers []string, now int64) string {
    if len(tiers) == 0 { return "false" } // unchanged default-deny (D-09)
    quoted := /* existing tier quoting */
    // tier AND (perpetual OR not-yet-expired). Sentinel 0 = perpetual (Pitfall 2).
    return fmt.Sprintf("tier in [%s] and (effective_to == 0 or effective_to >= %d)",
        strings.Join(quoted, ","), now)
}

// mongo/mongo.go Search — same semantics, bson form.
filter := bson.D{
    {Key: "tier", Value: bson.D{{Key: "$in", Value: tiers}}},
    {Key: "$or", Value: bson.A{
        bson.D{{Key: "effective_to", Value: 0}},
        bson.D{{Key: "effective_to", Value: bson.D{{Key: "$gte", Value: now}}}},
    }},
}
```

### Example 2: Routing trace via data-decision extension (D5-04)
```go
// Source: extends internal/wire/chunk.go DecisionPayload (verified lines 91-97).
// ADDITIVE fields — existing consumers ignore unknown fields (backward-compatible).

type DecisionPayload struct {
    Step       int     `json:"step"`
    Decision   string  `json:"decision"`            // + "route" as a new Decision value
    ToolCallID string  `json:"toolCallId,omitempty"`
    Query      string  `json:"query,omitempty"`
    Confidence float64 `json:"confidence,omitempty"`
    // D5-04 additions (routing trace):
    SkillScores map[string]float64 `json:"skillScores,omitempty"` // name → cosine
    Selected    string             `json:"selected,omitempty"`    // chosen Skill name
}
// ValidTypes UNCHANGED — data-decision already registered (17 stays 17 for routing).
```

```typescript
// Source: extends web/src/types/wire-types.ts DecisionPayload (verified lines 60-70).
export interface DecisionPayload {
  step: number;
  decision: 'retrieve' | 'answer' | 'refuse' | 'forced_stop' | 'route'; // + 'route'
  toolCallId?: string;
  query?: string;
  confidence?: number;
  // D5-04 additions:
  skillScores?: Record<string, number>;
  selected?: string;
}
```

### Example 3: The data-redline wire chunk (D5-14, 17→18)
```go
// Source: NEW — internal/wire/redline.go + ValidTypes entry in chunk.go.
type RedlinePayload struct {
    OriginalClause  string `json:"original_clause"`   // the flagged policy text
    ProposedRewrite string `json:"proposed_rewrite"`  // LLM's clause-level diff
    Rationale       string `json:"rationale"`          // why the rewrite fixes the gap
}
// ValidTypes gains: "data-redline": true,  // count 17 → 18
```
```typescript
// web/src/types/wire-types.ts — new variant in SkillForgeChunk union (D15 mirror).
| { type: 'data-redline'; id?: string; data: RedlinePayload }
```

### Example 4: KNOW-02 extension to effective fields (SC-8 indexer)
```go
// Source: extends internal/knowledge/indexer.go validateTiers (verified lines 266-278).
// Parse effective_from / effective_to from YAML frontmatter (the indexer already
// strips frontmatter via stripFrontmatter — extract BEFORE stripping).
func validateEffective(docs []retriever.Document) error {
    for i, d := range docs {
        // effective_to == 0 is the perpetual sentinel (valid). Negative or
        // effective_to < effective_from (when both nonzero) is a data bug.
        if d.EffectiveTo < 0 {
            return fmt.Errorf("doc[%d] source=%s has negative effective_to", i, d.Source)
        }
        if d.EffectiveFrom != 0 && d.EffectiveTo != 0 && d.EffectiveTo < d.EffectiveFrom {
            return fmt.Errorf("doc[%d] source=%s: effective_to < effective_from", i, d.Source)
        }
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Startup-only Skill loading (`Loader.Scan`) | fsnotify hot-reload with debounce + atomic snapshot | This phase (SYSK-04) | Skills update without restart; the loader docstring already anticipated this (`internal/skill/loader.go:39-40`). |
| Single-Skill `VerdictPipeline` | Route-then-run (router prelude + existing pipeline) | This phase (SYSK-05) | N-Skill platform behavior; the existing 10-step pipeline is preserved as-is. |
| Tier-only PRE-filter (`tier in [...]`) | Tier + effective-date PRE-filter (sentinel design) | This phase (CMPL-08) | Superseded policies filtered before similarity; existing corpus needs zero migration (sentinel default 0). |
| Verdict-only response | Verdict + counter-evidence annotation + conditional redline | This phase (CMPL-06/07) | Two post-verdict passes compose without touching the trust floor. |

**Deprecated/outdated:**
- **Milvus nullable-field filtering with `IS NULL OR ...`**: current Milvus docs note NULL-evaluates-FALSE + the nullable OR-rewrite has edge cases. The sentinel-value approach supersedes it for this use case. `[CITED: milvus.io/docs/nullable-and-default.md]`
- **Post-filtering effective dates in Go** (load all, then drop expired): violates the D7 PRE-similarity contract and wastes retrieval budget. The filter must be at the SDK layer, same as tier.

## Assumptions Log

> slopcheck was unavailable; per the protocol every package is `[ASSUMED]` by default. However, this phase introduces ZERO new external packages — all entries below are either in-repo code (verified by reading the source) or already-locked go.mod dependencies (verified via `go list -m`).

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `fsnotify` v1.5.4 works correctly on Windows 11 for the hot-reload watcher. | Standard Stack, Pattern 1 | LOW — fsnotify is documented cross-platform incl. Windows `[CITED: pkg.go.dev/github.com/fsnotify/fsnotify]`; the project env is win32. Already transitive via hertz (also cross-platform). |
| A2 | The specific Milvus nullable-field OR-rewrite bug numbers (referenced as #49891 etc.) are version-specific and may not apply to the exact Milvus server version the operator runs. | Pitfall 2 | MEDIUM — but the underlying "NULL evaluates FALSE" rule is officially documented and version-stable. The sentinel recommendation is robust regardless of the specific bug. Values `[ASSUMED]` for the bug IDs; `[CITED]` for the NULL-rule. |
| A3 | The wizard's guided interview can be driven by the same GLM-5.2 OpenAI-compatible adapter already wired (no new model client). | Standard Stack, Wizard | LOW — spike 001 validated Eino passthrough; spike 003 validated structured output at temperature 0. The wizard reuses both. `[VERIFIED: spike-findings SKILL.md]` |
| A4 | `internal/store.Store` exposes enough metadata for the SC-5 graph browser without schema changes. | Graph Browser | LOW-MEDIUM — the store ships audit + session; the graph browser primarily needs knowledge-tier metadata which lives in the retriever Hit/Document. May need a small read accessor. Planner verifies. |
| A5 | The existing `cosine()` in mongo/rank.go is numerically identical when applied to Skill-description vectors (different text domain). | Routing | LOW — cosine is domain-agnostic; BGE-M3 produces the same 1024-dim space for any text. |

**If this table were empty:** Not applicable — 5 assumptions logged. A1/A3/A5 are LOW risk (verified substrates). A2 is the one to surface to the user (the sentinel-vs-nullable deviation). A4 is a small planner-time check.

## Open Questions

1. **Sentinel `0` vs nullable `effective_to` (D5-15 deviation)**
   - What we know: CONTEXT.md says "null effective_to = perpetually valid." Milvus documents NULL-as-FALSE in filters, making the naive OR fragile.
   - What's unclear: whether the user considers the sentinel approach a material change to D5-15's intent or an acceptable implementation detail.
   - Recommendation: planner confirms with user. The sentinel preserves the *semantic* intent (perpetually valid) at lower risk. Flag as a `[NEEDS DECISION]` in the plan.

2. **Counter-evidence retrieval query**
   - What we know: D5-13 specifies a post-verdict pass over the 历史 tier.
   - What's unclear: does it use the SAME `qvec` as the verdict retrieval (simplest), or a re-formulated query derived from the verdict's citations (more precise)? CONTEXT.md is silent.
   - Recommendation: start with the same `qvec` (simplest, satisfies SC-6); a refined query is a v2 enhancement.

3. **Routing-trace emission timing**
   - What we know: D5-04 extends `data-decision` with `skill_scores[]` + `selected`.
   - What's unclear: is the routing trace emitted as a `data-decision` with `decision="route"` BEFORE the verdict pipeline runs, or only after?
   - Recommendation: emit BEFORE the verdict (so the inspector shows routing first, then agent decisions). The `decision` enum gains a `"route"` value.

4. **Graph browser data source**
   - What we know: Claude's discretion says use `store.Store` + knowledge metadata; lean lightweight tree.
   - What's unclear: does `store.Store` currently expose a "list all chunks grouped by tier/doc" query, or does the graph browser need to call a new `/api/knowledge/graph` endpoint backed by the retriever's metadata?
   - Recommendation: planner adds a thin read endpoint (`GET /api/knowledge/graph` → tier/doc/chunk tree from the retriever's knowledge of indexed docs). Confirm at plan time.

## Environment Availability

> The phase depends on fsnotify (already present), the existing Go/Hertz/Eino/Milvus/Mongo stack (all installed), and the existing React toolchain. Probed via `go list -m`, `go version`, and inspection of `web/`.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | BE changes | ✓ | go1.25.0 windows/amd64 | — |
| `github.com/fsnotify/fsnotify` | SC-2 hot-reload | ✓ (indirect) | v1.5.4 | Promote to direct; no download needed |
| `github.com/cloudwego/eino` | Wizard + redline LLM | ✓ | v0.7.7 | — |
| `github.com/cloudwego/hertz` | New endpoints + SSE | ✓ | v0.10.6 | — |
| Milvus server | SC-8 effective_to PRE-filter (Milvus backend) | ✗ (broken per D11) | — | **Mongo backend is the working default** (`RETRIEVER_BACKEND=mongo`). Milvus code path still must be correct for when the image is fixed, but v1 verification runs on Mongo. |
| MongoDB server | SC-8 effective_to PRE-filter (Mongo backend) | ✓ (D11) | 4.4+ | — |
| BGE-M3 embedder (D5-01 routing) | Skill-description vectors | ✓ | in-repo `internal/embedder/bge` | — |
| Node/npm + web toolchain | New UI panels | ✓ | (Phase-4 shipped; `cd web && npx tsc --noEmit` exits 0) | — |

**Missing dependencies with no fallback:**
- None. The broken internal Milvus is already mitigated by the Mongo backend (D11); the Milvus code path is maintained for parity but not a v1 execution blocker.

**Missing dependencies with fallback:**
- Internal Milvus image — fallback is Mongo (already the default). Effective_to filter syntax must be correct in BOTH `milvus/filter.go` and `mongo/mongo.go` regardless of which backend is active.

## Validation Architecture

> `workflow.nyquist_validation` is `true` in `.planning/config.json`. Include this section.

### Test Framework
| Property | Value |
|----------|-------|
| Framework (BE) | Go `testing` + `testify` v1.11.1 (`[VERIFIED: go.mod]`) |
| Framework (FE) | vitest + @testing-library/react (`[VERIFIED: Phase 4 — 84/84 green]`) |
| Config file (BE) | none (standard `go test`) |
| Config file (FE) | `web/vitest.config.*` (Phase-4 established) |
| Quick run command (BE) | `go test ./internal/skill/... ./internal/wire/... ./internal/retriever/... -count=1` |
| Quick run command (FE) | `cd web && npx vitest run` (or `npx tsc --noEmit` for the D15 drift alarm) |
| Full suite command (BE) | `go test ./...` |
| Full suite command (FE) | `cd web && npx vitest run && npx tsc --noEmit` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SYSK-04 | fsnotify reload after 1s debounce; in-flight pinned | unit + integration | `go test ./internal/skill -run TestRegistry_Watch -count=1` | ❌ Wave 0 (new registry.go) |
| SYSK-05 | cosine routing picks right Skill; trace carries score | unit | `go test ./internal/skill -run TestRouter -count=1` | ❌ Wave 0 (new router.go) |
| SYSK-06 | wizard output passes `skill.Validate` (7-field) | unit | `go test ./internal/skill -run TestWizard_GeneratesValid -count=1` | ❌ Wave 0 (new wizard.go) |
| AGNT-06 | inspector renders data-decision + tool/reasoning tree | unit (FE) | `cd web && npx vitest run inspector` | ❌ Wave 0 (new component) |
| KNOW-06 | graph browser shows tier→doc→chunk; click reveals chunk | unit (FE) | `cd web && npx vitest run graph` | ❌ Wave 0 (new component) |
| CMPL-06 | counter-evidence 历史 chunks emit as annotation; never grounded | unit | `go test ./internal/server -run TestCounterEvidence_NotGrounded -count=1` | ❌ Wave 0 (new annotate.go) |
| CMPL-07 | redline emits only on 不合规/需补充; data-redline wire type | unit | `go test ./internal/server -run TestRedline_Conditional -count=1` + `go test ./internal/wire -run TestValidTypes_DataRedline` | ❌ Wave 0 (extend chunk.go) |
| CMPL-08 | effective_to PRE-filter: expired → 0 hits; sentinel 0 → valid | unit | `go test ./internal/retriever/... -run TestEffectiveFilter -count=1` | ❌ Wave 0 (extend filter.go + mongo.go) |

### Sampling Rate
- **Per task commit:** `go test ./internal/skill/... ./internal/wire/... ./internal/retriever/... -count=1` + `cd web && npx tsc --noEmit` (the D15 drift alarm is cheap + catches the wire-break class early).
- **Per wave merge:** `go test ./...` + `cd web && npx vitest run`.
- **Phase gate:** Full suite green + a manual SC-walk (8 SCs) before `/gsd:verify-work`. The transport-swap invariant (D-04/D12) must stay green — run the Phase-4 `transport-swap.test.tsx` after every wire change.

### Wave 0 Gaps
- [ ] `internal/skill/registry_test.go` — covers SYSK-04 (atomic snapshot + debounce + validation-failure-keeps-old D5-08).
- [ ] `internal/skill/router_test.go` — covers SYSK-05 (cosine routing + ambiguity threshold D5-03).
- [ ] `internal/skill/wizard_test.go` — covers SYSK-06 (output passes `Validate`; temperature-by-type D5-12).
- [ ] `internal/wire/chunk_test.go` — extend for data-redline (18th type) + DecisionPayload routing fields.
- [ ] `internal/retriever/milvus/filter_test.go` + `internal/retriever/mongo/mongo_test.go` — extend for effective_to PRE-filter (sentinel 0 + expiry).
- [ ] `internal/server/annotate_test.go` — covers CMPL-06 (counter-evidence NEVER in GroundingChecker's map).
- [ ] `web/src/types/wire-types.ts` — D15 mirror (data-redline variant + DecisionPayload fields); `npx tsc --noEmit` is the gate.
- [ ] `web/src/components/inspector/*` + `web/src/components/graph/*` — new component tests.

*(If no gaps: N/A — but this phase adds 5 new BE files + 2 new FE component dirs, so Wave 0 test scaffolding is required.)*

## Security Domain

> `security_enforcement` is absent in `.planning/config.json` (absent = enabled). Include this section. Relevant threat surface for Phase 5: fsnotify path traversal, wizard output injection, redline content rendering, effective_to forgery.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture & Trust Boundary | yes | The counter-evidence annotation boundary (D5-13) is a new internal trust boundary — 历史-tier data must not cross into the grounding path. Enforce structurally (separate function + separate chunk map). |
| V4 Access Control | yes | Skill `reads` enforcement (CMPL-04) unchanged; routing (D5-05) must not bypass the per-Skill tier filter — the routed Skill's `ResolvedTiers()` still drives retrieval. |
| V5 Input Validation | yes | (a) fsnotify watcher must reject symlinks outside `skills/` — the existing `Loader.Scan` V12 path-traversal protection (`internal/skill/loader.go:104-118`) MUST extend to the watcher. (b) Wizard NL input → generated SKILL.md must pass `skill.Validate` (the 7-field gate) before write. (c) Wizard write path must constrain the target to `skills/<name>/SKILL.md` (no path escape in the user-chosen `<name>`). |
| V8 Data Protection | yes | effective_to forgery: a chunk with a maliciously-forged `effective_to` could suppress current policy or resurrect superseded policy. The `validateEffective` indexer check (D5-15 extension of KNOW-02) + the immutability of the authority tier mitigate. |
| V12 Files & Resources | yes | The fsnotify watcher + wizard file writes are new filesystem-touching surfaces. Reuse the loader's symlink-escape rejection. |

### Known Threat Patterns for the Skill Platform stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via wizard-supplied Skill name (e.g., `../../etc/skill`) | Tampering / Elevation | Sanitize the Skill name to `^[a-z0-9-]+$`; constrain writes to `skills/<name>/SKILL.md` via `filepath.Join` + the rooted-directory check already in `Loader.Scan`. `[VERIFIED: internal/skill/loader.go:113]` |
| Symlink escape via fsnotify-watched file replaced with a symlink | Tampering | The watcher re-parses on every change; the parse path must re-run the `EvalSymlinks` + rooted check (extend the existing loader guard to the reload path, not just startup Scan). |
| Prompt injection via wizard NL input → generated SKILL.md body | Spoofing | The generated SKILL.md is parsed as YAML frontmatter + Markdown body; a malicious NL input could try to inject YAML keys or escape the body. `skill.Validate` rejects unknown/invalid fields; the body is treated as data (never executed). |
| effective_to forgery resurrecting superseded policy | Repudiation / Tampering | Authority tier is append-only (architecture §3.4); the indexer's `validateEffective` rejects `effective_to < effective_from`; audit log records the effective window of every retrieved chunk. |
| Redline content XSS (LLM emits `<script>` in proposed_rewrite) | Tampering | React escapes text by default; render `proposed_rewrite` as text, NOT `dangerouslySetInnerHTML`. The Phase-4 T-1-08 source-URL allowlist pattern is the precedent. |
| data-redline wire chunk poisoning (extra fields smuggled) | Tampering | The wire chunk's `Data` is `json.RawMessage`; unmarshal strictly into `RedlinePayload` (no `interface{}`); reject unknown fields with `json.Decoder.DisallowUnknownFields()`. |

## Project Constraints (from CLAUDE.md)

> Extracted from the project-level CLAUDE.md (D:\CODE\ClaudeCode\ai-agent\CLAUDE.md). Treated as locked directives — research recommendations must not contradict these.

- **Tech stack (locked):** Go + Eino + Eino ADK + Hertz (BE); React 19 + Vite + TS + Vercel AI SDK v5 + shadcn/Tailwind (FE). This phase adds NO new stack — every recommendation is an extension of the locked stack. `[VERIFIED: CLAUDE.md ## Technology Stack]`
- **License:** Apache-2.0. All new files carry the SPDX header (matches existing `internal/wire/chunk.go:15`). `[VERIFIED: every .go file header]`
- **Referential constraint:** denova source usable as architecture reference but NOT its content-generation direction. The Skill Wizard (D5-10) mirrors denova's `skills-creator` *pattern* (guided interview), not any content-generation behavior. `[VERIFIED: CLAUDE.md ## Project / Constraints]`
- **Open source goal:** polish docs + easy start + measurable effect. The wizard + hot-reload closed loop (SC-3 → D2-07 → SC-2) IS the "easy start" story — a stranger can describe a Skill and get it running without code. This is load-bearing for the personal-brand goal.
- **SkillForge memory system hooks:** The global `~/.claude/CLAUDE.md` describes a hook-based memory system (SessionStart/Stop). This is orthogonal to Phase 5 (it's the operator's environment, not the SkillForge product). No interaction.
- **Project Skill Routing:** `spike-findings-ai-agent` (temperature + structured-output + refusal-by-construction) and `sketch-findings-ai-agent` (three-panel layout + chat-input design) auto-load. This research has consumed both: spike-findings informs the wizard temperature logic (D5-12) + redline determinism; sketch-findings hosts the four new UI panels within the locked three-column layout. `[VERIFIED: .claude/skills/*/SKILL.md]`
- **GSD Workflow Enforcement:** All changes go through a GSD workflow. This research is produced under `/gsd:plan-phase --research-phase`. The planner will create PLAN.md via the same flow. `[VERIFIED: CLAUDE.md ## GSD Workflow Enforcement]`

## Sources

### Primary (HIGH confidence)
- **Live codebase** (read in this session): `internal/wire/chunk.go` (ValidTypes 17-map, DecisionPayload), `internal/skill/{loader,manifest,validate,tier}.go` (7-field schema + file-driven registration + V12 symlink guard), `internal/retriever/retriever.go` (Document struct + Retriever interface), `internal/retriever/milvus/{filter,search}.go` (BuildMilvusFilter tier expr), `internal/retriever/mongo/{mongo,rank}.go` (cosine + tier $in filter), `internal/agent/callbacks.go` (newDecisionCallback + DecisionRecord), `internal/server/pipeline.go` (VerdictPipeline 10-step + retrievalScoreThreshold=0.3 + D2-01 GroundingChecker insertion), `internal/knowledge/indexer.go` (validateTiers + KNOW-02 + stripFrontmatter), `internal/embedder/embedder.go` (Embedder interface), `web/src/types/wire-types.ts` (SkillForgeChunk union + AssertAssignable drift alarm). `[VERIFIED: direct file reads]`
- **go.mod** (go 1.25.0): confirmed `fsnotify v1.5.4` indirect, `eino v0.7.7`, `hertz v0.10.6`, `milvus-sdk-go/v2 v2.4.2`, `mongo-driver/v2 v2.8.0`, `testify v1.11.1`. `[VERIFIED: go.mod]`
- **`.planning/phases/05-.../05-CONTEXT.md`** — 15 locked decisions D5-01..15 + discretion + deferred. `[VERIFIED]`
- **`.planning/SKILL_SCHEMA.md`** — 7-field spec (the SC-3 validator gate). `[VERIFIED]`

### Secondary (MEDIUM confidence — verified against official docs)
- [Milvus Nullable Fields docs](https://milvus.io/docs/nullable-and-default.md) — "NULL scalar values always evaluate to false in filter expressions." Directly informs the D5-15 sentinel recommendation. `[CITED]`
- [Milvus Boolean Expression docs](https://milvus.io/docs/boolean.md) — scalar filtering rules.
- [fsnotify issue #553 "correct way to determine when file writes are finished"](https://github.com/fsnotify/fsnotify/issues/553) — canonical debounce pattern + watch-the-directory rationale (atomic save+rename). `[CITED]`
- [fsnotify pkg.go.dev](https://pkg.go.dev/github.com/fsnotify/fsnotify) — cross-platform (incl. Windows) confirmation.
- [Go sync/atomic.Pointer](https://pkg.go.dev/sync/atomic#Pointer) — lock-free publication (Go 1.19+; project is 1.25).
- **spike-findings-ai-agent SKILL.md** — temperature passthrough (spike 001 ✓) + structured output (spike 003 ✓) + refusal-by-construction (✓). `[VERIFIED: .claude/skills/spike-findings-ai-agent/SKILL.md]`
- **sketch-findings-ai-agent SKILL.md** — three-panel 260/flex/420 layout + chat-input @-mention + denova dark theme. `[VERIFIED: .claude/skills/sketch-findings-ai-agent/SKILL.md]`

### Tertiary (LOW confidence — flagged for validation)
- Specific Milvus nullable-OR-rewrite bug issue numbers (referenced generally as #49891-class) — the "NULL evaluates FALSE" rule is officially documented, but the exact OR-rewrite bug numbers are version-specific and harder to pin down. The sentinel recommendation is robust regardless. `[ASSUMED]` on the bug IDs.
- Whether the operator's Milvus server version supports `add_field` / nullable on an existing collection (vs requiring drop+recreate). The dev path runs on Mongo (D11); the Milvus code is maintained for parity. `[ASSUMED]`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all entries verified against go.mod + in-repo source. Zero new packages.
- Architecture (route-then-run, hot-reload, post-verdict passes): HIGH — every extension point read in the live codebase; patterns map 1:1 to existing code.
- Milvus effective_to filter design: MEDIUM — the NULL-evaluates-FALSE rule is documented (HIGH); the specific nullable-OR-rewrite bugs are version-specific (LOW). Sentinel recommendation de-risks both.
- Pitfalls: HIGH for fsnotify/grounding/wizard (code-grounded + cited); MEDIUM for the Milvus nullable specifics (cited rule + assumed bug numbers).
- Security: HIGH — reuses existing V12/V5 controls already proven in Phases 2/4.

**Research date:** 2026-08-12
**Valid until:** 2026-09-11 (30 days — stable; the only fast-moving surface is Milvus server behavior, which the Mongo fallback absorbs)
