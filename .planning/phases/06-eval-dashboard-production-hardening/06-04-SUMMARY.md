---
phase: 06-eval-dashboard-production-hardening
plan: 04
subsystem: ui

tags:
  - react
  - typescript
  - vite
  - zustand
  - recharts
  - eval-dashboard
  - audit
  - d14-no-messages
  - d6-01
  - d6-03
  - d6-04
  - d6-07
  - d6-14
  - d6-16
  - sc-1
  - sc-3

requires:
  - phase: 06-eval-dashboard-production-hardening/02
    provides: "GET /api/eval/reports, /api/eval/frozen, /api/audit endpoints + EvalIndex / FrozenSetInfo / AuditRow shapes"

provides:
  - "Admin-mode /eval route with route branch in App.tsx (D6-01)"
  - "In-store minimal router (route + evalSubroute) with popstate sync for /, /eval, /eval/audit"
  - "EvalDashboard shell: top bar, manual Refresh, 30s poll, subroute tabs"
  - "SkillsTable: per-Skill latest run with 5 metrics + ✅/❌ threshold badge (SC-1)"
  - "TrendPanel: recharts line chart + gray dashed frozen-baseline ReferenceLine + >5% drift flag (D6-16, D6-14)"
  - "CrossEvalTab: per-judge faithfulness diff table with >5% flag (D6-14 / Pitfall 8)"
  - "AuditTable: filter bar mapping to /api/audit query params + redaction badges (D6-05, D6-07, SC-3)"
  - "Zustand eval slice with NO messages field (D14 invariant)"

affects:
  - 06-eval-dashboard-production-hardening
  - 07-open-source-packaging

tech-stack:
  added:
    - "recharts@3.10.1 — trend line chart + ReferenceLine (approved at checkpoint)"
  patterns:
    - "Minimal in-store router on app-store.ts instead of react-router (D6-01 / D14)"
    - "Plain JSON fetch for /eval data; chat SSE transport untouched (D-04/D12 isolation)"
    - "Data-driven panel branching: loading/error/empty/ready with data-testid on interactive nodes"
    - "Status-color tokens (status-compliant / status-non-compliant) reused for threshold badges"

key-files:
  created:
    - web/src/components/eval/EvalDashboard.tsx
    - web/src/components/eval/SkillsTable.tsx
    - web/src/components/eval/SkillsTable.test.tsx
    - web/src/components/eval/TrendPanel.tsx
    - web/src/components/eval/TrendPanel.test.tsx
    - web/src/components/eval/CrossEvalTab.tsx
    - web/src/components/eval/AuditTable.tsx
    - web/src/components/eval/AuditTable.test.tsx
    - web/src/components/eval/__fixtures__/index.json
    - web/src/components/eval/__fixtures__/report.json
    - web/src/hooks/useEvalReports.ts
    - web/src/types/eval.ts
  modified:
    - web/src/store/app-store.ts
    - web/src/App.tsx
    - web/src/components/layout/LeftSidebar.tsx
    - web/package.json
    - web/package-lock.json

key-decisions:
  - "Used a minimal in-store router (route + evalSubroute on app-store.ts) plus popstate listener instead of adding react-router, preserving 5 phases of avoiding a router dependency and keeping D14's store-first architecture."
  - "TrendPanel reads baseline_faithfulness from FrozenSetInfo for the dashed gray ReferenceLine; if no baseline is present the line is omitted and no drift flag can fire."
  - "CrossEvalTab is a thin view over archived per-judge IndexRun entries; it flags any Skill whose latest faithfulness delta across judges exceeds 5% (D6-14)."
  - "AuditTable debounces filter input changes at 300ms before dispatching setAuditFilter and fetching /api/audit, matching D6-03's low-frequency refresh posture."
  - "Redacted cells parse the server-sent '[REDACTED:...]' sentinel and render a short badge with the full reason in the title attribute (D6-07)."

patterns-established:
  - "Eval slice fields: route, evalSubroute, indexReports, auditRows, auditFilter, frozenBaseline, evalStatus, evalError — explicitly no messages field (D14)."
  - "EvalDashboard shell owns the polling interval; child panels are pure views over store data."
  - "All eval components expose data-testid for loading/error/empty/ready branches and per-row identifiers."

requirements-completed:
  - GEVL-06

# Metrics
duration: 30min
completed: 2026-08-13
---

# Phase 06 Plan 04: Eval Dashboard Frontend Summary

**Admin-mode eval dashboard UI with SkillsTable, recharts trend panel, cross-eval bias tab, and queryable audit table, all on a D14-safe Zustand slice without a messages field.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-08-13T08:41:00Z
- **Completed:** 2026-08-13T09:11:02Z
- **Tasks:** 3 (T1 remainder + T2 + T3)
- **Files modified:** 17

## Accomplishments

- **T1 remainder — route + store + shell:** Extended `app-store.ts` with the Phase-6 eval slice (`route`, `evalSubroute`, `indexReports`, `auditRows`, `auditFilter`, `frozenBaseline`, `evalStatus`, `evalError`) while preserving the D14 no-`messages` invariant. Branched `App.tsx` on `route === 'eval'` to render `EvalDashboard` instead of the chat three-panel, and added a `popstate` listener that syncs `/`, `/eval`, and `/eval/audit` with the store. Added an "Eval Dashboard" nav entry to `LeftSidebar`. Created the `EvalDashboard` shell with top bar, Refresh button, subroute tabs, and 30s polling via `useEvalReports`.
- **T2 — metrics + trend + cross-eval:** Implemented `SkillsTable` (latest run per Skill with faithfulness / citation precision / answer relevance / context recall / refusal accuracy + ✅/❌ badge), `TrendPanel` (recharts `LineChart` + gray dashed `ReferenceLine` frozen baseline + >5% drift flag), and `CrossEvalTab` (per-judge diff table). Added `__fixtures__/index.json` and `__fixtures__/report.json`. Co-located tests for `SkillsTable` and `TrendPanel` pass.
- **T3 — audit table + redaction:** Implemented `AuditTable` with a debounced filter bar (user / skill / refusal_reason / since / until / limit / offset), mapping to `/api/audit` query params, and gray `[REDACTED (90d+)]` badges for cells starting with `[REDACTED:`, with the full reason in the `title` attribute. Co-located `AuditTable.test.tsx` covers rows, redaction badges, filter-param dispatch, and empty state.
- **Verification:** `npx tsc --noEmit` is clean; `npx vitest run` passes 35 test files / 233 tests; `npm run build` succeeds.

## Task Commits

1. **Task 1 remainder: eval slice + App.tsx route branch + LeftSidebar nav + EvalDashboard shell** — `c6a43e2` (feat)
2. **Task 2: SkillsTable + TrendPanel + CrossEvalTab + fixtures + tests** — `17f74e3` (feat)
3. **Task 3: AuditTable with filter bar + redaction badges + test** — `0daf763` (feat)
4. **Plan metadata (SUMMARY.md)** — `17a5804` (feat: recharts install) is the prior dependency commit; this SUMMARY captures the completed plan.

## Files Created/Modified

**Created**
- `web/src/types/eval.ts` — TS mirror of EvalIndex / IndexRun / FrozenSetInfo / AuditRow / AuditFilter.
- `web/src/hooks/useEvalReports.ts` — 30s polling hook for `/api/eval/reports` and `/api/eval/frozen`.
- `web/src/components/eval/EvalDashboard.tsx` — admin-mode shell + top bar + subroute tabs.
- `web/src/components/eval/SkillsTable.tsx` — per-Skill metrics table.
- `web/src/components/eval/SkillsTable.test.tsx` — 5 tests for SC-1 surface.
- `web/src/components/eval/TrendPanel.tsx` — recharts trend + frozen baseline + drift flag.
- `web/src/components/eval/TrendPanel.test.tsx` — 5 tests for D6-16 surface.
- `web/src/components/eval/CrossEvalTab.tsx` — per-judge diff table.
- `web/src/components/eval/AuditTable.tsx` — audit query + filter + redaction badges.
- `web/src/components/eval/AuditTable.test.tsx` — 4 tests for SC-3 surface.
- `web/src/components/eval/__fixtures__/index.json` — sample EvalIndex for tests.
- `web/src/components/eval/__fixtures__/report.json` — sample eval report fixture.

**Modified**
- `web/src/store/app-store.ts` — added eval slice + actions; no messages field.
- `web/src/App.tsx` — route branch + popstate sync; chat three-panel preserved.
- `web/src/components/layout/LeftSidebar.tsx` — Eval Dashboard nav entry.
- `web/package.json` + `web/package-lock.json` — recharts@3.10.1 dependency (committed in `17a5804`).

## Decisions Made

- Chose the minimal in-store router (`route` / `evalSubroute` on the Zustand store) over `react-router` to keep the architecture aligned with D14 and the 5-phase pattern of store-first routing.
- Used recharts for the trend chart because it provides `ReferenceLine`, tooltips, and responsive containers that would be tedious to hand-roll for the headline SC-1 visual.
- Kept the audit fetch debounced at 300ms to avoid request spam while preserving D6-03's overall 30s polling cadence at the shell level.
- Rendered redacted audit cells as badges with the full server-sent reason in `title`, satisfying D6-07's hover-detail requirement without expanding raw content in the UI.

## Deviations from Plan

None - plan executed exactly as written.

## Auto-Fixed / Opportunistic Notes

- **Fixture-driven test adjustment:** The "renders one row per Skill with all 5 metrics" test initially asserted the first fixture run's values (0.92) but `SkillsTable` correctly picks the *latest* run per Skill, which is 0.91 in the fixture. Updated the test expectation to match the intended component behavior — no code change needed.
- **JSX escaping in CrossEvalTab:** Used `&gt;` entity for the `>5%` label so the JSX parser does not treat the bare `>` as a tag terminator.

## Issues Encountered

- `npm ls recharts` reported `(empty)` until `npm install` was re-run inside the worktree; after install, 3.10.1 resolved correctly and `npm run build` succeeded.
- The Vite production build emits a chunk-size warning (~843 KB / 245 KB gzip) because recharts is bundled in the main chunk. This is expected and acceptable for v1; code-splitting the eval route is a future optimization, not a blocker.

## Known Stubs

None. All dashboard components are wired to live data sources (`/api/eval/reports`, `/api/eval/frozen`, `/api/audit`) or store-driven fixtures in tests.

## Threat Flags

None introduced beyond the already-documented threat model:
- `/api/audit` redaction projection and `X-Admin-Token` gating remain server-side concerns from Plan 06-02.
- The dashboard does not read, store, or transmit any admin token; it always receives retention-redacted rows unless the server is configured otherwise.
- No `dangerouslySetInnerHTML` is used; React escapes interpolated audit text by default.

## User Setup Required

None beyond the recharts dependency, which was approved and installed at the human-verify checkpoint (`17a5804`).

## Next Phase Readiness

- The `/eval` admin-mode surface is complete and ready for Phase 7 DIST packaging.
- The backend endpoints from 06-02 are consumed as specified.
- No blockers.

---

*Phase: 06-eval-dashboard-production-hardening*
*Completed: 2026-08-13*