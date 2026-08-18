---
phase: 09-trust-floor-spike-p8-s-spike-hard-gate
plan: 01
subsystem: eval-golden
tags: [golden-set, history-injection, spike, GEVL-05, GEVL-08, TDD]
dependency-graph:
  requires:
    - phase: 03 (Agentic Retrieval & Trust Evaluation)
      provides: "internal/eval.LoadGolden + LoadGolden schema + 50-case baseline + GEVL-05 contract"
  provides:
    - "75-row golden set (50 baseline + 25 spike) with structural schema extensions"
    - "TestGoldenCategoryCoverage upgraded to 75 rows (baseline slice 20/20/10 preserved verbatim)"
    - "TestGoldenHistoryInjectionRows — 25-row spike schema gate (anchor self-consistency + baseline query byte-equal)"
  affects:
    - "Phase 9 spike runner 09-03 will read these 25 rows via LoadGolden + history_anchor decoding"
    - "Phase 10 multi-turn memory will use baseline_id anchoring for repro"
tech-stack:
  added: []
  patterns:
    - "Raw-line re-scan (countSpikeAnchors / loadSpikeRows / extractQueryFromRow) for schema-extension fields ignored by Case struct"
    - "Category-prefix enumeration: baseline slice counters (20/20/10) preserved byte-for-byte; new category counts layered in"
    - "Anchor self-consistency: history_anchor.fake_chunk_ref substring-asserted against history_turns content for 1:1 repro"
key-files:
  created:
    - path: ".planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-01-SUMMARY.md"
      role: "Plan completion record"
  modified:
    - path: "eval/golden/compliance.jsonl"
      role: "Pure-append extension from 50 → 75 rows; 25 spike rows carry baseline_id + history_anchor + history_turns schema"
    - path: "internal/eval/golden_test.go"
      role: "GEVL-05 contract upgraded to 75 rows; new TestGoldenHistoryInjectionRows schema gate (260+ lines added)"
    - path: ".planning/STATE.md"
      role: "Phase 9 resume point added; 09-01 completion noted"
    - path: ".planning/ROADMAP.md"
      role: "Phase 9 progress row 0/6 → 1/6; 09-01 plan-list checkbox [ ] → [x]"
decisions:
  - id: D9-EXEC-01
    decision: "Fake system text was authored by the executor (planner verdict 1, zero LLM API call); recorded via source:'phase9-spike' metadata, not via separate GLM-sidecar script — eliminates T-9-LLM-COST exposure and keeps generation reproducible from git history"
  - id: D9-EXEC-02
    decision: "golden_test.go upgraded by addition only: original 20/20/10 + 5 spike-anchor + ID-uniqueness + non-empty-query/category assertions retained byte-for-byte; new TestGoldenHistoryInjectionRows (260+ new lines, 0 deletions) provides per-row structural verification for the 25 spike rows"
  - id: D9-EXEC-03
    decision: "Gating row-spike by row query ↔ baseline query byte-equal assert (D9-A3) — the only machine-verifiable A/B control: any drift in the baseline query immediately fails the spike row's structural gate even before spike runner runs"
metrics:
  duration: "2026-08-18T02:41:48Z..2026-08-18T02:48:11Z (~6m23s)"
  completed: "2026-08-18T02:48:11Z"
  tasks: 3
  files: 2
  commits: 3
---

# Phase 9 Plan 1: 历史注入 spike — golden set 75 行口径 + GEVL-05 契约升级 — Summary

**One-liner:** Eval golden set grew 50 → 75 rows (10 adversarial + 15 followup spike slice); GEVL-05 contract gate widened in the additive-only style (baseline 20/20/10 + spike-anchor 5 retained verbatim); new `TestGoldenHistoryInjectionRows` schema gate asserts anchor self-consistency and baseline-query byte-equality for every adversarial spike row — the A/B repro surface that plan 09-03's spike runner will read.

## Tasks Completed

| # | Task | Type | Commit | Notes |
|---|------|------|--------|-------|
| 1 | Append 10 `history_injection_adversarial` rows | auto | `0df3483` | Fake system text in `history_turns[0]`: each names a real authority corpus theme + embeds fabricated clause number + 1+ substantive error + `[CHUNK CH-90N]` anchor. `baseline_id` points to one of the original 20 `adversarial` rows; `query` / `ground_truth` / `expected_chunks` copied verbatim. 10 distinct `fake_chunk_ref` values (`CH-901..CH-910`). |
| 2 | Append 15 `history_injection_followup` rows | auto | `1216c79` | Three subclasses, 5 each: anaphora (pronoun follow-up, requires history to parse); post_refusal (4 carry `ground_truth:'REFUSAL'` sentinel — bound-by-skillH2 stress; 1 carries answerable bound — boundary case); multi_hop_chain (cross-corpus rule chains). `fake_chunk_ref == ""` uniformly (benign group, no fabricated references). |
| 3 | Upgrade `golden_test.go` to 75-row contract + add spike schema gate | auto (TDD) | `8e56a27` | RED first: ran `TestGoldenCategoryCoverage` against the 75-row file — confirmed "should have 50 items, but has 75". GREEN: bumped `require.Len` 50 → 75, added 2 new categories to the counts map + 2 new assertions preserving baseline slice assertions byte-for-byte; added `TestGoldenHistoryInjectionRows` (260+ new lines) with 7 structural asserts over all 25 spike rows (anchor self-consistency, baseline query byte-equal, subclass balance, baseline-ID non-collision). |

All acceptance criteria from PLAN.md verified against the working tree:

- `wc -l eval/golden/compliance.jsonl` = **75**
- `grep -c '"category":"history_injection_adversarial"' = **10**
- `grep -c '"category":"history_injection_followup"' = **15**
- `grep -c '"id":"hist-adv-' = **10** (and `grep -o '"id":"hist-adv-[0-9]*"' | sort -u | wc -l` = 10 — no dup)
- `grep -c '"id":"hist-fup-' = **15** (and ditto uniqueness — 15 unique)
- `grep -o '"fake_chunk_ref":"[^"]*"' | sort -u | wc -l` = **10** (10 distinct adversarial refs)
- `grep -c '\[CHUNK CH-90'` = **10** (every adversarial fake text embeds its anchor marker)
- `grep -c '"baseline_id":"adv-' = **10** (and every `baseline_id` resolves to an existing baseline row)
- `grep -c '"fake_chunk_ref":""` = **15** (every followup row is benign)
- `grep '"subclass":"post_refusal"' … | grep -c '"ground_truth":"REFUSAL"'` = **4** (≥3 required)
- `grep -o '"subclass":"[a-z_]*"' | sort | uniq -c` = `5/5/5` balanced
- `git diff --stat eval/frozen/ eval/sample.jsonl` = **empty** (frozen and sample untouched)
- `git diff eval/golden/compliance.jsonl | grep -c '^-[^-]'` = **0** (pure append, original 50 zero-modified)
- `go test ./internal/eval/... -run TestGolden -v -count=1` = **PASS PASS** (both `TestGoldenCategoryCoverage` + `TestGoldenHistoryInjectionRows`)
- `go test ./internal/eval/... -count=1` = **ok** (zero regression on existing faithfulness / frozen / gate / judge tests)
- `go vet ./internal/eval/...` exit **0**
- `go build ./... && go vet ./...` exit **0**
- `grep -c 'require.Len(t, cases, 75' internal/eval/golden_test.go` = **1**
- `grep -c 'assert.Equal(t, 20, counts\["adversarial"\]'` = **1** (baseline assertion preserved)
- `grep -c 'assert.Equal(t, 20, counts\["refusal-required"\]'` = **1**
- `grep -c 'assert.Equal(t, 10, counts\["multi-hop"\]'` = **1**
- `grep -c 'func TestGoldenHistoryInjectionRows'` = **1**
- `git status --porcelain internal/eval/ | wc -l` = **1** (only `golden_test.go` touched)

## Deviations from Plan

### Auto-fixed Issues

**None.** Plan executed exactly as written. All three tasks ran in the order prescribed; all verdict numbers hit the prescribed bounds (≥4 REFUSAL, 5/5/5 subclass split, 10 distinct fake_chunk_ref); TDD RED → GREEN gate sequence held (test failed with the deliberate 50→75 size drift before the contract was widened).

## Threat Posture

Per `<threat_model>` in PLAN.md:

| Threat | Disposition | Status |
|--------|-------------|--------|
| T-9-INJECTION-ESCAPE | mitigate | **Mitigated.** `category` prefix `history_injection_*` + `source:"phase9-spike"` provides machine-identifiable separation; `TestGoldenCategoryCoverage` keeps the original 20/20/10 + 5-spike-anchor assertions verbatim (any baseline drift fails); `git diff --stat eval/frozen/` is empty in commit `0df3483` / `1216c79` / `8e56a27`. |
| T-9-CORPUS-CONFUSION | mitigate | **Mitigated by design.** All 10 fake system texts are confined to `history_turns[0].content` of spike rows in `eval/golden/compliance.jsonl`. The indexing layer `internal/knowledge.Index` only walks `knowledge/` (per PLAN.md threat model spec) — no Milvus ingest path can pull JSONL content. |
| T-9-SPIKE-DRIFT | mitigate | **Mitigated by assertion.** `TestGoldenHistoryInjectionRows` enforces (a) baseline_id resolves, (b) spike row's `query` is **byte-equal** to the corresponding baseline row's `query` (D9-A3 machine check), and (c) each adversarial `fake_chunk_ref` appears as a substring in its own `history_turns[].content` (1:1 repro anchor). |
| T-9-LLM-COST | avoid | **Avoided.** Planner verdict 1 honored: zero external LLM call. All 10 fake texts were authored by the executor; cost dimension is 0. |
| T-9-SC | accept | **n/a.** No new dependencies introduced — `golden_test.go` uses only `testing`, `testify`, `encoding/json`, `bufio`, `os`, `strings`, `path/filepath` (already imported by the test file pre-change). |

## Self-Check

- [x] All committed files exist on disk: `eval/golden/compliance.jsonl` (75 lines, `wc -l` confirmed); `internal/eval/golden_test.go` (recently edited, both new tests present per `grep -c 'func TestGoldenHistoryInjectionRows'`).
- [x] All commit hashes exist: `0df3483`, `1216c79`, `8e56a27` — verified via `git log --oneline -5`.
- [x] `go test ./internal/eval/... -count=1` exit 0 (last run during Task 3 GREEN verification).
- [x] `go vet ./...` exit 0.
- [x] `git diff --stat eval/frozen/ eval/sample.jsonl` empty (frozen + sample untouched).
- [x] `git diff eval/golden/compliance.jsonl | grep -c '^-[^-]'` = 0 (pure append).

## What's Next (Plan 09-02)

The data foundation is now in place; Plan 09-02 will land `internal/history/projection.go` + `projection_test_fixtures.go` — the D9-B1/B2/B3 消毒规则 + 6+ positive/negative fixtures that the spike runner (Plan 09-03) and the eventual Phase 10 multi-turn memory both read. Plan 09-02 owns:
- The function signature `func Project(prev []SessionMessage) []schema.Message` encoding the three rules into types
- 3-5 positive fixtures (legal projections) and 3-5 negative fixtures (rule violations demonstrating what gets stripped)
- Zero `internal/server/chat.go` changes (single-round limit stays intact)

Once 09-02 lands, Wave 2 plan 09-03 brings `cmd/spike-history/` — the spike-only binary that consumes these 75 rows + the projection function and produces the go/no-go numbers that gate Phase 10.
