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

// Package retriever defines the abstraction over tiered vector search
// backends (Milvus, MongoDB Atlas Vector Search, etc.). All
// implementations MUST enforce tier-filter PRE-similarity (SYSK-07, P-RAG-2)
// — this is the core trust boundary.
//
// This file holds ONLY the interface + the generic Hit / Document types.
// The Milvus implementation lives in internal/retriever/milvus; future
// backends (MongoDB, pgvector) will sit alongside it.
package retriever

import "context"

// Hit is a generic search result returned by all Retriever
// implementations. Tier is the CN-char tier value (e.g. "权威") that
// the Skill resolved from its reads slice; Source is the slash-
// normalized path to the original corpus file (e.g.
// "authority/travel_expense.md"). EffectiveFrom / EffectiveTo round-
// trip from the backend so the indexer / pipeline can audit which
// version of a policy each chunk was sourced from.
type Hit struct {
	ID            int64
	Score         float32
	Tier          string
	Content       string
	Source        string
	EffectiveFrom int64
	EffectiveTo   int64
}

// Document is the input to Retriever.Insert (used by the knowledge
// indexer). The ID is the numeric primary key (Milvus INT64 PK; the
// convention is 1-based and monotonically increasing across tier dirs).
//
// Plan 05-02 / D5-15: EffectiveFrom + EffectiveTo are the policy-
// version window (epoch seconds). Sentinel design — NOT nullable
// fields. Zero means "open-ended" / perpetually valid:
//
//   - EffectiveFrom = 0 AND EffectiveTo = 0 → perpetually valid (legacy).
//   - EffectiveFrom = 0 AND EffectiveTo > 0 → open-ended start, ends at <to>.
//   - EffectiveFrom > 0 AND EffectiveTo = 0 → starts at <from>, perpetual.
//   - EffectiveFrom > 0 AND EffectiveTo > 0 → bounded window.
//
// KNOWN-02 (extended) at the indexer layer rejects inverted windows
// (to < from when both nonzero) and negative values (to < 0).
type Document struct {
	ID            int64
	Tier          string
	Content       string
	Source        string
	Embedding     []float32
	EffectiveFrom int64
	EffectiveTo   int64
}

// Retriever is the abstraction over a tiered vector search backend.
//
// Tier-filter semantics: PRE-similarity at the SDK/Search layer
// (NEVER post-filter). Empty/zero tiers → backend returns 0 hits
// (refusal-by-construction, D-09).
//
// Plan 05-02 / D5-15: Search gains a `now int64` parameter that the
// pipeline computes once at entry. The backend applies the effective_to
// PRE-filter at the same layer as the tier filter (NOT a post-filter
// fallback). Sentinel-0 design — NOT nullable fields. The Milvus +
// Mongo backends implement `(effective_to == 0 OR effective_to >= now)`
// co-layered with the existing tier clause.
//
// Implementations:
//
//   - internal/retriever/milvus — the production implementation against
//     a Milvus v2.4.2 gRPC server (Phase 2 reference impl).
type Retriever interface {
	// Health reports whether the backend is reachable. Used by cmd/server
	// startup to fail fast on a misconfigured Milvus address.
	Health(ctx context.Context) error

	// Search returns up to topK hits whose tier is in the tiers slice AND
	// whose effective_to is in the valid window (sentinel: 0 = perpetual).
	// The filter is applied PRE-similarity at the SDK layer; an empty
	// tiers slice MUST return zero hits (refusal-by-construction). The
	// now parameter is the epoch-seconds cutoff the backend uses for
	// the effective_to PRE-filter (compute once at pipeline entry).
	Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]Hit, error)

	// Ensure creates (or no-ops) the collection / index needed by Insert
	// and Search. Idempotent — safe to call on every startup.
	Ensure(ctx context.Context) error

	// Insert adds docs to the backend. Caller pre-assigns numeric IDs;
	// implementations do NOT auto-dedup. The Document.EffectiveFrom /
	// EffectiveTo fields are written into the schema (default 0 for
	// legacy documents).
	Insert(ctx context.Context, docs []Document) error

	// Disconnect closes the underlying client connection. After
	// Disconnect the instance is unusable.
	Disconnect() error
}