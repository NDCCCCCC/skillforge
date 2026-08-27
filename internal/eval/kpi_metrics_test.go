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

package eval

import (
	"strings"
	"testing"
)

func TestExtractNumbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []float64
	}{
		{
			name:  "plain numbers",
			input: "贷款余额是42315000000元",
			want:  []float64{42315000000},
		},
		{
			name:  "numbers with commas",
			input: "4,231.5亿元",
			want:  []float64{4231.5},
		},
		{
			name:  "multiple numbers",
			input: "不良率2.3% 逾期率1.8%",
			want:  []float64{2.3, 1.8},
		},
		{
			name:  "mixed with text",
			input: "上月贷款余额4,231.5亿元，不良率2.3%",
			want:  []float64{4231.5, 2.3},
		},
		{
			name:  "negative number",
			input: "变化-123.5万",
			want:  []float64{-123.5},
		},
		{
			name:  "percentage",
			input: "利率为5.25%",
			want:  []float64{5.25},
		},
		{
			name:  "no numbers",
			input: "没有数字的文本",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractNumbers(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractNumbers(%q) len=%d, want len=%d; got=%v",
					tt.input, len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractNumbers(%q)[%d]=%v, want %v",
						tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", 42315000000.0, 42315000000.0, true},
		{"float32", float32(2.0), 2.0, true},
		{"int", 100, 100.0, true},
		{"int64", int64(999999999), 999999999.0, true},
		{"string int", "42315000000", 42315000000.0, true},
		{"string float", "2.3", 2.3, true},
		{"string negative", "-123.5", -123.5, true},
		{"string invalid", "not-a-number", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat(tt.input)
			if ok != tt.ok {
				t.Errorf("toFloat(%T(%v)) ok=%v, want ok=%v", tt.input, tt.input, ok, tt.ok)
				return
			}
			if ok && got != tt.want {
				t.Errorf("toFloat(%T(%v))=%v, want %v", tt.input, tt.input, got, tt.want)
			}
		})
	}
}

func TestThreeStepValueMatch(t *testing.T) {
	baseRows := []map[string]any{
		{"period": "2026-07", "loan_balance": 42315000000.0, "npl_ratio": 0.023},
		{"period": "2026-06", "loan_balance": 41800000000.0, "npl_ratio": 0.022},
	}

	tests := []struct {
		name       string
		caseID     string
		answerText string
		c          *KPICase
		wantReason string
		wantPass   bool
	}{
		{
			name:       "ok: as_of + channel + value match",
			caseID:     "kpi-001",
			answerText: "上月（as_of）贷款余额42315000000元，data:aggregated_only，KPI_LOAN_BALANCE",
			c: &KPICase{
				ID:                 "kpi-001",
				ExpectedKPI:        "KPI_LOAN_BALANCE",
				ExpectedValue:      42315000000.0,
				AllowedExpressions: []string{"4,231.5亿元", "423150000000"},
			},
			wantPass:   true,
			wantReason: "ok",
		},
		{
			name:       "fail: missing as_of",
			caseID:     "kpi-002",
			answerText: "不良率2.3%，data:aggregated_only",
			c: &KPICase{
				ID:            "kpi-002",
				ExpectedKPI:   "KPI_NPL_RATIO",
				ExpectedValue: 0.023,
			},
			wantPass:   false,
			wantReason: "missing_as_of",
		},
		{
			name:       "fail: missing channel",
			caseID:     "kpi-003",
			answerText: "截至上月的贷款余额是42315000000元",
			c: &KPICase{
				ID:            "kpi-003",
				ExpectedKPI:   "KPI_LOAN_BALANCE",
				ExpectedValue: 42315000000.0,
			},
			wantPass:   false,
			wantReason: "missing_channel",
		},
		{
			name:       "fail: value mismatch not in snapshot",
			caseID:     "kpi-004",
			answerText: "截至上月不良率3.5%，data:aggregated_only",
			c: &KPICase{
				ID:            "kpi-004",
				ExpectedKPI:   "KPI_NPL_RATIO",
				ExpectedValue: 0.023,
			},
			wantPass:   false,
			wantReason: "value_mismatch",
		},
		{
			name:       "ok: value matches allowed expression (4,231.5亿)",
			caseID:     "kpi-005",
			answerText: "截至上月，贷款余额4,231.5亿元，data:aggregated_only",
			c: &KPICase{
				ID:                 "kpi-005",
				ExpectedKPI:        "KPI_LOAN_BALANCE",
				ExpectedValue:      42315000000.0,
				AllowedExpressions: []string{"4231.5", "4231.5亿元"},
			},
			wantPass:   true,
			wantReason: "ok",
		},
		{
			name:       "ok: multi-KPI answer with both values",
			caseID:     "kpi-006",
			answerText: "截至上月，贷款余额42315000000元，不良率0.023，data:aggregated_only",
			c: &KPICase{
				ID:            "kpi-006",
				ExpectedKPIs:  []string{"KPI_LOAN_BALANCE", "KPI_NPL_RATIO"},
				ExpectedValues: []float64{42315000000.0, 0.023},
			},
			wantPass:   true,
			wantReason: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := JudgeKPIReferenceConsistency(tt.c, tt.answerText, baseRows)
			if !strings.HasPrefix(j.Reason, tt.wantReason) {
				t.Errorf("JudgeKPIReferenceConsistency reason=%q, want prefix %q", j.Reason, tt.wantReason)
			}
			if j.Passed != tt.wantPass {
				t.Errorf("JudgeKPIReferenceConsistency passed=%v, want %v", j.Passed, tt.wantPass)
			}
			if j.CaseID != tt.c.ID {
				t.Errorf("JudgeKPIReferenceConsistency CaseID=%q, want %q", j.CaseID, tt.c.ID)
			}
		})
	}
}

func TestContainsASOf(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"as_of 2026-08-01", true},
		{"数据截至2026-07", true},
		{"截至2026年Q2", true},
		{"截止2026-07-31", true},
		{"不良率2.3%", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := containsASOf(tt.input); got != tt.want {
				t.Errorf("containsASOf(%q)=%v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
