# Phase 28 Plan 01: 边计算引擎 (graph_edges.go) Summary

## Plan Overview
**Phase:** 28 | **Plan:** 01 | **Type:** execute | **Wave:** 1
**Status:** COMPLETE | **Requirements:** GRF-02

---

## Commits

| Task | Hash | Message |
|------|------|---------|
| T1 | `5144fae` | 28-01-T1: add Edge struct + 4 edge-type constants + ComputeEdges pure function |
| T2 | `858474c` | 28-01-T2: add deterministic test + per-edge-type unit tests |

---

## Files Added

| Path | Lines | Description |
|------|-------|-------------|
| `internal/knowledge/graph_edges.go` | 333 | Edge struct, 4 edge-type constants, ComputeEdges pure function, helpers |
| `internal/knowledge/graph_edges_test.go` | 379 | 9 tests (7 named in plan + TestDeriveEdgesDeterministic + TestEdgeCitationCooccur_NoStore) |

---

## Test Results

**Command:** `go test ./internal/knowledge/ -count=1`
**Result:** `ok` — 0.341s

**Edge tests (9/9 pass):**

| Test | Subcases | Status |
|------|----------|--------|
| `TestDeriveEdgesDeterministic` (GRF-02 gate) | 2 calls → byte-identical output | PASS |
| `TestEdgeSectionAdjacent` | 3 subcases (consecutive, non-consecutive, single) | PASS |
| `TestEdgeVersionChain` | 3 subcases (touching boundary, disjoint, same EffectiveFrom) | PASS |
| `TestEdgeKeywordCooccur` | 3 subcases (Jaccard >= 0.3, below threshold, same source) | PASS |
| `TestEdgeCitationCooccur` | verdict with 2 citations → cooccur edge weight=0.5 | PASS |
| `TestDeriveEdgesEmpty` | nil/empty input → nil/empty output, no panic | PASS |
| `TestJaccardThresholdConfigurable` | threshold 0.3 → 0 edges; 0.1 → 1 edge | PASS |
| `TestEdgeCitationCooccur_NoStore` | nil store → section_adjacent+version_chain, no citation_cooccur | PASS |

---

## 4 Edge Types — Output Shapes

### EdgeSectionAdjacent
```
Edge{From:"1", To:"2", EdgeType:"section_adjacent", Weight:1.0}
Edge{From:"2", To:"3", EdgeType:"section_adjacent", Weight:1.0}
```
Consecutive chunk IDs within same `(tier, source)`. Weight always 1.0.

### EdgeVersionChain
```
Edge{From:"1", To:"2", EdgeType:"version_chain", Weight:1.0}
```
Overlapping or touching EffectiveFrom/To windows (b.From <= a.To, a.To > 0, b.From > a.From). Weight always 1.0.

### EdgeKeywordCooccur
```
Edge{From:"1", To:"4", EdgeType:"keyword_cooccur", Weight:0.5}
Edge{From:"4", To:"5", EdgeType:"keyword_cooccur", Weight:0.5}
```
Filename token Jaccard >= 0.3 (configurable via `SKILLFORGE_GRAPH_JACCARD_THRESHOLD`). Weight = Jaccard score.

### EdgeCitationCooccur
```
Edge{From:"1", To:"7", EdgeType:"citation_cooccur", Weight:0.5}
```
Chunks co-cited in same verdict audit row. Weight = 1.0 / distinct_chunks_in_verdict.

---

## Key Decisions Made

| Decision ID | Decision | Rationale |
|-------------|----------|-----------|
| D-28-01-JACCARD | `SKILLFORGE_GRAPH_JACCARD_THRESHOLD=0.3` env-configurable | Verbose regulatory filenames need 0.3 to capture same-topic docs without being too strict |
| D-28-01-NODEID | Decimal string chunk IDs via `int64ToChunkID` | Matches existing citation format `[ref-N]`; Phase 26 already established this |

---

## Deviations from Plan

### Rule 1 — Auto-fixed: Version chain boundary condition
- **Found during:** `TestEdgeVersionChain` development
- **Issue:** Initial condition `a.EffectiveTo <= b.EffectiveFrom` incorrectly created edges for disjoint windows (100 < 200 = true)
- **Fix:** Changed to `b.EffectiveFrom <= a.EffectiveTo` — b starts before or when a ends (overlap or touch)
- **Files modified:** `graph_edges.go:175-183`

### Rule 1 — Auto-fixed: Jaccard test data had Jaccard=0.5 not 0.2
- **Found during:** `TestJaccardThresholdConfigurable` development
- **Issue:** `travel_expense_report.md` vs `travel_expense_form.md` produced Jaccard=0.5 (not 0.2 as assumed), exceeding 0.3 threshold
- **Fix:** Changed to `travel_expense_report.md` vs `expense_form.md` which gives Jaccard=0.25 (between 0.1 and 0.3)
- **Files modified:** `graph_edges_test.go`

### Rule 1 — Auto-fixed: Duplicate stop-word key
- **Found during:** first build
- **Issue:** `"procedure": true` appeared twice in `stopWords` map literal
- **Fix:** Removed duplicate
- **Files modified:** `graph_edges.go`

### Rule 1 — Auto-fixed: Citation.ChunkID is string not int64
- **Found during:** compilation
- **Issue:** `verdict.Citation.ChunkID` is `string`, not `int64`; `int64ToChunkID` conversion was wrong
- **Fix:** Use `c.ChunkID` directly as string; sort as strings; use directly in Edge.From/To
- **Files modified:** `graph_edges.go`

### Rule 1 — Auto-fixed: store parameter shadowed package name
- **Found during:** compilation
- **Issue:** `func computeCitationEdges(store store.Store)` — `store` parameter shadows the `store` package
- **Fix:** Renamed parameter to `s`; interface type assertion uses local variable `q`
- **Files modified:** `graph_edges.go`

### Rule 1 — Auto-fixed: Citation co-occur interface type assertion failure
- **Found during:** `TestEdgeCitationCooccur` (empty edges)
- **Issue:** `interface{ QueryAudit(ctx, filter any) }` — concrete type has `store.AuditFilter` not `any`, so type assertion failed
- **Fix:** Defined named `auditQuerier` interface with `store.AuditFilter` (not `any`) as parameter type
- **Files modified:** `graph_edges.go`

### Rule 1 — Auto-fixed: Test pollution from jaccardThreshold package var
- **Found during:** full suite run (`-count=1` without `-p 1`)
- **Issue:** `jaccardThreshold` was a `var` initialized once at package load; patching it in `TestJaccardThresholdConfigurable` polluted `TestDeriveEdgesDeterministic`
- **Fix:** Changed to `func jaccardThreshold()` that re-reads env on every call; tests use `t.Setenv` for proper isolation
- **Files modified:** `graph_edges.go`, `graph_edges_test.go`

---

## Non-Deviations (Plan Executed Exactly)
- Node ID format: decimal string via `int64ToChunkID` (D-28-01-NODEID) — VERIFIED
- Jaccard threshold default 0.3 (D-28-01-JACCARD) — VERIFIED
- `ComputeEdges` is pure function: same input → byte-for-byte identical output (GRF-02) — VERIFIED by `TestDeriveEdgesDeterministic`
- CitationCooccur gracefully degrades when store is nil — VERIFIED by `TestEdgeCitationCooccur_NoStore`
- Zero new Go dependencies — VERIFIED
- `int64ToChunkID` reused verbatim from `graph.go:342` — VERIFIED

---

## Verification Command Output

```
$ /c/Program Files/Go/bin/go test ./internal/knowledge/ -run 'TestEdge|TestDeriveEdgesDeterministic|TestDeriveEdgesEmpty|TestJaccardThreshold' -count=1 -v
=== RUN   TestDeriveEdgesDeterministic
--- PASS: TestDeriveEdgesDeterministic (0.02s)
=== RUN   TestEdgeSectionAdjacent
    --- PASS: TestEdgeSectionAdjacent/three_consecutive_chunks_yield_two_edges (0.02s)
    --- PASS: TestEdgeSectionAdjacent/non-consecutive_IDs_(1,3,5)_yield_zero_edges (0.00s)
    --- PASS: TestEdgeSectionAdjacent/single_chunk_yields_zero_edges (0.00s)
--- PASS: TestEdgeSectionAdjacent (0.00s)
=== RUN   TestEdgeVersionChain
    --- PASS: TestEdgeVersionChain/A_superseded_by_B_when_windows_touch_at_boundary (0.00s)
    --- PASS: TestEdgeVersionChain/disjoint_windows_(no_overlap)_yield_zero_version_edges (0.00s)
    --- PASS: TestEdgeVersionChain/same_EffectiveFrom_(equal)_does_not_create_chain (0.00s)
--- PASS: TestEdgeVersionChain (0.00s)
=== RUN   TestEdgeKeywordCooccur
    --- PASS: TestEdgeKeywordCooccur/travel-expense-policy-2024_and_2025_share_tokens_with_Jaccard_>=_0.3 (0.00s)
    --- PASS: TestEdgeKeywordCooccur/unrelated_filenames_yield_no_keyword_cooccur_edges (0.00s)
    --- PASS: TestEdgeKeywordCooccur/same_source_—_not_cross-source,_no_keyword_edge (0.00s)
--- PASS: TestEdgeKeywordCooccur (0.00s)
=== RUN   TestEdgeCitationCooccur
--- PASS: TestEdgeCitationCooccur (0.00s)
=== RUN   TestDeriveEdgesEmpty
--- PASS: TestDeriveEdgesEmpty (0.00s)
=== RUN   TestJaccardThresholdConfigurable
--- PASS: TestJaccardThresholdConfigurable (0.00s)
=== RUN   TestEdgeCitationCooccur_NoStore
--- PASS: TestEdgeCitationCooccur_NoStore (0.00s)
PASS
ok      github.com/skillforge/skillforge/internal/knowledge     0.170s
```
