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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
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

// ---------------------------------------------------------------------------
// Task 2 — behavior matrix + TestFeedbackInsertOnly structural lock.
// ---------------------------------------------------------------------------

// mustPutFeedback writes one vote plus its chunk refs and returns the
// feedback row id.
func mustPutFeedback(t *testing.T, s *SQLiteStore, row store.FeedbackRow, refs []store.ChunkRef) int64 {
	t.Helper()
	ctx := context.Background()
	inserted, err := s.PutFeedback(ctx, row)
	require.NoError(t, err)
	require.True(t, inserted, "fresh vote (session=%s turn=%d) must insert", row.SessionID, row.TurnID)
	stored, ok, err := s.FeedbackByTurn(ctx, row.SessionID, row.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, s.PutFeedbackChunkRefs(ctx, stored.ID, refs))
	return stored.ID
}

// mustPutDisposition writes one disposition row and returns its id.
func mustPutDisposition(t *testing.T, s *SQLiteStore, row store.DispositionRow) int64 {
	t.Helper()
	id, err := s.PutDisposition(context.Background(), row)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))
	return id
}

// feedbackRowCount is a white-box row counter for the idempotency
// assertions (no second store API is added just for tests).
func feedbackRowCount(t *testing.T, s *SQLiteStore) int {
	t.Helper()
	ctx := context.Background()
	var n int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM feedback`).Scan(&n))
	return n
}

// TestFeedbackIdempotentReplay locks D-29-02: the DB constraint — not
// the FE lock state — is the one-vote-per-message truth. A replayed
// (session, turn) vote is swallowed by ON CONFLICT DO NOTHING, reports
// inserted=false, and the read-back row is the FIRST row (original
// verdict, unchanged row count) — never a second row.
func TestFeedbackIdempotentReplay(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	first := store.FeedbackRow{
		TS:        time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		SessionID: "sess-vote-1",
		TurnID:    7,
		Verdict:   "outdated",
		Comment:   "条款已修订",
		SkillID:   "compliance",
		ChunkIDs:  "11,12",
	}
	inserted, err := s.PutFeedback(ctx, first)
	require.NoError(t, err)
	require.True(t, inserted)

	// Same key, different payload — the replay must NOT win.
	replay := first
	replay.Verdict = "helpful"
	replay.Comment = "第二次点错了"
	replay.TS = time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	inserted, err = s.PutFeedback(ctx, replay)
	require.NoError(t, err)
	assert.False(t, inserted, "replayed vote must report inserted=false")

	// The surviving row is the FIRST one, and there is only one row.
	got, ok, err := s.FeedbackByTurn(ctx, "sess-vote-1", 7)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "outdated", got.Verdict, "read-back must be the first row's verdict")
	assert.Equal(t, "条款已修订", got.Comment)
	assert.Equal(t, 1, feedbackRowCount(t, s), "replay must not add a row")

	// Unknown key → ok=false, zero row.
	missing, ok, err := s.FeedbackByTurn(ctx, "sess-vote-1", 999)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Zero(t, missing.TurnID)

	// Cross-session lookup must miss even for a turn id that exists in
	// another session (T-29-06 session binding).
	other, ok, err := s.FeedbackByTurn(ctx, "sess-vote-other", 7)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Zero(t, other.ID)
}

// TestFeedbackChunkRefsIdempotent locks the refs normalization: replaying
// the same (feedback_id, chunk_id) pair is a no-op, never a second row.
func TestFeedbackChunkRefsIdempotent(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	id := mustPutFeedback(t, s, store.FeedbackRow{
		SessionID: "sess-refs",
		TurnID:    3,
		Verdict:   "helpful",
		SkillID:   "compliance",
	}, []store.ChunkRef{{ChunkID: 11, KBSlug: "kb-authority"}})

	// Same pair again — swallowed.
	require.NoError(t, s.PutFeedbackChunkRefs(ctx, id, []store.ChunkRef{{ChunkID: 11, KBSlug: "kb-authority"}}))
	// New chunk for the same vote — appended.
	require.NoError(t, s.PutFeedbackChunkRefs(ctx, id, []store.ChunkRef{{ChunkID: 12, KBSlug: "kb-draft"}}))

	var n int
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM feedback_chunk_refs WHERE feedback_id = ?`, id).Scan(&n))
	assert.Equal(t, 2, n, "unique (feedback_id, chunk_id) must hold exactly one row per pair")

	// Empty ref slice is a legal no-op.
	require.NoError(t, s.PutFeedbackChunkRefs(ctx, id, nil))
}

// TestPendingReviewChunks locks the DERIVED pending read model (D-29-08 +
// Open Q1 reopen semantics):
//
//   - aggregation is per chunk with report_count descending priority;
//   - comments round-trip with empty notes as empty-string placeholders;
//   - a disposition with ts >= the vote ts closes the chunk (latest
//     disposition wins);
//   - a NEW outdated report with ts > the disposition ts reopens it.
func TestPendingReviewChunks(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Chunk 101 reported twice (one with a note), chunk 202 once.
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), SessionID: "s1", TurnID: 1,
		Verdict: "outdated", Comment: "制度已改版", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 101, KBSlug: "kb-authority"}})
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC), SessionID: "s2", TurnID: 2,
		Verdict: "outdated", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 101, KBSlug: "kb-authority"}})
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC), SessionID: "s3", TurnID: 3,
		Verdict: "outdated", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 202, KBSlug: "kb-draft"}})

	// Non-outdated votes never enter the queue.
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), SessionID: "s4", TurnID: 4,
		Verdict: "helpful", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 303, KBSlug: "kb-personal"}})

	pending, err := s.PendingReviewChunks(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 2, "two distinct chunks are pending")

	// Priority: report_count DESC.
	a, b := pending[0], pending[1]
	assert.Equal(t, int64(101), a.ChunkID)
	assert.Equal(t, 2, a.ReportCount)
	assert.Equal(t, int64(202), b.ChunkID)
	assert.Equal(t, 1, b.ReportCount)

	assert.Equal(t, "kb-authority", a.KBSlug)
	assert.True(t, a.LastReportedAt.Equal(time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)),
		"last_reported_at must be the newest qualifying vote")

	require.Len(t, a.Comments, 2, "one placeholder per report, empty note included")
	var nonEmpty, empty int
	for _, c := range a.Comments {
		if c == "" {
			empty++
			continue
		}
		nonEmpty++
		assert.Equal(t, "制度已改版", c)
	}
	assert.Equal(t, 1, nonEmpty)
	assert.Equal(t, 1, empty, "an empty note must occupy an empty-string placeholder")
	assert.Empty(t, b.Comments[0], "single empty-note report yields one empty string")

	count, err := s.PendingReviewCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// --- Disposition closes the chunk (latest disposition wins) ---
	mustPutDisposition(t, s, store.DispositionRow{
		TS:          time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC),
		ChunkID:     101,
		Disposition: "updated",
		Note:        "已换新版制度",
		Actor:       "admin",
	})

	pending, err = s.PendingReviewChunks(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(202), pending[0].ChunkID, "disposition with ts >= vote ts closes the chunk")
	count, err = s.PendingReviewCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// --- A NEWER outdated report reopens the chunk (Open Q1) ---
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC), SessionID: "s5", TurnID: 5,
		Verdict: "outdated", Comment: "新批次又过时了", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 101, KBSlug: "kb-authority"}})

	pending, err = s.PendingReviewChunks(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	// Per-vote NOT EXISTS: the Sept-3 disposition already resolved the
	// two older reports, so only the NEW post-disposition report counts.
	// report_count therefore reads the size of the UNRESOLVED batch (1),
	// which is the correct priority signal (D-29-08) — not 3.
	assert.Equal(t, int64(101), pending[0].ChunkID, "reopened chunk leads on the newest last_reported_at")
	assert.Equal(t, 1, pending[0].ReportCount, "only the post-disposition reports are unresolved")
	assert.Equal(t, int64(202), pending[1].ChunkID)
	assert.Equal(t, 1, pending[1].ReportCount)

	// A disposition AFTER the newest report closes it again.
	mustPutDisposition(t, s, store.DispositionRow{
		TS:          time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC),
		ChunkID:     101,
		Disposition: "false_positive",
	})
	pending, err = s.PendingReviewChunks(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(202), pending[0].ChunkID)

	// Empty queue on a fresh store → empty (non-nil) slice + count 0.
	fresh, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = fresh.Close() }()
	emptyPending, err := fresh.PendingReviewChunks(ctx)
	require.NoError(t, err)
	assert.NotNil(t, emptyPending)
	assert.Empty(t, emptyPending)
	emptyCount, err := fresh.PendingReviewCount(ctx)
	require.NoError(t, err)
	assert.Zero(t, emptyCount)
}

// TestResolvedDispositions locks the disposition-history filter: "" (empty)
// returns every row, a closed-vocabulary value returns only those rows.
func TestResolvedDispositions(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	mustPutDisposition(t, s, store.DispositionRow{
		TS: time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC), ChunkID: 101,
		Disposition: "updated", Note: "换新版", Actor: "admin",
	})
	mustPutDisposition(t, s, store.DispositionRow{
		TS: time.Date(2026, 9, 3, 11, 0, 0, 0, time.UTC), ChunkID: 202,
		Disposition: "false_positive", Actor: "admin",
	})
	mustPutDisposition(t, s, store.DispositionRow{
		TS: time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC), ChunkID: 303,
		Disposition: "ignored", Actor: "reviewer-2",
	})

	all, err := s.ResolvedDispositions(ctx, "")
	require.NoError(t, err)
	require.Len(t, all, 3)
	// Newest-first by id (insertion order).
	assert.Equal(t, "ignored", all[0].Disposition)
	assert.Equal(t, "reviewer-2", all[0].Actor, "actor round-trips")
	assert.Equal(t, int64(303), all[0].ChunkID)

	fp, err := s.ResolvedDispositions(ctx, "false_positive")
	require.NoError(t, err)
	require.Len(t, fp, 1)
	assert.Equal(t, int64(202), fp[0].ChunkID)
	assert.Empty(t, fp[0].Note, "empty note round-trips as empty string")

	// Unknown filter value fails closed — an empty result would be read
	// by the admin UI as "no history", which would be a lie.
	_, err = s.ResolvedDispositions(ctx, "nonsense")
	require.Error(t, err)
}

// TestFeedbackMetrics locks the FBK-03 aggregation semantics:
// per-Skill counts every vote exactly once (the full-denominator D-29-10
// raw counts), per-KB counts a multi-KB vote once PER KB (Open Q2), and
// the daily trend buckets by date(ts) ascending.
func TestFeedbackMetrics(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// 2 helpful + 1 unhelpful + 1 outdated, all on skill "compliance".
	// V1 cites two KBs (kb-a + kb-b) — the multi-KB vote.
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), SessionID: "m1", TurnID: 1,
		Verdict: "helpful", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 11, KBSlug: "kb-a"}, {ChunkID: 12, KBSlug: "kb-b"}})
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC), SessionID: "m2", TurnID: 2,
		Verdict: "helpful", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 11, KBSlug: "kb-a"}})
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC), SessionID: "m3", TurnID: 3,
		Verdict: "unhelpful", SkillID: "compliance",
	}, nil)
	mustPutFeedback(t, s, store.FeedbackRow{
		TS: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), SessionID: "m4", TurnID: 4,
		Verdict: "outdated", SkillID: "compliance",
	}, []store.ChunkRef{{ChunkID: 13, KBSlug: "kb-a"}})

	// --- per-Skill: raw counts for the full-vote denominator ---
	bySkill, err := s.FeedbackMetricsBySkill(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, bySkill, 1)
	assert.Equal(t, "compliance", bySkill[0].Key)
	assert.Equal(t, 2, bySkill[0].Helpful)
	assert.Equal(t, 1, bySkill[0].Unhelpful)
	assert.Equal(t, 1, bySkill[0].Outdated, "outdated stays in the denominator (D-29-10)")

	// --- per-KB: the multi-KB vote counts once per KB ---
	byKB, err := s.FeedbackMetricsByKB(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, byKB, 2)
	kbA, kbB := byKB[0], byKB[1]
	assert.Equal(t, "kb-a", kbA.Key)
	assert.Equal(t, 2, kbA.Helpful, "kb-a sees V1 and V2")
	assert.Equal(t, 0, kbA.Unhelpful)
	assert.Equal(t, 1, kbA.Outdated)
	assert.Equal(t, "kb-b", kbB.Key)
	assert.Equal(t, 1, kbB.Helpful, "kb-b sees the multi-KB vote V1 exactly once per KB")
	assert.Equal(t, 0, kbB.Unhelpful)
	assert.Equal(t, 0, kbB.Outdated)

	// The vote WITHOUT refs contributes to per-Skill but to no KB row
	// (refusal rounds / no-citation votes are honest per-KB gaps).
	assert.NotEqual(t, 3, kbA.Helpful)

	// --- `since` bounds the window ---
	bySkillSince, err := s.FeedbackMetricsBySkill(ctx, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, bySkillSince, 1)
	assert.Equal(t, 0, bySkillSince[0].Helpful)
	assert.Equal(t, 1, bySkillSince[0].Unhelpful)
	assert.Equal(t, 1, bySkillSince[0].Outdated)

	byKBSince, err := s.FeedbackMetricsByKB(ctx, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, byKBSince, 1)
	assert.Equal(t, "kb-a", byKBSince[0].Key)
	assert.Equal(t, 1, byKBSince[0].Outdated)

	// --- daily trend: day ascending, per-verdict buckets (verdict ASC
	// inside a day, so "outdated" precedes "unhelpful" alphabetically) ---
	trend, err := s.FeedbackDailyTrend(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, trend, 3)
	assert.Equal(t, store.FeedbackTrendRow{Day: "2026-09-01", Verdict: "helpful", Count: 2}, trend[0])
	assert.Equal(t, store.FeedbackTrendRow{Day: "2026-09-02", Verdict: "outdated", Count: 1}, trend[1])
	assert.Equal(t, store.FeedbackTrendRow{Day: "2026-09-02", Verdict: "unhelpful", Count: 1}, trend[2])

	trendSince, err := s.FeedbackDailyTrend(ctx, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, trendSince, 2)

	// Empty store → empty non-nil slices.
	fresh, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = fresh.Close() }()
	emptySkill, err := fresh.FeedbackMetricsBySkill(ctx, time.Time{})
	require.NoError(t, err)
	assert.NotNil(t, emptySkill)
	assert.Empty(t, emptySkill)
	emptyKB, err := fresh.FeedbackMetricsByKB(ctx, time.Time{})
	require.NoError(t, err)
	assert.NotNil(t, emptyKB)
	assert.Empty(t, emptyKB)
	emptyTrend, err := fresh.FeedbackDailyTrend(ctx, time.Time{})
	require.NoError(t, err)
	assert.NotNil(t, emptyTrend)
	assert.Empty(t, emptyTrend)
}

// TestGetSessionTurn locks the session-bound turn lookup used by the
// 29-05/29-06 write endpoints: the WHERE clause binds BOTH session_id
// and id — a turn id from another session must never satisfy it
// (T-29-06 cross-session repudiation guard).
func TestGetSessionTurn(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	asstID, err := s.AppendTurn(ctx, "sess-turn-a",
		store.SessionMessage{Role: "user", Content: "问题", Ts: time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)},
		store.SessionMessage{Role: "assistant", Content: "回答", Ts: time.Date(2026, 9, 1, 9, 0, 1, 0, time.UTC)})
	require.NoError(t, err)
	require.Greater(t, asstID, int64(0))

	got, ok, err := s.GetSessionTurn(ctx, "sess-turn-a", asstID)
	require.NoError(t, err)
	require.True(t, ok, "existing assistant turn must resolve")
	assert.Equal(t, asstID, got.ID)
	assert.Equal(t, "assistant", got.Role)
	assert.Equal(t, "回答", got.Content)

	// A user half also resolves — the ROLE enforcement belongs to the
	// handler (29-05/29-06), the store only guarantees session binding.
	turns, err := s.GetSession(ctx, "sess-turn-a")
	require.NoError(t, err)
	require.Len(t, turns, 2)
	user, ok, err := s.GetSessionTurn(ctx, "sess-turn-a", turns[0].ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "user", user.Role)

	// Cross-session id → miss, never a hit (the core T-29-06 lock).
	_, ok, err = s.GetSessionTurn(ctx, "sess-turn-b", asstID)
	require.NoError(t, err)
	assert.False(t, ok, "a turn id from another session must not resolve")

	// Unknown id → miss.
	_, ok, err = s.GetSessionTurn(ctx, "sess-turn-a", 987654)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestVotedVerdictsBySession locks the 29-04 history echo substrate: the
// map is {turnID: verdict} and contains ONLY the requested session's
// votes.
func TestVotedVerdictsBySession(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	mustPutFeedback(t, s, store.FeedbackRow{
		SessionID: "sess-echo-a", TurnID: 11, Verdict: "helpful", SkillID: "compliance",
	}, nil)
	mustPutFeedback(t, s, store.FeedbackRow{
		SessionID: "sess-echo-a", TurnID: 12, Verdict: "outdated", SkillID: "compliance",
	}, nil)
	mustPutFeedback(t, s, store.FeedbackRow{
		SessionID: "sess-echo-b", TurnID: 11, Verdict: "unhelpful", SkillID: "compliance",
	}, nil)

	votes, err := s.VotedVerdictsBySession(ctx, "sess-echo-a")
	require.NoError(t, err)
	require.Len(t, votes, 2)
	assert.Equal(t, "helpful", votes[11])
	assert.Equal(t, "outdated", votes[12])

	other, err := s.VotedVerdictsBySession(ctx, "sess-echo-b")
	require.NoError(t, err)
	require.Len(t, other, 1)
	assert.Equal(t, "unhelpful", other[11])

	none, err := s.VotedVerdictsBySession(ctx, "sess-echo-none")
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

// TestFeedbackInsertOnly locks the feedback anti-tampering contract in
// the kpi_calls_test.go two-segment shape:
//
//  1. source-grep: feedback.go contains no row-mutation or row-removal
//     verb — a structural lock that survives future edits better than
//     any behavioral test (T-3-AUDIT posture for the feedback family;
//     FBK-05's "feedback never mutates trust" starts with feedback
//     storage never mutating rows).
//  2. round-trip + idempotent replay: a fully-populated vote reads back
//     field-for-field, and the replay path surfaces the FIRST row rather
//     than writing a new one.
func TestFeedbackInsertOnly(t *testing.T) {
	ctx := context.Background()

	// --- Segment 1: source-grep structural lock ---
	src, err := os.ReadFile("feedback.go")
	require.NoError(t, err, "feedback.go must be readable from the package dir")
	source := string(src)
	assert.NotContains(t, source, "UPDATE ",
		"feedback.go must contain no row-mutation verb (T-3-AUDIT / FBK-05 storage posture)")
	assert.NotContains(t, source, "DELETE ",
		"feedback.go must contain no row-removal verb (T-3-AUDIT / FBK-05 storage posture)")

	// --- Segment 2: round-trip + idempotent replay ---
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ts := time.Date(2026, 9, 1, 8, 30, 5, 123456789, time.UTC)
	row := store.FeedbackRow{
		TS:        ts,
		SessionID: "sess-roundtrip",
		TurnID:    42,
		Verdict:   "outdated",
		Comment:   "多行说明\n第二行 — 条款 5.2 已废止",
		SkillID:   "compliance",
		ChunkIDs:  "101,102,103",
	}
	inserted, err := s.PutFeedback(ctx, row)
	require.NoError(t, err)
	require.True(t, inserted)

	got, ok, err := s.FeedbackByTurn(ctx, row.SessionID, row.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, row.SessionID, got.SessionID)
	assert.Equal(t, row.TurnID, got.TurnID)
	assert.Equal(t, row.Verdict, got.Verdict)
	assert.Equal(t, row.Comment, got.Comment, "comment must round-trip byte-equal (multi-line + CJK)")
	assert.Equal(t, row.SkillID, got.SkillID)
	assert.Equal(t, row.ChunkIDs, got.ChunkIDs)
	assert.True(t, row.TS.Equal(got.TS), "TS must round-trip: want %v got %v", row.TS, got.TS)
	assert.Greater(t, got.ID, int64(0), "auto-increment ID must be populated on reads")

	// Idempotent replay: inserted=false, FIRST row read back, row count 1.
	replay := row
	replay.Verdict = "unhelpful"
	inserted, err = s.PutFeedback(ctx, replay)
	require.NoError(t, err)
	assert.False(t, inserted)

	reread, ok, err := s.FeedbackByTurn(ctx, row.SessionID, row.TurnID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "outdated", reread.Verdict, "replay must read back the existing row, not a new one")
	assert.Equal(t, got.ID, reread.ID, "row id must be unchanged by the replay")
	assert.Equal(t, 1, feedbackRowCount(t, s))
}
