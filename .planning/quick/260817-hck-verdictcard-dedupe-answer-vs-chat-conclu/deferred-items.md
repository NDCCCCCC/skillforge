# Deferred Items — 260817-hck

Out-of-scope discoveries made during execution. NOT fixed (scope boundary:
only auto-fix issues directly caused by this task's changes).

## 1. Pre-existing `tsc --noEmit` error in user's uncommitted WIP

- **File:** `web/src/components/chat/parts/DataSubprogressPart.tsx:47`
- **Error:** `TS2339: Property 'state' does not exist on type 'SubprogressStep'`
- **Cause:** The working tree contained PRE-EXISTING uncommitted user
  modifications to `DataSubprogressPart.tsx` (timeline modernization) before
  this task started. The WIP reads `data.state === 'pending'`, but
  `SubprogressStep` in `web/src/types/verdict.ts:118` has no `state` field.
- **Evidence it pre-dates this task:** `git status` at session start already
  listed `M web/src/components/chat/parts/DataSubprogressPart.tsx`; the error
  involves no file touched by this plan; the plan's own files type-check clean.
- **Impact on this plan:** the plan's `npx tsc --noEmit` exit-0 gate cannot be
  met while the user's WIP remains in the tree. All 272 vitest tests pass.
- **Owner:** the user's in-progress DataSubprogressPart work (likely needs a
  `state?: 'pending' | 'done'` addition to `SubprogressStep`, or the wire
  payload cast widened — user's call).
