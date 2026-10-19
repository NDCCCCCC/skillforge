---
phase: 48-ci
plan: 11
subsystem: infra
tags: [github-actions, decision-record, verify-job, golangci-lint, first-run-recipe, gate-integrity]

requires:
  - phase: 48-ci
    plan: 06
    provides: "replay step faithfulness and step-scoped live secrets (no job-level env creep)"
  - phase: 48-ci
    plan: 07
    provides: "the step-level `if:` rejection rule on every go/fe/docker gate step (48-11's new job obeys it)"
  - phase: 48-ci
    plan: 09
    provides: "P4 revive sentinel — the lint teeth the new verify job re-proves on every run"

provides:
  - "The four recorded decision lines WR-06 / IN-08 / IN-07 / M1 that plan 48-10's disposition ledger and bookkeeping read verbatim"
  - "A fourth published job `verify` in ci.yml that runs bash verify-phase-48.sh with the pinned linter, so verifier drift turns a push/PR red instead of rotting on a maintainer host"
  - "The documented closed pull_request base set [main, 'phase-*'] in the ci.yml header next to invariant 1 (IN-08 option F)"
  - "The M1 run recipe (history decision -> transport-capable push -> gate-preserving trigger -> gh run watch -> red-run classification -> report format), executable verbatim from a host with git transport"

affects: [48-10, phase-49-hermetic-demo, phase-51-release-engineering]

actuals:
  tokens: 3400
  tasks: 3
  commits: 3

plan_head_before: 079d618f9efb23f42c41c9c40c94d070ca1901ca
# Production HEAD after Task 2. The third commit is the metadata commit that
# carries this file itself (its SHA lands in STATE.md state_head, the same
# one-commit-late convention 48-08-SUMMARY.md used).
plan_head_after: e112488a3ed15296c2067eda74d812967157c0a1

status: complete

metrics:
  duration: 11 min
  started: 2026-10-19T14:34:15Z
  completed: 2026-10-19T14:45:00Z

tech-stack:
  added: []
  patterns:
    - "A decision checkpoint resolved by the user is implemented by a continuation executor: the choice is written verbatim where the NEXT plan reads it, never paraphrased"
    - "The verify job installs the linter from the official install script instead of reusing the go job's lint action, so the gate self-proof never inherits the push-run whole-repo fallback that WR-06 leaves unmeasured"
    - "A blocked manual-only validation is recorded as `M1: BLOCKED — <reason>` with its run recipe persisted, never simulated"

key-files:
  created:
    - .planning/phases/48-ci/48-11-SUMMARY.md
  modified:
    - .github/workflows/ci.yml
    - .planning/STATE.md
    - .planning/WINDOWS.md

key-decisions:
  - "WR-06 = option D: record both candidate lint-scope fixes and decide from the first real run's evidence; NO code change to the lint step; deferral carried into plan 48-10's ledger"
  - "IN-08 = option F: document the closed PR base set [main, 'phase-*'] in the ci.yml header next to invariant 1; no trigger behavior change"
  - "IN-07 = option A: a new independent `verify` job (checkout -> composite go:true -> pinned golangci-lint v2.12.2 -> bash verify-phase-48.sh), no continue-on-error, no step-level if:, no widened permissions"
  - "M1 = BLOCKED: git transport to github.com unreachable; origin/main shares no history with this branch; recipe persisted for a future host; CICD-01/02/03 stay Pending"

requirements-completed: []
# CICD-01/02/03 are declared by this plan's frontmatter but deliberately stay
# Pending: this plan cannot produce the run that proves them (M1 blocked). Plan
# 48-10 owns flipping them on corroborated run evidence only.

coverage:
  - deliverable: "IN-08 option F: closed base set documented in the ci.yml header"
    kind: command
    ref: "python probe on .github/workflows/ci.yml (trigger block unchanged, push.branches==[main], no paths filter, no non-blocking tokens) -> NO-GATE-WEAKENING-OK"
    status: pass
    human_judgment: false
  - deliverable: "IN-07 option A: verify job wired per the option's shape"
    kind: command
    ref: "python probe on .github/workflows/ci.yml -> VERIFY-WIRING-OK extra job ['verify'] (checkout first, composite call, zero step-level if:, runs verify-phase-48.sh, no job-level permissions)"
    status: pass
    human_judgment: false
  - deliverable: "WR-06 option D and M1 BLOCKED recorded verbatim for plan 48-10"
    kind: command
    ref: "grep '^WR-06: Option D$' / '^IN-08: Option F$' / '^IN-07: Option A$' / '^M1: BLOCKED' on 48-11-SUMMARY.md"
    status: pass
    human_judgment: false
  - deliverable: "Phase gate green on the final tree"
    kind: command
    ref: "bash verify-phase-48.sh -> rc 0, 11/11 partitions, exactly one 'ALL PASSED'"
    status: pass
    human_judgment: false
  - deliverable: "The verify job actually runs green on a GitHub-hosted runner"
    kind: judgment
    ref: "no CI run exists or is obtainable from this host (M1 blocked)"
    status: unproven
    human_judgment: true
    rationale: "Runner-side behavior (PyYAML preinstalled-or-installed, install.sh download, golangci-lint 2.12.2 preflight) is only statically decidable here; the first real run is the proof, per the persisted recipe."
  - deliverable: "The WR-06 push-path question (does the lint action fall back to whole-repo on push?)"
    kind: judgment
    ref: "unmeasured by design — option D defers the decision to the first real run's evidence"
    status: unproven
    human_judgment: true
    rationale: "golangci-lint-action@v9 cannot be executed on a push event locally; both candidate fixes are recorded so the decision re-opens on evidence, not speculation."
---

# Phase 48 Plan 11: Review-Finding Decisions + the M1 Run Recipe Summary

**One-liner:** Settled the three review findings that were decisions, not code — WR-06 deferred to first-run evidence with both fixes recorded, IN-08 documented as a closed base set, IN-07 wired as a fourth published `verify` job that runs the phase gate in CI — and recorded M1 as BLOCKED with a verbatim run recipe, leaving CICD-01/02/03 honestly Pending.

Duration: 11 min (2026-10-19T14:34:15Z -> 2026-10-19T14:45:00Z). Tasks: 3 (all checkpoint tasks, resolved by the user's recorded decisions). Files: 1 production (ci.yml) + 3 planning artifacts.

## Recorded Decisions (verbatim, for plan 48-10's ledger)

WR-06: Option D
IN-08: Option F
IN-07: Option A
M1: BLOCKED — git transport to github.com is unreachable from this host (connect timeout on port 443; a TLS handshake failure on a second attempt); origin/main is a single unrelated commit (369b08a) with an empty merge-base against this branch, so GitHub cannot open a PR between them; and ci.yml has never been on the remote default branch, so no run of it can exist until the history question is decided. CICD-01/02/03 stay Pending.

### Rationale — WR-06: Option D

The review could not execute `golangci-lint-action@v9` on a push event, so whether the push run is red is **unmeasured**; what is measured is that the same configuration reports 45 findings when run whole-repo on this tree (errcheck 4, ineffassign 1, revive 27, staticcheck 8, unused 5). Option D refuses to trade one unknown for another: both candidate fixes are recorded here and the decision re-opens on the first real run's evidence. **Candidate A:** give the push run an explicit diff base — `args: --new-from-rev=${{ github.event.before }}` on the lint step so the whole-repo fallback never happens (risk: interaction with `only-new-issues: true`, unexercisable locally). **Candidate B:** gate the lint step to pull requests — recorded as **D-06-reversing and NOT chosen**: a skip-style `if:` on the lint gate is exactly the shape D-06 forbids and collides with plan 48-07's step-`if` rule, so it could only ever be a recorded D-06 amendment. No lint-step code change was made; the lint action, its `only-new-issues`, and the go job are byte-identical to before this plan.

### Rationale — IN-08: Option F

The `pull_request.branches: [main, 'phase-*']` filter has the same Pending-forever shape invariant 1 forbids for path filters, but on the base branch. Option F makes the closed set explicit next to invariant 1 instead of changing behavior: a PR based on any other long-lived branch starts no CI at all, and widening the set is now a documented header+trigger decision rather than a silent filter drop. Dropping the filter (option E) was the strictly-more-inclusive alternative and was not chosen; the trigger block is unchanged (`push.branches: [main]`, no path filters anywhere).

### Rationale — IN-07: Option A

The verifier was reachable only by a maintainer who remembered to run it. Option A adds a fourth published job name — a one-way choice under D-21, since branch protection matches job names — so verifier drift becomes a red check on every push and PR. The job follows the existing per-file permission model: no job-level permissions (inherits the workflow-level `contents: read` ceiling, strictly narrower than the go job's surface), checkout first, then the repository-local composite with `go: "true"` (and `node: "false"` — the script needs no Node), then the pinned linter, then `bash verify-phase-48.sh`. No `continue-on-error`, no step-level `if:` (the 48-07 rule), no widened `push.branches`, no `paths:` filter. The lint action is deliberately **not** reused in this job: it installs golangci-lint exactly v2.12.2 from the official install script onto `PATH` (the script's own preflight refuses any other binary), so the gate self-proof never inherits the action's push-run whole-repo fallback — the exact surface WR-06 leaves open. A PyYAML install step covers the gate's structural probes.

### Rationale — M1: BLOCKED

The one item no executor can produce from this host. Rather than fabricate, claim, or gate-weaken a run into existence, the block is recorded with its three named causes and the full run recipe below is persisted so a future host can execute it verbatim. This is the honest terminal state the plan defines, not a failure to work around.

## M1 Run Recipe (execute verbatim from a host with git transport)

1. **Decide the history question first.** `origin/main` is a single unrelated commit (`369b08a`) with an empty merge-base against this branch, so GitHub cannot open a PR between them. Choose one: replace the public `main` with this branch (force-push or point `main` at this work), treat this branch as the new default branch, or move the work to a fresh repository. Repository-visible decision — belongs to the repository owner.
2. **Push from a host with working git transport.** This host cannot reach github.com over git (`Failed to connect to github.com port 443`; a TLS handshake failure on another attempt). The `gh` CLI reaches the API but cannot push.
3. **Trigger a run through a path that does not weaken a gate.** Either open a PR whose base is `main` (or `phase-*`), or land the commit on `main` and let the `push` path run. Do **not** widen `push.branches`, add `workflow_dispatch:`, add a `paths:` filter, add `continue-on-error`, or add `|| true` — every one of those makes the run green by running less. With IN-07 implemented, the run now carries a fourth job: require `verify=success` alongside the three.
4. **Verify the run:**
   ```bash
   gh run list --workflow ci.yml --branch <branch> --limit 1 --json databaseId -q '.[0].databaseId'
   gh run watch <run-id> --exit-status
   gh run view <run-id> --json jobs -q '.jobs[] | "\(.name)=\(.conclusion)"'
   ```
   Require `go=success`, `fe=success`, `docker=success` (and `verify=success`).
5. **If a job is red, classify before changing anything** — flake (reproduce under WSL with the exact commands and quote the `-test.shuffle <seed>`), CGO/duckdb linkage on the runner, a cold-cache-only Docker failure (re-run once and record the difference), a platform-specific FE test (record the test name; do not edit the assertion first), or a verify-job environment gap (e.g. PyYAML/install.sh — fix by adding the missing dependency, never by relaxing the gate). A persistent red is a real break; it is never fixed by relaxing a gate.
6. **Report back:** run id, the triggering branch, the four conclusions, wall-clock duration, and any classification. The WR-06 decision (option D) re-opens on this evidence: if the `go` job is red on a push run, implement candidate A; if it is green, the action did not fall back to whole-repo lint and the file's comment stands.

## Handoff to Plan 48-10 (owns 48-SECURITY.md / 48-REVIEW-DISPOSITION.md)

- **WR-06 deferral** — record option D as an explicit deferral in `48-SECURITY.md`'s "What this audit does NOT establish": the push-path lint scope is unmeasured, both candidate fixes are named above, and the decision re-opens on the first run's evidence. The review-disposition row for WR-06 should read `deferred` with the missing measurement named.
- **IN-08 option F** — implemented (header documentation, no behavior change); record as fixed-and-implemented, not deferred.
- **IN-07 option A** — implemented (the `verify` job); record as fixed-and-implemented.
- **M1 BLOCKED** — the Class A statement in `48-SECURITY.md` must say no CI runner has executed the workflows; REQUIREMENTS.md keeps CICD-01/02/03 `Pending` (already true — not flipped by this plan); the ROADMAP "Blocked on" line states the transport truth. All three fact patterns are 48-10 Task 3/4 inputs and match this file's `M1:` line.

## Verification Results

| Check | Result |
|-------|--------|
| Baseline gate before edits | `bash verify-phase-48.sh` rc 0, 11/11 partitions, exactly one `ALL PASSED` |
| Gate after IN-08 F header edit | rc 0, `ALL PASSED`; trigger block byte-identical, `NO-GATE-WEAKENING-OK` probe pass |
| Gate after IN-07 A verify job | rc 0, 11/11 partitions, exactly one `ALL PASSED`; `VERIFY-WIRING-OK extra job ['verify']` |
| Plan task-1 verify probes | decision-line greps + no-gate-weakening probe — pass (lines above) |
| Plan task-2 verify probe | extra-job shape assertions — pass |
| Plan task-3 verify probe | `M1: BLOCKED` line present; REQUIREMENTS.md still `grep "CICD-0[123] \| Phase 48 \| Pending"` — pass |
| Verifier co-update | **Not needed — no partition tripped.** P5's universal checkout-first + composite rule and P8's per-file trust-surface scan already cover the new job generically; the job-specific shape is asserted by the plan's task-2 probe quoted above. No partition count, assertion text, or mutation probe was changed in `verify-phase-48.sh`. |

## Deviations from Plan

None - plan executed exactly as written. The two implementation details inside option A's declared shape (passing `node: "false"` to the composite, and the PyYAML install step) are prerequisites of the option's own step list ("ensure pinned golangci-lint on PATH -> bash verify-phase-48.sh"), not scope additions: neither weakens a gate, adds a conditional, or widens permissions.

## Issues Encountered

- **M1 blocked (by design, recorded).** Not a deviation — it is the plan's own defined terminal state for an unobtainable run. Tracked in REQUIREMENTS.md (Pending), STATE.md Blockers, and the WINDOWS.md ledger (one `unrun-verify` entry appended).
- **Commit message defect, fixed.** Task 2's commit message lost the word "verify" to shell command substitution (unescaped backticks); the commit was amended before any further work — content was never affected, only the message.

## Authentication Gates

None. Git transport to github.com is unreachable (a network fact, not a credential prompt); `gh` API access works but cannot push, so no auth step was available to perform.

## Known Stubs

None. No placeholder code, no unwired data paths; the only unproven item is the runner-side execution of the new job, which is recorded under coverage as human-judgment-pending rather than stubbed.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: supply-chain | .github/workflows/ci.yml | The `verify` job fetches and executes the official golangci-lint install script from raw.githubusercontent.com (curl piped to sh), pinned to v2.12.2 and independently checked by the gate's own version preflight. This network-fetch-and-execute surface is new relative to the plan's STRIDE register (T-48-42 covers the job-name surface, not the fetch); flagged for 48-10's security pass. |

## Self-Check: PASSED

- `48-11-SUMMARY.md` exists and carries the literal lines `WR-06: Option D`, `IN-08: Option F`, `IN-07: Option A`, and `M1: BLOCKED — …` at line start (greps re-run post-write).
- Commits verified on this branch: `2d2940d` (IN-08 F) and `e112488` (IN-07 A) both ancestors of HEAD; metadata commit carries this file, STATE.md, and WINDOWS.md.
- `.planning/REQUIREMENTS.md`: CICD-01/02/03 still `Pending` in both the checklist and the mapping table — not flipped (matches the recorded `M1: BLOCKED`).
- Final tree: `bash verify-phase-48.sh` rc 0, 11/11 partitions, exactly one `ALL PASSED`.
- No `paths:`/`paths-ignore:` key, no `continue-on-error`, no `workflow_dispatch:`, no `|| true`, no widened `push.branches` anywhere in ci.yml (probe re-run on the final tree).
