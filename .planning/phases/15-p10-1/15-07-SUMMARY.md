---
phase: 15-p10-1
plan: 07
subsystem: gate
tags: [contract, structural-bans, verify-script, makefile, gate]

# Dependency graph
requires:
  - phase: 15-p10-1
    provides: 15-06 delivered the streaming HTTP import surface; all import code paths exist to be scanned.
provides:
  - integration/contract/import_gates_test.go with three structural bans (multipart pre-parse, row INSERT, row-value logging).
  - scripts/verify-phase-15.sh 10-stage hermetic gate.
  - Makefile verify-phase-15 target.
affects:
  - integration/contract/import_gates_test.go
  - scripts/verify-phase-15.sh
  - Makefile

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "WalkDir structural ban gate (D8-04 pattern): scan production .go, skip non-product dirs, self-skip the gate file, report file:line."
    - "Negative proof: temporary probe injection proves the gate is not vacuous."
    - "Hermetic verify script: build/vet → race → split suites → contract bans → source-grep locks → frozen lock byte-identity."

key-files:
  created:
    - integration/contract/import_gates_test.go
    - scripts/verify-phase-15.sh
  modified:
    - Makefile

key-decisions:
  - "dump.go is explicitly excluded from the INSERT INTO ban: it is a parser/validator for untrusted .dump files, not a data-write path."
  - "Comment lines are ignored when scanning for .MultipartForm( / .FormFile( so import_api.go's safety documentation does not create false positives."
  - "Source-grep locks pin KMinDefault, PreviewSessionTTL, WithStreamBody/WithDisablePreParseMultipartForm, the datarules/ tracked directory, and the absence of hand-rolled HMAC."

patterns-established:
  - "Phase acceptance gate is a single executable script with machine-readable [STAGE-NN-PASS/FAIL] output."
  - "Structural bans are enforced by a //go:build contract test that fails with file:line pointers."

requirements-completed: [DATA-01, DATA-02]

# Metrics
duration: 30min
completed: 2026-08-24
---

# Phase 15 Plan 07: Structural Ban Gates + Hermetic Phase Gate Summary

**Closed Phase 15 with a 10-stage hermetic acceptance gate and three WalkDir structural bans that lock the import pipeline's security invariants.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-08-24T03:00:00+08:00
- **Completed:** 2026-08-24T03:30:00+08:00
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Created `integration/contract/import_gates_test.go` with three independent contract tests:
  1. `TestNoPreParseMultipartPath` — bans `.MultipartForm(` and `.FormFile(` across `internal/server/` and `internal/data/`.
  2. `TestNoRowInsertInImportPath` — bans `INSERT INTO` in `internal/data/` production code (except `dump.go`, the parser) and `internal/duckdb/importer_write.go`.
  3. `TestNoRowValueLoggingInImportAPI` — bans row-value identifiers inside `log.*` calls in `internal/server/import_api.go`.
- Performed negative-proof probes for each ban; each gate correctly failed with a file:line pointer and returned to green after the probe was removed.
- Created `scripts/verify-phase-15.sh`, a 10-stage hermetic gate:
  1. gcc precondition
  2. `go build ./...` + `go vet ./...`
  3. full-repo `go test ./... -count=1 -race`
  4. sqlite store suite
  5. duckdb staging/supersede/appender suite
  6. `internal/data` lock-name suite
  7. `internal/server` import HTTP suite
  8. contract structural bans
  9. source-grep locks
  10. frozen lock byte-identity
- Added `verify-phase-15` target to the Makefile.
- Full gate result: **PHASE 15 HERMETIC GATE: PASS — 10/10 stages** (run in WSL2 Ubuntu-22.04).

## Negative-Proof Evidence

Each probe was injected, the contract test failed, and the probe was removed.

| Ban | Probe injection | Gate output (excerpt) | Reverted |
|-----|-----------------|------------------------|----------|
| Multipart pre-parse | `_, _, _ = c.Request.FormFile("probe")` in `internal/server/import_api.go:123` | `pre-parse multipart API use detected: internal/server/import_api.go:123` | ✅ |
| Row-by-row INSERT | `const probeSQL = "INSERT INTO probe VALUES (1)"` in `internal/data/importer.go:38` | `row-by-row INSERT INTO found: internal/data/importer.go:38` | ✅ |
| Row-value logging | `log.Printf("NEGATIVE-PROBE preview SamplePairs=%v", []data.RowPair{})` in `internal/server/import_api.go:119` | `row-value logging detected: internal/server/import_api.go:119 contains 'SamplePairs' / 'RowPair' in log call` | ✅ |

## Files Created/Modified

- `integration/contract/import_gates_test.go` — new contract gate file.
- `scripts/verify-phase-15.sh` — new 10-stage hermetic acceptance gate.
- `Makefile` — added `verify-phase-15` target.

## Decisions Made

- `dump.go` is excluded from the `INSERT INTO` scan because its job is to inspect untrusted `.dump` input, not to execute SQL writes.
- Comment lines are ignored when scanning for multipart APIs so that `import_api.go`'s safety comments do not create false positives.
- The gate scans `internal/data/` test files as well as production files for multipart APIs: a regression test that falls back to `c.FormFile` would still leak raw data to disk.

## Deviations from Plan

None. Stage 06 and Stage 07 regexes use the exact test names from `15-VALIDATION.md` and `15-CONTEXT.md`; Stage 04 uses existing migration test names where the plan referenced generic `TestMigrate`/`TestSchema`.

## Issues Encountered

- The original `TestRawNeverOnDisk` (Plan 15-05) and `TestImportStreaming` (Plan 15-06) compared top-level entries of the global `os.TempDir()`. When running `go test ./...`, other packages create temp directories concurrently, causing flaky failures. Fixed by redirecting each package's tests to a package-private temp root via `TestMain` in `internal/data/importer_test.go` and `internal/server/import_api_test.go`.
- `scripts/verify-phase-15.sh` initially failed on WSL2 because `go` was not on `PATH`; the script now adds `/usr/local/go/bin` to `PATH` when `go` is missing and `/usr/local/go/bin/go` exists.

## User Setup Required

None.

## Next Phase Readiness

- Phase 15 is complete and hermetically gated.
- Phase 16 (dataset query / k-anonymity consumption layer) can consume the committed datasets registry and DuckDB files produced by the import pipeline.

---
*Phase: 15-p10-1*
*Completed: 2026-08-24*

## Self-Check: PASSED

- [x] `integration/contract/import_gates_test.go` created and green under `-tags=contract`
- [x] Negative proofs recorded for all three bans
- [x] `scripts/verify-phase-15.sh` created and runs 10/10 PASS in WSL2
- [x] `make verify-phase-15` dry-run prints the script invocation
- [x] `15-07-SUMMARY.md` created at `.planning/phases/15-p10-1/15-07-SUMMARY.md`
