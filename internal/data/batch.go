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

	"github.com/skillforge/skillforge/internal/store"
)

// registryName builds the composite dataset registry name.
func registryName(dataset, period string) string {
	return dataset + "__" + period
}

// newBatchID returns a crypto-random hex batch identifier.
func newBatchID() string {
	// Skeleton: deterministic placeholder for RED compile.
	return "batch-placeholder"
}

// checkIdempotent probes the latest batch for (dataset, period).
func checkIdempotent(ctx context.Context, seam ImportStoreSeam, dataset, period, contentSHA256 string) (existingBatchID string, supersededBatchID string, err error) {
	return "", "", nil
}

// writeAuditRow appends one INSERT-only import_batches audit row.
func writeAuditRow(ctx context.Context, seam ImportStoreSeam, row store.ImportBatchRow) error {
	return nil
}
