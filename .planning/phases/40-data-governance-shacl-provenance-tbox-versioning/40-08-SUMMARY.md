---
phase: "40"
plan: "08"
subsystem: admin-ontology
tags: [ontology, governance, tier, skill, provenance, kg-query]
dependency_graph:
  requires: ["40-04"]
  provides: ["skill-param-api", "kg-query-panel-skill-selector", "kg-query-panel-provenance-column"]
  affects: ["internal/server/admin_ontology.go", "web/src/components/admin/ontology/KGQueryPanel.tsx"]
tech_stack:
  added: []
  patterns: [skill-param tier filtering, QueryScoped, source_kind badge, 来源 column]
key_files:
  created: []
  modified:
    - path: "internal/server/admin_ontology.go"
      provides: "skill= query param + resolveSkillTiers + QueryScoped integration"
    - path: "web/src/components/admin/ontology/KGQueryPanel.tsx"
      provides: "Skill selector + 来源 column + summary line"
    - path: "web/src/components/admin/ontology/types.ts"
      provides: "ProvenanceInfo type + KGTriple.provenance field"
    - path: "web/src/components/admin/ontology/ontologyApi.ts"
      provides: "fetchKGQuery skill= param forwarding"
decisions:
  - id: "D-TIER-01"
    text: "Empty/nil allowedTiers = no filtering (admin baseline = all visible)"
  - id: "D-PROV-03"
    text: "KG query results carry provenance by default; 来源 column shows source_kind badge + path + line range"
metrics:
  duration: "~8 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 2
  files_modified: 4
  commits: 2
---

# Phase 40-08 Summary: KGQueryPanel Skill Selector + Provenance Column

## One-liner

Added Skill selector (demonstrating tier filtering via `QueryScoped`) and 来源 provenance column to the KGQueryPanel in the 本体 tab.

## What Was Built

### Task 1: `internal/server/admin_ontology.go` — skill param to KG query handler

**`resolveSkillTiers(skillName string) []string`**:
- Finds the skill individual in KG by name (`skill_{name}`)
- Queries `hasReadAccess` edges on that skill
- Returns tier short names (namespace prefix stripped)

**`adminOntologyKGQueryHandler`** updated:
- Accepts optional `skill=` query parameter
- When `skill` is provided: calls `QueryScoped(subj, pred, obj, allowedTiers)` with resolved tiers
- When `skill` is absent: calls unfiltered `Query` (admin baseline)

### Task 2: FE — KGQueryPanel Skill selector + 来源 column

**`KGQueryPanel.tsx`**:
- Skill selector `<select>` above the s/p/o grid (`data-testid="kg-query-skill-select"`)
- Default option: `""` → `不过滤（admin 基线）`
- Skill list populated by querying KG for `rdf:type = skf:Skill` on mount
- Selecting a skill re-runs query immediately with `skill=` param
- Results table gains 4th column `来源`:
  - `source_kind` badge: 文件 (file, green), 生成 (emitter, muted), 归纳 (induction, indigo)
  - `source_path` truncated with title tooltip
  - `L{start}–L{end}` line range when present
  - Muted `—` for legacy triples without provenance
- Summary line: `Skill 视角：{name|不过滤（admin 基线）} · 匹配 {n} 条`

**`types.ts`**:
- `KGTriple` gains optional `provenance?: ProvenanceInfo`
- New `ProvenanceInfo` interface: `source_kind`, `source_path`, `line_start?`, `line_end?`, `extracted_at`

**`ontologyApi.ts`**:
- `fetchKGQuery` gains optional `skill?: string` parameter, forwarded as `&skill=`

## Key Implementation Details

### Skill tier resolution
`resolveSkillTiers` queries KG for `hasReadAccess` triples on the skill individual. The tier names from `O` have the namespace prefix stripped before returning. Empty result (skill not found) returns `nil`, which `QueryScoped` treats as no filtering.

### `QueryScoped` integration
When `skill=` is provided, the full list of allowed tiers is passed to `QueryScoped`. Per D-TIER-01: empty `allowedTiers` means no filtering (handled at the `QueryScoped` level). Per D-TIER-03: schema layer is never filtered regardless. Per D-TIER-07: entities without `belongsToTier` are fail-closed.

### FE tier resolution
Skills are enumerated client-side purely for populating the `<select>` options. The `allowedTiers` derivation happens server-side via `resolveSkillTiers` — FE never computes tier access itself.

## Verified

- Go build (syntax check): clean
- TypeScript type check (`npx tsc --noEmit`): clean, no kg/query/ontologyApi/types errors
- Commits: `5a7520f` (BE), `c2c1425` (FE)

## Deviations from Plan

- None — plan executed as written.

## Commits

- `5a7520f` — `feat(40-08): add skill param to KG query handler with QueryScoped tier filtering`
- `c2c1425` — `feat(40-08): add Skill selector and 来源 provenance column to KGQueryPanel`

## Self-Check: PASSED

- [x] `adminOntologyKGQueryHandler` accepts `skill=` query param
- [x] `resolveSkillTiers` maps skill name to tier names via `hasReadAccess` KG triples
- [x] `QueryScoped` called with resolved tiers when skill is provided
- [x] `fetchKGQuery` forwards `skill=` param to backend
- [x] `KGTriple` has optional `provenance` field
- [x] Skill selector present in KGQueryPanel
- [x] Default option is `不过滤（admin 基线）`
- [x] Selecting skill re-queries with skill param
- [x] 来源 column shows source_kind badge + path + line range
- [x] Legacy triples show `—`
- [x] Summary line shows `Skill 视角：{name} · 匹配 {n} 条`
- [x] Both commits verified in git log
