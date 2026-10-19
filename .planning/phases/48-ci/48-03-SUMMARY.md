---
phase: 48-ci
plan: 03
subsystem: infra
tags: [github-actions, eval-gate, fork-pr, dependabot, skip-notice, go-version, roadmap]

requires:
  - phase: 47-ci
    provides: "stable test baseline (no flaky red) that a live gate would otherwise mask"
  - phase: 48-ci
    provides: "48-01's verify-phase-48.sh P1 scans every workflow under .github/workflows/ (parse + pull_request_target ban)"
provides:
  - "eval-gate.yml job-level three-clause gate: fork PRs and Dependabot runs never reach the live eval job (D-09)"
  - "eval-gate-skip-notice job emitting a literal ::notice:: so a skipped live gate is never read as a pass (D-10)"
  - "Go version single source of truth in eval-gate.yml: setup-go@v7 + go-version-file: go.mod, zero literals in .github/workflows/ (D-17)"
  - "ROADMAP SC-4 corrected: eval-gate is an existing live workflow; only --gate-check is hermetic (D-12/D-13)"
affects: [phase-49-hermetic-demo, phase-50-open-source-front, phase-51-release-engineering]

actuals:
  tokens: 958
  tasks: 3
  commits: 3

plan_head_before: cd612d8e3c2aebc1dc78c6a0695bf06909be8dbf
plan_head_after: 0f414583a0cb7bd5db41120aef692d5474c13991

tech-stack:
  added: []
  patterns:
    - "gate the secret-bearing live job, then break the skipped-job silence with a sibling notice job"
    - "logical-complement if: expressions so the notice fires exactly where the gate does not"
    - "Go version resolved from go.mod in every workflow — a go-version literal is a hard break under setup-go v6+"

key-files:
  created: []
  modified:
    - .github/workflows/eval-gate.yml
    - .planning/ROADMAP.md

key-decisions:
  - "The fork/Dependabot gate is a job-level if:, never a trigger change — the on: block is byte-identical and pull_request_target stays absent (D-11/T-48-16)"
  - "The three secret mappings are deliberately left in place; deleting them would make the live gate silently stop using secrets rather than making fork PRs safe"
  - "The skip-notice job has no checkout and no toolchain: it only needs to make the skip legible, not to reproduce the gate"
  - "ROADMAP SC-4 keeps its operative clause (live gates stay out of CI) and the stale 666/84 test-count line is untouched — count corrections are out of scope (D-12)"

requirements-completed: [CICD-01]

coverage:
  - id: D1
    description: "eval-gate job carries the three-clause fork/Dependabot if: gate (non-PR, same-repo PR, non-Dependabot) while its trigger, permissions and three secret mappings remain unchanged"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "PyYAML + substring assertions on .github/workflows/eval-gate.yml -> EVALGATE-OK / FORK-GATE-OK"
        status: pass
      - kind: other
        ref: "bash verify-phase-48.sh -> P1 parses both workflows, zero pull_request_target, exit 0"
        status: pass
    human_judgment: true
    rationale: "The workflow TEXT is proven statically. That GitHub actually runs the gate on a real fork PR and a real Dependabot PR is VALIDATION M2/M3 and closes only on the first such run."
  - id: D2
    description: "eval-gate-skip-notice runs on the logical complement and emits ::notice:: stating the skip is not a live-gate pass"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "PyYAML assertion: inverted if: fragments + literal '::notice::' in the single run step -> EVALGATE-OK"
        status: pass
    human_judgment: true
    rationale: "Whether the notice job actually executes (and whether its check is visible) depends on GitHub's event semantics; VALIDATION M2/M3 observe it on a real fork/Dependabot PR."
  - id: D3
    description: "eval-gate resolves Go from go.mod (setup-go@v7 + go-version-file) with zero go-version: literals under .github/workflows/"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "grep -rn 'go-version:' .github/workflows/ -> no match; exact-one go-version-file assertion -> GOVERSION-OK"
        status: pass
    human_judgment: false
  - id: D4
    description: "ROADMAP SC-4 no longer claims eval-gate is a hermetic precedent; exactly one line changed and the Goal line is untouched"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "old-phrase absence + new-phrase presence + git diff HEAD -U0 marker count == 2 -> SC4-OK"
        status: pass
    human_judgment: false

duration: 2min
completed: 2026-10-19
status: complete
---

# Phase 48 Plan 03: Eval-Gate Fork Boundary and SC-4 Truthfulness Summary

**A live eval gate that fork PRs and Dependabot can no longer reach, a visible `::notice::` skip so silence is never mistaken for a pass, `go-version-file: go.mod` in place of the hard-breaking 1.23 literal, and an SC-4 sentence that now describes the workflow it actually has.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-10-19T09:07:06Z
- **Completed:** 2026-10-19T09:09:22Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- `.github/workflows/eval-gate.yml`'s `eval-gate` job now carries a three-clause job-level `if:` — non-`pull_request` events, same-repository PRs, and actors other than `dependabot[bot]`. The trigger block, `permissions: {contents: read}`, and all three secret mappings are provably unchanged, so the hardening did not widen the trigger and did not quietly drop the live inputs.
- `eval-gate-skip-notice` runs on the exact complement of that expression and prints a literal `::notice::` saying the live gate was skipped because fork and Dependabot PRs receive no secrets and that this is **not** a live-gate pass. A green check now carries an explanation instead of being indistinguishable from a real run.
- `actions/setup-go@v5` + `go-version: '1.23'` became `actions/setup-go@v7` + `go-version-file: go.mod`; `actions/checkout@v4` and `actions/upload-artifact@v4` moved to `@v7`. Zero `go-version:` literals remain anywhere under `.github/workflows/`, so the directory-wide invariant later asserted as P6 already holds.
- `.planning/ROADMAP.md` SC-4 stopped claiming `--frozen` is a hermetic precedent. It now states the measured fact: `--frozen` bootstraps BGE-M3 / Milvus / GLM before scoring and `log.Fatalf`s without secrets, so only `--gate-check` is hermetic; fork and Dependabot PRs get the explained skip. Exactly one line moved.

## Task Commits

Each task was committed atomically:

1. **Task 1: Gate `eval-gate` for fork PRs and Dependabot, make the skip visible** - `1bebc0a` (ci)
2. **Task 2: Migrate `eval-gate` off the `go-version: '1.23'` literal** - `cd9f815` (ci)
3. **Task 3: Correct ROADMAP SC-4's false hermetic claim** - `0f41458` (docs)

## Files Created/Modified

- `.github/workflows/eval-gate.yml` - job-level fork/Dependabot `if:`; `eval-gate-skip-notice` job with `::notice::`; checkout/setup-go/upload-artifact pinned to v7; `go-version-file: go.mod`
- `.planning/ROADMAP.md` - Phase 48 Success Criteria item 4 only

## Decisions Made

- **Gate the job, not the trigger.** `pull_request_target` would have let fork-controlled workflow text run with base-repo secrets; the job-level `if:` keeps the existing `on:` block byte-identical and the token read-only.
- **Keep the secret mappings.** The three `secrets.*` lines are asserted still present: removing them would leave the gate green while silently ceasing to be a live gate.
- **The notice job is checked-out-free.** It needs no repository content, so it has a single `run:` step and a 5-minute timeout.
- **Do not reuse the composite action here.** `eval-gate` builds and runs `cmd/eval` only; pulling setup-node/setup-buildx in would be dead weight. D-17's requirement is the version source, not indirection.
- **Leave the stale test counts alone.** SC-4 correction is a precision fix, not a count audit; the `666 Go 测试 + 84 FE 测试` line was asserted unchanged.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The first draft of Task 1's explanatory comment spelled the forbidden trigger keyword while explaining why it is forbidden — which would have made the file's own `pull_request_target` ban unenforceable and failed both the task verify and `verify-phase-48.sh` P1. Reworded before any verification or commit; the committed file contains zero occurrences.
- `golangci-lint` used by `verify-phase-48.sh` resolves under `/c/Users/CPIC/go/bin` on this host (48-01 noted the same); the gate passed with that path prepended.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 48-04 can now assert P3/P4/P6/P22 over the final files: both gate clauses, the `::notice::` job, the directory-wide `go-version:` ban, and the SC-4 wording are all in place.
- Still unproven locally and requiring a real GitHub Actions observation: that a fork PR shows the notice job green and the live gate skipped (M2), and that a Dependabot PR behaves identically (M3).
- The live gate remains non-hermetic by design; Phase 49 owns making `--frozen` actually hermetic.

## Self-Check: PASSED

- `.github/workflows/eval-gate.yml` and `.planning/ROADMAP.md` exist and are tracked.
- Task commits present in history: `1bebc0a`, `cd9f815`, `0f41458` (3 commits, measured from the plan ledger base `cd612d8`).
- All three task `<verify>` commands re-ran green against the final tree (`EVALGATE-OK` / `FORK-GATE-OK`, `GOVERSION-OK`, `SC4-OK`).
- `bash verify-phase-48.sh` exits 0 with `=== Phase 48 Gate: ALL PASSED ===` after every task.

---
*Phase: 48-ci*
*Completed: 2026-10-19*
