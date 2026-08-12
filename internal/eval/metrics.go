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
	"fmt"
	"math"
	"strings"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
)

// AnswerRelevancy computes the mean pairwise cosine similarity between
// the original query and a set of LLM-generated questions that the
// given answer would naturally address.
//
// Algorithm (Go-authored — NOT ported from Ragas; simple shape that
// matches Ragas quality on a yes/no entailment surface):
//
//  1. judge.GenerateQuestions(answer) → N questions (N defaults to 3,
//     see judge.go generateQuestionsSystem).
//  2. Embed the original query + each generated question via the
//     caller-supplied embedder (BGE-M3 in production).
//  3. Score = mean cosine(query, q_i) across the N generated questions.
//
// A high score means the answer is topically aligned with what a user
// might have asked. A low score means the answer drifts off-topic.
//
// Edge cases:
//
//   - Empty answer → return 0.0 (no answer, no relevance).
//   - Judge returns 0 questions → return 0.0 (signal that the judge is
//     broken or the answer is empty/non-substantive).
//   - Embedder failure on any single question → return the error.
//
// The embedder arg is the same embedder.Embedder the production agent
// uses for retrieval (BGE-M3, 1024-dim). Reusing it means no new
// embedding dependency (D6 reversal benefit: single embedding model).
func AnswerRelevancy(ctx context.Context, query, answer string, emb embedder.Embedder, j Judge) (float64, error) {
	if j == nil {
		return 0, fmt.Errorf("eval.AnswerRelevancy: judge is nil")
	}
	if emb == nil {
		return 0, fmt.Errorf("eval.AnswerRelevancy: embedder is nil")
	}
	if strings.TrimSpace(answer) == "" {
		return 0.0, nil
	}
	questions, err := j.GenerateQuestions(ctx, answer)
	if err != nil {
		return 0, fmt.Errorf("eval.AnswerRelevancy: GenerateQuestions: %w", err)
	}
	if len(questions) == 0 {
		return 0.0, nil
	}

	qVec, err := emb.Embed(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("eval.AnswerRelevancy: embed query: %w", err)
	}

	var sum float64
	var n int
	for _, q := range questions {
		if strings.TrimSpace(q) == "" {
			continue
		}
		qv, err := emb.Embed(ctx, q)
		if err != nil {
			return 0, fmt.Errorf("eval.AnswerRelevancy: embed generated question: %w", err)
		}
		sum += float64(cosine(qVec, qv))
		n++
	}
	if n == 0 {
		return 0.0, nil
	}
	return sum / float64(n), nil
}

// ContextPrecision computes rank-weighted precision over the retrieved
// chunks: high-ranked relevant chunks contribute more than low-ranked
// relevant chunks.
//
// Algorithm (Go-authored — Ragas-equivalent shape):
//
//	For each hit at rank k (1-indexed):
//	  relevant_k = judge.IsChunkRelevant(query, hit.Content)
//	precision@k = (number of relevant in top-k) / k
//	Score = sum(precision@k * relevant_k for k=1..N) / sum(relevant_k)
//
// This is the standard Average Precision@N normalization: a perfect
// ranking (all relevant chunks ranked before any irrelevant ones)
// scores 1.0; random ordering scores ~0.5 for a half-relevant pool.
//
// Edge cases:
//
//   - Empty hits → return 0.0 (no chunks to score).
//   - All-irrelevant hits (sum(relevant_k) == 0) → fall back to
//     simple relevant/total = 0.0. (The formula's denominator is 0 in
//     this branch; we short-circuit rather than divide-by-zero.)
//   - All-relevant hits → score = mean of (1/k sum_{i<=k} 1/i/k) ...
//     the standard formula simplifies to a clean 1.0 when every hit
//     is relevant, which the rank-weighted average yields naturally.
func ContextPrecision(ctx context.Context, query string, hits []retriever.Hit, j Judge) (float64, error) {
	if j == nil {
		return 0, fmt.Errorf("eval.ContextPrecision: judge is nil")
	}
	if len(hits) == 0 {
		return 0.0, nil
	}

	relevantFlags := make([]bool, len(hits))
	var relSum float64
	for i, h := range hits {
		ok, err := j.IsChunkRelevant(ctx, query, h.Content)
		if err != nil {
			return 0, fmt.Errorf("eval.ContextPrecision: IsChunkRelevant(rank=%d): %w", i+1, err)
		}
		relevantFlags[i] = ok
		if ok {
			relSum++
		}
	}
	if relSum == 0 {
		// All-irrelevant — fall back to simple relevant/total = 0.0.
		// Kept as an explicit branch so the formula's divide-by-zero
		// is documented, not implicit.
		return 0.0, nil
	}

	// rank-weighted precision: precision@k = (#relevant in top-k)/k
	var numRel int
	var weightedSum float64
	for k := 0; k < len(hits); k++ {
		if relevantFlags[k] {
			numRel++
			precisionAtK := float64(numRel) / float64(k+1)
			weightedSum += precisionAtK
		}
	}
	return weightedSum / relSum, nil
}

// ContextRecall computes the fraction of ground-truth statements that
// can be attributed to the retrieved contexts.
//
// Algorithm (Go-authored — Ragas-equivalent shape):
//
//  1. If groundTruth == "REFUSAL": the expected behavior was a refusal.
//     Return 1.0 if the system actually refused (refused=true), 0.0
//     otherwise. This lets refusal-required golden cases score correctly
//     without a (non-existent) ground-truth body to attribute.
//  2. Otherwise: split groundTruth into sentence-ish statements, call
//     judge.IsAttributable(statement, concatenated-context) per
//     statement, score = attributable/total.
//
// Edge cases:
//
//   - Empty groundTruth (and not "REFUSAL") → return 0.0.
//   - All-unattributable → return 0.0.
//
// The split is intentionally simple (Chinese-aware: split on 。!？.!?;
// also newlines). Ragas uses nltk's sentence splitter; for the synthetic
// golden set, hand-authored ground_truth values use exactly one
// statement per line, so a newline-based split is sufficient and
// language-agnostic.
func ContextRecall(ctx context.Context, groundTruth string, hits []retriever.Hit, j Judge, refused bool) (float64, error) {
	if j == nil {
		return 0, fmt.Errorf("eval.ContextRecall: judge is nil")
	}

	// REFUSAL sentinel: the golden-set case expects a refusal.
	if strings.TrimSpace(groundTruth) == "REFUSAL" {
		if refused {
			return 1.0, nil
		}
		return 0.0, nil
	}

	statements := splitStatements(groundTruth)
	if len(statements) == 0 {
		return 0.0, nil
	}

	// Concatenate hit contents (the context the agent actually had).
	var ctxParts []string
	for _, h := range hits {
		ctxParts = append(ctxParts, h.Content)
	}
	ctxText := strings.Join(ctxParts, "\n")

	var attributable int
	for _, st := range statements {
		ok, err := j.IsAttributable(ctx, st, ctxText)
		if err != nil {
			return 0, fmt.Errorf("eval.ContextRecall: IsAttributable(%q): %w", st, err)
		}
		if ok {
			attributable++
		}
	}
	return float64(attributable) / float64(len(statements)), nil
}

// splitStatements breaks a ground-truth text into atomic statements.
// Chinese-aware: splits on 。!？.!?; and newlines. Empty fragments are
// dropped. The synthetic golden set is hand-authored so each statement
// is a complete sentence — no robust NLP sentence-boundary detection
// needed (the alternative would be to call judge.ExtractClaims on the
// ground truth, but that introduces a judge dependency into the
// ground-truth path which is undesirable for measurement hygiene).
func splitStatements(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// Replace all terminators with a canonical newline separator.
	repl := strings.NewReplacer(
		"。", "\n",
		"！", "\n",
		"？", "\n",
		".", "\n",
		"!", "\n",
		"?", "\n",
		";", "\n",
		"；", "\n",
	)
	parts := strings.Split(repl.Replace(text), "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cosine computes the cosine similarity between two equal-length float32
// vectors. Returns 0 for the zero vector or length-mismatched vectors
// (the caller is responsible for ensuring both vectors come from the
// same embedder — embedder.Embedder.Dimensions() is the contract).
//
// Mirrors the standard formula: dot(a,b) / (|a| * |b|). BGE-M3 vectors
// are L2-normalized at the model side, so |a|*|b| is ~1 — but the
// explicit normalization here is harmless and protects against a future
// embedder that does not pre-normalize.
func cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return 0
	}
	// float64 intermediates avoid float32 overflow on 1024-dim vectors.
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
