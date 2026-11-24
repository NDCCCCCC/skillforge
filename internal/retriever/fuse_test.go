// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 58-02 (PROV-01/PROV-02) -- fusion pins. The two rules under test
// are the ones that make provenance trustworthy:
//
//   A. aggregation key is the chunk PRIMARY KEY (same text in two docs
//      must NOT merge -- a text-keyed fuse would fabricate provenance);
//   B. dual-channel hits are hybrid and keep the RAW channel set;
//   C. ordering is RRF-based and deterministic;
//   D. graph-only hits stay graph (they are context candidates, and the
//      caller must not promote them without a vector relevance check).
package retriever

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuseChannelsAggregatesByChunkPKNotText(t *testing.T) {
	shared := "shentong tiaokuan wenben"
	vectorHits := []Hit{
		{ID: 1, Content: shared, Score: 0.9},
		{ID: 2, Content: shared, Score: 0.8},
	}
	out := FuseChannels(vectorHits, nil)
	require.Len(t, out, 2, "same text + different PK == two rows (a text key would merge them)")
	assert.Equal(t, int64(1), out[0].ID)
	assert.Equal(t, int64(2), out[1].ID)
	assert.Equal(t, ChannelVector, out[0].Origin)
}

func TestFuseChannelsMarksDualChannelAsHybridAndKeepsRawSet(t *testing.T) {
	vectorHits := []Hit{{ID: 7, Content: "content", Score: 0.7}}
	graphHits := []Hit{{ID: 7, ViaEntity: "http://skillforge.local/ontology#tbl_travel", ViaRels: []string{"referencesTable"}}}

	out := FuseChannels(vectorHits, graphHits)
	require.Len(t, out, 1)
	r := out[0]
	assert.Equal(t, ChannelHybrid, r.Origin)
	assert.Equal(t, []string{ChannelVector, ChannelGraph}, r.Channels,
		"the raw channel set must survive fusion (PROV-02)")
	assert.Equal(t, ChannelHybrid, r.Hit.Origin)
	assert.Equal(t, "http://skillforge.local/ontology#tbl_travel", r.Hit.ViaEntity)
	assert.Equal(t, []string{"referencesTable"}, r.Hit.ViaRels)
	assert.Equal(t, "content", r.Hit.Content)
}

func TestFuseChannelsKeepsGraphOnlyAsGraph(t *testing.T) {
	out := FuseChannels(nil, []Hit{{ID: 99, ViaEntity: "e"}})
	require.Len(t, out, 1)
	assert.Equal(t, ChannelGraph, out[0].Origin)
	assert.Equal(t, []string{ChannelGraph}, out[0].Channels)
	assert.Empty(t, out[0].Hit.Content, "a graph-only row carries no vector content")
}

func TestFuseChannelsOrdersByRRFThenID(t *testing.T) {
	vectorHits := []Hit{
		{ID: 1, Content: "a", Score: 0.5},
		{ID: 2, Content: "b", Score: 0.99},
	}
	graphHits := []Hit{{ID: 1, ViaEntity: "e"}}

	out := FuseChannels(vectorHits, graphHits)
	require.Len(t, out, 2)
	assert.Equal(t, int64(1), out[0].ID, "RRF gives the dual-channel hit the top slot")
	assert.Equal(t, int64(2), out[1].ID)

	again := FuseChannels(vectorHits, graphHits)
	assert.Equal(t, out, again)
}

func TestFuseChannelsEmptyInputs(t *testing.T) {
	assert.Empty(t, FuseChannels(nil, nil))
	out := FuseChannels([]Hit{{ID: 3, Content: "c"}}, nil)
	require.Len(t, out, 1)
	assert.Equal(t, ChannelVector, out[0].Origin)
	assert.Equal(t, []string{ChannelVector}, out[0].Channels)
}

func TestRRFRankConstantIsThePublishedBaseline(t *testing.T) {
	assert.Equal(t, 60, RRFRankConstant,
		"RRF k is the industry baseline; a custom weight would be a hardcoded opinion")
}
