// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     You may obtain a copy of the License at
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
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// getSessionSQL selects the message history for one session, oldest-
// first. The WHERE clause on session_id IS the SC-9 / AGNT-05 cross-
// session isolation enforcement — no row from another session can ever
// satisfy it.
const getSessionSQL = `
SELECT role, content, ts
FROM session_turn
WHERE session_id = ?
ORDER BY ts ASC
`

// GetSession returns the message history for sessionID in chronological
// order. Returns an empty (non-nil) slice for an unknown session_id;
// callers treat empty as "start fresh".
func (s *SQLiteStore) GetSession(ctx context.Context, sessionID string) ([]store.SessionMessage, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if sessionID == "" {
		return nil, errEmptySessionID
	}
	rows, err := s.db.QueryContext(ctx, getSessionSQL, sessionID)
	if err != nil {
		return nil, fmtError("GetSession", err)
	}
	defer rows.Close()

	out := []store.SessionMessage{}
	for rows.Next() {
		var m store.SessionMessage
		if err := rows.Scan(&m.Role, &m.Content, &m.Ts); err != nil {
			return nil, fmtError("GetSession.Scan", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("GetSession.Rows", err)
	}
	return out, nil
}

// appendTurnSQL inserts one turn-half. The UNIQUE(session_id, role, ts)
// constraint makes a replayed AppendTurn a no-op via ON CONFLICT DO
// NOTHING — the idempotency mechanism (TestAppendTurnIdempotent).
const appendTurnSQL = `
INSERT INTO session_turn (session_id, role, content, ts)
VALUES (?, ?, ?, ?)
ON CONFLICT(session_id, role, ts) DO NOTHING
`

// AppendTurn atomically appends a (user, assistant) turn pair for
// sessionID. Idempotent on (sessionID, user.Ts, assistant.Ts): the
// UNIQUE(session_id, role, ts) constraint + ON CONFLICT DO NOTHING
// means a replayed call is a no-op (TestAppendTurnIdempotent locks
// this). Both rows are written in a single transaction so a partial
// failure cannot leave an orphaned user turn without its assistant
// reply (or vice versa).
func (s *SQLiteStore) AppendTurn(ctx context.Context, sessionID string, user, assistant store.SessionMessage) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	if sessionID == "" {
		return errEmptySessionID
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmtError("AppendTurn.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }() // safe: no-op once committed

	if err := appendTurnHalf(ctx, tx, sessionID, user); err != nil {
		return err
	}
	if err := appendTurnHalf(ctx, tx, sessionID, assistant); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmtError("AppendTurn.Commit", err)
	}
	return nil
}

// appendTurnHalf inserts one SessionMessage within an existing
// transaction. The role CHECK constraint ("user" | "assistant") is the
// last line of defense against a malformed SessionMessage.
func appendTurnHalf(ctx context.Context, tx *sql.Tx, sessionID string, m store.SessionMessage) error {
	role := m.Role
	if role == "" {
		// Fall back to a safe default rather than tripping the CHECK
		// constraint — but the caller is expected to set Role explicitly.
		role = "user"
	}
	ts := m.Ts
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	if _, err := tx.ExecContext(ctx, appendTurnSQL, sessionID, role, m.Content, ts.UTC()); err != nil {
		return fmtError("AppendTurn.Exec", err)
	}
	return nil
}
