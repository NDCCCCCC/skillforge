---
phase: 48-ci
plan: 05
subsystem: infra
tags: [github-actions, verifier, trust-surface, mutation-testing, secrets, workflow-enumeration, permissions]

requires:
  - phase: 48-ci
    plan: 04
    provides: "scan_trust_surface as one shared shell function, P8's call site, and the P11 half-2 fixture harness this plan hardens"
provides:
  - "scan_trust_surface: every path-filter, secret, publication-token and permission rule runs per enumerated workflow file"
  - "both-extension enumeration (*.yml + *.yaml) in P1 and P8, with a ci.yml/ci.yaml subject guard"
  - "explicit named secret allowlist: GITHUB_TOKEN anywhere; GLM_API_KEY/BGE_M3_URL/MILVUS_ADDRESS only in eval-gate.yml"
  - "case-insensitive publication-token detection and value-shaped write detection"
  - "workflow- and job-level string permissions judged: none accepted, write-all and read-all rejected"
  - "P11 half 2 adversarial fixtures two-files, yaml-ext, lowercase-registry, stray-secret"
affects: [phase-48-ci, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 4998
  tasks: 3
  commits: 3

plan_head_before: 7dd020e2966bb6b70bc06c4b732b65d38a2c710a
plan_head_after: 11a3974c3f6d76b9059c893610504a0aef812ebf

tech-stack:
  added: []
  patterns:
    - "per-file policy scan: the rules are evaluated once per enumerated workflow, never against one privileged filename"
    - "explicit allowlists over accidental exemptions: a permitted secret names both the file and the secret"
    - "comment-stripped, value-shaped detection: prose about a write scope or publication token cannot trip the gate"
    - "adversarial fixtures run through the same shared function the real partition calls, and each must fail"

key-files:
  created: []
  modified:
    - verify-phase-48.sh

key-decisions:
  - "The secret allowlist is explicit and named — GITHUB_TOKEN anywhere; GLM_API_KEY, BGE_M3_URL and MILVUS_ADDRESS only in eval-gate.yml."
  - "read-all stays rejected: D-29 enumerates the ceiling as exactly {contents: read, pull-requests: read}, so accepting read-all would be a D-29 amendment first and a code change second."
  - "Every mutation probe writes under a trap and asserts git status --porcelain -- .github/workflows is empty, so a leaked fixture fails the probe rather than passing it."

patterns-established:
  - "A verifier claim is only as wide as its scope expression; the printed evidence line names the per-file computation."
  - "Both workflow extensions are enumerated everywhere a workflow file is counted or parsed."

requirements-completed: []

coverage:
  - id: D1
    description: "P8's path-filter and secret rules run per file, naming the offending file and rule"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#P8-PER-FILE-MUTATIONS-CAUGHT (zz-p8-paths.yml, zz-p8-secret.yml)"
        status: pass
    human_judgment: false
  - id: D2
    description: "P1 and P8 enumerate both *.yml and *.yaml workflow files"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#P1-YAML-ENUMERATION-OK and zz-p8-registry.yaml"
        status: pass
    human_judgment: false
  - id: D3
    description: "Publication tokens match case-insensitively and permissions string forms are judged"
    requirement: CICD-03
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#P11 fixtures lowercase-registry, yaml-ext and the none/read-all/write-all probes"
        status: pass
    human_judgment: false
  - id: D4
    description: "Four adversarial fixtures proven to fail through the shared scan_trust_surface"
    requirement: CICD-02
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh#P11 half 2 (two-files, yaml-ext, lowercase-registry, stray-secret)"
        status: pass
    human_judgment: false

duration: 6 min
completed: 2026-10-19
status: complete
---

# Phase 48 Plan 05: Per-file trust-surface scan Summary

**P8's trust-surface scan is now per-file over `*.yml` and `*.yaml`: path filters, secrets, publication tokens and string permissions are judged in every workflow, and four adversarial fixtures prove each rule bites.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-10-19T12:25:31Z
- **Completed:** 2026-10-19T12:32:33Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments

- `scan_trust_surface` now enumerates both workflow extensions at `-maxdepth 1`, keeps a zero-file guard, and keeps a subject guard requiring `ci.yml` or `ci.yaml`.
- The `paths:` / `paths-ignore:` and `secrets.` rules moved out of the single-file `ci_src` block into the per-file loop, so a newly added workflow can no longer escape them.
- The secret allowlist is explicit: `GITHUB_TOKEN` anywhere; `GLM_API_KEY`, `BGE_M3_URL`, `MILVUS_ADDRESS` only in `eval-gate.yml`; every other name fails with the file and secret named.
- Publication tokens (`ghcr.io`, `docker.io`, `quay.io`, `docker/login-action`, `registry:`, `dockerhub`, `docker push`, `packages: write`, `push: true`) are matched case-insensitively against comment-stripped text; `push: false` does not trip the rule.
- The raw `"write" in src` scan was replaced by a value-shaped, line-anchored check, so prose such as "this workflow writes the PR comment" no longer false-fires.
- Workflow- and job-level string permissions are judged: `none` accepted, `write-all` and `read-all` rejected, and the silent job-level string skip is closed.
- P1 parses the same file set P8 scans; the printed P8 evidence line states the per-file computation.
- P11 half 2 now runs eight fixtures — the original four vacuity cases plus `two-files`, `yaml-ext`, `lowercase-registry`, `stray-secret` — each required to fail for its named rule.

## Task Commits

Each task was committed atomically:

1. **Task 1: per-file path-filter and secret rules, both extensions, case-insensitive publication tokens** — `2387b61` (fix)
2. **Task 2: P1 parses every workflow file, not just *.yml** — `07882fc` (fix)
3. **Task 3: P11 half 2 proves the new per-file rules through the shared scan** — `11a3974` (test)

**Plan metadata:** committed separately (docs: complete plan)

## Files Created/Modified

- `verify-phase-48.sh` — `scan_trust_surface` rewritten (enumeration, subject guard, per-file rules, string permissions, evidence line); P1 enumeration widened; P8/P11 labels and comments updated; four new adversarial fixtures added to P11 half 2.

## Verification

Baseline and mutation evidence, all run on the real gate:

| Probe | Result |
|-------|--------|
| `bash verify-phase-48.sh` on the unmutated tree | rc=0, 11/11 partitions, single `ALL PASSED` |
| `zz-p8-paths.yml` with `push.paths` | rc=1, names `paths:` |
| `zz-p8-secret.yml` with `secrets.DEPLOY_KEY` | rc=1, names `DEPLOY_KEY` |
| `zz-p8-registry.yaml` with `docker push ghcr.io/...` | rc=1, names `ghcr.io` (also proves `.yaml` enumeration) |
| invalid `.yaml` (`on: [unclosed`) | P1 rc=1, names `zz-p1-invalid.yaml` |
| `two-files` fixture | rc=1, names `release.yml` + `paths:` |
| `yaml-ext` fixture | rc=1, names `other.yaml` + `packages: write` |
| `lowercase-registry` fixture | rc=1, names `ghcr.io` |
| `stray-secret` fixture | rc=1, names `DEPLOY_KEY` |
| `permissions: none` (workflow and job) | accepted (rc=0) |
| `permissions: read-all` (workflow and job) | rejected (rc=1) |
| `permissions: write-all` (workflow and job) | rejected (rc=1) |
| comment containing "writes" | accepted (rc=0) |
| `contents: write` | rejected (rc=1) |
| `secrets.GITHUB_TOKEN` | accepted (rc=0) |
| `secrets.GLM_API_KEY` in `ci.yml` | rejected (rc=1) |
| `secrets.GLM_API_KEY` in `eval-gate.yml` | accepted (rc=0) |
| `git status --porcelain -- .github/workflows` after every probe | empty (no fixture leaked) |

## Decisions Made

- The secret allowlist is explicit and named, replacing the old "whatever `ci.yml` contains is fine" accident.
- `read-all` remains rejected. This deliberately departs from IN-03's phrasing that P8 rejects a "valid read-only shorthand": D-29 is locked at exactly `{contents: read, pull-requests: read}`, so accepting `read-all` would widen the verifier's ceiling beyond the recorded decision. `none` and `write-all` handling closes the silent-skip half in full.
- Mutation probes assert that no fixture survives, so a probe that permanently changes the tree fails rather than passes.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Requirements Note

`requirements-completed: []` intentionally. The plan's frontmatter lists CICD-01/02/03 as the phase requirements it serves, but this gap-closure plan hardens the verifier; it does not obtain the first real GitHub Actions run. Per `48-VERIFICATION.md` and the 48-04 precedent, those three requirements remain blocked on the Class A evidence owned by 48-11. Marking them complete here would falsify the requirement record.

## Threat Flags

None. The change only narrows the verifier's assertion surface; no workflow, published check name, or runtime behavior was altered.

## Next Phase Readiness

- Ready for 48-06 (eval-gate live-gate integrity: `pipefail` and step-level secret scope) and the remaining gap-closure plans.
- CICD-01/02/03 stay blocked on one real CI run; this plan closes the Class B-1 verifier gap, not the Class A evidence gap.
- The security register re-closure (T-48-03/T-48-04/T-48-10/T-48-13 paths arm/T-48-20) is owned by 48-10 and now has mutation-proven evidence to cite.

## Self-Check: PASSED

- `verify-phase-48.sh` exists and is the only modified file.
- Task commits `2387b61`, `07882fc`, `11a3974` are ancestors of HEAD.
- Baseline gate exits 0 with 11/11 partitions and exactly one `ALL PASSED` print site.
- No fixture remains under `.github/workflows`.

---
*Phase: 48-ci*
*Completed: 2026-10-19*
