---
quick_id: 260811-uaf
slug: fix-centerpanel-tab-indicator-sync-with-
description: Fix CenterPanel tab indicator sync with centerView.kind
date: 2026-08-11
status: complete
commit: e757d6e
---

# Quick Task 260811-uaf — Complete

## What

Closed the last remaining UAT gap from Phase 4 (`04-streaming-trust-ui`): the CenterPanel tab
indicator was not following programmatic `setCenterView` changes. Selecting a skill (SkillPicker →
`skill-md`) or clicking a citation (CitationCluster → `chunk-source`) switched the panel **content**
but left the `审查结论` tab visually active. **04-UAT test 8** (severity: minor) is now green.

## Root cause

`<Tabs defaultValue="verdict" />` was uncontrolled. `defaultValue` seeds `useState` once; the Tabs
primitive had no controlled `value` prop, so it never re-read `centerView.kind`. Dual source of truth
(Tabs internal state vs Zustand store) — Pitfall-7-adjacent.

## Changes

| File | Change |
|------|--------|
| `web/src/components/ui/tabs.tsx` | Added optional `value?: string` prop. When set, Tabs is controlled (`activeValue = value ?? internalValue`); on click it skips internal `setInternalValue` and still fires `onValueChange`. `defaultValue` made optional. **Backward compatible** — Tabs' sole other caller passes only `defaultValue`. |
| `web/src/components/center/CenterPanel.tsx` | Added `activeTabValue(centerView.kind)` helper (`chunk-source`→引用原文, `skill-md`→Skill 定义, `empty`/`verdict`→审查结论). Replaced `<Tabs defaultValue=...>` with `<Tabs value={activeTabValue(centerView.kind)}>`. Updated header comment to document tabs as store-bound display indicators (D-02). |
| `web/src/__tests__/CenterPanel.test.tsx` | +1 test (`tab indicator follows programmatic setCenterView`): asserts `skill-md` → Skill 定义 active, `chunk-source` → 引用原文 active, and the previously-active tab goes `aria-selected="false"`. Covers both test-8 programmatic paths. |

## Verification

- `cd web && npm test -- --run` → **83 passed (83)** across 14 files. Baseline was 82; +1 new test.
- Existing `default tab is verdict — clicking second tab does NOT change centerView` test still green
  (D-02 pin preserved: clicks remain non-navigating).
- No TypeScript regressions (Vite `tsc`-via-vitest transform clean).

## Decision recap — controlled-value only, not bidirectional nav

Shipped the controlled-`value` direction (closes test 8). Did **not** wire `onValueChange → setCenterView`:

1. Existing D-02 test pins "tab click does not change store"; wiring would break that pin.
2. `centerView.verdictId` has **0 consumers** (dead field) and a bare chunk-source click has no chunk
   payload — clean bidirectional nav is not achievable for 2 of 3 tabs without further design.
3. `ChunkSourceView` has no empty state, so UI-SPEC 169 (`点击聊天中的引用以查看权威条款原文。`) is
   unimplemented — pulling full navigable-tabs in would expand scope.

Net effect: store is now the **single source of truth** for both indicator and body. The dual-source
smell is gone. Tab clicks are visual no-ops, consistent with D-02 (navigation flows through
SkillPicker / CitationCluster).

## Deferred

- Bidirectional tab navigation (tab click → setCenterView) with proper payloads.
- UI-SPEC 169 chunk-source empty-state copy; end-to-end verify UI-SPEC 170 skill-md empty state.
- These belong in a future Phase 4 follow-up or Phase 5 polish task, not this quick fix.

## Phase 4 status after this fix

All 12 UAT tests now pass (test 8 flipped to green). Phase 4 is ready to close via
`/gsd:verify-work 4` (complete_session → transition) — pending a check on whether
`/gsd:secure-phase 04` is required by config before advancing.
