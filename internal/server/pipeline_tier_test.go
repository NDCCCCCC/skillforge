// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Plan 26-05 — the refusal suggestion keyword is registry-derived.
//
// ruleBasedSuggestion's default keyword used to be the hardcoded literal
// "权威制度". The keyword is now built from the registry's first active
// authority-role base: its display name (seed data, the single home of CN
// badge text) plus the fixed keyword suffix. These tests pin both the
// derivation and the graceful-degradation paths.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
)

// TestPipelineSuggestionUsesDisplayName — with no resolved tiers, the
// suggested keyword comes from the registry's first authority-role base's
// DisplayName plus the fixed suffix. With the baseline seed display name
// the output is byte-identical to the pre-registry hardcoded default,
// which is what the frozen eval / FE retry-chip copy depends on.
func TestPipelineSuggestionUsesDisplayName(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})

	got := ruleBasedSuggestion("出差住宿标准是多少", nil, reg)
	assert.Equal(t, "权威制度 出差住宿标准是多少", got,
		"the keyword is DisplayName + the fixed suffix, then the original query")

	// Authority-role membership, not slug spelling: an authority base with
	// another display name changes the keyword accordingly.
	reg2 := kb.New([]kb.Entry{
		{Slug: "policy", DisplayName: "Policy", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	assert.Equal(t, "Policy制度 出差住宿标准是多少", ruleBasedSuggestion("出差住宿标准是多少", nil, reg2))
}

// TestPipelineNilRegistryGraceful — a nil registry (or one without any
// authority-role base) must degrade, never panic: the suggestion falls
// back to the resolved tier slug when there is one, and to the unsuggested
// query when there is nothing to derive a keyword from.
func TestPipelineNilRegistryGraceful(t *testing.T) {
	// Nil registry + no tiers → the query comes back untouched.
	assert.Equal(t, "出差标准", ruleBasedSuggestion("出差标准", nil, nil))

	// Nil registry + resolved tiers → the tier slug is still a usable hint.
	assert.Equal(t, "authority 出差标准", ruleBasedSuggestion("出差标准", []string{"authority"}, nil))

	// Registry with no authority-role base and no tiers → no keyword.
	empty := kb.New([]kb.Entry{
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
	})
	assert.Equal(t, "出差标准", ruleBasedSuggestion("出差标准", nil, empty))

	// A query that already contains the derived keyword is returned as-is
	// (the user already knows the keyword).
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	assert.Equal(t, "权威制度口径", ruleBasedSuggestion("权威制度口径", nil, reg))
}

// TestCitableChunksByID — the verdict-pipeline grounding closure keeps
// only authority-role chunks (KBM-03; the mirror of the agent-side
// accumulator gate). Nil registry = legacy posture (everything enters).
// Lookup failure (unregistered / deactivated slug) excludes the chunk —
// the same fail-closed outcome as a non-citable role.
func TestCitableChunksByID(t *testing.T) {
	chunks := []retriever.Hit{
		{ID: 1, Tier: "authority", Content: "authority clause"},
		{ID: 2, Tier: "historical", Content: "superseded clause"},
		{ID: 3, Tier: "unregistered-slug", Content: "ghost clause"},
	}

	// Nil registry = legacy: everything enters the closure.
	all := citableChunksByID(nil, chunks)
	assert.Len(t, all, 3, "legacy posture keeps every chunk")

	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "历史", Role: kb.RoleHistorical, RetrievalEnabled: true},
	})
	got := citableChunksByID(reg, chunks)
	assert.Len(t, got, 1, "only the authority-role chunk survives the gate")
	assert.Equal(t, "authority clause", got["1"])
	assert.NotContains(t, got, "2", "historical chunks are presentation-only, never evidence")
	assert.NotContains(t, got, "3", "unregistered slugs fail closed")
}
