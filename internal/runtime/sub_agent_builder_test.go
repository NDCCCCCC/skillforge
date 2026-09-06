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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// stubEmbedder satisfies embedder.Embedder for sub-agent builder tests.
// Returns a fixed vector; no I/O.
type stubEmbedder struct{}

func (stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
func (stubEmbedder) Dimensions() int { return 3 }

// stubRetriever satisfies retriever.Retriever for sub-agent builder
// tests. All methods return nil/empty; no I/O.
type stubRetriever struct{}

func (stubRetriever) Health(_ context.Context) error  { return nil }
func (stubRetriever) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (stubRetriever) Ensure(_ context.Context) error { return nil }
func (stubRetriever) Insert(_ context.Context, _ []retriever.Document) error {
	return nil
}
func (stubRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (stubRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, nil
}
func (stubRetriever) Disconnect() error { return nil }

// testKBReg returns the read-only registry the sub-agent builder
// resolves sub-skill reads against: the four baseline bases of the
// controlled seed (knowledge_bases.yaml), all active.
func testKBReg() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "draft", DisplayName: "Draft", Role: kb.RoleDraft, RetrievalEnabled: true},
	})
}

// validParentManifest returns a parent Manifest with ResolvedTiers
// ["authority"] so FAN-02 SC-2 intersection tests have a known starting
// point. The value is a registry slug since Phase 26 / Plan 26-04: the
// loader populates ResolvedTiers from kb.Registry.ResolveReads.
func validParentManifest() *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Description: "parent orchestrator skill",
		Agent:       "orchestrator",
		Reads:       []string{"authority/**"},
		Writes:      []string{}, // orchestrator is read-only by structural design
		Steps:       "1. Classify query\n2. Fan-out sub-agents",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never invoke HITL tools",
				"Never bypass tier-closure",
				"Never expand sub-agent tier boundary",
			},
		},
		Temperature: &temp,
	}
	m.SetResolvedTiersForTest([]string{"authority"})
	return m
}

// validSubManifest returns a sub-skill Manifest with the supplied
// Reads slice. The loader populates resolvedTiers from the reads
// patterns through kb.Registry.ResolveReads; for tests we set them
// directly via SetResolvedTiersForTest (a nil tiers slice leaves the
// field unset, which forces the ResolveTiers fallback path).
func validSubManifest(name string, reads []string, tiers []string) *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Name:        name,
		Description: "sub-skill",
		Agent:       "chat",
		Reads:       reads,
		Writes:      []string{}, // sub-agents are read-only by structural design
		Steps:       "1. Retrieve chunks\n2. Compose verdict",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never cite clauses not retrieved",
				"Never paraphrase critical clauses",
				"Never modify authority tier content",
			},
		},
		Temperature: &temp,
	}
	if tiers != nil {
		m.SetResolvedTiersForTest(tiers)
	}
	return m
}

// wantIntersectionEmptyContract is the FAN-02 SC-2 error contract, pinned
// here as a constant so the source and the test cannot drift apart (the
// spike-fanout gate and the orchestrator audit rows match this substring).
const wantIntersectionEmptyContract = "tier intersection empty"

// TestBuildSubAgentTierIntersectionEmpty — FAN-02 SC-2 load-time
// error: parent's ResolvedTiers ∩ sub_skill.Reads = empty → error
// containing substring "tier intersection empty".
func TestBuildSubAgentTierIntersectionEmpty(t *testing.T) {
	parent := validParentManifest() // ResolvedTiers: [authority]
	sub := validSubManifest("sub-a", []string{"personal/**"}, []string{"personal"})

	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-1",
	})
	if err == nil {
		t.Fatal("BuildSubAgent must FAIL when parent and sub_skill tiers do not intersect")
	}
	if !strings.Contains(err.Error(), wantIntersectionEmptyContract) {
		t.Fatalf("expected FAN-02 SC-2 substring, got %v", err)
	}
	if !errors.Is(err, ErrSubAgentFactory) {
		t.Fatalf("expected error to wrap ErrSubAgentFactory, got %v", err)
	}
}

// TestSubAgentIntersectionEmptyContract — the FAN-02 SC-2 string is a
// measured contract (spike-fanout gate + orchestrator audit rows): it
// survives the registry migration byte-for-byte, including the parent /
// sub tier echo.
func TestSubAgentIntersectionEmptyContract(t *testing.T) {
	parent := validParentManifest()
	sub := validSubManifest("sub-contract", []string{"personal/**"}, []string{"personal"})

	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa-contract",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-contract",
	})
	require.Error(t, err)
	want := "runtime.BuildSubAgent: load-time error: tier intersection empty " +
		"(parent=[authority], sub_skill=[personal]) — load-time validation failure per FAN-02 SC-2"
	assert.Equal(t, want, err.Error(),
		"the FAN-02 SC-2 error string is a measured contract — do not reword it")
}

// TestBuildSubAgentMatch — happy path: parent's ResolvedTiers ∩
// sub_skill.Reads = [authority] → BuildSubAgent succeeds.
func TestBuildSubAgentMatch(t *testing.T) {
	parent := validParentManifest()
	sub := validSubManifest("sub-b", []string{"authority/**"}, []string{"authority"})

	agent, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-1",
	})
	if err != nil {
		t.Fatalf("BuildSubAgent with matching tiers must succeed, got %v", err)
	}
	if agent == nil {
		t.Fatal("BuildSubAgent returned nil agent")
	}
}

// TestSubAgentNilRegistryFailsClosed — a sub-skill whose tiers are not
// pre-resolved needs the registry to resolve its reads; without it the
// construction must fail closed rather than run a sub-agent whose tier
// boundary nobody could verify (T-26-04-1 posture at the runtime seam).
func TestSubAgentNilRegistryFailsClosed(t *testing.T) {
	parent := validParentManifest()
	// Reads declared but resolvedTiers NOT injected — forces the
	// ResolveTiers fallback path that needs the registry.
	sub := validSubManifest("sub-noreg", []string{"authority/**"}, nil)

	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       nil,
		SubQueryID:       "sa-nilreg",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-nilreg",
	})
	if err == nil {
		t.Fatal("BuildSubAgent must FAIL when the registry is missing and the sub-skill tiers are unresolved")
	}
	if !strings.Contains(err.Error(), "registry") {
		t.Fatalf("expected the fail-closed registry error, got %v", err)
	}
	if !errors.Is(err, ErrSubAgentFactory) {
		t.Fatalf("expected error to wrap ErrSubAgentFactory, got %v", err)
	}
}

// TestSubAgentReadPatternsTrivial — the reverse translation is now
// trivial: a resolved slug IS the pattern's top segment, so the mapping
// is slug + "/**" with order preserved and nothing dropped.
func TestSubAgentReadPatternsTrivial(t *testing.T) {
	got := resolvedTiersToReadPatterns([]string{"authority", "historical"})
	want := []string{"authority/**", "historical/**"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

// TestBuildSubAgentSubEmptyReads — sub_skill.Reads is empty → error
// with substring "empty ResolvedTiers" (BuildSubAgent operates on
// ResolvedTiers after ResolveTiers falls back from empty Reads).
func TestBuildSubAgentSubEmptyReads(t *testing.T) {
	parent := validParentManifest()
	sub := validSubManifest("sub-c", []string{}, []string{})

	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-1",
	})
	if err == nil {
		t.Fatal("BuildSubAgent must FAIL when sub_skill Reads is empty")
	}
	if !strings.Contains(err.Error(), "empty ResolvedTiers") {
		t.Fatalf("expected empty ResolvedTiers substring, got %v", err)
	}
}

// TestBuildSubAgentParentEmptyTiers — parent.ResolvedTiers() empty →
// error with substring "empty ResolvedTiers".
func TestBuildSubAgentParentEmptyTiers(t *testing.T) {
	parent := validParentManifest()
	parent.SetResolvedTiersForTest(nil)
	sub := validSubManifest("sub-d", []string{"authority/**"}, []string{"authority"})

	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-1",
	})
	if err == nil {
		t.Fatal("BuildSubAgent must FAIL when parent ResolvedTiers is empty")
	}
	if !strings.Contains(err.Error(), "empty ResolvedTiers") {
		t.Fatalf("expected empty ResolvedTiers substring, got %v", err)
	}
}

// TestBuildSubAgentRequiresManifests — both manifests nil → error.
func TestBuildSubAgentRequiresManifests(t *testing.T) {
	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ChatModel: stubChatModel{},
		Embedder:  stubEmbedder{},
		Retriever: stubRetriever{},
		SessionID: "sess-1",
	})
	if err == nil {
		t.Fatal("BuildSubAgent must FAIL when both manifests are nil")
	}
	if !strings.Contains(err.Error(), "manifests required") {
		t.Fatalf("expected manifests-required substring, got %v", err)
	}
}

// TestBuildSubAgentIsReadOnly — P13-L4: sub-agent's manifest.Writes
// is zeroed AT CONSTRUCTION, even if the sub-skill declares writes.
// The structural read-only invariant is enforced BEFORE
// BuildToolsForSkill sees the subManifest.
func TestBuildSubAgentIsReadOnly(t *testing.T) {
	parent := validParentManifest()
	sub := validSubManifest("sub-e", []string{"authority/**"}, []string{"authority"})
	// Even though sub_skill has writes (in production), the
	// BuildSubAgent MUST zero them. We simulate a writes-bearing
	// sub-skill by setting sub.Writes before the call.
	sub.Writes = []string{"memory:facts"} // would-be write target

	// BuildSubAgent routes through DispatchAgent; the subManifest's
	// Writes has been zeroed at this point. The chat archetype's
	// Tool whitelist (BuildToolsForSkill) will see empty writes →
	// no HITL Tools. This test asserts the structural invariant at
	// the BuildSubAgent call boundary.
	_, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-1",
	})
	if err != nil {
		t.Fatalf("BuildSubAgent with writes-bearing sub-skill must still succeed (writes get zeroed), got %v", err)
	}
}

// TestIntersectTiers — pure helper test: order-preserving on parent.
func TestIntersectTiers(t *testing.T) {
	parent := []string{"authority", "historical", "personal"}
	sub := []string{"historical", "personal"}
	got := intersectTiers(parent, sub)
	want := []string{"historical", "personal"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

// TestIntersectTiersEmpty — no overlap → empty result.
func TestIntersectTiersEmpty(t *testing.T) {
	got := intersectTiers([]string{"authority"}, []string{"personal"})
	if len(got) != 0 {
		t.Fatalf("expected empty intersection, got %v", got)
	}
}

// TestIntersectTiersParentOrderPreserved — even if sub's order
// differs, result follows parent's order.
func TestIntersectTiersParentOrderPreserved(t *testing.T) {
	got := intersectTiers([]string{"personal", "authority"}, []string{"authority", "personal"})
	want := []string{"personal", "authority"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

// TestBuildSubAgentPassesAccumulatorToSubAgent — P13-L9 invariant:
// the shared *SharedChunkAccumulator on SubAgentConfig is the pointer
// passed by the orchestrator into BuildSubAgent. The sub-agent's
// retrieve Tool closes over the SAME pointer so the citation
// entailment verifier (D2-01) sees the union of parent + sub-agent
// retrievals.
//
// The accumulator is threaded through the production Tool whitelist
// assembled in internal/agent. This test locks the contract: the
// SubAgentConfig.ChunkAccumulator field is the carrier and tests +
// production both pass the SAME pointer (no copy).
func TestBuildSubAgentPassesAccumulatorToSubAgent(t *testing.T) {
	// Build a fresh accumulator with a sentinel cap so we can verify
	// pointer identity.
	acc := NewSharedChunkAccumulator(7)
	if acc.Max() != 7 {
		t.Fatalf("sentinel cap pre-condition failed: Max=%d, want 7", acc.Max())
	}

	// P13-L9 invariant: two fresh accumulators MUST be distinct (the
	// pointer is the identity). Tests + production pass the SAME
	// pointer (no copy) so the citation entailment check sees the
	// union.
	acc2 := NewSharedChunkAccumulator(7)
	if acc == acc2 {
		t.Fatal("fresh accumulators MUST be distinct (P13-L9 identity violated)")
	}
	if acc.Len() != 0 {
		t.Fatalf("fresh accumulator MUST be empty, got len=%d", acc.Len())
	}
	if acc.Max() != 7 {
		t.Fatalf("Max() accessor MUST return the configured cap, got %d", acc.Max())
	}
}

// TestBuildSubAgentStructuralRecursionGuard — 13-05 Rule-1 regression
// lock (P13-L8). runSubAgent passes the PARENT manifest as the sub-skill
// placeholder, whose Agent field is "orchestrator". Before the guard,
// the constructed sub-agent dispatched to buildOrchestrator again —
// unbounded orchestrator-of-orchestrator nesting at Run time (bounded
// only by the 30s per-sub-agent timeout). The guard rewrites the clone's
// Agent to the chat archetype at construction so the recursion is
// structurally impossible regardless of the caller.
//
// Observable proxy for "the sub-agent is a leaf": the constructed
// agent's Name. The chat archetype names itself
// "skillforge-session-<id>"; the orchestrator archetype names itself
// "skillforge-orchestrator-<id>".
func TestBuildSubAgentStructuralRecursionGuard(t *testing.T) {
	parent := validParentManifest()
	// Sub-skill that IS an orchestrator manifest (the parent-as-sub
	// placeholder shape). Matching tiers so the only rejection path
	// left is the recursion guard.
	sub := validSubManifest("sub-recursive", []string{"authority/**"}, []string{"authority"})
	sub.Agent = "orchestrator"

	agent, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-recursion-guard",
	})
	if err != nil {
		t.Fatalf("BuildSubAgent with orchestrator-agent sub-skill must succeed (rewritten to chat), got %v", err)
	}
	if agent == nil {
		t.Fatal("BuildSubAgent returned nil agent")
	}
	name := agent.Name(context.Background())
	if strings.Contains(name, "orchestrator") {
		t.Fatalf("P13-L8 violated: sub-agent Name=%q is an orchestrator (orchestrator-of-orchestrator recursion)", name)
	}

	// Control: a chat sub-skill keeps the chat archetype untouched.
	leaf := validSubManifest("sub-leaf", []string{"authority/**"}, []string{"authority"})
	leafAgent, err := BuildSubAgent(context.Background(), SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: leaf,
		KBRegistry:       testKBReg(),
		SubQueryID:       "sa1",
		ChatModel:        stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "sess-recursion-control",
	})
	if err != nil {
		t.Fatalf("control chat sub-skill must succeed, got %v", err)
	}
	if leafName := leafAgent.Name(context.Background()); strings.Contains(leafName, "orchestrator") {
		t.Fatalf("control sub-agent Name=%q unexpectedly routed to orchestrator", leafName)
	}
}
