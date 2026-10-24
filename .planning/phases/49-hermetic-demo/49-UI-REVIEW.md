---
phase: 49-hermetic-demo
reviewed: 2026-10-22
scope: single-badge addition
ui_spec: none (the phase's ui-plan-gate reports `frontend: false`)
pillar_score: none issued
status: reviewed
---

# Phase 49 — UI Review

## Scope of the frontend surface in this phase

Phase 49 touches the frontend in exactly one place (`49-02`, commit `bbd559f`):

| File | Change |
|---|---|
| `web/src/types/verdict.ts` | `VERDICT_MODES = ['stub']` + `VerdictMode` + `Verdict.mode?: VerdictMode` + the `isStubVerdict` type guard |
| `web/src/types/verdict.test.ts` | 6 cases: the mode enum, the wire back-compat (`real-model verdicts have no mode field`), the guard, the unchanged `isVerdict`, garbage rejection, the untouched compliance enum |
| `web/src/components/center/VerdictCard.tsx` | a `stub` badge (`data-testid="stub-mode-badge"`) on the compact stripe and on the detail card, rendered only when `isStubVerdict(verdict)` |
| `web/src/__tests__/VerdictCard.test.tsx` | the badge appears for a stub verdict and does not appear for a real one |

No other component, route, style token or layout was touched. The phase's `ui-plan-gate` reports `frontend: false`
(there is no UI design surface in the phase goal), so no `49-UI-SPEC.md` exists and none was required.

## Assessment

**A 6-pillar visual audit is not proportionate to this change**, and issuing a pillar score for a 3-line badge
would be noise. What matters is that the one user-visible behaviour is honest and covered:

- **The badge appears only for stub verdicts.** `isStubVerdict` narrows on `mode === 'stub'`; a real model's verdict
  has no `mode` key at all (the Go side is `json:"mode,omitempty"`), so the badge is structurally absent — the
  "stub 模式如实标注" requirement (D-08) cannot be bypassed by a real model, and the absence of the badge is itself
  the signal that the answer came from a model.
- **Both card surfaces carry it** (the compact stripe and the detail card), so the label is visible whichever
  surface the user is looking at.
- **The label text is `stub`, lowercase, in the existing badge vocabulary** — it reuses the card's own badge styling
  (`rounded-full border … uppercase tracking-wide`) rather than introducing a new visual language. The inline stripe
  uses `opacity-70` so it does not compete with the status word, the detail card uses `text-text-muted`.
- **Accessibility**: the badge is textual, not an icon, and the compact stripe's `ShieldCheck` icon keeps its
  `aria-hidden`. The badge is not a status indicator, so it needs no live-region semantics.

**Coverage**: `web/src/types/verdict.test.ts` (6 cases) and `web/src/__tests__/VerdictCard.test.tsx` (2 cases) — the
whole frontend suite is 86 files / 697 tests green.

## Residual risk

- The badge's visual weight (10px, 70% opacity) is a judgement call that no assertion can make; a human should look
  at it once in the running app during phase 50's README work, where the app is already being opened for the demo
  asset. Recorded as a manual item rather than a pillar score.

## Conclusion

The frontend delta is small, backward compatible, and mechanically covered. No pillar score is issued and no UI-SPEC
was required. One visual judgement is deferred to phase 50, where the app is opened anyway.
