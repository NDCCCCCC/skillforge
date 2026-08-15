---
quick: 260815-k8d
plan: 01
type: execute
subsystem: web/transport
tags: [draft-channel, draft-stream-chip, terminal-state, tdd, verified-idle]
tech_stack:
  patterns: [TDD-RED-GREEN, SSE-chunk-dispatch, terminal-idle-emit]
  added: []
key_files:
  created: []
  modified:
    - web/src/transport/MockChatTransport.ts
    - web/src/transport/RealChatTransport.ts
    - web/src/transport/RealChatTransport.test.ts
decisions:
  - "text-end is the load-bearing terminal signal for the conclusion text stream — emit ('idle', '') so DraftStreamChip unmounts after every successful verdict"
  - "MockChatTransport extends DraftStatus with 'idle' (single source of truth; RealChatTransport re-exports the same type)"
  - "text-end branch placed BEFORE controller.enqueue to keep the emit / enqueue ordering symmetric across both transports"
metrics:
  duration: 4m
  tasks: 2
  files_modified: 3
  commits: 2
  test_added: 1
  test_regression: 0
---

# Quick 260815-k8d: DraftStreamChip draftStatus verified → idle terminal transition

## One-liner

Closed the `pending → verified → idle` state-machine loop by emitting `('idle', '')` from both RealChatTransport and MockChatTransport when the conclusion text-end SSE frame arrives — DraftStreamChip now unmounts after every successful verdict instead of staying pinned at "✅ 已验证".

## Objective

Fix the DraftStreamChip permanent-hanging bug: after a successful verdict the chip displayed "✅ 已验证" indefinitely because no code path reset `draftStatus` back to `'idle'`. The fix routes the `text-end` SSE frame through `emitDraft('idle', '')` so RightChat unmounts the chip (`{draftStatus !== 'idle' && <DraftStreamChip … />}` already in place).

## Files Modified

| File | Change | Lines |
|------|--------|-------|
| `web/src/transport/RealChatTransport.ts` | Added `if (wireChunk.type === 'text-end') emitDraft('idle', '');` before `controller.enqueue(chunk)`. | +11 / -0 |
| `web/src/transport/MockChatTransport.ts` | Extended `DraftStatus` with `'idle'`. Added symmetric `if (chunkType === 'text-end') emitDraft('idle', '');` before `controller.enqueue`. | +14 / -1 |
| `web/src/transport/RealChatTransport.test.ts` | Added `'routes onDraftUpdate terminal: verified → idle on text-end (unmounts DraftStreamChip)'` test (full SSE sequence: start → text-start → 2× text-delta-draft → text-delta-draft-end → data-policy(verified) → text-delta(final) → text-end → finish). | +59 / -0 |

## Tasks Executed

| # | Type | Name | Commit | Status |
|---|------|------|--------|--------|
| 1 | auto (TDD-RED) | Add failing test for verified → idle terminal transition | `46d2593` | DONE — test failed with 4 emits (no idle) |
| 2 | auto (TDD-GREEN) | Emit idle on text-end in both transports | `df9e1b9` | DONE — test now passes (5 emits including idle) |

## Verification

- `cd web && npx tsc --noEmit` — exit 0 (no TypeScript errors; `DraftStatus` extension type-checks at all call sites)
- `cd web && npx vitest run src/transport/RealChatTransport.test.ts` — 5/5 PASS (4 existing + 1 new GREEN)
- `cd web && npx vitest run src/__tests__/MockChatTransport.test.ts` — 5/5 PASS (no regression)
- `cd web && npx vitest run` — 263/263 PASS across 37 files (was 262/262 before; +1 new test, 0 regressions)
- `cd web && npx vitest run src/components/chat/parts/DraftStreamChip.test.tsx` — 4/4 PASS

## Deviations from Plan

**1. Adjusted expected emit count** — plan spec said `updates.length >= 6`; actual post-fix count is 5 (2 draft deltas + 1 draft-end + 1 verified + 1 idle). Changed to `>= 5` to match the actual 2-draft-delta fixture (not 3 as in the older `routes onDraftUpdate: pending → verified` test). The semantic intent is unchanged: assert that the terminal emit is `idle`, not `verified`.

No other deviations. Plan executed exactly as written otherwise.

## Test Outputs

```
src/transport/RealChatTransport.test.ts  (5 tests)  17ms   ✓ all pass
  ✓ filters draft-* chunks from the ReadableStream surface
  ✓ routes onDraftUpdate in correct order: pending → verified
  ✓ routes onDraftUpdate: pending → refused
  ✓ routes onDraftUpdate terminal: verified → idle on text-end  ← NEW
  ✓ reconnectToStream returns null

src/__tests__/MockChatTransport.test.ts  (5 tests)  2883ms ✓ all pass
  ✓ sendMessages returns a ReadableStream emitting chunks in fixture order
  ✓ accepts a function fixture and resolves it once
  ✓ reconnectToStream returns null
  ✓ emits deterministic chunk order for identical fixture (snapshot)
  ✓ honors inter-chunk delay (delayMs ≥ 1)
```

## Known Stubs

None introduced by this plan. `app-store.setDraft` already accepts `'idle'` status (existing contract from Plan 260815-cos); RightChat already hides the chip when `draftStatus === 'idle'` (existing implementation). The fix is purely additive wiring — no stubs, no TODO, no placeholders.

## Threat Flags

None — this is a pure state-machine close-loop fix in the trusted frontend transport layer. No new network surface, no new auth path, no new file access pattern. STRIDE analysis unchanged from the prior `260815-cos` plan that introduced the draft channel.

## Success Criteria Check

- [x] DraftStreamChip unmounts after conclusion text-end (RightChat's `draftStatus !== 'idle'` guard)
- [x] State machine closed: `pending → verified → idle`
- [x] Both Real + Mock transports behave symmetrically on text-end
- [x] Zero regressions (263/263 tests pass)
- [x] TDD discipline: RED commit (`46d2593`) before GREEN commit (`df9e1b9`)

## TDD Gate Compliance

```
git log --oneline -3
df9e1b9 feat(260815-k8d): emit idle on text-end to unmount DraftStreamChip  ← GREEN
46d2593 test(260815-k8d): add failing test for verified → idle terminal transition  ← RED
175645f docs(260815-k8d): pre-dispatch plan for DraftStreamChip reset  ← base
```

RED gate present (`46d2593`) → GREEN gate present (`df9e1b9`). No REFACTOR commit (no duplication extracted; implementation is minimal and direct).