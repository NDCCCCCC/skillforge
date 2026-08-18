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
// D9-B1 keeps only user/assistant text; tool/system metadata cannot reach
// the LLM. D9-B2 replaces references in user text and preserves assistant
// citations. D9-B3 keeps the latest three pairs and removes an orphan
// assistant after filtering. D9-B4 records the contract as executable
// fixtures for Phase 10. D10-A2 renders refusal rounds as a neutral
// placeholder (zero refusal-semantic words) so refusal-semantic text
// never reaches the agent path — the Phase 9 spike proved the
// refusal-anchor -95pp root lives here. D10-B3 (Task 2) replaces the
// fixed 3-pair window with a char-budget window — budget>0 keeps MORE
// than 3 pairs when headroom exists; the 6-message cap survives only as
// the budget<=0 fallback ceiling in TruncateToCharBudget.
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

// Project returns a text-only, bounded Eino history for Phase 10 callers.
//
// Pipeline:
//  1. Role filter — keep only user / assistant (D9-B1).
//  2. User-side chunk-ref scrub — replace [CHUNK N] / [CH-N] / [id:N]
//     with [REF-N] counter; assistant side untouched (D9-B2).
//  3. Verdict render — assistant content that parses as verdict JSON is
//     replaced with the RenderConclusion plain-text body. Parse failure
//     is a pass-through (the message is either a bare refusal code
//     handled by isRefusalRound in neutral_placeholder.go or plain
//     assistant prose). The handler-only refusal renderer is NEVER
//     wired in here — that is the user-facing restore path (D10-A2 /
//     T-10-02b).
//  4. Window — Task 2 will replace the unconditional 6-message cap with
//     TruncateToCharBudget (the D10-B3 PRIMARY budget window). At Task
//     1 completion the legacy D9-B3 cap still applies so the Phase 9
//     fixtures stay green byte-for-byte.
//
// 5. Floors — minimum-pair guarantee; orphan-assistant head drop.
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
	// becomes the conclusion plain-text. Parse failure is a pass-through
	// (no sentinel): the unrenderableConclusionSentinel exists only for
	// direct callers that cannot propagate an error (T-10-01).
	for i, m := range filtered {
		if m.Role == "assistant" {
			if rendered, err := RenderConclusion(m.Content); err == nil {
				filtered[i].Content = rendered
			}
		}
	}
	// Task 1 keeps the D9-B3 6-message cap so the Phase 9 fixtures stay
	// green byte-for-byte. Task 2 removes this in favor of TruncateToCharBudget.
	if len(filtered) > 6 {
		filtered = filtered[len(filtered)-6:]
	}
	if len(filtered) < 2 {
		return nil
	}
	if filtered[0].Role == "assistant" {
		filtered = filtered[1:]
	}
	if len(filtered) < 2 {
		return nil
	}
	out := make([]schema.Message, len(filtered))
	for i, m := range filtered {
		role := schema.User
		if m.Role == "assistant" {
			role = schema.Assistant
		}
		out[i] = schema.Message{Role: role, Content: m.Content}
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