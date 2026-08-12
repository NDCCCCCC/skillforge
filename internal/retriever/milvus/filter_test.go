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

package milvus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildMilvusFilter_TiersOnly — Phase 2 baseline. Tiers still drives
// the clause; the effective clause is appended but is only present
// when the signature accepts a `now` parameter (Plan 05-02).
func TestBuildMilvusFilter_TiersOnly(t *testing.T) {
	got := BuildMilvusFilter([]string{"权威", "历史"}, 1700000000)
	assert.Contains(t, got, "tier in [\"权威\",\"历史\"]",
		"tier clause must include the resolved CN-char tier values")
	assert.NotContains(t, got, "false",
		"non-empty tiers must not return the default-deny 'false' literal")
}

// TestBuildMilvusFilter_EmptyTiers — D-09 default-deny. Empty / nil
// tiers → "false" exactly as before; the effective clause is NOT
// appended (it would never match anyway).
func TestBuildMilvusFilter_EmptyTiers(t *testing.T) {
	assert.Equal(t, "false", BuildMilvusFilter(nil, 1700000000))
	assert.Equal(t, "false", BuildMilvusFilter([]string{}, 1700000000))
}

// TestBuildMilvusFilter_EffectiveToSentinel — Plan 05-02 D5-15. The
// effective_to PRE-filter uses sentinel-0 design (NOT nullable). The
// resulting expression MUST be:
//
//	tier in ["权威"] and (effective_to == 0 or effective_to >= <now>)
//
// This catches the Milvus NULL-evaluates-FALSE pitfall — by NOT using
// nullable fields we sidestep the rule entirely.
func TestBuildMilvusFilter_EffectiveToSentinel(t *testing.T) {
	const now int64 = 1700000000
	got := BuildMilvusFilter([]string{"权威"}, now)
	assert.Contains(t, got, "effective_to == 0",
		"effective PRE-filter must include the perpetual sentinel clause")
	assert.Contains(t, got, "effective_to >= 1700000000",
		"effective PRE-filter must include the now-bound clause")
	// Order matters for short-circuit: tier first, effective second
	// (tier is the selectivity win).
	assert.Less(t, strings.Index(got, "tier in"),
		strings.Index(got, "effective_to"),
		"tier clause must precede effective clause for short-circuit")
}

// TestBuildMilvusFilter_EffectiveToNowZero — when now=0 (legacy /
// unspecified), the expression still parses (no panic). The
// expression is effectively "effective_to == 0 or effective_to >= 0"
// which passes every row that does not have a negative effective_to.
// Defensive — production always passes time.Now().Unix() but tests
// sometimes pass 0 to keep assertions simple.
func TestBuildMilvusFilter_EffectiveToNowZero(t *testing.T) {
	got := BuildMilvusFilter([]string{"权威"}, 0)
	assert.Contains(t, got, "effective_to >= 0",
		"now=0 must still produce a well-formed effective clause")
}
