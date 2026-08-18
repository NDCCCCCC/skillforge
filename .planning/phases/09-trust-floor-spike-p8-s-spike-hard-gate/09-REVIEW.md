---
phase: 09-trust-floor-spike-p8-s-spike-hard-gate
reviewed: 2026-08-18T10:30:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - cmd/spike-history/long.go
  - cmd/spike-history/main.go
  - cmd/spike-history/runner.go
  - cmd/spike-history/spike_row.go
  - cmd/spike-history/summary.go
  - cmd/spike-history/summary_test.go
  - internal/eval/golden_test.go
  - internal/history/projection.go
  - internal/history/projection_test_fixtures.go
findings:
  critical: 4
  warning: 11
  info: 10
  total: 25
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-08-18T10:30:00Z
**Depth:** standard (with cross-file verification against `internal/eval`, `internal/agent`, `internal/verdict`, `internal/retriever`, `internal/store`, `cmd/eval/main.go`, and `eval/golden/compliance.jsonl`)
**Files Reviewed:** 9
**Status:** issues_found

## Summary

This spike-phase code (cmd/spike-history runner + report layer, internal/history projection skeleton, golden-set extension tests) has a sound long-row synthesizer (deterministic, seeded, no answer leakage into filler) and a well-constructed golden-set schema gate. However, the live measurement path has structural defects that invalidate the spike's core signals, and the report layer claims a hard-gate criterion (SC-4) it does not implement.

Key concerns, in order of severity:

1. **The default invocation runs the wrong corpus.** `--category` defaults to `adversarial,followup`, but the spike categories are `history_injection_adversarial`/`history_injection_followup` and `filterByCategory` is exact-match. The default silently runs 20 *baseline* rows (no history at all), writes them to `adversarial.jsonl`, and `--summarize` then fails with "zero categories".
2. **Live-mode grounding is broken by a dry-run constant leak.** `buildUserMessageBody` labels retrieved chunks `[CHUNK 9991..9995]` (`dryRunHitBaseID+i`) instead of the hit's real `h.ID`. In live mode the model can only cite phantom IDs, and `verdict.GroundingCheck` (keyed by real IDs) marks every citation ungrounded — `grounding_pass=false`, `ungrounded_cites=5` on every live row.
3. **SC-4 (model/prompt pin match) is claimed but never enforced** — `runSummarize` only echoes the lock's `model_ver`/`prompt_hash` into the report; there is no `AssertPin` call and `SpikeResult` does not even record the run's model/prompt identity.
4. **The cost-limit guard has contradictory disable semantics** — `Record` treats `LimitUSD <= 0` as disabled, but `Exceeded()` returns true at 0, so `--cost-limit=0` silently processes zero rows and the final log then overstates the written count in *all* stop scenarios.

Determinism of the long-history synthesizer, JSONL schema consistency between writer (`SpikeResult`) and reader (`readJSONL`/`computeCategoryMean`), and the B1/B2 projection behavior were verified against the golden data and internal package contracts and are correct as written (one fixture contradiction excepted — WR-04).

## Critical Issues

### CR-01: Default `--category` matches baseline rows, not spike rows — default run evaluates the wrong corpus

**File:** `cmd/spike-history/main.go:120` (and `cmd/spike-history/spike_row.go:159-180`)
**Issue:** The flag default is `adversarial,followup`, split into `["adversarial","followup"]`. `filterByCategory` performs exact, case-sensitive matching against `row.Category`. The golden set's spike categories are `history_injection_adversarial` (10 rows) and `history_injection_followup` (15 rows); the only rows matching `adversarial` are the **20 baseline adversarial rows** (which carry no `history_turns`). Running `go run ./cmd/spike-history` with defaults therefore:
- runs 20 baseline rows with `HistoryTurns == nil` → history projection is skipped entirely → the "history injection spike" measures nothing about history injection;
- writes results to `reports/adversarial.jsonl` (a filename the summarize layer ignores);
- `--summarize` then exits 2 with "zero categories present".

There is no warning that the selected categories contained zero spike rows. This is a silent wrong-corpus run that still burns live API spend.
**Fix:** Default to the spike categories and fail loudly if no spike rows survive the filter:
```go
category := flag.String("category",
    "history_injection_adversarial,history_injection_followup",
    "comma list of spike categories to run")

// after filtering:
spikeRows := 0
for _, r := range rows {
    if strings.HasPrefix(r.Category, "history_injection_") { spikeRows++ }
}
if spikeRows == 0 {
    log.Fatalf("[FAIL] cmd/spike-history: category set %v selected 0 history_injection_* rows "+
        "(did you mean history_injection_adversarial,history_injection_followup?)", categories)
}
```

### CR-02: Live-mode chunk labels use the dry-run ID base — grounding check structurally broken, model cites phantom chunk IDs

**File:** `cmd/spike-history/runner.go:445` (`buildUserMessageBody`)
**Issue:**
```go
fmt.Fprintf(&b, "[CHUNK %d] tier=%s\n%s\n---\n", dryRunHitBaseID+i, h.Tier, h.Content)
```
This runs in **both** dry-run and live mode. In live mode `hits` come from `deps.Retriever.Search` with real IDs (1..N per `retriever.Hit.ID`), but the prompt labels them `9991..9995`. The production path this function claims to mirror uses the hit's real ID (`internal/agent/retrieve_tool.go:211`: `"[CHUNK %d] tier=%s source=%s\n..." , h.ID, ...`). Consequences in every live row:
- the model can only cite `9991..9995` (the labels it sees), never the real chunk IDs;
- `agent.ChunkAccumulator.ChunksByID()` keys chunks by the real ID (`chunkKey(h.ID)`), so `verdict.GroundingCheck` finds no citation in the map → `grounding_pass=false` and `ungrounded_cites=len(citations)` unconditionally;
- the verdict's citations (and the marshaled-verdict `answer` fed to the judge) reference chunks that do not exist, corrupting faithfulness scoring inputs too.

The spike's headline live signals (grounding, citation behavior) are garbage in live mode. (The dry-run path is coincidentally self-consistent because `dryRunHits`/`dryRunVerdict` both use 9991..9995.)
**Fix:**
```go
for i, h := range hits {
    fmt.Fprintf(&b, "[CHUNK %d] tier=%s source=%s\n%s\n---\n", h.ID, h.Tier, h.Source, h.Content)
}
```
This restores ID consistency with `acc.ChunksByID()` and re-aligns the framing with the T-2-21 production envelope (see also IN-09).

### CR-03: SC-4 "model + prompt pins match Phase 8 frozen lock" is claimed but never enforced

**File:** `cmd/spike-history/summary.go:33` (header claim), `cmd/spike-history/summary.go:781-853` (`runSummarize`), `cmd/spike-history/summary.go:245-268` (`loadBaseline`)
**Issue:** The file header lists "GEVL-08 SC-4: model + prompt pins match Phase 8 frozen lock" as one of the four decision artifacts, and `renderSummary` has an "SC-4 Evidence" section. But the code only *copies* `lock.ModelVersion`/`lock.PromptHash` into `baselineMetrics` for display. There is no `eval.AssertPin` call anywhere in `cmd/spike-history`, and `SpikeResult` carries no `model_ver`/`prompt_hash` of its own run, so there is nothing to compare against. An operator who runs the spike with `--judge-model=glm-4.7` (which, via `JUDGE_MODEL` → `eval.NewGLMChatModel`, changes the *generation* model too) gets a GO/NO-GO verdict computed against a baseline produced by a different model/prompt, with no error and a Reproducibility table that still shows the pinned values. The gate criterion is decorative.
**Fix:** Record the run identity in `SpikeResult` (e.g. `ModelVersion string \`json:"model_ver"\`` + `PromptHash string \`json:"prompt_hash"\`` populated from the env + `reprohash.PromptHash(skill, model, RenderSystemPrompt(manifest))` at run start), and in `runSummarize` compare per-row identity against the lock and fail (exit 2, like other operator errors) on any mismatch:
```go
for _, rows := range spike {
    for _, r := range rows {
        if r.ModelVersion != base.ModelVersion || r.PromptHash != base.PromptHash {
            fmt.Fprintf(os.Stderr, "[FAIL] summary: SC-4 pin drift on %s: run model=%q hash=%q vs lock model=%q hash=%q\n",
                r.CaseID, r.ModelVersion, r.PromptHash, base.ModelVersion, base.PromptHash)
            return 2
        }
    }
}
```

### CR-04: Cost-limit guard disable semantics contradict each other; `--cost-limit=0` silently runs zero rows and the final log overstates the write count

**File:** `cmd/spike-history/runner.go:121-142` (`Record`/`Exceeded`), `cmd/spike-history/main.go:242-268`
**Issue:** `Record` documents "LimitUSD <= 0 disables the guard" and no-ops in that case, but `Exceeded()` is `c.TotalUSD >= c.LimitUSD` — with `LimitUSD == 0` and `TotalUSD == 0` this is `0 >= 0` → **true**. The main loop checks `cost.Exceeded()` before the first row, so `--cost-limit=0` (the natural "no cap" value, reinforced by the sibling `--limit` flag's "0 = no cap" convention) breaks immediately, writes nothing, and exits successfully. Compounding it, the final line `log.Printf("[OK] ... wrote %d rows ...", len(rows), ...)` prints `len(rows)`, not the number actually written — it is wrong for the `--cost-limit=0` case, for every legitimate mid-run cost stop, and for rows skipped via the `RunEntry` error-continue path.
**Fix:**
```go
func (c *costState) Exceeded() bool {
    return c != nil && c.LimitUSD > 0 && c.TotalUSD >= c.LimitUSD
}
```
and in `main.go`, count actual writes and failures:
```go
written, failed := 0, 0
// ... in the loop: written++ after writeResult; failed++ on the error-continue path
log.Printf("[OK] cmd/spike-history: wrote %d/%d rows (%d errors) to %s (cost=$%.4f)",
    written, len(rows), failed, *outDir, cost.TotalUSD)
```

## Warnings

### WR-01: Cost guard omits judge LLM calls and completion tokens, while accruing fake cost in dry-run

**File:** `cmd/spike-history/runner.go:76-134` (call sites at 295, 302, 309, 317, 338, 416), `cmd/spike-history/main.go:242`
**Issue:** `cost.Record` is only ever called with `{sys, user}`. The four metric functions (`eval.Faithfulness`, `AnswerRelevancy`, `ContextPrecision`, `ContextRecall`) each issue multiple judge LLM calls per row (claim extraction + per-claim entailment, question generation, per-chunk relevance, per-statement attribution) — none of these prompt/response bytes are recorded, nor is the generation response. In live mode the "hard USD cap" undercounts the true spend by a large factor (judge traffic typically dominates), so the cap can be silently exceeded — the exact risk T-9-LLM-COST exists to mitigate. Conversely, in dry-run mode `costState` is constructed with `LimitUSD=5.00` and `Record` runs, so dry-run rows carry nonzero fake `cost_usd` in the JSONL (contradicting "dry-run never spends anything"), which `runSummarize` then sums into `TotalCostUSD`.
**Fix:** Record the judge-side bytes inside the metric wrappers (or expose a `cost.RecordTokens(promptBytes, completionBytes)` called around each metric), and construct the costState only for live runs (`if *dryRun { cost = &costState{} }` so `LimitUSD<=0` disables it, consistent with the CR-04 fix).

### WR-02: Refusal/error early-returns record all-zero metrics while the baseline producer scores refusal rows with sentinel conventions — drift gate compares differently-derived numbers

**File:** `cmd/spike-history/runner.go:291-320, 336-341` vs `cmd/eval/main.go:424-469`
**Issue:** When `RunEntry` refuses (generate error, unmarshal/validate failure, trust floor), it returns early with `Metrics` empty (all reads yield 0). The Phase 8 baseline producer handles the same situations by still computing metrics under refusal conventions: `Faithfulness("", ctx)` → 1.0, `AnswerRelevancy(query, "")` → 0.0, `ContextPrecision(query, hits)` → computed, `ContextRecall(gt, hits, refused=true)` → 1.0 for `REFUSAL` ground truth. So every spike row that refuses/errors contributes faith=0/crecall=0 where the baseline-equivalent would have contributed 1.0 — a systematic ~1.0 downward bias per refused row feeding `computeVerdict`'s 5% drift gate. The followup category contains 5 `post_refusal` rows (REFUSAL ground truth), so trust-floor refusals or transient API errors there translate directly into spurious NO-GO drift.
**Fix:** Mirror cmd/eval's conventions in `RunEntry`: on the refusal/error paths set `result.Metrics["faithfulness"] = 1.0` (empty answer), `result.Metrics["context_recall"] = eval.ContextRecall(ctx, row.GroundTruth, hits, judge, true)` (or 1.0 for the REFUSAL sentinel), `answer_relevancy = 0`, and compute `context_precision` from `hits` — or at minimum tag the row with an `error`/`refused_reason` field so the summarize layer can exclude non-comparable rows instead of averaging them.

### WR-03: Live generation call omits `WithTemperature(0)` and any JSON-response enforcement

**File:** `cmd/spike-history/runner.go:291`
**Issue:** `deps.ChatModel.Generate(ctx, []*schema.Message{sys, user})` passes no options. The judge path pins determinism explicitly (`internal/eval/judge.go:130`: `model.WithTemperature(0)`), and the verdict JSON schema was graduated from spike 003 under `response_format:{type:json_object}` + `thinking:{type:disabled}` (documented in `internal/verdict/verdict.go:36-40`). At provider-default temperature the live spike's generation is nondeterministic (Plan 09-06's reproducibility comparison cannot pass) and the raw `json.Unmarshal([]byte(raw.Content), &v)` is fragile against prose/fenced output — every malformed response is silently converted into a "refusal" (see WR-02), polluting `refusal_rate`.
**Fix:** `deps.ChatModel.Generate(ctx, []*schema.Message{sys, user}, model.WithTemperature(0))` and parse via `llm.ExtractJSON(raw.Content)` the same way `glmJudge.generateJSON` does.

### WR-04: Fixture `neg_history_truncated_to_three_pairs` contradicts `Project()` and the stated B3 contract

**File:** `internal/history/projection_test_fixtures.go:112-131` vs `internal/history/projection.go:49-51`
**Issue:** Input is 4 user/assistant pairs + 1 tool message. After B1 drops the tool, 8 messages remain; B3 keeps the last 6 → pairs "old two", "three", "four". The fixture's `Expected` contains only the "three" and "four" pairs (4 messages). The fixture contradicts both the code and its own rationale ("B3 retains only the last three complete pairs" — the last three pairs include "old two"). Since this file is deliberately not a `_test.go` and is the executable handoff contract for Phase 10 ("for _, f := range Fixtures { require.Equal(t, f.Expected, Project(f.Input)) }"), Phase 10 will either fail on it or "fix" `Project` to match a wrong expectation, corrupting the D9-B3 rule.
**Fix:** Correct the expectation to the last three pairs:
```go
Expected: []schema.Message{
    {Role: schema.User, Content: "old two"},
    {Role: schema.Assistant, Content: "old answer two"},
    {Role: schema.User, Content: "three"},
    {Role: schema.Assistant, Content: "three answer"},
    {Role: schema.User, Content: "four"},
    {Role: schema.Assistant, Content: "four answer"},
},
```

### WR-05: Dead `":refusal_accuracy"` suffix checks mislabel per-category status and misroute the escape recommendation

**File:** `cmd/spike-history/summary.go:606-615` (`renderSummary`), `cmd/spike-history/summary.go:523-526` (`suggestEscape`)
**Issue:** `populateCategoryWorst` (summary.go:135-154) sets `WorstMetric` to bare names (`"refusal_accuracy"`, no `category:` prefix), and nothing in the package ever produces a `"<cat>:refusal_accuracy"` form. Therefore `strings.HasSuffix(c.WorstMetric, ":refusal_accuracy")` is always false: (a) in the per-category status table, a category whose worst metric is `refusal_accuracy` with |delta| in (0.05, 0.10] — within the 10pp tolerance, a PASS — is rendered **FAIL** because the 5% branch applies; (b) in `suggestEscape`, the refusal-specific recommendation is unreachable and refusal-driven NO-GOs get the generic message.
**Fix:** Replace the suffix test with equality against the bare name:
```go
if c.WorstMetric == "refusal_accuracy" {
    status = map[bool]string{true: "FAIL", false: "PASS"}[math.Abs(c.WorstDelta) > thresholdRefusalDeltaPP]
} else {
    status = map[bool]string{true: "FAIL", false: "PASS"}[math.Abs(c.WorstDelta) > thresholdDriftPct]
}
```
and in `suggestEscape`, `case "refusal_accuracy":` alongside the other metric cases.

### WR-06: Dry-run passes a nil embedder to `AnswerRelevancy`, which errors by contract — `answer_relevancy` is always 0 in the smoke path

**File:** `cmd/spike-history/runner.go:375` (with `cmd/spike-history/main.go:174-181`)
**Issue:** In dry-run mode `deps.Embedder` is nil, but Step 6 still calls `eval.AnswerRelevancy(ctx, row.Query, answer, deps.Embedder, judge)`. `internal/eval/metrics.go:59-61` returns `error("embedder is nil")` before touching the StubJudge, so every dry-run row logs a WARN and records `answer_relevancy = 0`. This contradicts the stated intent that the StubJudge path yields "predictable 1.0 / 0.0 numbers", and a developer smoke-testing `--dry-run` sees a metric stuck at 0 with per-row warning spam.
**Fix:** In dry-run, either skip the metric (`result.Metrics["answer_relevancy"] = 1` with a comment) or pass a deterministic stub embedder (hash-based vector) instead of nil.

### WR-07: Append-only JSONL + summarize with no run scoping — a rerun silently contaminates the verdict math

**File:** `cmd/spike-history/runner.go:506-524` (`writeResult`, `O_APPEND`), `cmd/spike-history/summary.go:182-210, 298-330`
**Issue:** `writeResult` appends by design ("existing JSONL preserved"), and `loadSpikeResults`/`computeCategoryMean` read and average **all** rows in each file with no dedup by `case_id` and no run-window filter. Running the spike a second time (e.g. the Plan 09-06 reproducibility rerun, or after a cost-stop) then summarizing averages stale and fresh rows together — means, refusal rate and penetration counts are computed over duplicated mixed data, and the GO/NO-GO verdict is silently corrupted. The reproducibility check itself ("same seed → byte-equal output") also cannot be evaluated against an appended file.
**Fix:** Dedup on load (keep the last row per `case_id`), or key the JSONL by run (e.g. `<category>-<runstamp>.jsonl`), or add `--fresh` that truncates the target files at run start.

### WR-08: Unused `buildStore` call can fatally abort a live run and leaks a side-effect DB handle

**File:** `cmd/spike-history/main.go:199-203`
**Issue:** `_ = buildStore(ctx)` opens (and health-checks) a SQLite store the spike never uses. `buildStore` calls `log.Fatalf` on DSN/health failure, so an unrelated `skillforge.db` lock/permission problem kills the whole live spike before any row runs. It also creates/opens `skillforge.db` as a filesystem side effect of a read-only evaluation run, and the returned store is never `Close()`d.
**Fix:** Delete the call (keep the helper behind a comment for Phase 10, or gate it behind a flag). If kept, close it: `if s := buildStore(ctx); s != nil { defer s.Close() }`.

### WR-09: `short_history.md` "Query" column prints the case ID; the "query recoverable from JSONL" claim is false

**File:** `cmd/spike-history/summary.go:683-686` (and `SpikeResult` in `cmd/spike-history/runner.go:148-166`)
**Issue:** `q := r.CaseID; ... r.CaseID, truncate(q, 40)` — the Query column duplicates the Case ID column for every row. `truncate`'s doc says "The full query lives in the JSONL stream and is recoverable via grep", but `SpikeResult` has no query field, so the drill-down's most useful debugging column is dead weight and the doc claim is wrong.
**Fix:** Add `Query string \`json:"query,omitempty"\`` to `SpikeResult`, populate it in `RunEntry` from `row.Query`, and render `truncate(r.Query, 40)`.

### WR-10: Unsanitized `Category` from golden data flows into the output filename (path traversal)

**File:** `cmd/spike-history/runner.go:510`
**Issue:** `filepath.Join(outDir, category+".jsonl")` where `category` originates from the golden JSONL's `category` field (or the `--category` flag). A crafted golden row with `"category": "../../evil"` writes outside `--out`. Low exposure (local operator-controlled CLI), but it is data-driven path construction with zero validation on a file whose format this phase just widened.
**Fix:** Validate at parse time in `loadSpikeGolden`: `if !categoryRE.MatchString(r.Category) { return error }` with `var categoryRE = regexp.MustCompile(`^[a-z0-9_-]+$`)`.

### WR-11: `fillerTurns` collision-retry loop is unbounded — hangs when `pairCount > len(baseline)`

**File:** `cmd/spike-history/long.go:291-299`
**Issue:** The `for { idx = rng.Intn(...); if !dup { break } }` loop can never terminate once every baseline index is in `chosen`. Today's numbers make this unreachable (max 6 pairs vs 75 loaded rows), but the function's own doc says "a future caller might" pass different counts, and an infinite loop is the worst failure mode for a CLI. `loadLongHistoryRows` guards `len(baseline) == 0` but not `pairCount > len(baseline)`.
**Fix:** At the top of `fillerTurns`: `if pairCount > len(baseline) { pairCount = len(baseline) }` (allow replacement) or return an error / switch to sampling with replacement when the pool is exhausted.

## Info

### IN-01: Comment misstates Go semantics — defers do not run across `os.Exit`

**File:** `cmd/spike-history/main.go:154-159`
**Issue:** The comment claims "defer fires across os.Exit at the top of a function". Defers never run on `os.Exit`. Harmless today only because the `--summarize` branch exits before `ctx, cancel` is created — but the comment will mislead the first person who moves that branch below the context setup.
**Fix:** Replace with: "exits before ctx/cancel are created, so there is no deferred cleanup to run".

### IN-02: Deprecated, unused global `rand.Seed`

**File:** `cmd/spike-history/main.go:133-136`
**Issue:** `rand.Seed` is deprecated since Go 1.20 and seeds the *global* source, which nothing in the package consumes — `loadLongHistoryRows` uses its own `rand.New(rand.NewSource(seed))`. Dead code that implies the flag does more than it does.
**Fix:** Delete the call (the flag is already threaded explicitly into `loadLongHistoryRows`).

### IN-03: Dead code inventory

**File:** `cmd/spike-history/spike_row.go:190-204`, `cmd/spike-history/spike_row.go:86`, `cmd/spike-history/main.go:104, 269, 351-363`, `cmd/spike-history/runner.go:526-528`
**Issue:** `spikeRowsToBaselineIDs` is never called; the `SpikeRows` type alias is never used; `defaultTopK` is defined but `RunEntry` passes the magic literal `5` to `Search` (runner.go:262) instead; `_ = time.Now()` is unnecessary (`time` is used by `buildEmbedder`); `buildAgent` is never called in this binary (documented as intentional); `var _ = store.SessionMessage{}` is an import-keeper for an otherwise-unused import in runner.go.
**Fix:** Delete the unused symbols; use `defaultTopK` at the `Search` call site so the value has one home.

### IN-04: `loadLongHistoryRows` doc claims "baseline 50 rows" but loads all 75 (including spike queries) as the filler pool

**File:** `cmd/spike-history/long.go:125-131, 180-187`
**Issue:** `eval.LoadGolden` has no category filter; the filler pool includes the 25 spike rows. Adversarial spike queries are byte-equal to baseline queries (benign), but followup queries ("那第二条呢？…") can become filler "user" turns — semantically odd context and a doc/behavior mismatch. No ground-truth leakage occurs (ruling 1 holds: only `Query` is used, never `GroundTruth`).
**Fix:** Filter the filler pool to non-spike categories, or fix the comment.

### IN-05: Unused struct field and doc/code mismatch in renderers

**File:** `cmd/spike-history/summary.go:557-566, 748-755`
**Issue:** The `rows` struct in `renderSummary` declares `eps float64` (set to `1e-9`, never read). `boolGlyph`'s doc says "checkmark/cross glyph" but it returns the strings "PASS"/"FAIL" — which is also semantically odd for the `Refused` column, where a refusal is not universally a pass or fail.
**Fix:** Drop `eps`; rename the helper to `passFail` and adjust the doc (or render ✓/✗ as originally intended).

### IN-06: `truncate` slices bytes, splitting UTF-8 CJK queries mid-rune

**File:** `cmd/spike-history/summary.go:739-746`
**Issue:** `s[:max]` on a Chinese query (3 bytes/char) at max=40 produces invalid UTF-8 in the markdown table (mojibake in reports).
**Fix:** Iterate runes: convert to `[]rune` and slice that, or use a rune-counting loop.

### IN-07: `TestRunSummarizeHermetic` does not invoke `runSummarize` despite its name and doc

**File:** `cmd/spike-history/summary_test.go:320-324`
**Issue:** The doc says it "invokes runSummarize with stub baseline numbers", but the test re-implements runSummarize's body (load → compute → render → write) because the baseline-load path reads `eval/report.json` from disk. The real `runSummarize` — including its exit-code contract (0/1/2), its `loadBaseline` error paths, and the populateCategoryWorst wiring — is untested; a regression in `runSummarize` itself (e.g. the WR-05 status bug or a wrong file path) would not be caught.
**Fix:** Refactor `runSummarize` to take the baseline report/lock paths (or a `baselineMetrics`) as parameters and have the test drive the real function with stub paths.

### IN-08: Dead initialization in golden test

**File:** `internal/eval/golden_test.go:74, 92`
**Issue:** `spikeAnchors := 0` is immediately overwritten by `spikeAnchors = countSpikeAnchors(t, path)`; the comment block between them also ambiguously suggests detection happens in the loop above (it does not).
**Fix:** Declare at the assignment: `spikeAnchors := countSpikeAnchors(t, path)`.

### IN-09: Spike chunk framing drops the `source=` field versus the T-2-21 byte-identical production envelope

**File:** `cmd/spike-history/runner.go:444-446`
**Issue:** Production framing is `[CHUNK %d] tier=%s source=%s` (`internal/agent/retrieve_tool.go:211`, locked by test); the spike emits `[CHUNK %d] tier=%s` only. Secondary to CR-02 (fix both together) — but if the spike is meant to mirror the production prompt for drift comparability, the framing should be byte-identical.
**Fix:** Include `source=%s` in the fixed `Fprintf` (see CR-02).

### IN-10: `Project` scrubs refs before truncation — surviving turns can carry non-contiguous `[REF-N]` numbering

**File:** `internal/history/projection.go:44-51`
**Issue:** `scrubChunkRefs` numbers `[REF-N]` across the full filtered history, then B3 truncates to the last 6 messages. A long history whose early (truncated) user turns contained chunk refs yields a prompt whose first visible ref is e.g. `[REF-5]` with no `[REF-1..4]` in context. No fixture pins this behavior either way, so Phase 10 will inherit it silently.
**Fix:** Decide and pin it: either renumber after truncation (move the scrub loop below the truncation), or add a fixture documenting the non-contiguous numbering as intentional.

---

## Scope & Constraint Verification (informational)

- In-scope files only were reviewed; no source files were modified.
- Verified against the hard constraints stated for this phase: none of the reviewed files touch `cmd/eval`, `cmd/server`, `internal/server`, or `eval/frozen`; no `go.mod`/`go.sum` changes are implied by the imports (all dependencies already in-tree).
- Cross-checked facts used in findings: `retriever.Hit.ID` is `int64` (`internal/retriever/retriever.go:45-53`); accumulator keys chunks by decimal `h.ID` (`internal/agent/trust_floor.go:150-153`); `GroundingCheck` looks citations up by canonical bare ID (`internal/verdict/grounding.go:89-110, 146-172`); `eval.AnswerRelevancy` rejects nil embedder (`internal/eval/metrics.go:59-61`); `eval.NewGLMChatModel` reads `JUDGE_MODEL` (`internal/eval/chatmodel.go:96-99`); cmd/eval computes metrics for refused cases with sentinel conventions (`cmd/eval/main.go:424-469`); golden spike categories and fake refs (`CH-901`..`CH-910` embedded as `[CHUNK CH-901]`, matched by the scrub regex) per `eval/golden/compliance.jsonl` lines 51-75.

_Reviewed: 2026-08-18T10:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
