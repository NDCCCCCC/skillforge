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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
