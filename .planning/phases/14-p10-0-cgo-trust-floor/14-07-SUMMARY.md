---
phase: 14-p10-0-cgo-trust-floor
plan: 07
subsystem: verification / acceptance gate / frozen-lock governance
objective: "Ripple governance (re-lock verification + Phase 13 gate EXPECTED sync) + the Phase 14 hermetic acceptance gate (verify-phase-14.sh, 11 stages) + STRIDE-14 register."
tags: [PLAT-11, verify-gate, frozen-locks, T-8-12, STRIDE, race-detector]

requires:
  - phase: 14-01
    provides: "compliance.lock.json floor re-pin (58ffea08… → ca333fba…, D14-B3) — done in 14-01, verified here"
  - phase: 14-04
    provides: "platform-floor.lock.json frozen set (SC-5 hermetic layer)"
  - phase: 14-06
    provides: "duckdb 10-lock series consumed by gate Stage 05"

provides:
  - "scripts/verify-phase-14.sh — 11-stage hermetic acceptance gate, full pass on WSL 2026-08-22 (exit 0)"
  - "Makefile verify-phase-14 target (+ .PHONY), Windows-native caveat documented"
  - "docs/threat-model/STRIDE-14.md — T-14-01..20 + 06b + SC register with 3 residual-risk notes"
  - "Ripple governance: verify-phase-13.sh EXPECTED_COMPLIANCE_SHA synced to the 14-01 baseline (Phase 13 gate stays runnable)"
  - "Five-lock sha256 baseline archive (Stage 11 EXPECTED constants)"

affects:
  - scripts/verify-phase-13.sh (EXPECTED constant only)
  - internal/server/redline_test.go (race fix)
  - internal/duckdb (throughput test !race split)
  - 14-08 (operator gates documented in the script header)

tech-stack:
  added: []
  patterns:
    - "//go:build !race splits throughput measurement from the race gate (T-14-20)"
    - "Gate EXPECTED sha256 constants = the frozen-baseline lock (any lock tamper trips Stage 11)"

key-files:
  created:
    - scripts/verify-phase-14.sh
    - docs/threat-model/STRIDE-14.md
    - internal/duckdb/appender_throughput_test.go
  modified:
    - Makefile
    - scripts/verify-phase-13.sh
    - internal/duckdb/connector_test.go
    - internal/server/redline_test.go

key-decisions:
  - "Appender throughput floor moved behind //go:build !race (T-14-20): race instrumentation makes cgo-boundary rows/sec load-sensitive noise — observed 106k solo vs sub-100k under full-repo parallel load vs the 100k floor; non-race Stage 05 enforces the floor with ~6x headroom (617k rows/s WSL)"
  - "hitl.lock.json NOT re-locked: its prompt_hash is the SKILL.md file digest (079ad908…), NOT the rendered prompt — the floor never touches it. The floor-covered pin is compliance.lock.json (re-pinned 14-01). Plan's 'hitl pin necessarily invalid' assumption was wrong; recorded here + STRIDE-14 residual note 3"
  - "Gate runs 11 stages (plan said 9): platform-floor structural locks and the WalkDir no-bypass gate split into two stages; gcc precondition its own stage. More coverage than planned, satisfies the ≥9 run_stage acceptance"

patterns-established:
  - "Ripple governance for older gates: when a re-pin changes a frozen baseline, every older gate pinning that file must sync its EXPECTED constant in the same recorded action"

requirements-completed:
  - PLAT-11

# Metrics
duration: 240min (incl. 3 gate runs + full-repo -race triage)
completed: 2026-08-22

---

# Plan 14-07: Ripple Governance + Acceptance Gate Summary

**Five frozen locks byte-verified, 11-stage gate green on first full pass after two test-debt fixes, STRIDE-14 archived — the hermetic Phase 14 surface is closed; only the two live operator gates (14-08) remain.**

## Gate Run Evidence (WSL Ubuntu-22.04, gcc 11.4.0, go1.25.0, 2026-08-22T10:22:44Z)

| Stage | Face | Result |
|-------|------|--------|
| 01 | gcc precondition (CGO hard prerequisite) | PASS |
| 02 | go build + vet (full repo) | PASS |
| 03 | full-repo `go test ./... -count=1 -race -timeout 900s` (restored) | PASS |
| 04 | sqlite store suite + TestSqliteMattnZeroRegression | PASS |
| 05 | duckdb 10-lock series (boot-queries/WAL/throughput/concurrent + LRU×4 + staging×4) | PASS |
| 06 | platform-floor structural locks (prepend-both-paths/cannot-shadow/hash-covers/hot-reload) | PASS |
| 07 | WalkDir no-bypass gate (TestNoBypassOfPlatformFloorCompose) | PASS |
| 08 | PLAT-10 audit floor-version column + admin preview + HITL fail-closed | PASS |
| 09 | frozen-set hermetic judges (golden/schema/lock self-consistency) | PASS |
| 10 | structural-constant source-greps (5 literals) | PASS |
| 11 | frozen lock byte-identity (5 locks sha256) | PASS |

**PHASE 14 HERMETIC GATE: PASS — exit 0.**

## Frozen Baseline Archive (T-8-12 — recorded re-pins, Stage 11 EXPECTED constants)

| Lock | sha256 | Provenance |
|------|--------|------------|
| compliance.lock.json | `e77877bb3b3076bc67d33a1f7a1cf4bea3b9af20ebafdd457ef6e9c366ff6ce5` | 14-01 floor re-pin (prompt_hash `58ffea08…` → `ca333fba…`, commit 66ab8ce) |
| hitl.lock.json | `6e91881f4aafeffb843e996c89ff1c27ef22eb18fac94eceff9184f938481d08` | Phase 11 baseline UNCHANGED (pin is SKILL.md digest — see deviation 2) |
| platform-floor.lock.json | `b28f30ae37568de261aaa9a797cec05b42c75bd5f19c9d317a7cd270290e954a` | 14-04 creation (SC-5) |
| fanout.lock.json | `af3020fd7f38fce2f7396dce81eb151e4ed6952262565785554a59ff51e01252` | Phase 13 baseline UNCHANGED |
| deep.lock.json | `f3958dcd4930ac09e03c824f781df9c6eb04b691770cab4d512e11fba3c3c482` | Phase 10 placeholder UNCHANGED |

No-ripple verification: `git status eval/frozen/` clean; deep/fanout/platform-floor byte-identical to their creating commits.

## Accomplishments
- Race detector restored to the acceptance gate for the first time since Phase 12 dropped it — and the restoration immediately caught one real test debt (redline stub) and one measurement-design flaw (throughput under -race), both fixed in-plan
- Ripple governance kept verify-phase-13.sh runnable: its compliance EXPECTED synced to the 14-01 baseline in the same recorded action (old gate, new truth)
- STRIDE-14 register archived with 22 threats (T-14-01..20 + 06b + SC) all dispositioned; 3 residual notes document the regex-judge scope, the Windows-native duckdb constraint, and the hitl pin semantics

## Task Commits

1. **redline race fix** — `fix(14-07): synchronize stubChatModel count reads in redline tests` (stub accessor under mutex; test-goroutine bare reads were a data race once -race returned to the gate)
2. **throughput !race split** — `test(14-07): move appender throughput floor behind !race build tag` (5bfad6c, T-14-20)
3. **gate + register + ripple** — `feat(14-07): add Phase 14 hermetic gate + STRIDE-14 register + ripple governance` (1418cc4: verify-phase-14.sh + Makefile + STRIDE-14.md + verify-phase-13.sh EXPECTED sync)

**Plan metadata:** this SUMMARY commit (docs)

## Deviations from Plan
1. **Task 1 (re-lock) executed in 14-01, not here.** The compliance floor re-pin (58ffea08… → ca333fba…) was the direct artifact of 14-01's render convergence and was committed there (66ab8ce). 14-07's Task 1 therefore became verification + archive: AssertPin green against the new pin, sha256 recorded in Stage 11, git diff scope confirmed. The plan's acceptance "git diff --name-only eval/frozen/ shows exactly two files" is unsatisfiable post-hoc by design — re-touching hitl would be baseline tampering (T-8-12), not compliance.
2. **hitl.lock.json needed NO re-lock — plan assumption was wrong.** The plan assumed "floor prepend changes renderedPromptBody → hitl pin necessarily invalid". Empirically the hitl pin is the sha256 of `testdata/skills/hitl-smoke/SKILL.md` (matches `079ad908…` byte-for-byte), never passing through RenderSystemPrompt. The floor cannot move it. Floor coverage for the prompt surface lives in compliance.lock.json (re-pinned 14-01). Recorded in STRIDE-14 residual note 3.
3. **Gate is 11 stages, not 9** (platform-floor split + gcc precondition separated). Acceptance `run_stage count ≥ 9` satisfied; stage numbering in output reads 01-11 (printf %02d over the incremented counter).
4. **Two test-debt fixes landed inside this plan** (redline race accessor; throughput !race tag) — the plan's "跑绿" acceptance surfaced them; both are Phase-14-owned surfaces so fixing in-plan was the honest scope, each committed atomically.
5. Inline execution (subagent spawns unavailable this session — see 14-05-SUMMARY deviation).

## Self-Check: PASSED
- [x] `bash scripts/verify-phase-14.sh` on WSL: 11/11 [STAGE-xx-PASS], `PHASE 14 HERMETIC GATE: PASS`, exit 0
- [x] `grep -c run_stage scripts/verify-phase-14.sh` = 13 lines (helper def + comments + 11 invocations, ≥ 9); Stage 03 command contains `-race`
- [x] `grep -c verify-phase-14 Makefile` = 2 (.PHONY + target)
- [x] STRIDE-14 covers T-14-01..20 + 06b + SC, every row has a disposition
- [x] eval/frozen/ clean in git; 5 locks byte-identical to provenance commits
- [x] verify-phase-13.sh: only the EXPECTED_COMPLIANCE_SHA line changed (ripple governance comment included)
