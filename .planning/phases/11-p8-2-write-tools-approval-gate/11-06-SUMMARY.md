---
phase: 11
plan: 06
subsystem: frontend
tags: [hitl, approval-gate, zustand, sse, react, vitest]
requires:
  - 11-04 (BE approval endpoints + data-decision wire contract)
provides:
  - Zustand approval slice (pending/terminal/history/resumeBanner + 8 actions)
  - ApprovalCard store-derived overlay (6 visual states, button trio, edit mode, scope radio, deny prompt)
  - RealChatTransport data-decision dispatch (interrupted/tool-resumed/tool-denied/rule-matched)
  - D11-B4 read-side slice contract for 11-07 SkillPicker
affects:
  - 11-07 (consumes the slice shape + ApprovalCard mount)
  - 11-08 (E2E acceptance gate exercises this flow)
tech-stack:
  added: []
  patterns:
    - Store-derived overlay (D14): UI reads store.approval, never the SSE stream
    - D15 string-enum extension: DecisionPayload.decision +4 strings, zero new chunk types
    - D-04 dispatch-only interception: HITL chunks never enqueued to useChat
key-files:
  created:
    - web/src/components/ApprovalCard.tsx
    - web/src/components/ApprovalCard.test.tsx
  modified:
    - web/src/store/app-store.ts
    - web/src/store/app-store.test.ts
    - web/src/transport/RealChatTransport.ts
    - web/src/transport/RealChatTransport.test.ts
    - web/src/types/wire-types.ts
    - web/src/components/layout/ChatColumn.tsx
key-decisions:
  - "POST body uses the BE snake_case approvalPostRequest shape (frozen_args/approval_id) — BE is wire ground truth over the plan's camelCase sketch"
  - "Terminal transitions driven by HTTP status (200→approved / 410→expired / other→failed) because the BE streams resume events on 200 rather than returning JSON {status:'ok'}"
  - "HITL data-decision chunks are dispatched to the store and NEVER enqueued to useChat (D-04: pre-existing decisions flow through verbatim)"
requirements-completed: [HITL-01, HITL-02, HITL-07]
duration: 25 min
completed: 2026-08-20
---

# Phase 11 Plan 06: FE ApprovalCard + Approval Slice + Transport Dispatch Summary

Shipped the frontend half of the HITL approval gate: a D14-clean Zustand `approval` slice, the store-derived `ApprovalCard` overlay (6 visual states, 批准一次/批准并记住/拒绝, edit-before-approve, binary scope radio, ≥3-char deny reason), and the `RealChatTransport` data-decision dispatch for the 4 Phase 11 decision strings — with D-04 (no ApprovalCard in the SSE path) and D15 (no new chunk types) locked by grep guards and tests.

## Execution Metrics

- **Duration:** ~25 min (12:44–12:58 CST, interleaved with parallel 11-07 executor)
- **Tasks:** 7/7 complete
- **Files:** 8 (2 created, 6 modified)
- **Commits:** 7 (f30800a, 9de58ff, 6696e39, 7f61677, 5c7a990, c7b7113, 7b8e618)

## What Was Built

### Task 1 — `web/src/store/app-store.ts` (extended, f30800a)
`ApprovalSlice` on the store: `pending | terminal | history | resumeBanner` + 8 actions (`setPending`/`resolvePending`/`clearPending`/`clearTerminal`/`appendHistory`/`setResumeBanner`/`dismissResumeBanner`/`hydrateFromGet`). 8 exported types. `resolvePending` guarded against stale SSE events (approvalId must match live pending). History bounded to 200. `hydrateFromGet` mirrors the BE snake_case GET row and converts to the camelCase slice. Additive only; D14 grep guard = 0 matches.

### Task 2 — `web/src/components/ApprovalCard.tsx` (new, 9de58ff)
Store-derived z-25 overlay mounted from `ChatColumn.tsx` (plan Task 2-J integration point; chat-column root made `relative`). Pending card: mono tool name, derived what-line, target+reason line, FrozenArgs JSON collapsed by default (展开/收起), 编辑参数 edit mode with 已修改参数，待审批将以新值执行 banner (HITL-07), button trio, scope radio only under 批准并记住 (exactly 2 options, default session — D11-D3), deny prompt disabled under 3 chars (HITL-02). 5 terminal visuals (approved/denied/expired/auto-passed/failed) with status-token left borders. POSTs via `fetch` with the BE snake_case body; 200→approved / 410→expired / other→failed (D11-C3 fail-closed).

### Task 3 — `web/src/transport/RealChatTransport.ts` + `web/src/types/wire-types.ts` (extended, 6696e39)
`dispatchApprovalDecision` intercepts `interrupted` / `tool-resumed` / `tool-denied` / `rule-matched` in the SSE frame loop → store dispatch only, chunks never enqueued. Pre-existing decision strings (`route`/`retrieve`/`counter_evidence`/`history_*`) fall through verbatim (D-04). `DecisionPayload.decision` TS union extended by the same 4 strings (D15 string-enum extension, mirrors the Go plain-string field). ApprovalCard NOT imported (grep guard = 0).

### Tasks 4–6 — tests (7f61677, 5c7a990, c7b7113)
- `ApprovalCard.test.tsx` (new): TestApprovalCardRendering, TestApprovalCardStateTransitions (6 states), TestApprovalCardEditBeforeApprove (edited `frozen_args` reach POST body), TestApprovalCardScopeRadio (radio conditional + default session + exactly 2 options), TestApprovalCardDenyRequiresReason (submit gated ≥3 chars).
- `RealChatTransport.test.ts` (extended): TestTransportSwapWithApprovalCard (D-04 static + functional: HITL chunks dispatch-only, `route` decision flows through), TestWireTypeCompilation (D15: no data-* literals beyond data-decision; union carries 4 new + 6 existing strings), + bonus SSE race-safety test.
- `app-store.test.ts` (extended): TestSkillSwitchGuardDisabled, TestSkillSwitchGuardReenabled (D11-B4 read-side contract), TestResolvePendingRaceSafety.

### Task 7 — verification gate
- `npx tsc --noEmit` → exit 0
- `npx vitest run` → **50 files / 372 tests, all pass** (11 new; zero regressions)
- D14 guard: `grep 'messages.*approval|approval.*messages' app-store.ts` → 0
- D-04 guard: `grep 'ApprovalCard' RealChatTransport.ts` → 0
- D15 guard: `grep "type: 'data-" RealChatTransport.ts | grep -v data-decision` → 0
- D11-B4 guard: both SkillSwitchGuard test names present
- No new package.json dependencies; no new SSE chunk types

## Deviations from Plan

The plan was written against an idealized file layout; the actual repo differs. All deviations are Rule 1/3-class (missing critical / blocker) and preserve the plan's contracts.

1. **[Rule 3 — path drift]** `web/src/stores/app-store.ts` → actual `web/src/store/app-store.ts`; `web/src/lib/real-chat-transport.ts` → actual `web/src/transport/RealChatTransport.ts`. Both test files existed and were EXTENDED (not created).
2. **[Rule 3 — no axios]** axios is not in package.json and the plan forbids new deps → POST via `fetch` with `AbortSignal.timeout(10000)`.
3. **[Rule 3 — no user-event]** `@testing-library/user-event` not installed → `fireEvent` from `@testing-library/react`.
4. **[Rule 3 — BE wire ground truth]** POST body uses the BE's snake_case `approvalPostRequest` (`frozen_args`, `approval_id`, `target`); tests assert snake_case. `PendingApprovalRow` mirrors the BE GET row (snake_case, `created_at` RFC3339); `hydrateFromGet` converts and accepts a ctx for sessionId/skillId.
5. **[Rule 1 — substrate]** `web/src/types/wire-types.ts` extended (DecisionPayload union +4 strings) — not in the plan's touched list but it is the D15 compile-alarm substrate for exactly this extension.
6. **[Rule 1 — plan-sanctioned]** `ChatColumn.tsx` mount point added (plan Task 2-J explicitly instructs this).
7. **[Rule 3 — response shape]** BE streams resume events on POST 200 (no JSON `{status:'ok'}`) → terminal transitions driven by HTTP status.
8. **[Rule 3 — tooling]** eslint gate skipped: eslint is not installed in `web/` (no dep, no config). The repo's gates are tsc + vitest; both green.
9. **[Rule 3 — store shape]** flat `AppState` gained a nested `approval` object (selectors `s.approval.pending` work as the plan intended) instead of a `State = ApprovalSlice & …` intersection.
10. **Bonus:** 11th test (SSE race-safety in transport tests) beyond the plan's 10.
11. **Parallel execution:** the 11-07 executor interleaved commits on `master` during this run (its files are its own plan's ownership). All gates were re-verified green on the merged HEAD (372/372).

**Total deviations:** 10 auto-fixed + 1 note. **Impact:** none on contracts — every plan acceptance criterion is met on the actual repo layout; all invariant grep guards green.

## Authentication Gates

None.

## Issues Encountered

Transient duplicate-identifier tsc errors during Task 1 caused by the parallel 11-07 executor editing `app-store.ts` concurrently; resolved itself once both lines of work settled (final tree has exactly one `ApprovalSlice`; tsc exit 0).

## Next Phase Readiness

11-06 substrate complete. 11-07 (rules panel + audit tab + resume banner + SkillPicker guard) executed in parallel and its 12 tests are green on the same HEAD. Ready for **11-08** (E2E acceptance gate, Wave 5).

## Self-Check: PASSED

- key-files.created exist on disk (ApprovalCard.tsx, ApprovalCard.test.tsx) ✓
- `git log --grep="11-06"` returns 7 commits ✓
- All task acceptance_criteria re-run and passing ✓
- Plan-level verification commands green (tsc 0; vitest 372/372; 4 grep guards 0/present) ✓
