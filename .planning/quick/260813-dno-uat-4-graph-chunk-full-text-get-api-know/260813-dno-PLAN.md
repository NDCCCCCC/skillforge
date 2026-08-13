---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 260813-dno
type: execute
gap_closure: true
mode: quick
wave: 1
depends_on: []
files_modified:
  - internal/retriever/retriever.go
  - internal/retriever/milvus/client.go
  - internal/retriever/mongo/mongo.go
  - internal/retriever/mongo/mongo_test.go
  - internal/knowledge/graph.go
  - internal/knowledge/graph_test.go
  - internal/server/server.go
  - internal/server/knowledge.go
  - internal/server/knowledge_test.go
  - internal/server/testhelpers_test.go
  - internal/agent/agent_test.go
  - web/src/lib/chunk-lookup.ts
  - web/src/components/center/ChunkSourceView.tsx
  - web/src/__tests__/chunk-lookup.test.ts
  - web/src/__tests__/ChunkSourceView.test.tsx
  - .planning/phases/05-skill-platform-full-knowledge-boundaries/05-UAT.md
  - .planning/phases/05-skill-platform-full-knowledge-boundaries/05-REMAINING-WORK.md
  - .planning/STATE.md
  - .planning/quick/260813-dno-uat-4-graph-chunk-full-text-get-api-know/260813-dno-SUMMARY.md
autonomous: true
requirements: [KNOW-05, KNOW-06, CMPL-08, SC-5, SC-8]

# Quick-mode plan (no validate flag). must_haves omitted per quick-mode spec.
# Scope is LOCKED by user 2026-08-13 (full-text IN-SCOPE). Do NOT re-ask.
---

<objective>
Close UAT #4 ("Knowledge Graph chunk shows '(chunk not found in fixture)'") — Phase 5 gap-closure §3.

Add a metadata+content READ endpoint `GET /api/knowledge/chunk/:id` (verbatim text + metadata, fetched on-demand — preserves T-5-22 graph payload contract) and teach the FE to fetch it for graph-browsed (non-cited) chunks. The graph endpoint stays metadata-only; chunk text is fetched only when the operator clicks a chunk.

Purpose: The graph browser enumerates chunk IDs the fixture does not carry (graph chunks are metadata-only per T-5-22). ChunkSourceView currently renders `(chunk not found in fixture)` for every graph-browsed chunk because `lookupChunk` only resolves fixture/verdict citations. This is the last metadata-vs-content seam left half-wired in Phase 5.

Output: A working `GET /api/knowledge/chunk/:id` (milvus + mongo backends, mirroring the graph endpoint layering) + a `fetchChunkContent` FE helper + ChunkSourceView async-load on fixture-miss. UAT #4 flips to fixed.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/05-skill-platform-full-knowledge-boundaries/05-REMAINING-WORK.md
@.planning/HANDOFF.json
@./CLAUDE.md

<interfaces>
<!-- Verified against source 2026-08-13. Executor should use these directly. -->

## Backend seams (existing — extend, do not redesign)

### internal/retriever/retriever.go (Retriever interface — EXTEND)
```go
// Existing methods (do NOT change): Health, Search, Ensure, Insert, ListMetadata, Disconnect.
// ADD: GetChunk(ctx, id int64) (ChunkContent, error)

type Hit struct {              // EXISTING — do not reuse for GetChunk (carries meaningless Score)
    ID int64; Score float32; Tier, Content, Source string; EffectiveFrom, EffectiveTo int64
}
type Document struct {        // EXISTING — do not reuse (carries Embedding)
    ID int64; Tier, Content, Source string; Embedding []float32; EffectiveFrom, EffectiveTo int64
}
type DocumentMeta struct {    // EXISTING — metadata-only projection (ListMetadata return)
    ID int64; Tier, Source string; EffectiveFrom, EffectiveTo int64
}
// ADD: ChunkContent — lean (no Score, no Embedding). Fields mirror DocumentMeta + Content.
```

### internal/retriever/milvus/client.go (MilvusClient — EXTEND)
```go
type MilvusClient struct { mc client.Client; cfg Config }   // EXISTING

// EXISTING pattern to mirror EXACTLY for GetChunk — Query-by-expression (NOT vector Search):
func (m *MilvusClient) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
    rs, err := m.mc.Query(ctx, m.cfg.CollectionName, nil, "",   // "" = match-all expr
        []string{"id","tier","source","effective_from","effective_to"})
    // ... column-by-Name() loop, GetAsInt64/GetAsString per row ...
}
// For GetChunk: m.mc.Query(ctx, coll, nil, fmt.Sprintf("id == %d", id),
//   []string{"id","tier","content","source","effective_from","effective_to"})
// Empty/nil idCol OR idCol.Len()==0 → retriever.ErrChunkNotFound.
// NO `var _ retriever.Retriever = (*MilvusClient)(nil)` in this file today — ADD one as
// defense-in-depth while you're here (the other backends all have it).
```

### internal/retriever/mongo/mongo.go (Client — EXTEND)
```go
type Client struct { cfg Config; conn *mongo.Client; coll *mongo.Collection; finder rowFinder }
type rowFinder interface { FindRows(ctx context.Context, filter any) ([]docRow, error) } // EXISTING testable seam

// EXISTING pattern to mirror EXACTLY for GetChunk:
func (c *Client) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
    rows, err := c.finder.FindRows(ctx, bson.D{})   // match-all
    // map rows → DocumentMeta
}

// docRow (mongo/rank.go) carries ALL fields incl Content:
type docRow struct {
    ID int64 `bson:"id"`; Tier, Content, Source string; Embedding []float32
    EffectiveFrom, EffectiveTo int64
}
// For GetChunk: c.finder.FindRows(ctx, bson.D{{Key:"id", Value: id}})
// len(rows)==0 → retriever.ErrChunkNotFound. rows[0] → ChunkContent.
```

### internal/knowledge/graph.go (GraphBuilder — EXTEND; helpers live HERE)
```go
type GraphBuilder struct { retriever retriever.Retriever }   // EXISTING
var ErrNoRetriever = errors.New("knowledge.GraphBuilder: retriever is nil")   // EXISTING — reuse for GetChunk

// EXISTING helpers to reuse (do not duplicate):
func int64ToChunkID(id int64) string            // "1" for 1
func versionFromEffective(from, to int64) string // "自 YYYY-MM-DD" / "起失效" / "" etc.

// EXISTING wire types: Graph, TierNode, DocNode, ChunkNode (metadata-only, T-5-22).
// ADD: ChunkPayload (JSON-tagged for FE — id/tier/source/content/effectiveFrom/effectiveTo/version)
// ADD: func (b *GraphBuilder) GetChunk(ctx, id int64) (*ChunkPayload, error)
// ADD: var ErrChunkNotFound = errors.New("knowledge: chunk not found")
```

### internal/server/server.go (Deps + KnowledgeBuilder + New — EXTEND)
```go
type KnowledgeBuilder interface {   // EXISTING — extend
    Build(ctx context.Context) (*knowledge.Graph, error)
    // ADD: GetChunk(ctx context.Context, id int64) (*knowledge.ChunkPayload, error)
}
func New(addr string, deps *Deps) *server.Hertz {   // EXISTING — add ONE route line
    // ... h.GET("/api/knowledge/graph", deps.graphHandler) ...
    // ADD: h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)
}
```

### internal/server/knowledge.go (graphHandler pattern — MIRROR for chunkHandler)
```go
// EXISTING envelope — REUSE for chunkHandler 4xx/5xx:
type graphResponseError struct { Error string `json:"error"` }

// EXISTING posture to mirror:
//   nil Knowledge  → 503 {error:"knowledge graph not initialized..."}
//   err from Build → log [WARN] + 500 {error:"knowledge graph build failed — see server logs"}  (W-3: NO err.Error())
//   ok             → 200 + JSON
// For chunkHandler: nil→503; bad-id→400 {error:"invalid chunk id"}; not-found→404 {error:"chunk not found"};
//   other-err→log[WARN]+500 generic (W-3); ok→200 ChunkPayload.
```

### internal/server/knowledge_test.go (TEST PATTERN — MIRROR)
```go
// EXISTING: stubKnowledgeBuilder{graph, err, calls} + invokeGraphHandler helper + ut.PerformRequest
// ADD to stub: chunkPayload *knowledge.ChunkPayload; chunkErr error; getChunkCalls int
// ADD method: func (s *stubKnowledgeBuilder) GetChunk(_ context.Context, _ int64) (*knowledge.ChunkPayload, error)
// ADD helper: invokeChunkHandler (registers GET /api/knowledge/chunk/:id + PerformRequest)
// ADD tests: nil→503, bad-id(="abc")→400, found→200+JSON, not-found→404, route-registered≠404
```

## Compile-break sites (the interface-extension safety net)

Adding `GetChunk` to `retriever.Retriever` WILL break the build at every conformance. Verified list:
1. `internal/retriever/mongo/mongo.go:119` — production `*Client` (var _ assertion)
2. `internal/retriever/milvus/client.go` — production `*MilvusClient` (no var _ today; break surfaces at cmd/server use site — ADD var _)
3. `internal/knowledge/graph_test.go:53` — test stub `*stubMetaRetriever`
4. `internal/server/testhelpers_test.go:95` — test stub `*stubRetriever`
5. `internal/agent/agent_test.go:93` — test stub `*stubRetriever`
Adding `GetChunk` to `KnowledgeBuilder` breaks:
6. `internal/server/knowledge.go:100` — production `*knowledge.GraphBuilder` (var _)
7. `internal/server/knowledge_test.go:145` — test stub `*stubKnowledgeBuilder`
ALL 7 must be resolved in Task 1 before `go build ./...` will pass. Test stubs (3-5,7) can return `retriever.ErrChunkNotFound` / `knowledge.ErrChunkNotFound` trivially — they are not exercising GetChunk.

## Frontend seams (existing — extend, do not redesign)

### web/src/lib/chunk-lookup.ts (EXTEND; keep fixture path UNCHANGED)
```typescript
// EXISTING — KEEP UNCHANGED (citation path):
const NOT_FOUND_TEXT = '(chunk not found in fixture)';   // → EXPORT this const
export function lookupChunk(chunkId: string): ChunkLookupResult   // sync fixture read
export function extractChunkMetadata(chunkId: string): ChunkMetadata
// ADD: async fetchChunkContent(chunkId): Promise<{text: string; tier: Tier | null} | null>
//   GET /api/knowledge/chunk/:id — 200 → payload; 404 → null; other !ok → throw.
```

### web/src/components/center/ChunkSourceView.tsx (EXTEND)
```tsx
// EXISTING props (metadata comes from PROPS, not lookup):
interface ChunkSourceViewProps { chunkId: string; tier: Tier; version: string|null;
  effectiveDate: string|null; effectiveTo: number|null }

// EXISTING body: const result = useMemo(() => lookupChunk(chunkId), [chunkId]); body = result.text
// BUG to fix: data-tier={result.tier} → should be data-tier={tier} (the PROP).
//   For graph chunks not in fixture, result.tier falls back to '权威' regardless of the actual tier.

// ADD: when result.text === NOT_FOUND_TEXT (fixture miss — graph-browsed chunk),
//   useEffect[chunkId] → fetchChunkContent(chunkId) → render 加载中… / 加载片段失败 / fetched text.
```

### web/src/hooks/useKnowledgeGraph.ts (FETCH PATTERN — MIRROR, do not import)
```typescript
// EXISTING convention to mirror in fetchChunkContent:
fetch('/api/knowledge/graph', { headers: { Accept: 'application/json' } })
  .then(async (resp) => {
    if (resp.status === 503) { /* body.error fallback */ }
    if (!resp.ok) { /* HTTP status + body.error */ }
    return resp.json();
  })
// chunk-lookup's fetchChunkContent: 404 → null (distinct from graph's 503 handling), other !ok → throw.
```

### web/src/components/graph/GraphChunkNode.tsx (NO CHANGE — confirmed)
On click → setCenterView({kind:'chunk-source', chunkId, tier, version, effectiveDate, effectiveTo}).
ALL metadata IS passed as props already — ChunkSourceView receives it; only the BODY text needs the new fetch.

## Test patterns to mirror
- `web/src/__tests__/ChunkSourceView.test.tsx` — RTL + vitest; `render()` + `screen.getByTestId(...)`.
  NOTE: existing test at L92-105 asserts `(chunk not found in fixture)` for missing chunkId — MUST update
  to mock fetch + assert the fetched verbatim text renders instead (behavior change).
- `internal/retriever/mongo/mongo_test.go` — `fakeFinder{rows, err}` + `newClientWithFake(f)` seam.
- `internal/knowledge/graph_test.go` — `stubMetaRetriever{rows, err}` + full-interface conformance.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: BE — add GET /api/knowledge/chunk/:id (retriever → knowledge → server) + tests</name>
  <files>
internal/retriever/retriever.go,
internal/retriever/milvus/client.go,
internal/retriever/mongo/mongo.go,
internal/retriever/mongo/mongo_test.go,
internal/knowledge/graph.go,
internal/knowledge/graph_test.go,
internal/server/server.go,
internal/server/knowledge.go,
internal/server/knowledge_test.go,
internal/server/testhelpers_test.go,
internal/agent/agent_test.go
  </files>
  <action>
Implement the full backend slice for `GET /api/knowledge/chunk/:id` mirroring the graph endpoint's 3-layer structure (retriever → knowledge → server). Work bottom-up so each layer compiles before the next depends on it.

**Layer 1 — `internal/retriever/retriever.go`:**
- Add `"errors"` to the import block (currently only `"context"`).
- Add the lean wire type `ChunkContent` (do NOT reuse `Hit` — it carries a meaningless Score — or `Document` — it carries Embedding). Fields: `ID int64; Tier string; Source string; Content string; EffectiveFrom int64; EffectiveTo int64`. Add a doc comment explaining it is the by-id projection used by the on-demand chunk-content fetch (T-5-22 — graph payload stays metadata-only; this is the separate fetch path).
- Add `var ErrChunkNotFound = errors.New("retriever: chunk not found by id")`.
- Add `GetChunk(ctx context.Context, id int64) (ChunkContent, error)` to the `Retriever` interface. Doc-comment the contract: by-PK lookup, content included, `ErrChunkNotFound` when the id does not resolve.

**Layer 2 — milvus impl (`internal/retriever/milvus/client.go`):**
- Add `func (m *MilvusClient) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error)`.
- Mirror `ListMetadata` exactly: `rs, err := m.mc.Query(ctx, m.cfg.CollectionName, nil, fmt.Sprintf("id == %d", id), []string{"id","tier","content","source","effective_from","effective_to"})`. (id is int64 → `%d` is injection-safe; do NOT string-concat the raw URL param.) Wrap err as `milvus.GetChunk: Query: %w`.
- Column-by-`Name()` loop like ListMetadata, but also pull `contentCol`. If `idCol == nil` OR `idCol.Len() == 0` → `return retriever.ChunkContent{}, retriever.ErrChunkNotFound`. Read row 0 into `retriever.ChunkContent` and return.
- Add `var _ retriever.Retriever = (*MilvusClient)(nil)` near the type decl (defense-in-depth — the other backends all have it; this keeps future Retriever drift surfacing here, not at a distant call site).
- milvus is runtime-broken per D11 — this only needs to COMPILE and follow the same SDK `Query` shape as ListMetadata. Do not attempt to live-test it.

**Layer 2 — mongo impl (`internal/retriever/mongo/mongo.go`):**
- Add `func (c *Client) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error)` next to `ListMetadata`.
- `rows, err := c.finder.FindRows(ctx, bson.D{{Key: "id", Value: id}})`. On err → wrap as `mongo.GetChunk: FindRows: %w`.
- `if len(rows) == 0 { return retriever.ChunkContent{}, retriever.ErrChunkNotFound }`.
- Map `rows[0]` → `retriever.ChunkContent{ID: r.ID, Tier: r.Tier, Source: r.Source, Content: r.Content, EffectiveFrom: r.EffectiveFrom, EffectiveTo: r.EffectiveTo}` and return.

**Layer 3 — `internal/knowledge/graph.go`:**
- Add the JSON-tagged wire type `ChunkPayload` for the FE: `ID string json:"id"; Tier string json:"tier"; Source string json:"source"; Content string json:"content"; EffectiveFrom int64 json:"effectiveFrom"; EffectiveTo int64 json:"effectiveTo"; Version string json:"version"`.
- Add `var ErrChunkNotFound = errors.New("knowledge: chunk not found")` (self-contained layer — do NOT leak `retriever.ErrChunkNotFound` across the package boundary; the server maps `knowledge.ErrChunkNotFound` → 404).
- Add `func (b *GraphBuilder) GetChunk(ctx context.Context, id int64) (*ChunkPayload, error)`:
  - `if b.retriever == nil { return nil, ErrNoRetriever }` (reuse the existing sentinel).
  - `c, err := b.retriever.GetChunk(ctx, id)`.
  - `if errors.Is(err, retriever.ErrChunkNotFound) { return nil, ErrChunkNotFound }`.
  - `if err != nil { return nil, err }`.
  - Return `&ChunkPayload{ID: int64ToChunkID(c.ID), Tier: c.Tier, Source: c.Source, Content: c.Content, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Version: versionFromEffective(c.EffectiveFrom, c.EffectiveTo)}`. Reusing the existing helpers makes the ID + Version labels byte-identical to the graph node labels.

**Layer 4 — `internal/server/server.go`:**
- Extend the `KnowledgeBuilder` interface: add `GetChunk(ctx context.Context, id int64) (*knowledge.ChunkPayload, error)`.
- In `New()`, add `h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)` immediately after the graph route line.
- Update the `// Routes:` doc comment to list the new endpoint.

**Layer 4 — `internal/server/knowledge.go`:**
- Add imports `"strconv"` and `"errors"` (errors not currently imported in this file).
- Add `func (s *Deps) chunkHandler(ctx context.Context, c *app.RequestContext)` mirroring `graphHandler` posture:
  - `if s.Knowledge == nil { c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "knowledge graph not initialized (set Deps.Knowledge at server construction)"}); return }` (identical envelope wording to graphHandler — uniform FE handling).
  - `idStr := c.Param("id"); id, err := strconv.ParseInt(idStr, 10, 64)`; `if err != nil { c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid chunk id"}); return }`.
  - `payload, err := s.Knowledge.GetChunk(ctx, id)`.
  - `if errors.Is(err, knowledge.ErrChunkNotFound) { c.JSON(http.StatusNotFound, graphResponseError{Error: "chunk not found"}); return }`.
  - `if err != nil { log.Printf("[WARN] server: knowledge chunk fetch failed: %v", err); c.JSON(http.StatusInternalServerError, graphResponseError{Error: "knowledge chunk fetch failed — see server logs"}); return }` (W-3: do NOT leak err.Error() — mirrors graphHandler).
  - `c.JSON(http.StatusOK, payload)`.

**Compile-break stubs (must resolve before `go build ./...` passes):**
- `internal/knowledge/graph_test.go` `stubMetaRetriever`: add `chunk retriever.ChunkContent` + `chunkErr error` fields + `func (s *stubMetaRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) { return s.chunk, s.chunkErr }`. (The `var _ retriever.Retriever = (*stubMetaRetriever)(nil)` at L53 enforces this.)
- `internal/server/testhelpers_test.go` `stubRetriever`: add a trivial `GetChunk` returning `retriever.ChunkContent{}, retriever.ErrChunkNotFound`. This stub is not exercising GetChunk — it just needs to satisfy the interface.
- `internal/agent/agent_test.go` `stubRetriever`: same trivial `GetChunk` as above.
- `internal/server/knowledge_test.go` `stubKnowledgeBuilder`: add `chunkPayload *knowledge.ChunkPayload` + `chunkErr error` + `getChunkCalls int` fields + `func (s *stubKnowledgeBuilder) GetChunk(_ context.Context, _ int64) (*knowledge.ChunkPayload, error) { s.getChunkCalls++; return s.chunkPayload, s.chunkErr }`. (The `var _ KnowledgeBuilder = (*stubKnowledgeBuilder)(nil)` at L145 enforces this.)

**Tests — ADD (mirror existing patterns):**
- `internal/retriever/mongo/mongo_test.go`: `TestClient_GetChunk_Found` (inject `&fakeFinder{rows: []docRow{{ID:7, Tier:"权威", Source:"authority/x.md", Content:"正文", EffectiveFrom:0, EffectiveTo:0}}}` via `newClientWithFake`, call `GetChunk(ctx, 7)`, assert all 6 fields round-trip). `TestClient_GetChunk_NotFound` (`&fakeFinder{rows: nil}`, assert `errors.Is(err, retriever.ErrChunkNotFound)`).
- `internal/knowledge/graph_test.go`: `TestGraphBuilder_GetChunk_Found` (inject `&stubMetaRetriever{chunk: retriever.ChunkContent{ID:7, ...}}`, call `GetChunk(ctx, 7)`, assert `ChunkPayload.ID == "7"` and `Version` matches `versionFromEffective`). `TestGraphBuilder_GetChunk_NotFound` (`stubMetaRetriever{chunkErr: retriever.ErrChunkNotFound}`, assert `errors.Is(err, ErrChunkNotFound)`). `TestGraphBuilder_GetChunk_NilRetriever` (`New(nil)`, assert `errors.Is(err, ErrNoRetriever)`). `TestGraphBuilder_GetChunk_PropagatesRetrieverError` (`stubMetaRetriever{chunkErr: errors.New("boom")}`, assert the raw error propagates and is NOT `ErrChunkNotFound`).
- `internal/server/knowledge_test.go`: add `invokeChunkHandler(t, deps, idPath string)` helper that registers `GET /api/knowledge/chunk/:id` + `ut.PerformRequest(... "/api/knowledge/chunk/" + idPath ...)`. Tests: `TestChunkHandler_NilBuilderReturns503`, `TestChunkHandler_BadIDReturns400` (path `"abc"`), `TestChunkHandler_Found` (stub `chunkPayload: &knowledge.ChunkPayload{ID:"1", Tier:"权威", Source:"authority/a.md", Content:"正文", EffectiveFrom:0, EffectiveTo:0, Version:""}`, assert 200 + JSON round-trip + `getChunkCalls==1`), `TestChunkHandler_NotFoundReturns404` (stub `chunkErr: knowledge.ErrChunkNotFound`, assert 404 + envelope `{error:"chunk not found"}`), `TestChunkHandler_BuildErrorReturns500` (stub `chunkErr: errors.New("boom")`, assert 500 + generic message + `NotContains "boom"` per W-3), `TestChunkHandler_RouteRegistered` (`New(":0", deps)` + `ut.PerformRequest` → assert `!= 404`).

**NOTE on the milvus test:** do NOT add a live milvus GetChunk test (milvus is runtime-broken per D11). The `var _ retriever.Retriever = (*MilvusClient)(nil)` assertion + the compile is the safety net. The mongo test covers the by-id semantics; milvus mirrors the same SDK Query shape as its already-untested ListMetadata.

Run `go build ./...` FIRST after each layer to enumerate any conformance break you missed (the `var _` assertions are the safety net). Do not move to the next layer until the current one compiles.
  </action>
  <verify>
    <automated>cd /d/CODE/ClaudeCode/ai-agent && go build ./... && go vet ./... && go test ./internal/retriever/... ./internal/knowledge/... ./internal/server/... && go test -race ./internal/retriever/... ./internal/knowledge/... ./internal/server/...</automated>
  </verify>
  <done>
- `GET /api/knowledge/chunk/:id` is registered in `server.New` and never 404s.
- milvus + mongo backends both implement `GetChunk` (compile-time enforced via `var _`).
- `knowledge.GraphBuilder.GetChunk` returns `*ChunkPayload` (200), `ErrChunkNotFound` (→404), `ErrNoRetriever` (→500), or propagates other errors (→500).
- `chunkHandler` posture mirrors `graphHandler`: nil→503, bad-id→400, not-found→404, other-err→generic 500 (W-3, no err.Error leak), ok→200.
- `go build ./... && go vet ./...` green; touched-package `go test` + `go test -race` green with the new tests above.
  </done>
</task>

<task type="auto">
  <name>Task 2: FE — fetchChunkContent + ChunkSourceView async-load on fixture-miss + tests</name>
  <files>
web/src/lib/chunk-lookup.ts,
web/src/components/center/ChunkSourceView.tsx,
web/src/__tests__/chunk-lookup.test.ts,
web/src/__tests__/ChunkSourceView.test.tsx
  </files>
  <action>
Wire the FE half: a thin async helper + a ChunkSourceView branch that fires it ONLY when the fixture lookup misses (preserving the citation path unchanged).

**`web/src/lib/chunk-lookup.ts` (extend; do NOT change existing exports):**
- Promote `NOT_FOUND_TEXT` from a private const to an exported const: `export const NOT_FOUND_TEXT = '(chunk not found in fixture)';`. Keep the existing string value — ChunkSourceView compares against it to detect the fixture-miss branch.
- KEEP `lookupChunk` and `extractChunkMetadata` UNCHANGED (the verdict-citation path still reads the fixture synchronously — `CitationCluster` depends on it).
- Add `export async function fetchChunkContent(chunkId: string): Promise<{ text: string; tier: Tier | null } | null>`:
  - `const resp = await fetch(\`/api/knowledge/chunk/${encodeURIComponent(chunkId)}\`, { headers: { Accept: 'application/json' } })` — mirror the API-base + Accept-header convention in `useKnowledgeGraph.ts` (same `/api/knowledge/...` origin; Vite dev proxy handles it).
  - `if (resp.status === 404) return null;` (distinct from the graph hook's 503 handling — a missing chunk is a soft miss, not an initialization failure).
  - `if (!resp.ok) { const body = await resp.json().catch(() => ({})); throw new Error(body.error ?? \`HTTP ${resp.status} ${resp.statusText}\`); }` (let the caller render the error state — do NOT swallow).
  - `const body = (await resp.json()) as { id: string; tier: string; source: string; content: string; effectiveFrom: number; effectiveTo: number; version: string };`
  - `return { text: body.content, tier: (body.tier as Tier) ?? null };`

**`web/src/components/center/ChunkSourceView.tsx` (extend + one correctness fix):**
- Imports: add `useEffect, useState` to the react import; add `fetchChunkContent, NOT_FOUND_TEXT` to the chunk-lookup import.
- Keep the existing `const result = useMemo(() => lookupChunk(chunkId), [chunkId]);` — the fixture path is unchanged for citations.
- Add async state for the fixture-miss branch: `const [asyncText, setAsyncText] = useState<string | null>(null);` and `const [asyncStatus, setAsyncStatus] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');`.
- Add `useEffect(() => { ... }, [chunkId])` that ONLY fires when `result.text === NOT_FOUND_TEXT` (the fixture-miss case — i.e. graph-browsed chunks). Inside: set 'loading'; call `fetchChunkContent(chunkId)`; on success → `setAsyncText(data.text); setAsyncStatus('ready')`; on `null` (404) → `setAsyncText('（片段不存在）'); setAsyncStatus('ready')`; on throw → `setAsyncStatus('error')`. Guard against stale resolves with a `cancelled` flag set in the cleanup (mirror `useKnowledgeGraph`).
- Compute the rendered body: if `result.text !== NOT_FOUND_TEXT` → `result.text` (fixture hit); else switch on `asyncStatus`: 'loading' → `'加载中…'`, 'error' → `'加载片段失败'`, 'ready' → `asyncText ?? ''`. (Replace the current `{result.text}` inside `<pre data-testid="chunk-verbatim">` with this computed value.)
- Correctness fix: change `data-tier={result.tier}` → `data-tier={tier}` (use the PROP). For graph chunks not in the fixture, `result.tier` is always the `'权威'` fallback, which would lie about the tier the graph node actually carried. The existing data-tier tests still pass because for fixture chunkIds (c-001=权威, c-004=草稿) the fixture tier happens to match the prop tier.
- Leave the metadata row (TierBadge + version + VersionBadge + 生效日期 + 失效日期) UNCHANGED — it is already prop-driven and correct for both citation and graph paths.

**`web/src/__tests__/chunk-lookup.test.ts` (NEW — mirror the vitest pattern in ChunkSourceView.test.tsx):**
- `import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';`
- `beforeEach`/`afterEach`: `vi.spyOn(globalThis, 'fetch')` + `vi.restoreAllMocks()` to isolate fetch state between tests.
- `fetchChunkContent resolves {text, tier} on 200`: mock `fetch` to resolve `new Response(JSON.stringify({id:"1",tier:"权威",source:"authority/x.md",content:"正文内容",effectiveFrom:0,effectiveTo:0,version:""}), {status:200})`; `await fetchChunkContent("1")` → deepEqual `{text:"正文内容", tier:"权威"}`.
- `fetchChunkContent returns null on 404`: mock status 404; expect resolves `null`.
- `fetchChunkContent throws on 500`: mock status 500 + body `{error:"boom"}`; expect rejects with message containing "boom".

**`web/src/__tests__/ChunkSourceView.test.tsx` (UPDATE the broken assertion):**
- The existing test at L92-105 (`renders "(chunk not found in fixture)" and 未知 metadata for a missing chunkId`) ASSERTS THE OLD BEHAVIOR. It MUST be replaced because graph-browsed chunks now fetch. Rewrite as: mock `globalThis.fetch` (vitest `vi.spyOn`) to resolve `new Response(JSON.stringify({id:"c-graph-1",tier:"历史",source:"historical/old.md",content:"来自后端的原文",effectiveFrom:0,effectiveTo:0,version:""}), {status:200})`; render `<ChunkSourceView chunkId="c-graph-1" tier="历史" version={null} effectiveDate={null} effectiveTo={null} />`; `const verbatim = await screen.findByTestId('chunk-verbatim');` (use `findBy` for async); `expect(verbatim.textContent).toContain('来自后端的原文');` `expect(verbatim.textContent).not.toContain('(chunk not found in fixture)');` keep the metadata assertions (`版本: 未知`, `生效日期: 未知` — those are still prop-driven and correct).
- Add `beforeEach`/`afterEach` to mock + restore `globalThis.fetch` for the whole describe block.
- The other existing tests (fixture-hit cases c-001, c-002, c-004) need NO changes — `lookupChunk` is unchanged, so `result.text !== NOT_FOUND_TEXT` and the async branch never fires (the fetch mock is never invoked). This keeps the citation path regression-free.
- Add ONE new test: `renders '加载中…' before the fetch resolves for a fixture-miss chunk` — mock `fetch` to return a never-resolving promise (or `new Promise(() => {})`); render with a non-fixture chunkId; `expect(screen.getByTestId('chunk-verbatim').textContent).toContain('加载中…');`.

Run the full FE suite to confirm no other component regressed (CenterPanel and citations tests both touch ChunkSourceView).
  </action>
  <verify>
    <automated>cd /d/CODE/ClaudeCode/ai-agent/web && npx tsc --noEmit && npx vitest run</automated>
  </verify>
  <done>
- `lookupChunk` + `extractChunkMetadata` unchanged (citation path regression-free — the c-001/c-002/c-004 tests still pass without invoking fetch).
- `fetchChunkContent("1")` returns `{text, tier}` on 200, `null` on 404, throws on other errors.
- ChunkSourceView renders `result.text` for fixture hits; for fixture misses, renders `加载中…` → fetched verbatim text (or `加载片段失败` on error).
- `data-tier` now reflects the PROP (correct for graph-browsed chunks whose tier the fixture does not carry).
- `cd web && npx tsc --noEmit` exits 0; `npx vitest run` green (existing tests + the 3 new chunk-lookup tests + the updated ChunkSourceView test).
  </done>
</task>

<task type="auto">
  <name>Task 3: Verify full gates + flip UAT #4 / REMAINING §3 / STATE + write SUMMARY</name>
  <files>
.planning/phases/05-skill-platform-full-knowledge-boundaries/05-UAT.md,
.planning/phases/05-skill-platform-full-knowledge-boundaries/05-REMAINING-WORK.md,
.planning/STATE.md,
.planning/quick/260813-dno-uat-4-graph-chunk-full-text-get-api-know/260813-dno-SUMMARY.md
  </files>
  <action>
Run the cross-cutting verification bar (matches the §2 #7 / OOS-1 closeout standard), then update the tracking artifacts in ONE closeout commit so the code commit stays atomic with Task 1 + Task 2.

**Full verification gate (must ALL pass before touching docs):**
- `go build ./...` — full repo compile (catches accidental regressions outside touched packages).
- `go vet ./...` — full repo vet.
- `go test ./...` — full Go suite.
- `go test -race ./internal/retriever/... ./internal/knowledge/... ./internal/server/...` — race-clean on touched packages.
- `cd web && npx tsc --noEmit && npx vitest run` — full FE suite.
If any gate fails, STOP and fix before flipping the docs — do not mark UAT #4 fixed on a red bar.

**Manual sanity (NOT a checkpoint — automation only; the bar above is the gate):**
Optionally curl the new endpoint against a running server to confirm the wire shape end-to-end: `curl -s localhost:8888/api/knowledge/chunk/1 | jq .` should yield `{id:"1", tier:"权威", source:"...", content:"...", effectiveFrom:..., effectiveTo:..., version:"..."}`. Skip if no server is running (the unit + handler tests already pin the wire shape).

**Doc flips (mirror the §2 closeout pattern from commit 269cb0c):**
- `.planning/phases/05-skill-platform-full-knowledge-boundaries/05-UAT.md`: find Test 4 (Knowledge Graph chunk content) and flip `result: issue` → `result: fixed`; flip the #4 Gap `status: failed` (or `open`) → `status: fixed`. Add the fixing commit SHAs in an evidence note (you'll have them after committing Tasks 1 + 2). If the UAT file references `(chunk not found in fixture)` as expected-bad behavior, update the expected text to the fetched-verbatim contract.
- `.planning/phases/05-skill-platform-full-knowledge-boundaries/05-REMAINING-WORK.md`: mark §3 as `✅ DONE` at the heading (mirror the §1 / §2 `✅ DONE` annotation pattern); update §0's "REMAINING" count (currently "2 items" after §2 closed — now 1 item remains: §4); update the "Recommended Sequencing" section if it still lists §3 as pending.
- `.planning/STATE.md`: in the Session Continuity "Debug fixes" block, add a §3 bullet alongside the existing §2 bullet — note the fixing commits, that Phase 5 UAT is now 11/11 pass (or the new count after flipping #4), and that §4 (#2 left-sidebar IA) is the only remaining item. Update the `last_updated` frontmatter timestamp.
- Write `.planning/quick/260813-dno-uat-4-graph-chunk-full-text-get-api-know/260813-dno-SUMMARY.md` using the project's SUMMARY template (mirror the §2 SUMMARY at `.planning/quick/260813-1y0-hot-reload-toast-bridge-polling-b/260813-1y0-SUMMARY.md` if it exists, else the standard summary.md template). Record: problem (UAT #4), root cause (chunk-lookup only resolved fixture citations; no chunk-content BE endpoint), fix (the 3-layer BE + async FE fetch), files touched, verification evidence, decisions honored (full-text-in-scope per LOCKED decision), invariants preserved (T-5-22, W-3, D-09).

**Commit hygiene:**
- `branching_strategy: none` — commits land on master. Make the code commits first (Task 1 + Task 2 can be one or two commits — the executor's call), then a single closeout commit for the doc flips + SUMMARY. Match the §2 closeout pattern (commit 269cb0c).
- Do NOT stage the pre-existing unrelated drift in `.planning/config.json` (`_auto_chain_active`, GSD internal state) — it is not §3-related. Do NOT stage `.claude/` or `skillforge.db*` (never commit per CLAUDE.md).
- Commit message prefix suggestion: `feat(server): GET /api/knowledge/chunk/:id ...` for Task 1, `feat(web): ChunkSourceView async chunk-content fetch ...` for Task 2, `docs(05): close §3 graph chunk full-text — UAT #4 fixed` for the closeout.
  </action>
  <verify>
    <automated>cd /d/CODE/ClaudeCode/ai-agent && go build ./... && go vet ./... && go test ./... && go test -race ./internal/retriever/... ./internal/knowledge/... ./internal/server/... && cd web && npx tsc --noEmit && npx vitest run</automated>
  </verify>
  <done>
- Full repo `go build && go vet && go test` green; `-race` green on touched packages; `tsc --noEmit` + `vitest run` green.
- `05-UAT.md` Test 4 result → `fixed`; Gap #4 → `fixed`.
- `05-REMAINING-WORK.md` §3 → `✅ DONE`; §0 REMAINING count → 1 (only §4 left).
- `STATE.md` Session Continuity updated with the §3 bullet + commit SHAs; `last_updated` bumped.
- `260813-dno-SUMMARY.md` written.
- All commits on master; no unrelated drift staged.
  </done>
</task>

</tasks>

<verification>
End-to-end verification that UAT #4 is closed:
1. **BE unit:** `go test ./internal/retriever/... ./internal/knowledge/... ./internal/server/...` — the new `GetChunk` / `chunkHandler` tests pass (found / not-found / bad-id / nil-builder / 500 / route-registered).
2. **BE race:** `go test -race ./internal/retriever/... ./internal/knowledge/... ./internal/server/...` — no race in the new by-id fetch path.
3. **FE unit:** `cd web && npx vitest run` — `fetchChunkContent` resolves/null/throws; ChunkSourceView renders fetched text for fixture-miss + stays on fixture path for citations; `加载中…` loading state.
4. **Full repo gates:** `go build ./... && go vet ./... && go test ./...` + `cd web && npx tsc --noEmit` all green (no accidental regression outside the touched packages).
5. **Wire shape (optional, if server running):** `curl -s localhost:8888/api/knowledge/chunk/1` → 200 JSON `{id, tier, source, content, effectiveFrom, effectiveTo, version}`; `curl -s localhost:8888/api/knowledge/chunk/999999` → 404 `{error:"chunk not found"}`; `curl -s localhost:8888/api/knowledge/chunk/abc` → 400 `{error:"invalid chunk id"}`.
6. **Doc traceability:** UAT #4 flipped to fixed in `05-UAT.md`; §3 marked DONE in `05-REMAINING-WORK.md`; STATE.md updated.
</verification>

<success_criteria>
- Clicking a chunk in the SC-5 Knowledge Graph renders the verbatim chunk text (NOT `(chunk not found in fixture)`) in ChunkSourceView, sourced from `GET /api/knowledge/chunk/:id`.
- Citation path (verdict → ChunkSourceView) UNCHANGED — `lookupChunk` still reads the fixture synchronously; the fetch fires ONLY on fixture-miss.
- Graph payload contract (T-5-22) preserved — ChunkNode stays metadata-only; content is fetched on-demand.
- W-3 preserved — `chunkHandler` 500 envelope carries the generic message; no internal err.Error() leak.
- D-09 trust boundary preserved — this is a by-id READ of a chunk the graph already enumerated; it does NOT bypass tier filtering (the graph only lists existing chunk IDs).
- All full-repo gates green (go build/vet/test + go test -race on touched packages + tsc + vitest).
- UAT #4 marked fixed; Phase 5 gap-closure §3 closed.
</success_criteria>

<output>
Create `.planning/quick/260813-dno-uat-4-graph-chunk-full-text-get-api-know/260813-dno-SUMMARY.md` when done (Task 3).
</output>
