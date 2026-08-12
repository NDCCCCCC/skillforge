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

// Package wire test — Plan 03-01 Task 1 (TDD).
//
// TestDecisionChunkType pins the ARAG-01 / SC-1 observability substrate:
// the data-decision discriminator must be registerable in ValidTypes so
// IsValidType accepts it (T-1-11 mitigation preserved), its snake_case
// variant MUST be rejected (regression for the silent-drop bug), and
// marshalling a data-decision chunk round-trips the type discriminator
// plus the DecisionPayload on the .Data field.
//
// RED phase (this commit): subtest #1 ("valid") fails because
// "data-decision" is not yet in ValidTypes. Subtests #2 ("snake_case")
// and #3 ("round_trip") pass independently of the registry change but
// are committed together as the contract surface.
//
// Plan 05-01 Task 1 (TDD): extends the wire contract — DecisionPayload
// gains additive routing fields (D5-04) and the 18th wire type
// `data-redline` is registered (D5-14). The D15 drift alarm (TS mirror
// must stay in lockstep) is exercised by the TestValidTypes_Eighteen
// counter + the npm tsc --noEmit gate run from the orchestrator.

package wire

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecisionChunkType covers Task 1 behavior:
//   - Test 1 (valid):         IsValidType accepts "data-decision".
//   - Test 2 (snake_case):    IsValidType rejects "data_decision" (T-1-11 regression).
//   - Test 4 (round_trip):    marshalling UIMessageChunk{Type:"data-decision", Data:...}
//                             produces JSON with "type":"data-decision" + non-empty data.
func TestDecisionChunkType(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c := UIMessageChunk{Type: "data-decision"}
		assert.True(t, c.IsValidType(),
			"data-decision must be registered in ValidTypes so EmitChunk accepts it (ARAG-01 / SC-1)")
	})

	t.Run("snake_case_rejected", func(t *testing.T) {
		c := UIMessageChunk{Type: "data_decision"}
		assert.False(t, c.IsValidType(),
			"snake_case data_decision MUST be rejected — the AI SDK v5 parser silently drops it (T-1-11)")
	})

	t.Run("round_trip", func(t *testing.T) {
		// DecisionPayload per 03-RESEARCH.md §Code Examples lines 670-676.
		// Built as raw JSON here so the test does not depend on a
		// concrete Go struct that may move between packages; the GREEN
		// commit lands the exported wire.DecisionPayload alongside.
		payload := map[string]any{
			"step":        1,
			"decision":    "retrieve",
			"toolCallId":  "call_abc",
			"query":       "差旅费报销制度",
			"confidence":  0.0,
		}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)

		c := UIMessageChunk{Type: "data-decision", Data: raw}
		out, err := json.Marshal(c)
		require.NoError(t, err)

		var got struct {
			Type string            `json:"type"`
			Data map[string]any    `json:"data"`
		}
		require.NoError(t, json.Unmarshal(out, &got))
		assert.Equal(t, "data-decision", got.Type,
			"marshalled type discriminator must be exactly data-decision")
		assert.NotEmpty(t, got.Data,
			"Data field must round-trip non-empty so the React UI receives the decision payload")
		assert.Equal(t, "retrieve", got.Data["decision"],
			"decision field must survive the round-trip (D3-03 / SC-1 observability)")
	})
}

// TestValidTypes_Eighteen — Plan 05-01 Task 1: count assertion on the
// ValidTypes map. Phase 4 closed with 17 entries; Phase 5 adds exactly
// ONE (`data-redline`, D5-14). The plan FORBIDS additional types (the
// router/counter-evidence/redline-plan pipeline all use additive fields
// on existing chunks, not new discriminators).
//
// RED phase: this test FAILS today because `data-redline` is not yet
// in ValidTypes (counts 17, asserts 18). The GREEN commit registers
// the 18th type.
func TestValidTypes_Eighteen(t *testing.T) {
	const expected = 18
	assert.Equal(t, expected, len(ValidTypes),
		"ValidTypes must hold exactly %d entries after Plan 05-01 (was 17, +1 for data-redline)", expected)
	assert.True(t, ValidTypes["data-redline"],
		"data-redline MUST be registered as the 18th wire type (D5-14)")
	// Snake-case variant rejected — T-1-11 mitigation.
	assert.False(t, ValidTypes["data_redline"],
		"snake_case data_redline MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
}

// TestDecisionPayload_RouteFields — Plan 05-01 Task 1: DecisionPayload
// round-trip with the additive D5-04 routing fields. The fields are
// omitempty so non-routing decisions do not bloat the wire; a routing
// decision carries the full skillScores slice + selected + ambiguous.
//
// RED phase: FAILS today because SkillScores/Selected/Ambiguous are
// not yet on DecisionPayload.
func TestDecisionPayload_RouteFields(t *testing.T) {
	payload := DecisionPayload{
		Step:     1,
		Decision: "route",
		SkillScores: []SkillScore{
			{SkillID: "compliance", SkillName: "Compliance Review", Score: 0.91, Selected: true},
			{SkillID: "policy-lookup", SkillName: "Policy Lookup", Score: 0.42, Selected: false},
		},
		Selected:   "compliance",
		Ambiguous:  false,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got DecisionPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "route", got.Decision,
		"decision='route' MUST survive round-trip (D5-04)")
	require.Len(t, got.SkillScores, 2,
		"skillScores MUST round-trip with both entries intact (D5-04)")
	assert.Equal(t, "compliance", got.SkillScores[0].SkillID)
	assert.Equal(t, "Compliance Review", got.SkillScores[0].SkillName)
	assert.InDelta(t, 0.91, got.SkillScores[0].Score, 1e-9)
	assert.True(t, got.SkillScores[0].Selected)
	assert.Equal(t, "policy-lookup", got.SkillScores[1].SkillID)
	assert.False(t, got.SkillScores[1].Selected)
	assert.Equal(t, "compliance", got.Selected,
		"selected skill ID MUST survive round-trip (D5-04)")
	assert.False(t, got.Ambiguous,
		"ambiguous flag MUST round-trip (D5-04)")
}

// TestDecisionPayload_BackwardCompatible — Plan 05-01 Task 1: the
// original 5-field DecisionPayload shape (no skillScores/selected/
// ambiguous) MUST still unmarshal cleanly. Additive fields with
// omitempty preserve backward compatibility; if the GREEN commit
// accidentally made the new fields required, this test fails.
func TestDecisionPayload_BackwardCompatible(t *testing.T) {
	// Simulate a Phase-3/4 client (or stored decision JSON) that has
	// only the 5 original fields.
	raw := []byte(`{"step":2,"decision":"answer","toolCallId":"call_xyz","query":"差旅费","confidence":0.87}`)
	var got DecisionPayload
	require.NoError(t, json.Unmarshal(raw, &got),
		"legacy 5-field DecisionPayload MUST still unmarshal (backward compatibility)")
	assert.Equal(t, 2, got.Step)
	assert.Equal(t, "answer", got.Decision)
	assert.Equal(t, "call_xyz", got.ToolCallID)
	assert.Equal(t, "差旅费", got.Query)
	assert.InDelta(t, 0.87, got.Confidence, 1e-9)
	assert.Empty(t, got.SkillScores,
		"missing skillScores MUST decode to empty slice (NOT error)")
	assert.Empty(t, got.Selected,
		"missing selected MUST decode to empty string (NOT error)")
	assert.False(t, got.Ambiguous,
		"missing ambiguous MUST decode to false (NOT error)")
}

// TestDecisionPayload_NonRouteOmitsRoutingFields — Plan 05-01 Task 1:
// non-routing decisions MUST NOT bloat the wire with empty routing
// fields. omitempty on JSON tags is required; if a GREEN commit
// forgets omitempty, this test fails because the wire payload carries
// `"skillScores":null` etc.
func TestDecisionPayload_NonRouteOmitsRoutingFields(t *testing.T) {
	payload := DecisionPayload{
		Step:     1,
		Decision: "retrieve",
		ToolCallID: "call_1",
		Query:    "差旅费",
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	s := string(raw)
	assert.False(t, strings.Contains(s, "skillScores"),
		"non-routing decision MUST omit skillScores (omitempty required): %s", s)
	assert.False(t, strings.Contains(s, `"selected"`),
		"non-routing decision MUST omit selected (omitempty required): %s", s)
	assert.False(t, strings.Contains(s, `"ambiguous"`),
		"non-routing decision MUST omit ambiguous (omitempty required): %s", s)
}
