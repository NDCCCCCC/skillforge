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

// Package scheduler implements the cron-driven report regeneration service
// (Phase 19 / Plan 19-03).
package scheduler

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// TTLCleanupConfig configures the TTL cleanup job.
type TTLCleanupConfig struct {
	// TTL is how old a dataset must be before it is eligible for cleanup.
	// Datasets created before (now - TTL) are eligible.
	// Examples: 90 * 24 * time.Hour = 90 days
	TTL time.Duration
	// DatasetsDir is the directory containing .duckdb files (used for
	// validation — the file must exist at the registered FilePath).
	DatasetsDir string
}

// Detacher is the interface for releasing a dataset's file handle before deletion.
// Implemented by AttachLRU in internal/duckdb.
type Detacher interface {
	Detach(ctx context.Context, id string) error
}

// RunTTLCleanup scans the datasets registry for anonymized datasets older than TTL
// and deletes their files after DETACHing from the DuckDB engine.
//
// Key behaviors:
//   - Only datasets where Tier == "anonymized" are eligible.
//   - DETACH runs before os.Remove to release the Windows file handle.
//   - Audit rows (kpi_calls, report_runs, audit) are NEVER touched.
//   - If DETACH fails, the file deletion is skipped (no os.Remove attempted).
//   - If the file is already gone (stray registry entry), a warning is logged and
//     the iteration continues.
//   - The cleanup is idempotent — running twice on the same dataset is safe.
func RunTTLCleanup(ctx context.Context, storeDB store.Store, detacher Detacher, cfg TTLCleanupConfig) error {
	if cfg.TTL <= 0 {
		log.Printf("[WARN] scheduler: TTL cleanup: TTL must be positive, got %v — skipping", cfg.TTL)
		return nil
	}

	datasets, err := storeDB.ListDatasets(ctx)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-cfg.TTL)
	var deleted, skipped, failed int

	for _, ds := range datasets {
		// Only anonymized datasets are eligible (DATA-18 invariant)
		if ds.Tier != "anonymized" {
			continue
		}

		// Check if older than TTL
		if ds.CreatedAt.After(cutoff) {
			continue
		}

		// DETACH first — CRITICAL on Windows: releases the file handle so os.Remove succeeds.
		// If DETACH fails, skip the file rather than attempting os.Remove against a locked file.
		if detacher != nil {
			if err := detacher.Detach(ctx, ds.Name); err != nil {
				log.Printf("[WARN] scheduler: TTL cleanup: DETACH %s failed: %v — skipping file deletion", ds.Name, err)
				skipped++
				continue
			}
		}

		// Delete the file
		if err := os.Remove(ds.FilePath); err != nil {
			if os.IsNotExist(err) {
				log.Printf("[WARN] scheduler: TTL cleanup: file already gone (stray registry entry): %s", ds.FilePath)
				continue
			}
			log.Printf("[ERROR] scheduler: TTL cleanup: Remove %s failed: %v", ds.FilePath, err)
			failed++
			continue
		}

		log.Printf("[OK] scheduler: TTL cleanup: deleted %s (age: %v, tier: %s)", ds.Name, time.Since(ds.CreatedAt), ds.Tier)
		deleted++
	}

	log.Printf("[OK] scheduler: TTL cleanup complete: deleted=%d, skipped=%d, failed=%d, total_datasets_scanned=%d",
		deleted, skipped, failed, len(datasets))

	return nil
}
