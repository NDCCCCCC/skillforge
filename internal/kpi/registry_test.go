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

package kpi

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKPIRegistryImmutableAtRuntime uses reflect to assert that *Registry's
// exported method set is a subset of {Lookup, Catalog, Names, Signature} —
// zero mutator methods (KPI-01 structural lock).
func TestKPIRegistryImmutableAtRuntime(t *testing.T) {
	regType := reflect.TypeOf(&Registry{})

	// Collect all exported method names
	allowed := map[string]bool{
		"Lookup":    true,
		"Catalog":   true,
		"Names":     true,
		"Signature": true,
	}

	for i := 0; i < regType.NumMethod(); i++ {
		m := regType.Method(i)
		if !m.IsExported() {
			continue
		}
		assert.True(t, allowed[m.Name],
			"*Registry exports method %q which is not in the allowed read-only set {Lookup, Catalog, Names, Signature} — "+
				"KPI-01 requires zero mutator methods (no-registration-backdoor structural lock)", m.Name)
	}

	// Also verify the allowed set is exhaustive (all 4 methods exist)
	for name := range allowed {
		_, ok := regType.MethodByName(name)
		assert.True(t, ok, "*Registry is missing expected read-only method %q", name)
	}
}

// TestRegistryCatalogDefensiveCopy asserts that two calls to Catalog()
// return independent slices (modifying one does not affect the other).
func TestRegistryCatalogDefensiveCopy(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: kpi_a
    description: A
    sql: "SELECT x AS x, count(*) AS row_count FROM ds_loans.data GROUP BY x"
    count_column: row_count
  - name: kpi_b
    description: B
    sql: "SELECT y AS y, count(*) AS row_count FROM ds_loans.data GROUP BY y"
    count_column: row_count
`)

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	cat1 := reg.Catalog()
	cat2 := reg.Catalog()

	// Modify cat1
	cat1[0].Name = "MUTATED"

	// cat2 must be unaffected
	assert.NotEqual(t, "MUTATED", cat2[0].Name,
		"Catalog() must return defensive copies — mutation of one return must not affect another")
}

// TestRegistryNamesDefensiveCopy asserts that two calls to Names()
// return independent slices.
func TestRegistryNamesDefensiveCopy(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: kpi_a
    description: A
    sql: "SELECT x AS x, count(*) AS row_count FROM ds_loans.data GROUP BY x"
    count_column: row_count
`)

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	names1 := reg.Names()
	names2 := reg.Names()

	// Modify names1
	names1[0] = "MUTATED"

	// names2 must be unaffected
	assert.NotEqual(t, "MUTATED", names2[0],
		"Names() must return defensive copies")
}

// TestRegistryCatalogSorted asserts that Catalog returns entries in
// sorted (alphabetical) order by KPI name.
func TestRegistryCatalogSorted(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: z_kpi
    description: Z
    sql: "SELECT x AS x, count(*) AS row_count FROM ds_loans.data GROUP BY x"
    count_column: row_count
  - name: a_kpi
    description: A
    sql: "SELECT y AS y, count(*) AS row_count FROM ds_loans.data GROUP BY y"
    count_column: row_count
`)

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	cat := reg.Catalog()
	require.Len(t, cat, 2)
	assert.Equal(t, "a_kpi", cat[0].Name)
	assert.Equal(t, "z_kpi", cat[1].Name, "Catalog must be sorted by name")
}

// TestRegistryNamesSorted asserts that Names returns entries in sorted order.
func TestRegistryNamesSorted(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: z_kpi
    description: Z
    sql: "SELECT x AS x, count(*) AS row_count FROM ds_loans.data GROUP BY x"
    count_column: row_count
  - name: a_kpi
    description: A
    sql: "SELECT y AS y, count(*) AS row_count FROM ds_loans.data GROUP BY y"
    count_column: row_count
`)

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	names := reg.Names()
	assert.True(t, sort.StringsAreSorted(names), "Names must return sorted slice")
}

// TestRegistryCatalogParamsSummary verifies the params summary format.
func TestRegistryCatalogParamsSummary(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: kpi_with_params
    description: test
    sql: "SELECT status AS status, count(*) AS row_count FROM ds_loans.data WHERE (:status IS NULL OR status = :status) AND (:period IS NULL OR period = :period) GROUP BY status"
    count_column: row_count
    params:
      status:
        type: enum
        values: [a, b]
        required: false
      period:
        type: string
        pattern: "^2[0-9]{3}$"
        required: true
`)

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	cat := reg.Catalog()
	require.Len(t, cat, 1)
	// Summary should contain both params
	summary := cat[0].ParamsSummary
	assert.Contains(t, summary, "period:pattern")
	assert.Contains(t, summary, "status:enum(2)")
}

// TestRegistryEmptyCatalogAndNames verifies empty registry behavior.
func TestRegistryEmptyCatalogAndNames(t *testing.T) {
	dir := t.TempDir() // empty directory

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	cat := reg.Catalog()
	assert.Empty(t, cat)

	names := reg.Names()
	assert.Empty(t, names)

	sig := reg.Signature()
	assert.Empty(t, sig)
}

// TestRegistryRealLoansYAML loads the actual kpis/loans.yaml from the repo.
func TestRegistryRealLoansYAML(t *testing.T) {
	repoRoot := "../../../" // internal/kpi/ → repo root
	kpisDir := repoRoot + "kpis"
	if _, err := os.Stat(kpisDir); os.IsNotExist(err) {
		t.Skip("kpis/ directory not found (running from unexpected working directory)")
	}

	reg, sig, err := LoadKPIs(context.Background(), kpisDir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)
	require.NotNil(t, reg)

	names := reg.Names()
	assert.Equal(t, 2, len(names), "kpis/loans.yaml should define 2 KPIs")
	assert.Contains(t, names, "loan_overdue_rate_by_status")
	assert.Contains(t, names, "loan_balance_bucket_distribution")
	assert.NotEmpty(t, sig)
}

// TestRegistryCompiledKPIDataset verifies Dataset is propagated.
func TestRegistryCompiledKPIDataset(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: test_kpi
    description: test
    sql: "SELECT x AS x, count(*) AS row_count FROM ds_loans.data GROUP BY x"
    count_column: row_count
`)

	reg, _, err := LoadKPIs(context.Background(), dir, fakeSerializer{}, emptyLister{})
	require.NoError(t, err)

	kpi, ok := reg.Lookup("test_kpi")
	require.True(t, ok)
	assert.Equal(t, "loans", kpi.Dataset)
	assert.Equal(t, 0, kpi.FileKMin)
}
