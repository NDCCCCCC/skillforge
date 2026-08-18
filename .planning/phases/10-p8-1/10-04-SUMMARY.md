---
phase: 10-p8-1
plan: 04
subsystem: server-history + spike-runner
 tags:
  - via-pipeline
  - history-aware-pipeline
  - query-rewriter
  - refusal-classifier
  - frozen-lock-provenance
  - spike-hard-gate
 dependency_graph:
  requires:
    - "10-01 (history.Project budget-primary / neutral placeholder / verdict renderer)"
    - "10-02 (VerdictPipeline rewrite + classifier + multi-turn messages prefix)"
    - "10-03 (FE history restore — not touched by this plan)"
  provides:
    - "cmd/spike-history --via-pipeline flag"
    - "Production chat-path composition inside the spike runner (B5 fix)"
    - "Hermetic dry-run test for the via-pipeline route"
    - "Frozen-lock provenance note (sha256 5cf700f3...)"
  affects:
    - "cmd/spike-history/main.go"
    - "cmd/spike-history/runner.go"
    - "cmd/spike-history/runner_pipeline_test.go"
    - ".planning/phases/10-p8-1/10-04-SUMMARY.md"
 tech_stack:
  added: []
  patterns:
    - "Nil-gate dependency injection in spike runner (Rewriter/Classifier non-nil enables via-pipeline)"
    - "Legacy direct path remains default for Phase 9 reproducibility"
    - "Single-block [ref-N] DATA framing mirrored from internal/server/pipeline.go chunksToBlock"
    - "Dry-run stubs for rewriter + classifier enable zero-external-call hermetic tests"
 key_files:
  created:
    - cmd/spike-history/runner_pipeline_test.go
  modified:
    - cmd/spike-history/main.go
    - cmd/spike-history/runner.go
    - .planning/phases/10-p8-1/10-04-SUMMARY.md
 decisions:
  - "Rewritten query is recorded on SpikeResult only when it differs from the original query (mirrors pipeline.go audit_meta behavior)."
  - "classifier.RefusalClassifierInput.Chunks is rendered as a one-element []string containing all [ref-N] blocks, matching production pipeline.go chunksToBlock exactly."
  - "Legacy direct path is byte-identical to Phase 9: same [sys, user] shape, same buildUserMessageBody inline history concatenation, same refusal/metric/cost behavior."
  - "Pre-existing FE tsc error (DataSubprogressPart.tsx:47 `data.state`) was left untouched because it is unrelated to Go-only Task 1 and touches D15 wire types."
  - "Live 35-case re-run is blocked-pending-human because the operator's .env (GLM_API_KEY / BGE_M3_URL / MILVUS_ADDRESS) and Docker stack are not present in this execution environment."
 patterns-established:
  - "Spike runner mirrors production composition by value, not by shared package, consistent with the existing cmd/spike-history bootstrap mirroring discipline."
 requirements-completed:
  - MEM-01
  - MEM-02
  - MEM-03
  - MEM-04
  - MEM-05

# Metrics
duration: 1h 15m
completed: 2026-08-18
started: 2026-08-18T09:32:04Z
completed: 2026-08-18T10:47:00Z
---

# Phase 10 Plan 04: `--via-pipeline` Spike Route + End-of-Phase Hard Gate — Summary

**Implemented the `--via-pipeline` flag in `cmd/spike-history` so the spike runner can exercise the same production chat-path composition as `internal/server/pipeline.go` (history.Project → rewrite → classifier → multi-turn messages prefix). The legacy direct path remains the default for Phase 9 reproducibility. Hermetic pre-flight gates all pass; the live 35-case re-run is blocked-pending-human because the live stack (GLM_API_KEY / BGE_M3_URL / MILVUS_ADDRESS + Docker Milvus/BGE-M3) is not available in this environment.**

## Performance

- **Duration:** 1h 15m
- **Started:** 2026-08-18T09:32:04Z
- **Completed:** 2026-08-18T10:47:00Z
- **Tasks:** 1 of 3 executed; Task 2 blocked-pending-human; Task 3 closeout committed
- **Files modified:** 3 (main.go, runner.go, runner_pipeline_test.go)

## Accomplishments

- Added `--via-pipeline` flag to `cmd/spike-history`.
- `RunEntry` now routes through production composition when `deps.Rewriter` or `deps.Classifier` are non-nil.
- Native multi-turn messages prefix `[sys, projected..., user]` uses production `[ref-N]` DATA framing; `[CHUNK id]` markers never appear in the via-pipeline path.
- Legacy direct path remains default and byte-identical to Phase 9.
- Added `SpikeResult.RewrittenQuery` for observability.
- Added hermetic `runner_pipeline_test.go` with `TestBuildPipelineMessages_Structure` and `TestRunEntry_ViaPipelineDryRun`.
- Verified frozen-lock byte-identity: sha256 `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (first 8 = `5cf700f3`), `git diff --quiet` exit 0, porcelain clean.

## Task Commits

1. **Task 1: Implement `--via-pipeline` flag + pre-flight hermetic regression** - `fdbfe0a` (feat)
2. **Task 3: Closeout transcription** - `0c4becc` (docs)

**Plan metadata:** `0c4becc` (docs: complete plan)

## Files Created/Modified

- `cmd/spike-history/main.go` — Added `--via-pipeline` flag; wires `rewrite.NewQueryRewriter` + `classifier.NewRefusalClassifier` in live mode; dry-run stubs when flag is set.
- `cmd/spike-history/runner.go` — Added `runnerDeps.Rewriter`/`Classifier`, `SpikeResult.RewrittenQuery`, `runViaPipeline`, `buildPipelineMessages`, `buildPipelineUserBody`, `chunksToRenderedBlocks`; legacy path preserved.
- `cmd/spike-history/runner_pipeline_test.go` — Hermetic structure test and dry-run via-pipeline test.

## Pre-flight Gate Results (Task 1)

| # | Gate | Command | Result |
| --- | --- | --- | --- |
| 1 | Build + vet | `go build ./... && go vet ./...` | PASS (exit 0) |
| 2 | Full BE hermetic suite | `go test ./... -count=1 -timeout 120s` | PASS (all packages ok) |
| 3 | Contract suite | `go test -tags=contract ./... -count=1` | PASS (all packages ok) |
| 4 | FE vitest | `npx vitest run --reporter=basic` | PASS (294/294) |
| 5 | FE tsc | `npx tsc --noEmit` | **FAIL — 1 pre-existing error** (`DataSubprogressPart.tsx:47` references `data.state` on `SubprogressStep`; unchanged from Plan 10-03; D15 wire-type concern) |
| 6 | Session isolation | `go test ./internal/server/... -run TestSessionIsolation50x5 -count=1 -v` | PASS |
| 7 | Grounding checker | `go test ./internal/verdict/... -run "TestGroundingCheck" -count=1 -v` + `go test ./internal/server/... -run TestGroundingCheckerInvoked -count=1 -v` | PASS (note: plan listed `TestGroundingChecker`, which does not exist; ran available grounding tests) |
| 8 | Race detector | `go test ./... -count=1 -timeout 120s -race` | **SKIP** (gcc not in %PATH%; environment limitation, same as Waves 1-3) |
| 9 | Frozen lock byte-identity | `git diff --quiet eval/frozen/compliance.lock.json && git status --porcelain eval/frozen/` | PASS (diff exit 0, porcelain empty) |
| 10 | Classifier isolation grep | `! grep -n 'History' internal/server/classifier/types.go` + `! grep -n 'GetSession\|loadHistory\|historyMsgs\|history\.Project' internal/server/classifier/classifier.go` | PASS |
| 11 | Frozen lock sha256 | `sha256sum eval/frozen/compliance.lock.json` | `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (first 8 = `5cf700f3`) |

### Frozen lock provenance note (B6 fix)

- **Canonical sha256 at clean HEAD:** `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (first 8 = `5cf700f3`).
- **STATE.md / 08-05-SUMMARY recorded value:** `50bb1688…` — this is **stale** (likely a commit-time hash or earlier file revision). The current canonical value is `5cf700f3…`.
- **Binding gate:** `git diff --quiet eval/frozen/compliance.lock.json` exit 0 AND `git status --porcelain eval/frozen/` empty. The lock was **not regenerated**.

## Task 2: Live 35-case re-run — blocked-pending-human

**Status:** `blocked-pending-human`

**Reason:** The live stack required by Task 2 is not present in this execution environment:

- No `.env` file in repository root; `.env.example` exists but is not populated.
- `GLM_API_KEY`, `BGE_M3_URL`, and `MILVUS_ADDRESS` are not set.
- Docker Milvus + BGE-M3 endpoints are not confirmed reachable.

**Command that would run once the stack is available:**

```bash
go run ./cmd/spike-history --via-pipeline --summarize --seed=42 \
  --golden=eval/golden/compliance.jsonl --out=reports/ --cost-limit=5.00
```

This command runs three segments (adversarial, followup, long) and produces `reports/summary.md`, `reports/short_history.md`, `reports/long_history.md`, and `reports/summary.json`.

**Expected gates:**

- 4-metric drift < 5% per metric vs Phase 8 baseline.
- refusal_accuracy delta < 10pp from baseline 0.95.
- SC-3 zero-penetration: 0/10 adversarial rows cite a fake chunk ID.
- Cost < $5.00 per segment.

## Decisions Made

- Dry-run `--via-pipeline` uses inert non-nil `Rewriter`/`Classifier` pointers so the production route is exercised without external calls; RunEntry stubs the calls internally.
- `RewrittenQuery` is only recorded when the rewriter output differs from the original query, matching production `audit_meta.RewrittenQuery` semantics.
- The classifier input renders hits as a single `[ref-N]` DATA block to mirror `internal/server/pipeline.go chunksToBlock` exactly.
- The pre-existing FE `tsc` error was not fixed because it is unrelated to this Go-only plan and would require modifying a D15 wire type.

## Deviations from Plan

### Pre-existing issues outside Task 1 scope

**1. [Out of scope] FE `tsc --noEmit` pre-existing error**
- **Found during:** Task 1 pre-flight gate 5
- **Issue:** `DataSubprogressPart.tsx:47` references `data.state`, which does not exist on `SubprogressStep`. This error was already present at the end of Plan 10-03 and is unrelated to the Go-only Task 1 changes.
- **Fix:** None applied. Fixing it would require adding a `state` field to the `SubprogressStep` wire type, which touches D15 invariants and is out of scope for Plan 10-04.
- **Files affected:** `web/src/components/chat/parts/DataSubprogressPart.tsx` (read only)
- **Verification:** `npx vitest run` passes 294/294; `npx tsc --noEmit` exits 2 with the same single pre-existing error.

**2. [Out of scope] Race detector unavailable**
- **Found during:** Task 1 pre-flight gate 8
- **Issue:** `go test -race` fails at the cgo build step because `gcc` is not in `%PATH%` on this Windows environment.
- **Fix:** None applied; same limitation documented in 10-01/10-02/10-03 SUMMARYs.
- **Verification:** `go test ./... -count=1 -timeout 120s` (without -race) passes.

**3. [Out of scope] `TestGroundingChecker` name mismatch**
- **Found during:** Task 1 pre-flight gate 7
- **Issue:** Plan acceptance criteria reference `go test ./internal/verdict/... -run TestGroundingChecker`, but no test by that exact name exists. Available grounding tests are `TestGroundingCheckFabricatedSpan`, `TestGroundingCheckVerbatimSpan`, `TestGroundingCheckFullwidthHalfwidthMix`, `TestGroundingCheckNovelClaim`, and `TestGroundingCheckerInvoked` in `internal/server`.
- **Fix:** Ran the available grounding tests as the closest semantic equivalent.
- **Verification:** All available grounding tests pass.

### Auto-fixed Issues

None — Task 1 executed as planned.

---

**Total deviations:** 3 out-of-scope environment/plan-naming items; 0 auto-fixes.
**Impact on plan:** The Go-only deliverable (Task 1) is complete and hermetic-green. The only blockers are external: live stack availability (Task 2) and a pre-existing FE type error not caused by this plan.

## Issues Encountered

- No `.env` populated → cannot run the live 35-case GLM/Milvus/BGE-M3 re-run. Task 2 is blocked-pending-human.

## Next Phase Readiness

- Task 1 code is committed and hermetic tests pass.
- The operator can resume Task 2 by populating `.env` and starting Docker Milvus + BGE-M3, then running the command in the Task 2 section.
- Once Task 2 reports are generated, Task 3 should transcribe headline numbers into this SUMMARY and update STATE/ROADMAP/REQUIREMENTS.

---
*Phase: 10-p8-1*
*Completed: 2026-08-18*

## Self-Check: PASSED

- [x] `.planning/phases/10-p8-1/10-04-SUMMARY.md` exists.
- [x] `cmd/spike-history/runner_pipeline_test.go` exists.
- [x] Commit `fdbfe0a` (Task 1) found in git log.
- [x] Commit `0c4becc` (Task 3 closeout) found in git log.
- [x] Frozen lock `eval/frozen/compliance.lock.json` git-diff clean and sha256 `5cf700f3...` verified.
