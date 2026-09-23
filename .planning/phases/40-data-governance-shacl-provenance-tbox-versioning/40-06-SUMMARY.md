---
phase: "40"
plan: "06"
subsystem: governance
tags: [shacl, quarantine, provenance, governance, admin-api]
dependency_graph:
  requires: ["40-01", "40-02", "40-04"]
  provides: ["Quarantine", "QuarantinedEntities", "SetShaclReport", "GetShaclReport", "GET /api/admin/governance/shacl", "GET /api/admin/governance/provenance"]
  affects: ["internal/server/admin_governance.go", "internal/server/server.go", "cmd/server/main.go"]
tech_stack:
  added: []
  patterns: [boot-time SHACL validation, error/warning分流, quarantine isolation, provenance graph construction]
key_files:
  created:
    - path: "internal/server/admin_governance.go"
      provides: "SHACL report handler + provenance graph handler + buildProvenanceGraph"
  modified:
    - path: "internal/ontology/store/store.go"
      provides: "Quarantine map + shaclReport cache + Quarantine/QuarantinedEntities/SetShaclReport/GetShaclReport"
    - path: "cmd/server/main.go"
      provides: "Boot-time SHACL validation call + quarantine分流 + report cache"
    - path: "internal/server/server.go"
      provides: "Route registration for /governance/shacl + /governance/provenance"
decisions:
  - id: "D-SHACL-02"
    text: "error-level violation entities quarantined (moved from main KG); warning-level allowed through; boot never blocks on SHACL violations"
  - id: "D-SHACL-03"
    text: "Three-colour health: error=any quarantined, warn=warnings only, ok=zero violations"
  - id: "D-PROV-09"
    text: "Boot runs SHACL validation on every boot regardless of violations"
metrics:
  duration: "~15 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 2
  files_created: 1
  files_modified: 3
  tests_passed: 31 (store) + 10 (shacl) — no regression
  commits: 2
---

# Phase 40-06 Summary: Governance Admin API + Boot-Time SHACL Validation

## One-liner

Boot-time SHACL validation with error/warning分流 (error entities quarantined, warnings allowed), plus two admin HTTP routes: GET /api/admin/governance/shacl (cached report) and GET /api/admin/governance/provenance (graph including quarantined entities).

## What Was Built

### Task 1: Boot-time SHACL validation + quarantine

**`internal/ontology/store/store.go`**:

Extended `Store` with quarantine state and SHACL report cache:

```go
type Store struct {
    mu          sync.RWMutex
    all         []Triple
    byS         map[string][]int
    byP         map[string][]int
    byO         map[string][]int
    quarantined map[string][]Triple  // entity URI → its triples
    shaclReport atomic.Value         // *shacl.Report, stored as interface{}
}
```

New methods:
- `Quarantine(t Triple)` — removes from main indices (swap-remove + O(n) index rebuild), appends to `quarantined[entity]`
- `QuarantinedEntities() []QuarantinedEntity` — returns entity URIs with triple counts (store-side DTO, avoids circular import with shacl package)
- `SetShaclReport(report interface{})` / `GetShaclReport() interface{}` — atomic cache for the SHACL validation report

**`cmd/server/main.go`** (after KG restore + boot snapshot):

```go
shaclShapesPath := envOr("SKILLFORGE_SHACL_SHAPES",
    filepath.Join("internal", "ontology", "shacl", "shapes.ttl"))
shaclReport, shaclErr := shacl.Validate(ontologyKG, shaclShapesPath)
if shaclErr != nil {
    log.Printf("[WARN] cmd/server: SHACL validation: %v", shaclErr)
} else {
    for _, q := range shaclReport.Quarantined {
        for _, t := range ontologyKG.Query(q.Entity, "", "") {
            ontologyKG.Quarantine(t)
        }
    }
    ontologyKG.SetShaclReport(shaclReport)
    // log health + violation counts
}
```

D-SHACL-02: violations never block boot. SKILLSFORGE_SHACL_SHAPES env var for path configuration.

### Task 2: Governance admin API handlers

**`internal/server/admin_governance.go`**:

Three DTOs:
```go
type ProvenanceGraph struct { Nodes []GraphNode, Edges []GraphEdge }
type GraphNode struct { ID, Kind, Label string; Provenance *store.Provenance; Tier string }
type GraphEdge struct { Source, Target string }
```

Two handlers:
- `adminGovernanceShaclHandler` — GET /api/admin/governance/shacl → returns cached `*shacl.Report` or 503 "report not available"
- `adminGovernanceProvenanceHandler` — GET /api/admin/governance/provenance → returns `ProvenanceGraph` with all nodes including quarantined

`buildProvenanceGraph` algorithm:
1. Walk all KG triples
2. Note nodes (skf:note_N) — emit node + file node from provenance.sourcePath + edges
3. Table nodes (skf:tbl_X) — emit node + edges via referencesTable
4. Skill/Tier/KBCategory individuals — emit schema nodes
5. Quarantined entities — emit quarantined nodes with edges to their notes/tables

**`internal/server/server.go`**:
```
GET /api/admin/governance/shacl       → adminGovernanceShaclHandler
GET /api/admin/governance/provenance  → adminGovernanceProvenanceHandler
```
Registered after changelog/rollback routes, same token-gated (403 before 503) pattern.

## Key Implementation Details

### Circular import avoidance

`shacl` package imports `store`, so `store` cannot import `shacl`. Workarounds:
- `shaclReport` stored as `interface{}` in `Store`; callers type-assert
- `QuarantinedEntity` defined as a store-side DTO (not reusing `shacl.QuarantinedEntity`)
- `buildProvenanceGraph` in `admin_governance.go` accesses quarantine via `QuarantinedEntities()` which returns the store DTO

### Quarantine mechanics

`Quarantine` uses swap-remove on the `all` slice (O(1) remove, O(n) index rebuild). This mirrors `DeleteSubject`'s compaction pattern — the index rebuild is O(n) but acceptable at KG scale.

### Boot sequence integration

SHACL validation runs after KG snapshot restore + boot snapshot write, before the induction manager is constructed. This matches the plan's boot sequence: LoadSnapshot → [SHACL] → TBox灌入 → snapshot.

## Verified

- `go vet ./internal/ontology/store/` — clean
- `go test ./internal/ontology/store/ -v` — 31/31 PASS (no regression)
- `go test ./internal/ontology/shacl/ -v` — 10/10 PASS
- Both commits verified in git log

## Deviations from Plan

- **Circular import workaround**: shaclReport stored as `interface{}` rather than `*shacl.Report` directly; `QuarantinedEntity` duplicated as store-side DTO
- **SKILLFORGE_SHACL_SHAPES env var default**: resolved relative to repo root as `filepath.Join("internal", "ontology", "shacl", "shapes.ttl")` rather than relative to dataDir (boot runs from repo root)

## Commits

- `584b58b` — `feat(40-06): add quarantine state + boot-time SHACL validation`
- `156a23e` — `feat(40-06): add governance admin API handlers + route registration`

## Self-Check: PASSED

- `internal/server/admin_governance.go` created with both handlers
- `internal/server/server.go` has two new routes registered
- `cmd/server/main.go` has boot-time SHACL validation call
- `internal/ontology/store/store.go` has quarantine map + Quarantine() + QuarantinedEntities() + SetShaclReport() + GetShaclReport()
- Both commits verified in git log
- 31 store tests pass, 10 shacl tests pass
