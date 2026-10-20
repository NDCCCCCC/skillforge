---
phase: 48-ci
plan: 22
subsystem: ci
tags: [github-actions, ci-gate, trust-surface, yaml-parsing, mutation-testing, golangci-lint, threat-register, verifier-hardening]

# Dependency graph
requires:
  - phase: 48-21
    provides: the token-list ok_partition and the P11 half-2 fixture loop (eight fixtures, count check) this plan extends to twelve
  - phase: 48-20
    provides: the per-assertion-group token pattern whose mitigations T-48-70..T-48-72 register
provides:
  - parsed-document trust-surface rules: a parsed `on:` walk rejecting `paths`/`paths-ignore` on every event configuration mapping, and a recursive parsed-string walk rejecting out-of-allowlist `secrets.NAME`, `secrets[` and `toJSON(secrets` — both marker-bracketed (`# T-48-03:` / `# T-48-04:`) and layered OVER the retained, unchanged textual rules
  - four new P11 evasion fixtures (quoted-paths, space-paths, tojson-secrets, quoted-hash-secret); twelve fixture directories, fixture count 12
  - ci.yml verify-job install step declaring GOTOOLCHAIN=local / GOSUMDB / GOPROXY, with the corrected module-proxy comment and the GOTOOLCHAIN=local satisfiability argument
  - the two-file coordinated-bump note beside LINT_VERSION (exact sentence: "Bump together with LINT_VERSION in verify-phase-48.sh whenever the pinned golangci-lint version changes:")
  - 48-REVIEW-DISPOSITION-R1.md historical annotation (`Historical note (superseded 2026-10-20)` + `**Superseded text (2026-10-19):**` label, run 39380373930 retained)
  - 48-SECURITY.md rows T-48-70..T-48-79 (31 rows, 0 open, status verified, 31/31/0 audit trail + metric table, historical token-names marker)
affects: [verify-phase-48.sh, .github/workflows/ci.yml, round-5 mutation evidence, Phase 48 verification re-run]

# Actuals (#2632)
actuals:
  tokens: 7523
  tasks: 3
  commits: 3
commits: 3
plan_head_before: 8a4005780b16661b264c5cdd4a7c28dec11672c9
plan_head_after: 05ceef8c2e7302ec3ed659d62c8dd63842a3d701

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "column-0 marker comments bracket a probe rule so a range-deletion drill can delete exactly that rule — Python ignores comment indentation, so the markers sit at column 0 inside an indented block and the drill's startswith still matches"
    - "parsed-document primary + retained-textual second layer: the old text rule stays byte-identical beneath the new parsed walk, so a deletion drill's attribution loop runs past the fixtures the text rule still catches and reds on the fixture only the parsed walk was catching"

key-files:
  created: []
  modified:
    - verify-phase-48.sh
    - .github/workflows/ci.yml
    - .planning/phases/48-ci/48-REVIEW-DISPOSITION-R1.md
    - .planning/phases/48-ci/48-SECURITY.md

key-decisions:
  - "Both parsed walks are layered OVER the retained textual rules rather than replacing them: the pre-existing two-files fixture (unquoted `paths:`) and stray-secret fixture (plain `secrets.DEPLOY_KEY`) stay rejected with the walks deleted, so the P11 fixture loop reaches quoted-paths / tojson-secrets and each deletion drill attributes to the exact fixture only the deleted walk was catching"
  - "Drill markers sit at column 0 inside the indented Python probe: comment-only lines do not participate in Python's indentation structure, so the markers are simultaneously valid probe syntax and startswith-matchable range delimiters (the first drill attempt failed with indented markers — deleted nothing — and was fixed before commit)"
  - "T-48-75 is registered `medium` because 48-21-PLAN.md's own threat table declares it medium — the declaring plan and the registering record state the same severity for the same ID"
  - "requirements mark-complete deliberately not run: ready-ids returned all three IDs, but REQUIREMENTS.md already carries `Complete (run 39380373930)` in both places and the plan forbids requirement-status changes — marking would overwrite the canonical run-id status text with a bare Complete"
  - "The Approval paragraph keeps its history inline (`earlier approvals: 21/21 at the 48-19 reconciliation`) instead of deleting the prior approval record"

patterns-established:
  - "Parsed-vs-textual layering for trust-surface rules: judge the document GitHub parses as primary, keep the comment-stripped text scan as an unchanged second layer, and pin each evasion shape as a fixture whose rejection is drill-attributable"

requirements-completed: [CICD-01, CICD-02, CICD-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Parsed-document trust surface: parsed `on:` path-filter walk and recursive parsed-string secret walk in scan_trust_surface, layered over the retained textual rules; four evasion fixtures; 12-fixture loop; both deletion drills red P11 with fixture attribution"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "bash verify-phase-48.sh (exit 0, exactly one ALL PASSED, registry 11/11, all 12 fixtures rejected)"
        status: pass
      - kind: integration
        ref: "gate log grep: fixture 'quoted-paths'/'space-paths'/'tojson-secrets'/'quoted-hash-secret' rejected lines all present (NEW-FIXTURES-REJECTED)"
        status: pass
      - kind: integration
        ref: "sandbox drill: delete '# T-48-03: parsed path-filter walk'..'end' range -> exit 1, FAIL P11 'scan_trust_surface returned success on the 'quoted-paths' fixture'"
        status: pass
      - kind: integration
        ref: "sandbox drill: delete '# T-48-04: parsed secret walk'..'end' range -> exit 1, FAIL P11 'scan_trust_surface returned success on the 'tojson-secrets' fixture'"
        status: pass
    human_judgment: false
  - id: D2
    description: "Declared linter install environment: GOTOOLCHAIN/GOSUMDB/GOPROXY env on the verify job's install step, corrected module-proxy comment with satisfiability argument, two-file coordinated-bump note beside LINT_VERSION"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "grep: GOTOOLCHAIN: local + GOSUMDB: sum.golang.org + GOPROXY line + unchanged go install @v2.12.2 + stale no-network claim absent (LINT-INSTALL-ENV-OK)"
        status: pass
      - kind: integration
        ref: "grep: 'Bump together with' in verify-phase-48.sh (full sentence, single line), LINT_VERSION + verify-phase-48.sh named in ci.yml (LINT-VERSION-XREF-OK)"
        status: pass
      - kind: integration
        ref: "bash verify-phase-48.sh (exit 0, one ALL PASSED — P3/P8/P9 accept the new env block)"
        status: pass
    human_judgment: false
  - id: D3
    description: "R1 ledger historical annotation + round-5 register: annotation precedes the retained superseded paragraph and names run 39380373930; T-48-70..T-48-79 registered once each at 0 open; 31/31/0 arithmetic, audit trail, Sign-Off, Approval and 2026-10-20 metric table; STRUCT-OK/EVALGATE-OK narrative marked historical"
    requirement: CICD-01
    verification:
      - kind: integration
        ref: "grep set on 48-REVIEW-DISPOSITION-R1.md: annotation line 62 < 'remain Pending' line 70, run id present, 'Complete (run 39380373930)' literal present, 'Superseded text (2026-10-19)' present, file tracked (R1-HISTORICAL-ANNOTATED)"
        status: pass
      - kind: integration
        ref: "grep set on 48-SECURITY.md: ten IDs each exactly once, T-48-75 medium, threats_open: 0, status: verified, '31 rows', '31/31 closed, 0 open', 31/31 metric table, token-names historical marker (ROUND5-REGISTER-OK)"
        status: pass
      - kind: integration
        ref: "bash verify-phase-48.sh post-commit (exit 0, one ALL PASSED); git diff vs plan base touches exactly the four files_modified"
        status: pass
    human_judgment: false

# Metrics
duration: 17 min
completed: 2026-10-20
status: complete
---

# Phase 48 Plan 22: Parsed-Document Trust Surface, Declared Install Env, R1 Annotation + Round-5 Register Summary

**The verifier's path-filter and secret rules now judge the YAML document GitHub parses — quoted `"paths":`, spaced `paths :`, `toJSON(secrets)` and a quoted `#`-hidden secret are all pinned as rejected P11 fixtures with deletion drills attributing to each — the linter install declares its GOTOOLCHAIN/GOSUMDB/GOPROXY surface, and the last unannotated record (R1) is historical with the round-5 threats registered at 31 rows / 0 open.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-10-20T14:34:51Z
- **Completed:** 2026-10-20T14:52:08Z
- **Tasks:** 3 of 3
- **Files modified:** 4 (cumulative diff vs plan base: 235 insertions, 21 deletions)

## Accomplishments

- Closed 48-VERIFICATION.md advisory 2 / 48-REVIEW.md WR-02: `scan_trust_surface` gains a parsed `on:` walk (read via `doc.get("on", doc.get(True))`) that rejects `paths`/`paths-ignore` keys on every event configuration mapping, and a recursive walk over every parsed string value that rejects out-of-allowlist `secrets.NAME`, `secrets[` and `toJSON(secrets` — both marker-bracketed as `# T-48-03:`/`# T-48-04:` ranges, with the retained textual `paths:`/`paths-ignore:` rule and `SECRET_REF` scan kept byte-identical as second layers.
- The four WR-02 evasion shapes are pinned as P11 fixtures (`quoted-paths`, `space-paths`, `tojson-secrets`, `quoted-hash-secret`), created by the same extended `mkdir -p` list (twelve directories), appended at the END of the fixture loop, with the expected count raised 8 → 12 ("4 vacuity plus 8 per-file").
- Both mutation drills bite with exact attribution: deleting the parsed path walk reds P11 with `scan_trust_surface returned success on the 'quoted-paths' fixture`; deleting the parsed secret walk reds P11 naming `tojson-secrets` — possible only because the retained text rules keep `two-files` and `stray-secret` rejected so the loop reaches the right fixture.
- Closed advisory 3 / IN-01: the verify job's `install pinned golangci-lint` step declares `GOTOOLCHAIN: local`, `GOSUMDB: sum.golang.org` and `GOPROXY: "https://proxy.golang.org,direct"`; the overstated "no network fetch-and-execute surface" claim is replaced by the accurate module-proxy statement plus the satisfiability argument (composite installs Go 1.25.5 from go.mod; golangci-lint v2.12.2 requires go 1.25.0). Install command, `@v2.12.2` pin, `[ -x ]` postcondition and `$GITHUB_PATH` append unchanged.
- Closed advisory 4: the coordinated-bump note beside `LINT_VERSION` carries the exact required sentence and names both ci.yml locations (go job's `golangci-lint-action` `version:`, verify job's `go install ...@v2.12.2`); P3 and the P4 preflight fail loudly on drift.
- Closed truth 15's PARTIAL half: 48-REVIEW-DISPOSITION-R1.md's closing paragraph is preceded by the `Historical note (superseded 2026-10-20)` annotation (transport succeeded, six runs, run `39380373930` head `630f875` four-green, CICD-01/02/03 `Complete (run 39380373930)` in both REQUIREMENTS places) and the original paragraph is retained under a `**Superseded text (2026-10-19):**` label. No finding row removed, disposition table untouched, REQUIREMENTS.md not edited.
- Registered round-5 threats T-48-70..T-48-79 exactly once each in 48-SECURITY.md (T-48-75 at `medium` matching 48-21's table; the other seven at `high`), all `mitigate`/`closed`, each Mitigation naming its enforcing token/assertion and its drill; arithmetic paragraph, audit trail (31/31/0 naming plans 48-20..48-22), Sign-Off, Approval and the `Security Audit 2026-10-20` metric table all read 31 rows / 31/31 closed / 0 open; the 2026-10-19 table stays 20/20/0; the STRUCT-OK/EVALGATE-OK mutation narrative carries the historical token-names marker; `threats_open: 0` and `status: verified` unchanged.

## Task Commits

Each task was committed atomically:

1. **Task 1: drive the trust-surface path-filter and secret rules from the parsed document** - `7cbd385` (fix)
2. **Task 2: declare the linter install environment and the coordinated version bump** - `9865967` (fix)
3. **Task 3: annotate the R1 ledger and register the round-5 threats** - `05ceef8` (docs)

**Plan metadata:** pending (docs commit follows this summary)

## Files Created/Modified

- `verify-phase-48.sh` - parsed path-filter walk + parsed secret walk in `scan_trust_surface` (column-0 drill markers), four new evasion fixtures, twelve-directory `mkdir -p`, fixture loop 8 → 12, count check and `done_partition` string updated, coordinated-bump note beside `LINT_VERSION`
- `.github/workflows/ci.yml` - install-step `env:` with the three declared settings, rewritten verify-job comment (module-proxy truth + GOTOOLCHAIN=local satisfiability), no change to triggers, permissions, check names or install command
- `.planning/phases/48-ci/48-REVIEW-DISPOSITION-R1.md` - historical annotation + `**Superseded text (2026-10-19):**` label over the retained closing paragraph
- `.planning/phases/48-ci/48-SECURITY.md` - T-48-70..T-48-79 rows, 31-row arithmetic, 31/31/0 audit-trail row and metric table, Sign-Off/Approval counts, historical token-names marker

## Decisions Made

- Parsed walks layer OVER the retained textual rules (not replacements) — this is what makes each deletion drill attributable: with a walk deleted, the loop runs past the fixtures the text rule still catches and reds on the first fixture only the deleted walk was catching.
- Drill markers sit at column 0 inside the indented Python probe: Python ignores comment-only-line indentation, so the markers are valid probe syntax AND `startswith`-matchable range delimiters. (The first drill run with indented markers deleted nothing and still passed; fixed before commit.)
- `requirements.mark-complete` deliberately not run: `ready-ids` returned all three IDs, but REQUIREMENTS.md already carries `Complete (run 39380373930)` in both places and the plan's artifacts say "No requirement status changes" — marking would overwrite the canonical run-id status text. Recorded as the plan's expected no-op, not a skipped duty.
- The Approval paragraph retains its approval history inline rather than deleting the prior 21/21 record.

## Deviations from Plan

None - plan executed exactly as written.

(One execution-environment note, not a deviation: the two mutation-drill `<verify>` commands embed a `python - <<'PY'` heredoc inside a `&&` chain, which the interactive shell layer mangles — the 48-21 precedent — so each was reproduced verbatim from a script file; the drill logic, marker strings, sandbox layout and attribution greps are the plan's own.)

## Issues Encountered

None. The healthy gate passed on the first run after each task, and both deletion drills red on their first correctly-instrumented execution with the exact fixture attribution the plan's `fails_when` names.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four round-4 advisories are closed at the source and truth 15's PARTIAL half is flipped: every tracked record either states the four-green outcome or is explicitly annotated as superseded. The phase's remaining open items are the by-design M2/M3/M5 manual-only rows in 48-VALIDATION.md.
- The round-5 register (31 rows, 0 open, status verified) now cites a mutation drill for every row, and the R1 ledger can no longer be read as asserting the falsified no-transport state.
- CICD-01/02/03 remain Complete on run 39380373930; this plan changes no trigger, permission, published check name, install command or requirement status.

---
*Phase: 48-ci*
*Completed: 2026-10-20*

## Self-Check: PASSED

- All four `files_modified` artifacts exist on the committed tree: `verify-phase-48.sh` (parsed walks + 12 fixtures + coordinated-bump note), `.github/workflows/ci.yml` (three declared install env keys, corrected comment), `48-REVIEW-DISPOSITION-R1.md` (historical annotation preceding the retained superseded paragraph), `48-SECURITY.md` (T-48-70..T-48-79, 31 rows / 0 open / verified).
- Commits verified as ancestors of HEAD: `7cbd385` (Task 1), `9865967` (Task 2), `05ceef8` (Task 3), `71f1617` (SUMMARY), `32fac14` (state/roadmap tracking).
- Final gate run post-commit: exit 0, exactly one `ALL PASSED`, `registry 11/11, begin markers 11/11, success tokens 11/11`, zero `NOT VERIFIED`.
- Both deletion drills red with exact attribution (quoted-paths / tojson-secrets); `git diff` vs plan base `8a40057` touches exactly the four `files_modified` paths.
- `git rev-list --count` from the plan ledger: 3 task commits, matching the SUMMARY frontmatter.
- `requirements.ready-ids` returned all three IDs, but CICD-01/02/03 are already `Complete (run 39380373930)` in both REQUIREMENTS.md places and the plan forbids requirement-status changes — mark-complete deliberately not run (the plan's expected no-op; a bare mark would overwrite the canonical run-id text).
