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
	"fmt"
	"math"
	"sort"
)

// CaseScore is the per-case calibration input: the max retrieval score
// (highest cosine similarity among the chunks the retriever surfaced
// for the query) and whether the agent refused (refused=true means the
// trust floor fired OR the LLM answered "unanswered" — both indicate
// "the system did not produce a grounded answer").
//
// Used by Calibrate to derive a recommended retrievalScoreThreshold
// from the empirical score distribution. The 0.3 placeholder
// (internal/server/pipeline.go:43, D2-15) is replaced by this measured
// value via an operator-side ops decision (NOT automatic — the
// calibration data is a RECOMMENDATION; the operator reviews the
// rationale before swapping the literal).
type CaseScore struct {
	MaxScore float32
	Refused  bool
}

// Recommendation is the output of Calibrate, written to
// eval/calibration.json. The RecommendedThreshold field is the D2-15
// retirement vehicle; the operator manually swaps the literal in
// pipeline.go after reviewing the Rationale.
//
// ScoreDistribution carries the summary statistics (median, p25, p75)
// so the operator can see the shape of the score distribution — a
// bimodal distribution (refusals cluster low, answers cluster high)
// indicates the threshold has discriminative power; a flat distribution
// indicates the embedder's score signal is weak (the Phase 2 finding
// re Ollama BGE-M3).
type Recommendation struct {
	RecommendedThreshold  float64            `json:"recommended_threshold"`
	Rationale             string             `json:"rationale"`
	ScoreDistribution     map[string]float64 `json:"score_distribution"`
	RefusalBoundaryScore  float64            `json:"refusal_boundary_score"`
}

// Calibrate computes a RecommendedThreshold from the empirical score
// distribution of the golden set. The algorithm:
//
//  1. Split scores into ANSWERED (Refused=false) and REFUSED (Refused=true).
//  2. Compute the median, 25th, 75th percentiles of the combined
//     distribution for the ScoreDistribution summary.
//  3. RefusalBoundaryScore = the 5th percentile of REFUSED-case scores
//     (the score below which the eval data says "should be refused" —
//     a generous reading that tolerates borderline cases being kept).
//     If there are no refused cases (a flawless eval run, or a golden
//     set with no refusal-required category), the boundary falls back
//     to the 25th percentile of answered-case scores (a conservative
//     "below typical answer" heuristic).
//  4. RecommendedThreshold = midpoint between refusalBoundaryScore and
//     the 25th percentile of answered-case scores. The midpoint is a
//     heuristic separator; the operator may override after reviewing
//     the rationale. The midpoint avoids overfitting to a single
//     outlier in either direction.
//
// Edge cases:
//
//   - Empty input → returns a zero Recommendation with rationale
//     "no cases" (the caller surfaces this as a config error).
//   - Single-case input → RecommendedThreshold = that case's MaxScore.
//   - All-refused or all-answered → refusal branch falls back to the
//     answered distribution's 25th percentile (documented in the
//     Rationale field).
func Calibrate(scores []CaseScore) Recommendation {
	if len(scores) == 0 {
		return Recommendation{
			Rationale:            "no cases provided; threshold unchanged (keep the 0.3 placeholder)",
			ScoreDistribution:    map[string]float64{},
			RefusalBoundaryScore: 0,
		}
	}

	all := make([]float64, 0, len(scores))
	var answered, refused []float64
	for _, s := range scores {
		v := float64(s.MaxScore)
		all = append(all, v)
		if s.Refused {
			refused = append(refused, v)
		} else {
			answered = append(answered, v)
		}
	}

	dist := map[string]float64{
		"median": percentile(all, 0.50),
		"p25":    percentile(all, 0.25),
		"p75":    percentile(all, 0.75),
		"min":    minFloat64(all),
		"max":    maxFloat64(all),
		"count":  float64(len(all)),
	}

	refusalBoundary := percentile(refused, 0.05)
	rationaleTail := "5th percentile of refused-case scores"
	if len(refused) == 0 {
		// No refusals — fall back to answered 25th percentile.
		refusalBoundary = percentile(answered, 0.25)
		rationaleTail = "25th percentile of answered-case scores (no refused cases in the golden set; fallback heuristic)"
	}

	answeredP25 := percentile(answered, 0.25)
	if len(answered) == 0 {
		// All refused — use the global median as the upper anchor.
		answeredP25 = dist["median"]
	}
	recommended := (refusalBoundary + answeredP25) / 2

	// Clamp to [0, 1] — cosine similarity can be slightly negative on
	// pathological embedder output; the threshold is conceptually a
	// probability-like scalar in [0, 1].
	recommended = math.Max(0, math.Min(1, recommended))

	return Recommendation{
		RecommendedThreshold: recommended,
		Rationale: fmt.Sprintf(
			"midpoint of refusal boundary (%.4f, %s) and answered 25th percentile (%.4f); "+
				"%d cases (%d answered, %d refused). Operator should review the distribution "+
				"and adjust manually if the bimodal separation is unclear. See eval/README.md.",
			refusalBoundary, rationaleTail, answeredP25,
			len(all), len(answered), len(refused),
		),
		ScoreDistribution:    dist,
		RefusalBoundaryScore: refusalBoundary,
	}
}

// percentile computes the q-quantile of xs (q in [0,1]) using linear
// interpolation between the closest ranks — the same method numpy's
// default percentile() uses. The input slice is NOT mutated; the helper
// sorts a copy. Empty input returns 0.
//
// Verified against numpy.percentile(x, q*100, method='linear') for the
// canonical [1,2,3,4] cases in unit tests.
func percentile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	if len(xs) == 1 {
		return xs[0]
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)

	// Linear interpolation between closest ranks.
	rank := q * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

func minFloat64(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, v := range xs[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxFloat64(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, v := range xs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
