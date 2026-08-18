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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// canonicalRefusalSemanticWords is the test-side copy of the canonical
// 11-entry list. Kept in lockstep with neutral_placeholder.go
// RefusalSemanticWords — if you add or rename a word, update BOTH
// places. The grep gate (10-VALIDATION.md GG-A2-1) is the
// cross-platform comment-exempt equivalent.
var canonicalRefusalSemanticWords = []string{
	"拒绝",
	"不能",
	"无权",
	"无法",
	"不得",
	"不允许",
	"未在",
	"无可",
	"谢绝",
	"拒绝回答",
	"不能回答",
}

// TestNeutralPlaceholder_NoRefusalSemanticWords — the canonical
// D10-A2 contract gate. Asserts NeutralPlaceholderTemplate contains
// NONE of the canonical 11-entry word list (T-10-02 / T-9-FILLER-LEAK
// pattern from the Phase 9 spike). Compound entries are checked
// explicitly even though the 9 atomic stems cover them via substring
// — the explicit check guards against future refactors that might
// drop a stem.
func TestNeutralPlaceholder_NoRefusalSemanticWords(t *testing.T) {
	for _, w := range canonicalRefusalSemanticWords {
		assert.NotContains(t, NeutralPlaceholderTemplate, w,
			"placeholder %q contains refusal-semantic word %q — the canonical 11-entry list forbids every entry",
			NeutralPlaceholderTemplate, w)
	}
}

// TestRefusalSemanticWords_CanonicalListPinned — the canonical list
// is declared ONLY in the test file (neutral_placeholder.go keeps zero
// non-comment mentions of any refusal-semantic word so the GG-A2-1 CI
// grep gate stays at 0). This test pins the test-side list shape —
// if a future contributor adds a word here, they must ALSO update
// the GG-A2-1 grep pattern in 10-VALIDATION.md and the helper list
// in neutral_placeholder.go's documentation block.
func TestRefusalSemanticWords_CanonicalListPinned(t *testing.T) {
	assert.Equal(t, []string{
		"拒绝",
		"不能",
		"无权",
		"无法",
		"不得",
		"不允许",
		"未在",
		"无可",
		"谢绝",
		"拒绝回答",
		"不能回答",
	}, canonicalRefusalSemanticWords,
		"canonical word list drift detected — update GG-A2-1 in lockstep with this list")
}

// TestRenderNeutralPlaceholder_Deterministic — the helper is pure:
// every call returns the same fixed string regardless of the turn
// index argument.
func TestRenderNeutralPlaceholder_Deterministic(t *testing.T) {
	for _, idx := range []int{0, 1, 5, 100, -1} {
		assert.Equal(t, NeutralPlaceholderTemplate, RenderNeutralPlaceholder(idx),
			"turnIndex %d must not influence the placeholder string", idx)
	}
}

// TestIsRefusalContent_AllKnownAndUnknown — table-driven over the 5
// refusal-round reasons + 3 non-refusal strings (T-10-02).
func TestIsRefusalContent_AllKnownAndUnknown(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		// Known refusal reasons — 5 entries (3 D2-13 + 2 Phase 10 classifier).
		{name: "no_authorized_source", content: verdict.RefusalReasonNoAuthorizedSource, want: true},
		{name: "low_retrieval_score", content: verdict.RefusalReasonLowRetrievalScore, want: true},
		{name: "out_of_scope", content: verdict.RefusalReasonOutOfScope, want: true},
		{name: "out_of_topic_classifier", content: "out_of_topic", want: true},
		{name: "pii_request_classifier", content: "pii_request", want: true},
		// Non-refusal — 3 entries.
		{name: "plain_answer", content: "由部门负责人审批。", want: false},
		{name: "verdict_json", content: `{"status":"ok","answer":"x"}`, want: false},
		{name: "empty", content: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsRefusalContent(tc.content))
		})
	}
}

// TestIsRefusalRound_OnlyAssistantRole — isRefusalRound additionally
// requires the message role be "assistant"; user-side content is
// never re-classified.
func TestIsRefusalRound_OnlyAssistantRole(t *testing.T) {
	cases := []struct {
		name    string
		message store.SessionMessage
		want    bool
	}{
		{name: "user_with_refusal_code", message: store.SessionMessage{Role: "user", Content: "no_authorized_source"}, want: false},
		{name: "assistant_with_refusal_code", message: store.SessionMessage{Role: "assistant", Content: "no_authorized_source"}, want: true},
		{name: "assistant_with_answer", message: store.SessionMessage{Role: "assistant", Content: "ok"}, want: false},
		{name: "user_with_answer", message: store.SessionMessage{Role: "user", Content: "ok"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isRefusalRound(tc.message))
		})
	}
}

// TestProjection_NeutralPlaceholderSubstitutesRefusalRound — the
// end-to-end integration check: an assistant message whose value is a
// refusal-reason code MUST be replaced by the neutral placeholder in
// the projected history (D10-A2 primary contract).
func TestProjection_NeutralPlaceholderSubstitutesRefusalRound(t *testing.T) {
	in := []store.SessionMessage{
		{Role: "user", Content: "能告诉我财务制度吗？"},
		{Role: "assistant", Content: verdict.RefusalReasonOutOfScope},
	}
	out := Project(in)
	if assert.NotNil(t, out, "Project must not return nil for a user+assistant pair") {
		assert.Len(t, out, 2)
		assert.Equal(t, NeutralPlaceholderTemplate, out[1].Content,
			"refusal round must render as the neutral placeholder; got %q", out[1].Content)
		assert.NotContains(t, strings.Join([]string{out[0].Content, out[1].Content}, "\n"), "拒绝",
			"agent-path history must not leak refusal-semantic words")
	}
}