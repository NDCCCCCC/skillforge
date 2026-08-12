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

// Package mongo is the MongoDB-backed implementation of retriever.Retriever.
//
// MongoDB 4.4 has no native vector search. We store each document with its
// embedding array and apply the tier filter as a real MongoDB query
// ({tier: {$in: tiers}}) BEFORE computing cosine similarity in Go. This
// preserves the SYSK-07 / P-RAG-2 PRE-filter contract: the tier boundary
// is enforced by the database query itself, not by post-hoc application
// code. Empty tiers → {$in: []} matches nothing → 0 hits → refusal-by-
// construction (D-09).
//
// For Phase 2's ~30-chunk corpus the in-Go cosine is instant and
// mathematically exact. The collection schema:
//
//	id        int64   (numeric primary key, caller-assigned)
//	tier      string  (CN-char tier value: 权威/历史/个人/草稿)
//	content   string  (markdown body)
//	source    string  (slash-normalized relative path)
//	embedding []float32 (BGE-M3 1024-dim, stored as BSON array of doubles)
package mongo

import (
	"sort"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/vecmath"
)

// docRow is the row shape used by both the rank helper (pure logic) and
// the MongoDB decoder (mongo.go). The bson tags are inert metadata
// interpreted only by the mongo-driver decoder; keeping the type in this
// file means rank.go has zero external dependencies beyond the retriever
// types and so is unit-testable with no MongoDB process.
type docRow struct {
	ID        int64     `bson:"id"`
	Tier      string    `bson:"tier"`
	Content   string    `bson:"content"`
	Source    string    `bson:"source"`
	Embedding []float32 `bson:"embedding"`
}

// Cosine returns the cosine similarity between two equal-length
// vectors. Plan 05-02: re-exported from internal/vecmath (the leaf
// package shared with internal/skill/router.go) so callers within the
// mongo package can continue using the convenient local symbol. The
// canonical implementation lives in internal/vecmath — see that
// package for the algorithm details and length-mismatch panic.
//
// Originally lived here in Plan 02-04; promoted to a leaf package in
// Plan 05-02 because internal/skill/router.go needed to reuse cosine
// without creating a cycle through internal/retriever/mongo/seed.go
// (which itself imports internal/skill for TierEnum).
func Cosine(a, b []float32) float32 {
	return vecmath.Cosine(a, b)
}

// rank orders rows by cosine(queryVec, row.Embedding) descending and
// returns the top k as []retriever.Hit. Pure function — no I/O.
//
// Behavior:
//   - rows with embedding length != queryVec length are skipped (defensive
//     — production inserts a fixed-dim embedding, but a schema drift
//     would otherwise panic cosine() and crash Search).
//   - rows with empty embeddings are skipped (zero-norm → cosine returns
//     0, but they would still appear in the ranking at score=0; skip
//     keeps the output clean).
//   - k <= 0 returns an empty slice (defensive — callers should pass a
//     sensible topK but a regression here should not panic).
//   - ties: stable on (score desc, id asc) so re-runs produce identical
//     ordering — important for eval reproducibility (Ragas).
func rank(rows []docRow, queryVec []float32, k int) []retriever.Hit {
	if k <= 0 || len(rows) == 0 || len(queryVec) == 0 {
		return nil
	}
	type scored struct {
		row   docRow
		score float32
	}
	out := make([]scored, 0, len(rows))
	for _, r := range rows {
		if len(r.Embedding) != len(queryVec) || len(r.Embedding) == 0 {
			continue
		}
		out = append(out, scored{row: r, score: Cosine(queryVec, r.Embedding)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score // higher cosine first
		}
		return out[i].row.ID < out[j].row.ID // tie-break: lower ID first
	})
	if k > len(out) {
		k = len(out)
	}
	hits := make([]retriever.Hit, 0, k)
	for i := 0; i < k; i++ {
		s := out[i]
		hits = append(hits, retriever.Hit{
			ID:      s.row.ID,
			Score:   s.score,
			Tier:    s.row.Tier,
			Content: s.row.Content,
			Source:  s.row.Source,
		})
	}
	return hits
}
