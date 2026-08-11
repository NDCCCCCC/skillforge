# Phase 2: Compliance Vertical Slice - Research

**Researched:** 2026-08-11
**Domain:** Go backend — Hertz HTTP server + Eino ADK verdict pipeline + Milvus tier-filtered retrieval + Skill.md file-driven registration + programmatic grounding verifier
**Confidence:** HIGH (every API anchor below was verified by reading the exact Go module source in `~/go/pkg/mod/` at the versions pinned in `go.mod`; existing spike code already proves the verdict + SSE + Milvus contracts)

## Summary

Phase 2 graduates the Phase 1 spike assets into production packages and wires them into a single vertical slice: **one Skill (compliance) + one authoritative tier (权威) + one streaming endpoint (`POST /api/chat`)** that returns either a verifiable cited verdict or a structured refusal. The spike proved the three contracts in isolation (verdict JSON shape, UIMessageChunk SSE wire format, Milvus tier filter); Phase 2's job is to **connect them with real retrieval and add the programmatic grounding moat (D2-01)**.

Every external API the phase needs is already verified against the exact pinned versions in `go.mod`: `server.WithHostPorts` exists at `hertz@v0.10.6/pkg/app/server/option.go:158-159`; `adk.NewChatModelAgent` / `adk.NewRunner` / `runner.Query` / `adk.WithChatModelOptions` exist at `eino@v0.7.7/adk/{chatmodel.go:212, runner.go:59, runner.go:98, chatmodel.go:54}`; `width.Narrow.String(s)` exists at `golang.org/x/text@v0.13.0/width/width.go:195-197` for the 全角→半角 normalization (D2-05). The spike's `EmitChunk` / `StreamChunks` in `internal/spike/hertz_sse.go` use the verified `sse.NewWriter(c).WriteEvent("", "message", payload)` API and graduate directly.

The only genuinely new Go dependency is `golang.org/x/text/width` (currently `// indirect` at v0.13.0 — Phase 2 promotes it to a direct require). No npm, no PyPI, no Docker (Phase 7). The chunking strategy is deliberately minimal for Phase 2: **one hand-authored `.md` file = one Milvus chunk** (the corpus is ~30 small clauses, each ≤800 bytes, well under BGE-M3's 8192-token context). The full `eino-ext/components/document/transformer/splitter/markdown` hierarchical splitter is deferred to Phase 3 where corpus size warrants it.

**Primary recommendation:** Graduate spike assets via **MOVE + upgrade + re-import** (not copy): create `internal/{verdict,wire,retriever,skill,knowledge,server}` packages, move the spike code into them, upgrade `Verdict.Citations []string` → `[]Citation` per D2-06, and update `cmd/spiked/` + `integration/contract/contract_test.go` imports in the same change so the T-1-16 single-source-of-truth guarantee is preserved. Keep `internal/spike/` empty or as a deprecated re-export shim — do NOT keep a parallel copy (drift risk).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D2-01:** Go programmatic grounding check. Retrieval is a pipeline PRE-step (Go embeds query → tier-filtered Milvus search → inject chunks into prompt), NOT an agent-decided tool call. After the LLM emits a JSON verdict, Go code verifies each citation's `span` literally appears in the retrieved chunk text. The hallucination moat must NOT rely on the LLM policing itself.
- **D2-02:** Ungrounded-claim handling = downgrade to 需补充. Keep the grounded 合规/不合规 conclusion, set `review_required: true`, cap `confidence < 0.9`. Does NOT silently drop and does NOT nuke the whole verdict.
- **D2-03:** `span` = verbatim quote (exact substring from the chunk). Locked by D2-01.
- **D2-04:** citation object = minimal set `{chunk_id, span, tier}`. `score` and `effective_date` deferred to Phase 3/5.
- **D2-05:** normalized substring match (collapse whitespace + 全角→半角 punctuation + trim, then substring-contains). NOT fuzzy/edit-distance.
- **D2-06:** upgrade `Verdict.Citations` from `[]string` to `[]Citation{{ChunkID, Span, Tier}}`. `ValidStatuses` (合规/不合规/需补充/unanswered) and `Validate()` are reused.
- **D2-07:** file-driven registration. `skills/*/SKILL.md` is the single source of truth. Loader scans on startup; `GET /api/skills` lists; `POST /api/skills` is a dry-run validator (does NOT persist).
- **D2-08:** query endpoint = `POST /api/chat`. Skill specified in request body (Phase 2 default = compliance).
- **D2-09:** new `cmd/server` Hertz long-running service. `cmd/spiked` retained as the regression/spike binary.
- **D2-10:** 7-field schema (with `temperature`). ROADMAP SC-1 text "6-field" is stale. Phase 2 implements `.planning/SKILL_SCHEMA.md` v1.0 7-field schema. The 4 spike-era `testdata/skills/*/SKILL.md` fixtures (with `reasoning: false`, inline `reads: ["权威"]`, `guardrails.refuse_on_missing_tier`) must be reconciled to the 7-field spec.
- **D2-11:** hand-authored small authoritative corpus. 5–10 policy docs / ~30 chunks covering the spike's 5 queries, including the intentional refusal case (contact-info query). Reuse existing `auth_001.md`.
- **D2-12:** authoritative corpus at `knowledge/authority/*.md` (tier = top-level dir). `skills/compliance/` holds only `SKILL.md` and declares `reads: authority/**`. Indexer scans `knowledge/{tier}/` into Milvus with `tier` metadata. Migrate/reconcile existing `testdata/docs/` into `knowledge/`.

### Claude's Discretion
- Exact package layout for graduating spike assets (e.g., `internal/verdict`, `internal/wire`, `internal/retriever`, `internal/skill`, `internal/server`) — planner decides; keep `cmd/spiked` runnable.
- How `reads: authority/**` resolves to the Milvus tier value `权威` (loader mapping) — planner/researcher decides; the tier enum is 权威/历史/个人/草稿.
- Whether the grounding check also treats a "novel claim" (claim with no citation at all) via the same D2-02 downgrade path (recommend: yes).
- Exact 5-query compliance set (suggest: reuse spike 002/003's 5 queries for comparability).

### Refusal Taxonomy & Thresholds (Claude's discretion, already resolved in CONTEXT)
- **D2-13:** refusal reason enum = `{no_authorized_source, low_retrieval_score, out_of_scope}`. Structured refusal shape `{reason, can_user_rephrase: bool, suggested_alternative_query: string}` per ARAG-05.
- **D2-14:** `suggested_alternative_query` = LLM-generated (single-shot); rule-based fallback if the LLM omits it.
- **D2-15:** retrieval-score refusal threshold — conservative placeholder (cosine < 0.3 → `low_retrieval_score` refusal), marked for Phase 3 eval calibration. `review_required` confidence threshold pinned at `< 0.9` by SC-5.

### Deferred Ideas (OUT OF SCOPE)
- Multi-skill semantic routing (SYSK-05) — Phase 5.
- Autonomous retrieval decisions / multi-hop / full Self-RAG (ARAG-01/02/03) — Phase 3.
- Streaming Trust UI (FEUI-*) — Phase 4.
- Policy version tracking / counter-evidence / redline (CMPL-06/07/08) — Phase 5. Chunk schema stays `{id, tier, content, source, embedding}`.
- Ragas eval pipeline + 50-case golden set + audit SQL (GEVL-03/04/05) — Phase 3/6.
- Hot-reload (SYSK-04, fsnotify) — Phase 5.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SYSK-01 | User can create a Skill from a declarative `SKILL.md` file in a Skill directory | `internal/skill.Loader.Scan(skillsDir)` walks `skills/*/SKILL.md`; `cmd/server` registers the compliance Skill at startup; `GET /api/skills` lists registered Skills |
| SYSK-02 | System parses and validates `SKILL.md` against the 7-field schema | `internal/skill.Manifest` struct (7 fields) + `Validate()` enforcing: all 7 required, `temperature` float 0.0-1.0 (no default), `reads` non-empty + no `*` wildcard, `guardrails.prohibitions` ≥3, `agent` ∈ ADK types. `POST /api/skills` is the dry-run validator endpoint |
| SYSK-07 | System enforces `reads` as a hard filter at retrieval time | Reuses proven `spike.BuildMilvusFilter` → `tier in ["权威"]` applied PRE-similarity at SDK Search layer (`milvus_client.go:350-362`). Loader resolves `reads: [authority/**]` → tier value `权威` via the tier enum map |
| SYSK-08 | System enforces `temperature` as required (no default) + passes to LLM via Eino `GenerateOption(temperature)` | `internal/skill.Validate` rejects missing/non-float temperature; `cmd/server` verdict pipeline passes `model.WithTemperature(skill.Temperature)` via `adk.WithChatModelOptions` (verified `eino@v0.7.7/adk/chatmodel.go:54`) |
| CMPL-01 | Built-in `企业合规/制度审查` Skill as showcase | `skills/compliance/SKILL.md` (7-field, reads `authority/**`, temperature 0.0, guardrails with ≥3 prohibitions) |
| CMPL-02 | Structured verdict `{status, confidence, citations, review_required}` | `internal/verdict.Verdict` struct (graduates `spike.Verdict` with `Citations []Citation`); emitted as `data-policy` SSE chunk |
| CMPL-03 | `review_required: true` when confidence < 0.9 OR claim is novel | Grounding checker sets `review_required=true` + caps confidence < 0.9 on ungrounded/novel claims (D2-02). Validated by unit test |
| CMPL-04 | Compliance Skill reads ONLY 权威 tier | `BuildMilvusFilter(["权威"])` is the hard barrier; pipeline test asserts personal-tier query → 0 chunks → refusal |
| CMPL-05 | UI displays `needs human review` badge | Phase 2 emits the payload (`review_required: true` in `data-policy` chunk); the UI badge itself is Phase 4 (FEUI) |
| AGNT-03 | Stream agent events (text/reasoning/tool-call/source) via Hertz SSE | `internal/server` handler emits `source-document` (per citation) + `data-policy` (verdict) + `finish-step` chunks via graduated `EmitChunk` (verified `hertz@v0.10.6/pkg/protocol/sse/writer.go`) |
| ARAG-04 | Every cited claim = chunk ID + span; unverifiable claims stripped or refused | Grounding checker (D2-01/D2-05) verifies each citation's span via normalized substring match; ungrounded → D2-02 downgrade. `Citation{ChunkID, Span, Tier}` is the wire contract |
| ARAG-05 | Refusal includes structured reason `{reason, can_user_rephrase, suggested_alternative_query}` | `internal/verdict.Refusal` struct; emitted as `data-policy` chunk with `reason ∈ {no_authorized_source, low_retrieval_score, out_of_scope}` (D2-13) |
| GEVL-01 | System refuses when retrieval score < threshold | Pipeline checks `maxScore < 0.3` (D2-15 placeholder) → `low_retrieval_score` refusal. Calibrated in Phase 3 |
| GEVL-02 | System refuses when no tier-matched docs exist (refusal-by-construction) | Retrieval pre-step: 0 chunks from `SearchWithTierFilter` → immediate `no_authorized_source` refusal before any LLM call. This is the D-09 structural guarantee |
</phase_requirements>

## Architectural Responsibility Map

Phase 2 lives entirely in the **API / Backend (Go)** tier — no Browser, no Frontend Server, no CDN. The only external boundaries are the LLM API (GLM-5.2), the Milvus gRPC service, and the BGE-M3 HTTP service. The diagram below traces the primary use case (`POST /api/chat` → cited verdict) from request to SSE response.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP routing + SSE streaming | API / Backend (Go) | — | `cmd/server` owns the Hertz routes and SSE writer; no reverse proxy or frontend server in Phase 2 |
| Skill loading + 7-field validation | API / Backend (Go) | — | `internal/skill` parses YAML frontmatter at startup; purely file-driven, no DB |
| Query embedding | API / Backend (Go) | External service (BGE-M3) | Go calls the BGE-M3 HTTP endpoint; the embedding itself is computed externally |
| Tier-filtered retrieval | Database / Storage (Milvus) | API / Backend (Go) | Milvus executes the PRE-similarity `tier in [...]` filter; Go builds the filter expression and reads results |
| Verdict generation | External LLM API (GLM-5.2) | API / Backend (Go) | GLM-5.2 emits the JSON verdict; Go parses, validates, and grounds it |
| Programmatic grounding check | API / Backend (Go) | — | **Pure Go function** — the moat. No LLM involvement; deterministic substring match against retrieved chunk text (D2-01) |
| Refusal decision | API / Backend (Go) | — | Go decides refusal from retrieval result (0 chunks / low score) BEFORE calling the LLM; refusal-by-construction (D-09) |
| Knowledge indexing | API / Backend (Go) | Database / Storage (Milvus) | `internal/knowledge.Indexer` scans `knowledge/{tier}/*.md`, embeds via BGE-M3, inserts into Milvus with tier metadata |

**Critical tier boundary:** the `reads` pattern → Milvus tier filter is the ONLY knowledge-boundary enforcement. If the loader maps `authority/**` to the wrong tier value, or if `BuildMilvusFilter` is bypassed, the moat collapses. The grounding check is the SECOND boundary (per-claim), but it cannot compensate for retrieving from the wrong tier.

### System Architecture Diagram (POST /api/chat verdict flow)

```
Client (curl / future React)
   │
   │  POST /api/chat  { "query": "...", "skill": "compliance" }
   ▼
┌─────────────────────────────────────────────────────────────┐
│  cmd/server (Hertz, :8888)                                   │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  POST /api/chat handler                                │  │
│  │  1. Load compliance Skill (reads: authority/** → 权威) │  │
│  │  2. EmbedQuery(BGE-M3) ─────────────────────────┐     │  │
│  │  3. SearchWithTierFilter(tier in ["权威"], topK) │     │  │
│  │     ┌────────────────────────────────────────────▼───┐ │  │
│  │     │ IF 0 chunks  ──► emit refusal SSE ──► return   │ │  │
│  │     │ IF maxScore<0.3 ► emit low_retrieval SSE ─►ret │ │  │
│  │     └───────────────────────────────────────────────┘ │  │
│  │  4. Build prompt: system + injected chunks + query    │  │
│  │  5. ChatModelAgent verdict (temp=0, json_object)      │  │
│  │     ──► runner.Query ──► drain iterator ──► Verdict   │  │
│  │  6. spike.Validate(Verdict)                           │  │
│  │  7. GroundingChecker: for each citation               │  │
│  │       normalize(span) ⊆ normalize(chunk text)?        │  │
│  │       ungrounded ──► D2-02 downgrade                  │  │
│  │  8. Emit SSE stream:                                  │  │
│  │       source-document (per citation, SC-6)            │  │
│  │       data-policy (the Verdict)                       │  │
│  │       finish-step                                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  GET  /api/skills  ──► Loader.List() ──► JSON               │
│  POST /api/skills  ──► skill.Validate(body) ──► JSON        │
└─────────────────────────────────────────────────────────────┘
        │                  │                    │
        │ gRPC             │ HTTPS              │ HTTPS
        ▼                  ▼                    ▼
   ┌─────────┐      ┌─────────────┐      ┌──────────────────┐
   │ Milvus  │      │ BGE-M3      │      │ GLM-5.2 (Coding  │
   │ (内网)  │      │ (内网)      │      │ Plan, public)    │
   │ tier    │      │ /v1/embed   │      │ /api/coding/paas │
   │ filter  │      │ 1024-dim    │      │ /v4              │
   └─────────┘      └─────────────┘      └──────────────────┘
```

## Standard Stack

### Core (locked — verified at exact pinned versions)

| Library | Version | Purpose | Why Standard | Confidence |
|---------|---------|---------|--------------|------------|
| `github.com/cloudwego/eino` | v0.7.7 | ADK: `NewChatModelAgent`, `NewRunner`, `Runner.Query`, `WithChatModelOptions` | All four APIs verified at exact source paths in `~/go/pkg/mod/github.com/cloudwego/eino@v0.7.7/adk/{chatmodel.go:212, runner.go:59, runner.go:98, chatmodel.go:54}` | HIGH [VERIFIED: eino@v0.7.7 source] |
| `github.com/cloudwego/eino-ext/components/model/openai` | v0.1.5 | OpenAI-compat adapter for GLM-5.2 Coding Plan endpoint | `openai.NewChatModel` + `openai.WithExtraFields` proven in spike 001/003/e2e.go; Coding Plan endpoint confirmed | HIGH [VERIFIED: spike 003 source + e2e.go] |
| `github.com/cloudwego/hertz` | v0.10.6 | HTTP server + SSE writer | `server.WithHostPorts` verified at `pkg/app/server/option.go:158-159`; `sse.NewWriter` + `WriteEvent` verified at `pkg/protocol/sse/writer.go:41,46,64` | HIGH [VERIFIED: hertz@v0.10.6 source] |
| `github.com/cloudwego/hertz/pkg/protocol/sse` | bundled with hertz | SSE frame writer (`text/event-stream`) | `spike.EmitChunk` / `spike.StreamChunks` in `internal/spike/hertz_sse.go` already use it correctly; graduates directly | HIGH [VERIFIED: spike hertz_sse.go + writer.go] |
| `github.com/milvus-io/milvus-sdk-go/v2` | v2.4.2 | Milvus client (positional Search API) | `BuildMilvusFilter` + `SearchWithTierFilter` + `EnsureCollection` + `SeedDocuments` proven in spike; positional Search API verified at v2.4.2 (4th arg = filter) | HIGH [VERIFIED: spike milvus_client.go + SDK source] |
| `gopkg.in/yaml.v3` | v3.0.1 | SKILL.md YAML frontmatter parsing | Already a direct dep; spike's `parseSKILL` in `cmd/spiked/wire.go` proves the frontmatter-extraction pattern | HIGH [VERIFIED: go.mod + spike wire.go] |
| `golang.org/x/text/width` | v0.13.0 (promote indirect → direct) | 全角→半角 punctuation normalization (D2-05) | `width.Narrow` Transformer verified at `~/go/pkg/mod/golang.org/x/text@v0.13.0/width/width.go:195-197`; `width.Narrow.String(s)` does fullwidth→halfwidth. Currently `// indirect` in go.mod — Phase 2 promotes it | HIGH [VERIFIED: x/text@v0.13.0 source] |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | Already a direct dep; used throughout spike tests + contract test | HIGH [VERIFIED: go.mod] |

### Supporting (Phase 2 adds — all stdlib or first-party Go)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net/http` (stdlib) | go1.24.5 | BGE-M3 embedding HTTP client | Reuse `spike.EmbedQuery` HTTP POST pattern (OpenAI `/v1/embeddings` shape) |
| `path/filepath` (stdlib) | go1.24.5 | Walk `skills/*/SKILL.md` + `knowledge/{tier}/*.md` | `filepath.WalkDir` for the loader + indexer scans |
| `os` (stdlib) | go1.24.5 | Read SKILL.md + knowledge docs + env vars (`MILVUS_ADDRESS`, `BGE_M3_URL`, `GLM_API_KEY`) | Startup config + file reads |
| `encoding/json` (stdlib) | go1.24.5 | Verdict JSON parse, UIMessageChunk marshal | Verdict pipeline + SSE emission |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `golang.org/x/text/width.Narrow` for 全角→半角 | Hand-rolled rune map | Hand-roll misses edge cases (U+FF5E wave dash, U+3001 ideographic comma); `width.Narrow` is the Go-first-party canonical transform. **Use width.Narrow.** |
| `eino-ext/components/document/transformer/splitter/markdown` for chunking | One-chunk-per-file | Phase 2 corpus is ~30 small clauses (≤800 bytes each); a full hierarchical splitter is over-engineering. **Defer the splitter to Phase 3** when corpus grows. One-doc-one-chunk suffices for Phase 2's 5 queries |
| `hertz-contrib/cors` for CORS | Manual `Access-Control-Allow-Origin` header | Phase 2 has NO frontend (Phase 4 adds React). CORS is YAGNI now. **Defer to Phase 4.** curl/test-driven clients don't need CORS |
| Hertz `h.Spin()` (signal-based graceful shutdown) | `h.Run()` (blocks until `h.Close()`) | `Spin()` traps SIGINT/SIGTERM — correct for a long-running server binary. `Run()` is what the spike used because it needed programmatic `Close()`. **Use `Spin()` in `cmd/server`** |
| `cmd/spiked` retained as regression binary | Delete it | CONTEXT D2-09 + specifics explicitly require keeping it runnable. **Retain.** |

**Installation (Phase 2):**
```bash
# Only NEW direct dep (currently indirect at v0.13.0 — promote it)
go get golang.org/x/text@v0.13.0

# All others are already in go.mod at the pinned versions; no changes.
# Do NOT bump eino / hertz / eino-ext / milvus-sdk-go — the contract test
# (integration/contract/contract_test.go:82-105) pins them exactly.
```

**Version verification performed:**
- `go.mod` confirmed: eino v0.7.7, eino-ext openai v0.1.5, hertz v0.10.6, milvus-sdk-go v2.4.2, testify v1.11.1, yaml.v3 v3.0.1 — all already direct deps.
- `golang.org/x/text v0.13.0 // indirect` confirmed in go.mod (transitive via eino/hertz). Module cache has v0.13.0 through v0.40.0; v0.13.0 is the version already resolved, so promoting it avoids an unrelated bump.
- `~/go/pkg/mod/github.com/cloudwego/hertz@v0.10.6/pkg/app/server/option.go:158-159` confirmed `WithHostPorts`.
- `~/go/pkg/mod/golang.org/x/text@v0.13.0/width/width.go:184-197` confirmed `var Narrow Transformer = Transformer{narrowTransform{}}`.

## Package Legitimacy Audit

> **slopcheck was unavailable** (pip install failed — no public PyPI reach in this sandbox). Per the graceful-degradation protocol, **all packages below are tagged `[ASSUMED]`** and the planner must gate the single new direct dependency (`golang.org/x/text`) behind a `checkpoint:human-verify` task before `go mod tidy`. Every OTHER package is already pinned in `go.mod` and verified against the exact Go module source in the local cache — those are tagged `[VERIFIED]` because the source code was directly inspected, not just the registry.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `golang.org/x/text` | go.dev (proxy.golang.org) | 12+ yrs (Go first-party) | MASSIVE (every Go project) | `golang.org/x/text` (Google-maintained) | unavailable → `[ASSUMED]` | Planner adds `checkpoint:human-verify` before `go get`. Extremely low risk — it is already `// indirect` in go.mod at v0.13.0 (transitive via eino/hertz); Phase 2 just promotes it to a direct require. |
| `github.com/cloudwego/eino` | go.dev | 1+ yr (v0.7.7 tagged) | ByteDance official | `github.com/cloudwego/eino` | unavailable | Already pinned in go.mod; source verified. **No new install.** |
| `github.com/cloudwego/eino-ext/components/model/openai` | go.dev | 1+ yr | ByteDance official | `github.com/cloudwego/eino-ext` | unavailable | Already pinned at v0.1.5; source verified. **No new install.** |
| `github.com/cloudwego/hertz` | go.dev | 4+ yrs | ByteDance official | `github.com/cloudwego/hertz` | unavailable | Already pinned at v0.10.6; `WithHostPorts` + `sse.NewWriter` source verified. **No new install.** |
| `github.com/milvus-io/milvus-sdk-go/v2` | go.dev | 2+ yrs | Milvus official | `github.com/milvus-io/milvus-sdk-go` | unavailable | Already pinned at v2.4.2; positional Search API verified. **No new install.** |
| `gopkg.in/yaml.v3` | go.dev | mature | ubiquitous | `github.com/go-yaml/yaml` | unavailable | Already pinned at v3.0.1. **No new install.** |
| `github.com/stretchr/testify` | go.dev | mature | ubiquitous | `github.com/stretchr/testify` | unavailable | Already pinned at v1.11.1. **No new install.** |

**Packages removed due to slopcheck `[SLOP]` verdict:** none (slopcheck unavailable; no new packages introduced beyond the Go first-party `x/text`).
**Packages flagged as suspicious `[SUS]`:** none.
**Packages needing `checkpoint:human-verify`:** `golang.org/x/text@v0.13.0` (only because slopcheck was unavailable; it is Go-first-party and already transitively resolved, so risk is negligible).

*No `npm view` / `pip index versions` was run because Phase 2 adds no npm or PyPI dependencies (frontend is Phase 4; Python eval sidecar is Phase 3).*

## Architecture Patterns

### Recommended Project Structure (Phase 2 graduation)

```
ai-agent/
├── cmd/
│   ├── spiked/              # RETAINED — regression/spike binary (wire/sse/milvus/e2e subcommands)
│   │   └── main.go          # updated imports only (spike.* → verdict.*/wire.*/retriever.*)
│   └── server/              # NEW — long-running Hertz server (D2-09)
│       └── main.go          # server.Default(WithHostPorts(":8888")); h.Spin()
├── internal/
│   ├── spike/               # DEPRECATED — empty or thin re-export shim (do NOT keep parallel copy)
│   ├── verdict/             # NEW — graduates spike/verdict.go + D2-06 upgrade
│   │   ├── verdict.go       # Verdict{Status, Confidence, Citations []Citation, ReviewRequired}
│   │   ├── citation.go      # Citation{ChunkID, Span, Tier} + Refusal{Reason, CanRephrase, Suggested}
│   │   ├── validate.go      # Validate() + ValidStatuses (reused from spike)
│   │   └── grounding.go     # NEW — GroundingChecker (D2-01/D2-05): normalize + substring-contains
│   ├── wire/                # NEW — graduates spike/uimsg.go + spike/hertz_sse.go
│   │   ├── chunk.go         # UIMessageChunk, ValidTypes, IsValidType (reused verbatim)
│   │   └── sse.go           # EmitChunk, StreamChunks (reused verbatim)
│   ├── retriever/           # NEW — graduates spike/milvus_client.go
│   │   ├── client.go        # MilvusConfig, NewMilvusClient, EnsureCollection, SeedDocuments
│   │   ├── filter.go        # BuildMilvusFilter, tier enum map {authority:权威, ...}
│   │   ├── search.go        # SearchWithTierFilter, TierSearchResult, Document
│   │   └── embed.go         # EmbedQuery, EmbedQueryDeterministic (BGE-M3)
│   ├── skill/               # NEW — SKILL.md loader + 7-field validator
│   │   ├── manifest.go      # Manifest struct (7 fields) + Guardrails struct
│   │   ├── loader.go        # Scan(skillsDir): filepath.WalkDir + parse + validate + tier resolution
│   │   └── validate.go      # Validate(m *Manifest) error — SYSK-02 rules
│   ├── knowledge/           # NEW — corpus indexer (D2-11/D2-12)
│   │   └── indexer.go       # Index(knowledgeDir): scan knowledge/{tier}/*.md → embed → insert Milvus
│   └── server/              # NEW — Hertz handlers + verdict pipeline orchestration
│       ├── server.go        # New(addr) *Server; routes POST /api/chat, GET /api/skills, POST /api/skills
│       ├── chat.go          # chatHandler: the verdict pipeline (steps 1-8 in the diagram)
│       ├── skills.go        # listSkillsHandler, validateSkillHandler (dry-run)
│       └── pipeline.go      # VerdictPipeline: embed → retrieve → refuse-or-LLM → ground → emit
├── skills/
│   └── compliance/
│       └── SKILL.md         # NEW — the 7-field compliance Skill (showcase, CMPL-01)
├── knowledge/
│   ├── authority/           # migrated from testdata/docs/authority/ + expanded (D2-11/D2-12)
│   │   ├── travel_expense.md        # (was auth_001.md) — query 1
│   │   ├── contract_approval.md     # (was auth_002.md) — query 3
│   │   ├── non_compete.md           # (was auth_003.md) — query 2
│   │   ├── annual_leave.md          # NEW — query 4
│   │   └── ... (5-10 docs, ~30 chunks total)
│   ├── historical/          # empty in Phase 2 (stub for tier-bleed test fixtures)
│   ├── personal/            # empty in Phase 2 (the contact-info refusal case targets this gap)
│   └── draft/               # empty in Phase 2
├── integration/
│   └── contract/
│       └── contract_test.go # UPDATED — imports graduate to verdict.*/wire.*/retriever.*; []Citation shape
└── testdata/                # RETAINED — spike fixtures stay for cmd/spiked regression
```

### Pattern 1: Hertz long-running server with fixed-port binding
**What:** A Hertz server that binds a fixed port (default `:8888`) and runs until SIGINT. This is the Phase 2 production server (D2-09), distinct from the spike's dynamic-port `WithListener` pattern.

**When to use:** Any long-running SkillForge service. The spike's `server.Default(server.WithListener(ln))` (dynamic port, programmatic `Close()`) stays in `cmd/spiked` for test isolation; the production server uses `WithHostPorts` + `Spin()`.

**Example (verified against hertz@v0.10.6/pkg/app/server/option.go:158-159):**
```go
// internal/server/server.go
package server

import (
    "github.com/cloudwego/hertz/pkg/app/server"
)

func New(addr string, deps *Deps) *server.Hertz {
    h := server.Default(server.WithHostPorts(addr)) // verified: option.go:158
    // Routes (handlers live in chat.go / skills.go)
    h.POST("/api/chat", deps.ChatHandler)
    h.GET("/api/skills", deps.ListSkillsHandler)
    h.POST("/api/skills", deps.ValidateSkillHandler) // dry-run, does NOT persist
    return h
}

// cmd/server/main.go
func main() {
    deps := buildDeps() // load skills, dial Milvus, build ChatModel
    h := server.New(":8888", deps)
    h.Spin() // blocks until SIGINT/SIGTERM, then graceful shutdown
}
```

### Pattern 2: SSE streaming through Hertz (graduated from spike)
**What:** Emit `UIMessageChunk` frames as SSE `data:` lines. The spike proved the exact API at `hertz@v0.10.6/pkg/protocol/sse/writer.go:41,46,64`; `EmitChunk` / `StreamChunks` in `internal/spike/hertz_sse.go` graduate verbatim into `internal/wire/sse.go`.

**When to use:** Every streaming endpoint. For Phase 2 the `POST /api/chat` handler emits: `source-document` (per citation, SC-6), `data-policy` (the Verdict), `finish-step`.

**Example (verified — graduates from internal/spike/hertz_sse.go:76-89):**
```go
// internal/server/chat.go — the SSE emission tail of the verdict pipeline
func emitVerdictStream(c *app.RequestContext, v verdict.Verdict, chunks []retriever.TierSearchResult) error {
    // SC-6: one source-document chunk per citation (tier + chunk_id on the wire)
    for _, cit := range v.Citations {
        chunk := spike.UIMessageChunk{
            Type:    "source-document",
            ID:      "src-" + cit.ChunkID,
            SourceID: cit.ChunkID,
            Title:   sourcePathFor(chunks, cit.ChunkID),
            // tier rides in the data-policy Verdict payload (keeps source-document AI-SDK-v5-clean)
        }
        if err := wire.EmitChunk(c, chunk); err != nil {
            return fmt.Errorf("emit source-document for %s: %w", cit.ChunkID, err)
        }
    }
    // CMPL-02: the Verdict itself as a custom data-policy chunk
    payload, _ := json.Marshal(v)
    if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-policy", Data: payload}); err != nil {
        return fmt.Errorf("emit data-policy verdict: %w", err)
    }
    return wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
}
```

### Pattern 3: SKILL.md 7-field frontmatter parsing + validation
**What:** Parse YAML frontmatter (between `---` markers) out of a Markdown file, validate against the 7-field schema, resolve `reads` patterns to Milvus tier values. Graduates the spike's `parseSKILL` (`cmd/spiked/wire.go`) into `internal/skill`.

**When to use:** At server startup (load `skills/compliance/SKILL.md`) and at `POST /api/skills` (dry-run validate an uploaded Skill).

**Example (frontmatter extraction graduates from cmd/spiked/wire.go:parseSKILL):**
```go
// internal/skill/manifest.go
package skill

type Manifest struct {
    Description string    `yaml:"description"` // REQUIRED, non-empty
    Agent       string    `yaml:"agent"`        // REQUIRED, ∈ {ChatModelAgent, ReActAgent, SequentialAgent, ...}
    Reads       []string  `yaml:"reads"`        // REQUIRED, non-empty, no "*" wildcard
    Writes      []string  `yaml:"writes"`       // REQUIRED (can be [])
    Steps       string    `yaml:"steps"`        // REQUIRED, markdown ordered list (string form per SKILL_SCHEMA.md)
    Guardrails  Guardrails `yaml:"guardrails"`  // REQUIRED, prohibitions ≥3
    Temperature float32   `yaml:"temperature"`  // REQUIRED, [0.0, 1.0], NO default
}

type Guardrails struct {
    Prohibitions         []string            `yaml:"prohibitions"`          // ≥3 (P-SKILL-2)
    RequiredRefusalPhrases []string          `yaml:"required_refusal_phrases"`
    TierLimits           map[string][]string `yaml:"tier_limits"`
}

// internal/skill/validate.go — SYSK-02 enforcement
func Validate(m *Manifest) error {
    if strings.TrimSpace(m.Description) == "" {
        return errors.New("description is required (SKILL_SCHEMA field 1)")
    }
    if !validAgentTypes[m.Agent] {
        return fmt.Errorf("agent %q not a supported Eino ADK type", m.Agent)
    }
    if len(m.Reads) == 0 {
        return errors.New("reads is required and non-empty (default-deny, P-SKILL-3)")
    }
    for _, r := range m.Reads {
        if r == "*" {
            return errors.New(`reads: '*' wildcard rejected at load time (P-SKILL-3 anti-pattern)`)
        }
    }
    if m.Writes == nil {
        return errors.New("writes is required (use [] for read-only)")
    }
    if strings.TrimSpace(m.Steps) == "" {
        return errors.New("steps is required (declarative markdown ordered list)")
    }
    if len(m.Guardrails.Prohibitions) < 3 {
        return fmt.Errorf("guardrails.prohibitions must have ≥3 entries (P-SKILL-2), got %d", len(m.Guardrails.Prohibitions))
    }
    // SYSK-08: temperature required, no default
    if m.Temperature < 0.0 || m.Temperature > 1.0 {
        return fmt.Errorf("temperature must be [0.0, 1.0], got %f (SYSK-08)", m.Temperature)
    }
    return nil
}
```

### Pattern 4: `reads` pattern → Milvus tier value resolution
**What:** Convert a Skill's `reads: [authority/**]` into the Milvus tier filter value `权威`. The top-level directory name in the reads pattern maps to the CN-char tier enum.

**When to use:** At load time (precompute the resolved tier list) and at retrieval time (pass to `BuildMilvusFilter`).

```go
// internal/retriever/filter.go (extends the graduated spike/milvus_client.go)
package retriever

// TierEnum maps reads-pattern top-level dir → Milvus tier VARCHAR value.
// The Milvus collection stores tier as the CN-char value (D-07); the SKILL.md
// reads pattern uses the ASCII dir name (D2-12). This map is the ONLY place
// the two worlds meet — a typo here breaks the trust boundary silently.
var TierEnum = map[string]string{
    "authority":  "权威",
    "historical": "历史",
    "personal":   "个人",
    "draft":      "草稿",
}

// ResolveTiers converts reads patterns (e.g. "authority/**", "authority/policies/**")
// to the unique set of Milvus tier values the retriever may query.
// Unknown top-level dirs → error (default-deny: refuse rather than guess).
func ResolveTiers(reads []string) ([]string, error) {
    seen := map[string]bool{}
    var out []string
    for _, r := range reads {
        top := strings.SplitN(r, "/", 2)[0] // "authority/policies/**" → "authority"
        tier, ok := TierEnum[top]
        if !ok {
            return nil, fmt.Errorf("reads %q: unknown tier dir %q (known: %v)", r, top, knownTierDirs())
        }
        if !seen[tier] {
            seen[tier] = true
            out = append(out, tier)
        }
    }
    return out, nil
}

// BuildMilvusFilter graduates unchanged from spike/milvus_client.go:139-154.
// Empty/nil tiers → "false" (default-deny D-09).
```

### Pattern 5: Programmatic grounding checker (the D2-01 moat)
**What:** A pure Go function that verifies each citation's `span` literally appears in the retrieved chunk text after normalization (collapse whitespace + 全角→半角 + trim). This is the deterministic, independently-unit-testable hallucination barrier — NOT delegated to the LLM.

**When to use:** After parsing the LLM's verdict JSON, BEFORE emitting SSE. Every citation runs through this check.

**Example (verified against golang.org/x/text@v0.13.0/width/width.go:195-197):**
```go
// internal/verdict/grounding.go
package verdict

import (
    "strings"
    "golang.org/x/text/width"
)

// Citation is the D2-04 minimal citation object.
type Citation struct {
    ChunkID string `json:"chunk_id"`
    Span    string `json:"span"`         // verbatim quote (D2-03)
    Tier    string `json:"tier"`         // e.g. "权威"
}

// Normalize applies the D2-05 transformation: collapse whitespace +
// 全角→半角 punctuation + trim. Applied to BOTH span and chunk text before
// the substring-contains check so CN punctuation mixing (，vs ,) and
// incidental whitespace do not cause false-negatives. NOT fuzzy match.
func Normalize(s string) string {
    // 1. 全角 → 半角 (fullwidth/halfwidth → narrow). width.Narrow maps
    //    U+FF0C (，) → U+002C (,), U+3001 (、) → U+002C, U+FF08 → (, etc.
    half := width.Narrow.String(s) // verified: x/text@v0.13.0/width/width.go:195
    // 2. Collapse all whitespace runs to a single space.
    collapsed := strings.Join(strings.Fields(half), " ")
    // 3. Trim.
    return strings.TrimSpace(collapsed)
}

// GroundingChecker verifies each citation in v against the retrieved chunks.
// Returns a possibly-modified verdict (D2-02 downgrade applied to ungrounded
// claims) plus a list of ungrounded chunk IDs (for audit/logging).
//
// chunksByID is keyed by ChunkID → chunk text. A citation whose ChunkID is
// NOT in the map (novel claim, per Claude's discretion) is treated as
// ungrounded via the same D2-02 path.
func GroundingChecker(v Verdict, chunksByID map[string]string) (Verdict, []string) {
    var ungrounded []string
    anyUngrounded := false
    for i := range v.Citations {
        cit := &v.Citations[i]
        chunkText, ok := chunksByID[cit.ChunkID]
        if !ok {
            // Novel claim: chunk_id not in retrieved set.
            ungrounded = append(ungrounded, cit.ChunkID)
            anyUngrounded = true
            continue
        }
        if !strings.Contains(Normalize(chunkText), Normalize(cit.Span)) {
            ungrounded = append(ungrounded, cit.ChunkID)
            anyUngrounded = true
        }
    }
    if anyUngrounded {
        // D2-02: keep the grounded conclusion, force review, cap confidence.
        v.ReviewRequired = true
        if v.Confidence >= 0.9 {
            v.Confidence = 0.89 // cap just below the 0.9 review threshold (SC-5)
        }
        if v.Status == "合规" || v.Status == "不合规" {
            // Downgrade the conclusion to 需补充 (info insufficient) — reuses
            // the existing status per CMPL-03; does NOT silently drop.
            v.Status = "需补充"
        }
    }
    return v, ungrounded
}
```

### Pattern 6: Verdict pipeline composition (POST /api/chat handler)
**What:** The orchestrator that composes retrieval → refuse-or-LLM → ground → emit. This is where Phase 1's disconnected pieces (verdict e2e + Milvus tier filter) finally connect.

**When to use:** The `POST /api/chat` handler. Single entry point for the whole vertical slice.

```go
// internal/server/pipeline.go (skeletal — planner fills in deps)
func (s *Server) VerdictPipeline(ctx context.Context, c *app.RequestContext, query string, skill *skill.Manifest) error {
    // 1. Resolve tiers from skill.Reads → []string{"权威"}
    tiers, err := retriever.ResolveTiers(skill.Reads)
    if err != nil {
        return emitRefusal(c, verdict.Refusal{Reason: "out_of_scope", CanRephrase: false})
    }
    // 2. Embed query (BGE-M3 via retriever.EmbedQuery — proven in spike)
    qvec, err := s.embedder.EmbedQuery(ctx, s.bgeURL, query)
    if err != nil {
        return fmt.Errorf("embed query: %w", err)
    }
    // 3. Tier-filtered retrieval (PRE-similarity filter, proven in spike)
    filter := retriever.BuildMilvusFilter(tiers)
    chunks, err := retriever.SearchWithTierFilter(ctx, s.milvus, s.mcfg, qvec, filter, 5)
    if err != nil {
        return fmt.Errorf("retrieve: %w", err)
    }
    // 4. SC-2 / GEVL-02: 0 chunks → structured refusal BEFORE any LLM call
    if len(chunks) == 0 {
        return emitRefusal(c, verdict.Refusal{
            Reason: "no_authorized_source", CanRephrase: true,
            SuggestedAlternativeQuery: altQueryFor(query, tiers),
        })
    }
    // 5. GEVL-01 / D2-15: low-score refusal (placeholder threshold, Phase 3 calibrates)
    if maxScore(chunks) < 0.3 {
        return emitRefusal(c, verdict.Refusal{Reason: "low_retrieval_score", CanRephrase: true})
    }
    // 6. Build prompt: system + injected chunks + query (NEW prompt vs spike —
    //    the citation object is now {chunk_id, span, tier}, not a clause name)
    prompt := buildVerdictPrompt(query, chunks)
    // 7. ChatModelAgent verdict (temp=skill.Temperature, json_object, thinking:disabled)
    //    Reuses the EXACT call shape proven in cmd/spiked/e2e.go:198-214
    iter := s.runner.Query(ctx, prompt,
        adk.WithChatModelOptions([]model.Option{
            model.WithTemperature(float32(skill.Temperature)), // SYSK-08
            model.WithMaxTokens(600),
            openai.WithExtraFields(map[string]any{
                "response_format": map[string]string{"type": "json_object"},
                "thinking":        map[string]string{"type": "disabled"},
            }),
        }),
    )
    raw, err := drainFinalContent(iter) // graduates from cmd/spiked/e2e.go:349-374
    if err != nil {
        return fmt.Errorf("drain LLM: %w", err)
    }
    var v verdict.Verdict
    if err := json.Unmarshal([]byte(raw), &v); err != nil {
        return fmt.Errorf("parse verdict: %w (raw: %s)", err, truncate(raw, 200))
    }
    if ok, reason := verdict.Validate(v); !ok {
        return fmt.Errorf("invalid verdict: %s", reason)
    }
    // 8. D2-01: programmatic grounding check
    chunksByID := map[string]string{}
    for _, ch := range chunks {
        chunksByID[strconv.FormatInt(ch.ID, 10)] = ch.Content
    }
    v, ungrounded := verdict.GroundingChecker(v, chunksByID)
    // 9. Emit SSE stream (Pattern 2)
    return emitVerdictStream(c, v, chunks)
}
```

### Pattern 7: One-doc-one-chunk knowledge indexer (Phase 2 minimal)
**What:** Scan `knowledge/{tier}/*.md`, strip YAML frontmatter, embed the body, insert into Milvus with tier metadata. Each `.md` file becomes ONE chunk. The full markdown hierarchical splitter is Phase 3.

**Why one-chunk-per-file is sufficient for Phase 2:** D2-11 specifies "~30 chunks across 5-10 docs" — each doc is a single clause (~300-800 bytes, well under BGE-M3's 8192-token context). The existing `testdata/docs/authority/auth_001.md` (325 bytes, one clause) is the size template. Splitting these further would fragment the clause and HURT retrieval precision. Phase 3's larger corpus needs the splitter; Phase 2's hand-authored clauses do not.

```go
// internal/knowledge/indexer.go
func Index(ctx context.Context, knowledgeDir string, mc client.Client, cfg retriever.MilvusConfig, bgeURL string) error {
    tiers := []string{"authority", "historical", "personal", "draft"}
    var docs []retriever.Document
    nextID := int64(1)
    for _, tierDir := range tiers {
        tierVal, _ := retriever.TierEnum[tierDir] // "权威" etc.
        files, _ := filepath.Glob(filepath.Join(knowledgeDir, tierDir, "*.md"))
        for _, f := range files {
            raw, _ := os.ReadFile(f)
            body := stripFrontmatter(string(raw)) // reuse parseSKILL's --- splitter
            vec, err := retriever.EmbedQuery(ctx, bgeURL, body) // BGE-M3, proven
            if err != nil {
                return fmt.Errorf("embed %s: %w", f, err)
            }
            docs = append(docs, retriever.Document{
                ID: nextID, Tier: tierVal, Content: body,
                Source: filepath.ToSlash(strings.TrimPrefix(f, knowledgeDir+string(filepath.Separator))),
                Embedding: vec,
            })
            nextID++
        }
    }
    return retriever.SeedDocuments(ctx, mc, cfg, docs) // proven in spike
}
```

### Anti-Patterns to Avoid

- **Copying spike code into production packages and leaving the spike copy in place.** Two `Verdict` types drift immediately. Graduate by MOVE + update all importers (`cmd/spiked`, `contract_test.go`) in one change so the T-1-16 single-source-of-truth guarantee survives.
- **Putting the grounding check in the LLM prompt (asking the model to self-verify).** D2-01 explicitly forbids this: "The hallucination moat must NOT rely on the LLM policing itself." The check is pure Go.
- **Post-filtering retrieval results by tier (filtering after Milvus returns).** P-RAG-1. The tier filter MUST be PRE-similarity via the SDK `Search` `expr` arg (4th positional param, verified `milvus_client.go:350-362`). The spike already does this correctly — do not regress.
- **Defaulting `temperature` when it's missing.** SYSK-08 requires it to be REQUIRED (no default). A Skill with no temperature must FAIL validation, not silently default to 0.0.
- **Using fuzzy/edit-distance matching for the grounding check.** D2-05 explicitly says normalized substring-contains, NOT Levenshtein. Fuzzy match erodes the moat (a 1-edit-distance fabrication passes).
- **Bumping eino / hertz / eino-ext / milvus-sdk-go versions.** The contract test (`integration/contract/contract_test.go:82-105`) pins them exactly. A bump fails SC-5 and silently changes behavior. Phase 2 uses the SAME versions as Phase 1.
- **Hardcoding the GLM model name in log strings.** Per the temperature-field-validation skill (`temperature-field-validation.md` §"What to Avoid" #5): spike 002 had `(glm-4-flash)` hardcoded while `Model` was `glm-5.2`. Read from config.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 全角→半角 normalization (D2-05) | Hand-rolled rune map for U+FF00-U+FFEF + U+3000-U+303F | `golang.org/x/text/width.Narrow.String(s)` | Hand-roll misses edge cases (U+FF5E fullwidth tilde, U+3001 ideographic comma, U+FF65 halfwidth katakana middle dot); `width.Narrow` is the Go-first-party canonical East Asian width transform, covers the full Unicode width table |
| SSE frame writing | Manual `Content-Type` + chunked encoding + `data: ...\n\n` formatting | `sse.NewWriter(c).WriteEvent("", "message", payload)` (graduated into `wire.EmitChunk`) | Hertz sets headers correctly and flushes per-frame (verified `writer.go:46,157`); the spike already proved this — manual re-implementation risks header/flush bugs |
| Milvus tier filter expression | String concatenation of tier values | `retriever.BuildMilvusFilter(tiers)` (graduates from spike) | `%q` escaping handles CN chars + hostile input (apostrophes); empty → `"false"` default-deny (D-09). Already contract-tested |
| YAML frontmatter parsing | Regex on `---` markers | `gopkg.in/yaml.v3` + the spike's `parseSKILL` pattern | yaml.v3 handles edge cases (multi-line strings, quoted CN text); the spike already proved the extraction |
| GLM-5.2 structured output | Custom JSON parsing with tolerance | `response_format:{type:json_object}` + `verdict.Validate()` | Spike 003 proved 100% schema-valid output across 50 invocations; app-side `Validate()` is the second gate. Do NOT use `response_format:{type:json_schema}` (skill finding: GLM-5.2 may not support full schema validation) |
| BGE-M3 embedding HTTP client | Custom HTTP plumbing | `retriever.EmbedQuery` (graduates from spike) | Handles OpenAI-shape + TEI bare-array responses, retries, bearer auth. Already proven against the internal BGE-M3 endpoint |
| Eino ChatModelAgent + Runner | Manual message/role orchestration | `adk.NewChatModelAgent` + `adk.NewRunner` + `runner.Query` | Proven in spike e2e.go with the EXACT option shape needed (`WithTemperature` + `WithExtraFields{response_format, thinking}`) |

**Key insight:** Phase 2's deceptively complex problems (SSE framing, CN-char filter escaping, 全角→半角 normalization, OpenAI-compat structured output) are ALL already solved by either the spike or a first-party Go library. The only genuinely new logic is the grounding checker (Pattern 5) and the pipeline composition (Pattern 6) — both are pure Go orchestration, no library needed.

## Common Pitfalls

### Pitfall 1: Copying spike code instead of graduating it (drift)
**What goes wrong:** Two `Verdict` types exist — `spike.Verdict` (Citations []string) and `verdict.Verdict` (Citations []Citation). The contract test pins one shape; production emits the other. They silently diverge.
**Why it happens:** Copying feels safer than moving (don't break the spike). But the spike's whole value is being the regression contract; a parallel copy defeats it.
**How to avoid:** MOVE the code into `internal/{verdict,wire,retriever}/`, update `cmd/spiked` + `contract_test.go` imports in the SAME change. The D2-06 `[]string` → `[]Citation` upgrade forces this regardless.
**Warning signs:** `grep -r "type Verdict struct" internal/` returns more than one definition.

### Pitfall 2: Retrieving then filtering by tier (P-RAG-1 regression)
**What goes wrong:** Code retrieves top-K from Milvus with no filter, then drops non-authority chunks in Go. This leaks 个人/草稿 chunks into the prompt if the post-filter has a bug, AND wastes retrieval budget on irrelevant tiers.
**Why it happens:** Calling `Search` with `filter=""` (no filter) is simpler than building the expression.
**How to avoid:** ALWAYS pass `BuildMilvusFilter(tiers)` as the 4th positional arg to `mc.Search` (verified at `milvus_client.go:350-362`). The filter is PRE-similarity at the index scan.
**Warning signs:** Audit log shows retrieved chunks whose tier ∉ the Skill's resolved `reads` tiers.

### Pitfall 3: Grounding check delegated to the LLM (D2-01 violation)
**What goes wrong:** The system prompt says "only cite chunks you verified" and the code trusts the output. A fluent fabrication passes because the LLM is confident.
**Why it happens:** LLM self-verification feels sufficient in demos (it works on the happy path). It fails on adversarial queries.
**How to avoid:** The `GroundingChecker` pure-Go function runs on EVERY citation AFTER the LLM emits. Unit-test it independently with fabricated spans that do NOT appear in the chunk — they must fail.
**Warning signs:** No `verdict.GroundingChecker` call in the pipeline; or it's called but its result is ignored.

### Pitfall 4: Temperature defaulting to 0.0 when missing (SYSK-08 violation)
**What goes wrong:** A Skill author forgets `temperature:` and the loader silently sets 0.0. The Skill runs deterministically by accident, but the author never declared the intent — future creative Skills inherit a wrong default.
**Why it happens:** Go's zero value for `float32` is 0.0; `yaml.Unmarshal` into a `float32` field leaves it 0.0 if the key is absent. This is the trap.
**How to avoid:** Validate `m.Temperature` is present by parsing into a map first OR by checking the raw YAML contains the key. The cleanest: define `Temperature *float32` (pointer) — nil means missing → reject. Alternatively parse the frontmatter into `map[string]any` and assert `"temperature"` key exists before re-decoding into the struct.
**Warning signs:** A SKILL.md with no `temperature:` line loads successfully.

### Pitfall 5: Default-allow on empty `reads` (P-SKILL-3 regression)
**What goes wrong:** A Skill with `reads: []` (or missing) retrieves from ALL tiers. The moat collapses.
**Why it happens:** `BuildMilvusFilter([]string{})` returns `"false"` (default-deny) — BUT if the loader skips validation and the pipeline calls retrieval with an empty tier list resolved from an empty reads, the filter correctly returns 0 chunks. The trap is if the pipeline falls back to "no filter" when it sees 0 chunks (a misguided "be helpful" fallback).
**How to avoid:** `skill.Validate` rejects empty `reads` at load time. AND `BuildMilvusFilter(nil)` → `"false"` is the structural backstop (already proven). NEVER add a "retry without filter" fallback.
**Warning signs:** A Skill with `reads: []` loads; or 0-chunks retrieval triggers an unfiltered retry.

### Pitfall 6: GLM Coding Plan endpoint vs standard endpoint (skill finding)
**What goes wrong:** Using `https://open.bigmodel.cn/api/paas/v4` (standard) with a Coding Plan key returns `{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`.
**Why it happens:** Two endpoints look identical; the Coding Plan subscription scopes keys to `/api/coding/paas/v4`.
**How to avoid:** Default `GLM_BASE_URL` to `https://open.bigmodel.cn/api/coding/paas/v4` (the spike already does this at `cmd/spiked/e2e.go:132`). Document in `.env.example` (Phase 7 DIST-03).
**Warning signs:** GLM returns code 1113 with a valid key.

### Pitfall 7: Hardcoding `:8888` without env override
**What goes wrong:** Port conflicts on shared dev hosts; the spike already hit this with `:18080` (skill finding #7).
**Why it happens:** Copying `server.WithHostPorts(":8888")` verbatim.
**How to avoid:** Read `SERVER_ADDR` env var with `:8888` default: `addr := envOr("SERVER_ADDR", ":8888")`. Same pattern for `MILVUS_ADDRESS`, `BGE_M3_URL`, `GLM_API_KEY`.
**Warning signs:** `netstat -ano | grep :8888` shows a conflict before the server starts.

### Pitfall 8: BGE-M3 outage masked by deterministic fallback
**What goes wrong:** `EmbedQueryDeterministic` (SHA-256-seeded pseudo-random vector) silently kicks in when BGE-M3 is down. Retrieval still "works" but returns semantically meaningless results — the moat fails open.
**Why it happens:** The fallback exists for spike SC-2 (tier-filter assertion without embedding service). Production must NOT inherit it.
**How to avoid:** `cmd/server` calls `retriever.EmbedQuery` ONLY; on error, refuse the query (do NOT fall back). `EmbedQueryDeterministic` stays in the package for `cmd/spiked` regression only — mark it clearly as spike-only.
**Warning signs:** Server keeps answering queries during a BGE-M3 outage.

## Code Examples

Verified patterns from the existing spike source + module cache.

### Compliance Skill (`skills/compliance/SKILL.md`) — reconciled to 7-field spec
```yaml
---
description: 企业合规/制度审查。当用户问"这个报销合规吗"、"这个条款怎么解读"、"出差住宿费超标"、"竞业限制期"等企业内部合规问题时触发。不处理外部法律法规咨询、销售话术、营销文案或个人隐私信息查询。
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Read relevant policy chunks via authority tier filter
  2. Compose draft verdict with citations (chunk_id + verbatim span)
  3. If no authority chunks retrieved, refuse with no_authorized_source
  4. Output structured verdict or structured refusal
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal/contact information of any individual"
  required_refusal_phrases:
    - "no_authorized_source"
    - "out_of_scope"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
---

# Compliance Review Skill

企业合规/制度审查。仅依据权威 tier（authority/**）回答现行有效制度问题。
所有引用必须为 chunk_id + 逐字 span，由 Go 程序化校验（grounding check）。
```

### Verdict pipeline prompt (the NEW system prompt — distinct from spike e2e.go)
```text
You are a strict enterprise compliance auditor. You will receive:
1. A user query (Chinese enterprise policy question)
2. Retrieved authority-tier policy chunks, each formatted as:
   [CHUNK <id>] <text>

Output ONLY a JSON object with this exact schema:
{
  "status": "合规" | "不合规" | "需补充" | "unanswered",
  "confidence": <0.0 to 1.0>,
  "citations": [
    {"chunk_id": "<id from a [CHUNK ...] block>", "span": "<verbatim substring from that chunk>", "tier": "权威"}
  ],
  "review_required": <true if confidence<0.9 OR any claim lacks a grounded citation>
}

RULES:
- span MUST be a verbatim substring copied from the cited chunk_id's text (it will be programmatically verified).
- If the query asks for personal/contact information, output status="unanswered" citations=[] review_required=true.
- If no chunks are provided, output status="unanswered" citations=[] review_required=true.
- NO text outside the JSON object.
```
*Note: the spike e2e.go prompt (lines 90-98) requested `"citations": [<clause names>]` (string array). Phase 2's prompt requests the structured `{chunk_id, span, tier}` object per D2-04/D2-06. This is a deliberate contract evolution — the spike prompt stays in `cmd/spiked/e2e.go` for regression; the production prompt lives in `internal/server/pipeline.go`.*

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `Verdict.Citations []string` (clause IDs) | `[]Citation{ChunkID, Span, Tier}` | Phase 2 (D2-06) | Enables programmatic grounding (D2-01); the spike's string citations were ungroundable. The contract test must be updated to the new shape. |
| Spike-era SKILL.md fixtures with `reasoning: false` + inline `reads: ["权威"]` + `guardrails.refuse_on_missing_tier` | 7-field spec with `temperature` + structured `guardrails` (prohibitions/required_refusal_phrases/tier_limits) | Phase 2 (D2-10) | The 4 `testdata/skills/*/SKILL.md` fixtures must be reconciled OR left as spike regression data; the production `skills/compliance/SKILL.md` is the canonical 7-field example |
| Phase 1 verdict + retrieval DISCONNECTED (e2e used synthetic citations; Milvus was a separate subcommand) | Retrieval plugs into the verdict pipeline as a pre-step | Phase 2 | This is THE Phase 2 thesis proof. The connection point is `internal/server/pipeline.go`. |
| `testdata/docs/{authority,...}/*.md` (3 docs/authority) | `knowledge/{authority,...}/*.md` (5-10 docs/authority, ~30 chunks) | Phase 2 (D2-11/D2-12) | Migrate + expand. The tier is derived from the top-level dir name (D2-12), not from YAML frontmatter (the spike used frontmatter `tier:` — the indexer should still respect frontmatter if present, but the dir name is the source of truth). |

**Deprecated/outdated in Phase 2 scope:**
- The spike's `cmd/spiked/e2e.go` hardcoded citation injection (lines 237-239: `v.Citations = []string{citationID}`) — production uses real retrieved chunk IDs. Retained in `cmd/spiked` only as the regression baseline.
- The spike-era `SKILLFrontmatter` struct in `cmd/spiked/wire.go` (6 fields) — superseded by `skill.Manifest` (7 fields).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `width.Narrow.String(s)` correctly converts the CN fullwidth punctuation that appears in compliance docs (，。、；：""（）) to halfwidth equivalents. Verified against `x/text@v0.13.0/width/width.go:195-197` source — the `narrowTransform` maps EastAsianFullwidth + EastAsianWide + EastAsianAmbiguous to narrow. | Pattern 5 (Grounding) | LOW — if a specific punctuation char is not converted, the grounding check would false-negative on that char. Mitigation: unit test with the exact punctuation set from the compliance corpus. |
| A2 | One-chunk-per-`.md`-file is sufficient retrieval granularity for Phase 2's 5-query corpus. Based on D2-11 ("~30 chunks across 5-10 docs") + the existing `auth_001.md` size (325 bytes). | Pattern 7 (Indexer) | LOW — if a doc is too large (>4000 chars), BGE-M3 embedding quality degrades. Mitigation: cap doc size at indexing time; warn on overflow. |
| A3 | Hertz `h.Spin()` provides signal-based graceful shutdown equivalent to calling `Run` + trapping SIGINT. Based on the Phase 1 RESEARCH note (`hertz.go:56-79` — "Spin() calls Run in a goroutine then waits on a signal before calling Shutdown/Close"). | Pattern 1 (Server) | LOW — if Spin behaves differently, the server still runs; only graceful shutdown is affected. Mitigation: verify in Wave 0 with a SIGINT test. |
| A4 | The Milvus collection name can remain `skillforge_spike_test` OR graduate to `skillforge_knowledge` without affecting the contract test. The spike's `DefaultCollectionName` const is in `milvus_client.go:66`. | Pattern 7 (Indexer) | LOW — the contract test does not assert the collection name (it asserts the filter expression shape). The graduation is a naming change. |
| A5 | The Eino ADK `runner.Query(ctx, query string, opts...)` signature takes a plain string query (not `[]schema.Message`). Verified at `eino@v0.7.7/adk/runner.go:98` + used in `cmd/spiked/e2e.go:208`. | Pattern 6 (Pipeline) | LOW — already proven in spike e2e. |
| A6 | `golang.org/x/text@v0.13.0` (the currently-resolved indirect version) is compatible with go1.24.5. Based on the module cache having it transitively resolved via eino/hertz without build errors. | Standard Stack | LOW — if incompatible, `go get golang.org/x/text@latest` resolves a newer compatible version; the `width.Narrow` API is stable across all v0.13+ versions. |

## Open Questions

1. **Does the production Milvus collection name graduate from `skillforge_spike_test` to something else?**
   - What we know: spike used `skillforge_spike_test` (`milvus_client.go:66`); D2-12 implies a "real" knowledge collection.
   - What's unclear: whether to rename or keep the spike name for continuity.
   - Recommendation: planner decides (Claude's discretion per CONTEXT). Suggest `skillforge_knowledge` for the production collection; keep `skillforge_spike_test` for `cmd/spiked` regression. The collection name is a config value, not a contract.

2. **Should `steps` be a YAML string or a YAML list in the frontmatter?**
   - What we know: SKILL_SCHEMA.md §5 says "Markdown ordered list" format; the example shows it as a YAML block scalar string. But the Architecture doc showed it as a YAML list.
   - What's unclear: the canonical YAML representation.
   - Recommendation: accept EITHER (parse `steps` as `interface{}`, accept string or []string). The validator checks non-empty. This is the most permissive correct choice and aligns with the SKILL_SCHEMA.md "Pure declarative Markdown" philosophy.

3. **Where does `suggested_alternative_query` come from for the refusal path?**
   - What we know: D2-14 says LLM-generated with rule-based fallback. But for `no_authorized_source` refusals (0 chunks), we refuse BEFORE calling the LLM (D-09) — so there's no LLM in-flight to generate the suggestion.
   - What's unclear: whether to make a SEPARATE cheap LLM call just for the suggestion, or use pure rule-based for Phase 2.
   - Recommendation: Phase 2 uses rule-based fallback (e.g., rephrase to include the tier keyword `权威制度`). A separate LLM call for the suggestion is Phase 5 polish. Document this as a D2-14 simplification.

4. **Does the contract test need a NEW assertion for the `[]Citation` shape, or just an UPDATE of the existing `TestContractStructuredOutputSchema`?**
   - What we know: the existing test (`contract_test.go:213-262`) asserts `Citations: []string{"auth-001"}`. D2-06 changes this to `[]Citation{{ChunkID:"auth-001", Span:"...", Tier:"权威"}}`.
   - Recommendation: UPDATE the existing test in the same commit as the D2-06 migration. The test is the contract; it evolves with the schema. Add a new sub-test `TestContractCitationGrounding` that asserts a fabricated span fails `GroundingChecker`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Go code | ✓ | go1.24.5 windows/amd64 (`go version`) | — |
| `golang.org/x/text/width` | Grounding normalization (D2-05) | ✓ (indirect in go.mod, v0.13.0 in module cache) | v0.13.0 | Promote to direct dep via `go get golang.org/x/text@v0.13.0` |
| Milvus (internal) | Retrieval (SYSK-07, GEVL-02) | ✓ (per CONTEXT — 内网, `MILVUS_ADDRESS` env) | unspecified (SDK targets v2.4.x) | `EmbedQueryDeterministic` for spike regression only; production refuses on Milvus outage |
| BGE-M3 (internal) | Embedding (retrieval pre-step) | ✓ (per CONTEXT — 内网, `BGE_M3_URL` env) | unspecified (1024-dim dense) | Same as above — production refuses on outage |
| GLM-5.2 (Coding Plan) | Verdict generation | ✓ (per CONTEXT — public, `GLM_API_KEY` env) | glm-5.2 | None — verdict cannot be generated without LLM; emit error chunk |
| Hertz-capable port `:8888` | `cmd/server` | ✓ (localhost) | — | `SERVER_ADDR` env override |
| Docker | NOT required (Phase 7) | ✗ (per Phase 1 RESEARCH — `docker: command not found`) | — | N/A — Phase 2 has no Docker dependency |

**Missing dependencies with no fallback:** none — all required services (Milvus, BGE-M3, GLM) are on the internal/public network per CONTEXT, and the Go toolchain + module cache have every library needed.

**Missing dependencies with fallback:** none.

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify` v1.11.1 (already direct dep) |
| Config file | none (standard `go test`; `//go:build contract` tag gates the integration suite) |
| Quick run command | `go test ./internal/...` |
| Full suite command | `go test ./... && go test ./integration/contract -tags=contract -count=1 -timeout 10s` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SYSK-01 | Drop `skills/compliance/SKILL.md` → registered at startup; `GET /api/skills` lists it | unit + integration | `go test ./internal/skill/ -run TestLoader` | ❌ Wave 0 |
| SYSK-02 | 7-field validator accepts compliance Skill; rejects missing temperature / empty reads / `*` wildcard / <3 prohibitions | unit | `go test ./internal/skill/ -run TestValidate` | ❌ Wave 0 |
| SYSK-07 | `reads: [authority/**]` resolves to tier `权威`; retriever cannot query outside it | unit | `go test ./internal/retriever/ -run TestResolveTiers` | ❌ Wave 0 |
| SYSK-08 | Missing/non-float temperature rejected; `model.WithTemperature(skill.Temperature)` passed to runner | unit | `go test ./internal/skill/ -run TestValidateTemperature` | ❌ Wave 0 |
| CMPL-01 | `skills/compliance/SKILL.md` present + passes validator | integration | `go test ./internal/skill/ -run TestComplianceSkillLoads` | ❌ Wave 0 (depends on the Skill file) |
| CMPL-02 | Verdict struct has `{status, confidence, citations, review_required}`; emitted as `data-policy` SSE chunk | unit | `go test ./internal/verdict/ -run TestVerdict` | ❌ Wave 0 (upgrade existing `internal/spike/verdict_test.go`) |
| CMPL-03 | Grounding downgrade: ungrounded citation → `review_required=true`, confidence < 0.9, status → 需补充 | unit | `go test ./internal/verdict/ -run TestGroundingChecker` | ❌ Wave 0 |
| CMPL-04 | Personal-tier query → 0 authority chunks → refusal | unit + integration | `go test ./internal/server/ -run TestPersonalTierRefusal` | ❌ Wave 0 |
| CMPL-05 | `review_required:true` payload carries to `data-policy` chunk | unit | covered by `TestGroundingChecker` + `TestVerdict` | ❌ Wave 0 |
| AGNT-03 | SSE stream emits `source-document` (per citation) + `data-policy` + `finish-step` | integration | `go test ./internal/server/ -run TestChatHandlerSSE` (reuse spike `sse_test.go` pattern) | ❌ Wave 0 |
| ARAG-04 | Citation = `{chunk_id, span, tier}`; span programmatically verified in chunk text | unit | `go test ./internal/verdict/ -run TestGroundingChecker` | ❌ Wave 0 |
| ARAG-05 | Refusal shape `{reason, can_user_rephrase, suggested_alternative_query}` with reason ∈ enum | unit | `go test ./internal/verdict/ -run TestRefusal` | ❌ Wave 0 |
| GEVL-01 | `maxScore < 0.3` → `low_retrieval_score` refusal | unit | `go test ./internal/server/ -run TestLowScoreRefusal` | ❌ Wave 0 |
| GEVL-02 | 0 chunks from tier-filtered retrieval → `no_authorized_source` refusal BEFORE LLM call | unit + integration | `go test ./internal/server/ -run TestNoChunksRefusal` | ❌ Wave 0 |

### Success Criteria → Pitfall Mitigation Test Map
| SC / Pitfall | Guarantee | Test |
|---------------|-----------|------|
| SC-1 (drop SKILL.md → registers + validates) | P-SKILL-1..6 mitigations | `TestLoader` + `TestValidate` (SYSK-01/02) |
| SC-2 (no authority chunks → structured refusal) | P-COMPL-1, P-RAG-2 (no improvising) | `TestNoChunksRefusal` — asserts refusal emitted + NO LLM call made (mock the runner, assert it's never invoked) |
| SC-3 (cited verdict, span verbatim in chunk) | P-COMPL-1 (TOP risk), P-AGENTIC-4 | `TestGroundingChecker` — fabricated span (not in chunk) → downgrade; verbatim span → pass |
| SC-4 (out-of-scope tier → 0 chunks → refusal) | P-RAG-2 (tier bleed) | `TestPersonalTierRefusal` + existing `TestContractMilvusFilter` (contract test, retained) |
| SC-5 (review_required payload, confidence < 0.9) | P-COMPL-5 (HITL) | `TestGroundingChecker` — asserts confidence capped at 0.89 + status → 需补充 |
| SC-6 (SSE source event with tier) | AGNT-03, ARAG-04 | `TestChatHandlerSSE` — asserts `source-document` chunk present in wire output |
| P-COMPL-1 (fake citations) | D2-01 grounding | `TestGroundingChecker` adversarial cases: fabricated span, wrong chunk_id, novel claim (no citation) |
| P-SKILL-3 (`*` wildcard reads) | SYSK-02 validator | `TestValidate` rejects `reads: ["*"]` |
| P-RAG-2 (tier bleed) | D-09 default-deny | `TestContractMilvusFilter` (retained from Phase 1) + `TestResolveTiers` |

### Sampling Rate
- **Per task commit:** `go test ./internal/...` (fast unit tests, <5s)
- **Per wave merge:** `go test ./... && go test ./integration/contract -tags=contract -count=1 -timeout 10s`
- **Phase gate:** Full suite green before `/gsd:verify-work`. Live-LLM + live-Milvus tests gated behind `MILVUS_INTEGRATION=1` + `GLM_API_KEY` env (human-verify checkpoint, same pattern as Phase 1 Plan 02/04).

### Wave 0 Gaps
- [ ] `internal/verdict/verdict_test.go` — graduates + extends `internal/spike/verdict_test.go` for `[]Citation` shape
- [ ] `internal/verdict/grounding_test.go` — NEW: pure-function unit tests for `Normalize` + `GroundingChecker` (adversarial: fabricated span, novel claim, 全角/半角 mixing, whitespace)
- [ ] `internal/skill/validate_test.go` — NEW: 7-field validator (temperature required, `*` rejected, prohibitions ≥3)
- [ ] `internal/skill/loader_test.go` — NEW: parse `skills/compliance/SKILL.md`, assert tier resolution
- [ ] `internal/server/chat_test.go` — NEW: handler test using Hertz dynamic-port pattern (graduates `cmd/spiked/sse_test.go`); mock retriever + mock runner
- [ ] `integration/contract/contract_test.go` — UPDATE: imports graduate to `verdict.*` / `wire.*` / `retriever.*`; `TestContractStructuredOutputSchema` asserts `[]Citation` shape; add `TestContractCitationGrounding`
- [ ] No framework install needed (Go `testing` + `testify` already direct deps)

*(If no gaps: N/A — gaps listed above.)*

## Security Domain

> `security_enforcement` is not explicitly set in `.planning/config.json` — per the absence rule, treat as enabled. Included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (Phase 2 is single-tenant, no auth — `POST /api/chat` is open; `POST /api/skills` dry-run is non-mutating) | Defer auth to Phase 5+; document in `.env.example` |
| V3 Session Management | no (Phase 2 is stateless per-request; AGNT-05 session isolation is Phase 3) | — |
| V4 Access Control | yes (tier boundary is the access control) | `reads` → Milvus tier filter is the hard barrier (SYSK-07); grounding check is the per-claim barrier (D2-01) |
| V5 Input Validation | yes | `skill.Validate` (SYSK-02) + `verdict.Validate` + `BuildMilvusFilter`'s `%q` escaping (Pitfall 7 mitigation, spike proven) |
| V6 Cryptography | no (no crypto in Phase 2; API key is the only secret, passed via env) | — |
| V7 Error Handling | yes | Errors NEVER include the raw API key (T-1-13 mitigation, proven in spike); LLM error surfaces truncated raw verdict (200 chars) |
| V9 Communications | partial (HTTPS to GLM is public-network; Milvus + BGE-M3 are 内网) | GLM via `https://open.bigmodel.cn/api/coding/paas/v4` (HTTPS); internal services via gRPC/HTTP on 内网 |
| V12 Files & Resources | yes (Skill + knowledge files read from disk) | `filepath.WalkDir` constrained to `skills/` and `knowledge/` roots; no path traversal (resolve + verify under root) |

### Known Threat Patterns for the Go + LLM + Milvus stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Compliance hallucination (fabricated clause) | Tampering (P-COMPL-1) | D2-01 grounding check + D2-02 downgrade; verbatim span requirement; refusal-by-construction on 0 chunks |
| Tier bleed (个人/草稿 leaking into 权威 answers) | Information disclosure (P-RAG-2) | PRE-similarity Milvus tier filter (SYSK-07); `BuildMilvusFilter` default-deny on empty reads (D-09) |
| Prompt injection via retrieved chunk text | Tampering | Retrieved chunks are DATA, not instructions; the system prompt frames them as `[CHUNK <id>] <text>`; grounding check catches fabricated citations that result from injection |
| PII leakage (contact-info query) | Information disclosure | Contact-info query → 0 authority chunks → `no_authorized_source` refusal (SC-2/SC-4); even if chunks existed in personal tier, compliance Skill `reads: [authority/**]` cannot see them |
| API key leakage in logs/SSE | Information disclosure (T-1-13) | Trace log uses `<redacted>` for the key field (proven in `cmd/spiked/e2e.go:167`); `UIMessageChunk.ErrorText` NEVER carries the key |
| Path traversal in Skill/knowledge loader | Tampering | `filepath.WalkDir` rooted at `skills/` and `knowledge/`; reject symlinks escaping root; the dir-name → tier map rejects unknown top-level dirs |

## Sources

### Primary (HIGH confidence — verified by reading exact Go module source)
- `~/go/pkg/mod/github.com/cloudwego/eino@v0.7.7/adk/` — `chatmodel.go:54,212` (`WithChatModelOptions`, `NewChatModelAgent`), `runner.go:59,98` (`NewRunner`, `Runner.Query`), `interface.go:33,197` (`MessageVariant.IsStreaming`, `AgentEvent`)
- `~/go/pkg/mod/github.com/cloudwego/hertz@v0.10.6/pkg/app/server/option.go:158-159` — `WithHostPorts`
- `~/go/pkg/mod/github.com/cloudwego/hertz@v0.10.6/pkg/protocol/sse/writer.go:41,46,64,157` — `NewWriter`, auto Content-Type, `WriteEvent`, per-frame Flush
- `~/go/pkg/mod/golang.org/x/text@v0.13.0/width/width.go:184-197` — `var Narrow Transformer`
- `D:\code\ClaudeCode\ai-agent\internal\spike\{verdict,uimsg,milvus_client,hertz_sse}.go` — proven spike assets (the graduation source)
- `D:\code\ClaudeCode\ai-agent\cmd\spiked\{e2e,wire,sse}.go` — proven pipeline + SSE + frontmatter patterns
- `D:\code\ClaudeCode\ai-agent\integration\contract\contract_test.go` — the version-pin + wire-contract test (Phase 2 updates it)
- `D:\code\ClaudeCode\ai-agent\go.mod` — confirmed pinned versions (eino v0.7.7, eino-ext openai v0.1.5, hertz v0.10.6, milvus-sdk-go v2.4.2, testify v1.11.1, yaml.v3 v3.0.1, x/text v0.13.0 indirect)

### Secondary (MEDIUM confidence — project research + skill findings)
- `.planning/research/ARCHITECTURE.md` — 6-layer architecture, Skill→ADK mapping (note: ARCHITECTURE.md still describes the 6-field schema and endpoints like `/api/v1/chat/stream`; Phase 2 CONTEXT D2-08/D2-10 override to `POST /api/chat` + 7-field)
- `.planning/research/PITFALLS.md` — P-COMPL-1 (TOP risk), P-RAG-2 (tier bleed), P-SKILL-1..6
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md` — Coding Plan endpoint, `WithExtraFields` patterns, "What to Avoid" (6 findings directly cited above)
- `.planning/SKILL_SCHEMA.md` — the 7-field spec (the validation contract for SYSK-02)

### Tertiary (LOW confidence — none for Phase 2)
- All Phase 2 claims are verified against source or project research; no WebSearch-only claims.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library verified at the exact pinned version in the local Go module cache; the only new direct dep (`x/text`) is Go-first-party and already transitively resolved
- Architecture: HIGH — the spike already proved every contract (verdict JSON, UIMessageChunk SSE, Milvus tier filter); Phase 2's novel logic (grounding checker, pipeline composition) is pure Go orchestration with verified API anchors
- Pitfalls: HIGH — Phase 2 pitfalls are concrete regressions of patterns already mitigated in the spike (tier filter, version pinning) plus the new grounding check (D2-01) which is unit-testable in isolation
- Grounding normalization (D2-05): HIGH on `width.Narrow` API existence; MEDIUM on full punctuation coverage (A1) — mitigated by unit testing the exact corpus punctuation

**Research date:** 2026-08-11
**Valid until:** 2026-09-11 (30 days — stable; the Go module versions are pinned by the contract test, so drift is caught at CI)
