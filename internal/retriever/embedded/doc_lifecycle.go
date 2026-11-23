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

package embedded

// doc_lifecycle.go implements retriever.DocChunkStore on the in-process
// backend (Phase 53-03 / D-53-03) — the (tier, doc_id)-addressed
// primitives behind the upload upsert and the
// DELETE /api/admin/kb/:slug/docs/:doc_id endpoint. The semantics mirror
// milvus/doc_lifecycle.go one-for-one; the implementation is trivially
// in-memory (row scan + rebuild of the byID index under the write lock).

import (
	"context"
	"fmt"

	"github.com/skillforge/skillforge/internal/retriever"
)

// Compile-time proof that *Client satisfies the doc-lifecycle capability
// interface (same posture as milvus.MilvusClient's assertion).
var _ retriever.DocChunkStore = (*Client)(nil)

// DocChunkRows returns every row of one (tier, doc_id) as the
// retriever.DocChunk projection: PK, chunk_index, content_hash, embedding.
// The embedding lets the upsert re-insert unchanged chunks with their
// original PK AND original vector (the re-embed skip — D-53-03), exactly
// like the milvus implementation.
//
// Both tier and doc_id must match exactly (==, not prefix), so a doc_id
// collision across tiers can never leak rows between knowledge bases —
// the tier clause is the trust boundary (SYSK-07) and stays load-bearing
// on the lifecycle path. A doc_id with no rows returns (empty, nil).
func (c *Client) DocChunkRows(ctx context.Context, tier, docID string) ([]retriever.DocChunk, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, ErrDisconnected
	}
	out := make([]retriever.DocChunk, 0, 4)
	for _, d := range c.docs {
		if d.Tier != tier || d.DocID != docID {
			continue
		}
		out = append(out, retriever.DocChunk{
			ID:          d.ID,
			ChunkIndex:  d.ChunkIndex,
			ContentHash: d.ContentHash,
			Embedding:   d.Embedding,
		})
	}
	return out, nil
}

// DeleteDocRows removes EVERY row of one (tier, doc_id) and rebuilds the
// byID index. The delete is synchronous (an in-memory slice append) —
// there is no Milvus-style async-visibility window on this backend.
// A doc_id with no rows is a no-op, mirroring the milvus delete posture
// (missing rows ≠ error).
func (c *Client) DeleteDocRows(ctx context.Context, tier, docID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrDisconnected
	}
	kept := c.docs[:0]
	for _, d := range c.docs {
		if d.Tier == tier && d.DocID == docID {
			continue
		}
		kept = append(kept, d)
	}
	if len(kept) == len(c.docs) {
		return nil
	}
	c.docs = kept
	c.rebuildByID()
	return nil
}

// rebuildByID re-derives the ID → docs-index map after a removal.
// Split out so the invariant "len(docs) == len(byID)" (the Health check)
// has exactly one repair site.
func (c *Client) rebuildByID() {
	c.byID = make(map[int64]int, len(c.docs))
	for i, d := range c.docs {
		c.byID[d.ID] = i
	}
	if len(c.docs) != len(c.byID) {
		// Defensive: a duplicate PK would silently break GetChunk lookups.
		// Impossible via Insert (it rejects duplicate ids loudly), but the
		// check keeps the Health invariant honest if a future write path
		// forgets the gate.
		panic(fmt.Sprintf("embedded.rebuildByID: %d docs but %d unique ids", len(c.docs), len(c.byID)))
	}
}
