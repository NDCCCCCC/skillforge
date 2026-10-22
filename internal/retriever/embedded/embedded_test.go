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

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// newTestClient builds a bare in-memory client with the given dimensions.
// 零网络、零 embedder：本文件的测试只验证进程内语义。
func newTestClient(t *testing.T, dim int) *Client {
	t.Helper()
	c, err := New(Config{Dimensions: dim})
	require.NoError(t, err)
	return c
}

// rebuildIndex re-derives byID from docs after a test seeds docs directly.
func (c *Client) rebuildIndex() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID = make(map[int64]int, len(c.docs))
	for i, d := range c.docs {
		c.byID[d.ID] = i
	}
}

// TestSearchReturnsHitsForAllowedTier verifies the full Hit projection:
// ID / Score / Tier / Content / Source / EffectiveFrom / EffectiveTo must all
// round-trip so a citation can name the chunk AND the policy version.
func TestSearchReturnsHitsForAllowedTier(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{
		{ID: 1, Tier: "authority", Content: "差旅报销标准", Source: "authority/travel.md",
			Embedding: []float32{1, 0, 0}, EffectiveFrom: 100, EffectiveTo: 0},
		{ID: 2, Tier: "authority", Content: "another", Source: "authority/other.md",
			Embedding: []float32{0, 1, 0}, EffectiveFrom: 0, EffectiveTo: 9_999},
	}
	c.rebuildIndex()

	hits, err := c.Search(context.Background(), []float32{1, 0, 0}, []string{"authority"}, 5, 0)
	require.NoError(t, err)
	require.Len(t, hits, 2)

	assert.Equal(t, int64(1), hits[0].ID)
	assert.InDelta(t, float32(1), hits[0].Score, 1e-6)
	assert.Equal(t, "authority", hits[0].Tier)
	assert.Equal(t, "差旅报销标准", hits[0].Content)
	assert.Equal(t, "authority/travel.md", hits[0].Source)
	assert.Equal(t, int64(100), hits[0].EffectiveFrom)
	assert.Equal(t, int64(0), hits[0].EffectiveTo)
	assert.Less(t, hits[1].Score, hits[0].Score)
}

// TestSearchTopKAndQueryVecGuards covers the three cheap guards plus the
// dimension guard that keeps vecmath.Cosine from panicking.
func TestSearchTopKAndQueryVecGuards(t *testing.T) {
	c := newTestClient(t, 3)
	ctx := context.Background()

	hits, err := c.Search(ctx, vec(3, 1), []string{"authority"}, 0, 0)
	assert.Error(t, err)
	assert.Nil(t, hits)
	assert.Contains(t, err.Error(), "topK must be > 0")

	hits, err = c.Search(ctx, nil, []string{"authority"}, 5, 0)
	assert.Error(t, err)
	assert.Nil(t, hits)
	assert.Contains(t, err.Error(), "queryVec is empty")

	// 维度不符：必须返回错误而不是让 Cosine panic。
	var dimHits []retriever.Hit
	var dimErr error
	assert.NotPanics(t, func() {
		dimHits, dimErr = c.Search(ctx, vec(2, 1), []string{"authority"}, 5, 0)
	})
	require.Error(t, dimErr)
	assert.Nil(t, dimHits)
	assert.Contains(t, dimErr.Error(), "query vector dim 2 != configured dim 3")
}

// TestGetChunkReturnsErrChunkNotFound: the sentinel must be the SAME variable
// (internal/server matches it with errors.Is to map 404).
func TestGetChunkReturnsErrChunkNotFound(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{{ID: 5, Tier: "authority", Content: "body", Source: "authority/a.md"}}
	c.rebuildIndex()

	chunk, err := c.GetChunk(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), chunk.ID)
	assert.Equal(t, "body", chunk.Content)

	chunk, err = c.GetChunk(context.Background(), 404)
	assert.True(t, errors.Is(err, retriever.ErrChunkNotFound),
		"GetChunk must return the retriever.ErrChunkNotFound sentinel, got %v", err)
	assert.Equal(t, retriever.ChunkContent{}, chunk)
}

// TestGetMaxIDEmptyTierAndUnknownTierReturnZero: empty tier and unknown tier
// both return (0, nil) — never an error (retriever.go:209-212).
func TestGetMaxIDEmptyTierAndUnknownTierReturnZero(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{
		{ID: 3, Tier: "authority"},
		{ID: 8, Tier: "authority"},
		{ID: 12, Tier: "sql-guides"},
	}
	c.rebuildIndex()
	ctx := context.Background()

	id, err := c.GetMaxID(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id)

	id, err = c.GetMaxID(ctx, "does-not-exist")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id)

	id, err = c.GetMaxID(ctx, "authority")
	assert.NoError(t, err)
	assert.Equal(t, int64(8), id)

	id, err = c.GetMaxID(ctx, "sql-guides")
	assert.NoError(t, err)
	assert.Equal(t, int64(12), id)
}

// TestListMetadataIgnoresTierAndEffectiveWindow: the graph browse surface must
// expose EVERY row — including excluded tiers and expired effective_to — so
// superseded chunks keep their "已失效" badge instead of vanishing.
func TestListMetadataIgnoresTierAndEffectiveWindow(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{
		{ID: 9, Tier: "draft", Source: "draft/x.md", EffectiveFrom: 1, EffectiveTo: 2},
		{ID: 2, Tier: "historical", Source: "historical/y.md", EffectiveTo: 500},
		{ID: 5, Tier: "authority", Source: "authority/z.md"},
	}
	c.rebuildIndex()

	metas, err := c.ListMetadata(context.Background())
	require.NoError(t, err)
	require.Len(t, metas, 3, "ListMetadata must not filter by tier or effective window")

	// 输出按 ID 升序（确定性）。
	assert.Equal(t, []int64{2, 5, 9}, []int64{metas[0].ID, metas[1].ID, metas[2].ID})
	assert.Equal(t, "draft", metas[2].Tier)
	assert.Equal(t, int64(2), metas[2].EffectiveTo)
}

// TestEnsureIsIdempotent: three consecutive calls return nil and Health stays
// nil. Ensure must not accumulate state.
func TestEnsureIsIdempotent(t *testing.T) {
	c := newTestClient(t, 3)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		assert.NoError(t, c.Ensure(ctx), "Ensure call %d must be nil", i)
	}
	assert.NoError(t, c.Health(ctx))
}

// TestDisconnectMakesClientUnusable: after Disconnect every state-touching
// method returns ErrDisconnected, and a second Disconnect is still nil.
func TestDisconnectMakesClientUnusable(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{{ID: 1, Tier: "authority", Embedding: vec(3, 1)}}
	c.rebuildIndex()
	require.NoError(t, c.Disconnect())
	assert.NoError(t, c.Disconnect(), "Disconnect must be idempotent")

	ctx := context.Background()
	_, err := c.Search(ctx, vec(3, 1), []string{"authority"}, 5, 0)
	assert.True(t, errors.Is(err, ErrDisconnected))

	err = c.Insert(ctx, []retriever.Document{{ID: 2, Tier: "authority", Embedding: vec(3, 1)}})
	assert.True(t, errors.Is(err, ErrDisconnected))

	_, err = c.ListMetadata(ctx)
	assert.True(t, errors.Is(err, ErrDisconnected))

	_, err = c.GetChunk(ctx, 1)
	assert.True(t, errors.Is(err, ErrDisconnected))

	_, err = c.GetMaxID(ctx, "authority")
	assert.True(t, errors.Is(err, ErrDisconnected))

	assert.True(t, errors.Is(c.Health(ctx), ErrDisconnected))
}

// TestInsertRejectsDimMismatchAndDuplicateID: neither failure may mutate the
// store (no half-applied batch).
func TestInsertRejectsDimMismatchAndDuplicateID(t *testing.T) {
	c := newTestClient(t, 3)
	ctx := context.Background()

	require.NoError(t, c.Insert(ctx, []retriever.Document{
		{ID: 1, Tier: "authority", Content: "one", Embedding: vec(3, 1)},
	}))
	before, err := c.ListMetadata(ctx)
	require.NoError(t, err)
	require.Len(t, before, 1)

	err = c.Insert(ctx, []retriever.Document{
		{ID: 2, Tier: "authority", Embedding: vec(2, 1)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding dim 2 != configured dim 3")

	err = c.Insert(ctx, []retriever.Document{
		{ID: 1, Tier: "authority", Embedding: vec(3, 1)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id 1")

	err = c.Insert(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no docs to insert")

	after, err := c.ListMetadata(ctx)
	require.NoError(t, err)
	assert.Len(t, after, 1, "rejected inserts must not mutate the store")
}

// TestHealthReflectsInternalState locks "Health is not a constant nil": an
// index inconsistency must surface as an error.
func TestHealthReflectsInternalState(t *testing.T) {
	c := newTestClient(t, 3)
	ctx := context.Background()
	assert.NoError(t, c.Health(ctx))

	c.docs = []row{{ID: 1, Tier: "authority"}, {ID: 2, Tier: "authority"}}
	c.mu.Lock()
	c.byID = map[int64]int{1: 0} // 手工制造失配
	c.mu.Unlock()

	err := c.Health(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal index inconsistency")
}

// TestNewRejectsBadDimensions: startup dimension self-check (Pitfall 7) —
// New refuses a non-positive dimension and an embedder/dimension mismatch.
func TestNewRejectsBadDimensions(t *testing.T) {
	_, err := New(Config{Dimensions: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Dimensions must be > 0")

	_, err = New(Config{Dimensions: 3, Embedder: &fakeEmbedder{dim: 4}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedder dimensions 4 != configured dimensions 3")

	// 一致时通过。
	c, err := New(Config{Dimensions: 3, Embedder: &fakeEmbedder{dim: 3}})
	require.NoError(t, err)
	assert.NoError(t, c.Health(context.Background()))
}

// fakeEmbedder is a TEST-ONLY embedder stub: fixed-dimension deterministic
// vectors, zero network. 生产代码零替身分支（D-01）—— 它只存在于 _test.go，
// 且从不被 production 路径引用。
type fakeEmbedder struct {
	dim int
	// failAfter > 0 makes Embed fail on call number failAfter+1 (1-based),
	// used by the seed failure test.
	failAfter int
	calls     int
}

func (f *fakeEmbedder) Dimensions() int { return f.dim }

func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	f.calls++
	if f.failAfter > 0 && f.calls > f.failAfter {
		return nil, errors.New("fake embedder: endpoint unavailable (test)")
	}
	v := make([]float32, f.dim)
	if f.dim > 0 {
		// 与文本相关（首字节做归一化），保证不同文档得到不同但可复现的向量。
		var h uint32
		for i := 0; i < len(text); i++ {
			h = h*31 + uint32(text[i])
		}
		v[0] = float32(h%1000)/1000 + 0.001
		v[f.dim-1] = 0.001
	}
	return v, nil
}
