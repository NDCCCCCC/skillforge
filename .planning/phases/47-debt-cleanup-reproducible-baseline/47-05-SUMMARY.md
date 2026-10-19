---
phase: 47-debt-cleanup-reproducible-baseline
plan: 05
subsystem: testing (frontend)
tags: [vitest, react-testing-library, knowledge-graph, sc-5, d-46-def-01, retarget]

requires:
  - phase: 46-entity-shacl-shapes
    provides: "KnowledgeGraphBrowser mode chips (quick-260929-g7x) + the D-46-DEF-01 deferred record of this deterministic FE failure"
provides:
  - "SC-5 empty-corpus assertion retargeted at the mode that renders the hero (文档) — same test id, same fetch stub"
  - "Green frontend suite: 685/685 across 84 files, 0 failed / 0 skipped (was 684/1)"
  - "D-46-DEF-01 closure evidence for 47-06's disposition master table and 47-08's FE gate segment"
affects: [phase-48-ci-gates, 47-06-debt-disposition, 47-08-verification, phase-49-hermetic-suite]

tech-stack:
  added: []
  patterns:
    - "Retarget, don't weaken: when a component's default mode drifts, a behavioral test must name the mode it depends on instead of relying on an implicit default"
    - "Mutation-prove a test assertion before declaring green (flip the render guard, watch the test fail, revert)"

key-files:
  created: []
  modified:
    - web/src/__tests__/DetailPanel.test.tsx

key-decisions:
  - "Fix the assertion, not the implementation — D-46-DEF-01 is the same class as D-04b: the assertion drifted, the implementation is right"
  - "Use the user-visible trigger (the 文档 mode chip) rather than any internal knob, keeping the SC-5 evidence user-reachable (T-47-17)"
  - "Add a `toHaveTextContent('知识库为空')` pin on top of the retarget, so the assertion is strictly stronger than before, not merely restored"
  - "Do NOT write to 47-DEBT-DISPOSITION.md from this plan — that file is created by 47-06 in wave 3 and carries the D-46-DEF-01 row verbatim; the escalation channel is this SUMMARY"
  - "Did NOT touch web/src/components/admin/governance/* (plan 47-04, wave 2) or any other production file"

requirements-completed: [REL-04]

# Metrics
duration: 8min
completed: 2026-10-19
---

# Phase 47 Plan 05: SC-5 empty-state assertion retarget (D-46-DEF-01) Summary

**The SC-5 empty-corpus test now selects 文档 mode before asserting the hero, so the frontend suite is a clean 685/685 with no production component touched.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-10-19T04:05:00Z
- **Completed:** 2026-10-19T04:07:18Z
- **Tasks:** 1
- **Files modified:** 1

## Root cause (measured, not inferred)

`web/src/components/graph/KnowledgeGraphBrowser.tsx:75` defaults to
`useState<GraphViewMode>('ontology')` since quick-260929-g7x, and the
document-corpus states are all gated on that mode:

```tsx
if (mode === 'document' && (!graph || totalDocs === 0)) { ... data-testid="knowledge-graph-empty" ... 知识库为空 }
```

The SC-5 test asserted `knowledge-graph-empty` without ever selecting a mode,
so it was observing `ontology-graph-empty` / 本体暂无数据. Baseline run
(measured before the edit) reproduced this deterministically:

```
× DetailPanel > knowledge-graph kind mounts the empty-state hero when BE returns an empty corpus (SC-5)
  → Unable to find an element by: [data-testid="knowledge-graph-empty"]
  Tests  1 failed | 12 passed (13)
```

The implementation is correct — the assertion had drifted. Same class as D-04b.

## What changed

`web/src/__tests__/DetailPanel.test.tsx` — the SC-5 test only:

1. `fireEvent.click(screen.getByTestId('graph-mode-document'))` between the render
   and the `waitFor` that asserts `knowledge-graph-empty`. `fireEvent` was already
   imported; no new import, no new dependency.
2. An additional `expect(screen.getByTestId('knowledge-graph-empty')).toHaveTextContent('知识库为空')`
   — mirrors `KnowledgeGraphBrowser.test.tsx`'s own 文档-mode empty-state case, so the
   assertion pins both the test id and the copy.
3. Root-cause comment recording that the default flipped to 本体 mode and that the
   hero is a document-mode concern; the pre-existing leading comment no longer
   implies default-mode behavior.

Net diff: **1 file, +15 / -0**. No deletions, no rewrites of the stub payload.

## Evidence

### Full frontend suite

| | Test files | Tests passed | Failed | Skipped |
|---|---|---|---|---|
| Before this plan | 84 | 684 | 1 | 0 |
| After this plan | 84 | **685** | **0** | **0** |

`cd web && npx vitest run` → `Test Files 84 passed (84)` / `Tests 685 passed (685)`, 0 failed, 0 skipped, ~19s.
`cd web && npx tsc --noEmit` → exit 0.

### No production component was modified

`git diff --stat -- web/src/components/` → empty. The only tracked modification in the
working tree after the plan is `web/src/__tests__/DetailPanel.test.tsx` (+15/-0):

```
 web/src/__tests__/DetailPanel.test.tsx | 15 +++++++++++++++
 1 file changed, 15 insertions(+)
```

### The SC-5 assertion still proves the hero renders (mutation check)

To rule out the "retargeted into vacuity" failure mode, the production guard was
mutated locally, the suite run, and the file reverted:

```diff
- if (mode === 'document' && (!graph || totalDocs === 0)) {
+ if (mode === 'document' && false && (!graph || totalDocs === 0)) {
```

With the hero suppressed the retargeted test fails immediately:

```
× DetailPanel > knowledge-graph kind mounts the empty-state hero when BE returns an empty corpus (SC-5)
  → Unable to find an element by: [data-testid="knowledge-graph-empty"]
  Tests  1 failed | 12 passed (13)
```

The mutation was reverted immediately; `git status --porcelain -- web/src/components/`
is empty afterwards. The test is not a smoke check — a hero that stops rendering
still fails it.

### No skip / no waiver

`grep -n "it.skip\|it.todo\|describe.skip" web/src/__tests__/DetailPanel.test.tsx` → no matches.
No `it.todo`, no `xit`, no waiver entry, consistent with D-04f.

## Deviations from plan

None. The plan's contingency (escalate if the assertion still fails after the mode click)
did not trigger — the retarget was sufficient, so no `## Escalation — D-46-DEF-01` section exists.

## What plan 47-06 and 47-08 can cite

- **D-46-DEF-01 → closed.** FE-side, deterministic, fixed by retargeting the assertion
  (`5f9459a`). Evidence: 685/685 green, `tsc --noEmit` exit 0, mutation-proven assertion.
- **BL-04 (DetailPanel/emitter deferred tests).** This plan closes the DetailPanel half;
  the emitter half (`TestEmitSubCategoryInstanceCount` / `TestEmitSkillInstanceCount`) is
  plan 47-03's scope, not this one's.
- **Phase 48 CI / 47-08 FE gate.** The FE line can gate on `npx vitest run` + `npx tsc --noEmit`
  with an expected baseline of **84 files / 685 tests / 0 failed / 0 skipped**.
  If a future run reports 684 passed, the SC-5 regression has returned.

## Threat register outcome

| ID | Status | Note |
|---|---|---|
| T-47-17 Repudiation | mitigated | Same test id + same user-visible trigger (mode chip); mutation check proves a non-rendering hero fails |
| T-47-18 Tampering (prod bent to test) | mitigated | `git diff --stat web/src/components/` empty; the only production edit made was reverted and left uncommitted |
| T-47-19 Spoofing (green hides defect) | mitigated | No skip/todo/waiver; assertion strengthened with a copy pin |
| T-47-SC package installs | accepted | No installs; existing devDependencies only |

## Self-Check: PASSED

- [x] `web/src/__tests__/DetailPanel.test.tsx` contains the `graph-mode-document` click between render and the `knowledge-graph-empty` assertion
- [x] `npx vitest run src/__tests__/DetailPanel.test.tsx` → 13 passed / 0 failed
- [x] Full suite 685/685, 0 failed, 0 skipped
- [x] `npx tsc --noEmit` exit 0
- [x] `git diff --stat -- web/src/components/` empty; only the test file committed
- [x] `47-DEBT-DISPOSITION.md` NOT written (created by 47-06 in wave 3)
- [x] `.planning/STATE.md` and `.planning/ROADMAP.md` NOT modified (orchestrator owns)

---

*Plan 47-05 — commit `5f9459a`*
