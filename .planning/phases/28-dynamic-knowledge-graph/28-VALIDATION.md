# Phase 28 Validation

**Phase:** 28 — 知识图谱动态化 + 关系层 (Dynamic Knowledge Graph)
**Re-checked:** 2026-09-07 (after iteration-1 fix round)
**Status:** PASS — ready for execute-phase

Iteration-1 changes applied: 28-02 frontmatter `files_modified` corrected
(was `[]`; the plan body clearly requires server.go + knowledge.go +
knowledge_test.go; the PLAN-OUTLINE already listed the same three files).

## Plan Inventory (post-fix)

| Plan | Goals | Files in plan | Status |
|------|-------|---------------|--------|
| 28-01 | GRF-02 | graph_edges.go + graph_edges_test.go (new) | OK |
| 28-02 | GRF-03 | knowledge.go + knowledge_test.go + server.go (extend) | OK |
| 28-03 | GRF-05 | graph_cache.go + graph_cache_test.go (new) | OK |
| 28-04 | GRF-05 | main.go + server.go + graph.go (extend) | OK |
| 28-05 | GRF-04 | GraphCanvas.tsx + useGraphNeighbors.ts + knowledge.ts + NeighborExpandButton.tsx | OK |
| 28-06 | GRF-04 | InspectorRetrieval.tsx + InspectorRetrieval.test.tsx | OK |
| 28-07 | GRF-01..05 | verify-phase-28.sh + STRIDE-28.md + 28-UAT.md + Makefile + README | OK |

## Coverage Matrix

| Requirement | Plans | Test path | Status |
|-------------|-------|-----------|--------|
| GRF-01: N KB nodes from registry | 28-01 + 28-04 | reg.List() in graph.go (unchanged from Phase 26) + Deps.GraphLimits (additive) | ✓ |
| GRF-02: 4 deterministic edge types | 28-01 | 4 edge-type constants + ComputeEdges pure function + determinism test | ✓ |
| GRF-03: neighbors + path APIs | 28-02 | 2 new h.GET routes + route registration in server.go | ✓ |
| GRF-04: FE GraphCanvas upgrade | 28-05 + 28-06 | edge rendering in GraphCanvas + useGraphNeighbors hook + InspectorRetrieval panel | ✓ |
| GRF-05: config-driven limits + degraded | 28-03 + 28-04 | 4 SKILLFORGE_GRAPH_* env vars + sync.Map TTL cache + skeleton-degraded response | ✓ |

## Breaking-Change Call Site Registry

| Call site | Type | Location | Owned by | Status |
|-----------|------|----------|----------|--------|
| ComputeEdges pure function | NEW function | internal/knowledge/graph_edges.go | 28-01 T1 | ✓ OWNED |
| EdgeType constants | NEW type | internal/knowledge/graph_edges.go | 28-01 T1 | ✓ OWNED |
| neighborsHandler | NEW handler | internal/server/knowledge.go | 28-02 T1 | ✓ OWNED |
| pathHandler | NEW handler | internal/server/knowledge.go | 28-02 T1 | ✓ OWNED |
| Route registrations | ADD h.GET | internal/server/server.go | 28-02 T1 | ✓ OWNED |
| GraphCache TTL impl | NEW type | internal/knowledge/graph_cache.go | 28-03 T1 | ✓ OWNED |
| SetGraphCacheInvalidateHook | NEW func | internal/knowledge/graph_cache.go | 28-03 T2 | ✓ OWNED |
| GraphCacheInvalidateHook wiring | ADD main.go line | cmd/server/main.go | 28-04 T1 | ✓ OWNED |
| GraphLimits on Deps | ADD struct field | internal/server/server.go | 28-04 T1 | ✓ OWNED |
| 4 SKILLFORGE_GRAPH_* env parsing | ADD main.go | cmd/server/main.go | 28-04 T1 | ✓ OWNED |
| Degraded response shape | ADD logic | internal/server/knowledge.go | 28-04 T2 | ✓ OWNED |
| useGraphNeighbors hook | NEW FE hook | web/src/hooks/useGraphNeighbors.ts | 28-05 T2 | ✓ OWNED |
| GraphCanvas edge rendering | EXTEND component | web/src/components/graph/GraphCanvas.tsx | 28-05 T1 | ✓ OWNED |
| NeighborExpandButton | NEW component | web/src/components/graph/NeighborExpandButton.tsx | 28-05 T3 | ✓ OWNED |
| InspectorRetrieval neighbors panel | EXTEND component | web/src/components/inspector/InspectorRetrieval.tsx | 28-06 T1 | ✓ OWNED |

All 15 call sites owned at planning time — the same structural risk that
Phase 26 (5 unowned, dbd7f80) and Phase 27 (4 unowned, commit 6b5667c)
had to repair post-execution is closed at planning time here.

## Phase 26/27 Cross-Phase Contract Check

- **Phase 26 contract (KBRegistry-driven node set + 3-state walker):**
  Phase 28's `internal/knowledge/graph_edges.go` does NOT touch
  `WalkRegisteredDirs` — the existing dynamic node set + walker are
  preserved verbatim.

- **Phase 27 contract (no edits to upload.go for cross-phase hooks):**
  Phase 28's cache invalidation is wired via `SetGraphCacheInvalidateHook`
  in `main.go` (28-04 T1, near line 766 where SetDocUploadAuditHook is
  currently bound). NO edits to `internal/server/upload.go` are required
  by any Phase 28 plan — the callback-seam pattern from upload.go:71-72
  is followed verbatim without modifying upload.go itself.

## Decision Inventory (11 decisions)

All 11 decisions are consistent with research and downstream plans respect them.
No duplicates.

| Decision | Plan |
|----------|------|
| D-28-01-JACCARD (0.3) | 28-01 |
| D-28-01-NODEID (decimal string) | 28-01 |
| D-28-02-AUTH (public read) | 28-02 |
| D-28-03-TTL (5s/30s) | 28-03 |
| D-28-04-MAXNODES (50) | 28-04 |
| D-28-04-MAXEDGES (200) | 28-04 |
| D-28-04-DEPTH (2) | 28-04 |
| D-28-04-HOPS (4) | 28-04 |
| D-28-05-JACCARD-ENV (BE only) | 28-05 |
| D-28-06-PANEL (inline below chunk) | 28-06 |
| D-28-07-PHASELOCK (all 7 before Phase 29) | 28-07 |

## Self-Discipline Checks

| Check | Status |
|-------|--------|
| Zero LLM calls in graph edges | ✓ PASS (deterministic algorithms only) |
| Zero new Go packages | ✓ PASS (stdlib: sync, encoding/json, sort, strings) |
| Zero new npm packages | ✓ PASS (existing React/Tailwind/shadcn/Vite) |
| No new sqlite table | ✓ PASS (edges derived from existing DocumentMeta + AuditRow) |
| Node ID format unchanged | ✓ PASS (decimal string matches Phase 26) |
| Citation co-occurrence degrades gracefully | ✓ PASS (skips on store nil or query error) |
| Phase 26 KBRegistry + walker untouched | ✓ PASS (28-01 only adds graph_edges.go) |
| Phase 27 upload.go untouched | ✓ PASS (cross-phase hook via main.go callback seam) |

## Nyquist Validation

| Task | Plan | Wave | Automated? |
|------|------|------|-----------|
| 28-01 T1 | 28-01 | 1 | go test |
| 28-01 T2 | 28-01 | 1 | go test |
| 28-02 T1 | 28-02 | 2 | go test |
| 28-02 T2 | 28-02 | 2 | go test |
| 28-03 T1 | 28-03 | 2 | go test |
| 28-03 T2 | 28-03 | 2 | go test |
| 28-04 T1 | 28-04 | 2 | go build |
| 28-04 T2 | 28-04 | 2 | go build |
| 28-04 T3 | 28-04 | 2 | go test |
| 28-05 T1 | 28-05 | 3 | npm run build |
| 28-05 T2 | 28-05 | 3 | npm test --filter=useGraphNeighbors |
| 28-05 T3 | 28-05 | 3 | npm test --filter=NeighborExpandButton |
| 28-05 T4 | 28-05 | 3 | npm run build |
| 28-06 T1 | 28-06 | 3 | npm run build |
| 28-06 T2 | 28-06 | 3 | npm test --filter=InspectorRetrieval |
| 28-07 T1 | 28-07 | 4 | bash verify-phase-28.sh |
| 28-07 T2 | 28-07 | 4 | grep checks |
| 28-07 T3 | 28-07 | 4 | bash verify-phase-28.sh |
| 28-07 T4 | 28-07 | 4 | Human sign-off (operator runbook) |

19 tasks / 18 automated + 1 human checkpoint. PASS.

## Verdict

**PASS — ready for execute-phase.**

Recommendations for the executor:
1. Wave 1 (`28-01`) runs first — establishes the Edge compute primitive
2. Wave 2 (`28-02`, `28-03`, `28-04`) can run in parallel — no file conflicts
   after iteration-1 added server.go to 28-02's files_modified
3. Wave 3 (`28-05`, `28-06`) runs after Wave 2 — FE consumes the BE endpoints
4. Wave 4 (`28-07`) is the hygiene gate + operator checkpoint; do NOT skip
   the human sign-off (the operator runbook includes the Phase 27 joint
   acceptance test that verifies uploaded docs appear in the graph immediately)
5. The cross-phase cache invalidation hook MUST be wired in main.go
   (not in upload.go) — the callback-seam pattern is the contract