---
phase: 09-trust-floor-spike-p8-s-spike-hard-gate
plan: 03
subsystem: spike-runner
tags: [go, eino, history-injection, spike, GEVL-08, spike-only, dry-run]

# Dependency graph
requires:
  - phase: 09-trust-floor-spike-p8-s-spike-hard-gate
    provides: "Plan 09-01 75-row history-injection golden set + spike row schema (4 extension fields)"
  - phase: 09-trust-floor-spike-p8-s-spike-hard-gate
    provides: "Plan 09-02 history.Project() D9-B1/B2/B3 projection contract"

provides:
  - "cmd/spike-history binary (independent package main) — spike-only A/B runner, decoupled from cmd/eval"
  - "11-flag CLI: --golden / --out / --category / --judge-model / --limit / --seed / --with-history / --history-length / --summarize / --cost-limit / --dry-run"
  - "RunEntry() live + dry-run dual-path with 4 metrics + grounding + injection-penetration + cost guard"
  - "Per-entry JSONL stream (SpikeResult schema) consumed by Plan 09-05 summarize"
  - "Explicit stub gates for --history-length=long (→ Plan 09-04) and --summarize (→ Plan 09-05)"

affects: [phase 09-04, phase 09-05, phase 09-06, GEVL-08]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Independent spike-only package main (no reuse of cmd/eval's bootstrap)"
    - "Named return value (result, err) on RunEntry so defer-recover still produces non-nil result on the caller side"
    - "Dry-run mode uses sentinel chunk IDs 9991..9995 — high enough to never collide with real corpus chunks; deliberately avoids spike fake refs so dry-run always reports InjectionPenetrated=false"
    - "Per-entry JSONL append-mode writer (writeResult → <outDir>/<category>.jsonl) consumed by Plan 09-05"

key-files:
  created:
    - path: "cmd/spike-history/spike_row.go"
      role: "Spike row parser (loadSpikeGolden + filterByCategory + spikeRowHistoryTurnsToSessionMessages)"
    - path: "cmd/spike-history/runner.go"
      role: "RunEntry (live + dry-run dual-path) + writeResult + costState + dryRunHits + dryRunVerdict"
    - path: "cmd/spike-history/main.go"
      role: "Binary entry point — 11 flags + bootstrap mirror + main loop + cost-exceeded break"
    - path: ".planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-03-SUMMARY.md"
      role: "Plan completion record"
  modified: []

key-decisions:
  - "buildAgent added as 6th mirrored helper (5 mandated by Planner Ruling 1; 6th is a forward-compatibility hook for Plan 09-04/05)"
  - "Dry-run uses StubJudge with DefaultEntails=true / DefaultRelevant=true / DefaultAttributable=true — metrics return 1.0 deterministically; AnswerRelevancy is the one exception (needs embedder; logs a warn and scores 0 in dry-run)"
  - "Cost guard is wired but inert in dry-run (LimitUSD set but no API calls happen); Record/Exceeded safe on nil receiver so tests can opt out"
  - "ResolvedTiers passed as nil in dry-run (deps.Manifest is nil); trust floor still works correctly because dry-run never lets the accumulator be empty"
  - "Package doc + per-helper 'Mirrored from cmd/eval/main.go:L###' comments preserve the planner's refactor window for Phase 10"

patterns-established:
  - "Spike runner declares planner-ruling attributions in error strings (--history-length=long: not implemented in Plan 09-03; see Plan 09-04) so operators can grep to the owning plan"
  - "Panic guard via defer recover() uses named return so the recovered result still flows back to the main loop as a non-nil *SpikeResult (the canonical 'recovered panic collapses to nil' bug)"
  - "Dry-run guard pattern around every deps.Manifest / deps.Embedder access — bootstrap is skipped in dry-run so these are nil and would otherwise nil-panic"

requirements-completed: [GEVL-08]

# Metrics
duration: "13m45s"
completed: "2026-08-18T03:14:24Z"
---

# Phase 9 Plan 3: spike-only history-injection runner Summary

**`cmd/spike-history` — independent spike-only binary that runs the 25 history-injection golden rows through live + dry-run dual paths, writes per-entry JSONL with the 4 metrics + grounding + injection-penetration markers, and gates --history-length=long / --summarize to Plan 09-04 / 09-05 with grep-friendly error strings.**

## Performance

- **Duration:** 13m45s (2026-08-18T03:00:39Z to 2026-08-18T03:14:24Z)
- **Started:** 2026-08-18T03:00:39Z
- **Completed:** 2026-08-18T03:14:24Z
- **Tasks:** 3
- **Files modified:** 3 (1 added in Task 1, 1 added in Task 2, 1 expanded in Task 3)

## Accomplishments

- **`cmd/spike-history` builds independently of `cmd/eval` and `internal/server`** — zero changes to the hard-constraint surface (`git diff --stat cmd/eval/ internal/server/ eval/frozen/` empty).
- **11-flag CLI** — golden / out / category / judge-model / limit / seed / with-history / history-length / summarize / cost-limit / dry-run. Defaults match the planner's manifest (categories=adversarial,followup; judge-model=glm-5.2; cost-limit=5.00 USD).
- **RunEntry() live + dry-run dual path** — 8-step pipeline: history projection (history.Project) → preRetrieve (embed + retriever.Search) → messages build → chat.Generate → trust floor + grounding (agent.ApplyTrustFloor + verdict.GroundingCheck) → 4 metrics (Faithfulness / AnswerRelevancy / ContextPrecision / ContextRecall) → injection-penetration check → cost accumulation.
- **Dry-run mode is hermetic** — no API keys, no Milvus, no BGE-M3. StubJudge gives deterministic 1.0 metrics; dryRunHits returns synthetic chunks 9991..9995 (deliberately avoid spike fake refs so dry-run always reports InjectionPenetrated=false — a live verdict-pipeline bug would surface when switching to live mode).
- **Stub gates** — `--history-length=long` exits with `"--history-length=long: not implemented in Plan 09-03; see Plan 09-04"`; `--summarize=x` exits with `"--summarize=x: not implemented in Plan 09-03; see Plan 09-05"`. Both fail BEFORE bootstrap so operators do not need GLM_API_KEY.
- **Cost guard** — `costState.Record(msgs)` accumulates `bytes(content)/2 * 0.10 USD/MTok`; `Exceeded()` returns true when the cumulative spend reaches `--cost-limit`. Main loop `break`s on Exceeded (preserves already-written JSONL rows).
- **Per-entry JSONL schema (SpikeResult)** — locked and ready for Plan 09-05 summarize consumer. 7 mandatory fields: `run_id`, `with_history`, `history_length`, `category`, `metrics` (4 floats), `grounding_pass`, `injection_penetrated`. Optional: `baseline_id`, `subclass`, `anchor_turn`, `fake_chunk_ref`, `penetrated_fake_ref`, `cost_usd`, `ts`.

## Task Commits

Each task was committed atomically:

1. **Task 1: `spike_row.go` — spike row parser + category filter + history_turns translator** - `7632923` (feat) — 235 lines (≥80 required)
2. **Task 2: `runner.go` — RunEntry + writeResult + costState + dry-run stubs** - `b0b4571` (feat) — 516 lines (≥200 required)
3. **Task 3: `main.go` — flag parsing + bootstrap mirror + main loop** - `cdd1377` (feat) — 375 lines (≥200 required) + runner.go hardening for dry-run

## Files Created/Modified

- `cmd/spike-history/spike_row.go` — `spikeRow` struct (embeds `eval.Case` + 4 spike fields) + `loadSpikeGolden` (mirrors `eval.LoadGolden` discipline) + `filterByCategory` + `spikeRowsToBaselineIDs` + `spikeRowHistoryTurnsToSessionMessages`. Apache-2.0 header.
- `cmd/spike-history/runner.go` — `runnerDeps` + `costState` + `SpikeResult` + `RunEntry` (8 steps + panic recover) + `buildUserMessageBody` + `dryRunHits` + `dryRunVerdict` + `writeResult`. Apache-2.0 header.
- `cmd/spike-history/main.go` — 11-flag parser + bootstrap mirror (5 helpers + `buildAgent` forward-compat) + main loop with cost-exceeded break + stub gates. Apache-2.0 header.
- `.planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-03-SUMMARY.md` — this completion record.

## Decisions Made

- **buildAgent is the 6th bootstrap helper** (Plan 09-03 mandate was 5). The 6th is a forward-compat hook for Plan 09-04/05 to flip from `chat.Generate` direct calls to `agent.Run`. Not consumed by Plan 09-03; presence here means future plans don't need to re-derive it.
- **dryRunVerdict citations deliberately avoid spike fake refs.** The dry-run + adversarial row combo ALWAYS reports `InjectionPenetrated=false` — that is the intentional signal that the live verdict-pipeline logic is what actually detects (or fails to detect) injection. A live verdict-pipeline bug that cited the fake chunk would surface as `InjectionPenetrated=true` when the operator switches from `--dry-run` to live.
- **dryRunHits IDs start at 9991.** Real corpus chunks (compliance Skill's `knowledge/authority/*.md`) are well below 1000, and spike fake refs are `CH-901..CH-910` — both far from 9991..9995, so no collisions in the dry-run ChunkByID lookup.
- **Named return value `(result *SpikeResult, err error)` on RunEntry.** Without named returns, a recovered panic collapses the return to `(nil, nil)`, and the main loop's `writeResult` would emit `"null\n"` JSONL rows. The bug was caught during the first dry-run smoke and fixed before commit.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Recovered panic collapsed RunEntry return to nil (would have written `"null\n"` JSONL rows)**
- **Found during:** Task 3 verification (`go run ./cmd/spike-history --dry-run --limit=2 --category=adversarial --out=/tmp/spike-dry-test/`).
- **Issue:** First dry-run smoke produced a `null` JSONL file. Cause: `defer func() { if r := recover(); ... }()` was declared but RunEntry used an unnamed return `(result *SpikeResult, err error)`. Go's defer-recover semantics: after a recovered panic, the function returns the zero values of the return types — so `(nil, nil)` was returned. The main loop's `writeResult` then serialized a nil result.
- **Fix:** Switched the return signature to a named return `(result *SpikeResult, err error)` and assigned `result = &SpikeResult{...}` at the top of the function so the deferred recover still produces a non-nil result.
- **Files modified:** `cmd/spike-history/runner.go`.
- **Verification:** Second dry-run smoke produces 2 valid JSONL rows with all 7 mandatory fields present (verified via `python -c "import json;...subset check..."` per the plan's verification block).
- **Committed in:** `cdd1377` (Task 3 commit — included as part of the runner hardening that landed alongside main.go).

**2. [Rule 1 - Bug] `agent.RenderSystemPrompt(deps.Manifest)` nil-panicked in dry-run mode (deps.Manifest is nil)**
- **Found during:** Same Task 3 smoke run (surfaced as the panic that #1 was supposed to catch but couldn't because of the return signature bug).
- **Issue:** Dry-run mode does not bootstrap — `deps.Manifest` is nil. `agent.RenderSystemPrompt(deps.Manifest)` dereferences the nil pointer.
- **Fix:** Wrapped the system-prompt construction with `if !deps.DryRun { sys.Content = agent.RenderSystemPrompt(deps.Manifest) }`. Dry-run now sets the sentinel `[DRY-RUN system prompt omitted]` so a developer reading the log line does not see a 1.5 KiB prompt body.
- **Files modified:** `cmd/spike-history/runner.go`.
- **Verification:** Dry-run smoke now completes 2 rows without panicking.
- **Committed in:** `cdd1377` (same Task 3 commit).

**3. [Rule 1 - Bug] `deps.Manifest.ResolvedTiers()` nil-panicked in `ApplyTrustFloor` call**
- **Found during:** Same Task 3 smoke run (after fix #1 unmasked the panic path through `ApplyTrustFloor`).
- **Issue:** Same nil-manifest pattern: `ApplyTrustFloor(acc, row.Query, deps.Manifest.ResolvedTiers())` dereferences nil.
- **Fix:** Extracted `var resolvedTiers []string; if !deps.DryRun && deps.Manifest != nil { resolvedTiers = deps.Manifest.ResolvedTiers() }`. The trust floor still produces the correct outcome because dry-run never lets the accumulator be empty (dryRunHits always populates 5 chunks).
- **Files modified:** `cmd/spike-history/runner.go`.
- **Verification:** Dry-run smoke + Step 5 trust floor + Step 6 metrics all complete without panicking; `grounding_pass=true` and `metrics={faithfulness:1, context_precision:1, context_recall:1, answer_relevancy:0}` (answer_relevancy is 0 because dry-run has no embedder — known acceptable, see Decisions).
- **Committed in:** `cdd1377` (same Task 3 commit).

---

**Total deviations:** 3 auto-fixed (3 bugs, all caused by the same root pattern: dry-run skips bootstrap so several `deps.X` fields are nil and direct dereference nil-panics). All caught and fixed during the dry-run smoke verification step the plan mandated.

**Impact on plan:** All three bugs were surfaced by the plan's `--dry-run --limit=2 --category=adversarial` smoke step and fixed within Task 3's commit. No scope creep, no architectural changes. The fixes preserve the dry-run contract (zero API calls, zero deps required) while keeping the live-mode path untouched.

## Verification (Plan §Verification)

| Check | Result |
|-------|--------|
| `go build ./cmd/spike-history && go vet ./cmd/spike-history && go build ./... && go vet ./...` | PASS (exit 0) |
| `--dry-run --limit=2 --category=adversarial --out=/tmp/spike-dry-test/` | PASS — 2 valid JSONL rows written |
| `/tmp/spike-dry-test/adversarial.jsonl` schema | PASS — 7-key subset check via `python -c "import json;...subset check..."` (JSON-OK printed) |
| `--history-length=long --limit=1 --out=/tmp/x/` | PASS — exits 1 with `"[FAIL] --history-length=long: not implemented in Plan 09-03; see Plan 09-04"` |
| `--summarize=x --limit=1 --out=/tmp/x/` | PASS — exits 1 with `"[FAIL] --summarize=x: not implemented in Plan 09-03; see Plan 09-05"` |
| `git diff --stat cmd/eval/ internal/server/ eval/frozen/ cmd/server/` | PASS — empty (hard constraint: zero production code touched) |
| `git diff --stat go.mod go.sum` | PASS — empty (hard constraint: zero new dependencies) |

## Threat Posture

| Threat | Disposition | Status |
|--------|-------------|--------|
| T-9-LLM-COST | mitigate | **Mitigated.** `costState.Record(msgs)` uses `bytes/2 * 0.10 USD/MTok`; `Exceeded()` returns true when `TotalUSD >= LimitUSD` (default $5.00). Main loop `break`s on Exceeded, preserving already-written JSONL rows. Plan 09-06 live UAT must reconcile the estimate against the actual GLM invoice (the approximation is documented as `±50%` per the plan's threat model). |
| T-9-SPIKE-DRIFT | mitigate | **Mitigated.** `--seed=42` injected into `math/rand.Seed` (used by future Plan 09-04 noise fill). `SpikeResult.RunID = "<id>-<with_history>-<unixnano>"` so any drift traces to one specific call. |
| T-9-INJECTION-ESCAPE | mitigate | **Mitigated.** dryRunVerdict citations deliberately avoid spike fake refs (9991..9995 ≠ CH-901..CH-910), so dry-run + adversarial row always reports `InjectionPenetrated=false`. Live mode runs the real verdict pipeline where injection detection depends on the LLM's grounding behavior — exactly the SC-3 signal the plan needs. |
| T-9-CHATMODEL-PANIC | mitigate | **Mitigated.** `defer recover()` in RunEntry; panic prints a `[WARN]` log line and sets `Metrics["error"]=1, Refused=true` so the main loop continues to subsequent rows. |
| T-9-SC | accept | **n/a.** Zero new dependencies — `go.mod` and `go.sum` untouched. |

## Known Stubs

- `--history-length=long` — explicit error gate to Plan 09-04 (grep-friendly prefix in the error string).
- `--summarize` — explicit error gate to Plan 09-05 (same convention).
- `answer_relevancy` metric in dry-run mode scores 0 because the dry-run path does not bootstrap an embedder. The warn is logged (`eval.AnswerRelevancy: embedder is nil — scoring 0`) and the other 3 metrics (faithfulness / context_precision / context_recall) return 1.0 deterministically via StubJudge defaults. Live mode resolves this naturally — embedder is wired in the bootstrap.
- `buildAgent` helper is mirrored but unused by Plan 09-03 (Plan 09-03 deliberately does not call `agent.Run` because it does not accept history — Planner Ruling 6). The helper is included as a forward-compat hook for Plan 09-04/05.

## Issues Encountered

- **The first dry-run smoke crashed with `null` JSONL rows.** Root cause: the `defer recover()` pattern in RunEntry used an unnamed return, so a recovered panic collapsed to `(nil, nil)` — see Deviation #1. The plan's verification step (`go run ... --dry-run --limit=2 --category=adversarial`) caught this immediately. All three fixes (#1, #2, #3) are part of the same Task 3 commit because they all surfaced from the same smoke run.
- **Path translation between bash and Windows:** `/tmp/spike-dry-test/` on this Windows machine maps to `C:/Users/CPIC/AppData/Local/Temp/spike-dry-test/`. The plan's Python verification step uses `/tmp/spike-dry-test/adversarial.jsonl` literally; on Windows this resolves correctly because Git Bash translates `/tmp` → `/tmp` only in Git Bash context. The Python interpreter in `cmd.exe` context uses the literal `/tmp/spike-dry-test/` path which doesn't exist there. Verified by running the Python check from `bash` (the agent's shell) where the translation happens — the check prints `JSON-OK`.

## User Setup Required

None - no external service configuration required. The binary is hermetic in `--dry-run` mode (zero API keys / Milvus / BGE-M3 required). Live mode requires the same env vars as `cmd/eval`: `GLM_API_KEY`, `BGE_M3_URL`, `MILVUS_ADDRESS` (or `MONGO_URI`), `SQLITE_DSN` — all already documented in `.env.example` from earlier phases.

## Next Phase Readiness

Plan 09-04 (long-history spike path) and Plan 09-05 (summarize report) can both start in parallel:
- **Plan 09-04** extends `RunEntry`'s `--history-length=long` branch (currently a stub error). The function signature and 8-step pipeline are already in place — only the long-history fill (D9-D2中立合规问答) and the dedicated chunk-accumulator state for the longer pipeline need to be added.
- **Plan 09-05** consumes the JSONL stream Plan 09-03 writes (`<outDir>/<category>.jsonl`). The SpikeResult schema is locked in this plan. The summarize entry point is already gated in main.go (line 154: `--summarize` stub gate); Plan 09-05 implements `runSummarize` and removes the gate.

Plan 09-06 (live UAT) can run independently once both 09-04 and 09-05 land — it executes the live-mode binary against the full 75-row golden and validates the SC-1 / SC-3 / SC-4 numbers from the ROADMAP §Phase 9 success criteria.

## Self-Check

- [x] All three task commits exist on disk: `7632923` (Task 1), `b0b4571` (Task 2), `cdd1377` (Task 3).
- [x] All three created files exist: `cmd/spike-history/spike_row.go` (235 lines), `cmd/spike-history/runner.go` (516 lines), `cmd/spike-history/main.go` (375 lines).
- [x] `go build ./cmd/spike-history && go vet ./cmd/spike-history && go build ./... && go vet ./...` exit 0.
- [x] Dry-run smoke produces 2 valid JSONL rows; Python schema subset check prints `JSON-OK`.
- [x] Stub gates fire correctly: `--history-length=long` and `--summarize` both exit 1 with grep-friendly error messages.
- [x] Hard-constraint surface unchanged: `git diff --stat cmd/eval/ internal/server/ eval/frozen/ cmd/server/` is empty.
- [x] Zero new dependencies: `git diff --stat go.mod go.sum` is empty.

---
*Phase: 09-trust-floor-spike-p8-s-spike-hard-gate*
*Completed: 2026-08-18*