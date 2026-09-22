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

// Phase 39 / Plan 39-05 Task 1 — dead-letter ledger (D-DL-01) tests. This
// file lives in the external ecl_test package (not package ecl) because it
// exercises the REAL sqlite FailureStore implementation, and the sqlite
// package imports ecl for its compile-time conformance check — an external
// test package is the only import direction that avoids a test cycle.
// The manager's map-backed fake FailureStore lives in manager_test.go.
package ecl_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// newFailureStore opens a file-backed SQLite store in a temp dir (same
// construction discipline as the white-box sqlite tests) and returns the
// store typed as the narrow FailureStore seam.
func newFailureStore(t *testing.T) (*sqlite.SQLiteStore, ecl.FailureStore) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "ecl-failures.db")
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, s
}

// TestFailureStoreSchemaColumns asserts the ecl_failures DDL shape: the
// DDL pinned in 39-05 <interfaces> (note_id PK / stage / attempt_count /
// last_error / status / updated_at), in DDL order.
func TestFailureStoreSchemaColumns(t *testing.T) {
	s, _ := newFailureStore(t)

	rows, err := s.DB().Query("SELECT name, pk FROM pragma_table_info('ecl_failures')")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	type col struct {
		name string
		pk   int
	}
	var got []col
	for rows.Next() {
		var c col
		require.NoError(t, rows.Scan(&c.name, &c.pk))
		got = append(got, c)
	}
	require.NoError(t, rows.Err())

	want := []string{"note_id", "stage", "attempt_count", "last_error", "status", "updated_at"}
	require.Len(t, got, len(want))
	for i, w := range want {
		assert.Equal(t, w, got[i].name, "column %d drift breaks positional Scans", i)
	}
	assert.Equal(t, 1, got[0].pk, "note_id must be the PRIMARY KEY")
}

// TestFailureStoreLifecycle covers the upsert-insert then upsert-update
// transitions (pending → dead → resolved) — the mutable-status lifecycle
// D-DL-01 requires — plus ListFailures ordering and the GetFailure miss.
func TestFailureStoreLifecycle(t *testing.T) {
	_, fs := newFailureStore(t)

	// Insert: first failure of note 001 at extract.
	require.NoError(t, fs.UpsertFailure(ecl.Failure{
		NoteID:       "001",
		Stage:        "extract",
		AttemptCount: 1,
		LastError:    "llm: connection refused",
		Status:       ecl.FailurePending,
		UpdatedAt:    "2026-09-22T00:00:01Z",
	}))

	got, err := fs.GetFailure("001")
	require.NoError(t, err)
	assert.Equal(t, ecl.Failure{
		NoteID:       "001",
		Stage:        "extract",
		AttemptCount: 1,
		LastError:    "llm: connection refused",
		Status:       ecl.FailurePending,
		UpdatedAt:    "2026-09-22T00:00:01Z",
	}, got)

	// Update: attempt 3 failed → dead.
	require.NoError(t, fs.UpsertFailure(ecl.Failure{
		NoteID:       "001",
		Stage:        "extract",
		AttemptCount: 3,
		LastError:    "llm: connection refused",
		Status:       ecl.FailureDead,
		UpdatedAt:    "2026-09-22T00:00:21Z",
	}))
	got, err = fs.GetFailure("001")
	require.NoError(t, err)
	assert.Equal(t, ecl.FailureDead, got.Status)
	assert.Equal(t, 3, got.AttemptCount)

	// Update: manual retry succeeded → resolved (still one row, same PK).
	require.NoError(t, fs.UpsertFailure(ecl.Failure{
		NoteID:       "001",
		Stage:        "milvus-load",
		AttemptCount: 1,
		LastError:    "",
		Status:       ecl.FailureResolved,
		UpdatedAt:    "2026-09-22T00:01:00Z",
	}))
	rows, err := fs.ListFailures()
	require.NoError(t, err)
	require.Len(t, rows, 1, "upsert must not grow the ledger — one row per note")
	assert.Equal(t, ecl.FailureResolved, rows[0].Status)

	// Second note, then ListFailures ordering: newest updated_at first.
	require.NoError(t, fs.UpsertFailure(ecl.Failure{
		NoteID:       "002",
		Stage:        "kg-load",
		AttemptCount: 3,
		LastError:    "kg: note locked",
		Status:       ecl.FailureDead,
		UpdatedAt:    "2026-09-22T00:00:30Z",
	}))
	rows, err = fs.ListFailures()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "001", rows[0].NoteID, "newest updated_at first")
	assert.Equal(t, "002", rows[1].NoteID)

	// Unknown note → wrapped not-found error (errors.Is-able sentinel).
	_, err = fs.GetFailure("999")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sqlite.ErrNoFailureRecord),
		"miss must wrap the sentinel, got: %v", err)
}

// TestECLFailuresRestartPersistence proves D-DL-01's restart requirement:
// dead rows survive close + reopen from the same DSN, and the schemaSQL
// migration is idempotent across the two New() calls.
func TestECLFailuresRestartPersistence(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "ecl-restart.db")
	ctx := context.Background()

	s1, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	require.NoError(t, s1.UpsertFailure(ecl.Failure{
		NoteID:       "039",
		Stage:        "extract",
		AttemptCount: 3,
		LastError:    "llm: gateway timeout",
		Status:       ecl.FailureDead,
		UpdatedAt:    "2026-09-22T00:00:00Z",
	}))
	require.NoError(t, s1.Close())

	// Reopen from the same DSN — migrate() runs again (idempotent) and the
	// row must still be there.
	s2, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()

	got, err := s2.GetFailure("039")
	require.NoError(t, err)
	assert.Equal(t, ecl.FailureDead, got.Status)
	assert.Equal(t, "extract", got.Stage)
	assert.Equal(t, 3, got.AttemptCount)
	assert.Equal(t, "llm: gateway timeout", got.LastError)

	rows, err := s2.ListFailures()
	require.NoError(t, err)
	assert.Len(t, rows, 1, "restart must not duplicate ledger rows")
}
