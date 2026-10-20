---
phase: 48-ci
plan: 23
subsystem: ci
tags: [github-actions, ci-gate, supply-chain, trust-surface, verification, bash, python, pyyaml]

# Dependency graph
requires:
  - phase: 48-ci
    provides: verify-phase-48.sh with 11 partitions, per-assertion evidence tokens and the 48-22 parsed trust-surface walks (plans 48-20..48-22); the round-5 verification report (48-VERIFICATION.md gaps 1-2, mutation rows R11-R14)
provides:
  - Attributing P11 fixture loop: per-fixture registered rejection reason (case statement with refusing default arm), file-presence guard, explicit vacuity refusal, five per-class deposit tokens (P11-VACUITY-OK / P11-PATHFILTER-OK / P11-PERMSWRITE-OK / P11-PUBLICATION-OK / P11-SECRET-OK) required at ok_partition
  - 14 P11 fixtures (12 + quoted-push + dyn-secrets), every rejection attributed to its rule
  - Publication-token quoted variants push: 'true' and push: "true" (IN-R5-01 minimum fix)
  - Column-0 marker range around the parsed secret walk's secrets[ branch (# T-48-04: dynamic secrets[ branch / end) with its own fixture (IN-R5-02a)
  - Parsed exact-equality pin on the verify job's linter install-step env map, marker range # T-48-78: install env pin, evidence token P3-INSTENV-OK required at the P3 deposit (WR-R5-02)
  - Drill-proven reds for R11, R12, R13, R14 (round-5 reproductions now bite) and for the publication-revert / secrets-branch / env-assertion-deletion drills; R9/R10 keep their exact recorded messages; G1-G4 regression drills stay red
affects: [48-ci round-6 verification, 48-25 (security register cell correction), 48-26]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
actuals:
  tokens: 4930
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Layer-tolerant reason registration: a fixture catchable by two scan layers registers a substring both layers' messages contain, so a compound rule+fixture deletion still reaches the fixture that only the deleted layer caught"
    - "Class-token deposit: one success token per fixture class, all required at ok_partition — a deleted fixture class can no longer hide behind its siblings' evidence"

key-files:
  created:
    - .planning/phases/48-ci/48-23-SUMMARY.md
  modified:
    - verify-phase-48.sh

key-decisions:
  - "Registered rejection reasons are the empirically verified first lines (live baseline run), not the plan's read-derived sketch: yaml-ext is attributed to the publication token because `packages: write` is itself a PUBLICATION_TOKEN and the publication check runs before the WRITE_VALUE line scan"
  - "Layer-tolerant substrings for dual-layer fixtures (two-files -> path-filtered; stray-secret/quoted-hash-secret -> references secrets.) keep compound rule+fixture deletions attributable to the right fixture; single-layer fixtures (quoted-paths/space-paths/tojson-secrets/dyn-secrets) keep exact parsed-only substrings"
  - "The task-3 comment must not spell the joined passing-banner literal — the banner detector counts print sites in the gate's own source and a second occurrence reds P11"
  - "Parsed `with:`-mapping registry walk (IN-R5-01's other half) deliberately deferred: judging with.tags registries needs a D-26 registry-allowlist decision"
  - "requirements mark-complete deliberately not run: REQUIREMENTS.md already carries Complete (run 39380373930) in both places and the plan forbids requirement-status changes (same expected no-op as 48-22)"
  - "STATE.md / ROADMAP.md deliberately not touched: the dispatch objective assigns those writes to the orchestrator"

patterns-established:
  - "Per-class fixture evidence tokens in the trust-surface fixture loop"
  - "Marker-ranged assertion groups with their own fixture and their own deposit token (# T-48-04 dynamic secrets[ branch, # T-48-78 install env pin)"

requirements-completed: [CICD-01, CICD-02, CICD-03]

# Coverage metadata (#1602) — one entry per shipped deliverable.
coverage:
  - id: D1
    description: Attributing P11 fixture loop — per-fixture reason assertion with refusing default arm, file-presence guard, vacuity refusal, five per-class deposit tokens required at ok_partition
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh — exit 0, one ALL PASSED, 14/14 fixtures rejected for their registered reasons, all five class tokens printed"
        status: pass
      - kind: integration
        ref: "R11 sandbox drill (delete quoted-paths heredoc) — exit 1, 'fixture quoted-paths has no workflow file'"
        status: pass
      - kind: integration
        ref: "R12 sandbox drill (delete parsed path walk + both path fixtures) — exit 1 naming quoted-paths"
        status: pass
      - kind: integration
        ref: "R9/R10 sandbox drills — same recorded messages as 48-VERIFICATION round 5 (quoted-paths resp. tojson-secrets returned-success)"
        status: pass
    human_judgment: false
  - id: D2
    description: Publication-token quoted variants (push: 'true' / push: "true") plus the quoted-push fixture rejected by exactly that rule (IN-R5-01)
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "Healthy gate — 'fixture quoted-push rejected for registry/publication token' with token \"push: 'true'\" in the first line"
        status: pass
      - kind: integration
        ref: "Revert drill (delete the two variant lines) — exit 1, 'returned success on the quoted-push fixture'"
        status: pass
    human_judgment: false
  - id: D3
    description: secrets[ branch carries its own column-0 marker range and the dyn-secrets fixture only that branch catches (IN-R5-02a)
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "Healthy gate — 'fixture dyn-secrets rejected for dynamic secrets['"
        status: pass
      - kind: integration
        ref: "Branch drill (delete # T-48-04: dynamic secrets[ branch range) — exit 1, 'returned success on the dyn-secrets fixture'"
        status: pass
    human_judgment: false
  - id: D4
    description: Parsed exact-equality pin on the verify job's linter install-step env map with marker range # T-48-78: install env pin and the P3-INSTENV-OK deposit token (WR-R5-02)
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "Healthy gate — P3-INSTENV-OK printed and required by ok_partition P3"
        status: pass
      - kind: integration
        ref: "R13 drill (GOTOOLCHAIN: auto) — exit 1 naming the linter install step env with got/want maps"
        status: pass
      - kind: integration
        ref: "R14 drill (delete the three env lines) — exit 1, got env {}"
        status: pass
      - kind: integration
        ref: "Assertion-deletion drill — exit 1 naming partition P3 and the missing P3-INSTENV-OK token"
        status: pass
      - kind: integration
        ref: "Extra-key widening drill — exit 1 naming the linter install step env"
        status: pass
    human_judgment: false

# Metrics
duration: 29min
completed: 2026-10-21
status: complete
plan_head_before: 8f3d845bdedc86f4486b9a003e3a4287b9a2853e
plan_head_after: 550973022bbf313d32c0956a275a74ea1aaa402d
commits: 3
---

# Phase 48 Plan 23: Round-6 Gap Closure — Attributing Fixture Loop + Install-Env Pin Summary

**Both round-5 must-have gaps closed at the mechanism level: every P11 fixture rejection is now attributed per fixture and per class (R11/R12 red), and the declared install surface is pinned by an exact-equality assertion whose own deletion reds the gate (R13/R14 red) — the healthy gate stays green with 14/14 fixtures rejected.**

## Performance

- **Duration:** 29 min
- **Started:** 2026-10-20T23:21:07Z
- **Completed:** 2026-10-20T23:49:59Z
- **Tasks:** 3/3
- **Files modified:** 1 (`verify-phase-48.sh`)

## Accomplishments

- **Gap 1 (WR-R5-01, truth 12 FAILED) closed:** the P11 fixture loop registers a required first-line rejection reason per fixture (case statement with a refusing default arm), refuses a fixture directory with no workflow file before the scan runs, names the vacuity class explicitly for non-empty-dir fixtures, and deposits five per-class tokens that ok_partition all requires. R11 (delete the quoted-paths heredoc) and R12 (delete the parsed path walk together with both its fixtures) now exit 1 naming the fixture; R9/R10 keep their exact round-5 recorded messages.
- **Gap 2 (WR-R5-02, truth 9 PARTIAL) closed:** the P2/P3 probe now parses the verify job's linter install step and asserts its env map equals exactly `{GOTOOLCHAIN: local, GOSUMDB: sum.golang.org, GOPROXY: https://proxy.golang.org,direct}` by whole-dict equality, wrapped in the `# T-48-78: install env pin` marker range and bound to the new `P3-INSTENV-OK` deposit token. R13 (flip to auto), R14 (block deletion) and the assertion-deletion drill each exit 1; an extra-key widening also reds.
- **Advisory fixes carried:** IN-R5-01's minimum (publication-token quoted variants + attributing fixture) and IN-R5-02a (secrets[ branch marker range + its own fixture); fixture count 12 -> 14 in the count check, done_partition text and closing banner.
- **No recorded drill changed outcome:** R9/R10 messages preserved byte-for-byte; G1-G4 regression drills all still exit 1 with the right partition named.

## Task Commits

Each task was committed atomically:

1. **Task 1: attribute every P11 fixture rejection** — `31407ee` (feat)
2. **Task 2: publication-token quoted variants + secrets[ branch fixture** — `1e4f3ab` (feat)
3. **Task 3: pin the install-step env map exactly with its own evidence token** — `5509730` (feat)

**Plan metadata:** SUMMARY committed separately (see git log).

## Files Created/Modified

- `verify-phase-48.sh` — P11 fixture loop reworked (reason map, file-presence guard, vacuity refusal, per-class tokens, 14 fixtures); `PUBLICATION_TOKENS` gains the two quoted push spellings; the secrets[ branch carries column-0 drill markers; the P2/P3 probe gains the install-env exact-equality assertion and the P3 deposit gains `P3-INSTENV-OK`; P3/P11 done_partition texts and closing-banner lines updated.

## Decisions Made

- **Registered reasons are empirical, not sketched.** The plan's yaml-ext mapping (`contains a write scope`) is factually impossible: `packages: write` is itself a `PUBLICATION_TOKEN`, and the publication check runs before the WRITE_VALUE line scan, so the fixture's actual first line names the publication token. A live baseline run of the committed tree supplied every registered substring (Rule 1 fix, see Deviations).
- **Layer-tolerant substrings for dual-layer fixtures.** two-files registers `path-filtered` (both path-filter layers' messages contain it) and stray-secret/quoted-hash-secret register `references secrets.` (both secret layers), so after a parsed-walk deletion the loop still attributes earlier fixtures to their RULE and reaches the fixture only the deleted layer caught — which is exactly what the R12/R10 drills demand. Single-layer fixtures (quoted-paths, space-paths, tojson-secrets, dyn-secrets) keep exact parsed-only substrings; they are the attribution anchors for R9/R10/the branch drill.
- **IN-R5-01's parsed half deferred by plan instruction:** judging `with.tags` registries needs a D-26 registry-allowlist decision (D-26 territory); only the textual quoted-variant minimum was implemented.
- **requirements mark-complete deliberately not run:** REQUIREMENTS.md already carries `Complete (run 39380373930)` in both places and the plan forbids requirement-status changes (same expected no-op as 48-22).
- **STATE.md / ROADMAP.md untouched** per the dispatch objective (the orchestrator owns those writes).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] yaml-ext's registered reason corrected to the actual first line**
- **Found during:** Task 1 (baseline run before editing)
- **Issue:** The plan's mapping said `yaml-ext -> contains a write scope` (the WRITE_VALUE line scan), but the scan's check order puts the publication-token rule first, and `packages: write` is a PUBLICATION_TOKEN — the actual first line is `contains forbidden registry/publication token 'packages: write'`. Registering the plan's substring would have failed the healthy gate immediately.
- **Fix:** registered `registry/publication token` for yaml-ext (class token stays P11-PERMSWRITE-OK per plan).
- **Files modified:** verify-phase-48.sh (case statement)
- **Verification:** healthy gate green; yaml-ext line shows the publication-token message
- **Committed in:** `31407ee`

**2. [Rule 1 - Bug] two-files registered with the layer-tolerant substring `path-filtered`**
- **Found during:** Task 1 (R12 drill run)
- **Issue:** With the parsed path walk deleted (the R12 compound mutation), two-files' unquoted `paths:` key is still caught by the retained textual rule, whose message (`contains paths:`) lacks the plan-registered substring (`with a 'paths' key`) — the drill red-foamed at two-files with wrong-reason instead of naming quoted-paths as its acceptance criterion requires.
- **Fix:** two-files registers `path-filtered`, the substring both layers print; quoted-paths/space-paths keep the parsed-only substring.
- **Files modified:** verify-phase-48.sh (case statement)
- **Verification:** R12 drill exits 1 naming quoted-paths; healthy gate green with two-files rejected for `path-filtered`
- **Committed in:** `31407ee`

**3. [Rule 1 - Bug] stray-secret / quoted-hash-secret registered with the layer-tolerant substring `references secrets.`**
- **Found during:** Task 1 (R10 drill run)
- **Issue:** Same class as deviation 2: with the parsed secret walk deleted, stray-secret is still caught by the retained textual scan, whose message (`references secrets.DEPLOY_KEY`) lacks the `parsed value ` prefix — the R10 drill red-foamed at stray-secret instead of reaching tojson-secrets with its recorded message.
- **Fix:** both fixtures register `references secrets.`.
- **Files modified:** verify-phase-48.sh (case statement)
- **Verification:** R10 drill exits 1 with `returned success on the 'tojson-secrets' fixture` (round-5 recorded message preserved); healthy gate green
- **Committed in:** `31407ee`

**4. [Rule 1 - Bug] Task 3 comment spelled the joined passing-banner literal**
- **Found during:** Task 3 (healthy-gate run)
- **Issue:** The new WR-R5-02 comment contained the literal `ALL PASSED`; the P11 banner detector counts occurrences of that runtime-assembled token in the gate's own source and refuses at 2 — the gate red-foamed `the passing banner has 2 print sites`.
- **Fix:** comment reworded ("exit 0 under the passing banner"); the joined literal remains single-site.
- **Files modified:** verify-phase-48.sh (comment only)
- **Verification:** healthy gate exit 0 with exactly one `ALL PASSED`
- **Committed in:** `5509730`

---

**Total deviations:** 4 auto-fixed (4 x Rule 1 — all message/attribution corrections proven by drill or baseline runs, none change rule text or weaken any trust-surface rule)
**Impact on plan:** none on the goal — every drill the plan names bites as specified, and the healthy gate stays green; the deviations replace two factually wrong reason substrings with the empirically verified ones and keep compound-deletion attribution reachable.

## Issues Encountered

None blocking. The `.planning/STATE.md` modification visible in the worktree during execution is the orchestrator's own in-flight position write (predates this plan's commits); it was deliberately left unstaged and untouched.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Round-6 verification can reproduce every recorded drill: R11-R14 now red (previously green), R9/R10 keep their messages, and the three new drills (publication revert, secrets[ branch, env assertion) each red with partition/fixture/token named. G1-G4 stay red.
- The healthy gate exits 0 with exactly one `ALL PASSED`, 11/11 registry, 14/14 fixtures rejected for their registered reasons, five P11 class tokens and P3-INSTENV-OK present.
- Deferred (owned elsewhere): IN-R5-01's parsed `with:`-mapping registry walk (needs a D-26 registry-allowlist decision); the 48-SECURITY.md T-48-78 cell correction is plan 48-25's, per the plan's threat-model note; the round-6 record-boundary decision is plan 48-26's.
- The gate remains local-green only: the current `verify` job definition has not run on a public runner (record boundary stated in 48-VERIFICATION.md round 5 still applies).

---
*Phase: 48-ci*
*Completed: 2026-10-21*
