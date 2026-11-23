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

// Phase 54 / Plan 54-01 (ING-04/05) — the SQLite implementation of the
// ingest.JobStore narrow seam, backed by the ingest_jobs / ingest_steps /
// ingest_items DDL in schema.go. Same posture as ecl_failures.go: the
// mutable status columns ARE the point of these tables (retry semantics),
// the caller-set RFC3339Nano timestamps are stored verbatim, and the
// package imports the consuming package (ingest) the same way it imports
// knowledge/ecl for FailureStore — the seam interface lives with the
// consumer, never in the big store.Store.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/skillforge/skillforge/internal/ingest"
)

// Compile-time conformance check (D11 contract shape): *SQLiteStore
// satisfies the narrow ingest.JobStore seam the runner persists through.
var _ ingest.JobStore = (*SQLiteStore)(nil)

// ErrNoIngestJob is the sentinel wrapped by the lookups for missing jobs;
// callers test with errors.Is (or via ingest.JobStore.IsNotFound).
var ErrNoIngestJob = errors.New("ingest: no job record")

// jobColumns is the ingest_jobs projection shared by the read paths
// (positional Scan order — keep in sync with the DDL).
const jobColumns = `
job_id, slug, filename, sha256, mime, file_size, doc_id, source_path,
parser_name, status, error, created_at, updated_at
`

// createIngestJobSQL is a plain INSERT (no OR REPLACE): the PK is a
// freshly minted random id, so a collision is a programming error and
// must surface, never silently replace a live job's state machine.
const createIngestJobSQL = `
INSERT INTO ingest_jobs (
    job_id, slug, filename, sha256, mime, file_size, doc_id, source_path,
    parser_name, status, error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// createIngestStepSQL seeds one pending step row; the runner loops it
// over ingest.StepSequence inside one transaction.
const createIngestStepSQL = `
INSERT INTO ingest_steps (job_id, step, status, attempt, started_at, completed_at, error, artifact_ref)
VALUES (?, ?, 'pending', 0, '', '', '', '')
`

// updateIngestJobSQL transitions (status, error) + stamps updated_at.
// Unconditional by design — the only CAS-gated transition is ClaimJob.
const updateIngestJobSQL = `
UPDATE ingest_jobs SET status = ?, error = ?, updated_at = ?
WHERE job_id = ?
`

// setJobDocInfoSQL records the parse step's job-row outputs.
const setJobDocInfoSQL = `
UPDATE ingest_jobs
SET doc_id = ?, parser_name = ?, source_path = ?, updated_at = ?
WHERE job_id = ?
`

// upsertIngestStepSQL is the single write path for ingest_steps: the
// runner's per-step transition. ON CONFLICT(job_id, step) updates every
// mutable column; the PK pair never changes.
const upsertIngestStepSQL = `
INSERT INTO ingest_steps (job_id, step, status, attempt, started_at, completed_at, error, artifact_ref)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id, step) DO UPDATE SET
    status       = excluded.status,
    attempt      = excluded.attempt,
    started_at   = excluded.started_at,
    completed_at = excluded.completed_at,
    error        = excluded.error,
    artifact_ref = excluded.artifact_ref
`

// listIngestStepsSQL returns every step row of one job; ordering into
// StepSequence order happens in Go (the table stores no sequence column —
// see the DDL comment).
const listIngestStepsSQL = `
SELECT job_id, step, status, attempt, started_at, completed_at, error, artifact_ref
FROM ingest_steps
WHERE job_id = ?
`

// getIngestJobSQL is the id-keyed lookup behind the GET job endpoint.
const getIngestJobSQL = `
SELECT ` + jobColumns + `
FROM ingest_jobs
WHERE job_id = ?
`

// getIngestJobBySHASQL is the extended-dedup lookup: the NEWEST job for a
// (slug, sha256) pair. created_at is a caller-set RFC3339Nano string, so
// lexicographic DESC == chronological DESC; rowid DESC breaks exact-tie
// determinism (two jobs created within the same nanosecond is impossible
// in practice, but the tiebreak costs nothing).
const getIngestJobBySHASQL = `
SELECT ` + jobColumns + `
FROM ingest_jobs
WHERE slug = ? AND sha256 = ?
ORDER BY created_at DESC, rowid DESC
LIMIT 1
`

// claimIngestJobSQL is the runner's CAS: queued|partial → running in ONE
// statement; RowsAffected == 1 means THIS caller won the claim (T-39-20
// precedent — the check-then-set must be atomic at the SQL layer, not in
// Go).
const claimIngestJobSQL = `
UPDATE ingest_jobs SET status = 'running', updated_at = ?
WHERE job_id = ? AND status IN ('queued', 'partial')
`

// CreateJob implements ingest.JobStore.
func (s *SQLiteStore) CreateJob(ctx context.Context, j ingest.Job) error {
	_, err := s.db.ExecContext(ctx, createIngestJobSQL,
		j.JobID,
		j.Slug,
		j.Filename,
		j.SHA256,
		j.Mime,
		j.FileSize,
		j.DocID,
		j.SourcePath,
		j.ParserName,
		j.Status,
		j.Error,
		j.CreatedAt,
		j.UpdatedAt,
	)
	if err != nil {
		return fmtError("CreateJob", err)
	}
	return nil
}

// CreateSteps implements ingest.JobStore: one pending row per
// ingest.StepSequence step, in sequence order, inside one transaction.
func (s *SQLiteStore) CreateSteps(ctx context.Context, jobID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmtError("CreateSteps.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, step := range ingest.StepSequence {
		if _, err := tx.ExecContext(ctx, createIngestStepSQL, jobID, step); err != nil {
			return fmtError("CreateSteps("+step+")", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmtError("CreateSteps.Commit", err)
	}
	return nil
}

// UpdateJob implements ingest.JobStore. now is caller-set (RFC3339Nano
// stored verbatim).
func (s *SQLiteStore) UpdateJob(ctx context.Context, jobID, status, errText string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, updateIngestJobSQL,
		status,
		errText,
		now.UTC().Format(time.RFC3339Nano),
		jobID,
	)
	if err != nil {
		return fmtError("UpdateJob", err)
	}
	return nil
}

// SetJobDocInfo implements ingest.JobStore.
func (s *SQLiteStore) SetJobDocInfo(ctx context.Context, jobID, docID, parserName, sourcePath string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, setJobDocInfoSQL,
		docID,
		parserName,
		sourcePath,
		now.UTC().Format(time.RFC3339Nano),
		jobID,
	)
	if err != nil {
		return fmtError("SetJobDocInfo", err)
	}
	return nil
}

// UpsertStep implements ingest.JobStore. The row's timestamps are stored
// verbatim — the runner formats them.
func (s *SQLiteStore) UpsertStep(ctx context.Context, r ingest.StepRow) error {
	_, err := s.db.ExecContext(ctx, upsertIngestStepSQL,
		r.JobID,
		r.Step,
		r.Status,
		r.Attempt,
		r.StartedAt,
		r.CompletedAt,
		r.Error,
		r.ArtifactRef,
	)
	if err != nil {
		return fmtError("UpsertStep", err)
	}
	return nil
}

// ListSteps implements ingest.JobStore, ordered by ingest.StepSequence
// position (unknown step names sort last, in SQL name order among
// themselves).
func (s *SQLiteStore) ListSteps(ctx context.Context, jobID string) ([]ingest.StepRow, error) {
	rows, err := s.db.QueryContext(ctx, listIngestStepsSQL, jobID)
	if err != nil {
		return nil, fmtError("ListSteps", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]ingest.StepRow, 0, len(ingest.StepSequence))
	for rows.Next() {
		var r ingest.StepRow
		if err := rows.Scan(
			&r.JobID,
			&r.Step,
			&r.Status,
			&r.Attempt,
			&r.StartedAt,
			&r.CompletedAt,
			&r.Error,
			&r.ArtifactRef,
		); err != nil {
			return nil, fmtError("ListSteps.Scan", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListSteps.Rows", err)
	}
	sortIngestSteps(out)
	return out, nil
}

// sortIngestSteps orders rows by StepSequence index (stable, so unknown
// names keep their SQL order after the known ones).
func sortIngestSteps(rows []ingest.StepRow) {
	// Insertion sort: 8 rows, stable, no imports.
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && ingest.StepIndex(rows[j].Step) < ingest.StepIndex(rows[j-1].Step); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// GetJob implements ingest.JobStore.
func (s *SQLiteStore) GetJob(ctx context.Context, jobID string) (ingest.Job, error) {
	return scanIngestJob(s.db.QueryRowContext(ctx, getIngestJobSQL, jobID))
}

// GetJobBySHA256 implements ingest.JobStore.
func (s *SQLiteStore) GetJobBySHA256(ctx context.Context, slug, sha256 string) (ingest.Job, error) {
	return scanIngestJob(s.db.QueryRowContext(ctx, getIngestJobBySHASQL, slug, sha256))
}

// scanIngestJob scans one job row (positional order per jobColumns).
func scanIngestJob(row *sql.Row) (ingest.Job, error) {
	var j ingest.Job
	err := row.Scan(
		&j.JobID,
		&j.Slug,
		&j.Filename,
		&j.SHA256,
		&j.Mime,
		&j.FileSize,
		&j.DocID,
		&j.SourcePath,
		&j.ParserName,
		&j.Status,
		&j.Error,
		&j.CreatedAt,
		&j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ingest.Job{}, ErrNoIngestJob
		}
		return ingest.Job{}, fmtError("scanIngestJob", err)
	}
	return j, nil
}

// ClaimJob implements ingest.JobStore: the atomic queued|partial → running
// CAS. RowsAffected == 1 means this caller owns the job.
func (s *SQLiteStore) ClaimJob(ctx context.Context, jobID string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, claimIngestJobSQL,
		now.UTC().Format(time.RFC3339Nano),
		jobID,
	)
	if err != nil {
		return false, fmtError("ClaimJob", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmtError("ClaimJob.RowsAffected", err)
	}
	return n == 1, nil
}

// claimRetryIngestJobSQL — the retry-path CAS: FAILED or partial jobs
// only (queued/running have an active owner; done means the content is
// already in). One row matched = the retry owns the job from this
// instant; zero = a concurrent retry beat the caller (-> 409).
const claimRetryIngestJobSQL = `
UPDATE ingest_jobs
SET status = 'running', updated_at = ?
WHERE job_id = ? AND status IN ('failed', 'partial')
`

// staleStepSQL re-points one step at 'stale' with the supersede reason.
const staleStepSQL = `
UPDATE ingest_steps
SET status = 'stale', error = ?, completed_at = ?
WHERE job_id = ? AND step = ?
`

// failRunningStepSQL lands a running straggler (crash residue) at failed
// BEFORE staling it — a stale row whose previous state was running would
// lose the interruption fact; the SC 1 timeline shows the interrupt AND
// the supersede.
const failRunningStepSQL = `
UPDATE ingest_steps
SET status = 'failed', error = ?, completed_at = ?
WHERE job_id = ? AND step = ? AND status = 'running'
`

// ClaimJobRetry (54-03) — see ingest.JobStore for the contract.
func (s *SQLiteStore) ClaimJobRetry(ctx context.Context, jobID string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, claimRetryIngestJobSQL,
		now.UTC().Format(time.RFC3339Nano),
		jobID,
	)
	if err != nil {
		return false, fmtError("ClaimJobRetry", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmtError("ClaimJobRetry.RowsAffected", err)
	}
	return n == 1, nil
}

// CascadeStale (54-03) — see ingest.JobStore for the contract. Step
// ordering resolves through ingest.StepSequence (the app-level sequence
// is the source of truth; the table carries no order column). The whole
// cascade runs in one transaction: a partial stale on a retry request is
// worse than a rejected one (the retry would re-run steps whose
// downstream state is half-voided).
func (s *SQLiteStore) CascadeStale(ctx context.Context, jobID, fromStep string, now time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmtError("CascadeStale.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()

	nowS := now.UTC().Format(time.RFC3339Nano)
	const supersede = "superseded by upstream retry"
	staled := int64(0)

	after := false
	for _, name := range ingest.StepSequence {
		if name == fromStep {
			after = true
			continue
		}
		if !after {
			continue
		}
		// Running straggler first (crash residue) — failed THEN stale so
		// the timeline shows both facts.
		res, err := tx.ExecContext(ctx, failRunningStepSQL, supersede, nowS, jobID, name)
		if err != nil {
			return 0, fmtError("CascadeStale.failRunning("+name+")", err)
		}
		_ = res
		res, err = tx.ExecContext(ctx, staleStepSQL, supersede, nowS, jobID, name)
		if err != nil {
			return 0, fmtError("CascadeStale.stale("+name+")", err)
		}
		if n, err := res.RowsAffected(); err == nil {
			staled += n
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmtError("CascadeStale.Commit", err)
	}
	return staled, nil
}
