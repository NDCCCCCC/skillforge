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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// stagingEnv is the per-subtest fixture: a real sqlite registry plus
// staging/datasets directories under a temp root.
type stagingEnv struct {
	store       *sqlite.SQLiteStore
	stagingDir  string
	datasetsDir string
}

func newStagingEnv(t *testing.T) *stagingEnv {
	t.Helper()
	root := t.TempDir()
	env := &stagingEnv{
		stagingDir:  filepath.Join(root, "staging"),
		datasetsDir: filepath.Join(root, "datasets"),
	}
	require.NoError(t, os.MkdirAll(env.stagingDir, 0o755))
	require.NoError(t, os.MkdirAll(env.datasetsDir, 0o755))

	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: filepath.Join(root, "registry.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	env.store = s
	return env
}

// writeStagedFile materializes a tiny .duckdb file inside the staging dir
// (simulating a completed import batch) and returns its path.
func (e *stagingEnv) writeStagedFile(t *testing.T, id string) string {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(e.stagingDir, id+".duckdb")
	c, err := New(ctx, Config{DSN: path})
	require.NoError(t, err)
	_, err = c.DB().ExecContext(ctx, "CREATE TABLE marker AS SELECT '"+id+"' AS tag")
	require.NoError(t, err)
	require.NoError(t, c.Close())
	return path
}

func sampleRow(id, datasetsDir string) store.DatasetRow {
	return store.DatasetRow{
		Name:          id,
		Tier:          "权威",
		SchemaJSON:    `{"tables":{"marker":["tag"]}}`,
		FilePath:      filepath.Join(datasetsDir, id+".duckdb"),
		ContentSHA256: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		BatchID:       "batch-2026-08-22-001",
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}
}

// TestStagingAtomicRename is the 10-lock series lock #10 (D14-D4 + D14-D2):
// a dataset import commits atomically — staging rename and registry row land
// as one commit semantic, and the failure paths never leave a half-committed
// dataset behind.
func TestStagingAtomicRename(t *testing.T) {
	ctx := context.Background()

	t.Run("normal-commit", func(t *testing.T) {
		env := newStagingEnv(t)
		staged := env.writeStagedFile(t, "ds_commit_ok")

		row := sampleRow("ds_commit_ok", env.datasetsDir)
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: staged,
			DatasetID:   "ds_commit_ok",
			Row:         row,
			Registry:    env.store,
			DatasetsDir: env.datasetsDir,
		}))

		// File landed in datasets/ under its final name; staging is empty.
		target := filepath.Join(env.datasetsDir, "ds_commit_ok.duckdb")
		assert.FileExists(t, target)
		assert.NoFileExists(t, staged)

		// Registry row exists with every D14-D2 field intact.
		rows, err := env.store.ListDatasets(ctx)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		got := rows[0]
		assert.Equal(t, row.Name, got.Name)
		assert.Equal(t, row.Tier, got.Tier)
		assert.Equal(t, row.SchemaJSON, got.SchemaJSON)
		assert.Equal(t, row.FilePath, got.FilePath)
		assert.Equal(t, row.ContentSHA256, got.ContentSHA256)
		assert.Equal(t, row.BatchID, got.BatchID)
	})

	t.Run("interrupted-no-staging-file", func(t *testing.T) {
		env := newStagingEnv(t)

		err := CommitStagedDataset(ctx, CommitOptions{
			StagingPath: filepath.Join(env.stagingDir, "never_written.duckdb"),
			DatasetID:   "ds_interrupt",
			Row:         sampleRow("ds_interrupt", env.datasetsDir),
			Registry:    env.store,
			DatasetsDir: env.datasetsDir,
		})
		require.Error(t, err)

		// No half-state: datasets/ gained nothing, registry gained nothing.
		entries, readErr := os.ReadDir(env.datasetsDir)
		require.NoError(t, readErr)
		assert.Empty(t, entries, "datasets/ must stay empty after interrupted commit")
		rows, listErr := env.store.ListDatasets(ctx)
		require.NoError(t, listErr)
		assert.Empty(t, rows, "registry must stay empty after interrupted commit")
	})

	t.Run("target-exists-windows-lock", func(t *testing.T) {
		env := newStagingEnv(t)

		// First commit lands normally.
		staged := env.writeStagedFile(t, "ds_locked")
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: staged,
			DatasetID:   "ds_locked",
			Row:         sampleRow("ds_locked", env.datasetsDir),
			Registry:    env.store,
			DatasetsDir: env.datasetsDir,
		}))
		target := filepath.Join(env.datasetsDir, "ds_locked.duckdb")
		before, err := os.ReadFile(target)
		require.NoError(t, err)

		// Hold an open handle on the target (Windows-lock simulation; a no-op
		// extra guarantee on Linux) while a second import for the same id
		// tries to commit.
		handle, err := os.Open(target)
		require.NoError(t, err)
		defer func() { _ = handle.Close() }()

		staged2 := env.writeStagedFile(t, "ds_locked")
		err = CommitStagedDataset(ctx, CommitOptions{
			StagingPath: staged2,
			DatasetID:   "ds_locked",
			Row:         sampleRow("ds_locked", env.datasetsDir),
			Registry:    env.store,
			DatasetsDir: env.datasetsDir,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDatasetExists)

		// The committed dataset is untouched, byte-for-byte.
		after, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, before, after, "existing dataset content must not change on a rejected commit")

		rows, err := env.store.ListDatasets(ctx)
		require.NoError(t, err)
		assert.Len(t, rows, 1, "registry must not gain a second row")
	})

	t.Run("idempotent-replay-same-batch", func(t *testing.T) {
		env := newStagingEnv(t)
		staged := env.writeStagedFile(t, "ds_replay")
		row := sampleRow("ds_replay", env.datasetsDir)
		require.NoError(t, CommitStagedDataset(ctx, CommitOptions{
			StagingPath: staged,
			DatasetID:   "ds_replay",
			Row:         row,
			Registry:    env.store,
			DatasetsDir: env.datasetsDir,
		}))
		rowsBefore, err := env.store.ListDatasets(ctx)
		require.NoError(t, err)
		require.Len(t, rowsBefore, 1)
		target := filepath.Join(env.datasetsDir, "ds_replay.duckdb")
		before, err := os.ReadFile(target)
		require.NoError(t, err)

		// The SAME batch replays Commit for the already-committed id.
		stagedAgain := env.writeStagedFile(t, "ds_replay")
		err = CommitStagedDataset(ctx, CommitOptions{
			StagingPath: stagedAgain,
			DatasetID:   "ds_replay",
			Row:         row,
			Registry:    env.store,
			DatasetsDir: env.datasetsDir,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDatasetExists)

		rowsAfter, err := env.store.ListDatasets(ctx)
		require.NoError(t, err)
		assert.Len(t, rowsAfter, 1, "replayed commit must not duplicate the registry row")
		after, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, before, after, "replayed commit must not touch the committed file")
	})
}
