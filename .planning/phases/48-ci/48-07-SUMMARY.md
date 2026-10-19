---
phase: 48-ci
plan: 07
subsystem: infra
tags: [github-actions, verifier, mutation-testing, step-conditions, hermeticity, credentials, regex-anchoring]

requires:
  - phase: 48-ci
    plan: 05
    provides: "per-file P2/P3, P6 and P7 partitions over the shared workflow scan, plus the mutation-probe pattern this plan extends"
provides:
  - "P2/P3 reject any step-level `if:` on the go job, naming the step and condition (WR-01, T-48-07)"
  - "P6 and P7 reject any step-level `if:` on the fe and docker jobs"
  - "P2 judges the union of job-level and every step-level env map against {CGO_ENABLED} (IN-05, T-48-30)"
  - "P6 anchors the count rule to test-count phrasing in both orders and drops the loose `84 ` token (IN-04, T-48-32)"
  - "the go job's checkout sets persist-credentials: false, asserted by P3 (IN-10, T-48-31)"
affects: [phase-48-ci, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 1084
  tasks: 3
  commits: 3

plan_head_before: 72b72b0bc6f2e88f06ebf817d0a9d1317da3fae3
plan_head_after: 2100035442f6aa93a0e1b0f5df5beb7f7055c687

tech-stack:
  added: []
  patterns:
    - "step-level conditions are judged by key presence, never by value: false, always(), success() and a negated expression all change whether a gate runs"
    - "hermeticity is a union over job-level and step-level env maps, not a single map lookup"
    - "prose-based rules are constrained by the gap between the count word and the number, not by the digits, so two-digit suite counts stay catchable while ports/years and the file's own step names stay clear"
    - "mutation probes run under a trap and restore the workflow byte-identically"

key-files:
  created: []
  modified:
    - verify-phase-48.sh
    - .github/workflows/ci.yml

key-decisions:
  - "Reject a step-level `if:` by key presence with no allowlist. `if: false` is the obvious mutation, but `always()` and `success()` also decide whether the gate runs; an allowlist would reopen the same hole under a different expression."
  - "The count rule constrains the gap between the count word and the number (`[ \\t]*[:=]?[ \\t]*`) rather than widening to `[^\\n]{0,40}`. The loose form matches `go test ./... (CICD-01 / D-15)` on the untouched file and would red the gate's own baseline."
  - "persist-credentials: false is asserted on the go checkout only. The fe and docker jobs also run fork-authored content but carry no extra scope, so the plan's scope stays on the one job with a non-read-only permission map."
  - "CICD-01/02/03 stay unmarked. This plan re-closes verifier holes; it does not obtain the first real GitHub Actions run, which remains the Class A evidence owned by 48-11."

requirements-completed: []

coverage:
  - id: D1
    description: "go/fe/docker gate steps cannot be disabled by a step-level `if:`"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#P2-P3-STEP-IF-OK and #P6-P7-STEP-IF-OK (four injected false-valued conditions)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Hermeticity covers the union of job-level and step-level env"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#IN-04-05-10-CLOSED (PROBE_EXTRA_VAR injection; ENV-UNION-OK)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Count rule matches test-count phrasing only; ports and years do not match"
    requirement: CICD-02
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#COUNT-ANCHOR-OK and count-literal mutation"
        status: pass
    human_judgment: false
  - id: D4
    description: "go job checkout leaves no persisted runner token, and P3 catches its removal"
    requirement: CICD-03
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#CHECKOUT-CREDENTIALS-OK and P3-PERSIST-CREDENTIALS-OK"
        status: pass
    human_judgment: false

duration: 13 min
completed: 2026-10-19
status: complete
---

# Phase 48 Plan 07: Step-level gate skips, env union, count anchoring, credential-free checkout Summary

**Every gate step in all three CI lines now rejects a step-level `if:`, hermeticity is judged across the job+step env union, the count rule matches only test-count phrasing, and the go job's checkout no longer persists the runner token.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-10-19T13:01:19Z
- **Completed:** 2026-10-19T13:14:35Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- The shared P2/P3 Python probe rejects any `if` key on any `go`-job step. The message names the step (`name`, falling back to `uses`/`run`) and quotes the condition. `if: false` and a negated expression are equally violations; no allowlist exists because the job needs no conditional step.
- P6 and P7 carry the identical rule for the `fe` and `docker` jobs. The same walk is added after each job's `steps` list, so a false-valued condition on `npx vitest run` or either image build turns the gate red with `P6`/`P7` and `step-level if:` in the message.
- P2's hermeticity claim now computes `keys = set(job env) | set(every step env)` and requires exactly `{CGO_ENABLED}`. An injected `PROBE_EXTRA_VAR` on the `go test` step fails P2 and names the key.
- P6's `("666", "689", "84 ")` token loop is replaced by an anchored, case-insensitive regex covering both count orders (`689 tests`, `Tests  84 passed`) plus `\b666\b` / `\b689\b`. The unmutated file has zero matches; `8084` and `1984` do not match; an injected `689 tests` comment does.
- `.github/workflows/ci.yml` adds `with: {persist-credentials: false}` to the `go` job's `actions/checkout@v7`. P3 asserts the parsed value is exactly `False` and fails with an `IN-10` message when the key is removed.
- The final gate is green with all 11 partitions and a single `ALL PASSED` banner.

## Task Commits

Each task was committed atomically:

1. **Task 1: P2/P3 reject a step-level `if:` on the go job** — `d465f1f` (fix)
2. **Task 2: the same rule for fe and docker (P6/P7)** — `070f5b8` (fix)
3. **Task 3: env union, anchored count literal, credential-free checkout** — `2100035` (fix)

**Plan metadata:** committed separately (docs: complete plan)

## Files Created/Modified

- `verify-phase-48.sh` — P2/P3 step-`if` rejection and env union; P3 `persist-credentials` assertion; P6 step-`if` rejection and anchored count regex; P7 step-`if` rejection.
- `.github/workflows/ci.yml` — `go` job checkout gains `persist-credentials: false` with an explanatory comment.

## Verification

| Probe | Result |
|-------|--------|
| `bash verify-phase-48.sh` on the unmutated tree | rc=0, 11/11 partitions, `ALL PASSED` |
| `if: false` injected on `go test ./...` step | rc=1, message contains `step-level if:` |
| `if: false` injected on `build + vet` step | rc=1, message contains `step-level if:` |
| `if: false` injected on `unit tests` (fe) step | rc=1, message contains `step-level if:` |
| `if: false` injected on `build backend image` step | rc=1, message contains `step-level if:` |
| `PROBE_EXTRA_VAR` injected on the `go test` step env | rc=1, names `PROBE_EXTRA_VAR` |
| `# probe: echo "689 tests" ...` injected in ci.yml | rc=1, message contains `count` |
| anchored regex on `echo 689 tests` / `Tests  84 passed (84)` | matched |
| anchored regex on `echo port 8084 and 1984 cases` | no match |
| anchored regex on unmutated `ci.yml` | no match |
| go checkout parses as `persist-credentials: false` | `CHECKOUT-CREDENTIALS-OK` |
| `persist-credentials` block removed | rc=1, message contains `IN-10` |
| `ci.yml` after every mutation probe | byte-identical to the pre-probe copy |
| `git status --porcelain` for both files after probes | empty |

## Decisions Made

- Rejecting step conditions by key presence is the only stable form: `if: false`, `always()`, `success()` and a negated expression all change execution, so value-matching would reopen WR-01 under a different spelling.
- The count regex constrains the gap, not the digit count. `[^\\n]{0,40}` matches the file's own `go test ./... (CICD-01 / D-15)` step name and would red the baseline; `\d{3,}` would miss the two-digit Vitest counts the rule exists to catch.
- `persist-credentials: false` is applied to the `go` checkout only, matching IN-10's scope (the one job carrying `pull-requests: read`). `fe` and `docker` remain on the default.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The first baseline run after the initial edit failed in P6 with `NameError: name 're' is not defined`; the P6 probe's import line needed `re` in addition to `sys, yaml`. Fixed before the first commit; the added `re` is part of Task 3's commit.

## Requirements Note

`requirements-completed: []` intentionally, matching the 48-04/48-05 precedent. The plan frontmatter lists CICD-01/02/03 as the phase requirements it serves, but this gap-closure plan hardens the verifier; it does not obtain the first real GitHub Actions run. Those requirements remain blocked on Class A evidence owned by 48-11.

## Threat Flags

None. The change narrows the verifier's assertion surface and removes a persisted credential; no new network endpoint, auth path, or trust boundary is introduced.

## Next Phase Readiness

- Ready for 48-08 and the remaining gap-closure plans.
- T-48-29, T-48-30, T-48-31 and T-48-32 now have mutation-proven evidence to cite in the security register re-closure.
- CICD-01/02/03 still require one real CI run; no local claim can substitute for it.

## Self-Check: PASSED

- `verify-phase-48.sh` and `.github/workflows/ci.yml` exist and are the only files modified by this plan.
- Task commits `d465f1f`, `070f5b8`, `2100035` are ancestors of HEAD.
- Baseline gate exits 0 with 11/11 partitions and exactly one `ALL PASSED`.
- All mutation probes restored the workflow byte-identically; no fixture leaked.
