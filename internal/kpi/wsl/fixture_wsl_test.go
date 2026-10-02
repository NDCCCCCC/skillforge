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

package wsl

// Shared WSL fixture infrastructure for engine integration + snapshot tests.
//
// The fixtures stand up a real sqlite registry, a real staging→commit
// pipeline for a tiny loans dataset, a real DuckDB LRU + EngineAdapter,
// and a real DuckDBSQLSerializer for LoadKPIs' Layer 2 AST walk. The
// only stubs are the audit sink (captureAudit) and the now clock — those
// legs are covered hermetically by internal/kpi/executor_test.go.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// engineFixture owns the per-test WSL runtime: staging + datasets
// directories, a real sqlite registry, and a real DuckDB AttachLRU over
// a fresh Connector. Cleanup is wired into t.Cleanup.
type engineFixture struct {
	root        string
	stagingDir  string
	datasetsDir string
	store       *sqlite.SQLiteStore
	connector   *duckdb.Connector
	lru         *duckdb.AttachLRU
}

// newEngineFixture creates a fully wired WSL runtime. maxAttach is the
// LRU cap (small by default — one dataset is enough for v1 KPI tests).
func newEngineFixture(t *testing.T, maxAttach int) *engineFixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	ef := &engineFixture{
		root:        root,
		stagingDir:  filepath.Join(root, "staging"),
		datasetsDir: filepath.Join(root, "datasets"),
	}
	require.NoError(t, os.MkdirAll(ef.stagingDir, 0o755))
	require.NoError(t, os.MkdirAll(ef.datasetsDir, 0o755))

	s, err := sqlite.New(ctx, sqlite.Config{DSN: filepath.Join(root, "registry.db")})
	require.NoError(t, err, "open sqlite registry")
	t.Cleanup(func() { _ = s.Close() })
	ef.store = s

	c, err := duckdb.New(ctx, duckdb.Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err, "open duckdb connector")
	t.Cleanup(func() { _ = c.Close() })
	ef.connector = c

	lru, err := duckdb.NewAttachLRU(c, maxAttach)
	require.NoError(t, err, "open AttachLRU")
	t.Cleanup(func() { _ = lru.Close() })
	ef.lru = lru

	return ef
}

// writeLoansStaging materializes a tiny persistent .duckdb file with the
// loans-style schema (status VARCHAR, balance DOUBLE, overdue_days BIGINT)
// under the staging dir. Rows are written via INSERT statements so the
// fixture is self-contained (no appender / no WriteStagingBatch dep).
//
// Returns the staging path.
func (ef *engineFixture) writeLoansStaging(t *testing.T, datasetID string, rows []loanRow) string {
	t.Helper()
	ctx := context.Background()
	stagingPath := filepath.Join(ef.stagingDir, datasetID+".duckdb")
	// Dedicated connector so the staging file is its OWN catalog (not
	// the LRU's attached-catalog session).
	c, err := duckdb.New(ctx, duckdb.Config{DSN: stagingPath, MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	_, err = c.DB().ExecContext(ctx, `
		CREATE TABLE data (
			status VARCHAR,
			balance DOUBLE,
			overdue_days BIGINT
		)
	`)
	require.NoError(t, err, "create data table in staging")

	if len(rows) > 0 {
		// Sort for deterministic order (sort doesn't affect results, but
		// stable input helps debug-print readability).
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Status != rows[j].Status {
				return rows[i].Status < rows[j].Status
			}
			return rows[i].Balance < rows[j].Balance
		})
		for _, r := range rows {
			_, err = c.DB().ExecContext(ctx,
				"INSERT INTO data (status, balance, overdue_days) VALUES (?, ?, ?)",
				r.Status, r.Balance, r.OverdueDays)
			require.NoError(t, err, "insert row")
		}
	}
	// Force a checkpoint so the WAL is flushed before commit.
	_, err = c.DB().ExecContext(ctx, "CHECKPOINT")
	require.NoError(t, err)

	return stagingPath
}

// loanRow is one row of the loans fixture.
type loanRow struct {
	Status      string
	Balance     float64
	OverdueDays int64
}

// commitLoansDataset promotes a staged .duckdb file into the datasets
// directory and writes the corresponding sqlite row. The composite name
// follows the dataset__period convention (D14-D2 + D15-C2).
func (ef *engineFixture) commitLoansDataset(
	t *testing.T,
	logicalDataset, period string,
	stagingPath string,
	kMinOverride int,
) store.DatasetRow {
	t.Helper()
	ctx := context.Background()
	composite := logicalDataset + "__" + period
	require.True(t,
		duckdb.ValidateDatasetID(composite) == nil,
		"composite dataset id must satisfy ValidateDatasetID",
	)

	target := filepath.Join(ef.datasetsDir, composite+".duckdb")
	row := store.DatasetRow{
		Name:          composite,
		Tier:          "权威",
		SchemaJSON:    `{"cols":["status","balance","overdue_days"]}`,
		FilePath:      target,
		ContentSHA256: sha256Hex(filepath.Base(stagingPath)),
		BatchID:       "batch-wsl-" + period,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		Dataset:       logicalDataset,
		Period:        period,
		KMinOverride:  kMinOverride,
	}
	require.NoError(t, duckdb.CommitStagedDataset(ctx, duckdb.CommitOptions{
		StagingPath:  stagingPath,
		DatasetID:    composite,
		Row:          row,
		Registry:     ef.store,
		DatasetsDir:  ef.datasetsDir,
	}), "commit staged dataset")
	require.FileExists(t, target, "committed file must exist on disk")
	return row
}

// sha256Hex is a tiny helper for the SHA-256 content digest placeholder.
// The fixture value is for documentation only — the registry schema does
// not require a real digest at row creation time.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// writeLoansKPIYAML writes the loans-shaped KPI fixture file into dir.
// status is declared optional so nil-binding can be exercised; the WHERE
// clause uses the (:status IS NULL OR status = :status) pattern.
//
// The SQL uses the CANONICAL user form (colon placeholders, :status).
// The validator rewrites :status to $status for DuckDB before Layer 2
// AST walk (see internal/kpi/validate_text.go paramPlaceholderPattern +
// loader.go :param -> $param rewrite). The fixture must mirror the
// canon — the production kpis/loans.yaml uses the colon form.
func writeLoansKPIYAML(t *testing.T, dir string, requiredStatus bool) {
	t.Helper()
	required := "false"
	if requiredStatus {
		required = "true"
	}
	yaml := `
dataset: loans
description: WSL integration fixture（16-06）
kpis:
  - name: loan_overdue_rate_by_status
    description: 各状态贷款逾期率
    sql: |
      SELECT status AS status,
             count(*) AS row_count,
             round(100.0 * sum(CASE WHEN overdue_days > 0 THEN 1 ELSE 0 END) / count(*), 2) AS overdue_rate
      FROM ds_loans.data
      WHERE (:status IS NULL OR status = :status)
      GROUP BY status
    count_column: row_count
    max_rows: 1000
    params:
      status:
        type: enum
        values: [正常, 逾期, 已结清]
        required: ` + required + `
`
	path := filepath.Join(dir, "loans.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))
}

// loadWSLRegistry runs the full two-layer LoadKPIs pipeline against the
// WSL fixture dir using a real DuckDBSQLSerializer (Layer 2 AST walk).
// It uses ef.connector directly — the serializer only calls
// json_serialize_sql, which is independent of any ATTACH state, so
// sharing the LRU's connector is safe.
func loadWSLRegistry(t *testing.T, ef *engineFixture, kpiDir string) *kpi.Registry {
	t.Helper()
	serializer := duckdb.NewDuckDBSQLSerializer(ef.connector)
	reg, _, err := kpi.LoadKPIs(context.Background(), kpiDir, serializer, emptyLister{})
	require.NoError(t, err, "LoadKPIs wsl")
	require.NotNil(t, reg)
	return reg
}

// emptyLister models the bootstrap window (datasets empty → existence
// check skipped). Mirrors the hermetic fixture in executor_test.go so
// the two test layers look parallel.
type emptyLister struct{}

func (emptyLister) ListDatasets(context.Context) ([]string, error) {
	return nil, nil
}

// wsMeta adapts a sqlite store to kpi.DatasetMetaSource: returns the
// LATEST registered period for the given logical dataset (period string
// lexicographic ordering matches chronological for YYYY-MM).
type wsMeta struct {
	store *sqlite.SQLiteStore
}

func (m wsMeta) LatestDatasetVersion(_ context.Context, dataset string) (kpi.DatasetVersion, error) {
	rows, err := m.store.ListDatasets(context.Background())
	if err != nil {
		return kpi.DatasetVersion{}, err
	}
	var best *store.DatasetRow
	for i := range rows {
		r := rows[i]
		if r.Dataset != dataset {
			continue
		}
		if best == nil || r.Period > best.Period {
			r2 := r
			best = &r2
		}
	}
	if best == nil {
		return kpi.DatasetVersion{}, fmt.Errorf("wsMeta: dataset %q not found in registry", dataset)
	}
	return kpi.DatasetVersion{
		Period:       best.Period,
		FilePath:     best.FilePath,
		KMinOverride: best.KMinOverride,
	}, nil
}

func (m wsMeta) Periods(_ context.Context, dataset string) ([]string, error) {
	rows, err := m.store.ListDatasets(context.Background())
	if err != nil {
		return nil, err
	}
	var periods []string
	seen := make(map[string]bool)
	for _, r := range rows {
		if r.Dataset != dataset {
			continue
		}
		if !seen[r.Period] {
			periods = append(periods, r.Period)
			seen[r.Period] = true
		}
	}
	sort.Strings(periods)
	return periods, nil
}

// VersionForPeriod resolves the composite row for ONE exact period — the
// CallMultiPeriod per-period ATTACH seam (Phase 17 live-UAT fix). The WSL
// fixture registers one row per (dataset, period), so exact-match is the
// same bounded ListDatasets scan.
func (m wsMeta) VersionForPeriod(_ context.Context, dataset, period string) (kpi.DatasetVersion, error) {
	rows, err := m.store.ListDatasets(context.Background())
	if err != nil {
		return kpi.DatasetVersion{}, err
	}
	for i := range rows {
		r := rows[i]
		if r.Dataset == dataset && r.Period == period {
			return kpi.DatasetVersion{
				Period:       r.Period,
				FilePath:     r.FilePath,
				KMinOverride: r.KMinOverride,
			}, nil
		}
	}
	return kpi.DatasetVersion{}, nil
}
