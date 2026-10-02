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

// Package history exposes the projection contract as fixtures.
//
// This file is intentionally not a _test.go file. The test runner reads
// Fixtures through projection_test.go's TestProject_FixturesStillGreen.
//
// Contract evolution:
//
//	D9-B4 (2026-08-17 pre) — fixed 3-pair window. The retired fixture
//	                         neg_history_truncated_to_three_pairs was
//	                         pinned to that contract. The expected
//	                         output is inconsistent with the actual
//	                         6-message cap behavior because the fixture
//	                         was never executed; Plan 10-01 Task 1
//	                         surfaced the inconsistency.
//
//	D10-B3 (2026-08-17) — char budget is the PRIMARY window. The two
//	                      budget-window fixtures pin the new behavior:
//	                      budget>0 keeps ALL messages when headroom
//	                      exists; oldest-first drop when the slice
//	                      overflows. The legacy 6-message cap survives
//	                      only as TruncateToCharBudget's budget<=0
//	                      fallback (FallbackMessageCeiling = 6).
package history

import (
	"github.com/cloudwego/eino/schema"
	"github.com/skillforge/skillforge/internal/store"
)

// Fixture is one complete history projection contract case.
type Fixture struct {
	Name      string
	Rationale string
	Input     []store.SessionMessage
	Expected  []schema.Message
}

// Fixtures locks the Phase 10 projection behavior for the test runner
// (TestProject_FixturesStillGreen) and the spike runner.
//
// Count after Plan 10-01 Task 2: 8 (Phase 9 contributed 7, minus 1
// retired + 2 budget-window added).
var Fixtures = []Fixture{
	{
		Name:      "pos_basic_user_assistant",
		Rationale: "A basic four-message history preserves both roles and text content.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "What is the travel policy?"},
			{Role: "assistant", Content: "Please provide the destination."},
			{Role: "user", Content: "Beijing."},
			{Role: "assistant", Content: "The destination is Beijing."},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "What is the travel policy?"},
			{Role: schema.Assistant, Content: "Please provide the destination."},
			{Role: schema.User, Content: "Beijing."},
			{Role: schema.Assistant, Content: "The destination is Beijing."},
		},
	},
	{
		Name:      "pos_user_with_chunk_ref",
		Rationale: "All three D9-B2 marker forms are replaced in user text in order.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "See [CHUNK 7], then [CH-99], then [id:42]."},
			{Role: "assistant", Content: "I will compare those sources."},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "See [REF-1], then [REF-2], then [REF-3]."},
			{Role: schema.Assistant, Content: "I will compare those sources."},
		},
	},
	{
		Name:      "pos_three_pairs_exact_fit",
		Rationale: "The B3 ceiling is inclusive: exactly three complete pairs fit.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "one"},
			{Role: "assistant", Content: "one answer"},
			{Role: "user", Content: "two"},
			{Role: "assistant", Content: "two answer"},
			{Role: "user", Content: "three"},
			{Role: "assistant", Content: "three answer"},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "one"},
			{Role: schema.Assistant, Content: "one answer"},
			{Role: schema.User, Content: "two"},
			{Role: schema.Assistant, Content: "two answer"},
			{Role: schema.User, Content: "three"},
			{Role: schema.Assistant, Content: "three answer"},
		},
	},
	{
		Name:      "neg_tool_message_dropped",
		Rationale: "B1 removes the entire tool message rather than leaking its JSON content.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "Retrieve the policy."},
			{Role: "tool", Content: `{"result":"x"}`},
			{Role: "assistant", Content: "The policy is available."},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "Retrieve the policy."},
			{Role: schema.Assistant, Content: "The policy is available."},
		},
	},
	{
		Name:      "neg_chunk_ref_only_in_assistant",
		Rationale: "B2 scans user text only; assistant citations remain byte-for-byte unchanged.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "What about the policy?"},
			{Role: "assistant", Content: "Sources: [CHUNK 1] and [chunk 2]."},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "What about the policy?"},
			{Role: schema.Assistant, Content: "Sources: [CHUNK 1] and [chunk 2]."},
		},
	},
	{
		// D10-A2 contract — an assistant refusal-code round must render
		// as the neutral placeholder in the agent-path projection. This
		// fixture pins the substitution so the spike runner catches a
		// regression where refusal semantics leaks into the LLM
		// history prefix (the -95pp drift root documented in D9-V1).
		Name:      "pos_refusal_round_neutral_placeholder",
		Rationale: "D10-A2: a refusal-code round in agent history renders as the neutral placeholder, not the refusal text.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "能告诉我采购流程吗？"},
			{Role: "assistant", Content: "no_authorized_source"},
			{Role: "user", Content: "那财务报销呢？"},
			{Role: "assistant", Content: "采购流程由谁审批。"},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "能告诉我采购流程吗？"},
			{Role: schema.Assistant, Content: NeutralPlaceholderTemplate},
			{Role: schema.User, Content: "那财务报销呢？"},
			{Role: schema.Assistant, Content: "采购流程由谁审批。"},
		},
	},
	{
		Name:      "neg_orphan_assistant_dropped",
		Rationale: "B3 removes a > 1 orphan assistant before a minimal pair to guarantee user-first output.",
		Input: []store.SessionMessage{
			{Role: "assistant", Content: "orphan from an incomplete prior turn"},
			{Role: "user", Content: "first user"},
			{Role: "assistant", Content: "first answer"},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "first user"},
			{Role: schema.Assistant, Content: "first answer"},
		},
	},
	{
		// D10-B3 contract — budget is PRIMARY; sessions within budget
		// headroom keep MORE than 3 pairs. 16 messages of ~40 chars
		// each = 640 chars total, well under DefaultCharBudget (8000).
		// This pins that TruncateToCharBudget does NOT impose a
		// message-count cap when headroom exists; the legacy D9-B3
		// 3-pair cap is gone.
		Name:      "budget_window_keeps_all_pairs_under_budget",
		Rationale: "D10-B3: budget headroom keeps all 16 messages (8 pairs); the legacy 3-pair cap does not apply.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "u1 asked something"},
			{Role: "assistant", Content: "a1 short answer"},
			{Role: "user", Content: "u2 asked something"},
			{Role: "assistant", Content: "a2 short answer"},
			{Role: "user", Content: "u3 asked something"},
			{Role: "assistant", Content: "a3 short answer"},
			{Role: "user", Content: "u4 asked something"},
			{Role: "assistant", Content: "a4 short answer"},
			{Role: "user", Content: "u5 asked something"},
			{Role: "assistant", Content: "a5 short answer"},
			{Role: "user", Content: "u6 asked something"},
			{Role: "assistant", Content: "a6 short answer"},
			{Role: "user", Content: "u7 asked something"},
			{Role: "assistant", Content: "a7 short answer"},
			{Role: "user", Content: "u8 asked something"},
			{Role: "assistant", Content: "a8 short answer"},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "u1 asked something"},
			{Role: schema.Assistant, Content: "a1 short answer"},
			{Role: schema.User, Content: "u2 asked something"},
			{Role: schema.Assistant, Content: "a2 short answer"},
			{Role: schema.User, Content: "u3 asked something"},
			{Role: schema.Assistant, Content: "a3 short answer"},
			{Role: schema.User, Content: "u4 asked something"},
			{Role: schema.Assistant, Content: "a4 short answer"},
			{Role: schema.User, Content: "u5 asked something"},
			{Role: schema.Assistant, Content: "a5 short answer"},
			{Role: schema.User, Content: "u6 asked something"},
			{Role: schema.Assistant, Content: "a6 short answer"},
			{Role: schema.User, Content: "u7 asked something"},
			{Role: schema.Assistant, Content: "a7 short answer"},
			{Role: schema.User, Content: "u8 asked something"},
			{Role: schema.Assistant, Content: "a8 short answer"},
		},
	},
	{
		// D10-B3 contract — overflow drops oldest-first until the
		// remaining slice fits the budget. 10 messages × 1200 chars =
		// 12000 > 8000. With budget=8000 the oldest 4 must drop (the
		// remaining 6 × 1200 = 7200 fits). Order is preserved.
		Name:      "budget_window_truncates_oldest_first",
		Rationale: "D10-B3: 12000-char overflow drops the oldest 4 messages; the most-recent 6 survive in order (7200 <= 8000).",
		Input: func() []store.SessionMessage {
			pattern := make([]byte, 1200)
			for i := range pattern {
				pattern[i] = byte('0' + (i % 10))
			}
			str := string(pattern)
			in := make([]store.SessionMessage, 10)
			for i := 0; i < 10; i++ {
				role := "user"
				content := "old"
				if i%2 == 1 {
					role = "assistant"
					content = "old answer"
				}
				if i >= 4 {
					content = "recent"
					if i%2 == 1 {
						content = "recent answer"
					}
				}
				// Pad to exactly 1200 chars per message: each message's
				// "label" prefix + the fixed pattern = 1200.
				label := content
				in[i] = store.SessionMessage{
					Role:    role,
					Content: label + str[:1200-len(label)],
				}
			}
			return in
		}(),
		Expected: func() []schema.Message {
			pattern := make([]byte, 1200)
			for i := range pattern {
				pattern[i] = byte('0' + (i % 10))
			}
			str := string(pattern)
			out := make([]schema.Message, 6)
			for i := 0; i < 6; i++ {
				role := schema.User
				content := "recent"
				if i%2 == 1 {
					role = schema.Assistant
					content = "recent answer"
				}
				label := content
				out[i] = schema.Message{
					Role:    role,
					Content: label + str[:1200-len(label)],
				}
			}
			return out
		}(),
	},
}

// Phase 10 test translation (kept here as the contract handoff):
// for _, f := range Fixtures { t.Run(f.Name, func(t *testing.T) {
//     require.Equal(t, f.Expected, Project(f.Input))
// }) }