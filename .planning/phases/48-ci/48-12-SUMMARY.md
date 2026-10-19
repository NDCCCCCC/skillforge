---
phase: 48
plan: 12
subsystem: ci-workflows
tags: [supply-chain, secrets-hygiene, github-actions, gap-closure]
requires:
  - 48-VERIFICATION.md round 2 gap list (R2-CR-01, R2-WR-02, R2-WR-05 + advisories R2-IN-01/02/03, R2-WR-04)
  - verify-phase-48.sh LINT_VERSION=2.12.2 preflight (the version literal the install step must match)
provides:
  - ci.yml with zero network fetch-and-execute surface (module-proxy linter install + [ -x ] postcondition)
  - eval-gate.yml with live secrets scoped to the single consuming step
  - rename-proof P5a determinism segment (-list | grep -qx guard)
  - persist-credentials-free verify checkout + pinned pyyaml==6.0.3 + truthful four-job header
affects:
  - 48-13 (pins these shapes in P2/P3/P9 assertions)
  - 48-14 (re-records T-48-47 supply-chain row as closed; appends PyYAML legitimacy-audit row)
  - 48-15 (this is the tree the Class-A push publishes)
tech-stack:
  added: []
  patterns:
    - "go install <module>@<pinned-version> as the checksum-verified replacement for curl|sh bootstraps"
    - "[ -x ] executable postcondition after any install step that a later step depends on"
    - "go test -list '<name>' | grep -qx '<name>' existence guard to defeat -run zero-match vacuity"
key-files:
  created: []
  modified:
    - .github/workflows/ci.yml
    - .github/workflows/eval-gate.yml
decisions:
  - "Module-proxy install chosen over SHA-pinned install.sh + checksums.txt (48-VERIFICATION sanctioned shape 1): single command, no pipe, so R2-WR-02 closes by construction and no pipefail is cargo-culted into the step"
  - "P5a guard added under shell: bash + explicit pipefail (GitHub default bash -e {0} has none); both P2-pinned fragments kept byte-present"
  - "Gate check documented as the offline threshold comparison in the WR-08 comment, citing cmd/eval/main.go:463-467 short-circuit evidence"
metrics:
  duration: 10min
  completed: 2026-10-19
status: complete
actuals:
  tokens: 1712
  tasks: 3
  commits: 3
commits: 3
plan_head_before: 6bfd089f84bc767700924ca978f1094d16494f8e
plan_head_after: 2eae368bd6c51ca94adadcd0455a8f09ac3d4045
estimate:
  tokens: 9000
  tasks: 3
---

# Phase 48 Plan 12: Class-B workflow-side defect closure Summary

**One-liner:** ci.yml's verify job now bootstraps golangci-lint v2.12.2 through the Go module proxy with an executable postcondition (no fetch-and-execute surface anywhere in the file), live eval-gate secrets are scoped to the replay step only, and the determinism/P5a segment reds on a renamed test instead of passing vacuously.

## What Was Built

### Task 1 — module-proxy linter install (R2-CR-01, R2-WR-02) — tracer
- Replaced `curl -sSfL .../HEAD/install.sh | sh` with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` (checksum-DB verified via the module proxy).
- Added `[ -x "$LINT_BIN/golangci-lint" ] || exit 1` postcondition naming the expected path before the gate runs; kept `GITHUB_PATH` export.
- No `set -o pipefail` added — `go install` is a single command with an authoritative exit code, so R2-WR-02 (curl exit dropped by the pipe) closes by construction.
- Verify job `timeout-minutes` 15 → 25 (source build on a cold runner + the gate's P4 sentinel runs).
- Job comment rewritten: module-proxy mechanism, LINT_VERSION sync requirement, no remaining fetch-and-execute surface; WR-06 lint-action rationale kept.
- Probe: `INSTALL-OK`. Local teeth: `golangci-lint version` reports exactly 2.12.2 on this host.

### Task 2 — ci.yml hygiene (R2-IN-01, R2-IN-02, R2-IN-03, R2-WR-04)
- Header now names four jobs — `go` (CICD-01), `fe` (CICD-02), `docker` (CICD-03), `verify` (IN-07) — replacing the stale "Three independent jobs" text.
- Verify-job checkout carries `persist-credentials: false` (mirroring the go job's IN-10 hardening); job comment extended with the fork-PR self-attestation caveat.
- PyYAML pinned: `python -m pip install --quiet --disable-pip-version-check "pyyaml==6.0.3"`.
- P5a determinism step: `shell: bash` + `set -o pipefail` + `go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' | grep -qx 'TestDeriveEdgesDeterministic'` guard before the unchanged `-count=100` run. Teeth proven locally: the guard passes on the real test and reds on a nonexistent name.
- Probe: `HYGIENE-OK`. No `continue-on-error`, no `paths:`, no step-level `if`, no trigger widening.

### Task 3 — secret scoping (R2-WR-05)
- Deleted the three-secret `env:` block from the `Gate check` step; its run body and working-directory are unchanged. `--gate-check` short-circuits before any bootstrap (cmd/eval/main.go:463-467), so the grant was dead credential surface.
- Each of `secrets.GLM_API_KEY` / `secrets.BGE_M3_URL` / `secrets.MILVUS_ADDRESS` now occurs exactly once in the file, on the `Run frozen compliance replay` step (P9's secrets-intact assertion still holds).
- WR-08 job-env comment updated to the single consuming step, with the offline-threshold rationale and code citation.
- Probe: `SECRET-SCOPE-OK`.

## Verification

- `bash verify-phase-48.sh` exits 0 on the edited tree with 11/11 partitions and a single `ALL PASSED` (run after every task and once more at the end).
- All three plan verify probes printed their sentinels: `INSTALL-OK`, `HYGIENE-OK`, `SECRET-SCOPE-OK`.
- `git diff 6bfd089..2eae368 --stat` touches exactly `.github/workflows/ci.yml` and `.github/workflows/eval-gate.yml`.
- ci.yml re-proven free of `secrets.*`, `continue-on-error`, `pull_request_target`, `workflow_dispatch`, `|| true`, and write-permission scopes by the P1/P8/P9 assertions.

## Deviations from Plan

None - plan executed exactly as written.

## Authentication Gates

None — the gate's tooling precondition (golangci-lint v2.12.2 on PATH) was already met on this host.

## Commits

| Task | Commit | Files |
| ---- | ------ | ----- |
| 1 | 0e25ecc | .github/workflows/ci.yml |
| 2 | 05803d5 | .github/workflows/ci.yml |
| 3 | 2eae368 | .github/workflows/eval-gate.yml |

## Self-Check: PASSED

- Files verified present: `.github/workflows/ci.yml`, `.github/workflows/eval-gate.yml`, this SUMMARY.
- Commits verified in HEAD ancestry: 0e25ecc, 05803d5, 2eae368 (all ancestors of 2eae368 = plan_head_after).
- Measured commit count from ledger 6bfd089..HEAD = 3, matching the table above.
