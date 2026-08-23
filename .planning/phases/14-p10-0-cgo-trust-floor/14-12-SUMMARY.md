---
phase: 14-p10-0-cgo-trust-floor
plan: 12
subsystem: platform-trust
tags: [eval, refusal-accounting, frozen-locks, re-baseline, no-falsify, environment-stop]

requires:
  - phase: 14-p10-0-cgo-trust-floor
    plan: 11
    provides: cmd/eval classifyRunOutcome with in-schema refused verdict recognition + honest 0.888 STOP record routing to re-baseline Alternative

provides:
  - cmd/eval computeCaseMetrics helper with explicit refused=true → faithfulness=1.0 short-circuit
  - TestRefusedCaseFaithExemption hermetic regression lock locking the re-baseline semantics
  - Updated classifyRunOutcome godoc making the exemption a code-level contract
  - Honest environment-STOP record: BGE-M3 endpoint unreachable (no route to host); no replay executed; no falsified numbers

affects: [Phase 14 closeout, v1.0 milestone frozen-baseline ledger, eval-gate semantics]

tech-stack:
  added: []
  patterns:
    - "Explicit frozen exemption in scoring loop: refused verdicts short-circuit faithfulness before calling the chunk judge, making the re-baseline semantics independent of the metric implementation's empty-answer path"
    - "Hermetic regression lock with hostile StubJudge: refuses can be tested without live LLM calls"

key-files:
  created: []
  modified:
    - cmd/eval/main.go
    - cmd/eval/main_test.go

key-decisions:
  - "D14-G2 adopted: in-schema refused verdicts are explicitly exempt from faithfulness judging against retrieved chunks — the exemption lives in computeCaseMetrics, not in faithfulness.go's empty-answer short-circuit"
  - "Environment STOP honored: BGE-M3 endpoint no route to host prevented the authorized compliance replay; no replay was run, no metrics were fabricated, and platform-floor.lock.json 1.1.0 re-sign remains deferred"

patterns-established:
  - "When the live gate cannot run for environmental reasons, record the probe evidence and STOP cleanly rather than substituting prior numbers or running offline"

requirements-completed: [PLAT-09, PLAT-11]

# Metrics
duration: 16m
completed: 2026-08-23
---

# Phase 14 Plan 12: Re-baseline D14-B4 Gate Semantics + Platform-Floor.lock.json 1.1.0 Re-sign (Round-4 FINAL Gap Closure) Summary

**Explicit faithfulness exemption for in-schema refused verdicts code-locked with hermetic RED→GREEN regression test; authorized compliance replay blocked by BGE-M3 endpoint no-route-to-host, so platform-floor.lock.json 1.1.0 formal re-sign remains deferred under no-falsify discipline.**

## Performance

- **Duration:** 16m
- **Started:** 2026-08-23T04:22:04Z
- **Completed:** 2026-08-23T04:38:25Z
- **Tasks:** 3 (Task 1 complete + committed; Task 2 environment STOP; Task 3 closeout)
- **Files modified:** 2 production files

## Accomplishments

- **D14-B4 re-baseline semantics code-locked:** `computeCaseMetrics` in `cmd/eval/main.go` now explicitly short-circuits `faithfulness=1.0` when `refused=true`, before any call to `eval.Faithfulness`. The exemption is no longer implicit via the empty-answer path in `internal/eval/faithfulness.go:75-78`.
- **Hermetic RED→GREEN regression test shipped:** `TestRefusedCaseFaithExemption` uses a hostile `StubJudge` (`DefaultEntails=false`, `DefaultAttributable=false`, `DefaultRelevant=false`) plus refusal prose and hostile chunks; it asserts `refused=true` yields `faith=1.0` / `crecall=1.0` while `refused=false` yields `faith=0.0` / `crecall=0.0`.
- **classifyRunOutcome godoc updated:** the "Downstream conventions" block now documents that the 14-12 exemption is explicit in the scoring loop via `computeCaseMetrics`.
- **Full hermetic gate chain green after the fix:** `go test ./cmd/eval/`, `go test ./internal/eval/`, `go build ./...`, `go vet ./...`, `bash scripts/verify-phase-14.sh` (11/11 PASS), `make verify-phase-13` (9/9 PASS). Frozen locks + golden set `git diff --quiet` exit 0.

## Task Commits

1. **Task 1 Step 1: RED regression test** — `9015f4d` (test)
2. **Task 1 Step 2: GREEN explicit exemption** — `8340a10` (fix)

**Plan metadata:** this docs commit (Task 3).

## Files Created/Modified

- `cmd/eval/main.go` — extracted `computeCaseMetrics` with explicit `refused=true → faith=1.0` short-circuit; replaced inline runEvalPass 3c metric calls; updated `classifyRunOutcome` godoc.
- `cmd/eval/main_test.go` — added `TestRefusedCaseFaithExemption` hermetic regression lock plus local `stubEmbedder` helper.

## Decisions Made

- **D14-G2 adopted (re-baseline Alternative):** in-schema refused verdicts are judged against the frozen empty-answer convention, not against retrieved chunks. The exemption point is now `computeCaseMetrics`, making the semantics a code-level contract.
- **No-falsify STOP on environment failure:** the BGE-M3 endpoint was unreachable (`no route to host`). Rather than substitute the 14-11 replay numbers or skip the replay silently, the plan's STOP branch was triggered and the failure was recorded.

## Deviations from Plan

None - plan executed exactly as written. The environment STOP was a pre-declared branch condition, not a deviation.

## STOP Record — Task 2 single authorized compliance replay (14-12)

**Verdict: ENVIRONMENT STOP — BGE-M3 endpoint unreachable; replay NOT executed. Re-sign of `platform-floor.lock.json`, README caveat, and docs commit NOT executed.**

Pre-flight probe evidence (WSL Ubuntu-22.04, .env loaded):

```
BGE_M3_URL=http://10.62.0.123:11434/v1/embeddings
MILVUS_ADDRESS=10.62.10.34:19530
curl: (7) Failed to connect to 10.62.0.123 port 11434 after 4931 ms: No route to host
HTTP_BGE=000
```

The Milvus endpoint returned HTTP 404 to a plain HTTP GET probe, which is consistent with a gRPC service that does not speak unframed HTTP; the decisive failure is BGE-M3 at `10.62.0.123:11434` returning `No route to host`.

Because the eval pipeline requires `BGE_M3_URL` for both query embedding and answer-relevancy judging, running `cmd/eval --frozen=compliance --archive` under this network condition would fail immediately or produce invalid numbers. Per the plan's environment-STOP clause, the replay was not started and no metrics were recorded or fabricated.

Consequences:
- `build/1412-replay.log` does not exist.
- `eval/reports/<ts>.json` from this plan does not exist.
- `eval/report.json` was not overwritten by this plan (any pre-existing file is from earlier work and is untracked).
- `eval/frozen/platform-floor.lock.json` keeps its current honest state (`platform_floor_version=1.1.0` + 2026-08-22 metrics/signature).
- `eval/frozen/README.md` was not updated with a new caveat or fifth-lock entry.

## Issues Encountered

- **BGE-M3 endpoint unreachable.** Probe at `10.62.0.123:11434` returned `No route to host`. This is an infrastructure/environment condition, not a code defect. It prevents the authorized compliance replay from running.
- **WSL bash variable expansion quirk.** Initial attempts to probe endpoints inline failed because the Bash tool pre-expanded `$VAR` references before passing the command to WSL bash. This was resolved by writing probe commands to a script file and executing `bash /path/to/script.sh`, then the script was deleted.

## User Setup Required

None for the code changes. The **planner/user decision** from 14-10-SUMMARY/14-11-SUMMARY remains open, but it is now gated on an operator restoring the BGE-M3 endpoint before any authorized compliance replay can run. Once the endpoint is back:

```bash
# WSL Ubuntu-22.04
export PATH="$PATH:/usr/local/go/bin"
cd /mnt/d/code/ClaudeCode/ai-agent
source <(tr -d '\r' < .env | grep -E '^[A-Za-z_][A-Za-z0-9_]*=')
go run ./cmd/eval --frozen=compliance --archive
```

If the replay then passes (`overall.faithfulness >= 0.9`), execute the 14-10-PLAN Task 2 steps 3-5 chain (fill-lock + README caveat + docs commit). If it fails on a healthy service, escalate per the 14-10-SUMMARY Operator-Gate Addendum.

## Next Phase Readiness

- The 14-12 hermetic re-baseline is in place and locked; the measurement-layer semantics are now explicit and future-proof.
- The platform-floor.lock.json 1.1.0 formal re-sign remains deferred until a live replay can run in a healthy environment.
- Phase 15+ data-surface work can proceed on the 1.1.0 floor behavior; the only remaining Phase 14 item is the frozen-baseline formality.

## Self-Check: PASSED

- `cmd/eval/main.go` contains `computeCaseMetrics` with `if refused { faith = 1.0 }` — FOUND
- `cmd/eval/main.go` `classifyRunOutcome` godoc mentions "14-12 update" and `computeCaseMetrics` — FOUND
- `cmd/eval/main_test.go` contains `TestRefusedCaseFaithExemption` — FOUND
- `go test ./cmd/eval/ -run TestRefusedCaseFaithExemption -count=1 -v` PASS — verified
- `go test ./cmd/eval/ -run TestClassifyRunOutcome -count=1 -v` PASS — verified
- `bash scripts/verify-phase-14.sh` 11/11 PASS — verified
- `make verify-phase-13` 9/9 PASS — verified
- Frozen locks + golden set `git diff --quiet` exit 0 — verified
- `eval/frozen/platform-floor.lock.json` and `eval/frozen/README.md` NOT modified by this plan — verified
- Throwaway scripts `scripts/tmp-1412-*.sh` deleted — verified
- SUMMARY.md committed — FOUND
- STATE.md and ROADMAP.md updated — verified

---

*Phase: 14-p10-0-cgo-trust-floor*
*Completed: 2026-08-23*
