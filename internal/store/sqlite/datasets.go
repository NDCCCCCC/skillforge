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

package sqlite

import (
	"context"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putDatasetSQL is the single write path for the datasets registry
// (D14-D2). INSERT OR REPLACE keyed on name: the staging commit path
// (internal/duckdb.CommitStagedDataset) may replay a batch, and the
// registry upsert is idempotent by design. Every value is a `?`
// placeholder — the row never touches fmt.Sprintf (T-3-SQLI / T-14-16).
//
// Phase 15 / Plan 15-02: extended with dataset/period/k_min_override
// additive columns (DEFAULT '' / 0 for old callers).
const putDatasetSQL = `
INSERT OR REPLACE INTO datasets (
    name, tier, schema_json, file_path, content_sha256, batch_id, created_at,
    dataset, period, k_min_override
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// listDatasetsSQL returns every registry row ordered by name for stable
// admin/preview rendering.
const listDatasetsSQL = `
SELECT name, tier, schema_json, file_path, content_sha256, batch_id, created_at,
       dataset, period, k_min_override
FROM datasets
ORDER BY name ASC
`

// deleteDatasetSQL removes a registry row by exact name match. The `?`
// placeholder prevents LIKE wildcard injection (T-15-14 mitigation).
const deleteDatasetSQL = `DELETE FROM datasets WHERE name = ?`

// PutDataset upserts one dataset registry row. Idempotent on name.
func (s *SQLiteStore) PutDataset(ctx context.Context, row store.DatasetRow) error {
	_, err := s.db.ExecContext(ctx, putDatasetSQL,
		row.Name,
		row.Tier,
		row.SchemaJSON,
		row.FilePath,
		row.ContentSHA256,
		row.BatchID,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
		row.Dataset,
		row.Period,
		row.KMinOverride,
	)
	if err != nil {
		return fmtError("PutDataset", err)
	}
	return nil
}

// ListDatasets returns every registered dataset row ordered by name.
// Returns an empty (non-nil) slice for an empty registry.
func (s *SQLiteStore) ListDatasets(ctx context.Context) ([]store.DatasetRow, error) {
	rows, err := s.db.QueryContext(ctx, listDatasetsSQL)
	if err != nil {
		return nil, fmtError("ListDatasets", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.DatasetRow, 0)
	for rows.Next() {
		var r store.DatasetRow
		var createdAt string
		if err := rows.Scan(
			&r.Name,
			&r.Tier,
			&r.SchemaJSON,
			&r.FilePath,
			&r.ContentSHA256,
			&r.BatchID,
			&createdAt,
			&r.Dataset,
			&r.Period,
			&r.KMinOverride,
		); err != nil {
			return nil, fmtError("ListDatasets", err)
		}
		ts, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmtError("ListDatasets", err)
		}
		r.CreatedAt = ts.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListDatasets", err)
	}
	return out, nil
}

// DeleteDataset removes the dataset registry row with the exact name.
// Prefix deletion is intentionally NOT implemented in SQL (the `_` in LIKE
// is a wildcard); callers that need prefix semantics must ListDatasets and
// filter with strings.HasPrefix in Go (15-06 delete endpoint shape).
func (s *SQLiteStore) DeleteDataset(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, deleteDatasetSQL, name)
	if err != nil {
		return fmtError("DeleteDataset", err)
	}
	return nil
}

