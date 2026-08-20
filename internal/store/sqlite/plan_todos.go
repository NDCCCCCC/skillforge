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
	"database/sql"
	"errors"
	"time"
)

// plan_todos.go — Phase 12 / Plan 12-02 / P12-L4 + P12-L5.
//
// SEPARATE INSERT-only audit table that backs the DeepAgent
// write_todos Tool (internal/agent/deep_audit.go::writeTodosAuditedTool).
// This table is INTENTIONALLY DISTINCT from the canonical audit.action
// enum — replay is the hot path per PlanReplayView render (12-07), and
// a dedicated table keeps the SELECT lean (no WHERE filter, no audit
// row JOINs). The cross-reference for AuditTimelineTab (12-07) lives
// in the audit.action enum extension (action_plan_todo_upsert /
// action_plan_todo_complete); the PRIMARY substrate for replay is this
// table.
//
// P12-L4 INSERT-only posture: PutPlanTodo is the sole write path
// (mirrors Phase 11 audit.go's putAuditSQL pattern). There is
// deliberately no Update / Delete method — the schema is INSERT + SELECT
// only (T-3-AUDIT anti-tampering). Every value is a `?` placeholder
// (T-3-SQLI: no fmt.Sprintf with input).
//
// P12-L5 fail-closed posture: store closed → sql.ErrConnDone; empty
// RunID / TodoID → validation error. NO retry layer, NO swallow (the
// writeTodosAuditedTool surfaces the error to the LLM run).
//
// PRIMARY KEY (run_id, todo_id) enables idempotent upserts via
// `INSERT OR REPLACE` — replay reconstruction is "last write wins"
// (TestPlanTodosDuplicatePKUpsert pins this contract).

// PlanTodoOp is the discriminator on plan_todos.op. The replay query
// (QueryPlanTodos) returns rows in ts ASC order; the FE replay
// renderer (12-07 PlanReplayView) flips a todo to "completed" on the
// first 'complete' row it sees for that (run_id, todo_id) and uses
// 'upsert' rows for content / activeForm / status updates.
type PlanTodoOp string

// PlanTodoOpUpsert is the write-side operation: insert or replace a
// todo row (write_todos Tool emit). The PRIMARY KEY (run_id, todo_id)
// UNIQUE constraint turns a duplicate-key INSERT into a UNIQUE
// constraint error — the writer uses INSERT OR REPLACE for the
// last-write-wins semantics (TestPlanTodosDuplicatePKUpsert pins).
const PlanTodoOpUpsert PlanTodoOp = "upsert"

// PlanTodoOpComplete is the completion-side operation: a todo
// transitioned from {pending, in_progress} to completed. Recorded
// separately so replay reconstruction can show the "this finished at"
// timestamp without losing the last upsert's content/activeForm
// snapshot.
const PlanTodoOpComplete PlanTodoOp = "complete"

// PlanTodoRow is the typed input/output for plan_todos. Mirrors the
// SkillForge-owned wire.PlanTodo struct (1:1 field mapping, with Ts
// materialized as time.Time on the Go side and persisted as
// epoch-seconds INTEGER on the wire side).
type PlanTodoRow struct {
	RunID      string
	TodoID     string
	Content    string
	ActiveForm string
	Status     string
	Ts         time.Time
	Op         PlanTodoOp
}

// putPlanTodoSQL is the single INSERT path for plan_todos. Mirrors
// putAuditSQL's posture: ?-placeholders only (T-3-SQLI); no
// row-modification statement exists anywhere in the package
// (T-3-AUDIT INSERT-only invariant). PRIMARY KEY UNIQUE on
// (run_id, todo_id) makes a duplicate INSERT fail with a UNIQUE
// constraint error — the writer must use INSERT OR REPLACE for
// idempotent upserts (see PutPlanTodo implementation).
const putPlanTodoSQL = `
INSERT OR REPLACE INTO plan_todos (
    run_id, todo_id, content, active_form, status, ts, op
) VALUES (?, ?, ?, ?, ?, ?, ?)
`

// queryPlanTodosSQL is the replay query. ORDER BY ts ASC keeps the
// reconstruction deterministic (12-07 PlanReplayView walks rows in
// this order; ts ties break by sqlite rowid implicitly).
const queryPlanTodosSQL = `
SELECT run_id, todo_id, content, active_form, status, ts, op
FROM plan_todos
WHERE run_id = ?
ORDER BY ts ASC, rowid ASC
`

// PutPlanTodo appends (or replaces) an audit row for one todo in a
// run. P12-L4 INSERT-only / fail-closed: store closed →
// sql.ErrConnDone; empty RunID / TodoID → validation error; SQL
// failure → wrapped driver error.
//
// The PRIMARY KEY (run_id, todo_id) UNIQUE constraint enables
// idempotent upserts via INSERT OR REPLACE — re-writing the same todo
// updates content / activeForm / status / ts with last-write-wins
// semantics. TestPlanTodosDuplicatePKUpsert pins this contract: a
// second PutPlanTodo with the same (RunID, TodoID) REPLACES the prior
// row, not creates a second row (replay reconstruction needs the
// "latest content wins" behavior).
//
// The audit row is the durable substrate for PLAN-05 (replay
// reconstruction in 12-04 endpoint + 12-07 UI). The SSE chunk emit is
// the LIVE trace for PLAN-02 — both substrates share the same source
// row, written in one PUT call from writeTodosAuditedTool.
//
// Ts is auto-stamped to time.Now().UTC() when zero-valued so callers
// that pass a freshly-allocated struct get a server-side timestamp
// (no client clock trust required).
func (s *SQLiteStore) PutPlanTodo(ctx context.Context, row PlanTodoRow) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	if row.RunID == "" || row.TodoID == "" {
		return errors.New("sqlite.PutPlanTodo: RunID and TodoID are required")
	}
	if row.Ts.IsZero() {
		row.Ts = time.Now().UTC()
	}
	if _, err := s.db.ExecContext(ctx, putPlanTodoSQL,
		row.RunID,
		row.TodoID,
		row.Content,
		row.ActiveForm,
		row.Status,
		row.Ts.Unix(),
		string(row.Op),
	); err != nil {
		return fmtError("PutPlanTodo", err)
	}
	return nil
}

// QueryPlanTodos returns the full audit chain for runID, oldest-first.
// The 12-04 GET /api/run/:id/plan endpoint and the 12-07 PlanReplayView
// UI consume this; it is the hot path per render (per P12-L5 rationale:
// the dedicated table avoids audit.action filter + JOIN cost).
//
// Returns an empty (non-nil) slice for an unknown runID — callers
// treat empty as "no plan recorded" (the FE renders the empty state).
// Store closed → sql.ErrConnDone.
func (s *SQLiteStore) QueryPlanTodos(ctx context.Context, runID string) ([]PlanTodoRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if runID == "" {
		return nil, errors.New("sqlite.QueryPlanTodos: runID is required")
	}
	rows, err := s.db.QueryContext(ctx, queryPlanTodosSQL, runID)
	if err != nil {
		return nil, fmtError("QueryPlanTodos", err)
	}
	defer rows.Close()

	out := make([]PlanTodoRow, 0)
	for rows.Next() {
		var (
			r     PlanTodoRow
			ts    int64
			opStr string
		)
		if err := rows.Scan(&r.RunID, &r.TodoID, &r.Content, &r.ActiveForm, &r.Status, &ts, &opStr); err != nil {
			return nil, fmtError("QueryPlanTodos.Scan", err)
		}
		r.Ts = time.Unix(ts, 0).UTC()
		r.Op = PlanTodoOp(opStr)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("QueryPlanTodos.Rows", err)
	}
	return out, nil
}