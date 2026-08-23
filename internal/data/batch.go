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

package data

import (
	"context"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// registryName builds the composite dataset registry name.
func registryName(dataset, period string) string {
	return dataset + "__" + period
}

// newBatchID returns a crypto-random 16-byte hex batch identifier.
func newBatchID() (string, error) {
	return randomHex(16)
}

// checkIdempotent probes the latest batch for (dataset, period).
//   - latest == nil → ("", "", nil) : first import.
//   - latest.ContentSHA256 == contentSHA256 → (latest.BatchID, "", nil) : hit.
//   - otherwise → ("", latest.BatchID, nil) : supersede candidate.
func checkIdempotent(ctx context.Context, seam ImportStoreSeam, dataset, period, contentSHA256 string) (existingBatchID string, supersededBatchID string, err error) {
	latest, err := seam.LatestImportBatch(ctx, dataset, period)
	if err != nil {
		return "", "", err
	}
	if latest == nil {
		return "", "", nil
	}
	if latest.ContentSHA256 == contentSHA256 {
		return latest.BatchID, "", nil
	}
	return "", latest.BatchID, nil
}

// writeAuditRow appends one INSERT-only import_batches audit row.
func writeAuditRow(ctx context.Context, seam ImportStoreSeam, row store.ImportBatchRow) error {
	return seam.PutImportBatch(ctx, row)
}

// newAuditRow constructs a fully populated ImportBatchRow for finalizeBatch.
func newAuditRow(batchID, dataset, period, tier, contentSHA, rulesSHA string, rawSize, rows int64, discardTS, supersedes string, createdAt time.Time) store.ImportBatchRow {
	return store.ImportBatchRow{
		BatchID:              batchID,
		Dataset:              dataset,
		Period:               period,
		Tier:                 tier,
		ContentSHA256:        contentSHA,
		RulesSHA256:          rulesSHA,
		RawSizeBytes:         rawSize,
		DesensitizedRowCount: rows,
		DiscardTS:            discardTS,
		SupersedesBatchID:    supersedes,
		CreatedAt:            createdAt,
	}
}
