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

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

// TestProject_FixturesStillGreen iterates every exported Fixture and
// asserts Project(f.Input) equals f.Expected exactly. The fixtures are
// the executable contract for the history projection — they are what
// the spike runner reads to compare D9-B1/B2/B3 + D10-A2/B3 behavior
// end-to-end.
//
// Fixture count after Plan 10-01 Task 2: 8 (7 Phase 9 contributions
// minus the retired neg_history_truncated_to_three_pairs, plus the
// two new budget-window fixtures and the refusal-placeholder fixture).
func TestProject_FixturesStillGreen(t *testing.T) {
	for _, f := range Fixtures {
		t.Run(f.Name, func(t *testing.T) {
			got := Project(f.Input)
			assert.Equal(t, f.Expected, got, "fixture %q drift detected", f.Name)
		})
	}
}

// TestProject_ZeroChunkRefLiteral_UserOnly is the extended D9-B1/B2
// contract guard for the USER half of history. The scrub replaces every
// chunk reference with a [REF-N] counter; the verdict renderer never
// re-emits raw verdict JSON, so a leak on the user side trips this test
// immediately.
//
// The assistant side intentionally preserves chunk citations (D9-B2:
// "preserves assistant citations") — that is the entire point of the
// neg_chunk_ref_only_in_assistant fixture. The acceptance gate
// (must_haves.truths[0]: "History projection output contains zero
// [CHUNK literal") therefore applies to user content only.
func TestProject_ZeroChunkRefLiteral_UserOnly(t *testing.T) {
	for _, f := range Fixtures {
		t.Run(f.Name, func(t *testing.T) {
			out := Project(f.Input)
			for _, msg := range out {
				if msg.Role != schema.User {
					continue
				}
				assert.NotContains(t, msg.Content, "[CHUNK ",
					"fixture %q: user content leaked raw chunk reference %q",
					f.Name, msg.Content)
			}
		})
	}
}

// TestProject_NullSafety asserts Project handles a nil input slice
// without panicking — important because session stores can return nil
// for an empty session.
func TestProject_NullSafety(t *testing.T) {
	assert.Nil(t, Project(nil))
}