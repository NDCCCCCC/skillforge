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

package report

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tmplWithKPIs(t *testing.T) *ReportTemplate {
	t.Helper()
	return &ReportTemplate{
		Title: "2026-08",
		Sections: []SectionTemplate{
			{Title: "贷款质量", KPIs: []string{"KPI_NPL_RATIO", "KPI_INTEREST_INCOME"}, Slots: []Slot{{Type: "risk_note"}, {Type: "narrative"}}},
			{Title: "业务概览", KPIs: []string{"KPI_LOAN_BALANCE"}, Slots: []Slot{{Type: "narrative"}}},
		},
	}
}

// mapSnap is a minimal Snapshot adapter used by these tests.
type mapSnap map[string]any

func (m mapSnap) Get(k string) any { return m[k] }

// TestCheckNarrativeConsistencyDetectsMismatch — WR-04: a narrative
// number that matches NO section KPI within ±0.5% surfaces as a
// mismatch (the previous stub always returned Clean).
func TestCheckNarrativeConsistencyDetectsMismatch(t *testing.T) {
	tmpl := tmplWithKPIs(t)
	slots := SlotTexts{0: {0: "NPL 率 9.99%，业务稳健。"}} // 9.99 vs 1.23 / 1000000
	snap := mapSnap{"KPI_NPL_RATIO": 1.23, "KPI_INTEREST_INCOME": 1000000.0}
	got := CheckNarrativeConsistency(tmpl, slots, snap, DefaultValueFormatter)
	assert.False(t, got.Clean, "the 9.99 narrative value must not match any section KPI")
	require.Len(t, got.Mismatches, 1)
	assert.Equal(t, 0, got.Mismatches[0].SectionIdx)
	assert.Equal(t, "9.99", got.Mismatches[0].ActualValue)
}

// TestCheckNarrativeConsistencyMatchesPercentNarration — a percent
// narration like "1.23%" matches the underlying ratio stored as
// 0.0123 (×100 form).
func TestCheckNarrativeConsistencyMatchesPercentNarration(t *testing.T) {
	tmpl := tmplWithKPIs(t)
	slots := SlotTexts{0: {0: "NPL 率 1.23%。"}}
	snap := mapSnap{"KPI_NPL_RATIO": 0.0123, "KPI_INTEREST_INCOME": 0}
	got := CheckNarrativeConsistency(tmpl, slots, snap, DefaultValueFormatter)
	assert.True(t, got.Clean, "1.23%% should match 0.0123 via the ×100 form")
}

// TestCheckNarrativeConsistencySkipsAllowedExpressionsPerNumber —
// "环比增长5%" skips the comparison for the 5 (allowed expression
// precedes it) but a non-allowed 99 in the same text still mismatches.
func TestCheckNarrativeConsistencySkipsAllowedExpressionsPerNumber(t *testing.T) {
	tmpl := tmplWithKPIs(t)
	slots := SlotTexts{0: {0: "环比增长 5%，出现异常 99。"}}
	snap := mapSnap{"KPI_NPL_RATIO": 1.23, "KPI_INTEREST_INCOME": 0}
	got := CheckNarrativeConsistency(tmpl, slots, snap, DefaultValueFormatter)
	require.Len(t, got.Mismatches, 1, "only the 99 should be flagged (5 is allowed-expression-adjacent)")
	assert.Equal(t, "99", got.Mismatches[0].ActualValue)
}

// TestCheckNarrativeConsistencyNilSnapshotIsClean — Render's
// fail-safe: a nil snapshot returns Clean (no comparisons possible).
func TestCheckNarrativeConsistencyNilSnapshotIsClean(t *testing.T) {
	tmpl := tmplWithKPIs(t)
	slots := SlotTexts{0: {0: "随便写 42 也不行。"}}
	got := CheckNarrativeConsistency(tmpl, slots, nil, DefaultValueFormatter)
	assert.True(t, got.Clean)
	assert.Empty(t, got.Mismatches)
}

// TestRenderMarkersSurvivePipeline — the previous HTML-comment markers
// were stripped by goldmark + bluemonday; the new blockquote marker
// must survive the sandcastle and reach the published HTML byte stream
// as a visible inconsistency hint.
func TestRenderMarkersSurvivePipeline(t *testing.T) {
	tmpl := tmplWithKPIs(t)
	slots := SlotTexts{0: {0: "NPL 率 9.99%。"}}
	snap := mapSnap{"KPI_NPL_RATIO": 1.23, "KPI_INTEREST_INCOME": 0}
	html, err := Render(tmpl, slots, snap, DefaultValueFormatter)
	require.NoError(t, err)
	body := string(html)
	assert.True(t, strings.Contains(body, "一致性提示"),
		"the WR-04 marker text must reach the published HTML; got body head: %q", body[:min(200, len(body))])
	assert.True(t, strings.Contains(body, "9.99"),
		"the offending narrative value must appear in the marker")
	assert.False(t, strings.Contains(body, "<!-- consistency_mismatch"),
		"the old HTML-comment marker must not be present (goldmark would have stripped it)")
}