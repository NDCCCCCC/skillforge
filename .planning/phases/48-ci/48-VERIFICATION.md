---
phase: 48-ci
verified: 2026-10-20T07:32:04Z
status: gaps_found
score: 13/15 must-haves verified
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
  - .planning/phases/48-ci/48-16-PLAN.md
  - .planning/phases/48-ci/48-16-SUMMARY.md
  - .planning/phases/48-ci/48-17-PLAN.md
  - .planning/phases/48-ci/48-17-SUMMARY.md
  - .planning/phases/48-ci/48-18-PLAN.md
  - .planning/phases/48-ci/48-18-SUMMARY.md
  - .planning/phases/48-ci/48-19-PLAN.md
  - .planning/phases/48-ci/48-19-SUMMARY.md
  - Dockerfile.backend
  - internal/duckdb/connector_test.go
  - verify-phase-48.sh
  - web/src/components/chat/parts/Sparkline.test.tsx
covered_digest: "v3:sha256:cae1f6ad274b729e08081b213a2ae0fde642c1ff72899583d761d65426eab4ff"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 9/15
  gaps_closed:
    - "SC-1 / CICD-01 — run 39380373930 (head 630f875, push to main) concludes go=success; every go step (build+vet, lint, go test ./..., P5a and P5b) is success"
    - "SC-2 / CICD-02 — the same run concludes fe=success; install, typecheck and unit tests are each success"
    - "SC-3 / CICD-03 — the same run concludes docker=success; both the backend and frontend image builds are success"
    - "The verifier's whole-probe mutation class — D1 (P9 probe → ':'), D2 (shared P2/P3 probe → ':'), D3 (P9 probe deleted), P6 probe neutered, P10 evidence neutered and the P11 fixture loop neutered all now red the gate; the fe/docker job-level if: mutations (D10/D11) also red"
    - "The determinism shape is pinned — reintroducing `go test -list | grep -qx` and dropping the materialised list reds P2"
    - "The P5a root cause is now recorded as the workflow pipeline shape in VALIDATION M1/M1c, SECURITY M1(a) and ROADMAP Blocked-on; the runner-side remedy is withdrawn in place"
    - "The run count is six everywhere the five reconciled records state it (REQUIREMENTS, ROADMAP, VALIDATION, SECURITY, STATE); STATE line 117's falsified no-transport cause is annotated"
    - "The round-2 disposition ledger is closed (all nine findings fixed, frontmatter open: 0) and is tracked in git"
    - "CICD-01/02/03 are Complete in both REQUIREMENTS.md places citing run 39380373930"
  gaps_remaining:
    - "The verifier still folds an assertion-level neutralisation into PASS: P2 and P3 share one STRUCT-OK token, so deleting P2's entire assertion block leaves the gate at exit 0 / ALL PASSED (reproduced)"
    - "48-REVIEW-DISPOSITION-R1.md still asserts the falsified 'CICD-01/02/03 remain Pending' / 'no ci.yml run can exist yet' state without an annotation"
  regressions: []
gaps:
  - truth: "The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20)"
    status: failed
    reason: "Round 3's three recorded vacua (D1-D3) and the fe/docker job-level if: arm (D10/D11) are genuinely closed and I re-proved each by mutation. The generic claim is still false at assertion granularity: P2 and P3 share one STRUCT-OK token from a single Python probe, so deleting P2's entire assertion block (the five go-job command fragments, the materialised-list pin, the CGO/hermeticity checks) leaves the probe printing STRUCT-OK and the gate exits 0 with one ALL PASSED. Reproduced in a sandbox copy: every P2 assertion string absent, exit 0, ALL PASSED. The round-4 review's WR-03 records the same residual for P1, P4 and the single-print P6/P7/P9 probes."
    artifacts:
      - path: "verify-phase-48.sh"
        issue: "ok_partition binds a partition to a probe (log exists, non-empty, contains token) but not to that partition's own assertions: P2/P3 share STRUCT-OK, so either partition's assertion set can be deleted while both deposit PASS."
    missing:
      - "Emit one distinct token per assertion group (for example P2-CMDS-OK at the end of the P2 block and P3-LINT-OK at the end of the P3 block) and require each at deposit time; apply the same split to P1, P4 and the single-print P6/P7/P9 probes."
  - truth: "Every record that asserts a run or transport state is reconciled to the same outcome"
    status: partial
    reason: "The five records named by 48-19 (REQUIREMENTS, ROADMAP, VALIDATION, SECURITY, STATE) are reconciled: run count 6 everywhere, M1c reclassified as workflow pipeline shape, STATE line 117 annotated as falsified, CICD-01/02/03 Complete on run 39380373930. One tracked record still asserts the superseded state without annotation: 48-REVIEW-DISPOSITION-R1.md's closing paragraph says 'CICD-01/02/03 remain Pending', 'git transport to github.com is unreachable from this host' and 'no ci.yml run can exist yet'. Both are falsified by six public runs and four-green run 39380373930. 48-15-PLAN's own done criterion named 'both disposition ledgers' as in scope."
    artifacts:
      - path: ".planning/phases/48-ci/48-REVIEW-DISPOSITION-R1.md"
        issue: "Tail paragraph after the disposition table asserts the falsified no-transport/no-run state and 'remain Pending' with no correction note; the file is tracked."
    missing:
      - "Annotate the R1 ledger's closing paragraph as historical (transport succeeded, six runs exist, run 39380373930 is four-green, CICD-01/02/03 Complete), matching the convention the R2 ledger uses to retain history."
advisory:
  - finding: "The P11 else-detector still uses the SIGPIPE-prone `sed -n ... | grep -qE` idiom"
    category: other
    reason: "Latent today because the printed range is one line, but active in exactly the scenario the check exists for (a banner moved down), where it could silently stop detecting."
    evidence_status: "code read (verify-phase-48.sh:1409)"
  - finding: "The trust-surface path-filter and secret rules are text scans and accept valid-YAML evasion shapes (quoted `\"paths\":`, `toJSON(secrets)`)"
    category: security
    reason: "Both shapes are honoured by GitHub; round 3 reproduced each as passing the gate. Drive the rules from the parsed document (as the permissions rule already does)."
    evidence_status: "reproduced (round 3, D12/D13); re-raised by 48-REVIEW.md WR-02"
  - finding: "The go job's linter install inherits GOTOOLCHAIN/GOSUMDB/GOPROXY from the runner image, so ci.yml's 'no network fetch-and-execute surface' claim is overstated"
    category: security
    reason: "Safe to declare today: golangci-lint v2.12.2 needs go 1.25.0 and go.mod pins 1.25.5, so GOTOOLCHAIN=local is satisfiable without a download."
    evidence_status: "code read (ci.yml:230-236 — no env: block on the install step)"
  - finding: "The pinned linter version v2.12.2 is duplicated across ci.yml and verify-phase-48.sh"
    category: other
    reason: "A bump needs coordinated edits; the preflight and P3 fail loudly, so this is maintenance cost, not a correctness hole."
    evidence_status: "code read"
---

# Phase 48: CI 矩阵 Verification Report (Re-verification, round 4)

**Phase Goal:** 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Verified:** 2026-10-20T07:32:04Z
**Status:** gaps_found
**Re-verification:** Yes — after gap-closure plans 48-16..48-19 (round 4)

---

## Goal Achievement

### The public outcome is now real

Round 3 closed with all three published lines red. Round 4 fixed the three source causes and produced the run:

- **Run `39380373930`** (`head 630f875`, event `push`, branch `main`, 2026-10-20T06:26:30Z–06:29:55Z) concludes **`success`** at the workflow level. Per job: **`go=success`, `fe=success`, `docker=success`, `verify=success`**.
- Every gate step inside those jobs is `success`, not skipped: go `build + vet`, `lint (changed lines)`, `go test ./...`, `determinism -count=100 (P5a)`, `determinism -shuffle=on with seed (P5b)`; fe `install`, `typecheck`, `unit tests`; docker `build backend image` and `build frontend image`; verify `phase gate self-proof`.
- The public default branch tip is that same commit: `gh api …/git/ref/heads/main` → `630f8751ab65a07d71d5fa6f25fd8e13c3838ef7`. No non-`.planning` commit exists after `630f875`, so the green run covers the current implementation tree.
- Six `ci.yml` runs exist in total (runs 1–5 red, run 6 green), matching the count recorded in every reconciled record.
- CICD-01/02/03 are `Complete` in both `REQUIREMENTS.md` places, each citing `run 39380373930`.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 / CICD-01: the Go line runs on a public trigger and its published check is green | ✓ VERIFIED | Run 39380373930: `go=success`; every go step success (build+vet, lint, `go test ./... -count=1`, P5a, P5b). |
| 2 | SC-2 / CICD-02: the FE line runs on a public trigger and its published check is green | ✓ VERIFIED | Same run: `fe=success`; `npm ci`, `npx tsc --noEmit`, `npx vitest run` all success. |
| 3 | SC-3 / CICD-03: both Dockerfiles build in CI | ✓ VERIFIED | Same run: `docker=success`; `build backend image` and `build frontend image` both success. |
| 4 | A real GitHub Actions run of `ci.yml` exists on the public default branch through a gate-preserving trigger | ✓ VERIFIED | Six runs, event `push` to `main`; remote `main` = the green run's head `630f875`; `v1.5-archive` = `369b08a` preserved. |
| 5 | The repository-owner history decision is recorded and executed | ✓ VERIFIED | `48-15-SUMMARY.md` records option a (2026-10-20) with pre-decision evidence; `gh api` confirms the new `main` tip and the archive tag. |
| 6 | CICD-01/02/03 advance only against a corroborated four-green run | ✓ VERIFIED | Both REQUIREMENTS.md places are `Complete (run 39380373930)`; that run's four jobs are independently confirmed `success`. |
| 7 | No gate was weakened to manufacture the run | ✓ VERIFIED | `git diff 4a8e4c9..HEAD -- .github/workflows/` touches only the determinism step (pipe → materialised file) and one comment. No `continue-on-error`, no `paths:`/`paths-ignore:`, no `pull_request_target`, no `|| true`, no job/step `if:`, no secrets in `ci.yml`. |
| 8 | WR-06 (push-path lint scope) is decided on run evidence | ✓ VERIFIED | Both ledgers record `fixed — closed-by-evidence on run-3` (fast-forward push, `only-new-issues: true` alone, zero findings); run-3 id/head match. |
| 9 | The gate's bootstrap opens no unrecorded supply-chain surface; register == code | ✓ VERIFIED | `ci.yml:230-236` installs golangci-lint v2.12.2 through the module proxy with an `[ -x ]` postcondition; P2/P3 pin the install shape and the token binding (mutations red). |
| 10 | The live gate grants secrets only to the step that consumes them | ✓ VERIFIED | P9 passes; re-adding the three-secret `env:` block to `Gate check` reds P9 (reproduced). |
| 11 | The workflow-side Class-B closure holds, including the rename-proof determinism guard | ✓ VERIFIED | Four-job header, credential-free verify checkout, pinned PyYAML and the materialised-list guard are present; reintroducing `go test -list \| grep -qx` reds P2 (reproduced). |
| 12 | The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20) | ✗ FAILED | Whole-probe mutations now bite, but deleting P2's entire assertion block while P3 keeps its assertions still yields exit 0 / `ALL PASSED` (reproduced). The shared `STRUCT-OK` token is per-probe, not per-assertion. |
| 13 | SC-4: the live eval-gate never fakes green; its fork/Dependabot boundary, secrets and single Go-version source hold | ✓ VERIFIED | P9 passes on the current tree; the exact gate expression, complement notice, single-occurrence secrets, secret-free `Gate check` and `go-version-file: go.mod` are intact. |
| 14 | SC-5: `bash verify-phase-48.sh` exits 0 | ✓ VERIFIED | Local run: exit 0 with exactly one `ALL PASSED`; the same script is the `verify` job, `success` in the green run. |
| 15 | Every record that asserts a run or transport state is reconciled to the same outcome | ⚠️ PARTIAL | The five records named by 48-19 are reconciled (count 6, M1c corrected, STATE line 117 annotated). One tracked ledger still asserts the falsified state: `48-REVIEW-DISPOSITION-R1.md` says CICD remain Pending and no `ci.yml` run can exist, unannotated. |

**Score:** 13/15 truths verified (0 present-but-behavior-unverified, 1 partial, 1 failed)

---

## Mutation Evidence (independent, sandbox copies — the repository was not modified)

Each mutation was applied to a separate copy of `verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md`; the mutated text was re-read before running so a no-op could not fake a result.

| # | Mutation | Target finding | Result | Verdict |
|---|----------|----------------|--------|---------|
| D0 | baseline (unmodified copy) | — | exit 0, one `ALL PASSED` | control |
| D1 | P9 probe header → `:` (token kept) | round-3 CR-02 | exit 1, `P9 … probe log … is empty` | BITES — closed |
| D2 | shared P2/P3 probe header → `:` | round-3 CR-02 | exit 1, `P2 … probe log … is empty` | BITES — closed |
| D3 | P9 probe block deleted entirely | round-3 CR-02 | exit 1, `P9 … probe log … does not exist` | BITES — closed |
| D10 | job-level `if:` added to the `fe` job | round-3 D10 | exit 1, `fe job carries a job-level if:` | BITES — closed |
| D11 | job-level `if:` added to the `docker` job | round-3 D11 | exit 1, `docker job carries a job-level if:` | BITES — closed |
| M6 | P6 probe header → `:` | 48-17 P6 binding | exit 1, `P6 … probe log … is empty` | BITES — closed |
| M10 | P10 evidence write → empty log | 48-17 P10 binding | exit 1, `P10 … probe log … is empty` | BITES — closed |
| M11 | P11 fixture loop reduced to one unknown fixture | 48-17 P11 binding | exit 1, `fixture loop ran 1 fixture(s), want 8` | BITES — closed |
| M12 | reintroduce `go test -list \| grep -qx`, drop `go-test-list.txt` | round-3 regression / SC-1 pin | exit 1, `P2/P3 … structural contract is violated` | BITES — closed |
| M14 | re-add the three-secret `env:` to `Gate check` | R2-WR-05 regression | exit 1, `P9 … violated` | BITES — no regression |
| M13 | **delete P2's entire assertion block, keep P3's and the shared token** | round-3 CR-02 residual | **exit 0, `ALL PASSED`** | **VACUOUS — open gap** |

Reading of the table: every mutation round 3 recorded and every whole-probe/whole-loop neutralisation now bites, so truth 12's round-3 evidence is genuinely closed. What remains is the assertion-granularity class the round-4 review calls WR-03: a partition's own assertions can be deleted inside a shared probe while its success token is still printed.

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.github/workflows/ci.yml` | go/fe/docker + verify, four jobs, gate-preserving | ✓ VERIFIED | Triggers intact, no weakening token, no secrets; determinism step materialises the list; all four jobs green in run 39380373930. |
| `.github/workflows/eval-gate.yml` | gated live workflow, complement notice, secrets on the consumer only | ✓ VERIFIED | P9 passes; re-adding secrets to `Gate check` reds. |
| `.github/actions/setup/action.yml` | checkout-free composite, three pinned majors | ✓ VERIFIED | P5 passes; every job checks out first. |
| `.golangci.yml` | v2 schema, no baseline, revive positively assured | ✓ VERIFIED | P4 passes. |
| `verify-phase-48.sh` | 11 partitions, zero-waiver, evidence-bound tokens | ⚠️ PARTIAL | 11/11 on the healthy tree; whole-probe mutations bite. Assertion-level neutering inside the shared P2/P3 probe still passes. |
| `web/src/components/chat/parts/Sparkline.test.tsx` | FE test that passes on a Linux runner | ✓ VERIFIED | Resolves `Sparkline.tsx` via `new NodeURL('./Sparkline.tsx', import.meta.url)`; four assertions unchanged; `unit tests` green on the runner. |
| `Dockerfile.backend` | backend image that links the static duckdb archive | ✓ VERIFIED | glibc builder (`golang:1.25-bookworm`) and runtime (`debian:bookworm-slim` + libstdc++6 + libgcc-s1); both images built green. |
| `internal/duckdb/connector_test.go` | flake fix that preserves the invariant | ✓ VERIFIED | Best-of-5 minimum sampling; 50 readers and the 3.00x threshold unchanged; contention noise is one-sided. |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` debt markers exist in any deliverable — the debt-marker gate does not fire.

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `ci.yml` push to `main` | GitHub Actions run | `on.push.branches: [main]` | WIRED (observed) | Six runs, event `push`, branch `main`; remote `main` = run head. |
| `go` job | lint / build+vet / `go test ./...` / P5a / P5b | steps | WIRED (observed green) | Every step `success` in run 39380373930. |
| `fe` job | `npm ci` / `tsc --noEmit` / `vitest run` | steps | WIRED (observed green) | Every step `success`. |
| `docker` job | `docker/build-push-action@v7` ×2, `push: false` | steps | WIRED (observed green) | Both build steps `success`. |
| `verify` job | `bash verify-phase-48.sh` | last step | WIRED (observed green) | `phase gate self-proof` success on the public runner. |
| run conclusions | `REQUIREMENTS.md` CICD statuses | the blocked-branch rule | WIRED | Three `Complete (run 39380373930)` statuses, matching the corroborated run. |
| run evidence | WR-06 decision | both disposition ledgers | WIRED | `fixed — closed-by-evidence on run-3` in R1 and R2. |

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `REQUIREMENTS.md` CICD rows | run id `39380373930` | `gh run` job conclusions | Yes — four independently fetched job conclusions | ✓ FLOWING |
| `48-VALIDATION.md` M1 family | run id + heads | `gh run list` / `gh run view` | Yes — six runs, run-6 four-green | ✓ FLOWING |
| `48-19-SUMMARY.md` evidence | run id + step conclusions | GitHub Actions API | Yes — per-step `success` confirmed | ✓ FLOWING |
| `verify-phase-48.sh` tokens | probe logs | each probe's own output | Partly — whole-probe output binds; per-assertion output does not | ⚠️ PARTIAL |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase gate green on the final tree | `bash verify-phase-48.sh` (repo root) | exit 0, 11/11, one `ALL PASSED` | ✓ PASS |
| Public runs exist for this workflow | `gh run list --workflow ci.yml --limit 10` | 6 runs: 5 failure, 1 success | ✓ PASS |
| Per-job conclusions of the green run | `gh api …/runs/39380373930/jobs` | `go=success fe=success docker=success verify=success` | ✓ PASS |
| Per-step execution of the green run | same API, `.steps[]` | every gate step `success` (none skipped) | ✓ PASS |
| Remote default branch | `gh api …/git/ref/heads/main` | `630f875…` (the green run's head) | ✓ PASS |
| Commits after the green run | `git log 630f875..HEAD -- . ':(exclude).planning'` | empty (docs-only) | ✓ PASS |
| Archive tag preserved | `gh api …/git/ref/tags/v1.5-archive` | `369b08a…` | ✓ PASS |
| Assertion-level neutering | sandbox M13 | exit 0, `ALL PASSED` | ✗ FAIL (gap) |

## Probe Execution

The phase's probes are the 11 partitions of `verify-phase-48.sh`, plus the mutation drills above.

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| P1–P11 on the healthy tree | `bash verify-phase-48.sh` | exit 0, one `ALL PASSED` | PASS |
| P1–P11 on the public runner | run 39380373930 `verify` job | `phase gate self-proof` success | PASS |
| Whole-probe neutralisations (D1-D3, M6, M10, M11) | sandbox mutations | all exit 1 with the partition named | PASS |
| Job-level `if:` on fe/docker (D10/D11) | sandbox mutations | both exit 1 | PASS |
| Determinism-shape regression (M12) | sandbox mutation | exit 1, P2/P3 red | PASS |
| Assertion-level neutering (M13) | sandbox mutation | exit 0, `ALL PASSED` | FAIL (gap) |

## Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| CICD-01 | 48-01, 03, 04, 05, 06, 07, 08, 09, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19 | Go CI in GitHub Actions: golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet, PR/push triggered | ✓ SATISFIED | Run 39380373930 `go=success`; every go step success; `Complete (run 39380373930)` in both REQUIREMENTS.md places. |
| CICD-02 | 48-02, 04, 05, 07, 10, 11, 13, 15, 16, 17, 18, 19 | FE CI: `tsc --noEmit` + full `vitest run` | ✓ SATISFIED | Same run `fe=success`; install/typecheck/unit tests all success; both REQUIREMENTS places Complete. |
| CICD-03 | 48-02, 04, 05, 07, 10, 11, 13, 15, 16, 17, 18, 19 | Both Dockerfiles build in CI | ✓ SATISFIED | Same run `docker=success`; both image builds success; both REQUIREMENTS places Complete. |

**Cross-reference:** every plan in the phase declares its `requirements:` in frontmatter, and the union of those declarations is exactly `{CICD-01, CICD-02, CICD-03}` — the same set `REQUIREMENTS.md` maps to Phase 48. **No orphaned requirement, no unmapped ID.** `BL-02` remains a Deferred backlog item, not a Phase 48 requirement.

**Flip discipline:** the flip cites the corroborated run id in both places. The run's four job conclusions were fetched independently, not read from a summary.

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `verify-phase-48.sh` | 465, 473-474 | P2 and P3 share one `STRUCT-OK` token from one probe; either partition's assertion set can be deleted while both deposit PASS (M13) | 🛑 Blocker | The gate can report P2 green while P2's own checks are absent — the residual of round-3 CR-02. |
| `48-REVIEW-DISPOSITION-R1.md` | tail paragraph | "CICD-01/02/03 remain Pending", "no ci.yml run can exist yet", "transport … unreachable" with no correction note | ⚠️ Warning | A tracked record asserts a state falsified by six runs and a four-green run. |
| `verify-phase-48.sh` | 1409 | P11 else-detector uses `sed … \| grep -qE` (SIGPIPE-prone) | ℹ️ Info | Latent; review WR-01. |
| `verify-phase-48.sh` | 1114-1133 | trust-surface path-filter/secret rules are text scans (quoted `"paths":`, `toJSON(secrets)` pass) | ℹ️ Info | Review WR-02; warning-grade for the trust boundary. |
| `ci.yml` | 230-236 | linter install inherits `GOTOOLCHAIN`/`GOSUMDB`/`GOPROXY` from the runner image | ℹ️ Info | Review IN-01; safe today, but the header's claim is overstated. |

## Manual-Only Verifications (acknowledged, unchanged)

These are documented in `48-VALIDATION.md` as manual-only and do not block CICD-01/02/03. They are recorded here so the record does not imply they were automated:

| # | Verification | Why manual | Current state |
|---|--------------|------------|---------------|
| M2 | A fork PR triggers the live `eval-gate` and the notice job is green | Requires an actual fork PR | Open (by design) |
| M3 | A Dependabot PR is treated like a fork | Requires an actual Dependabot PR | Open (by design) |
| M5 | Docker cache hit on the second green run for both images | Requires the next green `docker` run | Open — the first green docker job is run 39380373930 |

## Human Verification Required

None beyond the acknowledged manual-only rows above. Every truth in this report was decided by evidence obtained directly (public run ids and job/step conclusions, `gh` refs, the local gate, and independent sandbox mutations).

## Gaps Summary

**The phase goal's public outcome is achieved.** For the first time, a real GitHub Actions run of `ci.yml` on the public default branch is four-green — `go=success`, `fe=success`, `docker=success`, `verify=success` — and every gate step inside those jobs executed and succeeded. The three previously red causes are fixed at the source (materialised `-list` output, module-relative Sparkline path, glibc backend image), the determinism shape is pinned, CICD-01/02/03 are `Complete` on that run id in both REQUIREMENTS.md places, and the remote default branch tip is the green run's head. Score moves 9/15 → 13/15; nine of the previous eleven gaps are closed.

**Two must-have truths keep the phase at `gaps_found`:**

1. **The verifier still folds an assertion-level neutralisation into PASS** (truth 12). Round 4 genuinely closed the whole-probe class and the fe/docker `if:` arm — all reproduced here. But P2 and P3 share one `STRUCT-OK` token, so deleting P2's entire assertion block leaves the gate at exit 0 / `ALL PASSED` (M13). The round-4 review independently records the same residual for P1, P4 and the single-print probes (WR-03). The fix is the review's: one distinct token per assertion group, required at deposit time.
2. **One tracked record still asserts the falsified state** (truth 15). The five records named by 48-19 are reconciled, including STATE line 117's annotation and the closed, tracked R2 ledger. `48-REVIEW-DISPOSITION-R1.md`'s closing paragraph still says CICD remain Pending, transport is unreachable, and no `ci.yml` run can exist — with no correction note, although 48-15's done criterion named both disposition ledgers.

Neither gap touches the public CI outcome or the requirement flips. The minimum path to `passed` is (1) split the shared success token and require a per-assertion token for P2/P3 (and, per the review, P1/P4/P6/P7/P9), and (2) annotate the R1 ledger's closing paragraph as historical.

---

_Verified: 2026-10-20T07:32:04Z_
_Verifier: Claude (gsd-verifier)_
