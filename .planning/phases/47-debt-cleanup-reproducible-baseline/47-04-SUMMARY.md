---
phase: 47-debt-cleanup-reproducible-baseline
plan: 04
subsystem: admin/governance
tags: [rel-02, rollback, shacl, refetch, callback-prop, wr-07]

# Dependency graph
requires:
  - phase: 47-02
    provides: "RollbackTBox republishes its post-rollback SHACL report into the store cache (e7ca19a) — GET /governance/shacl now returns trigger=rollback right after a rollback"
  - phase: 40-data-governance-shacl-provenance-tbox-versioning
    provides: "governanceApi fetch wrappers, HealthBanner trigger branch, ShaclReportView onRetry callback-prop precedent"
provides:
  - "TimelineView notifies its parent (onRollbackComplete) after a successful rollback, guarded so it cannot mask success as failure"
  - "One refreshShaclReport implementation in GovernanceTab serving both the manual retry and the post-rollback path, WR-07 stale-error semantics preserved"
  - "TimelineView.test.tsx — regression coverage for the callback contract (fired once on success, never on failure, throwing parent harmless)"
affects: [47-06 (cites the SHA for the WARNING-1 / REL-02 evidence row), 47-08 (FE gate segment runs TimelineView.test.tsx), phase-48-ci]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Optional callback prop as the child-to-parent signal; the parent owns the fetch and the state (mirrors ShaclReportView.onRetry)"
    - "Non-rethrowing refresh handler: a failed refresh is state, not a promise rejection the caller cannot handle"
    - "Mutation-verified guard tests: temporarily drop the try/catch, confirm the test fails, revert"

key-files:
  created:
    - web/src/components/admin/governance/TimelineView.test.tsx
  modified:
    - web/src/components/admin/governance/TimelineView.tsx
    - web/src/components/admin/governance/GovernanceTab.tsx

key-decisions:
  - "Callback fires after the changelog refetch, inside the success path but outside any awaited chain — it must not delay setRollingBack(null)"
  - "refreshShaclReport is a plain arrow function in the component body (no useCallback): it closes over the two setState setters, which are stable, and no lint rule in this repo requires memoization"
  - "GovernanceTab.tsx has no HealthBanner diff by design — the trigger === 'rollback' branch already existed and only needed a fresh report to become reachable"

patterns-established:
  - "Non-vacuous guard assertion: 'no .sf-toast-error in the document' plus 'success toast present' is only meaningful because a deliberately removed guard was shown to produce 回滚失败：refetch blew up"

requirements-completed: [REL-02]

# Metrics
duration: 8min
completed: 2026-10-19
---

# Phase 47 Plan 04: Post-Rollback SHACL Report Refetch (FE) Summary

**A successful rollback in 版本演进 now refetches the cached SHACL report, so the shared HealthBanner re-renders from the boot-time report to the post-rollback one and the previously dead `trigger === 'rollback'` branch shows 「回滚后」 — closing the frontend half of REL-02 (D-02b).**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-10-19
- **Completed:** 2026-10-19
- **Tasks:** 3
- **Files modified:** 3 (2 production, 1 new test)

## Accomplishments

- **Closed the hand-off that made 47-02's backend write invisible.** `RollbackTBox` has been republishing its revalidation report into the store cache since `e7ca19a`, but `TimelineView` only refetched the changelog and `GovernanceTab` fetched the report once on mount — switching to the 版本演进 view does not remount the tab, so the fresh report never reached the banner. `onRollbackComplete` closes that gap with one optional prop and no new UI.
- **A completed rollback can no longer be reported as a failed one.** The callback invocation is wrapped in `try { onRollbackComplete?.(); } catch {}`, so a throwing parent handler cannot reach the outer `catch` that renders `回滚失败：…` (threat T-47-14). This is mutation-verified, not merely asserted — see below.
- **One refresh implementation, two call sites.** The body previously inlined in `onRetry` moved verbatim into `refreshShaclReport`, WR-07 comment included, and now also serves the post-rollback path. It never rethrows: a failed refresh becomes `shaclError` state, which is exactly the "stale-but-honest report plus an error state" posture SC-3 asks for.
- **No render change was needed, and none was made.** `HealthBanner`'s `report.trigger === 'boot' ? '启动时' : '回滚后'` expression and the `VIEWS` array are byte-identical in `git diff`. The branch was never wrong — it was unreachable, because nothing ever delivered a `trigger: 'rollback'` report to it.
- **No dependency change.** `git diff 77a2b95..HEAD -- web/package.json web/package-lock.json` is empty; the new test uses only `vitest`, `@testing-library/react` and `@testing-library/jest-dom/vitest`, all already in devDependencies (T-47-SC).

## Task Commits

Each task was committed atomically, by explicit path (never `git add -A`):

1. **Task 1: Add the onRollbackComplete callback prop to TimelineView** — `197421e` (feat)
2. **Task 2: Extract one shared report-refresh handler and wire it to both call sites** — `bf1c1e0` (feat)
3. **Task 3: Add TimelineView.test.tsx covering the rollback completion callback** — `abbb594` (test)

**Evidence commit for plan 47-06:** `abbb594` — the REL-02 / WARNING-1 frontend evidence row should cite this SHA; pair it with `e7ca19a` (the backend half from 47-02).

## Files Created/Modified

- `web/src/components/admin/governance/TimelineView.tsx` — new exported `TimelineViewProps` with optional `onRollbackComplete`, component signature now `TimelineView({ onRollbackComplete }: TimelineViewProps = {})` (defaulted so an unwired parent still mounts). The success path invokes it inside a `try`/`catch` after the changelog refetch. The success toast literal and the `window.confirm` literal (including its 隔离区 sentence) are untouched — confirmed by `git diff 197421e`.
- `web/src/components/admin/governance/GovernanceTab.tsx` — `refreshShaclReport` defined once in the component body (one `fetchShaclReport()` call site inside it; the mount `useEffect` keeps its own), passed as `ShaclReportView.onRetry` and `TimelineView.onRollbackComplete`. `HealthBanner`, `VIEWS`, the view switcher and the mount effect are unchanged.
- `web/src/components/admin/governance/TimelineView.test.tsx` (new, tracked) — three cases: callback fired exactly once on success with the unchanged toast text; callback never fired on rollback failure; a throwing parent handler still leaves the success toast and no error toast, and the button re-enables (proving `finally` still runs). Drives only the pre-existing `governance-rollback-${version}` testids — no production component gained a `data-testid`.

## Banner-flip trace (mandatory — SC-2's central evidence)

The plan mandates quoting the two banner label strings **as observed**, not as expected. Observed by rendering the real `GovernanceTab` in jsdom with a URL-routed `fetch` stub whose `/api/admin/governance/shacl` answer switches from `trigger: "boot"` to `trigger: "rollback"` on the rollback POST, driving the existing `governance-view-timeline` and `governance-rollback-2` testids. Verbatim console output:

```
=== TRACE BEFORE ===
发现 error 级违规 — 1 个实体已隔离error 1warning 0校验时点 2026/10/19 11:00:00（启动时）· shapes data/shacl/shapes.ttl
=== TRACE AFTER  ===
发现 error 级违规 — 1 个实体已隔离error 3warning 0校验时点 2026/10/19 12:00:00（回滚后）· shapes data/shacl/shapes.ttl
=== TRACE TOAST  ===
已回滚至 v2（当前 v4）· 新增 2 条 violation
=== shacl GET count: 2
```

What this shows, term by term:

| Claim | Evidence in the trace |
|---|---|
| Banner label before rollback | `（启动时）` |
| Banner label after rollback | `（回滚后）` |
| Toast violation count | `新增 2 条 violation` |
| Refreshed report's `error_count` | `error 3` = pre-rollback `error 1` + the toast's `新增 2` — both derive from the same revalidation (D-02f) |
| Request cost | exactly **2** SHACL GETs — the mount fetch plus one post-rollback refetch. No polling, no retry loop (T-47-15) |
| No stale-error masking | the error branch never entered; `shaclError` stayed null, so the banner rendered the report, not 「校验报告加载失败」 (T-47-16) |

**Scope honesty — what this trace is and is not.** This is a jsdom execution of the *real, committed* FE components (`GovernanceTab`, `HealthBanner`, `TimelineView`, the real `governanceApi` fetch wrappers) with only the HTTP transport stubbed. It is **not** a live browser session against a running Hertz server, and the server-side half of the handshake is evidenced separately by 47-02's `TestRollbackShaclParity` (`e7ca19a`, `d3f86c2`), not by this trace. The temporary harness used for the trace was deleted after the run and is not part of the deliverable — `git status web/` is clean apart from two pre-existing untracked pnpm files unrelated to this phase. The trace proves the FE hand-off; the Go test proves the cache write; together they are the backend-plus-frontend handshake SC-2 requires. A live intranet run was not attempted because Milvus/BGE are intermittently unreachable per the project's standing environment caveat, and the FE contract is transport-independent.

## Verification

| Gate | Result |
|---|---|
| `npx tsc --noEmit` (in `web/`) | exit 0 (after Task 1, after Task 2, and after Task 3) |
| `npx vitest run src/components/admin/governance/ShaclReportView.test.tsx` (Task 1 gate) | exit 0 — 1 file / 9 tests passed |
| `npx vitest run src/components/admin/governance/` (Task 2 gate) | exit 0 — 2 files / 13 tests passed |
| `npx vitest run src/components/admin/governance/TimelineView.test.tsx` (Task 3 gate) | exit 0 — **3 passed / 0 failed / 0 skipped** |
| `npx vitest run` (full FE suite) | exit 0 — **85 files / 688 tests passed, 0 failed, 0 skipped** |
| Baseline comparison | plan 47-05 recorded 685 passed / 84 files; this plan adds +1 file and +3 tests → 688 / 85. No pre-existing test was modified, skipped or removed |
| `git diff 77a2b95..HEAD -- web/package.json web/package-lock.json` | empty — no dependency added (T-47-SC) |
| `git diff 77a2b95..HEAD --stat` | exactly 3 files: `GovernanceTab.tsx`, `TimelineView.tsx`, `TimelineView.test.tsx` |
| `git ls-files web/src/components/admin/governance/TimelineView.test.tsx` | non-empty — the new test is **tracked**, not left untracked |
| `git diff 197421e -- web/src/components/admin/governance/TimelineView.tsx` | empty — confirms the success toast literal `已回滚至 v${targetVersion}（当前 v${result.version}）· 新增 ${result.new_violations} 条 violation` and the `window.confirm` literal are byte-identical (D-02e / D-05b) |
| Occurrences of `已回滚至 v` changed in the Task 1 diff | 0 |
| `fetchShaclReport()` call sites in `GovernanceTab.tsx` | 2 — line 102 (mount `useEffect`, unchanged) and line 117 (inside `refreshShaclReport`, the single implementation) |
| `.planning/STATE.md` / `.planning/ROADMAP.md` | not modified |

### Mutation check (non-vacuity of the guard test)

The `try`/`catch` around `onRollbackComplete?.()` was temporarily removed and the new suite re-run:

```
× TRACE-free suite: a throwing parent handler does not turn a successful
  rollback into the failure toast
  → expect(document.body.querySelector('.sf-toast-error')).toBeNull()
    Received: 回滚失败：refetch blew up
      Tests  1 failed | 2 passed (3)
```

Without the guard, the user's completed rollback is reported as `回滚失败`. The mutation was reverted immediately (`git diff --stat` on `TimelineView.tsx` returned empty, i.e. byte-identical to commit `197421e`) and the suite returned to 3/3. The committed tree contains no probe code.

## Deviations from Plan

Two, both additive and inside the plan's stated intent:

1. **Task 3 ships three `it` cases, not two.** The plan's two required cases (fired once on success / never on failure) are both present. The third — a throwing parent handler must not produce `回滚失败：…` — was added because it is the only automated proof of threat T-47-14's mitigation, and because a `try`/`catch` that no test can fail is decoration. The plan's acceptance criterion ("contains two `it(` cases") is satisfied; the extra case strictly strengthens it.
2. **Task 1 also corrected the stale `// Refetch all three surfaces + switch view to shacl` comment.** That comment is the FE half of the bug this plan closes — it asserted a behaviour the code never had. It now names what actually happens. Comment-only; no behaviour, no copy visible to the user.

Not done, deliberately: the `window.confirm` text's 隔离区 sentence stays as-is even though rollback does not actually quarantine. That is **D-05**, a recorded known limitation owned by `47-DEBT-DISPOSITION.md`, and D-05b explicitly forbids fixing it inside REL-02. The banner's `N 个实体已隔离` is overstated on the rollback path for the same reason — visible in the trace above (`发现 error 级违规 — 1 个实体已隔离` on a rollback that performed no isolation). This is the known defect, reproduced, not introduced.

No other files were touched. The `useCallback` question resolved itself: no lint rule in this repo requires memoization for handlers passed to plain DOM-props components, and `refreshShaclReport` only closes over the two stable `setState` setters.

## Known Stubs

None. No placeholder values, no empty data paths, no unwired props. `onRollbackComplete` is genuinely consumed by `GovernanceTab`, which genuinely calls `fetchShaclReport()`, which genuinely updates the state the banner renders.

## Notes for Downstream Plans

- **47-06** should cite `abbb594` (FE) alongside `e7ca19a` (BE) for the WARNING-1 / REL-02 evidence row, and can point at the banner-flip trace in this SUMMARY as the handshake proof.
- **47-08**'s FE gate segment should run `npx vitest run src/components/admin/governance/TimelineView.test.tsx`; the full-suite baseline is now **688 / 85 files**.
- **REL-02 is now complete on both halves** — 47-02 wrote the post-rollback report into the cache, this plan delivers it to the banner. No further work is needed on WARNING-1.
- **D-05 remains open** (rollback does not execute quarantine, so both the confirm dialog and the banner overstate it). Its disposition belongs in `47-DEBT-DISPOSITION.md`, not here.

---
*Plan: 47-04 | Phase 47: 清债与可复现基线 | REL-02 frontend half*
