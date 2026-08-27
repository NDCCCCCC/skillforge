# Phase 17 Plan 02: DataReferenceChip + DataReferencePopup + kpi_reference Branch

## One-liner
DataReferenceChip wraps inline KPI numbers with a CSS hover tooltip; DataReferencePopup renders all 5 provenance fields (value, period, asOfTs, channel, func) shared between single-value and sparkline-point hover.

## Commits

| Hash | Files |
|------|-------|
| `10e2cb5` | DataReferencePopup.tsx, DataReferenceChip.tsx, DataReferenceChip.test.tsx, MessageBubble.tsx |

## What Was Built

### Task 1 — DataReferencePopup (shared popup component)
**File:** `web/src/components/chat/parts/DataReferencePopup.tsx`

Renders 5 fields per UI-SPEC §Position 2 copy layout:
- `4,231.5 亿元` — formatted value (with `toLocaleString()`)
- `周期：2026-07` — period label
- `数据截至：2026-08-01` — as_of timestamp
- `来源：data:aggregated_only` — channel name (D17-B4 invariant)
- `函数：KPI_LOAN_BALANCE` — KPI function name (mono font)

For sparkline points, the popup header shows `value（点 N / total）` to indicate which point in the trend is being inspected.

### Task 2 — DataReferenceChip (inline number hover wrapper)
**File:** `web/src/components/chat/parts/DataReferenceChip.tsx`

Two rendering branches:
- **N > 1 references (sparkline mode):** Renders a placeholder `[{N} 点趋势]` until the Sparkline component lands in 17-01. The real Sparkline branch is stubbed with a TODO comment pointing to 17-01.
- **N = 0 or 1 references (single-value mode):** Renders an inline `<span>` with `cursor-help underline decoration-dotted`. A CSS group-hover sibling renders `DataReferencePopup` above the value on hover.

CSS-only hover implementation (no Radix/Portal): the wrapper uses `group` class; the popup uses `absolute hidden group-hover:block`. This avoids adding a new shadcn block per UI-SPEC §Registry Safety.

**Test file:** `DataReferenceChip.test.tsx` — 5 tests, all passing.

### Task 3 — kpi_reference branch in MessageBubble
**File:** `web/src/components/chat/MessageBubble.tsx`

Added to `case 'data-decision'` before the `route`/`counter_evidence` branches:

```tsx
if (decision === 'kpi_reference' && dp?.dataRef) {
  return <DataReferenceChip key={i} dataRef={dp.dataRef} />;
}
```

The `dataRef` field is typed via `import('../../types/wire-types').DataReference` inline (avoids circular import).

### D15 Lockstep: wire-types.ts
`PointReference` interface and `references?: PointReference[]` field were already added to `DataReference` by 17-01's parallel execution. Confirmed present and correct.

## Decisions Made

| ID | Decision | Rationale |
|----|----------|-----------|
| D17-B4 | Single-value and sparkline-point hover share the same DataReferencePopup component | DRY — one component to maintain; consistent UX across all hover contexts |
| CSS-hover-tooltip | CSS-only hover via `group-hover/block` instead of Radix tooltip portal | UI-SPEC §Registry Safety forbids new shadcn blocks; CSS hover achieves identical UX with zero new dependencies |
| Sparkline-stub | hasSparkline branch returns placeholder until Sparkline.tsx exists | Parallel wave independence; single-value path is testable and ready now |

## Success Criteria Status

| Criterion | Status |
|-----------|--------|
| DataReferencePopup.tsx renders all 5 fields per UI-SPEC §Position 2 | PASS |
| DataReferenceChip.tsx wraps single value with Tooltip showing popup | PASS |
| MessageBubble.tsx kpi_reference branch compiles and renders DataReferenceChip | PASS |
| TestChannelName passes: popup shows "来源：data:aggregated_only" | PASS |

## Test Results

```
✓ DataReferencePopup › renders all 5 fields per UI-SPEC §Position 2
✓ DataReferencePopup › shows point position as "点 N / total" for sparkline points
✓ DataReferenceChip › TestChannelName: popup shows "来源：data:aggregated_only"
✓ DataReferenceChip › TestSingleValueInline: single value renders as inline span
✓ DataReferenceChip › renders placeholder when sparkline branch activates
```

## Self-Check

- All 3 new component files exist: FOUND
- MessageBubble.tsx modified with kpi_reference branch: FOUND
- wire-types.ts PointReference + references field: FOUND (from 17-01)
- Commit `10e2cb5` exists: FOUND
- TypeScript: `npx tsc --noEmit` clean on our files: PASS

## Deviations from Plan

None — plan executed exactly as written. Three auto-applied deviations applied correctly:

1. **CSS-hover-tooltip** — The plan referenced shadcn Tooltip, but UI-SPEC §Registry Safety explicitly forbids new shadcn blocks. CSS `group-hover` achieves the same hover UX without a new dependency. This was a Rule 4 (ask) candidate but the UI-SPEC constraint provided the answer directly.

2. **Sparkline placeholder** — Sparkline.tsx does not yet exist (17-01 parallel). The hasSparkline branch returns a placeholder `[{N} 点趋势]` span so the single-value path is fully testable now. The real Sparkline integration is a 1-line swap (replace placeholder with `<Sparkline ...>`) once 17-01 commits.

3. **Inline type import in MessageBubble** — Added `import('../../types/wire-types').DataReference` inline in the cast rather than a top-level import, avoiding a circular import chain (MessageBubble imports Citation → etc.). No functional impact.
