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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kpi"
)

// newTestRegistry returns the hermetic registry built from the real
// kpis/ corpus (mirrors loader_kpi_test.go's TestValidateAgainstKPI
// helper) — KPI_LOAN_BALANCE + KPI_NPL_RATIO + every other registered
// KPI resolve.
func newTestRegistry(t *testing.T) *kpi.Registry {
	t.Helper()
	reg, _, err := kpi.LoadKPIs(context.Background(), "../../kpis", corpusSerializer{}, emptyDatasetLister{})
	require.NoError(t, err, "kpis/ corpus must load (KPI_LOAN_BALANCE + KPI_NPL_RATIO)")
	return reg
}

// TestValidateTemplateKPIs_NilTemplateSkipped — the only skip path;
// a Skill without a template is fine.
func TestValidateTemplateKPIs_NilTemplateSkipped(t *testing.T) {
	require.NoError(t, validateTemplateKPIs("monthly-report", nil, newTestRegistry(t)))
}

// TestValidateTemplateKPIs_StructuralSurprises — WR-06 fail-closed:
// every structural surprise must surface as an error, never silent skip.
func TestValidateTemplateKPIs_StructuralSurprises(t *testing.T) {
	reg := newTestRegistry(t)

	cases := []struct {
		name     string
		template any
		wantSub  string
	}{
		{
			name:     "template not a map (string)",
			template: "oops",
			wantSub:  "must be a YAML mapping",
		},
		{
			name:     "sections field missing",
			template: map[string]any{"title": "t"},
			wantSub:  `missing required "sections"`,
		},
		{
			name:     "sections not a list",
			template: map[string]any{"sections": "oops"},
			wantSub:  "must be a list",
		},
		{
			name:     "section entry not a map",
			template: map[string]any{"sections": []any{"oops"}},
			wantSub:  "each entry must be a mapping",
		},
		{
			name: "kpis not a list",
			template: map[string]any{"sections": []any{
				map[string]any{"title": "s", "kpis": "oops"},
			}},
			wantSub: "must be a list",
		},
		{
			name: "kpi entry not a string",
			template: map[string]any{"sections": []any{
				map[string]any{"title": "s", "kpis": []any{42}},
			}},
			wantSub: "must be a string",
		},
		{
			name: "kpi name not registered",
			template: map[string]any{"sections": []any{
				map[string]any{"title": "s", "kpis": []any{"KPI_DOES_NOT_EXIST"}},
			}},
			wantSub: "unknown KPI",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTemplateKPIs("monthly-report", tc.template, reg)
			require.Error(t, err, "fail-closed: structural surprises must reject the skill load")
			assert.True(t, strings.Contains(err.Error(), tc.wantSub),
				"error must mention %q; got %q", tc.wantSub, err.Error())
		})
	}
}

// TestValidateTemplateKPIs_HappyPath — a structurally valid template
// whose KPI names resolve passes (mirrors the loader_kpi_test happy
// path for templates).
func TestValidateTemplateKPIs_HappyPath(t *testing.T) {
	reg := newTestRegistry(t)
	tmpl := map[string]any{
		"title": "2026-08 业务月报",
		"sections": []any{
			map[string]any{"title": "贷款质量", "kpis": []any{"KPI_NPL_RATIO"}, "slots": []any{}},
		},
	}
	require.NoError(t, validateTemplateKPIs("monthly-report", tmpl, reg))
}

// TestValidateTemplateKPIs_SectionWithoutKPIsIsOK — a section may
// declare no KPIs (slots-only).
func TestValidateTemplateKPIs_SectionWithoutKPIsIsOK(t *testing.T) {
	reg := newTestRegistry(t)
	tmpl := map[string]any{
		"title": "t",
		"sections": []any{
			map[string]any{"title": "s", "kpis": nil, "slots": []any{map[string]any{"type": "narrative"}}},
		},
	}
	require.NoError(t, validateTemplateKPIs("monthly-report", tmpl, reg))
}