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

// Package sqlite — fanout_events.go is Phase 13 / Plan 13-03's
// FAN-04 SC-4 audit RunPath/Transfer query helper. It exposes two
// query helpers that operate on the audit table (the source of truth)
// and filter on the new action_fanout_* enum values + the run_path
// column.
//
// The audit table is the primary substrate (P13-L6 — additive ALTER
// COLUMN migration; the audit row count grows by 3 rows per sub-agent
// per Run, and a JOIN on a separate table would be heavier than a
// WHERE filter on a nullable TEXT column indexed by
// idx_audit_run_path).
//
// All values are `?` placeholders (T-3-SQLI). The query helper
// methods are read-only (SELECT only) — T-3-AUDIT INSERT-only
// invariant protects the audit table.
package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/skillforge/skillforge/internal/store"
)

// queryFanoutByRunPathSQL is the FAN-04 SC-4 audit trace reconstruction
// query. Filters on action LIKE 'action_fanout_%' (the 3 fanout
// lifecycle enum values) AND run_path LIKE <prefix> (the slash-
// separated agent path). The idx_audit_run_path index keeps the LIKE
// prefix scan O(log N) instead of full table scan.
//
// SELECT column order matches the store.FanoutEvent struct order so the
// Scan call maps positionally.
const queryFanoutByRunPathSQL = `
SELECT id, ts, action, run_path, transfer_from, transfer_to, reason_synth
FROM (
    SELECT id,
           ts,
           action,
           run_path,
           transfer_from,
           transfer_to,
           CASE
               WHEN action = 'action_fanout_failed' THEN refusal_reason
               ELSE ''
           END AS reason_synth
    FROM audit
    WHERE action LIKE 'action_fanout_%'
      AND run_path LIKE ?
)
ORDER BY ts ASC, id ASC
`

// QueryFanoutByRunPath returns the ordered list of fanout lifecycle
// events for the supplied runPathPrefix. The query is a LIKE prefix
// match: runPathPrefix="parent_run" matches "parent_run",
// "parent_run/sa1", "parent_run/sa2", etc. Caller is expected to
// supply the parent run's ID (the slash between parent and sub_query
// is optional — the query matches both with and without).
//
// Performance: idx_audit_run_path keeps the prefix scan O(log N) for
// the run_path match; the action LIKE 'action_fanout_%' filter is
// row-level after the index hit. Returns an empty (non-nil) slice when
// no events match.
//
// Returns sql.ErrConnDone when the store is closed (matches the
// canonical SQLiteStore error posture).
func (s *SQLiteStore) QueryFanoutByRunPath(ctx context.Context, runPathPrefix string) ([]store.FanoutEvent, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if runPathPrefix == "" {
		return nil, errors.New("sqlite.QueryFanoutByRunPath: runPathPrefix is required")
	}
	rows, err := s.db.QueryContext(ctx, queryFanoutByRunPathSQL, runPathPrefix+"%")
	if err != nil {
		return nil, fmtError("QueryFanoutByRunPath", err)
	}
	defer rows.Close()

	var out []store.FanoutEvent
	for rows.Next() {
		var e store.FanoutEvent
		if err := rows.Scan(
			&e.ID, &e.Ts, &e.Action, &e.RunPath, &e.TransferFrom, &e.TransferTo, &e.Reason,
		); err != nil {
			return nil, fmtError("QueryFanoutByRunPath.Scan", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("QueryFanoutByRunPath.Rows", err)
	}
	if out == nil {
		out = []store.FanoutEvent{}
	}
	return out, nil
}

// queryFanoutBySessionSQL is the AgentInspector timeline helper. It
// returns ALL fanout lifecycle events for a session (regardless of
// run_path), newest-first. The session_id column is indexed by
// idx_audit_session_id (Phase 3 baseline).
const queryFanoutBySessionSQL = `
SELECT id, ts, action, run_path, transfer_from, transfer_to, reason_synth
FROM (
    SELECT id,
           ts,
           action,
           run_path,
           transfer_from,
           transfer_to,
           CASE
               WHEN action = 'action_fanout_failed' THEN refusal_reason
               ELSE ''
           END AS reason_synth
    FROM audit
    WHERE session_id = ?
      AND action LIKE 'action_fanout_%'
)
ORDER BY ts DESC, id DESC
LIMIT ?
`

// ListFanoutEvents returns fanout lifecycle events for the supplied
// sessionID, newest-first. limit <= 0 falls back to 100 (matches
// QueryAudit's default). The session-scoped query is used by the
// AgentInspector timeline (Phase 5 SC-4) to render the fanout trace
// alongside the existing decision timeline.
func (s *SQLiteStore) ListFanoutEvents(ctx context.Context, sessionID string, limit int) ([]store.FanoutEvent, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if sessionID == "" {
		return nil, errors.New("sqlite.ListFanoutEvents: sessionID is required")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, queryFanoutBySessionSQL, sessionID, limit)
	if err != nil {
		return nil, fmtError("ListFanoutEvents", err)
	}
	defer rows.Close()

	var out []store.FanoutEvent
	for rows.Next() {
		var e store.FanoutEvent
		if err := rows.Scan(
			&e.ID, &e.Ts, &e.Action, &e.RunPath, &e.TransferFrom, &e.TransferTo, &e.Reason,
		); err != nil {
			return nil, fmtError("ListFanoutEvents.Scan", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListFanoutEvents.Rows", err)
	}
	if out == nil {
		out = []store.FanoutEvent{}
	}
	return out, nil
}
