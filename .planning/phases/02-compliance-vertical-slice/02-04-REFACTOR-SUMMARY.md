# Plan 02-04 — Multi-Backend Interface Refactor (Extension)

**Date:** 2026-08-11
**Status:** Complete — behavior unchanged, platform now multi-backend-ready
**Trigger:** User requirement — "向量数据库和 BGE 需要支持多种类型（Milvus / MongoDB / …，BGE / OpenAI / …）"

## What Changed

The retriever and embedder layers were refactored from concrete (Milvus-only, BGE-only) packages into **interface + implementation** packages. Phase 2's behavior is 100% unchanged; the platform is now multi-backend-ready (a future MongoDB retriever or OpenAI embedder is a new package implementing an interface, not a search-and-replace).

## New Package Layout

| Package | Role |
|---------|------|
| `internal/retriever` | **Abstraction** — `Retriever` interface, `Hit`, `Document` types |
| `internal/retriever/milvus` | **Milvus impl** — `*MilvusClient` implements `retriever.Retriever`; `BuildMilvusFilter` stays here (Milvus-specific CN-char escaping) |
| `internal/embedder` | **Abstraction** — `Embedder` interface |
| `internal/embedder/bge` | **BGE-M3 impl** — `*BGE` implements `embedder.Embedder`; `EmbedQueryDeterministic` stays as a package-level offline fallback (NOT used by production) |

## Interface Contracts

```go
// internal/retriever
type Retriever interface {
    Health(ctx context.Context) error
    Search(ctx context.Context, queryVec []float32, tiers []string, topK int) ([]Hit, error)
    Ensure(ctx context.Context) error
    Insert(ctx context.Context, docs []Document) error
    Disconnect() error
}

// internal/embedder
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    Dimensions() int
}
```

**Tier-filter semantics preserved:** PRE-similarity at the SDK/Search layer (SYSK-07, P-RAG-2). Empty/zero tiers → 0 hits (refusal-by-construction, D-09). This is the core trust boundary and is enforced inside each implementation.

## Consumer Refactors

| Consumer | Change |
|----------|--------|
| `internal/server/server.go` `*Deps` | Fields now `embedder.Embedder` + `retriever.Retriever` (was concrete types) |
| `internal/server/pipeline.go` | Calls `Deps.Embedder.Embed` / `Deps.Retriever.Search` — pipeline is 100% backend-agnostic (no `internal/retriever/milvus` import) |
| `internal/knowledge/indexer.go` | Signature takes `retriever.Retriever` + `embedder.Embedder` (was `client.Client` + `MilvusConfig` + `BGEURL`) |
| `cmd/server/main.go` | Constructs `milvus.New(...)` + `bge.New(...)` and injects into `Deps` |
| `cmd/spiked/{milvus,milvus_test}.go` | Updated to new package paths |
| `integration/contract/contract_test.go` | Updated imports; SC-5 wire contract unchanged |
| `internal/server/{chat,chat_test,testhelpers_test}.go` | Mocks implement the new interfaces |

## What Did NOT Change

- SC-1..SC-6 behavior (live-verify.sh works unmodified against Milvus + BGE)
- The verdict pipeline logic (retrieval → LLM → grounding → SSE)
- The D2-01 grounding checker, D2-06 `[]Citation`, refusal-by-construction
- `cmd/spiked` regression subcommands
- The contract test (SC-5 wire format pinned)

## Multi-Backend Path Forward

To add a new backend (e.g., MongoDB Atlas Vector Search):
1. Create `internal/retriever/mongodb/` with a `*MongoClient` implementing `retriever.Retriever`
2. Implement tier-filter PRE-similarity (MongoDB's `$vectorSearch` + `$match` compound — the design work)
3. Add a runtime config switch in `cmd/server/main.go` (e.g., `RETRIEVER_BACKEND=milvus|mongodb`)
4. No changes to pipeline, indexer, server, tests — they consume the interface

Same pattern for a new embedder (e.g., OpenAI): `internal/embedder/openai/` implementing `embedder.Embedder`.

## Verification

- `go build ./...` — exit 0
- `go test ./internal/... ./cmd/...` — all green (knowledge, server, skill, verdict, spiked)
- `go test -tags=contract ./integration/contract/...` — green (SC-5 contract pinned, 19.6s)
- 10 atomic commits (reviewable history)

## Commits

1. `feat(retriever): add Retriever interface + Hit/Document types`
2. `feat(embedder): add Embedder interface`
3. `feat(retriever/milvus): migrate spike assets to milvus subpackage, implement Retriever`
4. `feat(embedder/bge): migrate EmbedQuery to bge subpackage, implement Embedder`
5. `refactor(knowledge): indexer takes retriever + embedder interfaces`
6. `refactor(server): Deps + pipeline consume retriever/embedder interfaces`
7. `refactor(cmd/server): main wires Milvus + BGE concrete impls into Deps`
8. `chore(cmd/spiked): update CLI subcommands to new package paths`
9. `test(contract): update integration contract for new package paths`
10. `refactor(retriever): remove old top-level impl (migrated to milvus subpackage)`

---
*Refactor added to Phase 2 per user direction (multi-backend support). Live 6-SC verification (02-04 Task 2) still pending human checkpoint.*
