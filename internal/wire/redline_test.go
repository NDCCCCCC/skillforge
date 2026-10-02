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

// Package wire — Plan 05-01 Task 1: RedlinePayload test (TDD RED phase).
//
// RedlinePayload is the JSON shape carried inside `data-redline` SSE
// chunks (the 18th wire type per D5-14). It carries the agent's
// clause-level redline proposal: original clause text, proposed
// rewrite, rationale, and optional clauseId. The struct is
// deliberately required on its core three fields (no omitempty) so a
// truncated JSON payload fails to parse — V8 wire-smuggling mitigation
// per 05-UI-SPEC §Registry Safety.
//
// RED phase (this file): the RedlinePayload type does not exist yet;
// this test file fails to COMPILE until the GREEN commit lands.

package wire

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedlinePayload_RequiredFields — V8 mitigation: missing required
// field on a redline payload MUST cause json.Unmarshal to fail. The
// 18th wire type carries agent-generated clause rewrites; a truncated
// payload silently degraded to zero-value fields would render the UI's
// diff view against blank data, masking the bug.
func TestRedlinePayload_RequiredFields(t *testing.T) {
	// Missing proposedRewrite → error.
	bad := []byte(`{"originalClause":"x"}`)
	var r RedlinePayload
	err := json.Unmarshal(bad, &r)
	require.Error(t, err,
		"missing required proposedRewrite MUST fail Unmarshal (V8 wire-smuggling mitigation): got %v", r)

	// Missing rationale → error.
	bad2 := []byte(`{"originalClause":"x","proposedRewrite":"y"}`)
	err = json.Unmarshal(bad2, &r)
	require.Error(t, err,
		"missing required rationale MUST fail Unmarshal (V8): got %v", r)

	// Missing originalClause → error.
	bad3 := []byte(`{"proposedRewrite":"y","rationale":"because"}`)
	err = json.Unmarshal(bad3, &r)
	require.Error(t, err,
		"missing required originalClause MUST fail Unmarshal (V8): got %v", r)
}

// TestRedlinePayload_RoundTrip — happy path: full payload marshals
// and unmarshals without loss; clauseId (the optional field) defaults
// to empty when absent and survives round-trip when present.
func TestRedlinePayload_RoundTrip(t *testing.T) {
	// Full payload (with clauseId).
	r := RedlinePayload{
		ClauseID:        "C-7.3.2",
		OriginalClause:  "员工差旅费应在30日内报销。",
		ProposedRewrite: "员工差旅费应在15个工作日内报销。",
		Rationale:       "缩短报销周期，提升员工满意度。",
	}
	raw, err := json.Marshal(r)
	require.NoError(t, err)

	var got RedlinePayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, r, got,
		"full RedlinePayload MUST round-trip without loss")
}

// TestRedlinePayload_ClauseIdOptional — clauseId has `omitempty` so a
// missing field is tolerated (some redlines target an ad-hoc clause
// without a stable id); the core three fields remain required.
func TestRedlinePayload_ClauseIdOptional(t *testing.T) {
	raw := []byte(`{"originalClause":"a","proposedRewrite":"b","rationale":"c"}`)
	var r RedlinePayload
	require.NoError(t, json.Unmarshal(raw, &r),
		"omitting clauseId MUST NOT error (omitempty on the optional id field)")
	assert.Empty(t, r.ClauseID,
		"missing clauseId MUST default to empty string")
	assert.Equal(t, "a", r.OriginalClause)
	assert.Equal(t, "b", r.ProposedRewrite)
	assert.Equal(t, "c", r.Rationale)
}
