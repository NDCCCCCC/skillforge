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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/stretchr/testify/assert"
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

// corpusSerializer is a hermetic kpi.SQLSerializer for loading the repo's
// real kpis/ corpus without CGO. It echoes a probe-1-style valid envelope
// whose schema_name is extracted from the statement text itself
// (ds_{dataset}.data qualified form — the v1 KPI dialect ValidateKPIFile
// enforces), so both kpis/loans.yaml and kpis/testdata.yaml compile.
//
// This mirrors the fakeSerializer discipline of internal/kpi/loader_test.go
// (probe fixtures, no CGO); that fake cannot be imported across packages
// because it lives in a _test.go file of a different package.
type corpusSerializer struct{}

var dsTableRefPattern = regexp.MustCompile(`ds_[A-Za-z0-9_-]+\.data`)

func (corpusSerializer) SerializeSQL(_ context.Context, statement string) (string, error) {
	schema := "ds_testdata" // fallback: unqualified table reference
	if m := dsTableRefPattern.FindString(statement); m != "" {
		schema = m[:len(m)-len(".data")]
	}
	const env = `{"error":false,"statements":[{"node":{"type":"SELECT_NODE",` +
		`"from_table":{"type":"BASE_TABLE","schema_name":"%s","table_name":"data"},` +
		`"select_list":[]}}]}`
	return fmt.Sprintf(env, schema), nil
}

// emptyDatasetLister is the bootstrap-window stub (Pitfall 2: empty dataset
// set → LoadKPIs skips the dataset-existence check). Same shape as
// internal/kpi's staticDatasetLister; redeclared here because that one is
// package-internal test code.
type emptyDatasetLister struct{}

func (emptyDatasetLister) ListDatasets(_ context.Context) ([]string, error) {
	return nil, nil
}

// TestValidateAgainstKPI locks the cmd/server boot contract (Phase 17
// DASH-01 wiring): AFTER Loader.Scan and AFTER kpi.LoadKPIs, every KPI name
// in a data-channel Skill's routing table must exist in the registry.
//
// The happy path uses the REAL repo directories — examples/skills (where
// kpi-dashboard lives) and kpis/ (what cmd/server loads at boot) — so a
// route added to SKILL.md without a matching kpis/*.yaml entry, or a KPI
// renamed in kpis/ without updating the routing table, fails here exactly
// as it would at boot.
func TestValidateAgainstKPI(t *testing.T) {
	// Build the registry hermetically from the real kpis/ corpus
	// (loans.yaml + testdata.yaml).
	reg, _, err := kpi.LoadKPIs(context.Background(), "../../kpis", corpusSerializer{}, emptyDatasetLister{})
	require.NoError(t, err, "kpis/ corpus must load (Layer 1 + hermetic Layer 2)")
	require.Contains(t, reg.Names(), "KPI_LOAN_BALANCE", "corpus must carry the routing-table names")

	// Load examples/skills (includes kpi-dashboard with data channel).
	sl := NewLoader("../../examples/skills")
	_, err = sl.Scan()
	require.NoError(t, err)

	// Happy path: every routed KPI resolves → nil.
	require.NoError(t, sl.ValidateAgainstKPI(reg),
		"kpi-dashboard routing table must be fully covered by the kpis/ registry")

	// Failure path: an empty registry knows no KPI → error naming the route.
	emptyReg, _, err := kpi.LoadKPIs(context.Background(), t.TempDir(), corpusSerializer{}, emptyDatasetLister{})
	require.NoError(t, err)
	err = sl.ValidateAgainstKPI(emptyReg)
	require.Error(t, err, "empty registry must fail the cross-check")
	assert.Contains(t, err.Error(), "unknown KPI")
	assert.Contains(t, err.Error(), "kpi-dashboard")

	// Skip contract: tier-channel-only Skills (no "data:" read) have no
	// routing table to validate. Built hermetically in a TempDir — the
	// deployed skills/ dir now also carries kpi-dashboard (data channel),
	// so scanning it would defeat the tier-only intent of this leg.
	tierDir := t.TempDir()
	tierSkill := filepath.Join(tierDir, "tier-only", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(tierSkill), 0o755))
	require.NoError(t, os.WriteFile(tierSkill, []byte(
		"---\ndescription: tier-only skill\nagent: ChatModelAgent\nreads:\n  - authority/**\nwrites: []\nsteps: |\n  1. answer\nguardrails:\n  prohibitions:\n    - \"Never fabricate citations\"\n    - \"Never answer without retrieved grounding\"\n    - \"Never leak other-tier content\"\ntemperature: 0.0\n---\n"), 0o644))
	tierLoader := NewLoader(tierDir)
	_, err = tierLoader.Scan()
	require.NoError(t, err)
	require.NoError(t, tierLoader.ValidateAgainstKPI(emptyReg),
		"tier-channel Skills must be skipped by ValidateAgainstKPI")
}
