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
	"crypto/sha256"
	"encoding/hex"
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

func TestLoadKPIs_ValidDirectory(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, sig, err := LoadKPIs(dir)
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

	_, _, err := LoadKPIs(dir)
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

	_, _, err := LoadKPIs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-file duplicates")
	assert.Contains(t, err.Error(), "loan_overdue_rate_by_status")
}

func TestLoadKPIs_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	reg, sig, err := LoadKPIs(dir)
	require.NoError(t, err)
	require.NotNil(t, reg)
	assert.Equal(t, 0, len(reg.Names()))
	assert.Empty(t, sig)
}

func TestLoadKPIs_NonexistentDirectory(t *testing.T) {
	reg, sig, err := LoadKPIs("/nonexistent/path")
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

	_, _, err := LoadKPIs(dir)
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
    sql: "SELECT count(*) AS row_count FROM ds_a.data GROUP BY 1"
    count_column: row_count
`)
	writeYAML(t, dir, "b.yaml", `dataset: b
description: B
k_min: 0
kpis:
  - name: kpi_b
    description: KPI B
    sql: "SELECT count(*) AS row_count FROM ds_b.data GROUP BY 1"
    count_column: row_count
`)

	_, sig, err := LoadKPIs(dir)
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
	// Write file in subdirectory (should be ignored)
	writeYAML(t, dir, "subdir/extra.yaml", `dataset: extra
description: Extra
k_min: 0
kpis:
  - name: extra_kpi
    description: should not be loaded
    sql: "SELECT count(*) AS row_count FROM ds_extra.data GROUP BY 1"
    count_column: row_count
`)

	reg, _, err := LoadKPIs(dir)
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

	reg, sig, err := LoadKPIs(dir)
	require.NoError(t, err)
	require.NotNil(t, reg)
	assert.Equal(t, 0, len(reg.Names()))
	assert.Empty(t, sig)
}

func TestLoadKPIs_ParamRewrite(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := LoadKPIs(dir)
	require.NoError(t, err)

	kpi, ok := reg.Lookup("loan_overdue_rate_by_status")
	require.True(t, ok)

	// SQL should have $param instead of :param
	assert.Contains(t, kpi.SQLRaw, "$status")
	assert.NotContains(t, kpi.SQLRaw, ":status")
}

func TestLoadKPIs_CompiledParamsEnum(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := LoadKPIs(dir)
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

	reg, _, err := LoadKPIs(dir)
	require.NoError(t, err)

	kpi, ok := reg.Lookup("loan_balance_bucket_distribution")
	require.True(t, ok)
	assert.Equal(t, 0, len(kpi.Params), "empty params should result in empty compiled params")
}

func TestLoadKPIs_LookupNotFound(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "loans.yaml", validLoansYAML)

	reg, _, err := LoadKPIs(dir)
	require.NoError(t, err)

	_, ok := reg.Lookup("nonexistent_kpi")
	assert.False(t, ok)
}
