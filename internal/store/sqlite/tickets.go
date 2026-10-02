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

// Phase 29 / Plan 29-02 — INSERT-only ticket storage (TKT-02 / D-29-14).
// A structural mirror of feedback.go: PutTicket is the single append
// path and TicketByTurn the single read path — this file deliberately
// ships no row-mutation or row-removal verb at all (locked by
// TestTicketsInsertOnly's source-grep assertion). Consequences of that
// discipline, both load-bearing for the 29-06 handler:
//
//   - the outbound dispatch outcome NEVER lands here; it is recorded as
//     an INSERT-only action_ticket_dispatch_* audit row, so provider and
//     dispatch_status stay exactly as written at insert time;
//   - the acceptance number is derived as "TKT-{row id}" from the read
//     back TicketRow.ID — there is no number column to write back, the
//     AUTOINCREMENT id minted at insert time IS the local number source.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putTicketSQL is the single write path for a ticket request (TKT-02 /
// D-29-14). Column order mirrors the CREATE TABLE DDL in schema.go
// one-to-one; id is omitted (AUTOINCREMENT assigns it). The
// UNIQUE(session_id, turn_id) constraint is the one-ticket-per-message
// truth: a replayed request hits ON CONFLICT DO NOTHING, which PutTicket
// surfaces as inserted=false.
const putTicketSQL = `
INSERT INTO tickets (ts, session_id, turn_id, description, chunk_ids, skill_id, provider, dispatch_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id, turn_id) DO NOTHING
`

// PutTicket appends one INSERT-only ticket request row. RowsAffected()==0
// marks the replay path: the caller must then read the FIRST row back via
// TicketByTurn and surface its existing acceptance number ("TKT-{id}") —
// never create a second ticket for the same message (D-29-14; the FE
// "已建单" state is only a cache).
func (s *SQLiteStore) PutTicket(ctx context.Context, row store.TicketRow) (bool, error) {
	if s.isClosed() {
		return false, sql.ErrConnDone
	}
	if row.SessionID == "" {
		return false, errEmptySessionID
	}
	ts := row.TS
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	// The DDL DEFAULT 'none' on provider / dispatch_status never fires for
	// a Go caller: the INSERT names every column, and a zero-value Go
	// field is the empty string, not SQL NULL. Apply the same documented
	// defaults here (the appendTurnHalf role-fallback pattern) so a
	// half-populated TicketRow lands in the legal 'none' state instead of
	// tripping the CHECK constraint; any NON-empty invalid value still
	// fails closed at the CHECK.
	provider := row.Provider
	if provider == "" {
		provider = "none"
	}
	dispatchStatus := row.DispatchStatus
	if dispatchStatus == "" {
		dispatchStatus = "none"
	}
	res, err := s.db.ExecContext(ctx, putTicketSQL,
		ts,
		row.SessionID,
		row.TurnID,
		row.Description,
		row.ChunkIDs,
		row.SkillID,
		provider,
		dispatchStatus,
	)
	if err != nil {
		return false, fmtError("PutTicket", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmtError("PutTicket.RowsAffected", err)
	}
	return n > 0, nil
}

// ticketByTurnSQL reads one ticket request. The WHERE clause binds BOTH
// session_id and turn_id — the session_id term is the T-29-06
// cross-session isolation enforcement (a turn id alone never resolves).
const ticketByTurnSQL = `
SELECT id, ts, session_id, turn_id, description, chunk_ids, skill_id, provider, dispatch_status
FROM tickets
WHERE session_id = ? AND turn_id = ?
`

// TicketByTurn returns the ticket for (sessionID, turnID), or ok=false
// when that pair has no ticket. The acceptance number derives from the
// returned TicketRow.ID as "TKT-{id}" (29-06 handler convention).
func (s *SQLiteStore) TicketByTurn(ctx context.Context, sessionID string, turnID int64) (store.TicketRow, bool, error) {
	if s.isClosed() {
		return store.TicketRow{}, false, sql.ErrConnDone
	}
	if sessionID == "" {
		return store.TicketRow{}, false, errEmptySessionID
	}
	var r store.TicketRow
	err := s.db.QueryRowContext(ctx, ticketByTurnSQL, sessionID, turnID).
		Scan(&r.ID, &r.TS, &r.SessionID, &r.TurnID, &r.Description, &r.ChunkIDs,
			&r.SkillID, &r.Provider, &r.DispatchStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return store.TicketRow{}, false, nil
	}
	if err != nil {
		return store.TicketRow{}, false, fmtError("TicketByTurn", err)
	}
	return r, true, nil
}

// queuedTicketsSQL returns tickets with dispatch_status='queued' created before
// olderThan. Used by the WR-02 startup sweep: INSERT-only re-dispatch without
// touching the ticket row's immutable dispatch_status.
const queuedTicketsSQL = `
SELECT id, ts, session_id, turn_id, description, chunk_ids, skill_id, provider, dispatch_status
FROM tickets
WHERE dispatch_status = 'queued' AND ts < ?
ORDER BY ts ASC
`

// QueuedTickets implements store.Store.QueuedTickets.
// The INSERT-only discipline is load-bearing: the sweep re-dispatches through
// the provider (emitting action_ticket_dispatch_* audit rows) without ever
// UPDATE-ing the ticket row's dispatch_status.
func (s *SQLiteStore) QueuedTickets(ctx context.Context, olderThan time.Time) ([]*store.TicketRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, queuedTicketsSQL, olderThan)
	if err != nil {
		return nil, fmtError("QueuedTickets", err)
	}
	defer rows.Close()
	var out []*store.TicketRow
	for rows.Next() {
		var r store.TicketRow
		if err := rows.Scan(&r.ID, &r.TS, &r.SessionID, &r.TurnID, &r.Description,
			&r.ChunkIDs, &r.SkillID, &r.Provider, &r.DispatchStatus); err != nil {
			return nil, fmtError("QueuedTickets.Scan", err)
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}
