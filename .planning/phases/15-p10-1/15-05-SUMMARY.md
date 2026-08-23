---
phase: 15-p10-1
plan: 05
subsystem: database
tags: [go, duckdb, eino, csv, dump, desensitization, hmac, bucket, audit]

requires:
  - phase: 15-01
    provides: desensitization pure functions, rules loader, decode stream
  - phase: 15-02
    provides: import_batches audit table + dataset registry seam
  - phase: 15-03
    provides: CommitStagedDataset / SupersedeStagedDataset / WriteStagingBatch
  - phase: 15-04
    provides: dump parser + preview session store + content-hash bridge

provides:
  - Three-channel import orchestrator: Preview / Commit / Push / DryRun
  - Single-pass streaming Commit with TeeReader→hasher before openChannel
  - Idempotent re-import and supersede chain via checkIdempotent
  - Machine-channel Push with first-100-row buffering for schema inference
  - Zero-write DryRun validation payload
  - TestRawNeverOnDisk library-level raw-leak guard

affects:
  - 15-06 (HTTP handlers consume Preview/Commit/Push signatures)
  - 15-07 (verify-phase-15.sh structural bans)

 tech-stack:
  added: []
  patterns:
    - "rowSource pull-style iterator for buffered prefix + streamed remainder"
    - "shared writeStaging + commitOrSupersede tail for human and machine channels"
    - "ImportStoreSeam narrow interface isolates importer from full store.Store"

key-files:
  created:
    - internal/data/batch.go
    - internal/data/batch_test.go
  modified:
    - internal/data/importer.go
    - internal/data/importer_test.go
    - internal/data/decode.go
    - internal/data/preview_test.go

key-decisions:
  - "D15-05-1: Push shares the same writeStaging + commitOrSupersede tail as Commit; the only front-end difference is the 100-row schema-inference buffer and the absence of a preview session."
  - "D15-05-2: DryRun is a first-class method returning DryRunResult with inferred columns and rule violations; it performs zero writes and leaves no staging/dataset/audit residue."
  - "D15-05-3: Content hash bridge remains enforced only after WriteStagingBatch exhausts the stream; ValidateCommitAgainstPreview and checkIdempotent are never run before the single-pass reader is consumed."

patterns-established:
  - "Pull-style rowSource with bufferedSource prefix replay for single-pass machine imports."
  - "Common commitOrSupersede tail that removes staging on idempotent hits or hash mismatches before any registry mutation."

requirements-completed: [DATA-01, DATA-02, DATA-15, DATA-16, DATA-17]

duration: 1h 05m
completed: 2026-08-24
---

# Phase 15 Plan 05: Three-Channel Import Orchestrator Summary

**Single-pass Preview/Commit/Push/DryRun orchestrator with content-hash bridge, idempotent supersede chain, and library-level proof that raw bytes never touch disk.**

## Performance

- **Duration:** 1h 05m
- **Started:** 2026-08-24T01:00:00+08:00
- **Completed:** 2026-08-24T02:06:00+08:00
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- Completed the human two-stage import: Preview (first 100 raw↔desensitized pairs + frozen schema) → Commit (re-upload hash bridge + atomic commit/supersede + audit row).
- Added the machine channel: Push commits directly without a preview session, buffering the first 100 rows for type inference and streaming the remainder through the same staging tail.
- Added DryRun: zero-side-effect validation that returns inferred columns and rule violations without writing staging, registry, or audit rows.
- Locked the single-pass contract with `oneShotReader` tests and `TestRawNeverOnDisk`, which scans `os.TempDir()` plus the staging/datasets directories for raw-file leaks.

## Task Commits

Each task was committed atomically:

1. **Task 1: Preview path** — `d3ba7cc` (feat)
2. **Task 2: Commit path** — `b1b8412` (test) → `a733564` (feat)
3. **Task 3: Push/DryRun/RawNeverOnDisk** — `7b2e154` (test) → `7aa2ced` (feat)

**Plan metadata:** docs commit (see git log `docs(15-05): complete plan 05 summary and update state/roadmap/requirements`)

## Files Created/Modified

- `internal/data/importer.go` — Core orchestrator: Preview, Commit, Push, DryRun, shared `writeStaging`/`commitOrSupersede`, `bufferedSource`, and schema-inference helpers.
- `internal/data/importer_test.go` — Preview tests plus Task 3 tests (Push direct, dump channel, DryRun zero-write, fail-closed unknown column, RawNeverOnDisk).
- `internal/data/batch.go` — `registryName`, `newBatchID`, `checkIdempotent`, `writeAuditRow`, `newAuditRow`.
- `internal/data/batch_test.go` — Hash-bridge, idempotent re-import, supersede chain, expired session, and oneShotReader contract tests.
- `internal/data/decode.go` — Fixed `strictUTF8Reader.Read` byte-discard bug when caller buffer was smaller than validated prefix.
- `internal/data/preview_test.go` — Renamed hash-bridge unit test to avoid collision with the integration test in `batch_test.go`.

## Decisions Made

- Followed the plan's interface contract exactly: `Importer.Push` rejects `DryRun=true` and routes to `Importer.DryRun`; `DryRunResult` carries `Columns` and `Violations`.
- Kept the content-hash bridge as a post-stream check: `ValidateCommitAgainstPreview` and `checkIdempotent` run only after `WriteStagingBatch` has exhausted the `iter.Seq`.
- Used a narrow `ImportStoreSeam` so `Importer` depends only on the five store methods it needs, mirroring the `duckdb.DatasetRegistry` precedent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Windows go-duckdb link failure:** internal/data tests (like internal/duckdb) must run under WSL2 Ubuntu-22.04 because MSYS2 gcc hits the documented emutls ABI mismatch. All verification was run under WSL.
- **`strictUTF8Reader.Read` dropped bytes:** Long CSV streams produced `record on line 61: wrong number of fields`. Root cause: the reader copied `c` bytes into the caller buffer but advanced `pend` by `n` validated bytes. Fix: advance by `c`. Verified with existing CSV tests and the new Push pipeline.

## User Setup Required

None - no external service configuration required.

## Verification

- `go test ./internal/data/... ./internal/duckdb/... ./internal/store/sqlite/... -count=1` — PASS
- `go vet ./internal/data/...` — PASS
- `grep` write-path check for literal `INSERT INTO` in `importer.go`/`batch.go` — none found (dump parser `dump.go` contains only parser/error-message literals).

## Next Phase Readiness

- Library-level import orchestration is complete and ready for 15-06 HTTP endpoint wiring.
- No blockers for the Hertz multipart/streaming handlers; the `Importer` API signatures match the 15-06 interface contract.

## Self-Check: PASSED

- [x] `.planning/phases/15-p10-1/15-05-SUMMARY.md` exists
- [x] Metadata docs commit exists in git history
- [x] All task commits (`d3ba7cc`, `b1b8412`, `a733564`, `7b2e154`, `7aa2ced`) exist in git history
- [x] Full verification suite passed before final commit

---
*Phase: 15-p10-1*
*Completed: 2026-08-24*

