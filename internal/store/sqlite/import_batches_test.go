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

// Phase 15 / Plan 15-02 — import_batches INSERT-only audit + dataset_deletes
// audit tests. Lives in internal/store/sqlite (white-box) because it asserts
// the exact row shape and constraint behavior of the audit tables.
package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// fullBatchRow returns a fully-populated ImportBatchRow for round-trip tests.
func fullBatchRow(batchID string) store.ImportBatchRow {
	return store.ImportBatchRow{
		BatchID:              batchID,
		Dataset:              "loans",
		Period:               "2026-07",
		Tier:                 "历史",
		ContentSHA256:        "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		RulesSHA256:          "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		RawSizeBytes:         12345,
		DesensitizedRowCount: 1000,
		DiscardTS:            "2026-07-15T10:00:00Z",
		SupersedesBatchID:    "b0",
		CreatedAt:            time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
	}
}

// TestBatchAuditRowShape — PutImportBatch writes a full 11-column row;
// ListImportBatches reads it back field-for-field; LatestImportBatch returns
// the newest batch for a (dataset, period) key (DATA-16 + DATA-17).
func TestBatchAuditRowShape(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	b1 := fullBatchRow("b1")
	b1.CreatedAt = time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	require.NoError(t, s.PutImportBatch(ctx, b1))

	b2 := fullBatchRow("b2")
	b2.SupersedesBatchID = "b1"
	b2.CreatedAt = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	require.NoError(t, s.PutImportBatch(ctx, b2))

	// ListImportBatches returns rows newest-first.
	rows, err := s.ListImportBatches(ctx, "loans")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	gotNewest := rows[0]
	gotOldest := rows[1]

	assert.Equal(t, b2.BatchID, gotNewest.BatchID)
	assert.Equal(t, b2.Dataset, gotNewest.Dataset)
	assert.Equal(t, b2.Period, gotNewest.Period)
	assert.Equal(t, b2.Tier, gotNewest.Tier)
	assert.Equal(t, b2.ContentSHA256, gotNewest.ContentSHA256)
	assert.Equal(t, b2.RulesSHA256, gotNewest.RulesSHA256)
	assert.Equal(t, b2.RawSizeBytes, gotNewest.RawSizeBytes)
	assert.Equal(t, b2.DesensitizedRowCount, gotNewest.DesensitizedRowCount)
	assert.Equal(t, b2.DiscardTS, gotNewest.DiscardTS)
	assert.Equal(t, b2.SupersedesBatchID, gotNewest.SupersedesBatchID)

	assert.Equal(t, b1.BatchID, gotOldest.BatchID)
	assert.Equal(t, "b0", gotOldest.SupersedesBatchID)

	// LatestImportBatch returns the newest batch for the key.
	latest, err := s.LatestImportBatch(ctx, "loans", "2026-07")
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "b2", latest.BatchID)
	assert.Equal(t, b2.RulesSHA256, latest.RulesSHA256)
}

// TestLatestImportBatchMissing — LatestImportBatch returns (nil, nil) when
// no batch matches the (dataset, period) key.
func TestLatestImportBatchMissing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	latest, err := s.LatestImportBatch(ctx, "none", "2026-07")
	require.NoError(t, err)
	assert.Nil(t, latest, "LatestImportBatch must return nil for missing key")
}

// TestImportBatchInsertOnly — the same batch_id written twice must produce a
// constraint error, not silently replace the row (T-15-13 mitigation).
func TestImportBatchInsertOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	b := fullBatchRow("b1")
	require.NoError(t, s.PutImportBatch(ctx, b))

	b2 := b
	b2.RawSizeBytes = 99999
	err = s.PutImportBatch(ctx, b2)
	require.Error(t, err, "duplicate batch_id must error")
	assert.Contains(t, err.Error(), "UNIQUE", "error should be a PRIMARY KEY/UNIQUE constraint failure")

	// Original row unchanged.
	latest, err := s.LatestImportBatch(ctx, "loans", "2026-07")
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, int64(12345), latest.RawSizeBytes, "original row must not be replaced")
}

// TestPutDatasetDelete — a dataset delete audit row round-trips.
func TestPutDatasetDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	del := store.DatasetDeleteRow{
		Name:      "loans__2026-07",
		Dataset:   "loans",
		DeletedAt: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
	}
	require.NoError(t, s.PutDatasetDelete(ctx, del))

	var name, dataset, deletedAt string
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT name, dataset, deleted_at FROM dataset_deletes ORDER BY id DESC LIMIT 1`).Scan(
		&name, &dataset, &deletedAt))
	assert.Equal(t, "loans__2026-07", name)
	assert.Equal(t, "loans", dataset)
	assert.Equal(t, del.DeletedAt.UTC().Format(time.RFC3339Nano), deletedAt)
}
