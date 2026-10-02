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

package mongo

import (
	"math"
	"testing"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// approxEq returns true when a and b are within 1e-5 of each other —
// tight enough to distinguish orthogonal (0) from identical (1) yet
// tolerant of float32 rounding in the dot/normalization products.
func approxEq(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-5
}

// TestCosine_IdenticalVectors returns 1.0 — the canonical sanity check.
// BGE-M3 vectors are L2-normalized in production so identical inputs are
// a valid baseline (cos of a vector with itself under L2 norm = 1).
func TestCosine_IdenticalVectors(t *testing.T) {
	v := []float32{1, 2, 3, 4}
	got := Cosine(v, v)
	assert.True(t, approxEq(got, 1.0), "cosine(v,v) = %v, want ~1.0", got)
}

// TestCosine_OrthogonalVectors returns 0 — the dot product is zero when
// the vectors share no direction.
func TestCosine_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	got := Cosine(a, b)
	assert.True(t, approxEq(got, 0), "cosine(orthogonal) = %v, want ~0", got)
}

// TestCosine_OppositeVectors returns -1 — same line, opposite direction.
func TestCosine_OppositeVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	got := Cosine(a, b)
	assert.True(t, approxEq(got, -1.0), "cosine(opposite) = %v, want ~-1.0", got)
}

// TestCosine_LengthMismatchPanics guards the caller contract: Search
// validates lengths before calling cosine, but a future change that
// skips the check should fail loud (panic) rather than silently read
// out-of-bounds memory.
func TestCosine_LengthMismatchPanics(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r, "cosine must panic on length mismatch")
	}()
	_ = Cosine([]float32{1, 2, 3}, []float32{1, 2})
}

// TestCosine_ZeroNormReturnsZero — the zero-vector case (all zeros) is
// a divide-by-zero trap. Returning 0 matches Milvus' behavior for the
// same input and prevents NaN from poisoning the ranking.
func TestCosine_ZeroNormReturnsZero(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	got := Cosine(a, b)
	assert.True(t, approxEq(got, 0), "cosine(0-norm, v) = %v, want 0 (not NaN)", got)
}

// TestCosine_EmptyVectors — zero-length input. cosine([]) has zero norm
// on both sides; we expect 0 (not a panic, not NaN).
func TestCosine_EmptyVectors(t *testing.T) {
	got := Cosine([]float32{}, []float32{})
	assert.True(t, approxEq(got, 0), "cosine([], []) = %v, want 0", got)
}

// TestCosine_KnownValue pins a hand-computed cosine so a future
// "optimization" that drops the normalization can't silently regress.
// a=[1,0,0], b=[1,1,0] -> dot=1, |a|=1, |b|=sqrt(2) -> cos=1/sqrt(2)≈0.7071
func TestCosine_KnownValue(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 1, 0}
	got := Cosine(a, b)
	want := float32(1.0 / math.Sqrt2)
	assert.True(t, approxEq(got, want), "cosine([1,0,0],[1,1,0]) = %v, want %v", got, want)
}

// TestRank_OrdersBySimilarityDesc — three rows with known cosines against
// the query; rank must return them sorted high-to-low.
func TestRank_OrdersBySimilarityDesc(t *testing.T) {
	q := []float32{1, 0}
	rows := []docRow{
		{ID: 1, Embedding: []float32{0, 1}},  // cos=0 (orthogonal)
		{ID: 2, Embedding: []float32{1, 0}},  // cos=1 (identical direction)
		{ID: 3, Embedding: []float32{-1, 0}}, // cos=-1 (opposite)
		{ID: 4, Embedding: []float32{1, 1}},  // cos≈0.7071 (45 degrees)
	}
	hits := rank(rows, q, 4)
	require.Len(t, hits, 4)
	// Expected order: 2 (cos=1), 4 (cos≈0.707), 1 (cos=0), 3 (cos=-1).
	assert.Equal(t, int64(2), hits[0].ID)
	assert.Equal(t, int64(4), hits[1].ID)
	assert.Equal(t, int64(1), hits[2].ID)
	assert.Equal(t, int64(3), hits[3].ID)
	assert.True(t, approxEq(hits[0].Score, 1.0))
	assert.True(t, approxEq(hits[3].Score, -1.0))
}

// TestRank_RespectsTopK — topK shorter than rows returns only the top K.
func TestRank_RespectsTopK(t *testing.T) {
	q := []float32{1, 0}
	rows := []docRow{
		{ID: 1, Embedding: []float32{1, 0}},
		{ID: 2, Embedding: []float32{0, 1}},
		{ID: 3, Embedding: []float32{-1, 0}},
	}
	hits := rank(rows, q, 2)
	require.Len(t, hits, 2)
	assert.Equal(t, int64(1), hits[0].ID) // cos=1
	// ID 2 and 3 both have non-positive cosine; the higher one wins (2 has cos=0, 3 has cos=-1).
	assert.Equal(t, int64(2), hits[1].ID)
}

// TestRank_TopKExceedsRows — k larger than rows returns all rows
// (defensive; the caller is responsible for sizing topK).
func TestRank_TopKExceedsRows(t *testing.T) {
	q := []float32{1, 0}
	rows := []docRow{{ID: 7, Embedding: []float32{1, 0}}}
	hits := rank(rows, q, 99)
	require.Len(t, hits, 1)
	assert.Equal(t, int64(7), hits[0].ID)
}

// TestRank_EmptyInputReturnsNil — no rows → no hits (not an error, not a
// panic). Search relies on this when the tier filter matches nothing.
func TestRank_EmptyInputReturnsNil(t *testing.T) {
	hits := rank(nil, []float32{1, 0}, 5)
	assert.Nil(t, hits)
}

// TestRank_EmptyQueryVecReturnsNil — defensive: a nil/empty query vector
// can't be ranked (all cosines would be 0). Search treats this as a
// programming error upstream and never calls rank with an empty query.
func TestRank_EmptyQueryVecReturnsNil(t *testing.T) {
	rows := []docRow{{ID: 1, Embedding: []float32{1, 0}}}
	hits := rank(rows, nil, 5)
	assert.Nil(t, hits)
	hits = rank(rows, []float32{}, 5)
	assert.Nil(t, hits)
}

// TestRank_ZeroOrNegativeKReturnsNil — k<=0 is a no-op (defensive).
func TestRank_ZeroOrNegativeKReturnsNil(t *testing.T) {
	rows := []docRow{{ID: 1, Embedding: []float32{1, 0}}}
	assert.Nil(t, rank(rows, []float32{1, 0}, 0))
	assert.Nil(t, rank(rows, []float32{1, 0}, -1))
}

// TestRank_SkipsDimensionMismatchedRows — a row whose embedding length
// differs from the query is silently skipped (defensive — production
// inserts a fixed dim, but a schema drift should not crash Search).
func TestRank_SkipsDimensionMismatchedRows(t *testing.T) {
	q := []float32{1, 0}
	rows := []docRow{
		{ID: 1, Embedding: []float32{1, 0}},    // match
		{ID: 2, Embedding: []float32{1, 0, 0}}, // wrong dim — skip
		{ID: 3, Embedding: nil},                // nil embedding — skip
		{ID: 4, Embedding: []float32{0, 1}},    // match (orthogonal)
	}
	hits := rank(rows, q, 4)
	require.Len(t, hits, 2, "only dim-matching rows participate in ranking")
	assert.Equal(t, int64(1), hits[0].ID)
	assert.Equal(t, int64(4), hits[1].ID)
}

// TestRank_TieBreakByID — equal cosines resolve to lower ID first so
// eval (Ragas) sees deterministic ordering across re-runs.
func TestRank_TieBreakByID(t *testing.T) {
	q := []float32{1, 0}
	rows := []docRow{
		{ID: 30, Embedding: []float32{1, 0}}, // cos=1
		{ID: 10, Embedding: []float32{1, 0}}, // cos=1 (tie)
		{ID: 20, Embedding: []float32{1, 0}}, // cos=1 (tie)
	}
	hits := rank(rows, q, 3)
	require.Len(t, hits, 3)
	assert.Equal(t, int64(10), hits[0].ID)
	assert.Equal(t, int64(20), hits[1].ID)
	assert.Equal(t, int64(30), hits[2].ID)
}

// TestRank_PreservesRowFields — the returned Hit carries Tier/Content/
// Source verbatim from the input row, not just ID+Score. The pipeline
// depends on Tier + Content + Source for the citation moat (D2-06).
func TestRank_PreservesRowFields(t *testing.T) {
	q := []float32{1, 0}
	rows := []docRow{{
		ID:        42,
		Tier:      "权威",
		Content:   "出差住宿费超过 500 元须审批。",
		Source:    "authority/travel_expense.md",
		Embedding: []float32{1, 0},
	}}
	hits := rank(rows, q, 1)
	require.Len(t, hits, 1)
	h := hits[0]
	assert.Equal(t, int64(42), h.ID)
	assert.Equal(t, "权威", h.Tier)
	assert.Equal(t, "出差住宿费超过 500 元须审批。", h.Content)
	assert.Equal(t, "authority/travel_expense.md", h.Source)
}

// TestRank_ReturnsHitsNotInternalType — compile-time guard that rank
// returns retriever.Hit (the cross-backend wire type), not the
// mongo-private docRow. A future contributor who tries to "optimize" by
// returning []docRow breaks this test, which is the canary for the
// retriever.Retriever contract.
func TestRank_ReturnsHitsNotInternalType(t *testing.T) {
	hits := rank([]docRow{{ID: 1, Embedding: []float32{1}}}, []float32{1}, 1)
	require.Len(t, hits, 1)
	var _ retriever.Hit = hits[0]
}
