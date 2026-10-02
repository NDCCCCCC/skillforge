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

package skill

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManifestValidatesOrchestratorAgent — Phase 13 / P13-L1: Agent:
// "orchestrator" is accepted by Validate(). The orchestrator archetype
// is read-only by structural design (subManifest.Writes = [] per
// P13-L4), but the orchestrator's own manifest MAY declare writes:[] to
// signal that the parent Run is read-only too.
func TestManifestValidatesOrchestratorAgent(t *testing.T) {
	m := validManifest()
	m.Agent = "orchestrator"
	m.Writes = []string{} // orchestrator is read-only by structural design
	require.NoError(t, Validate(m, testKBReg()),
		"manifest.Agent=\"orchestrator\" must validate (P13-L1 whitelist extension)")
}

// TestManifestRejectsUnknownAgentWithOrchestratorList — defense-in-depth:
// an unknown agent type is still rejected. The whitelist extension for
// orchestrator must not weaken the rejection of unknown values.
func TestManifestRejectsUnknownAgentWithOrchestratorList(t *testing.T) {
	m := validManifest()
	m.Agent = "bogus-archetype"
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent type",
		"unknown agent type error MUST name the failing field")
}

// TestValidAgentTypesContainsOrchestrator — pins the whitelist contents
// after the Phase 13 extension. Guards against future drift removing
// the orchestrator entry (which would silently break P13-L1 dispatch).
func TestValidAgentTypesContainsOrchestrator(t *testing.T) {
	assert.True(t, validAgentTypes["orchestrator"],
		"validAgentTypes MUST contain \"orchestrator\" per P13-L1 + 13-01 Task 1")
}

// TestValidAgentTypesContainsAllPhase12Archetypes — pin the existing
// chat / deep entries after the orchestrator addition.
func TestValidAgentTypesContainsAllPhase12Archetypes(t *testing.T) {
	for _, name := range []string{"chat", "deep", "orchestrator"} {
		assert.True(t, validAgentTypes[name],
			"validAgentTypes MUST contain canonical Phase 12/13 archetype %q", name)
	}
}

// TestValidAgentTypesRejectsUnknown — sanity check that the whitelist
// is not wide-open.
func TestValidAgentTypesRejectsUnknown(t *testing.T) {
	assert.False(t, validAgentTypes["nonsense"])
}

// TestValidateErrorMessageMentionsOrchestrator — when an unknown agent
// type is provided, the error message lists valid types including the
// new orchestrator entry. Operators reading the error see the full
// menu.
func TestValidateErrorMessageMentionsOrchestrator(t *testing.T) {
	m := validManifest()
	m.Agent = "bogus-archetype"
	err := Validate(m, testKBReg())
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "orchestrator"),
		"the unknown-agent-type error MUST list orchestrator in the valid types")
}

// --- Phase 16 / Plan 16-02 Task 1: reads channel vocabulary + ChannelKind ---

// TestManifestChannel_DataChannel verifies that reads: ["data:aggregated_only"]
// sets HasDataChannel()=true and Channel()==ChannelKindData.
func TestManifestChannel_DataChannel(t *testing.T) {
	m := validManifest()
	m.Reads = []string{ReadDataAggregatedOnly}
	assert.True(t, m.HasDataChannel(),
		"reads [data:aggregated_only] MUST set HasDataChannel()=true")
	assert.Equal(t, ChannelKindData, m.Channel(),
		"reads [data:aggregated_only] MUST yield Channel()==ChannelKindData")
}

// TestManifestChannel_DataChannel_DetectsAnyDataPrefix verifies that
// HasDataChannel() detects any "data:" prefixed value (not just the exact
// ReadDataAggregatedOnly constant). This ensures that unknown data:*
// modes like "data:external:authorized" are routed into the data-channel
// branch of Validate, where the exact-match check produces the correct
// "unknown data channel mode" error — rather than falling through to the
// tier-channel branch and hitting ResolveTiers' defense-in-depth guard.
func TestManifestChannel_DataChannel_DetectsAnyDataPrefix(t *testing.T) {
	m := validManifest()
	m.Reads = []string{"data:external:authorized"}
	assert.True(t, m.HasDataChannel(),
		"reads [data:external:authorized] MUST set HasDataChannel()=true (broad detection)")
	assert.Equal(t, ChannelKindData, m.Channel(),
		"reads [data:external:authorized] MUST yield Channel()==ChannelKindData")
}

// TestManifestChannel_TierChannel verifies that tier reads (the existing
// default) yield HasDataChannel()=false and Channel()==ChannelKindTier.
func TestManifestChannel_TierChannel(t *testing.T) {
	m := validManifest()
	// default manifest has reads: ["authority/**"]
	assert.False(t, m.HasDataChannel(),
		"tier reads MUST NOT set HasDataChannel()")
	assert.Equal(t, ChannelKindTier, m.Channel(),
		"tier reads MUST yield Channel()==ChannelKindTier")
}

// TestManifestChannel_EmptyReads verifies that empty reads yields
// HasDataChannel()=false (the actual rejection of empty reads is
// Validate's job — Channel() just reports the kind).
func TestManifestChannel_EmptyReads(t *testing.T) {
	m := validManifest()
	m.Reads = []string{}
	assert.False(t, m.HasDataChannel(),
		"empty reads MUST NOT set HasDataChannel()")
	assert.Equal(t, ChannelKindTier, m.Channel(),
		"empty reads defaults to ChannelKindTier (Validate rejects empty reads separately)")
}

// TestResolveTiers_DataPrefixDefenseInDepth verifies that ResolveTiers
// rejects "data:" prefixed reads with a defense-in-depth error (the
// normal path never routes data: values into ResolveTiers — Validate
// branches before calling it).
func TestResolveTiers_DataPrefixDefenseInDepth(t *testing.T) {
	_, err := ResolveTiers([]string{"data:aggregated_only"}, testKBReg())
	require.Error(t, err,
		"ResolveTiers MUST reject data: prefixed reads (defense-in-depth)")
	assert.Contains(t, err.Error(), "data:",
		"error MUST name the data: prefix")
	assert.Contains(t, err.Error(), "D16-C2",
		"error MUST cite D16-C2 for traceability")
}

// TestResolveTiers_ExistingTierBehavior_Unchanged is a regression lock:
// the resolved set is still the deduplicated top-segment set, with only
// the value source moved to the registry slugs (Phase 26 / Plan 26-04).
func TestResolveTiers_ExistingTierBehavior_Unchanged(t *testing.T) {
	// Re-run a subset of the delegation cases to prove zero regression.
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"authority", []string{"authority/**"}, []string{"authority"}},
		{"multi_tier", []string{"authority/**", "historical/**"}, []string{"authority", "historical"}},
		{"bare_dir", []string{"authority"}, []string{"authority"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTiers(tc.in, testKBReg())
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
