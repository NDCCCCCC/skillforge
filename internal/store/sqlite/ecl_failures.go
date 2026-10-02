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

// Phase 39 / Plan 39-05 (ONT-12 / D-DL-01) — the SQLite implementation of
// the ecl.FailureStore dead-letter ledger, backed by the ecl_failures DDL
// in schema.go. Unlike the INSERT-only audit tables, status transitions
// are the POINT of this table (pending → dead → resolved), so the single
// write path is an upsert; the mutable-status posture is sanctioned by
// D-DL-01 (not an audit table — see the DDL comment in schema.go).
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
)

// Compile-time conformance check (D11 contract shape): *SQLiteStore
// satisfies the narrow ecl.FailureStore seam the manager persists through.
var _ ecl.FailureStore = (*SQLiteStore)(nil)

// ErrNoFailureRecord is the sentinel wrapped by GetFailure for unknown
// notes; callers test with errors.Is. Exported so the ecl manager can
// distinguish a missing row from a store I/O error (IN-04).
var ErrNoFailureRecord = errors.New("ecl: no failure record")

// upsertFailureSQL is the single write path for ecl_failures. Status
// transitions are the point of the table, so ON CONFLICT(note_id) updates
// every mutable column; note_id is the PK and never changes.
const upsertFailureSQL = `
INSERT INTO ecl_failures (note_id, stage, attempt_count, last_error, status, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(note_id) DO UPDATE SET
    stage         = excluded.stage,
    attempt_count = excluded.attempt_count,
    last_error    = excluded.last_error,
    status        = excluded.status,
    updated_at    = excluded.updated_at
`

// failureColumns is the projection shared by the read paths (positional
// Scan order — keep in sync with the DDL).
const failureColumns = `note_id, stage, attempt_count, last_error, status, updated_at`

// listFailuresSQL returns every row, newest-updated first (the admin
// failures list ordering).
const listFailuresSQL = `
SELECT ` + failureColumns + `
FROM ecl_failures
ORDER BY updated_at DESC
`

// getFailureSQL is the single-note lookup RetryNote gates on.
const getFailureSQL = `
SELECT ` + failureColumns + `
FROM ecl_failures
WHERE note_id = ?
`

// UpsertFailure implements ecl.FailureStore. All values are parameterized
// (T-3-SQLI); updated_at is stored verbatim — the manager sets the
// RFC3339 UTC string so retry timing stays testable.
func (s *SQLiteStore) UpsertFailure(f ecl.Failure) error {
	_, err := s.db.Exec(upsertFailureSQL,
		f.NoteID,
		f.Stage,
		f.AttemptCount,
		f.LastError,
		f.Status,
		f.UpdatedAt,
	)
	if err != nil {
		return fmtError("UpsertFailure", err)
	}
	return nil
}

// ListFailures implements ecl.FailureStore.
func (s *SQLiteStore) ListFailures() ([]ecl.Failure, error) {
	rows, err := s.db.Query(listFailuresSQL)
	if err != nil {
		return nil, fmtError("ListFailures", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]ecl.Failure, 0)
	for rows.Next() {
		var f ecl.Failure
		if err := rows.Scan(
			&f.NoteID,
			&f.Stage,
			&f.AttemptCount,
			&f.LastError,
			&f.Status,
			&f.UpdatedAt,
		); err != nil {
			return nil, fmtError("ListFailures.Scan", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListFailures", err)
	}
	return out, nil
}

// GetFailure implements ecl.FailureStore. Unknown notes return a wrapped
// ErrNoFailureRecord (the manager maps any miss to its ErrUnknownNote
// sentinel — the ledger is the source of truth for retry eligibility).
func (s *SQLiteStore) GetFailure(noteID string) (ecl.Failure, error) {
	var f ecl.Failure
	err := s.db.QueryRow(getFailureSQL, noteID).Scan(
		&f.NoteID,
		&f.Stage,
		&f.AttemptCount,
		&f.LastError,
		&f.Status,
		&f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ecl.Failure{}, fmt.Errorf("%w for note %s", ErrNoFailureRecord, noteID)
		}
		return ecl.Failure{}, fmtError("GetFailure", err)
	}
	return f, nil
}

// IsNotFound implements ecl.FailureStore. It returns true when err wraps
// ErrNoFailureRecord — i.e., the ledger has no row for the requested note.
// Any other error (SQLite I/O failures) returns false, allowing the caller
// to distinguish "unknown note" (→ 404) from "store unavailable" (→ 500).
func (s *SQLiteStore) IsNotFound(err error) bool {
	return errors.Is(err, ErrNoFailureRecord)
}
