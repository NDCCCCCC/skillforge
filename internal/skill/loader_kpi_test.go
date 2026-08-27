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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKPISkillStepsRoutingTable(t *testing.T) {
	// Load kpi-dashboard SKILL.md
	sl := NewLoader("../../examples/skills")
	_, err := sl.Scan()
	require.NoError(t, err)

	// Find kpi-dashboard manifest
	var manifest *Manifest
	for _, m := range sl.Skills() {
		if m.Name == "kpi-dashboard" {
			manifest = m
			break
		}
	}
	require.NotNil(t, manifest, "kpi-dashboard skill not found")

	// Parse routing table
	entries, err := parseRoutingTable(manifest.Steps)
	require.NoError(t, err)
	require.Len(t, entries, 4, "expected 4 routing entries (KPI_LOAN_BALANCE, KPI_NPL_RATIO, KPI_INTEREST_INCOME, KPI_CUSTOMER_COUNT)")

	// All KPI names non-empty
	for _, e := range entries {
		require.NotEmpty(t, e.KPI)
		require.NotEmpty(t, e.Group)
	}

	// Verify KPI names expected
	kpiNames := make(map[string]bool)
	for _, e := range entries {
		kpiNames[e.KPI] = true
	}
	require.True(t, kpiNames["KPI_LOAN_BALANCE"])
	require.True(t, kpiNames["KPI_NPL_RATIO"])
	require.True(t, kpiNames["KPI_INTEREST_INCOME"])
	require.True(t, kpiNames["KPI_CUSTOMER_COUNT"])
}
