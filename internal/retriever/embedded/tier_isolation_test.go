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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTierAllowedEmptyIsDeny locks D-09 at the smallest unit: nil and empty
// slices both deny. This mirrors internal/retriever/milvus/filter.go:64-84
// (empty tier ⇒ literal "false") and mongo's {tier: {$in: []}}.
func TestTierAllowedEmptyIsDeny(t *testing.T) {
	assert.False(t, tierAllowed(nil, "authority"))
	assert.False(t, tierAllowed([]string{}, "authority"))
	assert.False(t, tierAllowed(nil, ""))
}

// TestTierAllowedExactMatchOnly: matching is exact and case-sensitive — a
// pseudo slug like "Authority" must not slip into the trust set.
func TestTierAllowedExactMatchOnly(t *testing.T) {
	tiers := []string{"authority", "sql-guides"}
	assert.True(t, tierAllowed(tiers, "authority"))
	assert.True(t, tierAllowed(tiers, "sql-guides"))
	assert.False(t, tierAllowed(tiers, "Authority"))
	assert.False(t, tierAllowed(tiers, "historical"))
	assert.False(t, tierAllowed(tiers, "authorit")) // prefix must not match
}

// TestCandidatesEmptyTiersYieldsNil: the empty-tiers early return must
// produce ZERO candidates (not "all rows"). This is the structural
// refusal-by-construction the whole tier boundary rests on.
func TestCandidatesEmptyTiersYieldsNil(t *testing.T) {
	docs := []row{
		{ID: 1, Tier: "authority"},
		{ID: 2, Tier: "historical"},
		{ID: 3, Tier: "sql-guides"},
	}
	assert.Len(t, candidates(docs, nil, 0), 0)
	assert.Len(t, candidates(docs, []string{}, 0), 0)
}

// TestCandidatesExcludesNonWhitelistedTier: only whitelisted tiers survive,
// and the surviving row keeps its identity.
func TestCandidatesExcludesNonWhitelistedTier(t *testing.T) {
	docs := []row{
		{ID: 1, Tier: "authority"},
		{ID: 2, Tier: "historical"},
		{ID: 3, Tier: "draft"},
	}
	got := candidates(docs, []string{"authority"}, 100)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].ID)
	assert.Equal(t, "authority", got[0].Tier)
}

// TestCandidatesExcludesExpiredEffectiveTo: sentinel 0 and effective_to == now
// both pass; effective_to < now is excluded. The boundary equality is
// load-bearing (mongo uses $gte: now; dropping it would hide a policy one
// second early).
func TestCandidatesExcludesExpiredEffectiveTo(t *testing.T) {
	const now int64 = 1_000
	docs := []row{
		{ID: 1, Tier: "authority", EffectiveTo: 0},      // perpetual
		{ID: 2, Tier: "authority", EffectiveTo: now},    // valid on the boundary
		{ID: 3, Tier: "authority", EffectiveTo: now + 1}, // future
		{ID: 4, Tier: "authority", EffectiveTo: now - 1}, // expired
	}
	got := candidates(docs, []string{"authority"}, now)
	require.Len(t, got, 3)
	assert.Equal(t, []int64{1, 2, 3}, []int64{got[0].ID, got[1].ID, got[2].ID})
}

// TestSearchEmptyTiersReturnsZeroHitsNoError: the public Search surface must
// return 0 hits and a nil error for empty tiers — NOT an error and NOT
// "everything". Same semantics as milvus's literal false and mongo's
// {$in: []}.
func TestSearchEmptyTiersReturnsZeroHitsNoError(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{
		{ID: 1, Tier: "authority", Content: "a", Embedding: vec(3, 1)},
		{ID: 2, Tier: "historical", Content: "b", Embedding: vec(3, 1)},
		{ID: 3, Tier: "sql-guides", Content: "c", Embedding: vec(3, 1)},
	}
	c.rebuildIndex()

	for _, tiers := range [][]string{nil, {}} {
		hits, err := c.Search(context.Background(), vec(3, 1), tiers, 5, 0)
		assert.NoError(t, err)
		assert.Len(t, hits, 0, "empty tiers must yield zero hits (refusal-by-construction)")
	}
}

// TestSearchNeverScoresExcludedTiers is THE structural proof that filtering
// happens BEFORE scoring (49-RESEARCH Pitfall 11).
//
// 构造方式：被禁 tier（historical）的行与 query 完全同向 ⇒ 它的 cosine 是
// 1.0，若实现退化成"全量打分后再 filter"，它会以最高分出现（哪怕最后被
// 过滤掉，只要过滤发生在打分之后，这条断言就能从"结果集"上看出差别：
// 若有人把过滤写成软过滤/后置过滤并保留前 k 截断，被禁行会挤掉合法行）。
// 这里同时断言结果集只含白名单 tier，且合法行没有被挤掉。
func TestSearchNeverScoresExcludedTiers(t *testing.T) {
	c := newTestClient(t, 3)
	// 被禁行与 query 完全同向（cos = 1.0），合法行稍微偏一点（cos < 1）。
	c.docs = []row{
		{ID: 1, Tier: "historical", Content: "forbidden", Embedding: []float32{1, 0, 0}},
		{ID: 2, Tier: "authority", Content: "allowed", Embedding: []float32{0.9, 0.1, 0}},
	}
	c.rebuildIndex()

	hits, err := c.Search(context.Background(), []float32{1, 0, 0}, []string{"authority"}, 1, 0)
	require.NoError(t, err)
	require.Len(t, hits, 1, "top-1 must be filled by the authorized row, not by a post-filtered forbidden one")
	assert.Equal(t, int64(2), hits[0].ID)
	assert.Equal(t, "authority", hits[0].Tier)
	assert.NotEqual(t, "forbidden", hits[0].Content)
}

// TestSearchCandidateSetEmptySkipsRank: when every row is excluded, Search
// returns (nil, nil) — rank is never entered (k=0 there returns nil anyway,
// but the contract is that the candidate set gate fires first).
func TestSearchCandidateSetEmptySkipsRank(t *testing.T) {
	c := newTestClient(t, 3)
	c.docs = []row{
		{ID: 1, Tier: "historical", Embedding: vec(3, 1)},
		{ID: 2, Tier: "authority", Embedding: vec(3, 1), EffectiveTo: 500},
	}
	c.rebuildIndex()

	hits, err := c.Search(context.Background(), vec(3, 1), []string{"authority"}, 5, 1_000)
	assert.NoError(t, err)
	assert.Len(t, hits, 0)
}
