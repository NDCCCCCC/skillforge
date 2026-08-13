---
quick_id: 260813-dno
slug: uat-4-graph-chunk-full-text-get-api-know
status: complete
date: 2026-08-13
commit: 8939a74 (BE) + aec582e (FE)
---

# Summary 260813-dno: UAT #4 — graph chunk full-text (GET /api/knowledge/chunk/:id)

## Outcome
DONE. Clicking a chunk in the SC-5 Knowledge Graph Browser now renders the
**verbatim chunk text** in `ChunkSourceView` instead of `(chunk not found in
fixture)`. The fix is a 3-layer backend slice + a thin async FE fetch that
fires ONLY on a fixture-miss (i.e. a graph-browsed chunk the fixture does not
carry). The graph payload stays metadata-only (T-5-22 preserved); the
citation path (verdict → fixture lookup) is unchanged. This was the last
metadata-vs-content seam left half-wired in Phase 5.

## Problem (UAT #4)
Graph chunk click → `ChunkSourceView` opened (tier + 当前有效 badge +
生效日期) but the body rendered `(chunk not found in fixture)`. Verbatim chunk
text was NOT displayed when navigating from the graph browser. Severity: major.

## Root cause
`web/src/lib/chunk-lookup.ts` `NOT_FOUND_TEXT` fallback — content lookup
resolved ONLY fixture/verdict citations. The backend exposed NO chunk-content
endpoint (routes: chat / skills / wizard[+/commit] / knowledge/graph). The
graph endpoint is metadata-only per T-5-22 (threat-mitigation: no verbatim
text over the graph wire), so graph-browsed chunks had no client-side content
resolution path. The seam between "graph enumerates chunk IDs" and "operator
clicks one to read it" was missing.

## Fix

### Backend (commit `8939a74`) — 3-layer slice mirroring the graph endpoint
- **`internal/retriever/retriever.go`:** added `ChunkContent` (lean by-id
  projection — no Score, no Embedding) + `ErrChunkNotFound` sentinel + the
  `GetChunk(ctx, id) (ChunkContent, error)` method on the `Retriever`
  interface. Doc-comment notes it is the on-demand content fetch path that
  preserves T-5-22 and D-09.
- **`internal/retriever/milvus/client.go`:** implemented `GetChunk` mirroring
  `ListMetadata`'s Query-by-expression shape (`id == %d` — int64 format is
  injection-safe; no raw URL-param concat). Empty `idCol` / zero rows →
  `retriever.ErrChunkNotFound`. Added `var _ retriever.Retriever =
  (*MilvusClient)(nil)` (defense-in-depth — the mongo backend already had it;
  this keeps future Retriever drift surfacing here, not at a distant call site).
- **`internal/retriever/mongo/mongo.go`:** implemented `GetChunk` via the
  existing `rowFinder` seam (`FindRows(ctx, bson.D{{Key:"id", Value: id}})`);
  `len(rows)==0` → `ErrChunkNotFound`; row[0] → `ChunkContent` 1:1.
- **`internal/knowledge/graph.go`:** added the JSON-tagged wire type
  `ChunkPayload` (id/tier/source/content/effectiveFrom/effectiveTo/version) +
  `ErrChunkNotFound` (self-contained layer — does NOT leak
  `retriever.ErrChunkNotFound` across the package boundary) +
  `GraphBuilder.GetChunk` which maps `retriever.ErrChunkNotFound` →
  `knowledge.ErrChunkNotFound` and reuses `int64ToChunkID` +
  `versionFromEffective` so the ID/Version labels are byte-identical to the
  graph node labels.
- **`internal/server/server.go`:** extended `KnowledgeBuilder` with `GetChunk`;
  registered `h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)`.
- **`internal/server/knowledge.go`:** added `chunkHandler` mirroring
  `graphHandler` posture EXACTLY so FE error handling is uniform across
  `/api/knowledge/*`: nil→503, bad-id (ParseInt)→400, `ErrChunkNotFound`→404,
  other-err→generic 500 (W-3: logs `[WARN]` full err server-side, NEVER echoes
  `err.Error()` to the client), ok→200 `ChunkPayload`.

### Frontend (commit `aec582e`) — async fetch on fixture-miss
- **`web/src/lib/chunk-lookup.ts`:** promoted `NOT_FOUND_TEXT` to an exported
  const (the fixture-miss signal). Added `fetchChunkContent(chunkId)`:
  `GET /api/knowledge/chunk/:id` — 200 → `{text, tier}`, 404 → `null` (soft
  miss, distinct from the graph hook's 503 handling), other `!ok` → throws
  (caller renders error, never swallowed). Mirrors the API-base + Accept-header
  convention in `useKnowledgeGraph.ts`. `lookupChunk` + `extractChunkMetadata`
  UNCHANGED (citation path regression-free).
- **`web/src/components/center/ChunkSourceView.tsx`:** added async state
  (`asyncText`/`asyncStatus`) + a `useEffect[chunkId]` that fires
  `fetchChunkContent` ONLY when `result.text === NOT_FOUND_TEXT` (the
  fixture-miss branch — i.e. graph-browsed chunks). Rendered body switches:
  fixture hit → `result.text`; fixture miss → `加载中…` / `加载片段失败` /
  ready `asyncText` (404 → `（片段不存在）`). Stale-resolve guard via a
  `cancelled` flag in the effect cleanup. **Correctness fix:** `data-tier`
  changed from `result.tier` (the lookup fallback, always `权威`) to the `tier`
  PROP — correct for graph-browsed chunks whose tier the fixture does not
  carry (the existing fixture-hit tests still pass because the prop tier
  matches the fixture tier).

### Tests
- **`internal/retriever/mongo/mongo_test.go`:** `TestClient_GetChunk_Found`
  (all 6 fields round-trip + filter is `{id:<id>}`), `_NotFound`
  (`ErrChunkNotFound`), `_FinderErrorPropagates` (raw err, NOT not-found).
- **`internal/knowledge/graph_test.go`:** `_Found` (ID via `int64ToChunkID`,
  Version via `versionFromEffective`), `_NotFound` (`ErrChunkNotFound` mapped,
  NOT leaking `retriever.ErrChunkNotFound`), `_NilRetriever` (`ErrNoRetriever`),
  `_PropagatesRetrieverError` (raw err, NOT not-found).
- **`internal/server/knowledge_test.go`:** `chunkHandler` 6-case —
  `_NilBuilderReturns503`, `_BadIDReturns400` (+ GetChunk NOT invoked),
  `_Found` (JSON round-trip + `getChunkCalls==1`), `_NotFoundReturns404`,
  `_BuildErrorReturns500` (generic msg, NOT `boom` per W-3), `_RouteRegistered`
  (never 404).
- **7 interface-extension conformance sites resolved** (the `var _` safety net
  caught all): `stubMetaRetriever` (graph_test), `stubRetriever`
  (testhelpers_test + agent_test — trivial `ErrChunkNotFound`), `stubKnowledgeBuilder`
  (knowledge_test), `*Client` + `*MilvusClient` production, `*GraphBuilder`
  production.
- **`web/src/__tests__/chunk-lookup.test.ts`** (new): 200 → `{text,tier}`,
  404 → `null`, 500 → throws `body.error`, path encoding.
- **`web/src/__tests__/ChunkSourceView.test.tsx`:** replaced the old
  `(chunk not found in fixture)` assertion (it pinned the bug) with a fetch
  mock + `findByTestId` asserting the fetched verbatim text renders; added
  `加载中…` loading-state + `加载片段失败` error-state tests. Fixture-hit
  cases (c-001/c-002/c-004) unchanged — `lookupChunk` still reads the fixture
  synchronously, so the fetch is never invoked.

## Verification (all green — the gate ran BEFORE any doc flip)
- **BE:** `go build ./...` + `go vet ./...` + `go test ./...` (full repo) +
  `go test -race ./internal/retriever/... ./internal/knowledge/... ./internal/server/...`.
- **FE:** `cd web && npx tsc --noEmit` (0 errors) + `npx vitest run` (29 files /
  196 tests pass — incl. the 4 new chunk-lookup tests + the updated
  ChunkSourceView suite).
- **Wire shape** (optional live curl, deferred to UAT): `GET /api/knowledge/chunk/1`
  → 200 `{id,tier,source,content,effectiveFrom,effectiveTo,version}`;
  `…/999999` → 404 `{error:"chunk not found"}`; `…/abc` → 400
  `{error:"invalid chunk id"}`. Pinned by the unit + handler tests.

## Decisions honored
- **full-text IN-SCOPE** (LOCKED by user 2026-08-13): added the content
  endpoint + taught the FE to fetch. Did NOT fall back to metadata-only.

## Invariants preserved
- **T-5-22:** `ChunkNode` in the graph payload stays metadata-only; content is
  fetched on-demand by the separate `GET /api/knowledge/chunk/:id` path. The
  graph response did NOT gain a content field.
- **W-3:** `chunkHandler` 500 envelope is generic
  (`"knowledge chunk fetch failed — see server logs"`); NEVER echoes
  `err.Error()`; full error logged `[WARN]` server-side. Mirrors `graphHandler`.
- **D-09:** by-id READ of a chunk the graph already enumerated — does NOT bypass
  tier filtering (the graph only lists existing chunk IDs; this is a by-PK read).
- **D-15 wire boundary:** `knowledge.ChunkPayload` is the JSON-tagged wire type;
  the FE `fetchChunkContent` body type matches it. No `retriever.ErrChunkNotFound`
  leaked across the knowledge package boundary (mapped to `knowledge.ErrChunkNotFound`).

## Notes
- milvus is runtime-broken per D11 (image configured for external etcd) — the
  `GetChunk` impl only needs to COMPILE and mirror the same SDK Query shape as
  `ListMetadata`. The by-id semantics are pinned by the mongo tests; a live
  milvus GetChunk test was deliberately NOT added (matches the existing
  untested `ListMetadata`).
- Pre-existing unrelated drift (`.planning/config.json` `_auto_chain_active`,
  `.claude/`, `skillforge.db*`) was NOT bundled into any commit — left
  unstaged per the do_not_commit instruction.
- `STATE.md` + `260813-dno-PLAN.md` were left unstaged — the orchestrator owns them.
