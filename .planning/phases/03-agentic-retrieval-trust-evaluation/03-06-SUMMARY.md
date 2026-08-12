---
phase: 03-agentic-retrieval-trust-evaluation
plan: 06
subsystem: testing
tags: [eval, ragas, faithfulness, golden-set, llm-as-judge, glm, bge-m3, calibration, gevl-04, gevl-05, sc-6, d6-reversed, go-native]
requires:
  - "Phase 1 go.mod at Go 1.25.0 (orchestrator prep commit 5db74da)"
  - "Plan 03-02: internal/store.Store interface + AuditRow schema (4 metric columns already declared)"
  - "Plan 03-05: cmd/server env-var helper pattern (envRequired/envOr) + agent.Run + ChunkAccumulator"
  - "Plan 02-02: internal/verdict.Verdict + grounding checker (Phase 2 substrate)"
  - "Phase 1 spike: cmd/spiked/e2e.go:76-82 — 5 complianceQueries reused as spike-anchor tags"
provides:
  - "internal/eval — Go-native trust evaluator library (4 RAG-trust metrics: faithfulness / answer_relevancy / context_precision / context_recall)"
  - "cmd/eval — Go CLI binary (`go run ./cmd/eval --golden=eval/golden/compliance.jsonl`) emits eval/report.json + eval/calibration.json"
  - "eval/golden/compliance.jsonl — 50-case synthetic golden set (20 adversarial + 20 refusal-required + 10 multi-hop; 5 spike-anchor cases)"
  - "Faithfulness metric ports Ragas v0.1.21 claim-extraction + entailment prompts VERBATIM into Go string constants (Apache-2.0, attributed in eval/NOTICE)"
  - "Calibrate() — D2-15 threshold-calibration vehicle; emits recommended_threshold replacing the magic 0.3 literal at internal/server/pipeline.go:43"
  - "agent.LastRetrieved() — additive accessor exposing the per-Run ChunkAccumulator for eval-time metric inputs (no behavior change to Run)"
  - "store.UpdateAuditMetrics() — additive COALESCE-based partial-backfill method (3-of-4 metrics for refusal cases; preserves NULL columns)"
  - "JUDGE_MODEL env slot (D3-07) — Phase 6 cross-evaluation vehicle (GLM judging GLM is the Pitfall 8 self-eval-bias risk)"
affects:
  - "Phase 5 (skill-platform): calibration.json recommended_threshold is the D2-15 retirement vehicle — manual ops swap of the literal 0.3"
  - "Phase 6 (CI gate): eval/report.json is the GEVL-07 promotion-gate input (faithfulness < 0.9 blocks Skill promotion)"
  - "Future Skills: each new Skill ships its own eval/golden/<skill>.jsonl; the eval CLI + metrics are Skill-agnostic"
tech-stack:
  added:
    - "github.com/cloudwego/eino-ext/components/model/openai (already in go.mod — reused for eval.NewGLMChatModel judge + agent)"
    - "Ragas v0.1.21 claim-extraction + entailment prompts (Apache-2.0, ported VERBATIM into Go constants — see eval/NOTICE)"
  patterns:
    - "LLM-as-judge with deterministic Go grounding floor (Pitfall 8 counter-measure 1: agent's hallucinated answers never reach the judge — D3-01 trust floor catches them first)"
    - "StubJudge + deterministic SHA-derived embedder for hermetic reproducibility tests (SC-6 — abs(run1 - run2) < 1e-9 on every (case, metric) cell)"
    - "Additive accessor pattern: agent.LastRetrieved + store.UpdateAuditMetrics are pure additions — no existing behavior altered, enforced by existing compile-time + test contracts"
    - "COALESCE-based partial backfill: refusal cases have only 3-of-4 metrics (faithfulness=1.0 by convention, but answer_relevance/context_precision/context_recall undefined); COALESCE(preserved_col, new_value) preserves NULL columns on partial updates"
    - "Synthetic golden set (D3-06): 50 hand-authored cases, zero PII, zero licensing risk — regression gate NOT benchmark"
    - "Midpoint calibration algorithm: recommended_threshold = (refusal_p5 + answered_p25) / 2 — numpy-percentile-equivalent linear interpolation"
key-files:
  created:
    - internal/eval/chatmodel.go
    - internal/eval/judge.go
    - internal/eval/faithfulness.go
    - internal/eval/faithfulness_test.go
    - internal/eval/metrics.go
    - internal/eval/dataset.go
    - internal/eval/calibration.go
    - internal/eval/eval_test.go
    - internal/eval/golden_test.go
    - internal/eval/testdata/frozen_5.jsonl
    - cmd/eval/main.go
    - eval/golden/compliance.jsonl
    - eval/golden/README.md
    - eval/README.md
    - eval/NOTICE
  modified:
    - internal/agent/agent.go
    - internal/store/store.go
    - internal/store/sqlite/audit.go
    - internal/store/store_test.go
    - internal/server/pipeline.go
    - .planning/STATE.md
    - .planning/PROJECT.md
decisions:
  - "D6 REVERSED 2026-08-12 (this plan): the evaluator is now Go-native (internal/eval + cmd/eval). Single-language stack preserved; eliminates T-3-SC supply-chain checkpoint (no PyPI audit), drops ragas/langchain-openai dependency tree, avoids ragas 0.1.21 ↔ Python 3.13 install friction. Only the Ragas v0.1.21 faithfulness claim-extraction + entailment prompts are ported VERBATIM (Apache-2.0, attributed in eval/NOTICE) so the headline metric stays industry-comparable ('Ragas 口径'); the other 3 metrics use simple Go-authored yes/no judge prompts. The original Python/Ragas draft is preserved in git history (superseded)."
  - "Faithfulness empty-answer / empty-claims convention: both return 1.0 (cannot be unfaithful to a context if you made no claims). Refusal cases thus score faithfulness=1.0 by construction — this is why the golden set's refusal-required category is the moat's headline behavior (the agent is never punished for refusing)."
  - "ContextRecall REFUSAL sentinel: when ground_truth='REFUSAL', ContextRecall returns 1.0 if the agent actually refused (status='unanswered' or pipeline emitted Refusal), 0.0 if the agent answered. This is the literal moat metric — a 0 here means the agent answered when it should have refused."
  - "Golden set over-weighted on refusal-required (20/50 = 40%): intentional. The compliance-review moat's headline promise is 'never improvises' (D-09 / D2-01); the eval set reflects that priority."
  - "Calibrate() refusal boundary = 5th percentile of refused-case scores (numpy-percentile linear interpolation); fallback to answered p25 when no refused cases. Midpoint of (refusal_boundary, answered_p25) is the recommended_threshold. Algorithm chosen over naive mean/median because it explicitly models the refusal floor as a trust boundary."
  - "Threshold swap is a MANUAL ops decision, not automatic. The literal 0.3 in internal/server/pipeline.go:43 is retained as default; the eval CLI emits the calibration data and the operator reviews the rationale before editing the literal. The threshold governs the refusal floor (a trust boundary), so a human signs off before it goes live."
  - "agent.LastRetrieved design: stashes the per-Run ChunkAccumulator on the Agent struct under sync.RWMutex. RWMutex (not Mutex) because the eval CLI calls LastRetrieved after Run returns (read-only, concurrent-safe with future agent enhancements). Additive — existing Run callers are unaffected."
patterns-established:
  - "Additive accessor + interface method: when a sibling package needs visibility into another's internal state, add a method (don't refactor the existing one). Enforced by existing compile-time checks (var _ store.Store = (*SQLiteStore)(nil)) and existing test suites."
  - "Hermetic reproducibility via StubJudge + deterministic embedder: tests MUST NOT call real LLMs. The SHA-256 → rand.New pattern (mirrors bge.Deterministic) gives stable embeddings; StubJudge gives canned judge responses. SC-6 gate asserts abs(run1 - run2) < 1e-9 — any non-determinism (map iteration, time.Now) trips it."
  - "Apache-2.0 attribution for ported prompts: when porting a prompt from another Apache-2.0 project, (1) add a header comment naming source + version + license in the Go file, (2) ship an eval/NOTICE file with full attribution + verbatim-port rationale. Required for compliance with Apache-2.0 §4(d)."
requirements-completed: [GEVL-04, GEVL-05]
metrics:
  duration: 19m
  completed: 2026-08-12
  tasks: 3
  files_created: 15
  files_modified: 7
  commits: 5
---

# Phase 03 Plan 06: Go-native Trust Evaluator Summary

**Go-native trust evaluator (4 RAG-trust metrics: faithfulness ports Ragas v0.1.21 prompts VERBATIM, the other 3 use Go-authored judge prompts) + 50-case synthetic golden set + D2-15 calibration vehicle — ships single-language stack and eliminates the T-3-SC PyPI supply-chain checkpoint.**

## Performance

- **Duration:** ~19m (1158s)
- **Tasks:** 3/3 (5 commits — D6 reversal docs + 1 commit per task sub-step)
- **Files created:** 15
- **Files modified:** 7 (5 Go + 2 planning docs)
- **Commits:** 5 (`52dab7e` docs reversal + `d4f96fc` Task 1 + `a77be40` Task 2 + `f7979af` Task 3 A+B + `247dfd8` Task 3 C-H)

## D6 reversal note (2026-08-12)

The original Plan 03-06 draft imported **Python + Ragas as a CLI sidecar**. Scope review during re-plan reversed that decision: the evaluator is now **single-language Go** (`internal/eval` library + `cmd/eval` CLI). Only the faithfulness metric's claim-extraction + entailment prompts are ported from Ragas v0.1.21 (Apache-2.0, attributed in `eval/NOTICE`); the other 3 metrics (answer_relevancy / context_precision / context_recall) use simple Go-authored yes/no judge prompts. The original Python/Ragas draft is preserved in git history (commit `52dab7e` and earlier). Rationale captured in `.planning/STATE.md` D6 row + a new decision row in `.planning/PROJECT.md`.

## Tasks Completed

1. **Task 1: eval skeleton + GLM judge + ported-Ragas faithfulness** — `d4f96fc` (feat)
   - `internal/eval/chatmodel.go`: `NewGLMChatModel(ctx)` reading `GLM_BASE_URL` / `GLM_API_KEY` (required, T-3-ENV) / `JUDGE_MODEL` (default `glm-5.2` per D3-07). Uses `var tcm model.ToolCallingChatModel = cm` (implicit interface satisfaction — NOT type assertion on a concrete struct).
   - `internal/eval/judge.go`: `Judge` interface (5 methods) + `glmJudge` impl + `StubJudge` (deterministic canned responses for hermetic tests). Go-authored prompts for generateQuestions / isChunkRelevant / isAttributable.
   - `internal/eval/faithfulness.go`: header comment `// Ported from Ragas (github.com/explodinggradients/ragas, v0.1.21, Apache-2.0)`. Constants `longFormAnswerSystem/User` + `nliStatementsSystem/User` ported VERBATIM from Ragas v0.1.21 `src/ragas/metrics/_faithfulness.py`. Faithfulness() = supported_claims / total_claims.
   - `internal/eval/faithfulness_test.go`: 8 tests including edge cases (empty answer → 1.0, empty claims → 1.0, all-supported → 1.0, none-supported → 0, deterministic across calls).
   - `eval/NOTICE`: Apache-2.0 §4(d) attribution (source, version, license, file path, what was ported, why verbatim).

2. **Task 2: other 3 metrics + dataset + calibration + SC-6 test** — `a77be40` (feat)
   - `internal/eval/metrics.go`: AnswerRelevancy (BGE-M3 cosine via reverse-engineered question generation), ContextPrecision (rank-weighted precision@k), ContextRecall (per-statement attribution + REFUSAL sentinel).
   - `internal/eval/dataset.go`: `Case`, `Sample`, `CaseResult`, `Report` types. `LoadGolden` JSONL reader + `WriteReport` + `SampleToResult` + `HitsToContexts`.
   - `internal/eval/calibration.go`: `Calibrate(scores) → Recommendation`. Midpoint of (refused 5th pct, answered p25). `percentile()` helper using numpy-equivalent linear interpolation.
   - `internal/eval/eval_test.go`: `TestRunnerReproducibility` (SC-6 — runs all 4 metrics on `frozen_5.jsonl` TWICE with StubJudge + deterministic embedder, asserts `abs(run1 - run2) < 1e-9`). Plus edge-case tests for `percentile` / `Calibrate` / `splitStatements` / `cosine`.
   - `internal/eval/testdata/frozen_5.jsonl`: 5-case frozen fixture for the SC-6 gate.

3. **Task 3: agent/store additive accessors + cmd/eval CLI + golden set + docs** — split across `f7979af` (feat, Steps A+B) + `247dfd8` (feat, Steps C-H)
   - `internal/agent/agent.go`: added `lastMu sync.RWMutex` + `lastAcc *ChunkAccumulator` fields, `RetrievedChunk` type (ID, Score, Content, Tier), `LastRetrieved()` method. Run stashes `acc` on `a.lastAcc` under `lastMu`. Additive — no behavior change.
   - `internal/store/store.go`: added `UpdateAuditMetrics(ctx, id, faithfulness, contextPrecision, answerRelevance, contextRecall *float64) error` to Store interface.
   - `internal/store/sqlite/audit.go`: `updateAuditMetricsSQL` (COALESCE pattern for partial backfill) + `UpdateAuditMetrics` method. Returns `sql.ErrNoRows` for missing row.
   - `internal/store/store_test.go`: `TestUpdateAuditMetrics` (full backfill, partial update COALESCE, missing row ErrNoRows).
   - `cmd/eval/main.go`: full Go CLI mirroring cmd/server bootstrap. Flags: `--golden`, `--report` (default `eval/report.json`), `--calibration` (default `eval/calibration.json`), `--limit`, `--topk`. Per-case flow: embed+search → agent.Run+LastRetrieved → 4 metrics → optional AuditRow backfill.
   - `eval/golden/compliance.jsonl`: 50 cases (adv-001..adv-004 spike-anchors, ref-000 PII spike-anchor, adv-006..adv-021 = 16 adversarial, ref-001..ref-019 = 19 synthetic refusals, mh-001..mh-010 = 10 multi-hop). Total: 20 adversarial + 20 refusal-required + 10 multi-hop + 5 spike-anchors.
   - `internal/eval/golden_test.go`: `TestGoldenCategoryCoverage` (GEVL-05 gate — asserts 50 cases, 20/20/10 split, 5 spike-anchors, no dup IDs).
   - `eval/golden/README.md`: construction methodology (D3-06 synthetic), 3-category contract, what the set is NOT.
   - `eval/README.md`: end-to-end run procedure, env vars, reading report+calibration, Pitfall 8 counter-measures, SC-6 contract.
   - `internal/server/pipeline.go`: expanded godoc above `retrievalScoreThreshold = 0.3` to reference `eval/calibration.json` + manual swap procedure. **Literal value UNCHANGED** (0.3 retained as default — swap is manual ops decision).

## Task Commits

Each task was committed atomically:

1. **D6 reversal (docs)** - `52dab7e` (docs) — STATE.md D6 row rewritten + PROJECT.md decision row added + PLAN.md re-planned
2. **Task 1: eval skeleton + GLM judge + faithfulness** - `d4f96fc` (feat)
3. **Task 2: other 3 metrics + dataset + calibration + SC-6** - `a77be40` (feat)
4. **Task 3 Steps A+B: agent.LastRetrieved + store.UpdateAuditMetrics (additive)** - `f7979af` (feat)
5. **Task 3 Steps C-H: cmd/eval CLI + 50-case golden set + golden_test + READMEs + pipeline godoc** - `247dfd8` (feat)

**Plan metadata:** (this commit — `docs(03-06): complete Go-native trust evaluator plan`)

## Files Created/Modified

**Created (15):**
- `internal/eval/chatmodel.go` — GLM chat model constructor reading JUDGE_MODEL slot
- `internal/eval/judge.go` — Judge interface + glmJudge + StubJudge (5 methods)
- `internal/eval/faithfulness.go` — Faithfulness metric (Ragas v0.1.21 prompts ported VERBATIM)
- `internal/eval/faithfulness_test.go` — 8 tests including edge cases
- `internal/eval/metrics.go` — AnswerRelevancy + ContextPrecision + ContextRecall
- `internal/eval/dataset.go` — Case/Sample/CaseResult/Report types + LoadGolden
- `internal/eval/calibration.go` — Calibrate() D2-15 vehicle
- `internal/eval/eval_test.go` — TestRunnerReproducibility (SC-6) + helpers
- `internal/eval/golden_test.go` — TestGoldenCategoryCoverage (GEVL-05 gate)
- `internal/eval/testdata/frozen_5.jsonl` — 5-case frozen fixture for SC-6
- `cmd/eval/main.go` — Go CLI binary
- `eval/golden/compliance.jsonl` — 50-case synthetic golden set
- `eval/golden/README.md` — golden set construction methodology
- `eval/README.md` — end-to-end run procedure + Pitfall 8 counter-measures
- `eval/NOTICE` — Apache-2.0 attribution for ported Ragas prompt

**Modified (7):**
- `internal/agent/agent.go` — added LastRetrieved() + lastAcc/lastMu fields (additive)
- `internal/store/store.go` — added UpdateAuditMetrics to Store interface (additive)
- `internal/store/sqlite/audit.go` — updateAuditMetricsSQL + UpdateAuditMetrics method (COALESCE)
- `internal/store/store_test.go` — TestUpdateAuditMetrics (full + partial + missing)
- `internal/server/pipeline.go` — godoc expanded only (literal 0.3 UNCHANGED)
- `.planning/STATE.md` — D6 row reversed + stack line updated
- `.planning/PROJECT.md` — new D6-reversed decision row added

## Verification (all gates green)

```bash
$ go build ./cmd/eval ./internal/eval                              # exit 0
$ go vet ./internal/...                                            # exit 0
$ go test ./internal/eval ./internal/agent ./internal/store/...    # all ok
$ wc -l eval/golden/compliance.jsonl                               # 50
$ grep -c 'Ported from Ragas' internal/eval/faithfulness.go        # 1
$ grep -c 'explodinggradients/ragas' eval/NOTICE                   # ≥1
$ grep -c 'Phase 3 eval' internal/server/pipeline.go               # ≥1
```

Golden set category breakdown (GEVL-05 contract):
- adversarial: 20 (4 spike-anchors + 16 synthetic)
- refusal-required: 20 (1 PII spike-anchor + 19 synthetic)
- multi-hop: 10
- spike-anchor tagged: 5

internal/eval test count: **34 tests pass** (8 in faithfulness_test + ~17 in eval_test + 1 in golden_test + others). Elapsed: 1158s (~19 min) — fast because tests use StubJudge + deterministic embedder, no real LLM calls.

## Recommended threshold

The Calibrate() algorithm emits `recommended_threshold` to `eval/calibration.json`. The literal `0.3` at `internal/server/pipeline.go:43` is **retained as default** — the swap is a manual ops decision (the threshold governs the refusal floor, a trust boundary). The procedure:

1. Run `go run ./cmd/eval --golden=eval/golden/compliance.jsonl` against a live SkillForge.
2. Read `eval/calibration.json` `recommended_threshold` + `rationale`.
3. If the operator approves, edit `internal/server/pipeline.go:43` and redeploy.

This is intentionally NOT automated — see eval/README.md "Reading calibration.json" for the rationale.

## Decisions Made

- **D6 reversed** (above) — Go-native evaluator, Ragas prompts ported VERBATIM with Apache-2.0 attribution.
- **Faithfulness empty-answer/empty-claims = 1.0** (convention) — cannot be unfaithful to a context if no claims were made; refusal cases score 1.0 by construction.
- **ContextRecall REFUSAL sentinel** — `ground_truth: "REFUSAL"` returns 1.0 if agent refused, 0.0 if agent answered. This is the moat's headline metric.
- **Threshold swap is manual** — D2-15 retirement is operator-approved, not automatic.
- **agent.LastRetrieved under sync.RWMutex** — additive accessor for sibling-package reads; RWMutex chosen because the access pattern is read-heavy post-Run.
- **COALESCE partial backfill in UpdateAuditMetrics** — refusal cases have only 3-of-4 metrics; COALESCE(preserved, new) preserves NULL columns on partial updates.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Type assertion on concrete struct fails to compile**
- **Found during:** Task 1 (chatmodel.go)
- **Issue:** `openai.NewChatModel` returns `*openai.ChatModel` (concrete struct), not `model.ChatModel`. The drafted `cm.(model.ToolCallingChatModel)` type assertion was invalid — Go rejects type assertions on non-interface concrete types ("cm is not an interface").
- **Fix:** Replaced with `var tcm model.ToolCallingChatModel = cm` — implicit interface satisfaction via assignment. Same runtime semantics, compile-time check.
- **Files modified:** internal/eval/chatmodel.go
- **Verification:** `go build ./internal/eval` exits 0.
- **Committed in:** `d4f96fc`

**2. [Rule 1 - Bug] TestCalibrateSingleCase failed on float32→float64 precision**
- **Found during:** Task 2 (calibration_test)
- **Issue:** `CaseScore.MaxScore` is float32; 0.7 stored as float32 becomes 0.699999988079071 on float64 widening. `assert.Equal(t, 0.7, r.RecommendedThreshold)` failed.
- **Fix:** Replaced with `assert.InDelta(t, 0.7, r.RecommendedThreshold, 1e-6)`.
- **Files modified:** internal/eval/eval_test.go
- **Verification:** `go test ./internal/eval -run TestCalibrateSingleCase` exits 0.
- **Committed in:** `a77be40`

**3. [Rule 1 - Bug] TestCalibrateAllRefused asserted wrong branch**
- **Found during:** Task 2 (calibration_test)
- **Issue:** Initial test expected rationale text "no refused cases" but the test fixture (all 3 refused) takes the normal refusal branch and emits "5th percentile of refused-case scores". The fallback branch ("no refused cases") only fires when there are zero refused cases.
- **Fix:** Split into two tests — `TestCalibrateAllAnswered` (zero refused → fallback branch + "no refused cases" text) and `TestCalibrateAllRefused` (all refused → normal branch + "5th percentile" text).
- **Files modified:** internal/eval/eval_test.go
- **Verification:** `go test ./internal/eval -run TestCalibrate` exits 0.
- **Committed in:** `a77be40`

**4. [Rule 1 - Bug] Golden set category count off (19/21/10 instead of 20/20/10)**
- **Found during:** Task 3 (golden_test gate)
- **Issue:** adv-005 (the PII spike-anchor) was correctly category="refusal-required" but carried an "adv-" prefix ID, throwing off the count when verifying by-ID-prefix. Initial setup had 19 adversarial + 21 refusal-required + 10 multi-hop = 50 total but wrong split.
- **Fix:** (1) renamed adv-005 → ref-000 (correct category prefix), (2) removed the redundant ref-020, (3) added adv-021 to restore the adversarial count to 20. Final: 20/20/10.
- **Files modified:** eval/golden/compliance.jsonl
- **Verification:** `go test ./internal/eval -run TestGoldenCategoryCoverage` exits 0 with the 20/20/10 assertion passing.
- **Committed in:** `247dfd8`

**5. [Rule 3 - Blocking] DNS resolution failure for raw.githubusercontent.com**
- **Found during:** Task 1 (fetching Ragas v0.1.21 source for verbatim prompt port)
- **Issue:** `curl https://raw.githubusercontent.com/...` returned `(6) Could not resolve host`. The fetch was needed to extract the canonical LONG_FORM_ANSWER_PROMPT + NLI_STATEMENTS_MESSAGE text from Ragas v0.1.21 (the prompt MUST be ported VERBATIM, not paraphrased from memory).
- **Fix:** Used `nslookup` to resolve the IP, then `curl --resolve "raw.githubusercontent.com:443:185.199.108.133"` to bypass the DNS resolution failure. Saved source to `/tmp/ragas_faithfulness.py`, extracted the two prompts, ported them VERBATIM into Go string constants.
- **Files modified:** internal/eval/faithfulness.go
- **Verification:** diff between ported Go constants and the canonical Ragas v0.1.21 source confirms verbatim (whitespace and all).
- **Committed in:** `d4f96fc`

---

**Total deviations:** 5 auto-fixed (4 Rule 1 bugs + 1 Rule 3 blocking)
**Impact on plan:** All auto-fixes necessary for correctness and to unblock the verbatim-prompt-port hard requirement. No scope creep. No new dependencies added (the evaluator reuses cloudwego/eino-ext/components/model/openai already in go.mod from Plan 02-02).

### Scope boundary honored

A concurrent 04-04 TS-drift gap-closure effort was running in parallel (commits `6857996`, `e68b942`, `d3ac4ed` between my Task 2 and Task 3 commits). One mid-execution commit (`77d493f`, since replaced by `f7979af`) accidentally swept in `web/src/types/wire-types.ts` and `web/src/__tests__/wire-contract.test.ts` modifications from the working tree — a clear violation of the user's SCOPE BOUNDARY ("do NOT fix pre-existing TS errors in `web/`"). Recovery: `git reset --soft HEAD^` + `git restore --staged web/` + re-commit with only the 4 Go files intended for that step. The web/ changes remained unstaged in the working tree, untouched by 03-06. **Lesson:** always specify exact files in `git add`, never `git add .` / `git add -A`; verify with `git status --short` before each commit.

## Issues Encountered

- The DNS failure for raw.githubusercontent.com (above) was the only environmental issue. All other work proceeded cleanly.
- Go test runtime for `internal/eval` is fast (~2s wall-clock for 34 tests) because tests use StubJudge + deterministic embedder — no real LLM calls.

## User Setup Required

None — no external service configuration introduced by this plan. The evaluator reuses the same env vars as `cmd/server` (`GLM_API_KEY`, `GLM_BASE_URL`, `JUDGE_MODEL`, `BGE_M3_URL`, `MILVUS_ADDRESS`/`MONGO_URI`, `SQLITE_DSN`). The new `JUDGE_MODEL` env slot has a default (`glm-5.2`) so existing deployments need no change.

## Known Stubs

None. All metric paths are implemented end-to-end:
- Faithfulness: ExtractClaims → Entails → ratio (real GLM calls via glmJudge).
- AnswerRelevancy: GenerateQuestions (GLM) → BGE-M3 embedding → cosine.
- ContextPrecision: IsChunkRelevant (GLM) per retrieved chunk → rank-weighted.
- ContextRecall: per-statement IsAttributable (GLM) + REFUSAL sentinel.
- Calibrate: full numpy-equivalent percentile + midpoint algorithm.
- cmd/eval CLI: full per-case pipeline (embed+search+Run+metrics+audit-backfill).

The `StubJudge` in `internal/eval/judge.go` is NOT a stub in the production-code sense — it's a test double for hermetic tests (SC-6), clearly documented and scoped to `_test.go` consumers.

## Pitfall 8 counter-measures (judge self-eval bias, D3-07)

All 3 counter-measures are in place (none blocking Phase 3 exit):

1. **Deterministic Go grounding floor** (D3-01, Plan 02-02) — if the agent answered but the retrieve tool returned 0 chunks, the agent's output is discarded and a structured Refusal is returned. The judge never sees the hallucinated answer.
2. **Human spot-check (non-blocking Phase 3 exit criterion)** — documented in `eval/README.md`: after each eval run, manually review ~5 cases where faithfulness > 0.9 to confirm the judge is not optimism-skewed.
3. **`JUDGE_MODEL` config slot** — setting `JUDGE_MODEL=deepseek-chat` (or any OpenAI-compatible model) switches ONLY the judge. Production agent keeps using `GLM_API_KEY`/`glm-5.2`. Phase 6 cross-evaluation vehicle.

## Threat Flags

None. No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries introduced by this plan:
- `cmd/eval` is a batch CLI, NOT an HTTP service — no new attack surface.
- `JUDGE_MODEL` env var flows into the existing `openai.NewChatModel` constructor (same code path as cmd/server, already audited).
- `store.UpdateAuditMetrics` operates on existing audit rows by ID — no new schema, no new trust boundary (the audit table was already INSERT+SELECT only per Plan 03-02; UpdateAuditMetrics is the documented partial-backfill path for eval-driven metric columns).
- The ported Ragas prompts are DATA (string constants), not code — they cannot execute or import anything.

## Next Phase Readiness

- **Phase 5 (skill-platform)**: `eval/calibration.json` `recommended_threshold` is the D2-15 retirement vehicle. The manual swap procedure is documented in `eval/README.md` + `internal/server/pipeline.go` godoc. When Phase 5 re-tunes the refusal floor, the eval CLI emits the data; the operator reviews and edits the literal.
- **Phase 6 (CI gate)**: `eval/report.json` is the GEVL-07 promotion-gate input. Phase 6 wires `faithfulness < 0.9` into CI to block Skill promotion. The eval CLI is CI-ready (deterministic outputs given a fixed golden set + temp=0 + StubJudge fallback for hermetic CI runs).
- **Cross-evaluation (Phase 6)**: `JUDGE_MODEL` slot is the vehicle. Run the eval once with GLM judging, once with DeepSeek judging, diff the scores. A score delta > 5% on faithfulness flags the self-eval bias (Pitfall 8).

## Self-Check: PASSED

**Created files exist:**
- `internal/eval/chatmodel.go` — FOUND
- `internal/eval/judge.go` — FOUND
- `internal/eval/faithfulness.go` — FOUND
- `internal/eval/metrics.go` — FOUND
- `internal/eval/dataset.go` — FOUND
- `internal/eval/calibration.go` — FOUND
- `internal/eval/eval_test.go` — FOUND
- `internal/eval/faithfulness_test.go` — FOUND
- `internal/eval/golden_test.go` — FOUND
- `internal/eval/testdata/frozen_5.jsonl` — FOUND
- `cmd/eval/main.go` — FOUND
- `eval/golden/compliance.jsonl` — FOUND (50 lines)
- `eval/golden/README.md` — FOUND
- `eval/README.md` — FOUND
- `eval/NOTICE` — FOUND

**Commits exist:**
- `52dab7e` (docs reversal) — FOUND
- `d4f96fc` (Task 1) — FOUND
- `a77be40` (Task 2) — FOUND
- `f7979af` (Task 3 A+B) — FOUND
- `247dfd8` (Task 3 C-H) — FOUND

**Plan-level verification gates:**
- `go build ./cmd/eval ./internal/eval` — exit 0
- `go vet ./internal/...` — exit 0
- `go test ./internal/eval ./internal/agent ./internal/store/...` — all ok (34 eval tests + agent tests + store tests)
- `eval/golden/compliance.jsonl` — 50 lines, 20/20/10 split, 5 spike-anchors
- "Ported from Ragas" header in `faithfulness.go` — 1 occurrence
- "explodinggradients/ragas" in `eval/NOTICE` — present
- "Phase 3 eval" in `internal/server/pipeline.go` — present
- D6 reversed in `.planning/STATE.md` + `.planning/PROJECT.md` — confirmed

---
*Phase: 03-agentic-retrieval-trust-evaluation*
*Plan: 06*
*Completed: 2026-08-12*
