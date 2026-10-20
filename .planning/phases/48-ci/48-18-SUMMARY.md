---
phase: 48-ci
plan: 18
subsystem: ci
tags: [record-reconciliation, root-cause-classification, run-history, disposition-ledger, github-actions]

requires:
  - phase: 48-ci
    provides: "round-3 verification evidence (four run ids/heads, the WSL exit-141 reproduction) and 48-16's materialised-list remedy"
provides:
  - "M1c (48-VALIDATION), M1(a) (48-SECURITY) and the ROADMAP Blocked-on paragraph reclassified to the workflow pipeline shape, with the reproduction and the 48-16 fix"
  - "the four-run history (39350901689 / 39352480525 / 39353593172 / 39355131431) recorded in VALIDATION, SECURITY (both M1 and Approval paragraphs) and ROADMAP"
  - "the falsified no-transport cause annotated in place in STATE.md, and the round-2 disposition ledger closed at open: 0"
affects: [48-19]

actuals:
  tokens: 7786
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "In-place correction: the falsified statement is quoted and annotated, never deleted, so the record keeps both what was believed and what the evidence shows"

key-files:
  created: []
  modified:
    - ".planning/phases/48-ci/48-VALIDATION.md"
    - ".planning/phases/48-ci/48-SECURITY.md"
    - ".planning/ROADMAP.md"
    - ".planning/STATE.md"
    - ".planning/phases/48-ci/48-REVIEW-DISPOSITION-R2.md"

key-decisions:
  - "The P5a red is recorded as a workflow pipeline shape in all three records: `go test -list | grep -qx` under `set -o pipefail` closes the pipe at the first match, the producer takes SIGPIPE, and the pipeline exits 141; the remedy recorded is the one applied by 48-16 (materialise the list to $RUNNER_TEMP before grep)"
  - "The withdrawn runner-side prescriptions (capture step output / abort early on output-buffer close / add -v) are named as withdrawn rather than silently removed, because they cannot fix a pipeline in the workflow"
  - "Every stated run count is now four, including SECURITY's independently stated Approval paragraph; run-4 (id 39355131431, head 304dcc4) is named in VALIDATION, SECURITY and ROADMAP"
  - "The falsified 'no git transport to github.com from this host' cause is retained verbatim in STATE.md as a quote and annotated as falsified by verification round 2 — the 48-11 decision entry is not deleted"
  - "CICD-01/02/03 are not flipped: record correction is not run evidence, and 48-19 alone owns the flip after a four-green run"
  - "The round-2 ledger close cites the closing plan and its proof per finding; the fork-PR self-attestation (IN-02) is recorded as documented, not removed"

requirements-completed: []

coverage:
  - id: D1
    description: "M1c/M1(a)/ROADMAP state the workflow pipeline shape with the WSL 141-vs-0 reproduction and the materialised-list fix, and the four-run history appears in all three records"
    requirement: "CICD-01"
    verification:
      - kind: unit
        ref: "grep gate RECORD-CLASS-OK (workflow pipeline shape in VALIDATION + SECURITY; 39355131431 in all three; no 'runner 端修复' / 'three times' / '3 次重跑'; SECURITY Approval says 'executed ci.yml four times on the public default branch')"
      - kind: unit
        ref: "grep gate FALSE-CAUSE-GONE (normalised SECURITY text has no 'on the runner at the test binary before any output captures'; VALIDATION has no 'the fix is runner-side')"
    human_judgment: false
  - id: D2
    description: "STATE.md no longer asserts the no-transport cause as fact and its run count matches the four real runs"
    requirement: "CICD-01"
    verification:
      - kind: unit
        ref: "grep gate STATE-RECONCILED (no falsified-cause phrase, no 'ci.yml ran three times', 39355131431 present)"
    human_judgment: false
  - id: D3
    description: "The round-2 disposition ledger is closed (nine fixed, open: 0) and tracked in git"
    requirement: "CICD-01"
    verification:
      - kind: unit
        ref: "grep gate R2-LEDGER-CLOSED (open: 0; no open row for the nine ids; git ls-files tracks the file)"
    human_judgment: false

duration: 5min
completed: 2026-10-20
status: complete
---

# Phase 48 Plan 18: Record Reconciliation and Closed R2 Ledger Summary

**Three records now name the workflow pipeline shape (not a runner environment) as the P5a cause, every stated run count is four with run-4 named — including SECURITY's Approval paragraph — the falsified transport claim is annotated rather than standing, and the round-2 disposition ledger is a tracked, closed record at `open: 0`.**

## Performance
- **Duration:** 5 min
- **Started:** 2026-10-20T05:52:11Z
- **Completed:** 2026-10-20T05:56:55Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Reclassified the go job's P5a red in all three records that carried the false cause: `go test … -list … | grep -qx …` under `set -o pipefail` lets `grep -q` close the pipe at the first match while `go test -list` still writes its trailing `ok` line, so the producer takes SIGPIPE and the pipeline exits 141. The WSL Ubuntu 22.04 / go1.25.5 reproduction (6/6 exit 141 with the exact line; exit 0 once materialised) is recorded as evidence, and the remedy recorded is 48-16's materialisation to `$RUNNER_TEMP/go-test-list.txt`.
- Recorded the four-run history — `39350901689` / `d168512`, `39352480525` / `62b6663`, `39353593172` / `a2a4c86`, `39355131431` / `304dcc4` — in VALIDATION, SECURITY (M1 paragraph and the Approval paragraph) and ROADMAP. No record claims a green run.
- Annotated the falsified "no git transport to github.com from this host" cause in the STATE.md 48-11 entry while keeping the historical decision, and corrected the 48-15 entry to four runs with the pipeline-shape M1c wording.
- Closed the round-2 ledger: all nine findings `fixed`, each Source cell naming the closing plan (48-12 / 48-13) and its proof; frontmatter `open: 0` with `total: 9`; a round-3 closure paragraph keeps the round-2 history.

## Task Commits
Each task was committed atomically:

1. **Task 1: reclassify M1c and correct the run count across VALIDATION, SECURITY and ROADMAP** - `0dcbf02` (docs)
2. **Task 2: strike the falsified no-transport cause and correct the run count in STATE.md** - `b1a6cfb` (docs)
3. **Task 3: close the round-2 disposition ledger and commit it** - `7461ad6` (docs)

**Plan metadata:** `docs(48-18): complete ...` (this file)

## Files Created/Modified
- `.planning/phases/48-ci/48-VALIDATION.md` - runner-evidence paragraph four runs + run-4; M1 row run-4; M1c rewritten to the workflow pipeline shape with the reproduction and 48-16's fix
- `.planning/phases/48-ci/48-SECURITY.md` - M1 paragraph four runs + run-4; M1 bullet (a) reclassified; Approval paragraph now states `the runner executed ci.yml four times on the public default branch` with all four ids/heads
- `.planning/ROADMAP.md` - Phase 48 `Blocked on`: four runs with ids/heads, P5a pipeline-shape classification and 48-16 fix, remedy list now `(ci.yml P5a 管道形态修复 / 测试 harness / Dockerfile 构建基础镜像)`
- `.planning/STATE.md` - 48-11 falsified-cause annotation; 48-15 four-run entry with run-4 and the pipeline-shape wording
- `.planning/phases/48-ci/48-REVIEW-DISPOSITION-R2.md` - nine `fixed` dispositions with Source cells, `open: 0`, round-3 closure paragraph

## Decisions Made
- **Correction is in-place, not deletion.** Falsified statements are quoted and annotated so the record preserves both the original belief and the evidence that overturned it (plan prohibition).
- **The remedy text follows the applied fix.** The withdrawn runner-side prescriptions are named as withdrawn for the same reason: a developer following them would not land four-green.
- **Run-4 is evidence, not a green.** The run cycle ends with three real reds; the record says so in every file.
- **No requirement moved.** CICD-01/02/03 stay Pending; `requirements.mark-complete` was deliberately not run.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- None. The `48-REVIEW-DISPOSITION-R2.md` file was already tracked at task start (the round-3 verifier's "untracked" observation no longer held), so the task's tracked-file assertion passed without a `git add -f`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 48-19 is the only plan that can flip CICD-01/02/03: push `main` once and confirm `go=fe=docker=verify=success` on the public runner.
- The phase gate was re-run after the record edits: `bash verify-phase-48.sh` exit 0 with exactly one `ALL PASSED`, so P10's ROADMAP scan still resolves the corrected SC-4 wording.
- Records and artifacts now agree on the cause and the run history; the remaining work is the three source-side fixes (ci.yml P5a already fixed by 48-16, Sparkline test path, Dockerfile.backend base).

---
*Phase: 48-ci*
*Completed: 2026-10-20*
