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

// Package sqlite test — Plan 12-02 Task 2 (TDD) — plan_todos table
// INSERT-only audit + replay substrate. White-box tests inspect the
// table shape via PRAGMA + raw SELECT for the DDL assertion; the
// hermetic INSERT/SELECT tests use the public SQLiteStore surface
// (PutPlanTodo + QueryPlanTodos) so the audit_writer seam is
// exercised end-to-end.
package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// TestPutPlanTodoInsertOnly — Task 2: write 3 todos with op=upsert;
// QueryPlanTodos returns 3 ordered rows. P12-L4 INSERT-only substrate
// for PLAN-02 (live trace) + PLAN-05 (replay query).
func TestPutPlanTodoInsertOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	rows := []sqlite.PlanTodoRow{
		{RunID: "r-1", TodoID: "t1", Content: "collect clauses", ActiveForm: "collecting", Status: "in_progress", Ts: base, Op: sqlite.PlanTodoOpUpsert},
		{RunID: "r-1", TodoID: "t2", Content: "draft answer", Status: "pending", Ts: base.Add(time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: "r-1", TodoID: "t3", Content: "cite sources", Status: "pending", Ts: base.Add(2 * time.Second), Op: sqlite.PlanTodoOpUpsert},
	}
	for _, r := range rows {
		require.NoError(t, s.PutPlanTodo(ctx, r))
	}

	got, err := s.QueryPlanTodos(ctx, "r-1")
	require.NoError(t, err)
	require.Len(t, got, 3, "3 upserts must produce 3 ordered rows")

	assert.Equal(t, "t1", got[0].TodoID)
	assert.Equal(t, "collect clauses", got[0].Content)
	assert.Equal(t, "collecting", got[0].ActiveForm, "activeForm must round-trip")
	assert.Equal(t, "in_progress", got[0].Status)
	assert.Equal(t, sqlite.PlanTodoOpUpsert, got[0].Op)

	assert.Equal(t, "t2", got[1].TodoID)
	assert.Equal(t, "draft answer", got[1].Content)
	assert.Equal(t, "pending", got[1].Status)
	assert.Equal(t, sqlite.PlanTodoOpUpsert, got[1].Op)

	assert.Equal(t, "t3", got[2].TodoID)
	assert.Equal(t, "cite sources", got[2].Content)
	assert.Equal(t, sqlite.PlanTodoOpUpsert, got[2].Op)
}

// TestQueryPlanTodosOrderedByTs — Task 2: writes are intentionally
// out-of-order (rows[1] is inserted FIRST with the highest ts; rows[0]
// is inserted SECOND with the lowest ts); QueryPlanTodos MUST return
// rows in ts ASC order regardless of insert order. The rowid tie
// breaker is exercised by the two upserts at the same ts below.
func TestQueryPlanTodosOrderedByTs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()

	// Out-of-order writes — second row has the earliest ts.
	require.NoError(t, s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "r-order", TodoID: "mid", Content: "middle", Status: "pending",
		Ts: base.Add(time.Second), Op: sqlite.PlanTodoOpUpsert,
	}))
	require.NoError(t, s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "r-order", TodoID: "first", Content: "earliest", Status: "pending",
		Ts: base, Op: sqlite.PlanTodoOpUpsert,
	}))
	require.NoError(t, s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "r-order", TodoID: "last", Content: "latest", Status: "pending",
		Ts: base.Add(2 * time.Second), Op: sqlite.PlanTodoOpUpsert,
	}))

	got, err := s.QueryPlanTodos(ctx, "r-order")
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "first", got[0].TodoID, "earliest ts must sort first")
	assert.Equal(t, "mid", got[1].TodoID)
	assert.Equal(t, "last", got[2].TodoID, "latest ts must sort last")
}

// TestPlanTodosEmptyResult — Task 2: QueryPlanTodos for an unknown
// runID returns an empty (non-nil) slice and no error. The FE renders
// the empty state copy for an unknown / fresh run.
func TestPlanTodosEmptyResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	got, err := s.QueryPlanTodos(ctx, "no-such-run")
	require.NoError(t, err)
	assert.NotNil(t, got, "empty slice MUST be non-nil so callers can range without nil checks")
	assert.Empty(t, got, "unknown runID must return empty slice (no rows)")
}

// TestPlanTodosClosedStoreGuard — Task 2: after Close, both
// PutPlanTodo and QueryPlanTodos return sql.ErrConnDone. P12-L4
// fail-closed posture; the writeTodosAuditedTool surfaces this to
// the LLM run.
func TestPlanTodosClosedStoreGuard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dsn := filepath.Join(t.TempDir(), "closed.db")
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	require.NoError(t, s.Close())

	err = s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "r", TodoID: "t", Content: "x", Status: "pending", Op: sqlite.PlanTodoOpUpsert,
	})
	assert.ErrorIs(t, err, sql.ErrConnDone,
		"PutPlanTodo after Close must return sql.ErrConnDone (P12-L4 fail-closed)")

	_, err = s.QueryPlanTodos(ctx, "r")
	assert.ErrorIs(t, err, sql.ErrConnDone,
		"QueryPlanTodos after Close must return sql.ErrConnDone")
}

// TestPlanTodosRequiresRunIDAndTodoID — Task 2: empty RunID or
// TodoID returns a validation error (the column is NOT NULL but the
// caller might still pass empty string; surface the error early so
// callers don't get a generic SQL constraint error).
func TestPlanTodosRequiresRunIDAndTodoID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	cases := []struct {
		name string
		row  sqlite.PlanTodoRow
	}{
		{"empty RunID", sqlite.PlanTodoRow{RunID: "", TodoID: "t1", Content: "x", Status: "pending", Op: sqlite.PlanTodoOpUpsert}},
		{"empty TodoID", sqlite.PlanTodoRow{RunID: "r1", TodoID: "", Content: "x", Status: "pending", Op: sqlite.PlanTodoOpUpsert}},
		{"both empty", sqlite.PlanTodoRow{RunID: "", TodoID: "", Content: "x", Status: "pending", Op: sqlite.PlanTodoOpUpsert}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.PutPlanTodo(ctx, tc.row)
			assert.Error(t, err, "PutPlanTodo must reject empty identity fields")
		})
	}
}

// TestPlanTodosReplayReconstruction — Task 2 (P12-L4 + P12-L5):
// a write_todos audit chain with 3 distinct todos + 2 'complete' rows
// round-trips through QueryPlanTodos. The PK (run_id, todo_id) is the
// UNIQUE identity — a 'complete' row for the same (run_id, todo_id)
// REPLACES the prior 'upsert' row with the same key (last-write-wins
// for replay consistency per TestPlanTodosDuplicatePKUpsert).
//
// This is the substrate for the 12-04 replay endpoint + 12-07
// PlanReplayView: the FE walks the rows in ts order and renders the
// FINAL state per (run_id, todo_id). The op discriminator tells the
// FE whether the row records a content/status update ('upsert') or a
// "this finished at" annotation ('complete').
func TestPlanTodosReplayReconstruction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	chain := []sqlite.PlanTodoRow{
		// Initial 3-todo plan (upsert).
		{RunID: "replay-1", TodoID: "t1", Content: "retrieve", Status: "pending", Ts: base, Op: sqlite.PlanTodoOpUpsert},
		{RunID: "replay-1", TodoID: "t2", Content: "draft", Status: "pending", Ts: base.Add(time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: "replay-1", TodoID: "t3", Content: "verify", Status: "pending", Ts: base.Add(2 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		// t1 finishes, t2 starts — write_todos Tool emits a fresh
		// upsert per todo with the new status. TestDuplicatePKUpsert
		// already covers the single-key last-write-wins behavior.
		{RunID: "replay-1", TodoID: "t1", Content: "retrieve", Status: "completed", Ts: base.Add(3 * time.Second), Op: sqlite.PlanTodoOpUpsert},
		{RunID: "replay-1", TodoID: "t2", Content: "draft", Status: "in_progress", Ts: base.Add(4 * time.Second), Op: sqlite.PlanTodoOpUpsert},
	}
	for _, r := range chain {
		require.NoError(t, s.PutPlanTodo(ctx, r))
	}

	got, err := s.QueryPlanTodos(ctx, "replay-1")
	require.NoError(t, err)
	require.Len(t, got, 3,
		"3 distinct (run_id, todo_id) keys must produce 3 rows (PK UNIQUE collapses same-key writes)")

	// Final-state reconstruction: the LATEST write per todo_id wins.
	// (INSERT OR REPLACE updates content/status/ts.)
	finalByTodo := map[string]sqlite.PlanTodoRow{}
	for _, r := range got {
		finalByTodo[r.TodoID] = r
	}

	// t1: last write was upsert with status=completed at base+3s.
	assert.Equal(t, "completed", finalByTodo["t1"].Status,
		"t1 final status is 'completed' (the upsert at base+3s REPLACED the original 'pending' upsert)")
	assert.Equal(t, base.Add(3*time.Second).Unix(), finalByTodo["t1"].Ts.Unix())
	assert.Equal(t, sqlite.PlanTodoOpUpsert, finalByTodo["t1"].Op)

	// t2: last write was upsert with status=in_progress at base+4s.
	assert.Equal(t, "in_progress", finalByTodo["t2"].Status)
	assert.Equal(t, base.Add(4*time.Second).Unix(), finalByTodo["t2"].Ts.Unix())
	assert.Equal(t, sqlite.PlanTodoOpUpsert, finalByTodo["t2"].Op)

	// t3: never updated; original 'pending' upsert survives.
	assert.Equal(t, "pending", finalByTodo["t3"].Status)
	assert.Equal(t, base.Add(2*time.Second).Unix(), finalByTodo["t3"].Ts.Unix())
	assert.Equal(t, "verify", finalByTodo["t3"].Content)

	// Now exercise the op='complete' discriminator on a NEW
	// (run_id, todo_id) key — the row must round-trip with the
	// PlanTodoOpComplete op (NOT collapse to 'upsert').
	require.NoError(t, s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "replay-1", TodoID: "t-finished", Content: "separate key",
		Status: "completed", Ts: base.Add(10 * time.Second), Op: sqlite.PlanTodoOpComplete,
	}))
	got, err = s.QueryPlanTodos(ctx, "replay-1")
	require.NoError(t, err)
	require.Len(t, got, 4, "the new (run_id, todo_id) key produces a 4th row")

	var completeRows []sqlite.PlanTodoRow
	for _, r := range got {
		if r.Op == sqlite.PlanTodoOpComplete {
			completeRows = append(completeRows, r)
		}
	}
	require.Len(t, completeRows, 1,
		"op='complete' rows must keep their discriminator intact (NOT silently coerced to 'upsert')")
	assert.Equal(t, "t-finished", completeRows[0].TodoID)
	assert.Equal(t, base.Add(10*time.Second).Unix(), completeRows[0].Ts.Unix(),
		"complete ts must round-trip (FE renders 'completed at X')")
}

// TestPlanTodosDuplicatePKUpsert — Task 2: a second PutPlanTodo with
// the same (RunID, TodoID) MUST REPLACE the prior row, not create a
// second row (INSERT OR REPLACE in the SQL constant enforces this).
// Replay reconstruction depends on "last write wins" so the FE shows
// the latest content/activeForm/status snapshot.
func TestPlanTodosDuplicatePKUpsert(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()

	// First upsert: t1 = pending.
	require.NoError(t, s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "dup-1", TodoID: "t1", Content: "original content", ActiveForm: "starting",
		Status: "pending", Ts: base, Op: sqlite.PlanTodoOpUpsert,
	}))

	// Second upsert: same (RunID, TodoID), new content + status + ts.
	require.NoError(t, s.PutPlanTodo(ctx, sqlite.PlanTodoRow{
		RunID: "dup-1", TodoID: "t1", Content: "updated content", ActiveForm: "mid-flight",
		Status: "in_progress", Ts: base.Add(time.Second), Op: sqlite.PlanTodoOpUpsert,
	}))

	got, err := s.QueryPlanTodos(ctx, "dup-1")
	require.NoError(t, err)
	require.Len(t, got, 1,
		"INSERT OR REPLACE must NOT create a second row; the (RunID, TodoID) PK keeps the count at 1")

	assert.Equal(t, "updated content", got[0].Content,
		"latest content must win (last-write-wins for replay consistency)")
	assert.Equal(t, "mid-flight", got[0].ActiveForm)
	assert.Equal(t, "in_progress", got[0].Status,
		"latest status must win (the FE renders the latest snapshot)")
	assert.Equal(t, sqlite.PlanTodoOpUpsert, got[0].Op,
		"op discriminator must NOT flip on a same-key upsert")
	assert.Equal(t, base.Add(time.Second).Unix(), got[0].Ts.Unix(),
		"latest ts must win")
}

// TestPlanTodosDDL — Task 2 white-box: after migrate(), the
// plan_todos table exists with the 7 expected columns in canonical
// order. Mirrors TestAuditMetaDDL's posture.
func TestPlanTodosDDL(t *testing.T) {
	expectedCols := []string{
		"run_id",
		"todo_id",
		"content",
		"active_form",
		"status",
		"ts",
		"op",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	rows, err := s.DB().QueryContext(ctx, `PRAGMA table_info(plan_todos)`)
	require.NoError(t, err, "PRAGMA table_info(plan_todos)")
	defer rows.Close()

	got := []string{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    any
			pk      int
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		got = append(got, name)
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, expectedCols, got,
		"plan_todos must have the expected 7 columns in canonical order (P12-L5 + P12-L4)")
}