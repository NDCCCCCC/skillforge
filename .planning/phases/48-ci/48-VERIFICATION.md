---
phase: 48-ci
verified: 2026-10-19T10:45:00Z
status: gaps_found
score: 3/9 must-haves verified
covered_files:
  - .github/actions/setup/action.yml
  - .github/workflows/ci.yml
  - .github/workflows/eval-gate.yml
  - .golangci.yml
  - .planning/phases/48-ci/48-01-PLAN.md
  - .planning/phases/48-ci/48-01-SUMMARY.md
  - .planning/phases/48-ci/48-02-PLAN.md
  - .planning/phases/48-ci/48-02-SUMMARY.md
  - .planning/phases/48-ci/48-03-PLAN.md
  - .planning/phases/48-ci/48-03-SUMMARY.md
  - .planning/phases/48-ci/48-04-PLAN.md
  - .planning/phases/48-ci/48-04-SUMMARY.md
  - verify-phase-48.sh
covered_digest: "v3:sha256:7960094c747d40303ce8bd841aac0aa41eed15cf031508311a7897fae5f1b3a0"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC-1 / CICD-01: a PR or push triggers the Go CI line and it is green (golangci-lint + go test ./... + CGO_ENABLED=1 build+vet)"
    status: failed
    reason: "The declaration is complete and statically asserted, but no ci.yml run has ever existed. `gh run list --workflow ci.yml` returns HTTP 404 'workflow ci.yml not found on the default branch'; `git ls-remote origin` fails at TLS. The outcome the criterion names — a public green light — is absent."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "Job declaration is correct; its runtime conclusion has never been observed."
    missing:
      - "One real GitHub Actions run with go=success, obtained from a host with working git transport to github.com (PR based on main, or the commit landed on main)"
  - truth: "SC-2 / CICD-02: a PR or push triggers the FE CI line and it is green (tsc --noEmit + vitest run)"
    status: failed
    reason: "The fe job declaration is correct and both commands reproduce green locally (tsc rc=0; vitest 85 files / 689 tests rc=0), but no CI run exists to prove the job is green on ubuntu-latest."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "fe job declared and locally reproducible; never executed on a runner."
    missing:
      - "A real GitHub Actions run with fe=success"
  - truth: "SC-3 / CICD-03: both Dockerfiles build successfully in CI"
    status: failed
    reason: "Declaration-only proof. `docker` is absent from the host (no docker/docker.exe on PATH), so no image was ever built; and no CI run exists to execute the docker job."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "Docker job structurally correct (two build steps, push: false, per-image scopes); never executed."
    missing:
      - "A real GitHub Actions run with docker=success"
  - truth: "SC-4: the live gate never reads as a fake green (skip notice is the exact complement; a failed eval run cannot report success)"
    status: failed
    reason: "Two concrete fake-green paths remain. WR-02: the notice's if: is not the exact complement of the gate's — a Dependabot-attributed push skips the live gate with no notice. WR-03: the replay step pipes ./bin/eval through tee with no pipefail, so a non-zero eval exit is discarded and the step reports success on a report the tool considered failed. The fork-PR / Dependabot-PR branch itself is correctly gated."
    artifacts:
      - path: ".github/workflows/eval-gate.yml"
        issue: "if: expressions are not exact complements (lines 56-59 vs 129-132); replay step lacks pipefail/shell: bash (lines 93-101)."
    missing:
      - "set -o pipefail (or shell: bash) on the frozen-replay step"
      - "A skip-notice condition that is the true logical inverse of the gate condition"
  - truth: "The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20)"
    status: failed
    reason: "Mutation-proven false in four independent ways. (a) WR-04: deleting `enable: [revive]` and its settings keeps `config verify` rc=0, sentinel A rc=1/errcheck, sentinel C rc=0 → P4 reports lint-teeth PASS with revive absent. (b) WR-01: five step-level `if: false` on the go job's gate steps leave the real P2/P3 probe printing STRUCT-OK rc=0. (c) WR-07: appending `|| true` to the fork gate, or wrapping it in a negation, leaves the real P9 probe printing EVALGATE-OK rc=0. (d) IN-06: P11 counts textual done_partition call sites, not executed partition bodies."
    artifacts:
      - path: "verify-phase-48.sh"
        issue: "P2/P3 (lines 197-249) do not inspect step-level if:; P4 (lines 343-419) has no positive revive sentinel; P9 (lines 718-730) asserts three substrings instead of the expression; P11 half 1 (lines 868-872) counts source text."
    missing:
      - "Step-level if: rejection in P2/P3 (and P6/P7)"
      - "A positive revive sentinel in P4 (package-comments violation)"
      - "Exact-expression or neutralisation-rejecting assertions in P9"
      - "Accounting derived from observable partition effects, not call-site text"
  - truth: "The trust-surface partition is directory-wide as its header and banner claim"
    status: failed
    reason: "CR-01: the path-filter and secret rules in scan_trust_surface read only `$dir/ci.yml`; every other rule is per-file. Reproduced with the real function: a second .yml carrying `paths: ['internal/**']` returned TRUST-SURFACE-OK rc=0, and so did one carrying `${{ secrets.DEPLOY_KEY }}`. WR-05: `find -name '*.yml'` makes .yaml workflows invisible; a .yaml with `permissions: contents: write` and `docker/login-action@v4` returned rc=0 while the identical .yml was rejected. IN-02: lowercase `ghcr.io` / `docker push` slips the case-sensitive registry list."
    artifacts:
      - path: "verify-phase-48.sh"
        issue: "scan_trust_surface lines 589, 592, 663-675 scope two rule families to one filename; P1 line 132 enumerates *.yml only."
    missing:
      - "Per-file path-filter and secret rules inside the existing for-loop"
      - "*.yaml enumeration in P1 and P8"
      - "Lowercase publication tokens (ghcr.io, docker push)"
---

# Phase 48: CI 矩阵 Verification Report

**Phase Goal:** 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Verified:** 2026-10-19T10:45:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification (no prior `48-VERIFICATION.md`)

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 / CICD-01: Go CI triggered and green | ✗ FAILED | Declaration complete (build+vet, pinned v2.12.2 lint with `pull-requests: read`, literal `go test ./...`, determinism pair with `-v`). No run: `gh run list --workflow ci.yml` → `HTTP 404: workflow ci.yml not found on the default branch`. |
| 2 | SC-2 / CICD-02: FE CI triggered and green | ✗ FAILED | `fe` job declares `npm ci` → `npx tsc --noEmit` → `npx vitest run` under `web/`. Locally reproduced: tsc rc=0; vitest 85 files / 689 tests rc=0. Never executed on a runner. |
| 3 | SC-3 / CICD-03: both Dockerfiles build in CI | ✗ FAILED | Two `docker/build-push-action@v7` steps, `push: false`, `scope=backend`/`scope=frontend`; both Dockerfiles exist. `docker`/`docker.exe` absent from the host; no image built; no run. |
| 4 | SC-4: live gate never fakes green | ✗ FAILED | Fork/Dependabot **PR** branch is correctly gated (3 clauses) and notified. But WR-02: notice is not the complement (Dependabot **push** skips silently); WR-03: `\| tee` without pipefail discards a non-zero `./bin/eval` exit (verified `cmd/eval/main.go:648-653` has a post-report `log.Fatalf`). |
| 5 | SC-5: `bash verify-phase-48.sh` exits 0 | ✓ VERIFIED | Ran it: rc=0, 11/11 partitions PASS, single `=== Phase 48 Gate: ALL PASSED ===` banner. |
| 6 | Verifier refuses vacuous passes (P23) | ✗ FAILED | Four mutation-proven vacua: WR-04 (revive deleted → P4 green), WR-01 (5 × `if: false` → P2/P3 green), WR-07 (`\|\| true` / negation → P9 green), IN-06 (call sites ≠ executed). See "Mutation Re-check". |
| 7 | Trust-surface is directory-wide as claimed | ✗ FAILED | CR-01: a second `.yml` with `paths:` or `secrets.DEPLOY_KEY` returns `TRUST-SURFACE-OK` rc=0. WR-05: `.yaml` with `contents: write` + `docker/login-action@v4` returns rc=0. |
| 8 | Fork/Dependabot gate declared correctly; secrets intact; one Go version source | ✓ VERIFIED | Three clauses present; `eval-gate-skip-notice` has `::notice::`; all three `secrets.*` mappings present; zero `go-version:` literals under `.github/workflows/` and `.github/actions/`; every `setup-go` uses `go-version-file: go.mod`. |
| 9 | No gate was weakened to manufacture a run | ✓ VERIFIED | Independently grepped: zero `continue-on-error`, zero `paths:`/`paths-ignore:`, zero `workflow_dispatch`, zero `\|\| true`, zero `pull_request_target`, zero real `secrets.*` in `ci.yml`; `push.branches: [main]` intact. |

**Score:** 3/9 truths verified (0 present, behavior-unverified)

### Deferred Items

None. Phases 49–51 do not address any of the failed truths: Phase 51 depends on the Phase 48 matrix for release tooling, but no later phase owns obtaining the first CI run or hardening the verifier. All gaps below are actionable now.

---

## Security Closure Re-check

The five mutation-proven findings were re-executed independently here, using the **real** probe code extracted from `verify-phase-48.sh` and the real `golangci-lint 2.12.2` binary. No repository file was modified.

| Finding | SECURITY rows affected | Verdict | Independent evidence |
|---------|------------------------|---------|----------------------|
| **CR-01** — P8's path/secret rules read only `ci.yml` | T-48-03, T-48-04 (claimed closed) | **CONFIRMED** | Second `.yml` with `paths: ['internal/**']` → `TRUST-SURFACE-OK (2 workflow file(s))`, rc=0. Second `.yml` with `${{ secrets.DEPLOY_KEY }}` → rc=0. |
| **WR-05** — `.yaml` invisible | T-48-10 (and P1 coverage) | **PARTIALLY CONFIRMED** | `.yaml` containing `permissions: contents: write` + `docker/login-action@v4` → rc=0 (counted as 1 file, not scanned), while the identical `.yml` → rc=1. **Sub-claim refuted:** P1's `pull_request_target` scan is `grep -rn` over the directory and *does* catch `.yaml`; only P1's parse/trigger loop and P8's enumeration are extension-blind. |
| **WR-07** — P9 substring clauses | T-48-02, T-48-18 (claimed closed) | **CONFIRMED** | Appending `\|\| true` → `EVALGATE-OK (2 jobs, 3-clause gate, notice present)`, rc=0. Wrapping the whole expression in a negation → same output, rc=0. |
| **WR-01** — step-level `if:` unchecked | T-48-07 (partial) | **CONFIRMED** | Five step-level `if: false` on build+vet, lint, go test, and both determinism steps → the real P2/P3 probe prints `STRUCT-OK`, rc=0. |
| **WR-04** — no positive `revive` sentinel | T-48-11, T-48-20 (claimed closed) | **CONFIRMED** | Config with `enable` and `settings` removed: `config verify` rc=0; sentinel A rc=1 naming `errcheck`; sentinel C rc=0 → P4 would report PASS with revive absent. |
| **WR-02** — notice is not the complement | T-48-15 ("exactly-inverted") | **CONFIRMED (logic)** | Gate = `(¬PR ∨ same_repo) ∧ ¬dependabot`; notice = `PR ∧ (fork ∨ dependabot)`. The region `event ≠ PR ∧ dependabot` is skipped by both. The SC-4 literal case (Dependabot **PR**) is covered; a Dependabot-attributed **push** is not. No runner observation exists. |
| **WR-03** — `tee` swallows exit code | Not previously mapped | **CONFIRMED (semantics + code)** | No `shell:` key on the replay step → GitHub's Linux default is `bash -e {0}`, without `pipefail`; the pipeline status is `tee`'s. `cmd/eval/main.go` has a `log.Fatalf` after `WriteReport` succeeds, and the following gate step reads that already-written report. |

**Consequence:** more than five register rows are affected. `48-SECURITY.md` already contains a narrative section admitting five falsified closures, but its frontmatter still says `threats_open: 0` / `status: verified` and the register still marks each row `closed`. The security record is internally inconsistent; the narrative is honest, the machine-readable verdict is not.

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.github/actions/setup/action.yml` | Checkout-free composite, three pinned majors | ✓ VERIFIED | `runs.using: composite`; inputs `go`/`node`/`buildx`; exactly one occurrence each of setup-go@v7 / setup-node@v7 / setup-buildx@v4; zero `actions/checkout`; `go-version-file: go.mod` once. |
| `.golangci.yml` | v2 schema, no baseline/allowlist | ✓ VERIFIED | v2 schema; `default: standard`; `revive` enabled; `unused-parameter` disabled; `std-error-handling` preset; `_test.go` errcheck exclusion. `config verify` rc=0 on 2.12.2. Runtime presence of revive is not positively assured (WR-04). |
| `.github/workflows/ci.yml` | Three jobs `go`/`fe`/`docker` | ✓ VERIFIED (declaration) | Exactly three jobs; checkout-first in each; no job-level `permissions`/`if:` on `fe`/`docker`; go job permissions exactly `{contents: read, pull-requests: read}`; three graceful timeouts (20/15/45). Runtime conclusion unproven. |
| `.github/workflows/eval-gate.yml` | Live workflow gated for fork/Dependabot, notified | ✓ VERIFIED (declaration) | Two jobs; three-clause gate; notice job with `::notice::`. Contains WR-02/WR-03 defects above. |
| `verify-phase-48.sh` | 11 partitions, zero-waiver, self-proving | ✓ VERIFIED (execution) / ✗ defective (assertions) | Runs rc=0 with 11 partitions; single RAN-gated banner. Its assertions are narrower than their claims (see truths 6–7). |

No stub artifacts were found: none of the five deliverables is a placeholder, and no `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` marker exists in any of them.

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| Every `ci.yml` job | `./.github/actions/setup` | step after `actions/checkout@v7` | WIRED | P5 confirms checkout-first in all three jobs; verified by reading the file. |
| `ci.yml` lint step | `.golangci.yml` | golangci-lint-action default config discovery | WIRED | No `args` override; repo-root config is auto-discovered. |
| `ci.yml` docker steps | `Dockerfile.backend` / `Dockerfile.frontend` | `with.file` | WIRED | Both files exist at the repository root. |
| `ci.yml` fe steps | `web/package-lock.json`, `web/` sources | `working-directory: web` + composite npm cache | WIRED | Lockfile exists; local `tsc`/`vitest` reproduce green. |
| `eval-gate.yml` | `go.mod`, `cmd/eval`, `examples/skills/compliance/eval-gate.json` | `go-version-file: go.mod`; build/run targets | WIRED | All targets exist on disk. |
| `verify-phase-48.sh` | `.github/workflows/`, `.planning/ROADMAP.md` | `$ROOT`-relative probes | WIRED | Executed successfully. |
| Any CI workflow | `verify-phase-48.sh` | — | NOT WIRED | IN-07: no workflow invokes the local gate; it can rot undetected. |

### Data-Flow Trace (Level 4)

Not applicable in the usual sense: this phase ships declaration files and a static verifier, not a rendering/query path. The equivalent trace — "does each declared command reach a real target?" — is covered in the Key Link table above. The one trace that would matter (declared job → runner conclusion) has **no data**: no run exists.

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase gate runs and is green | `bash verify-phase-48.sh` | rc=0; 11/11 PASS; single `ALL PASSED` | ✓ PASS |
| FE typecheck command is real | `cd web && npx tsc --noEmit` | rc=0 | ✓ PASS |
| FE test command is real | `cd web && npx vitest run` | 85 files / 689 tests, rc=0 | ✓ PASS |
| Shuffle seed reachable only under `-v` (D-19) | WSL `go test … -shuffle=on -v` vs without `-v` | `-test.shuffle 1792406095814820130` printed with `-v`; nothing without it | ✓ PASS |
| Determinism test target exists | `grep TestDeriveEdgesDeterministic` | Found at `internal/knowledge/graph_edges_test.go:50`; WSL `-shuffle=on -v` run PASS | ✓ PASS |
| CI run exists for this commit | `gh run list --workflow ci.yml` | `HTTP 404: workflow ci.yml not found on the default branch` | ✗ FAIL (missing evidence) |
| Docker image builds | — | `docker` / `docker.exe` absent from the host | ? SKIP |

### Probe Execution

No conventional `scripts/*/tests/probe-*.sh` probes exist for this phase. The phase's own probe is `verify-phase-48.sh`, executed above.

---

## Environment Facts Re-confirmed

| Claim | Verdict | Evidence |
|-------|---------|----------|
| git transport to github.com is unavailable | ✓ CONFIRMED | `git ls-remote origin HEAD` → `schannel: failed to receive handshake, SSL/TLS connection failed` (this run surfaced a TLS failure rather than the recorded 21 s connect timeout; same class — no transport). |
| `origin/main` is unrelated to this branch | ✓ CONFIRMED | `origin/main` = single commit `369b08a`; `git merge-base origin/main HEAD` empty; HEAD is 1839 commits ahead. |
| `ci.yml` has never been on the remote default branch | ✓ CONFIRMED | `gh run list --workflow ci.yml` → HTTP 404; the most recent run of any workflow is `eval-gate` on `main` from 2026-10-02. |
| Docker is absent | ✓ CONFIRMED | No `docker` or `docker.exe` on PATH. |
| Branch protection is off by decision (Option A) | ✓ CONFIRMED | `gh api .../branches/main/protection` → 404 `Branch not protected`; `48-04-SUMMARY.md` records `Decision: A` verbatim. |
| No gate was weakened to manufacture a run | ✓ CONFIRMED | See Truth 9. |

The environment-blocking recorded in `48-04-SUMMARY.md` is accurate and honest. `48-04-SUMMARY.md` carries `status: blocked` and `requirements-completed: []`; `REQUIREMENTS.md` still marks CICD-01/02/03 `Pending`.

---

## Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| CICD-01 | 48-01, 48-03, 48-04 | Go CI in GitHub Actions: golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet, PR/push triggered | ✗ BLOCKED (Pending) | Declaration complete and statically asserted; no CI run exists (404). |
| CICD-02 | 48-02, 48-04 | FE CI: `tsc --noEmit` + full `vitest run` | ✗ BLOCKED (Pending) | Declaration complete; commands reproduce locally; no CI run. |
| CICD-03 | 48-02, 48-04 | Docker build verification: both Dockerfiles build in CI | ✗ BLOCKED (Pending) | Declaration complete; no image ever built; no CI run. |

**Orphaned requirements:** none. All three IDs are declared in plan frontmatter, and no plan claims an ID that `REQUIREMENTS.md` does not map to Phase 48.

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `verify-phase-48.sh` | 589, 663-675 | Rule scope narrower than the advertised "directory-wide" claim | 🛑 Blocker | A new workflow escapes the `paths:`/`secrets.` guards (CR-01). |
| `verify-phase-48.sh` | 592 | `find -name '*.yml'` excludes `.yaml` | ⚠️ Warning | Extension-blind trust surface (WR-05). |
| `verify-phase-48.sh` | 228 | Only the job-level `if:` is checked | ⚠️ Warning | Step-level skip disables every gate step (WR-01). |
| `verify-phase-48.sh` | 721-730 | Three independent substrings for one expression | ⚠️ Warning | `\|\| true` / negation accepted (WR-07). |
| `verify-phase-48.sh` | 394-417 | No positive `revive` sentinel | 🛑 Blocker | P4 vacuous with respect to revive (WR-04). |
| `verify-phase-48.sh` | 869-872 | Counts call sites, not executed partitions | ⚠️ Warning | Commented-out body with a live `done_partition` still passes (IN-06). |
| `eval-gate.yml` | 93-101 | Pipeline through `tee` without pipefail | 🛑 Blocker | The live gate can report success on a failed run (WR-03). |
| `eval-gate.yml` | 123-132 | "exactly the complement" is false | ⚠️ Warning | Dependabot-attributed push skips with no notice (WR-02). |
| — | — | No workflow invokes `verify-phase-48.sh` | ℹ️ Info | Gate is a manual step and can drift (IN-07). |

No debt markers (`TBD`/`FIXME`/`XXX`) were found in any deliverable, so the debt-marker gate does not fire.

---

## Gaps Summary

Two independent classes of failure keep this phase from achieving its goal.

**Class A — the public green light does not exist (SC-1/2/3, CICD-01/02/03).** The workflows are correct, complete, hermetic, and unweakened; the static contract is genuinely good. But the phase goal is *"三线在 GitHub Actions 上公开绿灯"*, and no `ci.yml` run has ever existed. `gh run list` returns 404, `origin/main` shares no history, and git transport fails. The phase's own record says `status: blocked` and leaves the requirements Pending — that is the honest state and this verification confirms it. It is an environment block, not a code defect, but it is still a failed must-have: goal-backward, the outcome is absent.

**Class B — the gate's green means less than it claims (SC-4/SC-5 spirit).** This is where the phase's own thesis turns against it. The phase exists to prevent "a gate that passes without having run". Five of its own checks reproduce that exact defect when mutated: P8 reports directory-wide path/secret coverage from a single file; `.yaml` workflows are invisible; P9 accepts a neutralised fork gate; P2/P3 accept a `if: false`-disabled go job; P4 accepts a config with `revive` deleted. Independently reproduced against the real probe code, `48-SECURITY.md`'s five falsified closures are confirmed — with one nuance: WR-05's claim that `.yaml` escapes the `pull_request_target` scan is refuted, because that one rule uses recursive grep. The security register must not be read as `threats_open: 0` until these are folded in.

The deliverable quality is not uniform: the YAML is well-built, `ci.yml` has no governance weakening, and the `verify-phase-48.sh` banner/registry mechanism is genuinely sound. What fails is the *coverage surface* of several partitions and the *ground truth* of the CI outcome.

**Minimum path to close:** (1) obtain one real CI run from a host with git transport (PR to `main` or land on `main`) showing `go`/`fe`/`docker` all `success`; (2) harden P8 per-file for path/secret rules and enumerate `*.yaml`; (3) add a step-level `if:` rule to P2/P3/P6/P7; (4) add a positive `revive` sentinel; (5) assert P9 on the normalised expression rather than substrings; (6) add `set -o pipefail` to the replay step; (7) make the skip-notice the true inverse. Then update `48-SECURITY.md`'s register and `threats_open`.

---

_Verified: 2026-10-19T10:45:00Z_
_Verifier: Claude (gsd-verifier)_
