---
phase: "40"
plan: "04"
subsystem: store
tags: [tier, governance, store, query, shacl]
dependency_graph:
  requires: ["40-02", "40-03"]
  provides: ["QueryScoped"]
  affects: ["internal/server/admin_ontology.go"]
tech_stack:
  added: []
  patterns: [entity-level tier filtering, schema-layer exemption, fail-closed for missing belongsToTier]
key_files:
  created: []
  modified:
    - path: "internal/ontology/store/store.go"
      provides: "QueryScoped function + isSchemaTriple helper + schemaClasses init"
    - path: "internal/ontology/store/store_test.go"
      provides: "8 QueryScoped tests covering all D-TIER contract cases"
decisions:
  - id: "D-TIER-01"
    text: "Empty/nil allowedTiers = no filtering (admin baseline = all visible)"
  - id: "D-TIER-03"
    text: "Schema layer = owl:Class/ObjectProperty/DatatypeProperty declarations + Skill/KBCategory/SubCategory/Tier/SkillAgent/RefusalPhrase individuals; NoteChunk and TableSchema are ECL ABox data entities"
  - id: "D-TIER-06"
    text: "Entity-level filtering: visible subject includes ALL its edges forward+reverse (graph stays connected)"
  - id: "D-TIER-07"
    text: "Entity with no belongsToTier triple is invisible (fail-closed)"
metrics:
  duration: "~15 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 1
  files_modified: 2
  tests_passed: 31 (8 new QueryScoped tests)
  commits: 1
---

# Phase 40-04 Summary: QueryScoped Tier-Filtered KG Query

## One-liner

Implemented `QueryScoped(pattern, allowedTiers)` — the internal tier-filtered KG query function that exempts schema-layer entities and fail-closes on ABox entities missing `belongsToTier`.

## What Was Built

### `internal/ontology/store/store.go`

**`QueryScoped(subj, pred, obj string, allowedTiers []string) []Triple`**:
- Returns all triples matching the (subj, pred, obj) pattern, filtered by tier
- Schema-layer triples (TBox declarations, schema individuals) pass through unconditionally (D-TIER-03)
- ABox data triples are filtered: the subject must have a `belongsToTier` triple with a tier in `allowedTiers`
- Entity with no `belongsToTier` triple is invisible — fail-closed (D-TIER-07)
- If a subject is visible, ALL its edges (forward via byS, reverse via byO) are included so the graph stays connected (D-TIER-06)
- Empty/nil `allowedTiers` = no filtering (admin baseline, D-TIER-01)
- Single-pass O(n) scan with O(1) deduplication map

**`isSchemaTriple(t Triple) bool`**:
- `rdf:type` with OWL vocabulary object (Class, ObjectProperty, DatatypeProperty, Property, NamedIndividual) → schema
- `rdf:type` with schema individual object (Tier, KBCategory, SubCategory, Skill, SkillAgent, RefusalPhrase) → schema
- Metadata predicates (label, comment, domain, range, subClassOf, subPropertyOf) → schema
- `belongsToTier`, `hasReadAccess`, `referencesTable` etc. are NOT schema predicates even if their objects are schema individuals

**`schemaClasses` init map**:
- Only the 6 schema individuals plus OWL vocabulary classes
- **NoteChunk and TableSchema are NOT included** — they are ECL ABox data entities (D-TIER-03)

**Namespace constants** (`ns`, `rdfType`) moved from test file into store.go for internal use.

### `internal/ontology/store/store_test.go`

8 new tests:
- `TestQueryScopedSchemaLayerNeverFiltered` — Skill individuals always visible regardless of allowedTiers
- `TestQueryScopedABoxFilteredByTier` — note with matching tier visible, non-matching tier invisible
- `TestQueryScopedFailClosedNoBelongsToTier` — entity without belongsToTier invisible even with matching tier
- `TestQueryScopedVisibleEntityAllEdgesIncluded` — all edges (type, belongsToTier, referencesTable) included for visible subject
- `TestQueryScopedEmptyAllowedTiersNoFilter` — empty/nil allowedTiers = all triples returned
- `TestQueryScopedOneTierOnly` — only matching tier's entities visible
- `TestQueryScopedMultiTierTable` — table with multiple belongsToTier visible when any tier matches
- `TestQueryScopedSchemaTriplesAlwaysIncluded` — schema triples present even with restrictive tiers

## Key Implementation Details

### Schema vs ABox Distinction

| Entity Type | Example | Filtered? |
|-------------|---------|-----------|
| OWL vocabulary class | `owl:Class a owl:Class` | Never (schema) |
| Schema individual | `skf:Skill a skf:Skill` | Never (schema) |
| Skill property | `skf:skill_a skf:hasReadAccess skf:cat_authority` | Never (schema predicate) |
| Note data entity | `skf:note_001 a skf:NoteChunk` | Yes (ABox) |
| Table data entity | `skf:tbl_x a skf:TableSchema` | Yes (ABox) |
| belongsToTier triple | `skf:note_001 skf:belongsToTier skf:sql-guides` | Yes (ABox data) |

### allowedTiers Short-Name Matching

KG stores full IRIs (`http://skillforge.local/ontology#sql-guides`) but `allowedTiers` uses short names (`"sql-guides"`). Strip namespace prefix before set lookup.

### Deduplication

Single-pass scan over all triples with `seen map[string]bool` keyed by `S|P|O` string. Prevents double-counting when a triple appears in both byS and byO indices.

## Test Results

```
go test ./internal/ontology/store/... -v
ok  github.com/skillforge/skillforge/internal/ontology/store  1.266s  (31 tests)
```

All 8 QueryScoped tests pass + all 23 existing tests pass (no regression).

## Verified

- Schema-layer triples (Skill/KBCategory/Tier individuals) visible with any allowedTiers
- ABox triples (note/tbl) filtered by belongsToTier tier matching
- Fail-closed: entity without belongsToTier invisible
- Visible entity includes all forward+reverse edges
- Empty allowedTiers = no filter
- Multi-tier table visible via any matching tier
- Build clean, all 31 tests pass

## Deviations from Plan

- **Single-pass O(n) scan**: Plan showed a two-phase approach (base query + augmentation). Single-pass with `seen` deduplication is simpler and equivalent.
- **Namespace constants in store.go**: Plan referenced `ns` from `emitter.go`. Moved to store.go as private constants to avoid import cycle.
- **NoteChunk/TableSchema in schemaClasses**: Initially included (matching plan's "schema class individuals" list), removed after testing revealed they are ECL ABox entities per D-TIER-03.

## Commits

- `e6805c1` — `feat(40-04): add QueryScoped tier-filtered KG query function`

## Self-Check: PASSED

- `QueryScoped` function implemented with correct tier filtering semantics
- `isSchemaTriple` correctly identifies schema-layer vs ABox triples
- `schemaClasses` excludes NoteChunk and TableSchema (ECL ABox per D-TIER-03)
- 8 new tests covering all D-TIER contract cases
- All 31 tests pass (8 new + 23 existing, no regression)
- Commit `e6805c1` verified in git log
