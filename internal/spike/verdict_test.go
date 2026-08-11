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

// Package spike — tests for the Verdict schema gate (T-1-16 mitigation).
//
// These tests pin the schema at the type level so that the integration
// contract test (integration/contract/contract_test.go) can import the
// Validate function without re-implementing the schema rules. Drift
// between the cmd/spiked and integration/contract packages is caught
// at compile time (no local duplicate type); drift between the rules
// here and the rules the contract test expects is caught by the
// contract test's own Validate assertions (TestContractStructuredOutputSchema).
package spike

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateVerdictAcceptsValid covers every ValidStatuses key plus
// the boundary confidence values 0.0 and 1.0. A regression here would
// mean the production pipeline rejects verdicts the spec allows.
func TestValidateVerdictAcceptsValid(t *testing.T) {
	cases := []Verdict{
		{Status: "合规", Confidence: 0.85, Citations: []string{"auth_001"}, ReviewRequired: false},
		{Status: "不合规", Confidence: 0.95, Citations: []string{"auth_002"}, ReviewRequired: false},
		{Status: "需补充", Confidence: 0.4, Citations: []string{}, ReviewRequired: true},
		{Status: "unanswered", Confidence: 0.0, Citations: []string{}, ReviewRequired: true},
		{Status: "合规", Confidence: 1.0, Citations: []string{"auth_001", "auth_002"}, ReviewRequired: false},
	}
	for i, v := range cases {
		ok, reason := Validate(v)
		assert.True(t, ok, "case %d (%s) should validate; got reason: %s", i, v.Status, reason)
		assert.Empty(t, reason, "case %d (%s) should have empty reason", i, v.Status)
	}
}

// TestValidateVerdictRejectsUnknownStatus verifies the enum gate. A new
// status value can only be added by editing ValidStatuses in verdict.go
// — at which point this test must be updated. This forces an explicit
// decision when the schema grows.
func TestValidateVerdictRejectsUnknownStatus(t *testing.T) {
	unknown := []string{"INVALID", "", "compliant", "yes", "合规️"} // last is 合规 + variation selector
	for i, s := range unknown {
		v := Verdict{Status: s, Confidence: 0.5, Citations: []string{}}
		ok, reason := Validate(v)
		assert.False(t, ok, "case %d status=%q should be rejected", i, s)
		assert.Contains(t, reason, "invalid status", "case %d status=%q reason should mention invalid status", i, s)
	}
}

// TestValidateVerdictRejectsConfidenceRange checks both the negative
// and >1.0 sides. Confidence is a float and JSON decodes 1e308 silently
// — Validate is the only gate.
func TestValidateVerdictRejectsConfidenceRange(t *testing.T) {
	outOfRange := []float64{-0.01, -1.0, 1.0001, 1.5, 100.0}
	for i, c := range outOfRange {
		v := Verdict{Status: "合规", Confidence: c, Citations: []string{}}
		ok, reason := Validate(v)
		assert.False(t, ok, "case %d confidence=%f should be rejected", i, c)
		assert.Contains(t, reason, "confidence", "case %d confidence=%f reason should mention confidence", i, c)
	}
}

// TestValidateVerdictRejectsNilCitations covers the Go-specific footgun
// where a missing JSON array decodes to a nil slice. The contract test
// (TestContractStructuredOutputSchema) asserts that the round-trip JSON
// parse produces a non-nil empty slice when the LLM emits "citations":[].
// Here we assert Validate rejects the nil slice explicitly.
func TestValidateVerdictRejectsNilCitations(t *testing.T) {
	v := Verdict{Status: "合规", Confidence: 0.5, Citations: nil}
	ok, reason := Validate(v)
	assert.False(t, ok, "nil citations should be rejected (must be empty array at minimum)")
	assert.Contains(t, reason, "citations")
}

// TestValidStatusesEnum pins the exact rune sequence of the status enum.
// A typo here (e.g. a half-width vs full-width comma, a variation
// selector, a traditional vs simplified variant) would silently change
// semantics. The contract test re-checks this; this test catches the
// typo at the internal/spike package level first.
func TestValidStatusesEnum(t *testing.T) {
	expected := map[string]bool{
		"合规":      true,
		"不合规":     true,
		"需补充":     true,
		"unanswered": true,
	}
	assert.Equal(t, expected, ValidStatuses,
		"ValidStatuses must contain exactly the 4 expected keys with the exact rune sequences")
	assert.Equal(t, 4, len(ValidStatuses), "ValidStatuses must have exactly 4 entries")
}
