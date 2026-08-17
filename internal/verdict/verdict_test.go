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

// Package verdict — tests for the Verdict schema gate (T-1-16 mitigation).
//
// Graduated from internal/spike/verdict_test.go in Plan 02-01 with the
// D2-06 evolution: Citations is now []Citation instead of []string.
//
// These tests pin the schema at the type level so that the integration
// contract test (integration/contract/contract_test.go) can import the
// Validate function without re-implementing the schema rules. Drift
// between the scripts/spiked producer / 02-03 verdict pipeline producer and
// the integration/contract validator is caught at compile time (no local
// duplicate type); drift between the rules here and the rules the
// contract test expects is caught by the contract test's own Validate
// assertions (TestContractStructuredOutputSchema).
package verdict

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateVerdictAcceptsValid covers every ValidStatuses key plus
// the boundary confidence values 0.0 and 1.0. A regression here would
// mean the production pipeline rejects verdicts the spec allows.
//
// 260817-vsm: ValidStatuses is now platform-neutral
// {ok, needs_input, refused, error}. Compliance-specific verdict codes
// (compliant / non_compliant / not_applicable) live in
// Verdict.ComplianceStatus, not Verdict.Status, so they do NOT appear
// here.
//
// D2-06: citations use the new []Citation shape (not []string).
func TestValidateVerdictAcceptsValid(t *testing.T) {
	cases := []Verdict{
		{Status: "ok", Confidence: 0.85, Citations: []Citation{{ChunkID: "auth_001", Span: "出差住宿费超过标准额度时", Tier: "权威"}}, ReviewRequired: false, ComplianceStatus: "compliant"},
		{Status: "ok", Confidence: 0.95, Citations: []Citation{{ChunkID: "auth_002", Span: "竞业限制期", Tier: "权威"}}, ReviewRequired: false, ComplianceStatus: "non_compliant"},
		{Status: "needs_input", Confidence: 0.4, Citations: []Citation{}, ReviewRequired: true, ComplianceStatus: "needs_input"},
		{Status: "refused", Confidence: 0.0, Citations: []Citation{}, ReviewRequired: true},
		{Status: "ok", Confidence: 1.0, Citations: []Citation{{ChunkID: "auth_001", Span: "span1", Tier: "权威"}, {ChunkID: "auth_002", Span: "span2", Tier: "权威"}}, ReviewRequired: false, Answer: "由部门负责人审批"},
	}
	for i, v := range cases {
		ok, reason := Validate(v)
		assert.True(t, ok, "case %d (%s) should validate; got reason: %s", i, v.Status, reason)
		assert.Empty(t, reason, "case %d (%s) should have empty reason", i, v.Status)
	}
}

// TestValidateVerdictRejectsUnknownStatus verifies the enum gate. A new
// status value can only be added by editing ValidStatuses in validate.go
// — at which point this test must be updated. This forces an explicit
// decision when the schema grows.
//
// 260817-vsm: old compliance-specific Chinese values (合规 / 不合规 /
// 需补充 / unanswered) are now invalid Status values. They live in
// Verdict.ComplianceStatus when populated.
func TestValidateVerdictRejectsUnknownStatus(t *testing.T) {
	unknown := []string{"INVALID", "", "compliant", "yes", "合规️", "合规", "不合规", "需补充", "unanswered"} // last 4 were Status pre-260817-vsm
	for i, s := range unknown {
		v := Verdict{Status: s, Confidence: 0.5, Citations: []Citation{}}
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
		v := Verdict{Status: "ok", Confidence: c, Citations: []Citation{}}
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
	v := Verdict{Status: "ok", Confidence: 0.5, Citations: nil}
	ok, reason := Validate(v)
	assert.False(t, ok, "nil citations should be rejected (must be empty array at minimum)")
	assert.Contains(t, reason, "citations")
}

// TestValidStatusesEnum pins the exact rune sequence of the status enum.
// A typo here (e.g. a half-width vs full-width comma, a variation
// selector, a traditional vs simplified variant) would silently change
// semantics. The contract test re-checks this; this test catches the
// typo at the internal/verdict package level first.
//
// 260817-vsm: enum is platform-neutral; Compliance-Skill-specific codes
// are validated separately in TestComplianceStatusesEnum.
func TestValidStatusesEnum(t *testing.T) {
	expected := map[string]bool{
		"ok":          true,
		"needs_input": true,
		"refused":     true,
		"error":       true,
	}
	assert.Equal(t, expected, ValidStatuses,
		"ValidStatuses must contain exactly the 4 expected keys with the exact rune sequences")
	assert.Equal(t, 4, len(ValidStatuses), "ValidStatuses must have exactly 4 entries")
}

// TestComplianceStatusesEnum pins the Compliance-Skill-specific verdict
// enum (Verdict.ComplianceStatus). 260817-vsm: separated from the
// platform-neutral ValidStatuses to allow other Skills (simple-qa /
// calculator) to use the platform envelope without inheriting
// compliance vocabulary.
func TestComplianceStatusesEnum(t *testing.T) {
	expected := map[string]bool{
		"compliant":     true,
		"non_compliant": true,
		"not_applicable": true,
	}
	assert.Equal(t, expected, ComplianceStatuses,
		"ComplianceStatuses must contain exactly the 3 expected keys")
	assert.Equal(t, 3, len(ComplianceStatuses), "ComplianceStatuses must have exactly 3 entries")
}
