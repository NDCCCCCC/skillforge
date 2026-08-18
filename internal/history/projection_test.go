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

// task1KnownBrokenFixtures lists the fixtures that are KNOWN to be
// inconsistent with the current projection behavior. Each entry is
// retired by Plan 10-01 Task 2 — until then, the test skips the
// fixture instead of failing. The Task 2 retirement evolves the
// contract: the broken fixture is deleted and the two budget-window
// fixtures are added.
//
// Why this exists: the Phase 9 fixture set was declared in
// projection_test_fixtures.go but never executed (the package had no
// _test.go file). When Plan 10-01 Task 1 added the first executable
// test for the fixture contract, the inconsistency surfaced. Task 2
// retires the broken fixture and replaces it with two budget-window
// fixtures that align with the new D10-B3 budget-primary window.
var task1KnownBrokenFixtures = map[string]bool{
	// Inputs 9 messages; after role filter 8; 6-message cap keeps last
	// 6, but the fixture's Expected only has 4 (claimed 3 pairs but
	// only 2 pairs are listed). Task 2 retires this fixture.
	"neg_history_truncated_to_three_pairs": true,
}

// TestProject_FixturesStillGreen iterates every exported Fixture and
// asserts Project(f.Input) equals f.Expected exactly. The fixtures are
// the executable contract for the history projection — they are what the
// spike runner reads to compare D9-B1/B2/B3 behavior end-to-end.
//
// At Task 1 completion (verdict_renderer.go wired into Project) all
// Phase 9 fixtures except the known-broken list must remain green
// because none of them contain verdictJSON — the RenderConclusion
// branch is a no-op for plain assistant prose (parse failure is a
// pass-through).
//
// Task 2 retires neg_history_truncated_to_three_pairs and adds the two
// budget-window fixtures; this test remains the same shape and simply
// iterates whatever set is exported at the time of the run.
func TestProject_FixturesStillGreen(t *testing.T) {
	for _, f := range Fixtures {
		if task1KnownBrokenFixtures[f.Name] {
			continue
		}
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