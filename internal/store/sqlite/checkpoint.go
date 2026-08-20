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
	"database/sql"
	"errors"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
)

// Compile-time check that *SQLiteStore satisfies compose.CheckPointStore
// AND adk.CheckPointStore (they alias the same core interface at
// eino@v0.9.14/internal/core/interrupt.go:27-30). The single
// compile-time assertion locks both consumption sites; interface drift
// fails the build here, not at runtime — the D18 contract.
//
// The second assertion locks adk.CheckPointDeleter (D11-C2 — the
// Phase 11 expire path: when resume detects a skill_version OR
// prompt_hash mismatch, the chatHandler calls store.Delete(ctx,
// checkPointID) and returns state=\"expired\" to the FE).
var (
	_ compose.CheckPointStore = (*SQLiteStore)(nil)
	_ adk.CheckPointStore     = (*SQLiteStore)(nil)
	_ adk.CheckPointDeleter   = (*SQLiteStore)(nil)
)

// getCheckpointSQL reads one checkpoint payload by id. expires_at is
// deliberately NOT filtered this phase: nothing writes it yet (every
// row stores NULL), and all-NULL rows are indistinguishable from having
// no filter. Phase 11 adds the lifecycle predicate when it starts
// writing the column.
const getCheckpointSQL = `SELECT payload FROM checkpoint WHERE id = ?`

// setCheckpointSQL upserts one checkpoint. The ON CONFLICT(id) DO
// UPDATE arm makes a re-interrupted run replace its own checkpoint
// instead of resurrecting a stale one (TestCheckpointUpsUpdateOverwrites).
// Both values are `?` placeholders — checkPointID never touches string
// concatenation (T-8-04 / T-3-SQLI posture).
const setCheckpointSQL = `
INSERT INTO checkpoint (id, payload)
VALUES (?, ?)
ON CONFLICT(id) DO UPDATE SET payload = excluded.payload
`

// deleteCheckpointSQL is the D11-C2 expire-path DELETE. Plan 11-01
// adds the implementation + the adk.CheckPointDeleter compile-time
// assertion; the 11-04 expire-path (when resume detects a
// skill_version OR prompt_hash mismatch) calls
// store.Delete(ctx, checkPointID). Delete is idempotent: deleting a
// non-existent id returns nil (sql.Exec returns nil for 0 rows
// affected — the store treats missing rows as a no-op).
const deleteCheckpointSQL = `DELETE FROM checkpoint WHERE id = ?`

// Get returns the checkpoint payload stored under checkPointID. The
// store treats the payload as opaque bytes — it never parses or
// interprets checkpoint content (resume-data validation is is
// HITL territory). An unknown id returns (nil, false, nil): existed
// is a tri-state answer, NOT an error — the first run of any resume
// path legitimately finds no checkpoint (Pitfall 4 converse).
func (s *SQLiteStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	if s.isClosed() {
		return nil, false, sql.ErrConnDone
	}
	var payload []byte
	if err := s.db.QueryRowContext(ctx, getCheckpointSQL, checkPointID).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmtError("CheckpointGet", err)
	}
	return payload, true, nil
}

// Set stores checkPoint under checkPointID, overwriting any previous
// payload for the same id (upsert). The payload crosses the eino
// serialization boundary as bytes; SQLite stores it as a BLOB and
// returns it byte-identically — TestCheckpointRoundTrip and
// TestCheckpointSurvivesReopen lock this, including non-UTF8 bytes and
// the Close-then-reopen ("process restart") readback that an in-memory
// store cannot provide.
func (s *SQLiteStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	if _, err := s.db.ExecContext(ctx, setCheckpointSQL, checkPointID, checkPoint); err != nil {
		return fmtError("CheckpointSet", err)
	}
	return nil
}

// Delete removes the checkpoint stored under checkPointID (D11-C2
// expire path). Plan 11-01 ships the implementation; 11-04 wires the
// actual expire detection in the chatHandler boundary.
//
// The audit row (action_pending_approval) is NOT deleted — T-3-AUDIT
// INSERT-only (Phase 11 §1b Critical Failure Mode 3). Delete is
// idempotent: deleting a non-existent id is a no-op (sql.Exec
// returns nil for 0 rows affected). The function honors the
// store-wide isClosed() guard so a Delete on a closed store returns
// sql.ErrConnDone — the same disposition as Get / Set.
func (s *SQLiteStore) Delete(ctx context.Context, checkPointID string) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	if _, err := s.db.ExecContext(ctx, deleteCheckpointSQL, checkPointID); err != nil {
		return fmtError("CheckpointDelete", err)
	}
	return nil
}
