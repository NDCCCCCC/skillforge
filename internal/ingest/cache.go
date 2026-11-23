// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"time"

	"github.com/skillforge/skillforge/internal/knowledge"
)

// EmbedCache is the narrow seam behind the embed step's cache lookaside
// (54-02 / D-54-02): vectors are keyed by (model, content_hash) — the Dify
// `Embedding(model_name, hash)` uniqueness pattern — so a step retry or a
// re-upload of unchanged content resolves every unchanged chunk at zero
// embedding-endpoint cost.
//
// The SQLite implementation lives in internal/store/sqlite/embed_cache.go
// (rows are immutable: PutEmbedding is INSERT OR IGNORE, first write wins,
// no UPDATE path). The interface is deliberately two methods — the embed
// step needs exactly a batch-get and a put; everything else (dim drift
// validation, BLOB codec, batching) is implementation detail behind it.
type EmbedCache interface {
	// GetEmbeddings batch-resolves vectors for model across content hashes.
	// The returned map carries ONLY the hits — absent keys are cache misses
	// the caller embeds fresh.
	GetEmbeddings(ctx context.Context, model string, hashes []string) (map[string][]float32, error)

	// PutEmbedding stores one vector under (model, contentHash). now is
	// caller-set (RFC3339Nano discipline: the caller stamps, the store
	// persists verbatim).
	PutEmbedding(ctx context.Context, model, contentHash string, vec []float32, now time.Time) error
}

// contentHashOf returns the sha256 hex of one chunk's text — the cache key
// half that pairs with the embedder model name. Single home for the rule:
// the chunk step also hashes (to stamp chunks.jsonl) and MUST produce the
// identical digest, so both call sites share this helper.
func contentHashOf(chunk string) string {
	return knowledge.ContentHashHex(chunk)
}
