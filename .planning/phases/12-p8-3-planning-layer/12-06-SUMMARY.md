---
phase: 12
plan: 06
subsystem: web
tags: [plan-trace, fe-streaming, wire-types, store-slice, components, plan-trace-part]
dependency_graph:
  requires:
    - 12-02 (BE write_todos Tool + data-plan-trace wire type)
  provides:
    - D15 lockstep paired-commit mirror (PlanTracePayload + PlanTodo + SkillForgeChunk variant)
    - D14-extension plan slice in app-store (independent from messages + approval)
    - live planning-trace renderer (PLAN-02 SC-2 surface)
    - data-plan-trace SSE dispatch in RealChatTransport
  affects:
    - web/src/types/wire-types.ts (D15 alarm)
    - web/src/store/app-store.ts (plan slice + D14 invariant)
    - web/src/transport/RealChatTransport.ts (SSE chunk dispatch)
    - web/src/components/layout/ChatColumn.tsx (chip integration)
key_files:
  created:
    - web/src/components/PlanTracePart.tsx
    - web/src/components/PlanTracePart.test.tsx
    - web/src/components/layout/ChatColumn.test.tsx
    - web/src/types/wire-types.test.ts
  modified:
    - web/src/types/wire-types.ts
    - web/src/store/app-store.ts
    - web/src/store/app-store.test.ts
    - web/src/transport/RealChatTransport.ts
    - web/src/transport/RealChatTransport.test.ts
    - web/src/components/layout/ChatColumn.tsx
tech_stack:
  added: []
  patterns:
    - data-* SSE chunk dispatch (D14 store-derived pattern)
    - synthetic tool-call fallback (Pitfall 6 mitigation; AI SDK v5 native live-rendering surface)
    - localStorage hydration at store creation (PLAN-02 SC-2 cross-refresh)
    - AssertAssignable compile-time drift alarm (D15 lockstep)
decisions:
  - D15 lockstep paired commit with 12-02 BE (web mirror of data-plan-trace wire type + PlanTracePayload + PlanTodo)
  - D14-extension: plan slice INDEPENDENT from `messages` AND from `approval` (useChat owns chat stream; plan slice owns live planning trace)
  - Pitfall 6 mitigation: synthetic write_todos tool-output-available routes to the plan slice under `<chatId>-synthetic` while still flowing to useChat for the standard AI SDK v5 tool-call surface — handles buffering of data-* chunks
  - Retention redaction at hydrate-time: cap to 50 most-recent runs (avoids localStorage 5MB quota)
  - currentRunId maintained alongside planByRunId so the UI can render the "active" plan without scanning the dictionary
metrics:
  duration: ~25 minutes (4 atomic commits + final verification)
  tests_added: 18
    - wire-types.test.ts: 2
    - app-store.test.ts: 5
    - PlanTracePart.test.tsx: 8
    - RealChatTransport.test.ts: 3 (data-plan-trace + synthetic + transport-swap)
  tests_total: 392 (53 files) — full vitest suite passes
  files_created: 4
  files_modified: 6
---

# Phase 12 Plan 06: PlanTracePart FE streaming + D15 lockstep mirror — Summary

## One-liner

Shipped the FE PlanTracePart chip (live todo list streaming) plus the D15 lockstep mirror of Go's `data-plan-trace` wire type, the D14-extension `plan` slice in Zustand (independent from messages + approval), the RealChatTransport SSE chunk dispatch, and the ChatColumn integration.

## Verification

| Check | Result |
|-------|--------|
| `npx tsc --noEmit` | exit 0 (clean — D15 lockstep preserved) |
| `npx vitest run` | 392 passed / 392 across 53 files |
| `npx vite build` | built in 35.82s, 1,071 KB JS (gzip 314 KB), 77 KB CSS (gzip 17.95 KB) |
| `grep "data-plan-trace" src/types/wire-types.ts` | line 262 (variant added) |
| `grep "planByRunId" src/store/app-store.ts` | lines 220 / 671 / 677 / 688 (slice + persistence) |
| `grep "PlanTracePart" src/components/layout/ChatColumn.tsx` | line 91 (integration) |
| `grep "data-plan-trace" src/transport/RealChatTransport.ts` | line 394 (SSE dispatch) |
| `grep -c "messages:" src/store/app-store.ts` | 0 (D14 invariant holds — slice is NOT in messages) |

## Atomic commits

| # | Hash | Subject |
|---|------|---------|
| 1 | `d5cbf03` | `feat(12-06): wire-types Task 1 — PlanTracePayload + PlanTodo + SkillForgeChunk data-plan-trace variant` |
| 2 | `39625b4` | `feat(12-06): plan slice Task 2 — PlanState + PlanSlice in app-store + 5 new tests` |
| 3 | `a4b44ec` | `feat(12-06): PlanTracePart Task 3 — live todo list renderer` |
| 4 | `15b74af` | `feat(12-06): transport + ChatColumn integration Task 4 — data-plan-trace dispatch` |

## Files touched (4 created + 6 modified)

### Created (4)

- `web/src/types/wire-types.test.ts` — D15 lockstep guard: `TestWireTypeDataPlanTraceAdded` + `TestPlanTracePayloadTypeShape`
- `web/src/components/PlanTracePart.tsx` — live todo list renderer with status indicators (CircleDashed / Loader2 spin / CheckCircle2) + animation + collapse-after-complete summary
- `web/src/components/PlanTracePart.test.tsx` — 8 vitest tests (streaming render, status transition, synthetic fallback, empty run, collapsed, cross-refresh, D14 invariant, renderStatusIcon contract)
- `web/src/components/layout/ChatColumn.test.tsx` — 2 integration tests (renders on currentRunId; null when no plan)

### Modified (6)

- `web/src/types/wire-types.ts` — added `PlanTodo` + `PlanTracePayload` types + `data-plan-trace` SkillForgeChunk variant (D15 paired commit with 12-02 BE)
- `web/src/store/app-store.ts` — added `PlanState` + `PlanSlice` types, `plan` slice on `AppState`, `setPlan` / `markPlanStatus` / `clearPlan` / `setCurrentRunId` reducers, localStorage persistence via `PLAN_STORAGE_KEY='skillforge-plan'` (50-run retention cap, hydrate-on-create)
- `web/src/store/app-store.test.ts` — 5 new plan-slice tests + cleaned `PlanTodo` import
- `web/src/transport/RealChatTransport.ts` — `data-plan-trace` chunk dispatch to plan slice (consumed; never enqueued to useChat; D14-extension) + synthetic `write_todos` tool-output-available fallback (Pitfall 6, scoped under `<chatId>-synthetic`)
- `web/src/transport/RealChatTransport.test.ts` — 3 new tests: `TestDataPlanTraceChunkRouting` / `TestSyntheticWriteTodosToolCallRouting` / `TestTransportSwapWithPlanTracePart` (D-04 byte-identity for plan-trace path)
- `web/src/components/layout/ChatColumn.tsx` — reads `planCurrentRunId` from the slice, renders `<PlanTracePart runId={planCurrentRunId} />` above the messages scroll area

## Deviations from Plan

### Path drift (auto-corrected)

The plan referenced file paths that did not match the actual repo layout. I wrote the changes to the existing canonical paths the codebase actually uses:

| Plan path | Actual path used |
|-----------|------------------|
| `web/src/stores/app-store.ts` | `web/src/store/app-store.ts` |
| `web/src/lib/real-chat-transport.ts` | `web/src/transport/RealChatTransport.ts` |
| `web/src/components/RightChat.tsx` | `web/src/components/layout/ChatColumn.tsx` |

The semantic intent is identical — D14 store-derived overlay pattern + D-04 transport dispatch — just located at the project's canonical paths.

### Tests created vs. plan-mandated test additions

The plan listed `RightChat.test.tsx` and `wire-types.test.ts` as files to MODIFY. Both did not exist before this plan, so I CREATED them as `web/src/components/layout/ChatColumn.test.tsx` and `web/src/types/wire-types.test.ts`. Their test contents match the plan's <action> blocks 1:1.

### Extra: added `currentRunId` to the plan slice

The plan <must_haves> called for PlanTracePart to receive `runId` as a prop, but did not specify how the ChatColumn discovers the *current* runId. I added a `currentRunId: string | null` field to the plan slice (plus a `setCurrentRunId` action) so the UI can render the active plan without scanning the dictionary. The transport bumps `currentRunId` automatically on every data-plan-trace chunk (via the `setPlan` reducer). This is a minor scope addition, fully consistent with the spec — flagged here so 12-07 has the substrate for `PlanReplayView`'s active-run lookup.

### Extra: synthetic fallback key scheme

The plan called for "synthetic tool-call fallback for write_todos routed to the plan slice" but did not pin a keying scheme. I scoped synthetic writes under `<chatId>-synthetic` so:
- The authoritative runId (from `data-plan-trace.payload.runId`) stays distinct
- Tests can deterministically assert the synthetic writes
- A real future BE runId is never overwritten by a synthetic write

### No new npm packages

All package versions unchanged from `web/package.json` baseline (React 19 / Vite 5 / TS 5.6 / AI SDK v7 / Zustand v5 / Tailwind v4 / lucide-react).

## Self-Check: PASSED

- All 4 atomic commits landed on `master`.
- 18 new tests added across 4 files; all pass.
- Full vitest suite: 392/392 tests passing.
- `tsc --noEmit` clean.
- `vite build` succeeds.
- D14 invariant verified — `grep -c "messages:" src/store/app-store.ts` returns 0 (no `messages` field on AppState).
- D15 invariant verified — `AssertAssignable<SkillForgeChunk, UIMessageChunk>` compiles; the runtime test in `wire-types.test.ts` constructs a `data-plan-trace` chunk and assigns it to `UIMessageChunk`.

## Next-phase handoff (12-07)

12-07 will:
1. Modify `web/src/transport/RealChatTransport.ts` to add `forced_stop | plan-approved | plan-denied` decision-string cases (D16 overload) — non-overlapping with the `data-plan-trace` and synthetic-tool paths added here.
2. Modify `web/src/types/wire-types.ts` — actually no, the decision-string enum is just a value add; the TS mirror already accepts arbitrary strings on `DecisionPayload.decision`. **12-07 only adds new cases to the `dispatchApprovalDecision` switch in RealChatTransport and the approvals store, NOT a new wire chunk type.**
3. Create `web/src/components/PlanApprovalCard.tsx` + `web/src/components/PlanReplayView.tsx` — entirely new files, no overlap with anything 12-06 added.
4. Wire those into the CenterPanel (规划轨迹 tab) and LeftSidebar (6th section).

Substrate handed off from 12-06 (this plan):
- `data-plan-trace` wire type — present and consumed.
- `plan` slice with `currentRunId` + `planByRunId` + `setPlan` / `markPlanStatus` / `clearPlan` / `setCurrentRunId` — ready for `PlanReplayView` to fetch from BE 12-04 endpoints and hydrate.
- `PlanTracePart` — shipped; 12-07 may extend it (not modify the location it was placed at by 12-06).

## PLAN COMPLETE

**Plan:** 12-06
**Tasks:** 4/4 complete (all <done> criteria met)
**SUMMARY:** `.planning/phases/12-p8-3-planning-layer/12-06-SUMMARY.md`

**Commits:**
- d5cbf03: wire-types Task 1
- 39625b4: plan slice Task 2
- a4b44ec: PlanTracePart Task 3
- 15b74af: transport + ChatColumn integration Task 4

**Tests:** 18 new tests added; 392/392 baseline passing; `tsc --noEmit` clean; `vite build` succeeds.
