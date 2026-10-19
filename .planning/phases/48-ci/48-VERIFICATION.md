---
phase: 48-ci
verified: 2026-10-19T16:13:49Z
status: gaps_found
score: 5/9 must-haves verified
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
  - cmd/eval/main.go
  - verify-phase-48.sh
covered_digest: "v3:sha256:c2045553b8bc85752c01d9e67ceb90dea992752b32b98ff9e66121efe895bb1d"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/9
  gaps_closed:
    - "SC-4 fake-green arms: WR-03 (replay pipefail) and WR-02 (notice complement) closed and mutation-proven here"
    - "Trust-surface directory-wide: CR-01 (per-file path/secret rules), WR-05 (*.yaml enumeration), IN-02 (case-insensitive registry tokens) closed and mutation-proven here"
    - "Round-1 vacua WR-04 (positive revive sentinel D), WR-01 (step-level if: rejection), WR-07 (P9 exact-expression) closed and mutation-proven here"
  gaps_remaining:
    - "SC-1 / CICD-01 — no ci.yml run exists (Class A)"
    - "SC-2 / CICD-02 — no ci.yml run exists (Class A)"
    - "SC-3 / CICD-03 — no ci.yml run exists (Class A)"
    - "Truth 6 residual — the verifier still folds probe-deletion and verify-job neutralisation into PASS (IN-06 closed only against its round-1 mutation shape)"
  regressions: []
  new_defects_from_gap_closure:
    - "R2-CR-01: the 48-11 verify job installs golangci-lint via `curl | sh` from a mutable HEAD ref, no checksum — and the supply-chain threat_flag 48-11-SUMMARY routed to 48-10 never landed in 48-SECURITY.md"
    - "R2-WR-01: the verify job is invisible to every guard (deletion or job-level if: keeps the gate green — reproduced here)"
    - "R2-WR-03: begin-marker accounting counts bodies that started, not probes that ran (P8 probe deletion stays green — reproduced here)"
    - "R2-WR-02/04/05: curl exit dropped in the install pipeline; `-run` vacuity on test rename; `Gate check` granted three live secrets cmd/eval proves it cannot use"
gaps:
  - truth: "SC-1 / CICD-01: a PR or push triggers the Go CI line and it is green"
    status: failed
    reason: "No ci.yml run has ever existed — re-confirmed this pass: `gh run list --workflow ci.yml` returns HTTP 404. MATERIAL ENVIRONMENT CHANGE: `git ls-remote origin HEAD` now SUCCEEDS (exit 0, 369b08a) — the 'git transport unreachable' cause recorded in 48-11/ROADMAP no longer holds. What remains is recipe step 1 (origin/main is a single unrelated commit, so no PR can be opened) and actually pushing from a transport-capable host — this host now qualifies on transport."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "go job declaration complete and hardened (mutation-proven); its runtime conclusion has never been observed."
    missing:
      - "One real GitHub Actions run with go=success (and verify=success), obtained by executing the persisted M1 recipe — the history decision first, then push from this now-transport-capable host"
  - truth: "SC-2 / CICD-02: a PR or push triggers the FE CI line and it is green"
    status: failed
    reason: "Same Class-A absence: no run exists. The fe job's commands reproduced green locally in round 1; the runner conclusion is still unobserved."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "fe job declared; never executed on a runner."
    missing:
      - "A real GitHub Actions run with fe=success"
  - truth: "SC-3 / CICD-03: both Dockerfiles build successfully in CI"
    status: failed
    reason: "Same Class-A absence: no run exists; docker is still absent from this host, so no image was ever built anywhere."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "docker job structurally correct (two build steps, push: false, per-image scopes); never executed."
    missing:
      - "A real GitHub Actions run with docker=success"
  - truth: "The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20)"
    status: failed
    reason: "Three of four round-1 vacua are closed and were re-proven here by mutation (revive deletion now reds P4 via sentinel D; step-level if: reds P2/P3; || true reds P9). But the fixed code still folds two neutralisations into PASS, both reproduced by this verification with verified edits: (a) commenting out P8's scan_trust_surface call while keeping begin_partition/done_partition prints 'PASS P8 trust-surface (0 workflow(s) ...)' and ALL PASSED — the begin-marker accounting binds a body that started, not a probe that ran (R2-WR-03); (b) deleting the verify job entirely, or adding a job-level if: to it, leaves ALL PASSED — no probe reads jobs[\"verify\"] (R2-WR-01), so the IN-07 fix is itself unpinned."
    artifacts:
      - path: "verify-phase-48.sh"
        issue: "done_partition requires only a begin marker (lines 109-117); P8's message interpolates ${TRUST_SURFACE_COUNT:-0} (line 939) absorbing the unset probe; P2/P3 load jobs[\"go\"] only (line 208), P6/P7 likewise — no partition pins the verify job's existence, if:-freeness, or last step."
    missing:
      - "Each probe's success deposits a $WORK/ran/<P>.ok token and done_partition requires begin+ok; drop the :-0 default so a deleted probe fails under set -u"
      - "Extend P2/P3 with the verify-job contract: job exists, no job-level if:, no step-level if:, last step is bash verify-phase-48.sh"
  - truth: "The gate's own bootstrap does not open an unrecorded supply-chain surface (R2-CR-01)"
    status: failed
    reason: "The 48-11 verify job runs `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b \"$LINT_BIN\" v2.12.2` (ci.yml line 203): the bootstrap script executes verbatim from a mutable HEAD ref and the tarball is not checksum-verified, inside the very job whose purpose is proving the gate cannot be forged — with the runner token persisted (no persist-credentials: false). 48-11-SUMMARY carries `threat_flag: supply-chain` for exactly this line, routed to 48-10's security pass; 48-SECURITY.md contains zero supply-chain/curl/install.sh rows and still says threats_open: 0 — the register is silent on a surface its own plan identified."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "Line 203: curl|sh from HEAD, no checksum, no postcondition."
      - path: ".planning/phases/48-ci/48-SECURITY.md"
        issue: "No supply-chain row; threats_open: 0 contradicts 48-11-SUMMARY's own threat_flag."
    missing:
      - "Install via the Go module proxy (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`, checksum-DB verified) or pin install.sh to a full commit SHA and verify the binary against the release checksums.txt; add `[ -x ... ]` postcondition and set -o pipefail (also closes R2-WR-02)"
      - "Re-record the supply-chain row in 48-SECURITY.md so the register matches the code"
  - truth: "The live gate grants secrets only to steps that consume them (R2-WR-05, warning-grade)"
    status: partial
    reason: "eval-gate.yml's `Gate check` step carries GLM_API_KEY / BGE_M3_URL / MILVUS_ADDRESS, but cmd/eval short-circuits --gate-check BEFORE any bootstrap ('gating reads a file, so it must not require GLM_API_KEY / Milvus / BGE-M3', cmd/eval/main.go:462-464) — the code contradicts WR-08's recorded consuming-steps premise. Not a fake-green path; a secret-surface widening."
    artifacts:
      - path: ".github/workflows/eval-gate.yml"
        issue: "Lines 113-117: step-level env grants three live secrets to an offline threshold comparison."
    missing:
      - "Delete the env: block from Gate check; keep the three secrets on the replay step only"
advisory:
  - finding: "ci.yml header still says 'Three independent jobs' while the file ships four (R2-IN-01)"
    category: other
    reason: "Internal comment contradiction introduced by 48-11; one-line fix. No runtime defect."
    evidence_status: "code read (ci.yml:29-31 vs :169-185)"
  - finding: "A fork PR's green verify check is self-attested and the job persists the runner token; the 'strictly narrower surface' header omits this (R2-IN-02)"
    category: security
    reason: "Consistent with IN-10's recorded scoping decision (go job only), but the header claim should acknowledge the fork-PR caveat and take the free persist-credentials: false hardening."
    evidence_status: "code read (ci.yml:182-183, 192)"
  - finding: "PyYAML installed unpinned into the gate job (R2-IN-03)"
    category: security
    reason: "Unreviewed new releases reach the gate; pin to the locally validated version."
    evidence_status: "code read (ci.yml:206-207)"
  - finding: "Determinism step passes vacuously if the pinned test is renamed (R2-WR-04)"
    category: other
    reason: "`go test -run` with zero matches exits 0 — demonstrated here (`ok ... [no tests to run]`, rc=0). Test exists today; the defect is future drift. Fold the `-list | grep -qx` probe into the P5a step when closing the Class-B wave."
    evidence_status: "reproducible command output recorded in Behavioral Spot-Checks"
deferred: []
---

# Phase 48: CI 矩阵 Verification Report (Re-verification)

**Phase Goal:** 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Verified:** 2026-10-19T16:13:49Z
**Status:** gaps_found
**Re-verification:** Yes — after gap-closure plans 48-05..48-11

---

## Goal Achievement

### Observable Truths

Must-haves carried from the round-1 report (Step 0). Failed items received full three-level re-verification; passed items received quick regression checks. Every closure claim was tested by **independent mutation in a sandbox copy** — round 1 proved self-verified probes can be falsified, so no SUMMARY claim was accepted on narration.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 / CICD-01: Go CI triggered and green | ✗ FAILED | `gh run list --workflow ci.yml` → `HTTP 404` (re-run this pass). The go job is complete, hermetic, and mutation-hardened — but no run of it has ever existed. **Transport to github.com is restored** (`git ls-remote origin HEAD` → `369b08a`, exit 0) — see Environment. |
| 2 | SC-2 / CICD-02: FE CI triggered and green | ✗ FAILED | Same Class-A absence. fe job declaration unchanged and locally reproducible (round 1: tsc rc=0, vitest 85 files/689 tests rc=0); no runner observation. |
| 3 | SC-3 / CICD-03: Docker builds in CI | ✗ FAILED | Same Class-A absence. `docker` still absent from this host; no image built anywhere; no run. |
| 4 | SC-4: live gate never fakes green | ✓ VERIFIED | Both round-1 fake-green arms closed in `eval-gate.yml`: replay step now carries `shell: bash` + `set -o pipefail` (lines 102-105); the skip notice is the verbatim negation of the gate (lines 149-152). Mutation-proven here: appending `\|\| true` to the gate → P9 red; reverting the notice to the old narrower form → P9 red. Residual secret-minimization defect (R2-WR-05) recorded as a gap item — not a fake-green path. |
| 5 | SC-5: `bash verify-phase-48.sh` exits 0 | ✓ VERIFIED | Re-ran on the final tree: rc=0, 11/11 partitions, single `ALL PASSED`. |
| 6 | Verifier refuses vacuous passes (P23/T-48-11/T-48-20) | ✗ FAILED | 3 of 4 round-1 vacua closed and re-proven (M4/M5/M6 bite). But two new neutralisations pass, reproduced with verified edits: P8 probe deleted, markers kept → `PASS P8 trust-surface (0 workflow(s) ...)` + ALL PASSED; verify job deleted or given a job-level `if:` → ALL PASSED. The green still means less than it claims. |
| 7 | Trust-surface is directory-wide as claimed | ✓ VERIFIED | Per-file `paths:`/`secrets.` rules, `*.yml`+`*.yaml` enumeration, and case-insensitive registry tokens all present in `scan_trust_surface` and mutation-proven here: second `.yml` with `paths:` → red; `.yaml` with `packages: write` → red; lowercase `docker push ghcr.io/...` → red. |
| 8 | Fork/Dependabot gate declared; secrets intact; one Go version source | ✓ VERIFIED (regression) | Gate + complement notice present; all six secret-mapping references intact in eval-gate.yml; zero `go-version:` literals under `.github/`; `go-version-file: go.mod` on every setup-go. |
| 9 | No gate weakened to manufacture a run | ✓ VERIFIED (regression) | Fresh greps: zero `continue-on-error`, `\|\| true`, `workflow_dispatch`, `paths:`/`paths-ignore:`, `pull_request_target`, or any `secrets.*` in `ci.yml`; `push.branches: [main]` intact. |

**Score:** 5/9 truths verified (0 present, behavior-unverified)

**Honest weighing of the goal without M1 (as asked):** the phase goal names a public outcome — 三线在 GitHub Actions 上公开绿灯. Static + sentinel proof, however strong, cannot be that outcome; a `ci.yml` that has never executed is a declaration, and this verification confirms the declaration is now excellent. Class A remains a failed must-have. What changed materially this pass: **the environment block is half-lifted** — git transport to github.com works from this host as of this verification. The remaining obstacle is the history decision (recipe step 1, a repository-owner decision) plus execution. The honest terminal state is therefore no longer "cannot be produced from this host" but "one owner decision away from obtainable". An override (`must_have: "SC-1/2/3 … green"`, reason: M1 environment block + persisted recipe) remains available to the developer if the run is deliberately deferred again — none is recorded in this file's frontmatter, so the three truths stay FAILED.

### Decision Coverage

All trackable CONTEXT.md decisions are honored by shipped artifacts. (gsd_run check.decision-coverage-verify: 30/30 honored, 0 not honored.)

---

## Re-verification: Mutation Evidence

All mutations ran in a sandbox copy (`verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md` under mktemp); no repository file was modified. Edits for the vacuous results were assertion-checked (applied-text grep / YAML re-parse) so that a no-op edit could not fake a vacuity — two of my own first attempts were discarded for exactly that reason (a comment-only "revive" removal; a regex that matched nothing).

| # | Mutation | Round-1/2 finding | Result | Verdict |
|---|----------|-------------------|--------|---------|
| M1 | Second `.yml` carrying `paths: ["internal/**"]` | R1 CR-01 | rc=1, P8 red | BITES — closed |
| M2 | Offending surface in a `.yaml` file (`packages: write`) | R1 WR-05 | rc=1, P8 red | BITES — closed |
| M3 | Lowercase `docker push ghcr.io/...` in a second `.yml` | R1 IN-02 | rc=1, P8 red | BITES — closed |
| M4 | Step-level `if: false` on the go job's lint step | R1 WR-01 | rc=1, P3 red | BITES — closed |
| M5 | Remove `enable: [revive]` + settings (config stays valid) | R1 WR-04 | rc=1, `P4 sentinel D: the run failed but did not name revive` | BITES — closed |
| M6 | Append `\|\| true` to the fork gate | R1 WR-07 | rc=1, P9 red | BITES — closed |
| M7 | Revert the notice to the old narrower (non-complement) form | R1 WR-02 | rc=1, P9 red | BITES — closed |
| M8 | Comment out P8's `scan_trust_surface` call, keep begin/done markers | R2 WR-03 | rc=0, `PASS P8 trust-surface (0 workflow(s) ...)`, ALL PASSED | VACUOUS — open |
| M9 | Add job-level `if: github.event_name == 'push'` to the verify job | R2 WR-01 | rc=0, ALL PASSED | VACUOUS — open |
| M10 | Delete the verify job entirely (jobs → docker/fe/go) | R2 WR-01 | rc=0, ALL PASSED | VACUOUS — open |

M8's output is the exact "found nothing, reported green" class this phase's own header calls non-negotiable — the PASS line prints the zero count and the banner prints ALL PASSED. M9/M10 mean the IN-07 fix (wire the verifier into CI) is not itself pinned by the verifier: the rot IN-07 was meant to prevent can return unnoticed.

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.github/workflows/ci.yml` | go/fe/docker matrix + (48-11) verify job | ✓ VERIFIED (declaration) / ✗ defective (verify job surface) | Four jobs, checkout-first, no weakening tokens, no secrets. The verify job's `curl \| sh` bootstrap (R2-CR-01) and its invisibility to the guards (R2-WR-01) are open defects. |
| `.github/workflows/eval-gate.yml` | Gated live workflow, complement notice, pipefail replay | ✓ VERIFIED (declaration) | WR-02/WR-03 closed and mutation-proven. Residual: `Gate check` carries three live secrets it cannot consume (R2-WR-05, warning). |
| `.github/actions/setup/action.yml` | Checkout-free composite, three pinned majors | ✓ VERIFIED (regression) | Unchanged since round 1's verification. |
| `.golangci.yml` | v2 schema, no baseline/allowlist, revive positively assured | ✓ VERIFIED | Sentinel D mutation-proves revive's presence (M5). |
| `verify-phase-48.sh` | 11 partitions, zero-waiver, self-proving | ✓ VERIFIED (execution) / ✗ defective (two vacua) | rc=0, 11/11 on the final tree. M8/M9/M10 show two neutralisation classes still pass. No debt markers in any deliverable. |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers exist in any deliverable — the debt-marker gate does not fire.

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| Every `ci.yml` job incl. `verify` | `./.github/actions/setup` | step after `actions/checkout@v7` | WIRED | P5 iterates all jobs; verified by read. |
| `verify` job | `verify-phase-48.sh` | `run: bash verify-phase-48.sh` | WIRED (declaration) | The link exists in YAML but is unpinned by the verifier (M10: deleting it stays green). |
| `verify` job | golangci-lint v2.12.2 | `curl .../HEAD/install.sh \| sh` | WIRED, ✗ unverified supply chain | Mutable ref, no checksum (R2-CR-01). |
| `ci.yml` lint step | `.golangci.yml` | action default config discovery | WIRED | Unchanged. |
| `eval-gate.yml` replay step | `./bin/eval` exit code | `shell: bash` + `set -o pipefail` | WIRED | WR-03 fix confirmed in YAML; P9 does not pin `shell:`/`pipefail` presence — accepted, the semantics are bash-documented. |
| Any CI workflow | `verify-phase-48.sh` | `verify` job | WIRED (since 48-11) | IN-07 resolved at the declaration level; unpinned at the verifier level (truth 6). |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase gate green on final tree | `bash verify-phase-48.sh` | rc=0; 11/11 PASS; single `ALL PASSED` | ✓ PASS |
| CI run exists for this commit | `gh run list --workflow ci.yml` | `HTTP 404: workflow ci.yml not found on the default branch` | ✗ FAIL (Class A) |
| Git transport to github.com | `git ls-remote origin HEAD` | `369b08a6babf0cc97c7d5b6db3a3c89a2012b9de`, exit 0 | ✓ PASS — **restored since 48-11 recorded it blocked** |
| `-run` zero-match semantics (R2-WR-04) | `go test ./internal/knowledge/ -run '^TestNoSuchTestExists$'` | `ok ... [no tests to run]`, rc=0 | ✓ demonstrated (defect class real) |
| Docker image builds | — | `docker` absent from host | ? SKIP |
| FE typecheck + tests | (round 1: tsc rc=0; vitest 85/689 rc=0) | not re-run — fe job text unchanged; the failing element is the run, not the commands | — |

### Probe Execution

No conventional `scripts/*/tests/probe-*.sh` probes exist for this phase. The phase's own probe is `verify-phase-48.sh`, executed above (rc=0), then mutation-tested ten ways (table above).

---

## Environment Facts (changed since round 1)

| Claim | Round-1 verdict | This pass | Evidence |
|-------|-----------------|-----------|----------|
| git transport to github.com unavailable | ✓ confirmed (TLS/connect failures) | **✗ NO LONGER TRUE** | `git ls-remote origin HEAD` → exit 0, `369b08a` |
| `origin/main` unrelated to this branch | ✓ confirmed | ✓ still true | `origin/main` = 1 commit (`369b08a`); no PR can be opened between unrelated histories — recipe step 1 (history decision) still stands |
| `ci.yml` never on remote default branch | ✓ confirmed (404) | ✓ still true | `gh run list --workflow ci.yml` → HTTP 404; latest run of any workflow remains eval-gate on main, 2026-10-02 |
| Docker absent | ✓ confirmed | ✓ still true | no `docker`/`docker.exe` on PATH |
| Branch protection off (Option A) | ✓ confirmed | not re-checked (unchanged surface) | 48-04-SUMMARY / AR-48-01 |

Consequence: `48-11-SUMMARY.md`'s M1 record and ROADMAP's "git 传输到 github.com 不通" line are now partially stale. M1 is one owner decision + one push away, not environment-impossible. No run was produced by the phase, so CICD-01/02/03 stay Pending in `REQUIREMENTS.md` — correctly.

## Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| CICD-01 | 48-01, 03, 04, 05, 06, 07, 08, 09, 10, 11 | Go CI in GitHub Actions, PR/push triggered | ✗ BLOCKED (Pending) | Declaration hardened and mutation-proven; no run exists (404). |
| CICD-02 | 48-02, 04, 05, 07, 10, 11 | FE CI: `tsc --noEmit` + full `vitest run` | ✗ BLOCKED (Pending) | Same. |
| CICD-03 | 48-02 | Docker build verification in CI | ✗ BLOCKED (Pending) | Same; no image ever built. |

**Orphaned requirements:** none — every ID in `REQUIREMENTS.md`'s Phase 48 row is claimed by plan frontmatter, and no plan claims an unmapped ID.

## Anti-Patterns Found

All nine round-2 review findings were independently confirmed against the code before classification; the two blocker-grade ones were additionally reproduced by mutation (M8/M9/M10). Files were all git-modified after the round-1 `verified:` timestamp (15 commits, `e112488`..`2387b61`), so these are gap-closure-wave introductions — in-contract, blocking, with deterministic evidence.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `.github/workflows/ci.yml` | 203 | `curl \| sh` from mutable HEAD, no checksum; supply-chain threat dropped from 48-SECURITY.md | 🛑 Blocker | Arbitrary-code-execution surface inside the job that certifies the gate; register says `threats_open: 0` |
| `verify-phase-48.sh` / `ci.yml` | 208/580/654; 185-210 | verify job invisible to every guard — deletion or `if:` stays green (M9/M10) | 🛑 Blocker | The IN-07 fix can silently rot |
| `verify-phase-48.sh` | 109-117, 939 | Begin markers prove a body started, not a probe ran; `:-0` absorbs the unset probe (M8) | 🛑 Blocker | `PASS P8 (0 workflow(s))` + ALL PASSED with the probe deleted |
| `.github/workflows/ci.yml` | 200-204 | Install pipeline drops curl's exit status (no pipefail) | ⚠️ Warning | Failure surfaces 3 steps later, misattributed to the gate preflight |
| `.github/workflows/ci.yml` | 95-96 | `-run` vacuity if the pinned test is renamed | ⚠️ Warning | Future silent-drift class; demonstrated rc=0 on zero matches |
| `.github/workflows/eval-gate.yml` | 113-117 | `Gate check` granted 3 live secrets it provably cannot use | ⚠️ Warning | Secret-surface widening vs WR-08's recorded premise |
| `.github/workflows/ci.yml` | 29-31 | Header says "Three independent jobs"; file ships four | ℹ️ Info | Stale comment |
| `.github/workflows/ci.yml` | 182-192 | Fork-PR verify green is self-attested; token persisted; header omits it | ℹ️ Info | Consistent with IN-10 decision, unacknowledged |
| `.github/workflows/ci.yml` | 206-207 | PyYAML unpinned | ℹ️ Info | Unreviewed releases reach the gate |

## Human Verification Required

N/A — Infrastructure/foundation phase (CI) with no user-facing elements. No truth is left ⚠️ PRESENT_BEHAVIOR_UNVERIFIED: the two statically decidable truth classes were decided (SC-4, trust-surface — mutation-proven), and the runner-dependent outcome is captured as the FAILED Class-A truths, not as a behavioral gap. The first real run, when obtained, is the M1 acceptance act per the persisted recipe.

## Gaps Summary

The gap-closure wave did most of what round 1 asked, and this verification confirms it the hard way: seven of the eight round-1 defect classes were re-broken by mutation here and the gate went red every time (M1-M7). The trust surface is genuinely directory-wide now; the fork gate and its notice are a proven complement pair; the linter has positive teeth. That is real, verified progress — round 1's 3/9 is now 5/9.

Two things keep the phase at `gaps_found`.

**Class A — the public green light still does not exist (SC-1/2/3, CICD-01/02/03).** No `ci.yml` run has ever existed; the 404 reproduces. The goal's outcome is objectively absent and cannot be certified by any static proof, however strong. What is new: **transport is restored** — `git ls-remote origin HEAD` now succeeds from this host, so the recorded "cannot be produced from this host" cause no longer holds. The remaining path is exactly the persisted M1 recipe: decide the history question (origin/main is a single unrelated commit, so no PR is possible — force-push/replace main, re-point the default branch, or fresh repo), push from this host, and require `go=fe=docker=verify=success`. That is a repository-owner decision plus one push — not an environment impossibility. If the developer chooses to defer the run deliberately again, the honest instrument is an override in this file's frontmatter, not a silent carry-over.

**Class B — the hardening wave reintroduced the defect it was fixing (truth 6, R2-CR-01).** 48-11 added a fourth `verify` job and 48-09 added begin-marker accounting; neither was carried across the seam. Deleting the verify job, adding an `if:` to it, or deleting P8's probe behind its marker all keep the gate at ALL PASSED — reproduced with verified edits. And the new job bootstraps its linter with `curl | sh` from a mutable HEAD ref inside the very job that certifies the gate, while the supply-chain threat flag 48-11-SUMMARY routed to 48-10 never landed in the security register — `threats_open: 0` is again silent on a surface the phase itself identified, exactly the internal-inconsistency class round 1 called out. Five secondary findings (curl exit, `-run` vacuity, Gate-check secrets, stale header, unpinned PyYAML) ride along and should be folded into the same closure pass.

**Minimum path to close:** (1) execute the M1 recipe from this now-transport-capable host — history decision, push, one run with all four jobs success — then flip CICD-01/02/03; (2) give every partition a success token that `done_partition` requires, and drop the `:-0` default; (3) pin the verify job in P2/P3 (exists, no `if:` anywhere, last step is the gate); (4) replace the curl|sh bootstrap with `go install ...@v2.12.2` (or SHA-pinned script + checksum verification) plus a `[ -x ]` postcondition and `set -o pipefail`; (5) re-record the supply-chain row in `48-SECURITY.md` and dispose the round-2 ledger rows; (6) take the five secondary fixes.

---

_Verified: 2026-10-19T16:13:49Z_
_Verifier: Claude (gsd-verifier)_
