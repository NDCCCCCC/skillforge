---
phase: 48-ci
plan: 04
subsystem: infra
tags: [github-actions, verify-gate, self-probe, eval-gate, fork-pr, roadmap, branch-protection, blocked]

requires:
  - phase: 48-ci
    plan: 02
    provides: "verify-phase-48.sh partitions P5-P8 and EXPECTED_PARTITIONS=8 — the registry shape P9/P10/P11 extend"
  - phase: 48-ci
    plan: 03
    provides: "eval-gate.yml's three-clause fork gate, the ::notice:: skip-notice job, go-version-file: go.mod, and ROADMAP SC-4's corrected wording — all four are P9/P10's asserted subjects"
provides:
  - "verify-phase-48.sh partitions P9 (eval-gate-gate), P10 (sc4-wording), P11 (self-probe); EXPECTED_PARTITIONS=11"
  - "scan_trust_surface — P8's trust-surface scan as one shared shell function, with vacuity rules P8 now executes"
  - "the recorded branch-protection decision (Option A), so the absence of enforcement is an explicit choice"
affects: [phase-48-ci, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 4361
  tasks: 4
  commits: 2

plan_head_before: 88ae7ad6cfd23c6c1ac3c16ead20244c555f443b
plan_head_after: e3272e3bd29f8e4691f38a3d54c3d5fa63232c66

tech-stack:
  added: []
  patterns:
    - "runtime-assembled tokens: a checker that counts a banner must not spell the banner, or the count includes its own source"
    - "one shared scan function with two callers — the real directory and the vacuity fixtures exercise the same code"
    - "an empty partition is a failure condition, not a vacuous pass (empty jobs / absent permissions / empty permissions / empty file list)"

key-files:
  created: []
  modified:
    - verify-phase-48.sh

key-decisions:
  - "P9 asserts each fork-gate clause individually instead of matching the whole expression — a whole-expression substring match passes on two clauses out of three, and the missing clause prints by name"
  - "P11's banner token is assembled at runtime (printf 'ALL' 'PASSED') so the single-print-site count cannot see the checker's own comment or pattern text; the same trick makes the RAN-gate line unambiguous"
  - "P8's trust-surface scan is now ONE shell function scan_trust_surface, called by P8 over the real directory and by P11 over synthetic fixtures — a private probe copy could drift from what P8 runs, which would make the probe prove nothing"
  - "The four vacuity rules (zero files / empty jobs / absent permissions / empty permissions) were added to the code P8 executes, not to a probe: `every value is read-or-none` is satisfied vacuously by an empty mapping, so the rules had to land where the real scan runs"
  - "Task 3 (a real CI run) could not be executed from this host: git transport to github.com is unreachable and the PR path is closed by repo topology. CICD-01/02/03 are deliberately left Pending in REQUIREMENTS.md rather than marked complete"
  - "Branch protection required status checks: Option A — leave protection off. Recorded as an explicit decision so a later phase cannot read the absence of enforcement as an oversight"
  - "No environment variable, flag or argument disables P10 or P11; both halves of the self-probe run on every invocation"

requirements-completed: []

coverage:
  - id: D1
    description: "P9 eval-gate-gate: the three-clause fork/Dependabot gate, the visible ::notice:: skip notice, the intact three-secret wiring, the un-widened trigger, and the directory-wide single Go version source"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "bash verify-phase-48.sh -> exit 0, P9 PASS, EVALGATE-OK (2 jobs) + GOVERSION-OK (3 files)"
        status: pass
      - kind: other
        ref: "negative probe: mv eval-gate.yml out of tree -> rc=1, message names the missing file"
        status: pass
      - kind: other
        ref: "negative probe: dependabot clause neutralised -> P9 fails naming the clause"
        status: pass
      - kind: other
        ref: "negative probe: ::notice:: removed -> P9 fails"
        status: pass
      - kind: other
        ref: "negative probe: go-version literal in eval-gate.yml -> P9 fails naming file:line"
        status: pass
    human_judgment: false
  - id: D2
    description: "P10 sc4-wording: ROADMAP SC-4's old hermetic-precedent phrase absent, the live-workflow replacement present, both reported by name; no test-count assertion"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "bash verify-phase-48.sh -> P10 PASS with the two states printed individually"
        status: pass
      - kind: other
        ref: "negative probe: old phrase restored into ROADMAP.md -> P10 fails naming the phrase, file:line printed"
        status: pass
      - kind: other
        ref: "negative probe: replacement phrase removed -> P10 fails naming the missing wording"
        status: pass
    human_judgment: false
  - id: D3
    description: "P11 self-probe: registry recomputed against the done_partition call sites, one RAN-gated banner print site, and an empty partition proven to FAIL through the same scan_trust_surface P8 runs"
    requirement: CICD-01
    verification:
      - kind: other
        ref: "bash verify-phase-48.sh -> P11 PASS: 'registry 11/11, banner print sites 1 (line 961, RAN gate line 960)'"
        status: pass
      - kind: other
        ref: "negative probe: three vacuity rules neutered -> empty-jobs fixture passes the scan -> P11 fails with 'verifier is vacuous on an empty partition'"
        status: pass
      - kind: other
        ref: "4 fixtures (empty-jobs, empty-perms, no-perms, empty-dir) each rejected with their own named reason"
        status: pass
    human_judgment: false
  - id: D4
    description: "A real GitHub Actions run with go, fe and docker all green (VALIDATION M1) — the only venue where CICD-02 and CICD-03 can execute"
    requirement: CICD-02
    verification: []
    human_judgment: true
    rationale: "NOT ACHIEVED. git transport to github.com is unreachable from this host (3/3 ls-remote attempts timed out; push --dry-run fails at TLS/connect) and the trigger-preserving PR path is closed because origin/main shares no history with this branch. No run exists to inspect. The user must obtain this run from a host with working git transport."
  - id: D5
    description: "The branch-protection one-way door consciously decided and recorded"
    requirement: CICD-03
    verification:
      - kind: other
        ref: "Task 4 verify: grep 'required status check' + '^Decision: [ABC]$' against this file -> CHECKPOINT-RECORDED"
        status: pass
    human_judgment: true
    rationale: "The recording is machine-checked; the substance of the choice (Option A) is a user judgment about enforcement, collected by the orchestrator before this plan ran."

duration: 16min
completed: 2026-10-19
status: blocked
blockers:
  - "CICD-01/02/03 remain unproven: no GitHub Actions run exists for ci.yml, and none can be triggered from this host (see Issue 1)."
---

# Phase 48 Plan 04: Full Predicate Coverage and the Branch-Protection Decision Summary

**`verify-phase-48.sh` now carries eleven partitions covering predicates P1–P23 — including a self-probe that recomputes its own registry and proves an empty partition FAILS — and the branch-protection one-way door was consciously declined; the real CI run that CICD-01/02/03 need could not be obtained from this host.**

## Performance

- **Duration:** 16 min
- **Started:** 2026-10-19T09:39:42Z
- **Completed:** 2026-10-19T09:55:24Z
- **Tasks:** 4 (3 executed, 1 environment-blocked)
- **Files modified:** 1
- **Commits:** 2 task commits + 1 metadata commit

## Accomplishments

- **P9 `eval-gate-gate`** locks the fork boundary into the local gate. It asserts the `eval-gate` job's `if:` **clause by clause** — `github.event_name != 'pull_request'`, `head.repo.full_name == github.repository`, `github.actor != 'dependabot[bot]'` — and prints which clause is missing, because a whole-expression substring match would pass on two clauses out of three and re-open the fork hole. It also asserts the `eval-gate-skip-notice` job and its literal `::notice::`, the three intact secret mappings (deleting them would make the workflow green while it silently stopped being a live gate), the un-widened `pull_request`/`push` branch triggers normalized through `d.get('on', d.get(True))`, and a directory-wide single Go version source: no `go-version:` literal anywhere under `.github/workflows/` or `.github/actions/`, and every `setup-go` step carrying `go-version-file: go.mod`.
- **P10 `sc4-wording`** asserts the ROADMAP SC-4 correction in both directions and reports the two conditions separately (`old = absent`, `replacement = present`), so "the correction is missing" and "the correction is wrong" are distinguishable reds. It asserts nothing about test counts — the Goal line's stale `666 / 84` is explicitly out of D-12's scope, and a gate that pinned a count would be a future silent-drift failure.
- **P11 `self-probe`** is the partition that proves the other ten are not decorative. *Half 1* recomputes `EXPECTED_PARTITIONS` against the `done_partition` call sites actually present in the script and pins the passing banner to exactly one print site inside the RAN-gated branch. *Half 2* extracts P8's trust-surface scan into a single shell function `scan_trust_surface <workflow-dir>` — P8 calls it with the real `.github/workflows`, P11 calls it with synthetic fixtures — and requires the fixtures to **fail**. Four vacuity rules are now failure conditions in the code P8 actually executes: zero workflow files, an empty/absent `jobs:` mapping, an absent `permissions:` mapping, and an empty `permissions:` mapping.
- P1–P8 keep exactly the predicates their defining plans set; P8's only change is that its inline body became `scan_trust_surface "$ROOT/.github/workflows"`.
- `bash verify-phase-48.sh` exits 0 with `=== Phase 48 Gate: ALL PASSED ===` and eleven partition PASS lines.

## Task Commits

1. **Task 1: partition P9** — `0a2a1bf` (feat)
2. **Task 2: partitions P10 and P11** — `e3272e3` (feat)
3. **Task 3: prove CICD-01/02/03 on a real CI run** — *no commit*: the task produces evidence, not code, and it produced none (see Issue 1)
4. **Task 4: branch-protection checkpoint** — this SUMMARY (docs commit)

## Files Created/Modified

- `verify-phase-48.sh` — `EXPECTED_PARTITIONS` 8 → 11; new partitions P9, P10, P11; `scan_trust_surface` extracted from P8's inline body and hardened with four vacuity rules; the closing missing-partition loop and the `Partitions verified:` list extended to P11

## Decisions Made

- **P9 asserts clauses, not the expression.** The gate's three clauses are independently load-bearing, and a reader of a red run needs to know *which* one vanished.
- **Runtime-assembled banner token.** P11 counts the passing banner's print sites in its own source, so the script must contain exactly one literal occurrence of it. The token is built with `printf '%s %s' 'ALL' 'PASSED'`, and the RAN-gate line is matched via a two-part token, so the checker's own text cannot inflate either count. Phase 47's first verifier shipped that banner unconditionally; this is the mechanism that keeps it from recurring.
- **The vacuity rules went into the shared function, not into a probe.** `every value ∈ {read, none}` is satisfied **vacuously** by a mapping with no values, so a probe aimed at a private copy of P8's predicate could not fail — the refactor is what closes the gap in the code P8 runs.
- **`scan_trust_surface` returns; it never calls `fail`.** P11 has to *observe* the failure rather than abort on it, so the function writes its reason to a log and returns non-zero; each caller keeps the exit-code-authoritative failure at its own site.
- **CICD-01/02/03 were left Pending in REQUIREMENTS.md.** The deliverables exist and are structurally proven, but the CI-run proof this plan's success criterion 3 requires does not. Marking them complete would let a later phase read an unproven gate as a proven one — the exact class this phase exists to forbid.
- **Branch protection required status checks: Option A — leave protection off.** The gates run and are publicly visible; enforcement is a separate decision once the three jobs have been stable for a few merges. Recorded here as an explicit decision, not an omission.

Decision: A

The one-way door was **consciously declined**, not overlooked. `go` / `fe` / `docker` are therefore **not** registered as required status checks on `main`, and `GET /repos/NDCCCCCC/skillforge/branches/main/protection` still returns 404. A later phase that finds no enforcement configured must treat this as Option A, taken deliberately on 2026-10-19; it must not re-litigate it as a defect, nor discover it and "fix" it silently. Reversing it means registering `go` / `fe` / `docker` as required status checks on `main` (Option B or C), which publishes those three job names as an external contract — D-21 explicitly chose a single `ci.yml` with three jobs for that reason, and renaming a job afterwards would strand every open PR until the settings are edited. No workflow file was changed in response to this checkpoint, and no gate was weakened to make enforcement easier.

## The branch-protection option set, as presented

- **A. Leave protection off** (chosen) — the gates run and are publicly visible; enforcement is a separate decision once the three jobs have been stable for a few merges.
- **B. Require `go` + `fe` + `docker` on `main`** — installs the one-way door; needs an admin token with `administration: write`.
- **C. Require only `go`** — partial enforcement, and a partially required set gives the fork-PR path a weaker surface than it appears to.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `info()` mangled the vacuity evidence**

- **Found during:** Task 2 (first P11 run)
- **Issue:** P11 printed each fixture's rejection reason through `info()`, which uses `echo -e`. The reason quotes a **native Windows path**, and `echo -e` read its backslashes as escape sequences — `\t` in `...\Temp\tmp.XXXX\p11-fixtures\...` became a tab and a `\c`-class sequence swallowed the line, so the evidence was unreadable and consecutive lines ran together.
- **Fix:** the reason is rendered through `head -1 … | sed 's/^/       /'`, which is never re-interpreted. `info()`/`pass()`/`fail()` themselves were left alone — they carry the ANSI colour escapes that `echo -e` exists to interpret.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** all four fixture reasons now print intact, each naming its own rule.
- **Committed in:** `e3272e3`

**2. [Rule 1 - Bug] The self-probe's own comment defeated its banner count**

- **Found during:** Task 2 (first P11 run — `the passing banner has 2 print sites`)
- **Issue:** the explanatory comment quoted the banner literal while explaining why there must be exactly one of them, so P11's own grep counted two sites. This is the same self-reference class 48-01 handled for the forbidden CI keyword: a checker cannot count a string it also writes.
- **Fix:** the comment was reworded to "its green banner" and the runtime token assembly was introduced for the actual count.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** `grep -c '<banner token>' verify-phase-48.sh` = 1; P11 reports `banner print sites 1`.
- **Committed in:** `e3272e3`

**3. [Rule 2 - Missing Critical] Three vacuity rules would have been unreachable**

- **Found during:** Task 2 (designing half 2)
- **Issue:** the plan names four empty-partition failure conditions but specifies a single fixture (empty `jobs` **and** empty `permissions`). That fixture fails on the `jobs` rule before the permissions rules are ever evaluated, leaving "absent `permissions:`" and "empty `permissions:`" as dead branches inside the function — rules that exist but are never shown to fire, which is the vacuity the probe exists to disprove.
- **Fix:** three additional fixtures under `$WORK` (`empty-perms`, `no-perms`, `empty-dir`) are run through the same `scan_trust_surface`; each must fail, and each prints the rule that rejected it. This strengthens the probe without relaxing anything the plan required.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** each of the four fixtures is rejected with its own named reason; neutering the three mapping rules makes the `empty-jobs` fixture pass the scan and P11 then fails with `verifier is vacuous on an empty partition`.
- **Committed in:** `e3272e3`

---

**Total deviations:** 3 auto-fixed (2 bugs, 1 missing critical behaviour).
**Impact on plan:** none on scope. Deviation 3 is additive coverage inside the partition the plan defines; deviations 1 and 2 were defects in the new code, fixed at their source.

## Issue 1 (BLOCKING): the real CI run could not be obtained

Task 3 is the only execution venue for CICD-02 and CICD-03, and it did not run. Three independent facts, each verified on this host:

1. **git transport to github.com is unreachable.** `git ls-remote origin` failed 3 times out of 3; `git push --dry-run origin HEAD:refs/heads/probe-48-04` fails with `fatal: unable to access 'https://github.com/NDCCCCCC/skillforge.git/': Failed to connect to github.com port 443 after 21035 ms: Timed out` (a separate attempt reported `schannel: failed to receive handshake, SSL/TLS connection failed`). The `gh` CLI reaches the API — a different HTTP path — but `gh` cannot push. No proxy is configured (`env | grep -i proxy` is empty; `git config --get-regexp '^http\.'` shows only sslbackend/sslCAInfo/throttle settings).
2. **Even with transport, the trigger-preserving path is closed.** A push of this branch produces no run by design: 48-01 bound `push.branches: [main]`, and widening it (or adding `workflow_dispatch:` / a `paths:` filter) is forbidden by D-24 / Pitfall 10 / T-48-13's zero-weakening rule. The sanctioned alternative is a PR whose base is `main`, but **`origin/main` shares no history with this branch** — it is a single commit `369b08a` ("SkillForge v1.5 — Skill 驱动的企业知识问答平台"), this branch is 1833 commits ahead of it, and `git merge-base origin/main HEAD` is **empty**. GitHub cannot open a PR between unrelated histories, and merging this branch into the public snapshot is a repository-visible decision that is not this plan's to take.
3. **No run exists to inspect.** `gh run list --workflow ci.yml --branch phase-20-p10-6-admin-console` and `--branch main` both return `HTTP 404: workflow ci.yml not found on the default branch`. `ci.yml` has never been on the remote default branch, so the workflow-path resolver cannot find it — the fallback in the plan's own verify cannot succeed until `ci.yml` reaches `main`. The most recent run of any kind is `eval-gate` from 2026-10-02.

**The plan's own verify, run verbatim, fails** — at its second clause. `bash verify-phase-48.sh` passes (rc 0); `test -z "$(git status --porcelain)"` returns 1.

### The dirty-tree clause, precisely

`git status --porcelain` reports **98 untracked entries and 0 tracked modifications**. The intent of that clause is the Phase 47 lesson — an untracked `.github/` file that never reaches the remote is a workflow that never runs. That intent holds: **zero untracked entries are under `.github/`**. The 98 entries are pre-existing repository state, not produced by this plan: 42 under `.planning/quick/`, 8 under `.planning/debug/`, 2 under `.planning/phases/`, plus `web/pnpm-lock.yaml` / `web/pnpm-workspace.yaml`, several `skills/*` files, 5 files under `internal/server/`, and `test.md`. None was created or modified by this plan, and nothing this plan wrote lives outside `$WORK` (temp) and the single tracked file `verify-phase-48.sh`.

### Consequence

**CICD-01, CICD-02 and CICD-03 are left Pending in `.planning/REQUIREMENTS.md`.** Success criterion 3 (`a real GitHub Actions run shows go, fe, and docker all green`) is **not met**, and this plan's `status` is `blocked`. What is proven is the static contract: the three jobs exist, are hermetic, are blocking, publish nothing, and are asserted by eleven partitions. What is **not** proven is that they pass on `ubuntu-latest` — that needs a host with working git transport to github.com, then either a PR based on `main` or the commit landing on `main`.

## Zero-waiver audit

No gate was weakened to reach a green: zero `continue-on-error`, zero `paths:`/`paths-ignore:`, zero `|| true`, zero hard-coded test counts were added to any workflow or to `verify-phase-48.sh`. `push.branches` remains exactly `[main]`; `workflow_dispatch:` was not added to `ci.yml`; `pull_request_target` remains absent. `.github/workflows/` was not modified by this plan at all — no workflow file changed in response to the branch-protection checkpoint.

## Issues Encountered

1. **The real CI run could not be obtained** — see Issue 1 above. This is the blocking item.
2. `golangci-lint` resolves to `/c/Users/CPIC/go/bin/golangci-lint` (v2.12.2) on this host, not the `/c/CPIC/go/bin` the dispatch contract recorded. Both are the same 2.12.2 binary; the contract's PATH export worked unchanged. 48-01 and 48-03 noted the same.
3. 48-02's note that P8's GLM-secret rule is unreachable in practice still holds: inserting `secrets.GLM_API_KEY` into `ci.yml` fails at P2/P3 first. P9's secret-wiring assertions are on `eval-gate.yml`, which P2/P3 do not judge, so they are reachable.

## User Setup Required

**None for the artifacts this plan owns.** To close CICD-01/02/03, a host with working git transport to github.com must push this branch and then either open a PR based on `main` or land the commit on `main`, and corroborate with `gh run view <run-id> --json jobs -q '.jobs[] | "\(.name)=\(.conclusion)"'` showing `go=success`, `fe=success`, `docker=success` (VALIDATION M1). Do **not** widen `push.branches`, add `workflow_dispatch:`, or add a `paths:` filter to get there.

## Next Phase Readiness

- Phase 48's local half is complete: eleven partitions, P1–P23 covered, exit 0 with the single RAN-gated `ALL PASSED` banner.
- The CI half cannot be closed from this machine. Phase 49 (Hermetic Demo) and Phase 51 (Release Engineering) both build on the composite action and the three-job matrix; their own CI-dependent criteria inherit the same unblocking step.
- `.github/actions/setup/action.yml` remains the interface Phase 51 reuses (D-22/D-30), unchanged by this plan.

## Self-Check: PASSED

Scope: this self-check covers the summary's own artifact claims — that the files it names exist on disk and the commits it names are in history. It is **not** a claim that the plan is complete: Task 3 is blocked and success criterion 3 is unmet, as recorded above and in the frontmatter `status: blocked`.

- `verify-phase-48.sh` exists, is tracked, and is the only file this plan modified.
- Task commits present in history: `0a2a1bf`, `e3272e3` (2 commits, measured from the plan ledger base `88ae7ad`).
- `bash verify-phase-48.sh` exits 0 with `=== Phase 48 Gate: ALL PASSED ===` and eleven partition PASS lines.
- `EXPECTED_PARTITIONS=11`; `grep -c` of the banner token in the script = 1; `done_partition` call sites = 11.
- Task 1 verify printed `SELFCHECK-OK` and `P9-OK`; Task 2 verify printed `SELF-PROBE-OK`; Task 4 verify printed `CHECKPOINT-RECORDED`.
- Five P9 negative probes, two P10 negative probes and the P11 vacuity probe all failed in the required direction.
- **Task 3 verify FAILED (rc 1)** at `test -z "$(git status --porcelain)"`, and no CI run exists. This is recorded as the plan's blocking issue, not as a pass.

---
*Phase: 48-ci*
*Completed: 2026-10-19 (plan blocked on the CI-run criterion)*
