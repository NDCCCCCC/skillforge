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
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// listSessionsSQL derives the session roster from session_turn, one row
// per distinct session_id, newest-activity-first. Both the first user
// turn (derived title) and the newest turn ts are correlated subselects
// with explicit ORDER BY + LIMIT 1 — NOT MIN()/MAX() aggregates, whose
// result type loses the column's DATETIME affinity and fails to Scan
// into time.Time. session_meta is LEFT JOINed so the rename override
// surfaces as Title ('' when absent). No audit table involvement: the
// immutable trail stays decoupled from the user-facing roster.
const listSessionsSQL = `
SELECT st.session_id,
       COALESCE((
           SELECT u.content FROM session_turn u
           WHERE u.session_id = st.session_id AND u.role = 'user'
           ORDER BY u.ts ASC LIMIT 1
       ), '') AS first_query,
       (SELECT t2.ts FROM session_turn t2
        WHERE t2.session_id = st.session_id
        ORDER BY t2.ts DESC LIMIT 1) AS last_ts,
       COALESCE(sm.title, '') AS title
FROM session_turn st
LEFT JOIN session_meta sm ON sm.id = st.session_id
GROUP BY st.session_id
ORDER BY last_ts DESC
`

// titleLimit caps the derived title length in RUNES (Chinese-first
// product — rune count, not bytes).
const titleLimit = 40

// ListSessions returns the derived session roster, newest-first. An
// empty database returns an empty (non-nil) slice.
func (s *SQLiteStore) ListSessions(ctx context.Context) ([]store.SessionSummary, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, listSessionsSQL)
	if err != nil {
		return nil, fmtError("ListSessions", err)
	}
	defer rows.Close()

	out := []store.SessionSummary{}
	for rows.Next() {
		var (
			id            string
			firstQuery    string
			lastTs        time.Time
			titleOverride string
		)
		if err := rows.Scan(&id, &firstQuery, &lastTs, &titleOverride); err != nil {
			return nil, fmtError("ListSessions.Scan", err)
		}
		out = append(out, store.SessionSummary{
			ID:           id,
			DerivedTitle: truncateRunes(firstQuery, titleLimit),
			Title:        titleOverride,
			LastActive:   lastTs.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListSessions.Rows", err)
	}
	return out, nil
}

// truncateRunes caps s at n runes with an ellipsis marker when clipped.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// renameSessionSQL upserts the title override. ON CONFLICT DO UPDATE is
// the checkpoint-upsert pattern (checkpoint.go) — the meta row is the
// single write surface for rename, session_turn stays authoritative for
// existence/activity.
const renameSessionSQL = `
INSERT INTO session_meta (id, title) VALUES (?, ?)
ON CONFLICT(id) DO UPDATE SET title = excluded.title
`

// RenameSession upserts the session_meta title override. Legal before
// any turns exist (pre-seeding a rename); the roster join picks the
// override up as soon as turns arrive.
func (s *SQLiteStore) RenameSession(ctx context.Context, sessionID, title string) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	if sessionID == "" {
		return errEmptySessionID
	}
	if _, err := s.db.ExecContext(ctx, renameSessionSQL, sessionID, title); err != nil {
		return fmtError("RenameSession", err)
	}
	return nil
}

// deleteSessionTurnsSQL / deleteSessionMetaSQL remove the user-owned
// halves of a session in ONE transaction. Audit rows are deliberately
// absent — T-3-AUDIT forbids a runtime audit DELETE (the immutable
// trail survives user-visible session deletion).
const deleteSessionTurnsSQL = `DELETE FROM session_turn WHERE session_id = ?`
const deleteSessionMetaSQL = `DELETE FROM session_meta WHERE id = ?`

// DeleteSessionTurns removes sessionID's turns and meta row atomically.
// Idempotent: an unknown sessionID deletes zero rows and still succeeds.
func (s *SQLiteStore) DeleteSessionTurns(ctx context.Context, sessionID string) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	if sessionID == "" {
		return errEmptySessionID
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmtError("DeleteSessionTurns.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }() // safe: no-op once committed

	if _, err := tx.ExecContext(ctx, deleteSessionTurnsSQL, sessionID); err != nil {
		return fmtError("DeleteSessionTurns.Exec.turns", err)
	}
	if _, err := tx.ExecContext(ctx, deleteSessionMetaSQL, sessionID); err != nil {
		return fmtError("DeleteSessionTurns.Exec.meta", err)
	}
	if err := tx.Commit(); err != nil {
		return fmtError("DeleteSessionTurns.Commit", err)
	}
	return nil
}
