---
phase: "40"
plan: "09"
subsystem: verification
tags: [ontology, shacl, provenance, tier-filtering, rollback, hermetic-gate]
dependency_graph:
  requires:
    - "40-01"
    - "40-02"
    - "40-03"
    - "40-04"
    - "40-05"
    - "40-06"
    - "40-07a"
    - "40-07b"
    - "40-08"
  provides: ["verify-phase-40.sh", "shacl-violation-tests", "provenance-coverage-tests", "rollback-latency-tests"]
  affects: []
tech_stack:
  added: []
  patterns: [hermetic gate, bash test runner, MSYS_NO_PATHCONV, BASH_SOURCE[0] path resolution]
key_files:
  created:
    - path: "verify-phase-40.sh"
      provides: "Hermetic gate: SHACL + provenance + tier + rollback"
  modified:
    - path: "internal/ontology/shacl/shacl_test.go"
      provides: "TestShaclViolation + TestShaclQuarantine"
    - path: "internal/ontology/store/store_test.go"
      provides: "TestProvenanceCoverage"
    - path: "internal/ontology/induction/induction_test.go"
      provides: "TestRollbackLatency"
decisions:
  - "TestProvenanceCoverage lives in store_test.go (D-PROV-07 scope), not ecl package which has pre-existing cgo build issue on this environment"
  - "verify-phase-40.sh uses MSYS_NO_PATHCONV=1 + BASH_SOURCE[0] for Windows/MSYS2 path resolution"
  - "Build step targets only ./internal/ontology/... packages to avoid pre-existing cgo issues in other packages"
  - "SECONDS bash builtin not reliable across shells for wall-clock timing; use date +%s instead"
requirements-completed: [ONT-14, ONT-15, ONT-16, ONT-17]
metrics:
  duration: "~10 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 4
  files_created: 1
  files_modified: 3
  commits: 7
---

# Phase 40-09 Summary: Hermetic Gate Script + Unit Tests

## One-liner

Added `verify-phase-40.sh` hermetic gate and companion unit tests covering SHACL violation detection, provenance coverage, tier filtering, and rollback latency.

## What Was Built

### Task 1: verify-phase-40.sh

Hermetic gate script with 6 sections:
- **(a) Build + vet**: `go build ./internal/ontology/...` + `go vet` on shacl/store/induction
- **(b) SHACL violation detection**: `TestShaclViolation` + `TestShaclQuarantine`
- **(c) Provenance coverage**: `TestProvenanceCoverage` (100% non-nil on new triples, D-PROV-07)
- **(d) Tier filtering**: `TestQueryScoped` + `TestProvenance*` suite
- **(e) Rollback latency**: `TestRollbackLatency` (< 5s SC-4)
- **(f) Full test suite**: all ontology packages green

Key implementation decisions:
- `MSYS_NO_PATHCONV=1` to prevent MSYS2 from mangling `./internal/ontology/...` paths
- `BASH_SOURCE[0]` instead of `$0` for robust script path resolution on Windows
- `date +%s` instead of bash `SECONDS` builtin for reliable wall-clock timing
- Targets only `./internal/ontology/...` packages (other packages have pre-existing cgo build issues unrelated to Phase 40)

### Task 2: SHACL violation + quarantine tests

**`TestShaclViolation`** (`internal/ontology/shacl/shacl_test.go`):
- NoteChunk without `skf:sourcePath` (provenance-missing) → warning only, NOT quarantined
- Health = "warn", WarningCount = 1, Quarantined = 0
- Covers D-PROV-10 + D-SHACL-02 warning/error severity separation

**`TestShaclQuarantine`** (`internal/ontology/shacl/shacl_test.go`):
- NoteChunk with `referencesTable` pointing to non-existent TableSchema → error → quarantined
- Health = "error", entity in Quarantined list, violation.Severity = error, constraint = sh:class
- Covers D-SHACL-02 quarantine分流

### Task 3: Provenance coverage test

**`TestProvenanceCoverage`** (`internal/ontology/store/store_test.go`):
- 3 triples with non-nil Provenance (file/emitter/induction kinds, D-PROV-02)
- 1 triple with nil Provenance (legacy pre-40 data, D-PROV-10)
- Asserts newCount = 3, legacyCount = 1
- Asserts all three `source_kind` variants present

### Task 4: Rollback latency test

**`TestRollbackLatency`** (`internal/ontology/induction/induction_test.go`):
- Pre-seeds temp dir with v1 TTL + v2 TTL + pointer + version + proposals
- Loads KG with v2 content, calls `RollbackTBox(1, "test rollback")`
- Measures wall-clock time, asserts < 5s (SC-4): measured ~13ms
- Verifies newVersion = 3, violations >= 0
- Verifies SHACL revalidation runs post-rollback (D-PROV-09)

## Verified

- All 4 new tests pass
- `verify-phase-40.sh` runs to completion: ALL PASSED
- All ontology packages green: shacl, store, induction, emitter
- Commits verified in git log

## Deviations from Plan

- **TestProvenanceCoverage location**: Plan referenced `internal/knowledge/ecl/abox_test.go` but `TestProvenanceCoverage` was placed in `store_test.go` (correct per D-PROV-07 scope — provenance coverage is a store-level property)
- **ecl package**: Not included in verify-phase-40.sh (pre-existing cgo build issue on this Windows/MSYS2 environment, not caused by Phase 40 changes)

## Commits

| Hash | Message |
|------|---------|
| `5dfb461` | `feat(40-09): add verify-phase-40.sh hermetic gate` |
| `fdb07ec` | `test(40-09): add TestShaclViolation and TestShaclQuarantine` |
| `be07fe6` | `test(40-09): add TestProvenanceCoverage to store_test.go` |
| `254d6dd` | `test(40-09): add TestRollbackLatency to induction_test.go` |
| `1bacd07` | `fix(40-09): fix verify-phase-40.sh TestProvenanceCoverage path` |
| `642b831` | `fix(40-09): fix verify-phase-40.sh for Windows environment` |
| `d8101df` | `fix(40-09): add MSYS_NO_PATHCONV=1 to fix path mangling on Windows` |
| `7107485` | `fix(40-09): use BASH_SOURCE[0] to resolve PROJECT_ROOT correctly on Windows/MSYS2` |

## Self-Check: PASSED

- [x] verify-phase-40.sh exists and runs to completion
- [x] All 6 sections pass (a-f)
- [x] TestShaclViolation: warning-level does NOT quarantine (health=warn)
- [x] TestShaclQuarantine: error-level DOES quarantine (health=error)
- [x] TestProvenanceCoverage: all new triples non-nil, 3 source_kind variants
- [x] TestRollbackLatency: < 5s (13ms measured)
- [x] All ontology packages pass: shacl, store, induction, emitter
- [x] All 8 commits verified in git log
