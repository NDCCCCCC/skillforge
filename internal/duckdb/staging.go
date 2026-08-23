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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// Directory conventions for the data-support track (D14-D4, fixed in this
// phase; Phase 15's import pipeline consumes them directly). Both live under
// data/ on the SAME volume — the same-volume guarantee is what makes
// os.Rename atomic (POSIX rename(2); Windows MoveFileExW). The data/ tree is
// git-ignored (runtime state, never committed).
const (
	// DefaultStagingDir holds in-flight import batches. A file here is a
	// batch that has NOT been committed; a crash may leave orphans here
	// and that is by design — cleanup only ever touches staging.
	DefaultStagingDir = "data/staging"
	// DefaultDatasetsDir holds committed datasets. An invariant, not an
	// aspiration: every file here is complete and registered.
	DefaultDatasetsDir = "data/datasets"
)

// supersedeBackupName generates the transient backup filename for a supersede
// operation. The timestamp makes concurrent supersede attempts for the same
// dataset id produce distinct backup files. It is overridable in tests for
// deterministic failure injection.
var supersedeBackupName = func(datasetID string) string {
	return fmt.Sprintf(".superseded-%s-%d.duckdb", datasetID, time.Now().UnixNano())
}

// ErrDatasetExists is returned when a commit targets a dataset id whose
// final file already exists (replay of an already-committed batch, or an
// id collision). The commit is rejected WITHOUT side effects — the existing
// dataset is untouched (T-14-15).
var ErrDatasetExists = errors.New("duckdb: dataset already committed")

// DatasetRegistry is the narrow write+read seam the staging commit path
// needs from the sqlite store (interface isolation: staging depends on the
// two dataset operations, not the whole store.Store). The production
// implementation is *sqlite.SQLiteStore via PutDataset/ListDatasets.
type DatasetRegistry interface {
	PutDataset(ctx context.Context, row store.DatasetRow) error
}

// ErrSupersedeTargetMissing is returned when SupersedeStagedDataset is asked
// to replace a dataset that has not been committed yet. Use CommitStagedDataset
// for new datasets.
var ErrSupersedeTargetMissing = errors.New("duckdb: supersede target does not exist — use CommitStagedDataset for a new dataset")

// SupersedeOptions parameterizes SupersedeStagedDataset.
type SupersedeOptions struct {
	// StagingPath is the completed staging file (under DefaultStagingDir).
	// Required.
	StagingPath string
	// DatasetID is the validated dataset identifier; the final file name
	// is {DatasetsDir}/{DatasetID}.duckdb. Required; must satisfy
	// ValidateDatasetID and equal Row.Name.
	DatasetID string
	// Row is the registry row written after the rename. Required;
	// Row.Name must equal DatasetID.
	Row store.DatasetRow
	// Registry is the dataset registry (sqlite store). Required.
	Registry DatasetRegistry
	// DatasetsDir overrides DefaultDatasetsDir (tests use a temp dir;
	// empty falls back to the default).
	DatasetsDir string
	// BackupDir overrides DefaultStagingDir for the transient backup file
	// (tests use a temp dir; empty falls back to DefaultStagingDir).
	BackupDir string
	// LRU optionally supplies the lazy-ATTACH registry: when the dataset
	// id is currently ATTACHed, it is DETACHed FIRST so the engine drops
	// its file handle — on Windows the rename would otherwise fail on a
	// locked source/target (T-14-17). May be nil.
	LRU *AttachLRU
}

// CommitOptions parameterizes CommitStagedDataset.
type CommitOptions struct {
	// StagingPath is the completed staging file (under DefaultStagingDir).
	// Required.
	StagingPath string
	// DatasetID is the validated dataset identifier; the final file name
	// is {DatasetsDir}/{DatasetID}.duckdb. Required; must satisfy
	// ValidateDatasetID and equal Row.Name.
	DatasetID string
	// Row is the registry row written after the rename. Required;
	// Row.Name must equal DatasetID (the registry and the file name are
	// one identity).
	Row store.DatasetRow
	// Registry is the dataset registry (sqlite store). Required.
	Registry DatasetRegistry
	// DatasetsDir overrides DefaultDatasetsDir (tests use a temp dir;
	// empty falls back to the default).
	DatasetsDir string
	// LRU optionally supplies the lazy-ATTACH registry: when the dataset
	// id is currently ATTACHed, it is DETACHed FIRST so the engine drops
	// its file handle — on Windows the rename would otherwise fail on a
	// locked source/target (T-14-17). May be nil.
	LRU *AttachLRU
}

// CommitStagedDataset promotes a completed staging file into a committed
// dataset (D14-D4): atomic same-volume rename + registry row write with one
// commit semantic.
//
// Semantics:
//   - DatasetsDir is created if missing (MkdirAll — idempotent).
//   - An existing target file aborts with ErrDatasetExists BEFORE any
//     mutation (replay/id-collision guard; the existing dataset stays
//     byte-identical).
//   - A missing staging file aborts with a descriptive error — datasets/
//     and the registry gain nothing (no half-state, T-14-15).
//   - On ANY failure the staging file is removed (best-effort) so staging
//     never accumulates dead batches; on success the staging file is gone
//     by definition (it was renamed).
//   - If the registry write fails AFTER the rename, the rename is
//     compensated (file moved back to its staging path, best-effort). This
//     is an approximate transaction — the filesystem has no rollback; a
//     compensation failure returns an aggregate error for operator triage.
//
// Callers MUST NOT os.Remove a dataset file without a prior Detach — the
// engine holds a handle while a dataset is ATTACHed (Windows lock boundary,
// Pitfall 4).
func CommitStagedDataset(ctx context.Context, opts CommitOptions) error {
	if opts.Registry == nil {
		return fmtError("CommitStagedDataset", errors.New("registry is required"))
	}
	if err := ValidateDatasetID(opts.DatasetID); err != nil {
		return err
	}
	if opts.Row.Name != opts.DatasetID {
		return fmtError("CommitStagedDataset", fmt.Errorf(
			"row name %q does not match dataset id %q — the registry row and file name are one identity",
			opts.Row.Name, opts.DatasetID))
	}

	datasetsDir := opts.DatasetsDir
	if datasetsDir == "" {
		datasetsDir = DefaultDatasetsDir
	}
	target := filepath.Join(datasetsDir, opts.DatasetID+".duckdb")

	// Release the engine-side handle first if this id is ATTACHed
	// (idempotent — a no-op when not attached).
	if opts.LRU != nil {
		if err := opts.LRU.Detach(ctx, opts.DatasetID); err != nil {
			return fmtError("CommitStagedDataset", fmt.Errorf("pre-commit detach: %w", err))
		}
	}

	// Replay / id-collision guard: the target existing means this id was
	// already committed (or is being committed concurrently) — reject
	// without touching anything.
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", ErrDatasetExists, target)
	}
	if _, err := os.Stat(opts.StagingPath); err != nil {
		return fmtError("CommitStagedDataset", fmt.Errorf(
			"staging file %s missing — the batch did not complete: %w",
			opts.StagingPath, err))
	}
	if err := os.MkdirAll(datasetsDir, 0o755); err != nil {
		return fmtError("CommitStagedDataset", err)
	}

	// Failure cleanup: a commit that does not reach "renamed" removes its
	// staging residue. On success the defer is skipped (renamed=true).
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(opts.StagingPath)
		}
	}()

	// Same-volume atomic rename (staging/ and datasets/ both live under
	// data/). No two-phase copy + marker dance — the OS rename IS the
	// atomicity boundary (RESEARCH Don't Hand-Roll).
	if err := os.Rename(opts.StagingPath, target); err != nil {
		return fmtError("CommitStagedDataset", fmt.Errorf("atomic rename %s -> %s: %w",
			opts.StagingPath, target, err))
	}
	renamed = true

	if err := opts.Registry.PutDataset(ctx, opts.Row); err != nil {
		// Compensate: move the file back so datasets/ keeps its "complete
		// AND registered" invariant. Best-effort — a compensation failure
		// is an operator-visible aggregate error.
		if rbErr := os.Rename(target, opts.StagingPath); rbErr != nil {
			return fmtError("CommitStagedDataset", fmt.Errorf(
				"registry write failed (%v) AND rollback rename failed (%v) — dataset %s is on disk but UNREGISTERED; triage manually",
				err, rbErr, target))
		}
		// renamed stays true — the restored staging file is a COMPLETE
		// batch, not residue: keep it so the caller can retry the commit.
		return fmtError("CommitStagedDataset", fmt.Errorf("registry write failed (rolled back; staging preserved for retry): %w", err))
	}
	return nil
}

// SupersedeStagedDataset atomically replaces an already-committed dataset with
// a new staging file using a three-step backup-rename chain (D15-C1).
//
// This is the replacement-path counterpart to CommitStagedDataset. While
// CommitStagedDataset refuses to overwrite an existing target
// (ErrDatasetExists), SupersedeStagedDataset REQUIRES the target to exist and
// performs backup-rename-register so that a failure after the old file has been
// moved leaves the old data restorable.
//
// Semantics:
//   - Registry must be non-nil; DatasetID must satisfy ValidateDatasetID.
//   - Row.Name must equal DatasetID (the registry row and file name are one
//     identity).
//   - If LRU is non-nil, the dataset is DETACHed first so the engine releases
//     its file handle before any rename (Windows lock boundary, Pitfall 9).
//   - The target file must already exist; otherwise ErrSupersedeTargetMissing
//     is returned and callers should use CommitStagedDataset for a new dataset.
//   - The staging file must exist.
//   - Step 1: os.Rename(target, backup) where backup lives under BackupDir
//     (or DefaultStagingDir if empty) as
//     .superseded-{id}-{nanos}.duckdb. Same-volume atomic.
//   - Step 2: os.Rename(stagingNew, target).
//   - Step 3: Registry.PutDataset(newRow).
//   - If step 2 fails: restore backup to target (best-effort); staging is
//     removed as residue.
//   - If step 3 fails: move target back to staging and backup back to target
//     (best-effort) so the old dataset remains queryable and unregistered.
//   - On success: remove the backup file.
func SupersedeStagedDataset(ctx context.Context, opts SupersedeOptions) error {
	if opts.Registry == nil {
		return fmtError("SupersedeStagedDataset", errors.New("registry is required"))
	}
	if err := ValidateDatasetID(opts.DatasetID); err != nil {
		return err
	}
	if opts.Row.Name != opts.DatasetID {
		return fmtError("SupersedeStagedDataset", fmt.Errorf(
			"row name %q does not match dataset id %q — the registry row and file name are one identity",
			opts.Row.Name, opts.DatasetID))
	}

	datasetsDir := opts.DatasetsDir
	if datasetsDir == "" {
		datasetsDir = DefaultDatasetsDir
	}
	backupDir := opts.BackupDir
	if backupDir == "" {
		backupDir = DefaultStagingDir
	}
	target := filepath.Join(datasetsDir, opts.DatasetID+".duckdb")
	backup := filepath.Join(backupDir, supersedeBackupName(opts.DatasetID))

	// Release the engine-side handle first if this id is ATTACHed
	// (idempotent — a no-op when not attached).
	if opts.LRU != nil {
		if err := opts.LRU.Detach(ctx, opts.DatasetID); err != nil {
			return fmtError("SupersedeStagedDataset", fmt.Errorf("pre-supersede detach: %w", err))
		}
	}

	// The target must exist — supersede is replacement, not creation.
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrSupersedeTargetMissing, target)
		}
		return fmtError("SupersedeStagedDataset", fmt.Errorf("target stat failed: %w", err))
	}
	if _, err := os.Stat(opts.StagingPath); err != nil {
		return fmtError("SupersedeStagedDataset", fmt.Errorf(
			"staging file %s missing — the batch did not complete: %w",
			opts.StagingPath, err))
	}
	if err := os.MkdirAll(datasetsDir, 0o755); err != nil {
		return fmtError("SupersedeStagedDataset", err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmtError("SupersedeStagedDataset", err)
	}

	// Cleanup tracking: if step 2 never completes, the original staging file
	// is residue and should be removed. If step 2 completes, staging has been
	// promoted to target and no longer exists at StagingPath; preserving it
	// on a later rollback lets the caller retry.
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.Remove(opts.StagingPath)
		}
	}()

	// Step 1: move the existing dataset to a transient backup.
	if err := os.Rename(target, backup); err != nil {
		return fmtError("SupersedeStagedDataset", fmt.Errorf("backup rename %s -> %s: %w",
			target, backup, err))
	}

	// Step 2: promote the new staging file into place.
	if err := os.Rename(opts.StagingPath, target); err != nil {
		// Compensate: restore backup to target so old data is queryable.
		if rbErr := os.Rename(backup, target); rbErr != nil {
			return fmtError("SupersedeStagedDataset", fmt.Errorf(
				"rename staging->target failed (%v) AND rollback rename failed (%v) — dataset %s is on disk but UNREGISTERED; triage manually",
				err, rbErr, target))
		}
		return fmtError("SupersedeStagedDataset", fmt.Errorf("rename staging->target failed (rolled back): %w", err))
	}
	cleanupStaging = false

	// Step 3: register the new row.
	if err := opts.Registry.PutDataset(ctx, opts.Row); err != nil {
		// Compensate: move target back to staging, backup back to target.
		// Staging now holds the new data and is preserved for retry.
		var rbErrs []error
		if rbErr := os.Rename(target, opts.StagingPath); rbErr != nil {
			rbErrs = append(rbErrs, fmt.Errorf("rollback target->staging failed: %w", rbErr))
		}
		if rbErr := os.Rename(backup, target); rbErr != nil {
			rbErrs = append(rbErrs, fmt.Errorf("rollback backup->target failed: %w", rbErr))
		}
		if len(rbErrs) > 0 {
			return fmtError("SupersedeStagedDataset", fmt.Errorf(
				"registry write failed (%v) AND rollback failed (%v) — dataset %s is on disk but UNREGISTERED; triage manually",
				err, errors.Join(rbErrs...), target))
		}
		return fmtError("SupersedeStagedDataset", fmt.Errorf("registry write failed (rolled back; staging preserved for retry): %w", err))
	}

	// Success: the backup is no longer needed.
	if err := os.Remove(backup); err != nil {
		return fmtError("SupersedeStagedDataset", fmt.Errorf("remove backup %s: %w", backup, err))
	}
	return nil
}
