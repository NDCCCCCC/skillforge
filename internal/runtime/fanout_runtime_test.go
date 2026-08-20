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

package runtime

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

// TestSubAgentMaxIterationHardCap — P13-L3 source-grep lock. The
// constant must equal 5 (half of parent's DefaultMaxIteration=10).
func TestSubAgentMaxIterationHardCap(t *testing.T) {
	assert.Equal(t, 5, SubAgentMaxIteration,
		"P13-L3: SubAgentMaxIteration must equal 5 (half of parent's 10)")
}

// TestSubAgentTokenBudgetHardCap — P13-L3 source-grep lock. The
// constant must equal 20000 (half of parent's DefaultTokenBudgetPerRun).
func TestSubAgentTokenBudgetHardCap(t *testing.T) {
	assert.Equal(t, 20000, SubAgentTokenBudget,
		"P13-L3: SubAgentTokenBudget must equal 20000 (half of parent's 50000)")
}

// TestSubAgentBudgetConfigResolve — empty config → defaults from
// the package constants.
func TestSubAgentBudgetConfigResolve(t *testing.T) {
	cfg := SubAgentBudgetConfig{}.Resolve()
	assert.Equal(t, SubAgentMaxIteration, cfg.MaxIteration)
	assert.Equal(t, SubAgentTokenBudget, cfg.TokenBudget)
	assert.Equal(t, SubAgentLoopWindow, cfg.LoopWindow)
}

// TestSubAgentBudgetConfigResolvePreservesValues — non-zero values
// are preserved through Resolve (only zero values are filled).
func TestSubAgentBudgetConfigResolvePreservesValues(t *testing.T) {
	cfg := SubAgentBudgetConfig{
		MaxIteration: 3,
		TokenBudget:  1000,
		LoopWindow:   2,
	}.Resolve()
	assert.Equal(t, 3, cfg.MaxIteration)
	assert.Equal(t, 1000, cfg.TokenBudget)
	assert.Equal(t, 2, cfg.LoopWindow)
}

// TestSubAgentBudgetCallbacksReturnsEmpty — 13-01 seam: returns
// nil. 13-03 fills with the budget middleware slice.
func TestSubAgentBudgetCallbacksReturnsEmpty(t *testing.T) {
	cbs := SubAgentBudgetCallbacks(SubAgentBudgetConfig{SessionID: "sess"}.Resolve())
	assert.Nil(t, cbs, "13-01 ships an empty slice (seam); 13-03 fills with budget middleware")
}

// TestSharedChunkAccumulatorLRUCap — add 25 hits; Len() caps at 20;
// the oldest 5 are evicted.
func TestSharedChunkAccumulatorLRUCap(t *testing.T) {
	acc := NewSharedChunkAccumulator(0) // default cap
	for i := int64(1); i <= 25; i++ {
		added := acc.Add(retriever.Hit{ID: i})
		assert.True(t, added)
	}
	assert.Equal(t, 20, acc.Len(), "Len must equal max after exceeding capacity")
	assert.Equal(t, 20, acc.Max())
}

// TestSharedChunkAccumulatorDedup — add same hit twice; Len() == 1;
// Hits() returns one entry.
func TestSharedChunkAccumulatorDedup(t *testing.T) {
	acc := NewSharedChunkAccumulator(0)
	added1 := acc.Add(retriever.Hit{ID: 42})
	added2 := acc.Add(retriever.Hit{ID: 42})
	assert.True(t, added1, "first Add of new chunk returns true")
	assert.False(t, added2, "second Add of same chunk returns false (idempotent re-touch)")
	assert.Equal(t, 1, acc.Len())
	hits := acc.Hits()
	assert.Len(t, hits, 1)
	assert.Equal(t, int64(42), hits[0].ID)
}

// TestSharedChunkAccumulatorHitsOrder — 5 hits added in known order;
// Hits() returns oldest-first slice (deterministic for FAN-04
// per-sub-agent grounding).
func TestSharedChunkAccumulatorHitsOrder(t *testing.T) {
	acc := NewSharedChunkAccumulator(0)
	for i := int64(1); i <= 5; i++ {
		acc.Add(retriever.Hit{ID: i})
	}
	hits := acc.Hits()
	require.Len(t, hits, 5)
	// Oldest-first means first added is first returned.
	for i, h := range hits {
		assert.Equal(t, int64(i+1), h.ID, "hits[%d] must be ID=%d (oldest-first)", i, i+1)
	}
}

// TestSharedChunkAccumulatorLRUEvictsOldest — after exceeding cap,
// the OLDEST entries (first added) are evicted.
func TestSharedChunkAccumulatorLRUEvictsOldest(t *testing.T) {
	acc := NewSharedChunkAccumulator(3) // small cap for the test
	acc.Add(retriever.Hit{ID: 1})
	acc.Add(retriever.Hit{ID: 2})
	acc.Add(retriever.Hit{ID: 3})
	acc.Add(retriever.Hit{ID: 4}) // evicts ID=1
	acc.Add(retriever.Hit{ID: 5}) // evicts ID=2

	hits := acc.Hits()
	require.Len(t, hits, 3)
	assert.Equal(t, int64(3), hits[0].ID, "oldest surviving entry is ID=3")
	assert.Equal(t, int64(4), hits[1].ID)
	assert.Equal(t, int64(5), hits[2].ID, "newest entry is ID=5")
}

// TestSharedChunkAccumulatorConcurrent — 100 goroutines Add concurrently;
// final Len() == 100 OR max (whichever is smaller); no race.
func TestSharedChunkAccumulatorConcurrent(t *testing.T) {
	acc := NewSharedChunkAccumulator(50) // smaller cap to trigger eviction
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			acc.Add(retriever.Hit{ID: id})
		}(int64(i))
	}
	wg.Wait()
	// After 100 unique adds with cap=50, Len must equal 50.
	assert.Equal(t, 50, acc.Len(), "LRU cap enforces final size under concurrent adds")
}

// TestSubAgentFactoryError — the sentinel error is exported and
// matchable via errors.Is.
func TestSubAgentFactoryError(t *testing.T) {
	assert.NotNil(t, ErrSubAgentFactory)
	assert.True(t, IsSubAgentFactoryError(ErrSubAgentFactory))
}

// TestNewSharedChunkAccumulatorDefaults — maxEntries <= 0 falls back
// to SharedChunkAccumulatorMaxEntries (20).
func TestNewSharedChunkAccumulatorDefaults(t *testing.T) {
	acc := NewSharedChunkAccumulator(0)
	assert.Equal(t, SharedChunkAccumulatorMaxEntries, acc.Max())
}

// TestSharedChunkAccumulatorMaxReturnsConstant — Max() returns the
// cap that was set at construction (default or explicit).
func TestSharedChunkAccumulatorMaxReturnsConstant(t *testing.T) {
	acc := NewSharedChunkAccumulator(7)
	assert.Equal(t, 7, acc.Max())
}