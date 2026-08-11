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

// Package spike tests.
//
// These tests cover the pure-function behavior of BuildMilvusFilter — the
// security-critical helper that converts a Skill.reads slice into a Milvus
// filter expression. The filter must be correct WITHOUT a live Milvus
// instance because it is SkillForge's structural tier-isolation boundary
// (T-1-04 / T-1-05): a malformed expression would either leak cross-tier
// docs or zero out results.
//
// SDK-dependent helpers (NewMilvusClient, SeedDocuments, EmbedQuery, Search)
// are exercised by cmd/spiked/milvus_test.go's TestSeedAndFilterIntegration,
// which is gated behind MILVUS_INTEGRATION=1.
package spike

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildMilvusFilter(t *testing.T) {
	cases := []struct {
		name     string
		tiers    []string
		expected string
	}{
		{
			name:     "nil slice returns default-deny false",
			tiers:    nil,
			expected: "false",
		},
		{
			name:     "empty slice returns default-deny false",
			tiers:    []string{},
			expected: "false",
		},
		{
			name:     "single authority tier (CN chars)",
			tiers:    []string{"权威"},
			expected: `tier in ["权威"]`,
		},
		{
			name:     "single historical tier",
			tiers:    []string{"历史"},
			expected: `tier in ["历史"]`,
		},
		{
			name:     "single personal tier",
			tiers:    []string{"个人"},
			expected: `tier in ["个人"]`,
		},
		{
			name:     "single draft tier",
			tiers:    []string{"草稿"},
			expected: `tier in ["草稿"]`,
		},
		{
			name:     "multi tier authority+historical preserves order",
			tiers:    []string{"权威", "历史"},
			expected: `tier in ["权威","历史"]`,
		},
		{
			name:     "multi tier authority+historical+personal",
			tiers:    []string{"权威", "历史", "个人"},
			expected: `tier in ["权威","历史","个人"]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildMilvusFilter(tc.tiers)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestBuildMilvusFilterSpecialChars verifies Pitfall 7 mitigation: tier values
// containing characters that are hostile to naive string templating must be
// safely escaped so the resulting filter expression cannot be broken by a
// malicious or unusual tier name. We assert structural properties rather than
// exact bytes so the test is robust to Go's %q evolution.
func TestBuildMilvusFilterSpecialChars(t *testing.T) {
	t.Run("single quote inside tier name does not crash or break out of quotes", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = BuildMilvusFilter([]string{"O'Brien"})
		})
		got := BuildMilvusFilter([]string{"O'Brien"})
		// Must start with the tier-in prefix and contain the value escaped.
		assert.True(t, strings.HasPrefix(got, "tier in ["),
			"filter must start with 'tier in [', got: %q", got)
		assert.Contains(t, got, "O'Brien",
			"tier value must appear in filter: %q", got)
	})

	t.Run("double quote inside tier name is escaped (no raw unescaped quote)", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = BuildMilvusFilter([]string{`a"b`})
		})
		got := BuildMilvusFilter([]string{`a"b`})
		// The raw substring a"b (unescaped) must NOT appear — that would
		// indicate the double-quote was not escaped and could break parsing.
		assert.NotContains(t, got, `a"b`,
			"unescaped double-quote is a filter-injection vector: %q", got)
		// The escaped form (Go %q produces \" for ") MUST appear.
		assert.Contains(t, got, `\"`,
			"double-quote must be backslash-escaped: %q", got)
	})

	t.Run("backslash inside tier name is escaped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = BuildMilvusFilter([]string{`a\b`})
		})
		got := BuildMilvusFilter([]string{`a\b`})
		// Raw unescaped backslash must not appear as the 2-char sequence a\b.
		assert.NotContains(t, got, `a\b`,
			"unescaped backslash is a filter-injection vector: %q", got)
	})
}

// TestEmbedQueryDeterministicSanity verifies the offline embedding fallback:
// the deterministic hash-based embedder must return exactly 1024 floats for
// any non-empty input and be stable across calls (same input → same vector).
// This is the fallback path used when BGE_M3_URL is unreachable.
func TestEmbedQueryDeterministicSanity(t *testing.T) {
	vec := EmbedQueryDeterministic("出差住宿费超标 200 元")
	assert.Len(t, vec, 1024, "deterministic embedding must be 1024-dim (BGE-M3 dim)")

	// Stability: same input must yield identical output.
	vec2 := EmbedQueryDeterministic("出差住宿费超标 200 元")
	assert.Equal(t, vec, vec2, "deterministic embedding must be stable across calls")

	// Distinctness: different inputs should yield different vectors (this is
	// a probabilistic property of a hash-based generator, but for any two
	// distinct non-trivial inputs a SHA-256-seeded generator will differ).
	other := EmbedQueryDeterministic("竞业限制期最长不超过 24 个月")
	assert.NotEqual(t, vec, other, "distinct inputs must produce distinct embeddings")
}
