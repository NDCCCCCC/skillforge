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
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteStagingBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("roundTrip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "staging.duckdb")
		table := StagingTable{
			Name: "data",
			Columns: []ColumnSpec{
				{Name: "name", DuckDBType: "VARCHAR"},
				{Name: "n", DuckDBType: "BIGINT"},
				{Name: "score", DuckDBType: "DOUBLE"},
			},
		}
		rows := make([][]any, 1000)
		for i := range 1000 {
			rows[i] = []any{fmt.Sprintf("row-%d", i), int64(i), float64(i) * 0.5}
		}

		n, err := WriteStagingBatch(ctx, path, table, slices.Values(rows))
		require.NoError(t, err)
		assert.Equal(t, int64(1000), n)

		// Reopen the file with a fresh connector to prove persistence.
		c, err := New(ctx, Config{DSN: path, MaxOpenConns: 1})
		require.NoError(t, err)
		defer func() { require.NoError(t, c.Close()) }()

		var count int64
		require.NoError(t, c.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM data").Scan(&count))
		assert.Equal(t, int64(1000), count)

		var name string
		var num int64
		var score float64
		require.NoError(t, c.DB().QueryRowContext(ctx, "SELECT name, n, score FROM data WHERE n = 42").Scan(&name, &num, &score))
		assert.Equal(t, "row-42", name)
		assert.Equal(t, int64(42), num)
		assert.InDelta(t, 21.0, score, 0.0001)
	})

	t.Run("typeAllowlist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "staging.duckdb")
		table := StagingTable{
			Name: "data",
			Columns: []ColumnSpec{
				{Name: "flag", DuckDBType: "BIT"},
			},
		}
		_, err := WriteStagingBatch(ctx, path, table, slices.Values([][]any{}))
		require.Error(t, err, "BIT must be rejected before Appender creation")
	})

	t.Run("tableCreate", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "staging.duckdb")
		table := StagingTable{
			Name: "data",
			Columns: []ColumnSpec{
				{Name: "id", DuckDBType: "BIGINT"},
			},
		}
		_, err := WriteStagingBatch(ctx, path, table, slices.Values([][]any{{int64(1)}}))
		require.NoError(t, err)

		// A second write to the same staging file must reject because the
		// table already exists — import batches always target fresh staging files.
		_, err = WriteStagingBatch(ctx, path, table, slices.Values([][]any{{int64(2)}}))
		require.Error(t, err, "existing table must be rejected")
	})
}

// TestCompositeNameAttach is the A1 assumption probe: a composite dataset id
// with a hyphen ("loans__2026-07") can be written via WriteStagingBatch,
// committed via CommitStagedDataset, lazily ATTACHed via AttachLRU, and queried.
func TestCompositeNameAttach(t *testing.T) {
	ctx := context.Background()
	datasetsDir := t.TempDir()
	stagingDir := t.TempDir()
	const datasetID = "loans__2026-07"

	stagingPath := filepath.Join(stagingDir, "staging.duckdb")
	table := StagingTable{
		Name: "data",
		Columns: []ColumnSpec{
			{Name: "id", DuckDBType: "BIGINT"},
			{Name: "label", DuckDBType: "VARCHAR"},
		},
	}
	_, err := WriteStagingBatch(ctx, stagingPath, table, slices.Values([][]any{
		{int64(1), "a"},
		{int64(2), "b"},
	}))
	require.NoError(t, err)

	reg := newFakeRegistry()
	require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
		StagingPath: stagingPath,
		DatasetID:   datasetID,
		Row: store.DatasetRow{
			Name:          datasetID,
			Tier:          "历史",
			SchemaJSON:    `{"cols":["id","label"]}`,
			FilePath:      filepath.Join(datasetsDir, datasetID+".duckdb"),
			ContentSHA256: "sha",
			BatchID:       "batch-1",
			CreatedAt:     time.Now().UTC(),
		},
		Registry:    reg,
		DatasetsDir: datasetsDir,
	}))

	c, err := New(ctx, Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	lru, err := NewAttachLRU(c, 4)
	require.NoError(t, err)
	defer func() { require.NoError(t, lru.Close()) }()

	_, err = lru.Get(ctx, datasetID, filepath.Join(datasetsDir, datasetID+".duckdb"))
	require.NoError(t, err)

	queryer, ok := lru.conn.(driver.QueryerContext)
	require.True(t, ok)
	// The alias contains a hyphen, so it must be double-quoted in SQL.
	rows, err := queryer.QueryContext(ctx, `SELECT id FROM "ds_loans__2026-07".main.data ORDER BY id`, nil)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var ids []int64
	dest := make([]driver.Value, 1)
	for {
		if err := rows.Next(dest); err != nil {
			break
		}
		ids = append(ids, dest[0].(int64))
	}
	assert.Equal(t, []int64{1, 2}, ids)
}
