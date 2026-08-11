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

package wire

import (
	"encoding/json"
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
