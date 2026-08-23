---
phase: 14-p10-0-cgo-trust-floor
plan: 11
subsystem: platform-trust
tags: [eval, refusal-accounting, cgo, docker, frozen-locks, relock-chain, no-falsify]

requires:
  - phase: 14-p10-0-cgo-trust-floor
    plan: 10
    provides: floor 1.1.0 tail re-anchor + 14-10-SUMMARY Operator-Gate Addendum that documented the re-baseline Alternative (the pending decision this plan routes to)

provides:
  - cmd/eval classifyRunOutcome four-modality scoring-path classifier with in-schema verdict status:"refused" recognition (RED→GREEN hermetic lock)
  - Dockerfile.backend runtime libstdc++ libgcc fix (go-duckdb CGO dynamic-link dependency)
  - verify-phase-13 Stage 7 compliance pin synchronized to 14-10 re-lock (1dc6f148…, lockstep with verify-phase-14.sh Stage 11)
  - Honest 14-11 replay evidence (0.888 faithfulness, 19/20 refusals, BGE infra fault) recorded per no-falsify contract

affects: [Phase 14 closeout, v1.0 milestone frozen-baseline ledger, eval-gate semantics]

tech-stack:
  added: []
  patterns:
    - "Refusal accounting classifier: one pure function (classifyRunOutcome) owns the four scoring-path modalities with explicit priority (err > structured Refusal > in-schema refused verdict > scored verdict)"
    - "Frozen-baseline pin lockstep: verify-phase-13 Stage 7 rippled to the same 1dc6f148… pin as verify-phase-14.sh Stage 11; gate is green = trustworthy again"
    - "No-falsify STOP discipline: replay result 0.888 < 0.9 is recorded exactly as measured, no re-sign, no retry, no caveat; route to pre-declared 14-10 Addendum Alternative"

key-files:
  created: []
  modified:
    - cmd/eval/main.go
    - cmd/eval/main_test.go
    - Dockerfile.backend
    - scripts/verify-phase-13.sh

key-decisions:
  - "D14-F2 STOP discipline honored: replay faithfulness 0.888 < 0.9 with BGE endpoint failure mid-run plus adversarial-category drag; no re-sign, no retry, no README caveat — honest numbers recorded and routed to 14-10-SUMMARY Operator-Gate Addendum Alternative (explicit re-baseline of gate semantics)"
  - "Refusal accounting fix confirmed working: 19/20 refusal-required cases now score refused=true with faithfulness 1.0 and context_recall REFUSAL sentinel 1.0; the 14-10 measurement-layer defect is closed at the code level"

patterns-established:
  - "When a live gate FAILs because the measurement semantics are unclear, fix the accounting first (hermetic RED→GREEN), run one honest replay, and STOP on the number rather than re-roll for a better one"

requirements-completed: []

duration: 76m
completed: 2026-08-23
---

# Phase 14 Plan 11: CGO Trust Floor Verification Gap Closure (14-VERIFICATION gaps 1-3) Summary

**cmd/eval refusal-accounting fix extracted into classifyRunOutcome with hermetic RED→GREEN regression lock, Dockerfile runtime libstdc++ libgcc fix shipped, verify-phase-13 pin lockstep restored, and the single authorized compliance replay honestly STOPped at 0.888 faithfulness — no re-sign, no retry, no falsified evidence.**

## Performance

- **Duration:** 1h 16m
- **Started:** 2026-08-23T02:00:35Z
- **Completed:** 2026-08-23T03:16:08Z
- **Tasks:** 3
- **Files modified:** 4 production/planning files

## Accomplishments

- **Gap 1 measurement-layer defect permanently fixed:** `classifyRunOutcome` in `cmd/eval/main.go` now recognizes a well-formed verdict with `status:"refused"` (a `verdict.ValidStatuses` member) and routes it into the same frozen refusal conventions as Run-error / structured-refusal paths (`refused=true`, `answer=""`). A 6-row table-driven test (`TestClassifyRunOutcome`) was added RED → GREEN to lock the four outcome modalities + boundary guards + priority guard.
- **Gap 2 Docker runtime defect fixed:** `Dockerfile.backend` runtime stage now installs `libstdc++` and `libgcc` (go-duckdb v2.4.3 C++ engine dynamic-link dependencies). The fix is correct by construction; live Docker verification is blocked on this host because Docker is not installed (git-bash and WSL both report `docker: command not found`), recorded as `blocked-with-notes` per the 14-08 precedent.
- **Gap 3 Phase 13 pin ripple fixed:** `scripts/verify-phase-13.sh` Stage 7 `EXPECTED_COMPLIANCE_SHA` updated from the stale `e77877bb…` to the measured current `1dc6f148…` (lockstep with `verify-phase-14.sh` Stage 11). The outdated "Phase 8 pin (UNCHANGED)" narrative was replaced with the truthful re-lock history (14-01 D14-B3 + 14-10 floor-1.1.0 re-anchor; recorded T-8-12 actions). `make verify-phase-13` now passes 9/9 stages.
- **Honest replay evidence recorded:** the single authorized `cmd/eval --frozen=compliance --archive` replay completed 50/50 cases in ~52 min. AssertPin held (`frozen pin holds`, `prompt_hash=5b12acd9…`). The refusal-accounting fix performed as designed (refusal-required 19/20 refused, faith 0.9727), but overall faithfulness finished at **0.888** (< 0.9 gate). A mid-run BGE endpoint outage (`connect: no route to host`) affected several multi-hop cases. Per the no-falsify contract, the result is recorded exactly as measured — no re-sign, no retry, no caveat.

## Task Commits

1. **Task 1 Step 1: behavior-preserving refactor** — `105bcaa` (refactor)
2. **Task 1 Step 2: RED regression test** — `9f18349` (test)
3. **Task 1 Step 3: GREEN refusal fix** — `49feb59` (fix)
4. **Task 2 A: Dockerfile libstdc++ libgcc** — `58b6c0c` (fix)
5. **Task 2 B: verify-phase-13 pin ripple** — `b2be52d` (chore)

**Plan metadata:** this docs commit.

## Files Created/Modified

- `cmd/eval/main.go` — extracted `classifyRunOutcome`; folded `refusalReasonStr`; added the `v.Status == "refused"` refusal branch with extensive godoc linking the 14-10 root-cause analysis and frozen downstream conventions.
- `cmd/eval/main_test.go` — added `TestClassifyRunOutcome` 6-row table-driven regression lock (err / structured refusal / in-schema refused verdict RED core / ok / needs_input / err-wins priority guard).
- `Dockerfile.backend` — runtime `apk add` now includes `libstdc++ libgcc`; header + comment rationale for go-duckdb C++ engine dynamic linking.
- `scripts/verify-phase-13.sh` — Stage 7 compliance pin updated to `1dc6f148…`; comment block truthfully records the 14-01/14-10 re-lock history; FAIL copy names the real invariant (tampering or unrecorded re-lock).

## Decisions Made

- **D14-F2 / D14-G1 — Honest STOP at 0.888.** The refusal-accounting fix worked (refusal behavior restored to the frozen convention), but the overall gate did not clear. Rather than retry or hand-edit metrics, the replay result is recorded as the live outcome and the platform-floor lock re-sign is deferred. This follows the no-falsify contract encoded in the plan.
- **Routing to the pre-declared Addendum Alternative.** Because the result fell in the predicted 0.83–0.90 band, the next step is the 14-10-SUMMARY "Operator-Gate Addendum" Alternative: explicitly re-baseline the gate semantics so refusal prose is judged against ground-truth refusal criteria rather than retrieved context. This is a planner/user decision, not an executor invention.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Updated the Stage 7 FAIL copy to a truthful invariant description**
- **Found during:** Task 2 B (verify-phase-13 pin ripple)
- **Issue:** The mandated edit scope covered the pin value and the Stage 7 comment block, but the adjacent FAIL message still told the operator "Phase 8 baseline pin" — a stale framing that would mislead anyone investigating a future mismatch.
- **Fix:** Changed the FAIL copy to state the actual invariant: tampering or an unrecorded re-lock bypassing T-8-12 (pins move only via recorded re-locks: 14-01 / 14-10, lockstep with verify-phase-14.sh Stage 11).
- **Files modified:** `scripts/verify-phase-13.sh`
- **Verification:** `make verify-phase-13` 9/9 PASS (Stage 7 green with new pin)
- **Committed in:** `b2be52d`

---

**Total deviations:** 1 auto-fixed (1 missing-critical narrative accuracy)
**Impact on plan:** No scope creep; the change keeps the gate's failure message aligned with the real T-8-12 governance model.

## STOP Record — Task 3 single compliance replay (14-11)

**Verdict: FAIL against the faithfulness ≥ 0.9 gate → plan-mandated STOP. Re-sign of `platform-floor.lock.json`, README caveat, and docs commit NOT executed.**

Raw final numbers (archive `eval/reports/2026-08-23T03-13-46Z.json`, judge=glm-5.2, 50/50 cases, AssertPin held at `5b12acd9…`):

```
overall: faithfulness 0.8880 (gate ≥0.9; baseline 0.972)
per-category faithfulness: adversarial 0.7683 | multi-hop 0.9581 | refusal-required 0.9727
refusal_required: 20 cases, refused=true 19/20 (baseline 95%)
answer_relevance 0.4381 | context_precision 0.5870 | context_recall 0.7946
```

Infrastructure fault evidence (measurement-validity context, NOT an excuse to re-roll):
- BGE-M3 embedder became unreachable mid-run: `dial tcp 10.62.0.123:11434: connect: no route to host` for cases `mh-004`, `mh-005`, `mh-008`, plus the agent's pre-retrieve embed for `mh-008`.
- Affected cases were scored as refusals (agent.Run error path) or with `answer_relevancy=0`; they did not hang the run, but they are not a "healthy service" window.
- No content-filter 400s and no thread stalls; CPU advanced and the run completed/archived normally.

Fix-effect evidence (the measurement-layer defect is closed):
- Refusal cases that previously scored `refused=false` now score `refused=true` with `faithfulness=1.000` and `crecall=1.000` (e.g., `ref-000` through `ref-019`, 19/20).
- This is exactly the accounting the 14-10 Addendum identified as missing.

## Next-Step Routing (0.83–0.90 band)

The 14-10-SUMMARY "Operator-Gate Addendum" already recorded two planner options:

1. **Minimal candidate (this plan implemented):** set `refused=true` when `v.Status == "refused"` in `cmd/eval` scoring. This restores the REFUSAL sentinel and the empty/refusal-answer faith convention. It has been executed and verified hermetically.
2. **Alternative (next decision):** re-baseline the gate semantics explicitly — judge refusal prose against ground-truth refusal criteria rather than against retrieved context.

The 14-11 replay result (0.888) sits in the predicted 0.83–0.90 window, so the Alternative is the pre-declared escalation path. A planner or user must choose whether to:
- accept 0.888 as the new floor reality and re-baseline the gate, or
- investigate the adversarial-category drag (0.7683) independently.

Either way, no further executor-initiated replays are authorized.

## Issues Encountered

- **BGE-M3 endpoint dropped mid-replay** (`no route to host`). The run completed and archived, but several multi-hop cases were forced into the agent.Run-error refusal path. This is an infrastructure fault, not a code defect; it was recorded rather than used to justify a retry.
- **Docker unavailable on this host** for live Gap 2 verification. The libstdc++/libgcc fix is correct by construction (go-duckdb documented runtime dependency) and ships; live ldd/LINKS_OK evidence is deferred until an environment with Docker is available.

## User Setup Required

None for this plan. One **planner decision** is open: choose between accepting the 0.888 result and re-baselining the gate per the 14-10-SUMMARY Addendum Alternative, or another resolution path.

## Next Phase Readiness

- Phase 14 code-level fixes are complete and hermetically verified (verify-phase-14 11/11 PASS, verify-phase-13 9/9 PASS, all frozen locks byte-identical).
- The platform-floor.lock.json 1.1.0 formal re-sign remains deferred; the file still carries platform_floor_version=1.1.0 with the 2026-08-22 1.0.0-era metrics/signature. This is an honest partial state.
- Phase 15 (data import + de-identification) can proceed on the 1.1.0 floor behavior, but the phase closeout tag should not be applied until the re-sign decision is resolved.

## Self-Check: PASSED

- `cmd/eval/main.go` contains `classifyRunOutcome` and `verdict_status_refused` — FOUND
- `cmd/eval/main_test.go` contains `TestClassifyRunOutcome` with 6 rows — FOUND
- `go test ./cmd/eval/ -run TestClassifyRunOutcome -count=1 -v` PASS — verified in step 4
- `Dockerfile.backend` contains `libstdc++ libgcc` — FOUND
- `scripts/verify-phase-13.sh` contains `1dc6f148fe3debd0b572872d87126242362905b26ae1097b983ed8754277bbdc` — FOUND
- `make verify-phase-13` 9/9 PASS and `make verify-phase-14` 11/11 PASS — verified
- Frozen locks + golden set `git diff --quiet` clean — verified in step 4
- `eval/frozen/platform-floor.lock.json` NOT modified (no re-sign on STOP branch) — `git diff` exit 0
- `skillforge.db` / `.env` / `reports/` / `eval/report.json` / `eval/calibration.json` never staged — git status clean of them
- Throwaway `scripts/tmp-1411-*.sh` deleted — verified

---

*Phase: 14-p10-0-cgo-trust-floor*
*Completed: 2026-08-23*
