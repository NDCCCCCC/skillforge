# Phase 28 Plan Outline: Dynamic Knowledge Graph

**Phase:** 28 — 知识图谱动态化 + 关系层
**Plans:** 7 plans in 4 waves
**Depends on:** Phase 26 (KBRegistry-driven node set), Phase 27 (upload pipeline)
**Requirements:** GRF-01, GRF-02, GRF-03, GRF-04, GRF-05

---

## Wave Structure

| Wave | Plans | Contents | Depends On |
|------|-------|---------|-----------|
| 1 | 28-01 | Edge computation engine (graph_edges.go + tests) | — |
| 2 | 28-02 | API extension: neighbors + path handlers | 28-01 |
| 2 | 28-03 | TTL cache + cross-phase invalidation hooks | 28-01 |
| 2 | 28-04 | Config env vars: MAX_NODES/EDGES/DEPTH/HOPS | 28-02, 28-03 |
| 3 | 28-05 | FE GraphCanvas edge rendering + useGraphNeighbors hook | 28-02 |
| 3 | 28-06 | InspectorRetrieval neighbors panel integration | 28-05 |
| 4 | 28-07 | Hygiene gate + STRIDE + UAT + joint acceptance | ALL |

**Parallelization:** Wave 2 has 3 plans (28-02, 28-03, 28-04) that are pairwise dependent on 28-01 but independent of each other — they can run in parallel after 28-01 completes. Wave 3 plans (28-05, 28-06) are sequential (28-06 depends on 28-05).

---

## Dependency Graph

```
28-01 (Wave 1)
  ├── 28-02 (Wave 2): neighbors/path API
  ├── 28-03 (Wave 2): TTL cache + hooks
  └── 28-04 (Wave 2): config env vars
        │
        │  (28-04 also depends on 28-02 and 28-03)
        │
        └── 28-05 (Wave 3): FE GraphCanvas
              └── 28-06 (Wave 3): InspectorRetrieval integration
                    │
                    └── 28-07 (Wave 4): hygiene gate + UAT
```

---

## Plan Summary

| Plan | Objective | Tasks | Files Modified |
|------|-----------|-------|---------------|
| 28-01 | Edge computation engine | 2 | internal/knowledge/graph_edges.go, graph_edges_test.go |
| 28-02 | API extension: neighbors + path | 2 | internal/server/knowledge.go, knowledge_test.go, server.go |
| 28-03 | TTL cache + invalidation | 2 | internal/knowledge/graph_cache.go, graph_cache_test.go |
| 28-04 | Config env vars | 3 | cmd/server/main.go, server.go, knowledge.go |
| 28-05 | FE GraphCanvas + hook | 4 | web/src/components/graph/GraphCanvas.tsx, web/src/hooks/useGraphNeighbors.ts, web/src/types/knowledge.ts, NeighborExpandButton.tsx |
| 28-06 | InspectorRetrieval integration | 2 | web/src/components/inspector/InspectorRetrieval.tsx, InspectorRetrieval.test.tsx |
| 28-07 | Hygiene gate + UAT | 4 | scripts/verify-phase-28.sh, STRIDE-28.md, 28-UAT.md, Makefile, README.md |

**Total tasks:** 19
**Total files touched:** 18 new/modified across Go + TypeScript + Shell + Markdown

---

## Requirements Coverage

| Requirement | Plans | Status |
|-------------|-------|--------|
| GRF-01 (N KB nodes from registry) | 28-01, 28-04 | Plan 28-04: GraphBuilder.Build uses reg.List() — N dynamic |
| GRF-02 (deterministic edges) | 28-01 | 4 edge types: section_adjacent, version_chain, keyword_cooccur, citation_cooccur |
| GRF-03 (neighbors + path APIs) | 28-02 | neighborsHandler + pathHandler registered in server.go |
| GRF-04 (FE GraphCanvas upgrade) | 28-05, 28-06 | Edge rendering + neighbor expansion + InspectorRetrieval panel |
| GRF-05 (config-driven limits + degraded) | 28-03, 28-04 | 4 env vars + TTL cache + degraded responses |

---

## Key Decisions (Locked)

| ID | Decision | Rationale |
|----|----------|-----------|
| D-28-01-JACCARD | SKILLFORGE_GRAPH_JACCARD_THRESHOLD=0.3 | 0.3 captures same-topic docs without hairball |
| D-28-01-NODEID | chunk_id stays decimal string "7" | Matches [ref-N] citation format; tier/source in nodes[].tier |
| D-28-02-AUTH | neighbors/path are public read | Metadata only; admin token gates mutations |
| D-28-03-TTL | SKILLFORGE_GRAPH_CACHE_TTL_MS=5000ms | 5s: survives burst queries; new upload visible in 5-10s |
| D-28-03-CITATION-TTL | Citation sub-cache TTL=30s | Audit INSERT-only; advisory edges, not trust boundary |
| D-28-04-MAXNODES | SKILLFORGE_GRAPH_MAX_NODES=50 | ~48 nodes typical; SVG radial layout readable up to ~50 |
| D-28-04-MAXEDGES | SKILLFORGE_GRAPH_MAX_EDGES=200 | 200 = safe headroom before SVG clutter |
| D-28-04-DEPTH | SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT=2 | BFS depth 2 can reach 50-200 nodes; depth 3 = explosion |
| D-28-04-HOPS | SKILLFORGE_GRAPH_PATH_HOPS_LIMIT=4 | 4 hops covers most regulatory chains |
| D-28-05-JACCARD-ENV | Jaccard threshold env-var driven (BE only) | FE only renders; no Jaccard logic client-side |
| D-28-06-PANEL | Neighbors panel inline below chunk content | Minimal UX disruption; keeps citation context visible |
| D-28-07-PHASELOCK | All 7 plans before Phase 29 | GRF-01..05 all-or-nothing; Phase 29 depends on stable graph API |

---

## Phase 27 Joint Acceptance

Phase 28 closes with a joint acceptance test (from research §6):

1. Upload a new document via Phase 27 upload endpoint
2. GET /api/knowledge/graph shows new doc under the correct tier
3. GET /api/knowledge/graph/neighbors?chunk_id=<new>&depth=1 returns section-adjacent edges
4. Chat query citing the uploaded doc produces a verdict with [ref-N] pointing to the new chunk

Step 4 requires a live LLM endpoint; if unavailable, step 3 is sufficient for manual verification.

---

## Out of Scope (Deferred)

- **LLM entity extraction for graph edges** — PROJECT.md v1.2 Out of Scope; future candidate with eval gate
- **Graph stats API** (GET /api/knowledge/graph/stats) — Phase 28-03 suggests it; not required by GRF-03
- **Knowledge-base RBAC / multi-tenant** — v2
- **Per-chunk pagination** (large corpus chunk listing) — GRF-05 limit is on nodes returned, not on corpus enumeration

---

## Self-Discipline Checks

- Zero LLM calls anywhere in Phase 28 code
- Zero new Go packages (stdlib only)
- Zero new npm packages (existing Vite/React/Tailwind/shadcn only)
- No new sqlite table for graph metadata (derived from DocumentMeta + audit rows)
- Node ID format unchanged (decimal string "7")
- Citation co-occurrence edges: optional enhancement, degrade gracefully on audit scan failure
