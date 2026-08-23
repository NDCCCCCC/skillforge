---
phase: 15-p10-1
plan: 06
subsystem: api
tags: [hertz, multipart, streaming, duckdb, import, data-import, audit]

# Dependency graph
requires:
  - phase: 15-p10-1
    provides: 15-05 delivered the Importer (Preview/Commit/Push/DryRun), staging commit, and dataset registry.
provides:
  - Hertz server configured with WithStreamBody + WithDisablePreParseMultipartForm + WithMaxRequestBodySize.
  - Per-route Content-Length guard keeping the 2 GiB import limit off /api/chat.
  - POST /api/import/preview, /api/import/commit, /api/import/push (and dry_run) handlers.
  - DELETE /api/import/dataset/:name handler with Detach-before-Remove Windows lock ordering.
  - Admin-token gate on all import endpoints and the delete endpoint.
  - FAIL-closed 503 posture when SKILLFORGE_DATA_SALT is unset.
affects:
  - cmd/server/main.go wiring
  - internal/data/importer.go
  - internal/duckdb/AttachLRU
  - Phase 16 query layer (datasets registry consumption)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Stream-first multipart: multipart.NewReader over BodyStream + Header.MultipartFormBoundary; no c.FormFile / c.Request.MultipartForm."
    - "Per-route body-size guard so the large import envelope cannot be reused to flood chat routes."
    - "Detach-before-Remove for committed DuckDB datasets (Windows file-lock boundary)."
    - "INSERT-only deletion audit table (dataset_deletes)."

key-files:
  created:
    - internal/server/import_api.go
    - internal/server/import_api_test.go
    - internal/server/server_test.go
    - internal/data/importer_delete.go
  modified:
    - internal/server/server.go
    - internal/data/importer.go
    - cmd/server/main.go

key-decisions:
  - "File part is buffered in memory inside parseImportForm: the multipart reader contract requires consuming a part before calling NextPart again; in-memory buffering keeps bodies off os.TempDir() while still streaming the multipart envelope."
  - "Schema type inference now respects rule output type: hash columns → VARCHAR, bucket_fixed → DOUBLE; this prevents DuckDB appender cast errors that surfaced during Task 2 verification."
  - "Importer exposes LRU() as a test seam so delete tests can force a lazy ATTACH before the HTTP DELETE, proving the Windows Detach-before-Remove lock order."
  - "Logical dataset deletion (no __ suffix) is done by prefix filtering in Go, avoiding SQL LIKE wildcard ambiguity."

patterns-established:
  - "Import handler logs contain only batch-level metadata (row count, hashes, column names, ids, duration); SamplePairs and raw values never appear in logs (D15-B4)."
  - "All import mutation endpoints share isAdminRequest + Importer-nil 503 guards before any business logic."

requirements-completed: [DATA-01, DATA-15, DATA-17]

# Metrics
duration: 35min
completed: 2026-08-24
---

# Phase 15 Plan 06: HTTP Data-Import Surface Summary

**Streaming multipart import endpoints (preview/commit/push) plus an audited DELETE endpoint, wired into Hertz with admin-token gates and a split body-size policy.**

## Performance

- **Duration:** 35 min
- **Started:** 2026-08-24T02:21:24+08:00
- **Completed:** 2026-08-24T02:54:15+08:00
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Configured Hertz with the three required streaming options and a per-route Content-Length guard.
- Implemented POST /api/import/preview, /api/import/commit, and /api/import/push handlers using `multipart.NewReader` over `BodyStream`.
- Implemented DELETE /api/import/dataset/:name with Detach-before-Remove, file+registry+audit consistency, and both exact/logical-name deletion.
- Locked the two CONTEXT tests: `TestImportStreaming` (>16 MB without touching `os.TempDir()`) and `TestDeleteDatasetAudited` (lazy ATTACH then delete).
- Full `go test ./internal/server/... -count=1` regression suite passes.

## Task Commits

Each task was committed atomically:

1. **Task 1: server.go options, guard, env wiring** - `178ad68` (feat)
2. **Task 2: preview/commit/push handlers and tests** - `64247ab` (feat)
3. **Task 3: DELETE endpoint with audit trail** - `efaa6ca` (feat)

**Plan metadata:** pending (final docs commit)

## Files Created/Modified
- `internal/server/server.go` - Hertz constructor with streaming options, body-size guard, and import route registration.
- `internal/server/import_api.go` - Five import handlers (preview/commit/push/dry-run/delete) and shared multipart parser.
- `internal/server/import_api_test.go` - Streaming, auth, salt-unset, shape, hash bridge, dry-run, and delete test suites.
- `internal/server/server_test.go` - Route wiring, importer-nil 503, and Content-Length guard tests.
- `internal/data/importer.go` - Schema inference fix; `LRU()` test seam; `ImportStoreSeam` extended with `DeleteDataset`.
- `internal/data/importer_delete.go` - `DeleteDataset`, `deleteOneDataset`, and `ErrDatasetNotFound`.
- `cmd/server/main.go` - `SKILLFORGE_DATA_SALT`, body-size env parsing, and `Importer` construction.

## Decisions Made
- Buffers the file part in memory inside `parseImportForm` to satisfy the multipart reader contract while keeping imported bodies off `os.TempDir()`.
- Maps `hash` columns to `VARCHAR` and `bucket_fixed` columns to `DOUBLE` during schema inference so the DuckDB appender receives compatible values.
- Exposes `Importer.LRU()` only as a test seam for lock-order regression tests; production code uses it through the commit/delete paths.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed DuckDB schema type inference for rule outputs**
- **Found during:** Task 2 (push/commit handler verification)
- **Issue:** `TestImportStreaming` and `TestCommitViaPreview` failed with `cannot cast string to int64` because `inferSchemaTypes` inferred `BIGINT` for integer-looking raw `balance`/`overdue_days` columns, but `bucket_fixed` produces `float64` and `hash` produces `string`.
- **Fix:** Updated `inferSchemaTypes`/`inferSchemaTypesFromRows` to use the rule kind: `hash` → `VARCHAR`, `bucket_fixed` → `DOUBLE`, passthrough falls back to sampling. Extracted shared helper `inferSchemaTypesFromByCol`.
- **Files modified:** `internal/data/importer.go`
- **Verification:** `go test ./internal/server/ -run 'TestImportStreaming|TestCommitViaPreview' -count=1` passes.
- **Committed in:** `64247ab` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** The fix is required for correctness of the import write path; no scope creep.

## Issues Encountered
- `c.Request.MultipartFormBoundary()` returns an empty boundary until `MultipartForm()` is called; switched to `c.Request.Header.MultipartFormBoundary()`.
- Calling `NextPart()` before consuming the previous file part silently discarded file content; fixed by reading each file part fully inside `parseImportForm`.
- Running `./internal/server/...` and `./internal/data/...` together caused `TestRawNeverOnDisk` to observe temp dirs created by concurrent server-package tests; the test passes when run in isolation (`go test ./internal/data/... -count=1`).

## User Setup Required
None - the user_setup note from PLAN.md (`SKILLFORGE_DATA_SALT`) is documented in `cmd/server/main.go` and server logs; no USER-SETUP.md was generated.

## Next Phase Readiness
- Import HTTP surface is complete and tested.
- Ready for Phase 16 dataset query/k-anonymity layer to consume committed datasets.
- No blockers.

---
*Phase: 15-p10-1*
*Completed: 2026-08-24*

## Self-Check: PASSED

- [x] `15-06-SUMMARY.md` created at `.planning/phases/15-p10-1/15-06-SUMMARY.md`
- [x] Commits exist: `178ad68` (Task 1), `64247ab` (Task 2), `efaa6ca` (Task 3), `7a70189` (plan metadata)
