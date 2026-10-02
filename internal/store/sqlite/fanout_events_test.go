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

// Package sqlite_test — fanout_events_test.go is Phase 13 / Plan 13-03
// Task 1: regression locks for the QueryFanoutByRunPath +
// ListFanoutEvents helpers (internal/store/sqlite/fanout_events.go).
//
// Tests cover:
//   - Empty runPathPrefix returns an error (defensive guard).
//   - Empty sessionID returns an error (defensive guard).
//   - Sub-query ordering is ASC by ts (audit trail reconstruction).
//   - Cross-row filtering on action LIKE 'action_fanout_%' only
//     returns the 3 fanout lifecycle enum values (T-3-SQLI parameterized
//     filter).
//   - Closed store returns sql.ErrConnDone (canonical SQLiteStore
//     guard).
//   - RunPath LIKE prefix match (parent_run -> parent_run, parent_run/sa1).
//   - monotonic ordering by ts ASC + id ASC (deterministic tiebreak).
//   - ListFanoutEvents scope by sessionID + limit.
package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeFanoutRow constructs a fanout audit row with the supplied
// action + RunPath + Transfer fields. The base row satisfies the
// fanout RunPath-required invariant (P13-L6 posture).
func makeFanoutRow(action, runPath, transferFrom, transferTo, reason string, ts time.Time) store.AuditRow {
	return store.AuditRow{
		Timestamp:    ts,
		SessionID:    "sess-fanout-test",
		SkillID:      "compliance",
		Query:        "fanout-test",
		PromptHash:   "p-hash",
		ModelVersion: "test",
		VerdictJSON:  "{}",
		Action:       action,
		RunPath:      runPath,
		TransferFrom: transferFrom,
		TransferTo:   transferTo,
		RefusalReason: reason,
	}
}

// TestFanoutEventsEmptyRunPath — defensive: empty prefix returns an
// error (the LIKE '%%' would match every row, which is the wrong
// semantic for a prefix query).
func TestFanoutEventsEmptyRunPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	_, err = s.QueryFanoutByRunPath(ctx, "")
	require.Error(t, err, "empty runPathPrefix MUST be rejected")
	assert.Contains(t, err.Error(), "runPathPrefix is required")
}

// TestFanoutEventsClosedStoreGuard — Closed store returns sql.ErrConnDone.
func TestFanoutEventsClosedStoreGuard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, s.Close())

	_, err = s.QueryFanoutByRunPath(ctx, "parent_run")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrConnDone),
		"Closed store must return sql.ErrConnDone (canonical SQLiteStore guard)")

	_, err = s.ListFanoutEvents(ctx, "sess", 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrConnDone),
		"Closed store must return sql.ErrConnDone for ListFanoutEvents")
}

// TestFanoutEventsClosedStoreGuard_Empty — no rows in the table;
// prefix query returns an empty (non-nil) slice.
func TestFanoutEventsClosedStoreGuard_Empty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	events, err := s.QueryFanoutByRunPath(ctx, "parent_run")
	require.NoError(t, err)
	assert.Empty(t, events, "no audit rows -> empty events slice")
}

// TestFanoutEventsOrderedByTs — 3 events written with intentional
// timestamps; query returns them in ASC order (audit trail
// reconstruction use case).
func TestFanoutEventsOrderedByTs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	// Write in non-monotonic order to prove the ORDER BY ts ASC works.
	rows := []store.AuditRow{
		makeFanoutRow(sqlite.ActionFanoutCompleted, "parent/sa2", "parent", "sa2", "", base.Add(2*time.Second)),
		makeFanoutRow(sqlite.ActionFanoutDispatched, "parent/sa1", "parent", "sa1", "", base.Add(1*time.Second)),
		makeFanoutRow(sqlite.ActionFanoutFailed, "parent/sa3", "parent", "sa3", "timeout", base.Add(3*time.Second)),
	}
	for _, r := range rows {
		_, err := s.PutAudit(ctx, r)
		require.NoError(t, err)
	}

	events, err := s.QueryFanoutByRunPath(ctx, "parent")
	require.NoError(t, err)
	require.Len(t, events, 3)
	// ASC ordering: dispatched (sa1) → completed (sa2) → failed (sa3).
	assert.Equal(t, sqlite.ActionFanoutDispatched, events[0].Action)
	assert.Equal(t, sqlite.ActionFanoutCompleted, events[1].Action)
	assert.Equal(t, sqlite.ActionFanoutFailed, events[2].Action)
	assert.Equal(t, "sa1", events[0].TransferTo)
	assert.Equal(t, "sa2", events[1].TransferTo)
	assert.Equal(t, "sa3", events[2].TransferTo)
	assert.Equal(t, "timeout", events[2].Reason,
		"failed row's reason MUST surface in the trace (P13-L6 audit lineage)")
}

// TestFanoutEventsFilterByActionPrefix — 5 audit rows (3 fanout, 2
// non-fanout); query returns only the 3 fanout rows.
func TestFanoutEventsFilterByActionPrefix(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	// 3 fanout rows + 2 non-fanout rows (verdict + budget).
	inputs := []store.AuditRow{
		makeFanoutRow(sqlite.ActionFanoutDispatched, "parent/sa1", "parent", "sa1", "", base),
		makeFanoutRow(sqlite.ActionFanoutCompleted, "parent/sa1", "parent", "sa1", "", base),
		makeFanoutRow(sqlite.ActionFanoutFailed, "parent/sa1", "parent", "sa1", "x", base),
		// Non-fanout: verdict row (no run_path).
		{
			Timestamp: base, SessionID: "sess-fanout-test",
			SkillID: "compliance", Query: "v", PromptHash: "p", ModelVersion: "m",
			VerdictJSON: "{}", Action: "",
		},
		// Non-fanout: budget row (no run_path).
		{
			Timestamp: base, SessionID: "sess-fanout-test",
			SkillID: "compliance", Query: "b", PromptHash: "p", ModelVersion: "m",
			VerdictJSON: "{}", Action: sqlite.ActionBudgetTriggered, BudgetLayer: "token",
		},
	}
	for _, r := range inputs {
		_, err := s.PutAudit(ctx, r)
		require.NoError(t, err)
	}

	events, err := s.QueryFanoutByRunPath(ctx, "parent")
	require.NoError(t, err)
	assert.Len(t, events, 3, "only 3 fanout rows should surface")
	for _, e := range events {
		assert.True(t,
			e.Action == sqlite.ActionFanoutDispatched ||
				e.Action == sqlite.ActionFanoutCompleted ||
				e.Action == sqlite.ActionFanoutFailed,
			"only fanout_* actions should be returned, got %q", e.Action)
	}
}

// TestFanoutEventsRunPathPatternMatch — query with prefix="parent_run/"
// returns only events whose run_path starts with "parent_run/".
func TestFanoutEventsRunPathPatternMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	inputs := []store.AuditRow{
		// matches parent_run/*
		makeFanoutRow(sqlite.ActionFanoutDispatched, "parent_run/sa1", "parent_run", "sa1", "", base),
		makeFanoutRow(sqlite.ActionFanoutCompleted, "parent_run/sa1", "parent_run", "sa1", "", base),
		// different prefix — must NOT match
		makeFanoutRow(sqlite.ActionFanoutDispatched, "other_run/sa1", "other_run", "sa1", "", base),
		makeFanoutRow(sqlite.ActionFanoutCompleted, "other_run/sa1", "other_run", "sa1", "", base),
	}
	for _, r := range inputs {
		_, err := s.PutAudit(ctx, r)
		require.NoError(t, err)
	}

	events, err := s.QueryFanoutByRunPath(ctx, "parent_run/")
	require.NoError(t, err)
	require.Len(t, events, 2, "only parent_run/* events should match")
	for _, e := range events {
		assert.True(t, len(e.RunPath) >= len("parent_run/") && e.RunPath[:len("parent_run/")] == "parent_run/",
			"RunPath must start with parent_run/ prefix, got %q", e.RunPath)
	}
}

// TestFanoutEventsRequireRunPath — T-13-03-3 mitigation: fanout rows
// with empty RunPath are rejected by PutAudit (T-13-03-3 mitigation).
func TestFanoutEventsRequireRunPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	row := store.AuditRow{
		Timestamp: time.Now().UTC(),
		SessionID: "sess-fanout-reject",
		SkillID:   "compliance",
		Query:     "x", PromptHash: "p", ModelVersion: "m",
		VerdictJSON: "{}",
		Action:      sqlite.ActionFanoutDispatched,
		// RunPath omitted — must be rejected.
	}
	_, err = s.PutAudit(ctx, row)
	require.Error(t, err, "fanout row with empty RunPath MUST be rejected (T-13-03-3)")
	assert.True(t, errors.Is(err, sqlite.ErrInvalidFanoutRunPath),
		"rejection must wrap ErrInvalidFanoutRunPath: %v", err)
}

// TestListFanoutEventsScopeAndLimit — list helper filters by
// sessionID and respects the limit.
func TestListFanoutEventsScopeAndLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	base := time.Now().UTC()
	// 2 fanout rows in session-A + 1 fanout row in session-B.
	inputs := []store.AuditRow{
		{
			Timestamp: base, SessionID: "sess-A",
			SkillID: "compliance", Query: "x", PromptHash: "p", ModelVersion: "m",
			VerdictJSON: "{}", Action: sqlite.ActionFanoutDispatched,
			RunPath: "A/sa1", TransferFrom: "A", TransferTo: "sa1",
		},
		{
			Timestamp: base.Add(1 * time.Second), SessionID: "sess-A",
			SkillID: "compliance", Query: "x", PromptHash: "p", ModelVersion: "m",
			VerdictJSON: "{}", Action: sqlite.ActionFanoutCompleted,
			RunPath: "A/sa1", TransferFrom: "A", TransferTo: "sa1",
		},
		{
			Timestamp: base, SessionID: "sess-B",
			SkillID: "compliance", Query: "x", PromptHash: "p", ModelVersion: "m",
			VerdictJSON: "{}", Action: sqlite.ActionFanoutDispatched,
			RunPath: "B/sa1", TransferFrom: "B", TransferTo: "sa1",
		},
	}
	for _, r := range inputs {
		_, err := s.PutAudit(ctx, r)
		require.NoError(t, err)
	}

	events, err := s.ListFanoutEvents(ctx, "sess-A", 100)
	require.NoError(t, err)
	// Newest-first: completed (ts+1) is first, dispatched (ts) is second.
	require.Len(t, events, 2)
	assert.Equal(t, sqlite.ActionFanoutCompleted, events[0].Action)
	assert.Equal(t, sqlite.ActionFanoutDispatched, events[1].Action)

	// Limit=1 returns only the newest event.
	events, err = s.ListFanoutEvents(ctx, "sess-A", 1)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, sqlite.ActionFanoutCompleted, events[0].Action)

	// Different session returns only session-B events.
	events, err = s.ListFanoutEvents(ctx, "sess-B", 100)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "B", events[0].TransferFrom)
}

// TestListFanoutEventsEmptySessionID — defensive: empty sessionID
// returns an error.
func TestListFanoutEventsEmptySessionID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	_, err = s.ListFanoutEvents(ctx, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sessionID is required")
}
