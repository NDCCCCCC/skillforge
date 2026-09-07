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
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

// citableTestRegistry is the KBM-03 fixture: authority is the citable
// role, historical is a kept-but-never-citable revision.
func citableTestRegistry() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
	})
}

// TestSharedChunkAccumulatorCitableGate is the CR-02 (26-REVIEW) lock on
// the THIRD chunk-insertion point: the fan-out shared accumulator is the
// union the merged verdict's citations are entailment-checked against, so
// with a registry bound a non-authority hit must never enter it — exactly
// the gate internal/agent's ChunkAccumulator.Add applies at its insertion
// point.
func TestSharedChunkAccumulatorCitableGate(t *testing.T) {
	acc := NewSharedChunkAccumulator(0)
	acc.SetKBRegistry(citableTestRegistry())

	assert.True(t, acc.Add(retriever.Hit{ID: 1, Tier: "authority", Content: "authority text"}),
		"an authority-role hit is evidence and enters the union")
	assert.False(t, acc.Add(retriever.Hit{ID: 2, Tier: "historical", Content: "superseded text"}),
		"a historical-role hit must never enter the grounding union")
	assert.False(t, acc.Add(retriever.Hit{ID: 3, Tier: "never-registered", Content: "unknown tier"}),
		"a lookup failure means not-evidence (fail-closed)")

	hits := acc.Hits()
	require.Len(t, hits, 1, "only the citable hit survives the gate")
	assert.Equal(t, "authority", hits[0].Tier)

	// The gate frees the chunk ID too: a later citable hit with the same
	// ID must still be recordable (mirrors the agent accumulator contract).
	assert.True(t, acc.Add(retriever.Hit{ID: 2, Tier: "authority", Content: "authority text for id 2"}),
		"a citable hit reusing a dropped non-citable ID must be recorded")
	assert.Len(t, acc.Hits(), 2)
}

// TestSharedChunkAccumulatorNilRegistryLegacy — a nil registry is the
// explicit legacy posture (tests / degraded paths): every hit enters the
// union byte-for-byte as before the registry existed (Pattern R3).
func TestSharedChunkAccumulatorNilRegistryLegacy(t *testing.T) {
	acc := NewSharedChunkAccumulator(0) // no registry bound
	assert.True(t, acc.Add(retriever.Hit{ID: 1, Tier: "historical", Content: "x"}))
	assert.True(t, acc.Add(retriever.Hit{ID: 2, Tier: "authority", Content: "y"}))
	assert.Len(t, acc.Hits(), 2, "nil registry keeps the pre-26 posture")
}

// TestCitableChunksByIDFiltersNonAuthority locks the defense-in-depth
// filter ApplyTrustStage applies when it builds the grounding map: the
// map is the physical citation whitelist, so a non-authority chunk must
// not be a key an entailment check could succeed against.
func TestCitableChunksByIDFiltersNonAuthority(t *testing.T) {
	chunks := []retriever.Hit{
		{ID: 7, Tier: "authority", Content: "authority chunk"},
		{ID: 8, Tier: "historical", Content: "historical chunk"},
		{ID: 9, Tier: "unregistered", Content: "unknown chunk"},
	}

	bound := citableChunksByID(citableTestRegistry(), chunks)
	require.Contains(t, bound, "7")
	assert.NotContains(t, bound, "8", "historical material must not enter the grounding map")
	assert.NotContains(t, bound, "9", "unregistered material must not enter the grounding map")

	legacy := citableChunksByID(nil, chunks)
	assert.Len(t, legacy, 3, "nil registry keeps the legacy (ungated) map")
}

// TestMergedVerdictRefusesOnNonCitableOnlyUnion is the CR-02 behavioral
// regression: a fan-out run whose shared union holds ONLY non-authority
// material must not ground the merged verdict against it. With the gate
// in place the accumulator yields zero citable chunks, so the merged
// grounding pass hits the trust floor (refusal) instead of certifying
// citations sourced from historical material.
func TestMergedVerdictRefusesOnNonCitableOnlyUnion(t *testing.T) {
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "orchestrator"},
		ChatModel: stubChatModel{},
		SessionID: "sess-citable",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.kbReg = citableTestRegistry()

	merged := MergedVerdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations:  []verdict.Citation{{ChunkID: "2", Span: "superseded text", Tier: "historical"}},
	}

	acc := NewSharedChunkAccumulator(0)
	acc.SetKBRegistry(orch.kbReg)
	assert.False(t, acc.Add(retriever.Hit{ID: 2, Tier: "historical", Content: "superseded text"}),
		"the gate must drop the historical hit at the insertion point")
	require.Zero(t, acc.Len(), "the union is empty after the gate")

	out := orch.applyTrustStageToMerged(context.Background(), merged, acc)
	require.NotNil(t, out.Refusal, "non-authority-only union must refuse, not ground")
	assert.True(t, out.TrustFloorHit)
	assert.Nil(t, out.Verdict)

	// Control: the same union WITH an authority hit grounds the merged
	// verdict (the floor passes; the merged envelope is returned).
	acc2 := NewSharedChunkAccumulator(0)
	acc2.SetKBRegistry(orch.kbReg)
	assert.False(t, acc2.Add(retriever.Hit{ID: 2, Tier: "historical", Content: "superseded text"}))
	require.True(t, acc2.Add(retriever.Hit{ID: 1, Tier: "authority", Content: "authority text"}))
	out2 := orch.applyTrustStageToMerged(context.Background(), merged, acc2)
	assert.Nil(t, out2.Refusal, "a citable hit in the union keeps the merged verdict alive")
	require.NotNil(t, out2.Verdict)
	assert.Equal(t, "ok", out2.Verdict.Status)
}

// TestApplyTrustStageGroundsOnlyCitableChunks pins the trust-stage side:
// with a registry bound on the input, the grounding map excludes
// non-authority chunks, so a citation whose span exists ONLY in a
// historical chunk is ungrounded (downgraded), while the same citation
// grounds when no registry is bound (legacy posture).
func TestApplyTrustStageGroundsOnlyCitableChunks(t *testing.T) {
	chunks := []retriever.Hit{
		{ID: 2, Tier: "historical", Content: "annual leave is 3 days per the 2020 draft"},
	}
	in := &TrustStageInput{
		Manifest:   &skill.Manifest{Name: "compliance", Agent: "chat"},
		Output:     &schema.Message{Role: schema.Assistant, Content: "verdict"},
		Chunks:     chunks,
		KBRegistry: citableTestRegistry(),
	}
	// Grounding is exercised through the pure checker on the SAME map
	// ApplyTrustStage builds, since the 12-01 verdict stub carries no
	// citations of its own.
	got := citableChunksByID(in.KBRegistry, in.Chunks)
	report := verdict.GroundingCheck(verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations:  []verdict.Citation{{ChunkID: "2", Span: "3 days", Tier: "historical"}},
	}, got)
	assert.False(t, report.Grounded, "a historical-only citation must not ground once the registry is bound")

	legacy := citableChunksByID(nil, chunks)
	reportLegacy := verdict.GroundingCheck(verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations:  []verdict.Citation{{ChunkID: "2", Span: "3 days", Tier: "historical"}},
	}, legacy)
	assert.True(t, reportLegacy.Grounded, "legacy (nil registry) keeps today's grounding behavior")
}
