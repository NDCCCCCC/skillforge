---
phase: 15-p10-1
reviewed: 2026-08-24T08:15:00Z
resolved: 2026-08-24T12:25:00Z
depth: deep
files_reviewed: 27
files_reviewed_list:
  - internal/data/normalize.go
  - internal/data/desensitize.go
  - internal/data/rules.go
  - internal/data/decode.go
  - internal/data/dump.go
  - internal/data/preview.go
  - internal/data/importer.go
  - internal/data/batch.go
  - internal/data/importer_delete.go
  - internal/data/normalize_test.go
  - internal/data/desensitize_test.go
  - internal/data/rules_test.go
  - internal/data/decode_test.go
  - internal/data/dump_test.go
  - internal/data/preview_test.go
  - internal/data/importer_test.go
  - internal/data/batch_test.go
  - internal/store/sqlite/schema.go
  - internal/store/sqlite/import_batches.go
  - internal/store/sqlite/datasets.go
  - internal/store/store.go
  - internal/store/sqlite/import_batches_test.go
  - internal/store/sqlite/migrate_legacy_test.go
  - internal/duckdb/staging.go
  - internal/duckdb/importer_write.go
  - internal/duckdb/importer_write_test.go
  - internal/duckdb/supersede_test.go
  - internal/server/server.go
  - internal/server/import_api.go
  - internal/server/server_test.go
  - internal/server/import_api_test.go
  - cmd/server/main.go
  - datarules/loans.yaml
  - integration/contract/import_gates_test.go
  - scripts/verify-phase-15.sh
findings:
  critical: 2
  warning: 4
  info: 0
  total: 6
status: resolved
---

# Phase 15: Code Review Report

**Reviewed:** 2026-08-24T08:15:00Z
**Depth:** deep
**Files Reviewed:** 27
**Status:** resolved

## Summary

The Wave 4/5 review surfaced six issues across the import HTTP surface and the data write path. All six have been addressed:

- **CR-01:** `parseImportForm` now streams the file part through an `onFile` callback; the handler consumes the `*multipart.Part` synchronously in the request goroutine, eliminating the OOM risk from buffering the whole upload and avoiding cross-goroutine body-stream reads.
- **CR-02:** `pushBatch` and `finalizeBatch` enforce `ImportMaxRowsPerBatch` after `writeStaging` and remove the staging file on overflow.
- **WR-01:** `discard_ts` is captured after `writeStaging` succeeds in both `pushBatch` and `finalizeBatch`.
- **WR-02:** Audit rows (`import_batches` and `dataset_deletes`) are now written before filesystem/registry mutations; a dangling audit row on mutation failure is accepted as proof-of-attempt.
- **WR-03:** `inferDuckDBType` defaults to `VARCHAR` when every sample is NULL/empty.
- **WR-04:** `verify-phase-15.sh` Stage 04 regex updated to the actual test names.

Additionally, the `TestImportStreaming` regression that appeared while fixing CR-01 was root-caused to a latent bug in `strictUTF8Reader`: it could split a multi-byte UTF-8 rune across a small caller buffer, leaving the next `Read` starting with an invalid continuation byte. The reader was rewritten on top of `bufio.Reader.ReadRune` with a small per-rune output buffer, fixing the corruption and keeping the fail-closed contract.

## Findings (all addressed)

### CR-01: Import handler buffers the entire upload file in memory

**Status:** fixed

`internal/server/import_api.go` `parseImportForm` now takes an `onFile` callback; the file `*multipart.Part` is consumed synchronously by the callback in the same goroutine that parses multipart. The `importForm.File` field has been removed to prevent accidental buffering.

### CR-02: `ImportMaxRowsPerBatch` ceiling is not enforced on Commit/Push

**Status:** fixed

`internal/data/importer.go` `pushBatch` and `finalizeBatch` now check `rowCount > ImportMaxRowsPerBatch` immediately after `writeStaging` and remove the staging file before returning the overflow error.

### WR-01: `discard_ts` is captured before the raw stream is consumed

**Status:** fixed

`discardTS := time.Now().UTC().Format(...)` moved to after `writeStaging` returns successfully in both `pushBatch` and `finalizeBatch`.

### WR-02: Audit rows are written after filesystem/registry mutations

**Status:** fixed

`internal/data/importer.go` `commitOrSupersede` now writes the `import_batches` audit row before calling `duckdb.CommitStagedDataset` / `duckdb.SupersedeStagedDataset`. `internal/data/importer_delete.go` `deleteOneDataset` now writes the `dataset_deletes` audit row before detaching/removing the dataset and deleting the registry row.

### WR-03: All-NULL samples infer `BIGINT` instead of `VARCHAR`

**Status:** fixed

`inferDuckDBType` already tracked `hasConcrete`; when no concrete sample is seen it returns `VARCHAR`.

### WR-04: `verify-phase-15.sh` Stage 04 references non-existent tests

**Status:** fixed

Stage 04 `-run` regex updated to the actual sqlite store test names.

---

_Reviewed: 2026-08-24T08:15:00Z_  
_Reviewer: Claude (gsd-code-reviewer)_  
_Depth: deep_  
_Resolved: 2026-08-24T12:25:00Z_
