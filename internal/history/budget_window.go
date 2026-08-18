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
	"github.com/cloudwego/eino/schema"
)

// Budget constants — RF-6 (GLM tokenizer unconfigurable) forces
// char-approximation. The end-of-phase spike hard-gate calibrates the
// DefaultCharBudget against live GLM-5.2 + Milvus; values here are
// conservative-during-RF-6 placeholders that give the projection
// meaningful headroom for typical sessions and gracefully truncate
// pathological ones.
//
// Budget composition (DefaultCharBudget = 8000 is the share reserved
// for history after the other reservations eat the context window):
//
//	ReservedForChunks         7500  // current-turn retrieval context
//	ReservedForOutput          600  // LLM output buffer
//	ReservedForSystemPrompt   2500  // system prompt + Skill persona
//	ReservedForQueryAndUser    2000  // rewritten query + user template
//	--- sum above exceeds 8000; DefaultCharBudget is the history share.
//
// The numbers are intentionally not arithmetically tight — the
// platform context window is reserved in aggregate, not per-component.
// Tightening requires live calibration; loosening is harmless because
// TruncateToCharBudget is the only window.
const (
	// DefaultCharBudget is the budget Project passes to
	// TruncateToCharBudget. Sized to keep typical 3-5 turn sessions
	// intact while bounding pathological long sessions.
	DefaultCharBudget = 8000

	// ReservedForChunks is the LLM-context share reserved for the
	// current-turn retrieval context (chunks rendered into the user
	// query template). Sized for ~10-15 chunks at 500 chars each.
	ReservedForChunks = 7500

	// ReservedForOutput is the LLM-context share reserved for the
	// generation output. The Verdict envelope plus a long compliance
	// citation sentence stay well under 600 chars.
	ReservedForOutput = 600

	// ReservedForSystemPrompt is the LLM-context share reserved for the
	// platform system prompt and Skill persona. Phase 9 measured this
	// around 1800 chars; 2500 leaves headroom for Skill evolution.
	ReservedForSystemPrompt = 2500

	// ReservedForQueryAndUser is the LLM-context share reserved for the
	// rewritten query plus the user-message template framing.
	ReservedForQueryAndUser = 2000

	// FallbackMessageCeiling is the LEGACY D9-B3 message-count cap,
	// reachable only via the budget<=0 fallback in TruncateToCharBudget.
	// Three conversation pairs (six messages) was the D9-B3 empirical
	// limit; D10-B3 (2026-08-17) makes the char budget the primary
	// window and demotes the message-count cap to this fallback role.
	//
	// The legacy ceiling is preserved so callers that explicitly disable
	// the budget (budget<=0) still get a deterministic bound rather than
	// an unbounded projection.
	FallbackMessageCeiling = 6

	// SafeguardFloor is the absolute minimum messages TruncateToCharBudget
	// will return, regardless of budget. Two messages = the smallest
	// meaningful pair (the previous question and its answer). One
	// message collapses to an orphan and is rejected by Project's
	// post-truncate <2 floor; zero messages produces a 0-history
	// live-lock where the rewriter cannot resolve anaphora (T-10-03).
	SafeguardFloor = 2
)

// TruncateToCharBudget is the PRIMARY history window (D10-B3). It drops
// the OLDEST messages first until the slice either fits the budget or
// reaches the SafeguardFloor. A session whose total fits the budget
// keeps ALL its messages — including more than 3 pairs when headroom
// exists.
//
// Parameters:
//
//	messages  — the post-render candidate slice, in chronological
//	            order (oldest first; the LLM expects this order).
//	budget    — the maximum total Content-rune count. budget<=0
//	            activates the LEGACY fallback (most-recent
//	            FallbackMessageCeiling messages; the D9-B3 cap
//	            survives here only).
//
// Returns:
//
//	kept          — the truncated slice, in chronological order.
//	droppedCount  — number of messages dropped from the head.
//	truncated     — true iff at least one message was dropped.
//
// Algorithm:
//
//  1. budget<=0: keep the last FallbackMessageCeiling messages
//     (honoring SafeguardFloor on the lower end).
//  2. budget>0: total chars fits? return unchanged. Otherwise drop
//     oldest-first until totalChars(kept) <= budget OR len(kept)
//     reaches SafeguardFloor.
//
// Phase 9 spike ground-truth: char-approximation over-counts by
// ~10-15% relative to a true tokenizer (Chinese chars are usually
// 1-2 tokens; ASCII mixed Latin is closer to 1 token per 4 chars).
// The end-of-phase spike calibrates DefaultCharBudget against live
// GLM-5.2 so the empirical fit is acceptable.
func TruncateToCharBudget(messages []schema.Message, budget int) ([]schema.Message, int, bool) {
	if len(messages) == 0 {
		return nil, 0, false
	}

	// LEGACY fallback: budget disabled. Returns the most-recent
	// FallbackMessageCeiling messages, honoring SafeguardFloor.
	if budget <= 0 {
		keep := FallbackMessageCeiling
		if keep > len(messages) {
			keep = len(messages)
		}
		if keep < SafeguardFloor {
			keep = SafeguardFloor
			if keep > len(messages) {
				keep = len(messages)
			}
		}
		start := len(messages) - keep
		dropped := start
		out := make([]schema.Message, keep)
		copy(out, messages[start:])
		return out, dropped, dropped > 0
	}

	// PRIMARY window: budget>0. If we already fit, return all.
	if totalChars(messages) <= budget {
		out := make([]schema.Message, len(messages))
		copy(out, messages)
		return out, 0, false
	}

	// Oldest-first drop until we fit or hit SafeguardFloor.
	kept := make([]schema.Message, len(messages))
	copy(kept, messages)
	dropped := 0
	for len(kept) > SafeguardFloor && totalChars(kept) > budget {
		kept = kept[1:]
		dropped++
	}
	return kept, dropped, dropped > 0
}

// totalChars sums the Content length of every message in the slice.
// Used by TruncateToCharBudget to decide whether the slice fits the
// budget. Char-approximation (not rune-aware — UTF-8 byte length is
// used because Go's len(string) returns bytes; for ASCII Chinese the
// rune/byte ratio is ~3:1, so the byte count over-counts relative to
// a true tokenizer, which is the conservative direction for a budget
// gate).
func totalChars(messages []schema.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	return total
}