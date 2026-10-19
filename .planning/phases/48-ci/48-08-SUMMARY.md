---
phase: 48-ci
plan: 08
subsystem: infra
tags: [github-actions, eval-gate, verifier, mutation-testing, normalized-expression, complement-condition]

requires:
  - phase: 48-ci
    plan: 06
    provides: "pipefail-faithful replay step and step-scoped live secrets"
  - phase: 48-ci
    plan: 07
    provides: "step-condition rejection and the trap-protected mutation-probe pattern"

provides:
  - "P9 rejects a fork gate that is textually present but logically neutralised (WR-07, T-48-02, T-48-18)"
  - "eval-gate-skip-notice.if is the verbatim negation of eval-gate.if, so a Dependabot-attributed push is announced too (WR-02, T-48-15)"
  - "P9 pins the relationship notice == \"!(\" + gate + \")\" rather than judging two independent texts"
  - "P9's printed evidence names what it computed: exact gate expression, complement notice present (T-48-35)"

affects: [48-09, 48-10, 48-11, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 1647
  tasks: 2
  commits: 2

plan_head_before: ed2d118e3bc40482595e1d107c921d9f341513d3
plan_head_after: 36e6618422b3fa68611564d39cdb318ec607ee68

tech-stack:
  added: []
  patterns:
    - "Exact normalized equality on a folded YAML scalar closes the neutralisation class that substring checks cannot see: || true, an outer negation and a removed clause all change the normalized string"
    - "A diagnostic job's condition is asserted as the exact logical complement of the condition it announces, not as an independent expression"
    - "Mutation probes run under a trap and restore the workflow byte-identically before the baseline re-run"

key-files:
  created: []
  modified:
    - verify-phase-48.sh
    - .github/workflows/eval-gate.yml

key-decisions:
  - "P9 compares the whitespace-normalized eval-gate if: to the exact D-09 literal. No substring fallback is kept: equality subsumes the old per-clause diagnostics, and a fallback could pass where equality fails."
  - "The skip notice is the verbatim negation of the gate, asserted structurally as notice == \"!(\" + gate + \")\". Reverting to the narrower PR-only notice, removing the negation, or appending || true all fail P9."
  - "The printed evidence line was updated to `exact gate expression, complement notice present` so the evidence matches what is computed (T-48-35)."
  - "requirements-completed stays [] following the 48-04/48-05/48-06/48-07 precedent. This gap-closure plan re-closes verifier holes; it does not obtain the first real GitHub Actions run, which remains the Class A evidence owned by 48-11."

requirements-completed: []

coverage:
  - id: D1
    description: "A fork gate that is textually present but logically neutralised fails P9"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#P9 exact gate expression, baseline rc=0"
        status: pass
      - kind: integration
        ref: "mutation: gate + `|| true` => rc=1 and message contains `want exactly`"
        status: pass
      - kind: integration
        ref: "mutation: gate wrapped in outer negation => rc=1 and message contains `want exactly`"
        status: pass
      - kind: integration
        ref: "mutation: Dependabot clause removed => rc=1 and message contains `want exactly`"
        status: pass
    human_judgment: false
  - id: D2
    description: "The skip notice is the exact logical complement of the gate, including a Dependabot-attributed push"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "python YAML probe: normalized notice == \"!(\" + normalized gate + \")\" => NOTICE-COMPLEMENT-OK"
        status: pass
      - kind: integration
        ref: "mutation: old PR-only notice restored => rc=1 and P9 message names `complement`"
        status: pass
      - kind: integration
        ref: "mutation: notice + `|| true` => rc=1"
        status: pass
      - kind: integration
        ref: "mutation: bare un-negated gate as notice => rc=1 and message names `complement`"
        status: pass
      - kind: integration
        ref: "python YAML probe: ::notice:: step intact after every probe => NOTICE-STEP-INTACT"
        status: pass
    human_judgment: false

duration: 7 min
completed: 2026-10-19
status: complete
---

# Phase 48 Plan 08: Eval-Gate Expression Pinning and Complement Notice Summary

**P9 now compares the eval-gate `if:` to the exact normalized D-09 literal, and the skip notice is the verbatim negation of that gate, so neither a neutralising `|| true` nor a Dependabot-attributed push can hide a skipped live gate.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-10-19T13:26:47Z
- **Completed:** 2026-10-19T13:33:51Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Closed WR-07 / T-48-02 / T-48-18: P9 no longer accepts three independent substrings. It normalizes the parsed `eval-gate.if:` scalar (`" ".join(value.split())`) and compares it to the exact D-09 literal. Appending a top-level `|| true`, wrapping the expression in an outer negation, or removing any one clause now turns P9 red and prints both the actual and expected expressions. Re-closes the fork/secret boundary on evidence that survives the reviewer's neutralisation mutations.
- Closed WR-02 / T-48-15: `eval-gate-skip-notice.if` is now the verbatim negation of the gate expression. The omitted region — a Dependabot-attributed push to `main` — is now announced with `::notice::` instead of showing an unexplained green. The job key, name, `runs-on`, `timeout-minutes` and the notice step are unchanged.
- P9 pins the relationship, not two independent texts: it asserts `notice == "!(" + gate + ")"` after the same normalization. The old PR-only notice, a bare un-negated gate, and a notice with `|| true` appended all fail.
- Updated P9's printed evidence from `3-clause gate` to `exact gate expression, complement notice present`, so the partition's output states what it actually computed (T-48-35).
- The phase gate remains green: `bash verify-phase-48.sh` exits 0 with 11/11 partitions and exactly one `=== Phase 48 Gate: ALL PASSED ===` banner.

## Task Commits

Each task was committed atomically:

1. **Task 1: P9 asserts the normalized gate expression, not three substrings (WR-07)** — `ace4229` (fix)
2. **Task 2: make the skip notice the true complement and assert the relationship (WR-02)** — `36e6618` (fix)

_Note: Task 1 was `type="tracer"`; under `human_verify_mode: end-of-phase` with an automated-only `<verify>`, it was re-run end-to-end after the commit before expansion (`P9-GATE-EXPRESSION-OK`: baseline green plus three mutation probes caught)._

## Files Created/Modified

- `verify-phase-48.sh` — P9's per-clause substring loop replaced by normalized exact equality; notice substring checks replaced by complement equality; evidence and partition labels updated.
- `.github/workflows/eval-gate.yml` — `eval-gate-skip-notice.if` replaced with the verbatim negation of the gate; comment records the WR-02 rationale.

## Verification

| Probe | Result |
|-------|--------|
| `bash verify-phase-48.sh` on the unmutated tree | rc=0, 11/11 partitions, one `ALL PASSED` |
| Gate + `|| true` | rc=1, message contains `want exactly` |
| Gate wrapped in an outer negation | rc=1, message contains `want exactly` |
| Dependabot clause removed from the gate | rc=1, message contains `want exactly` |
| Normalized `notice == "!(" + gate + ")"` | `NOTICE-COMPLEMENT-OK` |
| Old PR-only notice restored | rc=1, P9 message names `complement` |
| Notice + `|| true` | rc=1 |
| Bare un-negated gate used as the notice | rc=1, P9 message names `complement` |
| `::notice::` step after every probe | `NOTICE-STEP-INTACT` |
| `eval-gate.yml` after every mutation probe | restored byte-identical (`git status` clean before the task commit) |
| Secrets, trigger block, and single Go version source | still asserted and green in the same P9 run |

## Decisions Made

- P9 uses exact equality with no substring fallback. A fallback is the exact hole WR-07 described: it can pass where equality fails. The clause diagnostics were not lost — the expected literal still prints in full.
- The notice is judged as a relationship (`"!(" + gate + ")"`), not as a second approved text. Two independent literals can drift; a derived literal cannot.
- `CICD-01` is not marked complete. As with 48-04 through 48-07, this plan hardens the verifier; the requirement still requires one real GitHub Actions run, owned by 48-11.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Threat Flags

None. The change narrows the verifier's accepted set (one exact expression instead of three substrings) and widens only a diagnostic job's condition. No new network endpoint, auth path, secret reference, or trust boundary is introduced.

## Next Phase Readiness

- WR-07 and WR-02 are closed at the workflow-text level with mutation-proven evidence. T-48-02, T-48-15 and T-48-18 can cite these probes in the security register re-closure.
- As with the rest of Phase 48, no live GitHub Actions run can be obtained from this host, so CICD-01 remains pending on the Class A evidence owned by 48-11.
- Ready for 48-09 and the remaining gap-closure plans.

---

*Phase: 48-ci*
*Completed: 2026-10-19*

## Self-Check: PASSED

- `FOUND: verify-phase-48.sh`
- `FOUND: .github/workflows/eval-gate.yml`
- `FOUND: .planning/phases/48-ci/48-08-SUMMARY.md`
- `FOUND: ace4229` (Task 1 commit is an ancestor of HEAD)
- `FOUND: 36e6618` (Task 2 commit is an ancestor of HEAD)
- `bash verify-phase-48.sh` => `=== Phase 48 Gate: ALL PASSED ===` (exit 0, 11/11 partitions, one banner)
