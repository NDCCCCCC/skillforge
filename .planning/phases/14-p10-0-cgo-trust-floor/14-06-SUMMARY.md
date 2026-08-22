---
phase: 14-p10-0-cgo-trust-floor
plan: 06
subsystem: database / duckdb / dataset lifecycle
objective: "Dataset lifecycle foundations: lazy ATTACH LRU (DETACH-first eviction), staging atomic rename (os.Rename, no half-states), sqlite datasets registry (same-db OLTP semantics)."
tags: [PLAT-09, duckdb, LRU, staging, atomic-rename, sqlite, datasets]

requires:
  - phase: 14-05
    provides: "internal/duckdb connector + AttachDataset/ValidateDatasetID + dedicated-connection pattern"

provides:
  - "AttachLRU: lazy ATTACH + LRU cap 8 over a dedicated driver connection (DETACH before bookkeeping removal — Windows lock boundary)"
  - "CommitStagedDataset: same-volume atomic rename + registry write with rollback compensation; ErrDatasetExists replay guard"
  - "sqlite datasets registry (PutDataset/ListDatasets) + DatasetRow; data/ directory convention fixed (git-ignored)"
  - "10-lock series complete: 9a-9d LRU locks + #10 TestStagingAtomicRename four faces"

affects:
  - internal/duckdb
  - internal/store (schema.go, store.go, sqlite/datasets.go)
  - 14-07 (verification gate consumes these locks)
  - Phase 15 import pipeline (consumes all three modules directly)

tech-stack:
  added: []  # zero new dependencies (T-14-SC accept)
  patterns:
    - "Dedicated driver connection owns the ATTACH catalog (DuckDB ATTACH is session-scoped)"
    - "Compensating rollback for fs+sqlite approximate transactions (Phase 19 TOCTOU pattern pre-staged)"
    - "INSERT OR REPLACE idempotent registry upsert"

key-files:
  created:
    - internal/duckdb/attach_lru.go
    - internal/duckdb/attach_lru_test.go
    - internal/duckdb/staging.go
    - internal/duckdb/staging_test.go
    - internal/store/sqlite/datasets.go
  modified:
    - internal/store/sqlite/schema.go
    - internal/store/store.go
    - .gitignore

key-decisions:
  - "AttachLRU holds a SINGLE dedicated driver connection: DuckDB ATTACH is session-scoped, so one fixed session keeps the attached-catalog deterministic across Get/Detach/AttachedIDs (plan did not specify; this is the session-visibility design decision)"
  - "Eviction runs DETACH BEFORE map/list removal — a failed DETACH keeps the entry (conservative; alias may still be live). DETACH-before-any-os.Remove documented as caller contract (Pitfall 4 / T-14-17)"
  - "CommitStagedDataset registry-write failure compensates by renaming the file back to staging; the restored staging file is PRESERVED for retry (not cleaned) — complete batch, not residue"
  - "MaxAttachedDatasets=8 (plan discretion; Phase 13 fanout LRU=20 as magnitude reference, dataset files are heavier OS resources)"

patterns-established:
  - "datasets/ invariant: complete AND registered — enforced by target-exists sentinel + rollback compensation"

requirements-completed:
  - PLAT-09

# Metrics
duration: 55min
completed: 2026-08-22

---

# Plan 14-06: Dataset Lifecycle Summary

**Lazy ATTACH LRU + atomic staging commit + sqlite registry — the 10-lock series is complete (9a-9d + #10), datasets/ can never hold a half-state.**

## Test Results (WSL Ubuntu-22.04, 2026-08-22)

| Lock | Face | Result |
|------|------|--------|
| 9a TestAttachLRUCap | cap 3 + 4th attach evicts LRU engine-side (duckdb_databases) | PASS |
| 9b TestAttachLRUTouchReorder | hit re-touch survives; untouched entry evicted | PASS |
| 9c TestAttachLRUInvalidID | `../escape` rejected, zero ATTACH executed | PASS |
| 9d TestAttachLRUDetachIdempotent | not-attached no-op + double-Detach safe | PASS |
| #10 TestStagingAtomicRename | normal / interrupted / target-exists(+open handle) / idempotent-replay | PASS ×4 |

Full-repo: `go test ./... -count=1` exit 0 (28 packages) in WSL; Windows build+vet green excluding internal/duckdb (known emutls limitation, see 14-05-SUMMARY).

## Accomplishments
- LRU eviction is asserted against ENGINE truth (duckdb_databases()), not just bookkeeping — proves the file handle actually released
- datasets registry participates in the same sqlite transaction domain as audit/checkpoint (D14-D2)
- data/staging + data/datasets directory convention fixed and git-ignored; Phase 15 import pipeline consumes directly

## Task Commits

1. **Task 1: lazy ATTACH LRU** - RED `attach_lru_test.go` + GREEN `attach_lru.go` (feat)
2. **Task 2: staging + registry** - RED `staging_test.go` + GREEN `5` files (feat)

**Plan metadata:** this SUMMARY commit (docs)

## Deviations from Plan
1. CommitStagedDataset takes a CommitOptions struct with an extra DatasetsDir override (tests need temp dirs; default constant preserved) — plan's opts sketch had StagingPath/DatasetID/Row/store/lru; struct shape is the same seam plus the dir override.
2. Bonus guard added beyond plan: Row.Name must equal DatasetID (registry/file-name identity check) and nil-Registry fast error.
3. Inline execution (subagent spawns unavailable this session — see 14-05-SUMMARY deviation 3).
4. Fixed a self-inflicted raw-string bug mid-task (backtick inside the schemaSQL SQL comment terminated the Go raw literal; caught by vet, fixed before commit — zero impact on any commit).

## Self-Check: PASSED
- [x] All 8 new tests green in WSL; full repo 28 packages zero regression
- [x] `CREATE TABLE IF NOT EXISTS datasets` count == 1; migrate() untouched (git diff empty)
- [x] os.Rename ≥1 in staging.go; zero two-phase-copy marker pattern
- [x] datasets.go zero fmt.Sprintf SQL; MaxAttachedDatasets = 8 source-grep locked
- [x] DETACH-before-removal order verified in eviction + Detach paths (review assertion)
- [x] .gitignore covers data/
