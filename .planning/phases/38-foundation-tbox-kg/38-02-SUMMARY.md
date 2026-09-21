---
plan: 38-02
title: internal/ontology/store — memory-map KG + JSON snapshot + debounce
status: complete
commit: 568b819
requirements: [ONT-06, ONT-07, ONT-08, ONT-09]
---

# Plan 38-02 Summary — Memory-map KG Store

## What Was Built

- `internal/ontology/store/store.go` — 纯 Go 三索引 KG store (spike 005d-b 生产化):
  - `Insert` / `Len` / `Query(subj, pred, obj)` 模式匹配 (最稀疏索引驱动 candidates)
  - `Adjacent1` / `Adjacent1Reverse` / `Adjacent2` / `Adjacent2Reverse` 双向遍历 (ONT-08)
  - `Snapshot` / `LoadSnapshot` JSON roundtrip (ONT-07)
  - `WriteSnapshot` 原子写 (temp + rename, 崩溃安全)
  - `SnapshotDebounced(path, delay)` — 静默期后单次落盘, 高频插入自动合并 (D-KG-03)
  - `LoadFromTurtle` — emitter TTL → store 桥接
  - `Stats()` — triple_count / byS / byP / byO / memory 估算 (admin 面)
  - `sync.RWMutex` 全程保护; 无 cgo (ONT-09)

## Tests

```
12/12 PASS — 含:
TestStoreAdjacent1Reverse   (反向 1-hop)
TestStoreAdjacent2Reverse   (反向 2-hop)
TestStoreSnapshotRoundtrip  (lossless + query 等价)
TestStoreLoadSpeed          (215 triples < 100ms)
TestStoreWriteSnapshotAtomic
TestStoreDebounceBehavior   (10 触发 → 1 次写, 最终状态完整)
TestStoreStats
```

## Deviations from Plan

- 无实质偏差。`Query` 增加了最稀疏索引选择 (spike 是固定顺序), 性能更稳。

## Self-Check: PASSED

- [x] store.go + store_test.go 落盘, `go test ./internal/ontology/store/` 全绿
- [x] `go vet` 通过
- [x] ONT-06/07/08/09 全覆盖
