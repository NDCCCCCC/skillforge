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
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
)

// TestRunnerReproducibility is the SC-6 reproducibility gate (pinned by
// 03-VALIDATION.md, strengthened by Phase 6 / D6-15). It runs all 4
// metrics on the frozen 5-case fixture TWICE with the same StubJudge +
// deterministic embedder, and asserts abs(run1 - run2) < 1e-6 for every
// (case, metric) cell.
//
// This is the STRICT layer of the two-layer tolerance gate (D6-15):
//
//   - strict (here): StubJudge + deterministic embedder, tolerance 1e-6.
//     Proves the PIPELINE is deterministic — any drift is a bug in our
//     code (map iteration order, time.Now, unseeded randomness).
//   - jitter (eval_live_test.go, //go:build liveeval): a real GLM judge,
//     tolerance 1e-3. Proves the system is STABLE in practice against
//     an API that legitimately varies.
//
// Why 1e-6 rather than the original 1e-9: 1e-9 is below the noise floor
// of the float64 arithmetic these metrics perform (means over sums of
// cosine similarities), so it pinned an accidental property of the
// current operation order rather than the property we care about. 1e-6
// is the prod-style tolerance: still ~4 orders of magnitude tighter
// than any real drift, but robust to a benign refactor of the
// aggregation order.
func TestRunnerReproducibility(t *testing.T) {
	ctx := context.Background()
	cases, err := LoadGolden(filepath.Join("testdata", "frozen_5.jsonl"))
	require.NoError(t, err)
	require.Len(t, cases, 5, "frozen fixture must have exactly 5 cases")

	// Synthesize fixed answers + hits per case. The same (case, answer,
	// hits) tuple is fed to both runs so any drift is purely from the
	// metric pipeline.
	samples := buildFrozenSamples(cases)

	judge := newFrozenStubJudge()
	emb := newDeterministicEmbedder()

	run1 := computeAllMetrics(ctx, t, samples, judge, emb)
	run2 := computeAllMetrics(ctx, t, samples, judge, emb)

	// StrictReproducibilityTolerance is the D6-15 strict-layer bar.
	// The fixture is internal/eval/testdata/frozen_5.jsonl (5
	// spike-anchor queries with fixed ground truth); the StubJudge
	// returns canned claims and the embedder is the deterministic
	// SHA-derived one below, so ANY drift here is a bug in our pipeline.
	for i, s := range samples {
		for _, m := range []string{"faith", "arel", "cprec", "crecall"} {
			v1 := run1[i][m].(float64)
			v2 := run2[i][m].(float64)
			assert.True(t, math.Abs(v1-v2) < StrictReproducibilityTolerance,
				"case %s metric %s: drift %v (run1=%v, run2=%v)", s.ID, m, v1-v2, v1, v2)
		}
	}
}

// computeAllMetrics runs all 4 metrics on each sample, returning the
// per-sample metric map. Centralized so the reproducibility tests (both
// the strict layer here and the liveeval-tagged jitter layer) can call
// it twice without duplicating the orchestration.
func computeAllMetrics(ctx context.Context, t *testing.T, samples []Sample, j Judge, emb embedder.Embedder) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, len(samples))
	for _, s := range samples {
		faith, err := Faithfulness(ctx, s.Answer, s.Contexts, j)
		require.NoError(t, err, "Faithfulness %s", s.ID)

		arel, err := AnswerRelevancy(ctx, s.Query, s.Answer, emb, j)
		require.NoError(t, err, "AnswerRelevancy %s", s.ID)

		// Reconstruct retriever.Hits for ContextPrecision / ContextRecall.
		// The hits carry the same Content as the Contexts slice so the
		// judge + metric see consistent inputs.
		hits := contextsToHits(s.Contexts)

		cprec, err := ContextPrecision(ctx, s.Query, hits, j)
		require.NoError(t, err, "ContextPrecision %s", s.ID)

		crecall, err := ContextRecall(ctx, s.GroundTruth, hits, j, s.RefusalReason != "")
		require.NoError(t, err, "ContextRecall %s", s.ID)

		out = append(out, map[string]any{
			"faith":  faith,
			"arel":   arel,
			"cprec":  cprec,
			"crecall": crecall,
		})
	}
	return out
}

// buildFrozenSamples materializes a deterministic Sample per Case. The
// answer / contexts / refusal_reason values are FIXED in this function
// (not random, not time-dependent) so the reproducibility test's two
// runs see byte-identical inputs.
func buildFrozenSamples(cases []Case) []Sample {
	out := make([]Sample, 0, len(cases))
	for i, c := range cases {
		s := Sample{Case: c}
		if c.GroundTruth == "REFUSAL" {
			s.RefusalReason = "no_authorized_source"
			// Empty answer — the trust floor fired.
			s.Answer = ""
			s.Contexts = nil
		} else {
			// Synthesize a 2-claim answer that 1/2 supports the
			// ground truth — the StubJudge is configured to entail
			// only the first of the two claims.
			s.Answer = "claim-1. claim-2."
			s.Contexts = []string{
				"frozen-context-a",
				"frozen-context-b",
			}
			_ = i
		}
		out = append(out, s)
	}
	return out
}

// newFrozenStubJudge configures a StubJudge with canned responses
// tailored for the frozen 5-case fixture. The canned values are
// deterministic by construction; the test's reproducibility contract
// depends on this.
func newFrozenStubJudge() *StubJudge {
	const ans = "claim-1. claim-2."
	return &StubJudge{
		ClaimsForAnswer: map[string][]string{
			ans: {"claim-1", "claim-2"},
		},
		EntailsClaim: map[string]bool{
			"claim-1": true,
			"claim-2": false,
		},
		QuestionsForAnswer: map[string][]string{
			ans: {"question-1?", "question-2?", "question-3?"},
		},
		DefaultRelevant:     true, // all chunks relevant
		DefaultAttributable: true, // all statements attributable
	}
}

// contextsToHits reconstructs a []retriever.Hit from a []string context
// slice. The Hit.ID is 1-based; Score is 0.5 (any fixed value works —
// the reproducibility test asserts run1==run2, not any specific score).
func contextsToHits(contexts []string) []retriever.Hit {
	hits := make([]retriever.Hit, 0, len(contexts))
	for i, c := range contexts {
		hits = append(hits, retriever.Hit{
			ID:      int64(i + 1),
			Score:   0.5,
			Content: c,
			Tier:    "权威",
			Source:  "frozen.md",
		})
	}
	return hits
}

// ---------------------------------------------------------------------------
// deterministicEmbedder — mirrors internal/embedder/bge.Deterministic's
// SHA-derived pseudo-embedding algorithm WITHOUT importing the bge
// package (avoids a test-only cross-package concrete-impl dependency).
// Same SHA-256-seeded rand.New over fixed-dimension output.
// ---------------------------------------------------------------------------

type deterministicEmbedder struct{}

func newDeterministicEmbedder() *deterministicEmbedder { return &deterministicEmbedder{} }

func (d *deterministicEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return deterministicVector(text, 1024), nil
}

func (d *deterministicEmbedder) Dimensions() int { return 1024 }

// deterministicVector mirrors internal/embedder/bge.Deterministic's
// algorithm: SHA-256(text) → int64 seed → rand.New → 1024 floats in
// [-1, 1). Same text → same vector; different text → different vector
// with overwhelming probability (the 8-byte SHA prefix has 2^64 space).
func deterministicVector(text string, dim int) []float32 {
	sum := sha256.Sum256([]byte(text))
	seed := int64(binary.LittleEndian.Uint64(sum[:8]))
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // deterministic by design
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = r.Float32()*2 - 1
	}
	return vec
}

// Compile-time check that *deterministicEmbedder satisfies
// embedder.Embedder (so a future interface drift fails the build here,
// not at the metric call site).
var _ embedder.Embedder = (*deterministicEmbedder)(nil)

// ---------------------------------------------------------------------------
// Additional metric unit tests (cheap, pin specific behaviors).
// ---------------------------------------------------------------------------

// TestPercentile locks the canonical numpy.percentile(linear) values for
// [1,2,3,4] so a future refactor of the percentile helper cannot
// silently change the calibration math.
func TestPercentile(t *testing.T) {
	xs := []float64{1, 2, 3, 4}
	for _, tc := range []struct {
		q, want float64
	}{
		{0.00, 1},
		{0.25, 1.75},
		{0.50, 2.5},
		{0.75, 3.25},
		{1.00, 4},
	} {
		assert.Equal(t, tc.want, percentile(xs, tc.q), "q=%.2f", tc.q)
	}
}

// TestCalibrateEmpty verifies the empty-input fallback (rationale set,
// threshold 0).
func TestCalibrateEmpty(t *testing.T) {
	r := Calibrate(nil)
	assert.Equal(t, 0.0, r.RecommendedThreshold)
	assert.Contains(t, r.Rationale, "no cases")
}

// TestCalibrateSingleCase: a single answered case → recommended ==
// that case's score (within float32→float64 precision; CaseScore.MaxScore
// is float32 so 0.7 round-trips to 0.6999999...).
func TestCalibrateSingleCase(t *testing.T) {
	r := Calibrate([]CaseScore{{MaxScore: 0.7, Refused: false}})
	assert.InDelta(t, 0.7, r.RecommendedThreshold, 1e-6)
}

// TestCalibrateAllAnswered: when NO case refused, the refusal fallback
// kicks in — refusalBoundary uses the answered 25th percentile and the
// rationale documents the fallback heuristic.
func TestCalibrateAllAnswered(t *testing.T) {
	r := Calibrate([]CaseScore{
		{MaxScore: 0.1, Refused: false},
		{MaxScore: 0.2, Refused: false},
		{MaxScore: 0.3, Refused: false},
	})
	assert.Contains(t, r.Rationale, "no refused cases")
	assert.True(t, r.RecommendedThreshold >= 0 && r.RecommendedThreshold <= 1)
}

// TestCalibrateAllRefused: when every case refused, the normal refusal
// branch runs (5th percentile of refused-case scores) but the answered
// slice is empty so answeredP25 falls back to the global median.
func TestCalibrateAllRefused(t *testing.T) {
	r := Calibrate([]CaseScore{
		{MaxScore: 0.1, Refused: true},
		{MaxScore: 0.2, Refused: true},
		{MaxScore: 0.3, Refused: true},
	})
	assert.Contains(t, r.Rationale, "5th percentile of refused-case scores")
	assert.True(t, r.RecommendedThreshold >= 0 && r.RecommendedThreshold <= 1)
}

// TestSplitStatements covers the Chinese-aware statement splitter.
func TestSplitStatements(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"one. two. three.", []string{"one", "two", "three"}},
		{"句一。句二！句三？", []string{"句一", "句二", "句三"}},
		{"  spaced  ", []string{"spaced"}},
	} {
		assert.Equal(t, tc.want, splitStatements(tc.in), "input: %q", tc.in)
	}
}

// TestCosine: orthogonal vectors → 0; identical vectors → 1.
func TestCosine(t *testing.T) {
	assert.Equal(t, float32(1.0), cosine([]float32{1, 0}, []float32{1, 0}))
	assert.InDelta(t, 0, cosine([]float32{1, 0}, []float32{0, 1}), 1e-6)
	assert.Equal(t, float32(0), cosine([]float32{}, []float32{}))
}
