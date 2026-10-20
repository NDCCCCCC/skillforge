---
phase: 48-ci
plan: 21
subsystem: ci
tags: [github-actions, ci-gate, mutation-testing, verifier-hardening, bash, python, sigpipe]

# Dependency graph
requires:
  - phase: 48-20
    provides: the token-list ok_partition (log exists + non-empty + contains EVERY token) and the per-assertion-group token pattern this plan extends to P6/P7/P9
  - phase: 48-17
    provides: evidence-bound ok_partition contract and the P11 half-1 banner-site checks this plan hardens
provides:
  - four P6 per-assertion-group tokens (P6-JOB-IF-OK / P6-STEP-IF-OK / P6-CMDS-OK / P6-COUNTS-OK) with # P6-A:..# P6-D: section markers
  - six P7 per-assertion-group tokens (P7-JOB-IF-OK / P7-STEP-IF-OK / P7-BUILDS-OK / P7-PUSH-OK / P7-NO-LOAD-OK / P7-CACHE-OK) with # P7-A:..# P7-F: section markers
  - six P9 per-assertion-group tokens (P9-JOBS-OK / P9-GATE-EXPR-OK / P9-NOTICE-OK / P9-WIRING-OK / P9-TRIGGERS-OK / P9-GOVERSION-OK)
  - the P11 else-detector consumes the printed gate-to-banner range via BANNER_RANGE before a non-quiet grep reads it to EOF
  - three executable deletion drills (P6 job-if, P7 cache-scope, P9 notice-group) plus the banner-in-else drill, each exiting 1 with the partition named
affects: [48-22, verify-phase-48.sh, mutation-evidence rounds]

# Actuals (#2632)
actuals:
  tokens: 3254
  tasks: 3
  commits: 3
commits: 3
plan_head_before: ea8ab09fa76635e10c5ee9126b22c0248b2aada7
plan_head_after: 5b6203aa0874b61c62f9def6579d374888d91c59

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "consumed-range detector: materialise a pipeline's input into a variable before a non-quiet grep reads it to end-of-input, so a pipefail SIGPIPE can never masquerade as a negative match"

key-files:
  created: []
  modified:
    - verify-phase-48.sh

key-decisions:
  - "P9's six group tokens replace the combined EVALGATE-OK/GOVERSION-OK print outright — the probe's informational job/file counts were print-only decoration and no assertion consumed them, so the per-group tokens carry the evidence alone"
  - "Section markers (# P6-A: .. # P6-D:, # P7-A: .. # P7-F:) sit at the START of each assertion group including its rationale comment, so a drill deleting a marker removes the group's comment, assertions and token as one unit"

patterns-established:
  - "Consumed-range pattern for pipefail-sensitive detectors: capture sed's range into a variable, printf it, grep WITHOUT -q (stdout to /dev/null) — the consumer reads to EOF and cannot EPIPE its producer"

requirements-completed: [CICD-01, CICD-02, CICD-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "P6/P7 token split: four P6 and six P7 per-assertion-group tokens, each printed at the end of its own group and required at deposit; deleting the P6 job-level if: group or the P7 cache-scope group reds the gate naming the partition"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh (exit 0, exactly one ALL PASSED, registry 11/11)"
        status: pass
      - kind: integration
        ref: "token grep: all ten P6/P7 tokens present, zero FE-LINE-OK / DOCKER-LINE-OK occurrences"
        status: pass
      - kind: integration
        ref: "sandbox drill: delete '# P6-A:'..'# P6-B:' -> exit 1, no ALL PASSED, output names partition P6 and P6-JOB-IF-OK"
        status: pass
      - kind: integration
        ref: "sandbox drill: delete '# P7-F:'..print(\"P7-CACHE-OK\") -> exit 1, no ALL PASSED, output names partition P7 and P7-CACHE-OK"
        status: pass
    human_judgment: false
  - id: D2
    description: "P9 token split: six per-assertion-group tokens replace the combined EVALGATE-OK/GOVERSION-OK print; deleting the notice group reds the gate naming P9 and P9-NOTICE-OK"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh (exit 0, exactly one ALL PASSED)"
        status: pass
      - kind: integration
        ref: "token grep: all six P9 tokens present, zero EVALGATE-OK occurrences"
        status: pass
      - kind: integration
        ref: "sandbox drill: delete '# --- the visible skip notice'..'# --- the live wiring' -> exit 1, no ALL PASSED, output names partition P9 and P9-NOTICE-OK"
        status: pass
    human_judgment: false
  - id: D3
    description: "P11 else-detector consumed-range refactor: the gate-to-banner range is captured into BANNER_RANGE before a non-quiet grep reads it to EOF; the banner-in-else mutation still reds P11"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "shape grep (run from a file to preserve quoting): BANNER_RANGE=\"$(sed -n present, printf '%s\\n' \"$BANNER_RANGE\" | grep -E present, old sed|grep -qE pipeline absent"
        status: pass
      - kind: integration
        ref: "sandbox drill: move the ALL PASSED banner after the RAN gate's else -> exit 1, 'FAIL P11 the passing banner sits after an else'"
        status: pass
      - kind: integration
        ref: "bash verify-phase-48.sh post-commit (exit 0, one ALL PASSED, registry 11/11, begin markers 11/11, success tokens 11/11)"
        status: pass
    human_judgment: false

# Metrics
duration: 16 min
completed: 2026-10-20
status: complete
---

# Phase 48 Plan 21: Per-Assertion-Group Tokens (P6/P7/P9) + P11 Consumed-Range Detector Summary

**Every multi-assertion probe in the verifier now binds its PASS to per-assertion-group evidence — P6 emits four tokens, P7 and P9 six each, all required at deposit time — and the P11 else-detector materialises its printed range before grepping, removing the last SIGPIPE false negative (WR-03 closed phase-wide; WR-01 closed).**

## Performance

- **Duration:** 16 min
- **Started:** 2026-10-20T13:53:10Z
- **Completed:** 2026-10-20T14:09:37Z
- **Tasks:** 3 of 3
- **Files modified:** 1 (`verify-phase-48.sh`; cumulative diff vs plan base: 46 insertions, 17 deletions)

## Accomplishments

- Closed the WR-03 residual for the three remaining single-print probes: P6's job-level `if:`, step-level `if:`, command/working-directory and count-literal groups each emit their own token (`P6-JOB-IF-OK` / `P6-STEP-IF-OK` / `P6-CMDS-OK` / `P6-COUNTS-OK`); P7's six groups emit `P7-JOB-IF-OK` through `P7-CACHE-OK`; P9's job-keys, gate-expression, notice, wiring, trigger and Go-version groups emit `P9-JOBS-OK` through `P9-GOVERSION-OK`. Deleting any one group now leaves that group's token missing at deposit time, and `ok_partition` names the partition and the missing token.
- Removed the last three per-probe shared tokens: `FE-LINE-OK`, `DOCKER-LINE-OK` and the combined `EVALGATE-OK ... GOVERSION-OK` print are gone from the script entirely.
- Closed verification advisory 1 / review WR-01: the P11 else-detector captures the gate-to-banner range into `BANNER_RANGE` and feeds a non-quiet grep (stdout to /dev/null) that reads to end-of-input, so under `set -o pipefail` a grown range can no longer EPIPE the producer into a silent "no else found".
- Proved each change by mutation: the P6 job-if, P7 cache-scope and P9 notice-group sandbox drills each exit 1 with no `ALL PASSED`, naming the partition and the missing token; the banner-in-else drill exits 1 with `FAIL P11 the passing banner sits after an else`.
- No assertion was removed or weakened: every check and error message is unchanged; the diff only adds group markers, token prints, the two deposit-token lists, and the P11 two-step detector. `EXPECTED_PARTITIONS` stays 11 and the healthy gate stays green (exit 0, exactly one `ALL PASSED`, registry 11/11 after every task and post-commit).

## Task Commits

Each task was committed atomically:

1. **Task 1: split P6 and P7 into per-assertion-group tokens** - `2606813` (fix)
2. **Task 2: split P9 into per-assertion-group tokens** - `97c021b` (fix)
3. **Task 3: make the P11 else-detector consume its input before grepping** - `5b6203a` (fix)

**Plan metadata:** pending (docs commit follows this summary)

## Files Created/Modified

- `verify-phase-48.sh` - four P6 / six P7 / six P9 per-assertion-group tokens with `# P6-A:`..`# P6-D:` and `# P7-A:`..`# P7-F:` section markers; token-list deposits for all three partitions; the P11 `BANNER_RANGE` consumed-range else-detector; updated `done_partition` strings and closing partition summary lines

## Decisions Made

- The P9 combined print's informational counts (`%d jobs`, `%d file(s)`) were dropped with the token: no assertion or downstream probe consumed them, and keeping them would have required re-attaching them to one arbitrary group's token.
- Section markers open each group including its rationale comment block, so the deletion drills remove a group's comment, assertions and token as one unit and the surviving probe stays syntactically valid Python.

## Deviations from Plan

None - plan executed exactly as written.

(One execution-environment note, not a deviation: Task 3 verify 2's `grep 'printf .%s\\n. ...'` was run from a script file because the interactive shell layer collapses `\\n`; the file run reproduces the plan's command verbatim and it passes against the committed source.)

## Issues Encountered

None. The healthy gate passed on the first run after each task, and all four mutation drills red on their first execution with the correct partition and token/placement attribution.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- WR-03 is now closed for every partition the review named (P1, P2, P3, P4 by 48-20; P6, P7, P9 here). P5, P8, P10 and P11 are single-assertion or already per-group, so no shared-token residual remains.
- WR-01 (verification advisory 1) is closed; the remaining open review items (WR-02 parsed-document trust-surface rules, IN-01 install env) belong to 48-22's scope.
- Threat-register rows T-48-73/74/75 (registered by 48-22 Task 3) now have their mitigations implemented and drill-proven.
- CICD-01/02/03 remain Complete on run 39380373930; this plan changes no workflow file and no requirement status.

---
*Phase: 48-ci*
*Completed: 2026-10-20*

## Self-Check: PASSED

- `verify-phase-48.sh` exists and carries all sixteen new per-assertion tokens (4 P6 + 6 P7 + 6 P9); zero `FE-LINE-OK`, `DOCKER-LINE-OK` or `EVALGATE-OK` occurrences remain; `BANNER_RANGE` is in the P11 half-1 detector.
- Commits verified as ancestors of HEAD: `2606813` (Task 1), `97c021b` (Task 2), `5b6203a` (Task 3), `27db69a` (SUMMARY), `f7a5938` (state/roadmap tracking).
- Final gate run post-commit: exit 0, exactly one `ALL PASSED`, `registry 11/11, begin markers 11/11, success tokens 11/11`.
- All four mutation drills (P6 job-if, P7 cache-scope, P9 notice-group, P11 banner-in-else) exited 1 with no `ALL PASSED`, naming the partition and the missing token / else placement.
- `requirements.ready-ids` returned `ready: []` for CICD-01/02/03 — already Complete on run 39380373930; no marking performed (expected no-op).
