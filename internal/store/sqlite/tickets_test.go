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

// Phase 29 / Plan 29-02 — tickets INSERT-only storage tests. Lives in
// internal/store/sqlite (white-box) because it asserts the exact DDL
// shape, column order, index set, and additive-migration behavior of the
// tickets table, plus (Task 3) behavior semantics and a source-grep
// structural lock on tickets.go itself.
package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// expectedTicketColumns mirrors the CREATE TABLE DDL in schema.go —
// TestTicketsSchema asserts PRAGMA table_info returns EXACTLY this name
// sequence in this order (column-order drift would silently break every
// positional Scan in tickets.go).
var expectedTicketColumns = []string{
	"id", "ts", "session_id", "turn_id", "description",
	"chunk_ids", "skill_id", "provider", "dispatch_status",
}

// TestTicketsSchema locks the tickets schema contract:
//
//  1. A fresh file-backed DB carries exactly the 9 DDL columns in DDL
//     order, the migrate()-tail session index, the UNIQUE(session_id,
//     turn_id) idempotency key (D-29-14 — one ticket per message), and
//     the dispatch_status CHECK vocabulary.
//  2. migrate() is idempotent — re-running it on the migrated DB is a
//     no-op.
//  3. The legacy upgrade path: a pre-Phase-29 deployment gains the
//     table additively via New() and its existing rows survive.
func TestTicketsSchema(t *testing.T) {
	ctx := context.Background()

	// --- Act 1: fresh file-backed DB ---
	s, err := New(ctx, Config{DSN: filepath.Join(t.TempDir(), "tickets-ddl.db")})
	require.NoError(t, err, "fresh file DB must open via New()")

	requireColumnsInOrder(t, s, "tickets", expectedTicketColumns)
	requireIndexExists(t, s, "idx_tickets_session")

	// Idempotency key: one ticket per (session, turn) is a DB
	// constraint, not an application convention (D-29-14).
	requireTableSQLContains(t, s, "tickets", "UNIQUE(session_id, turn_id)")
	// Closed dispatch_status vocabulary is storage-layer backstopped.
	requireTableSQLContains(t, s, "tickets",
		"CHECK (dispatch_status IN ('none','queued'))")

	// --- Act 2: migrate() idempotency on the already-migrated DB ---
	require.NoError(t, s.migrate(ctx), "re-running migrate must be a no-op")
	require.NoError(t, s.Close())

	// --- Act 3: legacy upgrade path (pre-Phase-29 DB without tickets) ---
	dsn := filepath.Join(t.TempDir(), "legacy-tickets-upgrade.db")
	seedLegacyDatasetsDB(t, dsn)

	legacy, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "pre-Phase-29 DB must upgrade via New() (additive tickets)")
	defer func() { _ = legacy.Close() }()

	requireColumnsInOrder(t, legacy, "tickets", expectedTicketColumns)
	requireIndexExists(t, legacy, "idx_tickets_session")

	// Additive discipline: the pre-existing datasets rows survive untouched.
	var dsCount int
	require.NoError(t, legacy.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datasets`).Scan(&dsCount))
	assert.Equal(t, 2, dsCount, "legacy datasets rows must survive the tickets migration")
}

// ---------------------------------------------------------------------------
// Task 3 — behavior matrix + TestTicketsInsertOnly structural lock.
// ---------------------------------------------------------------------------

// ticketRowCount is a white-box row counter for the idempotency
// assertions (no second store API is added just for tests).
func ticketRowCount(t *testing.T, s *SQLiteStore) int {
	t.Helper()
	ctx := context.Background()
	var n int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets`).Scan(&n))
	return n
}

// TestTicketIdempotentReplay locks D-29-14: one ticket per message. A
// replayed (session, turn) request is swallowed by ON CONFLICT DO
// NOTHING, reports inserted=false, and the read-back is the FIRST row
// with its original id — the existing acceptance number, never a second
// ticket.
func TestTicketIdempotentReplay(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	first := store.TicketRow{
		TS:          time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		SessionID:   "sess-ticket-1",
		TurnID:      9,
		Description: "差旅报销口径与制度 5.2 冲突，请核实",
		ChunkIDs:    "201,202",
		SkillID:     "compliance",
		Provider:    "none",
		DispatchStatus: "none",
	}
	inserted, err := s.PutTicket(ctx, first)
	require.NoError(t, err)
	require.True(t, inserted, "fresh ticket must insert")

	stored, ok, err := s.TicketByTurn(ctx, first.SessionID, first.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, stored.ID, int64(0), "auto-increment id must be populated on reads")

	// Same key replay — swallowed, FIRST row survives.
	replay := first
	replay.Description = "重复点击"
	replay.Provider = "webhook"
	replay.DispatchStatus = "queued"
	inserted, err = s.PutTicket(ctx, replay)
	require.NoError(t, err)
	assert.False(t, inserted, "replayed ticket must report inserted=false")

	reread, ok, err := s.TicketByTurn(ctx, first.SessionID, first.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, stored.ID, reread.ID, "row id (and thus the TKT-{id} number) must be unchanged")
	assert.Equal(t, 1, ticketRowCount(t, s), "replay must not add a row")

	// Cross-session lookup must miss even for a turn id that exists in
	// another session (T-29-06 session binding).
	_, ok, err = s.TicketByTurn(ctx, "sess-ticket-other", first.TurnID)
	require.NoError(t, err)
	assert.False(t, ok)

	// Unknown key → ok=false, zero row.
	missing, ok, err := s.TicketByTurn(ctx, "sess-ticket-1", 424242)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Zero(t, missing.ID)
}

// TestTicketRoundTrip locks that every TicketRow column round-trips
// byte-for-byte — provider and dispatch_status exactly as written (the
// dispatch outcome is NEVER written back; it lives in an
// action_ticket_dispatch_* audit row instead, TKT-02 INSERT-only).
func TestTicketRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ts := time.Date(2026, 9, 2, 8, 15, 30, 500000000, time.UTC)
	row := store.TicketRow{
		TS:          ts,
		SessionID:   "sess-ticket-rt",
		TurnID:      77,
		Description: "多行描述\n第二行：需要 IT 服务台跟进",
		ChunkIDs:    "301,302",
		SkillID:     "compliance",
		Provider:    "webhook",
		DispatchStatus: "queued",
	}
	inserted, err := s.PutTicket(ctx, row)
	require.NoError(t, err)
	require.True(t, inserted)

	got, ok, err := s.TicketByTurn(ctx, row.SessionID, row.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, row.SessionID, got.SessionID)
	assert.Equal(t, row.TurnID, got.TurnID)
	assert.Equal(t, row.Description, got.Description, "description must round-trip byte-equal (multi-line + CJK)")
	assert.Equal(t, row.ChunkIDs, got.ChunkIDs)
	assert.Equal(t, row.SkillID, got.SkillID)
	assert.Equal(t, "webhook", got.Provider, "provider must round-trip as written")
	assert.Equal(t, "queued", got.DispatchStatus, "dispatch_status must round-trip as written")
	assert.True(t, row.TS.Equal(got.TS), "TS must round-trip: want %v got %v", row.TS, got.TS)
	assert.Greater(t, got.ID, int64(0))

	// Zero-value provider / dispatch_status normalize to the documented
	// 'none' defaults (a Go zero field is "", which the SQL DEFAULT never
	// sees because the INSERT names every column) — the none-provider
	// store-only receipt path.
	minimal := store.TicketRow{
		SessionID:   "sess-ticket-min",
		TurnID:      1,
		Description: "最简工单",
	}
	inserted, err = s.PutTicket(ctx, minimal)
	require.NoError(t, err)
	require.True(t, inserted)
	gotMin, ok, err := s.TicketByTurn(ctx, minimal.SessionID, minimal.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Empty(t, gotMin.SkillID)
	assert.Equal(t, "none", gotMin.Provider, "empty provider normalizes to the 'none' default")
	assert.Equal(t, "none", gotMin.DispatchStatus, "empty dispatch_status normalizes to the 'none' default")
}

// TestTicketsInsertOnly locks the tickets anti-tampering contract in the
// kpi_calls_test.go two-segment shape:
//
//  1. source-grep: tickets.go contains no row-mutation or row-removal
//     verb — the outbound dispatch result goes to an audit row, never a
//     write-back to the ticket row (TKT-02 INSERT-only posture; also the
//     reason the acceptance number is derived as "TKT-{row id}" by the
//     29-06 handler instead of being stored).
//  2. round-trip + idempotent replay.
func TestTicketsInsertOnly(t *testing.T) {
	ctx := context.Background()

	// --- Segment 1: source-grep structural lock ---
	src, err := os.ReadFile("tickets.go")
	require.NoError(t, err, "tickets.go must be readable from the package dir")
	source := string(src)
	assert.NotContains(t, source, "UPDATE ",
		"tickets.go must contain no row-mutation verb (TKT-02 INSERT-only; dispatch outcome rides an audit row)")
	assert.NotContains(t, source, "DELETE ",
		"tickets.go must contain no row-removal verb (TKT-02 INSERT-only)")

	// --- Segment 2: round-trip + idempotent replay ---
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	row := store.TicketRow{
		SessionID:      "sess-insertonly",
		TurnID:         5,
		Description:    "描述",
		ChunkIDs:       "1,2",
		SkillID:        "compliance",
		Provider:       "none",
		DispatchStatus: "none",
	}
	inserted, err := s.PutTicket(ctx, row)
	require.NoError(t, err)
	require.True(t, inserted)

	got, ok, err := s.TicketByTurn(ctx, row.SessionID, row.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, row.Description, got.Description)

	inserted, err = s.PutTicket(ctx, row)
	require.NoError(t, err)
	assert.False(t, inserted)

	reread, ok, err := s.TicketByTurn(ctx, row.SessionID, row.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, got.ID, reread.ID, "replay must read back the existing row, not a new one")
	assert.Equal(t, 1, ticketRowCount(t, s))

	// Closed store → meaningful error on both paths.
	require.NoError(t, s.Close())
	_, err = s.PutTicket(ctx, row)
	assert.ErrorIs(t, err, sql.ErrConnDone)
	_, _, err = s.TicketByTurn(ctx, row.SessionID, row.TurnID)
	assert.ErrorIs(t, err, sql.ErrConnDone)
}
