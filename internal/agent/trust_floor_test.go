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

// Phase 26 / Plan 26-06 Task 1 — the KBM-03 citability gate tests.
//
// The gate lives at the ChunkAccumulator.Add insertion point (26-RESEARCH
// Pattern R3, insertion point (a)): a hit whose registry role is not
// kb.CitableRole is structurally excluded from the grounding closure
// (chunksByID / TierMatched), so the trust floor can never see it as
// evidence. Three layers stay distinct (Pitfall 2): retrieval still
// follows the Skill's reads declaration, prompt presentation is a
// separate annotate-channel concern, and THIS gate owns only the
// grounding closure.
//
// The nil-registry branch is the legacy posture: tests and degraded
// paths keep today's behavior byte-for-byte; production wiring is
// 26-09's scope.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/verdict"
)

// TestCitableGateDropsNonAuthority — the core KBM-03 lock: under a
// bound registry, authority-role hits become evidence and historical /
// personal hits (both reachable through a Skill's reads declaration)
// never enter the grounding closure.
func TestCitableGateDropsNonAuthority(t *testing.T) {
	acc := NewChunkAccumulator()
	acc.SetKBRegistry(testKBReg())
	acc.Add([]retriever.Hit{
		{ID: 1, Tier: "authority", Source: "authority/a.md", Content: "citable clause", Score: 0.9},
		{ID: 2, Tier: "historical", Source: "historical/h.md", Content: "superseded revision", Score: 0.9},
		{ID: 3, Tier: "personal", Source: "personal/p.md", Content: "user-private note", Score: 0.9},
	})
	acc.MarkClosed()

	chunks := acc.ChunksByID()
	require.Len(t, chunks, 1,
		"only the authority-role chunk may enter the grounding closure (KBM-03)")
	assert.Equal(t, "citable clause", chunks["1"],
		"the surviving chunk must be the authority-role hit")

	matched := acc.TierMatched()
	require.Len(t, matched, 1, "TierMatched feeds the trust floor — it must see citable hits only")
	assert.Equal(t, "authority", matched[0].Tier)
}

// TestCitableGateNilRegistryLegacy — an accumulator without
// SetKBRegistry keeps today's semantics exactly (nil = legacy
// tests/degraded posture; 26-RESEARCH Pattern R3 sanctioned).
func TestCitableGateNilRegistryLegacy(t *testing.T) {
	acc := NewChunkAccumulator()
	acc.Add([]retriever.Hit{
		{ID: 1, Tier: "authority", Source: "authority/a.md", Content: "citable clause", Score: 0.9},
		{ID: 2, Tier: "historical", Source: "historical/h.md", Content: "superseded revision", Score: 0.9},
		{ID: 3, Tier: "personal", Source: "personal/p.md", Content: "user-private note", Score: 0.9},
	})
	acc.MarkClosed()

	assert.Len(t, acc.ChunksByID(), 3,
		"nil registry must keep the pre-gate behavior byte-for-byte")
	assert.Len(t, acc.TierMatched(), 3)
}

// TestCitableGateUnknownTierDropped — a hit whose tier is not a
// registered slug fails Lookup, and a failed Lookup cannot pass the
// gate (fail-closed, same posture as an unregistered slug at skill
// validation).
func TestCitableGateUnknownTierDropped(t *testing.T) {
	acc := NewChunkAccumulator()
	acc.SetKBRegistry(testKBReg())
	acc.Add([]retriever.Hit{
		{ID: 7, Tier: "ghost", Source: "ghost/g.md", Content: "unregistered slug", Score: 0.9},
		{ID: 8, Tier: "authority", Source: "authority/a.md", Content: "citable clause", Score: 0.8},
	})
	acc.MarkClosed()

	chunks := acc.ChunksByID()
	require.Len(t, chunks, 1, "an unregistered slug must be dropped, not defaulted")
	assert.Equal(t, "citable clause", chunks["8"])
}

// TestCitableGateDeactivatedDropped — a deactivated KB is invisible to
// Lookup (KBM-02: deactivate, never destroy), so its hits are dropped
// exactly like unregistered ones.
func TestCitableGateDeactivatedDropped(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: false},
	})

	acc := NewChunkAccumulator()
	acc.SetKBRegistry(reg)
	acc.Add([]retriever.Hit{
		{ID: 1, Tier: "historical", Source: "historical/h.md", Content: "deactivated base", Score: 0.9},
		{ID: 2, Tier: "authority", Source: "authority/a.md", Content: "citable clause", Score: 0.8},
	})
	acc.MarkClosed()

	chunks := acc.ChunksByID()
	require.Len(t, chunks, 1, "a deactivated KB's hits must not enter the closure")
	assert.Equal(t, "citable clause", chunks["2"])
}

// TestTrustFloorRefusesWithoutCitable — the KBM-03 semantic
// consequence, locked on purpose: a Skill whose reads declare
// historical CAN retrieve historical hits, but with none of them
// citable the trust floor sees zero evidence and refuses. Grounded
// only in a non-authority role must produce a refusal, not a citation.
func TestTrustFloorRefusesWithoutCitable(t *testing.T) {
	acc := NewChunkAccumulator()
	acc.SetKBRegistry(testKBReg())
	acc.Add([]retriever.Hit{
		{ID: 4, Tier: "historical", Source: "historical/h1.md", Content: "old revision", Score: 0.9},
		{ID: 5, Tier: "historical", Source: "historical/h2.md", Content: "older revision", Score: 0.8},
	})
	acc.MarkClosed()

	refusal := ApplyTrustFloor(acc, "出差住宿标准", []string{"authority", "historical"})
	require.NotNil(t, refusal,
		"historical-only grounding must refuse (KBM-03: authority is the only evidence role)")
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, refusal.Reason)
	assert.True(t, refusal.CanRephrase)
}

// TestTrustFloorPassesWithCitable — the gate does not over-refuse: one
// authority hit among non-citable ones still satisfies the floor.
func TestTrustFloorPassesWithCitable(t *testing.T) {
	acc := NewChunkAccumulator()
	acc.SetKBRegistry(testKBReg())
	acc.Add([]retriever.Hit{
		{ID: 1, Tier: "historical", Source: "historical/h.md", Content: "old revision", Score: 0.9},
		{ID: 2, Tier: "authority", Source: "authority/a.md", Content: "current clause", Score: 0.8},
	})
	acc.MarkClosed()

	assert.Nil(t, ApplyTrustFloor(acc, "出差住宿标准", []string{"authority", "historical"}),
		"one citable hit must satisfy the trust floor")
}

// TestRuleBasedSuggestionUsesDisplayName — the refusal suggestion
// keyword is the registry DisplayName of the first active
// authority-role KB (T-26-06-4): with the controlled seed's display
// names the copy stays byte-identical, and the ASCII-name case proves
// the string is registry-driven rather than a compiled-in literal.
func TestRuleBasedSuggestionUsesDisplayName(t *testing.T) {
	seedNamed := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	acc := NewChunkAccumulator()
	acc.SetKBRegistry(seedNamed)
	refusal := ApplyTrustFloor(acc, "出差住宿标准", []string{"authority"})
	require.NotNil(t, refusal)
	assert.Equal(t, "出差住宿标准 权威", refusal.SuggestedAlternativeQuery,
		"seed display names keep the refusal copy byte-identical (T-26-06-4)")

	asciiNamed := NewChunkAccumulator()
	asciiNamed.SetKBRegistry(testKBReg())
	refusal = ApplyTrustFloor(asciiNamed, "出差住宿标准", []string{"authority"})
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.SuggestedAlternativeQuery, "Authority",
		"the keyword must come from the registry, not from a hard-coded literal")

	legacy := NewChunkAccumulator()
	refusal = ApplyTrustFloor(legacy, "出差住宿标准", []string{"权威"})
	require.NotNil(t, refusal)
	assert.Equal(t, "出差住宿标准 权威", refusal.SuggestedAlternativeQuery,
		"nil registry falls back to the first resolved tier (today's behavior)")

	emptyTiers := NewChunkAccumulator()
	refusal = ApplyTrustFloor(emptyTiers, "出差住宿标准", nil)
	require.NotNil(t, refusal)
	assert.Equal(t, "出差住宿标准 制度", refusal.SuggestedAlternativeQuery,
		"nil registry + empty resolved tiers keeps the generic-suffix fallback")
}

// TestBuildRunnerInjectsRegistry — the agent-path injection seam: the
// per-Run accumulator is constructed with the config's KBRegistry
// bound, and an unset field leaves the legacy nil posture. The
// observation point is the accumulator constructor the Runner/Agent
// assembly calls (the adk Runner's Agent field is unexported, so the
// seam helper is the behavioral proxy — same rationale as
// tools_for_channel_test.go's agentChannelTools helper).
func TestBuildRunnerInjectsRegistry(t *testing.T) {
	gated := newRunAccumulator(AgentConfig{KBRegistry: testKBReg()})
	if gated.kb == nil {
		t.Fatal("a configured KBRegistry must reach the per-Run accumulator")
	}
	gated.Add([]retriever.Hit{{ID: 3, Tier: "draft", Source: "draft/d.md", Content: "never citable", Score: 0.9}})
	gated.MarkClosed()
	assert.Empty(t, gated.ChunksByID(),
		"the injected registry must be live — draft-role hits can never pass the gate")

	legacy := newRunAccumulator(AgentConfig{})
	if legacy.kb != nil {
		t.Fatal("an unset KBRegistry must leave the accumulator in the nil/legacy posture")
	}
	legacy.Add([]retriever.Hit{{ID: 3, Tier: "draft", Source: "draft/d.md", Content: "legacy retains", Score: 0.9}})
	legacy.MarkClosed()
	assert.Len(t, legacy.ChunksByID(), 1, "legacy posture keeps pre-gate behavior")
}
