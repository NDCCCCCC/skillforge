---
phase: 48-ci
plan: 24
subsystem: ci
tags: [github-actions, ci-gate, verification, evidence-binding, comment-stripping, bash, python]

requires:
  - phase: 48-ci
    provides: verify-phase-48.sh with 11 partitions, per-assertion evidence tokens, the 48-23 attributing P11 fixture loop and the P3-INSTENV-OK env pin; the round-5 verification report and its mutation table (D0, R1-R14, G1-G4)
provides:
  - P4 config-verify group carries its own evidence token P4-CONFIG-OK (T-48-81): appended to p4-config.log only after the rc==0 refusal, joined into the p4.log concatenation behind the same -f guard as the sentinel logs, and required FIRST by ok_partition P4; the block is wrapped in the column-0 marker range "# T-48-81: config verify evidence binding" and deleting the range reds the gate naming P4 and the missing token
  - P1 pull_request_target scan judges comment-stripped text per enumerated file (IN-R5-03): hits report as native path:line with the matched stripped line, a positive canary proves the probe is live before any PASS deposits, and the decisive success (scanned == enumerated, hits == 0) is asserted before P1-NO-PRT deposits; a comment naming the event no longer false-fires the gate while a real trigger still reds with file:line
  - Dead code removed (IN-R5-05): the P2/P3 probe imports only sys, yaml; scan_trust_surface carries exactly one basename binding, hoisted ABOVE the parsed-secret-walk marker range so the R10 drill leaves the textual scan's binding intact
  - Round-6 regression record: the full recorded matrix (D0, R1-R14, G1-G4) re-run on the final tree — D0 green, all 19 mutations exit 1 with their recorded partitions/fixtures named; no recorded drill changed outcome
affects: [48-ci round-6 verification, 48-25 (security register rows T-48-81/T-48-82), 48-26]

actuals:
  tokens: 3100
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Live-probe canary: a scan probe is proven capable of reporting a hit (rc 1 on a synthetic positive) before its clean verdict can deposit PASS — closes the always-succeeds neutering direction a pure rc==0 assertion leaves open"
    - "Bindings used by a marker-ranged block AND its fallback must be hoisted OUTSIDE the range: the R10 drill deletes the parsed secret walk, so a variable defined inside it dies with the drill"

key-files:
  created:
    - .planning/phases/48-ci/48-24-SUMMARY.md
  modified:
    - verify-phase-48.sh

key-decisions:
  - "The single basename binding is hoisted above the # T-48-04 walk marker range, not kept at the plan's stated 'first' site: the first site sits inside the marker range the R10 drill deletes, so keeping it crashed the textual scan with a NameError and changed R10's recorded verdict (fixed at the source, drill untouched)"
  - "The P1 probe keeps raw-text file:line reporting on the NATIVE path, so the negative drill's zz-inr503-prt.yml:<line> evidence and the existing sed-indent rendering both work unchanged"
  - "The decisive-success assertion counts decisive probes against the enumeration (scanned == WF_COUNT) and requires hits == 0, mirroring P1's P1-PARSED count discipline; the positive canary covers the probe-hardwired-to-succeed direction the flag alone cannot"
  - "requirements mark-complete deliberately not run: REQUIREMENTS.md already carries Complete (run 39380373930) and the plan forbids requirement-status changes (same expected no-op as 48-22/48-23)"
  - "STATE.md / ROADMAP.md deliberately not touched: the dispatch objective assigns those writes to the orchestrator"

patterns-established:
  - "Live-probe canary before a clean-scan verdict deposits"
  - "Drill-survivable bindings: anything a marker-range deletion drill removes must not carry variables consumed outside the range"

requirements-completed: [CICD-01, CICD-02, CICD-03]

coverage:
  - id: D1
    description: P4 config-verify group self-evidencing — T-48-81 marker range, P4-CONFIG-OK appended after the rc refusal, guarded join into p4.log, required first at ok_partition P4 (IN-R5-02b)
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh — exit 0, exactly one ALL PASSED, P4-CONFIG-OK named in the P4 done_partition line"
        status: pass
      - kind: integration
        ref: "Marker-range deletion drill (48-22 sandbox shape, anchor asserted) — gate exit 1: \"partition P4 ... does not contain the probe-emitted token(s) 'P4-CONFIG-OK'\""
        status: pass
    human_judgment: false
  - id: D2
    description: Comment-stripped per-file pull_request_target scan with path:line reporting, live-probe canary and asserted decisive success (IN-R5-03)
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh — exit 0, one ALL PASSED; P1-NO-PRT present and 'grep -rn' count in the gate source is 0"
        status: pass
      - kind: integration
        ref: "Comment-only fixture drill (valid workflow, trigger named only inside a comment) — full gate exit 0 (COMMENT-ONLY-FIXTURE-GREEN)"
        status: pass
      - kind: integration
        ref: "Real-trigger fixture drill — gate exit 1 with 'zz-inr503-prt.yml:3:' and the matched line text in the output"
        status: pass
    human_judgment: false
  - id: D3
    description: Dead code removed — P2/P3 probe import reduced to sys, yaml; exactly one basename binding in scan_trust_surface, drill-survivable (IN-R5-05)
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "Anchored checks — p23 probe body has no re import/usage, exactly one 'import re, sys, yaml' left in the file (the P6 probe), count_phrase = re.compile intact, exactly one basename assignment"
        status: pass
      - kind: integration
        ref: "bash verify-phase-48.sh — exit 0, one ALL PASSED (stray-secret and quoted-hash-secret still rejected via the parsed walk in the D0 fixture log)"
        status: pass
    human_judgment: false
  - id: D4
    description: Full recorded drill matrix re-run on the final tree and recorded — D0 control green, R1-R14 and G1-G4 all exit 1 with recorded attributions
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "20 sandbox drills (asserted pre-mutation anchors): D0 exit 0 one ALL PASSED; R1 P2 four tokens missing; R2 P3 BLOCKING/LINT/VERIFYJOB/INSTENV missing; R3 P1 COMPOSITE-OK; R4 P4 A-ERRCHECK; R5 P6 JOB-IF; R6 P7 CACHE; R7 P9 NOTICE; R8 banner-after-else; R9 quoted-paths returned-success; R10 tojson-secrets returned-success; R11 quoted-paths no-workflow-file; R12 quoted-paths named; R13/R14 linter install env got/want; G1 P6 fe if; G2 P7 docker if; G3 P2 determinism pipe; G4 P9 Gate-check env — every verdict matches the round-5 record"
        status: pass
    human_judgment: false

duration: 33min
completed: 2026-10-21
status: complete
plan_head_before: 5bea15c999c0732ce3fb683af87f662a63c56dde
plan_head_after: df88aa35c23176323dbc69216876719a3d2c9192
commits: 3
---

# Phase 48 Plan 24: Round-6 Gap Closure — Self-Evidencing P4 Config Group + Comment-Stripped Trigger Scan Summary

**Every remaining unbound assertion group now carries its own evidence (P4-CONFIG-OK, T-48-81), the P1 trigger scan judges comment-stripped text so prose cannot red the gate while a real trigger still reds with file:line, the round-5 dead code is gone, and the full recorded drill matrix (D0, R1-R14, G1-G4) is re-proven on the final tree with no outcome changed.**

## Performance

- **Duration:** 33 min
- **Started:** 2026-10-21T00:00:39Z
- **Completed:** 2026-10-21T00:33:17Z
- **Tasks:** 3/3
- **Files modified:** 1 (`verify-phase-48.sh`)

## Accomplishments

- **IN-R5-02b closed:** the `golangci-lint config verify` group is wrapped in the `# T-48-81: config verify evidence binding` column-0 marker range; `P4-CONFIG-OK` is appended to `p4-config.log` only after the rc-equals-zero refusal, reaches `p4.log` through the same missing-file-tolerant guard as the sentinel logs, and is required FIRST by `ok_partition "P4"`. Deleting the marker range now reds naming partition P4 and the missing token — not by luck of sentinel D's revive attribution.
- **IN-R5-03 closed:** P1's banned-trigger scan reads comment-stripped text, one probe per enumerated file (the same multiline strip idiom scan_trust_surface uses; line numbers preserved). Hits render as native `path:line: matched-line` through the existing sed-indent pattern. A positive canary proves the probe is live before any PASS deposits; decisive success is asserted (scanned == enumerated, hits == 0) before `P1-NO-PRT`. The comment-only fixture runs the full gate green; the real-trigger fixture reds with `zz-inr503-prt.yml:3:` evidence. P1's zero-file enumeration guard is untouched and still precedes the scan.
- **IN-R5-05 closed:** the P2/P3 probe imports only `sys, yaml` (the `re` left behind by round-4's split); scan_trust_surface carries exactly one `basename` binding — deliberately hoisted above the `# T-48-04` walk marker range so the R10 deletion leaves the retained textual SECRET_REF scan's binding intact. The P6 probe import is untouched.
- **Regression duty discharged:** the full recorded matrix was re-run on the final tree — D0 green (exit 0, one `ALL PASSED`, 11/11 registry, 14/14 fixtures), R1-R14 all exit 1 with their recorded partitions/fixtures named, G1-G4 all still bite. No recorded drill changed outcome.

## Recorded Round-6 Drill Matrix (final tree, sandbox copies, anchors asserted)

| Drill | Mutation | Result |
|-------|----------|--------|
| D0 | none (control) | exit 0, exactly one `ALL PASSED` |
| R1 | delete P2's assertion block | exit 1 — partition P2, tokens `P2-CMDS-OK P2-DET-OK P2-SHUFFLE-OK P2-HERMETIC-OK` missing |
| R2 | delete the P3 section | exit 1 — partition P3, `P3-BLOCKING-OK P3-LINT-OK P3-VERIFYJOB-OK P3-INSTENV-OK` missing |
| R3 | delete the P1 composite group | exit 1 — partition P1, `P1-COMPOSITE-OK` |
| R4 | delete P4 sentinel A | exit 1 — partition P4, `P4-A-ERRCHECK` |
| R5 | delete the P6 job-if group | exit 1 — partition P6, `P6-JOB-IF-OK` |
| R6 | delete the P7 cache group | exit 1 — partition P7, `P7-CACHE-OK` |
| R7 | delete the P9 notice group | exit 1 — partition P9, `P9-NOTICE-OK` |
| R8 | banner transposed after the else | exit 1 — `the passing banner sits after an else` |
| R9 | delete the parsed path-filter walk | exit 1 — `returned success on the 'quoted-paths' fixture` |
| R10 | delete the parsed secret walk | exit 1 — `returned success on the 'tojson-secrets' fixture` |
| R11 | delete the quoted-paths heredoc | exit 1 — `fixture 'quoted-paths' has no workflow file to scan` |
| R12 | path walk + both path fixtures | exit 1 — P11 names `quoted-paths` |
| R13 | `GOTOOLCHAIN: local` -> `auto` | exit 1 — `the linter install step env is {'GOTOOLCHAIN': 'auto', ...}` (P3) |
| R14 | delete the install env block | exit 1 — `the linter install step env is {}` (P3) |
| G1 | job-level if on the fe job | exit 1 — P6, fe job-level if |
| G2 | job-level if on the docker job | exit 1 — P7, docker job-level if |
| G3 | reintroduce the `-list \| grep` pipe | exit 1 — P2, determinism pipe shape |
| G4 | three-secret env on Gate check | exit 1 — P9, `Gate check` carries an env |

## Task Commits

1. **Task 1: bind the P4 config-verify group to its own evidence token** — `438bca8` (feat)
2. **Task 2: scan comment-stripped text for the banned trigger event** — `e123787` (feat)
3. **Task 3: remove the round-5 dead code and re-prove the full drill matrix** — `df88aa3` (refactor)

**Plan metadata:** SUMMARY committed separately (see git log).

## Files Created/Modified

- `verify-phase-48.sh` — T-48-81 marker range around the config-verify block with the `P4-CONFIG-OK` token, guarded p4-config.log join, P4 deposit/done_partition/banner updates; P1-C rewritten as a per-file comment-stripped probe with canary + decisive-success assertions; P2/P3 probe import reduced; single drill-survivable basename binding; no other partition, rule, message or token touched.

## Decisions Made

- **The basename binding is drill-survivable, not "first".** The plan said to keep the first of the two assignments — but that site sits inside the `# T-48-04` marker range the R10 drill deletes. The matrix caught the consequence live (R10 crashed with `NameError: basename` at P8 instead of reding with its recorded message) and the fix hoists the single assignment above the range; R10 then reproduces its recorded verdict exactly. See Deviation 1.
- **Live-probe canary in P1.** The old raw grep's neutering refusal (`rc -ne 1`) had no direct equivalent once the probe returns 0 on success, so the probe is proven live against a synthetic positive before any clean verdict deposits — the faithful translation of "a probe that could not produce a decisive result must not deposit PASS".
- **requirements mark-complete deliberately not run:** REQUIREMENTS.md already carries `Complete (run 39380373930)` and the plan forbids requirement-status changes (same expected no-op as 48-22/48-23).
- **STATE.md / ROADMAP.md untouched** per the dispatch objective (the orchestrator owns those writes).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] R10 regression — the surviving basename binding died with the drill's deletion range**
- **Found during:** Task 3 (matrix run)
- **Issue:** The plan's stated removal (keep the first `basename` assignment, drop the second) left the survivor inside the `# T-48-04: parsed secret walk` marker range. The R10 drill deletes that range, so the retained textual SECRET_REF scan crashed with `NameError: name 'basename' is not defined` and R10 red at P8 with a traceback instead of its recorded `returned success on the 'tojson-secrets' fixture` verdict — exactly the "recorded drill no longer bites" regression the plan's STOP rule names.
- **Fix:** hoisted the single `basename = os.path.basename(p)` above the walk's marker range (still inside the per-file loop), removed the in-walk original; both secret layers resolve it and the walk's deletion leaves it intact. Fixed at the source; the drill was never weakened.
- **Files modified:** verify-phase-48.sh (scan_trust_surface)
- **Verification:** R10 re-run — exit 1 with the recorded message; basename count exactly 1; healthy gate green; full matrix re-run on the final tree with all 19 mutations matching their recorded verdicts
- **Committed in:** `df88aa3`

**2. [Rule 3 - Blocking] Task 3's absolute-line-number verify commands were stale after Task 2's insert**
- **Found during:** Task 3 (verification)
- **Issue:** `sed -n '295,300p'` located the P2/P3 probe import at pre-Task-2 line numbers; Task 2's legitimate +53-line P1-C rewrite shifted the probe to line 348, so the check as written cannot pass on the correctly-implemented tree. Likewise, two literals in the message check (`returned success on the 'quoted-paths' fixture` / `'tojson-secrets'`) can never appear in the gate source — line 2006 parameterizes the fixture name (`'${fixture}'`); those exact strings exist only in drill output.
- **Fix:** anchored equivalents — the p23 heredoc line anchors the import check and its body span (no `re` import/usage); exactly one `import re, sys, yaml` remains in the file (the P6 probe) with `count_phrase = re.compile(` intact; the parameterized template plus the other three source literals are checked, and the two recorded runtime messages are proven byte-for-byte by the R9/R10 drills in the matrix.
- **Files modified:** none (verification-side only)
- **Verification:** DEAD-CODE-GONE and ROUND6-MESSAGES-PRESENT on the final tree; R9/R10 outputs carry the exact recorded messages
- **Committed in:** `df88aa3` (the source it verifies)

---

**Total deviations:** 2 auto-fixed (1 x Rule 1, 1 x Rule 3 — one real behavioral regression caught by the plan's own matrix STOP rule and fixed at the source, one stale-verification-command substitution)
**Impact on plan:** none on the goal — every drill the plan names bites as specified on the final tree, the healthy gate stays green, and the recorded round-6 matrix states that no drill changed outcome.

## Issues Encountered

None blocking. The initial R10 regression (Deviation 1) was caught by the matrix's STOP rule before any commit and fixed at the source; the entire matrix was then re-run on the final tree so the recorded table reflects the committed state, not an intermediate one.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Round-6 verification can re-run every drill in the table above against the committed tree; each recorded verdict holds.
- The healthy gate exits 0 with exactly one `ALL PASSED`, P4-CONFIG-OK required first at the P4 deposit, P3-INSTENV-OK present, 14/14 fixtures rejected for their registered reasons.
- Deferred (owned elsewhere): 48-SECURITY.md rows for T-48-81/T-48-82 land via plan 48-25 per the plan's threat-model note; the public-push record decision is plan 48-26's; IN-R5-01's parsed `with:`-mapping registry walk remains deferred behind a D-26 registry-allowlist decision.
- The gate remains local-green only: the current `verify` job definition has not run on a public runner since 630f875 (record boundary stated in 48-VERIFICATION.md round 5 still applies).

---
*Phase: 48-ci*
*Completed: 2026-10-21*
