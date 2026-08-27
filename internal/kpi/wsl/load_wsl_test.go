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

// WSL-gated test for the full Layer 2 LoadKPIs pipeline (16-05 Task 2).
// This test exercises the production DuckDBSQLSerializer (CGO crossing
// point) end-to-end against the real kpis/loans.yaml corpus, plus
// fail-closed assertions for syntax-error / INSERT SQL. It is the
// authoritative WSL counterpart of the hermetic TestKPILoadFailClosed
// suite in internal/kpi/loader_test.go.
//
// Run via:
//   wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent \
//     && go test ./internal/kpi/wsl/ -run TestKPILoadFailClosedLayer2WSL -count=1 -v -timeout 180s'
package wsl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyDatasetLister is the bootstrap-window stub (Pitfall 2: empty
// registry → existence check skipped). The WSL test does NOT need real
// datasets because the SQL never actually executes — only the AST walk
// runs against json_serialize_sql.
type emptyDatasetLister struct{}

func (emptyDatasetLister) ListDatasets(_ context.Context) ([]string, error) {
	return nil, nil
}

// writeYAML is a minimal test helper for ad-hoc KPI YAML files in the
// WSL test directory (the kpi package's writeYAML is unexported).
func writeYAML(t *testing.T, dir, filename, content string) string {
	t.Helper()
	full := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}

// TestKPILoadFailClosedLayer2WSL runs the full two-layer LoadKPIs pipeline
// through the production DuckDBSQLSerializer against a real :memory:
// DuckDB engine. Three branches are locked:
//
//  1. Valid corpus (kpis/loans.yaml from the repo): LoadKPIs succeeds and
//     returns 2 compiled KPIs with $param form and TableRefCount populated.
//  2. Syntax-error SQL: LoadKPIs is rejected at Layer 2 (json_serialize_sql
//     envelope error:true → Rule 1 fail-closed).
//  3. INSERT SQL: LoadKPIs is rejected. Layer 1 may catch it first (no
//     GROUP BY / no aggregate function heuristic), but we only assert that
//     rejection happens regardless of which specific rule fires.
func TestKPILoadFailClosedLayer2WSL(t *testing.T) {
	ctx := context.Background()

	// Set up real DuckDB Connector (CGO).
	conn, err := duckdb.New(ctx, duckdb.Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()

	// Production Layer 2 adapter.
	ser := duckdb.NewDuckDBSQLSerializer(conn)
	lister := emptyDatasetLister{}

	t.Run("ValidRealCorpus", func(t *testing.T) {
		// Locate the repo's kpis/loans.yaml. From internal/kpi/wsl/ we go up
		// three levels to reach the repo root.
		repoRoot := "../../../"
		kpisDir := filepath.Join(repoRoot, "kpis")
		if _, err := os.Stat(kpisDir); os.IsNotExist(err) {
			t.Skip("kpis/ directory not found (running from unexpected working directory)")
		}

		reg, sig, err := kpi.LoadKPIs(ctx, kpisDir, ser, lister)
		require.NoError(t, err, "LoadKPIs must succeed on valid corpus with real DuckDB serializer")
		require.NotNil(t, reg)

		names := reg.Names()
		assert.Equal(t, 2, len(names), "kpis/loans.yaml should compile 2 KPIs")
		assert.Contains(t, names, "loan_overdue_rate_by_status")
		assert.Contains(t, names, "loan_balance_bucket_distribution")
		assert.NotEmpty(t, sig, "registry signature must be non-empty")

		// Layer 2 compilation artifacts (16-05 acceptance: $param form +
		// TableRefCount).
		overdue, ok := reg.Lookup("loan_overdue_rate_by_status")
		require.True(t, ok)
		assert.Contains(t, overdue.SQLDollar, "$status",
			"WSL: real DuckDB pipeline must produce $param form")
		assert.NotContains(t, overdue.SQLDollar, ":status",
			"WSL: real DuckDB pipeline must NOT retain :param form")
		assert.Equal(t, 1, overdue.TableRefCount,
			"WSL: single-table SELECT should report TableRefCount=1")
		assert.Contains(t, overdue.PreserveColumns, "status",
			"WSL: PreserveColumns must include GROUP BY dimension")
		assert.Contains(t, overdue.PreserveColumns, "row_count",
			"WSL: PreserveColumns must include CountColumn")
	})

	t.Run("SyntaxErrorSQL_Layer2Reject", func(t *testing.T) {
		// SQL passes Layer 1 (has ds_loans.data + AS row_count + GROUP BY) so
		// Layer 2 (json_serialize_sql) is the layer that catches the syntax
		// error.
		dir := t.TempDir()
		writeYAML(t, dir, "bad_syntax.yaml", `dataset: loans
description: syntax error
k_min: 0
kpis:
  - name: bad_kpi
    description: deliberately invalid SQL
    sql: "SELECT status AS status, count(*) AS row_count FROM ds_loans.data GROUP BY status ORDER BY TOTALLY_NOT_A_COLUMN"
    count_column: row_count
    params: {}
`)

		_, _, err := kpi.LoadKPIs(ctx, dir, ser, lister)
		require.Error(t, err, "LoadKPIs must reject SQL that fails DuckDB parser")
		// The exact error depends on which rule fires first (json_serialize_sql
		// envelope error:true → Rule 1; or AST shape anomaly → Rule 3). Both
		// are valid Layer-2 rejection paths. We assert rejection, not which
		// specific rule.
		assert.True(t,
			errContains(err, "SQL parse error") ||
				errContains(err, "ValidateSQLAST") ||
				errContains(err, "validate"),
			"WSL: error must trace to Layer 2 AST validation, got: %v", err)
	})

	t.Run("InsertSQL_Rejected", func(t *testing.T) {
		// INSERT is rejected either at Layer 1 (no GROUP BY / no agg func)
		// or at Layer 2 (envelope error:true for non-SELECT). The pipeline
		// is fail-closed at both layers; we assert rejection happens.
		dir := t.TempDir()
		writeYAML(t, dir, "insert.yaml", `dataset: loans
description: insert attempt
k_min: 0
kpis:
  - name: insert_kpi
    description: not a SELECT
    sql: "INSERT INTO ds_loans.data VALUES (1)"
    count_column: row_count
    params: {}
`)

		_, _, err := kpi.LoadKPIs(ctx, dir, ser, lister)
		require.Error(t, err, "LoadKPIs must reject non-SELECT statements")
	})
}

// errContains is a small convenience for the cross-layer error checks
// (avoids pulling in strings.Contains at every callsite).
func errContains(err error, substr string) bool {
	if err == nil {
		return false
	}
	for _, c := range []string{err.Error()} {
		if contains(c, substr) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
