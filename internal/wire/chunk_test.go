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
// must stay in lockstep) is exercised by the TestValidTypes_Nineteen
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
//     produces JSON with "type":"data-decision" + non-empty data.
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
			"step":       1,
			"decision":   "retrieve",
			"toolCallId": "call_abc",
			"query":      "差旅费报销制度",
			"confidence": 0.0,
		}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)

		c := UIMessageChunk{Type: "data-decision", Data: raw}
		out, err := json.Marshal(c)
		require.NoError(t, err)

		var got struct {
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
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

// TestValidTypes_Twenty — count assertion, extended by quick plan
// 260815-0xb: the FEUI-06 `data-subprogress` streaming-progress type
// became the 20th wire type when the VerdictPipeline started emitting
// per-stage progress chunks (retrieve / found_candidates / generate /
// self_verify). History: Phase 4 closed with 17 entries; Phase 5 added
// `data-redline` (18th, D5-14); the message-level `finish` terminator
// became the 19th when VerdictPipeline emitted the full AI SDK v5
// lifecycle.
//
// The plan FORBIDS additional types beyond this set (the router /
// counter-evidence / redline / subprogress pipeline all use additive
// fields on existing chunks, not new discriminators).
//
// RED phase (260815-0xb): FAILS before the change because
// `data-subprogress` is not yet in ValidTypes (counts 19, asserts 20).
// The GREEN commit registers the 20th type.
//
// Quick plan 260815-cos extends to 22 types — adds
// `text-delta-draft` + `text-delta-draft-end` for the LLM-token
// streaming channel that the DraftStreamChip listens to (FE renders 🟡
// 待校验 → ✅ 已验证 on grounding pass). Same T-1-11 rule applies
// (snake_case variants stay rejected).
func TestValidTypes_Twenty(t *testing.T) {
	const expected = 22
	assert.Equal(t, expected, len(ValidTypes),
		"ValidTypes must hold exactly %d entries (20 through data-subprogress, +2 for text-delta-draft + text-delta-draft-end per 260815-cos)", expected)
	assert.True(t, ValidTypes["data-redline"],
		"data-redline MUST be registered (18th wire type, D5-14)")
	assert.True(t, ValidTypes["finish"],
		"finish MUST be registered — VerdictPipeline emits it as the message terminator (AI SDK v5 schema member)")
	assert.True(t, ValidTypes["data-subprogress"],
		"data-subprogress MUST be registered (20th wire type, FEUI-06 / 260815-0xb)")
	assert.True(t, ValidTypes["text-delta-draft"],
		"text-delta-draft MUST be registered (21st wire type, draft streaming channel / 260815-cos)")
	assert.True(t, ValidTypes["text-delta-draft-end"],
		"text-delta-draft-end MUST be registered (22nd wire type, draft streaming end marker / 260815-cos)")
	// Snake-case variant rejected — T-1-11 mitigation.
	assert.False(t, ValidTypes["data_redline"],
		"snake_case data_redline MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["data_subprogress"],
		"snake_case data_subprogress MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["text_delta_draft"],
		"snake_case text_delta_draft MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["text_delta_draft_end"],
		"snake_case text_delta_draft_end MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
}

// TestSubprogressStepRoundTrip — quick plan 260815-0xb Task 1: the
// wire.SubprogressStep payload mirrors the FE SubprogressStep interface
// (web/src/types/verdict.ts) field-for-field. Every step the
// VerdictPipeline emits must round-trip its discriminator + tier/count
// so DataSubprogressPart renders the right Copywriting-Contract line.
//
// RED phase: FAILS before the change because wire.SubprogressStep does
// not exist yet.
func TestSubprogressStepRoundTrip(t *testing.T) {
	steps := []SubprogressStep{
		{Step: "retrieve", Tier: "权威"},
		{Step: "found_candidates", Tier: "权威", Count: 5},
		{Step: "generate"},
		{Step: "self_verify"},
	}
	for _, p := range steps {
		raw, err := json.Marshal(p)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, p.Step, got["step"],
			"step discriminator %q must round-trip", p.Step)
		if p.Tier != "" {
			assert.Equal(t, p.Tier, got["tier"],
				"tier must round-trip for %q (FE renders 检索 [tier] 知识层)", p.Step)
		}
		if p.Count != 0 {
			assert.InDelta(t, float64(p.Count), got["count"].(float64), 1e-9,
				"count must round-trip for %q (FE renders 找到 N 条候选)", p.Step)
		}
	}
	// omitempty contract: generate/self_verify carry neither tier nor
	// count — the wire payload must not bloat with zero-value fields.
	lean, err := json.Marshal(SubprogressStep{Step: "generate"})
	require.NoError(t, err)
	assert.Equal(t, `{"step":"generate"}`, string(lean),
		"omitempty required on tier/count so non-retrieval steps stay minimal")
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
		Selected:  "compliance",
		Ambiguous: false,
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
		Step:       1,
		Decision:   "retrieve",
		ToolCallID: "call_1",
		Query:      "差旅费",
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
