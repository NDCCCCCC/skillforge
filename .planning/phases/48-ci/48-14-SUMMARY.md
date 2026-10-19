---
phase: 48
plan: 14
subsystem: planning-docs (security register / research audit / roadmap)
tags: [security, supply-chain, docs, gap-closure, ci]
requires:
  - 48-12 (the module-proxy install fix the new register row cites)
  - 48-13 (the P2/P3 pin and reintroduction drills the new register row cites)
provides:
  - closed register row T-48-47 in 48-SECURITY.md (48-11-SUMMARY's threat_flag: supply-chain finally lands)
  - PyYAML legitimacy-audit row + RESOLVED dispositions for all four 48-RESEARCH.md open questions
  - restored-transport record in 48-SECURITY.md and ROADMAP.md (48-15 can no longer inherit a falsified blocker)
affects:
  - 48-15 (executes the run recipe; its environment story is now accurate)
tech-stack:
  added: []
  patterns:
    - register == code (frontmatter verdict must never contradict its own rows)
    - closure requires assertion + mutation, never string absence
key-files:
  created:
    - .planning/phases/48-ci/48-14-SUMMARY.md
  modified:
    - .planning/phases/48-ci/48-SECURITY.md
    - .planning/phases/48-ci/48-RESEARCH.md
    - .planning/ROADMAP.md
decisions:
  - "T-48-47 (not T-48-21) — the plan-level register IDs run to T-48-52 and T-48-21 is already claimed by 48-05-PLAN.md; the row matches 48-12-PLAN.md's threat model"
  - "Q1's disposition cites the shipped probe split (P3 asserts the go job's exact permissions map, P8 asserts every value is read/none) rather than the plan's shorthand 'P8 asserts the scope set'"
  - "The dated '## Security Audit 2026-10-19' trailing metric table (20/20/0) is left untouched as a historical snapshot; the audit trail, arithmetic paragraph and Sign-Off carry the current 21/21/0"
  - "CICD-01/02/03 deliberately NOT flipped — no run exists yet; the flip belongs to 48-15 against run evidence (T-48-46's recorded rule)"
metrics:
  duration: 5 min
  completed: 2026-10-20
actuals:
  tokens: 1475
  tasks: 2
  commits: 2
commits: 2
plan_head_before: 6d2810e567a6f8680714a5ddba2727be8237961f
plan_head_after: 57d294f78bf08167d4423f5be047569e5f1a374c
status: complete
---

# Phase 48 Plan 14: Supply-Chain Register Row + Environment Truth Re-record Summary

**One-liner:** The security register now names the supply-chain surface its own 48-11-SUMMARY flagged (closed row T-48-47, 21/21, `threats_open: 0`), the research audit row-lists pip pyyaml with its pin disposition and carries zero unresolved open questions, and both SECURITY and ROADMAP state the restored git transport (`ls-remote origin HEAD` exit 0) instead of the stale unreachable claim.

## What Was Built

### Task 1 — 48-SECURITY.md re-record (commit 235b6c4)

- Register row **T-48-47** appended after T-48-20: `Tampering` / `.github/workflows/ci.yml` verify job install step / `critical` / `mitigate` / `closed`. The Mitigation cell names the module-proxy install (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` at ci.yml:229 + the in-step `[ -x "$LINT_BIN/golangci-lint" ]` postcondition, 48-12 Task 1), the enforcing P2/P3 pin and reintroduction drill (48-13 Tasks 2-3), and 48-11-SUMMARY's `threat_flag: supply-chain` as the row's origin.
- Round-2 subsection `### Re-closure (plan 48-14, 2026-10-20 — round-2 gap closure)` added under "Why these closures changed" in the established table format (Falsified closure / Rows / Now proven by / Mutation that bites), recording that the gap-closure wave reintroduced the defect class it was fixing (R2-CR-01) — the same frontmatter-vs-prose contradiction class the file's header comment block already documents once.
- Audit trail row appended: `2026-10-20 | 21 | 21 | 0 | gsd gap-closure round 3 (plans 48-12..48-13) — supply-chain row recorded after fix+pin landed`.
- Arithmetic paragraph updated to **21 rows** (20 closed + 1 closed-accepted, 0 open); Sign-Off refreshed to APPROVED — 21/21 closed, 0 open, with the threat_flag explicitly received.
- Frontmatter comment block extended with the dated round-3 correction; `threats_open: 0` / `status: verified` remain truthful and machine-checkable against the rows.
- "What this audit does NOT establish" M1 bullet rewritten to the round-2 environment truth: `git ls-remote origin HEAD` **succeeds** from the executing host (exit 0, `369b08a`); the recorded "git transport unreachable" cause no longer holds. What remains: the repository-owner history decision (origin/main is a single unrelated commit, so no PR can be opened) plus one push from this transport-capable host, owned by plan 48-15 executing the recipe persisted in 48-11-SUMMARY.md. The "no CI runner has executed these workflows" statement is kept — still true.

### Task 2 — 48-RESEARCH.md + ROADMAP.md (commit 57d294f)

- Package Legitimacy Audit: the absolute "No npm / PyPI / crates packages are installed by this phase" preamble replaced with a scoped one (Actions verified against GitHub's release API; the one registry package is row-listed), and a **pyyaml** row appended (`github.com/yaml/pyyaml` | 6.0.3 | Official Org Yes | OK | Pinned `==6.0.3` in ci.yml's gate job (48-12 Task 2) — the version already validated by every local verifier run). Existing rows byte-identical.
- `## Open Questions` retitled `## Open Questions (RESOLVED 2026-10-20 — dispositions below; see 48-CONTEXT.md)`; each of the four questions keeps its original text and gains an inline `**RESOLVED**` disposition naming its decider:
  - Q1 (D-26 amendment) → **D-29**: read-only scopes only, `pull-requests: read` permitted for the lint job, zero write scopes; P3 asserts the exact permissions map, P8 asserts every value is `read`/`none`.
  - Q2 (D-22 amendment / checkout outside the composite) → **D-30**: checkout moves to each job, the composite keeps setup-go/node/buildx only, no redundant second checkout.
  - Q3 (full local gates in the script) → **Discretion** as shipped: CI invariants + lint teeth by default, the Go/FE suites are the `go`/`fe` jobs' own subject matter, no `--full` mode built.
  - Q4 (eval-gate action upgrades) → **shipped tree**: eval-gate.yml carries `actions/checkout@v7` (line 83) and `actions/upload-artifact@v7` (line 130).
- ROADMAP.md Phase 48 `**Blocked on**`: scoped replacement of only the stale transport clause — now "git 传输已恢复（`git ls-remote origin HEAD` → exit 0, `369b08a`；48-VERIFICATION round 2 环境事实）". The rest of the paragraph (origin/main unrelated history, recipe persisted, CICD-01/02/03 Pending) untouched, as are all other phase entries.
- `.planning/REQUIREMENTS.md` untouched: CICD-01/02/03 remain `Pending` (verified by grep post-commit).

## Verification Evidence

| Probe | Result |
|-------|--------|
| Task 1 automated verify | `ROW-OK` — T-48-47 row present, `threat_flag: supply-chain` cited, "21 rows" present, transport truth present, no mis-numbered T-48-21 row |
| Register row count | 21 (`grep -c "^| T-48-"` = 21); frontmatter `threats_open: 0` / `status: verified` coherent with it |
| Task 2 automated verify | `ROADMAP-OK` — pyyaml row present, RESOLVED heading present, ≥4 `**RESOLVED**` markers, stale transport text absent from ROADMAP, restored-transport fact present |
| Requirements guard | CICD-01/02/03 still `Pending` in REQUIREMENTS.md |
| Diff scope | exactly the three files in `files_modified` (46 insertions, 16 deletions total); ROADMAP diff confined to the Phase 48 Blocked-on clause (1 line) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Accuracy] Recorded actual eval-gate.yml line numbers instead of the plan's stale ones**
- **Found during:** Task 2
- **Issue:** The plan cited `actions/checkout@v7` at eval-gate.yml "line 81" and `actions/upload-artifact@v7` at "line 128"; the shipped file has them at lines 83 and 130.
- **Fix:** The Q4 RESOLVED disposition cites the verified current line numbers (83 / 130), so the doc stays greppable and true.
- **Files modified:** `.planning/phases/48-ci/48-RESEARCH.md`
- **Commit:** 57d294f

**2. [Rule 1 - Accuracy] Q1 disposition cites the shipped probe split, not the plan's shorthand**
- **Found during:** Task 2
- **Issue:** The plan's Q1 action text said "P8 asserts the scope set"; the shipped verifier (and 48-SECURITY.md row T-48-08) splits that across P3 (exact permissions map equality) and P8 (every value is `read`/`none`).
- **Fix:** The disposition names both probes as they actually assert.
- **Files modified:** `.planning/phases/48-ci/48-RESEARCH.md`
- **Commit:** 57d294f

### Considered and left alone

- The dated `## Security Audit 2026-10-19` trailing metric table (Threats found 20 / Closed 20 / Open 0) is a historical snapshot of that date's audit and is not one of the plan's listed sections; updating it to 21 would falsify history. The audit-trail table, the arithmetic paragraph and the Sign-Off all carry the current 21/21/0. If a future secure-phase run appends a fresh dated block, it will supersede this one naturally.
- STATE.md's Blockers entry still describing transport as unreachable was left untouched — it is a historical record of the 48-11 state and is outside this plan's `files_modified`; the authoritative environment facts now live in 48-SECURITY.md, 48-RESEARCH.md and ROADMAP.md.

## Authentication Gates

None. No auth-gated command was needed; both tasks are documentation edits verified by grep.

## Known Stubs

None. No code, no workflow, no verifier changes — planning documents only.

## Threat Flags

None. No new security-relevant surface was introduced; the plan's own STRIDE rows (T-48-53/54/T-48-SC) are satisfied by the edits above — the register/truth contradiction is repaired (T-48-53), the stale environment claims are corrected in both files citing the round-2 Environment Facts table (T-48-54), and the one registry package is row-listed with verdict OK and its pin disposition, leaving no `[ASSUMED]`/`[SUS]` package in the install set (T-48-SC).

## Self-Check: PASSED

- All four files exist on disk (48-SECURITY.md, 48-RESEARCH.md, ROADMAP.md, 48-14-SUMMARY.md).
- Both task commits are ancestors of HEAD: `235b6c4` (Task 1), `57d294f` (Task 2).
- Commits measured from the plan ledger `6d2810e..57d294f`: 2 (matches `commits: 2`).
