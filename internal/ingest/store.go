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

package ingest

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is the sentinel the JobStore wraps for a missing job row;
// callers distinguish it from store I/O errors via JobStore.IsNotFound
// (the ecl.FailureStore / ErrNoFailureRecord precedent) — the GET endpoint
// maps the former to 404 and the latter to 500.
var ErrNotFound = errors.New("ingest: no such job")

// JobStore is the narrow persistence seam for the ingest state machine
// (the ecl.FailureStore precedent: a deliberately small interface the
// sqlite store implements and tests fake with a map — NOT part of the big
// store.Store, because ingest.Runner depends on exactly these operations
// and nothing else). Server handlers reach it only through the Runner;
// they never touch the store directly.
//
// Timestamp discipline: every RFC3339Nano field inside Job / StepRow is
// SET BY THE CALLER and stored verbatim (ecl_failures.updated_at
// precedent) — no SQL defaults, so timing is testable. Methods that
// produce a timestamp take `now` explicitly.
type JobStore interface {
	// CreateJob inserts the job row. The PK is the freshly minted job_id,
	// so a PK collision is a programming error and surfaces as a plain
	// INSERT failure (deliberately NOT INSERT OR REPLACE — a silently
	// replaced job row would erase the first job's state machine).
	CreateJob(ctx context.Context, j Job) error

	// CreateSteps seeds one pending row per StepSequence step for the
	// job (attempt 0, empty timestamps), stamped with now. Called once
	// at Enqueue.
	CreateSteps(ctx context.Context, jobID string, now time.Time) error

	// UpdateJob transitions the job row's (status, error) and stamps
	// updated_at = now. Unconditional (no CAS) — the only CAS-gated
	// transition is the claim below; every other transition is owned by
	// the single runner goroutine that holds the claim.
	UpdateJob(ctx context.Context, jobID, status, errText string, now time.Time) error

	// SetJobDocInfo records the parse step's job-row outputs: the 53-01
	// persist doc_id, the parser registry name, and the raw-bytes archive
	// path (54-03 retry's staging source). Stamps updated_at = now.
	SetJobDocInfo(ctx context.Context, jobID, docID, parserName, sourcePath string, now time.Time) error

	// UpsertStep writes one step row — the runner's per-step transition
	// write (INSERT ... ON CONFLICT(job_id, step) DO UPDATE). Status
	// transitions ARE the point of this table; attempt is cumulative.
	UpsertStep(ctx context.Context, r StepRow) error

	// ListSteps returns every step row of one job in StepSequence order.
	// Unknown step names sort last.
	ListSteps(ctx context.Context, jobID string) ([]StepRow, error)

	// GetJob returns the job row. A missing job wraps ErrNotFound.
	GetJob(ctx context.Context, jobID string) (Job, error)

	// GetJobBySHA256 returns the NEWEST job row for (slug, sha256) —
	// the extended-dedup data source (upload handler: audit rows ∨
	// non-failed jobs, so a re-upload during an in-flight job cannot
	// fork a duplicate job). A job with no matching row wraps ErrNotFound.
	GetJobBySHA256(ctx context.Context, slug, sha256 string) (Job, error)

	// ClaimJob is the runner's CAS: it transitions the job to running
	// ONLY when the current status is queued or partial (the re-claimable
	// states), reporting whether THIS caller won. A false return means
	// another owner holds the job (or it reached a terminal state) — the
	// caller must not proceed. Stamps updated_at = now on success.
	ClaimJob(ctx context.Context, jobID string, now time.Time) (bool, error)

	// ClaimJobRetry is the retry-path CAS (54-03): claim a FAILED or
	// partial job for re-execution. A concurrent second retry (or an
	// enqueue racing the retry) finds no matching row -> false -> 409.
	// Unlike ClaimJob this does NOT match queued/running: those jobs have
	// an active owner already.
	ClaimJobRetry(ctx context.Context, jobID string, now time.Time) (bool, error)

	// CascadeStale marks every step AFTER fromStep as stale (upstream
	// re-ran, downstream results are void — Dify re_segment semantics),
	// inside ONE transaction: a running straggler (crash residue) is
	// failed first ("superseded by upstream retry") then staled, so the
	// stale write is not rejected by a status guard. Returns the number
	// of rows staled.
	CascadeStale(ctx context.Context, jobID, fromStep string, now time.Time) (int64, error)

	// IsNotFound reports whether err wraps ErrNotFound — letting callers
	// distinguish "unknown job" (→ 404) from a store failure (→ 500).
	IsNotFound(err error) bool
}
