---
phase: "17-p10-3"
plan: "01"
subsystem: "wire-types"
tags: ["go", "typescript", "svg", "sparkline", "wire-types", "kpi"]

# Dependency graph
requires:
  - phase: "16-p10-2"
    provides: "DataReference struct (D16-B3), kpi_call tool, SSE data-decision channel"
provides:
  - "PointReference struct (Go) + DataReference.References []PointReference field"
  - "PointReference TS interface + DataReference.references?: PointReference[] mirror"
  - "Pure SVG Sparkline component (zero recharts/sparklines/chart.js deps)"
affects:
  - "Phase 17 subsequent plans (17-02..17-09)"
  - "web/src/components/chat/MessageBubble.tsx (kpi_reference branch)"
  - "internal/agent/tools/kpi_tool.go (buildPointReferences)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D15 two-commit lockstep: Go wire field + TS mirror committed separately"
    - "D17-B1 zero-dependency SVG sparkline (no chart library)"
    - "Field addition on existing struct preserves ValidTypes count (23)"

key-files:
  created:
    - "web/src/components/chat/parts/Sparkline.tsx"
    - "web/src/components/chat/parts/Sparkline.test.tsx"
  modified:
    - "internal/wire/chunk.go"
    - "internal/wire/chunk_test.go"
    - "web/src/types/wire-types.ts"

key-decisions:
  - "D17-B2: PointReference struct with pointIndex/period/value/kMinApplied fields mirrors to TS"
  - "D17-B2: DataReference.References field uses omitempty (N=1 for single-period calls still works)"
  - "D17-B1: Sparkline uses pure SVG polyline+circles with useMemo autoscaling (±10% Y padding)"
  - "ValidTypes count stays at 23 (field addition, not union variant — TS AssertAssignable alarm stays green)"

patterns-established:
  - "PointReference struct pattern: pointIndex (int, 0-based) + Period (string) + Value (any) + KMinApplied (omitempty int)"
  - "Sparkline zero-dep gate: grep recharts|react-sparklines|chart.js returns 0 hits"

requirements-completed: ["DASH-03", "DASH-04"]

# Metrics
duration: 2min
completed: 2026-08-27
---

# Phase 17 Plan 01: PointReference wire types + pure SVG Sparkline component

**PointReference struct in Go + TS mirror + zero-dependency SVG Sparkline for KPI Dashboard Skill (D17-B2/D17-B1)**

## Performance

- **Duration:** 2 min
- **Started:** 2026-08-27T13:03:00Z
- **Completed:** 2026-08-27T13:05:00Z
- **Tasks:** 3
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments
- Added `PointReference` struct (pointIndex/period/value/kMinApplied) to Go wire types
- Added `References []PointReference` field to `DataReference` with omitempty
- Mirrored `PointReference` interface + `references?: PointReference[]` to TS wire-types.ts
- Created pure SVG `Sparkline` component (polyline + circles, useMemo autoscaling, zero chart deps)
- `ValidTypes` count stayed at 23 (field addition, not union variant)

## Task Commits

Each task was committed atomically:

1. **Task 1: Go wire types — PointReference + DataReference.References** - `4510742` (feat)
2. **Task 2: TypeScript wire types mirror (D15 lockstep)** - `e0db7b2` (feat)
3. **Task 3: Pure SVG Sparkline component (zero dependencies)** - `935b245` (feat)

**Plan metadata:** (no plan commit — this is the plan execution itself)

## Files Created/Modified
- `internal/wire/chunk.go` - Added PointReference struct + DataReference.References []PointReference field (D17-B2)
- `internal/wire/chunk_test.go` - Added TestDecisionDataRefWireReferences: round-trip PointIndex/Period/Value, ValidTypes count unchanged
- `web/src/types/wire-types.ts` - Mirrored PointReference interface + DataReference.references?: PointReference[] (D15 lockstep)
- `web/src/components/chat/parts/Sparkline.tsx` - Pure SVG polyline+circles component, useMemo autoscaling ±10%, data-testid on svg and per circle
- `web/src/components/chat/parts/Sparkline.test.tsx` - 4 sub-tests: TestSparklinePureSVG, TestSparklineHoverPoint, TestSparklineAutoscale, TestSparklineNoDeps

## Decisions Made
- Used `Value any` in Go PointReference (matches executor output type, formattable by FE)
- Sparkline Y-axis auto-scales with ±10% padding (min - padding to max + padding)
- X-axis step = 300 / (N-1) for N points; single-point case handled (xStep=0)
- Used `fireEvent` instead of `userEvent` in Sparkline tests (no @testing-library/user-event dependency)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `@testing-library/user-event` not installed in web package — rewrote Sparkline tests using `fireEvent` from `@testing-library/react` instead. Verified zero deps gate still passes.

## Next Phase Readiness
- `internal/wire/chunk.go` + `web/src/types/wire-types.ts` ready for 17-02 kpi_tool.go to call `buildPointReferences(snap.Rows)` and populate `References[]`
- Sparkline component ready for `DataReferenceChip` to render inline with `DataReference.references` data

---
*Phase: 17-p10-3 / 17-01*
*Completed: 2026-08-27*
