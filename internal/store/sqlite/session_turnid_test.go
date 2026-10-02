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

// Phase 29 / Plan 29-01 — turn-id semantics tests (D-29-01). Locks the
// AppendTurn return contract that Phase 29's feedback idempotency keys
// on: fresh insert returns the ASSISTANT row's session_turn.id, an
// idempotent replay returns exactly 0 (Pitfall 2 — LastInsertId on a
// swallowed conflict is a stale rowid under mattn/go-sqlite3), and
// GetSession surfaces each row's id.
package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// newTurnIDTestStore opens a hermetic :memory: store (mirrors the
// kpi_calls_test.go / session_meta_test.go skeleton).
func newTurnIDTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(context.Background(), Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// turnIDFixture returns the SAME (user, assistant) pair every call —
// replaying AppendTurn with these exact values is what hits the
// UNIQUE(session_id, role, ts) + ON CONFLICT DO NOTHING guard (same
// fixture construction as TestAppendTurnIdempotent).
func turnIDFixture() (store.SessionMessage, store.SessionMessage) {
	ts := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	return store.SessionMessage{Role: "user", Content: "年假有几天？", Ts: ts},
		store.SessionMessage{Role: "assistant", Content: `{"status":"compliant"}`, Ts: ts.Add(time.Second)}
}

// TestAppendTurnReturnsAssistantID locks the four turn-id semantics
// Plan 29-01 exists to establish.
func TestAppendTurnReturnsAssistantID(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh", func(t *testing.T) {
		s := newTurnIDTestStore(t)
		user, asst := turnIDFixture()

		id, err := s.AppendTurn(ctx, "sess-turn-fresh", user, asst)
		require.NoError(t, err)
		require.Greater(t, id, int64(0), "fresh insert must return the assistant row's session_turn.id")

		hist, err := s.GetSession(ctx, "sess-turn-fresh")
		require.NoError(t, err)
		require.Len(t, hist, 2)

		require.Equal(t, hist[1].ID, id, "returned id must equal the ASSISTANT row's id in GetSession")
		require.Greater(t, hist[1].ID, hist[0].ID, "assistant row id must be greater than the user row id")
	})

	t.Run("replay_conflict", func(t *testing.T) {
		s := newTurnIDTestStore(t)
		user, asst := turnIDFixture()
		const sess = "sess-turn-replay"

		first, err := s.AppendTurn(ctx, sess, user, asst)
		require.NoError(t, err)
		require.Greater(t, first, int64(0))

		// Identical replay: BOTH halves hit ON CONFLICT DO NOTHING. The
		// return value must be EXACTLY 0 — never the stale rowid of the
		// previous successful insert on the connection (Pitfall 2).
		replayID, err := s.AppendTurn(ctx, sess, user, asst)
		require.NoError(t, err)
		require.Equal(t, int64(0), replayID, "idempotent replay must return exactly 0 (not a stale rowid)")

		hist, err := s.GetSession(ctx, sess)
		require.NoError(t, err)
		require.Len(t, hist, 2, "replay must not duplicate rows — still one (user, assistant) pair")
	})

	t.Run("get_session_ids", func(t *testing.T) {
		s := newTurnIDTestStore(t)
		const sess = "sess-turn-ids"

		id1, err := s.AppendTurn(ctx, sess,
			store.SessionMessage{Role: "user", Content: "q1", Ts: time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)},
			store.SessionMessage{Role: "assistant", Content: "a1", Ts: time.Date(2026, 9, 8, 10, 0, 1, 0, time.UTC)},
		)
		require.NoError(t, err)
		id2, err := s.AppendTurn(ctx, sess,
			store.SessionMessage{Role: "user", Content: "q2", Ts: time.Date(2026, 9, 8, 10, 1, 0, 0, time.UTC)},
			store.SessionMessage{Role: "assistant", Content: "a2", Ts: time.Date(2026, 9, 8, 10, 1, 1, 0, time.UTC)},
		)
		require.NoError(t, err)

		hist, err := s.GetSession(ctx, sess)
		require.NoError(t, err)
		require.Len(t, hist, 4, "two turns = four rows")

		ids := make([]int64, 0, len(hist))
		for _, m := range hist {
			ids = append(ids, m.ID)
		}
		require.Greater(t, ids[1], ids[0])
		require.Greater(t, ids[2], ids[1])
		require.Greater(t, ids[3], ids[2])
		assert.Equal(t, id1, ids[1], "first turn's AppendTurn return == its assistant row id")
		assert.Equal(t, id2, ids[3], "second turn's AppendTurn return == its assistant row id")
		assert.NotEqual(t, id1, id2, "assistant ids must be distinct across turns")
	})

	t.Run("closed_store", func(t *testing.T) {
		s := newTurnIDTestStore(t)
		require.NoError(t, s.Close())
		user, asst := turnIDFixture()

		id, err := s.AppendTurn(ctx, "sess-turn-closed", user, asst)
		require.Error(t, err, "AppendTurn on a closed store must return an error")
		require.ErrorIs(t, err, sql.ErrConnDone)
		require.Equal(t, int64(0), id, "error path must return id 0")
	})
}
