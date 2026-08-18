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

package eval

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoldenCategoryCoverage is the GEVL-05 regression gate. It loads
// the production golden set (eval/golden/compliance.jsonl) and asserts:
//
//   - Exactly 75 cases total (50 baseline + 25 phase-9 history-injection spike).
//   - Exactly 20 adversarial + 20 refusal-required + 10 multi-hop (baseline slice).
//   - Exactly 10 history_injection_adversarial + 15 history_injection_followup (spike slice).
//   - The 5 spike-anchor cases are present (reused from
//     scripts/spiked/e2e.go:76-82 for cross-phase comparability).
//   - Every case has a non-empty query + category.
//
// Phase 9 extension (2026-08-18, GEVL-08 hard gate): the contract was widened
// from 50 to 75 rows. Baseline 20/20/10 + spike-anchor 5 assertions are
// preserved verbatim — any drift in the baseline slice still fails the gate.
// The 25 spike rows carry `source:"phase9-spike"` and additional schema
// fields (`history_anchor`, `history_turns`, optional `subclass`, optional
// `baseline_id`) which the Case struct intentionally ignores; the
// `TestGoldenHistoryInjectionRows` schema gate (below) re-decodes raw lines
// to verify the spike extension fields exist and are self-consistent.
//
// Pinned by 03-VALIDATION.md. If this test fails, the golden set has
// drifted from the GEVL-05 contract — fix the JSONL before re-running
// the eval CLI.
//
// NOTE: the test uses a relative path (../../eval/golden/compliance.jsonl)
// because internal/eval's test working directory is internal/eval/. The
// path is stable under `go test` (which does not chdir).
//
// eval/sample.jsonl stays at 50 cases (DIST-10 published sample;
// not synchronized with the spike). eval/frozen/compliance.jsonl is
// untouched (Phase 8 v0.9.14 baseline pin; CI gate reads this path only).
func TestGoldenCategoryCoverage(t *testing.T) {
	path := filepath.Join("..", "..", "eval", "golden", "compliance.jsonl")
	cases, err := LoadGolden(path)
	require.NoError(t, err, "load production golden set")
	require.Len(t, cases, 75, "GEVL-05 + Phase 9 spike contract: 50 baseline + 25 history-injection")

	counts := map[string]int{
		"adversarial":                 0,
		"refusal-required":            0,
		"multi-hop":                   0,
		"history_injection_adversarial": 0,
		"history_injection_followup":    0,
	}
	spikeAnchors := 0
	ids := make(map[string]struct{}, len(cases))
	for _, c := range cases {
		assert.NotEmpty(t, c.Query, "case %s has empty query", c.ID)
		assert.NotEmpty(t, c.Category, "case %s has empty category", c.ID)
		assert.Contains(t, counts, c.Category, "case %s has unknown category %q", c.ID, c.Category)
		counts[c.Category]++
		_, dup := ids[c.ID]
		assert.False(t, dup, "duplicate case id %q", c.ID)
		ids[c.ID] = struct{}{}
		// The "source" field is optional metadata; LoadGolden does not
		// surface it on Case (the field is not in the struct). We
		// detect spike-anchors by re-decoding the raw line here.
	}

	// Re-scan the raw file to count spike-anchor tags (the Case struct
	// does not carry the "source" field — it's metadata, not a metric
	// input).
	spikeAnchors = countSpikeAnchors(t, path)
	assert.Equal(t, 5, spikeAnchors, "expected 5 spike-anchor cases (scripts/spiked/e2e.go:76-82)")

	// Baseline slice — preserved verbatim from Phase 3 GEVL-05 contract.
	assert.Equal(t, 20, counts["adversarial"],
		"GEVL-05: exactly 20 adversarial cases (got %d)", counts["adversarial"])
	assert.Equal(t, 20, counts["refusal-required"],
		"GEVL-05: exactly 20 refusal-required cases (got %d)", counts["refusal-required"])
	assert.Equal(t, 10, counts["multi-hop"],
		"GEVL-05: exactly 10 multi-hop cases (got %d)", counts["multi-hop"])

	// Phase 9 spike slice (GEVL-08).
	assert.Equal(t, 10, counts["history_injection_adversarial"],
		"GEVL-08 spike: exactly 10 history_injection_adversarial (got %d)", counts["history_injection_adversarial"])
	assert.Equal(t, 15, counts["history_injection_followup"],
		"GEVL-08 spike: exactly 15 history_injection_followup (got %d)", counts["history_injection_followup"])
}

// countSpikeAnchors re-reads the golden file and counts lines tagged
// "source":"spike-anchor". Used because the Case struct intentionally
// does not surface the "source" metadata field (it is not a metric
// input). The count is asserted by TestGoldenCategoryCoverage.
func countSpikeAnchors(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var probe struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err == nil && probe.Source == "spike-anchor" {
			n++
		}
	}
	require.NoError(t, scanner.Err())
	return n
}

// spikeRow is the local probe struct used by TestGoldenHistoryInjectionRows.
// It re-decodes raw JSONL lines so it can inspect the spike-extension
// fields (`baseline_id`, `history_anchor`, `history_turns`, `subclass`),
// which the Case struct (internal/eval/dataset.go:49) intentionally does
// not surface — those fields are diagnostic metadata for phase-9 reports,
// not metric inputs.
type spikeRow struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Subclass    string `json:"subclass"`
	BaselineID  string `json:"baseline_id"`
	HistoryAnchor struct {
		TurnIndex    int    `json:"turn_index"`
		FakeChunkRef string `json:"fake_chunk_ref"`
	} `json:"history_anchor"`
	HistoryTurns []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"history_turns"`
}

// TestGoldenHistoryInjectionRows is the GEVL-08 spike row schema gate.
// It re-decodes the 25 phase-9 spike rows (history_injection_adversarial +
// history_injection_followup) and verifies:
//
//   - Category split: 10 adversarial + 15 followup (not re-asserting
//     counts here — TestGoldenCategoryCoverage owns that).
//   - Both categories' rows carry history_turns[] with length >= 2;
//     every turn has role ∈ {user, assistant} and non-empty content.
//   - history_anchor.turn_index >= 1 for both categories.
//   - Adversarial rows: fake_chunk_ref is non-empty, the 10 refs are
//     pairwise distinct, and each ref appears as a substring in some
//     history_turn[].content (the anchor is self-consistent — phase-9
//     reports rely on this for 1:1 reproduction).
//   - Adversarial rows: baseline_id is non-empty AND points to an existing
//     row in the same file AND that baseline row's query is byte-equal
//     to the spike row's query (D9-A3 A/B control).
//   - Followup rows: subclass ∈ {anaphora, post_refusal, multi_hop_chain},
//     5 rows per subclass, fake_chunk_ref == "" (benign group never
//     injects fabricated chunk references).
//   - IDs do not collide with the original 50 baseline IDs.
//
// This gate runs as the GREEN half of the phase-9 TDD pair:
// TestGoldenCategoryCoverage fails first (50→75 drift), then this test
// adds the per-spike-row structural assertions on top. Together they
// lock the spike row format as machine-verifiable.
func TestGoldenHistoryInjectionRows(t *testing.T) {
	path := filepath.Join("..", "..", "eval", "golden", "compliance.jsonl")

	rows := loadSpikeRows(t, path)
	require.NotEmpty(t, rows, "loadSpikeRows returned no rows")

	// Collect baseline queries — only the original 50 baseline rows, NOT
	// the 25 spike rows. We filter by category so spike IDs don't get
	// pulled into this map (they would self-collide below).
	baselineQueries := map[string]string{}
	cases, err := LoadGolden(path)
	require.NoError(t, err)
	for _, c := range cases {
		if c.Category == "history_injection_adversarial" ||
			c.Category == "history_injection_followup" {
			continue
		}
		baselineQueries[c.ID] = c.Query
	}

	// Bucket rows by spike category.
	var adv, fup []spikeRow
	for _, r := range rows {
		switch r.Category {
		case "history_injection_adversarial":
			adv = append(adv, r)
		case "history_injection_followup":
			fup = append(fup, r)
		}
	}
	require.Len(t, adv, 10, "expected 10 adversarial spike rows")
	require.Len(t, fup, 15, "expected 15 followup spike rows")

	// Baseline IDs (the original 50) must not collide with spike IDs.
	for _, r := range adv {
		_, isBaseline := baselineQueries[r.ID]
		assert.False(t, isBaseline, "adversarial spike ID %q collides with baseline ID", r.ID)
	}
	for _, r := range fup {
		_, isBaseline := baselineQueries[r.ID]
		assert.False(t, isBaseline, "followup spike ID %q collides with baseline ID", r.ID)
	}
	// All spike rows: history_turns + history_anchor shape checks.
	for _, r := range adv {
		assert.GreaterOrEqual(t, r.HistoryAnchor.TurnIndex, 1,
			"adversarial %s: turn_index must be >= 1", r.ID)
		assert.NotEmpty(t, r.HistoryAnchor.FakeChunkRef,
			"adversarial %s: fake_chunk_ref must be non-empty", r.ID)
		require.GreaterOrEqual(t, len(r.HistoryTurns), 2,
			"adversarial %s: history_turns must have >= 2 entries", r.ID)
		for i, turn := range r.HistoryTurns {
			assert.Contains(t, []string{"user", "assistant"}, turn.Role,
				"adversarial %s turn[%d]: invalid role %q", r.ID, i, turn.Role)
			assert.NotEmpty(t, strings.TrimSpace(turn.Content),
				"adversarial %s turn[%d]: empty content", r.ID, i)
		}
	}
	for _, r := range fup {
		assert.GreaterOrEqual(t, r.HistoryAnchor.TurnIndex, 1,
			"followup %s: turn_index must be >= 1", r.ID)
		assert.Equal(t, "", r.HistoryAnchor.FakeChunkRef,
			"followup %s: fake_chunk_ref must be empty (benign group)", r.ID)
		require.GreaterOrEqual(t, len(r.HistoryTurns), 2,
			"followup %s: history_turns must have >= 2 entries", r.ID)
		for i, turn := range r.HistoryTurns {
			assert.Contains(t, []string{"user", "assistant"}, turn.Role,
				"followup %s turn[%d]: invalid role %q", r.ID, i, turn.Role)
			assert.NotEmpty(t, strings.TrimSpace(turn.Content),
				"followup %s turn[%d]: empty content", r.ID, i)
		}
	}

	// Adversarial: 10 distinct fake_chunk_ref values + each appears as a
	// substring in some turn content (anchor self-consistency).
	advRefs := map[string]int{}
	for _, r := range adv {
		found := false
		for _, turn := range r.HistoryTurns {
			if strings.Contains(turn.Content, r.HistoryAnchor.FakeChunkRef) {
				found = true
				break
			}
		}
		assert.True(t, found,
			"adversarial %s: fake_chunk_ref %q not found in any history_turn content",
			r.ID, r.HistoryAnchor.FakeChunkRef)
		advRefs[r.HistoryAnchor.FakeChunkRef]++
	}
	assert.Len(t, advRefs, 10, "expected 10 distinct fake_chunk_ref values, got %d", len(advRefs))

	// Adversarial: baseline_id must resolve AND query must be byte-equal
	// (D9-A3 — A/B uses identical ground truth by construction).
	for _, r := range adv {
		assert.NotEmpty(t, r.BaselineID, "adversarial %s: baseline_id is empty", r.ID)
		baseQuery, ok := baselineQueries[r.BaselineID]
		assert.True(t, ok, "adversarial %s: baseline_id %q not found in golden set",
			r.ID, r.BaselineID)
		if ok {
			spikeQuery := extractQueryFromRow(t, path, r.ID)
			assert.Equal(t, baseQuery, spikeQuery,
				"adversarial %s: query must equal baseline %s query (D9-A3 byte-equal A/B)",
				r.ID, r.BaselineID)
		}
	}

	// Followup: subclass ∈ 3 allowed values, 5 per value, fake_chunk_ref empty.
	subclassCount := map[string]int{}
	for _, r := range fup {
		assert.Contains(t, []string{"anaphora", "post_refusal", "multi_hop_chain"}, r.Subclass,
			"followup %s: invalid subclass %q", r.ID, r.Subclass)
		subclassCount[r.Subclass]++
	}
	assert.Equal(t, 5, subclassCount["anaphora"], "expected 5 anaphora")
	assert.Equal(t, 5, subclassCount["post_refusal"], "expected 5 post_refusal")
	assert.Equal(t, 5, subclassCount["multi_hop_chain"], "expected 5 multi_hop_chain")
}

// loadSpikeRows re-reads the golden file and returns rows whose category
// is one of the phase-9 spike categories. Mirrors the countSpikeAnchors
// raw-line re-scan technique so the schema extension fields can be
// inspected regardless of the Case struct's intentional omissions.
func loadSpikeRows(t *testing.T, path string) []spikeRow {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var rows []spikeRow
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var r spikeRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		if r.Category == "history_injection_adversarial" ||
			r.Category == "history_injection_followup" {
			rows = append(rows, r)
		}
	}
	require.NoError(t, scanner.Err())
	return rows
}

// extractQueryFromRow re-scans the golden file to extract a single spike
// row's `query` field verbatim. Used by TestGoldenHistoryInjectionRows to
// compare each spike row's query against the byte-equal baseline query
// (D9-A3 A/B control). Returns the empty string when the row is absent.
func extractQueryFromRow(t *testing.T, path, id string) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var probe struct {
			ID    string `json:"id"`
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			continue
		}
		if probe.ID == id {
			return probe.Query
		}
	}
	require.NoError(t, scanner.Err())
	return ""
}
