---
phase: 48-ci
plan: 13
subsystem: ci-verifier
tags: [ci, verifier-hardening, mutation-testing, partition-accounting, supply-chain]
requires:
  - 48-12 (the workflow fixes this plan pins: module-proxy install, persist-credentials, -list guard, Gate-check secret removal)
provides:
  - success-token partition accounting in verify-phase-48.sh (done_partition requires begin + ok)
  - verify-job contract pinned in the shared P2/P3 probe (existence, if-freeness, gate terminus, credential-free checkout, module-proxy install)
  - five-way mutation evidence that M8/M9/M10 and both 48-12-fix reintroduction shapes now bite
affects:
  - 48-15 (the first real CI run executes this gate; the verify job in CI runs this hardened script)
tech-stack:
  added: []
  patterns:
    - success-token deposit immediately after each probe's exit-code assertion (begins/ok marker pair per partition)
    - runtime-assembled banned literals (SOFT_KEY convention extended to the fetch/script tokens)
    - four-way registry count (registry == call sites == begin markers == success tokens)
key-files:
  created: []
  modified:
    - verify-phase-48.sh
decisions:
  - "P11's registry count runs LAST (banner checks -> fixtures -> P11 token -> four-way count): the count demands P11's own success token, so the reverse order would be a self-failing gate"
  - "The fetch/script banned tokens follow the SOFT_KEY runtime-assembly rule — a gate that spells a banned CI keyword in its own source makes the repo-wide grep meaningless"
  - "TRUST_SURFACE_COUNT carries no :- default: a variable whose setter is the probe under test must abort under set -u, not interpolate a plausible zero"
  - "CICD-01/02/03 left Pending — hardening the verifier does not substitute for the first real GitHub Actions run (48-15); the same premature-flip this branch reverted in 957b7d3 is not repeated"
metrics:
  duration: 16 min
  completed: 2026-10-20
  tasks: 3
  files: 1
status: complete
actuals:
  tokens: 6443
  tasks: 3
  commits: 3
  plan_head_before: 957b7d31e7d2fa743c8d5fa6bb20fb9ad8de93bb
  plan_head_after: e5695ec20c9d90e537d64c8d8922d5d93905ad7c
commits: 3
---

# Phase 48 Plan 13: Verifier self-proof hardening (truth 6) Summary

**Success-token partition accounting (done_partition requires begin + ok), the verify-job contract pinned in P2/P3, and P2/P9 tightened for every 48-12 fix — with five sandbox mutation drills proving M8/M9/M10 and both reintroduction shapes now bite.**

## What Was Built

### Task 1 — success-token partition accounting (R2-WR-03), commit `f399b53`

- `ok_partition() { : > "$WORK/ran/$1.ok"; }` added next to `begin_partition`; deposits happen only AFTER each probe's own decisive exit-code assertion (11 sites — P2/P3 both deposit at the shared probe's rc check).
- `done_partition` now requires begin AND ok, with two distinct fail messages: begin missing = "its body did not run (IN-06)"; ok missing = "its probe did not run to success (R2-WR-03)".
- `${TRUST_SURFACE_COUNT:-0}` → bare `${TRUST_SURFACE_COUNT}`: a deleted P8 probe now aborts under `set -uo pipefail` instead of printing `PASS P8 trust-surface (0 workflow(s) ...)`.
- P11 reordered to banner checks → fixture loop → `ok_partition "P11"` → four-way registry count (`CALL_SITES` == `BEGIN_COUNT` == `OK_COUNT` == `EXPECTED_PARTITIONS`, all four numbers named in the fail message), closing at 11/11/11/11 on the healthy tree.
- Closing banner + header partition descriptions updated to the new accounting.

### Task 2 — pin the verify job, tighten P2/P9 (R2-WR-01), commit `2e7377c`

- The shared P2/P3 Python probe now also judges `jobs["verify"]`: existence; no `if` on the job mapping or any step (the 48-07 rule extended to the fourth job); the LAST step's `run` contains `bash verify-phase-48.sh` (gate-as-terminus, so reordering reds); first step `actions/checkout@v7` with `persist-credentials` exactly False (IN-10 extended); the install surface is the pinned module-proxy line `go install .../cmd/golangci-lint@v2.12.2`.
- Two banned ci.yml tokens (the fetch-and-pipe bootstrap prefix and the install-script name) are printf-assembled in the bash caller and passed as argv — never spelled joined in the gate's own source.
- P2: the `-count=100` determinism step must also carry `-list '^TestDeriveEdgesDeterministic$'` and `set -o pipefail` (R2-WR-04's guard is now pinned, not just present).
- P9: each of the three live secret mappings must occur EXACTLY once, and the offline `Gate check` step carries no `env:` key (R2-WR-05).

### Task 3 — mutation drills, commit `e5695ec`

All five drills ran in a `mktemp` sandbox copy (`verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md`), each mutation assertion-checked before the run; the repository tree was clean after every drill.

| Drill | Mutation (sandbox copy only) | rc | ALL PASSED | Matching failure line |
|-------|------------------------------|----|-----------|------------------------|
| 1 (M8) | Delete P8's `scan_trust_surface` block, keep begin/done markers | 1 | absent | `line 1064: TRUST_SURFACE_COUNT: unbound variable` (set -u abort naming the probe-owned variable — accepted P8 attribution) |
| 2 (M9) | Add job-level `if: github.event_name == 'push'` to the verify job | 1 | absent | `P3: the verify job carries a job-level if: ("github.event_name == 'push'") - the phase gate is skippable (R2-WR-01)` |
| 3 (M10) | Delete the verify job entirely (jobs → docker/fe/go) | 1 | absent | `P2: ci.yml has no 'verify' job (IN-07) - the phase gate is not wired into CI` |
| 4 | Re-add the three-secret env block to `Gate check` | 1 | absent | `P9 the 'Gate check' step carries an env: key - ... dead credential surface (R2-WR-05)` |
| 5 | Remove the `-list '^TestDeriveEdgesDeterministic$'` guard from the P5a step | 1 | absent | `P2: the -count=100 determinism step lost the -list existence guard - a renamed test would pass vacuously (R2-WR-04)` |

## Verification

- `bash verify-phase-48.sh` → rc=0, 11/11 partitions, exactly one `ALL PASSED`, after every task.
- P11 closing counts: registry 11/11, begin markers 11/11, success tokens 11/11.
- `git diff --stat` across the plan touches only `verify-phase-48.sh`; `git status --porcelain -- .github/workflows .planning/ROADMAP.md` empty after the drills.
- `EXPECTED_PARTITIONS` unchanged at 11; no partition added or removed.

## Deviations from Plan

**1. [Rule 1 - Bug] Task 1 in-task: the new header comment spelled the banned banner literal**
- **Found during:** Task 1 (first gate run red: "the passing banner has 2 print sites")
- **Issue:** The M8 narrative comment wrote `ALL PASSED` verbatim; P11's single-print-site rule greps the whole file, comments included.
- **Fix:** Reworded to "under the green banner"; re-run green.
- **Files modified:** verify-phase-48.sh (same commit)

**2. [Plan amendment] Task 3: P9's Gate-check env check ordered before the count check**
- **Found during:** Task 3 drill 4
- **Issue:** With the count loop first, a re-widened env block surfaces as "references secrets.X 2 time(s)" — a valid P9 red that does not name `Gate check`, which is what the drill requires as evidence.
- **Fix:** Reordered so the step-naming check fires first (commit `e5695ec`); both layers retained.

**3. [State protocol] `requirements mark-complete` deliberately NOT run**
- The plan frontmatter lists CICD-01/02/03, but those requirements are run-evidence-gated (the first real GitHub Actions run is 48-15's; the phase decision trail — including the revert in `957b7d3` — requires them to stay Pending until then). Marking them complete here would repeat the premature flip this branch already reverted.

## Threat Model Follow-Through

- **T-48-51** (success-token accounting) — landed in Task 1, mutation-proven by drill 1.
- **T-48-52** (verify job pinned by the same probe as the go job) — landed in Task 2, mutation-proven by drills 2 and 3.
- **T-48-SC** — no package installed by this plan; no new network endpoint, auth path, or trust-boundary surface introduced. No threat flags.

## Known Stubs

None — no placeholders, no unwired data paths, no skipped checks introduced.

## Self-Check: PASSED

- `verify-phase-48.sh` exists and is modified by all three commits (see `git log --oneline 957b7d3..e5695ec`).
- Commits `f399b53`, `2e7377c`, `e5695ec` are ancestors of HEAD.
- Ledger-measured commit count: 3 (`git rev-list --count 957b7d3..HEAD`).
