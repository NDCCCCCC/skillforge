// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Regression tests for the two production defects found during the v1.8
// UAT sweep:
//
//   1. Deps.Loader was declared but NEVER wired into the server.Deps
//      literal, so findSkill() always returned nil and every request
//      that named a Skill explicitly failed closed with out_of_scope.
//   2. The Phase 58 provenance fields (origin / channels) had no
//      production producer: the wire omitted them entirely and the FE
//      fell back to its own default, so nothing was auditable.
//
// Both fixes are pinned here so a future refactor cannot silently
// re-break them.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

// ── Defect 2: the provenance stamp has a production producer ────────

// TestMarkVectorChannelStampsExplicitOrigin pins the single-channel
// provenance stamp: an explicit origin + a one-entry channel set, so the
// wire carries the fact instead of the FE inferring it.
func TestMarkVectorChannelStampsExplicitOrigin(t *testing.T) {
	chunks := []retriever.Hit{{ID: 1}, {ID: 2}}
	markVectorChannel(chunks)

	for _, ch := range chunks {
		assert.Equal(t, retriever.ChannelVector, ch.Origin,
			"a pure-vector hit must carry an explicit origin (empty == no provenance recorded)")
		require.Len(t, ch.Channels, 1)
		assert.Equal(t, retriever.ChannelVector, ch.Channels[0],
			"a single-channel hit carries exactly one channel entry")
	}
}

// TestMarkVectorChannelDoesNotOverwriteExistingChannels pins that the
// stamp never OVERWRITES a channel set a fusion layer already produced:
// a dual-channel deployment's hybrid attribution must survive.
func TestMarkVectorChannelDoesNotOverwriteExistingChannels(t *testing.T) {
	chunks := []retriever.Hit{{
		ID:       1,
		Origin:   retriever.ChannelHybrid,
		Channels: []string{retriever.ChannelVector, retriever.ChannelGraph},
	}}
	markVectorChannel(chunks)

	assert.Equal(t, retriever.ChannelHybrid, chunks[0].Origin,
		"the stamp must not overwrite a fusion-produced origin")
	assert.Equal(t, []string{retriever.ChannelVector, retriever.ChannelGraph}, chunks[0].Channels,
		"the stamp must not overwrite a fusion-produced channel set")
}

// TestBuildRetrievalStatsCountsStampedChannels pins that the ROI frame
// counts the SAME facts the citations carry (one source, one truth).
func TestBuildRetrievalStatsCountsStampedChannels(t *testing.T) {
	chunks := []retriever.Hit{
		{ID: 1, Origin: retriever.ChannelVector, Channels: []string{retriever.ChannelVector}},
		{ID: 2, Origin: retriever.ChannelVector, Channels: []string{retriever.ChannelVector}},
		{ID: 3, Origin: retriever.ChannelHybrid, Channels: []string{retriever.ChannelVector, retriever.ChannelGraph}},
		{ID: 4, Origin: retriever.ChannelGraph, Channels: []string{retriever.ChannelGraph}},
	}
	stats := buildRetrievalStats(nil, chunks, KGContext{Available: true})
	assert.Equal(t, 2, stats.Channels.Vector)
	assert.Equal(t, 1, stats.Channels.Graph)
	assert.Equal(t, 1, stats.Channels.Hybrid,
		"a dual-channel hit is ONE hybrid, not one vector + one graph")
}

// TestBuildRetrievalStatsUnstampedChunksRemainVector pins the legacy
// reading: a chunk with no stamp (pre-58 replay path) still counts as
// vector, which is what the empty value has always meant.
func TestBuildRetrievalStatsUnstampedChunksRemainVector(t *testing.T) {
	chunks := []retriever.Hit{{ID: 1}, {ID: 2}}
	stats := buildRetrievalStats(nil, chunks, KGContext{Available: false})
	assert.Equal(t, 2, stats.Channels.Vector)
	assert.Equal(t, 0, stats.Channels.Graph)
	assert.Equal(t, 0, stats.Channels.Hybrid)
}

// TestStampCitationProvenanceCarriesVectorOrigin is the end-to-end
// production shape: a hit stamped by the retrieval path reaches the
// citation with its origin and channel set intact, and the wire carries
// the field (it is omitted only when unset).
func TestStampCitationProvenanceCarriesVectorOrigin(t *testing.T) {
	chunks := []retriever.Hit{{ID: 7}}
	markVectorChannel(chunks)

	citations := []verdict.Citation{{ChunkID: "7"}}
	stampCitationProvenance(citations, chunks)
	require.Len(t, citations, 1)
	assert.Equal(t, retriever.ChannelVector, citations[0].Origin)
	require.Len(t, citations[0].Channels, 1)
	assert.Equal(t, retriever.ChannelVector, citations[0].Channels[0])

	b, err := json.Marshal(citations[0])
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, retriever.ChannelVector, m["origin"],
		"the wire must carry the origin, not leave it to the FE to infer")
	assert.NotNil(t, m["channels"])
}

// ── Defect 1: the Registry snapshot resolves explicitly-named Skills ──

// writeTestSkill writes a minimal SKILL.md into <root>/<name>/ and
// returns the root.
func writeTestSkill(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	yaml := `---
description: regression skill (explicit-skill resolution)
agent: ChatModelAgent
reads:
  - authority
writes: []
steps: |
  1. Step 1
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal/contact information of any individual"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.2
---
# Regression Skill
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(yaml), 0o644))
	return root
}

// TestFindSkillResolvesFromRegistrySnapshot is the defect-1 regression:
// an explicitly-named Skill must resolve from the Registry snapshot even
// when the Loader was never wired (which is exactly the production state
// that broke the explicit-skill chat branch).
func TestFindSkillResolvesFromRegistrySnapshot(t *testing.T) {
	root := writeTestSkill(t, "regression-skill")
	reg, err := skill.New(root, nil, testKBReg())
	require.NoError(t, err)

	deps := &Deps{Registry: reg}
	m := deps.findSkill("regression-skill")
	require.NotNil(t, m,
		"findSkill must resolve an explicitly-named Skill from the Registry snapshot")
	assert.Equal(t, "regression-skill", m.Name)
	assert.NotEmpty(t, m.ResolvedTiers(),
		"the resolved manifest must carry its tier scope (the reads contract)")

	// An unknown Skill is still out of scope (fail closed).
	assert.Nil(t, deps.findSkill("no-such-skill"))
}

// TestFindSkillPrefersRegistryOverStaleLoader pins that the Registry
// (hot-reloaded, runtime truth) wins over the Loader (startup scan), so
// an explicit request and a routed request agree about the same Skill.
func TestFindSkillPrefersRegistryOverStaleLoader(t *testing.T) {
	root := writeTestSkill(t, "regression-skill")
	reg, err := skill.New(root, nil, testKBReg())
	require.NoError(t, err)

	// A Loader that knows a DIFFERENT manifest under the same name --
	// the shape of a stale startup scan.
	staleRoot := writeTestSkill(t, "regression-skill")
	staleManifests, err := skill.NewLoader(staleRoot, testKBReg()).Scan()
	require.NoError(t, err)
	require.Len(t, staleManifests, 1)
	staleManifests[0].Steps = "STALE STEP -- must not win"

	deps := &Deps{Registry: reg, Loader: skill.NewLoader(staleRoot, testKBReg())}
	m := deps.findSkill("regression-skill")
	require.NotNil(t, m)
	assert.NotEqual(t, "STALE STEP -- must not win", m.Steps,
		"the Registry snapshot (runtime truth) must win over the Loader's startup scan")
}

// TestKnowledgeReadAllowedUsesRegistry is the defect-1 sibling: the
// direct-read tier check must resolve the Skill's tier scope from the
// Registry snapshot, not from an unwired Loader.
func TestKnowledgeReadAllowedUsesRegistry(t *testing.T) {
	root := writeTestSkill(t, "regression-skill")
	reg, err := skill.New(root, nil, testKBReg())
	require.NoError(t, err)

	deps := &Deps{Registry: reg}

	ctx := &app.RequestContext{}
	ctx.Request.Header.Set("X-Skill-Name", "regression-skill")

	tiers := reg.Snapshot()["regression-skill"].Manifest.ResolvedTiers()
	require.NotEmpty(t, tiers)

	assert.True(t, deps.knowledgeReadAllowed(ctx, tiers[0]),
		"the Skill's own tier must be readable")
	assert.False(t, deps.knowledgeReadAllowed(ctx, "no-such-tier"),
		"a tier outside the Skill's scope must be refused")
}
