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

// Phase 29 / Plan 29-02 — feedback INSERT-only storage tests. Lives in
// internal/store/sqlite (white-box) because it asserts the exact DDL
// shape, column order, index set, and additive-migration behavior of the
// feedback family (feedback / feedback_chunk_refs /
// feedback_dispositions), plus (Task 2) behavior semantics and a
// source-grep structural lock on feedback.go itself.
package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedFeedbackColumns mirrors the CREATE TABLE DDL in schema.go —
// TestFeedbackSchema asserts PRAGMA table_info returns EXACTLY this name
// sequence in this order (column-order drift would silently break every
// positional Scan in feedback.go).
var expectedFeedbackColumns = []string{
	"id", "ts", "session_id", "turn_id", "verdict", "comment", "skill_id", "chunk_ids",
}

// expectedFeedbackChunkRefColumns mirrors the feedback_chunk_refs DDL.
var expectedFeedbackChunkRefColumns = []string{
	"feedback_id", "chunk_id", "kb_slug",
}

// expectedFeedbackDispositionColumns mirrors the feedback_dispositions DDL.
var expectedFeedbackDispositionColumns = []string{
	"id", "ts", "chunk_id", "disposition", "note", "actor",
}

// feedbackIndexes are the five feedback-family read-path indexes created
// at the migrate() tail (schema.go D14-E1 convention).
var feedbackIndexes = []string{
	"idx_feedback_session",
	"idx_feedback_ts",
	"idx_feedback_skill",
	"idx_refs_chunk",
	"idx_dispositions_chunk",
}

// requireColumnsInOrder asserts PRAGMA table_info(table) returns exactly
// the expected column names, in order.
func requireColumnsInOrder(t *testing.T, s *SQLiteStore, table string, expected []string) {
	t.Helper()
	ctx := context.Background()
	var gotNames []string
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	require.NoError(t, err)
	for rows.Next() {
		var (
			cid          int
			name, ctype  string
			notnull, pk  int
			defaultValue sql.NullString
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &defaultValue, &pk))
		gotNames = append(gotNames, name)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, expected, gotNames,
		"%s must carry exactly the DDL columns in DDL order", table)
}

// requireIndexExists asserts sqlite_master holds exactly one index with
// the given name (type='index').
func requireIndexExists(t *testing.T, s *SQLiteStore, idx string) {
	t.Helper()
	ctx := context.Background()
	var idxCount int
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&idxCount))
	assert.Equal(t, 1, idxCount, "%s must exist", idx)
}

// requireTableSQLContains asserts the stored CREATE statement of table
// contains the given fragment — the structural lock for the idempotency
// UNIQUE key and the CHECK closed-vocabulary backstops.
func requireTableSQLContains(t *testing.T, s *SQLiteStore, table, fragment string) {
	t.Helper()
	ctx := context.Background()
	var ddl string
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&ddl))
	assert.Contains(t, ddl, fragment,
		"%s DDL must contain %q (idempotency key / CHECK vocabulary is storage truth)", table, fragment)
}

// TestFeedbackSchema locks the feedback family schema contract:
//
//  1. A fresh file-backed DB carries exactly the DDL columns in DDL
//     order for all three feedback-family tables, plus the five
//     migrate()-tail indexes, plus the UNIQUE(session_id, turn_id)
//     idempotency key and both CHECK vocabularies.
//  2. migrate() is idempotent — re-running it on the migrated DB is a
//     no-op.
//  3. The legacy upgrade path: a pre-Phase-29 deployment (datasets
//     registry WITHOUT any feedback table) gains the tables additively
//     via New() and its existing rows survive untouched.
func TestFeedbackSchema(t *testing.T) {
	ctx := context.Background()

	// --- Act 1: fresh file-backed DB ---
	s, err := New(ctx, Config{DSN: filepath.Join(t.TempDir(), "feedback-ddl.db")})
	require.NoError(t, err, "fresh file DB must open via New()")

	requireColumnsInOrder(t, s, "feedback", expectedFeedbackColumns)
	requireColumnsInOrder(t, s, "feedback_chunk_refs", expectedFeedbackChunkRefColumns)
	requireColumnsInOrder(t, s, "feedback_dispositions", expectedFeedbackDispositionColumns)

	for _, idx := range feedbackIndexes {
		requireIndexExists(t, s, idx)
	}

	// Idempotency key: one vote per (session, turn) is a DB constraint,
	// not an application convention (D-29-02).
	requireTableSQLContains(t, s, "feedback", "UNIQUE(session_id, turn_id)")
	// Closed verdict vocabulary is storage-layer backstopped.
	requireTableSQLContains(t, s, "feedback",
		"CHECK (verdict IN ('helpful','unhelpful','outdated'))")

	// --- Act 2: migrate() idempotency on the already-migrated DB ---
	require.NoError(t, s.migrate(ctx), "re-running migrate must be a no-op")
	require.NoError(t, s.Close())

	// --- Act 3: legacy upgrade path (pre-Phase-29 DB without feedback) ---
	dsn := filepath.Join(t.TempDir(), "legacy-feedback-upgrade.db")
	seedLegacyDatasetsDB(t, dsn)

	legacy, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "pre-Phase-29 DB must upgrade via New() (additive feedback family)")
	defer func() { _ = legacy.Close() }()

	requireColumnsInOrder(t, legacy, "feedback", expectedFeedbackColumns)
	requireColumnsInOrder(t, legacy, "feedback_chunk_refs", expectedFeedbackChunkRefColumns)
	requireColumnsInOrder(t, legacy, "feedback_dispositions", expectedFeedbackDispositionColumns)
	for _, idx := range feedbackIndexes {
		requireIndexExists(t, legacy, idx)
	}

	// Additive discipline: the pre-existing datasets rows survive untouched.
	var dsCount int
	require.NoError(t, legacy.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datasets`).Scan(&dsCount))
	assert.Equal(t, 2, dsCount, "legacy datasets rows must survive the feedback migration")
}
