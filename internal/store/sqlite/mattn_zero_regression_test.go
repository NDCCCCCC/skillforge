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
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSqliteMattnZeroRegression is the 10th lock for the store layer:
// it proves that switching from modernc.org/sqlite to mattn/go-sqlite3
// leaves all existing semantics intact AND that busy_timeout=5000 is
// applied per-connection so concurrent writes on a file-backed database
// wait rather than failing with SQLITE_BUSY.
func TestSqliteMattnZeroRegression(t *testing.T) {
	t.Run("aggregate", func(t *testing.T) {
		ctx := context.Background()
		dsn := filepath.Join(t.TempDir(), "mattn-zero-aggregate.db")
		s, err := New(ctx, Config{DSN: dsn})
		require.NoError(t, err, "sqlite.New should open a file-backed mattn store")
		defer func() { _ = s.Close() }()

		// (a) audit write + read round-trip
		row := store.AuditRow{
			Timestamp:   time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC),
			SessionID:   "sess-aggregate",
			UserID:      "user-mattn",
			SkillID:     "compliance",
			Query:       "mattn aggregate round-trip?",
			PromptHash:  "deadbeef",
			ModelVersion: "glm-5.2",
			VerdictJSON: `{"status":"answer","confidence":0.9}`,
		}
		id, err := s.PutAudit(ctx, row)
		require.NoError(t, err, "PutAudit should insert under mattn")
		require.Greater(t, id, int64(0))

		got, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: row.SessionID})
		require.NoError(t, err)
		require.Len(t, got, 1, "audit row must round-trip")
		assert.Equal(t, row.Query, got[0].Query)

		// (b) checkpoint round-trip
		payload := []byte("mattn-checkpoint-payload")
		require.NoError(t, s.Set(ctx, "cp-mattn", payload), "Set checkpoint should work")
		gotPayload, existed, err := s.Get(ctx, "cp-mattn")
		require.NoError(t, err)
		require.True(t, existed, "checkpoint must exist")
		assert.Equal(t, payload, gotPayload, "checkpoint bytes must round-trip")

		// (c) session turn idempotency
		ts := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
		user := store.SessionMessage{Role: "user", Content: "u", Ts: ts}
		asst := store.SessionMessage{Role: "assistant", Content: "a", Ts: ts.Add(time.Second)}
		_, err = s.AppendTurn(ctx, "sess-turns", user, asst)
		require.NoError(t, err, "AppendTurn should insert")
		hist, err := s.GetSession(ctx, "sess-turns")
		require.NoError(t, err)
		require.Len(t, hist, 2, "session history must round-trip")

		// (d) SQLi DROP TABLE survival (T-3-SQLI posture survives driver swap)
		injection := "'; DROP TABLE audit; --"
		_, err = s.QueryAudit(ctx, store.AuditFilter{UserID: injection})
		require.NoError(t, err, "parameterized query must survive injection payload")
		remain, err := s.QueryAudit(ctx, store.AuditFilter{})
		require.NoError(t, err)
		require.Len(t, remain, 1, "audit table must survive DROP TABLE payload")
	})

	t.Run("concurrent_busy_timeout", func(t *testing.T) {
		ctx := context.Background()
		dsn := filepath.Join(t.TempDir(), "mattn-zero-concurrent.db")
		s, err := New(ctx, Config{DSN: dsn, MaxOpenConns: 4})
		require.NoError(t, err, "sqlite.New should open a file-backed store with pool=4")
		defer func() { _ = s.Close() }()
		require.Equal(t, 4, s.cfg.MaxOpenConns, "file DSN must keep MaxOpenConns=4")

		const (
			goroutines = 8
			perGoroutine = 10
		)
		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			errs []error
		)
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for j := 0; j < perGoroutine; j++ {
					row := store.AuditRow{
						Timestamp:    time.Now().UTC(),
						SessionID:    fmt.Sprintf("sess-concurrent-%d", i),
						UserID:       "user-c",
						SkillID:      "compliance",
						Query:        fmt.Sprintf("concurrent query %d-%d", i, j),
						PromptHash:   "cafebabe",
						ModelVersion: "glm-5.2",
						VerdictJSON:  `{}`,
					}
					if _, err := s.PutAudit(ctx, row); err != nil {
						mu.Lock()
						errs = append(errs, err)
						mu.Unlock()
					}
				}
			}(i)
		}
		wg.Wait()

		for _, err := range errs {
			msg := err.Error()
			assert.False(t, strings.Contains(msg, "locked") || strings.Contains(msg, "SQLITE_BUSY"),
				"busy_timeout=5000 must make writers wait, not fail immediately: %v", err)
		}
		require.Empty(t, errs, "all concurrent PutAudit calls must succeed")

		rows, err := s.QueryAudit(ctx, store.AuditFilter{Limit: 1000})
		require.NoError(t, err)
		assert.Len(t, rows, goroutines*perGoroutine, "every concurrent audit row must persist")
	})

	t.Run("memory_connection_pool", func(t *testing.T) {
		ctx := context.Background()

		mem, err := New(ctx, Config{DSN: ":memory:"})
		require.NoError(t, err, "sqlite.New should open an in-memory store")
		defer func() { _ = mem.Close() }()
		assert.Equal(t, 1, mem.cfg.MaxOpenConns, ":memory: must default to MaxOpenConns=1 (Pitfall 3)")

		file, err := New(ctx, Config{DSN: filepath.Join(t.TempDir(), "mattn-zero-file.db")})
		require.NoError(t, err, "sqlite.New should open a file-backed store")
		defer func() { _ = file.Close() }()
		assert.Equal(t, 4, file.cfg.MaxOpenConns, "file DSN must default to MaxOpenConns=4")
	})
}
