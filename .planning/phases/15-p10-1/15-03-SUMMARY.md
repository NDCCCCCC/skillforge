---
phase: 15-p10-1
plan: 03
subsystem: data

tags: [go, duckdb, staging, appender, supersede, atomic-rename, attach, composite-name]

requires:
  - phase: 15-p10-1
    plan: 02
    provides: internal/store/sqlite import_batches audit + datasets registry extensions
  - phase: 14-p10-0-cgo-trust-floor
    provides: internal/duckdb connector + CommitStagedDataset + AttachLRU

provides:
  - SupersedeStagedDataset three-step backup-rename-register chain (D15-C1)
  - WriteStagingBatch Appender-only bulk write helper (SC-1)
  - Composite dataset id ATTACH probe (A1 assumption closed)
  - Quoted ATTACH/DETACH identifiers for hyphenated dataset ids

affects:
  - 15-p10-1 plans 04-07 (dump parser, preview, importer orchestration, server API, gate)
  - 16-p11 (KPI queries consume committed datasets)

key-files:
  created:
    - internal/duckdb/supersede_test.go
    - internal/duckdb/importer_write.go
    - internal/duckdb/importer_write_test.go
  modified:
    - internal/duckdb/staging.go
    - internal/duckdb/connector.go
    - internal/duckdb/attach_lru.go

tech-stack:
  added: []
  patterns:
    - three-step backup-rename-register with compensation on step 2/3 failures
    - Appender-only bulk write path (no INSERT INTO)
    - type allowlist gate before Appender creation
    - deterministic test hook for timestamped backup paths
    - double-quoted SQL identifiers for ATTACH/DETACH aliases

key-decisions:
  - "Quoted ATTACH/DETACH aliases are required for composite dataset ids like loans__2026-07; unquoted hyphens are a DuckDB syntax error"
  - "Supersede staging file is preserved after registry-write rollback so the caller can retry (mirrors CommitStagedDataset retry semantics)"
  - "WriteStagingBatch rejects an existing table because import batches always target fresh staging files"
  - "BIT/ANY/INVALID types are rejected before Appender creation to avoid go-duckdb appender creation errors (T-15-16)"

patterns-established:
  - "cleanupStaging flag semantics: remove staging only when step 2 (staging->target) never completed"
  - "supersedeBackupName overridable var for deterministic failure injection in tests"
  - "defensive identifier allowlist for table/column names in staging DDL"

requirements-completed: [DATA-16]

# Metrics
duration: 12min
completed: 2026-08-23
---

# Phase 15 Plan 03: 数据导入管线 — 替换原子性与批量写入引擎面 Summary

**Engine-side atomicity and bulk-write slice for the data import pipeline: SupersedeStagedDataset three-step backup chain plus WriteStagingBatch Appender-only helper, with the A1 composite-name ATTACH assumption empirically closed.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-23T17:04:26Z
- **Completed:** 2026-08-23T17:16:44Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Implemented `SupersedeStagedDataset` in `internal/duckdb/staging.go` with the three-step backup-rename-register chain required by D15-C1: old file → backup, staging → target, registry update.
- Locked the failure-compensation semantics: step 2 failure restores backup→target; step 3 failure moves target→staging (preserved for retry) and backup→target, leaving old data queryable and old registry row intact.
- Added `ErrSupersedeTargetMissing` sentinel so callers attempting to supersede a non-existent dataset get a clear "use CommitStagedDataset" signal.
- Implemented `WriteStagingBatch` in `internal/duckdb/importer_write.go` — the production import pipeline's only allowed persistence path, using a dedicated Connector + `NewAppenderFromConn` with zero `INSERT INTO` usage.
- Enforced a defensive type allowlist `{VARCHAR, BIGINT, DOUBLE, TIMESTAMP}` before Appender creation, rejecting unsupported types like `BIT` (T-15-16).
- Enforced fresh-staging semantics by rejecting an existing target table (fail-closed against staging file reuse).
- Closed the A1 assumption: composite dataset id `loans__2026-07` is written, committed, lazily ATTACHed via `AttachLRU`, and queried successfully.
- Fixed pre-existing DuckDB identifier quoting so hyphenated dataset ids work end-to-end (ATTACH alias + DETACH both use double-quoted identifiers).

## Task Commits

Each task was committed atomically with RED/GREEN TDD:

1. **Task 1: SupersedeStagedDataset three-step backup chain**
   - RED: `test(15-03): add failing tests for SupersedeStagedDataset three-step backup chain` — `3aecddb`
   - GREEN: `feat(15-03): implement SupersedeStagedDataset three-step backup chain` — `593b705`

2. **Task 2: WriteStagingBatch Appender-only bulk write helper + composite name ATTACH probe**
   - RED: `test(15-03): add failing tests for WriteStagingBatch + composite name ATTACH probe` — `3f9aba3`
   - GREEN: `feat(15-03): implement WriteStagingBatch Appender-only bulk write helper` — `a390d45`

## Files Created/Modified

- `internal/duckdb/staging.go` — Added `ErrSupersedeTargetMissing`, `SupersedeOptions`, `SupersedeStagedDataset`, and `supersedeBackupName`; preserved `CommitStagedDataset` unchanged.
- `internal/duckdb/supersede_test.go` — `TestSupersedeAtomicSwap` with happy, `registryWriteFails`, `step1RenameFails`, `targetMissing`, and `detachFirst` subtests.
- `internal/duckdb/importer_write.go` — `ColumnSpec`, `StagingTable`, `WriteStagingBatch`, type/identifier allowlists, and Appender bulk-write implementation.
- `internal/duckdb/importer_write_test.go` — `TestWriteStagingBatch` (roundTrip, typeAllowlist, tableCreate) and `TestCompositeNameAttach`.
- `internal/duckdb/connector.go` — Quoted ATTACH alias `"ds_%s"` to support hyphenated dataset ids.
- `internal/duckdb/attach_lru.go` — Quoted DETACH identifier `"ds_%s"` to match quoted aliases.

## Decisions Made

- **Quoted ATTACH/DETACH identifiers.** Composite dataset ids such as `loans__2026-07` contain hyphens, which DuckDB rejects in unquoted identifiers. The existing `AttachDataset` and `execDetachLocked` were updated to double-quote the `ds_{id}` alias. This is a correctness fix for the A1 assumption rather than a behavior change for existing underscore-only ids.
- **Preserve staging on registry rollback.** When step 3 (`Registry.PutDataset`) fails, the implementation rolls the target back to staging and the backup back to target. The staging file now holds the new data and is preserved so the caller can retry, matching the retry semantics of `CommitStagedDataset`.
- **Fresh-staging table guard.** `WriteStagingBatch` rejects a staging file that already contains the target table. Import batches must write new staging files; an existing table signals reuse or corruption.
- **Type allowlist before Appender.** Unsupported DuckDB types (BIT, ANY, INVALID, etc.) are rejected before `NewAppenderFromConn` is called, turning a runtime Appender creation error into a clear, early error.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] `AttachDataset`/`Detach` did not quote hyphenated dataset aliases**
- **Found during:** Task 1 GREEN run (`TestSupersedeAtomicSwap/detachFirst` failed)
- **Issue:** `AttachDataset` built `ATTACH 'path' AS ds_loans__2026-08 (READ_ONLY)` and `execDetachLocked` built `DETACH ds_loans__2026-08`. DuckDB rejects unquoted identifiers containing hyphens with "syntax error at or near '-'". The same bug blocked the A1 `TestCompositeNameAttach` probe.
- **Fix:** Double-quoted the alias in both `AttachDataset` (`"ds_%s"`) and `execDetachLocked` (`"ds_%s"`). Existing underscore-only ids are unaffected; hyphenated composite ids now work.
- **Files modified:** `internal/duckdb/connector.go`, `internal/duckdb/attach_lru.go`
- **Verification:** `TestSupersedeAtomicSwap/detachFirst` and `TestCompositeNameAttach` both pass.
- **Committed in:** `593b705` (Task 1 GREEN)

**2. [Rule 3 - Blocking] Step 1 rename-failure injection required a deterministic backup path**
- **Found during:** Task 1 GREEN run (`TestSupersedeAtomicSwap/step1RenameFails` could not pre-create the destination because the backup filename embeds `time.Now().UnixNano()`)
- **Issue:** The plan specifies a timestamped backup filename for uniqueness, but a test cannot predict the exact nanosecond to pre-create the blocking directory.
- **Fix:** Extracted the backup-name generation into a package-level `supersedeBackupName` variable that tests override for deterministic failure injection. Production still uses `time.Now().UnixNano()`.
- **Files modified:** `internal/duckdb/staging.go`, `internal/duckdb/supersede_test.go`
- **Verification:** `TestSupersedeAtomicSwap/step1RenameFails` passes.
- **Committed in:** `593b705` (Task 1 GREEN)

**3. [Rule 1 - Bug] Initial rollback incorrectly deleted the preserved staging file**
- **Found during:** Task 1 GREEN run (`TestSupersedeAtomicSwap/registryWriteFails` asserted staging file should exist for retry)
- **Issue:** The first implementation reused a single `renamed` flag for defer cleanup. After a successful step 2 and failed step 3 with successful rollback, `renamed` was set to `false`, causing the defer to delete the staging file that now held the new data.
- **Fix:** Replaced the flag with `cleanupStaging`: `true` initially, set to `false` only when step 2 succeeds. On step 3 rollback, staging remains preserved; on step 2 failure, staging is removed as residue.
- **Files modified:** `internal/duckdb/staging.go`
- **Verification:** `TestSupersedeAtomicSwap/registryWriteFails` passes and asserts staging file exists.
- **Committed in:** `593b705` (Task 1 GREEN)

---

**Total deviations:** 3 auto-fixed (2 Rule 2 correctness, 1 Rule 3 blocking, 1 Rule 1 bug)
**Impact on plan:** All fixes are necessary for the plan's stated success criteria and tests. No scope creep.

## Issues Encountered

- The Windows-native MSYS2 gcc cannot link go-duckdb (documented emutls ABI mismatch). All `internal/duckdb` tests were run under WSL2 Ubuntu-22.04 with `/usr/local/go/bin/go` on the `/mnt/d` repository mount.
- `os.Rename(file, non-empty-dir)` semantics differ between Linux ext4 and NTFS via WSL 9p. The step-1 failure injection therefore pre-creates the exact backup path under a deterministic name rather than relying on filesystem-specific rename rejection.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 15-04 (sqlite .dump accept-list parser) can treat the staging file as the write target and rely on `WriteStagingBatch`.
- Plan 15-05 (importer orchestration + preview/commit) can call `WriteStagingBatch` for the bulk desensitized write and `SupersedeStagedDataset` for monthly replacement.
- Plan 15-06 (server API) can rely on composite dataset ids (`{dataset}__{period}`) being valid for both file names and ATTACH aliases.

## Self-Check: PASSED

- [x] Created files exist: `internal/duckdb/supersede_test.go`, `internal/duckdb/importer_write.go`, `internal/duckdb/importer_write_test.go`.
- [x] All task commits exist and are reachable: `3aecddb`, `593b705`, `3f9aba3`, `a390d45`.
- [x] `go test ./internal/duckdb/... -count=1` green under WSL2 Ubuntu-22.04.
- [x] `go vet ./internal/duckdb/...` green.
- [x] `internal/duckdb/importer_write.go` contains no `INSERT INTO` literal.
- [x] `internal/duckdb/staging.go` contains `SupersedeStagedDataset` and `ErrSupersedeTargetMissing`; `CommitStagedDataset` body unchanged.
- [x] `TestSupersedeAtomicSwap` includes `registryWriteFails` and `step1RenameFails` injection cases.
- [x] `TestCompositeNameAttach` passes with composite id `loans__2026-07`.

---
*Phase: 15-p10-1*
*Completed: 2026-08-23*
