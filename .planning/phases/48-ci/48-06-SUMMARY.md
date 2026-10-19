---
phase: 48-ci
plan: 06
subsystem: infra
tags: [github-actions, ci, eval-gate, pipefail, secrets, least-privilege]

# Dependency graph
requires:
  - phase: 48-ci
    provides: ".github/workflows/eval-gate.yml from Phase 22 + verify-phase-48.sh P9 partition"
provides:
  - "Frozen-replay step now fails the workflow on a non-zero ./bin/eval exit despite the tee pipeline"
  - "Live credentials scoped to the two consuming steps (replay, gate check) instead of all job steps"
affects: [48-11, "Phase 51 release engineering"]

# Actuals (#2632) — chars/4 over the realized diff
actuals:
  tokens: 558
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "GitHub Actions: explicit `shell: bash` plus a leading `set -o pipefail` for any pipeline whose producer's exit code must survive"
    - "Least-privilege secret mounting: step-level env on exactly the consuming steps"

key-files:
  created: []
  modified:
    - .github/workflows/eval-gate.yml

key-decisions:
  - "Keep `set -o pipefail` in the run body even though `shell: bash` already implies it, so the intent survives a later removal of the shell key (WR-03 fix guidance)"
  - "Scope the three live secrets to both consuming steps (replay and gate check) per the review's fix; P9's file-wide literal check keeps the wiring assertion green"
  - "No waiver token added: no continue-on-error, no `|| true`, no `exit 0`"

patterns-established:
  - "Pipefail-faithful replay: `shell: bash` + leading `set -o pipefail` + unchanged pipeline text"
  - "Step-scoped credentials: job-level env carries configuration only; secrets live on the steps that read them"

requirements-completed: []
# Intentionally empty, following the 48-04/48-05 precedent. The plan's frontmatter
# names CICD-01 as the requirement whose gate integrity this hardens, but this
# gap-closure plan does not obtain the first real GitHub Actions run. The plan's
# objective states no CICD requirement becomes closable locally, and STATE.md still
# records CICD-01/02/03 as unproven. Marking CICD-01 complete here would falsify
# the requirement record.

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "A non-zero `./bin/eval --frozen` exit fails the workflow even though its log still reaches the artifact through `tee`"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "python YAML probe: replay step shell == bash, `set -o pipefail` precedes the tee line, pipeline text unchanged"
        status: pass
      - kind: other
        ref: "mechanism probe: `bash -c 'set -o pipefail; ( exit 3 ) | tee /dev/null'` => rc=3; same under `set -e` without pipefail => rc=0"
        status: pass
      - kind: other
        ref: "grep: zero continue-on-error / `|| true` / `exit 0` in eval-gate.yml"
        status: pass
    human_judgment: false
  - id: D2
    description: "Only the replay and gate-check steps receive the three live credentials; job-level env carries no secrets and no non-secret key was dropped"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "python YAML probe: job env has zero `secrets.` values, all eight non-secret keys retained, exactly two steps carry all three keys, all three literals remain"
        status: pass
      - kind: integration
        ref: "bash verify-phase-48.sh => ALL PASSED (P9 EVALGATE-OK, secrets intact)"
        status: pass
    human_judgment: false

duration: 3 min
completed: 2026-10-19
status: complete
plan_head_before: 32fb6fc80362121f2b0660e0ad620d3592d1cf0e
plan_head_after: 10d46643f1336d9f79aacf49cdb3be66a0d01344
---

# Phase 48 Plan 06: Eval-Gate Integrity Hardening Summary

**Frozen replay now propagates `./bin/eval`'s exit code through `tee`, and the three live credentials are mounted only on the two steps that consume them**

## Performance

- **Duration:** 3 min
- **Started:** 2026-10-19T12:39:17Z
- **Completed:** 2026-10-19T12:42:47Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Closed WR-03: the `Run frozen compliance replay` step now declares `shell: bash` and begins its `run` body with `set -o pipefail`, so a non-zero `./bin/eval --frozen compliance` exit fails the workflow instead of being masked by `tee`'s zero status. This re-closes the T-48-26 (Repudiation) half of the SC-4 "never a fake green" gap.
- Closed WR-08: `GLM_API_KEY`, `BGE_M3_URL` and `MILVUS_ADDRESS` moved out of the `eval-gate` job-level `env:` into step-level `env:` on exactly `Run frozen compliance replay` and `Gate check`. `actions/checkout@v7`, `actions/setup-go@v7` and `actions/upload-artifact@v7` no longer see live credentials. Re-closes T-48-27 (Information Disclosure).
- Preserved the zero-weakening rule (T-48-28): no `continue-on-error`, no `|| true`, no `exit 0` was introduced; the phase gate remains fully enforcing.
- The phase verifier still exits 0: `bash verify-phase-48.sh` reports `=== Phase 48 Gate: ALL PASSED ===`, with P9's secret-intact assertion green (it matches `secrets.<NAME>` anywhere in the file).

## Task Commits

Each task was committed atomically:

1. **Task 1: make the frozen-replay step's exit code survive the tee (WR-03)** - `6f4926f` (fix)
2. **Task 2: scope the three live secrets to the steps that read them (WR-08)** - `10d4664` (fix)

_Note: Task 1 was `type="tracer"`; its `<verify>` was re-run end-to-end after the commit before expansion (`PIPEFAIL-STEP-OK`, `WR03-CLOSED`)._

## Files Created/Modified

- `.github/workflows/eval-gate.yml` - replay step gains `shell: bash` + leading `set -o pipefail`; three secrets move from job-level to step-level env on the replay and gate-check steps.

## Decisions Made

- Kept `set -o pipefail` in the `run` body despite `shell: bash` already implying it — the review's fix asks for belt-and-braces so the intent survives a later `shell:` key removal.
- Scoped secrets to both consuming steps per the WR-08 fix (not just the replay step) — this matches the plan and the review's stated boundary.
- Left all non-secret config at job level (`JUDGE_MODEL`, `BGE_M3_MODEL`, `RETRIEVER_BACKEND`, `SQLITE_DSN`, `PLATFORM_FLOOR_VERSION`, `SKIP_HISTORY_PIPELINE`, `GOCACHE`, `GOMODCACHE`) so cache paths and model config remain shared.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- WR-03 and WR-08 are closed at the workflow-text level. As with the rest of Phase 48, no live GitHub Actions run can be obtained from this host (git transport to github.com is unreachable), so CICD-01 remains pending on the Class A evidence owned by 48-11.
- Ready for the next gap-closure plan.

---
*Phase: 48-ci*
*Completed: 2026-10-19*

## Self-Check: PASSED

- `FOUND: .github/workflows/eval-gate.yml`
- `FOUND: .planning/phases/48-ci/48-06-SUMMARY.md`
- `FOUND: 6f4926f` (Task 1 commit is an ancestor of HEAD)
- `FOUND: 10d4664` (Task 2 commit is an ancestor of HEAD)
- `ACCEPTANCE-RECHECK-OK ['Run frozen compliance replay', 'Gate check']`
- `bash verify-phase-48.sh` => `=== Phase 48 Gate: ALL PASSED ===` (exit 0)
