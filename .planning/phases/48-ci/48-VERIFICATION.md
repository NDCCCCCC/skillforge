---
phase: 48-ci
verified: 2026-10-20T11:05:00Z
status: gaps_found
score: 9/15 must-haves verified
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
  - .planning/phases/48-ci/48-05-PLAN.md
  - .planning/phases/48-ci/48-05-SUMMARY.md
  - .planning/phases/48-ci/48-06-PLAN.md
  - .planning/phases/48-ci/48-06-SUMMARY.md
  - .planning/phases/48-ci/48-07-PLAN.md
  - .planning/phases/48-ci/48-07-SUMMARY.md
  - .planning/phases/48-ci/48-08-PLAN.md
  - .planning/phases/48-ci/48-08-SUMMARY.md
  - .planning/phases/48-ci/48-09-PLAN.md
  - .planning/phases/48-ci/48-09-SUMMARY.md
  - .planning/phases/48-ci/48-10-PLAN.md
  - .planning/phases/48-ci/48-10-SUMMARY.md
  - .planning/phases/48-ci/48-11-PLAN.md
  - .planning/phases/48-ci/48-11-SUMMARY.md
  - .planning/phases/48-ci/48-12-PLAN.md
  - .planning/phases/48-ci/48-12-SUMMARY.md
  - .planning/phases/48-ci/48-13-PLAN.md
  - .planning/phases/48-ci/48-13-SUMMARY.md
  - .planning/phases/48-ci/48-14-PLAN.md
  - .planning/phases/48-ci/48-14-SUMMARY.md
  - .planning/phases/48-ci/48-15-PLAN.md
  - .planning/phases/48-ci/48-15-SUMMARY.md
  - .planning/phases/48-ci/48-RESEARCH.md
  - .planning/phases/48-ci/48-REVIEW-DISPOSITION-R1.md
  - .planning/phases/48-ci/48-REVIEW-DISPOSITION.md
  - .planning/phases/48-ci/48-REVIEW.md
  - .planning/phases/48-ci/48-SECURITY.md
  - .planning/phases/48-ci/48-VALIDATION.md
  - cmd/eval/main.go
  - verify-phase-48.sh
covered_digest: "v3:sha256:58ea185db05b69835defc1190b32eae98a88a13a131d9b47d5aa9599ba50743b"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 5/9
  gaps_closed:
    - "R2-CR-01 — the verify job's `curl | sh` bootstrap is gone; ci.yml now installs golangci-lint v2.12.2 through the module proxy with a `[ -x ]` postcondition, and the P2/P3 probe pins that shape (re-adding a fetch/script install reds the gate — reproduced here)"
    - "R2-WR-05 — the three live secrets now sit only on the consuming replay step; re-adding the env block to `Gate check` reds P9 (reproduced here)"
    - "R2-WR-01 — the `verify` job is pinned by the shared P2/P3 probe: a job-level `if:` on it and deleting the job entirely both red the gate (M9/M10 re-proven here)"
    - "R2-WR-03 (M8 shape) — a deleted/neutered P8 probe now aborts on the unbound `TRUST_SURFACE_COUNT` instead of printing `PASS P8 (0 workflow(s))` (re-proven here)"
    - "R2-WR-02 / R2-WR-04 / R2-IN-01 / R2-IN-02 / R2-IN-03 — install pipeline has no pipe; the `-list` guard is pinned by P2; the four-job header is truthful; PyYAML is pinned; verify checkout is credential-free"
    - "Class-A environment block — the phase now has FOUR real, public GitHub Actions runs of `ci.yml` against the public default branch (the round-2 report's `gh run list --workflow ci.yml` → HTTP 404 no longer reproduces)"
  gaps_remaining:
    - "SC-1 / CICD-01 — the go job's published check is still `failure` in all four runs"
    - "SC-2 / CICD-02 — the fe job's published check is still `failure` in all four runs"
    - "SC-3 / CICD-03 — the docker job's published check is still `failure` in all four runs"
    - "Truth 6 (verifier honesty) — the success-token accounting still folds a neutered probe into PASS in 8 of 11 partitions, and a job-level `if:` on the `fe`/`docker` jobs is not rejected"
    - "Truth 15 (record reconciliation) — three records carry a false root-cause classification for the P5a red, and the run count is understated (3 recorded vs 4 real)"
  regressions:
    - "R2-WR-04's fix (48-12's rename-proof `-list | grep -qx` guard, commit 05803d5) turned a vacuous-pass defect into a deterministic red on every Linux runner: the `go` job can never be green with the step as written. Introduced by the gap-closure wave, surfaced by the first real run."
gaps:
  - truth: "SC-1 / CICD-01: a PR or push triggers the Go CI line and the published check is green"
    status: failed
    reason: "Four public runs exist (ids 39350901689 / 39352480525 / 39353593172 / 39355131431, event `push` to `main`, remote `main` = 304dcc4) and in every one of them `go=failure`. The failing step is the phase-added `determinism -count=100 (P5a)` guard, not the requirement's own gates: on run-4 the go job's `go build ./...`/vet, `golangci-lint-action@v9` and `go test ./... -count=1` steps all succeeded (runner log `0_go.txt`) and the job died at P5a with `Process completed with exit code 141` and an empty step log. The cause is in `ci.yml` itself and is deterministic: `go test … -list … | grep -qx …` under `set -o pipefail` — `grep -q` closes the pipe at the first match, `go test -list` still writes its trailing `ok <pkg> <time>` line, the producer takes SIGPIPE, and pipefail propagates 141."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "Line 105: `go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' | grep -qx 'TestDeriveEdgesDeterministic'` under `set -o pipefail` — deterministic exit 141 on Linux, introduced by 48-12 (commit 05803d5)."
      - path: ".planning/phases/48-ci/48-VALIDATION.md"
        issue: "M1c classifies the red as a runner-environment artefact and prescribes runner-side remedies; the cause is in the workflow."
    missing:
      - "One line: drop `-q` (`grep -x '…' >/dev/null`) or materialise the list to a file before grep — both verified exit 0 — then push to `main` for a green go job"
      - "Reclassification of M1c from 'runner environment' to 'workflow pipeline shape' in 48-VALIDATION.md, 48-SECURITY.md and ROADMAP.md"
  - truth: "SC-2 / CICD-02: a PR or push triggers the FE CI line and the published check is green"
    status: failed
    reason: "`fe=failure` in all four runs. `npm ci` and `npx tsc --noEmit` succeed; `npx vitest run` fails with exactly 1 of 689 tests: `TestSparklineNoDeps` — `ENOENT: no such file or directory, open 'D:/code/ClaudeCode/ai-agent/web/src/components/chat/parts/Sparkline.tsx'`. The cause is in the repository, not the runner: `web/src/components/chat/parts/Sparkline.test.tsx:80` hard-codes a Windows absolute path (it is the only file in `web/src` that does)."
    artifacts:
      - path: "web/src/components/chat/parts/Sparkline.test.tsx"
        issue: "Line 80: absolute Windows path baked at test-author time; the file does not exist on the Linux runner."
    missing:
      - "Resolve the path relative to the repo/test file (the assertion's intent is 'the component does not import a dependency'), then re-push for a green fe job"
  - truth: "SC-3 / CICD-03: both Dockerfiles build successfully in CI"
    status: failed
    reason: "`docker=failure` in all four runs at the `build backend image` step: the alpine-musl linker fails on the static duckdb archive — `undefined reference to 'backtrace'`, `'backtrace_symbols'`, `'malloc_trim'`, `'__res_init'` from `libduckdb_static.a` in `Dockerfile.backend`'s alpine base. The `build frontend image` step is never reached, so CICD-03's 'both Dockerfiles' half is entirely unproven either way."
    artifacts:
      - path: "Dockerfile.backend"
        issue: "alpine/musl base image cannot link the static duckdb archive's glibc dependencies."
    missing:
      - "Switch the backend build base to a glibc image (or apply musl-compatibility flags), and let the frontend image build run at least once"
  - truth: "The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20)"
    status: failed
    reason: "Round 2's three recorded vacua (M8/M9/M10) are genuinely closed and I re-proved each by mutation. But the generic class survives: `ok_partition` is a hand-placed line in each partition body, so neutering or deleting a probe while keeping the bookkeeping still prints `ALL PASSED` with exit 0. Reproduced here in a sandbox copy: P9's probe invocation replaced by `:` → exit 0, `ALL PASSED`; P9's probe invocation deleted entirely → exit 0, `ALL PASSED`; the shared P2/P3 probe invocation replaced by `:` → exit 0, `ALL PASSED`. Only P8 has a mechanical backstop (the unbound `TRUST_SURFACE_COUNT`). A second, distinct vacuity of the same family survives as well: the job-level `if:` rejection exists for the `go` and `verify` jobs but not for the `fe` and `docker` jobs — adding `if: github.event_name == 'push'` to either of them leaves the gate at exit 0 / `ALL PASSED`."
    artifacts:
      - path: "verify-phase-48.sh"
        issue: "`ok_partition` deposits are not derived from the probe's exit status; P6/P7 reject step-level `if:` but not a job-level `if:` on the fe/docker jobs (P3 does both for go, and both for verify)."
    missing:
      - "Route every probe through a wrapper that deposits the token only on rc 0 (or give each partition a probe-owned evidence variable interpolated into its `done_partition` message, the P8 pattern) — 8 of 11 partitions are currently unprotected"
      - "Extend the job-level `if:` rejection to the `fe` and `docker` jobs (P6/P7), matching P3's go-job and verify-job treatment"
  - truth: "The recorded root cause of the P5a red is true in every record that states it"
    status: partial
    reason: "Three records assert a cause that is false, and prescribe a fix that cannot work: `48-VALIDATION.md` M1c ('runner environment: SIGPIPE before any test output captures … the fix is runner-side (capture step output, abort early on output-buffer close, or add `-v`)'); `48-SECURITY.md` § 'What this audit does NOT establish' (a) ('the go job's determinism -count=100 (P5a) guard exits 141 (SIGPIPE) on the runner at the test binary before any output captures'); and `ROADMAP.md` § Phase 48 'Blocked on' ('都需要源侧单独的修复（runner 端修复 / 测试 harness / Dockerfile 构建基础镜像）'). None of these holds: the SIGPIPE happens in the `go test -list` producer of the first pipeline, before the `-count=100` test binary is even started, and the fix is a one-line workflow edit — not a runner-side remedy. The three records also mis-state the phase's remaining work, which is why the recorded 'next host' path cannot land four-green."
    artifacts:
      - path: ".planning/phases/48-ci/48-VALIDATION.md"
        issue: "M1c row — cause and remedy both wrong (reproduced on WSL go1.25.5: 6/6 exit 141 with the exact ci.yml line, exit 0 without `grep -q`; the same pipeline exits 0 on this Windows host, which is why 48-12's local check missed it)."
      - path: ".planning/phases/48-ci/48-SECURITY.md"
        issue: "M1 bullet (a) — 'at the test binary' is wrong."
      - path: ".planning/ROADMAP.md"
        issue: "Phase 48 'Blocked on' — prescribes a runner-side fix for a workflow-side bug."
    missing:
      - "Reclassify M1c in all three records to the workflow pipeline shape, with the reproduction as evidence"
  - truth: "Every record that asserts a run or transport state is reconciled to the same outcome"
    status: partial
    reason: "The reconciliation is directionally right (no record still claims 'no run exists' as a current fact — the run-absent paragraph was rewritten) but three residues remain. (1) The run count is understated: `48-VALIDATION.md`, `48-SECURITY.md` and `STATE.md` line 130 all say the runner executed `ci.yml` 'three times' and list only runs 1–3, while four runs exist (run-4, id 39355131431, is the one that carries the final reconciliation commit; `STATE.md` line 149 and `48-15-SUMMARY.md` say four). (2) `STATE.md` line 117 still literally asserts the falsified cause — 'M1 = BLOCKED (no git transport to github.com from this host …)' — and is only annotated by a later append (line 129), so the standalone line still states a falsified absence. (3) The round-2 disposition ledger `48-REVIEW-DISPOSITION-R2.md` still shows all nine round-2 findings as `open` (including R2-CR-01, which 48-12 fixed and 48-14 recorded as closed in the security register), and the file is untracked in git, so it was never part of the committed record."
    artifacts:
      - path: ".planning/STATE.md"
        issue: "Line 117 asserts the falsified no-transport cause; line 130 says three runs where four exist."
      - path: ".planning/phases/48-ci/48-VALIDATION.md"
        issue: "Runner-absence paragraph and the M1/M1a/M1b/M1c rows name three runs; four exist."
      - path: ".planning/phases/48-ci/48-SECURITY.md"
        issue: "M1 bullet says 'three times' and lists runs 1–3 only."
      - path: ".planning/phases/48-ci/48-REVIEW-DISPOSITION-R2.md"
        issue: "Untracked; all nine round-2 rows still 'open' although 48-12/48-13 closed them."
    missing:
      - "Update the run count and run-id list to four in 48-VALIDATION.md, 48-SECURITY.md and STATE.md"
      - "Strike or annotate the falsified cause in STATE.md line 117 rather than leaving it standing"
      - "Bring the round-2 disposition ledger to the closed state (or fold it into the round-3 ledger) and commit it"
advisory:
  - finding: "The `go` job's linter install inherits GOTOOLCHAIN/GOSUMDB/GOPROXY from the runner image, so ci.yml's claim that 'no network fetch-and-execute surface remains anywhere in this file' is overstated (IN-01)"
    category: security
    reason: "A `go` directive above the local toolchain would make `go install` download and run a different toolchain. Safe to declare today: golangci-lint v2.12.2 needs go 1.25.0 and go.mod pins 1.25.5, so `GOTOOLCHAIN=local` is satisfiable without a download."
    evidence_status: "code read (ci.yml:225-231 — no env: block on the install step)"
  - finding: "The pinned linter version `v2.12.2` is duplicated in four places across three files (IN-02)"
    category: other
    reason: "A bump needs four coordinated edits; the runtime preflight and P3 both fail loudly, so it is a maintenance cost, not a correctness hole."
    evidence_status: "code read (ci.yml:90, ci.yml:229, verify-phase-48.sh:140, verify-phase-48.sh:404)"
  - finding: "The trust-surface rules are regex-scanned over comment-stripped text, so a quoted `\"paths\":` key and a bare `toJSON(secrets)` both pass (review WR-02)"
    category: security
    reason: "Both shapes are valid YAML that GitHub honours: a path filter written with a quoted key leaves a required check Pending forever, and `toJSON(secrets)` expands every repository secret. I confirmed both pass the gate with exit 0 / `ALL PASSED` in a sandbox copy. Drive both rules from the parsed document (as the permissions rule already does) rather than from text."
    evidence_status: "reproduced (sandbox copy + an added workflow file carrying each shape; gate exit 0 / ALL PASSED)"
  - finding: "`eval-gate.yml` still persists the runner token in the only job that holds the three live secrets (review WR-03)"
    category: security
    reason: "The phase's own IN-10 rule is applied in ci.yml for the two jobs that execute checked-out content, but the gate does not check it, so the one workflow that holds live secrets is the one where the rule is silently absent. `persist-credentials: false` is a two-line fix."
    evidence_status: "code read (eval-gate.yml:83 — `actions/checkout@v7` with no `with:`)"
  - finding: "P11's else-detector uses the same SIGPIPE-prone `sed | grep -q` idiom (review WR-01)"
    category: other
    reason: "Latent today (the printed range is one line) but active in exactly the scenario the check exists for — a banner moved down — where the check would silently stop detecting."
    evidence_status: "code read (verify-phase-48.sh:1312)"
  - finding: "P1's `pull_request_target` scan matches comment text, and `scan_trust_surface` reuses one log path for P8 and the P11 fixtures (IN-03, IN-04)"
    category: other
    reason: "False-positive risk and coupling only; no runtime defect."
    evidence_status: "code read (verify-phase-48.sh:209-212, :851)"
  - finding: "The `verify` check is self-attested on fork PRs (IN-05)"
    category: other
    reason: "Already stated plainly in the ci.yml header; recorded here only so branch-protection discussions do not count it as an assurance."
    evidence_status: "code read (ci.yml:199-203)"
---

# Phase 48: CI 矩阵 Verification Report (Re-verification, round 3)

**Phase Goal:** 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Verified:** 2026-10-20T11:05:00Z
**Status:** gaps_found
**Re-verification:** Yes — after gap-closure plans 48-12..48-15 (round 3)

---

## Goal Achievement

### What actually changed since round 2

Round 2's headline was "no `ci.yml` run has ever existed; `gh run list --workflow ci.yml` → HTTP 404". **That is no longer true, and the change is real, not a re-documentation.** This pass confirms, from the GitHub API and from the runner's own job logs:

- **Four runs of `ci.yml` exist on the public default branch**, all `event: push` to `main`:
  `39350901689` (head `d168512`), `39352480525` (head `62b6663`), `39353593172` (head `a2a4c86`), `39355131431` (head `304dcc4`).
- The **repository-owner history decision was recorded and executed**: `git ls-remote git@github.com:NDCCCCCC/skillforge.git` now returns `refs/heads/main = 304dcc4` and `refs/tags/v1.5-archive = 369b08a` — the old public tip is preserved as a tag, exactly as option a specified.
- The `verify` job — the phase's own gate — is **`success` in all four runs**, i.e. `verify-phase-48.sh` really executes on a Linux runner and really passes there (with the module-proxy linter install).
- Every round-2 Class-B defect that was fixable in the workflow files is now closed and was **re-proven by mutation in this pass** (table below).

**But the goal is not achieved.** The goal names a public outcome — 三线在 GitHub Actions 上公开绿灯 — and in all four runs the three published lines are **red**: `go=failure`, `fe=failure`, `docker=failure`. The honest terminal state is now "the gate is public, wired and biting on three real defects", not "the gate is green".

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 / CICD-01: the Go line runs on a public trigger and its published check is green | ✗ FAILED | 4 runs, `go=failure` every time. On run-4 the go job's own requirement gates (build+vet, `golangci-lint-action@v9`, `go test ./... -count=1`) all succeeded; the job died at the phase-added P5a guard with `exit code 141` and an empty log. Root cause is `ci.yml:105` — see "The M1c red is a workflow bug" below. |
| 2 | SC-2 / CICD-02: the FE line runs on a public trigger and its published check is green | ✗ FAILED | `fe=failure` every run. `npm ci` + `npx tsc --noEmit` green; `npx vitest run` = 1 of 689 tests failing: `TestSparklineNoDeps` → `ENOENT … 'D:/code/ClaudeCode/ai-agent/web/src/components/chat/parts/Sparkline.tsx'`. Cause in-repo: `web/src/components/chat/parts/Sparkline.test.tsx:80` hard-codes a Windows absolute path. |
| 3 | SC-3 / CICD-03: both Dockerfiles build in CI | ✗ FAILED | `docker=failure` every run at `build backend image`: alpine-musl linker → `undefined reference to 'backtrace' / 'backtrace_symbols' / 'malloc_trim' / '__res_init'` from `libduckdb_static.a`. The frontend image build is never reached, so half of CICD-03 is unproven in either direction. |
| 4 | A real GitHub Actions run of `ci.yml` exists on the public default branch through a gate-preserving trigger | ✓ VERIFIED | 4 runs, ids above, `event: push` to `main`; remote `main` = `304dcc4`; `v1.5-archive` = `369b08a` preserved. The 404 baseline of round 2 no longer reproduces. |
| 5 | The repository-owner history decision is recorded (option, date, evidence) and executed | ✓ VERIFIED | `48-15-SUMMARY.md` § Task 1: "History decision: Option a", 2026-10-20, with the pre-decision `ls-remote` / empty-merge-base evidence and the exact push sequence; the remote state proves the push landed. |
| 6 | CICD-01/02/03 advance only against a corroborated four-green run | ✓ VERIFIED | All three still `Pending` in `REQUIREMENTS.md` (checklist lines 17–19 and mapping table lines 69–71). `48-15` did not touch the file. The T-48-46 rule was honoured. |
| 7 | No gate was weakened to manufacture a run | ✓ VERIFIED | `git diff --stat 4a8e4c9 HEAD -- .github/workflows/` is **empty** (the candidate-A experiment was reverted inside the plan). No `continue-on-error`, no `paths:`/`paths-ignore:`, no `pull_request_target`, no `|| true` in `ci.yml`; `push.branches: [main]` and `pull_request.branches: [main, 'phase-*']` unchanged. |
| 8 | WR-06 (push-path lint scope) is decided on run evidence | ✓ VERIFIED | run-1 reproduced the whole-repo fallback (force-push, no common ancestor, 11+ findings); run-2 tested candidate A and the action rejected it ("please, don't specify manually --new* args when requesting only new issues"); run-3 (fast-forward) had the lint step green with `only-new-issues: true` alone. Both ledgers updated to `fixed — closed-by-evidence on run-3`. |
| 9 | The gate's own bootstrap opens no unrecorded supply-chain surface; register == code | ✓ VERIFIED | `ci.yml:225-231` installs through the module proxy with an `[ -x ]` postcondition; the shared P2/P3 probe pins the install shape; register row **T-48-47** exists (21 rows, `threats_open: 0`, `status: verified`). |
| 10 | The live gate grants secrets only to steps that consume them | ✓ VERIFIED | `eval-gate.yml:96-99` — the three `secrets.*` mappings occur exactly once, on the frozen-replay step; `Gate check` carries no `env:`. Re-adding the block reds P9 (drill D4). |
| 11 | The workflow-side Class-B closure (four-job header, credential-free verify checkout, pinned PyYAML, rename-proof `-list` guard) holds | ⚠️ PARTIAL | Every element is present and pinned (removing the `-list` guard reds P2 — drill D5; re-adding a fetch/script install reds P3). **But the rename-proof guard itself is the defect**: its `grep -q`-under-`pipefail` shape makes the go job deterministically red on every Linux runner. 48-12 fixed a vacuity by introducing a functional regression that only a real runner could expose. |
| 12 | The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20) | ✗ FAILED | M8/M9/M10 closed and re-proven. But neutering or deleting a probe in 8 of 11 partitions still prints `ALL PASSED` (three mutations reproduced below), and a job-level `if:` on the `fe`/`docker` jobs is not rejected. |
| 13 | SC-4: the live eval-gate never fakes green; fork/Dependabot boundary, secrets and the single Go-version source hold | ✓ VERIFIED (regression) | Complement-notice equality, exact gate expression, secret count and `go-version:`-free directory all still enforced (mutations D4/D7 red). |
| 14 | SC-5: `bash verify-phase-48.sh` exits 0 | ✓ VERIFIED | exit 0 with a single `ALL PASSED` on this host, and the same script is the `verify` job — `success` in all four public runs. |
| 15 | Every record that asserts a run or transport state is reconciled to the same outcome | ⚠️ PARTIAL | Directionally reconciled, but three residues: a false root cause in three records, a run count of 3 where 4 exist, and a stale untracked round-2 ledger. |

**Score:** 9/15 truths verified (0 present-but-behavior-unverified, 2 partial, 4 failed)

---

## The M1c red is a workflow bug, not a runner-environment quirk

`48-VALIDATION.md` M1c, `48-SECURITY.md`'s M1 bullet (a) and `ROADMAP.md`'s Phase-48 "Blocked on" all classify the go job's exit-141 as a **runner environment** artefact and prescribe runner-side remedies ("capture step output, abort early on output-buffer close, or add `-v`"; "runner 端修复"). **That classification is wrong, and the review's CR-01 is right.** I reproduced the claim independently, on WSL Ubuntu 22.04 with `go1.25.5 linux/amd64` — the exact Go version `go-version-file: go.mod` installs on the runner — using the repository's real package and the exact `ci.yml` line:

```
--- A: current ci.yml P5a line 1 under pipefail (real package) ---
exit=141  log_bytes=0
--- B: same pipeline without grep -q ---
exit=0
--- C: no pipe at all (materialise to file) ---
go_test_list_exit=0 grep_exit=0
--- D: 6 iterations of the failing shape ---
iter=1..6 exit=141  (6/6)
--- E: second command alone ---
ok  github.com/skillforge/skillforge/internal/knowledge  0.148s   exit=0
```

The mechanism: `grep -q` exits at the first match and closes the pipe; `go test -list` always writes a trailing `ok <pkg> <time>` line after the list; that write takes EPIPE → SIGPIPE → the `go` command exits 141 → `pipefail` makes the pipeline 141 → `bash -e` aborts the step. The runner fingerprint matches exactly — `Process completed with exit code 141`, empty step log, the second command never running (it would have printed an `ok` line). The same line exits **0 on this Windows host (3/3)**, which is precisely why 48-12's local check ("the guard passes on the real test and reds on a nonexistent name") missed it: the defect is platform-dependent, and only the first real runner could find it.

Consequences for the phase record:

1. **The go job can never be green with the step as written.** The fix is one line — `grep -x '…' >/dev/null` instead of `grep -qx '…'`, or materialise the list to `$RUNNER_TEMP` and grep the file. Both were verified exit 0 here. With that one line, the go job's own requirement gates (lint, build+vet, `go test ./...`) are already green on the runner, so the go line would go green on the next push.
2. **The recorded classification is false in three files** (`48-VALIDATION.md` M1c, `48-SECURITY.md` M1(a), `ROADMAP.md` "Blocked on"). A phase whose own deliverable is "the gate is trustworthy" cannot carry a false root cause in three places: the "runner 端修复" prescription in ROADMAP is precisely what would keep the go line red.
3. This is a **regression introduced by the gap-closure wave itself** (48-12, commit `05803d5`), i.e. the same "wave reintroduces the defect class it was fixing" pattern that round 2 recorded for R2-CR-01.

---

## Mutation Evidence (independent, sandbox copy — the repository was not modified)

Every mutation was applied to a temp copy of `verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md`, and each edit was assertion-checked (the mutated text was re-read from the sandbox) so that a no-op could not fake a result.

| # | Mutation | Target finding | Result | Verdict |
|---|----------|----------------|--------|---------|
| D0 | baseline (unmodified copy) | — | exit 0, one `ALL PASSED` | control |
| D1 | P9's probe invocation → `:` (token kept) | CR-02 | exit 0, `ALL PASSED` | **VACUOUS** |
| D2 | shared P2/P3 probe invocation → `:` | CR-02 | exit 0, `ALL PASSED` | **VACUOUS** |
| D3 | P9's probe invocation deleted entirely | CR-02 | exit 0, `ALL PASSED` | **VACUOUS** |
| D4 | P8's `scan_trust_surface` call commented out | round-2 M8 | exit 1, no banner | BITES — closed |
| D5 | job-level `if:` added to the `verify` job | round-2 M9 | exit 1, "the verify job carries a job-level if:" | BITES — closed |
| D6 | `verify` job deleted entirely | round-2 M10 | exit 1, "ci.yml has no 'verify' job" | BITES — closed |
| D7 | three-secret `env:` re-added to `Gate check` | R2-WR-05 | exit 1, P9 red | BITES — closed |
| D8 | `-list` existence guard removed from the P5a step | R2-WR-04 | exit 1, P2 red | BITES — closed |
| D9 | step-level `if: false` on the fe `unit tests` step | WR-01 | exit 1, P6 red | BITES — closed |
| D10 | **job-level `if:` added to the `fe` job** | new | exit 0, `ALL PASSED` | **VACUOUS** |
| D11 | **job-level `if:` added to the `docker` job** | new | exit 0, `ALL PASSED` | **VACUOUS** |
| D12 | extra workflow carrying `"paths":` (quoted key) | WR-02 | exit 0, `ALL PASSED` | **VACUOUS** (warning-grade) |
| D13 | extra workflow carrying `toJSON(secrets)` | WR-02 | exit 0, `ALL PASSED` | **VACUOUS** (warning-grade) |

Reading of the table: everything 48-12/48-13 claimed to fix does bite (D4–D9). What remains is (a) the general probe-neutering class (D1–D3) — the success token is hand-placed, so it certifies that a line was reached, not that a probe ran; (b) the job-level `if:` arm of the 48-07 rule, which 48-13 extended to the `verify` job but not to `fe`/`docker`; (c) the two text-scanning trust-surface evasion shapes.

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.github/workflows/ci.yml` | go/fe/docker + verify, four jobs, gate-preserving | ✓ VERIFIED (declaration) / ✗ defective (P5a step) | Triggers intact, no weakening token, verify job credential-free, module-proxy linter install with `[ -x ]` postcondition, PyYAML pinned, four-job header truthful. The P5a step's pipeline shape is a deterministic Linux red. |
| `.github/workflows/eval-gate.yml` | gated live workflow, complement notice, secrets on the consumer only | ✓ VERIFIED | Three live secrets only on the replay step; `Gate check` credential-free (D7 red on re-add). Residual: the runner token is still persisted in this workflow (WR-03, advisory). |
| `.github/actions/setup/action.yml` | checkout-free composite, three pinned majors | ✓ VERIFIED (regression) | Unchanged since round 1; every job checks out first. |
| `.golangci.yml` | v2 schema, no baseline, revive positively assured | ✓ VERIFIED | Unchanged. |
| `verify-phase-48.sh` | 11 partitions, zero-waiver, self-proving | ✓ VERIFIED (execution) / ✗ defective (accounting) | 11/11 on the healthy tree, exit 0, and green on the real runner in all four public runs. But the token accounting does not bind a partition to its probe (D1–D3), and the job-level `if:` arm is missing for fe/docker (D10/D11). |
| `web/src/components/chat/parts/Sparkline.test.tsx` | FE test that passes on a Linux runner | ✗ defective | Line 80 hard-codes `D:/code/ClaudeCode/ai-agent/web/src/components/chat/parts/Sparkline.tsx` — the sole cause of the fe red. |
| `Dockerfile.backend` | backend image that links against the static duckdb archive | ✗ defective | alpine/musl base lacks the glibc symbols the archive needs. |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` debt markers exist in any deliverable — the debt-marker gate does not fire.

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `ci.yml` push to `main` | GitHub Actions run | `on.push.branches: [main]` | WIRED (observed) | Four runs, event `push`, branch `main`. |
| `go` job | `golangci-lint-action@v9` | step | WIRED (observed green on run-3/run-4) | |
| `go` job | `go test ./... -count=1` | step | WIRED (observed green, 50+ packages `ok`) | |
| `go` job | P5a determinism guard | step | WIRED but **defective** | Observed red, exit 141, cause in this file. |
| `fe` job | `npm ci` / `tsc --noEmit` / `vitest run` | steps | WIRED (observed green, green, red) | |
| `docker` job | `docker/build-push-action@v7` ×2, `push: false` | steps | WIRED (observed red on the backend image) | Frontend image build never reached. |
| `verify` job | `verify-phase-48.sh` | last step | WIRED (observed green ×4) | The gate really runs on a public runner. |
| `verify` job | golangci-lint v2.12.2 | `go install …@v2.12.2` + `[ -x ]` | WIRED (observed green ×4) | The module-proxy install works on a cold runner. |
| run conclusions | `REQUIREMENTS.md` CICD statuses | the blocked-branch rule | WIRED | Three Pending statuses, no flip without a four-green run. |
| run evidence | WR-06 decision | both disposition ledgers | WIRED | `fixed — closed-by-evidence on run-3` in R1 (both rows) and in the round-2 pointer. |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase gate green on the final tree | `bash verify-phase-48.sh` (sandbox copy) | exit 0, 11/11, one `ALL PASSED` | ✓ PASS |
| Public run exists for this workflow | `gh run list --workflow ci.yml --limit 6` | 4 runs, all `failure` (2026-10-20T00:59Z–01:48Z) | ✓ runs exist / ✗ not green |
| Per-job conclusions | `gh api …/runs/{id}/jobs` | every run: `go=failure fe=failure docker=failure verify=success` | observed |
| go job's requirement gates on run-4 | runner log `0_go.txt` | build+vet, lint action and `go test ./...` all succeeded; P5a failed with `exit code 141` | observed |
| fe job on run-4 | runner log `3_fe.txt` | `tsc --noEmit` clean; vitest 1/689 failed (`Sparkline.test.tsx:79` ENOENT on a `D:/…` path) | observed |
| docker job on run-4 | runner log `1_docker.txt` | backend build failed at link: `undefined reference to 'backtrace' / 'backtrace_symbols' / 'malloc_trim'` from `libduckdb_static.a` on alpine-musl | observed |
| P5a pipeline on Linux | WSL `go1.25.5` repro of the exact `ci.yml` line | exit 141, 6/6, empty log; exit 0 without `grep -q` | ✓ reproduced (defect confirmed) |
| P5a pipeline on this host | same pipeline under Git Bash | exit 0, 3/3 | platform divergence confirmed |
| Remote history after the decision | `git ls-remote git@github.com:NDCCCCCC/skillforge.git` | `refs/heads/main = 304dcc4`, `refs/tags/v1.5-archive = 369b08a` | ✓ PASS |
| No gate weakening in the published plan | `git diff --stat 4a8e4c9 HEAD -- .github/workflows/` | empty | ✓ PASS |
| Docker image build on this host | — | `docker` absent | ? SKIP (covered by the runner evidence instead) |

## Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| CICD-01 | 48-01, 03, 04, 05, 06, 07, 08, 09, 10, 11, 12, 13, 14, 15 | Go CI in GitHub Actions: golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet, PR/push triggered | ✗ BLOCKED (Pending) | The line runs on a public push and its three subject gates are green on the runner; the **published check is red** because of the phase's own P5a probe, whose cause is one line of `ci.yml`. |
| CICD-02 | 48-02, 04, 05, 07, 10, 11, 13, 15 | FE CI: `tsc --noEmit` + full `vitest run` | ✗ BLOCKED (Pending) | Runs on a public push; typecheck green, 1/689 tests red on a hard-coded Windows path in the repo. |
| CICD-03 | 48-02, 04, 05, 07, 10, 11, 13, 15 | Both Dockerfiles build in CI | ✗ BLOCKED (Pending) | Runs on a public push; backend image fails to link on alpine-musl, frontend image never attempted. |

**Cross-reference:** every plan in the phase (48-01 … 48-15) declares its `requirements:` in frontmatter, and the union of those declarations is exactly {CICD-01, CICD-02, CICD-03} — the same set `REQUIREMENTS.md` maps to Phase 48. **No orphaned requirement, no unmapped ID.**

**Flip discipline:** the three requirements remain `Pending` in both places they appear (checklist lines 17–19, mapping table lines 69–71). This is correct — the recipe's blocked-branch rule applies and no requirement was advanced on partial evidence. It also means the phase's requirements are honestly *not* met.

## Anti-Patterns Found

All round-3 review findings relevant to the deliverable files were checked against the code rather than accepted on narration; the two load-bearing ones (CR-01, CR-02) were reproduced by mutation.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `.github/workflows/ci.yml` | 105 | `go test … -list … \| grep -qx …` under `set -o pipefail` — `grep -q` closes the pipe, the producer takes SIGPIPE, exit 141 | 🛑 Blocker | The go job can never be green; the cause of the go red in runs 3 and 4, mis-classified as a runner-environment issue in three records |
| `verify-phase-48.sh` | 1079/228 etc. | `ok_partition` is a hand-placed line: a neutered or deleted probe still yields `ALL PASSED` (D1–D3) | 🛑 Blocker | The gate's own honesty claim (P23 / T-48-11 / T-48-20) is still not true for 8 of 11 partitions |
| `verify-phase-48.sh` | 688/764 (P6/P7) | job-level `if:` rejection exists for the `go` and `verify` jobs but not for `fe`/`docker` (D10/D11) | 🛑 Blocker | The fe/docker lines are skippable while the gate reports green — the exact class 48-13 closed for the verify job |
| `48-VALIDATION.md` / `48-SECURITY.md` / `ROADMAP.md` | M1c / M1(a) / Blocked-on | a false root cause and a remedy that cannot work ("runner-side fix") for a workflow-side bug | ⚠️ Warning | The phase's remaining-work statement is wrong; a developer following it would not land four-green |
| `48-VALIDATION.md` / `48-SECURITY.md` / `STATE.md` | runner-count paragraphs | "three runs" where four exist | ⚠️ Warning | Record undercount; the fourth run carries the final reconciliation commit |
| `.planning/STATE.md` | 117 | the falsified "no git transport to github.com from this host" cause still stands in the 48-11 entry (annotated only by a later append) | ⚠️ Warning | A standalone line in the project state file still asserts a falsified absence |
| `48-REVIEW-DISPOSITION-R2.md` | all rows | the round-2 ledger still shows all nine round-2 findings as `open` although 48-12/48-13 closed them; the file is untracked | ℹ️ Info | Ledger hygiene: the round-2 dispositions exist only in prose, not in the record |
| `verify-phase-48.sh` | 1312 | P11's else-detector uses the same SIGPIPE-prone idiom (latent) | ℹ️ Info | Review WR-01, not exercised today |
| `verify-phase-48.sh` | 945/1027-1044 | trust-surface rules are text-scanned: a quoted `"paths":` key and `toJSON(secrets)` both pass (D12/D13) | ℹ️ Info | Review WR-02, warning-grade for the trust boundary |

## Human Verification Required

None. This is an infrastructure phase with no user-facing surface, and every truth in it was decided
by evidence I could obtain directly (public run ids and job logs, WSL reproduction, sandbox
mutations). The one decision that remains is an owner decision, not a test: whether to accept the
three real reds (a one-line workflow fix, a test-path fix, a Dockerfile base-image change) as
follow-up work in a later plan, or to keep Phase 48 open until a four-green run exists. If the
developer chooses to defer deliberately, the honest instrument is an `overrides:` entry in this
file's frontmatter — no override is recorded here, so the four failed truths stay failed.

## Gaps Summary

**Round 3 moved the phase from "declared" to "executed", and that is the whole material change.**
Round 2 could not even fetch a run list; today four public runs of `ci.yml` exist against the
public default branch, the repository-history decision is recorded and executed (the old public tip
is preserved as `v1.5-archive`), the `verify` job — the phase's own gate — is green on a real
runner in every run, and every round-2 workflow-side defect is closed and re-proven by mutation
here. The score moves 5/9 → 9/15 (the denominator grew: the round-3 plans added six new
must-haves, of which four are verified and two partial).

**Four truths keep the phase at `gaps_found`, and they are all "the public line is red" or "the
gate can still lie":**

1. **The three published lines are red in all four runs** (SC-1/2/3 → CICD-01/02/03 stay
   `Pending`). Two of the three causes are genuine pre-existing repository defects that this
   phase's gate correctly caught — the FE test's hard-coded Windows path and the backend image's
   alpine/musl base. The third is a defect the phase itself introduced: 48-12's rename-proof
   `-list | grep -qx` guard is, on a Linux runner, a deterministic exit-141. The phase record
   mis-classifies that one as a runner-environment artefact in three files; I reproduced it on
   WSL with the runner's own Go version (6/6 exit 141 with the exact `ci.yml` line, exit 0
   without `grep -q`) and the same line exits 0 on this Windows host — which is exactly why the
   local check missed it. **One line fixes it**, and with it the go job's own requirement gates
   (lint, build+vet, `go test ./...`) are already green on the runner.
2. **The verifier's token accounting still does not prove that a probe ran** (truth 12). 48-13
   closed the three recorded vacua; it did not close the class. Deleting or neutering a probe in
   8 of 11 partitions still prints `ALL PASSED` with exit 0 (D1–D3), and the job-level `if:`
   arm of the 48-07 rule is missing for the `fe` and `docker` jobs (D10/D11) even though 48-13
   added it for the `verify` job. The mechanism is one line per partition: deposit the token
   only on the probe's rc, or give each partition a probe-owned evidence variable the way P8
   does.
3. **The records carry a false root cause and a stale run count** (truths 11 and 15). Three
   files prescribe a runner-side remedy for a workflow-side bug, four runs are recorded as three,
   and the round-2 disposition ledger is untracked and un-updated.

**Minimum path to `passed`:** (1) fix `ci.yml:105` (drop `-q` or materialise the list) — the go
line then goes green on the next push; (2) fix the Sparkline test's absolute path; (3) change
`Dockerfile.backend`'s base to a glibc image; (4) re-push `main` and capture one run with
`go=fe=docker=verify=success`, then flip CICD-01/02/03 citing that run id; (5) route every
partition's success token through a probe-returned rc, and extend the job-level `if:` rejection
to `fe`/`docker`; (6) correct M1c's classification in `48-VALIDATION.md`, `48-SECURITY.md` and
`ROADMAP.md`, update the run counts to four, and close the round-2 ledger.

---

_Verified: 2026-10-20T11:05:00Z_
_Verifier: Claude (gsd-verifier)_
