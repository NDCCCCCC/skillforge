---
phase: 39-ecl-pipeline
plan: 08
subsystem: ui
tags: [react, admin-console, polling, vitest, tailwind-v4, ecl-pipeline, dead-letter]

# Dependency graph
requires:
  - phase: 39-07
    provides: "3 token-gated /api/admin/pipeline/* routes — StatusDTO key set, run 200/409/403/503, retry 200/400/404/409/403/503"
  - phase: 38-05
    provides: "SubrouteTab registration idiom, ontology/ folder pattern, SchemaProposalsTab control-strip/table/badge idioms"
provides:
  - "9th admin tab 「ECL 管线」 (admin-tab-pipeline, Workflow icon) rendering /admin/pipeline per 39-UI-SPEC"
  - "pipeline/{types.ts,pipelineApi.ts,PipelineTab.tsx} — StatusDTO mirror + 3 X-Admin-Token calls with typed {status,message} errors"
  - "Polling hook pattern: 2s running / 10s idle, re-armed on stage flip, proven under vi.useFakeTimers"
  - "13-test pipeline suite: copy contract, cadence re-arm, confirm-gated run, dead-only retry, 403/503 inline copy, error survival"
affects: [40-governance (next tab registration), 41-platform (SPARQL playground tab)]

# Tech tracking
tech-stack:
  added: []   # zero new dependencies — UI-SPEC §Design System + Registry Safety
  patterns:
    - "Cadence-flip polling: window.setInterval effect keyed on the computed interval (isRunning ? 2000 : 10000) — re-arm on server-truth flip"
    - "Typed PipelineApiError {status, message} so the view branches 403/409/503 and renders server reasons verbatim"
    - "Server-truth-only lifecycle: no optimistic running state; Run/retry POSTs bypass the poller and trigger immediate fetchOnce"

key-files:
  created:
    - web/src/components/admin/pipeline/types.ts
    - web/src/components/admin/pipeline/pipelineApi.ts
    - web/src/components/admin/pipeline/PipelineTab.tsx
    - web/src/components/admin/pipeline/PipelineTab.test.tsx
  modified:
    - web/src/components/admin/AdminDashboard.tsx
    - web/src/store/app-store.ts
    - web/src/components/admin/ontology/OntologyTab.test.tsx
    - web/src/__tests__/AdminFeedbackTabs.test.tsx

key-decisions:
  - "PipelineTab copy/type/cadence tokens follow 39-UI-SPEC verbatim (checker 6/6 contract): confirm text, hint, stage labels, badge treatments, toasts, 403/503 lines, empty row — no paraphrase"
  - "Polling hook co-located in PipelineTab.tsx (UI-SPEC locks the new-file list to 3; useEvalReports interval shape with cadence in the effect deps)"
  - "未知 stage renders the raw BE key and counts as running (isRunning sentinel logic) — pulse badge + 2s cadence, never blank"
  - "SfToast consumed via window CustomEvent dispatch (viewport already mounted in AdminDashboard root) — never a second viewport"

patterns-established:
  - "PipelineApiError class: Error subclass carrying numeric status + raw server error string; network rejections normalize to {status:0, message:'网络错误'}"
  - "Stage-shaped test fixture (makeStatus + statusOverrides) for poll-driven component tests under fake timers"

requirements-completed: [ONT-13]

# Metrics
duration: 23min
completed: 2026-09-22
---

# Phase 39 Plan 08: ECL Pipeline Admin Tab Summary

**9th admin tab 「ECL 管线」 per the approved 39-UI-SPEC: confirm-gated Run ECL, server-truth-only progress/queue/failure chips, 6-column failures table with dead-only retry, and a 2s/10s self-re-arming polling hook — all copy verbatim, zero new deps or tokens.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-22T03:46:43Z
- **Completed:** 2026-09-22T04:09:49Z
- **Tasks:** 3 / 3
- **Files modified:** 8 (4 created, 4 modified)

## Accomplishments

- DTO mirror + API layer (`types.ts`, `pipelineApi.ts`): PipelineStatus/PipelineFailure field-for-field against the Plan 07 StatusDTO; fetchStatus/runPipeline/retryNote all carry `X-Admin-Token` from `sf_admin_token`; non-OK responses become typed `PipelineApiError {status, message}` (server `error` string verbatim; network → 网络错误)
- `PipelineTab.tsx`: control strip (Run ECL + stage badge + idempotency hint, SchemaProposalsTab box idiom), status strip (CapacityBar meter geometry with role="meter" + aria-label, mono 12/26 counter, 待加载队列/失败 chips, 当前文件 mono truncated row, aria-live="polite", idle hero 尚未运行…), failures table (locked 6 columns, dead-only 重试 with per-row in-flight disable, colSpan={6} empty row), loading/error trio with 403/503 locked inline copy
- Polling hook inside PipelineTab (useEvalReports shape): `cadence = isRunning ? 2000 : 10000` in the interval effect deps → re-arm on stage flip; poll errors never stop the loop; run/retry success triggers immediate fetchOnce
- 9th SubrouteTab (Workflow icon, label ECL 管线, testid admin-tab-pipeline) + `handleSubroute`/store union widened with `'pipeline'` + ViewBody branch; URL stays /admin
- 13-test suite under `vi.useFakeTimers`: cadence 2s→10s re-arm proven tick-by-tick; confirm-gated run (cancel = no POST) with 409 conflict toast; retry enablement exactly on dead + in-flight disable + 404 reason toast; 403/503 inline copy; poll survival across transient network failures

## Task Commits

1. **Task 1: types.ts + pipelineApi.ts** - `731fd3f` (feat)
2. **Task 3 RED: pipeline test suite + 9-tab pins** - `b0f6bb0` (test)
3. **Task 2: PipelineTab.tsx** - `dd65425` (feat)
4. **Task 3 GREEN: tab registration + store union + order-pin fix** - `0302bf2` (feat)

_Plan metadata commit follows this summary (docs). TDD note: Task 3 is the tdd task — test commit `b0f6bb0` precedes feat commit `0302bf2` (gate sequence present, see TDD Gate Compliance)._

## Files Created/Modified

- `web/src/components/admin/pipeline/types.ts` - PipelineStatus / PipelineFailure DTO mirror (status union exactly pending|dead|resolved)
- `web/src/components/admin/pipeline/pipelineApi.ts` - 3 calls + PipelineApiError (token header + typed error mapping)
- `web/src/components/admin/pipeline/PipelineTab.tsx` - the /admin/pipeline surface + co-located polling hook
- `web/src/components/admin/pipeline/PipelineTab.test.tsx` - 13 tests: copy contract, cadence, run/retry lifecycles, 403/503, error survival, 9-tab registration
- `web/src/store/app-store.ts` - adminSubroute union + setAdminSubroute widened with 'pipeline' (default stays 'batches')
- `web/src/components/admin/AdminDashboard.tsx` - 9th SubrouteTab + ViewBody branch + handleSubroute widening
- `web/src/components/admin/ontology/OntologyTab.test.tsx` - tab-count pin 8 → 9
- `web/src/__tests__/AdminFeedbackTabs.test.tsx` - tab-order pin extended with 本体 + ECL 管线

## Decisions Made

- Unknown BE stage keys render the raw string with running treatment (isRunning = stage ∉ {idle, done} drives cadence, Run button, AND pulse badge — literal acceptance-criterion reading), never blank, never a crash
- 失败 chip tints `text-status-needs-info` only when failures > 0 AND at least one dead row is present (plan's ">0 with dead entries")
- No-data poll states render 「—」 (progress/queue/failures/current file) rather than fabricating zeros — server-truth-only honesty rule
- The failures section always renders (empty row included) even before the first run, per UI-SPEC §Component 3

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] OntologyTab.test.tsx pinned the old 8-tab bar**
- **Found during:** Task 3 (test suite run)
- **Issue:** `expect(tabs.querySelectorAll('button')).toHaveLength(8)` fails once the 9th tab registers
- **Fix:** updated count pin to 9 (plan Task 3 action 4 explicitly sanctions fixing tests pinning the old union)
- **Files modified:** web/src/components/admin/ontology/OntologyTab.test.tsx
- **Verification:** full suite green
- **Committed in:** b0f6bb0

**2. [Rule 3 - Blocking] AdminFeedbackTabs.test.tsx pinned a stale 7-tab order list (pre-existing failure since Phase 38)**
- **Found during:** Task 3 (full-suite gate)
- **Issue:** `tabOrder()` expected the Phase 29 7-tab list; Phase 38's 本体 already made this test fail before this plan started (39-07 was Go-only so the FE suite gate never surfaced it); the 9th tab extends the same drift
- **Fix:** extended the expected order array with 本体 (8th) and ECL 管线 (9th)
- **Files modified:** web/src/__tests__/AdminFeedbackTabs.test.tsx
- **Verification:** full suite 628/628 green
- **Committed in:** 0302bf2

---

**Total deviations:** 2 auto-fixed (2 blocking test pins, both sanctioned by Task 3 action 4 "fix any test pinning the old tab union")
**Impact on plan:** test-only corrections to keep the whole-suite gate honest; zero production-code scope creep.

## TDD Gate Compliance

- Gate sequence present: `test(39-08)` b0f6bb0 (RED) → `feat(39-08)` 0302bf2 (GREEN). No refactor commit needed.
- RED honesty note: 12 of 13 new tests passed during RED because PipelineTab.tsx itself was Task 2's already-built implementation; the genuinely failing RED test was the 9-tab registration test (8 ≠ 9) — the Task 3 feature under test. Two RED-phase test bugs were fixed before the RED commit (stage-shaped fixture for the never-run idle state; TS CFA `never` narrowing on a closure-assigned callback replaced with an object holder).

## Issues Encountered

- `pnpm exec` / `pnpm test` could not run on this machine: the pre-existing untracked `web/pnpm-workspace.yaml` + `web/pnpm-lock.yaml` (protected "Phase 999.2" WIP, explicitly not touched) flip pnpm into workspace mode and its deps-status pre-run hook auto-invokes `pnpm install`, which dies on `[ERR_PNPM_IGNORED_BUILDS] esbuild@0.21.3`. Both earlier "exit 0" tsc attempts had silently never reached tsc (tail pipeline masked it).
- **Resolution:** ran the identical toolchain directly — `node node_modules/typescript/bin/tsc --noEmit -p tsconfig.json` (exit 0) and `node node_modules/vitest/vitest.mjs run` (628/628, exit 0) — bypassing only the broken pnpm pre-run hook; no repo file touched.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SC-4's UI half is live and regression-guarded; together with 39-07's API contract the /admin/pipeline surface is end-to-end against a running server (live run smoke is an operator gate per env-intranet-service-flakiness)
- Ready for 39-09 (final plan of Phase 39: phase verification / verify-phase-39.sh close-out)

---

## Self-Check: PASSED

- All 4 created files exist on disk; 4 modified files present with the expected content.
- Commit chain verified: 731fd3f → b0f6bb0 → dd65425 → 0302bf2 on phase-20-p10-6-admin-console (no deletions in any).
- Full verification re-run: tsc --noEmit exit 0; vitest 77 files / 628 tests passed, exit 0.
- Negative greps clean: no cancel/batch/auto-run controls, no 已删除/已丢弃/已永久失败, no dangerouslySetInnerHTML, no raw palette literals in new markup.

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
