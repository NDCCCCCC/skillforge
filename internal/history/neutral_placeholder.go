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
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// refusalRoundReasons is the set of assistant content values that
// identify a refusal round. The pipeline (Plan 02) writes the bare
// refusal-reason code into session_turn when no Verdict was produced
// (D9-B2 / D10-A2). The three canonical reasons come from the verdict
// package (D2-13); the two extra entries — out_of_topic and
// pii_request — are the classifier-decision refusal codes the Phase
// 10 query-only classifier emits (D10-A1).
//
// Detection contract: a refusal round is identified by Content
// containing ONLY the reason code (no JSON envelope). This matches the
// pipeline's write path which stores refusalReason as the raw Content.
//
// The canonical 11-entry refusal-semantic-word list lives in
// neutral_placeholder_test.go (canonicalRefusalSemanticWords) — the
// package file keeps ZERO non-comment mentions of any of those words
// so the GG-A2-1 CI grep gate (10-VALIDATION.md) stays at 0.
var refusalRoundReasons = map[string]bool{
	verdict.RefusalReasonNoAuthorizedSource: true,
	verdict.RefusalReasonLowRetrievalScore:  true,
	verdict.RefusalReasonOutOfScope:         true,
	"out_of_topic":                          true,
	"pii_request":                           true,
}

// NeutralPlaceholderTemplate is the deterministic, conversation-natural
// placeholder the agent-path projection substitutes for refusal rounds.
// It is fixed-text (no LLM call, no template parameters) so the
// classifier cannot anchor on its phrasing across turns — the entire
// point of D10-A2 is that the placeholder must NOT carry any
// refusal-semantic token. TestNeutralPlaceholder_NoRefusalSemanticWords
// asserts the placeholder is byte-clean against the canonical 11-entry
// word list (declared in the test file to keep this file GG-A2-1
// grep-clean); the CI grep gate (10-VALIDATION.md GG-A2-1) provides
// the cross-platform comment-exempt equivalent.
//
// Length: six Chinese characters. Short enough to never threaten the
// char budget; long enough to register as a coherent assistant turn
// in the LLM's role stream.
const NeutralPlaceholderTemplate = "已记录上文。"

// RenderNeutralPlaceholder returns the deterministic neutral
// placeholder string for a refusal round. The turnIndex parameter is
// preserved so future per-turn variants (e.g. turn-position-aware
// rephrasings) can fold in without churning the projection call site
// or breaking the fixture contract.
func RenderNeutralPlaceholder(turnIndex int) string {
	_ = turnIndex
	return NeutralPlaceholderTemplate
}

// isRefusalRound reports whether m is an assistant-side refusal turn
// (bare refusal-reason code in Content). Internal helper for Project;
// the companion IsRefusalContent exposes the same check to the Plan
// 10-03 user-facing handler (which renders the refusal text via
// RenderRefusal rather than substituting the placeholder).
func isRefusalRound(m store.SessionMessage) bool {
	if m.Role != "assistant" {
		return false
	}
	return IsRefusalContent(m.Content)
}

// IsRefusalContent reports whether content matches one of the refusal
// reason codes (any of the canonical D2-13 reasons or the Phase 10
// classifier-decision codes). Exported so the Plan 10-03 handler can
// route assistant content to either RenderConclusion / RenderRefusal
// / RenderNeutralPlaceholder without re-implementing the table.
func IsRefusalContent(content string) bool {
	return refusalRoundReasons[content]
}