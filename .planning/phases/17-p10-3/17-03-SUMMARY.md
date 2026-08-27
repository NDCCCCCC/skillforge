---
phase: "17"
plan: "03"
subsystem: ui
tags: [kpi, drillchip, zustand, react]

# Dependency graph
requires:
  - phase: "17-01"
    provides: KPI Dashboard Skill plan skeleton
provides:
  - DrillChip component (Badge with onClick, sets pendingDraft)
  - pendingDraft field + setPendingDraft action in app-store
  - ChatInput wired to consume pendingDraft on next render
affects:
  - Phase 17 P10.3 KPI Dashboard Skill downstream plans

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "DrillChip one-shot prefill: setPendingDraft in DrillChip, useEffect read-and-clear in ChatInput"
    - "consume-once pattern: pendingDraft cleared immediately after use"
    - "@ alias added to vitest.config.ts for @/ imports in test files"

key-files:
  created:
    - web/src/components/chat/parts/DrillChip.tsx
    - web/src/components/chat/parts/DrillChip.test.tsx
  modified:
    - web/src/store/app-store.ts (pendingDraft field + setPendingDraft action)
    - web/src/components/chat/ChatInput.tsx (pendingDraft useEffect + handleSend clear)
    - web/vitest.config.ts (@ alias for test resolution)

key-decisions:
  - "Badge variant='default' (not 'secondary') — 'secondary' is not a valid Badge variant per web/src/components/ui/badge.tsx"
  - "@/ imports in vitest resolved via vite alias, not relative paths — applied to vitest.config.ts"
  - "vi.fn() untyped for setPendingDraft mock — vitest TS constraint prevents vi.fn<[string]>()"

patterns-established:
  - "DrillChip prefill UX: Badge onClick → setPendingDraft → ChatInput useEffect consumes → clear"

requirements-completed: [DASH-04]

# Metrics
duration: 8min
completed: 2026-08-27
---

# Phase 17 Plan 03: DrillChip + pendingDraft Store + ChatInput Wiring Summary

DrillChip component implemented: click pre-fills chat input with KPI call text via a one-shot pendingDraft signal in the Zustand store.

## Performance

- **Duration:** 8 min
- **Started:** 2026-08-27T13:00:00Z
- **Completed:** 2026-08-27T13:08:00Z
- **Tasks:** 3/3
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- DrillChip renders a Badge with `data-testid="drill-chip"` and onClick calling `setPendingDraft('请调 {kpiName} periods={periodsTemplate}')`
- `pendingDraft: string` + `setPendingDraft(text)` added to app-store AppState/AppActions
- ChatInput useEffect reads pendingDraft, pre-fills textarea when empty, clears after use
- handleSend also clears pendingDraft if set (ensures no stale prefill on explicit send)
- All 11 tests pass (8 ChatInput + 3 DrillChip)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add pendingDraft to app-store** - `315dd22` (feat)
2. **Task 2: Create DrillChip component** - `315dd22` (feat)
3. **Task 3: Wire ChatInput to read pendingDraft** - `315dd22` (feat)

**Plan metadata:** `315dd22` (feat: complete plan)

## Files Created/Modified

- `web/src/store/app-store.ts` - Added `pendingDraft: string` field, `setPendingDraft` action
- `web/src/components/chat/ChatInput.tsx` - Added `pendingDraft` subscription, useEffect prefill, handleSend clear
- `web/src/components/chat/parts/DrillChip.tsx` - New: Badge-based drill chip with onClick prefill
- `web/src/components/chat/parts/DrillChip.test.tsx` - New: TestPrefill, TestBadgeRenders, TestClickCallsSetPendingDraft
- `web/vitest.config.ts` - Added `@` alias (was missing from vitest config, caused @/ import failures)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Badge variant 'secondary' does not exist**
- **Found during:** Task 2
- **Issue:** Plan specified `variant="secondary"` but `Badge` only supports `'default' | 'tier' | 'review' | 'status'`
- **Fix:** Changed to `variant="default"` — Badge renders with default styling + custom KPI chip CSS tokens
- **Files modified:** `web/src/components/chat/parts/DrillChip.tsx`
- **Commit:** `315dd22`

**2. [Rule 3 - Blocking Issue] vitest.config.ts missing @ alias**
- **Found during:** Task 2 (verification)
- **Issue:** `DrillChip.tsx` uses `@/store/app-store` but vitest.config.ts lacked the `@` alias, causing "Failed to resolve import" at test time
- **Fix:** Added `resolve.alias.@` to vitest.config.ts matching vite.config.ts
- **Files modified:** `web/vitest.config.ts`
- **Commit:** `315dd22`

**3. [Rule 1 - Bug] vi.fn<[string]>() TypeScript constraint violation**
- **Found during:** Verification (tsc --noEmit)
- **Issue:** `vi.fn<[string]>()` fails with "Type '[string]' does not satisfy the constraint 'Procedure'"
- **Fix:** Changed to untyped `vi.fn()` — sufficient for the assertion
- **Files modified:** `web/src/components/chat/parts/DrillChip.test.tsx`
- **Commit:** `315dd22`

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| none | — | pendingDraft is plain text set by DrillChip; ChatInput renders via safe textarea element |

## Self-Check: PASSED

- `pendingDraft` field present in app-store.ts with default ''
- `setPendingDraft` action present and calling `set({ pendingDraft: text })`
- DrillChip.tsx renders Badge with `data-testid="drill-chip"` and onClick calling setPendingDraft
- ChatInput.tsx has useEffect reading pendingDraft, setting text, and clearing
- handleSend clears pendingDraft after send
- 11 tests pass (8 ChatInput + 3 DrillChip)
- tsc --noEmit exits 0
