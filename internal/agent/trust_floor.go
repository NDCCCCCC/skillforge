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

package agent

import (
	"fmt"
	"strings"
	"sync"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/verdict"
)

// ChunkAccumulator is the per-Run, mutex-guarded collector that records
// every chunk the retrieve_compliance_chunks tool returns across all
// agent hops. The trust floor reads TierMatched() AFTER the agent loop
// finishes to decide whether the agent had ANY tier-matched grounding
// before producing its verdict (D3-01 / D-09 / KNOW-04).
//
// Why a dedicated type (vs. a plain []retriever.Hit):
//
//   - **Pitfall 2 (race safety)**: Eino's tool callbacks may fire from
//     different goroutines when the agent issues parallel tool calls
//     (the ReAct loop has a parallel branch). A plain slice append would
//     race; the mutex here makes concurrent Add calls safe. Locked by
//     TestAccumulatorConcurrent under -race.
//
//   - **D3-01 open question 2 (dedup by chunk ID)**: hop 1 may return
//     chunk X and hop 2 may return chunk X again (same ID, same content)
//     because the retriever does not track per-query state. The
//     accumulator dedups by chunk ID — TierMatched() reports 1 unique
//     chunk, NOT 2. Without dedup, a flaky retriever could inflate the
//     apparent grounding and bypass the floor. Locked by TestTrustFloorDedup.
//
//   - **Closed-flag discipline**: the trust floor reads TierMatched()
//     only AFTER markClosed() has been called (signaling iterator EOF).
//     This prevents a subtle TOCTOU where the floor reads a half-filled
//     accumulator while a callback is still in flight.
//
// Construction: callers use NewChunkAccumulator (returns a ready-to-use
// zero-value accumulator with the maps initialized).
type ChunkAccumulator struct {
	mu     sync.Mutex
	closed bool
	seen   map[string]struct{} // chunk ID (decimal string) → existence
	byID   map[string]retriever.Hit

	// callCount = number of retrieve tool calls (Add batches). Used by
	// HasMultipleRetrieveCalls for critique gating.
	callCount int
}

// NewChunkAccumulator returns a fresh, empty accumulator.
func NewChunkAccumulator() *ChunkAccumulator {
	return &ChunkAccumulator{
		seen: make(map[string]struct{}),
		byID: make(map[string]retriever.Hit),
	}
}

// Add records the given hits. Dedups by chunk ID (the D3-01 open
// question 2 resolution). Hits with ID=0 are still recorded (a zero ID
// is treated as a valid chunk ID; the retrieve tool does not skip ID=0).
// Safe for concurrent calls (mutex-guarded per Pitfall 2).
//
// Add increments the internal retrieve-call counter exactly once per
// invocation (regardless of how many hits are in the batch). The counter
// backs HasMultipleRetrieveCalls for critique gating.
//
// Add is a no-op when called after MarkClosed (defense-in-depth against
// a stray callback firing post-iterator-EOF).
func (a *ChunkAccumulator) Add(hits []retriever.Hit) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		// Stray callback after EOF — silently drop. This is a soft
		// invariant violation: it should never happen (Eino's callback
		// contract says no callbacks fire after Stream returns EOF),
		// but if it does we prefer stable state over a panic.
		return
	}
	a.callCount++
	for _, h := range hits {
		key := chunkKey(h.ID)
		if _, dup := a.seen[key]; dup {
			continue
		}
		a.seen[key] = struct{}{}
		a.byID[key] = h
	}
}

// TierMatched returns a freshly-allocated slice of the unique
// tier-matched hits collected so far. The slice is caller-owned — safe
// to iterate (and to mutate) without holding the accumulator's lock.
//
// The trust floor reads this AFTER MarkClosed to decide whether the
// agent had any grounding at all (KNOW-04 / D-09).
func (a *ChunkAccumulator) TierMatched() []retriever.Hit {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]retriever.Hit, 0, len(a.byID))
	for _, h := range a.byID {
		out = append(out, h)
	}
	return out
}

// ChunksByID returns a map from chunk ID (decimal string) → chunk
// Content. This is the shape verdict.GroundingChecker expects (its
// second argument is `map[string]string` keyed by chunk ID).
func (a *ChunkAccumulator) ChunksByID() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]string, len(a.byID))
	for k, h := range a.byID {
		out[k] = h.Content
	}
	return out
}

// MarkClosed signals that the agent loop has finished (the underlying
// react.Agent iterator returned EOF). Subsequent Add calls become
// no-ops. The trust floor reads state only after MarkClosed.
func (a *ChunkAccumulator) MarkClosed() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
}

// chunkKey renders a retriever.Hit.ID (int64) as the canonical map key
// the accumulator uses for dedup and ChunksByID lookup. Centralized here
// so Add / TierMatched / ChunksByID / GroundingChecker all agree on the
// key format (the retrieve tool emits `[CHUNK %d]` using the same
// decimal rendering — T-2-21 framing + chunk ID format are paired).
func chunkKey(id int64) string {
	// Avoid strconv to keep imports minimal; int64 → decimal is small.
	return fmt.Sprintf("%d", id)
}

// HasMultipleRetrieveCalls reports whether the retrieve tool was
// invoked more than once during the Run. Used by (*Agent).Run to
// decide whether MaybeCritique's multiHop gating branch should fire
// (critique runs unconditionally when multiHop=true per D3-05).
//
// The count is the number of distinct Add() call batches (one per
// retrieve invocation), NOT the number of unique chunks (one retrieve
// can return multiple chunks). Tests do not assert on this directly;
// it is a gating signal only.
func (a *ChunkAccumulator) HasMultipleRetrieveCalls() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.callCount > 1
}

// ApplyTrustFloor implements the D3-01 / D-09 / KNOW-04 refusal-by-
// construction invariant. The trust floor is the deterministic moat:
// even if the agent LLM produced a fully-formed 合规 verdict, if the
// chunkAccumulator recorded ZERO unique tier-matched chunks across all
// hops, the agent's output is discarded and a structured Refusal is
// returned in its place.
//
// The trust floor runs AFTER the agent loop finishes and BEFORE the
// optional critique pass. It is the only path that can produce a
// Refusal{Reason: NoAuthorizedSource}; the retrieve tool itself never
// refuses (it just returns an empty Chunks string), and the agent LLM
// cannot trigger this refusal by prompt (it has no API to write to the
// accumulator).
//
// Returns:
//
//   - nil when the accumulator has ≥1 chunk — caller proceeds to parse
//     the agent's verdict and apply GroundingChecker + maybeCritique.
//   - Non-nil *verdict.Refusal when the accumulator is empty — caller
//     returns (verdict.Verdict{}, refusal, nil) from (*Agent).Run.
//
// The SuggestedAlternativeQuery is rule-based (D2-14): it appends the
// first resolved tier keyword to the original query, giving the user a
// one-tap retry chip in the UI. Production code path: the suggester is
// extracted from internal/server/pipeline.go ruleBasedSuggestion (the
// trust floor reuses it verbatim via the local ruleBasedSuggestion
// helper — see ruleBasedSuggestion godoc for the source).
func ApplyTrustFloor(acc *ChunkAccumulator, originalQuery string, resolvedTiers []string) *verdict.Refusal {
	matched := acc.TierMatched()
	if len(matched) > 0 {
		return nil
	}
	return &verdict.Refusal{
		Reason:                    verdict.RefusalReasonNoAuthorizedSource,
		CanRephrase:               true,
		SuggestedAlternativeQuery: ruleBasedSuggestion(originalQuery, resolvedTiers),
	}
}

// ruleBasedSuggestion produces a rephrased query suggestion for the
// refusal path. Copied verbatim (logic-equivalent) from
// internal/server/pipeline.go:267-278 — the trust floor reuses the same
// heuristic so refusal suggestions stay consistent across the old
// pipeline (Plan 02-03) and the new agent (Plan 03-03).
//
// The suggestion appends the first resolved tier keyword (e.g. "权威")
// to the original query so a follow-up retrieve has a stronger tier
// signal. When the tier list is empty (default-deny skill — should not
// happen at runtime because the loader rejects empty reads, but
// defensive), the suggestion is the original query with a generic
// "制度" suffix.
//
// Production migration target (Plan 03-05): lift this into a shared
// internal/agent/prompt.go (or internal/suggester package) and have
// both pipeline.go and trust_floor.go import it — anti-pattern #1
// drift guard. Until then the duplication is intentional and
// regression-locked by TestTrustFloorForcesRefusal.
func ruleBasedSuggestion(originalQuery string, resolvedTiers []string) string {
	q := strings.TrimSpace(originalQuery)
	if q == "" {
		q = "相关制度"
	}
	if len(resolvedTiers) == 0 {
		return q + " 制度"
	}
	return q + " " + resolvedTiers[0]
}
