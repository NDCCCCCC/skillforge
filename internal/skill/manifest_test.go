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
	require.NoError(t, Validate(m),
		"manifest.Agent=\"orchestrator\" must validate (P13-L1 whitelist extension)")
}

// TestManifestRejectsUnknownAgentWithOrchestratorList — defense-in-depth:
// an unknown agent type is still rejected. The whitelist extension for
// orchestrator must not weaken the rejection of unknown values.
func TestManifestRejectsUnknownAgentWithOrchestratorList(t *testing.T) {
	m := validManifest()
	m.Agent = "bogus-archetype"
	err := Validate(m)
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
	err := Validate(m)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "orchestrator"),
		"the unknown-agent-type error MUST list orchestrator in the valid types")
}