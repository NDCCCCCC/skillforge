# Phase 03 — Deferred Items (Out-of-Scope Discoveries)

Tracked per the SCOPE BOUNDARY rule: pre-existing warnings, linting errors, or
failures in unrelated files are NOT auto-fixed by phase executors. Log them
here and surface in the phase SUMMARY so a future plan can address them.

## 2026-08-11 — Plan 03-01 Task 1 (GREEN verification)

- **Status:** acknowledged

**Discovery:** `cd web && npx tsc --noEmit` does NOT exit 0 — but **none of the
errors are caused by the 03-01 changes**. All errors live in Phase 4 test
files / components and reflect drift between Phase 4's wire-type fixtures and
the installed AI SDK v5 (the SDK was bumped past the version Phase 4 pinned).

**Affected files (all Phase 4 — NOT modified by 03-01):**

- `web/src/__tests__/MessageBubble.test.tsx` — uses `'dynamic-tool'` shape &
  `status: 'complete'`, both rejected by the current SDK (expects `'done'`).
- `web/src/__tests__/ReasoningPart.test.tsx` — `status: 'complete'` rejected.
- `web/src/__tests__/transport-swap.test.tsx` — `SkillForgeChunk[]` →
  `UIMessageChunk[]` assignability failures cascade from the `finish`
  variant's `finishReason?: string` vs SDK's `FinishReason` enum.
- `web/src/__tests__/wire-contract.test.ts` — compares the discriminated union
  against `"finish-message"` and `"tool-invocation"` (both intentionally
  absent per Pitfalls 1 & 3; the test predates the Pitfall 1/3 corrections).
- `web/src/components/chat/parts/ToolPart.tsx` — references `toolName` on a
  `tool-${string}` part shape that no longer carries that field.

**Verification that 03-01 is TS-clean:**

- `grep -E "^src" tsc-output` returns ZERO errors located in
  `web/src/types/wire-types.ts`.
- The D15 drift alarm line `_driftCheckTrue: _driftCheck = true`
  (wire-types.ts line ~99) is NOT in the error list — assignability holds
  for the new `data-decision` variant.
- The `wire-contract.test.ts(75,11)` error mentions "... 8 more ..." in the
  discriminator union — confirming the post-03-01 count, but the error
  itself (`"finish-message"` comparison) is Phase 4 vintage.

**Action:** Phase 4 wire-types were left intact per the SCOPE BOUNDARY rule.
A future plan (recommend filing under Phase 5 prep or a dedicated 04-04
"wire-contract realignment") should:
  1. Re-pin `ai` to the exact version Phase 4 was authored against, OR
  2. Migrate the Phase 4 fixtures / components to the current SDK shape.

Until then, the project-level TS compile stays non-zero — but the D15 drift
alarm (the actual 03-01 acceptance criterion) is green.
