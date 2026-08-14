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

package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// stubMetaRetriever is the test double for GraphBuilder. The retriever
// seam is read-only here (no Insert / Search), so the other Retriever
// methods are stubs returning zero values.
type stubMetaRetriever struct {
	rows     []retriever.DocumentMeta
	err      error
	chunk    retriever.ChunkContent
	chunkErr error
}

func (s *stubMetaRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return s.rows, s.err
}

// GetChunk returns the configured chunk / chunkErr. Plan 260813-dno —
// exercised by the GetChunk tests below; the existing graph Build tests
// leave both fields zero-valued (GetChunk is never called on the Build path).
func (s *stubMetaRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return s.chunk, s.chunkErr
}

func (s *stubMetaRetriever) Health(_ context.Context) error { return nil }
func (s *stubMetaRetriever) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (s *stubMetaRetriever) Ensure(_ context.Context) error                         { return nil }
func (s *stubMetaRetriever) Insert(_ context.Context, _ []retriever.Document) error { return nil }
func (s *stubMetaRetriever) Disconnect() error                                      { return nil }

// Compile-time check that *stubMetaRetriever satisfies the full
// retriever.Retriever interface — required by knowledge.New.
var _ retriever.Retriever = (*stubMetaRetriever)(nil)

// fixtureRows returns the canonical corpus-shaped metadata for tests:
// one doc per tier, two chunks in 权威, plus effective_* windows so the
// version-label path is exercised.
func fixtureRows() []retriever.DocumentMeta {
	const (
		jan2026 int64 = 1735689600 // 2025-01-01 UTC (use distinct value from formatDate path)
		jul2025 int64 = 1720137600 // 2025-07-05 UTC
	)
	return []retriever.DocumentMeta{
		{ID: 1, Tier: "权威", Source: "authority/contract-approval.md",
			EffectiveFrom: jan2026, EffectiveTo: 0},
		{ID: 2, Tier: "权威", Source: "authority/travel-expense.md",
			EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 3, Tier: "历史", Source: "historical/old-policy-2023.md",
			EffectiveFrom: 0, EffectiveTo: jul2025},
		{ID: 4, Tier: "个人", Source: "personal/dept-note.md",
			EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 5, Tier: "草稿", Source: "draft/draft-policy.md",
			EffectiveFrom: 0, EffectiveTo: 0},
	}
}

func TestGraphBuilder_TierStructure(t *testing.T) {
	g, err := New(&stubMetaRetriever{rows: fixtureRows()}).Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, g)

	// Always 4 tiers emitted (权威, 草稿, 历史, 个人) — deterministic
	// order per skill.TierEnumValues().
	require.Len(t, g.Tiers, 4)
	assert.Equal(t, "权威", g.Tiers[0].Tier)
	assert.Equal(t, "草稿", g.Tiers[1].Tier)
	assert.Equal(t, "历史", g.Tiers[2].Tier)
	assert.Equal(t, "个人", g.Tiers[3].Tier)

	// 权威 has 2 docs (contract-approval + travel-expense) + 2 chunks.
	权威 := g.Tiers[0]
	assert.Equal(t, 2, 权威.DocCount)
	assert.Equal(t, 2, 权威.ChunkCount)
	require.Len(t, 权威.Docs, 2)
	// Sort is alphabetical by source path — contract-approval comes
	// before travel-expense.
	assert.Equal(t, "contract-approval.md", 权威.Docs[0].Filename)
	assert.Equal(t, "travel-expense.md", 权威.Docs[1].Filename)

	// Each of 历史 / 个人 / 草稿 has exactly 1 doc + 1 chunk.
	for _, t0 := range []string{"历史", "个人", "草稿"} {
		var n *TierNode
		for i := range g.Tiers {
			if g.Tiers[i].Tier == t0 {
				n = &g.Tiers[i]
				break
			}
		}
		require.NotNil(t, n, "missing tier %s", t0)
		assert.Equal(t, 1, n.DocCount)
		assert.Equal(t, 1, n.ChunkCount)
		require.Len(t, n.Docs, 1)
	}
}

func TestGraphBuilder_EffectiveFields(t *testing.T) {
	g, err := New(&stubMetaRetriever{rows: fixtureRows()}).Build(context.Background())
	require.NoError(t, err)

	// Find 权威 → contract-approval → chunk with effective_from populated.
	var chunk ChunkNode
	var found bool
	for _, t0 := range g.Tiers {
		if t0.Tier != "权威" {
			continue
		}
		for _, d := range t0.Docs {
			if d.Filename != "contract-approval.md" {
				continue
			}
			require.Len(t, d.Chunks, 1)
			chunk = d.Chunks[0]
			found = true
		}
	}
	require.True(t, found, "missing contract-approval chunk")
	assert.Equal(t, int64(1735689600), chunk.EffectiveFrom, "effectiveFrom round-trips")
	assert.Equal(t, int64(0), chunk.EffectiveTo, "effectiveTo=0 means perpetual / open-ended")
	// Version label format: "自 <YYYY-MM-DD>" when from>0 and to==0.
	assert.True(t, strings.HasPrefix(chunk.Version, "自 "),
		"expected version label with '自 ' prefix, got %q", chunk.Version)

	// 历史 → old-policy → chunk with effective_to populated, from=0.
	for _, t0 := range g.Tiers {
		if t0.Tier != "历史" {
			continue
		}
		require.Len(t, t0.Docs, 1)
		require.Len(t, t0.Docs[0].Chunks, 1)
		历史chunk := t0.Docs[0].Chunks[0]
		assert.Equal(t, int64(0), 历史chunk.EffectiveFrom)
		assert.Equal(t, int64(1720137600), 历史chunk.EffectiveTo)
		assert.True(t, strings.HasSuffix(历史chunk.Version, "起失效"),
			"expected version label with '起失效' suffix, got %q", 历史chunk.Version)
	}
}

func TestGraphBuilder_Empty(t *testing.T) {
	g, err := New(&stubMetaRetriever{rows: nil}).Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, g)

	// 4 tier entries, all with 0 docs.
	require.Len(t, g.Tiers, 4)
	for _, t0 := range g.Tiers {
		assert.Equal(t, 0, t0.DocCount)
		assert.Equal(t, 0, t0.ChunkCount)
		assert.Empty(t, t0.Docs)
	}
}

func TestGraphBuilder_DeterministicOrder(t *testing.T) {
	// Two builds with the same rows must produce identical output
	// (regression for FE stable rendering / snapshot tests).
	g1, err := New(&stubMetaRetriever{rows: fixtureRows()}).Build(context.Background())
	require.NoError(t, err)
	g2, err := New(&stubMetaRetriever{rows: fixtureRows()}).Build(context.Background())
	require.NoError(t, err)

	for i := range g1.Tiers {
		assert.Equal(t, g1.Tiers[i].Tier, g2.Tiers[i].Tier)
		assert.Equal(t, len(g1.Tiers[i].Docs), len(g2.Tiers[i].Docs))
		for j := range g1.Tiers[i].Docs {
			assert.Equal(t, g1.Tiers[i].Docs[j].Filename, g2.Tiers[i].Docs[j].Filename)
			assert.Equal(t, g1.Tiers[i].Docs[j].Chunks, g2.Tiers[i].Docs[j].Chunks)
		}
	}
}

func TestGraphBuilder_NilRetriever(t *testing.T) {
	_, err := New(nil).Build(context.Background())
	assert.ErrorIs(t, err, ErrNoRetriever)
}

func TestGraphBuilder_RetrieverErrorPropagates(t *testing.T) {
	want := errors.New("list metadata blew up")
	_, err := New(&stubMetaRetriever{err: want}).Build(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}

func TestGraphBuilder_RejectsUnknownTier(t *testing.T) {
	bad := []retriever.DocumentMeta{
		{ID: 1, Tier: "权威", Source: "authority/a.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 2, Tier: "SecretTier", Source: "secret/x.md", EffectiveFrom: 0, EffectiveTo: 0},
	}
	_, err := New(&stubMetaRetriever{rows: bad}).Build(context.Background())
	require.Error(t, err)
	var inv ErrInvalidTier
	require.ErrorAs(t, err, &inv)
	assert.Equal(t, "SecretTier", inv.Tier)
}

func TestGraphBuilder_RejectsEmptySource(t *testing.T) {
	bad := []retriever.DocumentMeta{
		{ID: 1, Tier: "权威", Source: "", EffectiveFrom: 0, EffectiveTo: 0},
	}
	_, err := New(&stubMetaRetriever{rows: bad}).Build(context.Background())
	require.Error(t, err)
	var empty ErrEmptySource
	require.ErrorAs(t, err, &empty)
}

func TestGraphBuilder_GroupsBySource(t *testing.T) {
	// Two chunks sharing the same source path must collapse into one
	// DocNode (otherwise the FE would render the same .md twice).
	rows := []retriever.DocumentMeta{
		{ID: 10, Tier: "权威", Source: "authority/x.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 11, Tier: "权威", Source: "authority/x.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 12, Tier: "权威", Source: "authority/y.md", EffectiveFrom: 0, EffectiveTo: 0},
	}
	g, err := New(&stubMetaRetriever{rows: rows}).Build(context.Background())
	require.NoError(t, err)
	require.Len(t, g.Tiers, 4)
	权威 := g.Tiers[0]
	assert.Equal(t, 2, 权威.DocCount)
	assert.Equal(t, 3, 权威.ChunkCount)

	// Find x.md and confirm it carries both chunk IDs.
	var xDoc *DocNode
	for i := range 权威.Docs {
		if 权威.Docs[i].Filename == "x.md" {
			xDoc = &权威.Docs[i]
			break
		}
	}
	require.NotNil(t, xDoc)
	assert.Equal(t, 2, xDoc.ChunkCount)
	require.Len(t, xDoc.Chunks, 2)
	assert.Equal(t, "10", xDoc.Chunks[0].ID, "chunks sort by ID ascending")
	assert.Equal(t, "11", xDoc.Chunks[1].ID)
}

// Plan 260813-dno — GetChunk tests (the on-demand chunk-content fetch).

func TestGraphBuilder_GetChunk_Found(t *testing.T) {
	const (
		jan2026 int64 = 1735689600 // 2025-01-01 UTC
	)
	chunk := retriever.ChunkContent{
		ID:            7,
		Tier:          "权威",
		Source:        "authority/contract-approval.md",
		Content:       "出差住宿费超过 500 元须审批。",
		EffectiveFrom: jan2026,
		EffectiveTo:   0,
	}
	b := New(&stubMetaRetriever{chunk: chunk})

	got, err := b.GetChunk(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, got)

	// ID is rendered via the same int64ToChunkID the graph node uses.
	assert.Equal(t, "7", got.ID, "ID uses int64ToChunkID (byte-identical to graph node label)")
	assert.Equal(t, "权威", got.Tier)
	assert.Equal(t, "authority/contract-approval.md", got.Source)
	assert.Equal(t, "出差住宿费超过 500 元须审批。", got.Content)
	assert.Equal(t, jan2026, got.EffectiveFrom)
	assert.Equal(t, int64(0), got.EffectiveTo)
	// Version is derived from versionFromEffective — same label the graph node carries.
	expectedVer := versionFromEffective(jan2026, 0)
	assert.Equal(t, expectedVer, got.Version)
	assert.True(t, strings.HasPrefix(got.Version, "自 "), "version label mirrors graph node for {from>0,to==0}")
}

func TestGraphBuilder_GetChunk_NotFound(t *testing.T) {
	b := New(&stubMetaRetriever{chunkErr: retriever.ErrChunkNotFound})
	_, err := b.GetChunk(context.Background(), 999)
	require.Error(t, err)
	// retriever.ErrChunkNotFound is mapped to the layer-local sentinel.
	assert.ErrorIs(t, err, ErrChunkNotFound)
	// And is NOT the raw retriever error leaking across the package boundary.
	assert.NotErrorIs(t, err, retriever.ErrChunkNotFound)
}

func TestGraphBuilder_GetChunk_NilRetriever(t *testing.T) {
	b := New(nil)
	_, err := b.GetChunk(context.Background(), 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRetriever)
}

func TestGraphBuilder_GetChunk_PropagatesRetrieverError(t *testing.T) {
	boom := errors.New("boom: backend down")
	b := New(&stubMetaRetriever{chunkErr: boom})
	_, err := b.GetChunk(context.Background(), 1)
	require.Error(t, err)
	// The raw error propagates verbatim (server maps it to 500).
	assert.ErrorIs(t, err, boom)
	// And it is NOT the not-found sentinel (distinct code path).
	assert.NotErrorIs(t, err, ErrChunkNotFound)
}
