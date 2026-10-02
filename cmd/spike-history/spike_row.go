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

// spike_row.go is the spike-only JSONL parser for the history-injection
// golden set (Plan 09-03 / Phase 9 GEVL-08). It embeds the baseline
// eval.Case struct verbatim and adds four spike-only fields:
//
//   - BaselineID     — the spike row's mirror baseline case ID.
//   - Subclass       — fine-grained label (anaphora / post_refusal /
//                      multi_hop_chain) for follow-up spike rows.
//   - HistoryAnchor  — the diagnostic field that names the fake chunk
//                      ref the spike is testing and the turn_index it
//                      anchors to. Non-nil structure for all spike rows.
//                      zero values mean "no anchor" (baseline rows).
//   - HistoryTurns   — the A/B history payload (user + assistant
//                      SessionMessage pairs). nil = no history.
//
// Spike rows share the JSONL file with the baseline 50 cases; the
// json.Unmarshal call into the embedded eval.Case silently ignores
// extra fields and the additional spike fields remain zero/nil for
// baseline rows. The two regimes therefore coexist in one file (Plan
// 09-01 locked the contract — see internal/eval/golden_test.go
// TestGoldenHistoryInjectionRows).
//
// This file ONLY parses. It does not score, does not compute metrics,
// does not call any chat model. Evaluation lives in runner.go (Task 2).
// This split keeps the parsing layer hermetic and easy to unit-test.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/store"
)

// spikeRow is one JSONL row from the golden set, extended with the four
// spike-only fields Plan 09-01 appended. The Category set spans:
//
//   - "adversarial"                 (baseline; 20 rows)
//   - "refusal-required"            (baseline; 20 rows)
//   - "multi-hop"                   (baseline; 10 rows)
//   - "history_injection_adversarial" (spike; 10 rows — has anchor)
//   - "history_injection_followup"    (spike; 15 rows — anchor may be zero)
//
// The embedded eval.Case carries ID / Category / Query / GroundTruth /
// ExpectedChunks; the spike fields layer on top.
type spikeRow struct {
	eval.Case
	BaselineID   string `json:"baseline_id,omitempty"`
	Subclass     string `json:"subclass,omitempty"`
	HistoryAnchor struct {
		TurnIndex    int    `json:"turn_index"`
		FakeChunkRef string `json:"fake_chunk_ref"`
	} `json:"history_anchor"`
	HistoryTurns []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"history_turns"`
}

// SpikeRow is the exported alias. Lets runner.go and main.go consume the
// parser output without re-importing the lowercase name.
type SpikeRow = spikeRow

// SpikeRows is a typed slice alias for fluency at call sites.
type SpikeRows []SpikeRow

// loadSpikeGolden reads a golden-set JSONL file and returns all rows
// (baseline + spike) as SpikeRow values. Mirrors the parsing discipline
// of internal/eval/dataset.go:LoadGolden:
//
//   - Skip blank lines and lines beginning with '#' (comment surface).
//   - Trim whitespace, reject empty IDs / Categories / Queries.
//   - Bump the per-line token budget to 1 MiB so a long history_turns
//     line (multi-paragraph adversarial text) round-trips without
//     bufio.ErrTooLong.
//
// Spike rows and baseline rows coexist in the same file; both regimes
// pass through the same json.Unmarshal call. Baseline rows leave
// BaselineID/Subclass/HistoryAnchor/HistoryTurns at their zero values
// (empty / zero / nil); spike rows populate at least HistoryTurns.
//
// Returns an error when:
//   - the file cannot be opened.
//   - any non-comment line fails to parse as JSON.
//   - a parsed row has empty ID / Category / Query.
//   - zero rows load (caller bug — wrong path?).
func loadSpikeGolden(path string) ([]SpikeRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("spike-history: loadSpikeGolden: open %q: %w", path, err)
	}
	defer f.Close()

	var rows []SpikeRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var r SpikeRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("spike-history: loadSpikeGolden: line %d: parse JSON: %w", lineNo, err)
		}
		if r.ID == "" {
			return nil, fmt.Errorf("spike-history: loadSpikeGolden: line %d: row missing id", lineNo)
		}
		if r.Category == "" {
			return nil, fmt.Errorf("spike-history: loadSpikeGolden: line %d: row %q missing category", lineNo, r.ID)
		}
		if strings.TrimSpace(r.Query) == "" {
			return nil, fmt.Errorf("spike-history: loadSpikeGolden: line %d: row %q missing query", lineNo, r.ID)
		}
		rows = append(rows, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("spike-history: loadSpikeGolden: scan: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("spike-history: loadSpikeGolden: %q loaded 0 rows (expected >=1)", path)
	}
	return rows, nil
}

// filterByCategory returns the subset of rows whose Category is in the
// categories slice. An empty categories slice is a no-op (returns the
// input unchanged) — main.go's --category default ("adversarial,followup")
// always passes a populated list, but unit-test paths may want to pass
// nil to exercise the unfiltered branch.
//
// Matching is case-sensitive: the golden file uses lowercase category
// literals and the --category flag echoes those bytes verbatim. A
// future "be liberal in what you accept" relaxation would belong in
// main.go's flag parser, not here.
func filterByCategory(rows []SpikeRow, categories []string) []SpikeRow {
	if len(categories) == 0 {
		return rows
	}
	keep := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		if c == "" {
			continue
		}
		keep[c] = struct{}{}
	}
	if len(keep) == 0 {
		return rows
	}
	out := make([]SpikeRow, 0, len(rows))
	for _, r := range rows {
		if _, ok := keep[r.Category]; ok {
			out = append(out, r)
		}
	}
	return out
}

// spikeRowsToBaselineIDs returns the unique BaselineID values present
// in the input. Useful for diagnostics ("which baseline cases did our
// spike rows mirror?") and for cross-referencing spike results back to
// baseline numbers. Order is preserved: the first occurrence of each
// distinct BaselineID wins.
//
// Rows without a BaselineID (the baseline 50 rows themselves) are
// skipped. Returns an empty slice when no spike rows are present.
func spikeRowsToBaselineIDs(rows []SpikeRow) []string {
	seen := make(map[string]struct{}, len(rows))
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.BaselineID == "" {
			continue
		}
		if _, dup := seen[r.BaselineID]; dup {
			continue
		}
		seen[r.BaselineID] = struct{}{}
		out = append(out, r.BaselineID)
	}
	return out
}

// spikeRowHistoryTurnsToSessionMessages converts the JSON-friendly
// anonymous-struct HistoryTurns slice on a spike row into the
// []store.SessionMessage shape that history.Project (Plan 09-02) and
// store.GetSession consume. The translation is mechanical:
//
//	{Role, Content}    →   {Role, Content, Ts}
//
// Ts is set to time.Unix(0, 0) — the epoch second. history.Project
// drops Ts during projection (it only reads Role + Content), so the
// zero Ts is a sentinel "I don't care" rather than a real timestamp.
// Callers that DO care about Ts (Phase 10 replay harnesses) should
// set it themselves before calling history.Project.
//
// Returns nil when the row has no HistoryTurns — the caller then
// skips history projection entirely (runner's withHistory=true branch
// short-circuits on len(projected) == 0).
func spikeRowHistoryTurnsToSessionMessages(row SpikeRow) []store.SessionMessage {
	if len(row.HistoryTurns) == 0 {
		return nil
	}
	zero := time.Unix(0, 0).UTC()
	out := make([]store.SessionMessage, 0, len(row.HistoryTurns))
	for _, h := range row.HistoryTurns {
		out = append(out, store.SessionMessage{
			Role:    h.Role,
			Content: h.Content,
			Ts:      zero,
		})
	}
	return out
}