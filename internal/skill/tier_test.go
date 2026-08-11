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

// Package skill test — Plan 03-01 Task 2 (TDD).
//
// TestTierEnumByValue + TestTierEnumValuesDeterminism pin the
// TierEnumByValue / TierEnumValues reverse-lookup helpers that Plan
// 03-04's KNOW-02 indexer enforcement consumes. The indexer validates
// every Document.Tier against the canonical 4-tier CN-char set; these
// tests pin both the happy path (4 known tiers resolve) and the
// default-deny path (unknown / empty string → ok=false).
//
// RED phase (this commit): the test file fails to compile because
// TierEnumByValue and TierEnumValues do not exist yet. The GREEN
// commit lands both functions in tier.go.

package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTierEnumByValue covers Task 2 behavior tests 1-3:
// happy path for all 4 canonical CN-char values, default-deny for
// unknown + empty string. Table-driven so adding a future tier
// (e.g. "草稿" → "draft") is a one-line append.
func TestTierEnumByValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantDir  string
		wantOK   bool
	}{
		{name: "authority", value: "权威", wantDir: "authority", wantOK: true},
		{name: "historical", value: "历史", wantDir: "historical", wantOK: true},
		{name: "personal", value: "个人", wantDir: "personal", wantOK: true},
		{name: "draft", value: "草稿", wantDir: "draft", wantOK: true},
		{name: "unknown_CN", value: "未知层级", wantDir: "", wantOK: false},
		{name: "unknown_ascii_accidental", value: "authority", wantDir: "", wantOK: false},
		{name: "empty_string", value: "", wantDir: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotOK := TierEnumByValue(tt.value)
			assert.Equal(t, tt.wantDir, gotDir,
				"TierEnumByValue(%q) dir mismatch", tt.value)
			assert.Equal(t, tt.wantOK, gotOK,
				"TierEnumByValue(%q) ok mismatch — KNOW-02 indexer relies on "+
					"ok=false for any tier outside the canonical 4-value set", tt.value)
		})
	}
}

// TestTierEnumValuesDeterminism covers Task 2 behavior tests 4-5:
// exactly 4 elements, deterministic order (callers in error messages
// rely on stable ordering), and the slice is a fresh allocation per
// call so callers cannot mutate the package-level TierEnum map.
func TestTierEnumValuesDeterminism(t *testing.T) {
	first := TierEnumValues()
	second := TierEnumValues()

	// Test 4 — exactly the 4 canonical CN-char values, in the
	// TierEnum-key-sorted order documented in the godoc
	// (authority → draft → historical → personal ⇒ values
	// 权威, 草稿, 历史, 个人).
	want := []string{"权威", "草稿", "历史", "个人"}
	assert.Equal(t, want, first,
		"TierEnumValues must return the 4 canonical CN-char values in "+
			"TierEnum-key-sorted order (authority, draft, historical, personal)")

	// Test 5 — idempotent across calls (no internal mutation).
	assert.Equal(t, first, second,
		"TierEnumValues must be deterministic across calls — the package-level "+
			"TierEnum map is shared and MUST NOT be mutated by callers")

	// Mutating the returned slice MUST NOT affect the next call — proves
	// the implementation returns a fresh allocation each time.
	if len(first) > 0 {
		first[0] = "MUTATED"
		third := TierEnumValues()
		assert.Equal(t, want, third,
			"TierEnumValues must return a freshly-allocated slice — caller "+
				"mutation MUST NOT leak into subsequent calls or the TierEnum map")
	}
}
