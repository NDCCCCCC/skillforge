# Phase 28 PATTERNS

**Mapped:** 2026-09-07
**Files analyzed:** 12 new/modified files
**Analogs found:** 12 / 12

---

## Pattern 1: existing graph node set

- **Closest analog:** `internal/knowledge/graph.go` -- `GraphBuilder` struct and its `Build` method (lines 159-298)
- **Reuse vs adapt:** Phase 28 must reuse the existing `KBRegistry`-driven node set (tier -> doc -> chunk hierarchy) from Phase 26-03 and add chunk-level cross-doc edges on top. The `Graph` struct (lines 106-108), `TierNode` (lines 113-118), `DocNode` (lines 123-128), and `ChunkNode` (lines 132-137) types are the existing node shapes. `GraphBuilder.Build` at line 196 is the canonical node-set builder.
- **Pitfalls:** Do NOT regress Phase 26's three-state walker (active/deactivated/unregistered). The `Build` method already enforces this at lines 216-225: empty tier = `ErrInvalidTier` (hard fail), non-active tier = skipped with WARN (deactivated/retired base's stale rows). Any Phase 28 change to this decision table would break the KBM-02 invariant.
- **Key code to copy from:**
  - `GraphBuilder` struct: `internal/knowledge/graph.go:159`
  - `Build` tier re-check decision table: `internal/knowledge/graph.go:216-225`
  - Active-slug node set (ascending): `internal/knowledge/graph.go:258-265`

**Phase 28 reuse:** adapted verbatim -- extend `Graph` struct with an `Edges []Edge` field; do NOT touch `Build`'s decision table.

---

## Pattern 2: existing graphHandler contract

- **Closest analog:** `internal/server/knowledge.go` -- `graphHandler` (lines 158-181) and `chunkHandler` (lines 204-245)
- **Reuse vs adapt:** Keep `GET /api/knowledge/graph` returning the existing `{tiers: [...]}` payload shape (FE `GraphCanvas` depends on it). ADD two new handlers:
  - `GET /api/knowledge/graph/neighbors?chunk_id=X` -- returns `{neighbors: [{chunk_id, edge_type, tier, source}]}` for a given chunk's direct neighbors
  - `GET /api/knowledge/graph/path?from=X&to=Y&hops=N` -- returns `{path: [chunk_ids], edges: [{from, to, edge_type}]}` for a path query
- **Wire shape:** The `graphResponseError` envelope (lines 66-68) is the standard error shape across all `/api/knowledge/*` handlers; reuse it for the new endpoints.
- **503 pattern:** Both `graphHandler` and `chunkHandler` return 503 when `s.Knowledge == nil` (lines 159-164 / 205-209). The new neighbors/path handlers must follow the same nil-check pattern.
- **W-3 error masking:** Both handlers log the full error at WARN server-side but return a generic message to the FE (lines 174-177 / 235-241). Follow this for the new handlers.
- **Key code to copy from:**
  - `graphHandler` nil-check + 503: `internal/server/knowledge.go:158-164`
  - `graphHandler` error masking (W-3): `internal/server/knowledge.go:174-177`
  - `graphResponseError` envelope: `internal/server/knowledge.go:66-68`
  - `parseLenientChunkID` for URL-param chunk ID parsing: `internal/server/knowledge.go:100-146`

**Phase 28 reuse:** adapted verbatim -- add two new handler methods on `*Deps` following the same nil-check + W-3 masking pattern.

---

## Pattern 3: chunk metadata fields for relation derivation

- **Closest analog:** `internal/retriever/retriever.go` -- `Document` struct (lines 70-78), `DocumentMeta` struct (lines 89-95), `Hit` struct (lines 45-53)
- **Reuse:** Derive same-doc version chain from `EffectiveFrom`/`EffectiveTo` + `Source` -- these are all derivable from existing `DocumentMeta` fields with no schema migration. The `versionFromEffective` helper in `internal/knowledge/graph.go:401-412` already renders version labels from these fields. Derive cross-doc co-occurrence from content (rare-token Jaccard) -- these fields are all already on `DocumentMeta` and `Hit`.
- **No new fields needed:** The existing `DocumentMeta` struct carries `ID`, `Tier`, `Source`, `EffectiveFrom`, `EffectiveTo` -- exactly what graph edge derivation needs. No schema migration is required.
- **Key code to copy from:**
  - `Document` struct with `EffectiveFrom`/`EffectiveTo`: `internal/retriever/retriever.go:70-78`
  - `DocumentMeta` struct (metadata projection): `internal/retriever/retriever.go:89-95`
  - `Hit` struct with `EffectiveFrom`/`EffectiveTo`/`Source`/`Tier`: `internal/retriever/retriever.go:45-53`
  - `versionFromEffective` label renderer: `internal/knowledge/graph.go:401-412`

**Phase 28 reuse:** verbatim -- `DocumentMeta` fields are sufficient for all three edge derivations (version chain, co-occurrence, cross-tier).

---

## Pattern 4: citation co-occurrence edges

- **Closest analog:** `internal/server/pipeline.go` -- the `verdict.Citation` type and the `citableChunksByID` closure (lines 1048-1057) that builds the grounding map from `v.Citations`
- **Reuse:** Derive co-occurrence edges from verdict citations. The `verdict.Citation` struct has `ChunkID` and `Tier` fields. The `citableChunksByID` map keys chunks by `strconv.FormatInt(ch.ID, 10)` (line 1054). Track which chunks are cited in the same verdict by grouping `verdict.Citations` by verdict session -- this requires reading `store.AuditRow.VerdictJSON` for rows with non-empty `VerdictJSON`. The audit row's `VerdictJSON` field (lines 96-97 of `audit.go`) contains the full verdict JSON including the `citations` array.
- **Derivation approach:** After `GraphBuilder.Build` populates the hierarchy, a secondary pass over `store.QueryAudit` (filtering by `Action` = `"verdict"` and non-empty `VerdictJSON`) extracts citation pairs from each verdict's `citations` array. Chunks that appear together in the same verdict's citation list get a `co_cited` edge.
- **Key code to copy from:**
  - `verdict.Citation` struct: `internal/verdict/verdict.go` (imported at `pipeline.go:45`)
  - `citableChunksByID` map key shape (decimal string): `internal/server/pipeline.go:1048-1057`
  - `store.AuditRow.VerdictJSON` field: `internal/store/sqlite/audit.go:96-97`
  - `store.AuditRow.ChunkIDs` field (comma-separated chunk IDs): `internal/store/sqlite/audit.go:99`
  - `store.AuditFilter` with `Action` filter: `internal/store/sqlite/audit.go:177-179`

**Phase 28 reuse:** adapted verbatim -- parse `VerdictJSON` from audit rows to extract citation co-occurrence pairs; no new schema needed.

---

## Pattern 5: memoization cache pattern

- **Closest analog:** `internal/knowledge/indexer.go` -- the `indexSeedHook` callback pattern (lines 354-381) and `SetIndexAuditHook` setter pattern (lines 369-374)
- **Alternative analog:** `internal/server/upload.go` -- the `docUploadAuditHook` package var + `SetDocUploadAuditHook` setter (lines 67-72); also the `slugUploadMutexes sync.Map` pattern for per-request state (lines 74-82)
- **Adapt:** Implement a per-request memoization cache for the graph derivation with a 5-second TTL. Use `sync.Map` for thread-safety (same pattern as `slugUploadMutexes` at `upload.go:77`). Cache key: hash of the registry signature + corpus content hash. Cache invalidation: call `graphCacheInvalidate()` when `docUploadAuditHook` fires (Phase 27 -> 28 cross-phase hook).
- **The `sync.Map` pattern:** `upload.go:77` shows the exact idiom -- declare `var slugUploadMutexes sync.Map`, then `LoadOrStore` per-key. For the graph cache, use the same `sync.Map` idiom for the TTL-gated result.
- **Callback seam pattern:** The `SetIndexAuditHook` pattern (indexer.go:369-374) shows how to wire a boot-time callback that is nil by default and set from `main.go`. Phase 28 should add `var graphCacheInvalidateHook func()` + `SetGraphCacheInvalidateHook(fn func())` and wire it in `main.go` alongside the `SetDocUploadAuditHook` call at `main.go:766`.
- **Key code to copy from:**
  - `sync.Map` usage for per-key mutexes: `internal/server/upload.go:74-82`
  - `SetIndexAuditHook` setter pattern: `internal/knowledge/indexer.go:369-374`
  - `SetDocUploadAuditHook` setter + boot wiring: `internal/server/upload.go:71-72` + `cmd/server/main.go:766-776`

**Phase 28 reuse:** adapted verbatim -- use `sync.Map` for the cache and the callback-seam setter pattern for the invalidation hook.

---

## Pattern 6: threshold / cap pattern

- **Closest analog:** `internal/server/upload.go` -- `SKILLFORGE_UPLOAD_MAX_BYTES` env var handling (lines 312, 124-127) and the `UploadMaxBodyBytes` field in `Deps` (wired at `main.go:834`)
- **Reuse:** Define a `GraphLimits` struct in `internal/server/server.go`:

```go
type GraphLimits struct {
    MaxNodes        int
    MaxEdges        int
    NeighborDepthLimit int   // max depth for neighbors traversal
    PathHopsLimit   int     // max hops for path finding
}
```

Then in `main.go`, read four new env vars alongside the upload max:

```go
graphMaxNodes := parseIntEnvOrDefault("SKILLFORGE_GRAPH_MAX_NODES", 1000)
graphMaxEdges := parseIntEnvOrDefault("SKILLFORGE_GRAPH_MAX_EDGES", 5000)
graphNeighborDepth := parseIntEnvOrDefault("SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT", 3)
graphPathHops := parseIntEnvOrDefault("SKILLFORGE_GRAPH_PATH_HOPS_LIMIT", 5)
```

Wire `GraphLimits` into `Deps` and pass it to the graph builder/cache.

- **Key code to copy from:**
  - `parseBytesEnvOrDefault` helper: `cmd/server/main.go:166-177`
  - `SKILLFORGE_UPLOAD_MAX_BYTES` reading + wiring: `cmd/server/main.go:312` + `cmd/server/main.go:834`
  - `UploadMaxBodyBytes` field in `Deps`: `internal/server/server.go` (referenced at `upload.go:124`)
  - `uploadMaxBodyDefault` constant (100 MiB): `internal/server/upload.go:40`

**Phase 28 reuse:** adapted verbatim -- same `parse*EnvOrDefault` pattern for four new env vars; same `Deps` field wiring pattern.

---

## Pattern 7: degraded-mode response shape

- **Closest analog:** `internal/server/upload.go` -- 503 + `Retry-After: 30` on backend unavailable (lines 252-256)
- **Adapt:** When the graph exceeds `MAX_NODES` or `MAX_EDGES`, return a degraded response:

```json
{
  "nodes": [...skeleton tiers...],
  "edges": [],
  "degraded": "node_count_exceeded: MAX_NODES=100, actual=247"
}
```

The `nodes` skeleton is the existing `Graph` hierarchy (no edge data computed). The `degraded` reason string is explicit -- no silent truncation. Return HTTP 200 with the degraded body (the graph is still browsable, just incomplete).

- **Contrast with upload.go 503:** The upload 503 + `Retry-After` is for a transient backend failure that will recover. The graph degradation is a size-cap exceeded condition -- return 200 with the partial result and the explicit reason. Both use the `graphResponseError` envelope for the error message.
- **Key code to copy from:**
  - `Retry-After` header on backend unavailable: `internal/server/upload.go:252`
  - `graphResponseError` envelope (for the `degraded` reason): `internal/server/knowledge.go:66-68`
  - `isBackendUnavailable` helper: `internal/server/upload.go:436-449`

**Phase 28 reuse:** adapted verbatim -- 200 with explicit `degraded` reason field instead of 503 with `Retry-After`.

---

## Pattern 8: FE canvas upgrade

- **Closest analog:** `web/src/components/graph/GraphCanvas.tsx` -- the existing radial layout (lines 64-93), SVG rendering (lines 127-195), and the `PositionedTier`/`PositionedDoc` interfaces (lines 45-60)
- **Adapt:** Extend `GraphCanvas` to render cross-doc edges in addition to the existing tier->doc hierarchy edges. The new edge rendering should:
  1. Accept an additional `edges: Edge[]` prop alongside the existing `graph: KnowledgeGraph`
  2. Render cross-doc edges as curved lines (different stroke style from the existing straight tier->doc lines)
  3. Show edge labels (e.g., "co_cited", "version_chain", "cross_tier") on hover
  4. Implement click-to-expand-neighbor: clicking a cross-doc edge fetches `GET /api/knowledge/graph/neighbors?chunk_id=X` and highlights the neighbor node
- **TierBadge pattern:** The existing `TierLabel` component (lines 205-207) uses `useTierLabel` hook for registry-driven display names. Edge labels should follow the same registry-driven pattern.
- **Deterministic layout:** The existing radial layout is pure SVG + trigonometry with zero dependencies. Keep it -- the cross-doc edges render on top of the same layout.
- **Key code to copy from:**
  - `GraphCanvas` component + radial layout function: `web/src/components/graph/GraphCanvas.tsx:64-201`
  - `PositionedTier`/`PositionedDoc` interfaces: `web/src/components/graph/GraphCanvas.tsx:45-60`
  - `TIER_COLOR` palette keyed by slug: `web/src/components/graph/GraphCanvas.tsx:34-43`
  - `tierColor` helper: `web/src/components/graph/GraphCanvas.tsx:41-43`
  - `setCenterView` dispatch on click: `web/src/components/graph/GraphCanvas.tsx:120-123`
  - `TierLabel` component using `useTierLabel`: `web/src/components/graph/GraphCanvas.tsx:205-207`

**Phase 28 reuse:** adapted verbatim -- extend the existing component; reuse the radial layout, the slug-based color palette, and the `setCenterView` dispatch pattern.

---

## Pattern 9: FE neighbor API consumption

- **Closest analog:** `web/src/components/inspector/InspectorRetrieval.tsx` -- the `handleClick` function (lines 50-59) that dispatches `setCenterView` on chunk ID click
- **Adapt:** `InspectorRetrieval` (or a new `GraphNeighbors` component) calls `GET /api/knowledge/graph/neighbors?chunk_id=X` when a citation is clicked in the graph view. The response renders a "cited alongside" section showing neighbor chunk IDs, edge types, tier, and source.
- **Async fetch pattern:** The existing `InspectorRetrieval` uses `useEffect` + `firstSlugByRole` async resolution (lines 37-46). Follow the same pattern for the neighbors fetch.
- **Chunk ID handling:** The existing `parseLenientChunkID` helper on the BE handles各种 LLM-emitted formats (`CHUNK 7`, `ref-7`, `[ref-7]`). The FE should normalize chunk IDs before calling the API.
- **Key code to copy from:**
  - `handleClick` dispatch pattern: `web/src/components/inspector/InspectorRetrieval.tsx:50-59`
  - `useEffect` + async registry resolution: `web/src/components/inspector/InspectorRetrieval.tsx:37-46`
  - `useAppStore.getState()` for store dispatch: `web/src/components/inspector/InspectorRetrieval.tsx:51`
  - `setCenterView` signature for chunk-source: `web/src/components/inspector/InspectorRetrieval.tsx:52-58`

**Phase 28 reuse:** adapted verbatim -- same `setCenterView` dispatch pattern; same `useEffect` + async fetch pattern for the neighbors API call.

---

## Pattern 10: cross-phase invalidation hook (Phase 27 -> 28)

- **Closest analog:** `internal/knowledge/indexer.go` -- `SetIndexAuditHook` + `indexAuditHook` package var (lines 362-374) and the boot wiring in `main.go:356-357`
- **Second analog:** `internal/server/upload.go` -- `SetDocUploadAuditHook` + `docUploadAuditHook` package var (lines 67-72) and the boot wiring in `main.go:766-776`
- **Reuse verbatim:** Phase 28's graph cache invalidation hook follows the EXACT same callback-seam pattern. Add to `internal/knowledge/graph.go`:

```go
var graphCacheInvalidateHook func()

func SetGraphCacheInvalidateHook(fn func()) { graphCacheInvalidateHook = fn }
```

Then in `main.go`, after `server.SetDocUploadAuditHook(...)` (line 766), add:

```go
server.SetGraphCacheInvalidateHook(func() {
    // invalidate the graph cache
})
```

The Phase 27 upload handler already fires `docUploadAuditHook(actionDocUploaded, ...)` on successful upload (upload.go:295). Phase 28's invalidation hook is called from the same site -- no new audit event needed, just a second hook bound at boot.

- **Key code to copy from:**
  - `indexAuditHook` package var declaration: `internal/knowledge/indexer.go:367`
  - `SetIndexAuditHook` setter: `internal/knowledge/indexer.go:374`
  - `docUploadAuditHook` package var + setter: `internal/server/upload.go:67-72`
  - `main.go` boot wiring for `SetDocUploadAuditHook`: `cmd/server/main.go:766-776`
  - `main.go` boot wiring for `SetIndexAuditHook`: `cmd/server/main.go:356-357`

**Phase 28 reuse:** verbatim -- the callback-seam + boot-wiring pattern is already established; follow it exactly.

---

## No Analog Found

No files in the Phase 28 scope lack a close existing analog. All 10 patterns have concrete source files and line numbers above.

---

## Metadata

**Analog search scope:**
- `internal/knowledge/` -- graph builder, indexer, walker
- `internal/server/` -- knowledge HTTP handlers, upload handler, pipeline
- `internal/retriever/` -- Document, Hit, DocumentMeta types
- `internal/store/sqlite/` -- audit row shape, PutAudit, QueryAudit
- `internal/runtime/` -- trust_stage.go (citability gate)
- `web/src/components/graph/` -- GraphCanvas, GraphTree
- `web/src/components/inspector/` -- InspectorRetrieval
- `cmd/server/main.go` -- env var wiring, Deps assembly, hook boot wiring

**Files scanned:** 12 source files read in full; 2 additional files (main.go, upload.go) scanned for patterns.

**Pattern extraction date:** 2026-09-07

**Key invariants to preserve:**
1. Phase 26's three-state walker (active/deactivated/unregistered) -- `walk.go:123-131`
2. Phase 26's tier re-check in `GraphBuilder.Build` -- `graph.go:216-225`
3. Phase 26's KBRegistry-driven node set (active slugs ascending) -- `graph.go:258-265`
4. Phase 27's `action_doc_uploaded` audit event as the invalidation trigger site -- `upload.go:295`
5. The W-3 error masking convention on all `/api/knowledge/*` handlers -- `knowledge.go:174-177`
6. The citability gate path (`kb.Citable`) from `trust_stage.go` -- `runtime/trust_stage.go:248-257`
