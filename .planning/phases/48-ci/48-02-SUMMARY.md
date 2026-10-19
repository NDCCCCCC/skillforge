---
phase: 48-ci
plan: 02
subsystem: infra
tags: [github-actions, ci, frontend, docker, buildx, gha-cache, trust-surface]

requires:
  - phase: 48-ci
    provides: "48-01's ci.yml go job, checkout-first composite action, and verify-phase-48.sh P1-P4 registry (RAN/EXPECTED_PARTITIONS)"
  - phase: 48-ci
    provides: "48-03's eval-gate.yml, now part of the directory-wide P8 trust-surface scan"
provides:
  - "ci.yml `fe` job: npm ci -> npx tsc --noEmit -> npx vitest run, all under web/ (CICD-02)"
  - "ci.yml `docker` job: both Dockerfiles build with push: false and per-image type=gha cache scopes (CICD-03)"
  - "verify-phase-48.sh P5 composite / P6 fe-line / P7 docker-line / P8 trust-surface; EXPECTED_PARTITIONS=8"
affects: [phase-48-ci, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 3806
  tasks: 3
  commits: 3

plan_head_before: 3bae85af11c0ecc734d91b4dee94bc19418bd984
plan_head_after: 6d50abb9d5411d41b7d6c8c84a913619383a1b06

tech-stack:
  added: []
  patterns:
    - "single workflow, three always-on jobs; a skipped job is never used as a gate"
    - "checkout-first in every job, toolchain pins only in the composite"
    - "per-image GHA cache scopes so the second build cannot evict the first"
    - "directory-wide trust-surface partition: permissions, registry, path filters, secrets"

key-files:
  created: []
  modified:
    - .github/workflows/ci.yml
    - verify-phase-48.sh

key-decisions:
  - "The fe and docker jobs declare no job-level permissions and inherit the workflow-level contents: read ceiling — the only mutable-scope-adjacent grant in ci.yml remains the go job's pull-requests: read (D-29)."
  - "The docker job runs two independent docker/build-push-action@v7 steps with push: false; there is no tag, login, load, or registry step anywhere, so CI has no publication capability at all (D-24/D-26)."
  - "The forbidden compose-build literal is kept out of ci.yml even in prose, so a raw-text ban in the gate can distinguish the workflow's text from commentary about it."
  - "P5-P8 use the same fail-fast, one-Python-probe-per-partition shape as P2/P3: a partition's evidence and its judgment cannot drift apart."
  - "Docker is statically asserted only on this host; CICD-03 closes on the first real CI run (VALIDATION M1), never on a local build claim."

requirements-completed: [CICD-02, CICD-03]

coverage:
  - id: D1
    description: "ci.yml fe job: lockfile install, typecheck, full Vitest run, all with working-directory: web"
    requirement: CICD-02
    verification:
      - kind: other
        ref: "PyYAML assertions on ci.yml -> FE-OK / FE-LINE-OK"
        status: pass
      - kind: other
        ref: "verify-phase-48.sh P6 fe-line -> FE-LINE-OK"
        status: pass
    human_judgment: true
    rationale: "The workflow TEXT is proven statically and the same commands pass locally. That the fe job is green on ubuntu-latest (clean npm ci, runner Node 22) is only observable on a real CI run (VALIDATION M1)."
  - id: D2
    description: "ci.yml docker job: both Dockerfiles build with push: false, distinct backend/frontend cache scopes, no publish surface"
    requirement: CICD-03
    verification:
      - kind: other
        ref: "PyYAML assertions on ci.yml -> DOCKER-OK / DOCKER-LINE-OK"
        status: pass
      - kind: other
        ref: "verify-phase-48.sh P7 docker-line -> DOCKER-LINE-OK"
        status: pass
    human_judgment: true
    rationale: "Docker is absent from both Git Bash and WSL on this host, so no local build proof exists. The structural contract is asserted; both images actually building is VALIDATION M1."
  - id: D3
    description: "verify-phase-48.sh partitions P5-P8 with EXPECTED_PARTITIONS=8 and a single banner print site"
    requirement: CICD-03
    verification:
      - kind: other
        ref: "bash verify-phase-48.sh -> exit 0, eight partition PASS lines, '=== Phase 48 Gate: ALL PASSED ==='"
        status: pass
      - kind: other
        ref: "negative probes: push: true -> P7 fail; paths: -> P8 fail; secrets.CUSTOM_TOKEN -> P8 fail; missing composite -> non-zero naming the file"
        status: pass
      - kind: other
        ref: "grep -c 'ALL PASSED' verify-phase-48.sh -> 1; EXPECTED_PARTITIONS=8 present"
        status: pass
    human_judgment: false
  - id: D4
    description: "Local reproduction of the frontend gate commands the fe job runs"
    requirement: CICD-02
    verification:
      - kind: other
        ref: "cd web && npx tsc --noEmit -> exit 0"
        status: pass
      - kind: other
        ref: "cd web && npx vitest run -> exit 0, 85 test files / 689 tests passed"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-10-19
status: complete
---

# Phase 48 Plan 02: FE and Docker CI Lines Summary

**A frontend gate that installs from the lockfile, typechecks and runs the full Vitest suite; a Docker gate that builds both images with `push: false` and per-image GHA cache scopes; and four new phase-gate partitions that make the composite order, the FE line, the Docker line and the directory-wide trust surface statically decidable.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-10-19T09:23:21Z
- **Completed:** 2026-10-19T09:28:37Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- `.github/workflows/ci.yml` now carries exactly three jobs — `go`, `fe`, `docker`. The `fe` job checks out first, calls the composite with `go: "false"`, then runs `npm ci`, `npx tsc --noEmit` and `npx vitest run`, each pinned to `working-directory: web`. It declares no job-level permissions, no `if:` and no `continue-on-error`.
- The `docker` job builds `Dockerfile.backend` and `Dockerfile.frontend` from the repository root through `docker/build-push-action@v7`, with `push: false` on both and `scope=backend` / `scope=frontend` on both sides of the cache configuration. There is no login step, no tag key, no `load`, and no registry reference — the workflow has no publication path at all.
- `verify-phase-48.sh` grew from four partitions to eight. P5 proves composite parse, one occurrence of each pinned major, zero checkout in the composite, and checkout-first ordering in every job. P6 proves the three exact FE commands and the absence of a hard-coded test count. P7 proves the two build steps, `push: false`, and exactly-once cache scopes. P8 scans the whole workflow directory for read-only permissions, registry surface, path filters and non-token secrets.
- The closing banner still derives solely from `RAN == EXPECTED_PARTITIONS`; `ALL PASSED` remains a single print site.

## Task Commits

1. **Task 1: add the `fe` job** — `a58763d` (ci)
2. **Task 2: add the `docker` job** — `19ac480` (ci)
3. **Task 3: extend the gate with P5-P8** — `6d50abb` (chore)

## Files Created/Modified

- `.github/workflows/ci.yml` — added `fe` and `docker` jobs; the `go` job, trigger block, workflow-level permissions and concurrency are untouched
- `verify-phase-48.sh` — `EXPECTED_PARTITIONS=8`, four new partitions, the closing banner's missing-partition loop extended to P5-P8, and the verified-partitions list updated

## Decisions Made

- **No job-level permissions on `fe` or `docker`.** The workflow-level `contents: read` is the ceiling; these two jobs need no API access at all. The only scope added anywhere remains the `go` job's `pull-requests: read` (D-29).
- **The compose-build literal is absent even from comments.** A raw-text ban is only meaningful if the workflow's own prose cannot satisfy the banned pattern — the same discipline 48-01 applied to `continue-on-error`.
- **P5-P8 mirror the established probe shape.** One Python probe per partition, one log, exit code authoritative, and an empty file list is a failure rather than a vacuous pass.
- **Docker closes on CI, not here.** The job is authored and structurally asserted; no local image build is claimed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The docker job's explanatory comment contained the literal `docker compose`**

- **Found during:** Task 2 acceptance check
- **Issue:** The comment explaining why the compose-driven build path is absent spelled `docker compose`, so the task's own `assert bad not in s` failed. A raw-text ban cannot tell workflow text from commentary about it.
- **Fix:** Reworded the comment to "the compose-driven build path is deliberately absent" without the banned two-word token.
- **Files modified:** `.github/workflows/ci.yml`
- **Verification:** `DOCKER-OK` / `DOCKER-LINE-OK`, and all eight gate partitions pass.
- **Commit:** `19ac480`

---

**Total deviations:** 1 auto-fixed (bug).
**Impact on plan:** None on scope; the fix tightened the same invariant the gate asserts.

## Issues Encountered

- **Fail-fast ordering means P8's GLM secret check is unreachable in practice.** Inserting `secrets.GLM_API_KEY` into `ci.yml` fails at P2/P3 (the go job's live-service env ban) before P8 runs. The gate still exits non-zero, and P8's own secret check was proven with `secrets.CUSTOM_TOKEN` (P8 failed and named the reference). This is the intended fail-fast behaviour, not a gap.
- **Deleting the composite action is caught by P1, not P5.** P1 already requires the composite and fails first, naming the missing path. P5 carries the same existence assertion, but fail-fast makes P1 the observable failure. Non-zero-with-the-file-named is preserved either way.
- **Docker is absent from both Git Bash and WSL on this host.** CICD-03 therefore has no local execution proof; this is the documented Pitfall 12 / VALIDATION M1 limitation, not a defect.
- `npx vitest run` reproduced the measured baseline exactly: 85 test files / 689 tests, exit 0. `npx tsc --noEmit` exited 0. No test count was written into any workflow or gate.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 48-04 can now assert over the final `ci.yml`: three jobs, checkout-first in each, `push: false` twice, and the eight-partition gate.
- CICD-02 and CICD-03 remain unmarked in REQUIREMENTS.md because 48-04 also declares both IDs; the shared-ID gate releases them when 48-04's SUMMARY lands.
- Still unproven locally and requiring a real GitHub Actions observation: that `fe` and `docker` are green on `ubuntu-latest`, and that the per-image caches actually hit on a second run (VALIDATION M1 / M5).

## Self-Check: PASSED

- `.github/workflows/ci.yml` and `verify-phase-48.sh` exist and are tracked.
- Task commits present in history: `a58763d`, `19ac480`, `6d50abb` (3 commits, measured from the plan ledger base `3bae85a`).
- `bash verify-phase-48.sh` exits 0 with `=== Phase 48 Gate: ALL PASSED ===` and eight partition PASS lines.
- `ci.yml` parses with job keys exactly `['go', 'fe', 'docker']`.
- Negative probes for P7, P8 and the composite all exit non-zero with the expected message.

---
*Phase: 48-ci*
*Completed: 2026-10-19*
