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

// Package history contains the Phase 10 history projection contract.
//
// Phase 9 baseline (D9-B1/B2/B3/B4):
//
//	D9-B1 — keep only user/assistant text; tool/system metadata cannot
//	         reach the LLM.
//	D9-B2 — replace references in user text with [REF-N]; preserve
//	         assistant citations byte-for-byte.
//	D9-B3 — keep the latest three pairs and remove an orphan assistant
//	         after filtering.
//	D9-B4 — record the contract as executable fixtures for Phase 10.
//
// Phase 10 evolution (D10-A2 / D10-B3):
//
//	D10-A2 — refusal rounds in the agent path render as the neutral
//	         placeholder (zero refusal-semantic words). The Phase 9
//	         spike proved the -95pp refusal-anchor root lives here.
//	D10-B3 — char-budget window is the PRIMARY window; the fixed
//	         3-pair cap survives only as TruncateToCharBudget's
//	         budget<=0 fallback (FallbackMessageCeiling).
//
// D9-B2 intentionally leaves [REF-N] unpadded: mixed Chinese text and
// Normalize's whitespace collapse make padding semantically unnecessary
// and it would be discarded.
package history

import (
	"regexp"
	"strconv"

	"github.com/cloudwego/eino/schema"
	"github.com/skillforge/skillforge/internal/store"
)

var chunkRefPattern = regexp.MustCompile(`(?i)\[(?:CHUNK|CH-|id:)\s*([A-Za-z0-9_-]+)\s*\]`)

// Project returns a text-only, budget-bounded Eino history for Phase 10
// callers. Pipeline (in execution order):
//
//  1. Role filter — keep only user / assistant (D9-B1).
//  2. User-side chunk-ref scrub — replace [CHUNK N] / [CH-N] / [id:N]
//     with [REF-N] counter; assistant side untouched (D9-B2).
//  3. Verdict render — assistant content that parses as verdict JSON is
//     replaced with the RenderConclusion plain-text body. Parse failure
//     is a pass-through. The handler-only refusal renderer is NEVER
//     wired in here (D10-A2 / T-10-02b — refusal rounds are handled by
//     step 4).
//  4. Refusal-round placeholder — assistant content that is one of the
//     refusal-reason codes is replaced with the neutral placeholder
//     (D10-A2). Order matters: refusal detection runs AFTER the
//     verdict render so verdict JSON containing a refusal-shaped
//     Status is unaffected (it renders to the Status string and stays
//     a real answer to the user); only the bare-code path triggers
//     the placeholder rewrite.
//  5. Convert to []schema.Message.
//  6. Window — TruncateToCharBudget with DefaultCharBudget is the
//     PRIMARY and ONLY message window (D10-B3). The legacy 6-message
//     pre-cap is gone; the 6-message cap survives only via the
//     budget<=0 fallback inside TruncateToCharBudget.
//  7. Floors — minimum-pair guarantee; orphan-assistant head drop.
func Project(prev []store.SessionMessage) []schema.Message {
	filtered := make([]store.SessionMessage, 0, len(prev))
	for _, m := range prev {
		if m.Role == "user" || m.Role == "assistant" {
			filtered = append(filtered, m)
		}
	}
	for i, m := range filtered {
		if m.Role == "user" {
			filtered[i].Content = scrubChunkRefs(m.Content)
		}
	}
	// Verdict-render pass — assistant content that parses as verdict JSON
	// becomes the conclusion plain-text. Parse failure is a pass-through.
	for i, m := range filtered {
		if m.Role == "assistant" {
			if rendered, err := RenderConclusion(m.Content); err == nil {
				filtered[i].Content = rendered
			}
		}
	}
	// Refusal-round placeholder pass — assistant content that is a
	// bare refusal-reason code is replaced with the neutral placeholder.
	// Runs AFTER verdict render: a verdict JSON wrapping a refusal
	// Status renders to the Status string (real answer) and only the
	// bare-code path triggers the placeholder.
	for i, m := range filtered {
		if isRefusalRound(m) {
			filtered[i].Content = RenderNeutralPlaceholder(i)
		}
	}
	// Convert to schema.Message before budget-truncation (the budget
	// works on schema.Message.Content).
	out := make([]schema.Message, len(filtered))
	for i, m := range filtered {
		role := schema.User
		if m.Role == "assistant" {
			role = schema.Assistant
		}
		out[i] = schema.Message{Role: role, Content: m.Content}
	}
	// TruncateToCharBudget is the PRIMARY window. D10-B3. The legacy
	// 6-message pre-cap is gone.
	out, _, _ = TruncateToCharBudget(out, DefaultCharBudget)
	if len(out) < 2 {
		return nil
	}
	if out[0].Role == schema.Assistant {
		out = out[1:]
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func scrubChunkRefs(content string) string {
	ref := 0
	return chunkRefPattern.ReplaceAllStringFunc(content, func(string) string {
		ref++
		return "[REF-" + strconv.Itoa(ref) + "]"
	})
}