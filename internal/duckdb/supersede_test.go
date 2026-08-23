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

package duckdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcboeker/go-duckdb/v2"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRegistry is a test-double for the DatasetRegistry seam. It supports
// injection of a PutDataset failure so the supersede compensation path can be
// exercised (Pitfall 3 dual-injection requirement).
type fakeRegistry struct {
	rows map[string]store.DatasetRow
	fail error
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{rows: make(map[string]store.DatasetRow)}
}

func (f *fakeRegistry) PutDataset(ctx context.Context, row store.DatasetRow) error {
	if f.fail != nil {
		return f.fail
	}
	f.rows[row.Name] = row
	return nil
}

// writeDataFile creates a small DuckDB file at path with a single "data" table
// holding (id BIGINT, label VARCHAR) rows. It mirrors the Appender call shape
// used by WriteStagingBatch and the appender throughput baseline.
func writeDataFile(t *testing.T, ctx context.Context, path string, ids []int64) {
	t.Helper()
	c, err := New(ctx, Config{DSN: path, MaxOpenConns: 1})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	_, err = c.DB().ExecContext(ctx, "CREATE TABLE data (id BIGINT, label VARCHAR)")
	require.NoError(t, err)

	driverConn, err := c.connector.Connect(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, driverConn.Close()) }()

	app, err := duckdb.NewAppenderFromConn(driverConn, "main", "data")
	require.NoError(t, err)
	defer func() { require.NoError(t, app.Close()) }()

	for i, id := range ids {
		require.NoError(t, app.AppendRow(id, fmt.Sprintf("row-%d", i)))
	}
	require.NoError(t, app.Flush())
}

// queryDataIDs returns the ordered id column from the "data" table at path.
func queryDataIDs(t *testing.T, ctx context.Context, path string) []int64 {
	t.Helper()
	c, err := New(ctx, Config{DSN: path, MaxOpenConns: 1})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	rows, err := c.DB().QueryContext(ctx, "SELECT id FROM data ORDER BY id")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []int64
	for rows.Next() {
		var v int64
		require.NoError(t, rows.Scan(&v))
		out = append(out, v)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestSupersedeAtomicSwap(t *testing.T) {
	ctx := context.Background()

	t.Run("happy", func(t *testing.T) {
		datasetsDir := t.TempDir()
		backupDir := t.TempDir()
		stagingDir := t.TempDir()
		const datasetID = "loans__2026-07"

		oldStaging := filepath.Join(stagingDir, "old.duckdb")
		writeDataFile(t, ctx, oldStaging, []int64{1, 2, 3})

		reg := newFakeRegistry()
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: oldStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "old-sha",
				BatchID:       "batch-old",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			LRU:         nil,
		}))

		newStaging := filepath.Join(stagingDir, "new.duckdb")
		writeDataFile(t, ctx, newStaging, []int64{4, 5, 6})

		require.NoError(t, SupersedeStagedDataset(ctx, SupersedeOptions{
			StagingPath: newStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "new-sha",
				BatchID:       "batch-new",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			BackupDir:   backupDir,
			LRU:         nil,
		}))

		got := queryDataIDs(t, ctx, filepath.Join(datasetsDir, datasetID+".duckdb"))
		assert.Equal(t, []int64{4, 5, 6}, got, "target file must contain the new data after supersede")

		backups, err := os.ReadDir(backupDir)
		require.NoError(t, err)
		assert.Empty(t, backups, "backup file must be removed after successful supersede")

		require.Contains(t, reg.rows, datasetID)
		assert.Equal(t, "batch-new", reg.rows[datasetID].BatchID, "registry must reflect the new row")
	})

	t.Run("registryWriteFails", func(t *testing.T) {
		datasetsDir := t.TempDir()
		backupDir := t.TempDir()
		stagingDir := t.TempDir()
		const datasetID = "loans__2026-07"

		oldStaging := filepath.Join(stagingDir, "old.duckdb")
		writeDataFile(t, ctx, oldStaging, []int64{1, 2, 3})

		reg := newFakeRegistry()
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: oldStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "old-sha",
				BatchID:       "batch-old",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			LRU:         nil,
		}))

		newStaging := filepath.Join(stagingDir, "new.duckdb")
		writeDataFile(t, ctx, newStaging, []int64{7, 8, 9})

		failReg := newFakeRegistry()
		failReg.rows[datasetID] = reg.rows[datasetID] // seed with old row
		failReg.fail = errors.New("injected PutDataset failure")

		err := SupersedeStagedDataset(ctx, SupersedeOptions{
			StagingPath: newStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "fail-sha",
				BatchID:       "batch-fail",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    failReg,
			DatasetsDir: datasetsDir,
			BackupDir:   backupDir,
			LRU:         nil,
		})
		require.Error(t, err, "injected registry failure must surface")

		got := queryDataIDs(t, ctx, filepath.Join(datasetsDir, datasetID+".duckdb"))
		assert.Equal(t, []int64{1, 2, 3}, got, "old content must be restored when registry write fails")

		require.Contains(t, failReg.rows, datasetID)
		assert.Equal(t, "batch-old", failReg.rows[datasetID].BatchID, "old registry row must remain unchanged")

		// stagingNew is preserved as residue so the caller can retry; the plan
		// permits either preservation or cleanup as long as old data is intact.
		assert.FileExists(t, newStaging, "staging file preserved for retry after registry failure")
	})

	t.Run("step1RenameFails", func(t *testing.T) {
		datasetsDir := t.TempDir()
		backupDir := t.TempDir()
		stagingDir := t.TempDir()
		const datasetID = "loans__2026-07"

		oldStaging := filepath.Join(stagingDir, "old.duckdb")
		writeDataFile(t, ctx, oldStaging, []int64{1, 2, 3})

		reg := newFakeRegistry()
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: oldStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "old-sha",
				BatchID:       "batch-old",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			LRU:         nil,
		}))

		newStaging := filepath.Join(stagingDir, "new.duckdb")
		writeDataFile(t, ctx, newStaging, []int64{10, 11, 12})

		// Pre-create the backup path as a directory so os.Rename(target, backup)
		// cannot succeed.
		backupPath := filepath.Join(backupDir, fmt.Sprintf(".superseded-%s-%d.duckdb", datasetID, time.Now().UnixNano()))
		require.NoError(t, os.MkdirAll(backupPath, 0o755))

		err := SupersedeStagedDataset(ctx, SupersedeOptions{
			StagingPath: newStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "step1-sha",
				BatchID:       "batch-step1",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			BackupDir:   backupDir,
			LRU:         nil,
		})
		require.Error(t, err, "step1 rename failure must surface")

		got := queryDataIDs(t, ctx, filepath.Join(datasetsDir, datasetID+".duckdb"))
		assert.Equal(t, []int64{1, 2, 3}, got, "old content must remain intact when backup rename fails")
	})

	t.Run("targetMissing", func(t *testing.T) {
		datasetsDir := t.TempDir()
		backupDir := t.TempDir()
		stagingDir := t.TempDir()
		const datasetID = "loans__2026-07"

		newStaging := filepath.Join(stagingDir, "new.duckdb")
		writeDataFile(t, ctx, newStaging, []int64{1, 2, 3})

		err := SupersedeStagedDataset(ctx, SupersedeOptions{
			StagingPath: newStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "new-sha",
				BatchID:       "batch-new",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    newFakeRegistry(),
			DatasetsDir: datasetsDir,
			BackupDir:   backupDir,
			LRU:         nil,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSupersedeTargetMissing, "missing target must return ErrSupersedeTargetMissing")
	})

	t.Run("detachFirst", func(t *testing.T) {
		datasetsDir := t.TempDir()
		backupDir := t.TempDir()
		stagingDir := t.TempDir()
		const datasetID = "loans__2026-08"

		oldStaging := filepath.Join(stagingDir, "old.duckdb")
		writeDataFile(t, ctx, oldStaging, []int64{1, 2, 3})

		reg := newFakeRegistry()
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: oldStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "old-sha",
				BatchID:       "batch-old",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			LRU:         nil,
		}))

		// Open a dedicated connector and lazily attach the dataset. On Windows
		// the file is locked until Detach runs; SupersedeStagedDataset must
		// Detach before rename (Pitfall 9).
		c, err := New(ctx, Config{DSN: ":memory:", MaxOpenConns: 1})
		require.NoError(t, err)
		defer func() { require.NoError(t, c.Close()) }()

		lru, err := NewAttachLRU(c, 4)
		require.NoError(t, err)
		defer func() { require.NoError(t, lru.Close()) }()

		_, err = lru.Get(ctx, datasetID, filepath.Join(datasetsDir, datasetID+".duckdb"))
		require.NoError(t, err)
		attached, err := lru.AttachedIDs(ctx)
		require.NoError(t, err)
		require.Contains(t, attached, datasetID, "dataset must be attached before supersede")

		newStaging := filepath.Join(stagingDir, "new.duckdb")
		writeDataFile(t, ctx, newStaging, []int64{4, 5, 6})

		require.NoError(t, SupersedeStagedDataset(ctx, SupersedeOptions{
			StagingPath: newStaging,
			DatasetID:   datasetID,
			Row: store.DatasetRow{
				Name:          datasetID,
				Tier:          "历史",
				SchemaJSON:    `{"cols":["id","label"]}`,
				FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
				ContentSHA256: "new-sha",
				BatchID:       "batch-new",
				CreatedAt:     time.Now().UTC(),
			},
			Registry:    reg,
			DatasetsDir: datasetsDir,
			BackupDir:   backupDir,
			LRU:         lru,
		}))

		got := queryDataIDs(t, ctx, filepath.Join(datasetsDir, datasetID+".duckdb"))
		assert.Equal(t, []int64{4, 5, 6}, got, "supersede must succeed after detaching the attached dataset")

		attached, err = lru.AttachedIDs(ctx)
		require.NoError(t, err)
		assert.NotContains(t, attached, datasetID, "dataset must be detached after supersede")
	})
}
