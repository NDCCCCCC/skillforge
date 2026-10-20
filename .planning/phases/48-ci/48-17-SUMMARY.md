---
phase: 48-ci
plan: 17
subsystem: ci
tags: [verifier-honesty, mutation-testing, github-actions, evidence-binding, sigpipe, materialised-list]

requires:
  - phase: 48-ci
    provides: "the 11-partition verify-phase-48.sh, the four-job ci.yml, and 48-16's materialised-list determinism shape with its P2 pin landed in the same commit"
provides:
  - "evidence-bound success tokens for all 11 partitions (ok_partition requires a probe-written non-empty log containing a probe-emitted token)"
  - "job-level if: rejection on the fe and docker jobs (D10/D11 closed)"
  - "the 48-16 materialised-list determinism pin confirmed exact in P2, with a drill proving the piped shape reds P2"
affects: [48-18, 48-19]

actuals:
  tokens: 5505
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Evidence-bound success token: deposit only after the probe's own log exists, is non-empty, and contains a token the probe emitted"
    - "Assert grep's no-match exit code (1) explicitly, so a `:` substitution (status 0) fails"
    - "Materialise -q probe output on the asserted-success branch so a silent presence probe still has evidence"

key-files:
  created: []
  modified:
    - "verify-phase-48.sh"

key-decisions:
  - "ok_partition now takes (partition, probe-log, probe-emitted-token) and refuses a missing/empty/token-less log before writing the .ok marker — the P8 unbound-variable backstop generalised to all 11 partitions (truth 6 / D1-D3)"
  - "P1's per-file parse probe emits P1-PARSED and the deposit requires one line per enumerated workflow; the pull_request_target scan asserts grep's no-match status of exactly 1, so a `:` substitution fails"
  - "P4's sentinel B/C evidence is the linter's own '0 issues' line, and the partition deposits on sentinel D's positive revive attribution (the load-bearing probe)"
  - "P10 keeps its literal -q presence probes for the mutation surface, then materialises the replacement-phrase match as evidence on the asserted-success branch (p10-new non-empty, p10-old legitimately empty)"
  - "P11 counts the 8 fixture rejections into p11-fixtures.log and deposits on that log with the token 'rejected', still before half 3's registry count"
  - "The fe/docker job-level if: checks live in the existing per-job Python probes (P6/P7), matching P3's treatment of go/verify"
  - "CICD-01/02/03 deliberately stay Pending: hardening the verifier does not substitute for the four-green real run owned by 48-19, so requirements.mark-complete was not run"

requirements-completed: []

coverage:
  - id: D1
    description: "Every partition's PASS requires evidence produced by its own probe; deleting/neutering P9, the shared P2/P3 probe, P6, or the P10 greps reds the gate with no ALL PASSED banner"
    requirement: "CICD-01"
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh => exit 0, exactly one ALL PASSED"
      - kind: unit
        ref: "four sandbox drills (P9 probe deleted, P2/P3 probe deleted, P6 probe deleted, P10 greps -> :) each exit non-zero with no banner and the partition named"
    human_judgment: false
  - id: D2
    description: "A job-level if: on the fe or docker job is rejected, naming the partition and the expression"
    requirement: "CICD-02"
    verification:
      - kind: unit
        ref: "YAML round-trip adding if: to both jobs => P6 red; docker-only mutation => P7 red with the expression printed"
      - kind: integration
        ref: "bash verify-phase-48.sh on the unmutated ci.yml => exit 0"
    human_judgment: false
  - id: D3
    description: "P2 pins the 48-16 materialised-list determinism shape and rejects a pipe directly after -list; no set -o pipefail requirement remains"
    requirement: "CICD-01"
    verification:
      - kind: unit
        ref: "sandbox re-adding the piped -list command => P2 red with the SIGPIPE/exit-141 message"
      - kind: integration
        ref: "bash verify-phase-48.sh on the post-48-16 ci.yml => exit 0"
    human_judgment: false

duration: 11min
completed: 2026-10-20
status: complete
---

# Phase 48 Plan 17: Evidence-Bound Success Tokens and the Closed If Arm (Truth 6) Summary

**No partition can print PASS unless the probe it certifies actually ran: all eleven deposits now require a probe-written, non-empty log containing a token the probe itself emitted, and the fe/docker lines reject job-level `if:` — the D1-D3 and D10/D11 mutation classes are closed while the healthy tree stays green.**

## Performance
- **Duration:** 11 min
- **Started:** 2026-10-20T05:29:34Z
- **Completed:** 2026-10-20T05:40:47Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments
- Turned `ok_partition` from a one-argument bookkeeping line into an evidence gate: a missing log, an empty log, or a log without the probe-emitted token each fail with the partition named and no banner.
- Bound every partition to its own output: P1 requires one `P1-PARSED` line per enumerated workflow and asserts grep's no-match status explicitly; P2/P3 share `STRUCT-OK`; P4 requires non-empty sentinel B/C logs reporting `0 issues` and deposits on `revive`; P5-P9 deposit their own probe tokens; P10 materialises the replacement-phrase match; P11 counts the eight fixture rejections and deposits `rejected`.
- Closed D10/D11: the `fe` and `docker` job mappings now reject a job-level `if:` with the offending expression printed, matching P3's go/verify treatment.
- Confirmed the 48-16 determinism pin is present and exact in P2 — materialised list file, `grep -qx` on the file, explicit rejection of a pipe after `-list`, and no `set -o pipefail` requirement.
- Re-ran all six sandbox mutations: each reds the gate with the expected partition named and zero green banners; the unmutated gate exits 0 with exactly one `ALL PASSED`.

## Task Commits
Each task was committed atomically:
1. **Task 1: bind every success token to probe-owned evidence** - `ccb4284` (fix)
2. **Task 2: reject job-level if: on the fe and docker jobs** - `8c02d00` (fix)
3. **Task 3: name and pin the materialised-list determinism shape in P2** - `534c8f2` (fix)

**Plan metadata:** committed immediately after this file (`docs(48-17): complete ...`).

## Files Created/Modified
- `verify-phase-48.sh` - `ok_partition(partition, probe-log, probe-token)` with existence/non-empty/fixed-string-token checks; all 11 deposits updated; P1 `P1-PARSED` counting and explicit `pull_request_target` exit-code assertion; P4 sentinel B/C `0 issues` evidence; P10 evidence materialisation; P11 fixture counter/log; P6/P7 job-level `if:` rejection; P2 materialised-list descriptions. `EXPECTED_PARTITIONS` remains 11.

## Decisions Made
- **Evidence, not bookkeeping.** The token is deposited only from a log the probe wrote; this is the same mechanism P8 already used (the unbound `TRUST_SURFACE_COUNT`), generalised rather than replaced.
- **Grep status is asserted, not inferred.** P1 requires exactly status 1 (no match) so `:` (status 0) fails; the scan must have produced a decisive exit code.
- **P4's load-bearing deposit is sentinel D.** Sentinels B/C are supporting evidence (non-empty + `0 issues`); the positive revive attribution is what proves the linter is configured, not merely quiet.
- **P10 separates presence probes from evidence.** The `-q` probes remain literal for the mutation surface; the non-`-q` re-run materialises the match on the asserted-success branch only.
- **The 48-16 pin was confirmed, not rewritten.** No P2 assertion was loosened or re-added; only the description strings changed.
- **Requirements were not flipped.** Only a real four-green run closes CICD-01/02/03; that is 48-19's evidence, and `requirements.mark-complete` was deliberately not run.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- The plan's Task 1 P6-deletion drill (`sed -i '/p6\.log"/,/^PY$/d'`) matches both the P6 probe's `python ... > "$WORK/p6.log"` line and the later `cat "$WORK/p6.log"` line. The second range therefore also deletes the P7 probe. The drill's stated outcome still holds — exit non-zero, no `ALL PASSED`, and the attribution grep (`P6|P7`) matches — and the deposit failure names P7. No source change was needed; the mutation only affects a temp copy.
- The plan's Task 1 P9-drill and Task 2/3 drills were run verbatim from the plan and behaved as specified.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 48-18 can reconcile the records (M1c classification, run counts) without touching the verifier's new evidence contract.
- 48-19 is the only plan that can close CICD-01/02/03: push `main` once and confirm `go=fe=docker=verify=success` on the public runner.
- The verifier's PASS tokens are now honest for all eleven partitions; no partition's green can survive a deleted or neutered probe.

---
*Phase: 48-ci*
*Completed: 2026-10-20*

## Self-Check: PASSED

- `.planning/phases/48-ci/48-17-SUMMARY.md` exists.
- Task commits `ccb4284`, `8c02d00`, `534c8f2` are ancestors of HEAD (`534c8f2`).
- Cumulative diff vs plan head `b0563ba` touches only `verify-phase-48.sh` (138 insertions, 42 deletions); no deletions of tracked files.
- Final `bash verify-phase-48.sh` exits 0 with exactly one `ALL PASSED`.
