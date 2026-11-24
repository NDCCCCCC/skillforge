// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 58-04 (PROV-03) -- the channel-distribution frame pins.
//
// The frame's whole purpose is that a refusal can say WHAT was searched.
// These tests therefore check the two properties that make it honest:
//   A. counts come from the FUSED set (a dual-channel hit is ONE hybrid,
//      never vector+graph double-counted);
//   B. the degraded flag and KG stats travel with it, so "KG missing" is
//      distinguishable from "KG found nothing".
package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ontology/anchor"
	"github.com/skillforge/skillforge/internal/retriever"
)

func TestBuildRetrievalStatsFromFusedSet(t *testing.T) {
	fused := []retriever.HybridResult{
		{ID: 1, Origin: retriever.ChannelVector},
		{ID: 2, Origin: retriever.ChannelVector},
		{ID: 3, Origin: retriever.ChannelGraph},
		{ID: 4, Origin: retriever.ChannelHybrid},
	}
	kgc := KGContext{
		Available:       true,
		EntitiesMatched: 2,
		Anchors:         nil,
	}
	stats := buildRetrievalStats(fused, nil, kgc)
	assert.Equal(t, 2, stats.Channels.Vector)
	assert.Equal(t, 1, stats.Channels.Graph)
	assert.Equal(t, 1, stats.Channels.Hybrid,
		"a dual-channel hit is ONE hybrid, not one vector + one graph")
	assert.False(t, stats.Degraded)
	require.NotNil(t, stats.KG)
	assert.True(t, stats.KG.Available)
	assert.Equal(t, 2, stats.KG.EntitiesMatched)
}

func TestBuildRetrievalStatsVectorOnlyPath(t *testing.T) {
	chunks := []retriever.Hit{{ID: 1}, {ID: 2}, {ID: 3}}
	kgc := KGContext{Available: false}
	stats := buildRetrievalStats(nil, chunks, kgc)
	assert.Equal(t, 3, stats.Channels.Vector)
	assert.Equal(t, 0, stats.Channels.Graph)
	assert.Equal(t, 0, stats.Channels.Hybrid)
	// No KG == degraded, and the frame SAYS so.
	assert.True(t, stats.Degraded)
	require.NotNil(t, stats.KG)
	assert.False(t, stats.KG.Available)
}

func TestBuildRetrievalStatsCarriesAnchorLabels(t *testing.T) {
	kgc := KGContext{
		Available:       true,
		EntitiesMatched: 1,
		Anchors: []anchor.Match{
			{Label: "差旅住宿费标准"},
		},
	}
	stats := buildRetrievalStats(nil, []retriever.Hit{{ID: 1}}, kgc)
	require.Len(t, stats.AnchorEntities, 1)
	assert.Equal(t, "差旅住宿费标准", stats.AnchorEntities[0])
}
