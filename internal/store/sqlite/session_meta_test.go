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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// newMetaTestStore opens an in-memory store for session_meta tests.
func newMetaTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(context.Background(), Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedTurns appends one synthetic turn (user query + assistant verdict
// JSON) at the given timestamp — enough to build a roster with distinct
// session ids and activity times.
func seedTurns(t *testing.T, s *SQLiteStore, sessionID, userQuery string, ts time.Time) {
	t.Helper()
	err := s.AppendTurn(context.Background(), sessionID,
		store.SessionMessage{Role: "user", Content: userQuery, Ts: ts},
		store.SessionMessage{Role: "assistant", Content: `{"status":"compliant"}`, Ts: ts.Add(time.Second)})
	require.NoError(t, err)
}

func TestListSessionsDerivesRosterNewestFirst(t *testing.T) {
	s := newMetaTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	seedTurns(t, s, "sess-old", "第一句问话", base)
	seedTurns(t, s, "sess-new", "更新的问题", base.Add(2*time.Hour))

	rows, err := s.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// Newest activity first.
	assert.Equal(t, "sess-new", rows[0].ID)
	assert.Equal(t, "更新的问题", rows[0].DerivedTitle)
	assert.Equal(t, "", rows[0].Title, "no rename → empty override")
	assert.WithinDuration(t, base.Add(2*time.Hour).Add(time.Second), rows[0].LastActive, time.Second)

	assert.Equal(t, "sess-old", rows[1].ID)
	assert.WithinDuration(t, base.Add(time.Second), rows[1].LastActive, time.Second)
}

func TestListSessionsTruncatesDerivedTitleToFortyRunes(t *testing.T) {
	s := newMetaTestStore(t)
	long := ""
	for i := 0; i < 60; i++ {
		long += "字"
	}
	seedTurns(t, s, "sess-long", long, time.Now().UTC())

	rows, err := s.ListSessions(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Len(t, []rune(rows[0].DerivedTitle), 41, "40 runes + ellipsis")
	assert.Equal(t, string([]rune(long)[:40])+"…", rows[0].DerivedTitle)
}

func TestRenameSessionOverrideWinsOverDerived(t *testing.T) {
	s := newMetaTestStore(t)
	ctx := context.Background()
	seedTurns(t, s, "sess-a", "原始问题", time.Now().UTC())

	// Before rename: derived title.
	rows, err := s.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "原始问题", rows[0].DerivedTitle)

	require.NoError(t, s.RenameSession(ctx, "sess-a", "自定义名字"))

	rows, err = s.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "自定义名字", rows[0].Title, "override surfaces via LEFT JOIN")
	assert.Equal(t, "原始问题", rows[0].DerivedTitle, "derived stays intact underneath")

	// Rename again → upsert replaces (no duplicate meta row).
	require.NoError(t, s.RenameSession(ctx, "sess-a", "再改一次"))
	rows, err = s.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "再改一次", rows[0].Title)
}

func TestRenameSessionEmptyIDRejected(t *testing.T) {
	s := newMetaTestStore(t)
	err := s.RenameSession(context.Background(), "", "x")
	assert.ErrorIs(t, err, errEmptySessionID)
}

func TestDeleteSessionTurnsRemovesRosterEntryKeepsAudit(t *testing.T) {
	s := newMetaTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	seedTurns(t, s, "sess-doomed", "将被删除", now)
	require.NoError(t, s.RenameSession(ctx, "sess-doomed", "重命名过的"))

	// An audit row for the same session — the immutable trail.
	auditID, err := s.PutAudit(ctx, store.AuditRow{
		Timestamp:   now,
		SessionID:   "sess-doomed",
		SkillID:     "compliance",
		Query:       "将被删除",
		PromptHash:  "deadbeef",
		VerdictJSON: "{}",
	})
	require.NoError(t, err)
	require.Greater(t, auditID, int64(0))

	// History + roster both see the session before deletion.
	hist, err := s.GetSession(ctx, "sess-doomed")
	require.NoError(t, err)
	assert.Len(t, hist, 2)
	rows, err := s.ListSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// Delete — turns + meta gone, audit survives.
	require.NoError(t, s.DeleteSessionTurns(ctx, "sess-doomed"))

	hist, err = s.GetSession(ctx, "sess-doomed")
	require.NoError(t, err)
	assert.Empty(t, hist)
	rows, err = s.ListSessions(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)

	audits, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-doomed", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, audits, 1, "audit trail must survive session deletion (T-3-AUDIT)")
}

func TestDeleteSessionTurnsIdempotentUnknownID(t *testing.T) {
	s := newMetaTestStore(t)
	// Unknown id: zero rows affected, still success (T-10-11 posture —
	// no existence signal).
	assert.NoError(t, s.DeleteSessionTurns(context.Background(), "never-existed-1234"))
	// Empty id: rejected like every other session-scoped write.
	assert.ErrorIs(t, s.DeleteSessionTurns(context.Background(), ""), errEmptySessionID)
}

func TestListSessionsEmptyDB(t *testing.T) {
	s := newMetaTestStore(t)
	rows, err := s.ListSessions(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, rows)
	assert.Empty(t, rows)
}
