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

// Phase 15 / Plan 15-02 — datasets registry new-column round-trip + delete
// path tests.
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// TestDatasetRowNewColumnsRoundTrip — PutDataset writes the three Phase 15
// additive fields; ListDatasets reads them back. Zero-value rows (old callers
// that don't set the new fields) round-trip as ""/""/0.
func TestDatasetRowNewColumnsRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	row := store.DatasetRow{
		Name:          "loans__2026-07",
		Tier:          "历史",
		SchemaJSON:    `{"cols":[{"name":"a"}]}`,
		FilePath:      "data/datasets/loans__2026-07.duckdb",
		ContentSHA256: "aabbccdd",
		BatchID:       "b1",
		CreatedAt:     time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		Dataset:       "loans",
		Period:        "2026-07",
		KMinOverride:  10,
	}
	require.NoError(t, s.PutDataset(ctx, row))

	rows, err := s.ListDatasets(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, "loans", got.Dataset)
	assert.Equal(t, "2026-07", got.Period)
	assert.Equal(t, 10, got.KMinOverride)

	// Zero-value row: old caller that doesn't set the new fields.
	zero := store.DatasetRow{
		Name:          "legacy__2026-06",
		Tier:          "历史",
		SchemaJSON:    `{}`,
		FilePath:      "data/datasets/legacy__2026-06.duckdb",
		ContentSHA256: "00000000",
		BatchID:       "b0",
		CreatedAt:     time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
	}
	require.NoError(t, s.PutDataset(ctx, zero))

	rows, err = s.ListDatasets(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, r := range rows {
		if r.Name == "legacy__2026-06" {
			assert.Equal(t, "", r.Dataset, "unset Dataset defaults to ''")
			assert.Equal(t, "", r.Period, "unset Period defaults to ''")
			assert.Equal(t, 0, r.KMinOverride, "unset KMinOverride defaults to 0")
		}
	}
}

// TestDeleteDataset — DeleteDataset removes the registry row by exact name
// match (no LIKE wildcard injection). The deleted row is no longer listed.
func TestDeleteDataset(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	for _, name := range []string{"loans__2026-07", "loans__2026-08", "cards__2026-07"} {
		require.NoError(t, s.PutDataset(ctx, store.DatasetRow{
			Name:      name,
			Tier:      "历史",
			SchemaJSON: `{}`,
			FilePath:  "data/datasets/" + name + ".duckdb",
			BatchID:   "b1",
			CreatedAt: time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		}))
	}

	require.NoError(t, s.DeleteDataset(ctx, "loans__2026-07"))

	rows, err := s.ListDatasets(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	names := []string{rows[0].Name, rows[1].Name}
	assert.Contains(t, names, "loans__2026-08")
	assert.Contains(t, names, "cards__2026-07")
	assert.NotContains(t, names, "loans__2026-07")
}

// TestDeleteDatasetMissing — deleting a non-existent dataset is idempotent.
func TestDeleteDatasetMissing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	require.NoError(t, s.DeleteDataset(ctx, "none"))
	rows, err := s.ListDatasets(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
