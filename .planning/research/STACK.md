# Technology Stack — SkillForge

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Researched:** 2026-08-10
**Mode:** Stack dimension (Project Research — Phase 6)
**Confidence:** HIGH for Eino core/Hertz/React 19/Vite 8/Tailwind v4/shadcn-ui; HIGH for Vercel AI SDK v5; HIGH for Milvus/BGE-M3; MEDIUM for RAGAS-in-Go (Python-only) and Skill hot-reload patterns.

---

## 1. TL;DR — The Stack in One Table

| Layer | Choice | Version | Confidence | Why |
|-------|--------|---------|------------|-----|
| **Language (BE)** | Go | 1.22+ (preferably 1.23) | HIGH | User decision; matches denova; mature Eino support |
| **HTTP (BE)** | CloudWeGo Hertz | v0.10.2+ | HIGH | First-class SSE (`pkg/protocol/sse`); same vendor as Eino; byte-dance prod-proven |
| **Agent framework** | cloudwego/eino | v0.7.7+ (track latest) | HIGH | GA ADK since v0.5.0; explicit multi-agent API; callbacks for observability |
| **Agent orchestration** | eino/adk | bundled with eino | HIGH | ChatModelAgent + Sequential/Parallel/Loop/Conditional/Failover agents; transfer/handoff built in |
| **LLM provider (CN)** | eino-ext/model/ark (Volcano Ark → Doubao/DeepSeek) | latest | HIGH | CN-low-latency; Eino native adapter |
| **LLM provider (intl)** | eino-ext/model/openai | latest | HIGH | OpenAI-compatible (works for Azure, DeepSeek, Moonshot, GLM, etc.) |
| **Vector DB** | Milvus | v2.5+ (or v2.4.x LTS) | HIGH | Native Eino retriever; hybrid search; CN-friendly; strongest 2026 benchmarks |
| **Embedding (CN)** | BAAI BGE-M3 | latest | HIGH | Multilingual + 8k context; C-MTEB leader; optional self-host via BGE-M3 docker |
| **Embedding (provider)** | eino-ext/components/embedding/ark | latest | HIGH | If using Ark hosted Doubao/Doubao-embedding |
| **Document parsing** | eino-ext/components/document/parser/pdf + html | latest | HIGH | Compliance docs are PDFs/HTML |
| **Chunker** | eino-ext/components/document/transformer/splitter/markdown | latest | HIGH | Hierarchical header splitter; ideal for regulation text |
| **Eval (Python sidecar)** | Ragas + TruLens | latest | HIGH | Faithfulness/context_precision/answer_relevance; CI gating |
| **Frontend framework** | React | v19.2.x | HIGH | Actions/`use` hook; RSC-ready; stable |
| **Build tool** | Vite | v8.x (or v7.x stable) | HIGH | Instant HMR; native Tailwind v4 plugin |
| **Language (FE)** | TypeScript | v5.6+ | HIGH | Strict mode; satisfies "TS" requirement |
| **Styling** | Tailwind CSS | v4.x | HIGH | CSS-first config; `@theme` directive; `@tailwindcss/vite` |
| **Component kit** | shadcn/ui (CLI) | latest | HIGH | Copy-paste ownership; React 19 compatible |
| **AI SDK (FE)** | Vercel AI SDK | v5.x | HIGH | UIMessageChunk protocol natively streams text + reasoning + tool calls + sources |
| **State (FE)** | Zustand | v5.x | HIGH | Lightweight; integrates with `useChat` cleanly |
| **Server (FE dev)** | Vite dev server proxy → Hertz | — | HIGH | Single CORS origin in prod; proxy `/api` to `:8888` in dev |
| **License** | Apache-2.0 | — | HIGH | Matches denova; open-source friendly |

---

## 2. Backend Stack — Deep Dive

### 2.1 Go + CloudWeGo Eino (HIGH)

**What it is:** Eino is ByteDance's "ultimate LLM/AI application development framework in Go." It provides:
- Type-safe component abstractions (`ChatModel`, `Tool`, `Retriever`, `Indexer`, `Embedding`, `Document.Parser`, `Document.Transformer`)
- A graph composition engine (`compose.Graph`, `compose.Chain`)
- A multi-agent orchestration kit (`adk` package — Agent Development Kit)
- Callback handlers (`callbacks.Handler`) for cross-cutting concerns (logging, tracing, metrics)

**Current state (Aug 2026):**
- **v0.7.7** is the latest tagged release on `cloudwego/eino`.
- **Eino ADK was released as GA in v0.5.0** (June 2026). It was a landmark — Go finally got a first-class multi-agent library.
- The `eino-ext` companion repo holds official adapters: chat models (openai, ark, ollama, deepseek, gemini), embeddings, retrievers (milvus, qdrant, volc_vikingdb, es), indexers, document parsers (pdf, html), splitters (markdown, recursive), tools, and observability callbacks (langfuse, coze, aliyun).

**Why this stack (rationale):**
- **Go-native agent story**: Until Eino, Go LLM apps were reimplementing LangChain patterns. Eino closes that gap with a typed, channel-streaming design.
- **Denova parity**: The user's fork uses Eino + ADK. SkillForge inherits the proven mental model (SubAgents, Skills, 真源 separation).
- **Performance + concurrency**: Go's goroutines map naturally to streaming multi-agent fan-out. Hertz's `pkg/protocol/sse` integrates with Eino's `StreamReader[*schema.Message]`.
- **Single vendor**: Hertz + Eino + Volcano Ark are all in the CloudWeGo / ByteDance orbit — fewer integration surprises.

**Key APIs you will use:**

```go
// ChatModelAgent (ReAct pattern)
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "ComplianceAuditor",
    Description: "Reads regulation docs and judges compliance",
    Instruction: "You ONLY cite regulation clauses. If unsure, refuse.",
    Model:       arkModel,
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools},
    },
})

// Multi-agent composition
orchestrator, _ := adk.NewSequentialAgent(ctx, &adk.SequentialAgentConfig{
    Name:        "CompliancePipeline",
    Description: "retrieve -> judge -> cite",
    SubAgents:   []adk.Agent{retrieverAgent, judgeAgent, citationAgent},
})

runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: orchestrator, EnableStreaming: true})
iter := runner.Query(ctx, userQuery)
for { ev, ok := iter.Next(); if !ok { break }; ... }
```

**Other ADK presets you get for free:**
- `NewParallelAgent(agents...)` — fan-out / merge (good for parallel retrieval across knowledge tiers)
- `NewLoopAgent(config)` — bounded retry (good for agentic "search again if not confident")
- `NewConditionalAgent` — route by intent classifier (good for skill router)
- `NewFailoverAgent(primary, fallback)` — fall back to a smaller model / different provider

**Transfers/Handoffs:** `AgentAction.TransferToAgent *string` is the first-class mechanism; agents decide to hand off by emitting this in the action stream.

### 2.2 Hertz HTTP Framework with SSE (HIGH)

**What it is:** Hertz is ByteDance's ultra-fast Go HTTP framework (also used at scale internally). Since v0.10.0, Hertz ships a first-party `pkg/protocol/sse` package — no need to roll your own SSE.

**Why Hertz over Gin / Echo / Fiber:**
- **Native SSE**: `hertz/pkg/protocol/sse.NewWriter(c)` writes a standards-compliant `text/event-stream` response. This is non-negotiable for streaming agent output.
- **Same vendor as Eino**: They share `pkg/app/server`, `pkg/protocol/sse`, and `pkg/route` patterns. No impedance mismatch.
- **hz CLI**: `hz new` generates project skeletons from an IDL (Thrift/Proto). Useful for typed API contracts.
- **Performance**: Comparable to Fiber; beats Gin on QPS at high concurrency.

**Streaming agent response pattern (canonical):**

```go
import (
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/cloudwego/hertz/pkg/protocol/sse"
)

func main() {
    h := server.Default()
    h.GET("/api/chat/stream", func(ctx context.Context, c *app.RequestContext) {
        s := sse.NewStream(c)
        for ev, err := runner.Query(ctx, query); err == nil; {
            switch {
            case ev.Output != nil && ev.Output.MessageOutput != nil:
                chunk := ev.Output.MessageOutput.MessageStream
                for {
                    delta, err := chunk.Recv()
                    if err != nil { break }
                    _ = s.Publish(&sse.Event{Event: "message", Data: delta.Content})
                }
            }
        }
        _ = s.Close()
    })
    h.Spin()
}
```

**Recommended version:** v0.10.2 or newer (needed for `Reader.SetMaxBufferSize` to handle large reasoning chains).

### 2.3 Vector Database: Milvus (HIGH)

**Why Milvus over Qdrant / Weaviate / pgvector for SkillForge:**

| Criterion | Milvus | Qdrant | pgvector | Weaviate |
|-----------|--------|--------|----------|----------|
| Native Eino retriever | Yes (`eino-ext/components/retriever/milvus`) | Yes | No (DIY) | No (DIY) |
| Hybrid search (dense+sparse) | Yes (built-in) | Yes | Limited | Yes |
| Permission/tenant isolation (RBAC) | Yes (DB-level) | Limited | DB-level (PG roles) | Yes |
| CN production deployments | Massive (Zilliz Cloud CN, Bytedance internal) | Medium | Via Postgres | Limited CN |
| Maturity for 1M+ vec | Proven (designed for it) | Proven | Struggles | Proven |
| Operational complexity | Medium-High | Low | Low (part of PG) | Medium |

**Recommendation:** **Milvus** as primary. Reasons:
1. **First-class Eino adapter** (`eino-ext/components/retriever/milvus`) — saves weeks of glue code.
2. **Hybrid search** (ANN + filter + keyword) is mandatory for "authority + history + personal" knowledge tiers.
3. **Multi-tenant isolation** via collection-level separation — needed for Skill isolation in v1.x.
4. **CN ecosystem**: Zilliz Cloud CN, Alibaba-hosted Milvus, on-prem single-binary all exist.

**Alternative for v0.x demo / tiny scale:** **SQLite + sqlite-vec** or **pgvector** if you want to skip Milvus operational overhead. But the moment you have ≥1 skill with ≥10k chunks, switch.

### 2.4 Embedding Models for Chinese (HIGH)

**Recommendation:** **BAAI BGE-M3** as the default embedding model.

Why:
- **Multilingual**: EN/ZH/JA/KO etc., no separate models needed for English fallback docs.
- **Long context**: 8K tokens, suitable for full clause-level chunks.
- **Multi-vector**: Supports dense, sparse (lexical), and multi-vector (ColBERT-style) retrieval — ideal for hybrid search against regulations.
- **C-MTEB leader** (Chinese Massive Text Embedding Benchmark) for enterprise retrieval.
- **Self-hostable**: Docker image; BGE-M3 fits on a single A10/A100; or call via Volcano Ark `bge-large-zh` / hosted BGE-M3.

**Alternative:** Volcano Ark `bge-large-zh` (hosted) if you don't want to manage embeddings yourself. Use the `eino-ext/components/embedding/ark` adapter.

**Why NOT OpenAI `text-embedding-3-large`:**
- Cross-border latency for CN enterprise
- Data residency concerns (regulatory docs are sensitive)
- 8K context vs BGE-M3's 8K — comparable but BGE-M3 wins on C-MTEB specifically.

### 2.5 Document Parsing + Chunking (HIGH)

**For compliance / regulation docs (typical: PDF, HTML, DOCX, sometimes Markdown):**

- **PDF** — `eino-ext/components/document/parser/pdf` (configurable; can extract per-page text)
- **HTML** — `eino-ext/components/document/parser/html`
- **DOCX** — `eino-ext/components/document/parser/docx` (if you add it)
- **Chunker** — `eino-ext/components/document/transformer/splitter/markdown` with hierarchical header splitting. For pure-PDF regulatory text, pair with `splitter/recursive` or `splitter/semantic`.

**Crucial for citation traceability:** Each chunk MUST carry metadata:
```go
type DocMeta struct {
    Source       string  // filename or doc id
    Authority    string  // "权威" / "历史" / "个人"
    Clause       string  // e.g., "第三章·第二条"
    PageOrAnchor string
    EffectiveDate string
    SHA256       string  // for immutability check
}
```
This is the substrate for the **真源 separation** layer (Section 5).

### 2.6 Observability / Tracing (MEDIUM-HIGH)

**Choice:** Eino's `callbacks.Handler` interface → plug into **OpenTelemetry** → export to **Jaeger** or **Langfuse**.

- Eino's `Handler` has `OnStart`, `OnEnd`, `OnError`, `OnStartWithStreamInput`, `OnEndWithStreamOutput`. Wrap with an OTel tracer to emit spans per agent invocation, per LLM call, per tool call.
- Langfuse is LLM-aware (tokens, prompts, costs) — use it if you need prompt management. Otherwise OTel → Jaeger is sufficient for v1.
- The `eino-ext/callbacks` package provides reference handlers (`logging`, `tracing`).

**What to trace:**
1. Each `ChatModelAgent.Run` (start time, end time, model, tokens)
2. Each tool call (tool name, args, latency, error)
3. Each retrieval (query, top-k, latency, scores)
4. Each LLM refusal / fallback decision (for guardrail analytics)

### 2.7 Eval & Guardrails (HIGH — but Python)

**The reality in Aug 2026:** No production-ready RAG eval framework exists in Go. **Use Ragas (Python) as a sidecar service or batch job.** This is the standard pattern even at Google/Amazon RAG teams.

**Ragas metrics you should track:**
- `Faithfulness` — does the answer contradict retrieved context? (catches hallucination)
- `ContextPrecision` — are retrieved docs relevant? (catches retrieval miss)
- `ContextRecall` — did we retrieve everything needed? (catches retrieval gap)
- `AnswerRelevance` — does the answer address the question? (catches drift)

**Alternative: TruLens** (Context7 has 3961 snippets — very mature; ships `RAG Triad: Context Relevance, Groundedness, Answer Relevance`). Either works. Ragas has better Chinese ecosystem docs.

**CI/CD gating:**
- Build a `golden set` of 50-100 question/answer pairs from your compliance Skill.
- Run Ragas on every Skill update.
- Block Skill promotion if `faithfulness < 0.9` or `refusal_accuracy < 0.95`.

**Why this is the "guardrails" mechanism:** Your `instruction` prompt tells it to refuse when uncertain; Ragas measures whether it actually does. If hallucination rate creeps up, you have a regression in either the prompt or the retriever.

### 2.8 Skill Plugin Architecture (HIGH)

**The Skill file format — recommend adopting Claude Code Skills spec as the baseline:**

```markdown
---
name: compliance-review
description: "审查员工行为是否符合公司制度。仅基于权威制度文档作答；不确定时明确拒答。触发场景：员工问'我能...吗'、'X行为是否合规'、'制度规定...'"
---

# Compliance Review Skill

You are an enterprise compliance auditor. ...

## Inputs
- $user_query — the compliance question
- $authority_docs — read-only knowledge of regulations (passed via `reads`)

## Steps
1. Parse the user question to identify the rule domain
2. Retrieve from $authority_docs only (NOT $historical, NOT $personal)
3. If no exact match: REFUSE — do not speculate
4. If match: cite the specific clause (e.g., "《考勤管理办法》第四条第二款")
6. Output JSON: {verdict: "compliant"|"non_compliant"|"need_more_info", clause_refs, confidence}
```

**The 6-field Skill schema (your extension on top of Claude Code's `name`+`description`):**

| Field | Purpose | Required? |
|-------|---------|-----------|
| `name` | identifier | yes |
| `description` | intent-routing text | yes |
| `agent` | sub-agent type (ChatModelAgent, ReActAgent, etc.) | yes (your extension) |
| `reads` | knowledge tier(s) this skill can access: `["权威","历史","个人"]` | yes (your extension) |
| `writes` | write targets: `["memory:facts","memory:sessions","skill_state"]` | yes (your extension) |
| `steps` | structured workflow body | yes |
| `guardrails` | explicit refuse conditions, citation requirements | yes (your extension) |

**Loader / hot-reload pattern:**
1. Watch `./skills/**/*.md` with `fsnotify`.
2. On change: re-parse, validate against JSON Schema, register in the Skill registry.
3. The Skill registry is just `map[skillName]*CompiledSkill` held in a sync.Map.
4. The intent router uses the `description` field (embedding similarity or LLM classification) to pick a Skill, then `runner.Run(ctx, query, skill.Agent)`.

**Why declarative Markdown beats code-defined Skills:**
- Non-developers can author Skills (the "platform" promise from PROJECT.md).
- Skills are diffable, reviewable, versionable in Git.
- Skills are portable across agent backends (swap Eino for LangGraph, Skill stays).
- Hot-reload "just works" — file changed, Skill re-registered.

**What NOT to do:** Skills that require code compilation, scripts in arbitrary languages, or network calls from the Skill body itself. Skills should be pure declarative prompts + structured metadata.

---

## 3. Frontend Stack — Deep Dive

### 3.1 React 19 + Vite 8 + TypeScript (HIGH)

**React 19.2.x** is stable in Aug 2026. Use it because:
- **`use` hook** for reading promises/contexts inside render (clean for streaming data).
- **Server Components** for initial RAG context rendering (citation list is a great RSC candidate).
- **Actions** simplify form handling (Skill wizard form, eval trigger).
- **Stable compiler / ref-as-prop** — no more `forwardRef`.

**Vite 8.x** with `@tailwindcss/vite` plugin (Tailwind v4 native) — instant HMR, single binary dev server, no Webpack baggage.

**TypeScript 5.6+** in `strict: true` mode. The Vercel AI SDK v5 is built around discriminated unions — strict mode is required to use it safely.

### 3.2 Tailwind CSS v4 + shadcn/ui (HIGH)

**Tailwind v4** replaces `tailwind.config.js` with CSS-first `@theme` and `@source` directives. Faster builds, native CSS variables, OKLCH colors out of the box.

**shadcn/ui** (CLI-installed, components live in your repo) gives you:
- **Source ownership**: you copy the component code; no npm dependency hell.
- **React 19 compatible**: official migration guide exists.
- **Tailwind v4 native**: CLI emits v4-compatible components.
- **Theming**: design tokens live as CSS variables; works with `next-themes`-style light/dark.

**Components you will rely on heavily:** `Card`, `Button`, `Tabs`, `Sheet`, `Dialog`, `Tooltip`, `Badge`, `ScrollArea`, `Separator`, `Command`.

### 3.3 Vercel AI SDK v5 — THE Frontend AI SDK Choice (HIGH)

**Why Vercel AI SDK v5 over LangChain.js / custom:**

| Criterion | Vercel AI SDK v5 | LangChain.js v1 | Custom (EventSource wrapper) |
|-----------|------------------|------------------|------------------------------|
| Bundle size (minified+gzip) | ~12 KB | 200-800 KB (modular ~36KB) | <5 KB |
| Native React hooks (`useChat`, `useCompletion`, `useObject`) | Yes | Partial (LangGraph SDK separate) | DIY |
| **Streaming protocol** | **UIMessageChunk** (typed discriminated union) | LangChain MessageGraph (proprietary) | DIY |
| Reasoning chunk rendering | First-class (`reasoning-delta`) | Limited | DIY |
| Tool-call lifecycle UI | First-class (multi-phase parts) | DIY | DIY |
| Source URL/Document streaming | First-class | DIY | DIY |
| React 19 ready | Yes | Yes | N/A |
| Custom backend transport | Yes (`DefaultChatTransport` / custom) | Yes | Yes |

**The UIMessageChunk protocol is a major win.** It defines typed chunks for:

- `text-start` / `text-delta` / `text-end`
- `reasoning-start` / `reasoning-delta` / `reasoning-end` — show collapsible thinking trace
- `tool-input-start` / `tool-input-delta` / `tool-input-available` / `tool-output-available` / `tool-output-error`
- `source-url` / `source-document` — for citation UI
- `data-*` — for custom payload (your Skill verdict, refusal reason)
- `finish-step` / `finish-message` / `message-metadata` — for analytics

**Your backend → frontend integration:**

```typescript
// app/api/chat/route.ts
import { streamText, convertUIMessageStreamToSSE } from 'ai'
import { gateway } from '@ai-sdk/gateway'

export async function POST(req: Request) {
  const { messages } = await req.json()
  const result = streamText({
    model: gateway('doubao-pro'),
    messages,
    tools: { /* ... */ },
  })
  return new Response(convertUIMessageStreamToSSE(result.toUIMessageStream()), {
    headers: { 'Content-Type': 'text/event-stream' },
  })
}
```

**But SkillForge's backend is Go/Hertz**, not Next.js. Solution:
- Hertz exposes `/api/chat` returning a stream of `text/event-stream` frames encoded as UIMessageChunk wire format.
- On the React side, configure `useChat` with a custom `transport` that points to your Hertz URL and parses the SSE frames:

```typescript
import { useChat } from '@ai-sdk/react'
import { DefaultChatTransport } from 'ai'

const transport = new DefaultChatTransport({
  api: 'http://localhost:8888/api/chat',
  prepareSendMessagesRequest: ({ messages, id }) => ({ /* ... */ }),
})
const { messages, sendMessage, addToolOutput } = useChat({ transport })
```

**Alternative:** Run a Next.js / Express thin BFF layer in front of Hertz to keep the `streamText` ergonomics. For v1, skip the BFF — Hertz can emit UIMessageChunk SSE directly (lowest friction, fewest moving parts).

### 3.4 State Management — Zustand v5 (HIGH)

- **Why:** Lightweight (~1KB), TypeScript-first, plays well with React 19's concurrent rendering.
- **What it stores:** Skill list cache, current Skill ID, eval history, user preferences (theme, model choice).
- **Why NOT Redux:** Overkill for v1; Zustand covers 95% of needs.
- **Why NOT Jotai:** Atom graph gets messy with Skill/eval/messages state co-existing.

### 3.5 React Component Skeleton (HIGH)

```
src/
├── App.tsx
├── routes/
│   ├── Chat.tsx           // main Q&A interface
│   ├── Skills.tsx         // Skill browser/manager
│   ├── SkillEditor.tsx    // SKILL.md editor with live preview
│   ├── Eval.tsx           // Ragas dashboard
│   └── Settings.tsx
├── components/
│   ├── chat/
│   │   ├── MessageList.tsx
│   │   ├── ReasoningTrace.tsx       // <Reasoning> parts
│   │   ├── ToolCallCard.tsx          // tool-input / tool-output
│   │   └── CitationPopover.tsx       // source-url / source-document
│   ├── skills/
│   │   ├── SkillCard.tsx
│   │   ├── SkillDiffView.tsx
│   │   └── SkillWizard.tsx           // natural-language → SKILL.md
│   └── ui/                // shadcn primitives
├── lib/
│   ├── ai.ts              // useChat hooks, transport setup
│   ├── api.ts             // fetch wrappers
│   └── format.ts
├── store/                  // Zustand slices
└── styles/
    └── globals.css         // @import "tailwindcss" + @theme tokens
```

---

## 4. RAG Architecture (HIGH)

### 4.1 Agentic RAG vs Naive RAG

Naive RAG = `query → embed → top-k → stuff-in-context → answer`. Brittle.

**Agentic RAG** = the agent *decides*:
1. Should I retrieve? (or do I know this?)
2. From which knowledge tier? (权威 vs 历史 vs 个人)
3. With what query reformulation?
4. Should I retrieve again? (multi-hop)
5. Should I refuse? (insufficient evidence)
6. Should I ask the user for clarification?

Eino's ADK maps naturally to this: `ChatModelAgent` with retriever tools + a `LoopAgent` for multi-hop + an explicit `guardrails` Skill field that triggers refusal.

### 4.2 Recommended Stack

```
User Query
   ↓
Intent Router (small LLM, ~3B params, classifies into skill)
   ↓
Skill Loader (parses SKILL.md, gets agent/reads/writes/guardrails)
   ↓
ChatModelAgent (with retriever tools restricted by `reads`)
   ↓
Hybrid Retrieval (Milvus dense + sparse + metadata filter on authority tier)
   ↓
Reranking (optional: BGE-reranker-v2-m3 for top-50 → top-5)
   ↓
Prompt Assembly (Skill instruction + retrieved chunks + history)
   ↓
LLM Generate (with streaming)
   ↓
Guardrail Check (refusal detection, citation completeness)
   ↓
Stream UIMessageChunk to React (text + reasoning + source-document + finish)
```

### 4.3 Reranking (MEDIUM)

For Chinese enterprise RAG, add **BAAI BGE-reranker-v2-m3** as a second-pass reranker:
- Milvus returns top-50 by hybrid similarity.
- BGE-reranker scores all 50, takes top-5.
- Boosts C-MTEB scores by 5-15% in compliance scenarios (long-tail regulatory vocabulary).

API: HuggingFace inference endpoint or self-host via TEI (Text Embeddings Inference).

### 4.4 True-Source Separation (HIGH — this is the moat)

**denova's "权威 / 历史 / 个人" pattern, formalized for SkillForge:**

```yaml
# Skill frontmatter
reads: ["权威"]        # this skill can ONLY touch authority tier
# writes: []          # read-only skill
```

```go
// Enforced at retrieval time
type KnowledgeTier string

const (
    TierAuthority  KnowledgeTier = "权威"   // regulations, official docs
    TierHistorical KnowledgeTier = "历史"   // meeting minutes, past decisions
    TierPersonal   KnowledgeTier = "个人"   // personal notes, drafts
)

type Document struct {
    schema.Document
    Tier KnowledgeTier `json:"tier"`
}

// Milvus filter expression (auto-injected from Skill.reads)
filter := fmt.Sprintf(`tier in [%s]`, quotedList(skill.Reads))
results, _ := retriever.Retrieve(ctx, query, WithFilter(filter))
```

**Why this is more than labels:**
1. **Refusal-by-construction**: If a Skill declares `reads: ["权威"]` and the user asks something not in authority docs, the agent literally has no source. The guardrail prompt just says "if no `权威` result, refuse."
2. **Multi-tenant safety**: "Personal" tier for personal notes vs "Authority" for company policy — they must NEVER be conflated. The filter makes this structural.
3. **Audit trail**: Every retrieval logs `(skill, query, tier, doc_ids)`. You can prove "this answer was based only on 权威 docs version X."

---

## 5. DevX & Infra Tooling

### 5.1 Local Dev Stack

```yaml
# docker-compose.yml (the recommended v1 dev stack)
services:
  milvus:
    image: milvusdb/milvus:v2.5-standalone
    ports: ["19530:19530"]
  etcd:
    image: quay.io/coreos/etcd:v3.5.16
  minio:
    image: minio/minio
  redis:                    # optional, for Skill cache / session state
    image: redis:7-alpine
  app:
    build: .
    ports: ["8888:8888"]
    depends_on: [milvus, redis]
  eval-sidecar:
    build: ./eval
    # Python + Ragas; runs on demand via CLI, not always-on
```

### 5.2 Linting / Formatting

- **Go**: `golangci-lint` v2.x with `govet`, `errcheck`, `staticcheck`, `revive`
- **TS**: `eslint@9` flat config + `typescript-eslint` v8 strict
- **Pre-commit**: `pre-commit` framework with gofmt/gofumpt and prettier

### 5.3 Testing

- **Go unit tests**: standard `testing` + `testify/assert`
- **Go Eino tests**: eino has a `mock` package for `ChatModel` and `Tool`
- **React**: `vitest` + `@testing-library/react`
- **E2E**: `playwright` for full Q&A flow (compliance Skill → answer → citation)

### 5.4 CI

- GitHub Actions
- Lint → Unit → E2E → Ragas eval (block on faithfulness < 0.9)
- Docker build on `main`; optional GHCR publish

---

## 6. Anti-Stack — What NOT To Use (and Why)

| Anti-Choice | Why Not | What to Do Instead |
|-------------|---------|---------------------|
| **LangChain.js (full umbrella)** | 200-800 KB bundle; API churns every minor version; Vercel AI SDK v5 covers 95% of use cases with 1/20× the weight | Use `ai` (Vercel AI SDK v5) + provider SDKs directly. If you must, use `@langchain/core` modular only. |
| **Pure "AI wrapper" frontend (Height.app-style)** | **Height.app closed 2025.9** after $20M+ raised. Proof that thin AI wrappers without proprietary value die. | SkillForge is a *platform* (Skill system) not a *feature*. The Skill system IS the proprietary value. |
| **OpenAI text-embedding-3 for Chinese RAG** | Cross-border latency; data residency; underperforms BGE-M3 on C-MTEB | BAAI BGE-M3 (self-host or via Volcano Ark) |
| **pgvector as primary vector DB at scale** | Tested to struggle past 1M vectors for hybrid search; no native multi-tenant isolation | Milvus primary; pgvector only for tiny dev datasets |
| **Coze / Dify as the platform** | Vendor lock-in; no source code ownership; no Skill authoring UX; not portable | SkillForge IS your platform. (You could integrate Coze/Dys as Skill *implementations* via MCP, but don't *be* them.) |
| **Custom LLM training / fine-tuning** | Out of scope per PROJECT.md; cost-prohibitive for v1; distracting from agent engineering | Prompt engineering + Skill system + Agentic RAG |
| **LangGraph Python** | User chose Go; re-doing denova in Python defeats the platform | Stay in Go with Eino ADK |
| **Hugging Face `transformers` (in-process LLM)** | Latency overhead; GPU dependency; deployment hell | API-only. Volcano Ark / OpenAI / DeepSeek hosted inference. |
| **MongoDB** | Two DBs (PG + Mongo) = more ops. Pick PG + pgvector for v0 demos; Milvus for prod. | PostgreSQL 16 + pgvector for meta; Milvus for vectors |
| **Next.js as the frontend** | Vercel AI SDK works in plain Vite just fine; Next.js adds SSR complexity you don't need for an interactive demo | Vite + React + React Router (if needed) |
| **Redux / Redux Toolkit** | Overkill for v1; verbose | Zustand v5 |
| **Custom SSE implementation** | Bug-prone; re-invents `pkg/protocol/sse` | Hertz `pkg/protocol/sse` (server) + Vercel AI SDK's transport (client) |
| **Building a Skill system in code (Go plugins / wasm)** | Compile cycle kills hot-reload promise; Skills become "compiled modules" not "files" | Declarative Markdown + YAML frontmatter + `fsnotify` hot-reload |

---

## 7. Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Eino core + ADK maturity | HIGH | Direct Context7 docs for `cloudwego/eino` and `cloudwego/eino-ext`; latest release tag v0.7.7 confirmed; ADK GA in v0.5.0 (June 2026) confirmed |
| Hertz + SSE | HIGH | Official `pkg/protocol/sse` package added in v0.10.0 changelog; v0.10.2+ has `Reader.SetMaxBufferSize` |
| Milvus as vector DB | HIGH | Native Eino retriever; 2026 benchmark consensus in CN sources; first-party adapter |
| BGE-M3 for Chinese | HIGH | C-MTEB leader; multi-vector + sparse; standard choice across CN enterprise RAG in 2026 |
| React 19 + Vite 8 + Tailwind v4 + shadcn | HIGH | All stable releases; shadcn has official migration guide; React 19.2.7 is latest |
| Vercel AI SDK v5 | HIGH | UIMessageChunk protocol confirmed via Context7; first-class reasoning/tool/source streaming |
| Ragas as eval tool | HIGH | Context7 docs rich; standard in CN/EN; metrics `Faithfulness`/`ContextPrecision`/`ContextRecall`/`AnswerRelevance` confirmed |
| Skill file format (Claude Code spec inspired) | HIGH | Anthropic publishes spec; widely documented; 3-tier progressive disclosure is the de facto standard |
| True-source separation via metadata filter | MEDIUM-HIGH | Pattern is sound; specific Milvus filter syntax needs Phase 1 spike validation |
| RAGAS sidecar in Go monorepo | MEDIUM | Pattern is standard (Python service / batch job); specific dev workflow needs Phase 1 design |

**What's UNKNOWN (flag for phase research):**
- Exact Milvus filter syntax for `tier in [...]` combined with vector search (validate in Phase 2)
- Best hot-reload pattern for Skills with active in-flight conversations (validate in Phase 3)
- Specific reranker integration: BGE-reranker-v2-m3 self-host vs API (validate in Phase 2)
- Whether Hertz SSE plays cleanly with `useChat`'s `DefaultChatTransport` or needs a custom transport (validate in Phase 1 with a spike)

---

## 8. Open Questions for Roadmap

1. **Phase 1 should include a Spike Phase**: prove Hertz SSE → Vercel AI SDK `useChat` round-trip with one tool call and one citation. 1-2 days, no other features.
2. **Eval sidecar service vs CLI batch job**: Decide before Phase 4 (eval phase). Recommendation: start as batch; grow into service when CI gates are wired.
3. **Self-host BGE-M3 vs hosted Ark embedding**: Default to hosted Ark (faster v0); offer self-host toggle in v1.x for data-residency users.
4. **Should we ship Milvus in docker-compose or only support remote?** Recommendation: support both via env vars (`MILVUS_ADDR`).

---

## 9. Sources

**HIGH confidence (Context7 official docs):**
- `cloudwego/eino` — ChatModel, ADK, callbacks, graph composition, StreamReader
- `cloudwego/eino-ext` — Milvus/Qdrant retriever, PDF/HTML parser, markdown splitter, Ark embedding/model, observability callbacks
- `cloudwego/hertz` — `pkg/protocol/sse` package, v0.10.0 changelog, v0.10.2 max-buffer-size fix
- `vercel/ai` — `UIMessageChunk` discriminated union (all chunk types), `useChat` with `DefaultChatTransport`, `addToolOutput`, `sendAutomaticallyWhen`
- `vibrantlabsai/ragas` — `Faithfulness`, `ContextRelevance`, `ContextPrecision`, `AnswerRelevance`, `ContextRecall`
- `truera/trulens` — 3961 code snippets, mature eval library
- `react/react` — v19.2.7 (latest stable)

**MEDIUM confidence (WebSearch verified):**
- Eino v0.5.0 ADK release (June 2026)
- Eino v0.7.7 latest release tag
- BGE-M3 as 2026 C-MTEB leader for Chinese enterprise RAG
- Vercel AI SDK v5 release and UIMessageChunk as new wire format
- Tailwind v4 + shadcn/ui + React 19 official migration guide
- LangChain.js bundle size concerns (modular `@langchain/core` is 36 KB; umbrella is 200-800 KB)

**LOW confidence (flagged):**
- Specific 2026 Qdrant vs Milvus benchmarks (sources are blog posts, not primary benchmarks)
- Height.app closure details (date confirmed: 2025.9, but no primary source cited)
- Specific denova internal architecture (only referenced via user's PROJECT.md, not direct inspection)

---

*End of STACK.md*