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

	"github.com/skillforge/skillforge/internal/kb"
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

// graphSlugRoles maps the baseline slugs to their registry roles.
var graphSlugRoles = map[string]kb.Role{
	"authority":  kb.RoleAuthority,
	"historical": kb.RoleHistorical,
	"personal":   kb.RolePersonal,
	"draft":      kb.RoleDraft,
}

// graphEntries builds registry entries from slugs, all active unless the
// slug is listed in deactivated.
func graphEntries(slugs []string, deactivated ...string) []kb.Entry {
	off := make(map[string]bool, len(deactivated))
	for _, s := range deactivated {
		off[s] = true
	}
	entries := make([]kb.Entry, 0, len(slugs))
	for _, slug := range slugs {
		entries = append(entries, kb.Entry{
			Slug:             slug,
			DisplayName:      slug,
			Role:             graphSlugRoles[slug],
			RetrievalEnabled: !off[slug],
		})
	}
	return entries
}

// graphRegistry builds a read-only registry over the given slugs (any slug
// outside graphSlugRoles must be given a role explicitly by extending the
// map in the test that needs it).
func graphRegistry(slugs []string, deactivated ...string) kb.Registry {
	return kb.New(graphEntries(slugs, deactivated...))
}

// fixtureRows returns the canonical corpus-shaped metadata for tests:
// one doc per baseline slug, two chunks in authority, plus effective_*
// windows so the version-label path is exercised. Tier values are
// registry slugs — the only values the graph's Lookup re-check accepts.
func fixtureRows() []retriever.DocumentMeta {
	const (
		jan2026 int64 = 1735689600 // 2025-01-01 UTC (use distinct value from formatDate path)
		jul2025 int64 = 1720137600 // 2025-07-05 UTC
	)
	return []retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "authority/contract-approval.md",
			EffectiveFrom: jan2026, EffectiveTo: 0},
		{ID: 2, Tier: "authority", Source: "authority/travel-expense.md",
			EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 3, Tier: "historical", Source: "historical/old-policy-2023.md",
			EffectiveFrom: 0, EffectiveTo: jul2025},
		{ID: 4, Tier: "personal", Source: "personal/dept-note.md",
			EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 5, Tier: "draft", Source: "draft/draft-policy.md",
			EffectiveFrom: 0, EffectiveTo: 0},
	}
}

// baselineGraphSlugs is the ascending slug order the node set takes when
// the whole baseline registry is active.
var baselineGraphSlugs = []string{"authority", "draft", "historical", "personal"}

func TestGraphBuilder_TierStructure(t *testing.T) {
	g, err := New(&stubMetaRetriever{rows: fixtureRows()}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, g)

	// One node per ACTIVE registry entry, ascending by slug — the fixed
	// 4-tier emission is gone; the order is the registry's.
	require.Len(t, g.Tiers, 4)
	for i, slug := range baselineGraphSlugs {
		assert.Equal(t, slug, g.Tiers[i].Tier, "node %d must be slug %q (ascending registry order)", i, slug)
	}

	// authority has 2 docs (contract-approval + travel-expense) + 2 chunks.
	authority := g.Tiers[0]
	assert.Equal(t, 2, authority.DocCount)
	assert.Equal(t, 2, authority.ChunkCount)
	require.Len(t, authority.Docs, 2)
	// Sort is alphabetical by source path — contract-approval comes
	// before travel-expense.
	assert.Equal(t, "contract-approval.md", authority.Docs[0].Filename)
	assert.Equal(t, "travel-expense.md", authority.Docs[1].Filename)

	// Each of draft / historical / personal has exactly 1 doc + 1 chunk.
	for _, slug := range []string{"draft", "historical", "personal"} {
		var n *TierNode
		for i := range g.Tiers {
			if g.Tiers[i].Tier == slug {
				n = &g.Tiers[i]
				break
			}
		}
		require.NotNilf(t, n, "missing node %s", slug)
		assert.Equal(t, 1, n.DocCount)
		assert.Equal(t, 1, n.ChunkCount)
		require.Len(t, n.Docs, 1)
	}
}

func TestGraphBuilder_EffectiveFields(t *testing.T) {
	g, err := New(&stubMetaRetriever{rows: fixtureRows()}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.NoError(t, err)

	// Find authority → contract-approval → chunk with effective_from populated.
	var chunk ChunkNode
	var found bool
	for _, t0 := range g.Tiers {
		if t0.Tier != "authority" {
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

	// historical → old-policy → chunk with effective_to populated, from=0.
	for _, t0 := range g.Tiers {
		if t0.Tier != "historical" {
			continue
		}
		require.Len(t, t0.Docs, 1)
		require.Len(t, t0.Docs[0].Chunks, 1)
		historicalChunk := t0.Docs[0].Chunks[0]
		assert.Equal(t, int64(0), historicalChunk.EffectiveFrom)
		assert.Equal(t, int64(1720137600), historicalChunk.EffectiveTo)
		assert.True(t, strings.HasSuffix(historicalChunk.Version, "起失效"),
			"expected version label with '起失效' suffix, got %q", historicalChunk.Version)
	}
}

func TestGraphBuilder_Empty(t *testing.T) {
	g, err := New(&stubMetaRetriever{rows: nil}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, g)

	// One node per active registry entry, all with 0 docs — the empty
	// collection does not shrink the registry's node set.
	require.Len(t, g.Tiers, 4)
	for _, t0 := range g.Tiers {
		assert.Equal(t, 0, t0.DocCount)
		assert.Equal(t, 0, t0.ChunkCount)
		assert.Empty(t, t0.Docs)
	}
}

// TestGraphNodesFromRegistry — Phase 26 (KBM-05): the node set is a pure
// function of the registry. Four active entries → four node groups;
// deactivating personal removes its node; registering a fifth base adds a
// fifth node group (N dynamic, not fixed at 4).
func TestGraphNodesFromRegistry(t *testing.T) {
	rowsFor := func(slugs ...string) []retriever.DocumentMeta {
		rows := make([]retriever.DocumentMeta, 0, len(slugs))
		for i, slug := range slugs {
			rows = append(rows, retriever.DocumentMeta{
				ID:     int64(i + 1),
				Tier:   slug,
				Source: slug + "/doc.md",
			})
		}
		return rows
	}

	t.Run("four_active_entries_four_nodes", func(t *testing.T) {
		g, err := New(&stubMetaRetriever{rows: rowsFor("authority", "historical", "personal", "draft")},
			graphRegistry(testBaselineSlugs)).Build(context.Background())
		require.NoError(t, err)
		got := make([]string, 0, len(g.Tiers))
		for _, n := range g.Tiers {
			got = append(got, n.Tier)
		}
		assert.Equal(t, baselineGraphSlugs, got)
	})

	t.Run("deactivated_personal_node_disappears", func(t *testing.T) {
		// Rows only for the still-active bases: a deactivated base must not
		// be a node, and a backend row still carrying its slug is rejected
		// by the re-check (locked separately in TestGraphBuilder_RejectsDeactivatedTier).
		g, err := New(&stubMetaRetriever{rows: rowsFor("authority", "historical")},
			graphRegistry(testBaselineSlugs, "personal")).Build(context.Background())
		require.NoError(t, err)
		got := make([]string, 0, len(g.Tiers))
		for _, n := range g.Tiers {
			got = append(got, n.Tier)
			assert.NotEqual(t, "personal", n.Tier, "a deactivated base must not be a node")
		}
		assert.NotContains(t, got, "personal")
	})

	t.Run("fifth_active_entry_adds_a_node", func(t *testing.T) {
		entries := append(graphEntries(testBaselineSlugs),
			kb.Entry{Slug: "it-policies", DisplayName: "IT Policies", Role: kb.RoleAuthority, RetrievalEnabled: true})
		g, err := New(&stubMetaRetriever{rows: rowsFor("authority", "historical", "personal", "draft", "it-policies")},
			kb.New(entries)).Build(context.Background())
		require.NoError(t, err)
		require.Len(t, g.Tiers, 5, "the node set is N-dynamic, not fixed at 4")
		// Ascending: authority < draft < historical < it-policies < personal.
		assert.Equal(t, "it-policies", g.Tiers[3].Tier, "new base slots into ascending slug order")
	})
}

// TestGraphDeterministicOrder: the node set is ascending by slug, and two
// builds over the same rows produce identical output (regression for FE
// stable rendering / snapshot tests).
func TestGraphDeterministicOrder(t *testing.T) {
	g1, err := New(&stubMetaRetriever{rows: fixtureRows()}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.NoError(t, err)
	g2, err := New(&stubMetaRetriever{rows: fixtureRows()}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.NoError(t, err)

	for i := 1; i < len(g1.Tiers); i++ {
		assert.Less(t, g1.Tiers[i-1].Tier, g1.Tiers[i].Tier,
			"nodes must be ascending by slug (replaces the old TierEnumValues order)")
	}
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
	_, err := New(nil, graphRegistry(testBaselineSlugs)).Build(context.Background())
	assert.ErrorIs(t, err, ErrNoRetriever)
}

// TestGraphNilRegistryFailsClosed — Phase 26 (KBM-05): a nil registry has
// no honest node set and no honest tier re-check, so Build fails closed.
func TestGraphNilRegistryFailsClosed(t *testing.T) {
	_, err := New(&stubMetaRetriever{rows: fixtureRows()}, nil).Build(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRegistry)
}

func TestGraphBuilder_RetrieverErrorPropagates(t *testing.T) {
	want := errors.New("list metadata blew up")
	_, err := New(&stubMetaRetriever{err: want}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}

// TestGraphInvalidTierListsActiveSlugs — the re-check's error names the
// active slug set (the same fail-closed message discipline as the indexer
// gate), so an operator seeing a stale backend row knows what IS valid.
func TestGraphInvalidTierListsActiveSlugs(t *testing.T) {
	bad := []retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "authority/a.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 2, Tier: "SecretTier", Source: "secret/x.md", EffectiveFrom: 0, EffectiveTo: 0},
	}
	_, err := New(&stubMetaRetriever{rows: bad}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.Error(t, err)
	var inv ErrInvalidTier
	require.ErrorAs(t, err, &inv)
	assert.Equal(t, "SecretTier", inv.Tier)
	msg := err.Error()
	assert.Contains(t, msg, "KNOW-02", "the re-check keeps citing the requirement")
	for _, slug := range testBaselineSlugs {
		assert.Containsf(t, msg, slug, "the error must list active slug %q", slug)
	}
}

// A deactivated slug in a backend row must be rejected by the re-check —
// deactivation removes a base from the graph too, not just from retrieval.
func TestGraphBuilder_RejectsDeactivatedTier(t *testing.T) {
	bad := []retriever.DocumentMeta{
		{ID: 1, Tier: "personal", Source: "personal/dept-note.md", EffectiveFrom: 0, EffectiveTo: 0},
	}
	_, err := New(&stubMetaRetriever{rows: bad}, graphRegistry(testBaselineSlugs, "personal")).Build(context.Background())
	require.Error(t, err)
	var inv ErrInvalidTier
	require.ErrorAs(t, err, &inv)
	assert.Equal(t, "personal", inv.Tier)
}

func TestGraphBuilder_RejectsEmptySource(t *testing.T) {
	bad := []retriever.DocumentMeta{
		{ID: 1, Tier: "authority", Source: "", EffectiveFrom: 0, EffectiveTo: 0},
	}
	_, err := New(&stubMetaRetriever{rows: bad}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.Error(t, err)
	var empty ErrEmptySource
	require.ErrorAs(t, err, &empty)
}

func TestGraphBuilder_GroupsBySource(t *testing.T) {
	// Two chunks sharing the same source path must collapse into one
	// DocNode (otherwise the FE would render the same .md twice).
	rows := []retriever.DocumentMeta{
		{ID: 10, Tier: "authority", Source: "authority/x.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 11, Tier: "authority", Source: "authority/x.md", EffectiveFrom: 0, EffectiveTo: 0},
		{ID: 12, Tier: "authority", Source: "authority/y.md", EffectiveFrom: 0, EffectiveTo: 0},
	}
	g, err := New(&stubMetaRetriever{rows: rows}, graphRegistry(testBaselineSlugs)).Build(context.Background())
	require.NoError(t, err)
	require.Len(t, g.Tiers, 4)
	authority := g.Tiers[0]
	assert.Equal(t, 2, authority.DocCount)
	assert.Equal(t, 3, authority.ChunkCount)

	// Find x.md and confirm it carries both chunk IDs.
	var xDoc *DocNode
	for i := range authority.Docs {
		if authority.Docs[i].Filename == "x.md" {
			xDoc = &authority.Docs[i]
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
		Tier:          "authority",
		Source:        "authority/contract-approval.md",
		Content:       "出差住宿费超过 500 元须审批。",
		EffectiveFrom: jan2026,
		EffectiveTo:   0,
	}
	b := New(&stubMetaRetriever{chunk: chunk}, graphRegistry(testBaselineSlugs))

	got, err := b.GetChunk(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, got)

	// ID is rendered via the same int64ToChunkID the graph node uses.
	assert.Equal(t, "7", got.ID, "ID uses int64ToChunkID (byte-identical to graph node label)")
	assert.Equal(t, "authority", got.Tier)
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
	b := New(&stubMetaRetriever{chunkErr: retriever.ErrChunkNotFound}, graphRegistry(testBaselineSlugs))
	_, err := b.GetChunk(context.Background(), 999)
	require.Error(t, err)
	// retriever.ErrChunkNotFound is mapped to the layer-local sentinel.
	assert.ErrorIs(t, err, ErrChunkNotFound)
	// And is NOT the raw retriever error leaking across the package boundary.
	assert.NotErrorIs(t, err, retriever.ErrChunkNotFound)
}

func TestGraphBuilder_GetChunk_NilRetriever(t *testing.T) {
	b := New(nil, graphRegistry(testBaselineSlugs))
	_, err := b.GetChunk(context.Background(), 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRetriever)
}

func TestGraphBuilder_GetChunk_PropagatesRetrieverError(t *testing.T) {
	boom := errors.New("boom: backend down")
	b := New(&stubMetaRetriever{chunkErr: boom}, graphRegistry(testBaselineSlugs))
	_, err := b.GetChunk(context.Background(), 1)
	require.Error(t, err)
	// The raw error propagates verbatim (server maps it to 500).
	assert.ErrorIs(t, err, boom)
	// And it is NOT the not-found sentinel (distinct code path).
	assert.NotErrorIs(t, err, ErrChunkNotFound)
}
