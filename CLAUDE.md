<!-- GSD:project-start source:PROJECT.md -->
## Project

**SkillForge**

SkillForge 是一个面向企业的、可交互的 Web 应用，展示大模型 agent 工程能力。技术栈为 **Go + Eino（字节 CloudWeGo 开源的 LLM 框架）+ Eino ADK 多 agent 编排**（后端），**React 19 + Vite + TypeScript**（前端）。核心创新是"**Skill 驱动**"：企业通过编写或更新 Skill 文件（`SKILL.md`）让同一个 agent 内核适配任何业务领域（法律审查、HR 制度、IT 服务台、文档问答等），无需改代码。仓库在 `examples/skills/` 下提供三个开箱即用的示例 Skill（simple-qa、calculator、compliance），证明同一内核不改代码即可切换业务领域。开源发布，用于建个人技术品牌。

**Core Value:** **同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码** —— 这是从"应用"到"平台"的护城河，也是"通过更新 Skill 让项目更理解企业使用需求"这一设计哲学的直接落地。

### Constraints

- **Tech stack**：Go + Eino + Eino ADK + Hertz（后端，用户明确选择，对标 denova 路线）；React 19 + Vite + TypeScript + Vercel AI SDK + shadcn/Tailwind（前端）
- **License**：Apache-2.0（仿 denova，开源友好）
- **Deployment**：v1 本地启动 + Docker Compose；聚焦 Web 应用形态，非桌面级
- **Open source goal**：打磨文档 + 易上手 + 可衡量效果（用于个人品牌，而非纯面试展示）
- **Referential constraint**：denova 源码可作为实现参照（用户已 fork），但**不复制 denova 的内容生成方向**（小说 / RPG），仅借鉴 agent 工程架构（Skills / 真源分离 / Eino ADK）
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## 1. TL;DR — The Stack in One Table
| Layer | Choice | Version | Confidence | Why |
|-------|--------|---------|------------|-----|
| **Language (BE)** | Go | 1.22+ (preferably 1.23) | HIGH | User decision; matches denova; mature Eino support |
| **HTTP (BE)** | CloudWeGo Hertz | v0.10.2+ | HIGH | First-class SSE (`pkg/protocol/sse`); same vendor as Eino; byte-dance prod-proven |
| **Agent framework** | cloudwego/eino | v0.9.14（locked 2026-08-17；v0.10.0-alpha.x 明确不用） | HIGH | GA ADK since v0.5.0; explicit multi-agent API; callbacks for observability |
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
## 2. Backend Stack — Deep Dive
### 2.1 Go + CloudWeGo Eino (HIGH)
- Type-safe component abstractions (`ChatModel`, `Tool`, `Retriever`, `Indexer`, `Embedding`, `Document.Parser`, `Document.Transformer`)
- A graph composition engine (`compose.Graph`, `compose.Chain`)
- A multi-agent orchestration kit (`adk` package — Agent Development Kit)
- Callback handlers (`callbacks.Handler`) for cross-cutting concerns (logging, tracing, metrics)
- **v0.9.14** is the current locked release（2026-08-13 发布，升级决策 2026-08-17）on `cloudwego/eino`.
- **Eino ADK was released as GA in v0.5.0** (June 2026). It was a landmark — Go finally got a first-class multi-agent library.
- The `eino-ext` companion repo holds official adapters: chat models (openai, ark, ollama, deepseek, gemini), embeddings, retrievers (milvus, qdrant, volc_vikingdb, es), indexers, document parsers (pdf, html), splitters (markdown, recursive), tools, and observability callbacks (langfuse, coze, aliyun).
- **Go-native agent story**: Until Eino, Go LLM apps were reimplementing LangChain patterns. Eino closes that gap with a typed, channel-streaming design.
- **Denova parity**: The user's fork uses Eino + ADK. SkillForge inherits the proven mental model (SubAgents, Skills, 真源 separation).
- **Performance + concurrency**: Go's goroutines map naturally to streaming multi-agent fan-out. Hertz's `pkg/protocol/sse` integrates with Eino's `StreamReader[*schema.Message]`.
- **Single vendor**: Hertz + Eino + Volcano Ark are all in the CloudWeGo / ByteDance orbit — fewer integration surprises.
- `NewParallelAgent(agents...)` — fan-out / merge (good for parallel retrieval across knowledge tiers)
- `NewLoopAgent(config)` — bounded retry (good for agentic "search again if not confident")
- `NewConditionalAgent` — route by intent classifier (good for skill router)
- `NewFailoverAgent(primary, fallback)` — fall back to a smaller model / different provider
### 2.2 Hertz HTTP Framework with SSE (HIGH)
- **Native SSE**: `hertz/pkg/protocol/sse.NewWriter(c)` writes a standards-compliant `text/event-stream` response. This is non-negotiable for streaming agent output.
- **Same vendor as Eino**: They share `pkg/app/server`, `pkg/protocol/sse`, and `pkg/route` patterns. No impedance mismatch.
- **hz CLI**: `hz new` generates project skeletons from an IDL (Thrift/Proto). Useful for typed API contracts.
- **Performance**: Comparable to Fiber; beats Gin on QPS at high concurrency.
### 2.3 Vector Database: Milvus (HIGH)
| Criterion | Milvus | Qdrant | pgvector | Weaviate |
|-----------|--------|--------|----------|----------|
| Native Eino retriever | Yes (`eino-ext/components/retriever/milvus`) | Yes | No (DIY) | No (DIY) |
| Hybrid search (dense+sparse) | Yes (built-in) | Yes | Limited | Yes |
| Permission/tenant isolation (RBAC) | Yes (DB-level) | Limited | DB-level (PG roles) | Yes |
| CN production deployments | Massive (Zilliz Cloud CN, Bytedance internal) | Medium | Via Postgres | Limited CN |
| Maturity for 1M+ vec | Proven (designed for it) | Proven | Struggles | Proven |
| Operational complexity | Medium-High | Low | Low (part of PG) | Medium |
### 2.4 Embedding Models for Chinese (HIGH)
- **Multilingual**: EN/ZH/JA/KO etc., no separate models needed for English fallback docs.
- **Long context**: 8K tokens, suitable for full clause-level chunks.
- **Multi-vector**: Supports dense, sparse (lexical), and multi-vector (ColBERT-style) retrieval — ideal for hybrid search against regulations.
- **C-MTEB leader** (Chinese Massive Text Embedding Benchmark) for enterprise retrieval.
- **Self-hostable**: Docker image; BGE-M3 fits on a single A10/A100; or call via Volcano Ark `bge-large-zh` / hosted BGE-M3.
- Cross-border latency for CN enterprise
- Data residency concerns (regulatory docs are sensitive)
- 8K context vs BGE-M3's 8K — comparable but BGE-M3 wins on C-MTEB specifically.
### 2.5 Document Parsing + Chunking (HIGH)
- **PDF** — `eino-ext/components/document/parser/pdf` (configurable; can extract per-page text)
- **HTML** — `eino-ext/components/document/parser/html`
- **DOCX** — `eino-ext/components/document/parser/docx` (if you add it)
- **Chunker** — `eino-ext/components/document/transformer/splitter/markdown` with hierarchical header splitting. For pure-PDF regulatory text, pair with `splitter/recursive` or `splitter/semantic`.
### 2.6 Observability / Tracing (MEDIUM-HIGH)
- Eino's `Handler` has `OnStart`, `OnEnd`, `OnError`, `OnStartWithStreamInput`, `OnEndWithStreamOutput`. Wrap with an OTel tracer to emit spans per agent invocation, per LLM call, per tool call.
- Langfuse is LLM-aware (tokens, prompts, costs) — use it if you need prompt management. Otherwise OTel → Jaeger is sufficient for v1.
- The `eino-ext/callbacks` package provides reference handlers (`logging`, `tracing`).
### 2.7 Eval & Guardrails (HIGH — but Python)
- `Faithfulness` — does the answer contradict retrieved context? (catches hallucination)
- `ContextPrecision` — are retrieved docs relevant? (catches retrieval miss)
- `ContextRecall` — did we retrieve everything needed? (catches retrieval gap)
- `AnswerRelevance` — does the answer address the question? (catches drift)
- Build a `golden set` of 50-100 question/answer pairs from your compliance Skill.
- Run Ragas on every Skill update.
- Block Skill promotion if `faithfulness < 0.9` or `refusal_accuracy < 0.95`.
### 2.8 Skill Plugin Architecture (HIGH)
# Compliance Review Skill
## Inputs
- $user_query — the compliance question
- $authority_docs — read-only knowledge of regulations (passed via `reads`)
## Steps
| Field | Purpose | Required? |
|-------|---------|-----------|
| `name` | identifier | yes |
| `description` | intent-routing text | yes |
| `agent` | sub-agent type (ChatModelAgent, ReActAgent, etc.) | yes (your extension) |
| `reads` | knowledge tier(s) this skill can access: `["权威","历史","个人"]` | yes (your extension) |
| `writes` | write targets: `["memory:facts","memory:sessions","skill_state"]` | yes (your extension) |
| `steps` | structured workflow body | yes |
| `guardrails` | explicit refuse conditions, citation requirements | yes (your extension) |
- Non-developers can author Skills (the "platform" promise from PROJECT.md).
- Skills are diffable, reviewable, versionable in Git.
- Skills are portable across agent backends (swap Eino for LangGraph, Skill stays).
- Hot-reload "just works" — file changed, Skill re-registered.
## 3. Frontend Stack — Deep Dive
### 3.1 React 19 + Vite 8 + TypeScript (HIGH)
- **`use` hook** for reading promises/contexts inside render (clean for streaming data).
- **Server Components** for initial RAG context rendering (citation list is a great RSC candidate).
- **Actions** simplify form handling (Skill wizard form, eval trigger).
- **Stable compiler / ref-as-prop** — no more `forwardRef`.
### 3.2 Tailwind CSS v4 + shadcn/ui (HIGH)
- **Source ownership**: you copy the component code; no npm dependency hell.
- **React 19 compatible**: official migration guide exists.
- **Tailwind v4 native**: CLI emits v4-compatible components.
- **Theming**: design tokens live as CSS variables; works with `next-themes`-style light/dark.
### 3.3 Vercel AI SDK v5 — THE Frontend AI SDK Choice (HIGH)
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
- `text-start` / `text-delta` / `text-end`
- `reasoning-start` / `reasoning-delta` / `reasoning-end` — show collapsible thinking trace
- `tool-input-start` / `tool-input-delta` / `tool-input-available` / `tool-output-available` / `tool-output-error`
- `source-url` / `source-document` — for citation UI
- `data-*` — for custom payload (your Skill verdict, refusal reason)
- `finish-step` / `finish-message` / `message-metadata` — for analytics
- Hertz exposes `/api/chat` returning a stream of `text/event-stream` frames encoded as UIMessageChunk wire format.
- On the React side, configure `useChat` with a custom `transport` that points to your Hertz URL and parses the SSE frames:
### 3.4 State Management — Zustand v5 (HIGH)
- **Why:** Lightweight (~1KB), TypeScript-first, plays well with React 19's concurrent rendering.
- **What it stores:** Skill list cache, current Skill ID, eval history, user preferences (theme, model choice).
- **Why NOT Redux:** Overkill for v1; Zustand covers 95% of needs.
- **Why NOT Jotai:** Atom graph gets messy with Skill/eval/messages state co-existing.
### 3.5 React Component Skeleton (HIGH)
## 4. RAG Architecture (HIGH)
### 4.1 Agentic RAG vs Naive RAG
### 4.2 Recommended Stack
### 4.3 Reranking (MEDIUM)
- Milvus returns top-50 by hybrid similarity.
- BGE-reranker scores all 50, takes top-5.
- Boosts C-MTEB scores by 5-15% in compliance scenarios (long-tail regulatory vocabulary).
### 4.4 True-Source Separation (HIGH — this is the moat)
# Skill frontmatter
# writes: []          # read-only skill
## 5. DevX & Infra Tooling
### 5.1 Local Dev Stack
# docker-compose.yml (the recommended v1 dev stack)
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
- Exact Milvus filter syntax for `tier in [...]` combined with vector search (validate in Phase 2)
- Best hot-reload pattern for Skills with active in-flight conversations (validate in Phase 3)
- Specific reranker integration: BGE-reranker-v2-m3 self-host vs API (validate in Phase 2)
- Whether Hertz SSE plays cleanly with `useChat`'s `DefaultChatTransport` or needs a custom transport (validate in Phase 1 with a spike)
## 8. Open Questions for Roadmap
## 9. Sources
- `cloudwego/eino` — ChatModel, ADK, callbacks, graph composition, StreamReader
- `cloudwego/eino-ext` — Milvus/Qdrant retriever, PDF/HTML parser, markdown splitter, Ark embedding/model, observability callbacks
- `cloudwego/hertz` — `pkg/protocol/sse` package, v0.10.0 changelog, v0.10.2 max-buffer-size fix
- `vercel/ai` — `UIMessageChunk` discriminated union (all chunk types), `useChat` with `DefaultChatTransport`, `addToolOutput`, `sendAutomaticallyWhen`
- `vibrantlabsai/ragas` — `Faithfulness`, `ContextRelevance`, `ContextPrecision`, `AnswerRelevance`, `ContextRecall`
- `truera/trulens` — 3961 code snippets, mature eval library
- `react/react` — v19.2.7 (latest stable)
- Eino v0.5.0 ADK release (June 2026)
- Eino v0.7.7 latest release tag
- BGE-M3 as 2026 C-MTEB leader for Chinese enterprise RAG
- Vercel AI SDK v5 release and UIMessageChunk as new wire format
- Tailwind v4 + shadcn/ui + React 19 official migration guide
- LangChain.js bundle size concerns (modular `@langchain/core` is 36 KB; umbrella is 200-800 KB)
- Specific 2026 Qdrant vs Milvus benchmarks (sources are blog posts, not primary benchmarks)
- Height.app closure details (date confirmed: 2025.9, but no primary source cited)
- Specific denova internal architecture (only referenced via user's PROJECT.md, not direct inspection)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
