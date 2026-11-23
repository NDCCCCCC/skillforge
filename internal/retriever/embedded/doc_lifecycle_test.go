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

// Phase 53-03 (D-53-03) tests: the doc-lifecycle capability on the
// in-process backend, plus the ZERO-VALUE LEGACY REGRESSION — rows
// written without the three lifecycle fields (the pre-53 producer shape)
// must keep ranking and projecting exactly as before, with the lifecycle
// fields surfacing as zero values and nothing branching on them.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// docLifecycleClient builds a disconnected-free in-process client with a
// 4-dim schema (no embedder needed — Insert takes pre-built vectors).
func docLifecycleClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(Config{Dimensions: 4})
	require.NoError(t, err)
	return c
}

// insertDoc is the row factory: one 4-dim doc with the given lifecycle
// fields (legacy shape = pass the zero values).
func insertDoc(t *testing.T, c *Client, id int64, tier, source, docID string, chunkIndex int32, hash string) {
	t.Helper()
	err := c.Insert(context.Background(), []retriever.Document{{
		ID:          id,
		Tier:        tier,
		Content:     "内容 " + source,
		Source:      source,
		DocID:       docID,
		ChunkIndex:  chunkIndex,
		ContentHash: hash,
		Embedding:   []float32{1, 0, 0, 0},
	}})
	require.NoError(t, err)
}

func TestEmbeddedDocChunkRowsFiltersByTierAndDocID(t *testing.T) {
	c := docLifecycleClient(t)
	insertDoc(t, c, 1, "authority", "authority/a.md", "authority/a.md", 0, "hash-a0")
	insertDoc(t, c, 2, "authority", "authority/a.md", "authority/a.md", 1, "hash-a1")
	// A different tier carrying the SAME doc basename — the tier clause is
	// the trust boundary and must stay load-bearing on the lifecycle path.
	insertDoc(t, c, 3, "personal", "personal/a.md", "personal/a.md", 0, "hash-p0")
	// A different doc in the same tier.
	insertDoc(t, c, 4, "authority", "authority/b.md", "authority/b.md", 0, "hash-b0")

	rows, err := c.DocChunkRows(context.Background(), "authority", "authority/a.md")
	require.NoError(t, err)
	require.Len(t, rows, 2, "only the (authority, authority/a.md) rows")

	got := map[int64]retriever.DocChunk{}
	for _, r := range rows {
		got[r.ID] = r
	}
	require.Contains(t, got, int64(1))
	require.Contains(t, got, int64(2))
	assert.Equal(t, int32(0), got[1].ChunkIndex)
	assert.Equal(t, "hash-a1", got[2].ContentHash)
	// The stored vector rides along — the re-embed skip depends on it.
	assert.Equal(t, []float32{1, 0, 0, 0}, got[1].Embedding)

	// Unknown doc → empty, nil (first upload posture).
	rows, err = c.DocChunkRows(context.Background(), "authority", "authority/missing.md")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestEmbeddedDeleteDocRowsRemovesExactlyTheDoc(t *testing.T) {
	c := docLifecycleClient(t)
	insertDoc(t, c, 1, "authority", "authority/a.md", "authority/a.md", 0, "hash-a0")
	insertDoc(t, c, 2, "authority", "authority/a.md", "authority/a.md", 1, "hash-a1")
	insertDoc(t, c, 3, "personal", "personal/a.md", "personal/a.md", 0, "hash-p0")
	insertDoc(t, c, 4, "authority", "authority/b.md", "authority/b.md", 0, "hash-b0")

	require.NoError(t, c.DeleteDocRows(context.Background(), "authority", "authority/a.md"))

	// The doc's rows are gone.
	rows, err := c.DocChunkRows(context.Background(), "authority", "authority/a.md")
	require.NoError(t, err)
	assert.Empty(t, rows)
	// Everything else survives — including the same doc basename in the
	// other tier.
	rows, err = c.DocChunkRows(context.Background(), "personal", "personal/a.md")
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	rows, err = c.DocChunkRows(context.Background(), "authority", "authority/b.md")
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// The Health invariant (len(docs) == len(byID)) must survive the
	// removal + byID rebuild.
	assert.NoError(t, c.Health(context.Background()))
	// Deleting a doc that has no rows is a no-op, not an error.
	require.NoError(t, c.DeleteDocRows(context.Background(), "authority", "authority/ghost.md"))
}

func TestEmbeddedLegacyZeroValueRowsUnaffected(t *testing.T) {
	c := docLifecycleClient(t)
	// Pre-53 producer shape: DocID="", ChunkIndex=0, ContentHash="".
	insertDoc(t, c, 1, "authority", "authority/legacy.md", "", 0, "")
	insertDoc(t, c, 2, "authority", "authority/new.md", "authority/new.md", 0, "hash-new")

	// Search still ranks legacy rows; the lifecycle fields project as the
	// zero values (pure display metadata — nothing branches on them).
	hits, err := c.Search(context.Background(), []float32{1, 0, 0, 0}, []string{"authority"}, 10, 0)
	require.NoError(t, err)
	require.Len(t, hits, 2)
	bySource := map[string]retriever.Hit{}
	for _, h := range hits {
		bySource[h.Source] = h
	}
	legacy, ok := bySource["authority/legacy.md"]
	require.True(t, ok, "legacy row must still be retrievable")
	assert.Equal(t, "", legacy.DocID, "legacy row doc_id stays zero")
	assert.Equal(t, int32(0), legacy.ChunkIndex)
	assert.Equal(t, "", legacy.ContentHash)
	fresh, ok := bySource["authority/new.md"]
	require.True(t, ok, "lifecycle row projects its fields")
	assert.Equal(t, "authority/new.md", fresh.DocID)
	assert.Equal(t, "hash-new", fresh.ContentHash)

	// ListMetadata / GetChunk carry the same contract.
	metas, err := c.ListMetadata(context.Background())
	require.NoError(t, err)
	require.Len(t, metas, 2)
	for _, m := range metas {
		if m.ID == 1 {
			assert.Equal(t, "", m.DocID)
		}
		if m.ID == 2 {
			assert.Equal(t, "authority/new.md", m.DocID)
		}
	}
	chunk, err := c.GetChunk(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, "authority/new.md", chunk.DocID)
	assert.Equal(t, int32(0), chunk.ChunkIndex)
	assert.Equal(t, "hash-new", chunk.ContentHash)

	// And the upsert NEVER reuses a legacy row: DocChunkRows by the
	// legacy row's Source-like doc_id would match only rows actually
	// stamped — an empty doc_id lookup is not a thing callers do, and an
	// unknown doc_id stays empty.
	rows, err := c.DocChunkRows(context.Background(), "authority", "authority/legacy.md")
	require.NoError(t, err)
	assert.Empty(t, rows, "a legacy row has no doc_id, so it is invisible to the lifecycle lookup")
}

func TestEmbeddedDisconnectedLifecycleErrors(t *testing.T) {
	c := docLifecycleClient(t)
	require.NoError(t, c.Disconnect())
	_, err := c.DocChunkRows(context.Background(), "authority", "authority/a.md")
	assert.ErrorIs(t, err, ErrDisconnected)
	err = c.DeleteDocRows(context.Background(), "authority", "authority/a.md")
	assert.ErrorIs(t, err, ErrDisconnected)
}
