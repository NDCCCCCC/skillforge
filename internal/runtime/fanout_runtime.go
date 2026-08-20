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

// Package runtime — fanout_runtime.go is Phase 13 / Plan 13-01's
// per-sub-agent runtime substrate. It declares:
//
//   - **SubAgentBudgetConfig** + structural constants: the per-sub-agent
//     budget scope (P13-L3 — half of parent's 10/50000/3). The
//     constants are HARD-CODED and source-grep locked via
//     TestSubAgentMaxIterationHardCap (5) +
//     TestSubAgentTokenBudgetHardCap (20000). Config-driven knobs
//     drift; literal code constants cannot.
//
//   - **SharedChunkAccumulator**: an LRU wrapper around a slice of
//     retriever.Hit, capped at 20 entries (P13-L9 + Pitfall 6).
//     Sub-agents append to the SAME accumulator pointer so the
//     citation entailment check (D2-01) sees the union of parent
//     + sub-agent retrievals, but bounded — sub-agent 2's context
//     does not blow up because sub-agent 1 retrieved many chunks.
//
//   - **SubAgentBudgetCallbacks** seam: returns the eino v0.9.14
//     middleware slice consumed by the chat archetype's per-sub-agent
//     Runner. The full wiring (token accumulator + loop detect)
//     lands in 13-03; 13-01 ships the seam so the dispatch path
//     compiles and tests can be added incrementally.
package runtime

import (
	"container/list"
	"errors"
	"sync"

	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// SubAgentMaxIteration is the P13-L3 hard cap on per-sub-agent
// reasoning iterations. Source-grep locked via
// TestSubAgentMaxIterationHardCap. Half of the parent's
// DefaultMaxIteration (10) — sub-agent budget is bounded so one slow
// sub-agent cannot drain the parent's budget envelope.
//
// P12-L2 pattern is mirrored: the literal is the call-site source-
// grep target. Future 13-05 calibration may move the value up/down;
// every change is an explicit code edit + test assertion update,
// NOT a config knob.
const SubAgentMaxIteration = 5

// SubAgentTokenBudget is the P13-L3 hard cap on per-sub-agent token
// accumulation. Half of the parent's DefaultTokenBudgetPerRun
// (50000). Source-grep locked via TestSubAgentTokenBudgetHardCap.
const SubAgentTokenBudget = 20000

// SubAgentLoopWindow is the per-sub-agent rolling-3-call loop
// detector window (P12-L9 reused at the sub-agent scope). Same value
// as the parent's DefaultLoopDetectWindow; the sub-agent does not
// need a wider window because sub-agents do fewer iterations.
const SubAgentLoopWindow = 3

// SubAgentTimeoutSeconds is the per-sub-agent timeout
// (Pitfall 4 mitigation — Research §Pitfall 4). One slow sub-agent
// (retrieval backend stall, LLM timeout) must not starve the parent's
// overall response time.
const SubAgentTimeoutSeconds = 30

// SharedChunkAccumulatorMaxEntries is the LRU cap on the shared
// accumulator (Pitfall 6). N sub-agents × 5 chunks each can grow the
// accumulator to N×5 chunks; the cap prevents unbounded growth while
// preserving the "small + varied" subset the citation entailment
// verifier needs for grounding.
//
// Source-grep locked via TestSharedChunkAccumulatorLRUCap.
const SharedChunkAccumulatorMaxEntries = 20

// SubAgentBudgetConfig is the per-sub-agent budget scope (P13-L3).
// Zero values fall back to the package constants via Resolve().
//
// Mirrors the Phase 12 BudgetConfig shape (internal/runtime/budget.go)
// but is scoped to a SINGLE sub-agent's Run, not the parent's Run.
// The session ID is the CHILD session ID (parent:sub_query_id) so
// audit rows can be scoped to the sub-Run.
type SubAgentBudgetConfig struct {
	SessionID   string
	MaxIteration int
	TokenBudget  int
	LoopWindow   int
}

// Resolve returns a copy with zero values filled from the package
// constants. Callers MUST call Resolve before passing the config to
// a sub-agent Runner; the seam is fail-soft (zero values fall back to
// defaults) but the canonical path is Resolve-then-pass.
func (c SubAgentBudgetConfig) Resolve() SubAgentBudgetConfig {
	if c.MaxIteration <= 0 {
		c.MaxIteration = SubAgentMaxIteration
	}
	if c.TokenBudget <= 0 {
		c.TokenBudget = SubAgentTokenBudget
	}
	if c.LoopWindow <= 0 {
		c.LoopWindow = SubAgentLoopWindow
	}
	return c
}

// SubAgentBudgetCallbacks returns the eino v0.9.14 middleware slice
// for a sub-agent's Runner. Plan 13-03 fills this with the
// token-accumulator + loop-detect wrappers (reusing the Phase 12
// BudgetCallback); 13-01 ships the seam with an empty slice so the
// dispatch path compiles. The empty slice is structurally correct —
// the chat archetype's ChatModelAgent accepts zero middlewares.
//
// DO NOT call budget middleware from the parent Run: the parent's
// gate fires at MaxIteration=10/TokenBudget=50000; sub-agents are
// individually budget-gated by their own callbacks (added in 13-03).
func SubAgentBudgetCallbacks(cfg SubAgentBudgetConfig) []any {
	// 13-01 seam: returns empty slice. 13-03 wires the budget
	// callbacks (token accumulator + fnv64a loop detect) reusing
	// internal/agent/callbacks.NewBudgetCallback.
	_ = cfg.Resolve() // validate + apply defaults
	return nil
}

// SharedChunkAccumulator is a per-Run, mutex-guarded LRU accumulator
// shared across the parent orchestrator + all its sub-agents. The
// orchestrator passes the SAME pointer into every sub-agent's
// SubAgentConfig.ChunkAccumulator; sub-agent retrieve Tools append to
// the same map; the citation entailment verifier (D2-01) sees the
// UNION (Pitfall 6 mitigation — bounded union, no embedding cycle).
//
// Why an LRU (vs an unbounded slice):
//   - Memory + token-budget bound. Without the cap, N sub-agents
//     × 5 chunks each could grow the accumulator to 5N entries; the
//     downstream sub-agent's context window would balloon.
//   - Audit reproducibility. Hits() returns a deterministic
//     oldest-first slice (sorted insertion order, surviving eviction
//     keeps its position).
//   - Goroutine-safe. Sub-agents run in parallel (errgroup with
//     SetLimit(4)); a mutex is mandatory.
//
// The Hit key is the chunk ID (decimal string). Dedup is keyed on
// chunk ID (Pitfall 2 from internal/agent/trust_floor.go — D3-01
// open question 2 — same dedup-by-chunk-id posture).
type SharedChunkAccumulator struct {
	mu     sync.Mutex
	ll     *list.List               // doubly-linked; front = newest, back = oldest
	byID   map[string]*list.Element // chunk ID → list element
	max    int                      // max entries (default SharedChunkAccumulatorMaxEntries)
}

// sharedAccumulatorEntry is the LRU payload. The key field is a
// snapshot of the chunk ID for fast reads from the eviction path.
type sharedAccumulatorEntry struct {
	hit retriever.Hit
}

// NewSharedChunkAccumulator returns a fresh, empty LRU accumulator
// with the supplied cap. maxEntries <= 0 falls back to
// SharedChunkAccumulatorMaxEntries.
func NewSharedChunkAccumulator(maxEntries int) *SharedChunkAccumulator {
	if maxEntries <= 0 {
		maxEntries = SharedChunkAccumulatorMaxEntries
	}
	return &SharedChunkAccumulator{
		ll:   list.New(),
		byID: make(map[string]*list.Element),
		max:  maxEntries,
	}
}

// Add records a single hit. Dedup by chunk ID (Pitfall 2 from
// internal/agent/trust_floor.go). If the chunk ID is already
// present, the existing entry is moved to the front (LRU touch —
// "most recently used" position). If the LRU is at capacity and the
// chunk ID is new, the oldest entry is evicted.
//
// Returns true if a NEW entry was added; false if the chunk ID was
// already present (idempotent re-touch).
//
// Safe for concurrent calls (mutex-guarded).
func (s *SharedChunkAccumulator) Add(hit retriever.Hit) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := chunkKey(hit.ID)
	if elem, ok := s.byID[key]; ok {
		// Idempotent re-touch: move to front.
		s.ll.MoveToFront(elem)
		return false
	}
	// Evict oldest if at capacity.
	for s.ll.Len() >= s.max {
		oldest := s.ll.Back()
		if oldest == nil {
			break
		}
		evicted := oldest.Value.(*sharedAccumulatorEntry)
		delete(s.byID, chunkKey(evicted.hit.ID))
		s.ll.Remove(oldest)
	}
	elem := s.ll.PushFront(&sharedAccumulatorEntry{hit: hit})
	s.byID[key] = elem
	return true
}

// Hits returns the accumulator's entries in OLDEST-FIRST order.
// Choice of oldest-first is intentional: audit replay reads the
// accumulator in insertion order, so a fan-out run produces a
// deterministic citation entailment sequence regardless of the
// LRU's "recent touch" reordering.
//
// The returned slice is a snapshot — the caller may iterate
// concurrently with new Add calls without a race.
func (s *SharedChunkAccumulator) Hits() []retriever.Hit {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]retriever.Hit, 0, s.ll.Len())
	for e := s.ll.Back(); e != nil; e = e.Prev() {
		out = append(out, e.Value.(*sharedAccumulatorEntry).hit)
	}
	return out
}

// Len returns the current size of the accumulator.
func (s *SharedChunkAccumulator) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ll.Len()
}

// Max returns the LRU cap.
func (s *SharedChunkAccumulator) Max() int {
	return s.max
}

// SubAgentConfig is the per-Run construction envelope passed to
// BuildSubAgent. Mirrors DispatchConfig (Phase 12 / runtime/factory.go)
// but adds the sub-agent-specific fields: a ParentManifest reference
// (for tier-closure intersection), a SubQueryID (for audit RunPath),
// and a ChunkAccumulator pointer (shared with the parent Run so the
// citation entailment check sees the union).
//
// SubAgentConfig is declared in fanout_runtime.go (not
// sub_agent_builder.go) so the orchestrator can hold a SubAgentFactory
// interface reference and avoid a forward-declaration dependency
// cycle. The full BuildSubAgent implementation lands in 13-01 Task 3.
type SubAgentConfig struct {
	ParentManifest   *skill.Manifest
	SubSkillManifest *skill.Manifest
	SubQueryID       string
	ChatModel        model.ToolCallingChatModel
	Embedder         embedder.Embedder
	Retriever        retriever.Retriever
	SessionID        string
	ChunkAccumulator *SharedChunkAccumulator
	Instruction      string
}

// SubAgentFactoryError is the sentinel for BuildSubAgent load-time
// errors. Callers (the orchestrator's pre-fanout validation step)
// match on this error with errors.Is for the FAN-02 SC-2 contract.
var ErrSubAgentFactory = errors.New("runtime.BuildSubAgent: load-time error")

// IsSubAgentFactoryError reports whether err is (or wraps) the
// sub-agent factory sentinel. Used by the orchestrator's pre-fanout
// step to surface the specific FAN-02 SC-2 failure to the operator.
func IsSubAgentFactoryError(err error) bool {
	return errors.Is(err, ErrSubAgentFactory)
}