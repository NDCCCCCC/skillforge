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

package knowledge

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// stubAuditQuerier satisfies the interface expected by computeCitationEdges:
// it exposes QueryAudit(ctx, store.AuditFilter) ([]store.AuditRow, error).
type stubAuditQuerier struct {
	rows []store.AuditRow
	err  error
}

func (s *stubAuditQuerier) QueryAudit(_ context.Context, _ store.AuditFilter) ([]store.AuditRow, error) {
	return s.rows, s.err
}

// ------------------------------------------------------------------
// TestDeriveEdgesDeterministic — GRF-02 gate: same input → byte-identical output
// ------------------------------------------------------------------

func TestDeriveEdgesDeterministic(t *testing.T) {
	metas := []retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "travel_expense.md", EffectiveFrom: 0, EffectiveTo: 1735689600},
		{ID: 2, Tier: "authority", Source: "travel_expense.md", EffectiveFrom: 1735689601, EffectiveTo: 0},
		{ID: 3, Tier: "authority", Source: "travel_expense.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 4, Tier: "authority", Source: "travel_expense_policy.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 5, Tier: "historical", Source: "old_policy.md", EffectiveFrom: 0, EffectiveTo: 1720137600},
	}

	// Sort by (From, To, EdgeType, Weight) — the same four keys the
	// implementation's final comparator uses (graph_edges.go). Mirroring the
	// comparator is not optional: sorting on fewer keys would mask a Weight
	// divergence between two otherwise identical edges.
	sorted := func(e []Edge) []Edge {
		out := make([]Edge, len(e))
		copy(out, e)
		sort.Slice(out, func(i, j int) bool {
			if out[i].From != out[j].From {
				return out[i].From < out[j].From
			}
			if out[i].To != out[j].To {
				return out[i].To < out[j].To
			}
			if out[i].EdgeType != out[j].EdgeType {
				return out[i].EdgeType < out[j].EdgeType
			}
			return out[i].Weight < out[j].Weight
		})
		return out
	}

	ref, err := ComputeEdges(metas, nil)
	require.NoError(t, err)
	refSorted := sorted(ref)

	// Root cause this test used to miss: ComputeEdges built its `sources` slice
	// by ranging over a Go map, so the `i < j` Jaccard pairing loop emitted
	// keyword_cooccur edges with a randomly flipped From/To. Because
	// EdgeGraph.Neighbors is a DIRECTED traversal, a flip changes the reachable
	// node set — a real production nondeterminism, not a test artifact. The
	// GRF-02 contract (same input → byte-identical []Edge output) now covers
	// direction, not only the final sort order.
	//
	// 100 iterations, because the pre-fix failure rate was ~17-25% per call:
	// two consecutive runs proved almost nothing. Deliberately NOT a t.Run loop
	// — the gate's anti-vacuous --- PASS floor is calibrated against top-level
	// test names, so this must stay a single emitting test.
	//
	// FORBIDDEN to "fix" a red run here: widening an InDelta tolerance, or
	// switching to an order-insensitive / count-based comparison. Both would
	// launder a genuine implementation nondeterminism into a green test, which
	// is exactly the failure mode GRF-02 exists to prevent. If this test goes
	// red, the fix belongs in graph_edges.go.
	for run := 0; run < 100; run++ {
		got, err := ComputeEdges(metas, nil)
		require.NoError(t, err)
		gotSorted := sorted(got)
		assert.Equal(t, refSorted, gotSorted,
			"run %d: ComputeEdges output must be byte-identical for identical input (GRF-02)", run)
	}
}

// ------------------------------------------------------------------
// TestEdgeSectionAdjacent — consecutive IDs within same source get edges
// ------------------------------------------------------------------

func TestEdgeSectionAdjacent(t *testing.T) {
	tests := []struct {
		name      string
		metas     []retriever.DocumentMeta
		wantEdges int
	}{
		{
			name: "three consecutive chunks yield two edges",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 2, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 3, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
			},
			wantEdges: 2,
		},
		{
			name: "non-consecutive IDs (1,3,5) yield zero edges",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 3, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 5, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
			},
			wantEdges: 0,
		},
		{
			name: "single chunk yields zero edges",
			metas: []retriever.DocumentMeta{
				{ID: 7, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
			},
			wantEdges: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edges, err := ComputeEdges(tc.metas, nil)
			require.NoError(t, err)

			sectionEdges := filterEdges(edges, EdgeTypeSectionAdjacent)
			assert.Len(t, sectionEdges, tc.wantEdges, "section_adjacent edge count mismatch")

			for _, e := range sectionEdges {
				assert.InDelta(t, e.Weight, 1.0, 0, "section_adjacent weight must be 1.0")
			}
		})
	}
}

// ------------------------------------------------------------------
// TestEdgeVersionChain — EffectiveFrom/To windows indicating supersession
// ------------------------------------------------------------------

func TestEdgeVersionChain(t *testing.T) {
	tests := []struct {
		name      string
		metas     []retriever.DocumentMeta
		wantEdges int
	}{
		{
			name: "A superseded by B when windows touch at boundary",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 1735689600},
				{ID: 2, Tier: "authority", Source: "policy.md", EffectiveFrom: 1735689600, EffectiveTo: 0},
			},
			wantEdges: 1,
		},
		{
			name: "disjoint windows (no overlap) yield zero version edges",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 100},
				{ID: 2, Tier: "authority", Source: "policy.md", EffectiveFrom: 200, EffectiveTo: 0},
			},
			wantEdges: 0,
		},
		{
			name: "same EffectiveFrom (equal) does not create chain",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 1000, EffectiveTo: 2000},
				{ID: 2, Tier: "authority", Source: "policy.md", EffectiveFrom: 1000, EffectiveTo: 0},
			},
			wantEdges: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edges, err := ComputeEdges(tc.metas, nil)
			require.NoError(t, err)

			versionEdges := filterEdges(edges, EdgeTypeVersionChain)
			assert.Len(t, versionEdges, tc.wantEdges, "version_chain edge count mismatch")
			for _, e := range versionEdges {
				assert.InDelta(t, e.Weight, 1.0, 0, "version_chain weight must be 1.0")
			}
		})
	}
}

// ------------------------------------------------------------------
// TestEdgeKeywordCooccur — filename token Jaccard threshold
// ------------------------------------------------------------------

func TestEdgeKeywordCooccur(t *testing.T) {
	// Suppress nil-store WARN log during these tests.
	orig := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(orig)

	tests := []struct {
		name      string
		metas     []retriever.DocumentMeta
		wantEdges int
	}{
		{
			name: "travel-expense-policy-2024 and 2025 share tokens with Jaccard >= 0.3",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "travel_expense_policy_2024.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 2, Tier: "authority", Source: "travel_expense_policy_2025.md", EffectiveFrom: 0, EffectiveTo: 0},
			},
			wantEdges: 1,
		},
		{
			name: "unrelated filenames yield no keyword cooccur edges",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "travel_expense.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 2, Tier: "authority", Source: "procurement_policy.md", EffectiveFrom: 0, EffectiveTo: 0},
			},
			wantEdges: 0,
		},
		{
			name: "same source — not cross-source, no keyword edge",
			metas: []retriever.DocumentMeta{
				{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
				{ID: 2, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 0},
			},
			wantEdges: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edges, err := ComputeEdges(tc.metas, nil)
			require.NoError(t, err)

			keywordEdges := filterEdges(edges, EdgeTypeKeywordCooccur)
			assert.Len(t, keywordEdges, tc.wantEdges, "keyword_cooccur edge count mismatch")
			for _, e := range keywordEdges {
				assert.GreaterOrEqual(t, e.Weight, 0.3, "keyword_cooccur weight must be >= 0.3")
			}
		})
	}
}

// ------------------------------------------------------------------
// TestEdgeCitationCooccur — audit rows with same VerdictJSON
// ------------------------------------------------------------------

func TestEdgeCitationCooccur(t *testing.T) {
	// Set threshold to 0 to isolate citation edges (no keyword_cooccur).
	t.Setenv("SKILLFORGE_GRAPH_JACCARD_THRESHOLD", "0")

	v := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []verdict.Citation{
			{ChunkID: "1", Span: "span1", Tier: "authority"},
			{ChunkID: "7", Span: "span2", Tier: "authority"},
		},
	}
	vJSON, err := json.Marshal(v)
	require.NoError(t, err)

	stub := &stubAuditQuerier{
		rows: []store.AuditRow{
			{ID: 1, VerdictJSON: string(vJSON)},
		},
	}

	edges, err := ComputeEdges([]retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "a.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 7, Tier: "authority", Source: "b.md", EffectiveFrom: 0, EffectiveTo: 0},
	}, stub)
	require.NoError(t, err)

	citationEdges := filterEdges(edges, EdgeTypeCitationCooccur)
	assert.NotEmpty(t, citationEdges, "verdict with 2 citations must yield at least one cooccur edge")

	// Verify edge exists between 1 and 7.
	found := false
	for _, e := range citationEdges {
		if (e.From == "1" && e.To == "7") || (e.From == "7" && e.To == "1") {
			found = true
			assert.InDelta(t, e.Weight, 0.5, 0, "weight must be 1.0/2=0.5")
		}
	}
	assert.True(t, found, "edge between chunk 1 and chunk 7 must exist")
}

// ------------------------------------------------------------------
// TestDeriveEdgesEmpty — nil/empty input yields nil/empty, no panic
// ------------------------------------------------------------------

func TestDeriveEdgesEmpty(t *testing.T) {
	edges, err := ComputeEdges(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, edges)

	edges, err = ComputeEdges([]retriever.DocumentMeta{}, nil)
	assert.NoError(t, err)
	assert.Nil(t, edges)
}

// ------------------------------------------------------------------
// TestJaccardThresholdConfigurable — env override respected
// ------------------------------------------------------------------

func TestJaccardThresholdConfigurable(t *testing.T) {
	// Suppress nil-store WARN log.
	orig := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(orig)

	// travel_expense_report.md vs expense_form.md: Jaccard = 0.25.
	// With default threshold 0.3: no edge. With threshold 0.1: 1 edge.
	metas := []retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "travel_expense_report.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 2, Tier: "authority", Source: "expense_form.md", EffectiveFrom: 0, EffectiveTo: 0},
	}

	// With default 0.3 threshold → 0 edges.
	edges, err := ComputeEdges(metas, nil)
	require.NoError(t, err)
	keywordEdges := filterEdges(edges, EdgeTypeKeywordCooccur)
	assert.Len(t, keywordEdges, 0, "Jaccard 0.25 must not exceed threshold 0.3")

	// Set threshold to 0.1 via t.Setenv (proper test isolation) → 1 edge.
	t.Setenv("SKILLFORGE_GRAPH_JACCARD_THRESHOLD", "0.1")

	edges, err = ComputeEdges(metas, nil)
	require.NoError(t, err)
	keywordEdges = filterEdges(edges, EdgeTypeKeywordCooccur)
	assert.Len(t, keywordEdges, 1, "Jaccard 0.25 must exceed threshold 0.1")
}

// ------------------------------------------------------------------
// TestEdgeCitationCooccur_NoStore — nil store skips citation, no panic
// ------------------------------------------------------------------

func TestEdgeCitationCooccur_NoStore(t *testing.T) {
	// Suppress nil-store WARN log.
	orig := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(orig)

	metas := []retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "policy.md", EffectiveFrom: 0, EffectiveTo: 1735689600},
		{ID: 2, Tier: "authority", Source: "policy.md", EffectiveFrom: 1735689601, EffectiveTo: 0},
	}

	edges, err := ComputeEdges(metas, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, edges, "nil store must still yield section_adjacent + version_chain edges")

	// No citation_cooccur edges when store is nil.
	citationEdges := filterEdges(edges, EdgeTypeCitationCooccur)
	assert.Len(t, citationEdges, 0, "nil store must not produce citation_cooccur edges")
}

// ------------------------------------------------------------------
// Helper: filterEdges returns edges of a specific type.
// ------------------------------------------------------------------

func filterEdges(edges []Edge, edgeType string) []Edge {
	var filtered []Edge
	for _, e := range edges {
		if e.EdgeType == edgeType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
