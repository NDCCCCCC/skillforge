---
phase: 14-p10-0-cgo-trust-floor
plan: 05
subsystem: database / duckdb
objective: "SC-3 connector half: go-duckdb v2.4.3 hermetic boot-queries connector + WAL flush discipline + dataset ID injection guard + write-capacity microbenchmark gates."
tags: [PLAT-09, duckdb, cgo, hermetic, appender, WSL]

requires:
  - phase: 14-03
    provides: "MSYS2 gcc CGO toolchain gate + mattn sqlite precedent"

provides:
  - "internal/duckdb package: NewConnector boot-queries (per-connection hermetic SETs), SetMaxIdleConns(0), Close() double-close WAL flush"
  - "ValidateDatasetID allowlist + AttachDataset READ_ONLY channel (T-14-12/T-14-13 mitigations)"
  - "SC-3 four named test locks (10-lock series 6/7/8 + CloseFlushesWAL) + bonus AttachDataset injection lock"
  - "Write-capacity gates executable: Appender throughput + 50-reader concurrency, env-overridable thresholds"

affects:
  - internal/duckdb
  - 14-06 (attach LRU + staging build on this connector)
  - build-toolchain (Windows duckdb tests moved to WSL)

tech-stack:
  added:
    - "github.com/marcboeker/go-duckdb/v2 v2.4.3 (CGO DuckDB driver, MIT, DuckDB engine v1.4.1)"
  patterns:
    - "boot callback runs hermetic SETs under context.Background() — binding to a request ctx would poison lazily-opened connections"
    - "Threshold constants overridable via env (DUCKDB_APPENDER_MIN_RPS / DUCKDB_READER_BLOCK_FACTOR) for CI宽松档"

key-files:
  created:
    - internal/duckdb/connector.go
    - internal/duckdb/connector_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "duckdb tests run in WSL Ubuntu-22.04, not Windows: duckdb-go-bindings/windows-amd64@v0.1.21 prebuilt libs are emutls-toolchain-built and reference __emutls_v._ZSt11__once_call symbols that MSYS2 native-TLS libstdc++ cannot provide (ABI mismatch, upstream duckdb-go-bindings#105 unresolved). Windows-side go build ./... will fail on internal/duckdb from now on — Windows verification is build+vet excluding that package; WSL/Docker/Linux CI is the duckdb test surface"
  - "WSL toolchain installed 2026-08-22: build-essential (gcc 11.4.0) + Go 1.25.0 + GOPROXY=goproxy.cn; run via `MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c '...go test ./internal/duckdb/...'`"
  - "Explicit-close-only style inside test phases (no defer alongside explicit Close) — defer + explicit double-close produced 'already closed' errors in cleanup"

patterns-established:
  - "Hermetic boot-queries connector: single source of connection semantics for Phase 15 import pipeline and Phase 16 KPI channel"

requirements-completed:
  - PLAT-09

# Metrics
duration: 75min (including environment blocker investigation + WSL toolchain setup)
completed: 2026-08-22

---

# Plan 14-05: DuckDB Connector Summary

**Hermetic go-duckdb v2.4.3 connector locked by six tests — Windows-native linking proven impossible (emutls ABI), test surface moved to WSL Linux.**

## Measured Values (SC-3 capacity gates, WSL Ubuntu-22.04, 2026-08-22)

| Gate | Threshold | Measured | Result |
|------|-----------|----------|--------|
| Appender throughput | ≥100,000 rows/s | **638,094 rows/s** (500k rows, 10 mixed columns) | PASS 6.4× |
| 50 concurrent readers vs writer | factor <3.00× | **1.53×** (write alone 158ms; with readers 242ms) | PASS |
| Boot queries per connection | 4 conns × 2 settings all false | all false | PASS |
| WAL flush on double-close | reopen row count == 10000 | 10000 | PASS |

## Accomplishments
- go-duckdb v2.4.3 introduced as second CGO dependency; connector.go is the single import point (tidy-safe)
- All four SC-3 named locks + bonus AttachDataset injection lock green in WSL
- Full-repo zero regression: `go test ./... -count=1` exit 0 (28 packages) in WSL; Windows-side build+vet green for all packages except internal/duckdb

## Known Limitation (documented for open-source users)
`internal/duckdb` **cannot link on Windows-native MSYS2 gcc** — prebuilt Windows bindings are emutls-built (upstream issue #105, zero replies). Development workflow: duckdb tests run in WSL2/Docker/Linux CI; Windows devs run `go build $(go list ./... | grep -v internal/duckdb)`.

## Task Commits

1. **Task 1: connector + boot-queries + validateDatasetID + WAL flush** - `5bb2765` (feat)
2. **Task 2: capacity microbenchmark gates (throughput + 50-reader concurrency)** - `a8dcc48` (test, same file scope — see deviations)

**Plan metadata:** this SUMMARY commit (docs)

## Deviations from Plan
1. **TDD RED→GREEN sequence unrecoverable**: a prior session wrote all code but was terminated by the Windows emutls linker error before any commit existed. Reconstructing RED commits for already-green code was judged theater; both tasks landed as feat→test commits with the deviation recorded here.
2. **Task 2 has no separate file scope** — both benchmark tests live in connector_test.go alongside Task 1 tests; split into implementation commit (connector.go + go.mod/go.sum) + test commit (connector_test.go) instead of per-task commits.
3. **Inline execution instead of gsd-executor subagent**: all Agent() spawns fail with 400 [1214] modelCode in this session (known bigmodel proxy issue — settings.json fixed but requires session restart). Orchestrator executed inline per execute-phase runtime fallback rule.
4. Prior-session residue (connector.go/connector_test.go/go.mod) was verified against the plan and kept; only the defer double-close cleanup bug in TestDuckDBConcurrentReadNoBlockWrite was fixed (3 lines removed).

## Self-Check: PASSED
- [x] All 6 tests green in WSL (`go test ./internal/duckdb/ -count=1 -timeout 300s`)
- [x] Full-repo zero regression in WSL (exit 0, 28 ok packages)
- [x] Windows build+vet green excluding internal/duckdb
- [x] `grep -c "SetMaxIdleConns(0)" connector.go` = 1; `grep -c "autoinstall_known_extensions" connector.go` ≥ 1
- [x] go.mod contains `marcboeker/go-duckdb/v2 v2.4.3`
