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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFaithfulnessWithStubJudge locks the canonical Faithfulness
// aggregation math against a deterministic StubJudge. The scenario:
// 4 atomic claims, 3 supported → 0.75. Refusal answer → 1.0. Empty
// claims list → 1.0.
//
// This is the first test in internal/eval; it pins the "Ragas 口径"
// headline metric's arithmetic. The ported Ragas prompt TEXT is exercised
// end-to-end only via cmd/eval against the real GLM judge (the live
// path); here we contract that whatever the judge returns, Faithfulness
// aggregates it correctly.
func TestFaithfulnessWithStubJudge(t *testing.T) {
	ctx := context.Background()
	const answer = "Einstein was a German-born physicist. He developed relativity. He also worked on quantum mechanics. He lived in Tokyo."

	// 4 claims; Entails flips to true for 3 of them.
	claims := []string{
		"Albert Einstein was a German-born theoretical physicist.",
		"Albert Einstein was best known for developing the theory of relativity.",
		"Albert Einstein also made important contributions to the development of the theory of quantum mechanics.",
		"Albert Einstein lived in Tokyo.", // not supported → verdict 0
	}
	supported := map[string]bool{
		claims[0]: true,
		claims[1]: true,
		claims[2]: true,
		claims[3]: false,
	}
	judge := &StubJudge{
		ClaimsForAnswer: map[string][]string{answer: claims},
		EntailsClaim:    supported,
	}

	score, err := Faithfulness(ctx, answer, []string{"context-a", "context-b"}, judge)
	require.NoError(t, err)
	assert.Equal(t, 0.75, score, "3 supported / 4 total = 0.75")
}

// TestFaithfulnessEmptyAnswerIsOne locks the refusal-answer convention:
// an empty (or whitespace-only) answer gets faithfulness=1.0 because a
// refusal has no claims to refute. The deterministic Go trust floor
// (D3-01) catches the worst-case refusal-before-answer case; this test
// pins the metric's behavior in isolation.
func TestFaithfulnessEmptyAnswerIsOne(t *testing.T) {
	ctx := context.Background()
	judge := &StubJudge{DefaultClaims: []string{"should-not-be-returned"}}

	for _, tc := range []string{"", "   ", "\n\t"} {
		score, err := Faithfulness(ctx, tc, []string{"ctx"}, judge)
		require.NoError(t, err)
		assert.Equal(t, 1.0, score, "empty answer %q should map to 1.0", tc)
	}
}

// TestFaithfulnessEmptyClaimsListIsOne covers the case where the judge
// returns an empty claim list for a non-empty answer (the model gave up
// on claim extraction). Treated identically to empty answer —
// faithfulness=1.0, NOT 0.0 (a 0.0 here would penalize the agent for the
// judge's failure, inverting the trust signal).
func TestFaithfulnessEmptyClaimsListIsOne(t *testing.T) {
	ctx := context.Background()
	judge := &StubJudge{DefaultClaims: nil}
	score, err := Faithfulness(ctx, "non-empty answer", []string{"ctx"}, judge)
	require.NoError(t, err)
	assert.Equal(t, 1.0, score)
}

// TestFaithfulnessDeterministicAcrossCalls runs the metric twice with
// the same stub and asserts the score is byte-identical (SC-6 spirit —
// the StubJudge is deterministic by construction; this test documents
// the contract for anyone reading the test file).
func TestFaithfulnessDeterministicAcrossCalls(t *testing.T) {
	ctx := context.Background()
	claims := []string{"c1", "c2", "c3"}
	judge := &StubJudge{
		DefaultClaims: claims,
		EntailsClaim:  map[string]bool{"c1": true, "c2": true, "c3": false},
	}
	s1, err := Faithfulness(ctx, "any", []string{"ctx"}, judge)
	require.NoError(t, err)
	s2, err := Faithfulness(ctx, "any", []string{"ctx"}, judge)
	require.NoError(t, err)
	assert.True(t, math.Abs(s1-s2) < 1e-9, "deterministic: %v vs %v", s1, s2)
}

// TestFaithfulnessNilJudgeErrors locks the Rule 2 fail-fast: a nil Judge
// surfaces as a clear error instead of a nil-pointer panic.
func TestFaithfulnessNilJudgeErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Faithfulness(ctx, "answer", []string{"ctx"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "judge is nil")
}

// TestFaithfulnessAllSupportedIsOne and TestFaithfulnessNoneSupportedIsZero
// pin the boundary values.
func TestFaithfulnessAllSupportedIsOne(t *testing.T) {
	ctx := context.Background()
	judge := &StubJudge{
		DefaultClaims:  []string{"a", "b"},
		DefaultEntails: true,
	}
	score, err := Faithfulness(ctx, "ans", []string{"ctx"}, judge)
	require.NoError(t, err)
	assert.Equal(t, 1.0, score)
}

func TestFaithfulnessNoneSupportedIsZero(t *testing.T) {
	ctx := context.Background()
	judge := &StubJudge{
		DefaultClaims:  []string{"a", "b"},
		DefaultEntails: false,
	}
	score, err := Faithfulness(ctx, "ans", []string{"ctx"}, judge)
	require.NoError(t, err)
	assert.Equal(t, 0.0, score)
}
