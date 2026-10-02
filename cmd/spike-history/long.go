// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// long.go is the long-history synthesizer for cmd/spike-history (Plan 09-04
// of Phase 9 GEVL-08). It extends the short-history path (Plan 09-03) by
// producing 10 SpikeRow values whose HistoryTurns has 8, 12, or 15 entries
// (the planner ruling: rotate {8, 12, 15} cyclically across the 10 rows).
//
// Why a separate file:
//
//   - Plan 09-03 deliberately stubbed --history-length=long (grep-friendly
//     error string). This file owns the long-history implementation, and
//     main.go + runner.go swap their stubs for real dispatch.
//   - The D9-D2 ruling from the CONTEXT.md asks "what happens to the
//     trust floor under a long history?" — the answer is observable
//     here, not in the short-history runner. Spike JSONL for Plan 09-05
//     is therefore partitioned into per-category files (long.jsonl
//     lives alongside adversarial.jsonl / followup.jsonl).
//
// Three planner rulings, documented at length because the CONTEXT.md
// wording admits two readings; we picked one and bound the choice to
// godoc so future plans can grep for the rationale:
//
//	裁决 1 (filler 不泄题): filler 的 assistant 文本是固定模板
//	  "已记录您的问题 N（仅上下文，不进入证据）。"，绝不引用
//	  baseline.GroundTruth / ExpectedChunks。否则长历史会让 agent
//	  提前"看到答案"，spike 结果无法归因于历史注入漂移。
//
//	裁决 2 (signal 1 per row): CONTEXT §D9-D2 写"其中 3 轮含真实
//	  adversarial/followup 用例作为信号"，但 row-per-signal 是
//	  Plan 09-03 RunEntry 的不可变契约（一次 row 一次 LLM call）。
//	  本 plan 改为每 row 嵌 1 个 signal，10 rows 总信号覆盖率 100%
//	  （远超 30% 阈值）。3 signal/row 反而需要 row 内嵌 3 query，
//	  违背现有调用模型，故舍去。
//
//	裁决 3 (B3 硬限在 long 模式下自动截断): history.Project 的 B3
//	  规则（plan 09-02 写死末尾 6 条硬限）在 long 模式下仍生效，
//	  即 8/12/15 条 history 经 Project 截到 6 条。spike 报告
//	  （Plan 09-05）观察此截断是否仍漂移——若仍漂移则 Phase 10
//	  必须先做 MEM-05 summary 中间件；若不漂移则 B3 即可。
//
//	  Phase 10 处置（事实陈述，HYG-01 / Plan 26-10）：长会话的逃逸
//	  方案最终是 D10-B3 字符预算窗口——internal/history/projection.go
//	  的 TruncateToCharBudget 是 PRIMARY 且唯一的窗口（legacy
//	  6-message pre-cap 已移除）。上面"替换为 summarization 中间件"
//	  的分支没有发生：MEM-05 摘要中间件未实现，全仓 summariz* 命中
//	  均为本 spike 自身的 --summarize 报表生成（summary.go），与摘要
//	  压缩无关。spike 观察结论见 .planning/phases/09-* 与
//	  .planning/STATE.md 的 Phase 10 记录（D9-V2）。
//
// Hard guarantee: this file does NOT introduce a new dependency, does
// NOT modify spike_row.go / RunEntry / writeResult, and does NOT write
// to JSONL itself. It only returns []SpikeRow; main.go's main loop
// routes each row through writeResult using the synthesized Category
// "history_injection_long" so the JSONL lands in long.jsonl (planner
// ruling 5: 文件名互不重叠, 报告层按 Category 字段聚合).

package main

import (
	"fmt"
	"math/rand"

	"github.com/skillforge/skillforge/internal/eval"
)

// longCount is the number of synthetic long-history SpikeRows to
// produce. CONTEXT §D9-D2 sets the floor at "≈10 rows"; this is the
// round figure.
const longCount = 10

// fillerAssistantTemplate is the assistant-side filler text. Per
// planner ruling 1, it MUST NOT carry any answerable content: it
// acknowledges the user's prior question without quoting baseline
// ground_truth or expected_chunks. The "%d" argument is the filler
// pair index (1-based) so two pairs in the same row look distinct.
const fillerAssistantTemplate = "已记录您的问题 %d（仅上下文，不进入证据）。"

// longSynthCategory is the Category value written into each
// synthesized SpikeRow. Plan 09-03 main.go's writeResult uses the
// row.Category as the JSONL filename suffix; using a distinct name
// from history_injection_adversarial / history_injection_followup
// means long.jsonl never collides with adversarial.jsonl or
// followup.jsonl. Plan 09-05's report layer aggregates by Category
// field, so renaming the file is a layout detail — the Category
// field is the source of truth.
const longSynthCategory = "history_injection_long"

// lengthTargets is the rotating target history length. The 10 rows
// are produced in index order; row i takes lengthTargets[i % 3]:
//
//	i:   0  1  2  3  4  5  6  7  8  9
//	len: 8 12 15  8 12 15  8 12 15  8
//
// Per planner ruling 3, history.Project's B3 6-tail hard cap means
// long histories still hit the same 6-turn projection as short ones.
// The value of lengthTargets > 6 is to observe what the model DOES
// NOT see (the truncated tail) and how that interacts with the signal.
var lengthTargets = []int{8, 12, 15}

// historyTurn is a type alias matching the anonymous struct shape used
// by SpikeRow.HistoryTurns in spike_row.go. Using a type alias (not a
// named type) preserves structural identity with the embedded slice;
// otherwise Go would refuse the assignment `row.HistoryTurns = ...`
// because the anonymous struct type and the named type would be
// nominally distinct even with identical fields + tags.
type historyTurn = struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// loadLongHistoryRows synthesizes long-history SpikeRows from the
// golden set. Pipeline:
//
//  1. Load all 75 rows via loadSpikeGolden (returns baseline 50 +
//     spike 25).
//  2. Filter to spike rows (history_injection_adversarial +
//     history_injection_followup). These are the "signal pool" —
//     the long rows embed one of these as their payload.
//  3. Load baseline 50 rows via eval.LoadGolden (independent parse
//     path; SpikeRow is a different type so we re-parse).
//  4. Build a seeded RNG (rand.New(rand.NewSource(seed))). seed=42
//     is the default; operators can pass --seed to reroll.
//  5. Fisher-Yates shuffle the signal pool and take the first
//     longCount=10 rows. If the pool is smaller, return an error
//     (a missing spike row is a golden-set bug).
//  6. For each of the 10 selected signals (i), build a synthetic
//     SpikeRow:
//     - target = lengthTargets[i % 3]
//     - fillerCount = max-even-difference between target and
//       signal.HistoryTurns length (see computeFillerCount below)
//     - Pick fillerCount/2 distinct baseline IDs (without
//       replacement) and produce (user, assistant) pairs from them.
//     - Insert the signal at a deterministic random position in
//       the combined slice so the signal does not always sit at
//       the head (which would make it easier to ignore — we want
//       the agent to "miss" it during projection truncation).
//     - Build the final HistoryTurns slice in deterministic order.
//     - Copy ID / Query / GroundTruth / ExpectedChunks / BaselineID
//       / Subclass / HistoryAnchor from the signal. Overwrite
//       Category with longSynthCategory so writeResult routes to
//       long.jsonl.
//
// Determinism guarantee: same seed + same golden file → byte-equal
// output. Plan 09-06 live UAT's reproducibility check depends on this.
//
// Returns an error if:
//   - the golden file fails to load.
//   - the spike signal pool has fewer than longCount rows.
//   - the baseline filler pool is too small to satisfy a pick
//     request (caller bug — should be unreachable for the 50-row
//     baseline at longCount=10).
func loadLongHistoryRows(goldenPath string, seed int64) ([]SpikeRow, error) {
	// ---- Step 1: load all 75 rows ----
	allRows, err := loadSpikeGolden(goldenPath)
	if err != nil {
		return nil, fmt.Errorf("spike-history: loadLongHistoryRows: %w", err)
	}

	// ---- Step 2: filter to spike signal pool ----
	signalPool := make([]SpikeRow, 0, len(allRows))
	for _, r := range allRows {
		if r.Category == "history_injection_adversarial" ||
			r.Category == "history_injection_followup" {
			signalPool = append(signalPool, r)
		}
	}
	if len(signalPool) < longCount {
		return nil, fmt.Errorf(
			"spike-history: loadLongHistoryRows: signal pool too small: "+
				"have %d, need %d (golden file missing history_injection rows?)",
			len(signalPool), longCount)
	}

	// ---- Step 3: load baseline 50 rows for filler sampling ----
	baseline, err := eval.LoadGolden(goldenPath)
	if err != nil {
		return nil, fmt.Errorf("spike-history: loadLongHistoryRows: load baseline: %w", err)
	}
	if len(baseline) == 0 {
		return nil, fmt.Errorf("spike-history: loadLongHistoryRows: baseline pool is empty")
	}

	// ---- Step 4: seeded RNG ----
	rng := rand.New(rand.NewSource(seed))

	// ---- Step 5: shuffle signal pool and take longCount ----
	selectedSignals := shuffleAndTake(signalPool, longCount, rng)

	// ---- Step 6: build synthetic rows ----
	out := make([]SpikeRow, 0, longCount)
	for i, sig := range selectedSignals {
		targetLen := lengthTargets[i%len(lengthTargets)]
		fillerCount := computeFillerCount(targetLen, len(sig.HistoryTurns))

		if fillerCount == 0 {
			// signal already >= target; emit the signal as-is (rare
			// but possible for adversarial rows with long text).
			row := cloneSpikeRow(sig)
			row.Category = longSynthCategory
			out = append(out, row)
			continue
		}

		// fillerCount is the number of filler turns to produce (always
		// even — computeFillerCount drops the odd one). fillerTurns
		// derives pairCount = fillerCount/2 internally so the caller
		// does not have to repeat the halving.
		filler := fillerTurns(baseline, fillerCount, rng, i+1)

		// Pick a random insert position (0..fillerCount inclusive)
		// so the signal lands at a non-deterministic place. With
		// B3's 6-tail hard cap, signals at the head or middle get
		// truncated; signals near the tail survive — that's the
		// variation Plan 09-05 wants to observe.
		insertPos := rng.Intn(fillerCount + 1)

		// Stitch: [filler[0..insertPos), signal.HistoryTurns..., filler[insertPos..]]
		combined := make([]historyTurn, 0, fillerCount+len(sig.HistoryTurns))
		combined = append(combined, filler[:insertPos]...)
		combined = append(combined, sig.HistoryTurns...)
		combined = append(combined, filler[insertPos:]...)

		// Copy signal fields verbatim, then override Category and
		// HistoryTurns. Other fields (ID, Query, GroundTruth,
		// ExpectedChunks, BaselineID, Subclass, HistoryAnchor) are
		// preserved by value.
		row := cloneSpikeRow(sig)
		row.Category = longSynthCategory
		row.HistoryTurns = combined
		out = append(out, row)
	}

	return out, nil
}

// computeFillerCount returns the number of filler turns to produce
// given the target total length and the signal's own turn count.
// The filler must come in (user, assistant) pairs, so the returned
// count is always even — odd differences drop one turn. This means
// a target=15 signal with 2 turns produces 12 filler (6 pairs), total
// 14, not 15; the planner's "8/12/15" is an upper bound, not a
// strict equality. Negative differences (signal already longer than
// target) return 0 so the caller emits the signal as-is.
//
// Pulled out as a free function so a unit test (Phase 10) can pin
// the math without re-deriving from loadLongHistoryRows.
func computeFillerCount(targetLen, signalLen int) int {
	if targetLen <= signalLen {
		return 0
	}
	diff := targetLen - signalLen
	if diff%2 == 1 {
		diff--
	}
	return diff
}

// fillerTurns returns `count` filler turns formed by interleaving
// `count/2` (user, assistant) pairs. The user half is the baseline
// row's Query (so the filler sounds like real compliance Q&A context
// without leaking GroundTruth); the assistant half is the fixed
// `fillerAssistantTemplate` indexed by `pairOrdinal` (a stable 1-based
// counter so the same filler sequence in two different long rows
// still differs by ordinal — important for token diversity).
//
// The `count` argument MUST be even; odd values are floored
// defensively (loadLongHistoryRows never passes odd counts, but a
// future caller might).
//
// Baseline rows are picked without replacement: once a baseline ID
// is selected, it cannot appear again in the same long row. Cross-row
// repetition is allowed (and expected — the baseline pool is 50 rows
// and we may need to pull many more pairs).
func fillerTurns(baseline []eval.Case, count int, rng *rand.Rand, pairOrdinal int) []historyTurn {
	if count <= 0 || len(baseline) == 0 {
		return nil
	}
	pairCount := count / 2
	chosen := make(map[int]struct{}, pairCount)
	out := make([]historyTurn, 0, count)

	for p := 0; p < pairCount; p++ {
		// Pick a unique baseline index. Retry on collision; with
		// 50 baselines and <=5 pairs per row, collisions are rare
		// enough that this terminates quickly.
		var idx int
		for {
			idx = rng.Intn(len(baseline))
			if _, dup := chosen[idx]; !dup {
				chosen[idx] = struct{}{}
				break
			}
		}
		// User turn: the baseline's Query verbatim. This is the
		// "context" half — a real compliance Q&A, not a leak.
		out = append(out, historyTurn{Role: "user", Content: baseline[idx].Query})
		// Assistant turn: fixed template indexed by pairOrdinal.
		// PLANNER RULING 1 — never quote baseline[idx].GroundTruth
		// or baseline[idx].ExpectedChunks here.
		out = append(out, historyTurn{Role: "assistant", Content: fmt.Sprintf(fillerAssistantTemplate, pairOrdinal)})
	}
	return out
}

// shuffleAndTake performs a Fisher-Yates shuffle on pool (in place,
// in `rng` order) and returns the first n elements. Returned slice
// is freshly allocated; the original pool is not mutated. Generic
// over T so a future caller can reuse the helper on a different
// element type without re-deriving.
//
// If n >= len(pool), returns the shuffled pool. If n == 0, returns
// nil. Negative n is treated as 0 (defensive — the caller in this
// file never passes negative values).
func shuffleAndTake[T any](pool []T, n int, rng *rand.Rand) []T {
	if len(pool) == 0 || n <= 0 {
		return nil
	}
	if n >= len(pool) {
		n = len(pool)
	}
	// Copy so we never mutate the caller's slice.
	cp := make([]T, len(pool))
	copy(cp, pool)
	rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	out := make([]T, n)
	copy(out, cp[:n])
	return out
}

// cloneSpikeRow returns a deep copy of r. SpikeRow embeds eval.Case
// (value type) and holds slices for HistoryTurns; the value-semantic
// copy preserves eval.Case and copies the HistoryTurns header but
// shares the underlying array. For loadLongHistoryRows this is
// fine — we always REPLACE HistoryTurns on the returned row before
// emitting, so the original signal row's slice is never read after
// the copy.
//
// Kept as a helper (not inlined) so a future Plan 10 unit test can
// assert deep-copy semantics if needed.
func cloneSpikeRow(r SpikeRow) SpikeRow {
	out := r
	if r.HistoryTurns != nil {
		out.HistoryTurns = make([]historyTurn, len(r.HistoryTurns))
		copy(out.HistoryTurns, r.HistoryTurns)
	}
	return out
}