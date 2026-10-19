---
phase: 48-ci
plan: 01
subsystem: infra
tags: [github-actions, golangci-lint, ci, go, hermetic, composite-action]

requires:
  - phase: 47-ci
    provides: "verify-phase-47.sh zero-waiver gate + the P1/P5a/P5b command strings the CI go job must replicate (D-20)"
  - phase: 47-ci
    provides: "measured baselines — go test ./... exit 0 (47 pkgs), shuffle seed only visible under -v"
provides:
  - ".github/actions/setup/action.yml — checkout-free composite toolchain action (D-22/D-30), the single pin site Phase 51 reuses"
  - ".golangci.yml — v2-schema lint config, no baseline, no permanent exemption list (D-01/D-02/D-03/D-08)"
  - ".github/workflows/ci.yml — single-job workflow encoding the whole CICD-01 contract"
  - "verify-phase-48.sh — zero-waiver four-partition self-proof; partitions P5–P11 are appended by later plans"
affects: [phase-48-ci, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 6952
  tasks: 4
  commits: 4

plan_head_before: 300fc5b
plan_head_after: c13f463ad1a2303033200d0c5e0a9551b4e4289e

tech-stack:
  added: []
  patterns:
    - "checkout-first composite toolchain action — the local action cannot bootstrap its own checkout"
    - "changed-lines lint with the pull-requests: read permission its API call needs"
    - "zero-waiver verifier: every partition statically decidable, RAN counter gating the banner"

key-files:
  created:
    - .github/actions/setup/action.yml
    - .golangci.yml
    - .github/workflows/ci.yml
    - verify-phase-48.sh
  modified: []

key-decisions:
  - "The go job's permissions map is compared for EXACT equality in P3, not for containment — an extra scope is as much a scope change as a missing one (D-29)"
  - "P2 and P3 are decided by one Python probe so the two partitions cannot disagree about what ci.yml says"
  - "The 'continue-on-error' token is assembled at runtime in verify-phase-48.sh so the repo-wide grep over the script stays meaningful"

requirements-completed: [CICD-01]

coverage:
  - id: D1
    description: "Composite toolchain action .github/actions/setup with three pinned majors and no checkout"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "python -c \"...yaml.safe_load(.github/actions/setup/action.yml)...\" -> COMPOSITE-OK"
        status: pass
      - kind: other
        ref: "grep -c 'actions/checkout' .github/actions/setup/action.yml -> 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "golangci-lint v2 config that verifies on the pinned v2.12.2 binary, with no baseline and no named exemption list"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "golangci-lint config verify --config .golangci.yml -> exit 0"
        status: pass
      - kind: other
        ref: "verify-phase-48.sh P4 partition (config verify + sentinels A/B/C)"
        status: pass
    human_judgment: false
  - id: D3
    description: "ci.yml go job: hermetic build+vet, blocking changed-lines lint, literal go test ./..., determinism pair with a recoverable seed"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "verify-phase-48.sh P2/P3 partitions (STRUCT-OK)"
        status: pass
      - kind: other
        ref: "python PyYAML assertions on ci.yml -> GOJOB-OK / GO-LINE-OK"
        status: pass
    human_judgment: true
    rationale: "The static checks prove the workflow SAYS the right thing. That the go job is green on ubuntu-latest — CGO build, the lint action's own diff fetch, and the full ./... suite — can only be established by a real CI run (VALIDATION M1)."
  - id: D4
    description: "verify-phase-48.sh: zero-waiver four-partition harness that refuses to fold an unrun check into a pass"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "bash verify-phase-48.sh -> exit 0, '=== Phase 48 Gate: ALL PASSED ==='"
        status: pass
      - kind: other
        ref: "PATH-surgery probe -> 'LINT NOT VERIFIED' + rc=1, VERIFIER-HONEST-OK"
        status: pass
      - kind: other
        ref: "empty-workflows-dir probe -> 'FAIL ... must not pass', rc=1"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-10-19
status: complete
---

# Phase 48 Plan 01: CI Spine Summary

**A checkout-first composite toolchain action, a v2-schema `.golangci.yml`, a one-job `ci.yml` encoding CICD-01 end to end, and a zero-waiver `verify-phase-48.sh` that proves the linter would actually fire.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-10-19T08:23:47Z
- **Completed:** 2026-10-19T08:36:50Z
- **Tasks:** 4
- **Files modified:** 4 (all new)

## Accomplishments

- `.github/actions/setup/action.yml` carries the three toolchain majors in the one place Phase 51's release workflow will reuse, and contains zero occurrences of a checkout action — a repo-local composite resolves from `github.workspace`, so it cannot bootstrap its own.
- `.golangci.yml` verifies clean against the pinned v2.12.2 binary with no baseline file, no named exemption list, and no generated-code exclusion.
- `.github/workflows/ci.yml` is one `go` job: hermetic build+vet, blocking changed-lines lint holding the `pull-requests: read` scope its own diff fetch needs, the literal `go test ./...`, and the determinism pair with `-v` so the shuffle seed reaches the log.
- `verify-phase-48.sh` proves the gate rather than describing it. P4 runs three sentinel modules: an unchecked `json.Unmarshal` is reported as errcheck (the gate has teeth), while a bare deferred `Close` and an ignored function parameter stay quiet (the two exclusions absorb rather than merely suppress).

## Task Commits

1. **Task 1: Composite toolchain action** - `8fe0115` (chore)
2. **Task 2: `.golangci.yml` v2 config** - `1820e52` (chore)
3. **Task 3: `ci.yml` — the `go` job** - `66b78a4` (ci)
4. **Task 4: `verify-phase-48.sh`** - `c13f463` (chore)

## Files Created/Modified

- `.github/actions/setup/action.yml` - composite toolchain action; setup-go@v7 (version from go.mod), setup-node@v7, setup-buildx-action@v4
- `.golangci.yml` - golangci-lint v2 config: standard + revive, std-error-handling preset, `_test.go` errcheck exclusion, unused-parameter disabled
- `.github/workflows/ci.yml` - the `go` job, single workflow file for the whole Phase 48 matrix
- `verify-phase-48.sh` - partitions P1 workflow-parse, P2 go-line, P3 lint-line, P4 lint-teeth

## Decisions Made

- **P3 compares the permissions map for exact equality, not containment.** An unrequested extra scope is as much a scope change as a missing one; `in` would have accepted both a missing `pull-requests` and a `write` nobody asked for.
- **P2 and P3 share one Python probe.** Two probes reading the same file could disagree about it, and a disagreement between two "the workflow is fine" checks is indistinguishable from the workflow being fine.
- **The forbidden CI keyword is assembled at runtime inside the verifier.** Spelling `continue-on-error` in the checker's own source makes the repo-wide grep unable to distinguish the workflow's text from the checker's text. Same reasoning for keeping `ALL PASSED` at exactly one occurrence and `LINT NOT VERIFIED` at one.
- **The composite action is parsed but not required to carry a trigger block.** An action is only ever reached through a caller workflow, so requiring `on:` of it would be requiring a fiction.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Python could not open MSYS-form paths**
- **Found during:** Task 4 (P1 first run)
- **Issue:** Under Git Bash `$ROOT` is the MSYS spelling (`/d/code/...`), which the native Windows Python build cannot open — P1 died with `FileNotFoundError` on a file that plainly existed.
- **Fix:** Added a `native()` helper that converts via `cygpath` when present and is the identity on a POSIX host, so the same source runs unchanged on a Linux runner. Every Python probe now takes the converted path.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** P1 passes; `cygpath` absent on Linux degrades to identity.
- **Committed in:** `c13f463`

**2. [Rule 1 - Bug] `MSYS_NO_PATHCONV=1` blinded the native linter to every path**
- **Found during:** Task 4 (P4 config verify)
- **Issue:** The script inherits `MSYS_NO_PATHCONV=1` from Phase 47 so WSL paths are never mangled — and that same setting stops MSYS from translating `/d/code/...` into `D:\code\...` for a NATIVE binary. `golangci-lint` is a native Win32 build, so `config verify` exited 3 against a directory that did not exist, on a config that is in fact valid.
- **Fix:** `lint_run` converts each path-shaped argument explicitly instead of relying on the runtime. Without this the linter could have run against nothing and reported success.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** P4 config verify exit 0; all three sentinels fire/stay quiet as specified.
- **Committed in:** `c13f463`

**3. [Rule 1 - Bug] Sentinel logs were written relative, landing in the repo root**
- **Found during:** Task 4 (first P4 run)
- **Issue:** `lint_run` took a bare log filename and built the path from the caller's cwd, so `p4-sentinel-a.log` and `p4-config.log` were deposited next to the sources being judged — two untracked files in the repository root. A log that appears outside the temp dir is also a signal that a relative path leaked past it.
- **Fix:** `lint_run` prefixes `$WORK/` itself; all four logs are now absolute under the temp dir. The two stray files were removed.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** `git status` shows no `p4-*` artefacts after a full run.
- **Committed in:** `c13f463`

**4. [Rule 2 - Missing Critical] The empty-log rule misfired on `config verify`, which is silent on success**
- **Found during:** Task 4 (P4)
- **Issue:** WR-06 requires an empty log to fail, but `golangci-lint config verify` prints nothing when it ACCEPTS a config — the same shape as `tsc --noEmit`, which Phase 47 P6 already documents as silent-on-success. Applying the rule blindly fails the gate forever on a valid config, which is how a guard gets deleted instead of fixed.
- **Fix:** `config verify` is judged on its exit code (still authoritative) and an explicit evidence line is appended recording the code and whether tool output was present, so the log is never empty and an auditor can see what was judged. The empty-log rule still applies unmodified to all three sentinel runs, where silence would otherwise mean "zero findings".
- **Files modified:** `verify-phase-48.sh`
- **Verification:** P4 green; a deliberately broken config still fails the partition.
- **Committed in:** `c13f463`

**5. [Rule 1 - Bug] Header comments in `ci.yml` contained the literals the file must not contain**
- **Found during:** Task 3 (acceptance gate)
- **Issue:** The invariant block explaining "no `paths:` filters, no `secrets.*`" spelled both tokens — plus `write` — in comments, so the assertions and the file it judges could not be told apart. A grep for a forbidden token cannot distinguish the workflow from the prose about the workflow.
- **Fix:** Reworded the comments to name the invariants without reproducing the tokens; the same discipline then applied to `action.yml` (a checkout reference in its header) and `.golangci.yml` (an exemption-list reference).
- **Files modified:** `.github/workflows/ci.yml`, `.github/actions/setup/action.yml`, `.golangci.yml`
- **Verification:** `GOJOB-OK` / `GO-LINE-OK` / `COMPOSITE-OK` / `CONFIG-OK` all green with the negatives at zero.
- **Committed in:** `8fe0115`, `1820e52`, `66b78a4`

---

**Total deviations:** 5 auto-fixed (4 bugs, 1 missing critical behaviour)
**Impact on plan:** All five were correctness fixes inside the plan's own deliverables; none changed scope. Deviation 4 in particular hardened WR-06 rather than relaxing it — the rule now applies where silence is ambiguous and does not misfire where silence is a documented pass signal.

## Issues Encountered

- The first P4 run failed three ways in sequence (path form, PATH-conversion blindness, relative log paths) before the sentinels ran at all. Each was a distinct environment-mismatch bug rather than a logic error, and each is now fixed at its source rather than worked around at the call site.
- `golangci-lint` resolves to `/c/Users/CPIC/go/bin` on this host, not the `/c/CPIC/go/bin` recorded in the dispatch contract. Both are the same tool and the same 2.12.2 binary; the PATH export in the contract worked unchanged.
- The repository has no `.git/hooks` beyond stock samples and no `core.hooksPath`, so hooks are inert here — noted so a future executor does not read the absence as a skipped guard.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plans 48-02 (FE + Docker jobs) and 48-03 (eval-gate fork gate, ROADMAP SC-4 wording, directory sweep) build directly on this spine; `verify-phase-48.sh` already parses every workflow under `.github/workflows/` and rejects `pull_request_target`, so their additions are covered by P1 the moment they land.
- P4's three sentinels and the `LINT NOT VERIFIED` single print site are load-bearing for later plans — 48-02 Task 3 and 48-04 Task 2 both count `ALL PASSED` with `grep -c … = 1`, and this file keeps that at exactly one.
- Still unproven locally and needing a real CI run: that `go` is green on `ubuntu-latest`, and that `only-new-issues` actually narrows the lint scope on a PR touching one Go file (VALIDATION M1 / M4).

## Self-Check: PASSED

- All four task files exist and are tracked: `.github/actions/setup/action.yml`, `.golangci.yml`, `.github/workflows/ci.yml`, `verify-phase-48.sh`.
- All four task commits present in history: `8fe0115`, `1820e52`, `66b78a4`, `c13f463`.
- `bash verify-phase-48.sh` exits 0 with the ALL PASSED banner and four `PASS P…` lines.
- The linter-neutralized probe prints `LINT NOT VERIFIED` and exits 1; the empty-workflows-dir probe prints `FAIL` and exits 1.
- `grep -c 'ALL PASSED' verify-phase-48.sh` = 1; `grep -c 'continue-on-error' verify-phase-48.sh` = 0.

---
*Phase: 48-ci*
*Completed: 2026-10-19*
