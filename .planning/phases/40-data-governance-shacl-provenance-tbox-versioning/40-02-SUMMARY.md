---
phase: "40"
plan: "02"
subsystem: store
tags: [provenance, ontology, store, governance]
dependency_graph:
  requires: []
  provides: ["Triple.Provenance", "QueryResultsWithProvenance"]
  affects: ["internal/knowledge/ecl/abox.go"]
tech_stack:
  added: []
  patterns: [Provenance struct with JSON omitempty, backward-compatible snapshot loading]
key_files:
  created: []
  modified:
    - path: "internal/ontology/store/store.go"
      provides: "Triple.Provenance field + Provenance struct + QueryResultsWithProvenance method"
    - path: "internal/ontology/store/store_test.go"
      provides: "9 provenance tests covering JSON roundtrip, backward compat, empty result"
decisions:
  - id: "D-PROV-01"
    text: "Triple carries optional Provenance struct; json omitempty for backward snapshot compatibility"
  - id: "D-PROV-02"
    text: "source_kind enum: file | emitter | induction"
  - id: "D-PROV-05"
    text: "LineStart/LineEnd are 1-based, 0 means N/A, omitted from JSON when zero"
  - id: "D-PROV-10"
    text: "Pre-40 snapshots load normally; triples without Provenance are treated as legacy (warning at SHACL time)"
  - id: "D-PROV-11"
    text: "ExtractedAt RFC3339 UTC from server clock"
metrics:
  duration: "~2 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 2
  files_modified: 2
  tests_passed: 24
  commits: 1
---

# Phase 40-02 Summary: Provenance Struct on Triple

## One-liner

Extended the Triple struct with an optional Provenance field carrying source_kind / source_path / line range / extracted_at, fully backward-compatible with pre-40 JSON snapshots.

## What Was Built

### `internal/ontology/store/store.go`

**Provenance struct** (added after Triple struct):
```go
type Provenance struct {
    SourceKind  string `json:"source_kind"`            // "file" | "emitter" | "induction"
    SourcePath  string `json:"source_path"`            // file path | "emitter:v{N}" | proposal id
    LineStart   int    `json:"line_start,omitempty"`   // 1-based, 0 if N/A
    LineEnd     int    `json:"line_end,omitempty"`     // 1-based, 0 if N/A
    ExtractedAt string `json:"extracted_at"`           // RFC3339 UTC
}
```

**Triple extension**:
```go
type Triple struct {
    S          string       `json:"s"`
    P          string       `json:"p"`
    O          string       `json:"o"`
    Provenance *Provenance `json:"provenance,omitempty"`
}
```

**QueryResultsWithProvenance**: convenience method returning the same triples as `Query` but with the `Provenance` field accessible to callers. The different return type lets the compiler enforce provenance-aware handling.

**Query fix**: `Query` now initializes the output slice (`make([]Triple, 0, len(cands))`) so empty results return `[]` instead of `nil`, enabling callers to range without nil checks.

### `internal/ontology/store/store_test.go`

9 new tests:
- `TestProvenanceNilOmitsFromJSON` — nil Provenance absent from JSON output
- `TestProvenanceAllFieldsMarshaled` — all four Provenance fields roundtrip
- `TestProvenanceLineEndZeroOmitted` — zero line numbers omitted (omitempty)
- `TestLoadSnapshotOldFormatNoProvenance` — pre-40 JSON loads cleanly (D-PROV-10)
- `TestLoadSnapshotNewFormatWithProvenance` — new-format roundtrip preserves all fields
- `TestSnapshotBackwardCompatibility` — mixed old+new triples in one snapshot
- `TestQueryResultsWithProvenanceIdenticalToQuery` — helper returns identical results
- `TestQueryResultsWithProvenanceEmptyResult` — empty result is `[]`, not nil
- `TestProvenanceAllSourceKinds` — all three source_kind variants roundtrip

## Key Decisions

| ID | Decision |
|----|----------|
| D-PROV-01 | Provenance optional via pointer + `omitempty` — nil triples carry no overhead |
| D-PROV-02 | `source_kind` restricted to `file\|emitter\|induction` (D-PROV-02 locked enum) |
| D-PROV-05 | Line numbers 1-based; 0 means N/A and is omitted from JSON |
| D-PROV-10 | Pre-40 snapshots load without migration; legacy triples have `nil` Provenance (warning at SHACL time per D-PROV-10) |
| D-PROV-11 | `extracted_at` is caller-provided RFC3339 string; store does not set clock |

## Verified

- `go build ./internal/ontology/store/...` — clean
- `go test ./internal/ontology/store/... -v` — **24/24 PASS**

## Deviations from Plan

- **None** — plan executed exactly as written.

## Commits

- `faffb00` — `feat(40-02): add Provenance struct to Triple and QueryResultsWithProvenance method`

## Self-Check: PASSED

- `Triple.Provenance *Provenance` field present with `json:"provenance,omitempty"`
- `Provenance` struct with all 5 fields (source_kind/source_path/line_start/line_end/extracted_at)
- `QueryResultsWithProvenance` method on `*Store`
- 9 new tests all PASS
- Commit `faffb00` verified in git log
- All 24 tests in the package pass
