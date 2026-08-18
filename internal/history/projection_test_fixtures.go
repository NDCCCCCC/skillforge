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

// Package history exposes the Phase 9 projection contract as fixtures.
//
// This file is intentionally not a _test.go file. Phase 10 can consume
// Fixtures directly when adding projection_test.go tests.
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

// Fixtures locks the D9-B1/B2/B3 behavior for Phase 10 and the spike runner.
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
		Name:      "neg_history_truncated_to_three_pairs",
		Rationale: "B1 removes tool data before B3 retains only the last three complete pairs.",
		Input: []store.SessionMessage{
			{Role: "user", Content: "old one"},
			{Role: "assistant", Content: "old answer one"},
			{Role: "user", Content: "old two"},
			{Role: "assistant", Content: "old answer two"},
			{Role: "user", Content: "three"},
			{Role: "tool", Content: "hidden tool output"},
			{Role: "assistant", Content: "three answer"},
			{Role: "user", Content: "four"},
			{Role: "assistant", Content: "four answer"},
		},
		Expected: []schema.Message{
			{Role: schema.User, Content: "three"},
			{Role: schema.Assistant, Content: "three answer"},
			{Role: schema.User, Content: "four"},
			{Role: schema.Assistant, Content: "four answer"},
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
}

// Phase 10 test translation (kept here as the contract handoff):
// for _, f := range Fixtures { t.Run(f.Name, func(t *testing.T) {
//     require.Equal(t, f.Expected, Project(f.Input))
// }) }
