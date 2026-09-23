---
phase: "40"
plan: "07a"
subsystem: governance-fe
tags: [governance, shacl, tab-registration, frontend]
dependency_graph:
  requires: ["40-06"]
  provides: ["GovernanceTab", "governanceApi", "governance types", "10th tab registration"]
  affects: ["web/src/components/admin/AdminDashboard.tsx", "web/src/store/app-store.ts", "web/src/__tests__/AdminFeedbackTabs.test.tsx"]
tech_stack:
  added: []
  patterns: [10th SubrouteTab registration, three-view switcher, HealthBanner, snake_case API types]
key_files:
  created:
    - path: "web/src/components/admin/governance/GovernanceTab.tsx"
      provides: "GovernanceTab with three-view switcher + HealthBanner + stub views"
    - path: "web/src/components/admin/governance/types.ts"
      provides: "ShaclReport, ProvenanceGraph, ChangelogEntry, GraphNode types (snake_case)"
    - path: "web/src/components/admin/governance/governanceApi.ts"
      provides: "fetchShaclReport, fetchProvenanceGraph, fetchChangelog, rollbackTBox, runPipeline"
  modified:
    - path: "web/src/components/admin/AdminDashboard.tsx"
      provides: "ShieldCheck import, GovernanceTab import, 'governance' union, 10th SubrouteTab, ViewBody branch"
    - path: "web/src/store/app-store.ts"
      provides: "'governance' added to adminSubroute union + setAdminSubroute action"
    - path: "web/src/__tests__/AdminFeedbackTabs.test.tsx"
      provides: "tab order array extended to 10 entries (治理 last)"
decisions:
  - id: "UI-SPEC §Component 1"
    text: "GovernanceTab three-view switcher (校验报告/溯源图谱/版本演进) with role=tablist/tab/aria-selected"
  - id: "UI-SPEC §Component 3"
    text: "HealthBanner sub-component with red/warn/ok dot+label+counts, Run ECL 回填 CTA on yellow+provenance-missing"
  - id: "UI-SPEC §Copywriting"
    text: "Banner copy verbatim: error=发现 error 级违规, warn=存在 warning 级警告, ok=校验通过; meta line with validated_at+trigger+shapes_path"
metrics:
  duration: "~10 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 2
  files_created: 3
  files_modified: 3
  tests_passed: 6/6 (AdminFeedbackTabs)
  commits: 2
---

# Phase 40-07a Summary: Governance Tab Shell

## One-liner

Wire the `/admin/governance` tab shell: 10th tab registered with `ShieldCheck` icon, `GovernanceTab` with three-view switcher, `HealthBanner`, governance types, and `governanceApi`. This plan produces the infrastructure that 40-07b's views depend on.

## What Was Built

### Task 1: Register 10th tab + app-store union

**`AdminDashboard.tsx`**:
- Added `ShieldCheck` to lucide-react imports
- Added `GovernanceTab` import from `'./governance/GovernanceTab'`
- Widened `handleSubroute` union with `'governance'`
- Added 10th `SubrouteTab` entry: `ShieldCheck` icon, label `治理`, `data-testid="admin-tab-governance"`
- Added ViewBody branch: `{adminSubroute === 'governance' && <GovernanceTab />}`

**`app-store.ts`**:
- Extended `adminSubroute` union type with `'governance'`
- Extended `setAdminSubroute` action signature with `'governance'`

**`AdminFeedbackTabs.test.tsx`**:
- Extended tab order array from 9 to 10 entries: `治理` appended last

### Task 2: GovernanceTab shell + HealthBanner + types + api

**`governance/types.ts`** — snake_case types per 40-UI-SPEC §Component 2:
- `ShaclHealth`, `ViolationSeverity`
- `ShaclViolation` (severity, entity, constraint, detail, source_path, line_start, line_end)
- `QuarantinedEntity` (entity, triple_count, violations)
- `ShaclReport` (health, error_count, warning_count, warnings, quarantined, triple_count, shapes_path, validated_at, trigger)
- `ProvenanceKind`, `ProvenanceInfo` (source_kind, source_path, line_start, line_end, extracted_at)
- `GraphNode`, `GraphEdge`, `ProvenanceGraph`
- `ChangelogWho`, `ChangelogEntry`

**`governance/governanceApi.ts`** — mirrors `pipelineApi.ts` pattern:
- `GovernanceApiError extends Error` carrying `{status, message}`
- `fetchShaclReport()` → `GET /api/admin/governance/shacl`
- `fetchProvenanceGraph()` → `GET /api/admin/governance/provenance`
- `fetchChangelog()` → `GET /api/admin/ontology/changelog`
- `rollbackTBox(targetVersion, reason?)` → `POST /api/admin/ontology/rollback`
- `runPipeline()` → `POST /api/admin/pipeline/run`

**`GovernanceTab.tsx`** — three internal views (OntologyTab switcher idiom):
- `HealthBanner` sub-component exported for use by ShaclReportView
  - Red dot + label for `health=error`: `发现 error 级违规 — {n} 个实体已隔离`
  - Yellow dot + label for `health=warn`: `存在 warning 级警告 — 数据已放行`
  - Green dot + label for `health=ok`: `校验通过 — 0 违规`
  - `Run ECL 回填` primary button appears when `health=warn` + any warning has `constraint === 'provenance-missing'`
  - Meta line: validated_at local display + trigger label + shapes path
- Three-view switcher: `校验报告` / `溯源图谱` / `版本演进` with `role="tablist"` + `aria-selected`
- Stub view bodies (filled in 40-07b)

## UI-SPEC Verbatim Compliance

- [x] Three-color HealthBanner: `bg-status-non-compliant` (red) / `bg-status-needs-info` (yellow/warn) / `bg-status-compliant` (green)
- [x] `Run ECL 回填` CTA copy verbatim (40-UI-SPEC §Copywriting): `运行 ECL 管线？将重建 sql-guides 的切块、实体抽取与索引（幂等操作，可重复执行），并同时回填 provenance 与 belongsToTier。`
- [x] `data-testid="governance-health-banner"` on banner container
- [x] `data-testid="governance-backfill-btn"` on Run ECL CTA
- [x] View switcher testids: `governance-view-shacl`, `governance-view-provenance`, `governance-view-timeline`
- [x] Banner meta line shows `validated_at` local + `trigger` (启动时/回滚后) + `shapes_path`
- [x] `data-testid="admin-tab-governance"` on 10th tab

## Verified

- `npx tsc --noEmit` — clean (0 errors)
- `vitest run src/__tests__/AdminFeedbackTabs.test.tsx` — 6/6 PASS
- `git log --oneline -2` — both commits verified

## Commits

- `496bfbd` — `feat(40-07a): register 10th tab governance with ShieldCheck icon`
- `e3554f3` — `feat(40-07a): add governance types, api wrappers, and GovernanceTab shell`

## Deviations from Plan

None — plan executed exactly as written.

## Deferred Items

- Real view implementations (校验报告 / 溯源图谱 / 版本演进) — deferred to plan 40-07b
- Cytoscape.js provenance graph — deferred to plan 40-07b
- Timeline rollback interactions — deferred to plan 40-07b

## Self-Check: PASSED

- 10th tab registered with `ShieldCheck` icon — verified in AdminDashboard.tsx
- `adminSubroute` union widened — verified in app-store.ts
- `GovernanceTab` renders at correct route — verified in AdminDashboard.tsx
- `HealthBanner` with red/warn/ok states — verified in GovernanceTab.tsx
- `governanceApi.ts` calls correct endpoints — verified by code inspection
- `HealthBanner` exported for ShaclReportView use — verified in GovernanceTab.tsx
- AdminFeedbackTabs test extended to 10 tabs — 6/6 tests pass
- Both commits verified in git log
