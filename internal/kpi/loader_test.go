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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeYAML writes a KPI YAML file to dir/filename.
func writeYAML(t *testing.T, dir, filename, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", fullPath, err)
	}
}

const validLoansYAML = `dataset: loans
description: 贷款业务核心指标
k_min: 0
kpis:
  - name: loan_overdue_rate_by_status
    description: 各状态贷款逾期率
    sql: |
      SELECT status AS status, count(*) AS row_count
      FROM ds_loans.data
      WHERE (:status IS NULL OR status = :status)
      GROUP BY status
    count_column: row_count
    params:
      status:
        type: enum
        values: [正常, 逾期]
        required: false
  - name: loan_balance_bucket_distribution
    description: 贷款余额桶分布
    sql: |
      SELECT balance AS balance_bucket, count(*) AS row_count
      FROM ds_loans.data
      GROUP BY balance
      ORDER BY balance_bucket
    count_column: row_count
    params: {}
`

// okSerializer returns a fake SQLSerializer that responds to valid SQL
// with a probe-1-style envelope (single BASE_TABLE reference to ds_{dataset}.data).
func okSerializer(dataset string) *fakeSerializer {
	expectedSchema := "ds_" + dataset
	env := map[string]any{
		"error": false,
		"statements": []map[string]any{
			{
				"node": map[string]any{
					"type": "SELECT_NODE",
					"from_table": map[string]any{
						"type":        "BASE_TABLE",
						"schema_name": expectedSchema,
						"table_name":  "data",
					},
					"select_list": []any{},
				},
			},
		},
	}
	raw, _ := json.Marshal(env)
	return &fakeSerializer{json: string(raw)}
}

// errorSerializer returns a fake SQLSerializer that returns error:true envelope.
func errorSerializer(errorType, errorMsg string) *fakeSerializer {
	env := map[string]any{
		"error":         true,
		"error_type":    errorType,
		"error_message": errorMsg,
	}
	raw, _ := json.Marshal(env)
	return &fakeSerializer{json: string(raw)}
}

// emptyDatasetLister returns a DatasetLister that returns an empty list.
func emptyDatasetLister() DatasetLister {
	return &staticDatasetLister{datasets: nil}
}

// staticDatasetLister is a test DatasetLister returning a fixed list.
type staticDatasetLister struct {
	datasets []string
}

func (l *staticDatasetLister) ListDatasets(_ context.Context) ([]string, error) {
	return l.datasets, nil
}

// loadKPIsTestHelper calls LoadKPIs with the standard test setup:
// - context.Background()
// - okSerializer for the given dataset
// - emptyDatasetLister (bootstrap window)
func loadKPIsTestHelper(t *testing.T, dir, dataset string) (*Registry, Signature, error) {
	t.Helper()
	return LoadKPIs(context.Background(), dir, okSerializer(dataset), emptyDatasetLister())
}

func TestLoadKPIs_ValidDirectory(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, sig, err := loadKPIsTestHelper(t, dir, "loans")
	require.NoError(t, err)
	require.NotNil(t, reg)

	// Should have 2 KPIs
	names := reg.Names()
	assert.Equal(t, 2, len(names), "expected 2 KPIs")
	assert.Contains(t, names, "loan_overdue_rate_by_status")
	assert.Contains(t, names, "loan_balance_bucket_distribution")

	// Signature must be non-empty
	assert.NotEmpty(t, sig)

	// Verify signature = sha256 of file content
	raw, err := os.ReadFile(filepath.Join(dir, "loans.yaml"))
	require.NoError(t, err)
	h := sha256.Sum256(raw)
	expected := hex.EncodeToString(h[:])
	assert.Equal(t, expected, string(sig))
}

func TestLoadKPIs_BadFileRejectsEntireDirectory(t *testing.T) {
	dir := t.TempDir()
	// Write a valid file
	writeYAML(t, dir, "loans.yaml", validLoansYAML)
	// Write a bad file (empty dataset)
	writeYAML(t, dir, "bad.yaml", `dataset: ""
description: bad
k_min: 0
kpis:
  - name: k1
    description: test
    sql: "SELECT 1 AS row_count FROM ds_bad.data GROUP BY 1"
    count_column: row_count
`)

	_, _, err := LoadKPIs(context.Background(), dir, okSerializer("loans"), emptyDatasetLister())
	require.Error(t, err)
	// Error must mention the bad file path
	assert.Contains(t, err.Error(), "bad.yaml")
}

func TestLoadKPIs_CrossFileDuplicateName(t *testing.T) {
	dir := t.TempDir()

	// File 1: loans with loan_overdue_rate_by_status
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	// File 2: different dataset but same KPI name
	writeYAML(t, dir, "other.yaml", `dataset: other
description: other dataset
k_min: 0
kpis:
  - name: loan_overdue_rate_by_status
    description: duplicate name
    sql: "SELECT count(*) AS row_count FROM ds_other.data GROUP BY 1"
    count_column: row_count
`)

	_, _, err := LoadKPIs(context.Background(), dir, okSerializer("loans"), emptyDatasetLister())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-file duplicates")
	assert.Contains(t, err.Error(), "loan_overdue_rate_by_status")
}

func TestLoadKPIs_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	reg, sig, err := LoadKPIs(context.Background(), dir, okSerializer("loans"), emptyDatasetLister())
	require.NoError(t, err)
	require.NotNil(t, reg)
	assert.Equal(t, 0, len(reg.Names()))
	assert.Empty(t, sig)
}

func TestLoadKPIs_NonexistentDirectory(t *testing.T) {
	reg, sig, err := LoadKPIs(context.Background(), "/nonexistent/path", okSerializer("loans"), emptyDatasetLister())
	require.NoError(t, err)
	require.NotNil(t, reg)
	assert.Equal(t, 0, len(reg.Names()))
	assert.Empty(t, sig)
}

func TestLoadKPIs_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "real.yaml", validLoansYAML)

	// Create a symlink to the real file
	symlinkPath := filepath.Join(dir, "link.yaml")
	realPath := filepath.Join(dir, "real.yaml")
	if err := os.Symlink(realPath, symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	_, _, err := LoadKPIs(context.Background(), dir, okSerializer("loans"), emptyDatasetLister())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

func TestLoadKPIs_DirectorySignature(t *testing.T) {
	dir := t.TempDir()

	// Write two files
	writeYAML(t, dir, "a.yaml", `dataset: a
description: A
k_min: 0
kpis:
  - name: kpi_a
    description: KPI A
    sql: "SELECT x AS x, count(*) AS row_count FROM ds_a.data GROUP BY x"
    count_column: row_count
`)
	writeYAML(t, dir, "b.yaml", `dataset: b
description: B
k_min: 0
kpis:
  - name: kpi_b
    description: KPI B
    sql: "SELECT y AS y, count(*) AS row_count FROM ds_b.data GROUP BY y"
    count_column: row_count
`)

	// Create separate serializers for each dataset
	// We need a serializer that handles both datasets. Use a multi-dataset serializer.
	multiSer := &multiDatasetSerializer{datasets: map[string]bool{"a": true, "b": true}}
	_, sig, err := LoadKPIs(context.Background(), dir, multiSer, emptyDatasetLister())
	require.NoError(t, err)

	// Manually compute expected signature: files sorted by name, contents concatenated, sha256
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	h := sha256.New()
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".yaml") || f.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		require.NoError(t, err)
		h.Write(raw)
	}
	expected := hex.EncodeToString(h.Sum(nil))
	assert.Equal(t, expected, string(sig))
}

func TestLoadKPIs_SubdirectoriesNotScanned(t *testing.T) {
	dir := t.TempDir()
	// Write valid file in root
	writeYAML(t, dir, "loans.yaml", validLoansYAML)
	// Write file in subdirectory (should be ignored)
	writeYAML(t, dir, "subdir/extra.yaml", `dataset: extra
description: Extra
k_min: 0
kpis:
  - name: extra_kpi
    description: should not be loaded
    sql: "SELECT z AS z, count(*) AS row_count FROM ds_extra.data GROUP BY z"
    count_column: row_count
`)

	reg, _, err := loadKPIsTestHelper(t, dir, "loans")
	require.NoError(t, err)
	names := reg.Names()
	assert.Equal(t, 2, len(names), "subdirectory YAML should be ignored")
	for _, n := range names {
		assert.NotEqual(t, "extra_kpi", n)
	}
}

func TestLoadKPIs_NoYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	// Write a non-YAML file
	writeYAML(t, dir, "readme.txt", "this is not a YAML file")

	reg, sig, err := LoadKPIs(context.Background(), dir, okSerializer("loans"), emptyDatasetLister())
	require.NoError(t, err)
	require.NotNil(t, reg)
	assert.Equal(t, 0, len(reg.Names()))
	assert.Empty(t, sig)
}

func TestLoadKPIs_ParamRewrite(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := loadKPIsTestHelper(t, dir, "loans")
	require.NoError(t, err)

	kpi, ok := reg.Lookup("loan_overdue_rate_by_status")
	require.True(t, ok)

	// SQL should have $param instead of :param
	assert.Contains(t, kpi.SQLRaw, "$status")
	assert.NotContains(t, kpi.SQLRaw, ":status")
	// SQLDollar is same as SQLRaw
	assert.Equal(t, kpi.SQLRaw, kpi.SQLDollar)
	assert.Contains(t, kpi.SQLDollar, "$status")
}

func TestLoadKPIs_CompiledParamsEnum(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := loadKPIsTestHelper(t, dir, "loans")
	require.NoError(t, err)

	kpi, ok := reg.Lookup("loan_overdue_rate_by_status")
	require.True(t, ok)

	p, ok := kpi.Params["status"]
	require.True(t, ok)
	assert.Equal(t, ParamTypeEnum, p.Type)
	assert.Equal(t, []string{"正常", "逾期"}, p.EnumValues, "enum values should be sorted")
	assert.False(t, p.Required)
}

func TestLoadKPIs_CompiledParamsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := loadKPIsTestHelper(t, dir, "loans")
	require.NoError(t, err)

	kpi, ok := reg.Lookup("loan_balance_bucket_distribution")
	require.True(t, ok)
	assert.Equal(t, 0, len(kpi.Params), "empty params should result in empty compiled params")
}

func TestLoadKPIs_LookupNotFound(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := loadKPIsTestHelper(t, dir, "loans")
	require.NoError(t, err)

	_, ok := reg.Lookup("nonexistent_kpi")
	assert.False(t, ok)
}

// TestKPILoadFailClosed covers the 9 fail-closed branches for the full
// LoadKPIs pipeline (Layer 1 + Layer 2):
//  1. SQL syntax error (fake returns error:true envelope) → Layer 2 reject
//  2. Non-SELECT (INSERT envelope) → Layer 2 reject
//  3. Reference unregistered dataset (datasets list has loans but SQL ref ds_cards) → reject
//  4. Cross-dataset join (probe-4 fixture) → Layer 2 reject
//  5. Bootstrap window: empty datasets + valid file → pass
//  6. Non-empty datasets + declared dataset absent → reject
//  7. Valid full chain: LoadKPIs succeeds, SQLDollar has $status, TableRefCount==1, PreserveColumns has dimensions
//  8. GROUP BY expression → reject (v1 dialect fail-closed)
//  9. ser is nil → reject (fail-closed assembly convention)
func TestKPILoadFailClosed(t *testing.T) {
	t.Run("SQLSyntaxError_Layer2Reject", func(t *testing.T) {
		dir := t.TempDir()
		// SQL passes Layer 1 (has ds_loans.data + AS row_count + GROUP BY) but
		// the fake serializer returns error:true envelope so Layer 2 catches it.
		writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: bad_kpi
    description: bad sql
    sql: "SELECT status AS status, count(*) AS row_count FROM ds_loans.data GROUP BY status"
    count_column: row_count
    params: {}
`)
		ser := errorSerializer("parser", "syntax error at or near \"TOTALLY\"")
		_, _, err := LoadKPIs(context.Background(), dir, ser, emptyDatasetLister())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SQL parse error")
		assert.Contains(t, err.Error(), "parser")
	})

	t.Run("NonSelect_Layer2Reject", func(t *testing.T) {
		dir := t.TempDir()
		// Layer 1 text check catches INSERT before Layer 2 (no GROUP BY / no agg).
		// The pure-Layer-2 INSERT path is covered by probe-3 in validate_sql_test.go.
		// Here we verify the full pipeline rejects INSERT at Layer 1.
		writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: insert_kpi
    description: insert attempt
    sql: "INSERT INTO ds_loans.data VALUES (1)"
    count_column: row_count
    params: {}
`)
		ser := errorSerializer("not implemented", "Only SELECT statements can be serialized to json!")
		_, _, err := LoadKPIs(context.Background(), dir, ser, emptyDatasetLister())
		require.Error(t, err)
		// INSERT is caught at Layer 1 (the count_column text check fires first
		// because INSERT has no AS row_count; GROUP BY/agg check would also fire
		// but is downstream). Either branch is acceptable — Layer 2's probe-3
		// fixture path is the authoritative non-SELECT rejection (Layer 2 catches
		// at AST rule 3 / rule 1 envelope-error). Here we only assert that
		// rejection happens — the specific Layer 1 rule that fires is an
		// implementation detail of the text pre-check ordering.
		assert.Contains(t, err.Error(), "row_count")
	})

	t.Run("UnregisteredDataset_Reject", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, "cards.yaml", `dataset: cards
description: test
k_min: 0
kpis:
  - name: card_kpi
    description: card test
    sql: "SELECT status AS status, count(*) AS row_count FROM ds_cards.data GROUP BY status"
    count_column: row_count
    params: {}
`)
		ser := okSerializer("cards")
		// datasets list only contains "loans", not "cards"
		lister := &staticDatasetLister{datasets: []string{"loans"}}
		_, _, err := LoadKPIs(context.Background(), dir, ser, lister)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cards")
		assert.Contains(t, err.Error(), "not in the registered datasets")
	})

	t.Run("CrossDatasetJoin_Layer2Reject", func(t *testing.T) {
		dir := t.TempDir()
		// SQL references ds_loans.data (matches declared dataset) but
		// Layer 1 text pre-check will catch ds_cards in the SQL.
		// So we test with a SQL that has ds_loans.data in text but
		// the AST reveals a join to ds_cards.
		// However, Layer 1 rule 3 already catches ds_cards token.
		// This test validates the full pipeline behavior for cross-dataset join.
		writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: join_kpi
    description: cross join
    sql: "SELECT count(*) AS row_count FROM ds_loans.data JOIN ds_cards.data ON true GROUP BY 1"
    count_column: row_count
    params: {}
`)
		ser := okSerializer("loans")
		_, _, err := LoadKPIs(context.Background(), dir, ser, emptyDatasetLister())
		require.Error(t, err)
		// Caught by Layer 1 rule 3 (ds_cards token not matching ds_loans)
		assert.Contains(t, err.Error(), "ds_cards")
	})

	t.Run("BootstrapEmptyDatasets_Pass", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, "loans.yaml", validLoansYAML)

		reg, _, err := loadKPIsTestHelper(t, dir, "loans")
		require.NoError(t, err)
		require.NotNil(t, reg)
		assert.Equal(t, 2, len(reg.Names()))
	})

	t.Run("NonEmptyDatasets_DeclaredAbsentReject", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, "loans.yaml", validLoansYAML)

		// datasets has "other" but not "loans"
		lister := &staticDatasetLister{datasets: []string{"other"}}
		ser := okSerializer("loans")
		_, _, err := LoadKPIs(context.Background(), dir, ser, lister)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loans")
		assert.Contains(t, err.Error(), "not in the registered datasets")
	})

	t.Run("ValidFullChain", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, "loans.yaml", validLoansYAML)

		reg, _, err := loadKPIsTestHelper(t, dir, "loans")
		require.NoError(t, err)

		kpi, ok := reg.Lookup("loan_overdue_rate_by_status")
		require.True(t, ok)

		// SQLDollar contains $status and not :status
		assert.Contains(t, kpi.SQLDollar, "$status")
		assert.NotContains(t, kpi.SQLDollar, ":status")
		assert.Contains(t, kpi.SQLRaw, "$status")

		// TableRefCount == 1 (single BASE_TABLE ds_loans.data)
		assert.Equal(t, 1, kpi.TableRefCount)

		// PreserveColumns has dimension column "status" + CountColumn "row_count"
		assert.Contains(t, kpi.PreserveColumns, "status")
		assert.Contains(t, kpi.PreserveColumns, "row_count")

		// Signature unchanged by rewrite (signature is based on original file bytes)
		sig := reg.Signature()
		assert.NotEmpty(t, sig)
	})

	t.Run("GroupByExpression_Reject", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, "loans.yaml", `dataset: loans
description: test
k_min: 0
kpis:
  - name: expr_kpi
    description: expression in GROUP BY
    sql: "SELECT substr(status,1,2) AS status_prefix, count(*) AS row_count FROM ds_loans.data GROUP BY substr(status,1,2)"
    count_column: row_count
    params: {}
`)
		ser := okSerializer("loans")
		_, _, err := LoadKPIs(context.Background(), dir, ser, emptyDatasetLister())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a simple identifier")
		assert.Contains(t, err.Error(), "v1 KPI dialect")
	})

	t.Run("NilSerializer_Reject", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, "loans.yaml", validLoansYAML)

		_, _, err := LoadKPIs(context.Background(), dir, nil, emptyDatasetLister())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SQLSerializer is nil")
	})
}

// multiDatasetSerializer returns a valid envelope for any declared dataset.
// Used for tests with multiple dataset files.
type multiDatasetSerializer struct {
	datasets map[string]bool
}

func (m *multiDatasetSerializer) SerializeSQL(_ context.Context, statement string) (string, error) {
	// Determine which dataset from the statement
	for ds := range m.datasets {
		if strings.Contains(statement, "ds_"+ds) {
			env := map[string]any{
				"error": false,
				"statements": []map[string]any{
					{
						"node": map[string]any{
							"type": "SELECT_NODE",
							"from_table": map[string]any{
								"type":        "BASE_TABLE",
								"schema_name": "ds_" + ds,
								"table_name":  "data",
							},
							"select_list": []any{},
						},
					},
				},
			}
			raw, _ := json.Marshal(env)
			return string(raw), nil
		}
	}
	// Default: return error
	return `{"error":true,"error_type":"parser","error_message":"no matching dataset"}`, nil
}
