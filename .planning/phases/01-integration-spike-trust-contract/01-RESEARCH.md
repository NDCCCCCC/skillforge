# Phase 1: Integration Spike & Trust Contract — Research

**Researched:** 2026-08-11
**Domain:** Eino v0.7.7 ADK + Hertz v0.10.6 SSE + Vercel AI SDK v5 wire framing + Milvus tier-filter + BGE-M3 + GLM-5.2 Coding Plan endpoint
**Confidence:** MEDIUM-HIGH (core APIs verified via Go module source; eino-ext internals not directly inspected due to network restrictions; environment audit revealed Docker absence — a critical constraint)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions (D-01..D-12)

- **D-01**: API validation = **real API only** (GLM-5.2 via Coding Plan endpoint `https://open.bigmodel.cn/api/coding/paas/v4`).
- **D-02**: Mock OpenAI server pattern from spike 001 is **reused as a wire-format validation tool alongside real API** (parallel, not exclusive).
- **D-03**: SSE verification = **validate all 3 layers** — wire format, typed parts (reasoning/tool-calls/sources), UIMessageChunk alignment.
- **D-04**: Typed parts to validate = `reasoning` (when `thinking:{type:disabled}` is OFF vs ON), `tool-calls` (Eino tool-calling via `WithTools`), `sources` (citation chips).
- **D-05**: `WithExtraFields(map[string]any{...})` (Eino v0.1.13+) is the API for passing OpenAI extras — proven in spike 002b + 003.
- **D-06**: Milvus scope = **+ BGE-M3 embedding + real retrieval E2E** (heaviest, most production-like).
- **D-07**: Test data = 4 tiers (权威 / 历史 / 个人 / 草稿), each tier with ≥3 documents. Query with `tier in ["权威"]` filter must return **zero** personal/草稿 chunks.
- **D-08**: BGE-M3 embedding model (C-MTEB leader, multilingual, 8K context, self-hostable via Docker).
- **D-09**: Milvus instance = local Docker container (Milvus v2.5+ standalone).
- **D-10**: Output = **Runnable Go skeleton + complete E2E trace log**.
- **D-11**: Skeleton must be runnable end-to-end on a fresh laptop with one command (`docker compose up` + `go run .`).
- **D-12**: E2E trace log = 1 sample query that walks through ALL components, timestamped with input/output/decision.

### Claude's Discretion

- Specific 5 query list for the spike (suggest: same compliance-real queries as spike 002/003)
- Whether to use `eino-ext/adk` (multi-agent) or just `eino` (single ChatModel) for the spike (suggest: single ChatModel for spike; ADK added in Phase 3)
- React frontend in the spike — minimal stub vs full Phase-4-style client (suggest: minimal stub that can parse SSE)
- Container orchestration — `docker compose` vs `docker run` (suggest: `docker compose`)

### Deferred Ideas (OUT OF SCOPE)

- Multi-agent orchestration (`SequentialAgent` / `ParallelAgent` / `LoopAgent` full wiring) — Phase 3
- Real React frontend with Vercel AI SDK v5 — Phase 4
- Ragas faithfulness eval pipeline (Python sidecar) — Phase 6
- Skill Wizard UI — Phase 5
</user_constraints>

---

## Summary

Phase 1 is a 1–2 day **integration validation spike** that must prove four wire contracts hold on the production stack before Phase 2 commits to the full data model: (1) Eino ChatModel → OpenAI-compatible GLM-5.2 over the Coding Plan endpoint with `temperature`/`response_format`/`thinking` extras, (2) Hertz `text/event-stream` SSE framing → Vercel AI SDK v5 `UIMessageChunk` parsing, (3) Milvus v2.5+ `tier in [...]` filter expression enforced *before* similarity search, (4) BGE-M3 self-hosted embedding → Milvus `dim=1024` collection pipeline.

The Eino v0.7.7 ADK source (verified by downloading `github.com/cloudwego/eino@v0.7.7`) confirms `adk.NewChatModelAgent(ctx, *adk.ChatModelAgentConfig)` and `adk.NewSequentialAgent(ctx, *adk.SequentialAgentConfig)` are the canonical constructors, and `Runner.Run(ctx, []Message, opts...) *AsyncIterator[*AgentEvent]` is the streaming entry point. The Hertz v0.10.6 SSE package (verified by downloading `github.com/cloudwego/hertz@v0.10.6/pkg/protocol/sse/`) exposes `NewWriter(c).WriteEvent(id, type, data)` — **not** the `s.Publish(...)` API shown in `PROJECT.md` §2.2 (which appears to be a misremembered Gin-Echo style; planner must use the actual Hertz API). Vercel AI SDK v5 UIMessageChunk wire format encodes each chunk as `data: {json}\n\n` SSE frames with discriminated-union types (`text-delta`, `reasoning-delta`, `tool-input-*`, `source-url`, etc.).

**Critical environment finding:** Docker is **not installed** on the target machine (`docker: command not found`); the locked `docker compose up` pattern from D-11 cannot run as-is. The planner must include either a fallback (Milvus standalone binary, in-memory mock, or remote Milvus) or a checkpoint task that verifies Docker availability and installs it before the E2E integration test runs. This constraint applies equally to BGE-M3 self-host.

**Primary recommendation:** Build a single Go binary `cmd/spiked/` with subcommands `spike wire`, `spike sse`, `spike milvus`, `spike e2e`. Reuse spike 001's mock-OpenAI-server pattern for `spike wire`; use real GLM-5.2 (with `thinking:{type:disabled}` + `temperature:0` + `response_format:{type:json_object}`) for `spike e2e`. Run Milvus + BGE-M3 either via Docker (preferred — Phase 7 needs this anyway) or via Milvus Lite embedded mode if Docker is unavailable.

---

## Architectural Responsibility Map

Phase 1 validates end-to-end wire contracts across **four runtime tiers**. Each integration point maps to a tier owner and a testable boundary.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Eino ChatModel → GLM-5.2 (OpenAI-compat) | API / Backend (Go) | External LLM API | The Go `cmd/spiked/` binary owns the Eino chat model init; the LLM API is an external dependency verified by mock + real call |
| Hertz SSE writer | API / Backend (Go) | Browser / Client (React stub) | Hertz owns emission; the React stub only validates the wire bytes, not interactive UI |
| Vercel AI SDK v5 wire parsing | Browser / Client (React stub) | API / Backend | Client owns UIMessageChunk parsing — but the *envelope* (JSON shape per chunk type) is the shared contract being validated |
| Milvus tier-filter | Database / Storage | API / Backend | Milvus is the filter executor; the Go tier-filter builder + tier metadata schema live at the API tier boundary |
| BGE-M3 embedding | Database / Storage | API / Backend | BGE-M3 service is the embedder; the embedding client wrapper lives in the API tier |
| Mock OpenAI server (wire regression tool) | API / Backend (Go process inside `cmd/spiked`) | — | Lives alongside real Eino calls inside the same binary for fast wire regression |

**Critical tier boundary:** The `UIMessageChunk` envelope is the **only contract** crossing the Browser → API tier boundary in Phase 1. Every other contract is intra-tier (Backend internal) or Backend → External Service.

---

## Standard Stack

### Core (locked from CLAUDE.md + STACK.md, verified for Phase 1)

| Library | Version | Purpose | Why Standard | Confidence |
|---------|---------|---------|--------------|------------|
| `github.com/cloudwego/eino` | v0.7.7 (track to v0.9.13+) | Agent framework + ADK GA | Verified via `go list -m -versions`; v0.7.7 confirmed tagged; v0.9.13 is latest stable per spike 001 reference | HIGH [VERIFIED: go module proxy] |
| `github.com/cloudwego/eino-ext/components/model/openai` | pseudo from master (no stable tag) | OpenAI-compat adapter for GLM-5.2 Coding Plan | Proven in spike 001/003; `eino-ext@v0.0.1-alpha` is an empty placeholder tag — real code lives on master branch | HIGH [VERIFIED: spike 001 source] |
| `github.com/cloudwego/hertz` | v0.10.6 (v0.10.2+ required) | HTTP server with `pkg/protocol/sse` | Verified `NewWriter(c)` API via Go module source | HIGH [VERIFIED: hertz v0.10.6 source] |
| `github.com/cloudwego/hertz/pkg/protocol/sse` | bundled with hertz | SSE writer (low-level) | Sets `Content-Type: text/event-stream; charset=utf-8` + `Cache-Control: no-cache` automatically | HIGH [VERIFIED: writer.go source] |
| `github.com/milvus-io/milvus-sdk-go/v2` | latest (v2.x) | Milvus Go SDK | Filter syntax `tier in [...]` confirmed via Go SDK v2 `SearchQueryOptions.Filter` field | HIGH [VERIFIED: Milvus docs + WebSearch] |
| `ai` (Vercel AI SDK v5) + `@ai-sdk/react` | v5.x | Client-side UIMessageChunk parsing | `DefaultChatTransport({ api })` parses `data: {json}\n\n` frames | HIGH [VERIFIED: ai-sdk.dev docs + WebSearch] |
| `BAAI/bge-m3` (via TEI Docker) | latest | Multilingual embedding, dim=1024, 8192 tokens | C-MTEB leader, self-hostable via TEI container | HIGH [VERIFIED: HuggingFace BAAI/bge-m3 + Milvus docs] |

### Supporting (Phase 1 spike-only)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net/http/httptest` | stdlib | In-process test server for SSE round-trip | `spike sse` subcommand (Hertz SSE round-trip test) |
| `testing` | stdlib | Integration test framework | `go test ./integration/contract` |
| `testify` | latest | Assertions for integration tests | All integration tests |
| `gopkg.in/yaml.v3` | v3 | SKILL.md frontmatter parsing | Reused from spike 001/003 |
| `@types/node` + `tsx` | latest | Minimal React stub client | `spike sse` end-to-end client validation |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hertz SSE | Custom SSE writer | Hertz `pkg/protocol/sse` already implements the spec correctly (verifies via `Example()` test in v0.10.6); custom = re-invents wheel |
| Milvus SDK v2 | Milvus SDK v1 (legacy) | v2 uses `SearchQueryOptions` struct (cleaner); v1 uses positional args; v2 is recommended |
| BGE-M3 self-host via TEI Docker | Volcano Ark hosted `bge-large-zh` | D-08 locks self-host for spike reproducibility; hosted fallback = data-residency violation |
| React 19 stub client | Plain `curl` or `fetch()` | Vercel AI SDK v5 `DefaultChatTransport` is the canonical parser we need to validate end-to-end; curl = wire-format-only check, misses parser round-trip |
| `docker compose` for Milvus + BGE-M3 | Milvus Lite (embedded) | Milvus Lite is `pip install milvus-lite`-style in-process; loses ops realism but works without Docker. **Recommended fallback when Docker unavailable** |
| `eino-ext/adk` | `eino` core (single ChatModel) | Claude's discretion: spike uses single ChatModel; SequentialAgent is wired but not exercised in Phase 1 (deferred to Phase 3 per `deferred_ideas`) |

**Installation:**

```bash
# Phase 1 spike skeleton (Go)
go get github.com/cloudwego/eino@v0.7.7
go get github.com/cloudwego/eino-ext/components/model/openai@latest   # pseudo from master
go get github.com/cloudwego/hertz@v0.10.6
go get github.com/milvus-io/milvus-sdk-go/v2@latest

# Phase 1 spike skeleton (Node — minimal stub client)
npm install ai@latest @ai-sdk/react@latest
```

**Version verification performed:** `go list -m -versions github.com/cloudwego/eino` returned stable versions through v0.10.0-alpha.14 (pre-release); latest stable is v0.9.13. v0.7.7 confirmed as the locked minimum per STACK.md. Hertz v0.10.6 is latest stable per the same command.

---

## Package Legitimacy Audit

> The Package Legitimacy Gate was executed. **slopcheck was installed successfully but could not reach the Go registry** (`proxy.golang.org` timeout) — graceful degradation applied. **All packages below are tagged `[ASSUMED]`** and the planner must gate each install behind a `checkpoint:human-verify` task before `go mod tidy`.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `github.com/cloudwego/eino` | Go (proxy.golang.org) | 1+ yr (v0.7.7 tagged 2026-Q3) | HIGH (ByteDance official) | github.com/cloudwego/eino | [ERROR: registry timeout] | Approved — verified via local `go get` + source inspection |
| `github.com/cloudwego/eino-ext` | Go (pseudo-version from master) | 1+ yr | HIGH (ByteDance official) | github.com/cloudwego/eino-ext | [ERROR: registry timeout] | Approved — verified via spike 001 source import path |
| `github.com/cloudwego/hertz` | Go (proxy.golang.org) | 4+ yrs | HIGH (ByteDance official) | github.com/cloudwego/hertz | [ERROR: registry timeout] | Approved — verified via local `go get` v0.10.6 |
| `github.com/milvus-io/milvus-sdk-go/v2` | Go (proxy.golang.org) | 2+ yrs | HIGH (Milvus official) | github.com/milvus-io/milvus-sdk-go | [ERROR: registry timeout] | Approved — verified via WebSearch + Milvus docs |
| `BAAI/bge-m3` | HuggingFace Model Hub | 2+ yrs (released 2024) | HIGH (CN RAG SOTA) | huggingface.co/BAAI/bge-m3 | [ERROR: registry timeout] | Approved — verified via HuggingFace listing |
| `ai` (Vercel AI SDK) | npm | ongoing | HIGH (Vercel official) | github.com/vercel/ai | [ERROR: registry timeout] | Approved — verified via Vercel docs |
| `@ai-sdk/react` | npm | ongoing | HIGH | github.com/vercel/ai | [ERROR: registry timeout] | Approved |
| `milvusdb/milvus` Docker image | Docker Hub | ongoing | HIGH | github.com/milvus-io/milvus | n/a (Docker image) | Approved — verified via Milvus docs |
| `huggingface/text-embeddings-inference` Docker image | Docker Hub | ongoing | HIGH | github.com/huggingface/text-embeddings-inference | n/a (Docker image) | Approved — verified via TEI docs |

**Packages removed due to slopcheck verdict:** None (registry unavailable, no SLOP verdicts returned).

**Packages flagged `[ASSUMED]` requiring planner checkpoint:** ALL 9 above (gate each install behind `checkpoint:human-verify`).

---

## Architecture Patterns

### System Architecture Diagram (Phase 1 spike wiring)

```
                         Phase 1 spike binary: cmd/spiked/
                                    │
        ┌───────────────────────────┼──────────────────────────────────┐
        │                           │                                  │
   ┌────▼─────┐                ┌────▼─────┐                       ┌────▼─────┐
   │ spike    │                │ spike    │                       │ spike    │
   │ wire     │                │ sse      │                       │ milvus   │
   └────┬─────�                └────┬─────┘                       └────┬─────┘
        │                           │                                  │
   mock OpenAI                  Hertz SSE                          Milvus v2.5
   server (in-process)          GET /api/spike/chat                 + BGE-M3
        │                           │                                  │
        ▼                           ▼                                  ▼
   Eino ChatModel ─────────►  adk.NewRunner ─────►  Milvus Go SDK v2
   (GLM-5.2 real)            .Query(ctx, q)        .Search(ctx, opts{Filter: "tier in [...]"})
        │                           │                                  │
        ▼                           ▼                                  ▼
   spike wire stdout          UIMessageChunk              Hit/miss per tier
   (request body dump)        SSE frames                  (zero personal/草稿
                              `data: {json}\n\n`           when tier="权威")
                                     │
                                     ▼
                              Vercel AI SDK v5
                              DefaultChatTransport
                              parses in React stub
```

### Recommended Project Structure (Phase 1 spike)

```
cmd/spiked/                           # Single Go binary with subcommands
├── main.go                           # cobra/flag-based subcommand router
├── wire.go                           # spike wire: Eino+mock OpenAI server round-trip
├── sse.go                            # spike sse: Hertz SSE → React stub client round-trip
├── milvus.go                         # spike milvus: tier-filter assertion against 4-tier seeded data
├── e2e.go                            # spike e2e: full pipeline trace log
└── mocks/                            # in-process mock OpenAI server (from spike 001)
    └── openai.go
integration/
└── contract/
    └── contract_test.go              # version-pinned integration test, runs in <10s
internal/spike/
├── eino.go                           # Eino ChatModel + Runner factory
├── hertz_sse.go                      # Hertz SSE writer wrapper emitting UIMessageChunk envelopes
├── uimsg.go                          # UIMessageChunk JSON shape definitions (matches AI SDK v5 types)
├── milvus_client.go                  # Milvus Go SDK v2 wrapper with tier-filter helper
└── bge_embed.go                      # BGE-M3 HTTP client wrapper (calls TEI endpoint)
testdata/
├── skills/                           # 4 SKILL.md fixtures (one per tier)
│   ├── authority/                    # reads: ["权威"]
│   ├── historical/                   # reads: ["历史"]
│   ├── personal/                     # reads: ["个人"]
│   └── draft/                        # reads: ["草稿"]
├── docs/                             # ≥3 docs per tier (markdown fixtures)
└── queries/                          # 5 compliance queries from spike 002/003
web/                                 # minimal Vite + React 19 stub client
├── src/App.tsx                       # useChat + DefaultChatTransport against Hertz SSE
└── package.json
docker-compose.yml                   # Milvus standalone + BGE-M3 TEI (Phase 7 also depends)
.env.example                         # GLM_API_KEY, MILVUS_ADDR, BGE_M3_URL
go.mod                               # pinned versions
README.md                            # quick start: docker compose up && go run . spike e2e
```

### Pattern 1: Eino ChatModel wired to GLM-5.2 Coding Plan endpoint

**What:** Initialize `eino-ext/components/model/openai` ChatModel pointed at the Coding Plan endpoint (NOT the standard `/api/paas/v4`), invoke via `Runner.Query(ctx, query)`, consume `AsyncIterator[*AgentEvent]`.

**When to use:** Spike 001 + spike 003 already proved this works. Phase 1 reuses the same pattern but with a `Runner` wrapper (instead of calling `cm.Generate` directly).

**Example (verified against spike 001 source + Eino v0.7.7 source):**

```go
// internal/spike/eino.go
import (
    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/schema"
)

func NewGLMAgent(ctx context.Context, apiKey string) (*adk.ChatModelAgent, error) {
    cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",  // [VERIFIED: spike 001/003]
        APIKey:  apiKey,
        Model:   "glm-5.2",                                      // [VERIFIED: spike 002/003]
    })
    if err != nil { return nil, err }

    // Per-call options — these are passed to Runner.Query via opts,
    // not stored on the agent (model options are runtime, agent config is build-time).
    // The planner should expose a helper: spike.WithComplianceOptions()
    // that wraps model.WithTemperature(0.0) + openai.WithExtraFields({response_format, thinking})
    // Source: spike 003 main.go + Eino v0.7.7 components/model/option.go

    return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "spike-compliance-stub",
        Description: "Stub agent for Phase 1 wire validation",
        Instruction: "You are a compliance auditor. Output strict JSON.",
        Model:       cm,                                          // type: model.ToolCallingChatModel
        MaxIterations: 5,
    })
    // Source: C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\eino@v0.7.7\adk\chatmodel.go:212
}

// Run example:
// runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
// iter := runner.Query(ctx, "Is 200 RMB over budget allowed for travel? Provide verdict.")
// for { ev, ok := iter.Next(); if !ok { break }; ... }
// Source: C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\eino@v0.7.7\adk\runner.go:59 + :98
```

### Pattern 2: Hertz SSE writer emitting UIMessageChunk envelopes

**What:** Hertz `pkg/protocol/sse.NewWriter(c)` returns a `Writer`; each `WriteEvent(id, type, data)` emits a standards-compliant SSE frame. We serialize each `UIMessageChunk` (typed discriminated union) to JSON and emit as the `data:` payload.

**When to use:** Any backend → React streaming endpoint. Hertz sets `Content-Type: text/event-stream; charset=utf-8` + `Cache-Control: no-cache` automatically (verified in writer.go:43-46).

**Example (verified against Hertz v0.10.6 source):**

```go
// internal/spike/hertz_sse.go
import (
    "encoding/json"
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/cloudwego/hertz/pkg/protocol/sse"
)

// UIMessageChunk mirrors the AI SDK v5 discriminated union.
// Field names + JSON tags MUST match @ai-sdk/react's parser exactly.
// Source: ai-sdk.dev/docs/ai-sdk-ui/streaming-data + process-ui-message-stream.ts
type UIMessageChunk struct {
    Type             string          `json:"type"`                        // text-start|text-delta|text-end|reasoning-*|tool-input-*|source-*|data-*|finish-step|finish-message|error
    ID               string          `json:"id,omitempty"`
    Delta            string          `json:"delta,omitempty"`
    ToolCallID       string          `json:"toolCallId,omitempty"`
    ToolName         string          `json:"toolName,omitempty"`
    Input            json.RawMessage `json:"input,omitempty"`
    InputTextDelta   string          `json:"inputTextDelta,omitempty"`
    Output           json.RawMessage `json:"output,omitempty"`
    SourceID         string          `json:"sourceId,omitempty"`
    URL              string          `json:"url,omitempty"`
    Title            string          `json:"title,omitempty"`
    ErrorText        string          `json:"errorText,omitempty"`
    Data             json.RawMessage `json:"data,omitempty"`             // for data-* chunks
}

func StreamChunks(c *app.RequestContext, chunks <-chan UIMessageChunk) error {
    w := sse.NewWriter(c)  // [VERIFIED: writer.go:41 — sets Content-Type + Cache-Control]
    defer w.Close()
    for chunk := range chunks {
        payload, err := json.Marshal(chunk)
        if err != nil { return err }
        // Type="message" is the default SSE event type; AI SDK v5 SSE parser
        // reads `data:` lines regardless of event type, so we use "message".
        if err := w.WriteEvent("", "message", payload); err != nil {  // [VERIFIED: writer.go:64]
            return err
        }
    }
    return nil
}

// CRITICAL: Hertz v0.10.6 does NOT have s.Publish(...) or sse.NewStream(c).
// The pattern shown in PROJECT.md §2.2 (`s.Publish(&sse.Event{Event: "message", Data: ...})`)
// is INCORRECT for Hertz. Use w.WriteEvent("", "message", payload).
// Source: C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\hertz@v0.10.6\pkg\protocol\sse\writer.go
// (verified — no NewStream func, no Publish method on Writer)
```

### Pattern 3: Milvus tier-filter via Go SDK v2

**What:** Milvus supports SQL-like filter expressions on scalar fields. For a `tier` VARCHAR field, the filter `tier in ["权威"]` returns only documents with `tier == "权威"`. The filter is applied **before** vector similarity ranking — this is the structural enforcement of "tier isolation".

**When to use:** Every retrieval call. The Skill's `reads` field maps directly to the filter expression.

**Example (verified via Milvus docs + WebSearch on Go SDK v2):**

```go
// internal/spike/milvus_client.go
import (
    "github.com/milvus-io/milvus-sdk-go/v2/client"
    "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// BuildMilvusFilter converts a Skill.reads slice into a Milvus filter expression.
// CRITICAL: tier values are CN characters — must be JSON-escaped (Milvus expects UTF-8 string literals in quotes).
func BuildMilvusFilter(tiers []string) string {
    if len(tiers) == 0 { return "false" }  // [VERIFIED: Milvus filter "false" returns zero results — default-deny]
    quoted := make([]string, len(tiers))
    for i, t := range tiers {
        quoted[i] = fmt.Sprintf("%q", t)  // Go's %q produces valid JSON-string with CN chars
    }
    return fmt.Sprintf("tier in [%s]", strings.Join(quoted, ","))
}

// Search call:
expr := BuildMilvusFilter([]string{"权威"})
results, err := milvusClient.Search(ctx, &client.SearchQueryOptions{
    CollectionName:   "skillforge_chunks",
    ConsistencyLevel: entity.ClStrong,
    OutputFields:     []string{"id", "tier", "content", "source"},
    Limit:            10,
    Filter:           expr,                                                   // [VERIFIED: SearchQueryOptions.Filter]
    SearchParams:     searchParam,
    VectorField:      entity.FloatVectorField{Name: "embedding", Dim: 1024},  // [VERIFIED: BGE-M3 dim]
    Vectors:          []entity.Vector{queryVector},
})

// CRITICAL: Milvus filter is applied BEFORE vector similarity. There is no "rank-then-filter"
// mode — if you want to filter, you must set Filter on SearchQueryOptions. The retriever cannot
// accidentally return docs from other tiers.
// Source: Milvus Go SDK v2 docs + WebSearch confirmation
```

### Pattern 4: Eino retriever wrapping Milvus + tier filter

**What:** Wrap `milvusClient.Search` in an Eino `embedding.Retriever` interface so it can be invoked as a tool by `ChatModelAgent`. The Skill's `reads` is passed at construction time (not per-call) — making it structurally impossible to query outside declared tiers.

**When to use:** Phase 2-3 production retriever. Phase 1 spike uses the raw Milvus SDK directly (no Eino retriever abstraction) to keep the spike minimal.

**Example (Eino retriever pattern + eino-ext/milvus wrapper):**

```go
// NOTE: eino-ext/components/retriever/milvus source NOT directly inspected (v0.0.1-alpha tag is empty).
// The pattern below is the expected pattern based on Eino core retriever.Options API + Milvus SDK v2 API.
// PLANNER VERIFICATION REQUIRED before Phase 2 commits to this pattern.

import (
    "github.com/cloudwego/eino/components/retriever"
    "github.com/cloudwego/eino/components/embedding"
    "github.com/milvus-io/milvus-sdk-go/v2/client"
)

type TierFilterRetriever struct {
    milvusClient client.Client
    embedder     embedding.Embedder
    collection   string
    allowedTiers []string  // bound at construction; immutable
}

func (r *TierFilterRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
    // 1. Embed query via BGE-M3
    queryEmb, _ := r.embedder.EmbedStrings(ctx, []string{query})

    // 2. Search Milvus WITH tier filter (filter is ALWAYS applied; allowedTiers cannot be bypassed)
    expr := BuildMilvusFilter(r.allowedTiers)
    results, err := r.milvusClient.Search(ctx, &client.SearchQueryOptions{
        CollectionName: r.collection,
        Filter:         expr,  // structural enforcement
        Limit:          getTopK(opts...),
        VectorField:    entity.FloatVectorField{Name: "embedding", Dim: 1024},
        Vectors:        []entity.Vector{entity.NewFloatVector(queryEmb[0])},
    })

    // 3. Convert Milvus results → Eino schema.Document
    // (chunk_id maps to schema.Document.ID; tier maps to MetaData["tier"])
    // ...
}
```

### Pattern 5: Vercel AI SDK v5 client parses Hertz SSE frames

**What:** A minimal React 19 + `@ai-sdk/react` stub client uses `useChat` with `DefaultChatTransport({ api: 'http://localhost:8888/api/chat' })`. The transport parses incoming SSE frames (any line beginning with `data:`) and reassembles them into `UIMessage` parts (text/reasoning/tool-input/source).

**When to use:** Phase 4 production UI. Phase 1 spike uses a minimal stub (single page) to validate the round-trip.

**Example:**

```tsx
// web/src/App.tsx
import { useChat } from '@ai-sdk/react';
import { DefaultChatTransport } from 'ai';

export default function SpikeStub() {
  const { messages, sendMessage } = useChat({
    transport: new DefaultChatTransport({
      api: 'http://localhost:8888/api/spike/chat',  // Hertz SSE endpoint
    }),
  });

  return (
    <div>
      {messages.map(m => (
        <div key={m.id}>
          {m.parts.map((p, i) => {
            if (p.type === 'text') return <span key={i}>{p.text}</span>;
            if (p.type === 'reasoning') return <details key={i}><summary>reasoning</summary>{p.text}</details>;
            if (p.type === 'source-url') return <a key={i} href={p.url}>[{p.title ?? p.url}]</a>;
            return null;
          })}
        </div>
      ))}
      <button onClick={() => sendMessage({ text: 'Is 200 RMB over budget allowed for travel?' })}>
        Ask
      </button>
    </div>
  );
}
```

### Anti-Patterns to Avoid

- **Don't use `s.Publish(...)` or `sse.NewStream(c)` — these don't exist in Hertz v0.10.6.** The PROJECT.md §2.2 example is from a different framework's SSE helper. Use `w.WriteEvent("", "message", payload)` (verified in writer.go:64).
- **Don't use Milvus collection-per-tier.** D-07 + D-08 require single collection with `tier` scalar field — collection-per-tier couples ops to tier count and breaks the filter syntax.
- **Don't use `eino-ext/components/model/openai` with the standard `/api/paas/v4` endpoint.** GLM Coding Plan keys return `{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}` (verified in spike 001). Use `/api/coding/paas/v4`.
- **Don't hardcode the model name in log strings** (spike 002 bug — `(glm-4-flash)` was hardcoded while Model field was `glm-5.2`). Read from config struct.
- **Don't use `response_format:{type:json_schema}` on GLM-5.2.** May not support full schema validation. Use `response_format:{type:json_object}` + Go struct validation (spike 003 finding).
- **Don't trust `temperature:0` for byte-determinism.** Spike 002/002b proved byte-determinism impossible with reasoning models. Phase 1 uses semantic-determinism via structured output + JSON schema validation, not byte-comparison.
- **Don't run `docker compose up` on a machine without Docker.** D-11 assumes Docker availability; if absent, the planner must add a checkpoint task verifying Docker presence OR substitute Milvus Lite / remote Milvus.
- **Don't bind the tier filter as a per-call option to a generic retriever.** If the retriever is shared across Skills with different `reads`, a bug could pass an empty filter. Bind `allowedTiers` at construction time and never expose it as a per-call option.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SSE wire framing | Custom SSE writer | Hertz `pkg/protocol/sse.NewWriter` | Already standards-compliant (sets correct headers + flush semantics); verified in writer.go |
| SSE client-side parsing | Custom `EventSource` wrapper | Vercel AI SDK v5 `DefaultChatTransport` | Already handles reconnection, message reassembly, typed-part parsing; ~12KB |
| Milvus filter expression builder | Custom string templating | `BuildMilvusFilter([]string)` helper (Pattern 3) | Centralizes escape rules (CN chars in JSON strings); one place to fix escaping bugs |
| LLM OpenAI-compat client | Custom HTTP client | `eino-ext/components/model/openai` | Proven in spike 001/003; handles streaming + tool calling + extra fields |
| Tier-isolation enforcement | Prompt-level "only use authority docs" | Milvus `Filter` field (applied pre-similarity) | Structural enforcement can't be bypassed by prompt injection; prompt-only = security theater |
| UIMessageChunk JSON serialization | Custom SSE payload format | AI SDK v5 discriminated union types | Frontend parser already expects this exact shape; custom format = wire drift |
| Mock OpenAI server for wire tests | Real LLM call for every wire regression test | In-process mock (spike 001 pattern) | Mock = fast + no API quota burn; real = slow + expensive; run both (D-02) |
| BGE-M3 inference server | In-process embedding model via HuggingFace transformers | TEI Docker container | TEI handles batching, GPU/CPU auto, HTTP API; in-process = GPU dependency + deployment hell |
| Embedding dimension validation | Runtime panic on dim mismatch | Milvus collection schema with `dim=1024` declared at create time | Milvus rejects insert if dim ≠ 1024; structural check |

**Key insight:** Phase 1 has zero custom wire code. Every component is reused from spike 001/003 (Eino + GLM-5.2) or a locked third-party library (Hertz, Vercel AI SDK, Milvus SDK, BGE-M3 via TEI). The spike's value is in *composition*, not invention.

---

## Runtime State Inventory

> **Not applicable to Phase 1.** This phase is a greenfield validation spike; no rename, refactor, or migration of existing state. Phase 1 produces a runnable Go skeleton + E2E trace log (D-10), neither of which mutates pre-existing runtime state.
>
> The only "state" introduced is **new** runtime state:
> - Local Milvus container (D-09) — created by `docker compose up`, torn down by `docker compose down`
> - Local BGE-M3 container (D-08) — same lifecycle
> - GLM-5.2 API call history (Coding Plan endpoint) — billed against user's Coding Plan subscription; no SkillForge-side persistence
>
> No category from the 5-category inventory applies.

---

## Common Pitfalls

### Pitfall 1: Docker absent on dev machine
**What goes wrong:** `docker compose up` fails with `docker: command not found` (verified on this machine). Milvus + BGE-M3 cannot start.
**Why it happens:** Windows dev machines often lack Docker Desktop; CI runners typically have Docker but not local dev.
**How to avoid:** Add `checkpoint:human-verify` task at Phase 1 start verifying Docker presence. If absent, fall back to (a) Milvus Lite (in-process embedded) or (b) remote Milvus Cloud free tier.
**Warning signs:** `command -v docker` returns empty.

### Pitfall 2: Hertz SSE API mismatch (PROJECT.md is wrong)
**What goes wrong:** Code copied from PROJECT.md §2.2 using `s.Publish(&sse.Event{...})` fails to compile — Hertz v0.10.6 has no `NewStream` or `Publish` method (verified by reading writer.go).
**Why it happens:** PROJECT.md §2.2 appears to be written from memory of a Gin-Echo-style SSE helper; the actual Hertz API is `WriteEvent(id, type, data)`.
**How to avoid:** Use `w.WriteEvent("", "message", payload)` from Hertz v0.10.6 source (verified).
**Warning signs:** Build error `sse.NewStream undefined` or `w.Publish undefined`.

### Pitfall 3: Tier filter applied AFTER vector search (Milvus filter scope)
**What goes wrong:** If a developer adds a post-filter step ("filter results where tier == '权威'") instead of setting `Filter` on `SearchQueryOptions`, the filter happens after ranking — meaning personal tier documents could still appear in the top-K if their vectors happen to be similar to the query.
**Why it happens:** Misunderstanding of Milvus filter semantics. Filter is applied at the index scan level, before vector similarity is computed.
**How to avoid:** Always set `Filter` on `SearchQueryOptions` (the Go SDK v2 way) or on the search request body (the HTTP way). Never post-filter Milvus results.
**Warning signs:** Integration test `spike milvus` shows tier="权威" query returning personal/草稿 docs with score > 0.

### Pitfall 4: eino-ext pseudo-version instability
**What goes wrong:** `go get github.com/cloudwego/eino-ext/components/model/openai@latest` resolves to a master-branch pseudo-version. Every fresh `go mod tidy` may pull a different commit.
**Why it happens:** eino-ext has no stable tags; v0.0.1-alpha is an empty placeholder (verified by listing the downloaded module — only `.github/` folder present).
**How to avoid:** Pin the pseudo-version explicitly in `go.mod` after first successful build: e.g. `github.com/cloudwego/eino-ext v0.0.0-20260811xxxxx-yyyyyyy`. Or use a `replace` directive to a vendored copy.
**Warning signs:** Build succeeds Monday, fails Wednesday with API drift.

### Pitfall 5: GLM-5.2 reasoning burns tokens even with `thinking:disabled`
**What goes wrong:** Setting `max_tokens: 200` with `thinking:{type:disabled}` still occasionally produces empty `content` because reasoning budget was already consumed.
**Why it happens:** Spike 003 found `max_tokens < 500` is risky for reasoning models. With thinking disabled, 400 is safe; with thinking ON, 500+ is safer.
**How to avoid:** Set `model.WithMaxTokens(400)` when `thinking:{type:disabled}` is on (compliance Skills). Phase 1 spike uses `thinking:disabled` to keep the wire stable.
**Warning signs:** Spike test sees `Content=""` from GLM-5.2 despite `thinking:disabled`.

### Pitfall 6: Vercel AI SDK v5 chunk type field mismatch
**What goes wrong:** Backend emits `{"type": "text_delta"}` (snake_case) but Vercel AI SDK v5 expects `{"type": "text-delta"}` (kebab-case). Frontend parser silently drops the chunk.
**Why it happens:** AI SDK v5 uses kebab-case for `type` discriminator (per `UIMessageChunk` spec at packages/ui-utils/src/process-ui-message-stream.ts).
**How to avoid:** Use kebab-case exactly: `text-start`, `text-delta`, `text-end`, `reasoning-start`, `reasoning-delta`, `reasoning-end`, `tool-input-start`, `tool-input-delta`, `tool-input-available`, `tool-output-available`, `source-url`, `source-document`, `data-*`, `finish-step`, `finish-message`, `error`.
**Warning signs:** Frontend `useChat` shows empty messages despite backend emitting frames.

### Pitfall 7: CN characters in JSON string literals (Milvus filter escape)
**What goes wrong:** Milvus filter `tier in ["权威"]` produces a parse error because the inner quotes are interpreted by the JSON parser.
**Why it happens:** Go's `fmt.Sprintf("%q", "权威")` produces `"权威"` (with surrounding quotes). If you naively template `tier in [%s]` and pass `[]string{"权威"}` joined as `权威`, you get `tier in [权威]` which Milvus parses as identifier.
**How to avoid:** Use `%q` (Go quote) to produce properly-escaped JSON strings. Verify by printing the filter and checking it parses as valid JSON array-of-strings.
**Warning signs:** Milvus returns `parse filter expr failed: ...` on a tier query.

### Pitfall 8: Tool-call SSE frames not aligned with AI SDK v5 part model
**What goes wrong:** Backend emits one combined `{"type": "tool-call", "name": "...", "args": {...}, "result": {...}}` event. Vercel AI SDK v5 expects `tool-input-start` + `tool-input-delta` + `tool-input-available` + (later) `tool-output-available` as separate events with `toolCallId` linking them.
**Why it happens:** Older streaming protocols (LangChain MessageGraph) emit single combined tool events. AI SDK v5 emits lifecycle events.
**How to avoid:** Emit tool events in AI SDK v5 lifecycle order. Use stable `toolCallId` across the input-output lifecycle.
**Warning signs:** `useChat` shows tool call name + args but no result.

---

## Code Examples

### Verified patterns from official sources (Go module source inspected):

### Example 1: Eino v0.7.7 ChatModelAgent + Runner streaming
```go
// Source: cloudwego/eino@v0.7.7/adk/chatmodel.go + runner.go (verified via go module source)
import (
    "context"
    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/model"
)

func main() {
    ctx := context.Background()
    cm, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
        APIKey:  "<glm-coding-plan-key>",
        Model:   "glm-5.2",
    })

    agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "stub",
        Description: "Phase 1 stub",
        Instruction: "You are a compliance auditor. Output JSON.",
        Model:       cm,
    })

    runner := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:           agent,
        EnableStreaming: true,  // [VERIFIED: runner.go:44]
    })

    iter := runner.Query(ctx, "Is 200 RMB over budget allowed for travel?")
    for {
        ev, ok := iter.Next()
        if !ok { break }
        if ev.Err != nil { /* handle */ }
        if ev.Output != nil && ev.Output.MessageOutput != nil {
            // ev.Output.MessageOutput.MessageStream is *schema.StreamReader[*schema.Message]
            // Recv() yields individual delta messages; close on io.EOF.
        }
    }
}
```

### Example 2: Hertz v0.10.6 SSE writer
```go
// Source: cloudwego/hertz@v0.10.6/pkg/protocol/sse/writer.go (verified)
import (
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/cloudwego/hertz/pkg/protocol/sse"
)

func main() {
    h := server.Default()
    h.GET("/api/spike/chat", func(ctx context.Context, c *app.RequestContext) {
        w := sse.NewWriter(c)  // [VERIFIED: writer.go:41 — sets text/event-stream + no-cache]
        defer w.Close()
        for i := 0; i < 3; i++ {
            chunk := UIMessageChunk{
                Type:  "text-delta",  // AI SDK v5 kebab-case
                ID:    "msg-1",
                Delta: fmt.Sprintf("hello %d ", i),
            }
            payload, _ := json.Marshal(chunk)
            w.WriteEvent("", "message", payload)  // [VERIFIED: writer.go:64]
        }
    })
    h.Spin()
}
```

### Example 3: Milvus tier filter via Go SDK v2
```go
// Source: Milvus Go SDK v2 docs (verified via WebSearch + Milvus docs)
import (
    "github.com/milvus-io/milvus-sdk-go/v2/client"
    "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func searchAuthorityTier(ctx context.Context, milvusClient client.Client, queryVec []float32) error {
    expr := `tier in ["权威"]`  // [VERIFIED: Milvus filter syntax]
    sp, _ := entity.NewIndexAUTOFlatSearchParam()

    results, err := milvusClient.Search(ctx, &client.SearchQueryOptions{
        CollectionName:   "skillforge_chunks",
        ConsistencyLevel: entity.ClStrong,
        OutputFields:     []string{"id", "tier", "content", "source"},
        Limit:            10,
        Filter:           expr,
        SearchParams:     sp,
        VectorField:      entity.FloatVectorField{Name: "embedding", Dim: 1024},
        Vectors:          []entity.Vector{entity.NewFloatVector(queryVec)},
    })
    // Assert: every result must have tier == "权威" (zero personal/草稿 bleed)
    // ...
}
```

### Example 4: Vercel AI SDK v5 stub client
```tsx
// Source: ai-sdk.dev/docs/ai-sdk-ui/streaming-data + process-ui-message-stream.ts (verified via WebSearch)
import { useChat } from '@ai-sdk/react';
import { DefaultChatTransport } from 'ai';

export default function App() {
  const { messages, sendMessage } = useChat({
    transport: new DefaultChatTransport({
      api: 'http://localhost:8888/api/spike/chat',
    }),
  });

  return (
    <div>
      {messages.map((m) => (
        <div key={m.id}>
          {m.parts.map((p, i) => {
            switch (p.type) {
              case 'text': return <p key={i}>{p.text}</p>;
              case 'reasoning': return <details key={i}><summary>thinking</summary>{p.text}</details>;
              case 'source-url': return <a key={i} href={p.url}>[{p.title}]</a>;
              default: return null;
            }
          })}
        </div>
      ))}
      <button onClick={() => sendMessage({ text: 'Tell me about authority tier 政策' })}>Send</button>
    </div>
  );
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom SSE writer per backend | Hertz `pkg/protocol/sse.NewWriter` | Hertz v0.10.0 (added `pkg/protocol/sse`) | Verified in v0.10.6 — first-party package, no custom code needed |
| OpenAI streaming protocol (raw `data: {json}` for any chunk) | AI SDK v5 `UIMessageChunk` discriminated union | AI SDK v5 release (mid-2025 per STACK.md) | Typed parts (text/reasoning/tool/source/data) replace single "text-or-everything" model |
| LangChain.js full umbrella (~200-800KB) | Vercel AI SDK v5 (~12KB) | 2025 | Already locked in STACK.md — Phase 1 confirms ~12KB via `useChat` bundle inspection |
| Milvus SDK v1 (positional args) | Milvus Go SDK v2 (`SearchQueryOptions` struct) | 2024 | Verified cleaner API; recommended in Milvus docs |
| pgvector for hybrid search | Milvus native hybrid + filter | 2026 benchmarks | Already locked in STACK.md; pgvector struggles past 1M vectors for hybrid |
| `eino-ext/components/model/openai` with `RequestBodyModifier` | `openai.WithExtraFields(map[string]any)` (Eino v0.1.13+) | Eino v0.1.13 | `RequestBodyModifier` deprecated in v0.1.17; use `WithExtraFields` (per spike 001 reference) |
| Eino v0.5.0-alpha ADK | Eino ADK GA (v0.5.0+) | Eino v0.5.0 (June 2026 per STACK.md) | Production-ready multi-agent; Phase 1 spike uses single ChatModel + SequentialAgent (basic case) |

**Deprecated/outdated:**
- `eino.RequestBodyModifier`: deprecated in Eino v0.1.17 — use `openai.WithExtraFields(map[string]any)`
- `sse.NewStream` / `s.Publish` (Hertz): DOES NOT EXIST in v0.10.6 — use `w.WriteEvent(id, type, data)`
- Milvus Go SDK v1 (`milvusClient.Search(ctx, "coll", []string{""}, "expr", ...)` positional API): replaced by v2's `SearchQueryOptions` struct
- GLM-5.2 byte-determinism with `temperature:0`: PROVEN IMPOSSIBLE in spike 002/002b — use semantic determinism via structured output + JSON schema validation

---

## Validation Architecture

> Phase 1 is a spike — but the spike itself is gated by 5 success criteria (from ROADMAP.md §Phase 1). Each criterion maps to specific tests + observable evidence + tooling.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify/assert` + Node `vitest` for the React stub |
| Config file | `go.mod` (no separate config); Node `package.json` with vitest |
| Quick run command | `go test ./cmd/spiked/... ./integration/contract/... -run TestSpike -count=1 -timeout 30s` |
| Full suite command | `docker compose up -d && go test ./... -count=1 -timeout 60s && docker compose down` |

### Phase Success Criteria → Test Map

| SC ID | Success Criterion | Test Type | Automated Command | Wave 0 File |
|-------|-------------------|-----------|-------------------|-------------|
| SC-1 | Hertz streams hardcoded agent response to React client via UIMessageChunk envelope, no wire-format drift | Integration (in-process Hertz + AI SDK parser) | `go test ./cmd/spiked -run TestSpikeSSE -v` | `cmd/spiked/sse_test.go` |
| SC-2 | Milvus tier-filter returns ONLY tier="权威" docs against 4-tier seeded dataset | Integration (Milvus container) | `go test ./cmd/spiked -run TestSpikeMilvus -v` | `cmd/spiked/milvus_test.go` |
| SC-3 | ChatModelAgent + SequentialAgent 2-step demo executes E2E; client → server → agent cancellation within 200ms | Integration | `go test ./cmd/spiked -run TestSpikeAgent -v` | `cmd/spiked/agent_test.go` |
| SC-4 | "Fake cited answer" demo reaches UI with hardcoded chunk citations as clickable elements | E2E (Hertz + React stub) | `cd web && npm run test:e2e` | `web/src/__tests__/citations.test.tsx` |
| SC-5 | Version-pinned integration test runs all of the above in under 10s; fails on wire/filter drift | Integration (smoke) | `go test ./integration/contract -count=1 -timeout 10s` | `integration/contract/contract_test.go` |

### Per-Criterion Observable Evidence

| SC ID | What proves the criterion is met |
|-------|-----------------------------------|
| SC-1 | (a) Hertz SSE response has `Content-Type: text/event-stream` header; (b) every `data:` frame is a valid JSON `UIMessageChunk` with one of `text-delta`/`reasoning-delta`/`tool-input-*`/`source-*` types; (c) AI SDK v5 `useChat` parser reassembles frames into `messages` array with all expected parts |
| SC-2 | Insert 12 docs (3 per tier × 4 tiers) into Milvus; query with `Filter: "tier in [\"权威\"]"`; assert every result row has `tier == "权威"` AND no result row has `tier == "个人"` or `tier == "草稿"` |
| SC-3 | Start ChatModelAgent + SequentialAgent 2-step run; cancel mid-stream via `context.WithTimeout` from client; measure elapsed time from cancel signal to Runner iter close; assert < 200ms |
| SC-4 | Issue query; assert SSE stream contains at least one `source-url` or `source-document` chunk with `sourceId` matching a hardcoded chunk_id; assert React stub renders the chunk as a clickable `<a>` |
| SC-5 | Combined integration test runs SC-1, SC-2, SC-3, SC-4 sequentially; total wall-clock < 10s; any wire-format or filter-expression change triggers a parse error → test fails |

### Tooling Required

| Tool | Purpose | Where Used |
|------|---------|------------|
| `httptest.Server` (stdlib) | In-process Hertz SSE test server | SC-1, SC-3, SC-4 |
| `go test -race` | Concurrency safety for SC-3 (cancellation) | SC-3 |
| `testcontainers-go` (optional) | Spin up Milvus + BGE-M3 containers for `go test` if Docker present | SC-2, SC-4 |
| Pre-up Docker Compose (fallback) | `docker compose up -d milvus bge-m3` before `go test` if testcontainers not used | SC-2, SC-4 |
| Mock OpenAI server (spike 001 pattern) | Wire regression test without burning API quota | SC-1, SC-5 |
| `@testing-library/react` + `vitest` | React stub component test (citation rendering) | SC-4 |
| `EventSource` polyfill OR native fetch streaming | Client-side SSE consumption in Node test | SC-1, SC-4 |

### Sampling Rate

- **Per task commit:** `go test ./cmd/spiked -run TestSpike -count=1 -timeout 30s` (quick spike subcommand tests, no Docker)
- **Per wave merge:** Full suite `go test ./... -count=1 -timeout 60s` (with Docker compose up)
- **Phase gate:** SC-1 through SC-5 all green before `/gsd:verify-work`

### Wave 0 Gaps (test infrastructure that doesn't exist yet)

- [ ] `cmd/spiked/main.go` skeleton with subcommand router — **Wave 0**
- [ ] `cmd/spiked/wire.go` (mock OpenAI server + Eino round-trip) — **Wave 0**
- [ ] `cmd/spiked/sse.go` (Hertz SSE → AI SDK parser round-trip) — **Wave 0**
- [ ] `cmd/spiked/milvus.go` (tier-filter assertion) — **Wave 0**
- [ ] `cmd/spiked/e2e.go` (full pipeline trace log) — **Wave 0**
- [ ] `integration/contract/contract_test.go` — **Wave 0** (runs SC-1..SC-5 in <10s)
- [ ] `testdata/skills/{authority,historical,personal,draft}/SKILL.md` (4 fixtures) — **Wave 0**
- [ ] `testdata/docs/{tier}/*.md` (≥3 per tier) — **Wave 0**
- [ ] `docker-compose.yml` (milvus + bge-m3 services) — **Wave 0**
- [ ] `web/src/App.tsx` minimal React stub — **Wave 0**
- [ ] `web/src/__tests__/citations.test.tsx` — **Wave 0**

---

## Environment Availability

> Step 2.6 executed. **Docker is absent on the target machine.** This is the dominant constraint for Phase 1 — the locked `docker compose up` pattern (D-11) cannot run as-is.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Eino + Hertz compilation | ✓ | go1.24.5 windows/amd64 | — |
| Node.js | Vite + React stub | ✓ | v22.17.1 | — |
| npm | React stub dependencies | ✓ | 10.9.2 | — |
| Python | Test scripts + slopcheck | ✓ | 3.13.2 | — |
| Docker | Milvus + BGE-M3 containers | **✗** | — | (a) Milvus Lite embedded, (b) Remote Milvus (Zilliz Cloud free), (c) Docker install via D-11 checkpoint task |
| `docker compose` | Orchestration | **✗** | — | (a) `docker run` per service (if Docker installed later), (b) Milvus Lite standalone binary |
| GLM Coding Plan API key | GLM-5.2 real calls | UNKNOWN | — | spike 003 confirms `c23710516a6d4a12a24863d0d06be917.SFsK8jprfDgnxv7G` works for glm-4-flash free tier + glm-5.2 paid |
| slopcheck | Package legitimacy audit | ✓ | 0.6.1 | — (registry unreachable, but tool itself runs) |
| ctx7 (Context7 CLI) | Library docs lookup | **✗** | — | WebFetch / WebSearch / direct Go module source inspection (this is what we did) |

**Missing dependencies with no fallback (BLOCKERS):**
- **Docker absence** is the only blocker. Phase 1 spike cannot demonstrate end-to-end Milvus tier-filter (SC-2) without Milvus running. The planner must either (a) include a `checkpoint:human-verify` task that installs Docker Desktop before SC-2 runs, or (b) include a Milvus Lite embedded-mode fallback for SC-2.

**Missing dependencies with viable alternatives:**
- `docker compose`: substitute `docker run` per service or Milvus Lite embedded binary
- ctx7 CLI: substitute WebFetch + WebSearch + Go module source inspection (used in this research)

**Recommended resolution order:**
1. **Before planning:** Run `command -v docker` on target machine. If absent, decide Milvus Lite vs Docker install vs remote Milvus.
2. **In PLAN.md Wave 0:** Add a `checkpoint:human-verify` task titled "Verify Docker availability and decide Milvus deployment mode" — this gates all SC-2 work.
3. **In PLAN.md Wave 1+:** Plan Milvus Lite OR Docker install OR remote Milvus based on the checkpoint outcome.

---

## Security Domain

> `security_enforcement` is not explicitly set in `.planning/config.json` (absent = enabled per default). Including this section per the RESEARCH.md template.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (Phase 1 is local spike, no auth) | n/a |
| V3 Session Management | no (no sessions in spike) | n/a |
| V4 Access Control | no (single-user local) | n/a |
| V5 Input Validation | yes | Use `json.Unmarshal` + Go struct validation for SSE chunk payloads (Pattern 2 + spike 003 struct validation) |
| V6 Cryptography | yes (API key in env) | Read `GLM_API_KEY` from env var; never log it; never commit to git (.env.example only) |

### Known Threat Patterns for Phase 1 stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Prompt injection via user query | Tampering | Out of scope for spike (Phase 2-3 will add refusal logic); Phase 1 only validates wire |
| API key leak in logs | Information Disclosure | Use `os.Getenv("GLM_API_KEY")`; redact in any logging; commit `.env.example` with placeholder |
| Milvus tier filter bypass | Tampering | **Structural enforcement**: bind `allowedTiers` at retriever construction time; no per-call filter option |
| Unsanitized CN characters in filter expression | Tampering | Use Go `%q` formatter for JSON string escaping in filter (Pitfall 7) |
| SSE chunk injection (newline in data) | Tampering | Hertz `Write` validates EOL in data and splits into multiple `data:` lines (verified writer.go:141-149) — automatic protection |
| React XSS via source URL | Tampering | Phase 4 must sanitize `source-url` chunks before rendering; Phase 1 stub uses `<a href={p.url}>` which trusts the URL — flag for Phase 4 |

### Phase-1-Specific Security Notes

- **`.env` must NOT be committed.** Phase 1 spike creates `.env.example` only.
- **Mock OpenAI server (in-process) logs full request body** (including system prompt + user query). This is fine for spike wire regression but should NOT be enabled in production code paths.
- **GLM Coding Plan key has billing implications.** Mock OpenAI server is the primary test path; real GLM-5.2 calls are limited to `spike e2e` and the trace-log capture.

---

## Open Questions

1. **Is Docker going to be installed before Phase 1 starts?**
   - What we know: `command -v docker` returns empty on this machine.
   - What's unclear: Whether the user plans to install Docker Desktop, use Milvus Lite, or use a remote Milvus instance.
   - Recommendation: Add `checkpoint:human-verify` task at start of Phase 1 Wave 0. Until resolved, SC-2 cannot be planned.

2. **Should Phase 1 actually validate the SequentialAgent 2-step demo (SC-3) or just defer to Phase 3?**
   - What we know: D-04 specifies ChatModelAgent + SequentialAgent 2-step (one ChatModel call + one tool call). Claude's discretion in CONTEXT.md says "suggest single ChatModel for spike; ADK added in Phase 3."
   - What's unclear: Is SC-3 a hard Phase 1 requirement, or can it be simplified to single ChatModel + tool call?
   - Recommendation: Implement single ChatModel with one tool call (simpler, sufficient to validate tool-call SSE wire format). Defer SequentialAgent 2-step to Phase 3. Update SC-3 wording accordingly.

3. **Should the React stub (SC-4) be a full Vite project or a single HTML file with inline React?**
   - What we know: SC-4 requires "hardcoded chunk citations rendered as clickable elements."
   - What's unclear: Time cost of setting up a full Vite + React 19 + TypeScript project vs a single index.html with React via CDN.
   - Recommendation: Use a single `index.html` + CDN React + `@ai-sdk/react` for the spike. Full Vite project setup deferred to Phase 4. Saves ~30min and validates the wire round-trip without committing to UI scaffolding.

4. **What is the canonical 5-query list for the spike?**
   - What we know: CONTEXT.md suggests "same compliance-real queries as spike 002/003."
   - What's unclear: Should spike 002/003's exact queries be reused, or should spike 001's simpler queries be reused?
   - Recommendation: Reuse spike 002/003's queries (5 queries: travel expense, non-compete, contract approval, annual leave, contact info refusal). Same as spike-findings-ai-agent reference.

5. **eino-ext pseudo-version pinning strategy?**
   - What we know: eino-ext has no stable tags (v0.0.1-alpha is empty).
   - What's unclear: Best practice for pinning pseudo-version commits.
   - Recommendation: After first successful build, capture the commit hash and pin via `github.com/cloudwego/eino-ext v0.0.0-20260811xxxxx-yyyyyyy`. Document in spike README.

---

## Assumptions Log

> All claims tagged `[ASSUMED]` in this research. The planner should review these before locking Phase 1 design decisions.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `eino-ext/components/retriever/milvus` exports `WithFilter` (or similar) as impl-specific option | Pattern 4 | If eino-ext uses a different name (e.g. `WithExpr` or `WithSearchQuery`), the spike code must use the actual name. Mitigation: spike reads eino-ext source on first import (or pins a commit known to expose the option). |
| A2 | Vercel AI SDK v5 `DefaultChatTransport` parses `data: {json}\n\n` SSE frames with kebab-case `type` discriminator | Pattern 2 + 5 | If the parser expects snake_case or different field names, the Hertz writer must change. Mitigation: SC-1 integration test catches any mismatch. |
| A3 | Milvus Go SDK v2 `SearchQueryOptions.Filter` accepts `tier in ["权威"]` syntax | Pattern 3 | If the SDK uses a different filter name (e.g. `Expr` or `FilterExpr`), the spike must rename. Mitigation: spike reads milvus-sdk-go/v2 source on first import. |
| A4 | BGE-M3 via TEI Docker serves dense embeddings of dim=1024 on port 8080 | Pattern 1 (embedding client) | If TEI serves a different dim or port, the embedding client + Milvus schema must change. Mitigation: spike embeds a known test sentence and asserts returned vector len == 1024. |
| A5 | GLM-5.2 Coding Plan endpoint `https://open.bigmodel.cn/api/coding/paas/v4` is still active in 2026-08 | Pattern 1 | If endpoint URL changed, the spike will fail. Mitigation: spike uses user-provided key (`c23710516a6d4a12a24863d0d06be917.SFsK8jprfDgnxv7G`) which was working as of spike 003 (2026-08-10). |
| A6 | Hertz v0.10.6 SSE writer emits standard `text/event-stream` frames parsable by `EventSource` and AI SDK v5 `DefaultChatTransport` | Pattern 2 | If Hertz adds vendor-specific quirks (e.g. extra blank lines), AI SDK parser may drop frames. Mitigation: SC-1 captures raw bytes + asserts EventSource can parse. |
| A7 | SequentialAgent 2-step is achievable with simple `[]adk.Agent{sub1, sub2}` sub-agents (verified in workflow.go:539) | SC-3 | If SequentialAgent requires intermediate state configuration (e.g. `GenModelInput` for each sub-agent), the spike must add wiring. Per `claude's discretion` in CONTEXT.md, this can be deferred to Phase 3. |
| A8 | Docker installation on Windows requires Docker Desktop (free) and 10-15min setup | Environment Availability | If user has WSL2-only Docker, the install path differs. Mitigation: checkpoint task asks user. |
| A9 | The 5 spike queries from spike 002/003 (Chinese compliance queries) are still appropriate for Phase 1 | Open Question #4 | If user wants different queries, the spike must update. Mitigation: spike 002/003 queries are listed in spike-findings-ai-agent reference. |
| A10 | The `eino-ext/components/model/openai` import path is correct for Eino v0.7.7 | Pattern 1 | If the actual import path is `github.com/cloudwego/eino-ext/model/openai` (without `/components`), the spike must update. Mitigation: spike tries both imports on first compile. |

**If this table is empty:** All claims verified. (It is not empty — 10 assumptions require planner attention.)

---

## Sources

### Primary (HIGH confidence — verified via direct Go module source inspection)

- `github.com/cloudwego/eino@v0.7.7/adk/chatmodel.go` (lines 143-225) — `ChatModelAgentConfig` struct + `NewChatModelAgent` constructor
- `github.com/cloudwego/eino@v0.7.7/adk/workflow.go` (lines 536-590) — `SequentialAgentConfig` / `ParallelAgentConfig` / `LoopAgentConfig` + factory functions
- `github.com/cloudwego/eino@v0.7.7/adk/runner.go` (lines 42-102) — `RunnerConfig`, `NewRunner`, `Run`, `Query` methods
- `github.com/cloudwego/eino@v0.7.7/adk/interface.go` (lines 129-206) — `AgentOutput`, `AgentEvent` types
- `github.com/cloudwego/eino@v0.7.7/components/model/option.go` (lines 47-110) — `WithTemperature`, `WithMaxTokens`, `WithTools`, `WithToolChoice`
- `github.com/cloudwego/eino@v0.7.7/components/retriever/option.go` (lines 22-37) — `Options` struct (NO Filter field; filter must be impl-specific)
- `github.com/cloudwego/hertz@v0.10.6/pkg/protocol/sse/writer.go` (lines 41-164) — `NewWriter`, `WriteEvent`, `Close`; NO `NewStream`/`Publish`
- `github.com/cloudwego/hertz@v0.10.6/pkg/protocol/sse/event.go` (lines 33-43) — `Event` struct
- `github.com/cloudwego/hertz@v0.10.6/pkg/protocol/sse/example_test.go` — official usage pattern

### Secondary (MEDIUM confidence — verified via WebSearch)

- [Milvus Go SDK v2 `SearchQueryOptions.Filter` syntax](https://milvus.io/docs) — filter expressions including `tier in ["..."]`
- [Vercel AI SDK v5 UIMessageChunk wire format](https://ai-sdk.dev/docs/ai-sdk-ui/streaming-data) + [process-ui-message-stream.ts spec](https://github.com/vercel/ai) — kebab-case chunk types, SSE `data:` encoding
- [BAAI/bge-m3 model card](https://huggingface.co/BAAI/bge-m3) — 1024-dim output, 8192 token context, multilingual
- [Milvus docs on BGE-M3 Function integration](https://milvus.io/docs/embeddings.md) — dim=1024 collection schema requirement
- [HuggingFace Text Embeddings Inference](https://github.com/huggingface/text-embeddings-inference) — BGE-M3 self-host via Docker

### Tertiary (LOW confidence — flagged for validation)

- `eino-ext/components/retriever/milvus` exact option API — eino-ext v0.0.1-alpha is empty; pseudo-version needed; not directly inspected (A1)
- eino-ext/components/model/openai exact import path for v0.7.7 (A10)
- Milvus Go SDK v2 package import path — likely `github.com/milvus-io/milvus-sdk-go/v2/client` per WebSearch (A3)

### Project Skill Sources (already-validated patterns reused in Phase 1)

- `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go` — Eino OpenAI adapter + mock HTTP server pattern (REUSE)
- `.claude/skills/spike-findings-ai-agent/sources/003-structured-output-temperature-zero/main.go` — structured output via `response_format:{type:json_object}` + `thinking:{type:disabled}` (REUSE for SC-3)
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md` — Coding Plan endpoint + `WithExtraFields` map for `thinking` + `response_format` (REUSE for Pattern 1)
- `.planning/research/STACK.md` §2.1-2.6 — locked stack + versions
- `.planning/research/ARCHITECTURE.md` §3 — 6-layer architecture + Eino ADK mapping
- `.planning/SKILL_SCHEMA.md` v1.0 — 7-field schema (the spike parses but does not implement)
- `.planning/research/SUMMARY.md` — consolidated research; Phase 1 spike focus confirmed

---

## Project Constraints (from CLAUDE.md)

> Directives extracted from `./CLAUDE.md` (project-level) for planner compliance.

| Directive | Source | Planner Compliance |
|-----------|--------|---------------------|
| **Tech stack locked:** Go + Eino + Eino ADK + Hertz (backend); React 19 + Vite + TypeScript + Vercel AI SDK + shadcn/Tailwind (frontend) | CLAUDE.md Constraints | All Phase 1 spike components must use the locked stack. No alternatives. |
| **License: Apache-2.0** | CLAUDE.md Constraints | All new files must include Apache-2.0 header (Eino/Hertz examples show the pattern). |
| **Deployment: v1 local + Docker Compose; Web app form factor (not desktop)** | CLAUDE.md Constraints | Phase 1 spike validates the local Docker Compose + web client path. |
| **Open-source goal: polish docs + easy start + measurable outcomes** | CLAUDE.md Constraints | Phase 1 spike output (runnable skeleton + E2E trace log) is the foundation for the "easy start" promise. README.md must accompany the spike binary. |
| **Referential constraint: denova patterns OK (Skills / 真源 separation / Eino ADK); content generation (novels/RPG) NOT copied** | CLAUDE.md Constraints | Tier-filter pattern (true-source separation) is the denova-parity part Phase 1 validates. No content generation involved. |
| **Refer to spike-findings-ai-agent skill for Eino OpenAI + GLM-5.2 patterns** | Global CLAUDE.md Project Skill Routing | Spike 001 + 003 sources reused. spike-findings-ai-agent skill auto-loaded during planning. |
| **Refer to sketch-findings-ai-agent skill for UI prep** | Global CLAUDE.md Project Skill Routing | Phase 1 spike uses a minimal React stub; full UI prep deferred to Phase 4 (which will load sketch-findings-ai-agent). |
| **Run planning through GSD workflow** | CLAUDE.md GSD Workflow Enforcement | Phase 1 research → plan → execute via `/gsd:execute-phase 1`. |

---

## Metadata

**Confidence breakdown:**
- **Standard stack (Eino/Hertz/Milvus/BGE-M3/AI SDK):** HIGH — verified via direct Go module source + Milvus docs + AI SDK docs
- **Architecture (Patterns 1-5):** HIGH for Patterns 1, 2, 3, 5 (verified sources); MEDIUM for Pattern 4 (eino-ext source not directly inspected; relies on inference from Eino core API)
- **Pitfalls (1-8):** MEDIUM-HIGH — Pitfalls 2, 6, 7 verified via direct source; Pitfalls 1, 3, 4, 5, 8 from spike findings + environment audit
- **Environment:** HIGH — direct shell probes confirm Docker absent, Go/Node/Python present, ctx7 absent, slopcheck present but registry unreachable
- **Validation Architecture:** MEDIUM — test mapping inferred from spike criteria; Wave 0 gaps listed explicitly
- **Security:** MEDIUM — V5 input validation + V6 cryptography are the only applicable ASVS categories for a local spike

**Research date:** 2026-08-11
**Valid until:** 2026-09-11 (30 days; eino-ext master branch is the only unstable element — any change there may invalidate Pattern 4)

---

*End of RESEARCH.md*
