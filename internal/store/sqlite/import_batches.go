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

// Phase 15 / Plan 15-02 — INSERT-only import batch audit + dataset delete
// audit storage. This package deliberately provides NO UPDATE / DELETE
// paths for these rows (T-3-AUDIT anti-tampering).
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putImportBatchSQL is the single write path for the import_batches audit
// table (DATA-17 + D15-A4 + D15-C1). batch_id is the PRIMARY KEY; replays
// surface as constraint errors, never silent replacement (T-15-13).
const putImportBatchSQL = `
INSERT INTO import_batches (
    batch_id, dataset, period, tier, content_sha256, rules_sha256,
    raw_size_bytes, desensitized_row_count, discard_ts, supersedes_batch_id,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// getLatestBatchSQL returns the newest batch for a (dataset, period) key.
// Ordering by created_at DESC makes the idempotency probe deterministic
// when multiple batches exist (D15-C1 supersede chain).
const getLatestBatchSQL = `
SELECT batch_id, dataset, period, tier, content_sha256, rules_sha256,
       raw_size_bytes, desensitized_row_count, discard_ts, supersedes_batch_id,
       created_at
FROM import_batches
WHERE dataset = ? AND period = ?
ORDER BY created_at DESC
LIMIT 1
`

// listImportBatchesSQL returns every batch for a dataset, newest first.
const listImportBatchesSQL = `
SELECT batch_id, dataset, period, tier, content_sha256, rules_sha256,
       raw_size_bytes, desensitized_row_count, discard_ts, supersedes_batch_id,
       created_at
FROM import_batches
WHERE dataset = ?
ORDER BY created_at DESC
`

// putDatasetDeleteSQL appends one dataset deletion audit row (D15-C3).
const putDatasetDeleteSQL = `
INSERT INTO dataset_deletes (name, dataset, deleted_at)
VALUES (?, ?, ?)
`

// PutImportBatch appends one INSERT-only import batch audit row.
func (s *SQLiteStore) PutImportBatch(ctx context.Context, row store.ImportBatchRow) error {
	_, err := s.db.ExecContext(ctx, putImportBatchSQL,
		row.BatchID,
		row.Dataset,
		row.Period,
		row.Tier,
		row.ContentSHA256,
		row.RulesSHA256,
		row.RawSizeBytes,
		row.DesensitizedRowCount,
		row.DiscardTS,
		row.SupersedesBatchID,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmtError("PutImportBatch", err)
	}
	return nil
}

// LatestImportBatch returns the newest batch for the (dataset, period) key,
// or (nil, nil) when no batch exists.
func (s *SQLiteStore) LatestImportBatch(ctx context.Context, dataset, period string) (*store.ImportBatchRow, error) {
	var r store.ImportBatchRow
	var createdAt string
	err := s.db.QueryRowContext(ctx, getLatestBatchSQL, dataset, period).Scan(
		&r.BatchID,
		&r.Dataset,
		&r.Period,
		&r.Tier,
		&r.ContentSHA256,
		&r.RulesSHA256,
		&r.RawSizeBytes,
		&r.DesensitizedRowCount,
		&r.DiscardTS,
		&r.SupersedesBatchID,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmtError("LatestImportBatch", err)
	}
	ts, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmtError("LatestImportBatch", err)
	}
	r.CreatedAt = ts.UTC()
	return &r, nil
}

// ListImportBatches returns every batch audit row for dataset, newest first.
func (s *SQLiteStore) ListImportBatches(ctx context.Context, dataset string) ([]store.ImportBatchRow, error) {
	rows, err := s.db.QueryContext(ctx, listImportBatchesSQL, dataset)
	if err != nil {
		return nil, fmtError("ListImportBatches", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.ImportBatchRow, 0)
	for rows.Next() {
		var r store.ImportBatchRow
		var createdAt string
		if err := rows.Scan(
			&r.BatchID,
			&r.Dataset,
			&r.Period,
			&r.Tier,
			&r.ContentSHA256,
			&r.RulesSHA256,
			&r.RawSizeBytes,
			&r.DesensitizedRowCount,
			&r.DiscardTS,
			&r.SupersedesBatchID,
			&createdAt,
		); err != nil {
			return nil, fmtError("ListImportBatches", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmtError("ListImportBatches", err)
		}
		r.CreatedAt = ts.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListImportBatches", err)
	}
	return out, nil
}

// PutDatasetDelete appends one INSERT-only dataset deletion audit row.
func (s *SQLiteStore) PutDatasetDelete(ctx context.Context, row store.DatasetDeleteRow) error {
	_, err := s.db.ExecContext(ctx, putDatasetDeleteSQL,
		row.Name,
		row.Dataset,
		row.DeletedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmtError("PutDatasetDelete", err)
	}
	return nil
}
