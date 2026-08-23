---
phase: 15-p10-1
plan: 04
subsystem: data

tags: [go, sqlite, dump, parser, accept-list, preview, sha256, in-memory-store]

requires:
  - phase: 15-p10-1
    plan: 01
    provides: internal/data pure-function desensitization core (rules_sha256, decode, normalize, desensitize)
  - phase: 15-p10-1
    plan: 03
    provides: WriteStagingBatch Appender-only bulk write target + composite dataset id ATTACH support

provides:
  - internal/data/dump.go accept-list parser for sqlite3 .dump (zero SQL execution)
  - internal/data/preview.go in-memory preview session store with TTL/capacity/deep-copy snapshots
  - D15-B1 content-hash bridge between preview and commit requests
  - Four 3.53.0 ground-truth fixtures (basic / autoincrement / unistr / malicious)
  - 64 MiB dump-channel input fail-closed limit (2 GB upload envelope reserved for CSV/push)

affects:
  - 15-p10-1 plans 05-07 (importer orchestration, server API, gate)
  - 16-p11 (KPI queries consume committed datasets)

key-files:
  created:
    - internal/data/dump.go
    - internal/data/dump_test.go
    - internal/data/preview.go
    - internal/data/preview_test.go
    - internal/data/testdata/dump_basic.sql
    - internal/data/testdata/dump_autoincrement.sql
    - internal/data/testdata/dump_unistr.sql
    - internal/data/testdata/dump_malicious.sql
  modified: []

tech-stack:
  added: []
  patterns:
    - statement-level scanner respecting strings/blob literals/escapes
    - three-layer accept-list parser (boilerplate skip / CREATE TABLE / INSERT INTO same table)
    - fail-closed default-deny with offending-statement snippet
    - mutex+map+list in-memory store with TTL and capacity eviction
    - deep-copy snapshot return for concurrent-safe iteration

key-decisions:
  - "Kept the package-level fmtError wrapper ('data.%s: %w') for ParseDump and ValidateCommitAgainstPreview; tests assert on the required substring per the plan's '含' requirement."
  - "Declared DumpNullValue as an in-band non-printable sentinel so the parser never has to represent NULL with an empty string that could collide with real data."

patterns-established:
  - "Source-grep locked literals with explicit comments: DumpMaxInputBytes = 64 << 20, PreviewSessionTTL = 30 * time.Minute, PreviewSessionMaxEntries = 64"
  - "In-memory raw sample rows confined to PreviewSession; persistence/logging discipline delegated to HTTP handler layer (15-06/15-07)"

requirements-completed: [DATA-01, DATA-15]

# Metrics
duration: 9min
completed: 2026-08-23
---

# Phase 15 Plan 04: SQLite .dump accept-list parser + preview session store Summary

**SQLite .dump accept-list parser that never executes SQL, plus an in-memory preview session store with a content-hash bridge to prevent previewed file A from being committed as file B.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-08-23T17:23:59Z
- **Completed:** 2026-08-23T17:33:15Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Shipped `internal/data/dump.go` with a statement-level scanner that respects single-quoted strings, `''` escapes, and `x'/X'` blob literals.
- Implemented the three-layer accept-list: boilerplate skip (PRAGMA/BEGIN/COMMIT/CREATE INDEX/sqlite_sequence), exactly one CREATE TABLE, and INSERT INTO only that same table.
- Locked fail-closed rejection of DROP/UPDATE/PRAGMA/ATTACH/CREATE TRIGGER/CREATE VIEW/DELETE/multi-table statements, each error quoting the offending statement prefix.
- Rejected `unistr()` literals with Chinese cleanup guidance, closing the v1 boundary for TEXT-with-control-characters dumps.
- Enforced `DumpMaxInputBytes = 64 << 20` by byte counting before materialization completes.
- Created four ground-truth fixtures from sqlite3 3.53.0 output: basic, autoincrement boilerplate, unistr, and malicious.
- Shipped `internal/data/preview.go` with TTL/capacity bounded in-memory sessions, deep-copy snapshots on Get, and the `ValidateCommitAgainstPreview` hash bridge for D15-B1.
- Verified zero `database/sql` / `Exec` / `Query` references in `dump.go`.

## Task Commits

Each task was committed atomically with RED/GREEN TDD:

1. **Task 1: dump.go accept-list parser + 3.53.0 fixtures**
   - RED: `test(15-04): add failing tests for dump parser + 3.53.0 ground-truth fixtures` — `4df7556`
   - GREEN: `feat(15-04): implement dump.go accept-list parser + 64 MiB fail-closed limit` — `2b02e33`

2. **Task 2: preview.go in-memory session store + hash bridge**
   - RED: `test(15-04): add failing tests for preview session store + hash bridge` — `958e7a9`
   - GREEN: `feat(15-04): implement preview.go in-memory session store + hash bridge` — `6c37fbf`

**Plan metadata:** to be committed after this file.

## Files Created/Modified

- `internal/data/dump.go` — Three-layer accept-list parser, 64 MiB input guard, NULL sentinel, unistr rejection.
- `internal/data/dump_test.go` — Tests for basic fixture, autoincrement boilerplate, 9 negative cases, control case, unistr/blob/oversize/malicious.
- `internal/data/testdata/dump_basic.sql` — PRAGMA/BEGIN/COMMIT/CREATE INDEX + quote escape + Chinese + NULL + blob fixture.
- `internal/data/testdata/dump_autoincrement.sql` — writable_schema + sqlite_sequence trio fixture.
- `internal/data/testdata/dump_unistr.sql` — unistr() literal fixture.
- `internal/data/testdata/dump_malicious.sql` — Basic dump with appended DROP TABLE + ATTACH.
- `internal/data/preview.go` — `SessionStore`, `PreviewSession`, `RowPair`, `ValidateCommitAgainstPreview`, TTL/capacity constants.
- `internal/data/preview_test.go` — Save/Get round-trip, deep-copy snapshot, TTL miss, capacity eviction, hash bridge.

## Decisions Made

- Kept the package-level `fmtError("Fn", err)` wrapper for `ParseDump` and `ValidateCommitAgainstPreview` to match the rest of `internal/data`; tests assert on the required error substrings per the plan's "含" wording.
- Used an in-band non-printable sentinel (`\x00NULL\x00`) for SQL NULL rather than an empty string, avoiding ambiguity with legitimate blank cells.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 15-05 (importer orchestration) can call `ParseDump` for the dump channel and `SessionStore` + `ValidateCommitAgainstPreview` for the preview/commit flow.
- Plan 15-06 (server API) can rely on `PreviewSession` fields for the preview response and the hash bridge for the commit request.

## Self-Check: PASSED

- [x] Created files exist: `internal/data/dump.go`, `internal/data/dump_test.go`, `internal/data/preview.go`, `internal/data/preview_test.go`, and the four testdata fixtures.
- [x] All task commits exist and are reachable: `4df7556`, `2b02e33`, `958e7a9`, `6c37fbf`.
- [x] `go test ./internal/data/... -count=1` green under WSL2 Ubuntu-22.04.
- [x] `go vet ./internal/data/...` green.
- [x] `internal/data/dump.go` contains `func ParseDump(`, `DumpMaxInputBytes = 64 << 20`, and no `database/sql` / `Exec` / `Query` references.
- [x] `internal/data/preview.go` contains `PreviewSessionTTL = 30 * time.Minute`, `PreviewSessionMaxEntries = 64`, and `func ValidateCommitAgainstPreview(`.

---
*Phase: 15-p10-1*
*Completed: 2026-08-23*
