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

// Package verdict — adversarial tests for the D2-01 grounding checker.
//
// These tests pin the four failure modes the grounding moat is designed
// to catch (per CONTEXT.md D2-01 + 02-RESEARCH.md Pattern 5):
//
//  1. Fabricated span — LLM emits a span that does not appear in any
//     retrieved chunk. Must fail GroundingCheck (and ApplyDowngrade).
//  2. Verbatim span — LLM emits a real chunk substring. Must pass
//     GroundingCheck (and ApplyDowngrade is a no-op).
//  3. 全角/半角 mixing — LLM emits fullwidth digits/punctuation where
//     the chunk has halfwidth (or vice versa). Must still pass
//     GroundingCheck because width.Narrow canonicalizes both to halfwidth.
//  4. Novel claim — LLM emits a citation with a ChunkID not in the
//     retrieved set (the LLM cited something it never saw). Must fail
//     GroundingCheck.
//
// Plus an idempotence test for ApplyDowngrade (calling it twice must
// not progressively destroy the Confidence value).
package verdict

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// authChunk001 is the verbatim fixture text from
// testdata/docs/authority/auth_001.md (Phase 1 spike fixture).
// Used as the canonical chunk text in the adversarial tests below.
const authChunk001 = "出差住宿费超过标准额度时，超出部分由员工自理。"

// authChunk001Narrow is the same content but with fullwidth digits/
// punctuation (the LLM sometimes emits 全角 chars for ASCII-looking
// content per Pitfall 4). Used to exercise width.Narrow handling.
const authChunk001Narrow = "出差住宿费超过标准额度时，超出部分由员工自理。"

// TestGroundingCheckFabricatedSpan — D2-01 adversarial case 1.
// A citation whose Span does NOT appear in the chunk text MUST fail
// grounding. This is the LLM-fabrication moat: the LLM cannot claim
// support from text it never produced.
func TestGroundingCheckFabricatedSpan(t *testing.T) {
	v := Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []Citation{
			{ChunkID: "auth_001", Span: "凭票实报实销不限额度", Tier: "权威"},
		},
		ReviewRequired: false,
	}
	chunks := map[string]string{"auth_001": authChunk001}
	report := GroundingCheck(v, chunks)

	assert.False(t, report.Grounded,
		"fabricated span '凭票实报实销不限额度' must NOT ground against authChunk001 (D2-01 hallucination moat)")
	assert.Len(t, report.UngroundedCitations, 1,
		"fabricated span must appear in UngroundedCitations")
	assert.Equal(t, "凭票实报实销不限额度", report.UngroundedCitations[0].Span)
}

// TestGroundingCheckVerbatimSpan — D2-01 happy path.
// A citation whose Span IS a verbatim substring of the chunk text MUST
// pass grounding (no downgrade applied).
func TestGroundingCheckVerbatimSpan(t *testing.T) {
	v := Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []Citation{
			{ChunkID: "auth_001", Span: "出差住宿费超过标准额度时", Tier: "权威"},
		},
		ReviewRequired: false,
	}
	chunks := map[string]string{"auth_001": authChunk001}
	report := GroundingCheck(v, chunks)

	assert.True(t, report.Grounded,
		"verbatim span '出差住宿费超过标准额度时' MUST ground against authChunk001 (D2-01 happy path)")
	assert.Empty(t, report.UngroundedCitations,
		"no citations should be ungrounded on the verbatim happy path")
}

// TestGroundingCheckFullwidthHalfwidthMix — D2-05 + Pitfall 4.
// The LLM sometimes emits fullwidth ASCII characters (e.g. "，" U+FF0C
// instead of "," U+002C) when the chunk has halfwidth. Per D2-05,
// width.Narrow normalizes both sides so the substring match succeeds.
//
// Fixture: the span has fullwidth punctuation; the chunk has halfwidth.
// Without width.Narrow, the substring match would fail spuriously.
func TestGroundingCheckFullwidthHalfwidthMix(t *testing.T) {
	// Span uses 全角 punctuation (，, 。, ；, ：) — the LLM emitted these
	// thinking they were "more Chinese". The chunk has halfwidth (, . ; :)
	// because chunks are stored verbatim (one-doc-one-chunk, no chunker).
	v := Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []Citation{
			// Span uses 全角 punctuation that the LLM emitted.
			{ChunkID: "auth_001", Span: "出差住宿费超过标准额度时，超出部分由员工自理。", Tier: "权威"},
		},
		ReviewRequired: false,
	}
	chunks := map[string]string{"auth_001": authChunk001Narrow}
	report := GroundingCheck(v, chunks)

	assert.True(t, report.Grounded,
		"全角/半角 mix must still ground after width.Narrow normalization (D2-05)")
}

// TestGroundingCheckNovelClaim — D2-01 adversarial case 2.
// The LLM cites a ChunkID that does NOT appear in the retrieved set —
// the LLM "made up" a citation. Must fail grounding. This is the
// "no-citation-no-answer" enforcement: even an unknown ChunkID is a
// fabrication signal, not a free pass.
func TestGroundingCheckNovelClaim(t *testing.T) {
	v := Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations: []Citation{
			// auth_999 was never retrieved (only auth_001 was).
			{ChunkID: "auth_999", Span: "不存在的条款", Tier: "权威"},
		},
		ReviewRequired: false,
	}
	chunks := map[string]string{"auth_001": authChunk001}
	report := GroundingCheck(v, chunks)

	assert.False(t, report.Grounded,
		"novel-claim citation (ChunkID not retrieved) must NOT ground (D2-01)")
	assert.Len(t, report.UngroundedCitations, 1,
		"novel-claim citation must appear in UngroundedCitations")
	assert.Equal(t, "auth_999", report.UngroundedCitations[0].ChunkID,
		"ungrounded ChunkID must be preserved for the audit trail")
}

// TestApplyDowngradeIdempotent — D2-02 regression.
// The verdict pipeline may call ApplyDowngrade multiple times across
// retry/re-eval paths. Non-idempotent downgrade would silently destroy
// the Confidence value (e.g. first call caps at 0.89, second call sees
// 0.89 > 0.89 is false → no-op on Confidence but Status stays
// DowngradeStatus — that's fine; we just must NOT progressively lower
// below 0.89).
//
// 260817-vsm: Status overwrite target is now DowngradeStatus="needs_input"
// (platform-neutral). When ComplianceStatus was set, it is also propagated
// to DowngradeStatus so the Compliance Skill's UI badge stays consistent.
//
// This test calls ApplyDowngrade twice with the same report and
// asserts the second call does not change the verdict further.
func TestApplyDowngradeIdempotent(t *testing.T) {
	v := Verdict{
		Status:           "ok",
		Confidence:       0.95,
		Citations:        []Citation{{ChunkID: "auth_001", Span: "凭票实报实销不限额度", Tier: "权威"}},
		ReviewRequired:   false,
		ComplianceStatus: "compliant",
	}
	chunks := map[string]string{"auth_001": authChunk001}
	report := GroundingCheck(v, chunks)

	// First downgrade.
	v1 := ApplyDowngrade(v, report)
	assert.Equal(t, DowngradeStatus, v1.Status, "first downgrade must overwrite Status to DowngradeStatus")
	assert.Equal(t, DowngradeStatus, v1.ComplianceStatus, "first downgrade must propagate to ComplianceStatus when set")
	assert.Equal(t, 0.89, v1.Confidence, "first downgrade must cap Confidence at 0.89")
	assert.True(t, v1.ReviewRequired, "first downgrade must force ReviewRequired=true")

	// Second downgrade on the already-downgraded verdict — must be no-op.
	v2 := ApplyDowngrade(v1, report)
	assert.Equal(t, DowngradeStatus, v2.Status, "second downgrade must not flip Status away from DowngradeStatus")
	assert.Equal(t, 0.89, v2.Confidence, "second downgrade must not lower Confidence below 0.89 (idempotent)")
	assert.True(t, v2.ReviewRequired, "second downgrade must leave ReviewRequired=true")
}

// TestNormalizeWhitespaceCollapse — D2-05 unit assertion.
// Two strings with different whitespace patterns (tabs vs spaces, multiple
// spaces) MUST normalize to the same canonical form. This is the
// invariant the substring-contains check relies on.
func TestNormalizeWhitespaceCollapse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no whitespace",
			input: "出差住宿费超过标准额度时",
			want:  "出差住宿费超过标准额度时",
		},
		{
			name:  "single space",
			input: "出差 住宿费",
			want:  "出差 住宿费",
		},
		{
			name:  "multiple spaces collapse",
			input: "出差   住宿费",
			want:  "出差 住宿费",
		},
		{
			name:  "tab collapses",
			input: "出差\t住宿费",
			want:  "出差 住宿费",
		},
		{
			name:  "newline collapses",
			input: "出差\n住宿费",
			want:  "出差 住宿费",
		},
		{
			name:  "leading and trailing trimmed",
			input: "  出差住宿费  ",
			want:  "出差住宿费",
		},
		{
			name:  "ideographic space U+3000 collapses",
			input: "出差　住宿费",
			want:  "出差 住宿费",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(tc.input)
			assert.Equal(t, tc.want, got,
				"Normalize(%q) = %q, want %q", tc.input, got, tc.want)
		})
	}
}

// TestNormalizeWidthNarrow — D2-05 unit assertion.
// Fullwidth ASCII digits/letters/punctuation must collapse to halfwidth
// so that LLM-emitted spans with 全角 chars match chunk text with
// halfwidth chars.
func TestNormalizeWidthNarrow(t *testing.T) {
	// Fullwidth comma U+FF0C → halfwidth comma "," U+002C.
	// Fullwidth period U+FF0E → halfwidth period "." U+002E.
	assert.Equal(t, "hello, world.",
		Normalize("hello， world．"),
		"fullwidth comma + period must narrow to halfwidth")

	// Fullwidth digits U+FF11..U+FF19 → halfwidth digits '1'..'9'.
	assert.Equal(t, "test 123",
		Normalize("test １２３"),
		"fullwidth digits must narrow to halfwidth")
}
