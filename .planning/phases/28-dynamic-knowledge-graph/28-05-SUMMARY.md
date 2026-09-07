# Phase 28 Plan 05: FE GraphCanvas 升级 + useGraphNeighbors hook Summary

## Plan Overview
**Phase:** 28 | **Plan:** 05 | **Type:** execute | **Wave:** 3
**Status:** COMPLETE | **Requirements:** GRF-04

---

## Commits

| Task | Hash | Message |
|------|------|---------|
| T1 | `afc9377` | 28-05-T1: add Edge/NeighborResult/PathResult types to knowledge.ts |
| T2 | `028d3b9` | 28-05-T2: add useGraphNeighbors hook |
| T3 | `2c14387` | 28-05-T3: extend GraphCanvas with edge rendering + neighbor expansion |
| T4 | `1767b1d` | 28-05-T4: add NeighborExpandButton + FE tests |

---

## Files Added/Modified

| Path | Lines | Description |
|------|-------|-------------|
| `web/src/types/knowledge.ts` | +59 | Add EDGE_TYPE_* constants, GraphEdge, EdgeNode, NeighborResult, PathResult, KnowledgeGraph extended with degraded fields |
| `web/src/hooks/useGraphNeighbors.ts` | +147 | useGraphNeighbors hook with (chunkId,depth) cache + pending deduplication + _resetCache test seam |
| `web/src/hooks/__tests__/useGraphNeighbors.test.ts` | +127 | 6 tests: mount fetch, 500 error, network error, cache hit, refetch, enabled=false |
| `web/src/components/graph/GraphCanvas.tsx` | +228/-59 | Add EDGE_COLOR/EDGE_TYPE_LABEL maps, NeighborSubgraph sub-component, overlayEdges + neighborResult props, hover edge tooltip |
| `web/src/components/graph/GraphCanvas.test.tsx` | +93 | 3 new tests: overlay edge SVG lines count, onExpandNeighbors callback |
| `web/src/components/graph/NeighborExpandButton.tsx` | +47 | NeighborExpandButton component: spinner/disabled/onExpand |
| `web/src/components/graph/__tests__/NeighborExpandButton.test.tsx` | +67 | 8 tests: render, loading, disabled, onExpand, aria-label |

---

## API Contract (from Phase 28-02)

**GET /api/knowledge/graph** (updated)
- Now returns `edges: [{from, to, edge_type, weight, tier}]` field + `degraded: string | null`

**GET /api/knowledge/graph/neighbors?chunk_id=X&depth=N**
- Response: `{chunk_id, depth, nodes: [EdgeNode], edges: [GraphEdge], degraded: bool, degraded_reason: string}`

---

## Test Results

**Command:** `npm test -- --run`

```
Test Files  70 passed (70)
Tests       529 passed (529)
Duration    16.54s
```

**New tests (28-05):**
| File | Tests | Status |
|------|-------|--------|
| `useGraphNeighbors.test.ts` | 6 | ALL PASS |
| `NeighborExpandButton.test.tsx` | 8 | ALL PASS |
| `GraphCanvas.test.tsx` (overlay edge tests) | 3 | ALL PASS |

**Existing GraphCanvas tests:** 7/7 still pass (no regression)

---

## Verification Command Output

```bash
$ cd web && npm run build
✓ built in 17.68s  (chunk-size warning is pre-existing)

$ cd web && npm test -- --run
Test Files  70 passed (70)
Tests       529 passed (529)
```

---

## Key Decisions Made

| Decision ID | Decision | Rationale |
|-------------|----------|-----------|
| D-28-05-JACCARD-ENV | SKILLFORGE_GRAPH_JACCARD_THRESHOLD env var passed to BE only; FE has no Jaccard logic | Edge computation is pure BE (Plan 28-01); FE only renders the edge types returned by the API |
| D-28-05-CACHE-MAP | useGraphNeighbors uses a Map keyed by `${chunkId}:${depth}`, not a single global cache | Mirrors the kb-bases pattern but with per-key caching; depth cap from D-28-04-DEPTH=2 |
| D-28-05-NEIGHBOR-MODE | neighborResult prop enables neighbor subgraph mode; overlayEdges renders on radial graph | Two distinct visual modes avoid coordinate mapping complexity per research §7 Risk R2 |

---

## Non-Deviations (Plan Executed Exactly)
- EDGE_TYPE_SECTION_ADJACENT / VERSION_CHAIN / KEYWORDCOOCCUR / CITATIONCOOCCUR constants — VERIFIED
- edgeColor map matches 4 edge types to CSS variables — VERIFIED
- NeighborSubgraph renders center node + ring layout — VERIFIED
- GraphCanvas onExpandNeighbors callback fires with chunkId — VERIFIED
- overlayEdges prop renders SVG lines with edge-type coloring — VERIFIED
- EDGE_TYPE_LABEL Chinese labels (章节相邻, 版本链, 共现, 引用共现) — VERIFIED
- _resetCache test seam for cache isolation across tests — VERIFIED
- No new npm packages introduced — VERIFIED
- Zero TODO/FIXME in any new file — VERIFIED

---

## Deviations from Plan

### Rule 2 — Auto-fixed: neighborResult prop separation
- **Found during:** T3 implementation
- **Issue:** Initial design used `overlayEdges` alone to trigger neighbor mode, but `NeighborSubgraph` needs the full `NeighborResult` to render. A `null as unknown as NeighborResult` cast was crashing at runtime.
- **Fix:** Added a separate `neighborResult?: NeighborResult | null` prop; `isNeighborMode` now checks `Boolean(neighborResult)`; `NeighborSubgraph` receives `neighborResult!` directly. `overlayEdges` renders as overlay lines on the radial graph instead.
- **Files modified:** `web/src/components/graph/GraphCanvas.tsx`
- **Commit:** `2c14387`

### Rule 1 — Auto-fixed: module-level cache pollution in useGraphNeighbors tests
- **Found during:** first T2 test run
- **Issue:** `vi.resetModules()` was called AFTER `renderHook`, so the module-level cache was already populated. The hook returned cached (success) data even for error-path tests, causing `expect(data).toBeNull()` to fail.
- **Fix:** Added `_resetCache()` test seam to `useGraphNeighbors.ts`; `beforeEach` now calls `fresh()` which resets modules AND clears cache before each test.
- **Files modified:** `web/src/hooks/useGraphNeighbors.ts`, `web/src/hooks/__tests__/useGraphNeighbors.test.ts`
- **Commit:** `028d3b9`

---

## Threat Flags

None — only adds new FE types and a public-read fetch hook; no new endpoints, auth paths, or trust-boundary changes.

---

## Known Stubs

None — all new components and hooks are fully wired with real logic; no empty stubs remain.
