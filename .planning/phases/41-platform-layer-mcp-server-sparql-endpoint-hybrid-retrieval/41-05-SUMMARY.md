---
phase: "41"
plan: "05"
subsystem: admin-console-frontend
tags: [admin, platform-tab, codemirror, sparql, mcp, hybrid, view-switcher]

dependency_graph:
  requires:
    - phase: "41-04"
      provides: "PlatformTab placeholder shell, store union widened, AdminDashboard 11th tab"
    - phase: "41-01"
      provides: "MCP server 5 tools exposed via stdio"
    - phase: "41-02"
      provides: "SPARQL-like POST /api/sparql endpoint"
    - phase: "41-03"
      provides: "Hybrid retrieval POST /api/admin/platform/hybrid/search"
  provides:
    - "platform/types.ts: all TypeScript types matching 41-UI-SPEC contract"
    - "platform/platformApi.ts: API wrappers for all platform endpoints"
    - "PlatformTab.tsx: three-view switcher (SPARQL Playground / MCP Inspector / Hybrid 调试)"
    - "SparqlPlaygroundView.tsx: CodeMirror 6 editor + results table"
    - "McpInspectorView.tsx: tools table + test invocation panel"
    - "HybridDebugView.tsx: KG expansion + Milvus hits table"
  affects: []
  next_plan: "41-06 (MCP invoke route + SPARQL handler wiring)"

tech_stack:
  added:
    - "@codemirror/lang-json@^6.0.1"
    - "@codemirror/state@^6.0.1"
    - "@codemirror/view@^6.0.1"
    - "codemirror@^6.0.1"
  patterns:
    - "GovernanceTab view-switcher idiom (role=tablist + aria-selected)"
    - "CodeMirror 6 mount via useEffect + EditorView.destroy() cleanup"
    - "SchemaProposalsTab table idiom (overflow-hidden rounded-md border)"
    - "ProvenanceGraphView detail panel pattern (IRI / tier / provenance / path)"
    - "platformApi.ts PlatformApiError extends Error with status/message"
    - "X-Skill-Name header for tier isolation on SPARQL + hybrid endpoints"

key_files:
  created:
    - path: "web/src/components/admin/platform/types.ts"
      provides: "SparqlPattern/Response, McpTool/ServerStatus/Invoke/InvokeResponse, HybridQuery/Response types"
    - path: "web/src/components/admin/platform/platformApi.ts"
      provides: "sparqlQuery, fetchMcpStatus, fetchMcpTools, invokeMcpTool, runHybridSearch"
    - path: "web/src/components/admin/platform/SparqlPlaygroundView.tsx"
      provides: "SPARQL Playground: CodeMirror 6 editor, skill selector, limit control, Run/Clear, results table"
    - path: "web/src/components/admin/platform/McpInspectorView.tsx"
      provides: "MCP Inspector: server status row, tools table, Test Invocation Panel with CodeMirror params editor"
    - path: "web/src/components/admin/platform/HybridDebugView.tsx"
      provides: "Hybrid 调试: query input, max_hops/rerank controls, KG expansion + detail panel, Milvus hits table"
  modified:
    - path: "web/src/components/admin/platform/PlatformTab.tsx"
      provides: "Three-view switcher replacing placeholder from Plan 41-04"
    - path: "web/package.json"
      provides: "CodeMirror 6 dependencies added"

key_decisions:
  - "Skill state lifted to PlatformTab level and passed as props to SparqlPlaygroundView and HybridDebugView (MCP Inspector does not use skill filtering)"
  - "KG Expansion uses simple node-list table + detail panel (not Cytoscape) per 41-UI-SPEC §Discretion Resolutions planner discretion"
  - "CodeMirror mount in useEffect with EditorView.destroy() cleanup; Ctrl+Enter handler re-attaches on skillName/limit change to avoid stale closure"
  - "schemaToSkeleton() generates valid JSON from MCP tool inputSchema: null replaced with '' for required string fields"

requirements_completed: [PLAT-ONT-18, PLAT-ONT-19, PLAT-ONT-20]

# Metrics
duration: ~15min
completed: 2026-09-23
---

# Phase 41-05 Summary: Platform Tab Three-View Implementation

**Replaced the PlatformTab placeholder with three full views (SPARQL Playground / MCP Inspector / Hybrid 调试) and all supporting TypeScript types and API wrappers. CodeMirror 6 dependencies added to web/package.json.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-23T06:21:00Z
- **Completed:** 2026-09-23T06:36:00Z
- **Tasks:** 5 (all committed individually)
- **Commits:** 3

## Task Commits

Each task was committed atomically:

1. **Task 0: CodeMirror 6 deps** - `6e4cff4` (feat) — web/package.json updated
2. **Task 1: types.ts + platformApi.ts** - `3d23356` (feat) — 2 files created
3. **Task 2-5: Three views + PlatformTab replacement** - `3423893` (feat) — 4 files changed

## Accomplishments

- **CodeMirror 6 integration:** `@codemirror/lang-json`, `@codemirror/state`, `@codemirror/view`, `codemirror` all at `^6.0.1`, installed via `pnpm install`
- **types.ts:** All TypeScript types matching 41-UI-SPEC contract (SparqlPattern/Response, McpTool/ServerStatus/Invoke/InvokeResponse, HybridQuery/Response)
- **platformApi.ts:** API wrappers for all 5 endpoints (sparqlQuery, fetchMcpStatus, fetchMcpTools, invokeMcpTool, runHybridSearch) with PlatformApiError class and X-Skill-Name tier isolation header
- **PlatformTab.tsx:** Three-view switcher using GovernanceTab pattern (role="tablist" + aria-selected), skillName state lifted to tab level
- **SparqlPlaygroundView.tsx:** CodeMirror 6 editor (min-h-64), skill selector, limit select (10/25/50/100/500, default 50), Run Query + Clear buttons, results table with subject/predicate/object/来源 columns, Ctrl+Enter shortcut, loading/error/empty states
- **McpInspectorView.tsx:** Server status row (green/red dot + service_skill), tools table with Test buttons, Test Invocation Panel (CodeMirror params editor min-h-24 pre-populated from inputSchema, Invoke button, formatted JSON result area)
- **HybridDebugView.tsx:** Query input + Search icon + max_hops select + rerank toggle, KG expansion node-list + detail panel (IRI/tier/来源/路径/行号), Milvus hits table with top-5 highlighted (★ + bg-primary-dim/10)
- **TypeScript:** `npx tsc --noEmit` clean (0 errors)

## Files Created/Modified

- `web/package.json` — CodeMirror 6 dependencies added
- `web/src/components/admin/platform/types.ts` — All platform types (230 lines)
- `web/src/components/admin/platform/platformApi.ts` — API wrappers (230 lines)
- `web/src/components/admin/platform/PlatformTab.tsx` — Three-view switcher (replaces placeholder)
- `web/src/components/admin/platform/SparqlPlaygroundView.tsx` — SPARQL Playground view (300+ lines)
- `web/src/components/admin/platform/McpInspectorView.tsx` — MCP Inspector view (280+ lines)
- `web/src/components/admin/platform/HybridDebugView.tsx` — Hybrid 调试 view (280+ lines)

## Decisions Made

- Skill state lifted to PlatformTab level (passed as props to SparqlPlaygroundView and HybridDebugView; MCP Inspector is server-scoped only)
- KG Expansion uses node-list table + detail panel (not Cytoscape) per UI-SPEC discretion resolution
- CodeMirror cleanup via EditorView.destroy() in useEffect return; Ctrl+Enter handler re-attaches on skillName/limit deps to avoid stale closure
- schemaToSkeleton() replaces null with '' for required string fields to ensure valid JSON

## Deviations from Plan

None — plan executed exactly as written.

## Verification

- `npx tsc --noEmit` — clean (0 errors)
- `pnpm install` — dependencies installed successfully
- All 3 commits verified in git log

## Known Stubs

None — all views are fully implemented.

## Threat Flags

None — display-only debug panel, no new trust boundaries introduced.

---
*Phase: 41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval*
*Plan: 05*
*Completed: 2026-09-23*
