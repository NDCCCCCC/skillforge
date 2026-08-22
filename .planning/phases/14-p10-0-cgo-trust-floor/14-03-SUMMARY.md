---
phase: 14-p10-0-cgo-trust-floor
plan: 03
subsystem: database / platform-floor
objective: "CGO trust floor: zero-regression switch from modernc.org/sqlite to mattn/go-sqlite3 v1.14.50 with busy_timeout/WAL/foreign_keys in DSN and a Makefile gcc gate."
tags: [PLAT-10, sqlite, cgo, mattn, busy_timeout, CGO]

requires:
  - phase: 14-02
    provides: "PLAT-10 audit row substrate (platform_floor_version column)"

provides:
  - "mattn/go-sqlite3 v1.14.50 linked successfully under CGO"
  - "DSN-level _busy_timeout=5000 + _journal_mode=WAL + _foreign_keys=1 applied to every connection"
  - "Makefile check-cgo gate with actionable MSYS2 gcc guidance"
  - "TestSqliteMattnZeroRegression 10th store lock (aggregate + concurrency + pool)"

affects:
  - internal/store/sqlite
  - build-toolchain
  - 14-05 (go-duckdb CGO shared toolchain)

tech-stack:
  added:
    - "github.com/mattn/go-sqlite3 v1.14.50 (CGO SQLite3 driver, MIT)"
  removed:
    - "modernc.org/sqlite v1.56.0 (pure-Go driver)"
  patterns:
    - "DSN parameters for per-connection pragmas instead of post-open ExecContext"
    - "in-memory DSN defaults to MaxOpenConns=1 to avoid shared-cache deadlock"

key-files:
  created:
    - internal/store/sqlite/mattn_zero_regression_test.go
  modified:
    - internal/store/sqlite/sqlite.go
    - go.mod
    - go.sum
    - Makefile

key-decisions:
  - "Apply busy_timeout/WAL/foreign_keys to all DSNs (including :memory:) so tests using shared-cache in-memory get busy_timeout and foreign-key enforcement without post-open PRAGMAs"
  - "Treat any in-memory DSN (bare :memory: or file:...?mode=memory) as MaxOpenConns=1 to avoid cross-connection table-lock deadlocks under mattn"
  - "Install mingw-w64-ucrt-x86_64-make to verify the Makefile check-cgo target in this headless environment"

requirements-completed:
  - PLAT-10

# Metrics
duration: 30min
completed: 2026-08-22
---

# Phase 14 Plan 03: CGO SQLite Driver Switch Summary

**mattn/go-sqlite3 v1.14.50 CGO migration with DSN-level busy_timeout/WAL/foreign_keys and a Makefile gcc gate, plus the 10th store regression lock proving zero semantic loss.**

## Performance

- **Duration:** 30 min
- **Started:** 2026-08-22T04:24:00Z
- **Completed:** 2026-08-22T04:54:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Replaced `modernc.org/sqlite` with `github.com/mattn/go-sqlite3 v1.14.50` and switched driver name from `sqlite` to `sqlite3`.
- Moved `busy_timeout=5000`, `journal_mode=WAL`, and `foreign_keys=1` from post-open `PRAGMA` `ExecContext` calls into DSN parameters so every pooled connection inherits them (fixes Pitfall 2).
- Added `Makefile` `check-cgo` target that detects missing `gcc` and prints Windows/MSYS2 installation guidance.
- Added `TestSqliteMattnZeroRegression` covering audit round-trip, checkpoint round-trip, session idempotency, SQLi survival, 8-goroutine concurrent write storm with zero `SQLITE_BUSY`, and the `:memory:`/`file:` connection-pool defaults.
- All existing store tests remain green under the CGO driver.

## Task Commits

Each task was committed atomically:

1. **Task 1: gcc toolchain + mattn dependency + driver swap + DSN pragma migration + Makefile gcc gate** - `3c97666` (feat)
2. **Task 2: TestSqliteMattnZeroRegression aggregate + concurrency + pool lock** - `0b12c9a` (test)

**Plan metadata:** (pending final docs commit)

## Files Created/Modified

- `internal/store/sqlite/sqlite.go` - CGO driver import, `dsnWithPragmas`, `sql.Open("sqlite3", ...)`, updated `withDefaults` for in-memory DSNs.
- `go.mod` / `go.sum` - Added `mattn/go-sqlite3 v1.14.50`, removed `modernc.org/sqlite` and its indirects.
- `Makefile` - Added `check-cgo` target.
- `internal/store/sqlite/mattn_zero_regression_test.go` - New regression test (aggregate/concurrency/pool).

## Decisions Made

- **DSN pragmas for memory too**: The plan specified appending pragmas only to file-backed DSNs. We append them to `:memory:` as well because SQLite ignores `journal_mode=WAL` for memory databases, while `_busy_timeout` and `_foreign_keys` are needed by tests using `file:...?mode=memory&cache=shared`.
- **Single connection for any memory DSN**: The plan retained `:memory:` → `MaxOpenConns=1`. We expanded this to any DSN containing `mode=memory` because mattn's shared-cache in-memory database deadlocks across multiple pooled connections even with `busy_timeout`.
- **Install MSYS2 make**: Git Bash on this machine has no `make`; we installed `mingw-w64-ucrt-x86_64-make` so the `check-cgo` target could be executed and verified.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Concurrent in-memory store test failed with "database table is locked"**
- **Found during:** Task 1 verification (`go test ./internal/store/...`)
- **Issue:** `file:...?mode=memory&cache=shared` with `MaxOpenConns=4` caused cross-connection table-lock deadlocks under mattn; `busy_timeout` alone cannot resolve deadlocks.
- **Fix:** Updated `withDefaults` to set `MaxOpenConns=1` for any in-memory DSN (`isMemoryDSN`), not just bare `:memory:`.
- **Files modified:** `internal/store/sqlite/sqlite.go`
- **Verification:** `go test ./internal/store/... -count=1` PASS.
- **Committed in:** `3c97666` (Task 1)

**2. [Rule 3 - Blocking] Cascade-delete test failed because foreign_keys was not enabled for :memory:**
- **Found during:** Task 1 verification (`TestAuditMetaCascadeDelete`)
- **Issue:** Removing the post-open `PRAGMA foreign_keys=ON` left `:memory:` databases with foreign keys disabled, so `ON DELETE CASCADE` did not fire.
- **Fix:** Append `_foreign_keys=1` to the DSN for `:memory:` databases (via `dsnWithPragmas`) instead of relying on post-open PRAGMAs.
- **Files modified:** `internal/store/sqlite/sqlite.go`
- **Verification:** `TestAuditMetaCascadeDelete` PASS.
- **Committed in:** `3c97666` (Task 1)

**3. [Rule 3 - Blocking] Git Bash pipe broke CGO build (`cgo.exe: exit status 2`)**
- **Found during:** Task 1 verification (`go build ./...`)
- **Issue:** Invoking the Go toolchain through the Git Bash pipe caused `cgo.exe` to exit 2 silently; the same commands succeeded under an MSYS2 `bash -lc` environment.
- **Fix:** Used `/c/msys64/usr/bin/bash.exe -lc` with explicit `PATH`, `GOPATH`, `GOMODCACHE`, and `GOCACHE` for all CGO-enabled `go build`/`go vet`/`go test` invocations.
- **Files modified:** None (execution environment workaround)
- **Verification:** Full `go build ./... && go vet ./... && go test ./... -count=1` PASS under MSYS2 bash.
- **Committed in:** N/A (runtime environment fix)

**4. [Rule 3 - Blocking] `make check-cgo` could not run because `make` is not installed**
- **Found during:** Task 1 verification (`make check-cgo`)
- **Issue:** Git Bash has no `make` binary; the acceptance criteria expected `make check-cgo`.
- **Fix:** Installed `mingw-w64-ucrt-x86_64-make` via MSYS2 pacman and verified the target with `mingw32-make.exe check-cgo`.
- **Files modified:** None (build-tool environment)
- **Verification:** `mingw32-make.exe check-cgo` prints gcc version and exits 0.
- **Committed in:** N/A (environment setup)

---

**Total deviations:** 4 handled (1 Rule 1 bug, 3 Rule 3 blockers)
**Impact on plan:** All fixes were required for the CGO migration to pass existing tests. No scope creep.

## Issues Encountered

- **MSYS2 gcc already installed but `make` missing** — resolved by installing the UCRT64 make package.
- **Git Bash + CGO pipe interaction** — resolved by routing CGO commands through MSYS2 `bash -lc`.
- **Shared-cache in-memory deadlock under mattn** — resolved by forcing a single connection for memory DSNs.

## User Setup Required

None beyond the plan's existing `msys2-gcc` service, which was already present on this machine.

## Next Phase Readiness

- CGO toolchain is ready for Phase 14 Plan 05 (`go-duckdb` dependency introduction).
- The `check-cgo` Makefile gate gives future developers a clear failure path when `gcc` is missing.

---
*Phase: 14-p10-0-cgo-trust-floor*
*Completed: 2026-08-22*

## Self-Check: PASSED

- [x] `14-03-SUMMARY.md` created at `.planning/phases/14-p10-0-cgo-trust-floor/14-03-SUMMARY.md`
- [x] All commits (`3c97666`, `0b12c9a`, `9f303d5`) exist in `git log`
- [x] Created file `internal/store/sqlite/mattn_zero_regression_test.go` exists
- [x] `go build ./... && go vet ./... && go test ./... -count=1` all PASS under MSYS2 bash with CGO enabled