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

// Package embedder defines the abstraction over text-embedding models.
//
// All implementations for a given knowledge base MUST return vectors of
// the same dimension. Mixing dimensions across embeds will silently
// corrupt the vector space (Milvus insert validates dim against the
// collection schema, but the indexer would still embed mixed-dim
// vectors if the Embedder is swapped without re-indexing).
//
// This file holds ONLY the interface. The BGE-M3 implementation lives
// in internal/embedder/bge; future backends (OpenAI text-embedding-3,
// Cohere embed-v3, etc.) will sit alongside it.
package embedder

import "context"

// Embedder is the abstraction over a text-embedding model. Implementations
// must be safe for concurrent use by multiple goroutines (production
// embeds one query at a time per request, but the indexer may fan out
// across docs in Phase 3+).
type Embedder interface {
	// Embed returns a dense vector representation of text. The returned
	// slice length MUST equal Dimensions() on every call. Implementations
	// must NOT fall back to a degraded mode silently on transport
	// errors — they return the error to the caller, who decides whether
	// to refuse (production) or degrade (regression only).
	Embed(ctx context.Context, text string) ([]float32, error)

	// Dimensions returns the output dimensionality of the model. Used
	// by the retriever.Ensure + Insert paths to confirm dim consistency
	// between the embedder and the vector index.
	Dimensions() int
}