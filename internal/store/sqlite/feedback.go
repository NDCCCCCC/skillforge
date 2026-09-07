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

// Phase 29 / Plan 29-02 — INSERT-only feedback storage (FBK-01/02/03).
// Structural mirror of kpi_calls.go with the session.go ON CONFLICT
// idempotency form: exactly three write paths (PutFeedback,
// PutFeedbackChunkRefs, PutDisposition) append rows and do nothing else —
// this file deliberately ships no row-mutation or row-removal verb at all
// (T-3-AUDIT posture; locked by TestFeedbackInsertOnly's source-grep
// assertion). The admin pending view is DERIVED (a chunk is pending while
// it has an outdated vote with no disposition at or after that vote's ts)
// — no second write path exists anywhere.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putFeedbackSQL is the single write path for a vote (FBK-01 / D-29-02).
// Column order mirrors the CREATE TABLE DDL in schema.go one-to-one; id
// is omitted (AUTOINCREMENT assigns it). The UNIQUE(session_id, turn_id)
// constraint is the one-vote-per-message truth: a replayed vote hits
// ON CONFLICT DO NOTHING, which PutFeedback surfaces as inserted=false.
const putFeedbackSQL = `
INSERT INTO feedback (ts, session_id, turn_id, verdict, comment, skill_id, chunk_ids)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id, turn_id) DO NOTHING
`

// PutFeedback appends one INSERT-only vote row. RowsAffected()==0 marks
// the replay path: the caller must then read the FIRST row back via
// FeedbackByTurn and surface it as "already recorded" — never write a
// second vote (D-29-02; the FE lock state is only a cache).
func (s *SQLiteStore) PutFeedback(ctx context.Context, row store.FeedbackRow) (bool, error) {
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
	res, err := s.db.ExecContext(ctx, putFeedbackSQL,
		ts,
		row.SessionID,
		row.TurnID,
		row.Verdict,
		row.Comment,
		row.SkillID,
		row.ChunkIDs,
	)
	if err != nil {
		return false, fmtError("PutFeedback", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmtError("PutFeedback.RowsAffected", err)
	}
	return n > 0, nil
}

// feedbackByTurnSQL reads one vote. The WHERE clause binds BOTH
// session_id and turn_id — the session_id term is the T-29-06
// cross-session isolation enforcement (a turn id alone never resolves).
const feedbackByTurnSQL = `
SELECT id, ts, session_id, turn_id, verdict, comment, skill_id, chunk_ids
FROM feedback
WHERE session_id = ? AND turn_id = ?
`

// FeedbackByTurn returns the vote for (sessionID, turnID), or ok=false
// when that pair has no vote.
func (s *SQLiteStore) FeedbackByTurn(ctx context.Context, sessionID string, turnID int64) (store.FeedbackRow, bool, error) {
	if s.isClosed() {
		return store.FeedbackRow{}, false, sql.ErrConnDone
	}
	if sessionID == "" {
		return store.FeedbackRow{}, false, errEmptySessionID
	}
	var r store.FeedbackRow
	err := s.db.QueryRowContext(ctx, feedbackByTurnSQL, sessionID, turnID).
		Scan(&r.ID, &r.TS, &r.SessionID, &r.TurnID, &r.Verdict, &r.Comment, &r.SkillID, &r.ChunkIDs)
	if errors.Is(err, sql.ErrNoRows) {
		return store.FeedbackRow{}, false, nil
	}
	if err != nil {
		return store.FeedbackRow{}, false, fmtError("FeedbackByTurn", err)
	}
	return r, true, nil
}

// putFeedbackChunkRefSQL appends one normalized (vote, cited chunk)
// association row. The UNIQUE(feedback_id, chunk_id) constraint makes a
// replayed pair a no-op — one row per pair, forever (FBK-02/03
// aggregation substrate).
const putFeedbackChunkRefSQL = `
INSERT INTO feedback_chunk_refs (feedback_id, chunk_id, kb_slug)
VALUES (?, ?, ?)
ON CONFLICT(feedback_id, chunk_id) DO NOTHING
`

// PutFeedbackChunkRefs appends the normalized per-chunk rows of one
// vote. kb_slug is the knowledge_bases registry slug (retriever chunk
// Tier, Phase 26 semantics) resolved by the 29-05 vote path; this layer
// only persists it. An empty refs slice is a legal no-op (votes without
// citations — refusal rounds — simply have no rows).
func (s *SQLiteStore) PutFeedbackChunkRefs(ctx context.Context, feedbackID int64, refs []store.ChunkRef) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	for _, ref := range refs {
		if _, err := s.db.ExecContext(ctx, putFeedbackChunkRefSQL,
			feedbackID, ref.ChunkID, ref.KBSlug); err != nil {
			return fmtError("PutFeedbackChunkRefs", err)
		}
	}
	return nil
}

// votedVerdictsBySessionSQL feeds the 29-04 history echo (D-29-03): one
// row per vote of one session, so session restore can return
// voted_verdict inline with zero extra requests.
const votedVerdictsBySessionSQL = `
SELECT turn_id, verdict
FROM feedback
WHERE session_id = ?
`

// VotedVerdictsBySession returns {turnID: verdict} for sessionID. Only
// that session's votes are consulted (T-29-06). Empty (non-nil) map when
// the session has no votes.
func (s *SQLiteStore) VotedVerdictsBySession(ctx context.Context, sessionID string) (map[int64]string, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if sessionID == "" {
		return nil, errEmptySessionID
	}
	rows, err := s.db.QueryContext(ctx, votedVerdictsBySessionSQL, sessionID)
	if err != nil {
		return nil, fmtError("VotedVerdictsBySession", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64]string)
	for rows.Next() {
		var turnID int64
		var verdict string
		if err := rows.Scan(&turnID, &verdict); err != nil {
			return nil, fmtError("VotedVerdictsBySession", err)
		}
		out[turnID] = verdict
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("VotedVerdictsBySession", err)
	}
	return out, nil
}

// putDispositionSQL appends one review-queue disposition row (FBK-02 /
// D-29-09). There is NO UNIQUE constraint by design — multiple
// dispositions of one chunk are legal and the newest timestamp wins
// ("latest-disposition-wins", Open Q1); the pending derivation reads
// this table, it is never rewritten.
const putDispositionSQL = `
INSERT INTO feedback_dispositions (ts, chunk_id, disposition, note, actor)
VALUES (?, ?, ?, ?, ?)
`

// DispositionNames is the closed disposition vocabulary (D-29-09) — the
// same three values the DDL CHECK backstops. Used to fail closed on an
// unknown filter value instead of silently returning an empty history.
var DispositionNames = map[string]bool{
	"updated":        true,
	"false_positive": true,
	"ignored":        true,
}

// PutDisposition appends one INSERT-only disposition row and returns the
// assigned id (>0). An unknown disposition value is refused by the DDL
// CHECK constraint — the storage layer is the last line of defense, the
// closed vocabulary's single home is this package's DispositionNames.
func (s *SQLiteStore) PutDisposition(ctx context.Context, row store.DispositionRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	ts := row.TS
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	actor := row.Actor
	if actor == "" {
		actor = "admin"
	}
	res, err := s.db.ExecContext(ctx, putDispositionSQL,
		ts, row.ChunkID, row.Disposition, row.Note, actor)
	if err != nil {
		return 0, fmtError("PutDisposition", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmtError("PutDisposition.LastInsertId", err)
	}
	return id, nil
}

// pendingReviewChunksSQL derives the admin pending view (FBK-02 /
// D-29-08 / RESEARCH Pattern 5). Semantics, one by one:
//
//   - JOIN feedback_chunk_refs: a chunk enters the queue only through a
//     cited outdated vote (votes without refs never appear — an honest
//     per-chunk gap, mirroring the per-KB metrics gap).
//   - verdict = 'outdated': only outdated reports are reviewable.
//   - NOT EXISTS (... d.ts >= f.ts): a disposition whose ts is at or
//     after the vote's ts closes that vote ("latest disposition wins").
//     A NEWER outdated vote (ts > the disposition ts) reopens the chunk
//     — the Open Q1 resolved semantic, locked by TestPendingReviewChunks.
//   - GROUP BY chunk_id: one queue row per chunk; report_count is
//     itself the priority signal (D-29-08).
//   - comments via GROUP_CONCAT(char(10)): all qualifying notes in one
//     string, empty notes kept as empty placeholders; Go-side
//     strings.Split restores the per-report list.
const pendingReviewChunksSQL = `
SELECT r.chunk_id,
       MAX(r.kb_slug)                    AS kb_slug,
       COUNT(*)                          AS report_count,
       MAX(f.ts)                         AS last_reported_at,
       GROUP_CONCAT(f.comment, char(10)) AS comments
FROM feedback f
JOIN feedback_chunk_refs r ON r.feedback_id = f.id
WHERE f.verdict = 'outdated'
  AND NOT EXISTS (
      SELECT 1 FROM feedback_dispositions d
      WHERE d.chunk_id = r.chunk_id AND d.ts >= f.ts
  )
GROUP BY r.chunk_id
ORDER BY report_count DESC, last_reported_at DESC
`

// sqliteTSLayouts cover the text forms a bound time.Time produces in
// this package. mattn/go-sqlite3 writes time.Time with
// SQLiteTimestampFormats[0] ("2006-01-02 15:04:05.999999999-07:00"),
// whose OPTIONAL fractional part keeps lexicographic TEXT comparison
// chronologically correct even on zero-nanosecond stamps ("…:00+00:00"
// sorts BELOW "…:00.25+00:00" — the RFC3339Nano trailing-zero trap from
// D16-03-B does not apply here). RFC3339Nano/RFC3339 are fallbacks for
// rows written by the explicit-TEXT precedent (datasets.go /
// import_batches.go).
var sqliteTSLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	time.RFC3339Nano,
	time.RFC3339,
}

// parseSQLiteTS parses one stored timestamp text back into UTC
// time.Time. Needed for AGGREGATE columns (MAX(f.ts)) — sqlite does not
// propagate a declared type through MAX, so the driver hands the value
// over as plain TEXT there, unlike plain column scans which the driver
// converts by the column's DATETIME decltype.
func parseSQLiteTS(v string) (time.Time, error) {
	for _, layout := range sqliteTSLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable sqlite timestamp %q", v)
}

// PendingReviewChunks returns the derived pending view, highest priority
// first. Empty (non-nil) slice when nothing is pending.
func (s *SQLiteStore) PendingReviewChunks(ctx context.Context) ([]store.PendingReviewRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, pendingReviewChunksSQL)
	if err != nil {
		return nil, fmtError("PendingReviewChunks", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.PendingReviewRow, 0)
	for rows.Next() {
		var (
			r        store.PendingReviewRow
			comments string
			lastTS   string
		)
		if err := rows.Scan(&r.ChunkID, &r.KBSlug, &r.ReportCount, &lastTS, &comments); err != nil {
			return nil, fmtError("PendingReviewChunks", err)
		}
		lastReported, err := parseSQLiteTS(lastTS)
		if err != nil {
			return nil, fmtError("PendingReviewChunks", err)
		}
		r.LastReportedAt = lastReported
		r.Comments = strings.Split(comments, "\n")
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("PendingReviewChunks", err)
	}
	return out, nil
}

// pendingReviewCountSQL is the same derivation as
// pendingReviewChunksSQL collapsed to one aggregate — the admin badge
// counter counts distinct pending chunks.
const pendingReviewCountSQL = `
SELECT COUNT(DISTINCT r.chunk_id)
FROM feedback f
JOIN feedback_chunk_refs r ON r.feedback_id = f.id
WHERE f.verdict = 'outdated'
  AND NOT EXISTS (
      SELECT 1 FROM feedback_dispositions d
      WHERE d.chunk_id = r.chunk_id AND d.ts >= f.ts
  )
`

// PendingReviewCount returns the number of chunks currently pending.
func (s *SQLiteStore) PendingReviewCount(ctx context.Context) (int, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	var n int
	if err := s.db.QueryRowContext(ctx, pendingReviewCountSQL).Scan(&n); err != nil {
		return 0, fmtError("PendingReviewCount", err)
	}
	return n, nil
}

// listDispositionsBaseSQL selects the full disposition-history column
// set. The WHERE clause (if any) is assembled ONLY from the fixed clause
// constant below — column names are code, never caller input, and every
// VALUE rides in a `?` placeholder (T-3-SQLI posture, kpi_calls.go
// precedent).
const listDispositionsBaseSQL = `
SELECT id, ts, chunk_id, disposition, note, actor
FROM feedback_dispositions
`

// dispositionsClauseDisposition is the one sanctioned filter clause — a
// hardcoded constant so strings.Join can never see a caller-controlled
// string (T-3-SQLI).
const dispositionsClauseDisposition = "disposition = ?"

// listDispositionsTailSQL orders newest-first by insertion id.
const listDispositionsTailSQL = " ORDER BY id DESC"

// ResolvedDispositions returns the disposition history, newest-first.
// An empty disposition string means "all rows"; otherwise the value must
// be a closed-vocabulary disposition — an unknown value is an ERROR, not
// an empty result (the admin view would read empty as "no history",
// which would be a lie).
func (s *SQLiteStore) ResolvedDispositions(ctx context.Context, disposition string) ([]store.DispositionRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	var (
		clauses []string
		args    []any
	)
	if disposition != "" {
		if !DispositionNames[disposition] {
			return nil, fmtError("ResolvedDispositions",
				fmt.Errorf("unknown disposition %q (allowed: updated|false_positive|ignored)", disposition))
		}
		clauses = append(clauses, dispositionsClauseDisposition)
		args = append(args, disposition)
	}

	q := listDispositionsBaseSQL
	if len(clauses) > 0 {
		// strings.Join is safe — the only strings in `clauses` are the
		// hardcoded parameterized literals above, NEVER user input.
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += listDispositionsTailSQL

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmtError("ResolvedDispositions", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.DispositionRow, 0)
	for rows.Next() {
		var r store.DispositionRow
		if err := rows.Scan(&r.ID, &r.TS, &r.ChunkID, &r.Disposition, &r.Note, &r.Actor); err != nil {
			return nil, fmtError("ResolvedDispositions", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ResolvedDispositions", err)
	}
	return out, nil
}

// feedbackMetricsBySkillSQL aggregates raw verdict counts per skill_id
// (FBK-03 / D-29-10). outdated deliberately STAYS in the counts — the
// full-vote denominator (helpful + unhelpful + outdated) is computed by
// the Go-side metrics endpoint, never in SQL (no division-by-zero /
// float-precision handling in the storage layer). Every vote is counted
// exactly once in this dimension.
const feedbackMetricsBySkillSQL = `
SELECT skill_id,
       SUM(verdict = 'helpful')   AS helpful,
       SUM(verdict = 'unhelpful') AS unhelpful,
       SUM(verdict = 'outdated')  AS outdated
FROM feedback
WHERE ts >= ?
GROUP BY skill_id
ORDER BY skill_id ASC
`

// FeedbackMetricsBySkill returns raw per-Skill verdict counts for votes
// at or after since (zero time = no bound). Empty (non-nil) slice when
// nothing matches.
func (s *SQLiteStore) FeedbackMetricsBySkill(ctx context.Context, since time.Time) ([]store.FeedbackCountRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, feedbackMetricsBySkillSQL, since)
	if err != nil {
		return nil, fmtError("FeedbackMetricsBySkill", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.FeedbackCountRow, 0)
	for rows.Next() {
		var r store.FeedbackCountRow
		if err := rows.Scan(&r.Key, &r.Helpful, &r.Unhelpful, &r.Outdated); err != nil {
			return nil, fmtError("FeedbackMetricsBySkill", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("FeedbackMetricsBySkill", err)
	}
	return out, nil
}

// feedbackMetricsByKBSQL aggregates raw verdict counts per kb_slug,
// attributed through the normalized refs table (FBK-03). ACCOUNTING
// CAVEAT (Open Q2, locked by godoc + TestFeedbackMetrics): one vote
// citing chunks from N knowledge bases contributes ONE COUNT PER KB, so
// vote counts may repeat across KB rows. The per-Skill dimension and any
// total count every vote strictly once. Votes without refs (refusal
// rounds / no-citation turns) contribute to no KB row.
const feedbackMetricsByKBSQL = `
SELECT r.kb_slug,
       SUM(f.verdict = 'helpful')   AS helpful,
       SUM(f.verdict = 'unhelpful') AS unhelpful,
       SUM(f.verdict = 'outdated')  AS outdated
FROM feedback f
JOIN feedback_chunk_refs r ON r.feedback_id = f.id
WHERE f.ts >= ?
GROUP BY r.kb_slug
ORDER BY r.kb_slug ASC
`

// FeedbackMetricsByKB returns raw per-KB verdict counts for votes at or
// after since (zero time = no bound). Empty (non-nil) slice when nothing
// matches.
func (s *SQLiteStore) FeedbackMetricsByKB(ctx context.Context, since time.Time) ([]store.FeedbackCountRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, feedbackMetricsByKBSQL, since)
	if err != nil {
		return nil, fmtError("FeedbackMetricsByKB", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.FeedbackCountRow, 0)
	for rows.Next() {
		var r store.FeedbackCountRow
		if err := rows.Scan(&r.Key, &r.Helpful, &r.Unhelpful, &r.Outdated); err != nil {
			return nil, fmtError("FeedbackMetricsByKB", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("FeedbackMetricsByKB", err)
	}
	return out, nil
}

// feedbackDailyTrendSQL buckets votes by date(ts) at day granularity,
// one cell per (day, verdict) (FBK-03 / D-29-11). The weekly view is a
// caller-side merge of these buckets — the storage layer returns the
// finest granularity.
const feedbackDailyTrendSQL = `
SELECT date(ts) AS day, verdict, COUNT(*) AS vote_count
FROM feedback
WHERE ts >= ?
GROUP BY date(ts), verdict
ORDER BY day ASC, verdict ASC
`

// FeedbackDailyTrend returns the day-bucketed trend, day ascending.
// Empty (non-nil) slice when nothing matches.
func (s *SQLiteStore) FeedbackDailyTrend(ctx context.Context, since time.Time) ([]store.FeedbackTrendRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, feedbackDailyTrendSQL, since)
	if err != nil {
		return nil, fmtError("FeedbackDailyTrend", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.FeedbackTrendRow, 0)
	for rows.Next() {
		var r store.FeedbackTrendRow
		if err := rows.Scan(&r.Day, &r.Verdict, &r.Count); err != nil {
			return nil, fmtError("FeedbackDailyTrend", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("FeedbackDailyTrend", err)
	}
	return out, nil
}

// getSessionTurnSQL resolves one turn of one session. Both predicates
// are mandatory: id alone would let a turn id from another session
// resolve (T-29-06), which is exactly the cross-session repudiation
// guard the 29-05/29-06 write endpoints depend on.
const getSessionTurnSQL = `
SELECT id, role, content, ts
FROM session_turn
WHERE session_id = ? AND id = ?
`

// GetSessionTurn returns the turn (sessionID, id), or ok=false on a
// miss — including a turn id that exists but belongs to another session.
// Role enforcement (assistant-only for votes/tickets) belongs to the
// handler; the store guarantees session binding only.
func (s *SQLiteStore) GetSessionTurn(ctx context.Context, sessionID string, id int64) (store.SessionMessage, bool, error) {
	if s.isClosed() {
		return store.SessionMessage{}, false, sql.ErrConnDone
	}
	if sessionID == "" {
		return store.SessionMessage{}, false, errEmptySessionID
	}
	var m store.SessionMessage
	err := s.db.QueryRowContext(ctx, getSessionTurnSQL, sessionID, id).
		Scan(&m.ID, &m.Role, &m.Content, &m.Ts)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionMessage{}, false, nil
	}
	if err != nil {
		return store.SessionMessage{}, false, fmtError("GetSessionTurn", err)
	}
	return m, true, nil
}
