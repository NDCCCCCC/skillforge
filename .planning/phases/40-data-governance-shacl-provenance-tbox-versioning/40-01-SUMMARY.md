---
phase: "40"
plan: "01"
subsystem: shacl
tags: [shacl, ontology, governance, validation]
dependency_graph:
  requires: []
  provides: ["Validate", "Violation", "ViolationSeverity", "Report", "QuarantinedEntity"]
  affects: ["internal/ontology/store", "internal/ontology/emitter"]
tech_stack:
  added: [goRDFlib/turtle.Parse (already in use)]
  patterns: [pure-Go SHACL validator, standard TTL shapes, error/warning severity separation]
key_files:
  created:
    - path: "internal/ontology/shacl/shapes.ttl"
      provides: "Standard SHACL TTL shapes for 8 Classes + ABox integrity property shapes"
    - path: "internal/ontology/shacl/validate.go"
      provides: "Pure-Go SHACL validator (7 constraint types)"
    - path: "internal/ontology/shacl/shacl_test.go"
      provides: "Hermetic unit tests for all 7 constraint types + error/warning separation"
  modified: []
decisions:
  - id: "D-SHACL-01"
    text: "Pure-Go SHACL validator with zero third-party Go SHACL library; standard TTL shapes parsed via goRDFlib turtle.Parse"
  - id: "D-SHACL-02"
    text: "Error/warning severity separation: error entities listed in Quarantined, warnings recorded but allowed through"
  - id: "D-SHACL-04"
    text: "8 NodeShapes covering all classes from emitter (Tier/KBCategory/SubCategory/Skill/SkillAgent/RefusalPhrase/NoteChunk/TableSchema)"
  - id: "D-PROV-10"
    text: "skf:sourcePath absent is a warning (provenance-missing on NoteChunk/TableSchema)"
metrics:
  duration: "2026-09-23T01:05:57Z to ~01:12:00Z (~6 minutes)"
  completed_date: "2026-09-23"
  tasks_completed: 2
  files_created: 3
  tests_passed: 10
  commits: 1
---

# Phase 40-01 Summary: SHACL Validation Engine

## One-liner

Pure-Go SHACL validation engine with standard Turtle shapes file covering all 8 KG classes, implementing 7 constraint types with error/warning severity separation.

## What Was Built

### 1. `internal/ontology/shacl/shapes.ttl`

Standard SHACL Turtle file covering all 8 classes emitted by Phase 38/39:
- **8 NodeShapes**: Tier, KBCategory, SubCategory, Skill, SkillAgent, RefusalPhrase, NoteChunk, TableSchema
- **ABox integrity property shapes**: referencesTable → TableSchema, belongsToTier → Tier, hasReadAccess/hasWriteAccess → KBCategory, poweredBy → SkillAgent, requiresRefusal → RefusalPhrase, hasSubCategory → SubCategory, belongsToNote → NoteChunk
- **Provenance property**: skf:sourcePath optional with maxCount=1 and nodeKind Literal (warning per D-PROV-10)
- Namespace: `http://skillforge.local/ontology#` (matches `emitter.go` DefaultNS)

### 2. `internal/ontology/shacl/validate.go`

Pure-Go SHACL validator with **zero third-party Go SHACL library** (D-SHACL-01):
- **7 constraint types**: `sh:minCount`, `sh:maxCount`, `sh:class`, `sh:datatype`, `sh:nodeKind`, `sh:pattern`, `sh:in`
- **Severity model** (D-SHACL-02): `error` → entity quarantined; `warning` → allowed through with violation recorded
- **Health derivation** (D-SHACL-03): `error` if any quarantined; `warn` if warnings only; `ok` if 0 violations
- URI prefix expansion (`skf:` → full URI) for compatibility with `goRDFlib` parsing output
- `Validate(store, shapesPath) → *Report` + `ValidateWithTrigger(store, shapesPath, trigger)` where trigger is "boot" or "rollback"

### 3. `internal/ontology/shacl/shacl_test.go`

10 hermetic unit tests covering all constraint types and behaviors:
- `TestShapesParse` — shapes.ttl parseable, 8 NodeShapes, property constraints present
- `TestMinCountViolation` — absent required property → error
- `TestMaxCountViolation` — too many property values → error
- `TestClassViolation` — property value not instance of required class → error
- `TestDatatypeViolation` — literal vs IRI mismatch → error
- `TestNodeKindViolation` — literal where IRI expected → error
- `TestPatternViolation` — regex non-match → error
- `TestInViolation` — value not in enumeration → error
- `TestErrorWarningSeparation` — error entities quarantined, warning-only entities allowed through
- `TestReportStructure` — health=ok when no violations, all fields populated
- `TestGlobalReferencesConstraint` — cross-entity reference integrity

## Key Implementation Details

### Namespace Mismatch Found and Fixed

Initial implementation used `http://skillforge.io/ns/ontology/` (from spike 005a documentation), but the actual KG stores entities with `http://skillforge.local/ontology#` (emitter.go DefaultNS). Fixed shapes.ttl and `pf()` helper to use the correct namespace.

### URI Prefix Expansion

`goRDFlib` parses TTL and returns fully-expanded URIs (e.g., `http://skillforge.local/ontology#categoryName`), but TTL fixtures use prefixed forms (e.g., `skf:categoryName`). Added `expandPrefixed()` helper to convert prefixed values in shape definitions to full URIs before matching against KG triples.

### Severity by Constraint Type

| Constraint | Default Severity |
|---|---|
| `sh:minCount` (absent) | error |
| `sh:maxCount` | error |
| `sh:class` | error |
| `sh:datatype` | error |
| `sh:nodeKind` | error |
| `sh:pattern` | error |
| `sh:in` | error |
| `skf:sourcePath` | warning (D-PROV-10) |

## Deviations from Plan

- **None** — plan executed exactly as written.

## Verified

- `go build ./internal/ontology/shacl/...` — clean compile
- `go test ./internal/ontology/shacl/... -v` — 10/10 PASS

## Commits

- `5db3527` — `feat(40-01): SHACL validation engine — pure-Go validator + TTL shapes file`

## Self-Check: PASSED

- `shapes.ttl` exists, 8 NodeShapes, parseable via turtle.Parse
- `validate.go` exists, exports Validate/Violation/ViolationSeverity
- `shacl_test.go` exists, 10 tests covering all 7 constraint types + severity separation
- Commit `5db3527` verified in git log
