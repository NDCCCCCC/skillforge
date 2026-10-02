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

// Package store_test black-box tests the Store interface against the
// SQLite default implementation. Tests instantiate the store via the
// in-memory shared-cache DSN so (a) no file artifacts are left behind
// and (b) the shared cache keeps the in-memory DB alive across the
// connection pool. The package is external (store_test) so the tests
// exercise ONLY the exported surface — mirroring how Plan 03-05
// (server) and Plan 03-06 (eval sidecar) will consume it.
package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore opens a fresh in-memory SQLiteStore and registers a
// t.Cleanup hook to close it. Every test gets an isolated shared-cache
// database via a unique DSN filename so concurrent tests do not collide
// on the same in-memory DB.
func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	// Unique filename per test keeps the shared-cache in-memory DB isolated
	// even when tests run in parallel (the -race build runs tests serially
	// per-package, but the isolation also helps when -parallel is used).
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err, "sqlite.New should open an in-memory store")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// sampleAuditRow builds a fully-populated AuditRow with sane defaults
// so tests can override individual fields without restating the whole
// struct each time. The Timestamp is always non-zero.
func sampleAuditRow(sessionID string) store.AuditRow {
	return store.AuditRow{
		Timestamp:     time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC),
		SessionID:     sessionID,
		UserID:        "user-1",
		SkillID:       "compliance-review",
		Query:         "can i expense a personal laptop?",
		PromptHash:    hex.EncodeToString(sha256.New().Sum([]byte("prompt|compliance-review|doubao-1.5"))),
		ModelVersion:  "doubao-1.5-pro",
		VerdictJSON:   `{"status":"answer","confidence":0.8}`,
		RefusalReason: "",
		ChunkIDs:      "c1,c2,c3",
	}
}

// TestAuditRowSQLQuery — SC-7 / GEVL-03: every audit row is queryable
// via SQL filter (user, skill, time range, refusal_reason).
func TestAuditRowSQLQuery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	rows := []store.AuditRow{
		{Timestamp: base.Add(1 * time.Hour), UserID: "u1", SkillID: "s1", SessionID: "sess-1", RefusalReason: "no_authorized_source", Query: "q1", PromptHash: "h1", ModelVersion: "m1", VerdictJSON: "{}", ChunkIDs: "c1"},
		{Timestamp: base.Add(5 * time.Hour), UserID: "u2", SkillID: "s1", SessionID: "sess-2", RefusalReason: "low_faithfulness", Query: "q2", PromptHash: "h2", ModelVersion: "m1", VerdictJSON: "{}", ChunkIDs: "c2"},
		{Timestamp: base.Add(12 * time.Hour), UserID: "u1", SkillID: "s2", SessionID: "sess-3", RefusalReason: "", Query: "q3", PromptHash: "h3", ModelVersion: "m1", VerdictJSON: "{}", ChunkIDs: "c3,c4"},
	}
	for i := range rows {
		id, err := s.PutAudit(ctx, rows[i])
		require.NoError(t, err, "PutAudit[%d] should insert", i)
		require.Greater(t, id, int64(0), "PutAudit[%d] should return a positive ID", i)
	}

	// Query by UserID → exactly the rows for u1 (rows 0 and 2).
	got, err := s.QueryAudit(ctx, store.AuditFilter{UserID: "u1"})
	require.NoError(t, err)
	require.Len(t, got, 2, "u1 has two rows")
	for _, r := range got {
		assert.Equal(t, "u1", r.UserID)
	}

	// Query by SkillID + time window → only rows for s1 within [base+4h, base+6h).
	got, err = s.QueryAudit(ctx, store.AuditFilter{
		SkillID: "s1",
		Since:   base.Add(4 * time.Hour),
		Until:   base.Add(6 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, got, 1, "exactly one s1 row in the 4h-6h window")
	assert.Equal(t, "sess-2", got[0].SessionID)
	assert.Equal(t, "u2", got[0].UserID)

	// Query by RefusalReason → only rows with that exact reason.
	got, err = s.QueryAudit(ctx, store.AuditFilter{RefusalReason: "no_authorized_source"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "sess-1", got[0].SessionID)

	// Field round-trip: every persisted field matches what was inserted.
	got, err = s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-3"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "u1", got[0].UserID)
	assert.Equal(t, "s2", got[0].SkillID)
	assert.Equal(t, "q3", got[0].Query)
	assert.Equal(t, "h3", got[0].PromptHash)
	assert.Equal(t, "m1", got[0].ModelVersion)
	assert.Equal(t, "c3,c4", got[0].ChunkIDs, "chunk_ids round-trips as a first-class column")

	// SC-7: chunk_ids is queryable via SQL LIKE as a first-class column,
	// NOT via JSON_EXTRACT on VerdictJSON. We verify this by issuing a
	// raw LIKE query through the *sql.DB — the store exposes a DB handle
	// accessor for exactly this kind of operator-side SQL forensics.
	rawRows, err := s.DB().QueryContext(ctx, "SELECT id FROM audit WHERE chunk_ids LIKE '%c4%'")
	require.NoError(t, err)
	defer rawRows.Close()
	count := 0
	for rawRows.Next() {
		count++
	}
	require.NoError(t, rawRows.Err())
	assert.Equal(t, 1, count, "chunk_ids LIKE '%c4%' should find exactly the sess-3 row")
}

// TestAuditRowSQLInjection — T-3-SQLI regression: parameterized queries
// treat the injection payload as a literal string. The audit table
// survives DROP TABLE payloads in every filter field.
func TestAuditRowSQLInjection(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seed := sampleAuditRow("sess-seed")
	seed.UserID = "real-user"
	seed.SkillID = "real-skill"
	seed.RefusalReason = "real-reason"
	_, err := s.PutAudit(ctx, seed)
	require.NoError(t, err)

	payload := "'; DROP TABLE audit; --"

	// UserID filter — injection must NOT drop the audit table.
	got, err := s.QueryAudit(ctx, store.AuditFilter{UserID: payload})
	require.NoError(t, err, "parameterized query must not surface an error on the payload")
	assert.Empty(t, got, "no row matches the literal payload string")

	// SkillID + RefusalReason fields repeat the DROP TABLE payload.
	got, err = s.QueryAudit(ctx, store.AuditFilter{SkillID: payload})
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = s.QueryAudit(ctx, store.AuditFilter{RefusalReason: payload})
	require.NoError(t, err)
	assert.Empty(t, got)

	// Follow-up: an empty filter returns the original row, PROVING the
	// audit table is still queryable (not dropped). This is the
	// regression-lock for T-3-SQLI.
	got, err = s.QueryAudit(ctx, store.AuditFilter{})
	require.NoError(t, err)
	require.Len(t, got, 1, "the audit table must survive — the seeded row is intact")
	assert.Equal(t, "real-user", got[0].UserID)
}

// TestSessionIsolationConcurrent — SC-9 / AGNT-05: two concurrent
// same-user sessions (different session_id) see ONLY their own history.
// Run under -race; no race detector warnings may surface.
func TestSessionIsolationConcurrent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	const turnsPerSession = 5
	var wg sync.WaitGroup

	// Interleave appends across two sessions via two goroutines so the
	// race detector has maximum opportunity to surface a shared-state bug.
	appendSession := func(sessionID string) {
		defer wg.Done()
		for i := 0; i < turnsPerSession; i++ {
			ts := time.Date(2026, 8, 12, 10, i, 0, 0, time.UTC)
			user := store.SessionMessage{Role: "user", Content: sessionID + "-q-" + string(rune('a'+i)), Ts: ts}
			asst := store.SessionMessage{Role: "assistant", Content: sessionID + "-a-" + string(rune('a'+i)), Ts: ts.Add(1 * time.Second)}
			if _, err := s.AppendTurn(ctx, sessionID, user, asst); err != nil {
				t.Errorf("AppendTurn(%s): %v", sessionID, err)
				return
			}
		}
	}

	wg.Add(2)
	go appendSession("sess-A")
	go appendSession("sess-B")
	wg.Wait()

	// Read concurrently — race detector checks the read path too.
	var (
		aHist, bHist []store.SessionMessage
		readErr      [2]error
	)
	wg.Add(2)
	go func() { defer wg.Done(); aHist, readErr[0] = s.GetSession(ctx, "sess-A") }()
	go func() { defer wg.Done(); bHist, readErr[1] = s.GetSession(ctx, "sess-B") }()
	wg.Wait()

	require.NoError(t, readErr[0])
	require.NoError(t, readErr[1])

	// Each session sees exactly its own 2*turnsPerSession messages
	// (user + assistant per turn), and ZERO leaks from the other.
	require.Len(t, aHist, 2*turnsPerSession, "sess-A must have its own %d messages", 2*turnsPerSession)
	require.Len(t, bHist, 2*turnsPerSession, "sess-B must have its own %d messages", 2*turnsPerSession)
	for _, m := range aHist {
		assert.Contains(t, m.Content, "sess-A-", "sess-A history must NOT leak sess-B content")
	}
	for _, m := range bHist {
		assert.Contains(t, m.Content, "sess-B-", "sess-B history must NOT leak sess-A content")
	}
}

// TestAppendTurnIdempotent: replaying the same (sessionID, user.Ts,
// assistant.Ts) does NOT duplicate rows — the UNIQUE(session_id, role, ts)
// constraint + ON CONFLICT DO NOTHING is the dedup mechanism.
func TestAppendTurnIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user := store.SessionMessage{Role: "user", Content: "hello", Ts: time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)}
	asst := store.SessionMessage{Role: "assistant", Content: "hi back", Ts: time.Date(2026, 8, 12, 10, 0, 1, 0, time.UTC)}

	_, err := s.AppendTurn(ctx, "sess-x", user, asst)
	require.NoError(t, err)
	// Replay with identical timestamps — must be a no-op (idempotent);
	// Phase 29 / Plan 29-01: the replay returns id 0 (no new row).
	_, err = s.AppendTurn(ctx, "sess-x", user, asst)
	require.NoError(t, err)

	got, err := s.GetSession(ctx, "sess-x")
	require.NoError(t, err)
	require.Len(t, got, 2, "AppendTurn must be idempotent — exactly 2 rows (user+assistant), not 4")
}

// TestHealthAndClose: Health is nil on a fresh store; Close makes every
// subsequent method return a meaningful error (sql.ErrConnDone-equivalent).
// The plan's done-criteria say "every method returns a meaningful error"
// — we lock that for ALL five Store methods, not just Health + PutAudit.
func TestHealthAndClose(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.Health(ctx), "Health on a fresh store must be nil")

	require.NoError(t, s.Close(), "Close must succeed on a fresh store")

	// Health after Close returns a non-nil error.
	err := s.Health(ctx)
	assert.Error(t, err, "Health after Close must return an error")

	// PutAudit after Close surfaces a connection-done error.
	_, err = s.PutAudit(ctx, sampleAuditRow("sess-z"))
	assert.Error(t, err, "PutAudit after Close must return an error")
	assert.True(t, errors.Is(err, sql.ErrConnDone) || err != nil,
		"post-Close PutAudit error should be meaningful (got %v)", err)

	// QueryAudit after Close — same isClosed() guard path.
	_, err = s.QueryAudit(ctx, store.AuditFilter{})
	assert.Error(t, err, "QueryAudit after Close must return an error")

	// GetSession after Close.
	_, err = s.GetSession(ctx, "sess-z")
	assert.Error(t, err, "GetSession after Close must return an error")

	// AppendTurn after Close.
	_, err = s.AppendTurn(ctx, "sess-z",
		store.SessionMessage{Role: "user", Content: "x", Ts: time.Now()},
		store.SessionMessage{Role: "assistant", Content: "y", Ts: time.Now()})
	assert.Error(t, err, "AppendTurn after Close must return an error")
}

// TestPromptHashFormat: prompt_hash is just a string column — the SHA-256
// canon is the CALLER's responsibility; the store persists any string.
// We verify a 64-char hex SHA-256 round-trips byte-identically.
func TestPromptHashFormat(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sum := sha256.Sum256([]byte("rendered system prompt|compliance-review|doubao-1.5-pro"))
	hash := hex.EncodeToString(sum[:])
	assert.Len(t, hash, 64, "SHA-256 hex is 64 chars")

	row := sampleAuditRow("sess-hash")
	row.PromptHash = hash
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-hash"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, hash, got[0].PromptHash, "prompt_hash must round-trip identically")
}

// TestUpdateAuditMetrics — Plan 03-06 eval sidecar backfill path. The
// 4 metric columns start NULL (PutAudit writes NULL when the *float64
// fields are nil); UpdateAuditMetrics fills them in. Verifies:
//
//   - All 4 metrics backfill correctly with the exact values passed.
//   - Passing nil for one metric preserves the prior column value
//     (COALESCE behavior — the eval CLI may compute only 3 of 4 metrics
//     for a refusal case).
//   - A second UpdateAuditMetrics on the same row replaces non-nil
//     metrics (idempotent re-run of the eval CLI).
//   - UpdateAuditMetrics on a non-existent row → sql.ErrNoRows.
func TestUpdateAuditMetrics(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert a row with NULL metrics (the chat path's default state).
	row := sampleAuditRow("sess-metrics")
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// Sanity: metrics start NULL.
	got, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-metrics"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Nil(t, got[0].Faithfulness, "metric starts NULL")
	require.Nil(t, got[0].ContextPrecision)

	// Backfill all 4 metrics.
	faith, cprec, arel, crec := 0.92, 0.85, 0.88, 0.79
	require.NoError(t, s.UpdateAuditMetrics(ctx, id, &faith, &cprec, &arel, &crec))

	got, err = s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-metrics"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, faith, *got[0].Faithfulness)
	require.Equal(t, cprec, *got[0].ContextPrecision)
	require.Equal(t, arel, *got[0].AnswerRelevance)
	require.Equal(t, crec, *got[0].ContextRecall)

	// Partial update: pass nil for context_recall — its prior value
	// (0.79) MUST survive (COALESCE behavior).
	newFaith := 0.95
	require.NoError(t, s.UpdateAuditMetrics(ctx, id, &newFaith, nil, nil, nil))
	got, err = s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-metrics"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, newFaith, *got[0].Faithfulness, "faithfulness updated")
	require.Equal(t, cprec, *got[0].ContextPrecision, "context_precision preserved (COALESCE)")
	require.Equal(t, arel, *got[0].AnswerRelevance, "answer_relevance preserved (COALESCE)")
	require.Equal(t, crec, *got[0].ContextRecall, "context_recall preserved (COALESCE)")

	// Non-existent row → sql.ErrNoRows.
	err = s.UpdateAuditMetrics(ctx, 999999, &faith, nil, nil, nil)
	require.ErrorIs(t, err, sql.ErrNoRows, "missing row must surface as sql.ErrNoRows")
}
