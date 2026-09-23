---
phase: "40-data-governance-shacl-provenance-tbox-versioning"
plan: "07b"
subsystem: governance-fe
tags: [governance, shacl, cytoscape, provenance, tbox-versioning, react, tailwind]
dependency_graph:
  requires:
    - phase: "40-07a"
      provides: "GovernanceTab shell, HealthBanner, governance types, governanceApi"
  provides:
    - "ShaclReportView (quarantine red zone + warnings yellow zone)"
    - "ProvenanceGraphView (Cytoscape breadthfirst, search, detail panel)"
    - "TimelineView (version timeline, rollback lifecycle)"
    - "GovernanceTab wired with real view components"
    - "cytoscape ^3.34.0 in package.json"
  affects:
    - "40-08"
    - "admin/governance tab"
tech_stack:
  added: [cytoscape ^3.34.0]
  patterns: [Cytoscape container ref + useEffect mount (no React wrapper), CSS token getComputedStyle at mount, three-color health mapping, rollback confirm + toast lifecycle]
key_files:
  created:
    - path: "web/src/components/admin/governance/ShaclReportView.tsx"
      provides: "Quarantine table (red zone) + warnings table (yellow zone) per D-SHACL-03"
    - path: "web/src/components/admin/governance/ProvenanceGraphView.tsx"
      provides: "Cytoscape breadthfirst graph with search highlight + node detail panel"
    - path: "web/src/components/admin/governance/TimelineView.tsx"
      provides: "Version timeline (newest-first) with rollback lifecycle per D-LOG-02/03"
  modified:
    - path: "web/package.json"
      provides: "cytoscape ^3.34.0 added (no react-cytoscape wrapper, no layout plugins)"
    - path: "web/src/components/admin/governance/GovernanceTab.tsx"
      provides: "Real view imports wired; stub divs removed"
decisions:
  - "HealthBanner re-exported from GovernanceTab for ShaclReportView yellow-zone backfill CTA (provenenance-missing rows)"
  - "Cytoscape MutationObserver on documentElement for data-theme changes (graph recreate on theme toggle)"
  - "CSS.escape() used for search highlight node selector (handles special chars in node IDs)"
requirements-completed: [ONT-14, ONT-15, ONT-16, ONT-17]
patterns-established:
  - "Cytoscape plain integration: container ref + useEffect mount + cy.destroy() on unmount (no react-cytoscape wrapper)"
  - "CSS token resolution at mount: getComputedStyle(document.documentElement).getPropertyValue() — no hardcoded hex"
  - "Three-color health mapping: error→status-non-compliant (red), warn→status-needs-info (yellow), ok→status-compliant (green)"
  - "Rollback lifecycle: window.confirm → POST → success toast with new_violations count → refetch all three surfaces"
---

# Phase 40-07b Summary: Governance Views (ShaclReportView / ProvenanceGraphView / TimelineView)

**Three governance views complete the /admin/governance tab: quarantine red zone + warnings yellow zone (ShaclReportView), Cytoscape provenance graph with breadthfirst layout (ProvenanceGraphView), and version timeline with rollback lifecycle (TimelineView). Cytoscape ^3.34.0 pinned in package.json.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-23T03:07:10Z
- **Completed:** 2026-09-23T03:22:00Z
- **Tasks:** 5
- **Files created:** 3
- **Files modified:** 2

## Accomplishments

- ShaclReportView: quarantine table (red zone) with entity/triple/constraint/detail/source/line columns + warnings table (yellow zone, no triple column) + provenance-missing yellow badge
- ProvenanceGraphView: Cytoscape breadthfirst layout (directed, roots=file/skillmd, levelSep:48, nodeSep:16), autoungrabify, CSS token colors via getComputedStyle, search with neighborhood highlight + dim, node click detail panel
- TimelineView: newest-first entries, who badge (admin/induction:llm/system:rollback), current badge, rollback lifecycle with new-violation toast
- GovernanceTab wired with real views; stub divs removed
- Cytoscape ^3.34.0 in package.json (plain npm, no wrapper libs, no layout plugins)

## Task Commits

1. **Task 1: Add cytoscape dependency** - `358cfec` (feat) + **Task 2: ShaclReportView** - `358cfec` (feat) — combined commit
2. **Task 3: ProvenanceGraphView** - `5044031` (feat)
3. **Task 4: TimelineView** - `d84557f` (feat)
4. **Task 5: Update GovernanceTab** - `2a0a154` (feat)

**Plan metadata:** `2a0a154` (feat: wire real views into GovernanceTab)

## Files Created/Modified

- `web/package.json` - cytoscape ^3.34.0 added; no react-cytoscape*, no layout plugins
- `web/src/components/admin/governance/ShaclReportView.tsx` - quarantine red zone + warnings yellow zone per D-SHACL-03
- `web/src/components/admin/governance/ProvenanceGraphView.tsx` - Cytoscape breadthfirst + search + detail panel per D-PROV-04
- `web/src/components/admin/governance/TimelineView.tsx` - version timeline + rollback lifecycle per D-LOG-02/03
- `web/src/components/admin/governance/GovernanceTab.tsx` - real view imports wired; stub divs replaced

## Decisions Made

- HealthBanner re-exported from GovernanceTab for ShaclReportView yellow-zone backfill CTA (provenenance-missing rows reference it)
- Cytoscape MutationObserver on documentElement for data-theme changes triggers graph recreate (theme toggle compatibility)
- CSS.escape() used for search highlight node selector (node IDs may contain special chars)

## Deviations from Plan

None - plan executed exactly as written.

## Verified

- `npx tsc --noEmit` — clean (0 errors)
- cytoscape in package.json with exact ^3.34.0 version
- No react-cytoscape wrapper libs, no layout plugins (dagre/cose-bilkent)
- All three views implemented per UI-SPEC §Component 4/5/6
- GovernanceTab stub divs replaced with real component mounts

## Self-Check: PASSED

- cytoscape ^3.x pinned in package.json — verified
- ShaclReportView three-color mapping (red quarantine / yellow warnings / green ok) — verified
- ProvenanceGraphView breadthfirst + autoungrabify + search highlight + detail panel — verified
- TimelineView rollback confirm shows new-violation count — verified
- No banned features (no 重新校验 button, no auto-fix, no Milvus work) — verified
- All 5 commits in git log — verified

---
*Phase: 40-07b*
*Completed: 2026-09-23*
