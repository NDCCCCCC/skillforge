---
phase: 39-ecl-pipeline
plan: 09
subsystem: knowledge
tags: [ecl-pipeline, golden-set, f1-harness, d-gt-01, human-review, verify-script, hermetic-gate, onT-10]

# Dependency graph
requires:
  - phase: 39-03
    provides: WalkAndMerge chunker + StubExtractor (the deterministic drafter + extractor under test)
  - phase: 39-04
    provides: LoadGolden / ScoreTables / ScoreRelated / MacroF1 scoring math
  - phase: 39-05
    provides: ABox + manager + failure-store suites covered by the gate
  - phase: 39-06/07/08
    provides: indexer routing, admin pipeline routes, FE pipeline tab suites covered by the gate
provides:
  - eval/ecl-golden/sql-guides.jsonl — 26-row HUMAN-REVIEWED ground truth (D-GT-01 closed: regex-drafted, user-approved, LLM-free)
  - internal/knowledge/ecl/extract_golden_test.go — TestStubVsGoldenF1, the SC-2 macro-F1 >= 0.8 harness gate
  - scripts/verify-phase-39.sh — hermetic SC-1..6 aggregate gate (SC-6: the phase's executable definition of done)
  - golden-review-checklist.md — per-note review RECORD with sign-off + policy decision
affects: [phase-40-governance, live-LLM-F1-operator-gate, future-corpus-reviews]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Golden-set harness convention: empty-vs-empty = perfect agreement at harness level (LLM-gate zero-denominator convention untouched)"
    - "verify script vacuous-run guard: regex -run stages must also clear a minimum --- PASS line count"
    - "Split-toolchain FE gate: runner cascade with explicit PHASE39_FE_GATE=skip-external operator acknowledgment"

key-files:
  created:
    - eval/ecl-golden/sql-guides.jsonl
    - internal/knowledge/ecl/extract_golden_test.go
    - scripts/verify-phase-39.sh
  modified:
    - .planning/phases/39-ecl-pipeline/golden-review-checklist.md

key-decisions:
  - "Golden set policy A (writes-only): read-only FROM/JOIN etl_* refs (~26 across 14 notes) deliberately NOT golden as select — prompt touch-semantics vs writes-only golden is a DOCUMENTED live-gate risk, not a phase defect"
  - "Row 039 corrected by human review: deleted 4 CREATE OR REPLACE FUNCTION false positives (3 function names + 1 parameter), kept the 3 real merge-into write targets — this is why the harness scores 039 at F1 0.6 BY DESIGN"
  - "Harness scoring: empty-vs-empty notes score (1,1,1) agreement — ScoreTables/ScoreRelated's pinned 0.0 zero-denominator convention stays for the LLM gate only"
  - "verify-phase-39.sh FE stage: node→pnpm runner cascade, then PHASE39_FE_GATE=skip-external as an explicit FIRST-position operator override (WSL here has no node + disabled interop + a broken Windows pnpm shim on PATH)"

patterns-established:
  - "Phase gate script: every regex `-run` stage pairs exit-code with a minimum `--- PASS` count so renamed tests cannot vacuously pass"
  - "Golden review provenance: reviewer evidence = deterministic SQL grep presented to the user; LLM excluded from all golden content decisions (D-GT-01 audit trail in-repo)"

requirements-completed: [ONT-10, ONT-11, ONT-12, ONT-13]

# Metrics
duration: 45min (df28992 12:20:48 → cad6d05 13:00:31 +08:00, checkpoint wait excluded)
completed: 2026-09-22
---

# Phase 39 Plan 09: Golden Set + F1 Harness + Hermetic Gate Summary

**Human-reviewed 26-row ECL golden set (policy A writes-only) committed with a stub-vs-golden macro-F1 harness gate (tables 0.9846 / related 1.0) and verify-phase-39.sh — the hermetic SC-1..6 all-green phase gate.**

## Performance

- **Duration:** ~45 min execution across Task 1 + Task 2 closeout + Task 3 (checkpoint wait excluded)
- **Started:** 2026-09-22T12:20:48+08:00 (Task 1 commit df28992)
- **Completed:** 2026-09-22T13:05+08:00
- **Tasks:** 3 of 3 (Task 2 was the D-GT-01 human review checkpoint — resolved: "approved A")
- **Files modified:** 4

## Accomplishments

- D-GT-01 fully closed: golden set at `eval/ecl-golden/sql-guides.jsonl` was regex-drafted by the stub (zero LLM involvement), then human-reviewed — user approved policy A (writes-only) with one correction batch on row 039
- SC-2 harness gate green: `TestStubVsGoldenF1` scores the deterministic stub against the reviewed golden — macro-F1 tables 0.9846, related 1.0, both >= 0.8
- SC-6 shipped: `scripts/verify-phase-39.sh` — 8-stage hermetic gate covering chunker boundary, F1 harness, extraction seam + math, ABox/manager, ontology tree, retriever, admin pipeline gates, FE contract, indexer routing; exits 0 all-green

## Task Commits

Each task was committed atomically:

1. **Task 1: Golden drafter + draft jsonl + review checklist** - `df28992` (feat)
2. **Task 2: HUMAN REVIEW checkpoint (D-GT-01)** — resolved "approved A" with 039 corrections; no commit (review record landed in Task 3's commit)
3. **Task 3: Apply review + F1 harness test + verify-phase-39.sh** - `cad6d05` (feat)

**Plan metadata:** (added after SUMMARY)

## Files Created/Modified

- `eval/ecl-golden/sql-guides.jsonl` — the committed 26-row ground truth (row 039 = 3 merge-into update targets after human correction)
- `.planning/phases/39-ecl-pipeline/golden-review-checklist.md` — review RECORD: per-row verdicts, sign-off (user, 2026-09-22, approved policy A), corrections log, live-gate-risk note
- `internal/knowledge/ecl/extract_golden_test.go` — `TestStubVsGoldenF1`: LoadGolden (hard-required, never skipped) + StubExtract over WalkAndMerge + MacroF1, pinned 26-row count with corpus-drift message, per-note transparency logging
- `scripts/verify-phase-39.sh` — hermetic SC-1..6 gate (WSL header, ok/fail counters, per-stage logs, PASS-line minimums, FE runner cascade, non-zero exit on any fail)

## Decisions Made

- **Policy A (writes-only)** — user's call: read-only `FROM`/`JOIN etl_*` references are NOT golden. Accepted implication recorded on the checklist: the live-LLM F1 operator gate scores touch-semantics extraction against a writes-only yardstick (documented risk, revisit only if live F1 nears 0.8).
- **Harness empty-vs-empty convention** — 24 of 26 notes have empty golden tables AND the stub deterministically emits none; the harness scores that agreement (1,1,1) before invoking ScoreTables/ScoreRelated. The pinned 0.0 zero-denominator convention (TestF1ScoreTablesEmptySides) is untouched — it remains the LLM-gate convention where "predicted nothing" must never be rewardable.
- **FE stage design** — see Deviations.

## Deviations from Plan

### Review-outcome note (not a deviation — by design)

**F1 = 0.6 on note 039 is the gate working correctly.** The human review deleted 4 stub false positives from golden row 039 (CREATE OR REPLACE FUNCTION artifacts), so the stub now over-predicts 7 vs golden 3 → P=3/7, R=1.0, F1=0.6 for that one note; the other 25 notes score 1.0 and macro-F1 is 0.9846. This number is the harness validating that stub, golden, and scoring all wire together — the legitimate place where "human corrections may legitimately pull it below 1.0" (plan's own words for the 0.8 gate).

### Auto-fixed Issues

**1. [Rule 3 - Blocking] FE suite cannot execute inside the WSL verify script**
- **Found during:** Task 3 (verify-phase-39.sh stage 7)
- **Issue:** This box's WSL has no node, WSL interop is disabled (`cmd.exe`/`node.exe` → Exec format error), and the only pnpm visible under WSL is the broken Windows shim on PATH — while the Go toolchain is WSL-only. The plan-literal `pnpm -C web test -- --run` can never run inside the same script invocation as the Go stages on this machine.
- **Fix:** Stage 7 resolves its runner: node (direct vitest, 39-08 precedent bypassing the broken pnpm pre-run hook) → pnpm → `PHASE39_FE_GATE=skip-external` as an explicit FIRST-position operator override acknowledging an out-of-band FE run. Without the variable, a missing/unusable FE runtime is a clean `[SC-FAIL]` and the gate exits non-zero (validated: strict-mode run recorded the failure without crashing). Never silent — the delegation prints `[SC-EXT]` lines and the summary line carries "(SC-4 UI delegated out-of-band)".
- **Files modified:** scripts/verify-phase-39.sh
- **Verification:** FE suite run out-of-band on the Windows side: 77 files / 628 tests passed, exit 0; strict-mode run exits FAIL cleanly; acknowledged run exits 0.
- **Committed in:** cad6d05

**2. [Rule 1 - Bug] Regex `-run` stages could pass vacuously**
- **Found during:** Task 3 (script assembly)
- **Issue:** `go test -run 'Pattern'` with zero matching tests exits 0 — a renamed test would silently void a stage.
- **Fix:** every regex-gated stage pairs the exit code with a minimum `--- PASS` line count (pinned pair = exactly 2; suites >= 10; named test = exact name grep).
- **Files modified:** scripts/verify-phase-39.sh
- **Committed in:** cad6d05

---

**Total deviations:** 2 auto-fixed (1 blocking environment, 1 bug), 1 by-design note
**Impact on plan:** Gate integrity preserved or strengthened; no scope creep.

## Issues Encountered

- WSL `bash -lc` probe initially reported `EXIT_CODE=0` for the strict-mode FAIL run — outer Git Bash expanded `$?` before WSL saw it (quoting bug in the probe command, not the script). Re-ran with single quotes: strict mode correctly exits 1.

## User Setup Required

None — no external service configuration required. (Live-LLM F1 operator gate remains a manual gate per RESEARCH; FE suite runs on the FE-toolchain side on this box.)

## Next Phase Readiness

- Phase 39's executable definition of done is green: `bash scripts/verify-phase-39.sh` exits 0 (8 passed / 0 failed, FE delegated with acknowledgment)
- Phase 39 remaining closeout: phase-level verification/audit per orchestrator; Phase 40 (Governance: SHACL + Provenance) builds on the ABox/KG surfaces this phase's gate protects
- Carry-forward: writes-only golden vs touch-semantics prompt is a recorded live-gate risk for whenever the live-LLM F1 operator gate first runs

## Self-Check: PASSED

- eval/ecl-golden/sql-guides.jsonl exists, 26 valid JSONL lines, row 039 exact per review — FOUND
- .planning/phases/39-ecl-pipeline/golden-review-checklist.md sign-off filled — FOUND
- internal/knowledge/ecl/extract_golden_test.go exists, TestStubVsGoldenF1 PASS (F1 0.9846 / 1.0) — FOUND
- scripts/verify-phase-39.sh exists, exits 0 all-green — FOUND
- Commits df28992 + cad6d05 present in git log — FOUND

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
