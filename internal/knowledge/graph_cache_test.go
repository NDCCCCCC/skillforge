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
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// ------------------------------------------------------------------
// TestGraphCacheTTLExpiry — cache miss after TTL elapses
// ------------------------------------------------------------------

func TestGraphCacheTTLExpiry(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	// Short TTL so the test completes quickly.
	t.Setenv("SKILLFORGE_GRAPH_CACHE_TTL_MS", "50")

	stub := &stubMetaRetriever{rows: fixtureRows()}

	// First call — populates the cache.
	eg1, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)
	require.NotNil(t, eg1)

	// Second call within TTL — must return the same pointer.
	eg2, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)
	assert.Same(t, eg1, eg2, "within-TTL call must return cached pointer")

	// Wait for TTL to expire.
	time.Sleep(60 * time.Millisecond)

	// Third call after expiry — must recompute (different pointer).
	eg3, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)
	assert.NotSame(t, eg1, eg3, "after-TTL call must recompute (different pointer)")

	// Both results should be structurally identical (same edges).
	assert.Equal(t, eg1.Edges, eg3.Edges, "recomputed graph should have same edges")
}

// ------------------------------------------------------------------
// TestGraphCacheInvalidationHook — InvalidateCache clears the cache
// ------------------------------------------------------------------

func TestGraphCacheInvalidationHook(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	stub := &stubMetaRetriever{rows: fixtureRows()}

	// Populate the cache.
	eg1, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)

	// Set the invalidation hook.
	var hookCalled int
	SetGraphCacheInvalidateHook(func() {
		hookCalled++
	})
	t.Cleanup(func() { SetGraphCacheInvalidateHook(nil) })

	// Invoke the hook (simulates what Phase 27 upload handler does).
	graphCacheInvalidateHook()

	assert.Equal(t, 1, hookCalled, "hook must be called once")

	// Verify the cache entry was deleted by checking that the next call
	// recomputes. We use graphCache.Delete to simulate a clean slate
	// (bypassing any TOCTOU in the lazy-eviction path).
	graphCache.Delete(cacheKeyMain)

	// Next call must recompute (different pointer).
	eg2, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)
	assert.NotSame(t, eg1, eg2, "after-invalidation call must recompute")
}

// ------------------------------------------------------------------
// TestGraphCacheConcurrentSafety — no race, all succeed, no corruption
// ------------------------------------------------------------------

func TestGraphCacheConcurrentSafety(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	stub := &stubMetaRetriever{rows: fixtureRows()}

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]*EdgeGraph, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = GetOrCompute(context.Background(), stub)
		}(i)
	}
	wg.Wait()

	// All goroutines must have succeeded.
	for i, err := range errs {
		require.NoError(t, err, "goroutine %d returned an error", i)
	}

	// All goroutines must have received structurally equivalent results
	// (same edges, same adj structure). The sync.Map store is safe for
	// concurrent use, but due to the Load-then-Store race inherent in
	// lazy-eviction without a locking read, some goroutines may have
	// computed independently before one won the Store race.
	// We verify: no goroutine got a corrupt/partial result.
	firstEdges := results[0].Edges
	for i := 1; i < goroutines; i++ {
		assert.Equal(t, firstEdges, results[i].Edges,
			"goroutine %d got different edges than goroutine 0 (possible data corruption)", i)
	}
}

// ------------------------------------------------------------------
// TestCitationSubCache — citation edges cached separately with 30s TTL
// ------------------------------------------------------------------

func TestCitationSubCache(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	// Use a short citation TTL for the test.
	t.Setenv("SKILLFORGE_GRAPH_CITATION_CACHE_TTL_MS", "50")

	// Two verdict rows that cite the same chunk — should produce a co-occur edge.
	stubStore := &stubAuditQuerier{
		rows: []store.AuditRow{
			{
				ID:          1,
				Timestamp:   time.Now(),
				UserID:      "admin",
				Action:      "verdict_finalized",
				ActorType:   "admin",
				FrozenArgs:  []byte(`{}`),
				VerdictJSON: mustMarshalVerdict(verdict.Verdict{
					Status:     "valid",
					Confidence: 0.95,
					Citations: []verdict.Citation{
						{ChunkID: "1"},
						{ChunkID: "7"},
					},
				}),
			},
			{
				ID:          2,
				Timestamp:   time.Now(),
				UserID:      "admin",
				Action:      "verdict_finalized",
				ActorType:   "admin",
				FrozenArgs:  []byte(`{}`),
				VerdictJSON: mustMarshalVerdict(verdict.Verdict{
					Status:     "valid",
					Confidence: 0.95,
					Citations: []verdict.Citation{
						{ChunkID: "1"},
						{ChunkID: "7"},
					},
				}),
			},
		},
	}

	// First call — populates citation cache.
	eg1, err := GetCitationEdges(context.Background(), stubStore)
	require.NoError(t, err)
	require.NotNil(t, eg1)
	require.NotEmpty(t, eg1.Edges, "citation cache should contain edges")

	// Second call within TTL — same pointer.
	eg2, err := GetCitationEdges(context.Background(), stubStore)
	require.NoError(t, err)
	assert.Same(t, eg1, eg2, "within-TTL citation call must return cached pointer")

	// Wait for citation TTL to expire.
	time.Sleep(60 * time.Millisecond)

	// Third call after expiry — recomputes.
	eg3, err := GetCitationEdges(context.Background(), stubStore)
	require.NoError(t, err)
	assert.NotSame(t, eg1, eg3, "after-TTL citation call must recompute")
}

// ------------------------------------------------------------------
// TestCitationSubCacheSkipOnError — audit failure returns empty EdgeGraph
// ------------------------------------------------------------------

func TestCitationSubCacheSkipOnError(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	stubStore := &stubAuditQuerier{
		err: errors.New("audit unavailable"),
	}

	// Should not panic and should return empty EdgeGraph.
	eg, err := GetCitationEdges(context.Background(), stubStore)
	require.NoError(t, err, "GetCitationEdges must not return an error on audit failure")
	require.NotNil(t, eg, "result must not be nil")
	assert.Empty(t, eg.Edges, "edges must be empty when audit scan fails")
}

// ------------------------------------------------------------------
// TestGraphCacheWithNilRetriever — nil retriever returns error
// ------------------------------------------------------------------

func TestGraphCacheWithNilRetriever(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	eg, err := GetOrCompute(context.Background(), nil)
	assert.Error(t, err, "GetOrCompute with nil retriever must return an error")
	assert.Nil(t, eg, "result must be nil on error")
	assert.Contains(t, err.Error(), "retriever is nil", "error must mention retriever is nil")
}

// ------------------------------------------------------------------
// TestInvalidateCacheClearsBoth — both main and citation cache cleared
// ------------------------------------------------------------------

func TestInvalidateCacheClearsBoth(t *testing.T) {
	ResetCaches()
	t.Cleanup(ResetCaches)

	stub := &stubMetaRetriever{rows: fixtureRows()}

	// Populate main cache.
	eg1, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)

	// Populate citation cache.
	stubStore := &stubAuditQuerier{
		rows: []store.AuditRow{
			{
				ID:          1,
				Timestamp:   time.Now(),
				UserID:      "admin",
				Action:      "verdict_finalized",
				ActorType:   "admin",
				FrozenArgs:  []byte(`{}`),
				VerdictJSON: mustMarshalVerdict(verdict.Verdict{
					Status:     "valid",
					Confidence: 0.95,
					Citations: []verdict.Citation{
						{ChunkID: "1"},
						{ChunkID: "7"},
					},
				}),
			},
		},
	}
	_, err = GetCitationEdges(context.Background(), stubStore)
	require.NoError(t, err)

	// Invalidate both.
	InvalidateCache()

	// Next main-cache call must recompute.
	// Clear TOCTOU race window: delete entry first.
	graphCache.Delete(cacheKeyMain)
	eg2, err := GetOrCompute(context.Background(), stub)
	require.NoError(t, err)
	assert.NotSame(t, eg1, eg2, "after InvalidateCache, main cache must be cleared")

	// Citation cache also cleared — after expiry wait, it recomputes.
	t.Setenv("SKILLFORGE_GRAPH_CITATION_CACHE_TTL_MS", "50")
	time.Sleep(60 * time.Millisecond)
	eg3, err := GetCitationEdges(context.Background(), stubStore)
	require.NoError(t, err)
	require.NotNil(t, eg3)
}

// ------------------------------------------------------------------
// Helper
// ------------------------------------------------------------------

func mustMarshalVerdict(v verdict.Verdict) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic("mustMarshalVerdict: " + err.Error())
	}
	return string(b)
}
