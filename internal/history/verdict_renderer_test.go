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

package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderConclusion_FactualQuery — when Answer is non-empty the
// function returns the Answer string unmodified. This is the 260817-vsm
// verdict-subject-mismatch primary path.
func TestRenderConclusion_FactualQuery(t *testing.T) {
	const verdictJSON = `{
		"status": "ok",
		"confidence": 0.92,
		"citations": [],
		"review_required": false,
		"answer": "由部门负责人审批。",
		"compliance_status": "不适用"
	}`
	got, err := RenderConclusion(verdictJSON)
	require.NoError(t, err)
	assert.Equal(t, "由部门负责人审批。", got)
}

// TestRenderConclusion_ComplianceVerdict — when Answer is empty and
// ComplianceStatus is non-empty, the function returns ComplianceStatus.
func TestRenderConclusion_ComplianceVerdict(t *testing.T) {
	const verdictJSON = `{
		"status": "ok",
		"confidence": 0.85,
		"citations": [{"chunk_id":"1","span":"条款文本","tier":"权威"}],
		"review_required": false,
		"answer": "",
		"compliance_status": "合规"
	}`
	got, err := RenderConclusion(verdictJSON)
	require.NoError(t, err)
	assert.Equal(t, "合规", got)
}

// TestRenderConclusion_MalformedJSON — returns a non-nil error on
// malformed JSON. The sentinel-recovery path lives in
// RenderConclusionOrSentinel.
func TestRenderConclusion_MalformedJSON(t *testing.T) {
	got, err := RenderConclusion(`{"status": "ok",`)
	require.Error(t, err)
	assert.Empty(t, got)
	assert.Equal(t, unrenderableConclusionSentinel, RenderConclusionOrSentinel(`{"status": "ok",`))
}

// TestRenderConclusion_FallsBackToStatus — when both Answer and
// ComplianceStatus are empty, the function falls back to the
// platform-neutral Status field.
func TestRenderConclusion_FallsBackToStatus(t *testing.T) {
	const verdictJSON = `{
		"status": "needs_input",
		"confidence": 0.5,
		"citations": [],
		"review_required": true,
		"answer": "",
		"compliance_status": ""
	}`
	got, err := RenderConclusion(verdictJSON)
	require.NoError(t, err)
	assert.Equal(t, "needs_input", got)
}

// TestRenderConclusion_EmptyJSON — empty string is an error (covers
// the IsRefusalContent cross-check: an empty assistant message is
// treated as a pass-through, not a verdict).
func TestRenderConclusion_EmptyJSON(t *testing.T) {
	_, err := RenderConclusion("")
	require.Error(t, err)
}

// TestRenderRefusal_AllKnownCodes — table-driven over the three known
// refusal reason codes; each must render the exact
// 「无法给出结论：<mapped sentence>。」 string (visual parity with
// chat.go refusalText).
func TestRenderRefusal_AllKnownCodes(t *testing.T) {
	cases := []struct {
		name    string
		reason  string
		wantSub string
		want    string
	}{
		{
			name:    "no_authorized_source",
			reason:  "no_authorized_source",
			wantSub: "未检索到可回答该问题的授权知识源",
			want:    "无法给出结论：未检索到可回答该问题的授权知识源。",
		},
		{
			name:    "low_retrieval_score",
			reason:  "low_retrieval_score",
			wantSub: "检索到的条款与该问题相关度不足",
			want:    "无法给出结论：检索到的条款与该问题相关度不足。",
		},
		{
			name:    "out_of_scope",
			reason:  "out_of_scope",
			wantSub: "该问题超出当前 Skill 的处理范围",
			want:    "无法给出结论：该问题超出当前 Skill 的处理范围。",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderRefusal(tc.reason)
			assert.Equal(t, tc.want, got)
			assert.Contains(t, got, tc.wantSub)
		})
	}
}

// TestRenderRefusal_UnknownCode — the unknown-reason fallback returns
// the generic refusal sentence (matches chat.go refusalText's miss
// branch).
func TestRenderRefusal_UnknownCode(t *testing.T) {
	got := RenderRefusal("totally_unknown_reason")
	want := "无法给出结论：无法基于已授权的知识源回答该问题。"
	assert.Equal(t, want, got)
}

// TestRenderRefusal_EmptyReason — empty reason is an unknown code and
// falls back to the generic sentence.
func TestRenderRefusal_EmptyReason(t *testing.T) {
	got := RenderRefusal("")
	want := "无法给出结论：无法基于已授权的知识源回答该问题。"
	assert.Equal(t, want, got)
}