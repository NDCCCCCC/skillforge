---
phase: 15-p10-1
plan: 02
subsystem: data

tags: [go, sqlite, migration, audit, datasets, import-batches]

requires:
  - phase: 15-p10-1
    plan: 01
    provides: internal/data pure-function desensitization core (rules_sha256, decode, normalize, desensitize)
  - phase: 14-p10-0-cgo-trust-floor
    provides: internal/store/sqlite scaffold + datasets registry + D14-E1 index-after-ALTER contract

provides:
  - internal/store/sqlite schema migration with datasets 3 additive columns + import_batches + dataset_deletes tables
  - internal/store ImportBatchRow / DatasetDeleteRow types + Store interface extension
  - internal/store/sqlite/import_batches.go INSERT-only batch audit CRUD
  - internal/store/sqlite/datasets.go extended 10-column round-trip + exact-name DeleteDataset
  - DATA-16 idempotency query surface (LatestImportBatch)
  - DATA-17 audit row shape locked by TestBatchAuditRowShape

affects:
  - 15-p10-1 plans 03-07 (batch orchestration, importer, server API, gate)
  - 16-p11 (KPI queries consume datasets registry + k_min_override)
  - 20-p10-1-admin (ADM-02 batch view consumes ListImportBatches)

key-files:
  created:
    - internal/store/sqlite/import_batches.go
    - internal/store/sqlite/import_batches_test.go
    - internal/store/sqlite/datasets_test.go
  modified:
    - internal/store/sqlite/schema.go
    - internal/store/sqlite/schema_test.go
    - internal/store/sqlite/migrate_legacy_test.go
    - internal/store/sqlite/datasets.go
    - internal/store/store.go

tech-stack:
  added: []
  patterns:
    - additive ALTER-gated migration with legacy upgrade test
    - index-after-ALTER ordering contract (D14-E1 generalization)
    - INSERT-only audit table with PK constraint rejecting replays
    - exact-name parameterized DELETE with no LIKE wildcard injection
    - 10-column DatasetRow round-trip preserving old-caller zero values

key-decisions:
  - "import_batches uses a dedicated table (option A from 15-RESEARCH Open Q3) rather than audit.action enum + frozen_args BLOB, keeping batch queries SQL-first for ADM-02"
  - "DeleteDataset uses exact-name `?` parameterized DELETE; prefix deletion is intentionally left to Go-side filtering to avoid `_` LIKE wildcard injection"
  - "KMinOverride=0 means 'use global default KMinDefault' to keep old callers compatible without requiring every import to specify k_min"

patterns-established:
  - "ALTER-gated additive column migration for datasets registry"
  - "INSERT-only batch audit table with batch_id PRIMARY KEY (replays surface as constraint errors)"
  - "Index creation after ALTER loop for columns added by migration"
  - "Exact-name parameterized deletion path with dataset_deletes audit row"

requirements-completed: [DATA-16, DATA-17]

# Metrics
duration: 25min
completed: 2026-08-23
---

# Phase 15 Plan 02: 数据导入管线 — 注册表与审计存储面 Summary

**Additive SQLite schema migration for the datasets registry plus INSERT-only import_batches / dataset_deletes audit tables, with Store interface extensions and parameterized read/write paths for idempotency and deletion audit.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-08-23T16:48:28Z
- **Completed:** 2026-08-23T17:13:28Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Migrated the `datasets` registry with three additive columns (`dataset`, `period`, `k_min_override`) using ALTER-gated idempotent migration, preserving existing Phase 14 rows.
- Created the `import_batches` INSERT-only audit table with 11 columns covering DATA-17 raw-discard proof, D15-A4 rules_sha256, and D15-C1 supersede chain.
- Created the `dataset_deletes` INSERT-only deletion audit table for D15-C3.
- Enforced the D14-E1 ordering contract by creating `idx_import_batches_dataset_period` after the ALTER loop, with both fresh-DB and legacy-upgrade tests.
- Extended `store.Store` with `PutImportBatch`, `LatestImportBatch`, `ListImportBatches`, `PutDatasetDelete`, and `DeleteDataset`.
- Implemented exact-name parameterized `DeleteDataset` (no SQL LIKE wildcard injection) and documented prefix-deletion semantics.
- Locked the shape with `TestBatchAuditRowShape`, `TestImportBatchInsertOnly`, `TestPutDatasetDelete`, `TestDeleteDataset`, and `TestDatasetRowNewColumnsRoundTrip`.

## Task Commits

Each task was committed atomically:

1. **Task 1: schema.go additive migration（datasets 三列 + import_batches + dataset_deletes + 索引时序）** — `393f90a` (feat)
2. **Task 2: store.go 类型/接口扩展 + import_batches.go + datasets 删除路径** — `44d5be6` (test, RED) → `df85b3a` (feat, GREEN)

**Plan metadata:** to be committed after this file.

## Files Created/Modified

- `internal/store/sqlite/schema.go` — datasets 3 additive columns in schemaSQL + ALTER constants + datasetImportColumns loop + import_batches/dataset_deletes CREATE TABLE + idx_import_batches_dataset_period const.
- `internal/store/sqlite/schema_test.go` — `TestDatasetsDDL`, `TestImportBatchesDDL`, `TestDatasetDeletesDDL`, `TestImportBatchesIndexExists`.
- `internal/store/sqlite/migrate_legacy_test.go` — `TestMigrateLegacyDatasetsUpgradePath`, `TestMigrateLegacyDatasetsIdempotentReopen`.
- `internal/store/sqlite/datasets.go` — 10-column put/list SQL + `DeleteDataset` exact-name DELETE.
- `internal/store/sqlite/import_batches.go` — `PutImportBatch`, `LatestImportBatch`, `ListImportBatches`, `PutDatasetDelete`.
- `internal/store/store.go` — `DatasetRow` extension, new `ImportBatchRow`/`DatasetDeleteRow`, Store interface extension.
- `internal/store/sqlite/import_batches_test.go` — batch audit shape, insert-only constraint, dataset delete audit tests.
- `internal/store/sqlite/datasets_test.go` — new column round-trip, delete path tests.

## Decisions Made

- **Dedicated `import_batches` table (option A).** Chosen over reusing `audit.action` + `frozen_args` because batch queries are the hot path for the admin import view (ADM-02) and must be SQL-first.
- **Exact-name `DELETE` for `DeleteDataset`.** Prefix deletion is intentionally not implemented in SQL because `_` is a LIKE wildcard; callers that need prefix semantics use `ListDatasets` + `strings.HasPrefix` in Go.
- **`KMinOverride=0` means global default.** This keeps old callers and pre-Phase-15 rows compatible without forcing every import to specify a k_min value.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- During Task 2 RED commit, the initial test edit accidentally dropped the closing brace of `TestMigrateLegacyIdempotentReopen` in `migrate_legacy_test.go`. This was caught by the compiler and fixed before the GREEN commit; no functional impact.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 15-03 (preview/commit engine atomics / batch orchestration) can now use `LatestImportBatch` for DATA-16 idempotency checks.
- Plan 15-05/15-06 (importer + server API) can call `PutImportBatch`, `ListImportBatches`, `DeleteDataset`, and `PutDatasetDelete`.
- Plan 16 (KPI queries) can read `KMinOverride` from `DatasetRow`.

## Self-Check: PASSED

- [x] Created files exist: `internal/store/sqlite/import_batches.go`, `internal/store/sqlite/import_batches_test.go`, `internal/store/sqlite/datasets_test.go`.
- [x] All task commits exist and are reachable: `393f90a`, `44d5be6`, `df85b3a`.
- [x] `go test ./internal/store/sqlite/... -count=1` green.
- [x] `go vet ./internal/store/...` green.
- [x] `go build ./...` green (Store interface fully implemented).
- [x] `internal/store/sqlite/import_batches.go` contains `INSERT INTO import_batches`, no `OR REPLACE`, no `fmt.Sprintf`.
- [x] `internal/store/store.go` contains `type ImportBatchRow struct` and `LatestImportBatch(ctx context.Context`.

---
*Phase: 15-p10-1*
*Completed: 2026-08-23*
