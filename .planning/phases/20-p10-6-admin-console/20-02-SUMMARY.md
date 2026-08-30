# Phase 20 Plan 20-02: Import Batch List View — Summary

**Phase:** 20-p10-6-admin-console
**Plan:** 02
**Status:** COMPLETE
**Completed:** 2026-08-30
**Commit:** e12dd9e

---

## One-Liner

`GET /api/admin/batches` handler with `ListAllImportBatches` store method, and `ImportBatchList` React component with status badges, human-readable raw size, and empty state.

---

## Files Created

| File | Purpose |
|------|---------|
| `internal/server/admin_batches.go` | `GET /api/admin/batches` handler — 403 gate, 503 if store nil, 200 with `{batches:[...]}` envelope |
| `internal/server/admin_batches_test.go` | Unit tests: 403 without token, 200 with valid token, correct JSON shape, nil store 503, empty array 200, retrieval-only source-grep |
| `web/src/components/admin/ImportBatchList.tsx` | React table component: Dataset \| Status badge \| Row Count \| Raw Size \| Discarded At; empty state |

---

## Files Modified

| File | Change |
|------|--------|
| `internal/store/store.go` | Added `ListAllImportBatches(ctx) ([]ImportBatchRow, error)` to Store interface |
| `internal/store/sqlite/import_batches.go` | Added `listAllImportBatchesSQL` const and `ListAllImportBatches` implementation (no WHERE filter — all datasets) |
| `internal/server/server.go` | Registered `GET /api/admin/batches` route after table-counts |
| `web/src/components/admin/AdminDashboard.tsx` | Replaced batches placeholder with `<ImportBatchList />` component |

---

## Decisions Made

| ID | Decision | Rationale |
|----|----------|-----------|
| D-20-02-01 | `ListAllImportBatches` as new store method | `ListImportBatches(dataset)` filters by dataset; admin view needs all datasets; empty string pass-through would be a silent semantic change |
| D-20-02-02 | `batchStatus` derived from `DiscardTS` | `DiscardTS != ""` → "completed" (raw data discarded after desensitization); else "pending" |
| D-20-02-03 | Raw size formatting in FE | `formatBytes` utility renders bytes as B/KB/MB; `toLocaleString` for row count |
| D-20-02-04 | Token from `localStorage['sf_admin_token']` | Mirrors CapacityMonitor pattern; BE validates via `isAdminRequest` constant-time compare |

---

## Deviations from Plan

None — plan executed exactly as written.

### Auto-Fixed Issues

None encountered.

---

## Threat Mitigations (T-20-02-MUTATE, T-20-02-INJECT)

| Threat | Mitigation | Implementation |
|--------|------------|----------------|
| T-20-02-MUTATE | Zero mutation verbs enforced | Source-grep gate in `admin_batches_test.go` + route registered as GET only |
| T-20-02-INJECT | No caller input in SQL | `ListAllImportBatches` has no WHERE parameters — entire table scanned by creation order |

---

## Verification

- `grep -cE "POST|PUT|DELETE|PATCH" internal/server/admin_batches.go` = 0 (retrieval-only)
- `go vet` / `gofmt -e` on all modified Go files — no errors
- `npx tsc --noEmit` in web/ — PASSED (no output)
- `AdminDashboard` renders `<ImportBatchList />` when `adminSubroute === 'batches'`
- `ImportBatchList` fetches `/api/admin/batches` with `X-Admin-Token` header from localStorage

---

## Success Criteria

| # | Criterion | Status |
|---|-----------|--------|
| 1 | `GET /api/admin/batches` returns 200 with valid token, 403 without | `isAdminRequest` gate in handler — verified by test |
| 2 | Response body has `batches` array with correct field names | `importBatchesAdminResponse` envelope with `batch_id`, `dataset`, `status`, `desensitized_row_count`, `raw_size_bytes`, `discard_ts`, `created_at` |
| 3 | ImportBatchList renders status badges with correct colors | StatusBadge: completed=green, pending=amber, running=amber+pulse, failed=red |
| 4 | Empty state shows when API returns empty array | `EmptyState` component with "暂无导入批次" + "数据导入后将显示在此处" |
| 5 | No edit or action buttons in the table | Table-only render with no action columns |

---

## TDD Gate Compliance

This plan does not have `type: tdd` — TDD gate check not applicable.

---

## Self-Check

- [x] `internal/server/admin_batches.go` exists with 0 mutation verbs
- [x] `internal/server/admin_batches_test.go` exists with 2 tests (handler + retrieval-only)
- [x] `web/src/components/admin/ImportBatchList.tsx` exists with table, status badges, empty state
- [x] `internal/store/store.go` has `ListAllImportBatches` in interface
- [x] `internal/store/sqlite/import_batches.go` has `ListAllImportBatches` implementation
- [x] `internal/server/server.go` has `GET /api/admin/batches` registered
- [x] `web/src/components/admin/AdminDashboard.tsx` renders `<ImportBatchList />` for batches tab
- [x] Commits 04ed58a, 135193b, e12dd9e exist

**Self-Check: PASSED**
