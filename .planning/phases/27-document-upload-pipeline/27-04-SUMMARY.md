# Phase 27 Plan 04: Dual Retriever Backend Adapter — COMPLETE

**Plan:** 27-04
**Phase:** 27 (Document Upload Pipeline)
**Tasks:** 6/6 complete
**Status:** COMPLETE
**Commits:** `f19c26a`, `71d9549`, `926437c`, `1d7c844`, `5a9fbd5`, `a4f498a`
**Completed:** 2026-09-07

---

## Executive Summary

Implemented the dual IndexerBackend adapter layer and GetMaxID real implementations, completing the upload pipeline infrastructure (DOC-02, DOC-05). Milvus and MongoDB backends now both expose a narrow `Insert`-only interface for the upload handler, and both implement `GetMaxID` on their concrete retriever types per the Phase 27/02 contract.

---

## Task Results

### Task 1: IndexerBackend interface — COMPLETE

**Commit:** `f19c26a`
**Files:** `internal/retriever/backend.go` (36 lines)

Defined `IndexerBackend` interface with a single `Insert(ctx, []Document) error` method. Includes `IndexerBackendNotWiredError` constant for 503 responses.

**Key design:** The interface lives in the `retriever` package (same as `Retriever`) to keep `Document` type visibility clean without an extra package.

---

### Task 2: MilvusIndexerBackend adapter — COMPLETE

**Commit:** `71d9549`
**Files:** `internal/retriever/milvus/backend.go` (41 lines), `internal/retriever/milvus/backend_test.go` (77 lines)

`MilvusIndexerBackend` wraps `*MilvusClient` and delegates `Insert` to it. Compile-time assertion mirrors `kb.Store` pattern from Phase 26.

Two tests: `TestMilvusIndexerBackendInsert` (call forwarding), `TestMilvusIndexerBackendInsertError` (error propagation). Both pass.

---

### Task 3: MongoIndexerBackend adapter — COMPLETE

**Commit:** `926437c`
**Files:** `internal/retriever/mongo/backend.go` (42 lines), `internal/retriever/mongo/backend_test.go` (79 lines)

`MongoIndexerBackend` wraps `*Client` and delegates `Insert` to it. Compile-time assertion same pattern as Milvus.

Two tests: `TestMongoIndexerBackendInsert` (call forwarding), `TestMongoIndexerBackendInsertError` (error propagation). Both pass.

---

### Task 4: Wire IndexerBackend in main.go — COMPLETE

**Commit:** `1d7c844`
**Files:** `cmd/server/main.go` (+14/-5), `internal/server/server.go` (-9/+4)

After `buildRetriever` returns, a switch on `RETRIEVER_BACKEND` constructs the narrow adapter:

```go
var indexerBackend retriever.IndexerBackend
switch backend {
case "mongo":
    indexerBackend = mongo.NewIndexerBackend(r.(*mongo.Client))
default:
    indexerBackend = milvus.NewIndexerBackend(r.(*milvus.MilvusClient))
}
```

`server.go`: removed the inline stub `IndexerBackend` interface declaration, replacing it with `retriever.IndexerBackend` (now resolved from `internal/retriever/backend.go`).

**Cascade note:** `IndexerBackend: r` (where `r` was the Retriever) is replaced with `IndexerBackend: indexerBackend` (the narrow adapter). Both `MilvusClient` and `Client` satisfy `IndexerBackend` since they both implement `Insert`.

**Build note:** `cmd/server` requires WSL (DuckDB/CGO linker issue documented in CLAUDE.md). `internal/retriever` and `internal/server` both compile cleanly on Windows.

---

### Task 5: MilvusClient.GetMaxID implementation — COMPLETE

**Commit:** `5a9fbd5`
**Files:** `internal/retriever/milvus/client.go` (+36/-6), `internal/retriever/milvus/get_max_id_test.go` (152 lines)

Replaced the stub with a real implementation:

```go
func (m *MilvusClient) GetMaxID(ctx context.Context, tier string) (int64, error) {
    if m == nil || m.cfg.CollectionName == "" {
        return 0, nil
    }
    expr := fmt.Sprintf(`tier == "%s"`, tier)
    rs, err := m.mc.Query(ctx, m.cfg.CollectionName, nil, expr, []string{"id"}, client.WithLimit(10000))
    if err != nil {
        return 0, fmt.Errorf("milvus.GetMaxID: Query: %w", err)
    }
    var maxID int64
    for _, col := range rs {
        if col.Name() != "id" { continue }
        for i := 0; i < col.Len(); i++ {
            if v, e := col.GetAsInt64(i); e == nil && v > maxID { maxID = v }
        }
    }
    return maxID, nil
}
```

- Uses Milvus Query API with `tier == "$tier"` expression, limit 10000
- Client-side scan for max ID (Milvus Query has no ORDER BY)
- Returns 0 on empty result (no error)

Three tests (all pass):

| Test | Input | Expected |
|------|-------|----------|
| `TestMilvusClientGetMaxIDEmpty` | Empty result | `(0, nil)` |
| `TestMilvusClientGetMaxIDPopulated` | Rows `{1,2,3,4,5}` | `(5, nil)` |
| `TestMilvusClientGetMaxIDUnknownTier` | Unknown tier | `(0, nil)` |

Hermetic mock: `getMaxIDMockClient` embeds `client.Client` interface, overrides `Query` to return configurable `client.ResultSet` slices.

---

### Task 6: MongoClient.GetMaxID implementation — COMPLETE

**Commit:** `a4f498a`
**Files:** `internal/retriever/mongo/mongo.go` (+60/-4), `internal/retriever/mongo/get_max_id_test.go` (175 lines)

Replaced the stub with a real aggregation pipeline implementation:

```go
func (c *Client) GetMaxID(ctx context.Context, tier string) (int64, error) {
    if c == nil || c.agg == nil {
        return 0, nil
    }
    pipeline := mongo.Pipeline{
        {{Key: "$match", Value: bson.M{"tier": tier}}},
        {{Key: "$sort", Value: bson.M{"id": -1}}},
        {{Key: "$limit", Value: 1}},
    }
    cur, err := c.agg.Aggregate(ctx, pipeline)
    if err != nil { return 0, fmt.Errorf("mongo.GetMaxID: Aggregate: %w", err) }
    var rows []struct{ ID int64 `bson:"id"` }
    if err := cur.All(ctx, &rows); err != nil { return 0, fmt.Errorf("mongo.GetMaxID: cursor.All: %w", err) }
    if len(rows) == 0 { return 0, nil }
    return rows[0].ID, nil
}
```

**Aggregator seam (Rule 2 auto-fix):** Added `aggregator` interface + `aggregatorCursor` interface + `mongoCollectionAggregator` adapter (mirrors the existing `rowFinder` pattern). `New()` wires `agg: mongoCollectionAggregator{coll: coll}`. Hermetic tests inject `fakeAggregator`.

Three tests (all pass):

| Test | Input | Expected |
|------|-------|----------|
| `TestMongoClientGetMaxIDEmpty` | Empty result | `(0, nil)` |
| `TestMongoClientGetMaxIDPopulated` | Sorted rows `{5,4,3,2,1}` | `(5, nil)` |
| `TestMongoClientGetMaxIDUnknownTier` | Unknown tier | `(0, nil)` |

Mock returns pre-sorted rows `{5}` reflecting the `$sort: {id: -1}` + `$limit: 1` pipeline behavior in production MongoDB.

---

## Files Added/Modified

| Path | Lines | Delta | Description |
|------|-------|-------|-------------|
| `internal/retriever/backend.go` | 36 | +36 | IndexerBackend interface + IndexerBackendNotWiredError |
| `internal/retriever/milvus/backend.go` | 41 | +41 | MilvusIndexerBackend struct + Insert + NewIndexerBackend |
| `internal/retriever/milvus/backend_test.go` | 77 | +77 | 2 tests: call forwarding + error propagation |
| `internal/retriever/milvus/client.go` | +36/-6 | +36/-6 | GetMaxID real impl (query + client-side max scan) |
| `internal/retriever/milvus/get_max_id_test.go` | 152 | +152 | 3 tests: empty/populated/unknown-tier |
| `internal/retriever/mongo/backend.go` | 42 | +42 | MongoIndexerBackend struct + Insert + NewIndexerBackend |
| `internal/retriever/mongo/backend_test.go` | 79 | +79 | 2 tests: call forwarding + error propagation |
| `internal/retriever/mongo/mongo.go` | +60/-4 | +60/-4 | GetMaxID impl + aggregator/aggregatorCursor interfaces + mongoCollectionAggregator |
| `internal/retriever/mongo/get_max_id_test.go` | 175 | +175 | 3 tests: empty/populated/unknown-tier |
| `cmd/server/main.go` | | +14/-5 | RETRIEVER_BACKEND switch wires IndexerBackend adapter |
| `internal/server/server.go` | | -9/+4 | Remove inline stub; use retriever.IndexerBackend |

---

## Deviations from Plan

### Deviation 1: Nil check guard uses `c.agg == nil` (not `c.coll == nil`) for Mongo GetMaxID

**Plan said:** Guard should check the collection handle.

**Found:** The `Client.coll` field is `*mongo.Collection` (concrete type), which is `nil` when the client is constructed in tests without a live connection. Using `c.agg == nil` correctly gates only the production path, since `agg` is always non-nil in production (wired in `New()`).

**Fix:** Changed nil guard from `c.coll == nil` to `c.agg == nil`. This is a correctness fix, not a deviation.

---

## Cascade Fixes Applied

### 1. Aggregator seam for hermetic MongoDB testing

**Problem:** `Client.coll` is `*mongo.Collection` (concrete), preventing test injection without a live MongoDB.

**Fix:** Added `aggregator` interface + `aggregatorCursor` interface + `mongoCollectionAggregator` adapter, following the existing `rowFinder` pattern. `New()` wires it; tests inject `fakeAggregator`.

### 2. `getMaxIDMockClient` embeds `client.Client` interface

**Problem:** The full Milvus SDK `client.Client` interface has 50+ methods.

**Fix:** Embed the interface and only override `Query`. The same pattern was already established in `client_recreate_test.go` with `recreateMockClient`.

---

## Verification

**Hermetic gate:**
```
go build ./internal/retriever/...  → OK
go build ./internal/server/...     → OK
go test ./internal/retriever/milvus/... → ok
go test ./internal/retriever/mongo/...  → ok
```

**Build note:** `cmd/server` requires WSL (pre-existing DuckDB/CGO linker issue, documented in CLAUDE.md). Both `internal/retriever` and `internal/server` compile cleanly on Windows.

**Test counts:**

| Package | Tests | Result |
|---------|-------|--------|
| `internal/retriever/milvus` | `TestMilvusIndexerBackendInsert`, `TestMilvusIndexerBackendInsertError`, `TestMilvusClientGetMaxIDEmpty`, `TestMilvusClientGetMaxIDPopulated`, `TestMilvusClientGetMaxIDUnknownTier` | 5/5 PASS |
| `internal/retriever/mongo` | `TestMongoIndexerBackendInsert`, `TestMongoIndexerBackendInsertError`, `TestMongoClientGetMaxIDEmpty`, `TestMongoClientGetMaxIDPopulated`, `TestMongoClientGetMaxIDUnknownTier` | 5/5 PASS |

---

## Commits

```
a4f498a feat(27-04-T6): MongoClient.GetMaxID impl + tests + aggregator seam
5a9fbd5 feat(27-04-T5): MilvusClient.GetMaxID impl + 3 hermetic tests
1d7c844 feat(27-04-T4): wire IndexerBackend in main.go via RETRIEVER_BACKEND switch
926437c feat(27-04-T3): MongoIndexerBackend adapter + tests
71d9549 feat(27-04-T2): MilvusIndexerBackend adapter + tests
f19c26a feat(27-04-T1): define IndexerBackend interface in internal/retriever/backend.go
```

All commits properly attributed with `Co-Authored-By: Claude <noreply@anthropic.com>`.

---

## Self-Check

- [x] `IndexerBackend` interface has exactly 1 method: `Insert(ctx, []Document) error`
- [x] `MilvusIndexerBackend.Insert` delegates to `MilvusClient.Insert`
- [x] `MongoIndexerBackend.Insert` delegates to `Client.Insert`
- [x] Both adapters have compile-time assertions: `var _ retriever.IndexerBackend = (*XxxIndexerBackend)(nil)`
- [x] `main.go` switch covers both `milvus` and `mongo` backends
- [x] `MilvusClient.GetMaxID` uses Query with `tier == "$tier"` expression + client-side max scan
- [x] `MongoClient.GetMaxID` uses aggregation pipeline `$match/$sort/$limit`
- [x] Both return 0 on empty result
- [x] Hermetic tests use mocks (no live Milvus/Mongo required)
- [x] No TODO/FIXME in new files
- [x] All 10 tests pass
- [x] All 6 commits properly attributed

**Self-Check: PASSED**
