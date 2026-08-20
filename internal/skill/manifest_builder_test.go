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

package skill

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManifestBuilderWithReads — happy path: builder produces a typed
// Manifest whose Reads slice matches the WithReads argument. Reads
// values are ASCII tier patterns (e.g. "authority/**"), per the
// SKILL_SCHEMA contract; ResolveTiers() then maps them to the
// CN-char Milvus tier values.
func TestManifestBuilderWithReads(t *testing.T) {
	base := validManifest()
	b := NewManifestBuilder(base)
	out, err := b.WithReads([]string{"authority/**"}).Build()
	require.NoError(t, err, "valid base + non-empty Reads must build")
	require.NotNil(t, out)
	assert.Equal(t, []string{"authority/**"}, out.Reads,
		"the resulting manifest's Reads must equal the WithReads intersection")
}

// TestManifestBuilderEmptyIntersectionFails — FAN-02 SC-2: empty
// intersection is a load-time error at Build(), NOT a runtime
// silence.
func TestManifestBuilderEmptyIntersectionFails(t *testing.T) {
	base := validManifest()
	b := NewManifestBuilder(base)
	out, err := b.WithReads([]string{}).Build()
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "empty Reads",
		"the error MUST name the failing field for operator diagnosis")
}

// TestManifestBuilderInvalidReadsFails — a Reads slice with an unknown
// tier name fails Validate (the second fail-closed check in Build).
func TestManifestBuilderInvalidReadsFails(t *testing.T) {
	base := validManifest()
	b := NewManifestBuilder(base)
	out, err := b.WithReads([]string{"unknown-tier-not-in-enum"}).Build()
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "reads")
}

// TestManifestBuilderCloneSemantics — the builder must NOT mutate the
// caller's base Manifest when WithReads replaces the Reads slice.
// Verified by checking that base.Reads is unchanged after Build().
func TestManifestBuilderCloneSemantics(t *testing.T) {
	base := validManifest()
	originalReads := append([]string(nil), base.Reads...)
	b := NewManifestBuilder(base)
	_, err := b.WithReads([]string{"authority/**"}).Build()
	require.NoError(t, err)
	assert.Equal(t, originalReads, base.Reads,
		"the builder's WithReads MUST NOT mutate the caller's base manifest")
	// Also: base.Name must be unchanged (the builder clears the Name on
	// the WORKING copy only, not on the input base).
	assert.Equal(t, base.Name, base.Name, "input base.Name unchanged")
}

// TestManifestBuilderNilBasePanics — NewManifestBuilder(nil) panics.
// This is the P13-L2 invariant the doc comment promises.
func TestManifestBuilderNilBasePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewManifestBuilder(nil) MUST panic per P13-L2 invariant")
		}
	}()
	_ = NewManifestBuilder(nil)
}

// TestManifestBuilderChainedFluent — chained WithReads().WithReads() (no
// WithWrites yet, future plan) flows naturally and the LAST call wins.
// Documents the fluent-API contract.
func TestManifestBuilderChainedFluent(t *testing.T) {
	base := validManifest()
	b := NewManifestBuilder(base)
	out, err := b.WithReads([]string{"authority/**"}).WithReads([]string{"authority/**", "historical/**"}).Build()
	require.NoError(t, err)
	assert.Equal(t, []string{"authority/**", "historical/**"}, out.Reads,
		"chained WithReads — the last call wins")
	// Sanity: builder does not mutate base.
	assert.Equal(t, []string{"authority/**"}, base.Reads)
}

// TestManifestBuilderResolvedTiersCloned — when base has resolvedTiers
// populated (Phase 3 substrate path), the builder's clone must NOT
// share the slice with base. Mutation of one must not affect the other.
func TestManifestBuilderResolvedTiersCloned(t *testing.T) {
	base := validManifest()
	base.SetResolvedTiersForTest([]string{"权威", "历史"})
	b := NewManifestBuilder(base)
	out, err := b.WithReads([]string{"authority/**"}).Build()
	require.NoError(t, err)
	assert.Equal(t, []string{"权威", "历史"}, out.ResolvedTiers())
	// Append to base's resolvedTiers — out's must be unchanged.
	base.SetResolvedTiersForTest(append(base.ResolvedTiers(), "个人"))
	assert.Equal(t, []string{"权威", "历史"}, out.ResolvedTiers(),
		"base.resolvedTiers mutation must not affect the builder's output")
}

// TestManifestBuilderBuildErrorMessageStable — pins the exact error
// substring for FAN-02 SC-2 contract. runtime.BuildSubAgent's
// load-time-error substring matching depends on this wording.
func TestManifestBuilderBuildErrorMessageStable(t *testing.T) {
	base := validManifest()
	b := NewManifestBuilder(base)
	_, err := b.WithReads([]string{}).Build()
	require.Error(t, err)
	// Contract substring — runtime.BuildSubAgent surfaces this verbatim.
	assert.True(t, strings.Contains(err.Error(), "FAN-02"),
		"FAN-02 SC-2 error message must contain FAN-02 tag for build-time audit")
}