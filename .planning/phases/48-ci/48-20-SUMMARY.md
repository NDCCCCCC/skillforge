---
phase: 48-ci
plan: 20
subsystem: ci
tags: [github-actions, ci-gate, mutation-testing, golangci-lint, bash, verifier-hardening]

# Dependency graph
requires:
  - phase: 48-17
    provides: evidence-bound ok_partition (log exists + non-empty + contains token) and the begin/ok done_partition contract this plan extends
  - phase: 48-16
    provides: the materialised-list determinism shape P2's P2-DET-OK group certifies
provides:
  - ok_partition accepts one-or-more required tokens and reports every missing token at once
  - per-assertion-group success tokens P3-STEP-IF-OK / P2-CMDS-OK / P2-DET-OK / P2-SHUFFLE-OK / P2-HERMETIC-OK / P3-BLOCKING-OK / P3-LINT-OK / P3-VERIFYJOB-OK in the shared P2/P3 probe
  - P1 tokens P1-PARSED + P1-COMPOSITE-OK + P1-NO-PRT with # P1-B / # P1-C section markers
  - P4 per-sentinel tokens P4-A-ERRCHECK / P4-B-QUIET / P4-C-QUIET / P4-D-REVIVE concatenated missing-file-tolerantly into p4.log
  - four executable deletion drills (M13 P2, P3, P1-composite, P4-sentinel-A) each exiting 1 with the partition and missing token named
affects: [48-21, 48-22, verify-phase-48.sh, mutation-evidence rounds]

# Actuals (#2632)
actuals:
  tokens: 3962
  tasks: 2
  commits: 2
commits: 2
plan_head_before: 0daf890844a3b69553804d3c34c868d7df212ebb
plan_head_after: d58376a419c91ea3af62416395a82b78bee22f99

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "per-assertion-group evidence tokens: a partition's PASS deposit requires every one of its assertion groups to have printed its own token"
    - "missing-file-tolerant evidence concatenation (truncate, then append each log only when it exists) so a deleted probe group cannot hide the missing token at deposit time"

key-files:
  created: []
  modified:
    - verify-phase-48.sh

key-decisions:
  - "ok_partition collects ALL missing tokens and reports them in one failure message listing the full required set — the drill attribution greps (P3-BLOCKING-OK, P4-A-ERRCHECK) must appear in the output even when a different token of the same partition failed first"
  - "P3-STEP-IF-OK is emitted by the step-level if: loop that precedes the P2 section, so the P3-section deletion drill still leaves P3-STEP-IF-OK present and reds on P3-BLOCKING-OK/P3-LINT-OK — per the plan's own must_haves"
  - "failure-message prefix changed from the hard-coded 'P11 partition ...' to 'partition ...' (review IN-02 direction) so a P1/P2/P3/P4 deposit failure names the partition it actually concerns"

patterns-established:
  - "Token-per-assertion-group: one success token per assertion group inside a multi-group probe, required at deposit time (WR-03 fix shape, generalises to P6/P7/P9 in a later plan)"

requirements-completed: [CICD-01, CICD-02, CICD-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "P2/P3 token split: ok_partition token-list gate + eight per-assertion-group tokens; deleting P2's whole assertion block (M13) or P3's section reds the gate naming the partition"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh (exit 0, exactly one ALL PASSED, registry 11/11)"
        status: pass
      - kind: integration
        ref: "sandbox mutation drill: delete lines from '# --- P2 ---' to '# --- P3 ---' -> exit 1, FAIL partition P2 ... 'P2-CMDS-OK P2-DET-OK P2-SHUFFLE-OK P2-HERMETIC-OK', no ALL PASSED"
        status: pass
      - kind: integration
        ref: "sandbox mutation drill: delete lines from '# --- P3 ---' to print(\"P3-VERIFYJOB-OK\") -> exit 1, FAIL partition P3 ... P3-BLOCKING-OK, no ALL PASSED"
        status: pass
    human_judgment: false
  - id: D2
    description: "P1/P4 token split: composite-parse and prt-scan tokens with # P1-B/# P1-C markers; four per-sentinel tokens concatenated into p4.log; composite-group and sentinel-A deletions red the gate"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh (exit 0, exactly one ALL PASSED)"
        status: pass
      - kind: integration
        ref: "sandbox mutation drill: delete '# P1-B:'..'# P1-C:' -> exit 1, FAIL partition P1 ... P1-COMPOSITE-OK, no ALL PASSED"
        status: pass
      - kind: integration
        ref: "sandbox mutation drill: delete '# Sentinel A'..'Sentinel B' -> exit 1, FAIL partition P4 ... P4-A-ERRCHECK, no ALL PASSED"
        status: pass
    human_judgment: false
  - id: D3
    description: "Healthy tree unaffected: EXPECTED_PARTITIONS stays 11, the gate exits 0 with exactly one ALL PASSED and 11/11 registry agreement after every change"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh run after each task and post-commit (three runs, all exit 0, one ALL PASSED, 'registry 11/11, begin markers 11/11, success tokens 11/11')"
        status: pass
    human_judgment: false

# Metrics
duration: 13 min
completed: 2026-10-20
status: complete
---

# Phase 48 Plan 20: Per-Assertion-Group Success Tokens (P1/P2/P3/P4) Summary

**The verifier no longer folds an assertion-level neutralisation into PASS: P2/P3's shared probe now emits eight per-assertion tokens and P1/P4 three/four, each required at deposit time, with four sandbox drills proving every deletion reds the gate naming the partition (M13 closed for these four partitions).**

## Performance

- **Duration:** 13 min
- **Started:** 2026-10-20T13:21:33Z
- **Completed:** 2026-10-20T13:35:02Z
- **Tasks:** 2 of 2
- **Files modified:** 1 (`verify-phase-48.sh`; the `.planning/STATE.md` working-tree edit present at dispatch belongs to the orchestrator and was not committed here)

## Accomplishments
- Closed the failed half of 48-VERIFICATION.md truth 12 for the two sharpest partitions: P2 can no longer borrow P3's evidence and vice versa — deleting P2's entire assertion block now exits 1 with `partition P2 ... 'P2-CMDS-OK P2-DET-OK P2-SHUFFLE-OK P2-HERMETIC-OK'` instead of the reproduced `exit 0 / ALL PASSED` (mutation M13).
- `ok_partition` upgraded from a one-token gate to a token-list gate (arity `>= 3`, set -u safe, per-partition `partition <pn>` prefix), reporting every missing token in one message.
- P1's three evidence groups (per-file parse, composite parse, `pull_request_target` scan) and P4's four sentinels each emit their own token; P4's four logs concatenate into `p4.log` via a missing-file-tolerant truncate-then-append loop, so a deleted sentinel cannot hide its absence.
- No assertion was removed or weakened: every command fragment, determinism pin, hermeticity check, sentinel and exit-code assertion remains; `EXPECTED_PARTITIONS` stays 11.

## Task Commits

Each task was committed atomically:

1. **Task 1: split the shared P2/P3 token into per-assertion-group tokens** (tracer) - `55cc33f` (fix)
2. **Task 2: split P1 and P4 into per-assertion-group tokens** - `d58376a` (fix)

**Plan metadata:** pending (docs commit follows this summary)

## Files Created/Modified
- `verify-phase-48.sh` - token-list `ok_partition`; eight P2/P3 group tokens; P1 `P1-COMPOSITE-OK`/`P1-NO-PRT` + `# P1-B:`/`# P1-C:` markers; P4 `P4-A-ERRCHECK`/`P4-B-QUIET`/`P4-C-QUIET`/`P4-D-REVIVE` + tolerant `p4.log` concatenation; updated `done_partition` strings and closing partition summary lines

## Decisions Made
- `ok_partition` reports ALL missing tokens together, with the full required set quoted — the drills grep the output for a specific token (`P3-BLOCKING-OK`, `P4-A-ERRCHECK`) that may not be the first one missing.
- Kept `P3-STEP-IF-OK` attached to the go-job step-level `if:` loop exactly as the plan specifies; the P3-section drill therefore reds on that group's siblings, and the plan's must_have truth 2 is satisfied verbatim.
- Applied review IN-02's prefix fix (`partition ...` instead of `P11 partition ...`) as part of rewriting the failure messages the plan already mandated.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. The healthy gate passed on the first run after each task; all four mutation drills red on their first execution with the correct partition and token attribution.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Truth 12's failed half is closed for P1/P2/P3/P4; the remaining WR-03 residual (per-assertion tokens for the single-print probes P6/P7/P9) and the R1-ledger annotation gap (truth 15) stay with plans 48-21/48-22.
- The threat-register rows T-48-70/71/72 (48-22 Task 3 registrations) now have their mitigations implemented and drill-proven.
- CICD-01/02/03 remain Complete on run 39380373930; this plan changes no workflow file and no requirement status.

---
*Phase: 48-ci*
*Completed: 2026-10-20*

## Self-Check: PASSED

- `verify-phase-48.sh` exists and carries the token-list `ok_partition` plus all fifteen per-assertion tokens (8 P2/P3 + 3 P1 + 4 P4); zero `STRUCT-OK` occurrences remain.
- Commits verified as ancestors of HEAD: `55cc33f` (Task 1), `d58376a` (Task 2), `8e30f74` (SUMMARY), `b8e91ef` (state/roadmap tracking).
- Final gate run: exit 0, exactly one `ALL PASSED`, `registry 11/11, begin markers 11/11, success tokens 11/11`.
- `requirements.ready-ids` returned `ready: []` for CICD-01/02/03 — no duplicate or regressive marking; they remain Complete on run 39380373930.
