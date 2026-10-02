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
//
// Phase 12 / Plan 12-02 extends to 23 types — adds `data-plan-trace`
// (P12-L6 new discriminator, NOT an overload of data-decision). It
// carries the DeepAgent write_todos snapshot on every Tool call so
// the React UI can render the live planning trace (PLAN-02 BE
// substrate). The TS mirror ships in 12-06 under the D15 lockstep
// invariant — this count bump is the BE half of the paired commit.
//
// Phase 29 / Plan 29-04 extends to 24 types — adds `data-turn`
// (FBK-01 server-side message identity). Registered in the SAME commit
// as the TS union member (D15 paired-commit invariant); the frame is
// emitted after writeAuditAndTurn and before the terminal frames, so
// refusal rounds carry a vote key too (Open Q3).
func TestValidTypes_Twenty(t *testing.T) {
	const expected = 24
	assert.Equal(t, expected, len(ValidTypes),
		"ValidTypes must hold exactly %d entries (23 through Plan 12-02, +1 for data-turn per Plan 29-04)", expected)
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
	assert.True(t, ValidTypes["data-plan-trace"],
		"data-plan-trace MUST be registered (23rd wire type, PLAN-02 live todo stream substrate, P12-L6 / Plan 12-02)")
	assert.True(t, ValidTypes["data-turn"],
		"data-turn MUST be registered (24th wire type, FBK-01 turn identity, Plan 29-04)")
	// Snake-case variant rejected — T-1-11 mitigation.
	assert.False(t, ValidTypes["data_redline"],
		"snake_case data_redline MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["data_subprogress"],
		"snake_case data_subprogress MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["text_delta_draft"],
		"snake_case text_delta_draft MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["text_delta_draft_end"],
		"snake_case text_delta_draft_end MUST NOT be in ValidTypes (T-1-11 silent-drop regression)")
	assert.False(t, ValidTypes["data_plan_trace"],
		"snake_case data_plan_trace MUST NOT be in ValidTypes (T-1-11 silent-drop regression; Plan 12-02)")
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

// TestValidTypesContainsDataPlanTrace — Plan 12-02 Task 1: the
// `data-plan-trace` discriminator MUST be registered in ValidTypes so
// EmitChunk accepts it (T-1-11 mitigation preserved — the same shape
// as TestDecisionChunkType#valid). Phase 12's DeepAgent
// writeTodosAuditedTool emits one of these chunks per write_todos
// call (PLAN-02 BE substrate). The TS mirror ships in 12-06 under the
// D15 lockstep invariant.
//
// RED phase: FAILS before the GREEN commit because
// `data-plan-trace` is not yet in ValidTypes. The GREEN commit
// registers the 23rd type per TestValidTypes_Twenty.
func TestValidTypesContainsDataPlanTrace(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c := UIMessageChunk{Type: "data-plan-trace"}
		assert.True(t, c.IsValidType(),
			"data-plan-trace must be registered in ValidTypes so EmitChunk accepts it (PLAN-02 live todo stream substrate)")
	})

	t.Run("snake_case_rejected", func(t *testing.T) {
		c := UIMessageChunk{Type: "data_plan_trace"}
		assert.False(t, c.IsValidType(),
			"snake_case data_plan_trace MUST be rejected — the AI SDK v5 parser silently drops it (T-1-11)")
	})
}

// TestPlanTracePayloadRoundTrip — Plan 12-02 Task 1: the wire.PlanTracePayload
// payload mirrors the FE SkillForgeChunk.data-plan-trace variant
// (web/src/types/wire-types.ts) field-for-field. Every writeTodosAuditedTool
// call (PLAN-02 + PLAN-05 substrate) round-trips its runId / todos / ts
// so the React UI can render the live planning trace AND the
// QueryPlanTodos replay reconstruction can rebuild the same view from
// the audit table.
//
// RED phase: FAILS before the change because wire.PlanTracePayload does
// not exist yet.
func TestPlanTracePayloadRoundTrip(t *testing.T) {
	payload := PlanTracePayload{
		RunID: "r1",
		Todos: []PlanTodo{
			{ID: "t1", Content: "step 1", Status: "pending"},
			{ID: "t2", Content: "step 2", ActiveForm: "doing step 2", Status: "in_progress"},
			{ID: "t3", Content: "step 3", Status: "completed"},
		},
		Ts: 1234567890,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got PlanTracePayload
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, "r1", got.RunID,
		"runId must round-trip (FE scopes the rendered trace to one conversation)")
	require.Len(t, got.Todos, 3,
		"todos slice must round-trip with all 3 entries intact (PLAN-02 live todo stream substrate)")
	assert.Equal(t, "t1", got.Todos[0].ID)
	assert.Equal(t, "step 1", got.Todos[0].Content)
	assert.Equal(t, "pending", got.Todos[0].Status)
	assert.Equal(t, "t2", got.Todos[1].ID)
	assert.Equal(t, "doing step 2", got.Todos[1].ActiveForm,
		"activeForm (omitempty) must round-trip when present — FE renders the active step indicator")
	assert.Equal(t, "in_progress", got.Todos[1].Status)
	assert.Equal(t, "t3", got.Todos[2].ID)
	assert.Equal(t, "completed", got.Todos[2].Status)
	assert.Equal(t, int64(1234567890), got.Ts,
		"ts (epoch-seconds) must round-trip (FE renders the trace timestamp)")

	// ActiveForm omitempty: a todo without activeForm must NOT bloat the
	// wire with "activeForm":"".
	lean, err := json.Marshal(PlanTodo{ID: "t1", Content: "step 1", Status: "pending"})
	require.NoError(t, err)
	assert.Equal(t, `{"id":"t1","content":"step 1","status":"pending"}`, string(lean),
		"omitempty required on activeForm so non-active todos stay minimal")
}

// TestDataPlanTraceChunkDispatch — Plan 12-02 Task 1: assert that a
// UIMessageChunk with Type="data-plan-trace" can be classified via
// ValidTypes[chunk.Type] AND that the Data field round-trips a
// PlanTracePayload JSON body. Mirrors TestDecisionChunkType#round_trip
// for the data-decision discriminator.
//
// RED phase: FAILS before the change because the type isn't registered
// and the payload type doesn't exist.
func TestDataPlanTraceChunkDispatch(t *testing.T) {
	payload := PlanTracePayload{
		RunID: "sess-trace-1",
		Todos: []PlanTodo{{ID: "t1", Content: "collect policy clauses", Status: "in_progress"}},
		Ts:    1700000000,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	c := UIMessageChunk{Type: "data-plan-trace", Data: raw}
	assert.True(t, c.IsValidType(),
		"data-plan-trace chunk must be accepted (ValidTypes entry exists)")

	out, err := json.Marshal(c)
	require.NoError(t, err)

	var got struct {
		Type string           `json:"type"`
		Data PlanTracePayload `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "data-plan-trace", got.Type,
		"marshalled type discriminator must be exactly data-plan-trace (P12-L6 + D15 lockstep)")
	assert.Equal(t, "sess-trace-1", got.Data.RunID,
		"data.runId must round-trip (FE scopes the trace by runId)")
	require.Len(t, got.Data.Todos, 1,
		"data.todos must round-trip with all entries (PLAN-02 live trace)")
	assert.Equal(t, "collect policy clauses", got.Data.Todos[0].Content)
}

// ---------------------------------------------------------------------
// Plan 12-05 Task 4 — P12-L3 forced_stop extension + ValidTypes
// invariants.
// ---------------------------------------------------------------------

// TestForcedStopDataDecisionRouting — Plan 12-05 Task 4: forced_stop
// rides the EXISTING data-decision discriminator (D16 overload); the
// DecisionPayload.Decision string field accepts the new value
// byte-identical through JSON. The reason rides the Query field per
// the Phase 10 D10-C3 transparency convention (no new Reason field —
// keeping the wire union untouched avoids churning the FE D15 mirror
// for a parsing-shape that already works).
//
// Mirrors TestDecisionChunkType#round_trip with the budget-gate
// payload shape. The chunk Type discriminator is unchanged.
func TestForcedStopDataDecisionRouting(t *testing.T) {
	const reason = "PLAN-04 budget exhausted: token (used 2000 > budget 1500)"

	payload := DecisionPayload{
		Step:     0, // budget gates are Step=0 (operator events, not agent hops)
		Decision: DecisionForcedStop,
		Query:    reason,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	// The chunk uses the EXISTING discriminator (P12-L3 lock — D15 wire
	// union is untouched).
	c := UIMessageChunk{Type: "data-decision", Data: raw}
	require.True(t, c.IsValidType(),
		"forced_stop MUST ride the existing discriminator — the chunk type is unchanged (P12-L3 lock)")

	out, err := json.Marshal(c)
	require.NoError(t, err)

	var got struct {
		Type string          `json:"type"`
		Data DecisionPayload `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "data-decision", got.Type,
		"the type discriminator MUST stay data-decision — forced_stop is a new VALUE in the Decision string enum, not a new chunk type")
	assert.Equal(t, DecisionForcedStop, got.Data.Decision,
		"decision field MUST equal wire.DecisionForcedStop (12-05)")
	assert.Equal(t, reason, got.Data.Query,
		"reason MUST round-trip via the Query field (Phase 10 D10-C3 transparency convention)")
	assert.Equal(t, 0, got.Data.Step,
		"budget-gate forced_stop is a Step=0 operator event, not a numbered agent hop")

	// The constant is the literal "forced_stop" — locks the wire
	// contract value (changing it would break the FE parser silently).
	assert.Equal(t, "forced_stop", DecisionForcedStop,
		"wire.DecisionForcedStop MUST be the literal \"forced_stop\" — the wire-contract value")
}

// TestDataDecisionFanoutEnum — Phase 13 / Plan 13-03 Task 1: the
// `data-decision` discriminator's `decision` STRING enum extends by
// 3 values (fanout_dispatched | fanout_completed | fanout_failed).
// D16 overload pattern (no new wire chunk types — P13-L7 invariant).
//
// The new fields RunPath / TransferFrom / TransferTo on DecisionPayload
// round-trip byte-identical through JSON. The chunk Type stays
// data-decision (P13-L7 / D-04 transport-swap preserved). The D15
// lockstep invariant is exercised via the matching TS mirror update
// in web/src/types/wire-types.ts (npx tsc --noEmit exits 0).
//
// RED before this commit: the DecisionPayload struct does not have
// RunPath / TransferFrom / TransferTo fields; the test fails to
// compile.
func TestDataDecisionFanoutEnum(t *testing.T) {
	cases := []struct {
		name        string
		decision    string
		runPath     string
		transferFrom string
		transferTo  string
	}{
		{"fanout_dispatched", "fanout_dispatched", "parent/sa1", "parent", "sa1"},
		{"fanout_completed", "fanout_completed", "parent/sa2", "parent", "sa2"},
		{"fanout_failed", "fanout_failed", "parent/sa3", "parent", "sa3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := DecisionPayload{
				Step:         0, // fanout events are operator events, not numbered agent hops
				Decision:     tc.decision,
				RunPath:      tc.runPath,
				TransferFrom: tc.transferFrom,
				TransferTo:   tc.transferTo,
				Query:        "fanout lifecycle event",
			}
			raw, err := json.Marshal(payload)
			require.NoError(t, err)

			// The chunk uses the EXISTING discriminator (P13-L7 lock —
			// D15 wire union is untouched).
			c := UIMessageChunk{Type: "data-decision", Data: raw}
			require.True(t, c.IsValidType(),
				"fanout_* MUST ride the existing discriminator — the chunk type is unchanged (P13-L7 lock)")

			out, err := json.Marshal(c)
			require.NoError(t, err)

			var got struct {
				Type string          `json:"type"`
				Data DecisionPayload `json:"data"`
			}
			require.NoError(t, json.Unmarshal(out, &got))
			assert.Equal(t, "data-decision", got.Type,
				"the type discriminator MUST stay data-decision — fanout_* are new VALUES in the Decision string enum, not new chunk types")
			assert.Equal(t, tc.decision, got.Data.Decision,
				"decision field MUST equal wire.DecisionFanout* (P13-L7)")
			assert.Equal(t, tc.runPath, got.Data.RunPath,
				"runPath field MUST round-trip (P13-L7 audit RunPath substrate)")
			assert.Equal(t, tc.transferFrom, got.Data.TransferFrom,
				"transferFrom field MUST round-trip (P13-L7 audit TransferFrom substrate)")
			assert.Equal(t, tc.transferTo, got.Data.TransferTo,
				"transferTo field MUST round-trip (P13-L7 audit TransferTo substrate)")
		})
	}

	// The catalog constants are the literal strings — locks the wire
	// contract value (changing any would break the FE parser silently).
	assert.Equal(t, "fanout_dispatched", DecisionFanoutDispatched,
		"wire.DecisionFanoutDispatched MUST be the literal \"fanout_dispatched\"")
	assert.Equal(t, "fanout_completed", DecisionFanoutCompleted,
		"wire.DecisionFanoutCompleted MUST be the literal \"fanout_completed\"")
	assert.Equal(t, "fanout_failed", DecisionFanoutFailed,
		"wire.DecisionFanoutFailed MUST be the literal \"fanout_failed\"")
}

// TestValidTypesUnchangedAfterFanout — Phase 13 / Plan 13-03 Task 1:
// the FAN-01 SC-1 sub-agent lifecycle adds 3 new VALUES in the
// DecisionPayload.Decision string enum (fanout_dispatched,
// fanout_completed, fanout_failed) but does NOT add a new wire chunk
// discriminator. The ValidTypes count MUST stay at the Plan 12-05
// baseline (23 entries); the BE half of the D15 lockstep invariant.
//
// The duplicate count assertion guards against accidental additions
// to ValidTypes in a future phase. Adding a new discriminator
// silently would break the D15 TS mirror (AssertAssignable fails to
// compile) AND this test would catch the drift at the BE level first.
func TestValidTypesUnchangedAfterFanout(t *testing.T) {
	assert.Equal(t, 24, len(ValidTypes),
		"ValidTypes MUST stay at 24 entries (23 through 13-03 fanout, +1 for data-turn per Plan 29-04); fanout_* are new VALUES in the Decision string enum, NOT new chunk types")
	assert.True(t, ValidTypes["data-decision"],
		"data-decision MUST remain in ValidTypes (fanout_* rides it, P13-L7)")
	// Hypothetical new discriminator like 'data-fanout' must NOT exist.
	assert.False(t, ValidTypes["data-fanout"],
		"kebab-case variants of a hypothetical new discriminator MUST NOT exist (P13-L7 lock)")
	assert.False(t, ValidTypes["data_fanout"],
		"snake_case data_fanout MUST NOT be in ValidTypes — would have been the rejected counterpart of a hypothetical new discriminator (P13-L7 / T-1-11)")
}

// TestValidTypesUnchangedAfterBudgetGate — Plan 12-05 Task 4: the
// P12-L3 invariant. The 3-layer budget gate adds a new VALUE in the
// DecisionPayload.Decision string enum but does NOT add a new wire
// chunk discriminator. The ValidTypes count MUST stay at the
// Plan 12-02 baseline (23 — see TestValidTypes_Twenty); the BE half
// of the D15 lockstep invariant.
//
// The duplicate count assertion guards against accidental additions
// to ValidTypes in a future phase. Adding a new discriminator
// silently would break the D15 TS mirror (AssertAssignable fails to
// compile) AND this test would catch the drift at the BE level first.
func TestValidTypesUnchangedAfterBudgetGate(t *testing.T) {
	assert.Equal(t, len(ValidTypes), 24,
		"ValidTypes MUST stay at 24 entries (23 through the 12-05 budget gate, +1 for data-turn per Plan 29-04); forced_stop is a new VALUE in the Decision string enum, NOT a new chunk type")
	assert.True(t, ValidTypes["data-decision"],
		"data-decision MUST remain in ValidTypes (forced_stop rides it, P12-L3)")
	// Budget-trigger chunks would have used a hypothetical new
	// discriminator like `data-budget` — assert it does NOT exist.
	assert.False(t, ValidTypes["data-budget"],
		"snake_case / kebab-case variants of a hypothetical new discriminator MUST NOT exist (P12-L3 lock)")
	assert.False(t, ValidTypes["data_budget"],
		"snake_case data_budget MUST NOT be in ValidTypes — would have been the rejected counterpart of a hypothetical new discriminator (P12-L3 / T-1-11)")
}

// TestDecisionDataRefWireReferences — Phase 17 / Plan 17-01 Task 1: the
// DataReference.References []PointReference field (D17-B2) carries per-point
// sparkline metadata. Serializing a DataReference with References populated,
// deserializing, and asserting PointIndex/Period/Value round-trips correctly.
// ValidTypes count MUST stay at 23 (field addition, not union variant).
func TestDecisionDataRefWireReferences(t *testing.T) {
	refs := []PointReference{
		{PointIndex: 0, Period: "2026-05", Value: float64(4231)},
		{PointIndex: 1, Period: "2026-06", Value: float64(4198)},
		{PointIndex: 2, Period: "2026-07", Value: float64(4150)},
	}
	dr := DataReference{
		Func:        "loan_overdue_rate",
		Args:        map[string]any{"status": "逾期"},
		ASOfPeriod:  "2026-07",
		ASOfTs:      "2026-07-15T10:30:00Z",
		RowCount:    3,
		Suppressed:  0,
		KMinApplied: 5,
		References:  refs,
	}
	raw, err := json.Marshal(dr)
	require.NoError(t, err)
	s := string(raw)
	assert.Contains(t, s, `"references"`, "marshal MUST contain references field (D17-B2)")
	assert.Contains(t, s, `"pointIndex"`, "marshal MUST contain pointIndex field")
	assert.Contains(t, s, `"period"`, "marshal MUST contain period field")
	assert.Contains(t, s, `"value"`, "marshal MUST contain value field")
	assert.Contains(t, s, `"2026-05"`, "first period MUST round-trip")
	assert.Contains(t, s, `"2026-07"`, "last period MUST round-trip")

	// Unmarshal round-trip.
	var got DataReference
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Len(t, got.References, 3, "References slice MUST have 3 entries after round-trip")
	assert.Equal(t, 0, got.References[0].PointIndex)
	assert.Equal(t, "2026-05", got.References[0].Period)
	assert.Equal(t, float64(4231), got.References[0].Value)
	assert.Equal(t, 2, got.References[2].PointIndex)
	assert.Equal(t, "2026-07", got.References[2].Period)
	assert.Equal(t, float64(4150), got.References[2].Value)

	// ValidTypes count unchanged (field addition, not union variant).
	assert.Equal(t, 24, len(ValidTypes),
		"ValidTypes MUST stay at 24 after D17-B2 References addition (field, not variant; 23 through Plan 12-02 + data-turn per Plan 29-04)")
}

// TestDecisionDataRefWire — Phase 16 / Plan 16-04 Task 1: the DecisionPayload
// additive DataRef field (D16-B3) carries structured data-citation metadata
// on kpi_reference decisions. The data-decision discriminator is unchanged
// (D16 overload pattern, same as P13-L7 fanout_*). rows body is NOT carried
// on SSE (volume control + agent already has the full snapshot from tool
// result; FE Phase 17 sparkline reads from kpi_calls audit rows). The
// ValidTypes count MUST stay at 23 (no new wire chunk types).
func TestDecisionDataRefWire(t *testing.T) {
	t0 := "2026-07-15T10:30:00Z"

	// 1. Marshal with DataRef present — must contain all expected fields.
	payload := DecisionPayload{
		Step:     1,
		Decision: DecisionKPIReference,
		DataRef: &DataReference{
			Func:        "loan_overdue_rate_by_status",
			Args:        map[string]any{"status": "逾期"},
			ASOfPeriod:  "2026-07",
			ASOfTs:      t0,
			RowCount:    7,
			Suppressed:  2,
			KMinApplied: 10,
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	s := string(raw)
	assert.Contains(t, s, `"dataRef"`,
		"marshal MUST contain dataRef field (D16-B3)")
	assert.Contains(t, s, `"kpi_reference"`,
		"decision MUST be kpi_reference (D16-B3)")
	assert.Contains(t, s, `"asOfPeriod":"2026-07"`,
		"asOfPeriod field MUST round-trip (D16-B4)")
	assert.Contains(t, s, `"suppressed":2`,
		"suppressed count MUST round-trip (D16-D1)")

	// 2. DataRef nil → omitempty — must NOT contain "dataRef" substring.
	nilPayload := DecisionPayload{
		Step:     1,
		Decision: "retrieve",
		Query:    "差旅费",
	}
	nilRaw, err := json.Marshal(nilPayload)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(nilRaw), "dataRef"),
		"nil DataRef MUST be omitted (omitempty lock — D16-B3)")

	// 3. Unmarshal round-trip — all fields byte-identical.
	var got DecisionPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, 1, got.Step)
	assert.Equal(t, DecisionKPIReference, got.Decision)
	require.NotNil(t, got.DataRef, "DataRef MUST be non-nil after round-trip")
	assert.Equal(t, "loan_overdue_rate_by_status", got.DataRef.Func)
	assert.Equal(t, "2026-07", got.DataRef.ASOfPeriod)
	assert.Equal(t, t0, got.DataRef.ASOfTs)
	assert.Equal(t, 7, got.DataRef.RowCount)
	assert.Equal(t, 2, got.DataRef.Suppressed)
	assert.Equal(t, 10, got.DataRef.KMinApplied)
	require.Contains(t, got.DataRef.Args, "status")
	assert.Equal(t, "逾期", got.DataRef.Args["status"])

	// 4. ValidTypes count MUST stay flat here (no new chunk type added
	//    by D16-B3; the 24th entry is Plan 29-04's data-turn).
	assert.Equal(t, 24, len(ValidTypes),
		"ValidTypes MUST stay at 24 after D16-B3 DataRef addition (data-decision discriminator unchanged — D16 overload pattern; 23 through Plan 12-02 + data-turn per Plan 29-04)")
}
