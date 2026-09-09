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

// Phase 20 / Plan 20-02 (ADM-02) — admin read-only view of the import batch
// audit log. Serves GET /api/admin/batches.
//
// READ-ONLY BY CONSTRUCTION (ADM-02 / T-20-02-MUTATE). The import_batches
// table is INSERT-only (DATA-17); this handler only retrieves rows. The route
// table registers it for retrieval only, and the companion test source-greps
// this file to keep it that way.
//
// Trust model:
//   - X-Admin-Token gate via isAdminRequest (same crypto/subtle compare as
//     GET /api/audit and GET /api/admin/platform-prompt; unset env → 403
//     for every caller).
//   - No caller input reaches SQL: ListAllImportBatches has no WHERE
//     parameters — the entire table is scanned in creation order.
//   - Fail-closed when the store is nil (no audit backend → 503), mirroring
//     the posture of every other admin handler.
package server

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// importBatchAdminRow is one batch row as rendered for the admin view.
type importBatchAdminRow struct {
	BatchID              string `json:"batch_id"`
	Dataset             string `json:"dataset"`
	Period              string `json:"period"`
	Status              string `json:"status"`
	DesensitizedRowCount int64  `json:"desensitized_row_count"`
	RawSizeBytes        int64  `json:"raw_size_bytes"`
	DiscardTS           string `json:"discard_ts"`
	CreatedAt           string `json:"created_at"`
}

// importBatchesAdminResponse is the 200 envelope for the admin batch list view.
type importBatchesAdminResponse struct {
	Batches []importBatchAdminRow `json:"batches"`
}

// batchesAdminHandler serves GET /api/admin/batches.
//
// Response envelope (200):
//
//	{
//	  "batches": [
//	    {
//	      "batch_id":     "b-001",
//	      "dataset":      "loans",
//	      "period":       "2026-07",
//	      "status":       "completed",
//	      "desensitized_row_count": 5000,
//	      "raw_size_bytes": 1024000,
//	      "discard_ts":   "2026-07-01T12:00:00Z",
//	      "created_at":   "2026-07-01T10:00:00Z"
//	    }
//	  ]
//	}
//
// Errors: 403 (admin token missing/wrong/unset), 503 (store nil), 500 (read failure).
func (s *Deps) batchesAdminHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "store not wired (server started without an audit backend)",
		})
		return
	}

	rows, err := s.Store.ListAllImportBatches(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "list import batches: " + err.Error(),
		})
		return
	}

	batches := make([]importBatchAdminRow, 0, len(rows))
	for _, r := range rows {
		batches = append(batches, importBatchAdminRow{
			BatchID:              r.BatchID,
			Dataset:             r.Dataset,
			Period:              r.Period,
			Status:              batchStatus(r),
			DesensitizedRowCount: r.DesensitizedRowCount,
			RawSizeBytes:        r.RawSizeBytes,
			DiscardTS:           r.DiscardTS,
			CreatedAt:           r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, importBatchesAdminResponse{
		Batches: batches,
	})
}

// batchStatus returns the human-readable status string for a batch based on
// the discard timestamp. A batch that has been discarded is "completed".
// Batches that exist but are not yet discarded are "pending" (import in
// progress). The raw dataset is discarded after desensitization completes.
func batchStatus(r store.ImportBatchRow) string {
	if r.DiscardTS != "" {
		return "completed"
	}
	return "pending"
}
