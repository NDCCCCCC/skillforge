# Phase 16 Deferred / Out-of-Scope Items

## 2026-08-27 — 16-03 execution discoveries (out of scope, not fixed here)

- **Status:** acknowledged

1. **internal/kpi WIP does not compile on the shared tree** — `internal/kpi/kmin.go:124/:127` reference undefined `jsonNumber` / `errNonNumericCount`. This file is uncommitted work-in-progress owned by plans 16-05/16-06 (interrupted parallel session); per executor scope rules 16-03 did not touch it. Discovered during 16-03's `go build ./...` verification gate; zero packages currently import internal/kpi, so no committed consumer is affected. The 16-05/16-06 executors must land the missing symbols.
2. **`go build ./...` main-package link failure on Windows host** — cmd/server + cmd/eval fail at the LINK stage against duckdb static libs under MSYS2 gcc (`collect2.exe: ld returned 1 exit status`). Pre-existing environment limitation documented in D15-01-1; duckdb-linked suites and binaries run under WSL2. Scoped verification for 16-03 used `go build` over all ./internal/... except kpi (PASS) plus `go test ./internal/store/... -count=1` (PASS).
